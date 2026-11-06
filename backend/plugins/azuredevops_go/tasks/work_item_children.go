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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
)

func init() {
	RegisterSubtaskMeta(&CollectApiWorkItemCommentsMeta)
	RegisterSubtaskMeta(&ExtractApiWorkItemCommentsMeta)
	RegisterSubtaskMeta(&ConvertWorkItemCommentsMeta)
	RegisterSubtaskMeta(&CollectApiWorkItemUpdatesMeta)
	RegisterSubtaskMeta(&ExtractApiWorkItemUpdatesMeta)
	RegisterSubtaskMeta(&ConvertWorkItemUpdatesMeta)
}

const (
	RawWorkItemCommentTable = "azuredevops_go_api_work_item_comments"
	RawWorkItemUpdateTable  = "azuredevops_go_api_work_item_updates"
)

var CollectApiWorkItemCommentsMeta = plugin.SubTaskMeta{
	Name:             "collectApiWorkItemComments",
	EntryPoint:       CollectApiWorkItemComments,
	EnabledByDefault: true,
	Description:      "Collect Azure Boards work item comments when the scope config asks for them.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.AzuredevopsWorkItem{}.TableName()},
	ProductTables:    []string{RawWorkItemCommentTable},
}

var ExtractApiWorkItemCommentsMeta = plugin.SubTaskMeta{
	Name:             "extractApiWorkItemComments",
	EntryPoint:       ExtractApiWorkItemComments,
	EnabledByDefault: true,
	Description:      "Extract Azure Boards work item comments into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{RawWorkItemCommentTable},
	ProductTables:    []string{models.AzuredevopsWorkItemComment{}.TableName()},
}

var ConvertWorkItemCommentsMeta = plugin.SubTaskMeta{
	Name:             "convertWorkItemComments",
	EntryPoint:       ConvertWorkItemComments,
	EnabledByDefault: true,
	Description:      "Convert Azure Boards work item comments into issue comments.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.AzuredevopsWorkItemComment{}.TableName()},
	ProductTables:    []string{ticket.IssueComment{}.TableName()},
}

var CollectApiWorkItemUpdatesMeta = plugin.SubTaskMeta{
	Name:             "collectApiWorkItemUpdates",
	EntryPoint:       CollectApiWorkItemUpdates,
	EnabledByDefault: true,
	Description:      "Collect Azure Boards work item revisions when the scope config asks for changelogs.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.AzuredevopsWorkItem{}.TableName()},
	ProductTables:    []string{RawWorkItemUpdateTable},
}

var ExtractApiWorkItemUpdatesMeta = plugin.SubTaskMeta{
	Name:             "extractApiWorkItemUpdates",
	EntryPoint:       ExtractApiWorkItemUpdates,
	EnabledByDefault: true,
	Description:      "Extract mapped Azure Boards field changes into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{RawWorkItemUpdateTable},
	ProductTables:    []string{models.AzuredevopsWorkItemUpdate{}.TableName()},
}

var ConvertWorkItemUpdatesMeta = plugin.SubTaskMeta{
	Name:             "convertWorkItemUpdates",
	EntryPoint:       ConvertWorkItemUpdates,
	EnabledByDefault: true,
	Description:      "Convert Azure Boards field changes into issue changelogs.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.AzuredevopsWorkItemUpdate{}.TableName()},
	ProductTables:    []string{ticket.IssueChangelogs{}.TableName()},
}

type workItemRefInput struct {
	WorkItemId int
}

func CollectApiWorkItemComments(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	if data.Options.ScopeConfig == nil || !data.Options.ScopeConfig.CollectComments {
		return nil
	}
	return collectWorkItemChildren(taskCtx, data, RawWorkItemCommentTable, "comments", "7.1-preview.3", true)
}

func CollectApiWorkItemUpdates(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	if data.Options.ScopeConfig == nil || !data.Options.ScopeConfig.CollectChangelogs {
		return nil
	}
	return collectWorkItemChildren(taskCtx, data, RawWorkItemUpdateTable, "updates", "7.1", false)
}

