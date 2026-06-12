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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/q_dev/models"
)

var _ plugin.MigrationScript = (*addAuthTypeToConnection)(nil)

type addAuthTypeToConnection struct{}

func (*addAuthTypeToConnection) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	if !db.HasColumn("_tool_q_dev_connections", "auth_type") {
		if err := db.AddColumn("_tool_q_dev_connections", "auth_type", dal.Varchar); err != nil {
			return errors.Default.Wrap(err, "failed to add auth_type to _tool_q_dev_connections")
		}
	}

	// Default existing rows to "access_key" since they were created before IAM role support
	if err := db.Exec("UPDATE _tool_q_dev_connections SET auth_type = ? WHERE auth_type IS NULL OR auth_type = ''", models.AuthTypeAccessKey); err != nil {
		return errors.Default.Wrap(err, "failed to set default auth_type for existing connections")
	}

	return nil
}

func (*addAuthTypeToConnection) Version() uint64 {
	return 20260320000001
}

func (*addAuthTypeToConnection) Name() string {
	return "add auth_type column to _tool_q_dev_connections for IAM role support"
}
