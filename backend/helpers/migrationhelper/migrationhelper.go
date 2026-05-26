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

package migrationhelper

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// AutoMigrateTables runs AutoMigrate for multiple tables
func AutoMigrateTables(basicRes context.BasicRes, dst ...interface{}) errors.Error {
	db := basicRes.GetDal()
	for _, entity := range dst {
		err := db.AutoMigrate(entity)
		if err != nil {
			return err
		}
		_ = db.First(entity)
	}
	return nil
}

// ChangeColumnsType change the type of specified columns for the table
func ChangeColumnsType[D any](
	basicRes context.BasicRes,
	script plugin.MigrationScript,
	tableName string,
	columns []string,
	update func(tmpColumnParams []interface{}) errors.Error,
) (err errors.Error) {
	db := basicRes.GetDal()
	tmpColumnsNames := make([]string, len(columns))
	for i, v := range columns {
		tmpColumnsNames[i] = fmt.Sprintf("%s_%s", v, hashScript(script))
		err = db.RenameColumn(tableName, v, tmpColumnsNames[i])
		if err != nil {
			return err
		}

		defer func(tmpColumnName string, ColumnsName string) {
			if err != nil {
				err1 := db.RenameColumn(tableName, tmpColumnName, ColumnsName)
				if err1 != nil {
					err = errors.Default.Wrap(err, fmt.Sprintf("RollBack by RenameColum failed.Relevant data needs to be repaired manually.%s", err1.Error()))
				}
			}
		}(tmpColumnsNames[i], v)
	}

	err = db.AutoMigrate(new(D), dal.From(tableName))
	if err != nil {
		return errors.Default.Wrap(err, "AutoMigrate for Add Column Error")
	}

	defer func() {
		if err != nil {
			err1 := db.DropColumns(tableName, columns...)
			if err1 != nil {
				err = errors.Default.Wrap(err, fmt.Sprintf("RollBack by DropColumn failed.Relevant data needs to be repaired manually.%s", err1.Error()))
			}
		}
	}()

	if update == nil {
		dalSet := make([]dal.DalSet, 0, len(columns))
		for i, v := range columns {
			dalSet = append(dalSet, dal.DalSet{
				ColumnName: v,
				Value:      tmpColumnsNames[i],
			})
		}
		err = db.UpdateColumns(
			new(D),
			dalSet,
			dal.Where("1=1"),
		)
	} else {
		params := make([]interface{}, 0, len(tmpColumnsNames))
		for _, v := range tmpColumnsNames {
			params = append(params, dal.ClauseColumn{Name: v})
		}
		err = update(params)
	}
	if err != nil {
		return err
	}

	err = db.DropColumns(tableName, tmpColumnsNames...)
	if err != nil {
		return err
	}

	return nil
}

// ChangeColumnsType change the type of specified columns for the table
func ChangePrimaryKeyColumnsType[D any](
	basicRes context.BasicRes,
	script plugin.MigrationScript,
	tableName string,
	TargetPriColumns []string,
	update func(tmpColumnParams []interface{}) errors.Error,
) (err errors.Error) {
	db := basicRes.GetDal()
	dbUrl := basicRes.GetConfig("DB_URL")
	if dbUrl == "" {
		return errors.BadInput.New("DB_URL is required")
	}
	u, errParse := url.Parse(dbUrl)
	if errParse != nil {
		return errors.Convert(errParse)
	}
	// Delete the primary key
	if u.Scheme == "mysql" {
		sql := fmt.Sprintf("ALTER TABLE %s DROP PRIMARY KEY", tableName)
		if err := db.Exec(sql); err != nil {
			return err
		}
	} else {
		sql := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s_pkey", tableName, tableName)
		if err := db.Exec(sql); err != nil {
			return err
		}
	}
	// Change the type of the primary key
	err = ChangeColumnsType[D](basicRes, script, tableName, TargetPriColumns, update)
	if err != nil {
		return err
	}

	// Add the primary key
	sql := fmt.Sprintf("ALTER TABLE %s ADD PRIMARY KEY (%s)", tableName, strings.Join(TargetPriColumns, ","))
	if err := db.Exec(sql); err != nil {
		return err
	}

	return nil
}

// TransformColumns change the type of specified columns for the table and transform data one by one
func TransformColumns[S any, D any](
	basicRes context.BasicRes,
	script plugin.MigrationScript,
	tableName string,
	columns []string,
	transform func(src *S) (*D, errors.Error),
) (err errors.Error) {
	db := basicRes.GetDal()
	return ChangeColumnsType[D](
		basicRes,
		script,
		tableName,
		columns,
		func(tmpColumnParams []interface{}) errors.Error {
			// create selectStr for transform tmpColumnsNames
			params := make([]interface{}, 0, len(columns)*2)
			selectStr := " * "
			for i, v := range columns {
				selectStr += ",? as ?"
				params = append(params, tmpColumnParams[i])
				params = append(params, dal.ClauseColumn{Name: v})
			}

			cursor, err := db.Cursor(
				dal.Select(selectStr, params...),
				dal.From(dal.ClauseTable{Name: tableName}),
			)
			if err != nil {
				return errors.Default.Wrap(err, fmt.Sprintf("failed to load data from src table [%s]", tableName))
			}

			defer cursor.Close()
			batch, err := helper.NewBatchSave(basicRes, reflect.TypeOf(new(D)), 200, tableName)
			if err != nil {
				return errors.Default.Wrap(err, fmt.Sprintf("failed to instantiate BatchSave for table [%s]", tableName))
			}
			defer batch.Close()
			for cursor.Next() {
				src := new(S)
				err = db.Fetch(cursor, src)
				if err != nil {
					return errors.Default.Wrap(err, fmt.Sprintf("fail to load record from table [%s]", tableName))
				}

				dst, err := transform(src)
				if err != nil {
					return errors.Default.Wrap(err, fmt.Sprintf("failed to update row %v", src))
				}
				if dst == nil {
					continue
				}
				err = batch.Add(dst)
				if err != nil {
					return errors.Default.Wrap(err, fmt.Sprintf("push to BatchSave failed %v", dst))
				}
			}
			return nil
		},
	)
}

