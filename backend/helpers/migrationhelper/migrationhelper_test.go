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
	"crypto/sha256"
	"encoding/hex"
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/unithelper"
	mockcontext "github.com/apache/incubator-devlake/mocks/core/context"
	mockdal "github.com/apache/incubator-devlake/mocks/core/dal"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const TestTableNameSrc string = "test_src"
const TestTableNameDst string = "test_dst"
const TestColumeName string = "id"

type TestSrcTable struct {
	Id        string `gorm:"type:varchar(255)"`
	Name      string `gorm:"type:varchar(255)"`
	CommitSha string `gorm:"type:varchar(40)"`
}

type TestDstTable struct {
	Id        string `gorm:"type:varchar(255)"`
	Name      string `gorm:"type:text"`
	CommitSha string `gorm:"type:varchar(40)"`
}

var _ plugin.MigrationScript = (*TestScript)(nil)

type TestScript struct{}

func (*TestScript) Up(basicRes context.BasicRes) errors.Error {
	return nil
}

func (*TestScript) Version() uint64 {
	return 110100100116102
}

func (*TestScript) Name() string {
	return "This is only the test script for unit test"
}

var TestError = errors.Default.New("TestError")

func TestTransformTable(t *testing.T) {
	mockRows := new(mockdal.Rows)
	mockRows.On("Next").Return(true).Times(3)
	mockRows.On("Next").Return(false).Once()
	mockRows.On("Close").Return(nil).Twice()

	mockDal := new(mockdal.Dal)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil).Once()
	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return(
		[]reflect.StructField{
			{Name: "Id", Type: reflect.TypeOf("")},
		},
	)
	// create the test data
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test1"
		dst.CommitSha = "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test2"
		dst.CommitSha = "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test3"
		dst.CommitSha = "57ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()

	// checking if it Create Drop and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("DropTables", mock.Anything).Run(func(args mock.Arguments) {
		tmpname, ok := args.Get(0).([]interface{})[0].(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, TestTableNameSrc, tmpname)
	}).Return(nil).Once()
	mockDal.On("RenameTable", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		oldname, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, oldname)
		tmpname, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, oldname, tmpname)
	}).Return(nil).Once()

	// checking the test data
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dts := args.Get(0).([]*TestDstTable)
		assert.Equal(t, dts[0].Name, "test1")
		assert.Equal(t, dts[0].CommitSha, "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[0].Id, "1b4f0e9851971998e732078544c96b36c3d01cedf7caa332359d6f1d8356701485d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[1].Name, "test2")
		assert.Equal(t, dts[1].CommitSha, "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[1].Id, "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c75285d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[2].Name, "test3")
		assert.Equal(t, dts[2].CommitSha, "57ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307")
		assert.Equal(t, dts[2].Id, "fd61a03af4f77d870fc21e05e7e80678095c92d808cfb3b5c279ee04c74aca1357ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307")
	}).Return(nil).Once()

	// for Primarykey  autoincrement checking
	mockDal.On("GetColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName := args.Get(0).(dal.Tabler).TableName()
		assert.Equal(t, tableName, TestTableNameSrc)
	}).Return([]dal.ColumnMeta{}, nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := TransformTable(mockRes, &TestScript{}, TestTableNameSrc,
		func(src *TestSrcTable) (*TestDstTable, errors.Error) {
			shaName := sha256.New()
			shaName.Write([]byte(src.Name))
			return &TestDstTable{
				Id:        hex.EncodeToString(shaName.Sum(nil)) + src.CommitSha,
				Name:      src.Name,
				CommitSha: src.CommitSha,
			}, nil
		})

	assert.Nil(t, err)
}

