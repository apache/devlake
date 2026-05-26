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

package e2ehelper

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cast"

	"github.com/apache/incubator-devlake/core/config"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/runner"
	"github.com/apache/incubator-devlake/core/utils"
	"github.com/apache/incubator-devlake/helpers/pluginhelper"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	contextimpl "github.com/apache/incubator-devlake/impls/context"
	"github.com/apache/incubator-devlake/impls/dalgorm"
	"github.com/apache/incubator-devlake/impls/logruslog"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// DataFlowTester provides a universal data integrity validation facility to help `Plugin` verifying records between
// each step.
//
// How it works:
//
//   1. Flush specified `table` and import data from a `csv` file by `ImportCsv` method
//   2. Execute specified `subtask` by `Subtask` method
//   3. Verify actual data from specified table against expected data from another `csv` file
//   4. Repeat step 2 and 3
//
// Recommended Usage:

// DataFlowTester use `N`
//  1. Create a folder under your plugin root folder. i.e. `plugins/gitlab/e2e/ to host all your e2e-tests`
//  2. Create a folder named `tables` to hold all data in `csv` format
//  3. Create e2e test-cases to cover all possible data-flow routes
//
// Example code:
//
//	See [Gitlab Project Data Flow Test](plugins/gitlab/e2e/project_test.go) for detail
type DataFlowTester struct {
	Cfg    *viper.Viper
	Db     *gorm.DB
	Dal    dal.Dal
	T      *testing.T
	Name   string
	Plugin plugin.PluginMeta
	Log    log.Logger
}

// TableOptions FIXME ...
type TableOptions struct {
	// CSVRelPath relative path to the CSV file that contains the seeded data
	CSVRelPath string
	// TargetFields the fields (columns) to consider for verification. Leave empty to default to all.
	TargetFields []string
	// IgnoreFields the fields (columns) to ignore/skip.
	IgnoreFields []string
	// IgnoreTypes similar to IgnoreFields, this will ignore the fields contained in the type. Useful for ignoring embedded
	// types and their fields in the target model
	IgnoreTypes []interface{}
	// if Nullable is set to be true, only the string `NULL` will be taken as NULL
	Nullable bool
}

// NewDataFlowTester create a *DataFlowTester to help developer test their subtasks data flow
func NewDataFlowTester(t *testing.T, pluginName string, pluginMeta plugin.PluginMeta) *DataFlowTester {
	err := plugin.RegisterPlugin(pluginName, pluginMeta)
	if err != nil {
		panic(err)
	}
	cfg := config.GetConfig()
	e2eDbUrl := cfg.GetString(`E2E_DB_URL`)
	if e2eDbUrl == `` {
		panic(errors.Default.New(`e2e can only run with E2E_DB_URL, please set it in environment variable or .env file`))
	}
	cfg.Set(`DB_URL`, cfg.GetString(`E2E_DB_URL`))
	db, err := runner.NewGormDb(cfg, logruslog.Global)
	if err != nil {
		// if here fail with error `access denied for user` you need to create database by your self as follow command
		// create databases lake_test;
		// grant all on lake_test.* to 'merico'@'%';
		panic(err)
	}
	errors.Must(db.AutoMigrate(&models.SubtaskState{}))
	df := &DataFlowTester{
		Cfg:    cfg,
		Db:     db,
		Dal:    dalgorm.NewDalgorm(db),
		T:      t,
		Name:   pluginName,
		Plugin: pluginMeta,
		Log:    logruslog.Global,
	}
	return df
}

// ImportCsvIntoRawTable imports records from specified csv file into target raw table, note that existing data would be deleted first.
func (t *DataFlowTester) ImportCsvIntoRawTable(csvRelPath string, rawTableName string) {
	csvIter, err := pluginhelper.NewCsvFileIterator(csvRelPath)
	if err != nil {
		panic(err)
	}
	defer csvIter.Close()
	t.FlushRawTable(rawTableName)
	// load rows and insert into target table
	for csvIter.HasNext() {
		toInsertValues := csvIter.Fetch()
		toInsertValues[`data`] = []byte(toInsertValues[`data`].(string))
		result := t.Db.Table(rawTableName).Create(toInsertValues)
		if result.Error != nil {
			panic(result.Error)
		}
		assert.Equal(t.T, int64(1), result.RowsAffected)
	}
}

