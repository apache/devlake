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
	"net/url"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addCompositePrimaryKeys)(nil)

type addCompositePrimaryKeys struct{}

func (script *addCompositePrimaryKeys) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	dbURL := basicRes.GetConfig("DB_URL")
	if dbURL == "" {
		return errors.BadInput.New("DB_URL is required")
	}
	u, err := url.Parse(dbURL)
	if err != nil {
		return errors.Convert(err)
	}

	tables := []string{
		"_tool_agentready_assessments",
		"_tool_agentready_findings",
		"_tool_agentready_metrics",
	}
	for _, table := range tables {
		if u.Scheme == "mysql" {
			if err := db.Exec("ALTER TABLE " + table + " DROP PRIMARY KEY"); err != nil {
				return err
			}
		} else {
			if err := db.Exec("ALTER TABLE " + table + " DROP CONSTRAINT " + table + "_pkey"); err != nil {
				return err
			}
		}
		if err := db.Exec("ALTER TABLE " + table + " ADD PRIMARY KEY (id, connection_id)"); err != nil {
			return err
		}
	}
	return nil
}

func (script *addCompositePrimaryKeys) Version() uint64 {
	return 20260609000001
}

func (script *addCompositePrimaryKeys) Name() string {
	return "agentready add composite primary keys for connection_id"
}
