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
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
)

func init() {
	RegisterSubtaskMeta(&CollectApiIterationsMeta)
	RegisterSubtaskMeta(&ExtractApiIterationsMeta)
	RegisterSubtaskMeta(&ConvertIterationsMeta)
}

const RawIterationTable = "azuredevops_go_api_iterations"

var CollectApiIterationsMeta = plugin.SubTaskMeta{
	Name:             "collectApiIterations",
	EntryPoint:       CollectApiIterations,
	EnabledByDefault: true,
	Description:      "Collect Azure Boards iterations and team time frames.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{RawIterationTable},
}

var ExtractApiIterationsMeta = plugin.SubTaskMeta{
	Name:             "extractApiIterations",
	EntryPoint:       ExtractApiIterations,
	EnabledByDefault: true,
	Description:      "Extract Azure Boards iterations into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{RawIterationTable},
	ProductTables:    []string{models.AzuredevopsIteration{}.TableName()},
}

var ConvertIterationsMeta = plugin.SubTaskMeta{
	Name:             "convertIterations",
	EntryPoint:       ConvertIterations,
	EnabledByDefault: true,
	Description:      "Convert Azure Boards iterations into sprints.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.AzuredevopsIteration{}.TableName()},
	ProductTables:    []string{ticket.Sprint{}.TableName(), ticket.BoardSprint{}.TableName()},
}

func CollectApiIterations(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	timeFrames := loadIterationTimeFrames(taskCtx, data)
	rawArgs := &api.RawDataSubTaskArgs{Ctx: taskCtx, Table: RawIterationTable, Options: data.Options}
	collector, err := api.NewStatefulApiCollector(*rawArgs)
	if err != nil {
		return err
	}
	err = collector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: *rawArgs,
		ApiClient:          data.ApiClient,
		UrlTemplate:        fmt.Sprintf("%s/_apis/wit/classificationnodes/Iterations", adoScopePrefix(data)),
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("$depth", "10")
			query.Set("api-version", "7.1")
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var root map[string]interface{}
			if parseErr := readJSON(res, &root); parseErr != nil {
				return nil, parseErr
			}
			flat := flattenIterationNodes(root)
			messages := make([]json.RawMessage, 0, len(flat))
			for _, node := range flat {
				identifier := asString(node["identifier"])
				if identifier == "" {
					continue
				}
				if frame := timeFrames[identifier]; frame != "" {
					node["_timeFrame"] = frame
				}
				encoded, encodeErr := json.Marshal(node)
				if encodeErr != nil {
					return nil, errors.Convert(encodeErr)
				}
				messages = append(messages, encoded)
			}
			return messages, nil
		},
		AfterResponse: change203To401,
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

func flattenIterationNodes(node map[string]interface{}) []map[string]interface{} {
	if node == nil {
		return nil
	}
	children, _ := node["children"].([]interface{})
	delete(node, "children")
	nodes := []map[string]interface{}{node}
	for _, child := range children {
		childNode, ok := child.(map[string]interface{})
		if !ok {
			continue
		}
		nodes = append(nodes, flattenIterationNodes(childNode)...)
	}
	return nodes
}

