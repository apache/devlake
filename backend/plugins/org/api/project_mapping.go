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
	"fmt"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/plugin"
	"net/http"
	"strings"

	"github.com/gocarina/gocsv"
)

// GetProjectMapping returns all project mapping in csv format
// @Summary      Get project_mapping.csv file
// @Description  get project_mapping.csv file
// @Tags 		 plugins/org
// @Produce      text/csv
// @Param        fake_data    query     bool  false  "return fake data or not"
// @Success      200 {object} plugin.ApiResourceOutput
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/org/project_mapping.csv [get]
func (h *Handlers) GetProjectMapping(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	input.Query.Get("fake_data")
	var mapping []projectMapping
	var err error
	if input.Query.Get("fake_data") == "true" {
		mapping = fakeProjectMapping
	} else {
		mapping, err = h.store.findAllProjectMapping()
		if err != nil {
			return nil, errors.Convert(err)
		}
	}
	blob, err := gocsv.MarshalBytes(mapping)
	if err != nil {
		return nil, errors.Convert(err)
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

// CreateProjectMapping accepts a CSV file containing project mapping and saves it to the database
// @Summary      Upload project_mapping.csv file
// @Description  upload project_mapping.csv file
// @Tags 		 plugins/org
// @Accept       multipart/form-data
// @Param        file formData file true "select file to upload"
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/org/project_mapping.csv [put]
func (h *Handlers) CreateProjectMapping(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var mapping []projectMapping
	err := h.unmarshal(input.Request, &mapping)
	if err != nil {
		return nil, err
	}
	err = checkProjectMappings(mapping)
	if err != nil {
		return nil, err
	}
	var pm *projectMapping
	var items []interface{}
	for _, tm := range pm.toDomainLayer(mapping) {
		items = append(items, tm)
	}
	err = h.store.replace(projectMappingDeletions(mapping), items)
	if err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Status: http.StatusOK}, nil
}

// checkProjectMappings rejects a file with an empty ProjectName, Table or RowId in any row.
// An empty field usually means a wrong header, such as RowID. Saving such a file would
// delete the real mappings of the project.
func checkProjectMappings(mapping []projectMapping) errors.Error {
	for i, m := range mapping {
		if strings.TrimSpace(m.ProjectName) == "" || strings.TrimSpace(m.Table) == "" || strings.TrimSpace(m.RowId) == "" {
			return errors.BadInput.New(fmt.Sprintf("row %d: ProjectName, Table and RowId must not be empty", i+1))
		}
	}
	return nil
}

// projectMappingDeletions selects the existing mappings of the projects named in mapping,
// 1000 project names per deletion. Mappings of other projects are kept, because blueprints
// also write this table.
func projectMappingDeletions(mapping []projectMapping) []deletion {
	var projectNames []string
	seen := make(map[string]bool)
	for _, m := range mapping {
		if !seen[m.ProjectName] {
			seen[m.ProjectName] = true
			projectNames = append(projectNames, m.ProjectName)
		}
	}
	var deletions []deletion
	for start := 0; start < len(projectNames); start += 1000 {
		deletions = append(deletions, deletion{
			table: &crossdomain.ProjectMapping{},
			where: dal.Where("project_name IN ?", projectNames[start:min(start+1000, len(projectNames))]),
		})
	}
	return deletions
}
