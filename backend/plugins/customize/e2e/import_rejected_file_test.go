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

package e2e

import (
	"io"
	"strings"
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/qa"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/customize/service"
)

const rejectedQaProjectId = "csv-qa"

// rejectedSnapshot is a table that a rejected import must leave as the valid import left it.
type rejectedSnapshot struct {
	table        dal.Tabler
	csvRelPath   string
	targetFields []string
}

type rejectedImportCase struct {
	name      string
	validFile string
	badFile   string
	importCsv func(svc *service.Service, file io.ReadCloser) errors.Error
	// setUp runs after the valid import, to fill the other tables that the import deletes from.
	setUp     func(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, svc *service.Service)
	snapshots []rejectedSnapshot
}

// A rejected file must change nothing: not the rows the import deletes first, and not the rows
// of the file before the bad line.
func TestImportRejectedFileChangesNothing(t *testing.T) {
	// Line 3 of each *_bad.csv file has fewer fields than its header, and csv.Reader rejects a
	// record whose field count differs from the header's with "wrong number of fields".
	cases := []rejectedImportCase{
		{
			name:      "issues",
			validFile: "raw_tables/rejected_issues_valid.csv",
			badFile:   "raw_tables/rejected_issues_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportIssue("csv-board", file, false)
			},
			snapshots: []rejectedSnapshot{
				{&ticket.Issue{}, "snapshot_tables/rejected_issues_issues.csv", []string{"id", "title", "creator_name", "assignee_name"}},
				{&ticket.IssueLabel{}, "snapshot_tables/rejected_issues_issue_labels.csv", []string{"issue_id", "label_name"}},
				{&ticket.BoardIssue{}, "snapshot_tables/rejected_issues_board_issues.csv", []string{"board_id", "issue_id"}},
				{&crossdomain.Account{}, "snapshot_tables/rejected_issues_accounts.csv", []string{"id", "user_name"}},
			},
		},
		{
			name:      "issue commits",
			validFile: "raw_tables/rejected_issue_commits_valid.csv",
			badFile:   "raw_tables/rejected_issue_commits_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportIssueCommit("csv-board", file)
			},
			snapshots: []rejectedSnapshot{
				{&crossdomain.IssueCommit{}, "snapshot_tables/rejected_issue_commits_issue_commits.csv", []string{"issue_id", "commit_sha"}},
			},
		},
		{
			name:      "issue repo commits",
			validFile: "raw_tables/rejected_issue_repo_commits_valid.csv",
			badFile:   "raw_tables/rejected_issue_repo_commits_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportIssueRepoCommit("csv-board", file, false)
			},
			snapshots: []rejectedSnapshot{
				{&crossdomain.IssueRepoCommit{}, "snapshot_tables/rejected_issue_repo_commits_issue_repo_commits.csv", []string{"issue_id", "commit_sha", "repo_url"}},
				{&crossdomain.IssueCommit{}, "snapshot_tables/rejected_issue_repo_commits_issue_commits.csv", []string{"issue_id", "commit_sha"}},
			},
		},
		{
			name:      "issue changelogs",
			validFile: "raw_tables/rejected_issue_changelogs_valid.csv",
			badFile:   "raw_tables/rejected_issue_changelogs_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportIssueChangelog("csv-board", file, false)
			},
			snapshots: []rejectedSnapshot{
				{&ticket.IssueChangelogs{}, "snapshot_tables/rejected_issue_changelogs_issue_changelogs.csv", []string{"id", "issue_id", "original_to_value"}},
			},
		},
		{
			name:      "issue worklogs",
			validFile: "raw_tables/rejected_issue_worklogs_valid.csv",
			badFile:   "raw_tables/rejected_issue_worklogs_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportIssueWorklog("csv-board", file, false)
			},
			snapshots: []rejectedSnapshot{
				{&ticket.IssueWorklog{}, "snapshot_tables/rejected_issue_worklogs_issue_worklogs.csv", []string{"id", "issue_id", "time_spent_minutes"}},
			},
		},
		{
			name:      "sprints",
			validFile: "raw_tables/rejected_sprints_valid.csv",
			badFile:   "raw_tables/rejected_sprints_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportSprint("csv-board", file, false)
			},
			snapshots: []rejectedSnapshot{
				{&ticket.Sprint{}, "snapshot_tables/rejected_sprints_sprints.csv", []string{"id", "name"}},
				{&ticket.BoardSprint{}, "snapshot_tables/rejected_sprints_board_sprints.csv", []string{"board_id", "sprint_id"}},
			},
		},
		{
			name:      "qa apis",
			validFile: "raw_tables/rejected_qa_apis_valid.csv",
			badFile:   "raw_tables/rejected_qa_apis_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportQaApis(rejectedQaProjectId, file, false)
			},
			snapshots: []rejectedSnapshot{
				{&qa.QaApi{}, "snapshot_tables/rejected_qa_apis_qa_apis.csv", []string{"id", "name", "creator_id"}},
				{&crossdomain.Account{}, "snapshot_tables/rejected_qa_apis_accounts.csv", []string{"id", "user_name"}},
			},
		},
		{
			name:      "qa test cases",
			validFile: "raw_tables/rejected_qa_test_cases_valid.csv",
			badFile:   "raw_tables/rejected_qa_test_cases_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportQaTestCases(rejectedQaProjectId, "CSV QA", file, false)
			},
			setUp: func(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, svc *service.Service) {
				t.Helper()
				err := svc.ImportQaApis(rejectedQaProjectId, openCsv(t, "raw_tables/rejected_qa_apis_valid.csv"), false)
				if err != nil {
					t.Fatal(err)
				}
				err = svc.ImportQaTestCaseExecutions(rejectedQaProjectId, openCsv(t, "raw_tables/rejected_qa_test_case_executions_valid.csv"), false)
				if err != nil {
					t.Fatal(err)
				}
			},
			snapshots: []rejectedSnapshot{
				{&qa.QaTestCase{}, "snapshot_tables/rejected_qa_test_cases_qa_test_cases.csv", []string{"id", "name", "creator_id"}},
				{&qa.QaApi{}, "snapshot_tables/rejected_qa_test_cases_qa_apis.csv", []string{"id", "name"}},
				{&qa.QaTestCaseExecution{}, "snapshot_tables/rejected_qa_test_cases_qa_test_case_executions.csv", []string{"id", "status"}},
				{&crossdomain.Account{}, "snapshot_tables/rejected_qa_test_cases_accounts.csv", []string{"id", "user_name"}},
			},
		},
		{
			name:      "qa test case executions",
			validFile: "raw_tables/rejected_qa_test_case_executions_valid.csv",
			badFile:   "raw_tables/rejected_qa_test_case_executions_bad.csv",
			importCsv: func(svc *service.Service, file io.ReadCloser) errors.Error {
				return svc.ImportQaTestCaseExecutions(rejectedQaProjectId, file, false)
			},
			snapshots: []rejectedSnapshot{
				{&qa.QaTestCaseExecution{}, "snapshot_tables/rejected_qa_test_case_executions_qa_test_case_executions.csv", []string{"id", "status", "creator_id"}},
				{&crossdomain.Account{}, "snapshot_tables/rejected_qa_test_case_executions_accounts.csv", []string{"id", "user_name"}},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dataflowTester, svc := newImportTester(t)
			// The issue-scoped imports delete only the rows of the issues on the board.
			dataflowTester.ImportCsvIntoTabler("raw_tables/rejected_board_issues.csv", &ticket.BoardIssue{})

			err := c.importCsv(svc, openCsv(t, c.validFile))
			if err != nil {
				t.Fatal(err)
			}
			if c.setUp != nil {
				c.setUp(t, dataflowTester, svc)
			}
			verifyRejectedSnapshots(t, dataflowTester, c.snapshots)

			err = c.importCsv(svc, openCsv(t, c.badFile))
			if err == nil {
				t.Fatal("the import accepted a file with a bad line")
			}
			if !strings.Contains(err.Error(), "wrong number of fields") {
				t.Fatalf("the import failed for another reason: %v", err)
			}
			verifyRejectedSnapshots(t, dataflowTester, c.snapshots)
		})
	}
}

