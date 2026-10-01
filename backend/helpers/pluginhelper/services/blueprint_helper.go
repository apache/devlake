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

package services

import (
	"fmt"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/impls/logruslog"
)

type BlueprintManager struct {
	db dal.Dal
}

type GetBlueprintQuery struct {
	Enable      *bool
	IsManual    *bool
	Label       string
	SkipRecords int
	PageSize    int
	Mode        string
	Type        string
}

type BlueprintProjectPairs struct {
	Projects   []string `json:"projects"`
	Blueprints []string `json:"blueprints"`
}

func NewBlueprintManager(db dal.Dal) *BlueprintManager {
	return &BlueprintManager{
		db: db,
	}
}

func NewBlueprintProjectPairs(bps []*models.Blueprint) *BlueprintProjectPairs {
	pairs := &BlueprintProjectPairs{}
	for _, bp := range bps {
		pairs.Blueprints = append(pairs.Blueprints, bp.Name)
		pairs.Projects = append(pairs.Projects, bp.ProjectName)
	}
	return pairs
}

// SaveDbBlueprint accepts a Blueprint instance and upsert it to database
func (b *BlueprintManager) SaveDbBlueprint(blueprint *models.Blueprint) errors.Error {
	var err error
	if blueprint.ID != 0 {
		err = b.db.Update(&blueprint)
	} else {
		err = b.db.Create(&blueprint)
	}
	if err != nil {
		return errors.Default.Wrap(err, "error creating DB blueprint")
	}
	err = b.db.Delete(&models.BlueprintLabel{}, dal.Where(`blueprint_id = ?`, blueprint.ID))
	if err != nil {
		return errors.Default.Wrap(err, "error delete DB blueprint's old labelModels")
	}
	if len(blueprint.Labels) > 0 {
		blueprintLabels := make([]*models.BlueprintLabel, 0)
		for i := range blueprint.Labels {
			blueprintLabels = append(blueprintLabels, &models.BlueprintLabel{
				BlueprintId: blueprint.ID,
				Name:        blueprint.Labels[i],
			})
		}
		err = b.db.Create(&blueprintLabels)
		if err != nil {
			return errors.Default.Wrap(err, "error creating DB blueprint's labelModels")
		}
	}
	errors.Must(b.db.Delete(&models.BlueprintConnection{}, dal.Where("blueprint_id = ?", blueprint.ID)))
	errors.Must(b.db.Delete(&models.BlueprintScope{}, dal.Where("blueprint_id = ?", blueprint.ID)))
	for _, conn := range blueprint.Connections {
		conn.BlueprintId = blueprint.ID
		// validate
		errors.Must(b.db.Create(conn))
		for _, scope := range conn.Scopes {
			scope.BlueprintId = blueprint.ID
			scope.PluginName = conn.PluginName
			scope.ConnectionId = conn.ConnectionId
			// validate
			errors.Must(b.db.Create(scope))
		}
	}
	return nil
}

const (
	BP_TYPE_ALL     = "ALL"
	BP_TYPE_MANUAL  = "MANUAL"
	BP_TYPE_DAILY   = "DAILY"
	BP_TYPE_WEEKLY  = "WEEKLY"
	BP_TYPE_MONTHLY = "MONTHLY"
	BP_TYPE_CUSTOM  = "CUSTOM"
	BP_CRON_DAILY   = "0 0 * * *"
	BP_CRON_WEEKLY  = "0 0 * * 1"
	BP_CRON_MONTHLY = "0 0 1 * *"
)

// GetDbBlueprints returns a paginated list of Blueprints based on `query`
func (b *BlueprintManager) GetDbBlueprints(query *GetBlueprintQuery) ([]*models.Blueprint, int64, errors.Error) {
	// process query parameters
	clauses := []dal.Clause{dal.From(&models.Blueprint{})}
	// clauses := []dal.Clause{dal.From("_devlake_blueprints")}
	if query.Enable != nil {
		clauses = append(clauses, dal.Where("enable = ?", *query.Enable))
	}
	if query.IsManual != nil {
		clauses = append(clauses, dal.Where("is_manual = ?", *query.IsManual))
	} else {
		switch query.Type {
		case "", BP_TYPE_ALL:
			// do nothing
		case BP_TYPE_MANUAL:
			clauses = append(clauses, dal.Where("is_manual = ?", true))
		case BP_TYPE_DAILY:
			clauses = append(clauses, dal.Where("is_manual = ? AND cron_config = ?", false, BP_CRON_DAILY))
		case BP_TYPE_WEEKLY:
			clauses = append(clauses, dal.Where("is_manual = ? AND cron_config = ?", false, BP_CRON_WEEKLY))
		case BP_TYPE_MONTHLY:
			clauses = append(clauses, dal.Where("is_manual = ? AND cron_config = ?", false, BP_CRON_MONTHLY))
		case BP_TYPE_CUSTOM:
			clauses = append(clauses, dal.Where("is_manual = ? AND cron_config NOT IN ?", false, []string{BP_CRON_DAILY, BP_CRON_WEEKLY, BP_CRON_MONTHLY}))
		default:
			return nil, 0, errors.BadInput.New(fmt.Sprintf("invalid type %s", query.Type))
		}
	}
	if query.Label != "" {
		clauses = append(clauses,
			dal.Join("left join _devlake_blueprint_labels bl ON bl.blueprint_id = _devlake_blueprints.id"),
			dal.Where("bl.name = ?", query.Label),
		)
	}
	if query.Mode != "" {
		clauses = append(clauses, dal.Where("mode = ?", query.Mode))
	}

	// count total records
	// var count int64
	count := errors.Must1(b.db.Count(clauses...))
	clauses = append(clauses, dal.Orderby("id DESC"))
	// load paginated blueprints from database
	if query.SkipRecords != 0 {
		clauses = append(clauses, dal.Offset(query.SkipRecords))
	}
	if query.PageSize != 0 {
		clauses = append(clauses, dal.Limit(query.PageSize))
	}
	dbBlueprints := make([]*models.Blueprint, 0)
	errors.Must(b.db.All(&dbBlueprints, clauses...))

	// load labels for blueprints
	for _, dbBlueprint := range dbBlueprints {
		err := b.fillBlueprintDetail(dbBlueprint)
		if err != nil {
			return nil, 0, err
		}
	}

	return dbBlueprints, count, nil
}

