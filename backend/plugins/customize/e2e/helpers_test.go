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
	"os"
	"strings"
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/qa"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/customize/impl"
	"github.com/apache/devlake/plugins/customize/service"
)

// issueHeader is the header of the issue CSV files that the tests build in memory.
const issueHeader = "id,url,issue_key,title,original_type,original_status,created_date"

// newImportTester returns a DataFlowTester with empty import tables, and a Service on its database.
func newImportTester(t *testing.T) (*e2ehelper.DataFlowTester, *service.Service) {
	t.Helper()
	var plugin impl.Customize
	dataflowTester := e2ehelper.NewDataFlowTester(t, "customize", plugin)
	flushImportTables(t, dataflowTester)
	return dataflowTester, service.NewService(dataflowTester.Dal)
}

// flushImportTables empties every table that the customize imports read or write.
func flushImportTables(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) {
	t.Helper()
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.IssueLabel{})
	dataflowTester.FlushTabler(&ticket.SprintIssue{})
	dataflowTester.FlushTabler(&ticket.IssueChangelogs{})
	dataflowTester.FlushTabler(&ticket.IssueWorklog{})
	dataflowTester.FlushTabler(&ticket.Sprint{})
	dataflowTester.FlushTabler(&ticket.BoardSprint{})
	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.FlushTabler(&crossdomain.IssueCommit{})
	dataflowTester.FlushTabler(&crossdomain.IssueRepoCommit{})
	dataflowTester.FlushTabler(&qa.QaProject{})
	dataflowTester.FlushTabler(&qa.QaApi{})
	dataflowTester.FlushTabler(&qa.QaTestCase{})
	dataflowTester.FlushTabler(&qa.QaTestCaseExecution{})
}

// openCsv opens a CSV file for an import and closes it when the test ends.
func openCsv(t *testing.T, path string) io.ReadCloser {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

// csvFile returns the lines as the content of a CSV file.
func csvFile(lines ...string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n") + "\n"))
}