func verifyRejectedSnapshots(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, snapshots []rejectedSnapshot) {
	t.Helper()
	for _, snapshot := range snapshots {
		dataflowTester.VerifyTableWithOptions(snapshot.table, e2ehelper.TableOptions{
			CSVRelPath:   snapshot.csvRelPath,
			TargetFields: snapshot.targetFields,
		})
	}
}

// A creator name too long for an account id rejects the whole QA file, like any other row that
// cannot be saved, and the accounts of the file's earlier rows are rolled back with it.
func TestImportQaApisRejectsLongCreatorName(t *testing.T) {
	dataflowTester, svc := newImportTester(t)
	snapshots := []rejectedSnapshot{
		{&qa.QaApi{}, "snapshot_tables/rejected_qa_apis_qa_apis.csv", []string{"id", "name", "creator_id"}},
		{&crossdomain.Account{}, "snapshot_tables/rejected_qa_apis_accounts.csv", []string{"id", "user_name"}},
	}
	err := svc.ImportQaApis(rejectedQaProjectId, openCsv(t, "raw_tables/rejected_qa_apis_valid.csv"), false)
	if err != nil {
		t.Fatal(err)
	}
	verifyRejectedSnapshots(t, dataflowTester, snapshots)

	// Line 2 names a 250-character creator: csv:CsvAccount:0:<name> overflows the varchar(255) id.
	err = svc.ImportQaApis(rejectedQaProjectId, openCsv(t, "raw_tables/rejected_qa_apis_long_creator.csv"), false)
	if err == nil {
		t.Fatal("the import accepted a creator name too long for an account id")
	}
	verifyRejectedSnapshots(t, dataflowTester, snapshots)
}

// A rejected issues file must not create or rename its board either.
func TestImportIssueToBoardRejectedFileKeepsBoard(t *testing.T) {
	dataflowTester, svc := newImportTester(t)
	snapshots := []rejectedSnapshot{
		{&ticket.Board{}, "snapshot_tables/rejected_issues_boards.csv", []string{"id", "name"}},
		{&ticket.Issue{}, "snapshot_tables/rejected_issues_issues.csv", []string{"id", "title", "creator_name", "assignee_name"}},
	}
	err := svc.ImportIssueToBoard("csv-board", "CSV Board", openCsv(t, "raw_tables/rejected_issues_valid.csv"), false)
	if err != nil {
		t.Fatal(err)
	}
	verifyRejectedSnapshots(t, dataflowTester, snapshots)

	err = svc.ImportIssueToBoard("csv-board", "Renamed Board", openCsv(t, "raw_tables/rejected_issues_bad.csv"), false)
	if err == nil || !strings.Contains(err.Error(), "wrong number of fields") {
		t.Fatalf("the import must reject the file for its bad line, got: %v", err)
	}
	err = svc.ImportIssueToBoard("csv-board-2", "New Board", openCsv(t, "raw_tables/rejected_issues_bad.csv"), false)
	if err == nil || !strings.Contains(err.Error(), "wrong number of fields") {
		t.Fatalf("the import must reject the file for its bad line, got: %v", err)
	}
	verifyRejectedSnapshots(t, dataflowTester, snapshots)
}
