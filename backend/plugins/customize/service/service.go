/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/qa"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/helpers/pluginhelper"
	customizeModels "github.com/apache/incubator-devlake/plugins/customize/models"
)

// Service wraps database operations
type Service struct {
	dal         dal.Dal
	nameChecker *regexp.Regexp
}

func NewService(dal dal.Dal) *Service {
	return &Service{dal: dal, nameChecker: regexp.MustCompile(`^x_[a-zA-Z0-9_]{0,50}$`)}
}

// GetFields returns all the fields of the table
func (s *Service) GetFields(table string) ([]customizeModels.CustomizedField, errors.Error) {
	// the customized fields created before v0.16.0 were not recorded in the table `_tool_customized_field`, we should take care of them
	columns, err := s.dal.GetColumns(&customizeModels.Table{Name: table}, func(columnMeta dal.ColumnMeta) bool {
		return true
	})
	if err != nil {
		return nil, errors.Default.Wrap(err, "GetColumns error")
	}
	ff, err := s.getCustomizedFields(table)
	if err != nil {
		return nil, err
	}
	fieldMap := make(map[string]customizeModels.CustomizedField)
	for _, f := range ff {
		fieldMap[f.ColumnName] = f
	}
	var result []customizeModels.CustomizedField
	for _, col := range columns {
		// original fields
		if !strings.HasPrefix(col.Name(), "x_") {
			dataType, _ := col.ColumnType()
			result = append(result, customizeModels.CustomizedField{
				TbName:     table,
				ColumnName: col.Name(),
				DataType:   dal.ColumnType(dataType),
			})
			// customized fields
		} else {
			if field, ok := fieldMap[col.Name()]; ok {
				result = append(result, field)
			} else {
				result = append(result, customizeModels.CustomizedField{
					ColumnName: col.Name(),
					DataType:   dal.Varchar,
				})
			}
		}
	}
	return result, nil
}

// checkField checks if the field exist in table
func (s *Service) checkField(table, field string) (bool, errors.Error) {
	if table == "" {
		return false, errors.Default.New("empty table name")
	}
	if !strings.HasPrefix(field, "x_") {
		return false, errors.Default.New("column name should start with `x_`")
	}
	if !s.nameChecker.MatchString(field) {
		return false, errors.Default.New("invalid column name")
	}
	fields, err := s.GetFields(table)
	if err != nil {
		return false, err
	}
	for _, fld := range fields {
		if fld.ColumnName == field {
			return true, nil
		}
	}
	return false, nil
}

// CreateField creates a new column for the table cf.TbName and creates a new record in the table `_tool_customized_fields`
func (s *Service) CreateField(cf *customizeModels.CustomizedField) errors.Error {
	exists, err := s.checkField(cf.TbName, cf.ColumnName)
	if err != nil {
		return err
	}
	if exists {
		return errors.BadInput.New(fmt.Sprintf("the column %s already exists", cf.ColumnName))
	}
	err = s.dal.Create(cf)
	if err != nil {
		return errors.Default.Wrap(err, "create customizedField")
	}
	err = s.dal.AddColumn(cf.TbName, cf.ColumnName, cf.DataType)
	if err != nil {
		return errors.Default.Wrap(err, "AddColumn error")
	}
	return nil
}

// DeleteField deletes the `field` form the `table`
func (s *Service) DeleteField(table, field string) errors.Error {
	exists, err := s.checkField(table, field)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	err = s.dal.DropColumns(table, field)
	if err != nil {
		return errors.Default.Wrap(err, "DropColumn error")
	}
	return s.dal.Delete(&customizeModels.CustomizedField{}, dal.Where("tb_name = ? AND column_name = ?", table, field))
}

// getCustomizedFields returns all the customized fields definitions of the table
func (s *Service) getCustomizedFields(table string) ([]customizeModels.CustomizedField, errors.Error) {
	var result []customizeModels.CustomizedField
	err := s.dal.All(&result, dal.Where("tb_name = ?", table))
	return result, err
}

