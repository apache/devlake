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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInlineAccountQueryAccount(t *testing.T) {
	var nilQuery *GraphqlInlineAccountQuery
	assert.Equal(t, 0, nilQuery.Account().Id)

	user := &GraphqlInlineAccountQuery{
		GithubAccountEdge: GithubAccountEdge{Login: `octocat`, Id: 583231, Name: `The Octocat`},
	}
	assert.Equal(t, `octocat`, user.Account().Login)
	assert.Equal(t, 583231, user.Account().Id)
}

func TestInlineActorQueryAccount(t *testing.T) {
	var nilQuery *GraphqlInlineActorQuery
	assert.Equal(t, GithubAccountEdge{}, nilQuery.Account())

	// an actor that resolved to neither User nor Bot, e.g. a deleted account
	empty := &GraphqlInlineActorQuery{}
	assert.Equal(t, GithubAccountEdge{}, empty.Account())

	user := &GraphqlInlineActorQuery{
		GithubAccountEdge: GithubAccountEdge{Login: `octocat`, Id: 583231, Email: `octocat@github.com`},
	}
	assert.Equal(t, `octocat`, user.Account().Login)
	assert.Equal(t, `octocat@github.com`, user.Account().Email)

	// Bot.login has no `[bot]` suffix, the REST collector stores one
	bot := &GraphqlInlineActorQuery{
		Bot: GithubBotEdge{Login: `dependabot`, Id: 49699333, HtmlUrl: `https://github.com/apps/dependabot`},
	}
	assert.Equal(t, `dependabot[bot]`, bot.Account().Login)
	assert.Equal(t, 49699333, bot.Account().Id)
	assert.Equal(t, `https://github.com/apps/dependabot`, bot.Account().HtmlUrl)
	assert.Empty(t, bot.Account().Name)

	// a login that already carries the suffix is left alone
	suffixed := &GraphqlInlineActorQuery{
		Bot: GithubBotEdge{Login: `renovate[bot]`, Id: 29139614},
	}
	assert.Equal(t, `renovate[bot]`, suffixed.Account().Login)
}

// the raw layer holds the marshalled query result, so a bot author has to survive
// the round trip the extractors read it back through
func TestInlineActorQueryUnmarshalBotAuthor(t *testing.T) {
	raw := `{"Login":"","Id":0,"Name":"","Company":"","Email":"","AvatarUrl":"","HtmlUrl":"",
		"Bot":{"Login":"dependabot","Id":49699333,"AvatarUrl":"","HtmlUrl":""}}`
	actor := &GraphqlInlineActorQuery{}
	assert.Nil(t, json.Unmarshal([]byte(raw), actor))
	assert.Equal(t, `dependabot[bot]`, actor.Account().Login)
	assert.Equal(t, 49699333, actor.Account().Id)

	// rows collected before the Bot fragment existed still extract as before
	legacy := `{"Login":"octocat","Id":583231,"Name":"The Octocat","Company":"","Email":"","AvatarUrl":"","HtmlUrl":""}`
	actor = &GraphqlInlineActorQuery{}
	assert.Nil(t, json.Unmarshal([]byte(legacy), actor))
	assert.Equal(t, `octocat`, actor.Account().Login)
}

func TestExtractGraphqlPreAccountSkipsUnresolvedActors(t *testing.T) {
	var results []interface{}
	var missing *GraphqlInlineActorQuery
	extractGraphqlPreAccount(&results, missing.Account(), 1, 1)
	assert.Empty(t, results)

	bot := &GraphqlInlineActorQuery{Bot: GithubBotEdge{Login: `dependabot`, Id: 49699333}}
	extractGraphqlPreAccount(&results, bot.Account(), 1, 1)
	assert.Len(t, results, 1)
}

// The client decoder fills overlapping fields in both inline fragments. Exercise
// that decoder, then the same JSON round trip used by collection and extraction.
func TestInlineActorQueryClientRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, actorType, login, wantLogin string
		id                                int
	}{
		{"copilot", "Bot", "copilot-pull-request-reviewer", "copilot-pull-request-reviewer[bot]", 175728472},
		{"human", "User", "octocat", "octocat", 583231},
		{"suffixed bot", "Bot", "renovate[bot]", "renovate[bot]", 29139614},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := graphql.NewClient("https://github.invalid/graphql", &http.Client{
				Transport: actorRoundTripper(func(req *http.Request) (*http.Response, error) {
					var request struct{ Query string }
					require.NoError(t, json.NewDecoder(req.Body).Decode(&request))
					assert.Contains(t, request.Query, "__typename")
					actor := map[string]interface{}{
						"login": tc.login, "databaseId": tc.id,
						"avatarUrl": "https://github.invalid/avatar", "url": "https://github.invalid/profile",
					}
					// Return only requested fields, as GitHub would. This also lets the
					// regression expose the old accessor's incorrect login before the fix.
					if strings.Contains(request.Query, "__typename") {
						actor["__typename"] = tc.actorType
					}
					if tc.actorType == "User" {
						actor["name"], actor["company"], actor["email"] = "The Octocat", "GitHub", "octocat@github.com"
					}
					body, err := json.Marshal(map[string]interface{}{"data": map[string]interface{}{"author": actor}})
					require.NoError(t, err)
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
				}),
			})
			var query struct{ Author *GraphqlInlineActorQuery }
			dataErrors, err := client.Query(context.Background(), &query, nil)
			require.NoError(t, err)
			require.Empty(t, dataErrors)
			require.NotNil(t, query.Author)
			// This is the decoder behavior that hand-built fragment tests missed.
			assert.Equal(t, tc.id, query.Author.GithubAccountEdge.Id)
			assert.Equal(t, tc.id, query.Author.Bot.Id)
			raw, err := json.Marshal(query)
			require.NoError(t, err)
			var extracted struct{ Author *GraphqlInlineActorQuery }
			require.NoError(t, json.Unmarshal(raw, &extracted))
			account := extracted.Author.Account()
			assert.Equal(t, tc.wantLogin, account.Login)
			assert.Equal(t, tc.id, account.Id)
			assert.Equal(t, "https://github.invalid/avatar", account.AvatarUrl)
			assert.Equal(t, "https://github.invalid/profile", account.HtmlUrl)
			if tc.actorType == "User" {
				assert.Equal(t, "The Octocat", account.Name)
				assert.Equal(t, "octocat@github.com", account.Email)
			} else {
				assert.Empty(t, account.Name)
				assert.Empty(t, account.Email)
			}
		})
	}
}

