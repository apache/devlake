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

package parser

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	giturls "github.com/chainguard-dev/git-urls"
)

var _ RepoCloner = (*GitcliCloner)(nil)
var ErrNoData = errors.NotModified.New("No data to be collected")

// CloneRepoConfig is the configuration for the CloneRepo method
// the subtask should run in Full Sync mode whenever the configuration is changed
type CloneRepoConfig struct {
	UseGoGit        *bool
	SkipCommitStat  *bool
	SkipCommitFiles *bool
	NoShallowClone  bool
}

type GitcliCloner struct {
	ctx          plugin.SubTaskContext
	taskData     *GitExtractorTaskData
	logger       log.Logger
	stateManager *api.SubtaskStateManager
	since        *time.Time
	remoteUrl    string
	localDir     string
	success      bool
	syncEnvs     []string
	syncArgs     []string
}

func NewGitcliCloner(ctx plugin.SubTaskContext, localDir string) (*GitcliCloner, errors.Error) {
	taskData := ctx.GetData().(*GitExtractorTaskData)
	stateManager := errors.Must1(api.NewSubtaskStateManager(&api.SubtaskCommonArgs{
		SubTaskContext: ctx,
		Params:         taskData.Options.GitExtractorApiParams,
		SubtaskConfig: CloneRepoConfig{
			UseGoGit:        taskData.Options.UseGoGit,
			SkipCommitStat:  taskData.Options.SkipCommitStat,
			SkipCommitFiles: taskData.Options.SkipCommitFiles,
			NoShallowClone:  taskData.Options.NoShallowClone,
		},
	}))

	cloner := &GitcliCloner{
		ctx:          ctx,
		taskData:     taskData,
		logger:       ctx.GetLogger().Nested("gitcli"),
		stateManager: stateManager,
		since:        stateManager.GetSince(),
		remoteUrl:    taskData.Options.Url,
		localDir:     localDir,
		success:      false,
	}
	return cloner, cloner.prepareSync()
}

func (g *GitcliCloner) prepareSync() errors.Error {
	taskData := g.taskData
	if *taskData.Options.SkipCommitStat {
		g.syncArgs = append(g.syncArgs, "--filter=blob:none")
	}
	remoteUrl, e := giturls.Parse(g.remoteUrl)
	if e != nil {
		return errors.Convert(e)
	}
	// support proxy
	if remoteUrl.Scheme == "http" || remoteUrl.Scheme == "https" {
		if taskData.Options.Proxy != "" {
			g.syncEnvs = append(g.syncEnvs, fmt.Sprintf("HTTPS_PROXY=%s", taskData.Options.Proxy))
		}
		if remoteUrl.Scheme == "https" && g.ctx.GetConfigReader().GetBool("IN_SECURE_SKIP_VERIFY") {
			g.syncEnvs = append(g.syncEnvs, "GIT_SSL_NO_VERIFY=true")
		}
	} else if remoteUrl.Scheme == "ssh" {
		var sshCmdArgs []string
		if taskData.Options.Proxy != "" {
			parsedProxyURL, e := url.Parse(taskData.Options.Proxy)
			if e != nil {
				return errors.BadInput.Wrap(e, "failed to parse the proxy URL")
			}
			proxyCommand := "corkscrew"
			sshCmdArgs = append(sshCmdArgs, "-o", fmt.Sprintf(`ProxyCommand="%s %s %s %%h %%p"`, proxyCommand, parsedProxyURL.Hostname(), parsedProxyURL.Port()))
		}
		// support private key
		if taskData.Options.PrivateKey != "" {
			// Create a restricted temp directory so the key file is protected from the moment of creation (CWE-367, CWE-732)
			pkDir, err := os.MkdirTemp("", "gitext-pk-")
			if err != nil {
				g.logger.Error(err, "create temp private key dir error")
				return errors.Default.New("failed to handle the private key")
			}
			defer os.RemoveAll(pkDir)
			pkFilePath := path.Join(pkDir, "pk")
			pkFile, err := os.OpenFile(pkFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				g.logger.Error(err, "create private key file error")
				return errors.Default.New("failed to handle the private key")
			}
			if _, e := pkFile.WriteString(taskData.Options.PrivateKey + "\n"); e != nil {
				g.logger.Error(e, "write private key file error")
				return errors.Default.New("failed to write the private key")
			}
			pkFile.Close()

			if taskData.Options.Passphrase != "" {
				// Pass passphrase via stdin to avoid exposure in /proc/<pid>/cmdline (CWE-214)
				pp := exec.CommandContext(
					g.ctx.GetContext(),
					"ssh-keygen", "-p",
					"-N", "",
					"-f", pkFilePath,
				)
				pp.Stdin = strings.NewReader(taskData.Options.Passphrase + "\n")
				if _, pperr := pp.CombinedOutput(); pperr != nil {
					g.logger.Error(pperr, "change private key passphrase error")
					return errors.Default.New("failed to decrypt the private key")
				}
			}
			sshCmdArgs = append(sshCmdArgs, fmt.Sprintf("-i %s -o StrictHostKeyChecking=no", pkFilePath))
		}
		if len(sshCmdArgs) > 0 {
			g.syncEnvs = append(g.syncEnvs, fmt.Sprintf("GIT_SSH_COMMAND=ssh %s", strings.Join(sshCmdArgs, " ")))
		}
	}
	return nil
}

