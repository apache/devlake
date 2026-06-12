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

type modifyCommitCharacterType0508 struct{}

func (*modifyCommitCharacterType0508) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	// VARBINARY is MySQL-specific; PostgreSQL uses bytea natively
	if db.Dialect() == "mysql" {
		err := db.Exec(`ALTER TABLE commit_files MODIFY COLUMN file_path VARBINARY(1024);`)
		if err != nil {
			return err
		}
	}
	return nil

}

func (*modifyCommitCharacterType0508) Version() uint64 {
	return 20240508160001
}

func (*modifyCommitCharacterType0508) Name() string {
	return "modify commit character type to VARBINARY(1024)"
}