// GetDbBlueprint returns the detail of a given Blueprint ID
func (b *BlueprintManager) GetDbBlueprint(blueprintId uint64) (*models.Blueprint, errors.Error) {
	blueprint := &models.Blueprint{}
	err := b.db.First(blueprint, dal.Where("id = ?", blueprintId))
	if err != nil {
		if b.db.IsErrorNotFound(err) {
			return nil, errors.NotFound.Wrap(err, "could not find blueprint in DB")
		}
		return nil, errors.Default.Wrap(err, "error getting blueprint from DB")
	}
	err = b.fillBlueprintDetail(blueprint)
	if err != nil {
		return nil, err
	}
	return blueprint, nil
}

// GetBlueprintsByScopes returns all blueprints that have these scopeIds and this connection Id
func (b *BlueprintManager) GetBlueprintsByScopeId(connectionId uint64, pluginName string, scopeId string) []*models.Blueprint {
	blueprints := make([]*models.Blueprint, 0)
	errors.Must(b.db.All(
		&blueprints,
		dal.From("_devlake_blueprints bp"),
		dal.Join("JOIN _devlake_blueprint_scopes sc ON sc.blueprint_id = bp.id"),
		dal.Where(
			"mode = ? AND sc.connection_id = ? AND sc.plugin_name = ? AND sc.scope_id = ?",
			"NORMAL",
			connectionId,
			pluginName,
			scopeId,
		),
	))
	return blueprints
}

// GetBlueprintsByScopes returns all blueprints that have these scopeIds and this connection Id
func (b *BlueprintManager) GetBlueprintsByConnection(plugin string, connectionId uint64) []*models.Blueprint {
	blueprints := make([]*models.Blueprint, 0)
	errors.Must(b.db.All(
		&blueprints,
		dal.From("_devlake_blueprints bp"),
		dal.Join("JOIN _devlake_blueprint_connections conn ON conn.blueprint_id = bp.id"),
		dal.Where(
			"mode = ? AND conn.connection_id = ? AND conn.plugin_name = ?",
			"NORMAL",
			connectionId,
			plugin,
		),
	))
	return blueprints
}

// GetDbBlueprintByProjectName returns the detail of a given projectName
func (b *BlueprintManager) GetDbBlueprintByProjectName(projectName string) (*models.Blueprint, errors.Error) {
	dbBlueprint := &models.Blueprint{}
	err := b.db.First(dbBlueprint, dal.Where("project_name = ?", projectName))
	if err != nil {
		if b.db.IsErrorNotFound(err) {
			return nil, errors.NotFound.Wrap(err, fmt.Sprintf("could not find blueprint in DB by projectName %s", projectName))
		}
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error getting blueprint from DB by projectName %s", projectName))
	}
	err = b.fillBlueprintDetail(dbBlueprint)
	if err != nil {
		return nil, err
	}
	return dbBlueprint, nil
}

// DeleteBlueprint deletes a blueprint by its id
func (b *BlueprintManager) DeleteBlueprint(id uint64) errors.Error {
	var err errors.Error
	tx := b.db.Begin()
	defer func() {
		if r := recover(); r != nil || err != nil {
			err = tx.Rollback()
			if err != nil {
				logruslog.Global.Error(err, "DeleteBlueprint: failed to rollback")
			}
		}
	}()
	err = b.DeleteBlueprintInTransaction(tx, id)
	if err != nil {
		return err
	}
	err = tx.Commit()
	return err
}

// DeleteBlueprintInTransaction removes a blueprint and its dependent records using
// the caller's transaction. The caller owns commit and rollback.
func (b *BlueprintManager) DeleteBlueprintInTransaction(tx dal.Transaction, id uint64) errors.Error {
	err := tx.Delete(&models.BlueprintLabel{}, dal.Where("blueprint_id = ?", id))
	if err != nil {
		return err
	}
	err = tx.Delete(&models.Blueprint{
		Model: common.Model{
			ID: id,
		},
	})
	if err != nil {
		return err
	}
	err = tx.Delete(&models.BlueprintConnection{}, dal.Where("blueprint_id = ?", id))
	if err != nil {
		return err
	}
	err = tx.Delete(&models.BlueprintScope{}, dal.Where("blueprint_id = ?", id))
	if err != nil {
		return err
	}
	return nil
}

func (b *BlueprintManager) fillBlueprintDetail(blueprint *models.Blueprint) errors.Error {
	errors.Must(
		b.db.Pluck(
			"name",
			&blueprint.Labels, dal.From(&models.BlueprintLabel{}), dal.Where("blueprint_id = ?", blueprint.ID),
		),
	)
	errors.Must(
		b.db.All(
			&blueprint.Connections,
			dal.Where("blueprint_id = ?", blueprint.ID),
		),
	)
	for _, conn := range blueprint.Connections {
		errors.Must(
			b.db.All(
				&conn.Scopes,
				dal.Where(
					"blueprint_id = ? AND plugin_name = ? AND connection_id = ?",
					blueprint.ID,
					conn.PluginName,
					conn.ConnectionId,
				),
			),
		)
	}
	return nil
}
