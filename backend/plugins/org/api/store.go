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

package api

import (
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/helpers/dbhelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"reflect"
)

type store interface {
	findAllUsers() ([]user, errors.Error)
	findAllTeams() ([]team, errors.Error)
	findAllAccounts() ([]account, errors.Error)
	findAllUserAccounts() ([]userAccount, errors.Error)
	findAllProjectMapping() ([]projectMapping, errors.Error)
	replace(deletions []deletion, items []interface{}) errors.Error
}

// deletion selects the rows of one table that replace deletes.
type deletion struct {
	table interface{}
	where dal.Clause
}

// allRows selects every row of table.
func allRows(table interface{}) deletion {
	return deletion{table: table, where: dal.Where("1=1")}
}

type dbStore struct {
	db       dal.Dal
	basicRes context.BasicRes
}

func NewDbStore(db dal.Dal, basicRes context.BasicRes) *dbStore {
	return &dbStore{db: db, basicRes: basicRes}
}

// txBasicRes gives a BatchSaveDivider the transaction as its Dal.
type txBasicRes struct {
	context.BasicRes
	tx dal.Dal
}

func (r txBasicRes) GetDal() dal.Dal {
	return r.tx
}

func (r txBasicRes) NestedLogger(name string) context.BasicRes {
	return txBasicRes{BasicRes: r.BasicRes.NestedLogger(name), tx: r.tx}
}

func (r txBasicRes) ReplaceLogger(logger log.Logger) context.BasicRes {
	return txBasicRes{BasicRes: r.BasicRes.ReplaceLogger(logger), tx: r.tx}
}

func (d *dbStore) findAllUsers() ([]user, errors.Error) {
	var u *user
	var uu []crossdomain.User
	err := d.db.All(&uu)
	if err != nil {
		return nil, err
	}
	var tus []crossdomain.TeamUser
	err = d.db.All(&tus)
	if err != nil {
		return nil, err
	}
	return u.fromDomainLayer(uu, tus), nil
}
func (d *dbStore) findAllTeams() ([]team, errors.Error) {
	var tt []crossdomain.Team
	err := d.db.All(&tt)
	if err != nil {
		return nil, err
	}
	var t *team
	return t.fromDomainLayer(tt), nil
}
func (d *dbStore) findAllAccounts() ([]account, errors.Error) {
	var aa []crossdomain.Account
	err := d.db.All(&aa)
	if err != nil {
		return nil, err
	}
	var ua []crossdomain.UserAccount
	err = d.db.All(&ua)
	if err != nil {
		return nil, err
	}
	var a *account
	return a.fromDomainLayer(aa, ua), nil
}

func (d *dbStore) findAllUserAccounts() ([]userAccount, errors.Error) {
	var uas []crossdomain.UserAccount
	err := d.db.All(&uas)
	if err != nil {
		return nil, err
	}

	var au *userAccount
	return au.fromDomainLayer(uas), nil
}

func (d *dbStore) findAllProjectMapping() ([]projectMapping, errors.Error) {
	var mapping []crossdomain.ProjectMapping
	err := d.db.All(&mapping)
	if err != nil {
		return nil, err
	}
	var pm *projectMapping
	return pm.fromDomainLayer(mapping), nil
}

// replace deletes the rows that deletions select, then saves items, in one transaction.
// If any step fails, nothing changes. Each call uses a new BatchSaveDivider, because a
// BatchSave keeps its first error and rejects every later row.
func (d *dbStore) replace(deletions []deletion, items []interface{}) (err errors.Error) {
	txHelper := dbhelper.NewTxHelper(d.basicRes, &err)
	defer txHelper.End()
	tx := txHelper.Begin()
	for _, del := range deletions {
		err = tx.Delete(del.table, del.where)
		if err != nil {
			return err
		}
	}
	divider := helper.NewBatchSaveDivider(txBasicRes{BasicRes: d.basicRes, tx: tx}, 1000, "", "")
	var batch *helper.BatchSave
	for _, item := range items {
		batch, err = divider.ForType(reflect.TypeOf(item))
		if err != nil {
			return err
		}
		err = batch.Add(item)
		if err != nil {
			return err
		}
	}
	err = divider.Close()
	return err
}