// ImportIssue import csv file to the table `issues`, and create relations to boards
// issue could exist in multiple boards, so we should only delete an old records when it doesn't belong to another board
func (s *Service) ImportIssue(boardId string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		// not delete accounts data since account may be referenced by others
		err := s.dal.Delete(
			&ticket.Issue{},
			dal.Where("id IN (SELECT issue_id FROM board_issues WHERE board_id=? AND issue_id NOT IN (SELECT issue_id FROM board_issues WHERE board_id!=?))", boardId, boardId),
		)
		if err != nil {
			return err
		}

		err = s.dal.Delete(
			&ticket.IssueLabel{},
			dal.Where("issue_id IN (SELECT issue_id FROM board_issues WHERE board_id=? AND issue_id NOT IN (SELECT issue_id FROM board_issues WHERE board_id!=?))", boardId, boardId),
		)
		if err != nil {
			return err
		}

		err = s.dal.Delete(
			&ticket.BoardIssue{},
			dal.Where("board_id = ?", boardId),
		)
		if err != nil {
			return err
		}
	}
	return s.importCSV(file, boardId, s.issueHandlerFactory(boardId, incremental))
}

// SaveBoard make sure the board exists in table `boards`
func (s *Service) SaveBoard(boardId, boardName string) errors.Error {
	return s.dal.CreateOrUpdate(&ticket.Board{
		DomainEntity: domainlayer.DomainEntity{
			Id: boardId,
		},
		Name: boardName,
		Type: "csv",
	})
}

// ImportIssueCommit imports csv file into the table `issue_commits`
func (s *Service) ImportIssueCommit(boardId string, file io.ReadCloser) errors.Error {
	err := s.dal.Delete(
		&crossdomain.IssueCommit{},
		dal.Where("issue_id IN (SELECT issue_id FROM board_issues WHERE board_id=? AND issue_id NOT IN (SELECT issue_id FROM board_issues WHERE board_id!=?))", boardId, boardId),
	)
	if err != nil {
		return err
	}
	return s.importCSV(file, boardId, s.issueCommitHandler)
}

// ImportIssueRepoCommit imports data to the table `issue_repo_commits` and `issue_commits`
func (s *Service) ImportIssueRepoCommit(boardId string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		// delete old records of the table `issue_repo_commit` and `issue_commit`
		err := s.dal.Delete(
			&crossdomain.IssueRepoCommit{},
			dal.Where("issue_id IN (SELECT issue_id FROM board_issues WHERE board_id=? AND issue_id NOT IN (SELECT issue_id FROM board_issues WHERE board_id!=?))", boardId, boardId),
		)
		if err != nil {
			return err
		}
		err = s.dal.Delete(
			&crossdomain.IssueCommit{},
			dal.Where("issue_id IN (SELECT issue_id FROM board_issues WHERE board_id=? AND issue_id NOT IN (SELECT issue_id FROM board_issues WHERE board_id!=?))", boardId, boardId),
		)
		if err != nil {
			return err
		}
	}
	return s.importCSV(file, boardId, s.issueRepoCommitHandler)
}

// importCSV extract records from csv file, and save them to DB using recordHandler
// the rawDataParams is used to identify the data source,
// the recordHandler is used to handle the record, it should return an error if the record is invalid
// the `created_at` and `updated_at` will be set to the current time
func (s *Service) importCSV(file io.ReadCloser, rawDataParams string, recordHandler func(map[string]interface{}) errors.Error) errors.Error {
	iterator, err := pluginhelper.NewCsvFileIteratorFromFile(file)
	if err != nil {
		return err
	}
	var hasNext bool
	var line int
	now := time.Now()
	for {
		line++
		if hasNext, err = iterator.HasNextWithError(); !hasNext {
			return errors.BadInput.Wrap(err, fmt.Sprintf("error on processing the line:%d", line))
		} else {
			record := iterator.Fetch()
			record["_raw_data_params"] = rawDataParams
			for k, v := range record {
				if v.(string) == "NULL" {
					record[k] = nil
				}
			}
			record["created_at"] = now
			record["updated_at"] = now
			err = recordHandler(record)
			if err != nil {
				return errors.BadInput.Wrap(err, fmt.Sprintf("error on processing the line:%d", line))
			}
		}
	}
}

