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

package plugininfo

import (
	"fmt"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/domaininfo"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	"github.com/apache/devlake/server/api/shared"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/schema"
	"net/http"
	"reflect"
	"sync"
)

type SubTaskMeta struct {
	Name             string   `json:"name"`
	Required         bool     `json:"required"`
	EnabledByDefault bool     `json:"enabled_by_default"`
	Description      string   `json:"description"`
	DomainTypes      []string `json:"domain_types"`
}

func CreateSubTaskMeta(subTaskMeta []plugin.SubTaskMeta) []SubTaskMeta {
	ret := make([]SubTaskMeta, 0, len(subTaskMeta))
	for _, meta := range subTaskMeta {
		ret = append(ret, SubTaskMeta{
			Name:             meta.Name,
			Required:         meta.Required,
			EnabledByDefault: meta.EnabledByDefault,
			Description:      meta.Description,
			DomainTypes:      meta.DomainTypes,
		})
	}
	return ret
}

type TableInfo struct {
	Name         string `json:"name"`
	Tags         string `json:"tags"`
	DbName       string `json:"db_name"`
	DataType     string `json:"data_type"`
	GORMDataType string `json:"gorm_data_type"`
}

type TableInfos struct {
	TableName string       `json:"table_name"`
	Field     []*TableInfo `json:"field"`
	Error     *string      `json:"error"`
}

type PluginMetric struct {
	RequiredDataEntities []map[string]interface{} `json:"requiredDataEntities"`
	RunAfter             []string                 `json:"runAfter"`
	IsProjectMetric      bool                     `json:"isProjectMetric"`
}

type PluginMeta struct {
	Plugin string       `json:"plugin"`
	Metric PluginMetric `json:"metric"`
}

type PluginMetas []PluginMeta

func NewTableInfos(table dal.Tabler) *TableInfos {
	tableInfos := &TableInfos{
		TableName: table.TableName(),
		Error:     nil,
	}

	fieldInfos := utils.WalkFields(reflect.TypeOf(table), nil)
	schema, err := schema.Parse(table, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		errstr := err.Error()
		tableInfos.Error = &errstr
	}

	tableInfos.Field = make([]*TableInfo, 0, len(fieldInfos))
	for _, field := range fieldInfos {
		dbName := ""
		dataType := ""
		gormDataType := ""
		if schema != nil {
			if dbfield, ok := schema.FieldsByName[field.Name]; ok {
				dbName = dbfield.DBName
				dataType = string(dbfield.DataType)
				gormDataType = string(dbfield.GORMDataType)
			}
		}
		tableInfos.Field = append(tableInfos.Field, &TableInfo{
			Name:         field.Name,
			Tags:         string(field.Tag),
			DbName:       dbName,
			DataType:     dataType,
			GORMDataType: gormDataType,
		})
	}

	return tableInfos
}

type PluginInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Tables      []*TableInfos `json:"tables"`
	TaskMeta    []SubTaskMeta `json:"task_mata"`
}

func NewPluginInfo() *PluginInfo {
	return &PluginInfo{
		Tables: make([]*TableInfos, 0),
	}
}

type TotalInfo struct {
	DomainInfos []*TableInfos
	PluginInfos []*PluginInfo
}

func NewTotalInfo() *TotalInfo {
	return &TotalInfo{
		DomainInfos: make([]*TableInfos, 0),
		PluginInfos: make([]*PluginInfo, 0),
	}
}

// @Get detail info of plugins
// @Description GET /plugininfo
// @Description RETURN SAMPLE
// @Tags framework/plugininfo
// @Success 200
// @Failure 400  {string} errcode.Error "Bad Request"
// @Router /plugininfo [get]
func Get(c *gin.Context) {
	info := NewTotalInfo()

	// set the domain layer tables info.
	domaininfo := domaininfo.GetDomainTablesInfo()
	for _, table := range domaininfo {
		tableInfo := NewTableInfos(table)
		info.DomainInfos = append(info.DomainInfos, tableInfo)
	}

	// plugin info
	err := plugin.TraversalPlugin(func(name string, p plugin.PluginMeta) errors.Error {
		infoPlugin := NewPluginInfo()
		info.PluginInfos = append(info.PluginInfos, infoPlugin)

		// plugin name and description
		infoPlugin.Name = name
		infoPlugin.Description = p.Description()

		// if this plugin has the plugin task info
		if pt, ok := p.(plugin.PluginTask); ok {
			infoPlugin.TaskMeta = CreateSubTaskMeta(pt.SubTaskMetas())
		}

		// if this plugin has the plugin model info
		if pm, ok := p.(plugin.PluginModel); ok {
			tables := pm.GetTablesInfo()
			for _, table := range tables {
				TableInfos := NewTableInfos(table)
				infoPlugin.Tables = append(infoPlugin.Tables, TableInfos)
			}
		}

		return nil
	})

	if err != nil {
		shared.ApiOutputError(c, errors.Default.Wrap(err, "error getting plugin info of plugins"))
	}

	shared.ApiOutputSuccess(c, info, http.StatusOK)
}

// @Get name list of plugins
// @Description GET /plugins
// @Description RETURN SAMPLE
// @Tags framework/plugins
// @Success 200  {object} PluginMetas
// @Failure 400  {string} errcode.Error "Bad Request"
// @Router /plugins [get]
func GetPluginMetas(c *gin.Context) {
	var metas PluginMetas
	err := plugin.TraversalPlugin(func(name string, p plugin.PluginMeta) errors.Error {
		pluginMeta := PluginMeta{
			Plugin: name,
		}

		// if this plugin has the plugin task info
		if pt, ok := p.(plugin.PluginMetric); ok {
			var err1 errors.Error
			metric := PluginMetric{}

			metric.RequiredDataEntities, err1 = pt.RequiredDataEntities()
			if err1 != nil {
				return errors.Default.Wrap(err1, fmt.Sprintf("failed to get RequiredDataEntities on plugin %s", name))
			}

			metric.RunAfter, err1 = pt.RunAfter()
			if err1 != nil {
				return errors.Default.Wrap(err1, fmt.Sprintf("failed to get RunAfter on plugin %s", name))
			}

			metric.IsProjectMetric = pt.IsProjectMetric()

			pluginMeta.Metric = metric
		}

		metas = append(metas, pluginMeta)

		return nil
	})

	if err != nil {
		shared.ApiOutputError(c, errors.Default.Wrap(err, "error getting plugin info of plugins"))
	}

	shared.ApiOutputSuccess(c, metas, http.StatusOK)
}
