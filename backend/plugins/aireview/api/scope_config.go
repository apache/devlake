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

package api

import (
	"net/http"
	"strconv"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/aireview/models"
)

// GetScopeConfigs returns a list of scope configurations
// @Summary Get scope configurations
// @Description Get a list of AI Review scope configurations
// @Tags plugins/aireview
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(50)
// @Success 200 {object} map[string]any
// @Router /plugins/aireview/scope-configs [get]
func GetScopeConfigs(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// Parse pagination
	page, _ := strconv.Atoi(input.Query.Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(input.Query.Get("pageSize"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// Get total count
	total, err := db.Count(dal.From(&models.AiReviewScopeConfig{}))
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to count scope configs")
	}

	// Get paginated results
	var configs []models.AiReviewScopeConfig
	err = db.All(&configs,
		dal.From(&models.AiReviewScopeConfig{}),
		dal.Orderby("id DESC"),
		dal.Limit(pageSize),
		dal.Offset(offset),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query scope configs")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"scopeConfigs": configs,
			"page":         page,
			"pageSize":     pageSize,
			"total":        total,
		},
		Status: http.StatusOK,
	}, nil
}

// GetScopeConfig returns a single scope configuration by ID
// @Summary Get scope configuration by ID
// @Description Get a single AI Review scope configuration
// @Tags plugins/aireview
// @Param id path int true "Scope Config ID"
// @Success 200 {object} models.AiReviewScopeConfig
// @Router /plugins/aireview/scope-configs/{id} [get]
func GetScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	configId, err := strconv.ParseUint(input.Params["id"], 10, 64)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid scope config id")
	}

	var config models.AiReviewScopeConfig
	dbErr := db.First(&config, dal.Where("id = ?", configId))
	if dbErr != nil {
		if db.IsErrorNotFound(dbErr) {
			return nil, errors.NotFound.Wrap(dbErr, "scope config not found")
		}
		return nil, errors.Default.Wrap(dbErr, "failed to get scope config")
	}

	return &plugin.ApiResourceOutput{
		Body:   config,
		Status: http.StatusOK,
	}, nil
}

// CreateScopeConfig creates a new scope configuration
// @Summary Create scope configuration
// @Description Create a new AI Review scope configuration
// @Tags plugins/aireview
// @Accept json
// @Param body body models.AiReviewScopeConfig true "Scope configuration"
// @Success 201 {object} models.AiReviewScopeConfig
// @Router /plugins/aireview/scope-configs [post]
func CreateScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	config := models.GetDefaultScopeConfig()

	// Decode request body into config (overwriting defaults where provided)
	err := api.DecodeMapStruct(input.Body, config, true)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode scope config")
	}

	// Upsert by name: if a scope config with the same name already exists, update it.
	// This handles the common case where name="" and the unique index would otherwise reject the insert.
	var existing models.AiReviewScopeConfig
	dbErr := db.First(&existing, dal.Where("name = ?", config.Name))
	if dbErr == nil {
		config.ID = existing.ID
		dbErr = db.Update(config)
		if dbErr != nil {
			return nil, errors.Default.Wrap(dbErr, "failed to update scope config")
		}
		return &plugin.ApiResourceOutput{
			Body:   config,
			Status: http.StatusOK,
		}, nil
	}

	// Only fall through to Create when the lookup returned "not found".
	// Any other error (connection failure, etc.) must be surfaced immediately.
	if !db.IsErrorNotFound(dbErr) {
		return nil, errors.Default.Wrap(dbErr, "failed to look up scope config by name")
	}

	// No existing record — create new
	dbErr = db.Create(config)
	if dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to create scope config")
	}

	return &plugin.ApiResourceOutput{
		Body:   config,
		Status: http.StatusCreated,
	}, nil
}

// UpdateScopeConfig updates an existing scope configuration
// @Summary Update scope configuration
// @Description Update an existing AI Review scope configuration
// @Tags plugins/aireview
// @Accept json
// @Param id path int true "Scope Config ID"
// @Param body body models.AiReviewScopeConfig true "Scope configuration"
// @Success 200 {object} models.AiReviewScopeConfig
// @Router /plugins/aireview/scope-configs/{id} [patch]
func UpdateScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	configId, err := strconv.ParseUint(input.Params["id"], 10, 64)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid scope config id")
	}

	// Get existing config
	var config models.AiReviewScopeConfig
	dbErr := db.First(&config, dal.Where("id = ?", configId))
	if dbErr != nil {
		if db.IsErrorNotFound(dbErr) {
			return nil, errors.NotFound.Wrap(dbErr, "scope config not found")
		}
		return nil, errors.Default.Wrap(dbErr, "failed to get scope config")
	}

	// Decode request body into config (updating only provided fields)
	err = api.DecodeMapStruct(input.Body, &config, true)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode scope config")
	}

	// Ensure ID is preserved
	config.ID = configId

	// Update in database
	dbErr = db.Update(&config)
	if dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to update scope config")
	}

	return &plugin.ApiResourceOutput{
		Body:   config,
		Status: http.StatusOK,
	}, nil
}

// DeleteScopeConfig deletes a scope configuration
// @Summary Delete scope configuration
// @Description Delete an AI Review scope configuration
// @Tags plugins/aireview
// @Param id path int true "Scope Config ID"
// @Success 204
// @Router /plugins/aireview/scope-configs/{id} [delete]
func DeleteScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	configId, err := strconv.ParseUint(input.Params["id"], 10, 64)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid scope config id")
	}

	// Check if config exists
	var config models.AiReviewScopeConfig
	dbErr := db.First(&config, dal.Where("id = ?", configId))
	if dbErr != nil {
		if db.IsErrorNotFound(dbErr) {
			return nil, errors.NotFound.Wrap(dbErr, "scope config not found")
		}
		return nil, errors.Default.Wrap(dbErr, "failed to get scope config")
	}

	// Delete from database
	dbErr = db.Delete(&config)
	if dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to delete scope config")
	}

	return &plugin.ApiResourceOutput{
		Status: http.StatusNoContent,
	}, nil
}

// GetDefaultScopeConfig returns the default scope configuration
// @Summary Get default scope configuration
// @Description Get the default AI Review scope configuration with all default values
// @Tags plugins/aireview
// @Success 200 {object} models.AiReviewScopeConfig
// @Router /plugins/aireview/scope-configs/default [get]
func GetDefaultScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return &plugin.ApiResourceOutput{
		Body:   models.GetDefaultScopeConfig(),
		Status: http.StatusOK,
	}, nil
}