func (g *GitcliCloner) IsIncremental() bool {
	if g != nil && g.stateManager != nil {
		if g.stateManager.GetSince() != nil {
			return true
		}
		return g.stateManager.IsIncremental()
	}

	return false
}

func (g *GitcliCloner) CloneRepo() errors.Error {
	if g.since == nil {
		// full sync
		if err := g.fullClone(); err != nil {
			return err
		}
	} else {
		if g.taskData.Options.NoShallowClone {
			// data source does not support shallow clone
			//   1. perform a full clone to accommodate
			//   2. perform a local shallow clone to reduce the libgit2 memory usage
			if err := g.doubleClone(); err != nil {
				return err
			}
		} else {
			// data source support shallow clone
			if err := g.shallowClone(); err != nil {
				return err
			}
		}
		if err := g.deepen(); err != nil {
			return err
		}
		g.success = true
	}
	return nil
}

func (g *GitcliCloner) CloseRepo() errors.Error {
	if g.success {
		g.logger.Info("save state")
		return g.stateManager.Close()
	}
	return nil
}

func (g *GitcliCloner) fullClone() errors.Error {
	return g.gitClone(g.remoteUrl, g.localDir, "--bare")
}

func (g *GitcliCloner) deepen() errors.Error {
	// deepen the commits by 1 more step to avoid https://github.com/apache/incubator-devlake/issues/7426
	// fixes error described on https://stackoverflow.com/questions/63878612/git-fatal-error-in-object-unshallow-sha-1
	// It might be caused by the commit which being deepen has multiple parent(e.g. a merge commit), not sure.
	if err := g.gitCmd("repack", "-d"); err != nil {
		return errors.Default.Wrap(err, "failed to repack the repo")
	}
	// deepen would fail on a EMPTY repo, ignore the error
	if err := g.gitFetch("--deepen=1"); err != nil {
		g.logger.Error(err, "failed to deepen the cloned repo")
	}
	return nil
}

