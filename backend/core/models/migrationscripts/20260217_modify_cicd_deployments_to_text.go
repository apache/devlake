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
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*modifyCicdDeploymentsToText)(nil)

type modifyCicdDeploymentsToText struct{}

type cicdDeployment20260217 struct {
	Name string
}

func (cicdDeployment20260217) TableName() string {
	return "cicd_deployments"
}

func (script *modifyCicdDeploymentsToText) Up(basicRes context.BasicRes) errors.Error {
	// cicd_deployments.name might be text, we ought to change the type
	// for the column from `varchar(255)` to `text`
	db := basicRes.GetDal()
	return migrationhelper.ChangeColumnsType[cicdDeployment20260217](
		basicRes,
		script,
		cicdDeployment20260217{}.TableName(),
		[]string{"name"},
		func(tmpColumnParams []interface{}) errors.Error {
			return db.UpdateColumn(
				&cicdDeployment20260217{},
				"name",
				dal.DalClause{Expr: " ? ", Params: tmpColumnParams},
				dal.Where("? != '' ", tmpColumnParams...),
			)
		},
	)
}

func (*modifyCicdDeploymentsToText) Version() uint64 {
	return 20260217145125
}

func (*modifyCicdDeploymentsToText) Name() string {
	return "modify cicd_deployments name from varchar to text"
}
