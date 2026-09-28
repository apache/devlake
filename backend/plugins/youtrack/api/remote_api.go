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
	"fmt"
	"net/url"
	"strconv"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/youtrack/models"
)

// YoutrackRemotePagination drives `$skip`/`$top` pagination through the
// `/admin/projects` listing when listing remote scopes for the config UI.
type YoutrackRemotePagination struct {
	Skip int `json:"skip"`
	Top  int `json:"top"`
}

const remoteScopesPageSize = 100

// youtrackProject mirrors the shape of a YouTrack `Project` as returned by
// `GET /admin/projects`. The scope
// picker's fields query requests exactly these attributes.
type youtrackProject struct {
	Id          string `json:"id"`
	ShortName   string `json:"shortName"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
}

// listYoutrackRemoteScopes lists the instance's projects as selectable
// scopes. YouTrack projects are a flat list, so there are no intermediate
// groups. Despite the `/admin/` path segment, visibility follows the caller's
// project permissions, so any token that can read projects works here.
func listYoutrackRemoteScopes(
	_ *models.YoutrackConnection,
	apiClient plugin.ApiClient,
	_ string,
	page YoutrackRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.YoutrackProject],
	nextPage *YoutrackRemotePagination,
	err errors.Error,
) {
	if page.Top == 0 {
		page.Top = remoteScopesPageSize
	}
	query := url.Values{
		"fields": {"id,shortName,name,description,archived"},
		"$top":   {strconv.Itoa(page.Top)},
		"$skip":  {strconv.Itoa(page.Skip)},
	}
	res, err := apiClient.Get("admin/projects", query, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to query YouTrack projects")
	}
	var projects []youtrackProject
	if err := api.UnmarshalResponse(res, &projects); err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal YouTrack projects response")
	}
	return mapYoutrackProjectsToScopeEntries(projects), nextPageFromYoutrack(page, len(projects)), nil
}

// mapYoutrackProjectsToScopeEntries converts a projects page into scope-list
// entries. Each project is a selectable (leaf) scope. The display name is
// `shortName (name)` — users recognize projects by the issue-key prefix
// (`PROJ` in `PROJ-123`), not by the internal id. Archived projects stay
// selectable but are visibly de-emphasized.
func mapYoutrackProjectsToScopeEntries(projects []youtrackProject) []dsmodels.DsRemoteApiScopeListEntry[models.YoutrackProject] {
	children := make([]dsmodels.DsRemoteApiScopeListEntry[models.YoutrackProject], 0, len(projects))
	for _, project := range projects {
		project := project
		displayName := fmt.Sprintf("%s (%s)", project.ShortName, project.Name)
		if project.Archived {
			displayName = fmt.Sprintf("%s [archived]", displayName)
		}
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.YoutrackProject]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: nil,
			Id:       project.Id,
			Name:     displayName,
			FullName: displayName,
			Data: &models.YoutrackProject{
				Id:          project.Id,
				ShortName:   project.ShortName,
				Name:        project.Name,
				Description: project.Description,
				Archived:    project.Archived,
			},
		})
	}
	return children
}

// nextPageFromYoutrack returns the cursor for the following page, or nil when
// the projects listing has been fully traversed (a short page is the last
// one; a full page may have more behind it).
func nextPageFromYoutrack(page YoutrackRemotePagination, fetched int) *YoutrackRemotePagination {
	if fetched < page.Top {
		return nil
	}
	return &YoutrackRemotePagination{Skip: page.Skip + page.Top, Top: page.Top}
}

// RemoteScopes lists the YouTrack projects visible to the connection so the
// config UI can enumerate selectable scopes.
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}
