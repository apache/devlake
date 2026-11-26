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
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/plugin"
	"net/http"

	"github.com/gocarina/gocsv"
)

// GetUserAccountMapping returns all user/account mapping in csv format
// @Summary      Get user_account_mapping.csv.csv file
// @Description  get user_account_mapping.csv.csv file
// @Tags 		 plugins/org
// @Produce      text/csv
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/org/user_account_mapping.csv [get]
func (h *Handlers) GetUserAccountMapping(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	accounts, err := h.store.findAllAccounts()
	if err != nil {
		return nil, err
	}
	blob, err := errors.Convert01(gocsv.MarshalBytes(accounts))
	if err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{
		Body:   nil,
		Status: http.StatusOK,
		File: &plugin.OutputFile{
			ContentType: "text/csv",
			Data:        blob,
		},
	}, nil
}

// CreateUserAccountMapping accepts a CSV file containing user/account mapping and saves it to the database
// @Summary      Upload user_account_mapping.csv.csv file
// @Description  upload user_account_mapping.csv.csv file
// @Tags 		 plugins/org
// @Accept       multipart/form-data
// @Param        file formData file true "select file to upload"
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/org/user_account_mapping.csv [put]
func (h *Handlers) CreateUserAccountMapping(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var aa []account
	err := h.unmarshal(input.Request, &aa)
	if err != nil {
		return nil, err
	}
	var a *account
	var items []interface{}
	userAccounts := a.toDomainLayer(aa)
	for _, userAccount := range userAccounts {
		items = append(items, userAccount)
	}
	err = h.store.replace([]deletion{allRows(&crossdomain.UserAccount{})}, items)
	if err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Status: http.StatusOK}, nil
}
