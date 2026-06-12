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

type modifyCommitCharacterType struct{}

func (*modifyCommitCharacterType) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	// Binary column types are MySQL-specific; PostgreSQL uses bytea natively
	if db.Dialect() == "mysql" {
		err := db.Exec(`ALTER TABLE commits MODIFY COLUMN message LONGTEXT CHARACTER SET binary;`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE commit_files MODIFY COLUMN file_path VARBINARY(255);`)
		if err != nil {
			return err
		}
	}
	return nil

}

func (*modifyCommitCharacterType) Version() uint64 {
	return 20240325160001
}

func (*modifyCommitCharacterType) Name() string {
	return "support commits with ascii-extended character set"
}
