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

package plugins

import (
	"testing"

	"github.com/apache/devlake/helpers/unithelper"
	ae "github.com/apache/devlake/plugins/ae/impl"
	argocd "github.com/apache/devlake/plugins/argocd/impl"
	asana "github.com/apache/devlake/plugins/asana/impl"
	azuredevops "github.com/apache/devlake/plugins/azuredevops_go/impl"
	bamboo "github.com/apache/devlake/plugins/bamboo/impl"
	bitbucket "github.com/apache/devlake/plugins/bitbucket/impl"
	bitbucket_server "github.com/apache/devlake/plugins/bitbucket_server/impl"
	circleci "github.com/apache/devlake/plugins/circleci/impl"
	claudeCode "github.com/apache/devlake/plugins/claude_code/impl"
	clickup "github.com/apache/devlake/plugins/clickup/impl"
	cursor "github.com/apache/devlake/plugins/cursor/impl"
	customize "github.com/apache/devlake/plugins/customize/impl"
	dora "github.com/apache/devlake/plugins/dora/impl"
	feishu "github.com/apache/devlake/plugins/feishu/impl"
	copilot "github.com/apache/devlake/plugins/gh-copilot/impl"
	gitee "github.com/apache/devlake/plugins/gitee/impl"
	gitextractor "github.com/apache/devlake/plugins/gitextractor/impl"
	github "github.com/apache/devlake/plugins/github/impl"
	githubGraphql "github.com/apache/devlake/plugins/github_graphql/impl"
	gitlab "github.com/apache/devlake/plugins/gitlab/impl"
	icla "github.com/apache/devlake/plugins/icla/impl"
	incidentio "github.com/apache/devlake/plugins/incidentio/impl"
	issueTrace "github.com/apache/devlake/plugins/issue_trace/impl"
	jenkins "github.com/apache/devlake/plugins/jenkins/impl"
	jira "github.com/apache/devlake/plugins/jira/impl"
	kiro "github.com/apache/devlake/plugins/kiro/impl"
	linear "github.com/apache/devlake/plugins/linear/impl"
	linker "github.com/apache/devlake/plugins/linker/impl"
	opsgenie "github.com/apache/devlake/plugins/opsgenie/impl"
	org "github.com/apache/devlake/plugins/org/impl"
	pagerduty "github.com/apache/devlake/plugins/pagerduty/impl"
	refdiff "github.com/apache/devlake/plugins/refdiff/impl"
	rootly "github.com/apache/devlake/plugins/rootly/impl"
	slack "github.com/apache/devlake/plugins/slack/impl"
	sonarqube "github.com/apache/devlake/plugins/sonarqube/impl"
	starrocks "github.com/apache/devlake/plugins/starrocks/impl"
	taiga "github.com/apache/devlake/plugins/taiga/impl"
	tapd "github.com/apache/devlake/plugins/tapd/impl"
	teambition "github.com/apache/devlake/plugins/teambition/impl"
	tempo "github.com/apache/devlake/plugins/tempo/impl"
	testmo "github.com/apache/devlake/plugins/testmo/impl"
	trello "github.com/apache/devlake/plugins/trello/impl"
	webhook "github.com/apache/devlake/plugins/webhook/impl"
	youtrack "github.com/apache/devlake/plugins/youtrack/impl"
	zentao "github.com/apache/devlake/plugins/zentao/impl"
)

