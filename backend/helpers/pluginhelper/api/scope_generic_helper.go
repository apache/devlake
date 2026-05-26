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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer/domaininfo"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/dbhelper"
	serviceHelper "github.com/apache/incubator-devlake/helpers/pluginhelper/services"
	"github.com/go-playground/validator/v10"
)

type NoScopeConfig struct{}

type (
	GenericScopeApiHelper[Conn any, Scope plugin.ToolLayerScope, ScopeConfig any] struct {
		basicRes         context.BasicRes
		log              log.Logger
		db               dal.Dal
		validator        *validator.Validate
		reflectionParams *ReflectionParameters
		dbHelper         ScopeDatabaseHelper[Conn, Scope, ScopeConfig]
		bpManager        *serviceHelper.BlueprintManager
		connHelper       *ConnectionApiHelper
		opts             *ScopeHelperOptions
		plugin           string
	}
	// as of golang v1.20, embedding generic fields is not supported
	// let's divide the struct into two parts for swagger doc to work
	// https://stackoverflow.com/questions/66118867/go-generics-is-it-possible-to-embed-generic-structs
	ScopeResDoc[ScopeConfig any] struct {
		ScopeConfig *ScopeConfig        `mapstructure:"scopeConfig,omitempty" json:"scopeConfig"`
		Blueprints  []*models.Blueprint `mapstructure:"blueprints,omitempty" json:"blueprints"`
	}
	// Alias, for swagger purposes
	ScopeRes[Scope plugin.ToolLayerScope, ScopeConfig any] struct {
		Scope       Scope               `mapstructure:"scope,omitempty" json:"scope,omitempty"`
		ScopeConfig *ScopeConfig        `mapstructure:"scopeConfig,omitempty" json:"scopeConfig,omitempty"`
		Blueprints  []*models.Blueprint `mapstructure:"blueprints,omitempty" json:"blueprints,omitempty"`
	}
	ScopeListRes[Scope plugin.ToolLayerScope, ScopeConfig any] struct {
		Scopes []*ScopeRes[Scope, ScopeConfig] `mapstructure:"scopes" json:"scopes"`
		Count  int64                           `mapstructure:"count" json:"count"`
	}
	ReflectionParameters struct {
		// This corresponds to the struct field of the scope struct's ID field
		ScopeIdFieldName string `validate:"required"`
		// This corresponds to the database column name of the scope struct's ID (typically primary key) field
		ScopeIdColumnName string `validate:"required"`
		// This corresponds to the scope field on the ApiParams struct of a plugin.
		RawScopeParamName string `validate:"required"`
		// This corresponds to the scope field for allowing data scope search.
		SearchScopeParamName string
	}
	ScopeHelperOptions struct {
		// Define this if the raw params doesn't store the ScopeId but a different attribute of the Scope (e.g. Name)
		GetScopeParamValue func(db dal.Dal, scopeId string) (string, errors.Error)
		IsRemote           bool
	}
)

type (
	requestParams struct {
		connectionId uint64
		scopeId      string
		plugin       string
	}
	deleteRequestParams struct {
		requestParams
		deleteDataOnly bool
	}

	getRequestParams struct {
		requestParams
		loadBlueprints bool
	}
)

func NewGenericScopeHelper[Conn any, Scope plugin.ToolLayerScope, ScopeConfig any](
	basicRes context.BasicRes,
	vld *validator.Validate,
	connHelper *ConnectionApiHelper,
	dbHelper ScopeDatabaseHelper[Conn, Scope, ScopeConfig],
	params *ReflectionParameters,
	opts *ScopeHelperOptions,
) *GenericScopeApiHelper[Conn, Scope, ScopeConfig] {
	if connHelper == nil {
		panic("nil connHelper")
	}
	if params == nil {
		panic("reflection params not provided")
	}
	err := vld.Struct(params)
	if err != nil {
		panic(err)
	}
	if opts == nil {
		opts = &ScopeHelperOptions{}
	}
	return &GenericScopeApiHelper[Conn, Scope, ScopeConfig]{
		basicRes:         basicRes,
		log:              basicRes.GetLogger(),
		db:               basicRes.GetDal(),
		validator:        vld,
		reflectionParams: params,
		dbHelper:         dbHelper,
		bpManager:        serviceHelper.NewBlueprintManager(basicRes.GetDal()),
		connHelper:       connHelper,
		opts:             opts,
		plugin:           connHelper.pluginName,
	}
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) DbHelper() ScopeDatabaseHelper[Conn, Scope, ScopeConfig] {
	return gs.dbHelper
}

