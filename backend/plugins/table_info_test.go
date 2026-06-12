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

	"github.com/apache/incubator-devlake/helpers/unithelper"
	ae "github.com/apache/incubator-devlake/plugins/ae/impl"
	argocd "github.com/apache/incubator-devlake/plugins/argocd/impl"
	asana "github.com/apache/incubator-devlake/plugins/asana/impl"
	azuredevops "github.com/apache/incubator-devlake/plugins/azuredevops_go/impl"
	bamboo "github.com/apache/incubator-devlake/plugins/bamboo/impl"
	bitbucket "github.com/apache/incubator-devlake/plugins/bitbucket/impl"
	bitbucket_server "github.com/apache/incubator-devlake/plugins/bitbucket_server/impl"
	claudeCode "github.com/apache/incubator-devlake/plugins/claude_code/impl"
	circleci "github.com/apache/incubator-devlake/plugins/circleci/impl"
	customize "github.com/apache/incubator-devlake/plugins/customize/impl"
	dbt "github.com/apache/incubator-devlake/plugins/dbt/impl"
	dora "github.com/apache/incubator-devlake/plugins/dora/impl"
	feishu "github.com/apache/incubator-devlake/plugins/feishu/impl"
	copilot "github.com/apache/incubator-devlake/plugins/gh-copilot/impl"
	gitee "github.com/apache/incubator-devlake/plugins/gitee/impl"
	gitextractor "github.com/apache/incubator-devlake/plugins/gitextractor/impl"
	github "github.com/apache/incubator-devlake/plugins/github/impl"
	githubGraphql "github.com/apache/incubator-devlake/plugins/github_graphql/impl"
	gitlab "github.com/apache/incubator-devlake/plugins/gitlab/impl"
	icla "github.com/apache/incubator-devlake/plugins/icla/impl"
	issueTrace "github.com/apache/incubator-devlake/plugins/issue_trace/impl"
	jenkins "github.com/apache/incubator-devlake/plugins/jenkins/impl"
	jira "github.com/apache/incubator-devlake/plugins/jira/impl"
	linker "github.com/apache/incubator-devlake/plugins/linker/impl"
	opsgenie "github.com/apache/incubator-devlake/plugins/opsgenie/impl"
	org "github.com/apache/incubator-devlake/plugins/org/impl"
	pagerduty "github.com/apache/incubator-devlake/plugins/pagerduty/impl"
	q_dev "github.com/apache/incubator-devlake/plugins/q_dev/impl"
	refdiff "github.com/apache/incubator-devlake/plugins/refdiff/impl"
	rootly "github.com/apache/incubator-devlake/plugins/rootly/impl"
	slack "github.com/apache/incubator-devlake/plugins/slack/impl"
	sonarqube "github.com/apache/incubator-devlake/plugins/sonarqube/impl"
	starrocks "github.com/apache/incubator-devlake/plugins/starrocks/impl"
	taiga "github.com/apache/incubator-devlake/plugins/taiga/impl"
	tapd "github.com/apache/incubator-devlake/plugins/tapd/impl"
	teambition "github.com/apache/incubator-devlake/plugins/teambition/impl"
	tempo "github.com/apache/incubator-devlake/plugins/tempo/impl"
	testmo "github.com/apache/incubator-devlake/plugins/testmo/impl"
	trello "github.com/apache/incubator-devlake/plugins/trello/impl"
	webhook "github.com/apache/incubator-devlake/plugins/webhook/impl"
	zentao "github.com/apache/incubator-devlake/plugins/zentao/impl"
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
	checker.FeedIn("dbt", dbt.Dbt{}.GetTablesInfo)
	checker.FeedIn("dora/models", dora.Dora{}.GetTablesInfo)
	checker.FeedIn("feishu/models", feishu.Feishu{}.GetTablesInfo)
	checker.FeedIn("gitee/models", gitee.Gitee{}.GetTablesInfo)
	checker.FeedIn("gitextractor/models", gitextractor.GitExtractor{}.GetTablesInfo)
	checker.FeedIn("github/models", github.Github{}.GetTablesInfo)
	checker.FeedIn("github_graphql", githubGraphql.GithubGraphql{}.GetTablesInfo)
	checker.FeedIn("gitlab/models", gitlab.Gitlab{}.GetTablesInfo)
	checker.FeedIn("icla/models", icla.Icla{}.GetTablesInfo)
	checker.FeedIn("jenkins/models", jenkins.Jenkins{}.GetTablesInfo)
	checker.FeedIn("jira/models", jira.Jira{}.GetTablesInfo)
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
	checker.FeedIn("zentao/models", zentao.Zentao{}.GetTablesInfo)
	checker.FeedIn("claude_code/models", claudeCode.ClaudeCode{}.GetTablesInfo)
	checker.FeedIn("circleci/models", circleci.Circleci{}.GetTablesInfo)
	checker.FeedIn("opsgenie/models", opsgenie.Opsgenie{}.GetTablesInfo)
	checker.FeedIn("linker/models", linker.Linker{}.GetTablesInfo)
	checker.FeedIn("issue_trace/models", issueTrace.IssueTrace{}.GetTablesInfo)
	checker.FeedIn("q_dev/models", q_dev.QDev{}.GetTablesInfo)
	checker.FeedIn("gh-copilot/models", copilot.GhCopilot{}.GetTablesInfo)
	err := checker.Verify()
	if err != nil {
		t.Error(err)
	}
}
