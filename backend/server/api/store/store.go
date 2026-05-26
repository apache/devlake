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

package store

import (
	"fmt"
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/server/api/shared"
	"github.com/apache/incubator-devlake/server/services"

	"github.com/gin-gonic/gin"
)

// @Summary Get the value by given key
// @Description Get the value by given key
// @Tags framework/projects
// @Param storeKey path string true "storeKey"
// @Success 200  {object} interface{}
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /store/{storeKey} [get]
func GetStore(c *gin.Context) {
	storeKey := c.Param("storeKey")
	result, err := services.GetStore(storeKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching %s data", storeKey)})
		return
	}

	shared.ApiOutputSuccess(c, result.StoreValue, http.StatusOK)
}

// @Summary Put the value by given key
// @Description Put the value by given key
// @Tags framework/projects
// @Accept application/json
// @Param storeKey path string true "storeKey"
// @Param project body interface{} false "json"
// @Success 200  {object} interface{}
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /store/{storeKey} [PUT]
func PutStore(c *gin.Context) {
	storeKey := c.Param("storeKey")
	var body models.Store
	err := c.ShouldBind(&body.StoreValue)
	if err != nil {
		shared.ApiOutputError(c, errors.BadInput.Wrap(err, shared.BadRequestBody))
		return
	}
	body.StoreKey = storeKey

	result, err := services.PutStore(storeKey, &body)
	if err != nil {
		shared.ApiOutputError(c, errors.Default.Wrap(err, fmt.Sprintf("PutStore: failed to put %s", storeKey)))
		return
	}

	shared.ApiOutputSuccess(c, result.StoreValue, http.StatusCreated)
}