// createOrUpdateAccount creates or updates an account based on the provided name.
// It returns the account ID and an error if any occurred.
func (s *Service) createOrUpdateAccount(accountName string, rawDataParams string) (string, errors.Error) {
	if accountName == "" {
		return "", nil // Return empty ID if name is empty, no error needed here.
	}
	now := time.Now()
	accountId := fmt.Sprintf("csv:CsvAccount:0:%s", accountName)
	account := &crossdomain.Account{
		DomainEntity: domainlayer.DomainEntity{
			Id: accountId,
			NoPKModel: common.NoPKModel{
				RawDataOrigin: common.RawDataOrigin{
					RawDataParams: rawDataParams,
				},
			},
		},
		FullName:    accountName,
		UserName:    accountName,
		CreatedDate: &now, // FIXME: will update created_date if already exists. to debug, using created_at instead
	}
	err := s.dal.CreateOrUpdate(account)
	if err != nil {
		return "", errors.Default.Wrap(err, fmt.Sprintf("failed to create or update account for %s", accountName))
	}
	return accountId, nil
}

// getStringField extracts a string field from a record map.
// If required is true, it returns an error if the field is missing, nil, empty, or not a string.
// If required is false, it returns an empty string without error if the field is missing or nil,
// but returns an error if the field exists and is not a string.
func getStringField(record map[string]interface{}, fieldName string, required bool) (string, errors.Error) {
	value, ok := record[fieldName]
	if !ok || value == nil {
		if required {
			return "", errors.Default.New(fmt.Sprintf("record without required field %s", fieldName))
		}
		return "", nil // Field missing or nil, but not required
	}

	strValue, ok := value.(string)
	if !ok {
		return "", errors.Default.New(fmt.Sprintf("%s is not a string", fieldName))
	}

	if required && strValue == "" {
		return "", errors.Default.New(fmt.Sprintf("invalid or empty required field %s", fieldName))
	}

	return strValue, nil
}

