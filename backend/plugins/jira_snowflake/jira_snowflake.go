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

package main

import (
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/plugins/jira_snowflake/impl"
	"github.com/spf13/cobra"
)

// PluginEntry is exported for the DevLake framework to discover and load this plugin.
var PluginEntry impl.JiraSnowflake //nolint

func main() {
	cmd := &cobra.Command{Use: "jira_snowflake"}
	boardId := cmd.Flags().Uint64P("boardId", "b", 0, "Jira board ID to sync")
	projectKeys := cmd.Flags().StringSliceP("projectKeys", "p", nil, "Jira project keys for this board")
	timeAfter := cmd.Flags().StringP("timeAfter", "a", "", "only sync records created/updated after this time (RFC3339)")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		runner.DirectRun(cmd, args, PluginEntry, map[string]interface{}{
			"boardId":     *boardId,
			"projectKeys": *projectKeys,
		}, *timeAfter)
	}
	runner.RunCmd(cmd)
}