func (t *DataFlowTester) importCsv(csvRelPath string, dst schema.Tabler, nullable bool) {
	csvIter, err := pluginhelper.NewCsvFileIterator(csvRelPath)
	if err != nil {
		panic(err)
	}
	defer csvIter.Close()
	t.FlushTabler(dst)
	// load rows and insert into target table
	for csvIter.HasNext() {
		toInsertValues := csvIter.Fetch()
		for i := range toInsertValues {
			if nullable {
				if toInsertValues[i].(string) == `NULL` {
					toInsertValues[i] = nil
				}
			} else {
				if toInsertValues[i].(string) == `` {
					toInsertValues[i] = nil
				}
			}
		}
		result := t.Db.Model(dst).Create(toInsertValues)
		if result.Error != nil {
			panic(result.Error)
		}
		assert.Equal(t.T, int64(1), result.RowsAffected)
	}
}

// ImportCsvIntoTabler imports records from specified csv file into target tabler, the empty string will be taken as NULL. note that existing data would be deleted first.
func (t *DataFlowTester) ImportCsvIntoTabler(csvRelPath string, dst schema.Tabler) {
	t.importCsv(csvRelPath, dst, false)
}

// ImportNullableCsvIntoTabler imports records from specified csv file into target tabler, the `NULL` will be taken as NULL. note that existing data would be deleted first.
func (t *DataFlowTester) ImportNullableCsvIntoTabler(csvRelPath string, dst schema.Tabler) {
	t.importCsv(csvRelPath, dst, true)
}

// FlushRawTable migrate table and deletes all records from specified table
func (t *DataFlowTester) FlushRawTable(rawTableName string) {
	// flush target table
	err := t.Db.Migrator().DropTable(rawTableName)
	if err != nil {
		panic(err)
	}
	err = t.Db.Table(rawTableName).AutoMigrate(&api.RawData{})
	if err != nil {
		panic(err)
	}
}

// FlushTabler migrate table and deletes all records from specified table
func (t *DataFlowTester) FlushTabler(dst schema.Tabler) {
	// flush target table
	err := t.Db.Migrator().DropTable(dst)
	if err != nil {
		panic(err)
	}
	err = t.Db.AutoMigrate(dst)
	if err != nil {
		panic(err)
	}
}

// Subtask executes specified subtasks
func (t *DataFlowTester) Subtask(subtaskMeta plugin.SubTaskMeta, taskData interface{}) {
	t.FlushTabler(&models.SubtaskState{})
	subtaskCtx := t.SubtaskContext(taskData)
	err := subtaskMeta.EntryPoint(subtaskCtx)
	if err != nil {
		panic(err)
	}
}

// SubtaskContext creates a subtask context
func (t *DataFlowTester) SubtaskContext(taskData interface{}) plugin.SubTaskContext {
	syncPolicy := &models.SyncPolicy{
		TriggerSyncPolicy: models.TriggerSyncPolicy{
			FullSync: true,
		},
	}
	return contextimpl.NewStandaloneSubTaskContext(context.Background(), runner.CreateBasicRes(t.Cfg, t.Log, t.Db), t.Name, taskData, t.Name, syncPolicy)
}

func filterColumn(column dal.ColumnMeta, opts TableOptions) bool {
	for _, ignore := range opts.IgnoreFields {
		if column.Name() == ignore {
			return false
		}
	}
	if len(opts.TargetFields) == 0 {
		return true
	}
	targetFound := false
	for _, target := range opts.TargetFields {
		if column.Name() == target {
			targetFound = true
			break
		}
	}
	return targetFound
}

