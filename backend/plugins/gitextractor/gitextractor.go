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
	"github.com/apache/devlake/plugins/gitextractor/impl"
	"github.com/spf13/cobra"
)

var PluginEntry impl.GitExtractor //nolint

// standalone mode for debugging
func main() {
	cmd := &cobra.Command{Use: "gitextractor"}
	url := cmd.Flags().StringP("url", "l", "", "repo url")
	repoId := cmd.Flags().StringP("repoId", "i", "", "domain layer repo id")
	user := cmd.Flags().StringP("user", "u", "", "username")
	password := cmd.Flags().StringP("password", "p", "", "password")
	// pk := cmd.Flags().StringP("privateKey", "k", "", "private key file")
	// pkPass := cmd.Flags().StringP("privateKeyPassPhrase", "P", "", "passphrase for private key")
	proxy := cmd.Flags().StringP("proxy", "x", "", "proxy")
	useGoGit := cmd.Flags().BoolP("useGoGit", "g", false, "use go-git instead of libgit2")
	useGitCli := cmd.Flags().BoolP("useGitCli", "", false, "use the git command line collector")
	skipCommitStat := cmd.Flags().BoolP("skipCommitStat", "S", false, "")
	skipCommitFiles := cmd.Flags().BoolP("skipCommitFiles", "F", true, "")
	noShallowClone := cmd.Flags().BoolP("noShallowClone", "A", false, "")
	timeAfter := cmd.Flags().StringP("timeAfter", "a", "", "collect data that are created after specified time, ie 2006-01-02T15:04:05Z")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("repoId")

	cmd.Run = func(c *cobra.Command, args []string) {
		runner.DirectRun(c, args, PluginEntry, map[string]interface{}{
			"url":      *url,
			"repoId":   *repoId,
			"user":     *user,
			"password": *password,
			// "privateKey": *
			// "passphrase"
			"proxy":           *proxy,
			"useGoGit":        *useGoGit,
			"useGitCli":       *useGitCli,
			"skipCommitStat":  skipCommitStat,
			"skipCommitFiles": skipCommitFiles,
			"noShallowClone":  noShallowClone,
		}, *timeAfter)
	}
	runner.RunCmd(cmd)
}
