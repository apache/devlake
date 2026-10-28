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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/apache/devlake/core/errors"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

// --- serviceShortCode ---

func TestServiceShortCode_AllServices(t *testing.T) {
	cases := map[string]string{
		"github":             "gh",
		"github_enterprise":  "ghe",
		"gitlab":             "gl",
		"gitlab_enterprise":  "gle",
		"bitbucket":          "bb",
		"bitbucket_server":   "bbs",
	}
	for service, want := range cases {
		got, err := serviceShortCode(service)
		require.NoError(t, err)
		assert.Equal(t, want, got, "service %q", service)
	}
}

func TestServiceShortCode_Unknown(t *testing.T) {
	_, err := serviceShortCode("unknown")
	assert.Error(t, err)
}

// --- fetchRepoYaml ---

func newTestGraphQLApiClient(t *testing.T, handler http.HandlerFunc) *helper.ApiAsyncClient {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	apiClient := &helper.ApiClient{}
	apiClient.Setup(ts.URL, nil, 10*time.Second)
	return &helper.ApiAsyncClient{ApiClient: apiClient}
}

func TestFetchRepoYaml_Success(t *testing.T) {
	yamlContent := "coverage:\n  status:\n    patch:\n      default:\n        target: 80%\n"
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/graphql/gh", r.URL.Path)

		var reqBody map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
		assert.Contains(t, reqBody["query"], "GetRepoSettings")

		vars, ok := reqBody["variables"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "konflux-ci", vars["name"])
		assert.Equal(t, "build-service", vars["repo"])

		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"owner": map[string]interface{}{
					"repository": map[string]interface{}{
						"yaml": yamlContent,
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "konflux-ci", "build-service")
	assert.NoError(t, err)
	assert.Contains(t, got, "target: 80%")
}

func TestFetchRepoYaml_NullYaml(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"owner":{"repository":{"yaml":null}}}}`))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestFetchRepoYaml_HTTP401(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Unauthorized"}`))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
}

func TestFetchRepoYaml_HTTP404(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found"}`))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
}

func TestFetchRepoYaml_ResponseTooLarge(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, maxGraphQLResponseSize+1))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "exceeds size limit")
}

func TestFetchRepoYaml_GraphQLErrors(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Repository not found"}]}`))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "Repository not found")
}

func TestFetchRepoYaml_EmptyResponseBody(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
}

func TestFetchRepoYaml_InvalidJSON(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "error decoding GraphQL response")
}

func TestFetchRepoYaml_EmptyYamlString(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"owner":{"repository":{"yaml":""}}}}`))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestFetchRepoYaml_YamlExceedsSizeLimit(t *testing.T) {
	oversizedYaml := strings.Repeat("x", maxConfigSize+1)
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"owner": map[string]interface{}{
					"repository": map[string]interface{}{
						"yaml": oversizedYaml,
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	got, err := fetchRepoYaml(apiClient, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "repo yaml exceeds size limit")
}

func TestFetchRepoYaml_PostError(t *testing.T) {
	apiClient := &helper.ApiClient{}
	apiClient.Setup("http://127.0.0.1:1", nil, 50*time.Millisecond)
	client := &helper.ApiAsyncClient{ApiClient: apiClient}

	got, err := fetchRepoYaml(client, "gh", "owner", "repo")
	assert.Error(t, err)
	assert.Empty(t, got)
}

// --- CollectRepoConfig (integration) ---

func TestCollectRepoConfig_SavesParsedConfig(t *testing.T) {
	yamlContent := "coverage:\n  status:\n    patch:\n      default:\n        target: 80%\n"
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"owner": map[string]interface{}{
					"repository": map[string]interface{}{
						"yaml": yamlContent,
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetData").Return(&CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "konflux-ci/build-service",
		},
		ApiClient: apiClient,
		Service:   "github",
		Repo:      &models.CodecovRepo{Branch: "main"},
	})
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

	mockDal.On("CreateOrUpdate", mock.MatchedBy(func(entity interface{}) bool {
		cfg, ok := entity.(*models.CodecovRepoConfig)
		if !ok {
			return false
		}
		return cfg.ConnectionId == 1 &&
			cfg.RepoId == "konflux-ci/build-service" &&
			cfg.ConfigSource == "codecov-graphql" &&
			cfg.PatchTarget != nil && *cfg.PatchTarget == 80.0
	}), mock.Anything).Return(nil).Once()

	err := CollectRepoConfig(mockCtx)
	assert.NoError(t, err)
	mockDal.AssertExpectations(t)
}

func TestCollectRepoConfig_SkipsWhenRepoMissing(t *testing.T) {
	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetData").Return(&CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "konflux-ci/build-service",
		},
		Service: "github",
		Repo:    nil,
	})
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Once()

	err := CollectRepoConfig(mockCtx)
	assert.NoError(t, err)
	mockDal.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}

func TestCollectRepoConfig_ReturnsFetchError(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Unauthorized"}`))
	})

	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetData").Return(&CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "konflux-ci/build-service",
		},
		Service:   "github",
		ApiClient: apiClient,
		Repo:      &models.CodecovRepo{},
	})
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

	err := CollectRepoConfig(mockCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch repo yaml via GraphQL")
	mockDal.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}

