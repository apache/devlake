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

package e2e

import (
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/org/api"
)

var projectMappingFields = []string{"project_name", "table", "row_id"}

// newProjectMappingUpload flushes project_mapping and adds a mapping of the project Other,
// as a blueprint writes it. The uploads below never name Other, so its mapping must stay.
func newProjectMappingUpload(t *testing.T) (*e2ehelper.DataFlowTester, *api.Handlers) {
	t.Helper()
	dataflowTester, handlers := newUploadHandlers(t)
	dataflowTester.FlushTabler(&crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./raw_tables/project_mapping_other.csv", &crossdomain.ProjectMapping{})
	return dataflowTester, handlers
}

func TestProjectMappingUploadReplacesProjectsInFile(t *testing.T) {
	dataflowTester, handlers := newProjectMappingUpload(t)

	if _, err := handlers.CreateProjectMapping(uploadInput(t, "./raw_tables/upload_project_mapping_three.csv")); err != nil {
		t.Fatal(err)
	}
	dataflowTester.VerifyTable(crossdomain.ProjectMapping{}, "./snapshot_tables/upload_project_mapping_three.csv", projectMappingFields)

	if _, err := handlers.CreateProjectMapping(uploadInput(t, "./raw_tables/upload_project_mapping_two.csv")); err != nil {
		t.Fatal(err)
	}
	dataflowTester.VerifyTable(crossdomain.ProjectMapping{}, "./snapshot_tables/upload_project_mapping_two.csv", projectMappingFields)
}

// The too_long file has a RowId of 300 characters. The column is varchar(255), so MySQL
// in strict mode (the default) and PostgreSQL reject the row after the delete has run.
func TestProjectMappingUploadFailureKeepsMappings(t *testing.T) {
	dataflowTester, handlers := newProjectMappingUpload(t)
	if _, err := handlers.CreateProjectMapping(uploadInput(t, "./raw_tables/upload_project_mapping_two.csv")); err != nil {
		t.Fatal(err)
	}

	if _, err := handlers.CreateProjectMapping(uploadInput(t, "./raw_tables/upload_project_mapping_too_long.csv")); err == nil {
		t.Error("the upload with a RowId that is too long must return an error")
	}
	dataflowTester.VerifyTable(crossdomain.ProjectMapping{}, "./snapshot_tables/upload_project_mapping_two.csv", projectMappingFields)
}

// A file can list the same mapping twice. PostgreSQL rejects an upsert that writes one key
// twice ("ON CONFLICT DO UPDATE command cannot affect row a second time"). BatchSave in
// the store removes the duplicate, so this test guards that the upload keeps saving through it.
func TestProjectMappingUploadAcceptsDuplicateRows(t *testing.T) {
	dataflowTester, handlers := newProjectMappingUpload(t)

	if _, err := handlers.CreateProjectMapping(uploadInput(t, "./raw_tables/upload_project_mapping_duplicate.csv")); err != nil {
		t.Fatal(err)
	}
	dataflowTester.VerifyTable(crossdomain.ProjectMapping{}, "./snapshot_tables/upload_project_mapping_two.csv", projectMappingFields)
}
