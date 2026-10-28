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

package migrationscripts

import (
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	pluginhelper "github.com/apache/devlake/helpers/pluginhelper/api"
)

var _ plugin.MigrationScript = (*addRawCicdTestJobsTable)(nil)

type addRawCicdTestJobsTable struct{}

// rawCicdTestJobsData is a wrapper around RawData to specify the table name
type rawCicdTestJobsData struct {
	pluginhelper.RawData
}

func (rawCicdTestJobsData) TableName() string {
	return "_raw_cicd_test_jobs"
}

func (*addRawCicdTestJobsTable) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Create _raw_cicd_test_jobs table using RawData schema
	// Use a wrapper struct with TableName() method to specify the table name
	rawDataModel := &rawCicdTestJobsData{}

	// Check if table already exists
	if db.HasTable(rawDataModel) {
		// Table already exists, no need to create it
		return nil
	}

	// Use GORM's AutoMigrate to create the table with RawData schema
	err := db.AutoMigrate(rawDataModel)
	if err != nil {
		return errors.Default.Wrap(err, "failed to create _raw_cicd_test_jobs table")
	}

	return nil
}

func (*addRawCicdTestJobsTable) Version() uint64 {
	return 20250109000001
}

func (*addRawCicdTestJobsTable) Name() string {
	return "add _raw_cicd_test_jobs table for storing raw Prow job JSON data"
}
