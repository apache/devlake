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
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*increaseCqIssuesProjectKeyLength)(nil)

type increaseCqIssuesProjectKeyLength struct{}

func (script *increaseCqIssuesProjectKeyLength) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	if err := db.ModifyColumnType("cq_issues", "project_key", "varchar(500)"); err != nil {
		return err
	}
	return db.ModifyColumnType("cq_file_metrics", "project_key", "varchar(500)")
}

func (*increaseCqIssuesProjectKeyLength) Version() uint64 {
	return 20260317000000
}

func (*increaseCqIssuesProjectKeyLength) Name() string {
	return "increase cq_issues and cq_file_metrics project_key length to 500"
}
