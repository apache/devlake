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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/apache/incubator-devlake/helpers/pluginhelper/services"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/impls/logruslog"
	"github.com/robfig/cron/v3"
)

var (
	blueprintLog = logruslog.Global.Nested("blueprint")
	ErrEmptyPlan = errors.Default.New("empty plan")
)

// BlueprintQuery is a query for GetBlueprints
type BlueprintQuery struct {
	Pagination
	Enable   *bool  `form:"enable,omitempty"`
	IsManual *bool  `form:"isManual"`
	Label    string `form:"label"`
	// isManual must be omitted or `null` for type to take effect
	Type string `form:"type" enums:"ALL,MANUAL,DAILY,WEEKLY,MONTHLY,CUSTOM" validate:"oneof=ALL MANUAL DAILY WEEKLY MONTHLY CUSTOM"`
}

type BlueprintJob struct {
	Blueprint *models.Blueprint
}

func (bj BlueprintJob) Run() {
	blueprint := bj.Blueprint
	pipeline, err := createPipelineByBlueprint(blueprint, &blueprint.SyncPolicy)
	if err == ErrEmptyPlan {
		blueprintLog.Info("Empty plan, blueprint id:[%d] blueprint name:[%s]", blueprint.ID, blueprint.Name)
		return
	}
	if err != nil {
		blueprintLog.Error(err, fmt.Sprintf("run cron job failed on blueprint:[%d][%s]", blueprint.ID, blueprint.Name))
	} else {
		blueprintLog.Info("Run new cron job successfully,blueprint id:[%d] pipeline id:[%d]", blueprint.ID, pipeline.ID)
	}
}

// CreateBlueprint accepts a Blueprint instance and insert it to database
func CreateBlueprint(blueprint *models.Blueprint) errors.Error {
	_, err := saveBlueprint(blueprint)
	return err
}

// GetBlueprints returns a paginated list of Blueprints based on `query`
func GetBlueprints(query *BlueprintQuery, shouldSanitize bool) ([]*models.Blueprint, int64, errors.Error) {
	blueprints, count, err := bpManager.GetDbBlueprints(&services.GetBlueprintQuery{
		Enable:      query.Enable,
		IsManual:    query.IsManual,
		Label:       query.Label,
		SkipRecords: query.GetSkip(),
		PageSize:    query.GetPageSize(),
		Type:        query.Type,
	})
	if err != nil {
		return nil, 0, err
	}
	if shouldSanitize {
		for idx, bp := range blueprints {
			if err := SanitizeBlueprint(bp); err != nil {
				return nil, 0, errors.Convert(err)
			} else {
				blueprints[idx] = bp
			}
		}
	}
	return blueprints, count, nil
}

// GetBlueprint returns the detail of a given Blueprint ID
func GetBlueprint(blueprintId uint64, shouldSanitize bool) (*models.Blueprint, errors.Error) {
	blueprint, err := bpManager.GetDbBlueprint(blueprintId)
	if err != nil {
		if db.IsErrorNotFound(err) {
			return nil, errors.NotFound.New("blueprint not found")
		}
		return nil, errors.Internal.Wrap(err, "error getting the blueprint from database")
	}
	if shouldSanitize {
		if err := SanitizeBlueprint(blueprint); err != nil {
			return nil, errors.Convert(err)
		}
	}
	return blueprint, nil
}

// GetBlueprintByProjectName returns the detail of a given ProjectName
func GetBlueprintByProjectName(projectName string) (*models.Blueprint, errors.Error) {
	if projectName == "" {
		return nil, errors.Internal.New("can not use the empty projectName to search the unique blueprint")
	}
	blueprint, err := bpManager.GetDbBlueprintByProjectName(projectName)
	if err != nil {
		// Allow specific projectName to fail to find the corresponding blueprint
		if db.IsErrorNotFound(err) {
			return nil, nil
		}
		return nil, errors.Internal.Wrap(err, fmt.Sprintf("error getting the blueprint from database with project %s", projectName))
	}
	return blueprint, nil
}

func validateBlueprintAndMakePlan(blueprint *models.Blueprint) errors.Error {
	// validation
	err := vld.Struct(blueprint)
	if err != nil {
		return errors.BadInput.WrapRaw(err)
	}

	// checking if the project exist
	if blueprint.ProjectName != "" {
		_, err := GetProject(blueprint.ProjectName)
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("invalid projectName: [%s] for the blueprint [%s]", blueprint.ProjectName, blueprint.Name))
		}

		bp, err := GetBlueprintByProjectName(blueprint.ProjectName)
		if err != nil {
			return err
		}
		if bp != nil {
			if bp.ID != blueprint.ID {
				return errors.Default.New(fmt.Sprintf("Each project can only be used by one blueprint. The currently selected projectName: [%s] has been used by blueprint: [id:%d] [name:%s] and cannot be reused.", bp.ProjectName, bp.ID, bp.Name))
			}
		}
	}

	if strings.ToLower(blueprint.CronConfig) == "manual" {
		blueprint.IsManual = true
	}
	if !blueprint.IsManual {
		_, err = cron.ParseStandard(blueprint.CronConfig)
		if err != nil {
			return errors.Default.Wrap(err, "invalid cronConfig")
		}
	}
	if blueprint.Mode == models.BLUEPRINT_MODE_ADVANCED {
		if len(blueprint.Plan) == 0 {
			return errors.BadInput.New("invalid plan")
		}
	} else if blueprint.Mode == models.BLUEPRINT_MODE_NORMAL {
		var e errors.Error
		blueprint.Plan, e = MakePlanForBlueprint(blueprint, &blueprint.SyncPolicy)
		if e != nil {
			return e
		}
	}
	return nil
}

