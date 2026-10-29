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

package e2ehelper

import (
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	plugin "github.com/apache/devlake/core/plugin"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestModel struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	IssueId      int    `gorm:"primaryKey;autoIncrement:false"`
	LabelName    string `gorm:"primaryKey;type:varchar(255)"`
	common.NoPKModel
}

func (t TestModel) TableName() string {
	return "_tool_test_model"
}

type TestOptions struct {
	ProjectId int
}

type TestTaskData struct {
	Options *TestOptions
}

// testExtractIssuesMeta stands in for a real plugin's subtask so this example
// stays self-contained: helpers must not import a concrete plugin.
var testExtractIssuesMeta = plugin.SubTaskMeta{
	Name: "extractIssues",
	EntryPoint: func(taskCtx plugin.SubTaskContext) errors.Error {
		return nil
	},
}

func ExampleDataFlowTester() {
	var t *testing.T // stub

	var pluginMeta plugin.PluginMeta
	dataflowTester := NewDataFlowTester(t, "test_plugin", pluginMeta)

	taskData := &TestTaskData{
		Options: &TestOptions{
			ProjectId: 666888,
		},
	}

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./tables/_raw_test_plugin_api_issues.csv", "_raw_test_plugin_api_issues")

	// verify extraction
	dataflowTester.FlushTabler(&TestModel{})
	dataflowTester.Subtask(testExtractIssuesMeta, taskData)
	dataflowTester.VerifyTable(
		&TestModel{},
		"tables/_tool_test_model.csv",
		[]string{
			"connection_id",
			"issue_id",
			"label_name",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)
}

func TestGetTableMetaData(t *testing.T) {
	var meta plugin.PluginMeta
	dataflowTester := NewDataFlowTester(t, "test_dataflow", meta)
	dataflowTester.FlushTabler(&TestModel{})
	t.Run("dal_get_columns", func(t *testing.T) {
		names, err := dal.GetColumnNames(dataflowTester.Dal, &TestModel{}, nil)
		assert.Equal(t, err, nil)
		assert.Equal(t, 9, len(names))
		for _, e := range []string{
			"connection_id",
			"issue_id",
			"label_name",
			"created_at",
			"updated_at",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		} {
			assert.Contains(t, names, e)
		}
	})
	t.Run("extract_columns", func(t *testing.T) {
		columns := dataflowTester.extractColumns(&common.RawDataOrigin{})
		assert.Equal(t, 4, len(columns))
		for _, e := range []string{
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		} {
			assert.Contains(t, columns, e)
		}
	})
	t.Run("dal_get_pk_column_names", func(t *testing.T) {
		fields, err := dal.GetPrimarykeyColumnNames(dataflowTester.Dal, &TestModel{})
		assert.Equal(t, err, nil)
		assert.Equal(t, 3, len(fields))
		for _, e := range []string{
			"connection_id",
			"issue_id",
			"label_name",
		} {
			assert.Contains(t, fields, e)
		}
	})
	t.Run("resolve_fields_targetFieldsOnly", func(t *testing.T) {
		fields := dataflowTester.resolveTargetFields(&TestModel{}, TableOptions{
			TargetFields: []string{"connection_id"},
			IgnoreFields: nil,
			IgnoreTypes:  nil,
		})
		assert.Equal(t, 1, len(fields))
		for _, e := range []string{"connection_id"} {
			assert.Contains(t, fields, e)
		}
	})
	t.Run("resolve_fields_ignoreFieldsOnly", func(t *testing.T) {
		fields := dataflowTester.resolveTargetFields(&TestModel{}, TableOptions{
			TargetFields: nil,
			IgnoreFields: []string{
				"label_name",
				"created_at",
				"updated_at",
				"_raw_data_params",
				"_raw_data_table",
				"_raw_data_id",
				"_raw_data_remark",
			},
			IgnoreTypes: nil,
		})
		assert.Equal(t, 2, len(fields))
		for _, e := range []string{"connection_id", "issue_id"} {
			assert.Contains(t, fields, e)
		}
	})
	t.Run("resolve_fields_ignoreFieldsOnly", func(t *testing.T) {
		fields := dataflowTester.resolveTargetFields(&TestModel{}, TableOptions{
			TargetFields: nil,
			IgnoreFields: []string{
				"label_name",
				"created_at",
				"updated_at",
				"_raw_data_params",
				"_raw_data_table",
				"_raw_data_id",
				"_raw_data_remark",
			},
			IgnoreTypes: nil,
		})
		assert.Equal(t, 2, len(fields))
		for _, e := range []string{"connection_id", "issue_id"} {
			assert.Contains(t, fields, e)
		}
	})
	t.Run("resolve_fields_ignoreType", func(t *testing.T) {
		fields := dataflowTester.resolveTargetFields(&TestModel{}, TableOptions{
			TargetFields: nil,
			IgnoreFields: nil,
			IgnoreTypes:  []interface{}{&common.NoPKModel{}},
		})
		assert.Equal(t, 3, len(fields))
		for _, e := range []string{
			"connection_id",
			"issue_id",
			"label_name",
		} {
			assert.Contains(t, fields, e)
		}
	})
	t.Run("resolve_fields_ignoreType_ignoreFields", func(t *testing.T) {
		fields := dataflowTester.resolveTargetFields(&TestModel{}, TableOptions{
			TargetFields: nil,
			IgnoreFields: []string{"label_name"},
			IgnoreTypes:  []interface{}{&common.NoPKModel{}},
		})
		assert.Equal(t, 2, len(fields))
		for _, e := range []string{
			"connection_id",
			"issue_id",
		} {
			assert.Contains(t, fields, e)
		}
	})
	t.Run("resolve_fields_targetFields_ignoreType_ignoreFields", func(t *testing.T) {
		fields := dataflowTester.resolveTargetFields(&TestModel{}, TableOptions{
			TargetFields: []string{"label_name", "createdAt", "connection_id"},
			IgnoreFields: []string{"label_name"},
			IgnoreTypes:  []interface{}{&common.NoPKModel{}},
		})
		assert.Equal(t, 1, len(fields))
		for _, e := range []string{
			"connection_id",
		} {
			assert.Contains(t, fields, e)
		}
	})
}