func collectWorkItemChildren(taskCtx plugin.SubTaskContext, data *AzuredevopsTaskData, table, suffix, apiVersion string, comments bool) errors.Error {
	ids, err := listWorkItemIDs(taskCtx, data)
	if err != nil {
		return err
	}
	rawArgs := &api.RawDataSubTaskArgs{Ctx: taskCtx, Table: table, Options: data.Options}
	iterator := api.NewQueueIterator()
	for _, id := range ids {
		iterator.Push(workItemRefInput{WorkItemId: id})
	}
	var tokens sync.Map
	collector, err := api.NewStatefulApiCollector(*rawArgs)
	if err != nil {
		return err
	}
	err = collector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: *rawArgs,
		ApiClient:          data.ApiClient,
		Input:              iterator,
		PageSize:           1,
		UrlTemplate:        fmt.Sprintf("%s/_apis/wit/workitems/{{ .Input.WorkItemId }}/%s", adoScopePrefix(data), suffix),
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("api-version", apiVersion)
			if reqData.CustomData != nil {
				query.Set("continuationToken", fmt.Sprint(reqData.CustomData))
			}
			return query, nil
		},
		GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
			input := prevReqData.Input.(workItemRefInput)
			token, _ := tokens.Load(input.WorkItemId)
			text, _ := token.(string)
			if text == "" {
				return nil, api.ErrFinishCollect
			}
			tokens.Delete(input.WorkItemId)
			return text, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			if comments {
				var body struct {
					Comments          []json.RawMessage `json:"comments"`
					ContinuationToken string            `json:"continuationToken"`
				}
				if parseErr := api.UnmarshalResponse(res, &body); parseErr != nil {
					return nil, parseErr
				}
				if id := workItemIDFromRequest(res); id != 0 && body.ContinuationToken != "" {
					tokens.Store(id, body.ContinuationToken)
				}
				return body.Comments, nil
			}
			var body continuationBody
			if parseErr := api.UnmarshalResponse(res, &body); parseErr != nil {
				return nil, parseErr
			}
			if id := workItemIDFromRequest(res); id != 0 && body.ContinuationToken != "" {
				tokens.Store(id, body.ContinuationToken)
			}
			return body.Value, nil
		},
		AfterResponse: change203To401,
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

func workItemIDFromRequest(res *http.Response) int {
	if res == nil || res.Request == nil || res.Request.URL == nil {
		return 0
	}
	match := workItemIDFromURL.FindStringSubmatch(res.Request.URL.Path)
	if len(match) < 2 {
		return 0
	}
	return asInt(match[1])
}

func listWorkItemIDs(taskCtx plugin.SubTaskContext, data *AzuredevopsTaskData) ([]int, errors.Error) {
	var rows []models.AzuredevopsWorkItem
	err := taskCtx.GetDal().All(
		&rows,
		dal.Select("azuredevops_id, work_item_type"),
		dal.From(&models.AzuredevopsWorkItem{}),
		dal.Where("connection_id = ? AND project_id = ?", data.Options.ConnectionId, data.Options.ProjectId),
	)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		if shouldConvertWorkItem(row.WorkItemType) {
			ids = append(ids, row.AzuredevopsId)
		}
	}
	return ids, nil
}

func ExtractApiWorkItemComments(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	if data.Options.ScopeConfig == nil || !data.Options.ScopeConfig.CollectComments {
		return nil
	}
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawWorkItemCommentTable)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				ID           int                    `json:"id"`
				Text         string                 `json:"text"`
				IsDeleted    bool                   `json:"isDeleted"`
				CreatedBy    map[string]interface{} `json:"createdBy"`
				CreatedDate  interface{}            `json:"createdDate"`
				ModifiedDate interface{}            `json:"modifiedDate"`
				WorkItemId   int                    `json:"workItemId"`
			}
			if err := errors.Convert(json.Unmarshal(row.Data, &raw)); err != nil {
				return nil, err
			}
			if raw.IsDeleted || raw.Text == "" {
				return nil, nil
			}
			workItemID := raw.WorkItemId
			if workItemID == 0 {
				var input workItemRefInput
				_ = json.Unmarshal(row.Input, &input)
				workItemID = input.WorkItemId
			}
			authorID, authorName := identityFromRef(raw.CreatedBy)
			return []interface{}{&models.AzuredevopsWorkItemComment{
				ConnectionId:  data.Options.ConnectionId,
				WorkItemId:    workItemID,
				AzuredevopsId: raw.ID,
				ProjectId:     data.Options.ProjectId,
				Body:          raw.Text,
				AuthorId:      authorID,
				AuthorName:    authorName,
				CreatedDate:   parseAdoTime(raw.CreatedDate),
				UpdatedDate:   parseAdoTime(raw.ModifiedDate),
			}}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func ConvertWorkItemComments(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	if data.Options.ScopeConfig == nil || !data.Options.ScopeConfig.CollectComments {
		return nil
	}
	return convertProjectRows(taskCtx, models.AzuredevopsWorkItemComment{}.TableName(), &models.AzuredevopsWorkItemComment{}, func(row interface{}) ([]interface{}, errors.Error) {
		comment := row.(*models.AzuredevopsWorkItemComment)
		if comment.Body == "" {
			return nil, nil
		}
		issueID := didgen.NewDomainIdGenerator(&models.AzuredevopsWorkItem{}).Generate(data.Options.ConnectionId, comment.WorkItemId)
		commentID := didgen.NewDomainIdGenerator(&models.AzuredevopsWorkItemComment{}).Generate(data.Options.ConnectionId, comment.WorkItemId, comment.AzuredevopsId)
		created := comment.CreatedDate
		var createdValue = commentCreated(created)
		return []interface{}{&ticket.IssueComment{
			DomainEntity: domainlayer.NewDomainEntity(commentID),
			IssueId:      issueID,
			Body:         comment.Body,
			AccountId:    comment.AuthorId,
			CreatedDate:  createdValue,
			UpdatedDate:  comment.UpdatedDate,
		}}, nil
	})
}