func saveBlueprint(blueprint *models.Blueprint) (*models.Blueprint, errors.Error) {
	// validation
	err := validateBlueprintAndMakePlan(blueprint)
	if err != nil {
		return nil, errors.BadInput.WrapRaw(err)
	}
	err = bpManager.SaveDbBlueprint(blueprint)
	if err != nil {
		return nil, err
	}

	// reload schedule
	err = reloadBlueprint(blueprint)
	if err != nil {
		return nil, err
	}
	// done
	return blueprint, nil
}

// PatchBlueprint FIXME ...
func PatchBlueprint(id uint64, body map[string]interface{}) (*models.Blueprint, errors.Error) {
	// load record from db
	blueprint, err := GetBlueprint(id, false)
	if err != nil {
		return nil, err
	}

	originMode := blueprint.Mode
	err = helper.DecodeMapStruct(body, blueprint, true)
	if err != nil {
		return nil, err
	}

	// make sure mode is not being updated
	if originMode != blueprint.Mode {
		return nil, errors.Default.New("mode is not updatable")
	}
	// syncPolicy can be updated, so we need to decode it again
	err = helper.DecodeMapStruct(body, &blueprint.SyncPolicy, true)
	if err != nil {
		return nil, err
	}
	if blueprint.SyncPolicy.TimeAfter != nil && blueprint.SyncPolicy.TimeAfter.IsZero() {
		blueprint.SyncPolicy.TimeAfter = nil
	}

	blueprint, err = saveBlueprint(blueprint)
	if err != nil {
		return nil, err
	}
	if err := SanitizeBlueprint(blueprint); err != nil {
		return nil, errors.Convert(err)
	}
	return blueprint, nil
}

// DeleteBlueprint FIXME ...
func DeleteBlueprint(id uint64) errors.Error {
	bp, err := bpManager.GetDbBlueprint(id)
	if err != nil {
		return err
	}
	pipelinesAreUnfinished, err := thereAreUnfinishedPipelinesUnderBlueprint(bp.ID)
	if err != nil {
		return err
	}
	if pipelinesAreUnfinished {
		return errors.Default.New("There are unfinished pipelines in the current project. It cannot be deleted at this time.")
	}
	err = bpManager.DeleteBlueprint(bp.ID)
	if err != nil {
		return errors.Default.Wrap(err, "Failed to delete the blueprint")
	}
	return nil
}

var blueprintReloadLock sync.Mutex
var bpCronIdMap map[uint64]cron.EntryID

// ReloadBlueprints reloads cronjobs based on blueprints
func ReloadBlueprints() (err errors.Error) {
	enable := true
	isManual := false
	blueprints, _, err := bpManager.GetDbBlueprints(&services.GetBlueprintQuery{
		Enable:   &enable,
		IsManual: &isManual,
	})
	if err != nil {
		return err
	}
	for _, e := range cronManager.Entries() {
		cronManager.Remove(e.ID)
	}
	cronManager.Stop()
	bpCronIdMap = make(map[uint64]cron.EntryID, len(blueprints))
	for _, blueprint := range blueprints {
		err := reloadBlueprint(blueprint)
		if err != nil {
			return err
		}
	}
	cronManager.Start()
	logger.Info("total %d blueprints were scheduled", len(blueprints))
	return nil
}

func reloadBlueprint(blueprint *models.Blueprint) errors.Error {
	// preventing concurrent reloads. It would be better to use Table Lock , however, it requires massive refactor
	// like the `bpManager` must accept transaction. Use mutex as a temporary fix.
	blueprintReloadLock.Lock()
	defer blueprintReloadLock.Unlock()

	cronId, scheduled := bpCronIdMap[blueprint.ID]
	if scheduled {
		cronManager.Remove(cronId)
		delete(bpCronIdMap, blueprint.ID)
		logger.Info("removed blueprint %d from cronjobs, cron id: %v", blueprint.ID, cronId)
	}
	if blueprint.Enable && !blueprint.IsManual {
		if cronId, err := cronManager.AddJob(blueprint.CronConfig, &BlueprintJob{blueprint}); err != nil {
			blueprintLog.Error(err, failToCreateCronJob)
			return errors.Default.Wrap(err, "created cron job failed")
		} else {
			bpCronIdMap[blueprint.ID] = cronId
			logger.Info("added blueprint %d to cronjobs, cron id: %v, cron config: %s", blueprint.ID, cronId, blueprint.CronConfig)
		}
	}
	return nil
}

