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

// Package schema_e2e contains a cross-plugin regression guard that runs the
// REAL migration scripts of every built-in Go plugin and then asserts that the
// resulting database schema still matches what each runtime GORM model expects.
//
// It lives in an `e2e` package on purpose: it needs a real database
// (E2E_DB_URL) and is therefore only executed by `make e2e-test-go-plugins`
// (scripts/e2e-test-go-plugins.sh selects packages whose import path contains
// "e2e"), and excluded from the DB-less unit test run
// (scripts/unit-test-go.sh skips paths matching "e2e").
package schema_e2e

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/migration"
	coreMigration "github.com/apache/devlake/core/models/migrationscripts"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/impls/dalgorm"
	"github.com/apache/devlake/impls/logruslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"

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
	grafanaIrm "github.com/apache/devlake/plugins/grafana_irm/impl"
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
	zentao "github.com/apache/devlake/plugins/zentao/impl"
)

// allGoPlugins lists EVERY built-in Go plugin. Keep it in sync with the plugin
// directories under backend/plugins/ (the TestAllGoPluginsListed guard below
// fails if a new plugin's `impl` package is added but not registered here).
func allGoPlugins() []plugin.PluginMeta {
	return []plugin.PluginMeta{
		ae.AE{},
		argocd.ArgoCD{},
		asana.Asana{},
		azuredevops.Azuredevops{},
		bamboo.Bamboo{},
		bitbucket.Bitbucket{},
		bitbucket_server.BitbucketServer{},
		circleci.Circleci{},
		claudeCode.ClaudeCode{},
		clickup.ClickUp{},
		cursor.Cursor{},
		customize.Customize{},
		dora.Dora{},
		feishu.Feishu{},
		copilot.GhCopilot{},
		gitee.Gitee{},
		gitextractor.GitExtractor{},
		github.Github{},
		githubGraphql.GithubGraphql{},
		gitlab.Gitlab{},
		grafanaIrm.GrafanaIrm{},
		icla.Icla{},
		incidentio.Incidentio{},
		issueTrace.IssueTrace{},
		jenkins.Jenkins{},
		jira.Jira{},
		kiro.Kiro{},
		linear.Linear{},
		linker.Linker{},
		opsgenie.Opsgenie{},
		org.Org{},
		pagerduty.PagerDuty{},
		refdiff.RefDiff{},
		rootly.Rootly{},
		slack.Slack{},
		sonarqube.Sonarqube{},
		starrocks.StarRocks{},
		taiga.Taiga{},
		tapd.Tapd{},
		teambition.Teambition{},
		tempo.Tempo{},
		testmo.Testmo{},
		trello.Trello{},
		webhook.Webhook{},
		zentao.Zentao{},
	}
}

// TestAllGoPluginsListed guarantees allGoPlugins() stays complete: it counts the
// plugin directories that ship an `impl` package and fails if that number does
// not match the registered list. This makes the schema-drift guard below
// automatically cover any newly added plugin.
func TestAllGoPluginsListed(t *testing.T) {
	entries, err := os.ReadDir("..")
	require.NoError(t, err)
	dirsWithImpl := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if info, statErr := os.Stat(filepath.Join("..", e.Name(), "impl")); statErr == nil && info.IsDir() {
			dirsWithImpl++
		}
	}
	assert.Equalf(t, dirsWithImpl, len(allGoPlugins()),
		"number of plugin dirs with an impl/ package (%d) != registered plugins (%d); "+
			"add the new plugin to allGoPlugins() in plugins/schema_e2e/migration_schema_test.go",
		dirsWithImpl, len(allGoPlugins()))
}

// TestMigrationSchemaMatchesModels applies the real framework + plugin migration
// scripts and then verifies, for every plugin model, that each column the
// runtime GORM model declares actually exists in the migrated table.
//
// This is a cross-plugin generalization of the Jira Sprint Report regression:
// a migration created `_tool_jira_sprint_reports` without embedding
// common.NoPKModel, so the `_raw_data_*` columns were missing and the
// ApiExtractor cleanup query failed at runtime with
// "Unknown column '_raw_data_table' in 'where clause'".
//
// Tables that no migration creates (e.g. models materialized lazily at runtime)
// are skipped, so the check specifically targets *drift* between an existing
// table and its model — which is exactly the failure mode above.
//
// The migrations run against a dedicated, empty database (see
// e2ehelper.NewIsolatedMigrationDb) because the shared e2e database is polluted
// by the other e2e tests, which AutoMigrate tables without recording anything
// in `_devlake_migration_history`.
func TestMigrationSchemaMatchesModels(t *testing.T) {
	db := e2ehelper.NewIsolatedMigrationDb(t, "schema_drift")
	dalInstance := dalgorm.NewDalgorm(db)
	basicRes := runner.CreateBasicRes(config.GetConfig(), logruslog.Global, db)

	// Apply the migrations exactly the way the server does on startup.
	migrator, migErr := migration.NewMigrator(basicRes)
	require.NoError(t, migErr)
	migrator.Register(coreMigration.All(), "Framework")
	for _, p := range allGoPlugins() {
		if migratable, ok := p.(plugin.PluginMigration); ok {
			migrator.Register(migratable.MigrationScripts(), p.Name())
		}
	}
	require.NoError(t, migrator.Execute())

	keepAll := func(dal.ColumnMeta) bool { return true }

	for _, p := range allGoPlugins() {
		modeler, ok := p.(plugin.PluginModel)
		if !ok {
			continue
		}
		p := p
		t.Run(p.Name(), func(t *testing.T) {
			for _, table := range modeler.GetTablesInfo() {
				table := table
				// Columns that actually exist in the migrated table.
				actualColumns, colErr := dal.GetColumnNames(dalInstance, table, keepAll)
				if colErr != nil || len(actualColumns) == 0 {
					// No migration created this table (e.g. runtime-only /
					// dynamic model) — nothing to validate for drift.
					t.Logf("skip %q: table not present after migrations", table.TableName())
					continue
				}
				existing := make(map[string]struct{}, len(actualColumns))
				for _, c := range actualColumns {
					existing[c] = struct{}{}
				}

				// Columns the runtime GORM model expects.
				sch, parseErr := schema.Parse(table, &sync.Map{}, schema.NamingStrategy{})
				require.NoErrorf(t, parseErr, "unable to parse schema for %T", table)
				for _, field := range sch.Fields {
					if field.DBName == "" || field.IgnoreMigration {
						continue
					}
					_, present := existing[field.DBName]
					assert.Truef(t, present,
						"[%s] table %q is missing column %q expected by model %T — "+
							"did a migration script forget to embed common.NoPKModel (raw-data columns) or add the field?",
						p.Name(), table.TableName(), field.DBName, table)
				}
			}
		})
	}
}
