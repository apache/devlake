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
)

type addServiceToConnections struct{}

func (*addServiceToConnections) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	return db.AutoMigrate(&addServiceToConnections20260804{})
}

func (*addServiceToConnections) Version() uint64 {
	return 20260804000000
}

func (*addServiceToConnections) Name() string {
	return "Codecov add service field to connections for multi-service support"
}

type addServiceToConnections20260804 struct {
	Service string `gorm:"column:service;type:varchar(100);default:github"`
}

func (addServiceToConnections20260804) TableName() string {
	return "_tool_codecov_connections"
}
