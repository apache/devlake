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

package srvhelper

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer/domaininfo"
	"github.com/apache/incubator-devlake/core/plugin"
)

type ScopePagination struct {
	Pagination   `mapstructure:",squash"`
	ConnectionId uint64 `json:"connectionId" mapstructure:"connectionId" validate:"required"`
	Blueprints   bool   `json:"blueprints" mapstructure:"blueprints"`
}

type ScopeDetail[S plugin.ToolLayerScope, SC plugin.ToolLayerScopeConfig] struct {
	Scope       S                   `json:"scope"`
	ScopeConfig *SC                 `json:"scopeConfig,omitempty"`
	Blueprints  []*models.Blueprint `json:"blueprints,omitempty"`
}

type ScopeSrvHelper[C plugin.ToolLayerConnection, S plugin.ToolLayerScope, SC plugin.ToolLayerScopeConfig] struct {
	*ModelSrvHelper[S]
	pluginName string
}

// NewScopeSrvHelper creates a ScopeDalHelper for scope management
func NewScopeSrvHelper[
	C plugin.ToolLayerConnection,
	S plugin.ToolLayerScope,
	SC plugin.ToolLayerScopeConfig,
](
	basicRes context.BasicRes,
	pluginName string,
	searchColumns []string,
) *ScopeSrvHelper[C, S, SC] {
	return &ScopeSrvHelper[C, S, SC]{
		ModelSrvHelper: NewModelSrvHelper[S](basicRes, searchColumns),
		pluginName:     pluginName,
	}
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) GetPluginName() string {
	return scopeSrv.pluginName
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) Validate(scope *S) errors.Error {
	connectionId := (*scope).ScopeConnectionId()
	connectionCount := errors.Must1(scopeSrv.db.Count(dal.From(new(SC)), dal.Where("id = ?", connectionId)))
	if connectionCount == 0 {
		return errors.BadInput.New("connectionId is invalid")
	}
	scopeConfigId := (*scope).ScopeScopeConfigId()
	scopeConfigCount := errors.Must1(scopeSrv.db.Count(dal.From(new(SC)), dal.Where("id = ?", scopeConfigId)))
	if scopeConfigCount == 0 {
		return errors.BadInput.New("scopeConfigId is invalid")
	}
	return nil
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) GetScopeDetail(includeBlueprints bool, pkv ...interface{}) (*ScopeDetail[S, SC], errors.Error) {
	scope, err := scopeSrv.ModelSrvHelper.FindByPk(pkv...)
	if err != nil {
		return nil, err
	}
	s := *scope
	scopeDetail := &ScopeDetail[S, SC]{
		Scope:       s,
		ScopeConfig: scopeSrv.getScopeConfig(s.ScopeScopeConfigId()),
	}
	if includeBlueprints {
		scopeDetail.Blueprints = scopeSrv.getAllBlueprinsByScope(s.ScopeConnectionId(), s.ScopeId())
	}
	return scopeDetail, nil
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) GetScopeLatestSyncState(pkv ...interface{}) ([]*models.LatestSyncState, errors.Error) {
	scope, err := scopeSrv.ModelSrvHelper.FindByPk(pkv...)
	if err != nil {
		return nil, err
	}
	s := *scope
	params := plugin.MarshalScopeParams(s.ScopeParams())
	scopeSrv.log.Debug("scope: %#+v, params: %+v", s, params)
	scopeSyncStates := []*models.LatestSyncState{}
	if err := scopeSrv.db.All(
		&scopeSyncStates,
		dal.Select("raw_data_table, latest_success_start, raw_data_params"),
		dal.From("_devlake_collector_latest_state"),
		dal.Where("raw_data_params = ?", params),
	); err != nil {
		return nil, err
	}
	scopeSrv.log.Debug("param: %+v, resp: %+v", scopeSyncStates)
	sort.Slice(scopeSyncStates, func(i, j int) bool {
		if scopeSyncStates[i].LatestSuccessStart != nil && scopeSyncStates[j].LatestSuccessStart != nil {
			return scopeSyncStates[i].LatestSuccessStart.After(*scopeSyncStates[j].LatestSuccessStart)
		}
		return false
	})
	return scopeSyncStates, nil
}