func loadIterationTimeFrames(taskCtx plugin.SubTaskContext, data *AzuredevopsTaskData) map[string]string {
	frames := map[string]string{}
	path := fmt.Sprintf("%s_apis/projects/%s/teams?api-version=7.1", adoOrgPrefix(data), url.PathEscape(data.Options.ProjectId))
	res, err := data.ApiClient.Get(path, nil, nil)
	if err != nil {
		taskCtx.GetLogger().Warn(err, "failed to list Azure DevOps teams for iteration time frames")
		return frames
	}
	var teams struct {
		Value []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"value"`
	}
	if err = readJSON(res, &teams); err != nil {
		taskCtx.GetLogger().Warn(err, "failed to parse Azure DevOps teams for iteration time frames")
		return frames
	}
	rank := map[string]int{"current": 3, "future": 2, "past": 1}
	for _, team := range teams.Value {
		teamRef := team.ID
		if teamRef == "" {
			teamRef = team.Name
		}
		if teamRef == "" {
			continue
		}
		teamPath := fmt.Sprintf("%s/%s/_apis/work/teamsettings/iterations?api-version=7.1", adoScopePrefix(data), url.PathEscape(teamRef))
		teamRes, teamErr := data.ApiClient.Get(teamPath, nil, nil)
		if teamErr != nil {
			taskCtx.GetLogger().Warn(teamErr, "failed to list iterations for team %s", teamRef)
			continue
		}
		var body struct {
			Value []struct {
				ID         string `json:"id"`
				Path       string `json:"path"`
				Attributes struct {
					TimeFrame string `json:"timeFrame"`
				} `json:"attributes"`
			} `json:"value"`
		}
		if teamErr = readJSON(teamRes, &body); teamErr != nil {
			continue
		}
		for _, item := range body.Value {
			frame := strings.ToLower(item.Attributes.TimeFrame)
			if item.ID == "" || rank[frame] == 0 {
				continue
			}
			if rank[frame] > rank[frames[item.ID]] {
				frames[item.ID] = frame
			}
		}
	}
	return frames
}

func ExtractApiIterations(taskCtx plugin.SubTaskContext) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawIterationTable)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				ID          int                    `json:"id"`
				Identifier  string                 `json:"identifier"`
				Name        string                 `json:"name"`
				Path        string                 `json:"path"`
				HasChildren bool                   `json:"hasChildren"`
				Attributes  map[string]interface{} `json:"attributes"`
				TimeFrame   string                 `json:"_timeFrame"`
			}
			if err := errors.Convert(json.Unmarshal(row.Data, &raw)); err != nil {
				return nil, err
			}
			if raw.Identifier == "" {
				return nil, nil
			}
			attributes := raw.Attributes
			if attributes == nil {
				attributes = map[string]interface{}{}
			}
			return []interface{}{&models.AzuredevopsIteration{
				ConnectionId: data.Options.ConnectionId,
				Identifier:   raw.Identifier,
				NodeId:       raw.ID,
				OrgId:        data.Options.OrganizationId,
				ProjectId:    data.Options.ProjectId,
				Name:         raw.Name,
				Path:         raw.Path,
				PathKey:      iterationPathKey(raw.Path, data.Options.ProjectId),
				HasChildren:  raw.HasChildren,
				StartedDate:  parseAdoTime(attributes["startDate"]),
				EndedDate:    parseAdoTime(attributes["finishDate"]),
				TimeFrame:    raw.TimeFrame,
			}}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func ConvertIterations(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	iterationIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsIteration{})
	boardID := projectBoardID(data)
	return convertProjectRows(taskCtx, models.AzuredevopsIteration{}.TableName(), &models.AzuredevopsIteration{}, func(row interface{}) ([]interface{}, errors.Error) {
		iteration := row.(*models.AzuredevopsIteration)
		if iteration.HasChildren || iteration.PathKey == "" {
			return nil, nil
		}
		status := sprintStatus(iteration.TimeFrame, iteration.StartedDate, iteration.EndedDate)
		var completed *time.Time
		if status == "CLOSED" {
			completed = iteration.EndedDate
		}
		sprintID := iterationIdGen.Generate(data.Options.ConnectionId, iteration.PathKey)
		sprint := &ticket.Sprint{
			DomainEntity:    domainlayer.NewDomainEntity(sprintID),
			Name:            iteration.Name,
			Url:             fmt.Sprintf("https://dev.azure.com/%s/%s/_settings/work-team?_a=iterations", iteration.OrgId, iteration.ProjectId),
			Status:          status,
			StartedDate:     iteration.StartedDate,
			EndedDate:       iteration.EndedDate,
			CompletedDate:   completed,
			OriginalBoardID: boardID,
		}
		return []interface{}{
			sprint,
			&ticket.BoardSprint{BoardId: boardID, SprintId: sprintID},
		}, nil
	})
}

func sprintStatus(timeFrame string, started, ended *time.Time) string {
	switch strings.ToLower(timeFrame) {
	case "past":
		return "CLOSED"
	case "current":
		return "ACTIVE"
	case "future":
		return "FUTURE"
	}
	now := time.Now().UTC()
	if ended != nil && ended.Before(now) {
		return "CLOSED"
	}
	if started != nil && started.After(now) {
		return "FUTURE"
	}
	if started != nil {
		return "ACTIVE"
	}
	return ""
}