// UpdateCSVs scans the supplied directory for .csv files and applies the `updater` function to each. It saves back the result.
// This is useful when you need to bulk-change a lot of entries across many CSV files in a systematic way.
func (t *DataFlowTester) UpdateCSVs(csvDir string, updater func(fileName string, record *pluginhelper.CsvRecord)) errors.Error {
	var csvFiles []string
	err := filepath.WalkDir(csvDir, func(file string, dir fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(file) == ".csv" {
			csvFiles = append(csvFiles, file)
		}
		return nil
	})
	if err != nil {
		return errors.Convert(err)
	}
	for _, csvFile := range csvFiles {
		csvIter, err := pluginhelper.NewCsvFileIterator(csvFile)
		if err != nil {
			return errors.Convert(err)
		}
		columns := csvIter.GetColumns()
		var entries [][]string
		func() {
			defer csvIter.Close()
			for csvIter.HasNext() {
				records := csvIter.FetchRecords()
				var csvEntry []string
				for _, record := range records {
					updater(filepath.Base(csvFile), record)
					csvEntry = append(csvEntry, cast.ToString(record.Value))
				}
				entries = append(entries, csvEntry)
			}
		}()
		if len(entries) == 0 {
			continue
		}
		func() {
			csvWriter, _ := pluginhelper.NewCsvFileWriter(csvFile, columns)
			defer csvWriter.Close()
			for _, entry := range entries {
				csvWriter.Write(entry)
			}
			csvWriter.Flush()
		}()
	}
	return nil
}

// CreateSnapshot reads rows from database and write them into .csv file.
func (t *DataFlowTester) CreateSnapshot(dst schema.Tabler, opts TableOptions) {
	location, _ := time.LoadLocation(`UTC`)

	targetFields := t.resolveTargetFields(dst, opts)
	pkColumnNames, err := dal.GetPrimarykeyColumnNames(t.Dal, dst)
	if err != nil {
		panic(err)
	}
	for i := 0; i < len(pkColumnNames); i++ {
		group := strings.Split(pkColumnNames[i], ".")
		if len(group) > 1 {
			pkColumnNames[i] = group[len(group)-1]
		}
	}
	allFields := append(pkColumnNames, targetFields...)
	allFields = utils.StringsUniq(allFields)
	dbCursor, err := t.Dal.Cursor(
		dal.Select(strings.Join(allFields, `,`)),
		dal.From(dst.TableName()),
		dal.Orderby(strings.Join(pkColumnNames, `,`)),
	)
	if err != nil {
		panic(errors.Default.Wrap(err, fmt.Sprintf("unable to run select query on table %s", dst.TableName())))
	}

	columns, err := errors.Convert01(dbCursor.Columns())
	if err != nil {
		panic(errors.Default.Wrap(err, fmt.Sprintf("unable to get columns from table %s", dst.TableName())))
	}
	csvWriter, _ := pluginhelper.NewCsvFileWriter(opts.CSVRelPath, columns)
	defer csvWriter.Close()

	// define how to scan value
	columnTypes, _ := dbCursor.ColumnTypes()
	forScanValues := make([]interface{}, len(allFields))
	for i, columnType := range columnTypes {
		if columnType.ScanType().Name() == `Time` || columnType.ScanType().Name() == `NullTime` {
			forScanValues[i] = new(sql.NullTime)
		} else if columnType.ScanType().Name() == `bool` {
			forScanValues[i] = new(bool)
		} else if columnType.ScanType().Name() == `RawBytes` {
			forScanValues[i] = new(sql.NullString)
		} else if columnType.ScanType().Name() == `NullInt64` {
			forScanValues[i] = new(sql.NullInt64)
		} else {
			forScanValues[i] = new(sql.NullString)
		}
	}

	for dbCursor.Next() {
		err = errors.Convert(dbCursor.Scan(forScanValues...))
		if err != nil {
			panic(errors.Default.Wrap(err, fmt.Sprintf("unable to scan row on table %s: %v", dst.TableName(), err)))
		}
		values := make([]string, len(allFields))
		for i := range forScanValues {
			switch forScanValues[i].(type) {
			case *sql.NullTime:
				value := forScanValues[i].(*sql.NullTime)
				if value.Valid {
					values[i] = value.Time.In(location).Format("2006-01-02T15:04:05.000-07:00")
				} else {
					if opts.Nullable {
						values[i] = "NULL"
					} else {
						values[i] = ""
					}
				}
			case *bool:
				if *forScanValues[i].(*bool) {
					values[i] = `1`
				} else {
					values[i] = `0`
				}
			case *sql.NullString:
				value := *forScanValues[i].(*sql.NullString)
				if value.Valid {
					values[i] = value.String
				} else {
					if opts.Nullable {
						values[i] = "NULL"
					} else {
						values[i] = ""
					}
				}
			case *sql.NullInt64:
				value := *forScanValues[i].(*sql.NullInt64)
				if value.Valid {
					values[i] = strconv.FormatInt(value.Int64, 10)
				} else {
					if opts.Nullable {
						values[i] = "NULL"
					} else {
						values[i] = ""
					}
				}
			case *string:
				values[i] = fmt.Sprint(*forScanValues[i].(*string))
			}
		}
		csvWriter.Write(values)
	}
	fmt.Printf("created CSV file: %s\n", opts.CSVRelPath)
}

