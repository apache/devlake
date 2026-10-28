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
	"fmt"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addNoPKColumns)(nil)

type addNoPKColumns struct{}

// Tables whose init snapshot embedded an unexported struct. GORM dropped
// created_at, updated_at, and the _raw_data_* columns. Databases that already
// ran 20261020000001 keep that shape until this script adds the columns.
var aisdlcNoPKTables = []string{
	"_tool_aisdlc_scopes",
	"_tool_aisdlc_raw_files",
	"_tool_aisdlc_checkpoints",
	"_tool_aisdlc_executions",
	"_tool_aisdlc_work_items",
	"_tool_aisdlc_observations",
	"_tool_aisdlc_evidence",
	"_tool_aisdlc_item_links",
	"_tool_aisdlc_unknowns",
	"_tool_aisdlc_times",
}

func (*addNoPKColumns) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	timeType := "timestamp NULL"
	idType := "bigint NULL"
	if db.Dialect() == "mysql" {
		timeType = "datetime(3) NULL"
		idType = "bigint unsigned NULL"
	}
	columns := []struct {
		name string
		sql  string
	}{
		{"created_at", timeType},
		{"updated_at", timeType},
		{"_raw_data_params", "varchar(255) NULL"},
		{"_raw_data_table", "varchar(255) NULL"},
		{"_raw_data_id", idType},
		{"_raw_data_remark", "varchar(255) NULL"},
	}
	for _, table := range aisdlcNoPKTables {
		addedParams := false
		for _, column := range columns {
			if db.HasColumn(table, column.name) {
				continue
			}
			stmt := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", table, column.name, column.sql)
			if err := db.Exec(stmt); err != nil {
				return errors.Default.Wrap(err, "add "+table+"."+column.name)
			}
			if column.name == "_raw_data_params" {
				addedParams = true
			}
		}
		if addedParams {
			index := fmt.Sprintf("idx_%s_raw_data_params", table)
			_ = db.Exec(fmt.Sprintf("CREATE INDEX `%s` ON `%s` (`_raw_data_params`)", index, table))
		}
	}
	return nil
}

func (*addNoPKColumns) Version() uint64 {
	return 20261026000001
}

func (*addNoPKColumns) Name() string {
	return "add aisdlc columns skipped by the init snapshot"
}
