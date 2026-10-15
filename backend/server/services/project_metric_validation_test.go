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

package services

import (
	"encoding/json"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerTestMetricPlugin registers a metric plugin mock. Calls without a matching expectation
// fail the test, so a mock without expectations proves the plugin is never invoked.
func registerTestMetricPlugin(t *testing.T, name string) *mockplugin.CompositeMetricPluginBlueprintV200 {
	t.Helper()
	metricPlugin := mockplugin.NewCompositeMetricPluginBlueprintV200(t)
	metricPlugin.On("Name").Return(name).Maybe()
	registerProjectDeleteTestPlugin(t, metricPlugin)
	return metricPlugin
}

func TestValidateProjectMetrics(t *testing.T) {
	t.Run("passes enabled metrics with valid options", func(t *testing.T) {
		metricPlugin := registerTestMetricPlugin(t, "test-metric-valid")
		option := json.RawMessage(`{"pattern":"^feat/"}`)
		metricPlugin.On("MakeMetricPluginPipelinePlanV200", "test-project", option).
			Return(models.PipelinePlan{}, nil).Once()

		err := validateProjectMetrics("test-project", []*models.BaseMetric{
			{PluginName: "test-metric-valid", PluginOption: option, Enable: true},
		})

		assert.NoError(t, err)
	})

	t.Run("treats empty options as an empty object", func(t *testing.T) {
		metricPlugin := registerTestMetricPlugin(t, "test-metric-empty")
		metricPlugin.On("MakeMetricPluginPipelinePlanV200", "test-project", json.RawMessage("{}")).
			Return(models.PipelinePlan{}, nil).Once()

		err := validateProjectMetrics("test-project", []*models.BaseMetric{
			{PluginName: "test-metric-empty", Enable: true},
		})

		assert.NoError(t, err)
	})

	t.Run("rejects invalid options as bad input", func(t *testing.T) {
		metricPlugin := registerTestMetricPlugin(t, "test-metric-invalid")
		pluginErr := errors.Default.New("invalid somePattern")
		metricPlugin.On("MakeMetricPluginPipelinePlanV200", "test-project", json.RawMessage(`{}`)).
			Return(nil, pluginErr).Once()

		err := validateProjectMetrics("test-project", []*models.BaseMetric{
			{PluginName: "test-metric-invalid", PluginOption: json.RawMessage(`{}`), Enable: true},
		})

		require.Error(t, err)
		assert.Equal(t, errors.BadInput, err.GetType())
		assert.ErrorIs(t, err, pluginErr)
		assert.Contains(t, err.Error(), "invalid options for metric plugin test-metric-invalid")
	})

	t.Run("skips disabled and nil metrics", func(t *testing.T) {
		registerTestMetricPlugin(t, "test-metric-disabled")

		err := validateProjectMetrics("test-project", []*models.BaseMetric{
			nil,
			{PluginName: "test-metric-disabled", PluginOption: json.RawMessage(`{"bad"`), Enable: false},
			{PluginName: "test-metric-not-registered", Enable: false},
		})

		assert.NoError(t, err)
	})

	t.Run("rejects enabled metrics whose plugin is not registered", func(t *testing.T) {
		err := validateProjectMetrics("test-project", []*models.BaseMetric{
			{PluginName: "test-metric-not-registered", Enable: true},
		})

		require.Error(t, err)
		assert.Equal(t, errors.BadInput, err.GetType())
		// GetPlugin's message depends on the global registry state, so only the wrapper is asserted
		assert.Contains(t, err.Error(), "invalid options for metric plugin test-metric-not-registered")
	})

	t.Run("rejects enabled metrics whose plugin is not a metric plugin", func(t *testing.T) {
		registerProjectDeleteTestPlugin(t, &testOrdinaryPlugin{name: "test-metric-ordinary"})

		err := validateProjectMetrics("test-project", []*models.BaseMetric{
			{PluginName: "test-metric-ordinary", Enable: true},
		})

		require.Error(t, err)
		assert.Equal(t, errors.BadInput, err.GetType())
		assert.Contains(t, err.Error(), "does not support MetricPluginBlueprintV200")
	})
}

func withProjectTestValidator(t *testing.T) {
	t.Helper()
	previousVld := vld
	vld = validator.New()
	t.Cleanup(func() {
		vld = previousVld
	})
}

func TestCreateProject_RejectsInvalidMetricOptionsBeforeWriting(t *testing.T) {
	withProjectTestValidator(t)
	// no expectations: any database call fails the test
	withProjectTestDatabase(t, mockdal.NewDal(t))
	metricPlugin := registerTestMetricPlugin(t, "test-metric-create")
	pluginErr := errors.BadInput.New("invalid somePattern")
	metricPlugin.On("MakeMetricPluginPipelinePlanV200", "new-project", json.RawMessage(`{"somePattern":"("}`)).
		Return(nil, pluginErr).Once()

	output, err := CreateProject(&models.ApiInputProject{
		BaseProject: models.BaseProject{Name: "new-project"},
		Metrics: []*models.BaseMetric{
			{PluginName: "test-metric-create", PluginOption: json.RawMessage(`{"somePattern":"("}`), Enable: true},
		},
	})

	assert.Nil(t, output)
	require.Error(t, err)
	assert.Equal(t, errors.BadInput, err.GetType())
	assert.ErrorIs(t, err, pluginErr)
}

func TestPatchProject_RejectsInvalidMetricOptionsBeforeWriting(t *testing.T) {
	// no expectations: any database call fails the test
	withProjectTestDatabase(t, mockdal.NewDal(t))
	metricPlugin := registerTestMetricPlugin(t, "test-metric-patch")
	pluginErr := errors.BadInput.New("invalid somePattern")
	metricPlugin.On("MakeMetricPluginPipelinePlanV200", "renamed-project", json.RawMessage(`{"somePattern":"("}`)).
		Return(nil, pluginErr).Once()

	output, err := PatchProject("old-project", map[string]interface{}{
		"name": "renamed-project",
		"metrics": []interface{}{
			map[string]interface{}{
				"pluginName":   "test-metric-patch",
				"pluginOption": map[string]interface{}{"somePattern": "("},
				"enable":       true,
			},
		},
	})

	assert.Nil(t, output)
	require.Error(t, err)
	assert.Equal(t, errors.BadInput, err.GetType())
	assert.ErrorIs(t, err, pluginErr)
}

func TestPatchProject_ValidatesMetricOptionsWithCurrentNameWhenNotRenamed(t *testing.T) {
	withProjectTestDatabase(t, mockdal.NewDal(t))
	metricPlugin := registerTestMetricPlugin(t, "test-metric-patch-name")
	pluginErr := errors.BadInput.New("invalid somePattern")
	metricPlugin.On("MakeMetricPluginPipelinePlanV200", "old-project", json.RawMessage(`{}`)).
		Return(nil, pluginErr).Once()

	_, err := PatchProject("old-project", map[string]interface{}{
		"metrics": []interface{}{
			map[string]interface{}{"pluginName": "test-metric-patch-name", "pluginOption": map[string]interface{}{}, "enable": true},
		},
	})

	assert.ErrorIs(t, err, pluginErr)
}