func TestTransformTable_RollBack(t *testing.T) {
	mockRows := new(mockdal.Rows)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Close").Return(nil).Twice()

	mockDal := new(mockdal.Dal)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil).Once()
	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return(
		[]reflect.StructField{
			{Name: "Id", Type: reflect.TypeOf("")},
		},
	)

	// retruen the error when fetch for rollback
	mockDal.On("Fetch", mock.Anything, mock.Anything).Return(TestError).Once()

	// checking if it AutoMigrate and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("RenameTable", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		oldname, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, oldname)
		tmpname, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, oldname, tmpname)
	}).Return(nil).Once()

	// checking if Rename and Drop RollBack working with right table
	mockDal.On("RenameTable", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tmpname, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, TestTableNameSrc, tmpname)
		oldname, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, oldname, TestTableNameSrc)
	}).Return(nil).Once()
	mockDal.On("DropTables", mock.Anything).Run(func(args mock.Arguments) {
		oldname, ok := args.Get(0).([]interface{})[0].(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, oldname, TestTableNameSrc)
	}).Return(nil).Once()

	// for Primarykey  autoincrement checking
	mockDal.On("GetColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName := args.Get(0).(dal.Tabler).TableName()
		assert.Equal(t, tableName, TestTableNameSrc)
	}).Return([]dal.ColumnMeta{}, nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := TransformTable(mockRes, &TestScript{}, TestTableNameSrc,
		func(src *TestSrcTable) (*TestDstTable, errors.Error) {
			shaName := sha256.New()
			shaName.Write([]byte(src.Name))
			return &TestDstTable{
				Id:        hex.EncodeToString(shaName.Sum(nil)) + src.CommitSha,
				Name:      src.Name,
				CommitSha: src.CommitSha,
			}, nil
		})

	assert.Contains(t, err.Unwrap().Error(), TestError.Unwrap().Error())
}

func TestCopyTableColumns(t *testing.T) {
	mockRows := new(mockdal.Rows)

	mockRows.On("Next").Return(true).Times(3)
	mockRows.On("Next").Return(false).Once()
	mockRows.On("Close").Return(nil).Twice()

	mockDal := new(mockdal.Dal)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil).Once()
	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return(
		[]reflect.StructField{
			{Name: "Id", Type: reflect.TypeOf("")},
		},
	)
	// create the test data
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test1"
		dst.CommitSha = "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test2"
		dst.CommitSha = "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test3"
		dst.CommitSha = "57ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()

	// checking if it Create Drop and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("DropTables", mock.Anything).Run(func(args mock.Arguments) {
		tmpname, ok := args.Get(0).([]interface{})[0].(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, TestTableNameSrc, tmpname)
	}).Return(nil).Once()
	mockDal.On("RenameTable", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		oldname, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, oldname)
		tmpname, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, oldname, tmpname)
	}).Return(nil).Once()

	// checking the test data
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dts := args.Get(0).([]*TestDstTable)
		assert.Equal(t, dts[0].Name, "test1")
		assert.Equal(t, dts[0].CommitSha, "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[0].Id, "1b4f0e9851971998e732078544c96b36c3d01cedf7caa332359d6f1d8356701485d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[1].Name, "test2")
		assert.Equal(t, dts[1].CommitSha, "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[1].Id, "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c75285d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[2].Name, "test3")
		assert.Equal(t, dts[2].CommitSha, "57ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307")
		assert.Equal(t, dts[2].Id, "fd61a03af4f77d870fc21e05e7e80678095c92d808cfb3b5c279ee04c74aca1357ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307")
	}).Return(nil).Once()

	// for Primarykey  autoincrement checking
	mockDal.On("GetColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName := args.Get(0).(dal.Tabler).TableName()
		assert.Equal(t, tableName, TestTableNameSrc)
	}).Return([]dal.ColumnMeta{}, nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := CopyTableColumns(mockRes, TestTableNameSrc, TestTableNameDst,
		func(src *TestSrcTable) (*TestDstTable, errors.Error) {
			shaName := sha256.New()
			shaName.Write([]byte(src.Name))
			return &TestDstTable{
				Id:        hex.EncodeToString(shaName.Sum(nil)) + src.CommitSha,
				Name:      src.Name,
				CommitSha: src.CommitSha,
			}, nil
		})

	assert.Nil(t, err)
}

