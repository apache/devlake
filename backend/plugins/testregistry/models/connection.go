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

package models

import (
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

const (
	CIToolOpenshiftCI = "Openshift CI"
	CIToolTektonCI    = "Tekton CI"
)

type TestRegistryConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	CITool                string `mapstructure:"ciTool" json:"ciTool" validate:"required" gorm:"column:ci_tool;type:varchar(50)"` // CI tool type: Openshift CI or Tekton CI
	Project               string `mapstructure:"project" json:"project" validate:"required" gorm:"column:project;type:varchar(200)"`

	// Openshift CI fields
	GitHubOrganization string `mapstructure:"githubOrganization" json:"githubOrganization" gorm:"column:github_organization;type:varchar(200)"` // GitHub organization (required when CI tool is Openshift CI)
	GitHubToken        string `mapstructure:"githubToken" json:"githubToken" gorm:"column:github_token;serializer:encdec"`                      // GitHub token (required when CI tool is Openshift CI, encrypted)

	// Tekton CI fields
	QuayOrganization string `mapstructure:"quayOrganization" json:"quayOrganization" gorm:"column:quay_organization;type:varchar(200)"` // Quay.io organization (required when CI tool is Tekton CI)

	// JUnit XML file matching configuration
	// Regex pattern to match JUnit XML file names in artifacts
	// Default: "(devlake-|e2e|qd-report-)[0-9a-z-]+\\.(xml|junit)" - matches files starting with "devlake-", "e2e", or "qd-report-"
	JUnitRegex string `mapstructure:"junitRegex" json:"junitRegex" gorm:"column:junit_regex;type:varchar(500)"` // Regex pattern for JUnit file matching (optional, uses default if empty)
}

func (TestRegistryConnection) TableName() string {
	return "_tool_testregistry_connections"
}

func (c TestRegistryConnection) Sanitize() TestRegistryConnection {
	if c.GitHubToken != "" {
		c.GitHubToken = utils.SanitizeString(c.GitHubToken)
	}
	return c
}

func (connection *TestRegistryConnection) MergeFromRequest(target *TestRegistryConnection, body map[string]interface{}) error {
	// Preserve existing GitHub token if it wasn't changed (user sent sanitized version)
	existingToken := target.GitHubToken
	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}
	modifiedToken := target.GitHubToken

	// If token is empty or matches the sanitized version, restore the original
	if modifiedToken == "" || modifiedToken == utils.SanitizeString(existingToken) {
		target.GitHubToken = existingToken
	}

	return nil
}