// MapScopeDetails returns scope details (scope and scopeConfig) for the given blueprint scopes
func (scopeSrv *ScopeSrvHelper[C, S, SC]) MapScopeDetails(connectionId uint64, bpScopes []*models.BlueprintScope) ([]*ScopeDetail[S, SC], errors.Error) {
	var err errors.Error
	scopeDetails := make([]*ScopeDetail[S, SC], len(bpScopes))
	for i, bpScope := range bpScopes {
		scopeDetails[i], err = scopeSrv.GetScopeDetail(false, connectionId, bpScope.ScopeId)
		if err != nil {
			return nil, err
		}
		if scopeDetails[i].ScopeConfig == nil {
			scopeDetails[i].ScopeConfig = new(SC)
		}
		setDefaultEntities(scopeDetails[i].ScopeConfig)
	}
	return scopeDetails, nil
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) GetScopesPage(pagination *ScopePagination) ([]*ScopeDetail[S, SC], int64, errors.Error) {
	if pagination.ConnectionId < 1 {
		return nil, 0, errors.BadInput.New("connectionId is required")
	}
	scopes, count, err := scopeSrv.ModelSrvHelper.GetPage(
		&pagination.Pagination,
		dal.Where("connection_id = ?", pagination.ConnectionId),
	)
	if err != nil {
		return nil, 0, err
	}

	data := make([]*ScopeDetail[S, SC], len(scopes))
	for i, s := range scopes {
		// load blueprints
		scope := *s
		scopeDetail := &ScopeDetail[S, SC]{
			Scope:       scope,
			ScopeConfig: scopeSrv.getScopeConfig(scope.ScopeScopeConfigId()),
		}
		if pagination.Blueprints {
			scopeDetail.Blueprints = scopeSrv.getAllBlueprinsByScope(scope.ScopeConnectionId(), scope.ScopeId())
		}
		data[i] = scopeDetail
	}
	return data, count, nil
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) DeleteScope(scope *S, dataOnly bool) (refs *DsRefs, err errors.Error) {
	err = scopeSrv.ModelSrvHelper.NoRunningPipeline(func(tx dal.Transaction) errors.Error {
		s := *scope
		// check referencing blueprints
		if !dataOnly {
			refs = toDsRefs(scopeSrv.getAllBlueprinsByScope(s.ScopeConnectionId(), s.ScopeId()))
			if refs != nil {
				return errors.Conflict.New("Cannot delete the scope because it is referenced by blueprints")
			}
			errors.Must(tx.Delete(scope))
		}
		// delete data
		scopeSrv.deleteScopeData(s, tx)
		return nil
	})
	return
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) getScopeConfig(scopeConfigId uint64) *SC {
	if scopeConfigId < 1 {
		return nil
	}
	var scopeConfig SC
	err := scopeSrv.db.First(
		&scopeConfig,
		dal.Where(
			"id = ?",
			scopeConfigId,
		),
	)
	if err != nil {
		return nil
	}
	return &scopeConfig
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) getAllBlueprinsByScope(connectionId uint64, scopeId string) []*models.Blueprint {
	blueprints := make([]*models.Blueprint, 0)
	errors.Must(scopeSrv.db.All(
		&blueprints,
		dal.From("_devlake_blueprints bp"),
		dal.Join("JOIN _devlake_blueprint_scopes sc ON sc.blueprint_id = bp.id"),
		dal.Where(
			"mode = ? AND sc.connection_id = ? AND sc.plugin_name = ? AND sc.scope_id = ?",
			"NORMAL",
			connectionId,
			scopeSrv.pluginName,
			scopeId,
		),
	))
	for _, bp := range blueprints {
		bp.Plan = nil
	}
	return blueprints
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) deleteScopeData(scope plugin.ToolLayerScope, tx dal.Transaction) {
	rawDataParams := plugin.MarshalScopeParams(scope.ScopeParams())
	generateWhereClause := func(table string) (string, []any) {
		var where string
		var params []interface{}
		if strings.HasPrefix(table, "_raw_") {
			// raw table: should check connection and scope
			where = "params = ?"
			params = []interface{}{rawDataParams}
		} else if strings.HasPrefix(table, "_tool_") {
			// tool layer table: should check connection and scope
			where = "_raw_data_params = ?"
			params = []interface{}{rawDataParams}
		} else {
			// framework tables: should check plugin, connection and scope
			if table == (models.CollectorLatestState{}.TableName()) {
				// diff sync state
				where = "raw_data_table LIKE ? AND raw_data_params = ?"
			} else {
				// domain layer table
				where = "_raw_data_table LIKE ? AND _raw_data_params = ?"
			}
			rawDataTablePrefix := fmt.Sprintf("_raw_%s%%", scopeSrv.pluginName)
			params = []interface{}{rawDataTablePrefix, rawDataParams}
		}
		return where, params
	}
	tables := errors.Must1(scopeSrv.getAffectedTables())
	for _, table := range tables {
		if err := dal.ValidateTableName(table); err != nil {
			panic(errors.Default.Wrap(err, fmt.Sprintf("unsafe table name %q when deleting scope data", table)))
		}
		where, params := generateWhereClause(table)
		scopeSrv.log.Info("deleting data from table %s with WHERE \"%s\" and params: \"%v\"", table, where, params)
		sql := fmt.Sprintf("DELETE FROM %s WHERE %s", table, where)
		errors.Must(tx.Exec(sql, params...))
	}
}