// ExportRawTable reads rows from raw table and write them into .csv file.
func (t *DataFlowTester) ExportRawTable(rawTableName string, csvRelPath string) {
	location, _ := time.LoadLocation(`UTC`)
	allFields := []string{`id`, `params`, `data`, `url`, `input`, `created_at`}
	rawRows := &[]api.RawData{}
	err := t.Dal.All(
		rawRows,
		dal.Select(`id, params, data, url, input, created_at`),
		dal.From(rawTableName),
		dal.Orderby(`id`),
	)
	if err != nil {
		panic(err)
	}

	csvWriter, _ := pluginhelper.NewCsvFileWriter(csvRelPath, allFields)
	defer csvWriter.Close()

	for _, rawRow := range *rawRows {
		csvWriter.Write([]string{
			strconv.FormatUint(rawRow.ID, 10),
			rawRow.Params,
			string(rawRow.Data),
			rawRow.Url,
			string(rawRow.Input),
			rawRow.CreatedAt.In(location).Format("2006-01-02T15:04:05.000-07:00"),
		})
	}
}

func formatDbValue(value interface{}, nullable bool) string {
	if nullable && value == nil {
		return "NULL"
	}
	location, _ := time.LoadLocation(`UTC`)
	switch value := value.(type) {
	case time.Time:
		return value.In(location).Format("2006-01-02T15:04:05.000-07:00")
	case bool:
		if value {
			return `1`
		} else {
			return `0`
		}
	default:
		if value != nil {
			return fmt.Sprint(value)
		}
	}
	return ``
}

// ColumnWithRawData create a Column string with _raw_data_* appending
func ColumnWithRawData(column ...string) []string {
	return append(
		column,
		"_raw_data_params",
		"_raw_data_table",
		"_raw_data_id",
		"_raw_data_remark",
	)
}

// VerifyTableWithRawData use VerifyTable and append the _raw_data_* checking after targetFields
func (t *DataFlowTester) VerifyTableWithRawData(dst schema.Tabler, csvRelPath string, targetFields []string) {
	t.VerifyTable(dst, csvRelPath, ColumnWithRawData(targetFields...))
}

// VerifyTable reads rows from csv file and compare with records from database one by one. You must specify the
// Primary Key Fields with `pkFields` so DataFlowTester could select the exact record from database, as well as which
// fields to compare with by specifying `targetFields` parameter. Leaving `targetFields` empty/nil will compare all fields.
func (t *DataFlowTester) VerifyTable(dst schema.Tabler, csvRelPath string, targetFields []string) {
	t.VerifyTableWithOptions(dst, TableOptions{
		CSVRelPath:   csvRelPath,
		TargetFields: targetFields,
	})
}

func (t *DataFlowTester) extractColumns(ifc interface{}) []string {
	sch, err := schema.Parse(ifc, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		panic(fmt.Sprintf("error getting object schema: %v", err))
	}
	var columns []string
	for _, f := range sch.Fields {
		columns = append(columns, f.DBName)
	}
	return columns
}

