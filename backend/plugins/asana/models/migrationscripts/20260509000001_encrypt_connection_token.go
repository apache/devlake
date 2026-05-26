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
)

type asanaConnectionTokenPlain struct {
	ID    uint64 `gorm:"primaryKey"`
	Token string
}

func (asanaConnectionTokenPlain) TableName() string {
	return "_tool_asana_connections"
}

type encryptConnectionToken struct{}

func (*encryptConnectionToken) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	encKey := basicRes.GetConfig(plugin.EncodeKeyEnvStr)
	if encKey == "" {
		return errors.BadInput.New("asana invalid encKey")
	}

	cursor, err := db.Cursor(dal.From(&asanaConnectionTokenPlain{}))
	if err != nil {
		return err
	}
	defer cursor.Close()

	for cursor.Next() {
		row := &asanaConnectionTokenPlain{}
		if err = db.Fetch(cursor, row); err != nil {
			return err
		}
		if row.Token == "" {
			continue
		}
		encryptedToken, err := plugin.Encrypt(encKey, row.Token)
		if err != nil {
			return err
		}
		err = db.UpdateColumns(
			row.TableName(),
			[]dal.DalSet{{ColumnName: "token", Value: encryptedToken}},
			dal.Where("id = ?", row.ID),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (*encryptConnectionToken) Version() uint64 {
	return 20260509000001
}

func (*encryptConnectionToken) Name() string {
	return "encrypt asana connection token"
}
