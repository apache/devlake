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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
)

const (
	DbtProjectBaseDirConfigKey = "DBT_PROJECTS_DIR"
	dbtProjectBaseDirName      = "devlake-dbt-projects"
)

func PrepareOptions(op *DbtOptions, configuredBaseDir string) errors.Error {
	if op == nil {
		return errors.Default.New("dbt options are required")
	}

	baseDir, err := normalizeBaseDir(configuredBaseDir)
	if err != nil {
		return err
	}
	op.ProjectBaseDir = baseDir
	op.ProjectPath = strings.TrimSpace(op.ProjectPath)
	op.ProjectGitURL = strings.TrimSpace(op.ProjectGitURL)

	if op.ProjectGitURL != "" {
		if err := validateProjectGitURL(op.ProjectGitURL); err != nil {
			return err
		}
		op.ProjectPath = ""
		op.ManagedProjectDir = true
		return nil
	}

	if op.ProjectPath == "" {
		return errors.Default.New("projectPath is required for local dbt projects")
	}

	projectPath, err := normalizePathWithinBase(baseDir, op.ProjectPath)
	if err != nil {
		return err
	}
	op.ProjectPath = projectPath
	return nil
}

func normalizeBaseDir(configuredBaseDir string) (string, errors.Error) {
	baseDir := strings.TrimSpace(configuredBaseDir)
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), dbtProjectBaseDirName)
	}
	baseDir, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", errors.Convert(err)
	}
	return baseDir, nil
}

func normalizePathWithinBase(baseDir string, candidate string) (string, errors.Error) {
	normalizedPath, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", errors.Convert(err)
	}
	rel, err := filepath.Rel(baseDir, normalizedPath)
	if err != nil {
		return "", errors.Convert(err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.Default.New("projectPath must stay within " + baseDir)
	}
	return normalizedPath, nil
}

func validateProjectGitURL(rawURL string) errors.Error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.Convert(err)
	}
	if u.Scheme != "https" && u.Scheme != "ssh" {
		return errors.Default.New("projectGitURL must use https:// or ssh://")
	}
	if u.Host == "" {
		return errors.Default.New("projectGitURL must include a hostname")
	}
	if u.Path == "" || u.Path == "/" {
		return errors.Default.New("projectGitURL must include a repository path")
	}
	return nil
}

func ensureProjectBaseDir(baseDir string) (string, errors.Error) {
	baseDir, err := normalizeBaseDir(baseDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", errors.Convert(err)
	}
	return baseDir, nil
}

func CleanupManagedProjectDir(op *DbtOptions) errors.Error {
	if op == nil || !op.ManagedProjectDir || op.ProjectPath == "" {
		return nil
	}
	projectPath, err := normalizePathWithinBase(op.ProjectBaseDir, op.ProjectPath)
	if err != nil {
		return err
	}
	return errors.Convert(os.RemoveAll(projectPath))
}