func TestCollectRepoConfig_InvalidFullName(t *testing.T) {
	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetData").Return(&CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "invalid-full-name-without-slash",
		},
		Service: "github",
		Repo:    &models.CodecovRepo{},
	})
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)

	err := CollectRepoConfig(mockCtx)
	assert.Error(t, err)
	mockDal.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}

func TestCollectRepoConfig_SkipsUnsupportedService(t *testing.T) {
	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetData").Return(&CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "konflux-ci/build-service",
		},
		Service: "unknown-provider",
		Repo:    &models.CodecovRepo{},
	})
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

	err := CollectRepoConfig(mockCtx)
	assert.NoError(t, err)
	mockDal.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}

func TestCollectRepoConfig_NoConfigFound(t *testing.T) {
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"owner":{"repository":{"yaml":null}}}}`))
	})

	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetData").Return(&CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "konflux-ci/build-service",
		},
		Service:   "github",
		ApiClient: apiClient,
		Repo:      &models.CodecovRepo{},
	})
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

	err := CollectRepoConfig(mockCtx)
	assert.NoError(t, err)
	mockDal.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}

func TestCollectRepoConfig_CreateOrUpdateError(t *testing.T) {
	yamlContent := "coverage:\n  status:\n    patch:\n      default:\n        target: 80%\n"
	apiClient := newTestGraphQLApiClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"owner": map[string]interface{}{
					"repository": map[string]interface{}{
						"yaml": yamlContent,
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetData").Return(&CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "konflux-ci/build-service",
		},
		Service:   "github",
		ApiClient: apiClient,
		Repo:      &models.CodecovRepo{},
	})
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
		Return(errors.Default.New("db unavailable")).Once()

	err := CollectRepoConfig(mockCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save repo config")
}

// --- parseCodecovYaml ---

func TestParseCodecovYaml_FullConfig(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      default:
        target: 80%
        threshold: 1%
        informational: false
    patch:
      default:
        target: 90%
        threshold: 5%
        informational: true
`
	cfg := parseCodecovYaml(raw)

	assert.NotNil(t, cfg.projectTarget)
	assert.Equal(t, 80.0, *cfg.projectTarget)
	assert.False(t, cfg.projectTargetAuto)
	assert.NotNil(t, cfg.projectThreshold)
	assert.Equal(t, 1.0, *cfg.projectThreshold)
	assert.False(t, cfg.projectInformational)

	assert.NotNil(t, cfg.patchTarget)
	assert.Equal(t, 90.0, *cfg.patchTarget)
	assert.False(t, cfg.patchTargetAuto)
	assert.NotNil(t, cfg.patchThreshold)
	assert.Equal(t, 5.0, *cfg.patchThreshold)
	assert.True(t, cfg.patchInformational)
}

func TestParseCodecovYaml_AutoTarget(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      default:
        target: auto
    patch:
      default:
        target: auto
`
	cfg := parseCodecovYaml(raw)

	assert.Nil(t, cfg.projectTarget)
	assert.True(t, cfg.projectTargetAuto)
	assert.Nil(t, cfg.patchTarget)
	assert.True(t, cfg.patchTargetAuto)
}

func TestParseCodecovYaml_NumericTargets(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      default:
        target: 75
        threshold: 2
    patch:
      default:
        target: 85.5
        threshold: 0.5
`
	cfg := parseCodecovYaml(raw)

	assert.NotNil(t, cfg.projectTarget)
	assert.Equal(t, 75.0, *cfg.projectTarget)
	assert.NotNil(t, cfg.projectThreshold)
	assert.Equal(t, 2.0, *cfg.projectThreshold)

	assert.NotNil(t, cfg.patchTarget)
	assert.Equal(t, 85.5, *cfg.patchTarget)
	assert.NotNil(t, cfg.patchThreshold)
	assert.Equal(t, 0.5, *cfg.patchThreshold)
}

func TestParseCodecovYaml_Empty(t *testing.T) {
	cfg := parseCodecovYaml("")
	assert.Nil(t, cfg.projectTarget)
	assert.Nil(t, cfg.patchTarget)
	assert.False(t, cfg.projectTargetAuto)
	assert.False(t, cfg.patchTargetAuto)
}