func Test_GetPluginTablesInfo(t *testing.T) {
	// Make sure EVERY Go plugin is listed here
	checker := unithelper.NewTableInfoChecker(unithelper.TableInfoCheckerConfig{
		ValidatePluginCount: true,
	})
	checker.FeedIn("ae/models", ae.AE{}.GetTablesInfo)
	checker.FeedIn("azuredevops_go/models", azuredevops.Azuredevops{}.GetTablesInfo)
	checker.FeedIn("bamboo/models", bamboo.Bamboo{}.GetTablesInfo)
	checker.FeedIn("bitbucket/models", bitbucket.Bitbucket{}.GetTablesInfo)
	checker.FeedIn("bitbucket_server/models", bitbucket_server.BitbucketServer{}.GetTablesInfo)
	checker.FeedIn("argocd/models", argocd.ArgoCD{}.GetTablesInfo)
	checker.FeedIn("asana/models", asana.Asana{}.GetTablesInfo)
	checker.FeedIn("customize/models", customize.Customize{}.GetTablesInfo)
	checker.FeedIn("dora/models", dora.Dora{}.GetTablesInfo)
	checker.FeedIn("feishu/models", feishu.Feishu{}.GetTablesInfo)
	checker.FeedIn("gitee/models", gitee.Gitee{}.GetTablesInfo)
	checker.FeedIn("gitextractor/models", gitextractor.GitExtractor{}.GetTablesInfo)
	checker.FeedIn("github/models", github.Github{}.GetTablesInfo)
	checker.FeedIn("github_graphql", githubGraphql.GithubGraphql{}.GetTablesInfo)
	checker.FeedIn("gitlab/models", gitlab.Gitlab{}.GetTablesInfo)
	checker.FeedIn("icla/models", icla.Icla{}.GetTablesInfo)
	checker.FeedIn("incidentio/models", incidentio.Incidentio{}.GetTablesInfo)
	checker.FeedIn("jenkins/models", jenkins.Jenkins{}.GetTablesInfo)
	checker.FeedIn("jira/models", jira.Jira{}.GetTablesInfo)
	checker.FeedIn("linear/models", linear.Linear{}.GetTablesInfo)
	checker.FeedIn("org", org.Org{}.GetTablesInfo)
	checker.FeedIn("pagerduty/models", pagerduty.PagerDuty{}.GetTablesInfo)
	checker.FeedIn("refdiff/models", refdiff.RefDiff{}.GetTablesInfo)
	checker.FeedIn("rootly/models", rootly.Rootly{}.GetTablesInfo)
	checker.FeedIn("slack/models", slack.Slack{}.GetTablesInfo)
	checker.FeedIn("sonarqube/models", sonarqube.Sonarqube{}.GetTablesInfo)
	checker.FeedIn("starrocks", starrocks.StarRocks{}.GetTablesInfo)
	checker.FeedIn("taiga/models", taiga.Taiga{}.GetTablesInfo)
	checker.FeedIn("tapd/models", tapd.Tapd{}.GetTablesInfo)
	checker.FeedIn("teambition/models", teambition.Teambition{}.GetTablesInfo)
	checker.FeedIn("tempo/models", tempo.Tempo{}.GetTablesInfo)
	checker.FeedIn("testmo/models", testmo.Testmo{}.GetTablesInfo)
	checker.FeedIn("trello/models", trello.Trello{}.GetTablesInfo)
	checker.FeedIn("webhook/models", webhook.Webhook{}.GetTablesInfo)
	checker.FeedIn("youtrack/models", youtrack.Youtrack{}.GetTablesInfo)
	checker.FeedIn("zentao/models", zentao.Zentao{}.GetTablesInfo)
	checker.FeedIn("claude_code/models", claudeCode.ClaudeCode{}.GetTablesInfo)
	checker.FeedIn("cursor/models", cursor.Cursor{}.GetTablesInfo)
	checker.FeedIn("circleci/models", circleci.Circleci{}.GetTablesInfo)
	checker.FeedIn("clickup/models", clickup.ClickUp{}.GetTablesInfo)
	checker.FeedIn("opsgenie/models", opsgenie.Opsgenie{}.GetTablesInfo)
	checker.FeedIn("linker/models", linker.Linker{}.GetTablesInfo)
	checker.FeedIn("issue_trace/models", issueTrace.IssueTrace{}.GetTablesInfo)
	checker.FeedIn("kiro/models", kiro.Kiro{}.GetTablesInfo)
	checker.FeedIn("gh-copilot/models", copilot.GhCopilot{}.GetTablesInfo)
	err := checker.Verify()
	if err != nil {
		t.Error(err)
	}
}
