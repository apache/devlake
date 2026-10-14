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
	"os"
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/customize/impl"
	"github.com/apache/devlake/plugins/customize/service"
)

func TestImportIssueChangelogDataFlow(t *testing.T) {
	var plugin impl.Customize
	dataflowTester := e2ehelper.NewDataFlowTester(t, "customize", plugin)

	// 清空表
	dataflowTester.FlushTabler(&ticket.IssueChangelogs{})
	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})

	// 初始化服务
	svc := service.NewService(dataflowTester.Dal)

	// 导入全量数据
	changelogFile, err := os.Open("raw_tables/issue_changelogs.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer changelogFile.Close()
	err = svc.ImportIssueChangelog("TEST_BOARD", changelogFile, false)
	if err != nil {
		t.Fatal(err)
	}

	// 验证全量导入结果
	dataflowTester.VerifyTableWithRawData(
		ticket.IssueChangelogs{},
		"snapshot_tables/issue_changelogs.csv",
		[]string{
			"id",
			"issue_id",
			"author_id",
			"field_name",
			"original_from_value",
			"original_to_value",
			"created_date",
		})

	// 导入增量数据
	incrementalFile, err := os.Open("raw_tables/issue_changelogs_incremental.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer incrementalFile.Close()
	err = svc.ImportIssueChangelog("TEST_BOARD", incrementalFile, true)
	if err != nil {
		t.Fatal(err)
	}

	// 验证增量导入结果
	dataflowTester.VerifyTableWithRawData(
		ticket.IssueChangelogs{},
		"snapshot_tables/issue_changelogs_incremental.csv",
		[]string{
			"id",
			"issue_id",
			"author_id",
			"field_name",
			"original_from_value",
			"original_to_value",
			"created_date",
		})

	dataflowTester.VerifyTable(
		crossdomain.Account{},
		"snapshot_tables/accounts_from_issue_changelogs.csv",
		[]string{
			"id",
			"full_name",
			"user_name",
		},
	)
}
