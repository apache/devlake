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

package services

import (
	"testing"

	"github.com/apache/devlake/core/plugin"
	ae "github.com/apache/devlake/plugins/ae/impl"
	bamboo "github.com/apache/devlake/plugins/bamboo/impl"
	bitbucket "github.com/apache/devlake/plugins/bitbucket/impl"
	customize "github.com/apache/devlake/plugins/customize/impl"
	dora "github.com/apache/devlake/plugins/dora/impl"
	feishu "github.com/apache/devlake/plugins/feishu/impl"
	gitee "github.com/apache/devlake/plugins/gitee/impl"
	gitextractor "github.com/apache/devlake/plugins/gitextractor/impl"
	github "github.com/apache/devlake/plugins/github/impl"
	githubGraphql "github.com/apache/devlake/plugins/github_graphql/impl"
	gitlab "github.com/apache/devlake/plugins/gitlab/impl"
	grafanaIrm "github.com/apache/devlake/plugins/grafana_irm/impl"
	icla "github.com/apache/devlake/plugins/icla/impl"
	incidentio "github.com/apache/devlake/plugins/incidentio/impl"
	jenkins "github.com/apache/devlake/plugins/jenkins/impl"
	jira "github.com/apache/devlake/plugins/jira/impl"
	org "github.com/apache/devlake/plugins/org/impl"
	pagerduty "github.com/apache/devlake/plugins/pagerduty/impl"
	refdiff "github.com/apache/devlake/plugins/refdiff/impl"
	rootly "github.com/apache/devlake/plugins/rootly/impl"
	slack "github.com/apache/devlake/plugins/slack/impl"
	sonarqube "github.com/apache/devlake/plugins/sonarqube/impl"
	starrocks "github.com/apache/devlake/plugins/starrocks/impl"
	tapd "github.com/apache/devlake/plugins/tapd/impl"
	teambition "github.com/apache/devlake/plugins/teambition/impl"
	testmo "github.com/apache/devlake/plugins/testmo/impl"
	trello "github.com/apache/devlake/plugins/trello/impl"
	webhook "github.com/apache/devlake/plugins/webhook/impl"
	zentao "github.com/apache/devlake/plugins/zentao/impl"
	"github.com/apache/devlake/test/helper"
	"github.com/stretchr/testify/require"
)

func TestStartup(t *testing.T) {
	client := helper.StartDevLakeServer(t, loadGoPlugins())
	projects := client.ListProjects()
	require.Equal(t, 0, int(projects.Count))
}

func loadGoPlugins() []plugin.PluginMeta {
	return []plugin.PluginMeta{
		ae.AE{},
		bamboo.Bamboo{},
		bitbucket.Bitbucket{},
		customize.Customize{},
		dora.Dora{},
		feishu.Feishu{},
		gitee.Gitee{},
		gitextractor.GitExtractor{},
		github.Github{},
		githubGraphql.GithubGraphql{},
		gitlab.Gitlab{},
		grafanaIrm.GrafanaIrm{},
		icla.Icla{},
		incidentio.Incidentio{},
		jenkins.Jenkins{},
		jira.Jira{},
		org.Org{},
		pagerduty.PagerDuty{},
		refdiff.RefDiff{},
		rootly.Rootly{},
		slack.Slack{},
		sonarqube.Sonarqube{},
		starrocks.StarRocks{},
		tapd.Tapd{},
		teambition.Teambition{},
		testmo.Testmo{},
		trello.Trello{},
		webhook.Webhook{},
		zentao.Zentao{},
	}
}