func (t *DataFlowTester) resolveTargetFields(dst schema.Tabler, opts TableOptions) []string {
	for _, ignore := range opts.IgnoreTypes {
		opts.IgnoreFields = append(opts.IgnoreFields, t.extractColumns(ignore)...)
	}
	var targetFields []string
	if len(opts.TargetFields) == 0 || len(opts.IgnoreFields) > 0 {
		names, err := dal.GetColumnNames(t.Dal, dst, func(cm dal.ColumnMeta) bool {
			return filterColumn(cm, opts)
		})
		if err != nil {
			panic(err)
		}
		targetFields = append(targetFields, names...)
	} else {
		targetFields = opts.TargetFields
	}
	return targetFields
}

// VerifyTableWithOptions extends VerifyTable and allows for more advanced usages using TableOptions
func (t *DataFlowTester) VerifyTableWithOptions(dst schema.Tabler, opts TableOptions) {
	if opts.CSVRelPath == "" {
		panic("CSV relative path missing")
	}
	_, err := os.Stat(opts.CSVRelPath)
	if os.IsNotExist(err) {
		t.CreateSnapshot(dst, opts)
		return
	}

	targetFields := t.resolveTargetFields(dst, opts)
	pkColumns, err := dal.GetPrimarykeyColumns(t.Dal, dst)
	if err != nil {
		panic(err)
	}

	csvIter, _ := pluginhelper.NewCsvFileIterator(opts.CSVRelPath)
	defer csvIter.Close()

	var expectedRowTotal int64
	csvMap := map[string]map[string]interface{}{}
	for csvIter.HasNext() {
		expectedRowTotal++
		expected := csvIter.Fetch()
		pkValues := make([]string, 0, len(pkColumns))
		for _, pkc := range pkColumns {
			pkValues = append(pkValues, expected[pkc.Name()].(string))
		}
		pkValueStr := strings.Join(pkValues, `-`)
		_, ok := csvMap[pkValueStr]
		assert.False(t.T, ok, fmt.Sprintf(`%s duplicated in csv (with params from csv %s)`, dst.TableName(), pkValues))
		for _, ignore := range opts.IgnoreFields {
			delete(expected, ignore)
		}
		csvMap[pkValueStr] = expected
	}

	var actualRowTotal int64
	dbRows := &[]map[string]interface{}{}
	err = t.Db.Table(dst.TableName()).Find(dbRows).Error
	if err != nil {
		panic(err)
	}
	for _, actual := range *dbRows {
		actualRowTotal++
		pkValues := make([]string, 0, len(pkColumns))
		for _, pkc := range pkColumns {
			pkValues = append(pkValues, formatDbValue(actual[pkc.Name()], opts.Nullable))
		}
		pk := strings.Join(pkValues, `-`)
		expected, ok := csvMap[pk]
		assert.True(t.T, ok, fmt.Sprintf(`record in table %s not found (with params from csv %s)`, dst.TableName(), pk))
		if !ok {
			continue
		}
		for _, field := range targetFields {
			expectation := formatDbValue(expected[field], opts.Nullable)
			reality := formatDbValue(actual[field], opts.Nullable)
			if !assert.Equal(t.T, expectation, reality, fmt.Sprintf(`%s.%s not match (with params from csv %s)`, dst.TableName(), field, pkValues)) {
				_ = t.T // useful for debugging
			}
		}
	}

	assert.Equal(t.T, expectedRowTotal, actualRowTotal, fmt.Sprintf(`records in %s count not match expectation count,[expected(CSV):%d][actual(DB):%d]`, dst.TableName(), expectedRowTotal, actualRowTotal))
}

// DuplicateRecords duplicates all records in the specified table and change the specified fields to the specified values
func DuplicateRecords[T dal.Tabler](t *DataFlowTester, fields map[string]interface{}) {
	all := make([]*T, 0)
	errors.Must(t.Dal.All(&all))
	for _, row := range all {
		val := reflect.ValueOf(row)
		for k, v := range fields {
			if f, ok := v.(func() interface{}); ok {
				v = f()
			}
			f1 := val.Elem().FieldByName(k)
			f1.Set(reflect.ValueOf(v))
		}
	}
	errors.Must(t.Dal.Create(all))
}
