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

package tasks

import (
	"reflect"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/issue_trace/models"
	"github.com/apache/incubator-devlake/plugins/issue_trace/utils"
)

type IssueStatusWithoutChangeLog struct {
	common.RawDataOrigin
	IssueId        string
	Status         string
	OriginalStatus string
	CreatedDate    time.Time
}

type StatusChangeLogResult struct {
	common.RawDataOrigin
	IssueId           string
	LogCreatedDate    time.Time
	IssueCreatedDate  time.Time
	FromValue         string
	ToValue           string
	OriginalFromValue string
	OriginalToValue   string
}

var ConvertIssueStatusHistoryMeta = plugin.SubTaskMeta{
	Name:             "ConvertIssueStatusHistory",
	EntryPoint:       ConvertIssueStatusHistory,
	EnabledByDefault: true,
	Description:      "Convert changelogs to issue status history",
}

func ConvertIssueStatusHistory(taskCtx plugin.SubTaskContext) errors.Error {
	logger := taskCtx.GetLogger()
	options := taskCtx.GetData().(*TaskData)
	scopeIds := options.ScopeIds

	db := taskCtx.GetDal()
	inserter := helper.NewBatchSaveDivider(taskCtx, utils.BATCH_SIZE, "", "")
	defer inserter.Close()
	batchInserter, err := inserter.ForType(reflect.TypeOf(&models.IssueStatusHistory{}))
	if err != nil {
		logger.Error(err, "Failed to create batch insert")
		return err
	}

	// handle issues not appeared in change logs (hasn't changed)
	logger.Info("get issues not appeared in change logs, board %s", scopeIds)
	now := time.Now()
	clauses := []dal.Clause{
		dal.Select("issues.id AS issue_id, issues.status, issues.original_status, issues.created_date," +
			"board_issues.board_id as _raw_data_params, issues._raw_data_table as _raw_data_table, issues._raw_data_id as _raw_data_id"),
		dal.From("issues"),
		dal.Join("INNER JOIN board_issues ON board_issues.issue_id = issues.id"),
		dal.Join("LEFT JOIN issue_changelogs ON issue_changelogs.issue_id=issues.id AND issue_changelogs.field_name='status'"),
		dal.Where("board_issues.board_id in ? AND issue_changelogs.field_name IS NULL", scopeIds),
	}
	statusFromIssueCursor, err := db.Cursor(clauses...)
	if err != nil {
		logger.Error(err, "Failed to query issue status")
		return err
	}
	defer statusFromIssueCursor.Close()
	statusFromIssueConvertor, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:    taskCtx,
			Params: options.Options,
			Table:  "issues",
		},
		InputRowType: reflect.TypeOf(IssueStatusWithoutChangeLog{}),
		Input:        statusFromIssueCursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			if ctxErr := utils.CheckCancel(taskCtx); ctxErr != nil {
				return nil, ctxErr
			}
			issue := inputRow.(*IssueStatusWithoutChangeLog)
			var statusSeconds int64 = 0
			if now.After(issue.CreatedDate) {
				statusSeconds = now.Unix() - issue.CreatedDate.Unix()
			}
			err = batchInserter.Add(&models.IssueStatusHistory{
				NoPKModel: common.NoPKModel{
					RawDataOrigin: common.RawDataOrigin{
						RawDataTable:  issue.RawDataTable,
						RawDataParams: issue.RawDataParams,
						RawDataId:     issue.RawDataId,
					},
				},
				IssueId:           issue.IssueId,
				Status:            issue.Status,
				OriginalStatus:    issue.OriginalStatus,
				StartDate:         issue.CreatedDate,
				EndDate:           &now,
				StatusTimeMinutes: int32(statusSeconds / 60),
				IsFirstStatus:     true,
			})
			return nil, err
		},
	})
	if err != nil {
		logger.Error(err, "Failed to create statusFromIssueConvertor")
		return err
	}
	err = statusFromIssueConvertor.Execute()
	if err != nil {
		logger.Error(err, "Failed to execute statusFromIssueConvertor")
		return err
	}

	// handle issues with changelogs
	logger.Info("get issue status change log, board %s", scopeIds)
	clauses = []dal.Clause{
		dal.Select("issue_changelogs.issue_id, issue_changelogs.created_date AS log_created_date, " +
			"issue_changelogs.to_value, issue_changelogs.original_to_value, issue_changelogs.from_value, " +
			"issue_changelogs.original_from_value, issues.created_date AS issue_created_date," +
			"board_issues.board_id as _raw_data_params, issue_changelogs._raw_data_table as _raw_data_table, issue_changelogs._raw_data_id as _raw_data_id"),
		dal.From("issue_changelogs"),
		dal.Join("INNER JOIN issues ON issues.id = issue_changelogs.issue_id"),
		dal.Join("INNER JOIN board_issues ON board_issues.issue_id = issue_changelogs.issue_id"),
		dal.Where("board_issues.board_id in ? AND issue_changelogs.field_name = 'status'", scopeIds),
		dal.Orderby("issue_changelogs.issue_id ASC, issue_changelogs.created_date ASC"),
	}
	statusFromChangelogCursor, err := db.Cursor(clauses...)
	if err != nil {
		logger.Error(err, "Failed to query status changelogs")
		return err
	}
	defer statusFromChangelogCursor.Close()

	var currentIssue string
	var currentLogs = make([]*StatusChangeLogResult, 0)

	statusFromChangelogConvertor, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:    taskCtx,
			Params: options.Options,
			Table:  "issue_changelogs",
		},
		InputRowType: reflect.TypeOf(StatusChangeLogResult{}),
		Input:        statusFromChangelogCursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			if ctxErr := utils.CheckCancel(taskCtx); ctxErr != nil {
				return nil, ctxErr
			}
			logRow := inputRow.(*StatusChangeLogResult)
			if logRow.IssueId != currentIssue { // reach new issue section
				if len(currentLogs) > 0 {
					historyRows := buildStatusHistoryRecords(currentLogs)
					for _, r := range historyRows {
						if r.EndDate != nil {
							var seconds int64 = 0
							if r.EndDate.After(r.StartDate) {
								seconds = r.EndDate.Unix() - r.StartDate.Unix()
							}
							r.StatusTimeMinutes = int32(seconds / 60)
						}
						err = batchInserter.Add(r)
						if err != nil {
							return nil, err
						}
					}
				}
				currentIssue = logRow.IssueId
				currentLogs = make([]*StatusChangeLogResult, 0)
			}
			currentLogs = append(currentLogs, logRow)
			return nil, nil
		},
	})
	if err != nil {
		logger.Error(err, "Failed to create statusFromChangelogConvertor")
		return err
	}
	err = statusFromChangelogConvertor.Execute()
	if err != nil {
		logger.Error(err, "Failed to execute statusFromChangelogConvertor")
		return err
	}

	if len(currentLogs) > 0 {
		historyRows := buildStatusHistoryRecords(currentLogs)
		for _, r := range historyRows {
			if r.EndDate != nil {
				var seconds int64 = 0
				if r.EndDate.After(r.StartDate) {
					seconds = r.EndDate.Unix() - r.StartDate.Unix()
				}
				r.StatusTimeMinutes = int32(seconds / 60)
			}
			err = batchInserter.Add(r)
			if err != nil {
				return err
			}
		}
	}
	logger.Info("issues status history converted successfully")
	return nil
}