// hacky, temporary solution
func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) setRawDataOrigin(scopes ...*Scope) {
	for _, scope := range scopes {
		if !setRawDataOrigin(scope, common.RawDataOrigin{
			RawDataTable:  fmt.Sprintf("_raw_%s_scopes", gs.plugin),
			RawDataParams: plugin.MarshalScopeParams((*scope).ScopeParams()),
		}) {
			panic("RawDataOrigin could not be set")
		}
	}
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) PutScopes(input *plugin.ApiResourceInput, scopes []*Scope) ([]*ScopeRes[Scope, ScopeConfig], errors.Error) {
	params, err := gs.extractFromReqParam(input, false)
	if err != nil {
		return nil, err
	}
	err = gs.dbHelper.VerifyConnection(params.connectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error verifying connection for connection ID %d", params.connectionId))
	}
	if len(scopes) == 0 {
		return nil, nil
	}
	err = gs.validatePrimaryKeys(scopes)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, scope := range scopes {
		// Set the connection ID, CreatedAt, and UpdatedAt fields
		gs.setScopeFields(scope, params.connectionId, &now, &now)
		err = gs.verifyScope(scope, gs.validator)
		if err != nil {
			return nil, errors.Default.Wrap(err, "error verifying scope")
		}
	}
	gs.setRawDataOrigin(scopes...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error saving scope")
	}
	err = gs.dbHelper.SaveScope(scopes)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error saving scope")
	}
	apiScopes, err := gs.addScopeConfig(scopes...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error associating scope config to scope")
	}
	return apiScopes, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) UpdateScope(input *plugin.ApiResourceInput) (*ScopeRes[Scope, ScopeConfig], errors.Error) {
	params, err := gs.extractFromReqParam(input, true)
	if err != nil {
		return nil, err
	}
	err = gs.dbHelper.VerifyConnection(params.connectionId)
	if err != nil {
		return nil, err
	}
	scope, err := gs.dbHelper.GetScope(params.connectionId, params.scopeId)
	if err != nil {
		return nil, err
	}
	err = DecodeMapStruct(input.Body, scope, true)
	if err != nil {
		return nil, errors.Default.Wrap(err, "patch scope error")
	}
	err = gs.verifyScope(scope, gs.validator)
	if err != nil {
		return nil, errors.Default.Wrap(err, "Invalid scope")
	}
	gs.setRawDataOrigin(scope)
	err = gs.dbHelper.UpdateScope(scope)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error on saving Scope")
	}
	scopeRes, err := gs.addScopeConfig(scope)
	if err != nil {
		return nil, err
	}
	return scopeRes[0], nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) GetScopes(input *plugin.ApiResourceInput) (*ScopeListRes[Scope, ScopeConfig], errors.Error) {
	params, err := gs.extractFromGetReqParam(input, false)
	if err != nil {
		return nil, errors.BadInput.New("invalid path params: \"connectionId\" not set")
	}
	err = gs.dbHelper.VerifyConnection(params.connectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error verifying connection for connection ID %d", params.connectionId))
	}
	scopes, count, err := gs.dbHelper.ListScopes(input, params.connectionId)
	if err != nil {
		return nil, err
	}
	apiScopes, err := gs.addScopeConfig(scopes...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error associating scope configs with scopes")
	}
	// return empty array rather than nil in case of no scopes
	if len(apiScopes) > 0 && params.loadBlueprints {
		for _, apiScope := range apiScopes {
			apiScope.Blueprints = gs.bpManager.GetBlueprintsByScopeId(params.connectionId, params.plugin, apiScope.Scope.ScopeId())
		}
	}
	return &ScopeListRes[Scope, ScopeConfig]{
		Scopes: apiScopes,
		Count:  count,
	}, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) GetScope(input *plugin.ApiResourceInput) (*ScopeRes[Scope, ScopeConfig], errors.Error) {
	params, err := gs.extractFromGetReqParam(input, true)
	if err != nil {
		return nil, err
	}
	err = gs.dbHelper.VerifyConnection(params.connectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error verifying connection for connection ID %d", params.connectionId))
	}
	scope, err := gs.dbHelper.GetScope(params.connectionId, params.scopeId)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error retrieving scope with scope ID %s", params.scopeId))
	}
	apiScopes, err := gs.addScopeConfig(scope)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error associating scope config with scope %s", params.scopeId))
	}
	scopeRes := apiScopes[0]
	if params.loadBlueprints {
		scopeRes.Blueprints = gs.bpManager.GetBlueprintsByScopeId(params.connectionId, params.plugin, params.scopeId)
	}
	return scopeRes, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) DeleteScope(input *plugin.ApiResourceInput) (refs *serviceHelper.BlueprintProjectPairs, err errors.Error) {
	txHelper := dbhelper.NewTxHelper(gs.basicRes, &err)
	defer txHelper.End()
	tx := txHelper.Begin()
	err = txHelper.LockTablesTimeout(2*time.Second, dal.LockTables{{Table: "_devlake_pipelines"}})
	if err != nil {
		err = errors.Conflict.Wrap(err, "This data scope cannot be deleted due to a table lock error. There might be running pipeline(s) or other deletion operations in progress.")
		return
	}
	count := errors.Must1(tx.Count(
		dal.From("_devlake_pipelines"),
		dal.Where("status = ?", models.TASK_RUNNING),
	))
	if count > 0 {
		err = errors.Conflict.New("This data scope cannot be deleted because a pipeline is running. Please try again after you cancel the pipeline or wait for it to finish.")
		return
	}
	// time.Sleep(1 * time.Minute) # uncomment this line if you were to verify pipelines get blocked while deleting data
	params, err := gs.extractFromDeleteReqParam(input)
	if err != nil {
		return nil, err
	}
	err = gs.dbHelper.VerifyConnection(params.connectionId)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, fmt.Sprintf("error verifying connection for connection ID %d", params.connectionId))
	}
	scope, err := gs.dbHelper.GetScope(params.connectionId, params.scopeId)
	if err != nil {
		return nil, err
	}

	if !params.deleteDataOnly {
		blueprints := gs.bpManager.GetBlueprintsByScopeId(params.connectionId, params.plugin, params.scopeId)
		if len(blueprints) > 0 {
			refs = &serviceHelper.BlueprintProjectPairs{}
			for _, bp := range blueprints {
				refs.Blueprints = append(refs.Blueprints, bp.Name)
				refs.Projects = append(refs.Projects, bp.ProjectName)
			}
			return refs, errors.Conflict.New("Found one or more references to this scope")
		}
	}
	if err = gs.deleteScopeData(*scope); err != nil {
		return nil, err
	}
	if !params.deleteDataOnly {
		// Delete the scope itself
		errors.Must(gs.dbHelper.DeleteScope(scope))
	}
	return nil, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) addScopeConfig(scopes ...*Scope) ([]*ScopeRes[Scope, ScopeConfig], errors.Error) {
	apiScopes := make([]*ScopeRes[Scope, ScopeConfig], len(scopes))
	for i, scope := range scopes {
		apiScopes[i] = &ScopeRes[Scope, ScopeConfig]{
			Scope: *scope,
		}
		scIdField := reflectField(scope, "ScopeConfigId")
		if scIdField.IsValid() && scIdField.Uint() > 0 {
			scopeConfig, err := gs.dbHelper.GetScopeConfig(scIdField.Uint())
			if err != nil {
				return nil, err
			}
			apiScopes[i].ScopeConfig = scopeConfig
		}
	}
	return apiScopes, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) extractFromReqParam(input *plugin.ApiResourceInput, withScopeId bool) (*requestParams, errors.Error) {
	connectionId, err := strconv.ParseUint(input.Params["connectionId"], 10, 64)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "Invalid \"connectionId\"")
	}
	if connectionId == 0 {
		return nil, errors.BadInput.New("\"connectionId\" cannot be 0")
	}
	var scopeId string
	if withScopeId {
		scopeId = input.Params["scopeId"]
		// Path params that use `/*param` handlers instead of `/:param` start with a /, so remove it
		if scopeId[0] == '/' {
			scopeId = scopeId[1:]
		}
	}
	return &requestParams{
		connectionId: connectionId,
		plugin:       gs.plugin,
		scopeId:      scopeId,
	}, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) extractFromDeleteReqParam(input *plugin.ApiResourceInput) (*deleteRequestParams, errors.Error) {
	params, err := gs.extractFromReqParam(input, true)
	if err != nil {
		return nil, err
	}
	var deleteDataOnly bool
	{
		ddo, ok := input.Query["delete_data_only"]
		if ok {
			deleteDataOnly, err = errors.Convert01(strconv.ParseBool(ddo[0]))
			if err != nil {
				deleteDataOnly = false
			}
		}
	}
	return &deleteRequestParams{
		requestParams:  *params,
		deleteDataOnly: deleteDataOnly,
	}, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) extractFromGetReqParam(input *plugin.ApiResourceInput, withScopeId bool) (*getRequestParams, errors.Error) {
	params, err := gs.extractFromReqParam(input, withScopeId)
	if err != nil {
		return nil, err
	}
	var loadBlueprints bool
	{
		lbps, ok := input.Query["blueprints"]
		if ok {
			loadBlueprints, err = errors.Convert01(strconv.ParseBool(lbps[0]))
			if err != nil {
				loadBlueprints = false
			}
		}
	}
	return &getRequestParams{
		requestParams:  *params,
		loadBlueprints: loadBlueprints,
	}, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) createRawParams(connectionId uint64, scopeId any) string {
	// TODO for future: have ScopeParams expose a constructor so we pass the variables to that instead of this hack
	paramsMap := map[string]any{
		"ConnectionId":                        connectionId,
		gs.reflectionParams.RawScopeParamName: scopeId,
	}
	return plugin.MarshalScopeParams(paramsMap)
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) setScopeFields(p interface{}, connectionId uint64, createdAt *time.Time, updatedAt *time.Time) {
	pType := reflect.TypeOf(p)
	if pType.Kind() != reflect.Ptr {
		panic("expected a pointer to a struct")
	}
	pValue := reflectValue(p)
	// set connectionId
	connIdField := pValue.FieldByName("ConnectionId")
	connIdField.SetUint(connectionId)

	// set raw params
	rawParams := pValue.FieldByName("RawDataParams")
	if !rawParams.IsValid() {
		panic("scope is missing the field \"RawDataParams\"")
	}
	scopeIdField := pValue.FieldByName(gs.reflectionParams.ScopeIdFieldName)
	rawParams.Set(reflect.ValueOf(gs.createRawParams(connectionId, scopeIdField.Interface())))

	// set CreatedAt
	createdAtField := pValue.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.Type().AssignableTo(reflect.TypeOf(createdAt)) {
		createdAtField.Set(reflect.ValueOf(createdAt))
	}

	// set UpdatedAt
	updatedAtField := pValue.FieldByName("UpdatedAt")
	if !updatedAtField.IsValid() || (updatedAt != nil && !updatedAtField.Type().AssignableTo(reflect.TypeOf(updatedAt))) {
		return
	}
	if updatedAt == nil {
		// if updatedAt is nil, set UpdatedAt to be nil
		updatedAtField.Set(reflect.Zero(updatedAtField.Type()))
	} else {
		// if updatedAt is not nil, set UpdatedAt to be the value
		updatedAtFieldValue := reflect.ValueOf(updatedAt)
		updatedAtField.Set(updatedAtFieldValue)
	}
}