type actorRoundTripper func(*http.Request) (*http.Response, error)

func (f actorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestInlineActorQueryRawCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      GithubAccountEdge
	}{
		{"null", `null`, GithubAccountEdge{}},
		{"unresolved", `{}`, GithubAccountEdge{}},
		{"unsupported explicit type", `{"TypeName":"Organization","Login":"org","Id":1,"Bot":{"Login":"org","Id":1}}`, GithubAccountEdge{}},
		{"unresolved explicit bot", `{"TypeName":"Bot","Login":"stale-user","Id":1}`, GithubAccountEdge{}},
		{"explicit user selects user only", `{"TypeName":"User","Bot":{"Login":"stale-bot","Id":1}}`, GithubAccountEdge{}},
		{"legacy user", `{"Login":"octocat","Id":583231}`, GithubAccountEdge{Login: "octocat", Id: 583231}},
		{"legacy bot", `{"Bot":{"Login":"dependabot","Id":49699333}}`, GithubAccountEdge{Login: "dependabot[bot]", Id: 49699333}},
		// Old ambiguous records keep the historical user-first behavior; only a
		// fresh collection can supply the missing type discriminator.
		{"legacy overlapping fragments", `{"Login":"copilot-pull-request-reviewer","Id":175728472,"Bot":{"Login":"copilot-pull-request-reviewer","Id":175728472}}`, GithubAccountEdge{Login: "copilot-pull-request-reviewer", Id: 175728472}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var actor *GraphqlInlineActorQuery
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &actor))
			assert.Equal(t, tc.want, actor.Account())
		})
	}
}

// Bot fragments and actor type discriminators must never reach User-only fields.
func TestBotFragmentOnlySpreadOnActorFields(t *testing.T) {
	for _, query := range []interface{}{
		&GraphqlQueryPrWrapper{}, &GraphqlQueryPrDetailWrapper{},
		&GraphqlQueryIssueWrapper{}, &GraphqlQueryIssueDetailWrapper{},
	} {
		generated, _ := graphql.ConstructQuery(query, map[string]interface{}{
			"pullRequest": []map[string]interface{}{{"number": graphql.Int(1)}},
			"issue":       []map[string]interface{}{{"number": graphql.Int(1)}},
		})
		actorSelection := `{__typename,... on User{login,databaseId,name,company,email,avatarUrl,url},... on Bot{login,databaseId,avatarUrl,url}}`
		assert.Contains(t, generated, "author"+actorSelection)
		if strings.Contains(generated, "mergedBy") {
			assert.Contains(t, generated, "mergedBy"+actorSelection)
			// PR authors, review authors and mergedBy are all Actor fields.
			assert.Equal(t, 3, strings.Count(generated, actorSelection))
		}
		for _, prefix := range []string{"assignees(first: 1){nodes", "assignees(first: 100){nodes", "user"} {
			if strings.Contains(generated, prefix+"{") {
				assert.Contains(t, generated, prefix+`{... on User{login,databaseId,name,company,email,avatarUrl,url}}`)
			}
		}
	}
}

func TestInlineActorQueryClientUnresolved(t *testing.T) {
	for _, actorJSON := range []string{`null`, `{}`, `{"__typename":"Mannequin"}`} {
		t.Run(actorJSON, func(t *testing.T) {
			client := graphql.NewClient("https://github.invalid/graphql", &http.Client{
				Transport: actorRoundTripper(func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{"data":{"author":` + actorJSON + `}}`))}, nil
				}),
			})
			var query struct{ Author *GraphqlInlineActorQuery }
			dataErrors, err := client.Query(context.Background(), &query, nil)
			require.NoError(t, err)
			require.Empty(t, dataErrors)
			raw, err := json.Marshal(query)
			require.NoError(t, err)
			var extracted struct{ Author *GraphqlInlineActorQuery }
			require.NoError(t, json.Unmarshal(raw, &extracted))
			assert.Equal(t, GithubAccountEdge{}, extracted.Author.Account())
		})
	}
}
