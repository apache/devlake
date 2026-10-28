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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
	"gopkg.in/yaml.v3"
)

var CollectRepoConfigMeta = plugin.SubTaskMeta{
	Name:             "CollectRepoConfig",
	EntryPoint:       CollectRepoConfig,
	EnabledByDefault: true,
	Description:      "Fetch codecov.yml via Codecov GraphQL and parse coverage thresholds",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

const maxConfigSize = 1 << 20 // 1 MiB

// maxGraphQLResponseSize allows JSON wrapper overhead around the yaml payload.
const maxGraphQLResponseSize = maxConfigSize + (64 << 10)

const repoYamlGraphQLQuery = `query GetRepoSettings($name: String!, $repo: String!) {
  owner(username: $name) {
    repository(name: $repo) {
      ... on Repository {
        yaml
      }
    }
  }
}`

type graphqlRepoYamlResponse struct {
	Data struct {
		Owner struct {
			Repository struct {
				Yaml *string `json:"yaml"`
			} `json:"repository"`
		} `json:"owner"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// codecovYaml represents the relevant parts of a codecov.yml file
type codecovYaml struct {
	Coverage struct {
		Status struct {
			Project map[string]statusConfig `yaml:"project"`
			Patch   map[string]statusConfig `yaml:"patch"`
		} `yaml:"status"`
	} `yaml:"coverage"`
}

type statusConfig struct {
	Target        interface{} `yaml:"target"`
	Threshold     interface{} `yaml:"threshold"`
	Informational interface{} `yaml:"informational"`
}

func serviceShortCode(service string) (string, error) {
	switch service {
	case "github":
		return "gh", nil
	case "github_enterprise":
		return "ghe", nil
	case "gitlab":
		return "gl", nil
	case "gitlab_enterprise":
		return "gle", nil
	case "bitbucket":
		return "bb", nil
	case "bitbucket_server":
		return "bbs", nil
	default:
		return "", fmt.Errorf("unsupported service %q", service)
	}
}

func fetchRepoYaml(apiClient *helper.ApiAsyncClient, serviceShort, owner, repo string) (string, errors.Error) {
	path := fmt.Sprintf("/graphql/%s", serviceShort)
	body := map[string]interface{}{
		"query": repoYamlGraphQLQuery,
		"variables": map[string]string{
			"name": owner,
			"repo": repo,
		},
	}

	res, err := apiClient.Post(path, nil, body, nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.HttpStatus(res.StatusCode).New(
			fmt.Sprintf("GraphQL request failed with HTTP %d", res.StatusCode),
		)
	}

	limited := io.LimitReader(res.Body, maxGraphQLResponseSize+1)
	resBody, readErr := io.ReadAll(limited)
	if readErr != nil {
		return "", errors.Default.Wrap(readErr, "error reading GraphQL response body")
	}
	if len(resBody) == 0 {
		return "", helper.ErrEmptyResponse
	}
	if len(resBody) > maxGraphQLResponseSize {
		return "", errors.Default.New(fmt.Sprintf("GraphQL response exceeds size limit (%d bytes)", maxGraphQLResponseSize))
	}

	var result graphqlRepoYamlResponse
	if err := errors.Convert(json.Unmarshal(resBody, &result)); err != nil {
		return "", errors.HttpStatus(res.StatusCode).Wrap(err, "error decoding GraphQL response")
	}

	if len(result.Errors) > 0 {
		msgs := make([]string, len(result.Errors))
		for i, gqlErr := range result.Errors {
			msgs[i] = gqlErr.Message
		}
		return "", errors.Default.New(fmt.Sprintf("GraphQL errors: %s", strings.Join(msgs, "; ")))
	}

	if result.Data.Owner.Repository.Yaml == nil {
		return "", nil
	}

	rawYaml := *result.Data.Owner.Repository.Yaml
	if rawYaml == "" {
		return "", nil
	}

	if len(rawYaml) > maxConfigSize {
		return "", errors.Default.New(fmt.Sprintf("repo yaml exceeds size limit (%d bytes)", maxConfigSize))
	}

	return rawYaml, nil
}

func CollectRepoConfig(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()

	owner, repo, err := ParseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	if data.Repo == nil {
		logger.Warn(nil, "[Codecov] CollectRepoConfig: No repo data available for %s, skipping", data.Options.FullName)
		return nil
	}

	serviceShort, mapErr := serviceShortCode(data.Service)
	if mapErr != nil {
		logger.Warn(nil, "[Codecov] CollectRepoConfig: unsupported service %q for %s/%s, skipping", data.Service, owner, repo)
		return nil
	}

	logger.Info("[Codecov] CollectRepoConfig: Fetching codecov config for %s/%s (service=%s)", owner, repo, data.Service)

	rawYaml, fetchErr := fetchRepoYaml(data.ApiClient, serviceShort, owner, repo)
	if fetchErr != nil {
		return errors.Default.Wrap(fetchErr, fmt.Sprintf("failed to fetch repo yaml via GraphQL for %s/%s", owner, repo))
	}

	if rawYaml == "" {
		logger.Info("[Codecov] CollectRepoConfig: No codecov config found for %s/%s", owner, repo)
		return nil
	}

	logger.Info("[Codecov] CollectRepoConfig: Found repo yaml for %s/%s (%d bytes)", owner, repo, len(rawYaml))

	config := parseCodecovYaml(rawYaml)

	repoConfig := &models.CodecovRepoConfig{
		NoPKModel:            common.NoPKModel{},
		ConnectionId:         data.Options.ConnectionId,
		RepoId:               data.Options.FullName,
		ConfigSource:         "codecov-graphql",
		RawYaml:              rawYaml,
		ProjectTarget:        config.projectTarget,
		ProjectTargetAuto:    config.projectTargetAuto,
		ProjectThreshold:     config.projectThreshold,
		ProjectInformational: config.projectInformational,
		PatchTarget:          config.patchTarget,
		PatchTargetAuto:      config.patchTargetAuto,
		PatchThreshold:       config.patchThreshold,
		PatchInformational:   config.patchInformational,
	}

	if err := db.CreateOrUpdate(repoConfig); err != nil {
		return errors.Default.Wrap(err, "failed to save repo config")
	}

	logger.Info("[Codecov] CollectRepoConfig: Saved config for %s/%s (patch_target=%v, project_target=%v)",
		owner, repo, config.patchTarget, config.projectTarget)
	return nil
}

type parsedConfig struct {
	projectTarget        *float64
	projectTargetAuto    bool
	projectThreshold     *float64
	projectInformational bool
	patchTarget          *float64
	patchTargetAuto      bool
	patchThreshold       *float64
	patchInformational   bool
}

func parseCodecovYaml(raw string) parsedConfig {
	var cfg codecovYaml
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return parsedConfig{}
	}

	result := parsedConfig{}

	if def, ok := cfg.Coverage.Status.Project["default"]; ok {
		result.projectTarget, result.projectTargetAuto = parseTarget(def.Target)
		result.projectThreshold = parsePercentage(def.Threshold)
		result.projectInformational = parseBool(def.Informational)
	}

	if def, ok := cfg.Coverage.Status.Patch["default"]; ok {
		result.patchTarget, result.patchTargetAuto = parseTarget(def.Target)
		result.patchThreshold = parsePercentage(def.Threshold)
		result.patchInformational = parseBool(def.Informational)
	}

	return result
}

// parseTarget handles target values: "auto", "80%", 80, 80.0
func parseTarget(v interface{}) (target *float64, isAuto bool) {
	if v == nil {
		return nil, false
	}
	switch val := v.(type) {
	case string:
		val = strings.TrimSpace(val)
		if strings.EqualFold(val, "auto") {
			return nil, true
		}
		val = strings.TrimSuffix(val, "%")
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return &f, false
		}
	case int:
		f := float64(val)
		return &f, false
	case float64:
		return &val, false
	}
	return nil, false
}

// parsePercentage handles threshold values: "1%", 1, 1.0
func parsePercentage(v interface{}) *float64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		val = strings.TrimSpace(strings.TrimSuffix(val, "%"))
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return &f
		}
	case int:
		f := float64(val)
		return &f
	case float64:
		return &val
	}
	return nil
}

func parseBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	}
	return false
}