// issueHandlerFactory returns a handler that save record into `issues`, `board_issues` and `issue_labels` table
func (s *Service) issueHandlerFactory(boardId string, incremental bool) func(record map[string]interface{}) errors.Error {
	return func(record map[string]interface{}) errors.Error {
		var err errors.Error
		id, err := getStringField(record, "id", true)
		if err != nil {
			return err
		}

		// Handle labels
		labels, err := getStringField(record, "labels", false)
		if err != nil {
			return err
		}
		if labels != "" {
			var issueLabels []*ticket.IssueLabel
			appearedLabels := make(map[string]struct{}) // record the labels that have appeared
			for _, label := range strings.Split(labels, ",") {
				label = strings.TrimSpace(label)
				if label == "" {
					continue
				}
				if _, appeared := appearedLabels[label]; !appeared {
					issueLabel := &ticket.IssueLabel{
						IssueId:   id,
						LabelName: label,
						NoPKModel: common.NoPKModel{
							RawDataOrigin: common.RawDataOrigin{
								RawDataParams: boardId,
							},
						},
					}
					issueLabels = append(issueLabels, issueLabel)
					appearedLabels[label] = struct{}{}
				}
			}
			if len(issueLabels) > 0 {
				err = s.dal.CreateOrUpdate(issueLabels)
				if err != nil {
					return err
				}
			}
		}
		delete(record, "labels") // Remove labels from record map as it's handled

		// Handle creator and assignee accounts
		rawDataParams, err := getStringField(record, "_raw_data_params", true)
		if err != nil {
			// This should ideally not happen as it's set in importCSV, but good to check
			return err
		}

		// Handle creator
		creatorName, err := getStringField(record, "creator_name", false)
		if err != nil {
			return err
		}
		creatorId, err := s.createOrUpdateAccount(creatorName, rawDataParams)
		if err != nil {
			return err
		}
		if creatorId != "" {
			record["creator_id"] = creatorId
		}

		// Handle assignee
		assigneeName, err := getStringField(record, "assignee_name", false)
		if err != nil {
			return err
		}
		assigneeId, err := s.createOrUpdateAccount(assigneeName, rawDataParams)
		if err != nil {
			return err
		}
		if assigneeId != "" {
			record["assignee_id"] = assigneeId
		}

		// Handle sprint_ids
		sprintIds, err := getStringField(record, "sprint_ids", false)
		if err != nil {
			return err
		}
		sprints := strings.Split(strings.TrimSpace(sprintIds), ",")
		for _, sprintId := range sprints {
			sprintId = strings.TrimSpace(sprintId)
			if sprintId != "" {
				err = s.dal.CreateOrUpdate(&ticket.SprintIssue{
					SprintId: sprintId,
					IssueId:  id,
				})
				if err != nil {
					return err
				}
			}
		}
		delete(record, "sprint_ids")

		// Handle issues
		err = s.dal.CreateWithMap(&ticket.Issue{}, record)
		if err != nil {
			return err
		}

		// Handle board_issues
		err = s.dal.CreateOrUpdate(&ticket.BoardIssue{
			BoardId: boardId,
			IssueId: id,
		})
		if err != nil {
			return err
		}
		return nil
	}
}

// issueCommitHandler save record into `issue_commits` table
func (s *Service) issueCommitHandler(record map[string]interface{}) errors.Error {
	return s.dal.CreateWithMap(&crossdomain.IssueCommit{}, record)
}

// ImportQaApis imports csv file to the table `qa_apis`
func (s *Service) ImportQaApis(qaProjectId string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		// delete old data associated with this qaProjectId
		err := s.dal.Delete(&qa.QaApi{}, dal.Where("qa_project_id = ?", qaProjectId))
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("failed to delete old qa_apis for qaProjectId %s", qaProjectId))
		}
	}
	return s.importCSV(file, qaProjectId, s.qaApiHandler(qaProjectId))
}

// qaApiHandler saves a record into the `qa_apis` table
func (s *Service) qaApiHandler(qaProjectId string) func(record map[string]interface{}) errors.Error {
	return func(record map[string]interface{}) errors.Error {
		creatorName, err := getStringField(record, "creator_name", false)
		if err != nil {
			return err
		}
		if creatorName != "" {
			creatorId, _ := s.createOrUpdateAccount(creatorName, qaProjectId)
			if creatorId != "" {
				record["creator_id"] = creatorId
			}
		}
		delete(record, "creator_name")
		record["qa_project_id"] = qaProjectId
		return s.dal.CreateWithMap(&qa.QaApi{}, record)
	}
}

// ImportQaTestCases imports csv file to the table `qa_test_cases`
func (s *Service) ImportQaTestCases(qaProjectId, qaProjectName string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		// delete old data associated with this qaProjectId
		// delete qa_test_cases
		err := s.dal.Delete(&qa.QaTestCase{}, dal.Where("qa_project_id = ?", qaProjectId))
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("failed to delete old qa_test_cases for qaProjectId %s", qaProjectId))
		}
		// delete qa_apis
		err = s.dal.Delete(&qa.QaApi{}, dal.Where("qa_project_id = ?", qaProjectId))
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("failed to delete old qa_apis for qaProjectId %s", qaProjectId))
		}
		// delete qa_test_case_executions
		err = s.dal.Delete(&qa.QaTestCaseExecution{}, dal.Where("qa_project_id = ?", qaProjectId))
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("failed to delete old qa_test_case_executions for qaProjectId %s", qaProjectId))
		}
		// never delete data in qa_projects
	}
	// create or update qa_projects
	err := s.dal.CreateOrUpdate(&qa.QaProject{
		DomainEntityExtended: domainlayer.DomainEntityExtended{
			Id: qaProjectId,
		},
		Name: qaProjectName,
	})
	if err != nil {
		return err
	}
	return s.importCSV(file, qaProjectId, s.qaTestCaseHandler(qaProjectId))
}

