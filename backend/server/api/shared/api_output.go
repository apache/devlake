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

package shared

import (
	"net/http"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/impls/logruslog"

	"github.com/gin-gonic/gin"
)

const BadRequestBody = "bad request body format"

type TypedApiBody[T any] struct {
	Code    int      `json:"code"`
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Causes  []string `json:"causes"`
	Data    T        `json:"data"`
}

type ApiBody TypedApiBody[interface{}]

type ResponsePipelines struct {
	Count     int64              `json:"count"`
	Pipelines []*models.Pipeline `json:"pipelines"`
}

// ApiOutputErrorWithCustomCode writes a JSON error message to the HTTP response body
func ApiOutputErrorWithCustomCode(c *gin.Context, code int, err error) {
	if e, ok := err.(errors.Error); ok {
		logruslog.Global.Error(err, "HTTP %d error", ClientHttpCode(e.GetType().GetHttpCode()))
		messages := e.Messages()
		c.JSON(ClientHttpCode(e.GetType().GetHttpCode()), &ApiBody{
			Success: false,
			Message: e.Error(),
			Code:    code,
			Causes:  messages.Causes(),
		})
	} else {
		logruslog.Global.Error(err, "HTTP %d error (native)", http.StatusInternalServerError)
		c.JSON(http.StatusInternalServerError, &ApiBody{
			Success: false,
			Code:    code,
			Message: err.Error(),
		})
	}
	c.Writer.Header().Set("Content-Type", "application/json")
}

// ApiOutputAdvancedErrorWithCustomCode writes a JSON error message to the HTTP response body
func ApiOutputAdvancedErrorWithCustomCode(c *gin.Context, httpStatusCode, customBusinessCode int, err error) {
	if e, ok := err.(errors.Error); ok {
		logruslog.Global.Error(err, "HTTP %d error", ClientHttpCode(e.GetType().GetHttpCode()))
		messages := e.Messages()
		c.JSON(ClientHttpCode(e.GetType().GetHttpCode()), &ApiBody{
			Success: false,
			Message: e.Error(),
			Code:    customBusinessCode,
			Causes:  messages.Causes(),
		})
	} else {
		logruslog.Global.Error(err, "HTTP %d error (native)", http.StatusInternalServerError)
		c.JSON(httpStatusCode, &ApiBody{
			Success: false,
			Code:    customBusinessCode,
			Message: err.Error(),
		})
	}
	c.Writer.Header().Set("Content-Type", "application/json")
}

// ApiOutputError writes a JSON error message to the HTTP response body
func ApiOutputError(c *gin.Context, err error) {
	if e, ok := err.(errors.Error); ok {
		logruslog.Global.Error(err, "HTTP %d error", ClientHttpCode(e.GetType().GetHttpCode()))
		messages := e.Messages()
		c.JSON(ClientHttpCode(e.GetType().GetHttpCode()), &ApiBody{
			Success: false,
			Message: e.Error(),
			Causes:  messages.Causes(),
		})
	} else {
		logruslog.Global.Error(err, "HTTP %d error (native)", http.StatusInternalServerError)
		c.JSON(http.StatusInternalServerError, &ApiBody{
			Success: false,
			Message: err.Error(),
		})
	}
	c.Writer.Header().Set("Content-Type", "application/json")
}

// ApiOutputSuccess writes a JSON success message to the HTTP response body
func ApiOutputSuccess(c *gin.Context, body interface{}, status int) {
	if body == nil {
		body = &ApiBody{
			Success: true,
			Message: "success",
		}
	}
	c.JSON(status, body)
}

// ApiOutputAbort writes the HTTP response code header and saves the error internally, but doesn't push it to the response
func ApiOutputAbort(c *gin.Context, err error) {
	if e, ok := err.(errors.Error); ok {
		logruslog.Global.Error(err, "HTTP %d abort-error", ClientHttpCode(e.GetType().GetHttpCode()))
		_ = c.AbortWithError(ClientHttpCode(e.GetType().GetHttpCode()), errors.Default.New(e.Messages().Format()))
	} else {
		logruslog.Global.Error(err, "HTTP %d abort-error (native)", http.StatusInternalServerError)
		_ = c.AbortWithError(http.StatusInternalServerError, err)
	}
}

// ClientHttpCode maps an upstream 401 to 502, since clients treat 401 as an expired DevLake session
func ClientHttpCode(code int) int {
	if code == http.StatusUnauthorized {
		return http.StatusBadGateway
	}
	return code
}
