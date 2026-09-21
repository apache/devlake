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

package main // must be main for plugin entry point

import (
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/spf13/cobra"
)

var PluginEntry impl.Youtrack //nolint

// standalone mode for debugging
func main() {
	cmd := &cobra.Command{Use: "youtrack"}
	connectionId := cmd.Flags().Uint64P("connection", "c", 0, "youtrack connection id")
	projectId := cmd.Flags().StringP("project", "p", "", "youtrack project id")
	timeAfter := cmd.Flags().StringP("timeAfter", "a", "", "collect data that are created after specified time, ie 2006-01-02T15:04:05Z")
	_ = cmd.MarkFlagRequired("connection")
	_ = cmd.MarkFlagRequired("project")
	cmd.Run = func(c *cobra.Command, args []string) {
		runner.DirectRun(c, args, PluginEntry, map[string]interface{}{
			"connectionId": *connectionId,
			"projectId":    *projectId,
		}, *timeAfter)
	}
	runner.RunCmd(cmd)
}