func TestParseCodecovYaml_InvalidYaml(t *testing.T) {
	cfg := parseCodecovYaml("{{{{not yaml")
	assert.Nil(t, cfg.projectTarget)
	assert.Nil(t, cfg.patchTarget)
}

func TestParseCodecovYaml_NoDefault(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      custom-status:
        target: 50%
    patch:
      custom-status:
        target: 60%
`
	cfg := parseCodecovYaml(raw)
	assert.Nil(t, cfg.projectTarget)
	assert.Nil(t, cfg.patchTarget)
}

func TestParseCodecovYaml_OnlyPatch(t *testing.T) {
	raw := `
coverage:
  status:
    patch:
      default:
        target: 70%
        informational: true
`
	cfg := parseCodecovYaml(raw)
	assert.Nil(t, cfg.projectTarget)
	assert.NotNil(t, cfg.patchTarget)
	assert.Equal(t, 70.0, *cfg.patchTarget)
	assert.True(t, cfg.patchInformational)
	assert.False(t, cfg.projectInformational)
}

// --- parseTarget ---

func TestParseTarget_Nil(t *testing.T) {
	target, auto := parseTarget(nil)
	assert.Nil(t, target)
	assert.False(t, auto)
}

func TestParseTarget_Auto(t *testing.T) {
	target, auto := parseTarget("auto")
	assert.Nil(t, target)
	assert.True(t, auto)
}

func TestParseTarget_AutoUpperCase(t *testing.T) {
	target, auto := parseTarget("AUTO")
	assert.Nil(t, target)
	assert.True(t, auto)
}

func TestParseTarget_StringPercent(t *testing.T) {
	target, auto := parseTarget("80%")
	assert.NotNil(t, target)
	assert.Equal(t, 80.0, *target)
	assert.False(t, auto)
}

func TestParseTarget_StringNumber(t *testing.T) {
	target, auto := parseTarget("75.5")
	assert.NotNil(t, target)
	assert.Equal(t, 75.5, *target)
	assert.False(t, auto)
}

func TestParseTarget_Int(t *testing.T) {
	target, auto := parseTarget(80)
	assert.NotNil(t, target)
	assert.Equal(t, 80.0, *target)
	assert.False(t, auto)
}

func TestParseTarget_Float(t *testing.T) {
	target, auto := parseTarget(85.5)
	assert.NotNil(t, target)
	assert.Equal(t, 85.5, *target)
	assert.False(t, auto)
}

func TestParseTarget_InvalidString(t *testing.T) {
	target, auto := parseTarget("not-a-number")
	assert.Nil(t, target)
	assert.False(t, auto)
}

func TestParseTarget_StringWithSpaces(t *testing.T) {
	target, auto := parseTarget("  90%  ")
	assert.NotNil(t, target)
	assert.Equal(t, 90.0, *target)
	assert.False(t, auto)
}

// --- parsePercentage ---

func TestParsePercentage_Nil(t *testing.T) {
	assert.Nil(t, parsePercentage(nil))
}

func TestParsePercentage_StringPercent(t *testing.T) {
	result := parsePercentage("1%")
	assert.NotNil(t, result)
	assert.Equal(t, 1.0, *result)
}

func TestParsePercentage_StringNumber(t *testing.T) {
	result := parsePercentage("5.5")
	assert.NotNil(t, result)
	assert.Equal(t, 5.5, *result)
}

func TestParsePercentage_Int(t *testing.T) {
	result := parsePercentage(3)
	assert.NotNil(t, result)
	assert.Equal(t, 3.0, *result)
}

func TestParsePercentage_Float(t *testing.T) {
	result := parsePercentage(2.5)
	assert.NotNil(t, result)
	assert.Equal(t, 2.5, *result)
}

func TestParsePercentage_InvalidString(t *testing.T) {
	assert.Nil(t, parsePercentage("abc"))
}

// --- parseBool ---

func TestParseBool_Nil(t *testing.T) {
	assert.False(t, parseBool(nil))
}

func TestParseBool_True(t *testing.T) {
	assert.True(t, parseBool(true))
}

func TestParseBool_False(t *testing.T) {
	assert.False(t, parseBool(false))
}

func TestParseBool_StringTrue(t *testing.T) {
	assert.True(t, parseBool("true"))
	assert.True(t, parseBool("TRUE"))
	assert.True(t, parseBool("True"))
}

func TestParseBool_StringFalse(t *testing.T) {
	assert.False(t, parseBool("false"))
	assert.False(t, parseBool("something"))
}

func TestParseBool_OtherType(t *testing.T) {
	assert.False(t, parseBool(42))
}
