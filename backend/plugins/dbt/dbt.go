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
	"github.com/apache/incubator-devlake/core/runner"
	"github.com/apache/incubator-devlake/plugins/dbt/impl"
	"github.com/spf13/cobra"
)

var PluginEntry impl.Dbt

// standalone mode for debugging
func main() {
	dbtCmd := &cobra.Command{Use: "dbt"}
	projectPath := dbtCmd.Flags().StringP("projectPath", "p", "/Users/abeizn/demoapp", "user dbt project directory.")
	projectGitURL := dbtCmd.Flags().StringP("projectGitURL", "g", "", "user dbt project git url.")
	projectName := dbtCmd.Flags().StringP("projectName", "n", "demoapp", "user dbt project name.")
	projectTarget := dbtCmd.Flags().StringP("projectTarget", "o", "dev", "this is the default target your dbt project will use.")
	modelsSlice := []string{"my_first_dbt_model", "my_second_dbt_model"}
	selectedModels := dbtCmd.Flags().StringSliceP("models", "m", modelsSlice, "dbt select models")
	failFast := dbtCmd.Flags().BoolP("failFast", "", false, "dbt fail fast")
	profilesPath := dbtCmd.Flags().StringP("profilesPath", "", "/Users/abeizn/.dbt", "dbt profiles path")
	profile := dbtCmd.Flags().StringP("profile", "", "default", "dbt profile")
	threads := dbtCmd.Flags().IntP("threads", "", 1, "dbt threads")
	noVersionCheck := dbtCmd.Flags().BoolP("noVersionCheck", "", false, "dbt no version check")
	excludeModels := dbtCmd.Flags().StringSliceP("excludeModels", "", []string{}, "dbt exclude models")
	selector := dbtCmd.Flags().StringP("selector", "", "", "dbt selector")
	state := dbtCmd.Flags().StringP("state", "", "", "dbt state")
	deferFlag := dbtCmd.Flags().BoolP("defer", "", false, "dbt defer")
	noDefer := dbtCmd.Flags().BoolP("noDefer", "", false, "dbt no defer")
	fullRefresh := dbtCmd.Flags().BoolP("fullRefresh", "", false, "dbt full refresh")
	dbtArgs := dbtCmd.Flags().StringSliceP("args", "a", []string{}, "dbt run args")
	projectVars := make(map[string]string)
	projectVars["event_min_id"] = "7581"
	projectVars["event_max_id"] = "7582"
	dbtCmd.Flags().StringToStringVarP(&projectVars, "projectVars", "v", projectVars, "dbt provides variables to provide data to models for compilation.")
	timeAfter := dbtCmd.Flags().StringP("timeAfter", "a", "", "collect data that are created after specified time, ie 2006-01-02T15:04:05Z")

	dbtCmd.Run = func(cmd *cobra.Command, args []string) {
		projectVarsConvert := make(map[string]interface{}, len(projectVars))
		for k, v := range projectVars {
			projectVarsConvert[k] = v
		}
		runner.DirectRun(cmd, args, PluginEntry, map[string]interface{}{
			"projectPath":    *projectPath,
			"projectName":    *projectName,
			"projectTarget":  *projectTarget,
			"selectedModels": *selectedModels,
			"projectVars":    projectVarsConvert,
			"projectGitURL":  *projectGitURL,
			"args":           dbtArgs,
			"failFast":       *failFast,
			"profilesPath":   *profilesPath,
			"profile":        *profile,
			"threads":        *threads,
			"noVersionCheck": *noVersionCheck,
			"excludeModels":  *excludeModels,
			"selector":       *selector,
			"state":          *state,
			"defer":          *deferFlag,
			"noDefer":        *noDefer,
			"fullRefresh":    *fullRefresh,
		}, *timeAfter)
	}
	runner.RunCmd(dbtCmd)
}
