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
	"os"
	"testing"

	"github.com/apache/devlake/core/plugin"
)

// jiraPluginStub satisfies plugin.PluginMeta so that didgen.NewDomainIdGenerator
// can resolve model types from github.com/apache/devlake/plugins/jira/...
// in unit tests without importing the full jira impl package.
type jiraPluginStub struct{}

func (jiraPluginStub) Name() string        { return "jira" }
func (jiraPluginStub) RootPkgPath() string { return "github.com/apache/devlake/plugins/jira" }
func (jiraPluginStub) Description() string { return "" }

func TestMain(m *testing.M) {
	// Register the jira plugin so that didgen can resolve jira model types
	// (e.g. JiraSprint, JiraIssue) without a running DevLake server.
	_ = plugin.RegisterPlugin("jira", jiraPluginStub{})
	// Also register ourselves so that domain ID generation for our own types works.
	_ = plugin.RegisterPlugin("jira_snowflake", JiraSnowflake{})
	os.Exit(m.Run())
}

// JiraSnowflake is a minimal PluginMeta stub for jira_snowflake model types.
type JiraSnowflake struct{}

func (JiraSnowflake) Name() string { return "jira_snowflake" }
func (JiraSnowflake) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/jira_snowflake"
}
func (JiraSnowflake) Description() string { return "" }
