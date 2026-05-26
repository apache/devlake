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

package dalgorm

import (
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	Varchar ColumnType = "varchar(255)"
	Text    ColumnType = "text"
	Int     ColumnType = "bigint"
	Time    ColumnType = "timestamp"
	Float   ColumnType = "float"
)

type ColumnType string

func (c ColumnType) String() string {
	return string(c)
}

// Dalgorm implements the dal.Dal interface with gorm
type Dalgorm struct {
	db *gorm.DB
}

var _ dal.Dal = (*Dalgorm)(nil)

func transformParams(params []interface{}) []interface{} {
	tp := make([]interface{}, 0, len(params))

	for _, v := range params {
		switch p := v.(type) {
		case dal.ClauseColumn:
			tp = append(tp, clause.Column{
				Table: p.Table,
				Name:  p.Name,
				Alias: p.Alias,
				Raw:   p.Raw,
			})
		case dal.ClauseTable:
			tp = append(tp, clause.Table{
				Name:  p.Name,
				Alias: p.Alias,
				Raw:   p.Raw,
			})
		default:
			tp = append(tp, p)
		}
	}

	return tp
}

func buildTx(tx *gorm.DB, clauses []dal.Clause) *gorm.DB {
	for _, c := range clauses {
		t := c.Type
		d := c.Data
		switch t {
		case dal.JoinClause:
			tx = tx.Joins(d.(dal.DalClause).Expr, transformParams(d.(dal.DalClause).Params)...)
		case dal.WhereClause:
			tx = tx.Where(d.(dal.DalClause).Expr, transformParams(d.(dal.DalClause).Params)...)
		case dal.OrderbyClause:
			tx = tx.Order(d.(string))
		case dal.LimitClause:
			tx = tx.Limit(d.(int))
		case dal.OffsetClause:
			tx = tx.Offset(d.(int))
		case dal.FromClause:
			switch dd := d.(type) {
			case string:
				tx = tx.Table(dd)
			case dal.DalClause:
				tx = tx.Table(dd.Expr, transformParams(dd.Params)...)
			case dal.ClauseTable:
				tx = tx.Table(" ? ", clause.Table{
					Name:  dd.Name,
					Alias: dd.Alias,
					Raw:   dd.Raw,
				})
			case models.DynamicTabler:
				tx = tx.Table(dd.TableName())
			default:
				tx = tx.Model(d)
			}
		case dal.SelectClause:
			tx = tx.Select(d.(dal.DalClause).Expr, transformParams(d.(dal.DalClause).Params)...)
		case dal.GroupbyClause:
			tx = tx.Group(d.(string))
		case dal.HavingClause:
			tx = tx.Having(d.(dal.DalClause).Expr, transformParams(d.(dal.DalClause).Params)...)
		case dal.LockClause:
			locking := clause.Locking{}
			params := d.([]bool)
			write := params[0]
			if write {
				locking.Strength = "UPDATE"
			}
			nowait := params[1]
			if nowait {
				locking.Options = "NOWAIT"
			}
			tx = tx.Clauses(locking)
		}
	}
	return tx
}

var _ dal.Dal = (*Dalgorm)(nil)

// Exec executes raw sql query
func (d *Dalgorm) Exec(query string, params ...interface{}) errors.Error {
	err := validateQuery(query)
	if err != nil {
		return err
	}
	return d.convertGormError(d.db.Exec(query, transformParams(params)...).Error)
}

var txPattern = regexp.MustCompile(`(?i)^[\s;]*(begin|(start\s+transaction))[\s;]*$`)

func validateQuery(query string) errors.Error {
	if txPattern.MatchString(query) { // regexp.MatchString is thread-safe
		return errors.Default.New("illegal invocation, use the `Begin()` method instead")
	}
	return nil
}

func (d *Dalgorm) unwrapDynamic(entityPtr *interface{}, clausesPtr *[]dal.Clause) {
	if dynamic, ok := (*entityPtr).(models.DynamicTabler); ok {
		if clausesPtr != nil {
			*clausesPtr = append(*clausesPtr, dal.From(dynamic.TableName()))
		}
		*entityPtr = dynamic.Unwrap()
	} else if clausesPtr != nil {
		// try to add dal.From if it does not exist
		for _, c := range *clausesPtr {
			if c.Type == dal.FromClause {
				return
			}
		}
		*clausesPtr = append(*clausesPtr, dal.From(*entityPtr))
	}
}

// AutoMigrate runs auto migration for given models
func (d *Dalgorm) AutoMigrate(entity interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entity, &clauses)
	err := buildTx(d.db, clauses).AutoMigrate(entity)
	if err == nil {
		// fix pg cache plan error
		_ = d.First(entity, clauses...)
	}
	return d.convertGormError(err)
}

