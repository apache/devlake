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
	"strings"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
)

type jiraScopeConfig20261118 struct{}

func (jiraScopeConfig20261118) TableName() string {
	return "_tool_jira_scope_configs"
}

// convertScopeConfigByteaToJson20261118 fixes the PostgreSQL column types of
// type_mappings and remotelink_repo_pattern. Both were created from
// json.RawMessage and therefore became bytea, while the model writes them
// with the json serializer as text. PostgreSQL parses that text as bytea
// escape input, so any value containing a backslash (e.g. the generated
// remote link regex `\S+`) fails with "invalid input syntax for type bytea".
// MySQL stores them as longblob and is not affected.
type convertScopeConfigByteaToJson20261118 struct{}

func (*convertScopeConfigByteaToJson20261118) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	if db.Dialect() != "postgres" {
		return nil
	}
	columns, err := db.GetColumns(&jiraScopeConfig20261118{}, func(cm dal.ColumnMeta) bool {
		name := cm.Name()
		return (name == "type_mappings" || name == "remotelink_repo_pattern") &&
			strings.EqualFold(cm.DatabaseTypeName(), "bytea")
	})
	if err != nil {
		return err
	}
	for _, cm := range columns {
		column := cm.Name()
		// empty values (e.g. left by 20230609_clear_repo_pattern) are not valid JSON
		err = db.Exec(`ALTER TABLE _tool_jira_scope_configs ALTER COLUMN ` + column + ` TYPE json USING
			CASE WHEN ` + column + ` IS NULL OR length(` + column + `) = 0 THEN NULL
			ELSE convert_from(` + column + `, 'UTF8')::json END`)
		if err != nil {
			return err
		}
	}
	return nil
}

func (*convertScopeConfigByteaToJson20261118) Version() uint64 {
	return 20261118120000
}

func (*convertScopeConfigByteaToJson20261118) Name() string {
	return "convert bytea columns of _tool_jira_scope_configs to json on postgres"
}
