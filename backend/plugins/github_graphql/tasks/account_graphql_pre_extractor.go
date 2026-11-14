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

	"github.com/apache/devlake/plugins/github/models"
)

// botLoginSuffix is how the REST API reports a bot login: `dependabot[bot]`.
// GraphQL's `Bot.login` omits it, so it is appended back to give a bot the same
// identity no matter which collector produced it.
const botLoginSuffix = `[bot]`

type GithubAccountEdge struct {
	Login     string
	Id        int `graphql:"databaseId"`
	Name      string
	Company   string
	Email     string
	AvatarUrl string
	HtmlUrl   string `graphql:"url"`
}

// GithubBotEdge holds what a `Bot` actor exposes. The GitHub schema gives `Bot`
// a narrower field set than `User`: no name, company or email.
type GithubBotEdge struct {
	Login     string
	Id        int `graphql:"databaseId"`
	AvatarUrl string
	HtmlUrl   string `graphql:"url"`
}

// GraphqlInlineAccountQuery is for fields the schema types as `User`, such as
// assignees and commit authors.
type GraphqlInlineAccountQuery struct {
	GithubAccountEdge `graphql:"... on User"`
}

// Account returns the account this query resolved to, or a zero value.
func (q *GraphqlInlineAccountQuery) Account() GithubAccountEdge {
	if q == nil {
		return GithubAccountEdge{}
	}
	return q.GithubAccountEdge
}

// GraphqlInlineActorQuery is for fields the schema types as `Actor`, such as a
// pull request author or the user who merged it. `Actor` is implemented by
// `User` and `Bot` among others, so spreading only `... on User` leaves every
// GitHub App, Dependabot or Renovate author empty.
//
// The two inline queries cannot be merged into one: spreading `... on Bot` on a
// `User`-typed field makes GitHub reject the whole query with
// "Fragment on Bot can't be spread inside User".
type GraphqlInlineActorQuery struct {
	TypeName          string `graphql:"__typename"`
	GithubAccountEdge `graphql:"... on User"`
	Bot               GithubBotEdge `graphql:"... on Bot"`
}

// Account returns the actor as an account, whichever type it resolved to.
func (q *GraphqlInlineActorQuery) Account() GithubAccountEdge {
	if q == nil {
		return GithubAccountEdge{}
	}
	// The GraphQL decoder populates overlapping fields in both fragments.
	// Only __typename identifies which fragment actually resolved.
	switch q.TypeName {
	case "User":
		return q.GithubAccountEdge
	case "Bot":
		// Normalize the bot fragment below.
	case "":
		// Historical raw records lack a discriminator. Preserve their original
		// user-first behavior, including when both fragments are populated.
		if q.GithubAccountEdge.Id != 0 {
			return q.GithubAccountEdge
		}
	default:
		return GithubAccountEdge{}
	}
	if q.Bot.Id == 0 {
		return GithubAccountEdge{}
	}
	login := q.Bot.Login
	if !strings.HasSuffix(login, botLoginSuffix) {
		login += botLoginSuffix
	}
	// Name, Company and Email stay empty: that is what the REST collector stores
	// for a bot as well.
	return GithubAccountEdge{
		Login:     login,
		Id:        q.Bot.Id,
		AvatarUrl: q.Bot.AvatarUrl,
		HtmlUrl:   q.Bot.HtmlUrl,
	}
}

func extractGraphqlPreAccount(result *[]interface{}, account GithubAccountEdge, repoId int, connId uint64) {
	if account.Id == 0 {
		return
	}
	*result = append(*result, &models.GithubRepoAccount{
		ConnectionId: connId,
		RepoGithubId: repoId,
		Login:        account.Login,
		AccountId:    account.Id,
	})
}