func createPipelineByBlueprint(blueprint *models.Blueprint, syncPolicy *models.SyncPolicy) (*models.Pipeline, errors.Error) {
	var plan models.PipelinePlan
	var err errors.Error
	if blueprint.Mode == models.BLUEPRINT_MODE_NORMAL {
		plan, err = MakePlanForBlueprint(blueprint, syncPolicy)
		if err != nil {
			blueprintLog.Error(err, fmt.Sprintf("failed to MakePlanForBlueprint on blueprint:[%d][%s]", blueprint.ID, blueprint.Name))
			return nil, err
		}
	} else {
		plan = blueprint.Plan
	}

	newPipeline := models.NewPipeline{}
	newPipeline.Plan = plan
	newPipeline.Name = blueprint.Name
	newPipeline.BlueprintId = blueprint.ID
	newPipeline.Labels = blueprint.Labels
	newPipeline.Priority = blueprint.Priority
	newPipeline.SyncPolicy = blueprint.SyncPolicy

	// if the plan is empty, we should not create the pipeline
	// var shouldCreatePipeline bool
	// for _, stage := range plan {
	// 	for _, task := range stage {
	// 		switch task.Plugin {
	// 		case "org", "refdiff", "dora":
	// 		default:
	// 			if !plan.IsEmpty() {
	// 				shouldCreatePipeline = true
	// 			}
	// 		}
	// 	}
	// }
	// if !shouldCreatePipeline {
	// 	return nil, ErrEmptyPlan
	// }
	pipeline, err := CreatePipeline(&newPipeline, false)
	// Return all created tasks to the User
	if err != nil {
		blueprintLog.Error(err, fmt.Sprintf("%s on blueprint:[%d][%s]", failToCreateCronJob, blueprint.ID, blueprint.Name))
		return nil, errors.Convert(err)
	}
	return pipeline, nil
}

// MakePlanForBlueprint generates pipeline plan by version
func MakePlanForBlueprint(blueprint *models.Blueprint, syncPolicy *models.SyncPolicy) (models.PipelinePlan, errors.Error) {
	var plan models.PipelinePlan
	// load project metric plugins and convert it to a map
	metrics := make(map[string]json.RawMessage)
	projectMetrics := make([]models.ProjectMetricSetting, 0)
	if blueprint.ProjectName != "" {
		err := db.All(&projectMetrics, dal.Where("project_name = ? AND enable = ?", blueprint.ProjectName, true))
		if err != nil {
			return nil, err
		}
		for _, projectMetric := range projectMetrics {
			metrics[projectMetric.PluginName] = projectMetric.PluginOption
		}
	}
	skipCollectors := false
	if syncPolicy != nil && syncPolicy.SkipCollectors {
		skipCollectors = true
	}
	plan, err := GeneratePlanJsonV200(blueprint.ProjectName, blueprint.Connections, metrics, skipCollectors)
	if err != nil {
		return nil, err
	}
	return SequentializePipelinePlans(blueprint.BeforePlan, plan, blueprint.AfterPlan), nil
}

// ParallelizePipelinePlans merges multiple pipelines into one unified plan
// by assuming they can be executed in parallel
func ParallelizePipelinePlans(plans ...models.PipelinePlan) models.PipelinePlan {
	merged := make(models.PipelinePlan, 0)
	// iterate all pipelineTasks and try to merge them into `merged`
	for _, plan := range plans {
		// add all stages from plan to merged
		for index, stage := range plan {
			if index >= len(merged) {
				merged = append(merged, nil)
			}
			// add all tasks from plan to target respectively
			merged[index] = append(merged[index], stage...)
		}
	}
	return merged
}

// SequentializePipelinePlans merges multiple pipelines into one unified plan
// by assuming they must be executed in sequential order
func SequentializePipelinePlans(plans ...models.PipelinePlan) models.PipelinePlan {
	merged := make(models.PipelinePlan, 0)
	// iterate all pipelineTasks and try to merge them into `merged`
	for _, plan := range plans {
		merged = append(merged, plan...)
	}
	return merged
}

// TriggerBlueprint triggers blueprint immediately
func TriggerBlueprint(id uint64, triggerSyncPolicy *models.TriggerSyncPolicy, shouldSanitize bool) (*models.Pipeline, errors.Error) {
	// load record from db
	blueprint, err := GetBlueprint(id, false)
	if err != nil {
		logger.Error(err, "GetBlueprint, id: %d", id)
		return nil, err
	}
	if !blueprint.Enable {
		return nil, errors.BadInput.New("blueprint is not enabled")
	}
	blueprint.SkipCollectors = triggerSyncPolicy.SkipCollectors
	blueprint.FullSync = triggerSyncPolicy.FullSync
	pipeline, err := createPipelineByBlueprint(blueprint, &models.SyncPolicy{
		SkipOnFail:        false,
		TimeAfter:         nil,
		TriggerSyncPolicy: *triggerSyncPolicy,
	})
	if err != nil {
		return nil, err
	}
	if shouldSanitize {
		if err := SanitizePipeline(pipeline); err != nil {
			return nil, errors.Convert(err)
		}
	}
	return pipeline, nil
}
