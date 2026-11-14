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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/plugins/github/models"
)

type GithubAccountResponse struct {
	Login             string `json:"login"`
	Id                int    `json:"id"`
	NodeId            string `json:"node_id"`
	AvatarUrl         string `json:"avatar_url"`
	GravatarId        string `json:"gravatar_id"`
	Url               string `json:"url"`
	HtmlUrl           string `json:"html_url"`
	FollowersUrl      string `json:"followers_url"`
	FollowingUrl      string `json:"following_url"`
	GistsUrl          string `json:"gists_url"`
	StarredUrl        string `json:"starred_url"`
	SubscriptionsUrl  string `json:"subscriptions_url"`
	OrganizationsUrl  string `json:"organizations_url"`
	ReposUrl          string `json:"repos_url"`
	EventsUrl         string `json:"events_url"`
	ReceivedEventsUrl string `json:"received_events_url"`
	Type              string `json:"type"`
	SiteAdmin         bool   `json:"site_admin"`
}

func convertAccount(res *GithubAccountResponse, repoId int, connId uint64) (*models.GithubRepoAccount, errors.Error) {
	login := res.Login
	// Some REST endpoints return bot aliases without the usual suffix. Keep
	// the explicit type available to conversion even without profile enrichment.
	if res.Type == "Bot" && !strings.HasSuffix(login, "[bot]") {
		login += "[bot]"
	}
	githubAccount := &models.GithubRepoAccount{
		ConnectionId: connId,
		RepoGithubId: repoId,
		Login:        login,
		AccountId:    res.Id,
	}
	return githubAccount, nil
}