// returnPrimaryKeyValue returns a string containing the primary key value(s) of a struct, concatenated with "-" between them.
// This function receives an interface{} type argument p, which can be a pointer to any struct.
// The function uses reflection to iterate through the fields of the struct, and checks if each field is tagged as "primaryKey".
func returnPrimaryKeyValue(p interface{}) string {
	result := ""
	// get the type and value of the input interface using reflection
	t := reflectType(p)
	v := reflectValue(p)
	// iterate over each field in the struct type
	for i := 0; i < t.NumField(); i++ {
		// get the i-th field
		field := t.Field(i)

		// check if the field is marked as "primaryKey" in the struct tag
		if strings.Contains(string(field.Tag), "primaryKey") {
			// if this is the first primaryKey field encountered, set the result to be its value
			if result == "" {
				result = fmt.Sprintf("%v", v.Field(i).Interface())
			} else {
				// if this is not the first primaryKey field, append its value to the result with a "-" separator
				result = fmt.Sprintf("%s-%v", result, v.Field(i).Interface())
			}
		}
	}

	// return the final primary key value as a string
	return result
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) verifyScope(scope interface{}, vld *validator.Validate) errors.Error {
	if gs.opts.IsRemote {
		return nil
	}
	pType := reflect.TypeOf(scope)
	if pType.Kind() != reflect.Ptr {
		panic("expected a pointer to a struct")
	}
	if err := vld.Struct(scope); err != nil {
		return errors.Default.Wrap(err, "error validating target")
	}
	return nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) validatePrimaryKeys(scopes []*Scope) errors.Error {
	if gs.opts.IsRemote {
		return nil
	}
	keeper := make(map[string]struct{})
	for _, scope := range scopes {
		// Ensure that the primary key value is unique
		primaryValueStr := returnPrimaryKeyValue(scope)
		if _, ok := keeper[primaryValueStr]; ok {
			return errors.BadInput.New("duplicate scope was requested")
		} else {
			keeper[primaryValueStr] = struct{}{}
		}
	}
	return nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) deleteScopeData(scope plugin.ToolLayerScope) errors.Error {
	// find all tables for this plugin
	tables, err := gs.getAffectedTables(gs.plugin)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error getting database tables managed by plugin %s", gs.plugin))
	}
	scopeParams := plugin.MarshalScopeParams(scope.ScopeParams())
	err = gs.transactionalDelete(tables, scopeParams)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error deleting data bound to scope %s for plugin %s", scopeParams, gs.plugin))
	}
	return nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) transactionalDelete(tables []string, rawDataParams string) errors.Error {
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
			rawDataTablePrefix := fmt.Sprintf("_raw_%s%%", gs.plugin)
			params = []interface{}{rawDataTablePrefix, rawDataParams}
		}
		return where, params
	}
	tx := gs.db.Begin()
	for _, table := range tables {
		if err := dal.ValidateTableName(table); err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("unsafe table name %q when deleting scope data", table))
		}
		where, params := generateWhereClause(table)
		gs.log.Info("deleting data from table %s with WHERE \"%s\" and params: \"%v\"", table, where, params)
		sql := fmt.Sprintf("DELETE FROM %s WHERE %s", table, where)
		err := tx.Exec(sql, params...)
		if err != nil {
			err2 := tx.Rollback()
			if err2 != nil {
				gs.log.Warn(err2, fmt.Sprintf("error rolling back table data deletion transaction. sql: %s params: %v", sql, params))
			}
			return err
		}
	}
	err := tx.Commit()
	if err != nil {
		return errors.Default.Wrap(err, "error committing delete transaction for plugin tables")
	}
	// validate everything was deleted
	var failedTables []string
	for _, table := range tables {
		where, params := generateWhereClause(table)
		count, err := gs.db.Count(dal.From(table), dal.Where(where, params...))
		if err != nil {
			return err
		}
		if count > 0 {
			failedTables = append(failedTables, table)
		}
	}
	if len(failedTables) > 0 {
		return errors.Default.New(fmt.Sprintf("Failed to delete all expected rows from the following table(s): %v", failedTables))
	}
	return nil
}

