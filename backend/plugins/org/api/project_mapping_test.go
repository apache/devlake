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
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/plugin"
	"github.com/stretchr/testify/assert"
)

// fakeStore records the arguments of replace. Its other methods are not used here.
type fakeStore struct {
	store
	replaced  bool
	deletions []deletion
	items     []interface{}
	err       errors.Error
}

func (f *fakeStore) replace(deletions []deletion, items []interface{}) errors.Error {
	f.replaced = true
	f.deletions = deletions
	f.items = items
	return f.err
}

func csvUpload(t *testing.T, content string) *plugin.ApiResourceInput {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "project_mapping.csv")
	assert.NoError(t, err)
	_, err = part.Write([]byte(content))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPut, "/plugins/org/project_mapping.csv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return &plugin.ApiResourceInput{Request: req}
}

func TestCheckProjectMappings(t *testing.T) {
	tests := []struct {
		name    string
		mapping []projectMapping
		wantErr string
	}{
		{
			name:    "all fields set",
			mapping: []projectMapping{{ProjectName: "Payments", Table: "repos", RowId: "github:GithubRepo:1:7301"}},
		},
		{
			name:    "empty project name",
			mapping: []projectMapping{{ProjectName: "", Table: "repos", RowId: "github:GithubRepo:1:7301"}},
			wantErr: "row 1: ProjectName, Table and RowId must not be empty",
		},
		{
			name:    "blank table",
			mapping: []projectMapping{{ProjectName: "Payments", Table: "  ", RowId: "github:GithubRepo:1:7301"}},
			wantErr: "row 1: ProjectName, Table and RowId must not be empty",
		},
		{
			name: "empty row id in the second row",
			mapping: []projectMapping{
				{ProjectName: "Payments", Table: "repos", RowId: "github:GithubRepo:1:7301"},
				{ProjectName: "Payments", Table: "repos", RowId: ""},
			},
			wantErr: "row 2: ProjectName, Table and RowId must not be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkProjectMappings(tt.mapping)
			if tt.wantErr == "" {
				assert.Nil(t, err)
				return
			}
			if assert.NotNil(t, err) {
				assert.Equal(t, errors.BadInput, err.GetType())
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestProjectMappingDeletions(t *testing.T) {
	mapping := []projectMapping{
		{ProjectName: "Payments", Table: "repos", RowId: "github:GithubRepo:1:7301"},
		{ProjectName: "Billing", Table: "boards", RowId: "jira:JiraBoard:1:7302"},
		{ProjectName: "Payments", Table: "boards", RowId: "jira:JiraBoard:1:7303"},
	}
	assert.Equal(t, []deletion{{
		table: &crossdomain.ProjectMapping{},
		where: dal.Where("project_name IN ?", []string{"Payments", "Billing"}),
	}}, projectMappingDeletions(mapping))

	var many []projectMapping
	var names []string
	for i := 0; i < 1001; i++ {
		name := fmt.Sprintf("project-%d", i)
		names = append(names, name)
		many = append(many, projectMapping{ProjectName: name, Table: "repos", RowId: "github:GithubRepo:1:1"})
	}
	assert.Equal(t, []deletion{
		{table: &crossdomain.ProjectMapping{}, where: dal.Where("project_name IN ?", names[:1000])},
		{table: &crossdomain.ProjectMapping{}, where: dal.Where("project_name IN ?", names[1000:])},
	}, projectMappingDeletions(many), "1001 project names are deleted in two statements")
}

func TestCreateProjectMapping(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		storeErr     errors.Error
		wantErr      string
		wantReplaced bool
		wantItems    int
	}{
		{
			name:         "replaces the mappings of the projects in the file",
			file:         "ProjectName,Table,RowId\nPayments,repos,github:GithubRepo:1:7301\nPayments,boards,jira:JiraBoard:1:7302\n",
			wantReplaced: true,
			wantItems:    2,
		},
		{
			name:    "rejects a wrong header before it changes anything",
			file:    "ProjectName,Table,RowID\nPayments,repos,github:GithubRepo:1:7301\n",
			wantErr: "row 1: ProjectName, Table and RowId must not be empty",
		},
		{
			name:         "returns the error of a failed save",
			file:         "ProjectName,Table,RowId\nPayments,repos,github:GithubRepo:1:7301\n",
			storeErr:     errors.Default.New("save failed"),
			wantErr:      "save failed",
			wantReplaced: true,
			wantItems:    1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeStore{err: tt.storeErr}
			h := &Handlers{store: fake}
			_, err := h.CreateProjectMapping(csvUpload(t, tt.file))
			if tt.wantErr == "" {
				assert.Nil(t, err)
			} else if assert.NotNil(t, err) {
				assert.Contains(t, err.Error(), tt.wantErr)
			}
			assert.Equal(t, tt.wantReplaced, fake.replaced)
			assert.Len(t, fake.items, tt.wantItems)
		})
	}
}