// TransformTable can be used when we need to change the table structure and reprocess all the data in the table.
func TransformTable[S any, D any](
	basicRes context.BasicRes,
	script plugin.MigrationScript,
	tableName string,
	transform func(*S) (*D, errors.Error),
) (err errors.Error) {
	db := basicRes.GetDal()
	tmpTableName := fmt.Sprintf("%s_%s", tableName, hashScript(script))

	err = PrimarykeyIsAutoIncrement(db, tableName)
	if err != nil {
		return errors.Default.Wrap(err, "failed to check PrimarykeyIsAutoIncrement on TransformTable")
	}

	// rename the src to tmp in case of failure
	err = db.RenameTable(tableName, tmpTableName)
	if err != nil {
		return errors.Default.Wrap(
			err,
			fmt.Sprintf("failed to rename rename src table [%s] to [%s]", tableName, tmpTableName),
		)
	}

	// rollback for error
	defer func() {
		if err != nil {
			err1 := db.RenameTable(tmpTableName, tableName)
			if err1 != nil {
				msg := fmt.Sprintf(
					"fail to rollback table [%s] to [%s]:[%s], you may have to do it manually",
					tmpTableName,
					tableName,
					err1.Error(),
				)
				err = errors.Default.Wrap(err, msg)
			}
		}
	}()

	// create new table with the same name
	err = db.AutoMigrate(new(D), dal.From(tableName))
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error on auto migrate [%s]", tableName))
	}

	// rollback for error
	defer func() {
		if err != nil {
			err1 := db.DropTables(tableName)
			if err1 != nil {
				msg := fmt.Sprintf(
					"fail to drop table [%s]:[%s], you may have to do it manually",
					tableName,
					err1.Error(),
				)
				err = errors.Default.Wrap(err, msg)
			}
		}
	}()

	err = CopyTableColumns(basicRes, tmpTableName, tableName, transform)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error to CopyTableColumn from [%s] to [%s]", tmpTableName, tableName))
	}

	// drop the temp table: we can safely ignore the error because it doesn't matter and there will be nothing we
	// can do in terms of rollback or anything in that nature.
	_ = db.DropTables(tmpTableName)

	return err
}

// CopyTableColumns can copy data from src table to dst table
func CopyTableColumns[S any, D any](
	basicRes context.BasicRes,
	srcTableName string,
	dstTableName string,
	transform func(*S) (*D, errors.Error),
) (err errors.Error) {
	db := basicRes.GetDal()

	cursor, err := db.Cursor(dal.From(srcTableName))
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to load data from src table [%s]", srcTableName))
	}
	if !reflect.ValueOf(cursor).IsNil() {
		defer cursor.Close()
	}

	batch, err := helper.NewBatchSave(basicRes, reflect.TypeOf(new(D)), 200, dstTableName)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to instantiate BatchSave for table [%s]", srcTableName))
	}
	defer batch.Close()

	if reflect.ValueOf(cursor).IsNil() {
		return nil
	}
	for cursor.Next() {
		srcTable := new(S)
		err1 := db.Fetch(cursor, srcTable)
		if err1 != nil {
			return errors.Default.Wrap(err1, fmt.Sprintf("fail to load record from table [%s]", srcTableName))
		}

		dst, err1 := transform(srcTable)
		if err1 != nil {
			return errors.Default.Wrap(err1, fmt.Sprintf("failed to transform row %v", srcTable))
		}

		err1 = batch.Add(dst)
		if err1 != nil {
			return errors.Default.Wrap(err1, fmt.Sprintf("push to BatchSave failed %v", dstTableName))
		}
	}

	return err
}

// PrimarykeyIsAutoIncrement check if the Primarykey is auto increment
func PrimarykeyIsAutoIncrement(db dal.Dal, tableName string) errors.Error {
	var pkcs []dal.ColumnMeta
	pkcs, err := dal.GetPrimarykeyColumns(db, dal.DefaultTabler{Name: tableName})
	if err != nil {
		return errors.Default.Wrap(
			err,
			fmt.Sprintf("failed to GetPrimarykeyColumns from table [%s]", tableName),
		)
	}

	for _, pkc := range pkcs {
		isAutoIncrement, ok := pkc.AutoIncrement()
		if !ok {
			// if this pk has not concept about isAutoIncrement that means it is not AutoIncrement
			continue
		}

		if isAutoIncrement {
			return errors.Default.New(
				fmt.Sprintf("the Primarykey[%s] is AutoIncrement on table [%s]", pkc.Name(), tableName),
			)
		}
	}
	return nil
}

func hashScript(script plugin.MigrationScript) string {
	hasher := md5.New()
	_, err := hasher.Write([]byte(fmt.Sprintf("%s:%v", script.Name(), script.Version())))
	if err != nil {
		panic(err)
	}
	return strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
}
