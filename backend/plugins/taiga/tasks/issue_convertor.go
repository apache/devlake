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
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
)

var ConvertIssuesMeta = plugin.SubTaskMeta{
	Name:             "convertIssues",
	EntryPoint:       ConvertIssues,
	EnabledByDefault: true,
	Description:      "convert Taiga issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

// taigaIssueTypeToDevLake maps Taiga's customizable issue type names to
// DevLake standard issue types. Taiga's default types are Bug, Enhancement,
// and Question; additional custom types fall back to REQUIREMENT.
func taigaIssueTypeToDevLake(typeName string) string {
	switch strings.ToLower(typeName) {
	case "bug":
		return "BUG"
	case "enhancement", "feature":
		return "REQUIREMENT"
	case "question":
		return "QUESTION"
	default:
		return "REQUIREMENT"
	}
}

func ConvertIssues(subtaskCtx plugin.SubTaskContext) errors.Error {
	logger := subtaskCtx.GetLogger()
	data := subtaskCtx.GetData().(*TaigaTaskData)
	db := subtaskCtx.GetDal()

	issueIdGen := didgen.NewDomainIdGenerator(&models.TaigaIssue{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.TaigaProject{})
	boardId := boardIdGen.Generate(data.Options.ConnectionId, data.Options.ProjectId)

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.TaigaIssue]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          RAW_ISSUE_TABLE,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select("*"),
				dal.From(&models.TaigaIssue{}),
				dal.Where("connection_id = ?", data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(taigaIssue *models.TaigaIssue) ([]interface{}, errors.Error) {
			var result []interface{}

			devLakeType := taigaIssueTypeToDevLake(taigaIssue.IssueTypeName)
			originalType := taigaIssue.IssueTypeName
			if originalType == "" {
				originalType = "Issue"
			}

			issue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: issueIdGen.Generate(taigaIssue.ConnectionId, taigaIssue.IssueId),
				},
				IssueKey:       taigaIssue.Subject,
				Title:          taigaIssue.Subject,
				Type:           devLakeType,
				OriginalType:   originalType,
				Status:         taigaIssue.Status,
				OriginalStatus: taigaIssue.Status,
				Priority:       taigaIssue.Priority,
				CreatedDate:    taigaIssue.CreatedDate,
				UpdatedDate:    taigaIssue.ModifiedDate,
				ResolutionDate: taigaIssue.FinishedDate,
			}

			result = append(result, issue)

			boardIssue := &ticket.BoardIssue{
				BoardId: boardId,
				IssueId: issue.Id,
			}
			result = append(result, boardIssue)

			logger.Debug("converted issue %d (type: %s → %s)", taigaIssue.IssueId, originalType, devLakeType)
			return result, nil
		},
	})

	if err != nil {
		return err
	}

	return converter.Execute()
}
