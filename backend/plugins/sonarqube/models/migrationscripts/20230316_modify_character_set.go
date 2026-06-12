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
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
)

type modifyCharacterSet struct{}

func (*modifyCharacterSet) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	// Character set conversion is MySQL-specific; PostgreSQL uses UTF-8 by default
	if db.Dialect() == "mysql" {
		err := db.Exec(`ALTER TABLE _tool_sonarqube_projects CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE _tool_sonarqube_issues CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE _tool_sonarqube_issue_code_blocks CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE _tool_sonarqube_hotspots CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE _tool_sonarqube_file_metrics CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE _tool_sonarqube_accounts CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE cq_projects CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE cq_issues CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE cq_issue_code_blocks CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE cq_file_metrics CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
		if err != nil {
			return err
		}
	}
	return nil

}

func (*modifyCharacterSet) Version() uint64 {
	return 20230316220071
}

func (*modifyCharacterSet) Name() string {
	return "sonarqube modify character set"
}