func (g *GitcliCloner) shallowClone() errors.Error {
	// to fetch newly added commits from ALL branches, we need to the following guide:
	//    https://stackoverflow.com/questions/23708231/git-shallow-clone-clone-depth-misses-remote-branches
	// 1. clone the repo with depth 1
	if err := g.gitClone(g.remoteUrl, g.localDir, "--depth=1", "--bare"); err != nil {
		return err
	}
	// 2. configure to fetch all branches from the remote server, so we can collect new commits from them
	gitConfig, err := os.OpenFile(path.Join(g.localDir, "config"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Default.Wrap(err, "failed to open git config file")
	}
	_, err = gitConfig.WriteString("\tfetch = +refs/heads/*:refs/remotes/origin/*\n")
	if err != nil {
		return errors.Default.Wrap(err, "failed to write to git config file")
	}
	g.logger.Debug("updated git config to fetch all remote branches")
	// 3. fetch all branches with depth=1 so the next step would collect fewer commits
	// (I don't know why, but it reduced total number of commits from 18k to 7k on https://gitlab.com/gitlab-org/gitlab-foss.git with the same parameters)
	if err := g.gitFetch("--depth=1", "origin"); err != nil {
		return errors.Default.Wrap(err, "failed to fetch all branches from the remote server")
	}
	// 4. fetch all new commits from all branches since the given time
	if err := g.gitFetch(fmt.Sprintf("--shallow-since=%s", g.since.Format(time.RFC3339))); err != nil {
		g.logger.Warn(err, "shallow fetch failed")
	}
	return nil
}

func (g *GitcliCloner) doubleClone() errors.Error {
	intermediaryDir, e := os.MkdirTemp("", "gitextint")
	if e != nil {
		return errors.Convert(e)
	}
	defer os.RemoveAll(intermediaryDir) // CWE-459: ensure cleanup on all exit paths
	// step 1: full clone into a intermediary dir
	backup := g.localDir
	g.localDir = intermediaryDir
	if err := g.fullClone(); err != nil {
		return err
	}
	g.localDir = backup
	// step 2: perform shallow clone against the intermediary dir
	backup = g.remoteUrl
	g.remoteUrl = fmt.Sprintf("file://%s", intermediaryDir) // the file:// prefix is required for shallow clone to work
	if err := g.shallowClone(); err != nil {
		return err
	}
	g.remoteUrl = backup
	return nil
}

func (g *GitcliCloner) gitClone(args ...string) errors.Error {
	args = append(args, g.syncArgs...)
	return g.git(g.syncEnvs, "", "clone", args...)
}

func (g *GitcliCloner) gitFetch(args ...string) errors.Error {
	empty, err := g.repoIsEmpty(args...)
	if err != nil {
		g.logger.Error(err, "repo is empty")
		return err
	}
	if empty {
		g.logger.Info("repo is empty, doesn't need to fetch")
		return nil
	}
	args = append(args, g.syncArgs...)
	return g.git(g.syncEnvs, g.localDir, "fetch", args...)
}

func (g *GitcliCloner) repoIsEmpty(args ...string) (bool, errors.Error) {
	// try to run command: git log
	// if repo is empty, it will return an error
	err := g.git(g.syncEnvs, g.localDir, "log")
	if err != nil {
		g.logger.Warn(err, "git log failed")
		return true, nil
	}
	return false, nil
}

func (g *GitcliCloner) gitCmd(gitcmd string, args ...string) errors.Error {
	return g.git(nil, g.localDir, gitcmd, args...)
}

func (g *GitcliCloner) git(env []string, dir string, gitcmd string, args ...string) errors.Error {
	g.logger.Debug("git %s %v", gitcmd, sanitizeArgs(args)) // CWE-532: sanitize before logging
	args = append([]string{gitcmd}, args...)
	cmd := exec.CommandContext(g.ctx.GetContext(), "git", args...)
	cmd.Env = env
	cmd.Dir = dir
	return g.execCommand(cmd)
}

func (g *GitcliCloner) execCommand(cmd *exec.Cmd) errors.Error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		g.logger.Debug("err: %v, output: %s", err, string(output))
		outputString := string(output)
		if strings.Contains(outputString, "fatal: error processing shallow info: 4") ||
			strings.Contains(outputString, "fatal: the remote end hung up unexpectedly") {
			return ErrNoData
		}
		return errors.Default.New(fmt.Sprintf("git cmd %v in %s failed: %s", sanitizeArgs(cmd.Args), cmd.Dir, generateErrMsg(output, err)))
	}
	return nil
}
func generateErrMsg(output []byte, err error) string {
	errMsg := strings.TrimSpace(string(output))
	if errMsg == "" {
		errMsg = err.Error()
	}
	if errMsg == "" {
		errMsg = "unknown error"
	}
	return errMsg
}

// sensitiveQueryParams lists URL query parameter names that may carry credentials.
var sensitiveQueryParams = []string{"token", "access_token", "private_token", "api_key", "key", "apikey"}

func sanitizeArgs(args []string) []string {
	var ret []string
	for _, arg := range args {
		// Redact Authorization header values (e.g. --header=Authorization: Bearer TOKEN) (CWE-532)
		lower := strings.ToLower(arg)
		if authIdx := strings.Index(lower, "authorization:"); authIdx != -1 {
			colonPos := authIdx + len("authorization:")
			rest := strings.TrimSpace(arg[colonPos:])
			parts := strings.SplitN(rest, " ", 2)
			if len(parts) == 2 {
				arg = arg[:colonPos] + " " + parts[0] + " " + strings.Repeat("*", len(parts[1]))
			} else if len(rest) > 0 {
				arg = arg[:colonPos] + " " + strings.Repeat("*", len(rest))
			}
			ret = append(ret, arg)
			continue
		}
		u, err := url.Parse(arg)
		if err == nil && u != nil && u.User != nil {
			password, ok := u.User.Password()
			if ok {
				// Redact password in user:password@host URLs
				arg = strings.Replace(arg, password, strings.Repeat("*", len(password)), -1)
			} else {
				// Redact username-only tokens (e.g. https://TOKEN@github.com/..., GitHub App pattern)
				username := u.User.Username()
				if len(username) >= 16 {
					arg = strings.Replace(arg, username, strings.Repeat("*", len(username)), -1)
				}
			}
		}
		// Redact sensitive query parameters (e.g. ?private_token=..., ?access_token=...)
		if err == nil && u != nil && u.RawQuery != "" {
			q := u.Query()
			for _, param := range sensitiveQueryParams {
				if val := q.Get(param); val != "" {
					arg = strings.Replace(arg, val, strings.Repeat("*", len(val)), -1)
				}
			}
		}
		ret = append(ret, arg)
	}
	return ret
}
