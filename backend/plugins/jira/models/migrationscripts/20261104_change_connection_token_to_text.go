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
)

var _ plugin.MigrationScript = (*changeConnectionTokenToText20261104)(nil)

type changeConnectionTokenToText20261104 struct{}

// Up widens the token column. The token is stored encrypted and base64 encoded,
// which is longer than the plain token, so varchar(255) cut long tokens
// (for example scoped service-account tokens).
func (*changeConnectionTokenToText20261104) Up(basicRes context.BasicRes) errors.Error {
	return basicRes.GetDal().ModifyColumnType("_tool_jira_connections", "token", "text")
}

func (*changeConnectionTokenToText20261104) Version() uint64 {
	return 20261104120000
}

func (*changeConnectionTokenToText20261104) Name() string {
	return "change token type to text in _tool_jira_connections"
}