func TestCopyTableColumns_RollBack(t *testing.T) {
	mockRows := new(mockdal.Rows)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Close").Return(nil).Twice()

	mockDal := new(mockdal.Dal)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil).Once()
	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return(
		[]reflect.StructField{
			{Name: "Id", Type: reflect.TypeOf("")},
		},
	)

	// retruen the error when fetch for rollback
	mockDal.On("Fetch", mock.Anything, mock.Anything).Return(TestError).Once()

	// checking if it AutoMigrate and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("RenameTable", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		oldname, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, oldname)
		tmpname, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, oldname, tmpname)
	}).Return(nil).Once()

	// checking if Rename and Drop RollBack working with right table
	mockDal.On("RenameTable", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tmpname, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, TestTableNameSrc, tmpname)
		oldname, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, oldname, TestTableNameSrc)
	}).Return(nil).Once()
	mockDal.On("DropTables", mock.Anything).Run(func(args mock.Arguments) {
		oldname, ok := args.Get(0).([]interface{})[0].(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, oldname, TestTableNameSrc)
	}).Return(nil).Once()

	// for Primarykey  autoincrement checking
	mockDal.On("GetColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName := args.Get(0).(dal.Tabler).TableName()
		assert.Equal(t, tableName, TestTableNameSrc)
	}).Return([]dal.ColumnMeta{}, nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := CopyTableColumns(mockRes, TestTableNameSrc, TestTableNameDst,
		func(src *TestSrcTable) (*TestDstTable, errors.Error) {
			shaName := sha256.New()
			shaName.Write([]byte(src.Name))
			return &TestDstTable{
				Id:        hex.EncodeToString(shaName.Sum(nil)) + src.CommitSha,
				Name:      src.Name,
				CommitSha: src.CommitSha,
			}, nil
		})

	assert.Contains(t, err.Unwrap().Error(), TestError.Unwrap().Error())
}

func TestTransformColumns(t *testing.T) {
	mockRows := new(mockdal.Rows)
	mockRows.On("Next").Return(true).Times(3)
	mockRows.On("Next").Return(false).Once()
	mockRows.On("Close").Return(nil).Twice()

	mockDal := new(mockdal.Dal)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil).Once()
	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return(
		[]reflect.StructField{
			{Name: "Id", Type: reflect.TypeOf("")},
		},
	)
	// create the test data
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test1"
		dst.CommitSha = "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test2"
		dst.CommitSha = "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()
	mockDal.On("Fetch", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*TestSrcTable)
		dst.Name = "test3"
		dst.CommitSha = "57ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307"
		dst.Id = dst.Name + dst.CommitSha
	}).Return(nil).Once()

	// checking if it Create Drop and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("DropColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, tableName)
		tmpcolumnNames, ok := args.Get(1).([]string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, TestColumeName, tmpcolumnNames[0])
	}).Return(nil).Once()
	mockDal.On("RenameColumn", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, tableName, TestTableNameSrc)
		columnName, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, columnName, TestColumeName)
		tmpColumnName, ok := args.Get(2).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, columnName, tmpColumnName)
	}).Return(nil).Once()

	// checking the test data
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dts := args.Get(0).([]*TestDstTable)
		assert.Equal(t, dts[0].Name, "test1")
		assert.Equal(t, dts[0].CommitSha, "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[0].Id, "1b4f0e9851971998e732078544c96b36c3d01cedf7caa332359d6f1d8356701485d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[1].Name, "test2")
		assert.Equal(t, dts[1].CommitSha, "85d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[1].Id, "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c75285d898dab1984d744f99a3a9127aefd43632e000f3ef48c29d0c5b043cf251ed")
		assert.Equal(t, dts[2].Name, "test3")
		assert.Equal(t, dts[2].CommitSha, "57ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307")
		assert.Equal(t, dts[2].Id, "fd61a03af4f77d870fc21e05e7e80678095c92d808cfb3b5c279ee04c74aca1357ef3d346f24f386216563752b0c447a35c041e0b7143f929dc4de27742e3307")
	}).Return(nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := TransformColumns(mockRes, &TestScript{}, TestTableNameSrc,
		[]string{
			TestColumeName,
		},
		func(src *TestSrcTable) (*TestDstTable, errors.Error) {
			shaName := sha256.New()
			shaName.Write([]byte(src.Name))
			return &TestDstTable{
				Id:        hex.EncodeToString(shaName.Sum(nil)) + src.CommitSha,
				Name:      src.Name,
				CommitSha: src.CommitSha,
			}, nil
		})

	assert.Nil(t, err)
}

