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

	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/qa"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/customize/impl"
	"github.com/apache/incubator-devlake/plugins/customize/service"
)

func TestImportQaTestCaseExecutionsDataFlow(t *testing.T) {
	var plugin impl.Customize
	dataflowTester := e2ehelper.NewDataFlowTester(t, "customize", plugin)

	// Flush the relevant table
	dataflowTester.FlushTabler(&qa.QaTestCaseExecution{})
	dataflowTester.FlushTabler(&crossdomain.Account{})

	// Create a new service instance
	svc := service.NewService(dataflowTester.Dal)

	// Assume a dummy CSV file exists for testing
	// You will need to create this file at backend/plugins/customize/e2e/raw_tables/qa_test_case_executions_input.csv
	// with appropriate test data.
	qaTestCaseExecutionsFile, err := os.Open("raw_tables/qa_test_case_executions_input.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer qaTestCaseExecutionsFile.Close()

	// Define a dummy qaProjectId
	qaProjectId := "test-qa-project-id"

	// Import data from the CSV file
	err = svc.ImportQaTestCaseExecutions(qaProjectId, qaTestCaseExecutionsFile, false) // Use false for initial import
	if err != nil {
		t.Fatal(err)
	}

	// Verify the imported data in qa_test_case_executions table
	// You will need to create the expected snapshot file at backend/plugins/customize/e2e/snapshot_tables/qa_test_case_executions_output.csv
	// and define the columns to verify based on your qa_test_case_executions_input.csv and the qa.QaTestCaseExecution struct.
	dataflowTester.VerifyTableWithRawData(
		&qa.QaTestCaseExecution{},
		"snapshot_tables/qa_test_case_executions_output.csv",
		[]string{
			"id",
			"qa_project_id",
			"qa_test_case_id",
			"create_time",
			"start_time",
			"finish_time",
			"creator_id",
			"is_invalid",
			"status",
		})
	dataflowTester.VerifyTableWithRawData(
		&crossdomain.Account{},
		"snapshot_tables/accounts_from_qa_test_case_executions_output.csv",
		[]string{
			"id",
			"full_name",
			"user_name",
		},
	)

	// Add incremental import test
	qaTestCaseExecutionsIncrementalFile, err := os.Open("raw_tables/qa_test_case_executions_input_incremental.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer qaTestCaseExecutionsIncrementalFile.Close()
	err = svc.ImportQaTestCaseExecutions(qaProjectId, qaTestCaseExecutionsIncrementalFile, true) // Use true for incremental import
	if err != nil {
		t.Fatal(err)
	}
	dataflowTester.VerifyTableWithRawData(
		&qa.QaTestCaseExecution{},
		"snapshot_tables/qa_test_case_executions_output_incremental.csv",
		[]string{
			"id",
			"qa_project_id",
			"qa_test_case_id",
			"create_time",
			"start_time",
			"finish_time",
			"creator_id",
			"is_invalid",
			"status",
		})

	dataflowTester.VerifyTableWithRawData(
		&crossdomain.Account{},
		"snapshot_tables/accounts_from_qa_test_case_executions_output_incremental.csv",
		[]string{
			"id",
			"full_name",
			"user_name",
		},
	)
}

// TestImportQaTestCaseExecutions_NoIsInvalidColumn tests backward compatibility:
// Verifies that importing CSV files without the is_invalid column works correctly,
// and the is_invalid field defaults to false.
func TestImportQaTestCaseExecutions_NoIsInvalidColumn(t *testing.T) {
	var plugin impl.Customize
	dataflowTester := e2ehelper.NewDataFlowTester(t, "customize", plugin)

	// Flush the relevant table
	dataflowTester.FlushTabler(&qa.QaTestCaseExecution{})
	dataflowTester.FlushTabler(&crossdomain.Account{})

	// Create a new service instance
	svc := service.NewService(dataflowTester.Dal)

	// Use the existing CSV file that does NOT contain is_invalid column
	// This simulates backward compatibility with old CSV files
	qaTestCaseExecutionsFile, err := os.Open("raw_tables/qa_test_case_executions_input.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer qaTestCaseExecutionsFile.Close()

	// Define a dummy qaProjectId
	qaProjectId := "test-backward-compat-project"

	// Import data from the CSV file (which has no is_invalid column)
	err = svc.ImportQaTestCaseExecutions(qaProjectId, qaTestCaseExecutionsFile, false)
	if err != nil {
		t.Fatalf("ImportQaTestCaseExecutions failed: %v", err)
	}

	// Verify the imported data has is_invalid defaulted to false
	dataflowTester.VerifyTableWithRawData(
		&qa.QaTestCaseExecution{},
		"snapshot_tables/qa_test_case_executions_output_no_is_invalid_column.csv",
		[]string{
			"id",
			"qa_project_id",
			"qa_test_case_id",
			"create_time",
			"start_time",
			"finish_time",
			"creator_id",
			"is_invalid",
			"status",
		})
}