// Cursor returns a database cursor, cursor is especially useful when handling big amount of rows of data
func (d *Dalgorm) Cursor(clauses ...dal.Clause) (dal.Rows, errors.Error) {
	rows, err := buildTx(d.db, clauses).Rows()
	return rows, d.convertGormError(err)
}

// CursorTx FIXME ...
func (d *Dalgorm) CursorTx(clauses ...dal.Clause) *gorm.DB {
	return buildTx(d.db, clauses)
}

// Fetch loads row data from `cursor` into `dst`
func (d *Dalgorm) Fetch(cursor dal.Rows, dst interface{}) errors.Error {
	if rows, ok := cursor.(*sql.Rows); ok {
		return d.convertGormError(d.db.ScanRows(rows, dst))
	} else {
		return errors.Default.New(fmt.Sprintf("can not support type %s to be a dal.Rows interface", reflect.TypeOf(cursor).String()))
	}
}

// All loads matched rows from database to `dst`, USE IT WITH COUTIOUS!!
func (d *Dalgorm) All(dst interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&dst, &clauses)
	return d.convertGormError(buildTx(d.db, clauses).Find(dst).Error)
}

// First loads first matched row from database to `dst`, error will be returned if no records were found
func (d *Dalgorm) First(dst interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&dst, &clauses)
	return d.convertGormError(buildTx(d.db, clauses).First(dst).Error)
}

// Count total records
func (d *Dalgorm) Count(clauses ...dal.Clause) (int64, errors.Error) {
	var count int64
	err := buildTx(d.db, clauses).Count(&count).Error
	return errors.Convert01(count, err)
}

// Pluck used to query single column
func (d *Dalgorm) Pluck(column string, dest interface{}, clauses ...dal.Clause) errors.Error {
	return d.convertGormError(buildTx(d.db, clauses).Pluck(column, dest).Error)
}

// Create insert record to database
func (d *Dalgorm) Create(entity interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entity, &clauses)
	return d.convertGormError(buildTx(d.db, clauses).Create(entity).Error)
}

// CreateWithMap insert record to database
func (d *Dalgorm) CreateWithMap(entity interface{}, record map[string]interface{}) errors.Error {
	d.unwrapDynamic(&entity, nil)
	if record != nil {
		if id, ok := record["id"]; ok && id != nil {
			var columns []string
			for column := range record {
				columns = append(columns, column)
			}
			return d.convertGormError(buildTx(d.db, nil).Model(entity).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns(columns),
			}).Create(record).Error)
		} else {
			return d.convertGormError(buildTx(d.db, nil).Model(entity).Clauses(clause.OnConflict{UpdateAll: true}).Create(record).Error)
		}
	}
	return d.convertGormError(buildTx(d.db, nil).Model(entity).Clauses(clause.OnConflict{UpdateAll: true}).Create(record).Error)
}

// Update updates record
func (d *Dalgorm) Update(entity interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entity, &clauses)
	return d.convertGormError(buildTx(d.db, clauses).Save(entity).Error)
}

// CreateOrUpdate tries to create the record, or fallback to update all if failed
func (d *Dalgorm) CreateOrUpdate(entity interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entity, &clauses)
	return d.convertGormError(buildTx(d.db, clauses).Clauses(clause.OnConflict{UpdateAll: true}).Create(entity).Error)
}

// CreateIfNotExist tries to create the record if not exist
func (d *Dalgorm) CreateIfNotExist(entity interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entity, &clauses)
	return d.convertGormError(buildTx(d.db, clauses).Clauses(clause.OnConflict{DoNothing: true}).Create(entity).Error)
}

// Delete records from database
func (d *Dalgorm) Delete(entity interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entity, &clauses)
	return d.convertGormError(buildTx(d.db, clauses).Delete(entity).Error)
}

// UpdateColumn allows you to update multiple records
func (d *Dalgorm) UpdateColumn(entityOrTable interface{}, columnName string, value interface{}, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entityOrTable, &clauses)
	if expr, ok := value.(dal.DalClause); ok {
		value = gorm.Expr(expr.Expr, transformParams(expr.Params)...)
	}
	return d.convertGormError(buildTx(d.db, clauses).Update(columnName, value).Error)
}