// qaTestCaseHandler saves a record into the `qa_test_cases` table
func (s *Service) qaTestCaseHandler(qaProjectId string) func(record map[string]interface{}) errors.Error {
	return func(record map[string]interface{}) errors.Error {
		creatorName, _ := getStringField(record, "creator_name", false)
		if creatorName != "" {
			creatorId, _ := s.createOrUpdateAccount(creatorName, qaProjectId)
			record["creator_id"] = creatorId
		}
		// remove fields
		delete(record, "creator_name")
		record["qa_project_id"] = qaProjectId
		return s.dal.CreateWithMap(&qa.QaTestCase{}, record)
	}
}

// ImportQaTestCaseExecutions imports csv file to the table `qa_test_case_executions`
func (s *Service) ImportQaTestCaseExecutions(qaProjectId string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		// delete old data associated with this qaProjectId
		err := s.dal.Delete(&qa.QaTestCaseExecution{}, dal.Where("qa_project_id = ?", qaProjectId))
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("failed to delete old qa_test_case_executions for qaProjectId %s", qaProjectId))
		}
	}
	return s.importCSV(file, qaProjectId, s.qaTestCaseExecutionHandler(qaProjectId))
}

// qaTestCaseExecutionHandler saves a record into the `qa_test_case_executions` table
func (s *Service) qaTestCaseExecutionHandler(qaProjectId string) func(record map[string]interface{}) errors.Error {
	// Assuming qa.QaTestCaseExecution model exists and CreateWithMap is suitable
	return func(record map[string]interface{}) errors.Error {
		creatorName, _ := getStringField(record, "creator_name", false)
		if creatorName != "" {
			creatorId, _ := s.createOrUpdateAccount(creatorName, qaProjectId)
			record["creator_id"] = creatorId
		}
		delete(record, "creator_name")
		record["qa_project_id"] = qaProjectId
		// Set default value for is_invalid if not present or empty in the CSV
		if isInvalid, exists := record["is_invalid"]; !exists || isInvalid == "" {
			record["is_invalid"] = false
		}
		return s.dal.CreateWithMap(&qa.QaTestCaseExecution{}, record)
	}
}

// issueRepoCommitHandlerFactory returns a handler that will populate the `issue_commits` and `issue_repo_commits` table
// the issueCommitsFields is used to filter the fields that should be inserted into the `issue_commits` table
func (s *Service) issueRepoCommitHandler(record map[string]interface{}) errors.Error {
	err := s.dal.CreateWithMap(&crossdomain.IssueRepoCommit{}, record)
	if err != nil {
		return err
	}
	// remove fields that not in table `issue_commits`
	delete(record, "host")
	delete(record, "namespace")
	delete(record, "repo_name")
	delete(record, "repo_url")
	return s.dal.CreateWithMap(&crossdomain.IssueCommit{}, record)
}

// ImportSprint imports csv file into the table `sprints`
func (s *Service) ImportSprint(boardId string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		err := s.dal.Delete(
			&ticket.Sprint{},
			dal.Where("id IN (SELECT sprint_id FROM board_sprints WHERE board_id=? AND sprint_id NOT IN (SELECT sprint_id FROM board_sprints WHERE board_id!=?))", boardId, boardId),
		)
		if err != nil {
			return err
		}
		err = s.dal.Delete(
			&ticket.BoardSprint{},
			dal.Where("board_id = ?", boardId),
		)
		if err != nil {
			return err
		}
	}
	return s.importCSV(file, boardId, s.sprintHandler(boardId))
}