func buildStatusHistoryRecords(logs []*StatusChangeLogResult) []*models.IssueStatusHistory {
	if len(logs) == 0 {
		return make([]*models.IssueStatusHistory, 0)
	}
	firstChangelog := logs[0]

	result := []*models.IssueStatusHistory{
		{
			NoPKModel: common.NoPKModel{
				RawDataOrigin: common.RawDataOrigin{
					RawDataTable:  firstChangelog.RawDataTable,
					RawDataParams: firstChangelog.RawDataParams,
					RawDataId:     firstChangelog.RawDataId,
				},
			},
			IssueId:        firstChangelog.IssueId,
			Status:         firstChangelog.FromValue,
			OriginalStatus: firstChangelog.OriginalFromValue,
			StartDate:      firstChangelog.IssueCreatedDate,
			EndDate:        &firstChangelog.LogCreatedDate,
			IsFirstStatus:  true,
		},
	}
	for _, row := range logs {
		lastResult := len(result) - 1
		lastStatus := result[lastResult].Status
		if row.ToValue != "" {
			lastStatus = row.ToValue
		}
		result = append(result, &models.IssueStatusHistory{
			NoPKModel: common.NoPKModel{
				RawDataOrigin: common.RawDataOrigin{
					RawDataTable:  row.RawDataTable,
					RawDataParams: row.RawDataParams,
					RawDataId:     row.RawDataId,
				},
			},
			IssueId:        row.IssueId,
			Status:         lastStatus,
			OriginalStatus: row.OriginalToValue,
			StartDate:      row.LogCreatedDate,
			EndDate:        nil,
		})
		result[lastResult].EndDate = &row.LogCreatedDate
	}
	now := time.Now()
	result[len(result)-1].EndDate = &now
	result[len(result)-1].IsCurrentStatus = true
	return result
}