func TestTransformColumns_RollBack(t *testing.T) {
	mockRows := new(mockdal.Rows)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Close").Return(nil).Twice()

	mockDal := new(mockdal.Dal)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil).Once()
	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return(
		[]reflect.StructField{
			{Name: "Id", Type: reflect.TypeOf("")},
		},
	)

	// retruen the error when fetch for rollback
	mockDal.On("Fetch", mock.Anything, mock.Anything).Return(TestError).Once()

	// checking if it AutoMigrate and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("RenameColumn", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, tableName, TestTableNameSrc)
		columnName, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, columnName, TestColumeName)
		tmpColumnName, ok := args.Get(2).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, columnName, tmpColumnName)
	}).Return(nil).Once()

	// checking if Rename and Drop RollBack working with right table
	mockDal.On("RenameColumn", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, tableName, TestTableNameSrc)
		columnName, ok := args.Get(2).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, columnName, TestColumeName)
		tmpColumnName, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, columnName, tmpColumnName)
	}).Return(nil).Once()
	mockDal.On("DropColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, tableName)
		tmpcolumnNames, ok := args.Get(1).([]string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestColumeName, tmpcolumnNames[0])
	}).Return(nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := TransformColumns(mockRes, &TestScript{}, TestTableNameSrc,
		[]string{
			TestColumeName,
		},
		func(src *TestSrcTable) (*TestDstTable, errors.Error) {
			shaName := sha256.New()
			shaName.Write([]byte(src.Name))
			return &TestDstTable{
				Id:        hex.EncodeToString(shaName.Sum(nil)) + src.CommitSha,
				Name:      src.Name,
				CommitSha: src.CommitSha,
			}, nil
		})

	assert.Contains(t, err.Unwrap().Error(), TestError.Unwrap().Error())
}

func TestChangeColumnsType(t *testing.T) {
	mockDal := new(mockdal.Dal)

	// checking if it Create Drop and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("DropColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, tableName)
		tmpcolumnNames, ok := args.Get(1).([]string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, TestColumeName, tmpcolumnNames[0])
	}).Return(nil).Once()
	mockDal.On("RenameColumn", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, tableName, TestTableNameSrc)
		columnName, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, columnName, TestColumeName)
		tmpColumnName, ok := args.Get(2).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, columnName, tmpColumnName)
	}).Return(nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := ChangeColumnsType[TestDstTable](mockRes, &TestScript{}, TestTableNameSrc,
		[]string{
			TestColumeName,
		},
		func(tmpColumnParams []interface{}) errors.Error {
			assert.Equal(t, len(tmpColumnParams), 1)
			cp, ok := (tmpColumnParams[0]).(dal.ClauseColumn)
			assert.Equal(t, ok, true)
			assert.NotEqual(t, TestColumeName, cp.Name)
			return nil
		})

	assert.Nil(t, err)
}

func TestChangeColumnsType_Rollback(t *testing.T) {
	mockDal := new(mockdal.Dal)

	// checking if it AutoMigrate and Rename the right table
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_, ok := args.Get(0).(*TestDstTable)
		assert.Equal(t, ok, true)
	}).Return(nil).Once()
	mockDal.On("RenameColumn", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, tableName, TestTableNameSrc)
		columnName, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, columnName, TestColumeName)
		tmpColumnName, ok := args.Get(2).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, columnName, tmpColumnName)
	}).Return(nil).Once()

	// checking if Rename and Drop RollBack working with right table
	mockDal.On("RenameColumn", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, tableName, TestTableNameSrc)
		columnName, ok := args.Get(2).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, columnName, TestColumeName)
		tmpColumnName, ok := args.Get(1).(string)
		assert.Equal(t, ok, true)
		assert.NotEqual(t, columnName, tmpColumnName)
	}).Return(nil).Once()
	mockDal.On("DropColumns", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		tableName, ok := args.Get(0).(string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestTableNameSrc, tableName)
		tmpcolumnNames, ok := args.Get(1).([]string)
		assert.Equal(t, ok, true)
		assert.Equal(t, TestColumeName, tmpcolumnNames[0])
	}).Return(nil).Once()

	mockLog := unithelper.DummyLogger()
	mockRes := new(mockcontext.BasicRes)

	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(mockLog)

	err := ChangeColumnsType[TestDstTable](mockRes, &TestScript{}, TestTableNameSrc,
		[]string{
			TestColumeName,
		},
		func(tmpColumnParams []interface{}) errors.Error {
			return TestError
		})

	assert.Contains(t, err.Unwrap().Error(), TestError.Unwrap().Error())
}
