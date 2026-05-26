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

package api

import (
	"reflect"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	plugin "github.com/apache/incubator-devlake/core/plugin"
)

type StatefulDataConverterArgs[InputType any] struct {
	*SubtaskCommonArgs
	Input         func(*SubtaskStateManager) (dal.Rows, errors.Error)
	BeforeConvert func(issue *InputType, stateManager *SubtaskStateManager) errors.Error
	Convert       func(row *InputType) ([]any, errors.Error)
	BatchSize     int
}

// StatefulDataConverter is a struct that manages the stateful data conversion process.
// It facilitates converting data from a database cursor and saving it into arbitrary tables.
// The converter determines the operating mode (Incremental/FullSync) based on the stored state and configuration.
// It then calls the provided `Input` function to obtain the `dal.Rows` (the database cursor) and processes each
// record individually through the `Convert` function, saving the results to the database.
//
// For Incremental mode to work properly, it is crucial to check `stateManager.IsIncremental()` and utilize
// `stateManager.GetSince()` to build your query in the `Input` function, ensuring that only the necessary
// records are fetched.
//
// The converter automatically detects if the configuration has changed since the last run. If a change is detected,
// it will automatically switch to Full-Sync mode.
//
// Example:
//
// converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.JiraIssue]{
// 	SubtaskCommonArgs: &api.SubtaskCommonArgs{
// 		SubTaskContext: subtaskCtx,
// 		Table:          RAW_ISSUE_TABLE,
// 		Params: JiraApiParams{
// 			ConnectionId: data.Options.ConnectionId,
// 			BoardId:      data.Options.BoardId,
// 		},
// 		SubtaskConfig: mappings,
// 	},
// 	Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
// 		clauses := []dal.Clause{
// 			dal.Select("_tool_jira_issues.*"),
// 			dal.From("_tool_jira_issues"),
// 			dal.Join(`left join _tool_jira_board_issues
// 				on _tool_jira_board_issues.issue_id = _tool_jira_issues.issue_id
// 				and _tool_jira_board_issues.connection_id = _tool_jira_issues.connection_id`),
// 			dal.Where(
// 				"_tool_jira_board_issues.connection_id = ? AND _tool_jira_board_issues.board_id = ?",
// 				data.Options.ConnectionId,
// 				data.Options.BoardId,
// 			),
// 		}
// 		if stateManager.IsIncremental() { // IMPORTANT: to filter records for Incremental Mode
// 			since := stateManager.GetSince()
// 			if since != nil {
// 				clauses = append(clauses, dal.Where("_tool_jira_issues.updated_at >= ? ", since))
// 			}
// 		}
// 		return db.Cursor(clauses...)
// 	},
//	BeforeConvert: func(jiraIssue *models.GitlabMergeRequest, stateManager *api.SubtaskStateManager) errors.Error {
//		// It is important to delete all existing child-records under DiffSync Mode
//		issueId := issueIdGen.Generate(data.Options.ConnectionId, jiraIssue.IssueId)
//		if err := db.Delete(&ticket.IssueAssignee{}, dal.Where("issue_id = ?", issueId)); err != nil {
//			return err
//		}
//		...
//		return nil
//	},
// 	Convert: func(jiraIssue *models.JiraIssue) ([]interface{}, errors.Error) {
// 	},
// })

// if err != nil {
// 	return err
// }

// return converter.Execute()
type StatefulDataConverter[InputType any] struct {
	*StatefulDataConverterArgs[InputType]
	*SubtaskStateManager
}

func NewStatefulDataConverter[
	InputType any,
](
	args *StatefulDataConverterArgs[InputType],
) (*StatefulDataConverter[InputType], errors.Error) {
	// process args
	if args.BatchSize == 0 {
		args.BatchSize = 500
	}
	stateManager, err := NewSubtaskStateManager(args.SubtaskCommonArgs)
	if err != nil {
		return nil, err
	}
	return &StatefulDataConverter[InputType]{
		StatefulDataConverterArgs: args,
		SubtaskStateManager:       stateManager,
	}, nil
}

func (converter *StatefulDataConverter[InputType]) Execute() errors.Error {
	// load data from database
	db := converter.GetDal()

	table := converter.GetRawDataTable()
	params := converter.GetRawDataParams()

	// batch save divider
	RAW_DATA_ORIGIN := "RawDataOrigin"
	divider := NewBatchSaveDivider(converter, converter.BatchSize, table, params)
	divider.SetIncrementalMode(converter.IsIncremental())

	// set progress
	converter.SetProgress(0, -1)

	cursor, err := converter.Input(converter.SubtaskStateManager)
	if err != nil {
		return err
	}
	defer cursor.Close()
	ctx := converter.GetContext()
	// iterate all rows
	for cursor.Next() {
		select {
		case <-ctx.Done():
			return errors.Convert(ctx.Err())
		default:
		}
		inputRow := new(InputType)
		err := db.Fetch(cursor, inputRow)
		if err != nil {
			return errors.Default.Wrap(err, "error fetching rows")
		}

		if converter.BeforeConvert != nil {
			err = converter.BeforeConvert(inputRow, converter.SubtaskStateManager)
			if err != nil {
				return err
			}
		}

		results, err := converter.Convert(inputRow)
		if err != nil {
			return errors.Default.Wrap(err, "error calling Converter plugin implementation")
		}

		for _, result := range results {
			// get the batch operator for the specific type
			batch, err := divider.ForType(reflect.TypeOf(result))
			if err != nil {
				return errors.Default.Wrap(err, "error getting batch from result")
			}
			// set raw data origin field
			origin := reflect.ValueOf(result).Elem().FieldByName(RAW_DATA_ORIGIN)
			if origin.IsValid() {
				origin.Set(reflect.ValueOf(inputRow).Elem().FieldByName(RAW_DATA_ORIGIN))
			}
			// records get saved into db when slots were max outed
			err = batch.Add(result)
			if err != nil {
				return errors.Default.Wrap(err, "error adding result to batch")
			}
		}
		converter.IncProgress(1)
	}

	// save the last batches
	err = divider.Close()
	if err != nil {
		return err
	}
	// save the incremental state
	return converter.SubtaskStateManager.Close()
}

// Check if DataConverter implements SubTask interface
var _ plugin.SubTask = (*StatefulDataConverter[any])(nil)