// UpdateColumns allows you to update multiple columns of multiple records
func (d *Dalgorm) UpdateColumns(entityOrTable interface{}, set []dal.DalSet, clauses ...dal.Clause) errors.Error {
	d.unwrapDynamic(&entityOrTable, &clauses)
	updatesSet := make(map[string]interface{})

	for _, s := range set {
		if expr, ok := s.Value.(dal.DalClause); ok {
			s.Value = gorm.Expr(expr.Expr, transformParams(expr.Params)...)
		}
		updatesSet[s.ColumnName] = s.Value
	}

	clauses = append(clauses, dal.From(entityOrTable))
	return d.convertGormError(buildTx(d.db, clauses).Updates(updatesSet).Error)
}

// UpdateAllColumn updated all Columns of entity
func (d *Dalgorm) UpdateAllColumn(entity interface{}, clauses ...dal.Clause) errors.Error {
	return d.convertGormError(buildTx(d.db, clauses).UpdateColumns(entity).Error)
}

// GetColumns FIXME ...
func (d *Dalgorm) GetColumns(dst dal.Tabler, filter func(columnMeta dal.ColumnMeta) bool) (cms []dal.ColumnMeta, _ errors.Error) {
	columnTypes, err := d.db.Migrator().ColumnTypes(dst.TableName())
	if err != nil {
		return nil, d.convertGormError(err)
	}
	for _, columnType := range columnTypes {
		if filter == nil {
			cms = append(cms, columnType)
		} else if filter(columnType) {
			cms = append(cms, columnType)
		}
	}
	return cms, nil
}

// AddColumn add one column for the table
func (d *Dalgorm) AddColumn(table, columnName string, columnType dal.ColumnType) errors.Error {
	// work around the error `cached plan must not change result type` for postgres
	// wrap in func(){} to make the linter happy
	defer func() {
		_ = d.Exec("SELECT * FROM ? LIMIT 1", clause.Table{Name: table})
	}()
	return d.Exec("ALTER TABLE ? ADD ? ?", clause.Table{Name: table}, clause.Column{Name: columnName}, clause.Expr{SQL: columnType.String()})
}

// DropColumns drop one column from the table
func (d *Dalgorm) DropColumns(table string, columnNames ...string) errors.Error {
	// work around the error `cached plan must not change result type` for postgres
	// wrap in func(){} to make the linter happy
	defer func() {
		_ = d.Exec("SELECT * FROM ? LIMIT 1", clause.Table{Name: table})
	}()
	for _, columnName := range columnNames {
		err := d.Exec("ALTER TABLE ? DROP COLUMN ?", clause.Table{Name: table}, clause.Column{Name: columnName})
		// err := d.db.Migrator().DropColumn(table, columnName)
		if err != nil {
			return d.convertGormError(err)
		}
	}
	return nil
}

// GetPrimaryKeyFields get the PrimaryKey from `gorm` tag
func (d *Dalgorm) GetPrimaryKeyFields(t reflect.Type) []reflect.StructField {
	return utils.WalkFields(t, func(field *reflect.StructField) bool {
		return strings.Contains(strings.ToLower(field.Tag.Get("gorm")), "primarykey")
	})
}

// RenameColumn renames column name for specified table
func (d *Dalgorm) RenameColumn(table, oldColumnName, newColumnName string) errors.Error {
	// work around the error `cached plan must not change result type` for postgres
	// wrap in func(){} to make the linter happy
	defer func() {
		_ = d.Exec("SELECT * FROM ? LIMIT 1", clause.Table{Name: table})
	}()
	return d.Exec(
		"ALTER TABLE ? RENAME COLUMN ? TO ?",
		clause.Table{Name: table},
		clause.Column{Name: oldColumnName},
		clause.Column{Name: newColumnName},
	)
}

// ModifyColumnType modify column type
func (d *Dalgorm) ModifyColumnType(table, columnName, columnType string) errors.Error {
	// work around the error `cached plan must not change result type` for postgres
	// wrap in func(){} to make the linter happy
	defer func() {
		_ = d.Exec("SELECT * FROM ? LIMIT 1", clause.Table{Name: table})
	}()
	query := "ALTER TABLE ? MODIFY COLUMN ? %s"
	if d.db.Dialector.Name() == "postgres" {
		query = "ALTER TABLE ? ALTER COLUMN ? TYPE %s"
	}
	return d.Exec(
		fmt.Sprintf(query, columnType),
		clause.Table{Name: table},
		clause.Column{Name: columnName},
	)
}

// AllTables returns all tables in the database
func (d *Dalgorm) AllTables() ([]string, errors.Error) {
	var tableSql string
	if d.db.Dialector.Name() == "mysql" {
		tableSql = "show tables"
	} else {
		tableSql = "select table_name from information_schema.tables where table_schema = 'public' and table_name not like '_devlake%'"
	}
	var tables []string
	err := d.db.Raw(tableSql).Scan(&tables).Error
	if err != nil {
		return nil, d.convertGormError(err)
	}
	var filteredTables []string
	for _, table := range tables {
		if !strings.HasPrefix(table, "_devlake") {
			filteredTables = append(filteredTables, table)
		}
	}
	return filteredTables, nil
}

