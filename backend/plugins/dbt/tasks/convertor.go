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
	"bufio"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/spf13/viper"
)

func DbtConverter(taskCtx plugin.SubTaskContext) (err errors.Error) {
	logger := taskCtx.GetLogger()
	taskCtx.SetProgress(0, -1)
	data := taskCtx.GetData().(*DbtTaskData)
	models := data.Options.SelectedModels
	projectPath := data.Options.ProjectPath
	projectName := data.Options.ProjectName
	projectTarget := data.Options.ProjectTarget
	projectVars := data.Options.ProjectVars
	args := data.Options.Args
	failFast := data.Options.FailFast
	threads := data.Options.Threads
	noVersionCheck := data.Options.NoVersionCheck
	excludeModels := data.Options.ExcludeModels
	selector := data.Options.Selector
	state := data.Options.State
	deferFlag := data.Options.Defer
	noDefer := data.Options.NoDefer
	fullRefresh := data.Options.FullRefresh
	profilesPath := data.Options.ProfilesPath
	profile := data.Options.Profile

	defaultProfilesPath := filepath.Join(projectPath, "profiles.yml")
	_, err = errors.Convert01(os.Stat(defaultProfilesPath))
	// if profiles.yml not exist, create it manually
	if err != nil {
		dbUrl := taskCtx.GetConfig("DB_URL")
		u, err := errors.Convert01(url.Parse(dbUrl))
		if err != nil {
			return err
		}
		dbType := u.Scheme
		dbUsername := u.User.Username()
		dbPassword, _ := u.User.Password()
		dbServer, dbPort, _ := net.SplitHostPort(u.Host)
		dbDataBase := u.Path[1:]
		var dbSchema string
		flag := strings.Compare(dbType, "mysql")
		if flag == 0 {
			// mysql database
			dbSchema = dbDataBase
		} else {
			// other database
			mapQuery, err := errors.Convert01(url.ParseQuery(u.RawQuery))
			if err != nil {
				return err
			}
			if value, ok := mapQuery["search_path"]; ok {
				if len(value) < 1 {
					return errors.Default.New("DB_URL search_path parses error")
				}
				dbSchema = value[0]
			} else {
				dbSchema = "public"
			}
		}
		config := viper.New()
		config.Set(projectName+".target", projectTarget)
		config.Set(projectName+".outputs."+projectTarget+".type", dbType)
		dbPortInt, _ := strconv.Atoi(dbPort)
		config.Set(projectName+".outputs."+projectTarget+".port", dbPortInt)
		config.Set(projectName+".outputs."+projectTarget+".password", dbPassword)
		config.Set(projectName+".outputs."+projectTarget+".schema", dbSchema)
		if flag == 0 {
			config.Set(projectName+".outputs."+projectTarget+".server", dbServer)
			config.Set(projectName+".outputs."+projectTarget+".username", dbUsername)
			config.Set(projectName+".outputs."+projectTarget+".database", dbDataBase)
		} else {
			config.Set(projectName+".outputs."+projectTarget+".host", dbServer)
			config.Set(projectName+".outputs."+projectTarget+".user", dbUsername)
			config.Set(projectName+".outputs."+projectTarget+".dbname", dbDataBase)
		}
		err = errors.Convert(config.WriteConfigAs(defaultProfilesPath))
		if err != nil {
			return err
		}
	}
	// if package.yml exist, install dbt dependencies
	defaultPackagesPath := filepath.Join(projectPath, "packages.yml")
	_, err = errors.Convert01(os.Stat(defaultPackagesPath))
	if err == nil {
		cmd := exec.Command("dbt", "deps")
		err = errors.Convert(cmd.Start())
		if err != nil {
			return err
		}
	}

	dbtExecParams := []string{"dbt", "run", "--project-dir", projectPath}
	if projectVars != nil {
		jsonProjectVars, err := json.Marshal(projectVars)
		if err != nil {
			return errors.Default.New("parameters vars json marshal error")
		}
		dbtExecParams = append(dbtExecParams, "--vars")
		dbtExecParams = append(dbtExecParams, string(jsonProjectVars))
	}
	if len(models) > 0 {
		dbtExecParams = append(dbtExecParams, "--select")
		dbtExecParams = append(dbtExecParams, models...)
	}
	if args != nil {
		dbtExecParams = append(dbtExecParams, args...)
	}
	if failFast {
		dbtExecParams = append(dbtExecParams, "--fail-fast")
	}
	if threads != 0 {
		dbtExecParams = append(dbtExecParams, "--threads")
		dbtExecParams = append(dbtExecParams, strconv.Itoa(threads))
	}
	if noVersionCheck {
		dbtExecParams = append(dbtExecParams, "--no-version-check")
	}
	if excludeModels != nil {
		dbtExecParams = append(dbtExecParams, "--exclude")
		dbtExecParams = append(dbtExecParams, excludeModels...)
	}
	if selector != "" {
		dbtExecParams = append(dbtExecParams, "--selector")
		dbtExecParams = append(dbtExecParams, selector)
	}
	if state != "" {
		dbtExecParams = append(dbtExecParams, "--state")
		dbtExecParams = append(dbtExecParams, state)
	}
	if deferFlag {
		dbtExecParams = append(dbtExecParams, "--defer")
	}
	if noDefer {
		dbtExecParams = append(dbtExecParams, "--no-defer")
	}
	if fullRefresh {
		dbtExecParams = append(dbtExecParams, "--full-refresh")
	}
	if profilesPath != "" {
		dbtExecParams = append(dbtExecParams, "--profiles-dir")
		dbtExecParams = append(dbtExecParams, profilesPath)
	} else {
		// default projectPath
		dbtExecParams = append(dbtExecParams, "--profiles-dir")
		dbtExecParams = append(dbtExecParams, projectPath)
	}
	if profile != "" {
		dbtExecParams = append(dbtExecParams, "--profile")
		dbtExecParams = append(dbtExecParams, profile)
	}
	cmd := exec.Command(dbtExecParams[0], dbtExecParams[1:]...)
	logger.Info("dbt run script: ", cmd)

	stdout, stdoutErr := cmd.StdoutPipe()
	if stdoutErr != nil {
		return errors.Convert(stdoutErr)
	}

	if err = errors.Convert(cmd.Start()); err != nil {
		return err
	}

	// prevent zombie process
	var errStr string
	defer func() {
		err = errors.Convert(cmd.Wait())
		if err != nil {
			logger.Error(err, "The DBT project run failed!")
			err = errors.SubtaskErr.New(errStr)
		} else {
			logger.Info("The DBT project run ended.")
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Info(line)
		if strings.Contains(line, "Encountered an error") || errStr != "" {
			errStr += line + "\n"
		}
		if strings.Contains(line, "of") && strings.Contains(line, "OK") {
			taskCtx.IncProgress(1)
		}
	}
	if err = errors.Convert(scanner.Err()); err != nil {
		logger.Error(err, "dbt read stdout failed.")
		return err
	}

	// close stdout
	if closeErr := stdout.Close(); closeErr != nil && err == nil {
		logger.Error(closeErr, "dbt close stdout failed.")
		return errors.Convert(closeErr)
	}

	return err
}

var DbtConverterMeta = plugin.SubTaskMeta{
	Name:             "DbtConverter",
	EntryPoint:       DbtConverter,
	EnabledByDefault: true,
	Description:      "Convert data by dbt",
	Dependencies:     []*plugin.SubTaskMeta{&GitMeta},
}
