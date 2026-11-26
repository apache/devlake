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
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/e2ehelper"
	contextimpl "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/plugins/org/api"
	"github.com/apache/devlake/plugins/org/impl"
)

// newUploadHandlers returns a DataFlowTester and the org API handlers, which write to the tester's database.
// The store methods are not exported, so the tests reach them through the handlers.
func newUploadHandlers(t *testing.T) (*e2ehelper.DataFlowTester, *api.Handlers) {
	t.Helper()
	dataflowTester := e2ehelper.NewDataFlowTester(t, "org", impl.Org{})
	basicRes := contextimpl.NewDefaultBasicRes(dataflowTester.Cfg, dataflowTester.Log, dataflowTester.Dal)
	return dataflowTester, api.NewHandlers(basicRes)
}

// uploadInput returns an API input whose request uploads the CSV file at csvRelPath.
func uploadInput(t *testing.T, csvRelPath string) *plugin.ApiResourceInput {
	t.Helper()
	content, err := os.ReadFile(csvRelPath)
	if err != nil {
		t.Fatal(err)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "upload.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/plugins/org/upload.csv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return &plugin.ApiResourceInput{Request: req}
}