// DropTables drop multiple tables by Model Pointer or Table Name
func (d *Dalgorm) DropTables(dst ...interface{}) errors.Error {
	return d.convertGormError(d.db.Migrator().DropTable(dst...))
}

// HasTable checks if table exists
func (d *Dalgorm) HasTable(table interface{}) bool {
	return d.db.Migrator().HasTable(table)
}

// HasColumn checks if column exists
func (d *Dalgorm) HasColumn(table interface{}, columnName string) bool {
	migrator := d.db.Migrator()
	// Workaround in case table is a string
	// which cause migrator.HasColumn to panic
	// see: https://github.com/go-gorm/gorm/issues/5809
	_, isString := table.(string)
	if isString {
		columnTypes, err := migrator.ColumnTypes(table)
		if err != nil {
			return false
		}
		for _, columnType := range columnTypes {
			if columnType.Name() == columnName {
				return true
			}
		}
		return false
	}
	return migrator.HasColumn(table, columnName)
}

// RenameTable renames table name
func (d *Dalgorm) RenameTable(oldName, newName string) errors.Error {
	err := d.db.Migrator().RenameTable(oldName, newName)
	return d.convertGormError(err)
}

// DropIndexes drops indexes for specified table
func (d *Dalgorm) DropIndexes(table string, indexNames ...string) errors.Error {
	for _, indexName := range indexNames {
		err := d.db.Migrator().DropIndex(table, indexName)
		if err != nil {
			return d.convertGormError(err)
		}
	}
	return nil
}

// DropIndexes drops the index of specified columns
func (d *Dalgorm) DropIndex(table string, columnNames ...string) errors.Error {
	indexName := fmt.Sprintf("idx_%s_%s", table, strings.Join(columnNames, "_"))
	return d.DropIndexes(table, indexName)
}

// Dialect returns the dialect of the database
func (d *Dalgorm) Dialect() string {
	return d.db.Dialector.Name()
}

// Session creates a new manual transaction for special scenarios
func (d *Dalgorm) Session(config dal.SessionConfig) dal.Dal {
	session := d.db.Session(&gorm.Session{
		PrepareStmt:            config.PrepareStmt,
		SkipDefaultTransaction: config.SkipDefaultTransaction,
	})
	return NewDalgorm(session)
}

// Begin create a new transaction
func (d *Dalgorm) Begin() dal.Transaction {
	return newTransaction(d)
}

// IsErrorNotFound checking if the sql error is not found.
func (d *Dalgorm) IsErrorNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// IsDuplicationError checking if the sql error is not found.
func (d *Dalgorm) IsDuplicationError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}

// IsCachedPlanError checks if the error is related to postgres cached query plan
// This error occurs occasionally in Postgres when reusing a cached query
// plan. It can be safely ignored since it does not actually affect results.
func (d *Dalgorm) IsCachedPlanError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "cached plan must not change result type")
}

// IsJsonOrderError checks if the error is related to postgres json ordering
func (d *Dalgorm) IsJsonOrderError(err error) bool {
	return strings.Contains(err.Error(), "identify an ordering operator for type json")
}

// IsTableExist checks if table exists
func (d *Dalgorm) IsTableExist(err error) bool {
	return strings.Contains(err.Error(), "Unknown table")
}

// RawCursor (Deprecated) executes raw sql query and returns a database cursor
func (d *Dalgorm) RawCursor(query string, params ...interface{}) (*sql.Rows, errors.Error) {
	rows, err := d.db.Raw(query, params...).Rows()
	return rows, d.convertGormError(err)
}

// NewDalgorm creates a *Dalgorm
func NewDalgorm(db *gorm.DB) *Dalgorm {
	return &Dalgorm{db}
}

func (d *Dalgorm) convertGormError(err error) errors.Error {
	if err == nil {
		return nil
	}
	if d.IsErrorNotFound(err) {
		return errors.NotFound.WrapRaw(err)
	}
	if d.IsDuplicationError(err) {
		return errors.BadInput.WrapRaw(err)
	}
	if d.IsJsonOrderError(err) {
		return errors.BadInput.WrapRaw(err)
	}
	if d.IsCachedPlanError(err) {
		return nil
	}
	if d.IsTableExist(err) {
		return errors.BadInput.WrapRaw(err)
	}

	return errors.Internal.WrapRaw(err)
}