// GetScopeLatestSyncState only works for remote plugins.
// Make sure all remote plugins save their state in table `_devlake_collector_latest_state`.
// For golang version plugin, use `dsHelper.ScopeApi.GetScopeLatestSyncState` instead.
func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) GetScopeLatestSyncState(input *plugin.ApiResourceInput) ([]*models.LatestSyncState, errors.Error) {
	scope, err := gs.GetScope(input)
	if err != nil {
		return nil, err
	}
	params := plugin.MarshalScopeParams(scope.Scope.ScopeParams())
	scopeSyncStates := []*models.LatestSyncState{}
	if err := gs.db.All(
		&scopeSyncStates,
		dal.Select("raw_data_table, latest_success_start, raw_data_params"),
		dal.From("_devlake_collector_latest_state"),
		dal.Where("raw_data_params = ?", params),
	); err != nil {
		return nil, err
	}
	return scopeSyncStates, nil
}

func (gs *GenericScopeApiHelper[Conn, Scope, ScopeConfig]) getAffectedTables(pluginName string) ([]string, errors.Error) {
	var tables []string
	meta, err := plugin.GetPlugin(pluginName)
	if err != nil {
		return nil, err
	}
	if pluginModel, ok := meta.(plugin.PluginModel); !ok {
		return nil, errors.Default.New(fmt.Sprintf("plugin \"%s\" does not implement listing its tables", pluginName))
	} else {
		// Unfortunately, can't cache the tables because Python creates some tables on a per-demand basis, so such a cache would possibly get outdated.
		// It's a rare scenario in practice, but might as well play it safe and sacrifice some performance here
		var allTables []string
		if allTables, err = gs.db.AllTables(); err != nil {
			return nil, err
		}
		// collect raw tables
		for _, table := range allTables {
			if strings.HasPrefix(table, "_raw_"+pluginName) {
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
	}
	gs.log.Debug("Discovered %d tables used by plugin \"%s\": %v", len(tables), pluginName, tables)
	return tables, nil
}

func isScopeModel(obj dal.Tabler) bool {
	_, ok := obj.(plugin.ToolLayerScope)
	return ok
}