// sprintHandler saves a record into the `sprints` table
func (s *Service) sprintHandler(boardId string) func(record map[string]interface{}) errors.Error {
	return func(record map[string]interface{}) errors.Error {
		id, err := getStringField(record, "id", true)
		if err != nil {
			return err
		}
		record["original_board_id"] = boardId
		err = s.dal.CreateWithMap(&ticket.Sprint{}, record)
		if err != nil {
			return err
		}

		// Create board_sprint relation
		return s.dal.CreateOrUpdate(&ticket.BoardSprint{
			BoardId:  boardId,
			SprintId: id,
		})
	}
}

// ImportIssueChangelog imports csv file into the table `issue_changelogs`
func (s *Service) ImportIssueChangelog(boardId string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		err := s.dal.Delete(
			&ticket.IssueChangelogs{},
			dal.Where("issue_id IN (SELECT issue_id FROM board_issues WHERE board_id=? AND issue_id NOT IN (SELECT issue_id FROM board_issues WHERE board_id!=?))", boardId, boardId),
		)
		if err != nil {
			return err
		}
	}
	return s.importCSV(file, boardId, s.issueChangelogHandler)
}

// issueChangelogHandler saves a record into the `issue_changelogs` table
func (s *Service) issueChangelogHandler(record map[string]interface{}) errors.Error {
	// create account
	authorName, err := getStringField(record, "author_name", false)
	if err != nil {
		return err
	}
	rawDataParams, err := getStringField(record, "_raw_data_params", true)
	if err != nil {
		return err
	}
	if authorName != "" {
		authorId, err := s.createOrUpdateAccount(authorName, rawDataParams)
		if err != nil {
			return err
		}
		record["author_id"] = authorId
	}
	// set field_id = field_name
	fieldName, err := getStringField(record, "field_name", true)
	if err != nil {
		return err
	}
	record["field_id"] = fieldName
	// handle assignee
	if fieldName == "assignee" {
		originalFromValue, err := getStringField(record, "original_from_value", false)
		if err != nil {
			return err
		}
		originalToValue, err := getStringField(record, "original_to_value", false)
		if err != nil {
			return err
		}
		fromId, err := s.createOrUpdateAccount(originalFromValue, rawDataParams)
		if err != nil {
			return err
		}
		record["original_from_value"] = fromId
		toId, err := s.createOrUpdateAccount(originalToValue, rawDataParams)
		if err != nil {
			return err
		}
		record["original_to_value"] = toId
	}
	return s.dal.CreateWithMap(&ticket.IssueChangelogs{}, record)
}

// ImportIssueWorklog imports csv file into the table `issue_worklogs`
func (s *Service) ImportIssueWorklog(boardId string, file io.ReadCloser, incremental bool) errors.Error {
	if !incremental {
		err := s.dal.Delete(
			&ticket.IssueWorklog{},
			dal.Where("issue_id IN (SELECT issue_id FROM board_issues WHERE board_id=? AND issue_id NOT IN (SELECT issue_id FROM board_issues WHERE board_id!=?))", boardId, boardId),
		)
		if err != nil {
			return err
		}
	}
	return s.importCSV(file, boardId, s.issueWorklogHandler)
}

// issueWorklogHandler saves a record into the `issue_worklogs` table
func (s *Service) issueWorklogHandler(record map[string]interface{}) errors.Error {
	// create account
	authorName, err := getStringField(record, "author_name", false)
	if err != nil {
		return err
	}
	if authorName != "" {
		rawDataParams, err := getStringField(record, "_raw_data_params", true)
		if err != nil {
			return err
		}
		authorId, err := s.createOrUpdateAccount(authorName, rawDataParams)
		if err != nil {
			return err
		}
		record["author_id"] = authorId
	}
	delete(record, "author_name")
	return s.dal.CreateWithMap(&ticket.IssueWorklog{}, record)
}