func (scopeSrv *ScopeSrvHelper[C, S, SC]) getAffectedTables() ([]string, errors.Error) {
	var tables []string
	meta, err := plugin.GetPlugin(scopeSrv.pluginName)
	if err != nil {
		return nil, err
	}
	pluginModel, ok := meta.(plugin.PluginModel)
	if !ok {
		panic(errors.Default.New(fmt.Sprintf("plugin \"%s\" does not implement listing its tables", scopeSrv.pluginName)))
	}
	// Unfortunately, can't cache the tables because Python creates some tables on a per-demand basis, so such a cache would possibly get outdated.
	// It's a rare scenario in practice, but might as well play it safe and sacrifice some performance here
	var allTables []string
	if allTables, err = scopeSrv.db.AllTables(); err != nil {
		return nil, err
	}
	// collect raw tables
	for _, table := range allTables {
		if strings.HasPrefix(table, "_raw_"+scopeSrv.pluginName) {
			tables = append(tables, table)
		}
	}
	// collect tool tables
	toolModels := pluginModel.GetTablesInfo()
	for _, toolModel := range toolModels {
		if !isScopeModel(toolModel) && hasField(toolModel, "RawDataParams") {
			tables = append(tables, toolModel.TableName())
		}
	}
	// collect domain tables
	for _, domainModel := range domaininfo.GetDomainTablesInfo() {
		// we only care about tables with RawOrigin
		ok = hasField(domainModel, "RawDataParams")
		if ok {
			tables = append(tables, domainModel.TableName())
		}
	}
	// additional tables
	tables = append(tables, models.CollectorLatestState{}.TableName())
	scopeSrv.log.Debug("Discovered %d tables used by plugin \"%s\": %v", len(tables), scopeSrv.pluginName, tables)
	return tables, nil
}

// TODO: sort out the follow functions
func isScopeModel(obj dal.Tabler) bool {
	_, ok := obj.(plugin.ToolLayerScope)
	return ok
}

func hasField(obj any, fieldName string) bool {
	obj = models.UnwrapObject(obj)
	_, ok := reflectType(obj).FieldByName(fieldName)
	return ok
}

func reflectType(obj any) reflect.Type {
	obj = models.UnwrapObject(obj)
	typ := reflect.TypeOf(obj)
	kind := typ.Kind()
	for kind == reflect.Ptr {
		typ = typ.Elem()
		kind = typ.Kind()
	}
	return typ
}

func setDefaultEntities(sc interface{}) {
	v := reflect.ValueOf(sc)
	if v.Kind() != reflect.Pointer {
		panic(fmt.Errorf("sc must be a pointer"))
	}
	entities := v.Elem().FieldByName("Entities")
	if !entities.IsValid() ||
		!(entities.Kind() == reflect.Array || entities.Kind() == reflect.Slice) ||
		entities.Type().Elem().Kind() != reflect.String {
		return
	}
	if entities.IsNil() || entities.Len() == 0 {
		entities.Set(reflect.ValueOf(plugin.DOMAIN_TYPES))
	}
}
