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
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

type ProjectService interface {
	RenameProject(db dal.Transaction, oldProjectName, newProjectName string) errors.Error
}

var projectService ProjectService

// ProjectQuery used to query projects as the api project input
type ProjectQuery struct {
	Pagination
	Keyword *string `json:"keyword" form:"keyword"`
}

func (query *ProjectQuery) GetKeyword() string {
	if query != nil && query.Keyword != nil {
		return strings.ToLower(*query.Keyword)
	}
	return ""
}

// GetProjects returns a paginated list of Projects based on `query`
func GetProjects(query *ProjectQuery) ([]*models.ApiOutputProject, int64, errors.Error) {
	// verify input
	if err := VerifyStruct(query); err != nil {
		return nil, 0, err
	}
	clauses := []dal.Clause{
		dal.From(&models.Project{}),
	}
	if query.Keyword != nil {
		clauses = append(clauses, dal.Where("LOWER(name) LIKE ?", "%"+query.GetKeyword()+"%"))
	}

	count, err := db.Count(clauses...)
	if err != nil {
		return nil, 0, errors.Default.Wrap(err, "error getting DB count of project")
	}

	clauses = append(clauses,
		dal.Orderby("created_at DESC"),
		dal.Offset(query.GetSkip()),
		dal.Limit(query.GetPageSize()),
	)
	projects := make([]*models.Project, count)
	err = db.All(&projects, clauses...)
	if err != nil {
		return nil, 0, errors.Default.Wrap(err, "error finding DB project")
	}
	apiOutProjects := make([]*models.ApiOutputProject, len(projects))
	g := new(errgroup.Group)
	for idx, project := range projects {
		tmpProject := *project
		tmpIdx := idx
		g.Go(func() error {
			apiOutputProject, err := makeProjectOutput(&tmpProject, true)
			if err != nil {
				logger.Error(err, "makeProjectOutput, name: %s", tmpProject.Name)
				return errors.Default.Wrap(err, "error making project output")
			}
			apiOutProjects[tmpIdx] = apiOutputProject
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, 0, errors.Convert(err)
	}
	return apiOutProjects, count, nil
}

// CreateProject accepts a project instance and insert it to database
func CreateProject(projectInput *models.ApiInputProject) (*models.ApiOutputProject, errors.Error) {
	// verify input
	if err := VerifyStruct(projectInput); err != nil {
		return nil, err
	}

	// create transaction to update multiple tables
	var err errors.Error
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil || err != nil {
			err = tx.Rollback()
			if err != nil {
				logger.Error(err, "PatchProject: failed to rollback")
			}
		}
	}()

	// create project first
	project := &models.Project{}
	project.BaseProject = projectInput.BaseProject
	err = db.Create(project)
	if err != nil {
		if db.IsDuplicationError(err) {
			return nil, errors.BadInput.New(fmt.Sprintf("A project with name [%s] already exists", project.Name))
		}
		return nil, errors.Default.Wrap(err, "error creating DB project")
	}

	// check if we need flush the Metrics
	if len(projectInput.Metrics) > 0 {
		err = refreshProjectMetrics(tx, projectInput)
		if err != nil {
			return nil, err
		}
	}

	// create blueprint
	blueprint := &models.Blueprint{
		Name:        project.Name + "-Blueprint",
		ProjectName: project.Name,
		Mode:        "NORMAL",
		Enable:      true,
		CronConfig:  "0 0 * * *",
		IsManual:    false,
		SyncPolicy: models.SyncPolicy{
			TimeAfter: func() *time.Time {
				t := time.Now().AddDate(0, -6, 0)
				t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
				return &t
			}(),
		},
		Connections: nil,
	}
	if projectInput.Blueprint != nil {
		blueprint = projectInput.Blueprint
	}
	err = tx.Create(blueprint)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error creating DB blueprint")
	}

	// all good, commit transaction
	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	// reload schedule
	err = reloadBlueprint(blueprint)
	if err != nil {
		return nil, err
	}

	return makeProjectOutput(project, false)
}

// GetProject returns a Project
func GetProject(name string) (*models.ApiOutputProject, errors.Error) {
	// verify input
	if name == "" {
		return nil, errors.BadInput.New("project name is missing")
	}
	// load project
	project, err := getProjectByName(db, name)
	if err != nil {
		return nil, err
	}
	// convert to api output
	return makeProjectOutput(project, false)
}

// PatchProject FIXME ...
func PatchProject(name string, body map[string]interface{}) (*models.ApiOutputProject, errors.Error) {
	projectInput := &models.ApiInputProject{}

	// load input
	err := helper.DecodeMapStruct(body, projectInput, true)
	if err != nil {
		return nil, err
	}

	// wrap all operation inside a transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil || err != nil {
			err = tx.Rollback()
			if err != nil {
				logger.Error(err, "PatchProject: failed to rollback")
			}
		}
	}()

	project, err := getProjectByName(tx, name, dal.Lock(true, false))
	if err != nil {
		return nil, err
	}

	// allowed to changed the name
	if projectInput.Name == "" {
		projectInput.Name = name
	}
	project.BaseProject = projectInput.BaseProject

	// name changed, updates the related entities as well
	if name != project.Name {
		// ProjectMetric
		err = tx.UpdateColumn(
			&models.ProjectMetricSetting{},
			"project_name", project.Name,
			dal.Where("project_name = ?", name),
		)
		if err != nil {
			return nil, err
		}

		// ProjectPrMetric
		err = tx.UpdateColumn(
			&crossdomain.ProjectPrMetric{},
			"project_name", project.Name,
			dal.Where("project_name = ?", name),
		)
		if err != nil {
			return nil, err
		}

		// ProjectIncidentDeploymentRelationship
		err = tx.UpdateColumn(
			&crossdomain.ProjectIncidentDeploymentRelationship{},
			"project_name", project.Name,
			dal.Where("project_name = ?", name),
		)
		if err != nil {
			return nil, err
		}

		// ProjectMapping
		err = tx.UpdateColumn(
			&crossdomain.ProjectMapping{},
			"project_name", project.Name,
			dal.Where("project_name = ?", name),
		)
		if err != nil {
			return nil, err
		}

		// Blueprint
		err = tx.UpdateColumn(
			&models.Blueprint{},
			"project_name", project.Name,
			dal.Where("project_name = ?", name),
		)
		if err != nil {
			return nil, err
		}
		if projectService != nil {
			if err := projectService.RenameProject(tx, name, project.Name); err != nil {
				return nil, err
			}
		}
		// rename project
		err = tx.UpdateColumn(
			&models.Project{},
			"name", project.Name,
			dal.Where("name = ?", name),
		)
	}

	// Blueprint
	// err = tx.UpdateColumn(
	// 	&models.Blueprint{},
	// 	"enable", projectInput.Enable,
	// 	dal.Where("project_name = ?", name),
	// )
	// if err != nil {
	// 	return nil, err
	// }

	// refresh project metrics if needed
	if len(projectInput.Metrics) > 0 {
		err = refreshProjectMetrics(tx, projectInput)
		if err != nil {
			return nil, err
		}
	}

	// update project itself
	err = tx.Update(project)
	if err != nil {
		return nil, err
	}

	// commit the transaction
	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	// all good, render output
	return makeProjectOutput(project, false)
}

func thereAreUnfinishedPipelinesUnderProject(projectName string) (bool, errors.Error) {
	blueprint, err := GetBlueprintByProjectName(projectName)
	if err != nil {
		return false, err
	}
	return thereAreUnfinishedPipelinesUnderBlueprint(blueprint.ID)
}

func thereAreUnfinishedPipelinesUnderBlueprint(blueprintID uint64) (bool, errors.Error) {
	// get the first page
	dbPipelines, count, err := GetDbPipelines(&PipelineQuery{BlueprintId: blueprintID})
	if err != nil {
		return false, err
	}
	if count <= 0 {
		return false, nil
	}
	for _, pipeline := range dbPipelines {
		if !slices.Contains(models.FinishedTaskStatus, pipeline.Status) {
			return true, nil
		}
	}
	return false, nil
}

// DeleteProject FIXME ...
func DeleteProject(name string) errors.Error {
	// verify input
	if name == "" {
		return errors.BadInput.New("project name is missing")
	}
	// verify exists
	_, err := getProjectByName(db, name)
	if err != nil {
		return err
	}
	// make sure there is no running pipelines in current projects
	pipelinesAreUnfinished, err := thereAreUnfinishedPipelinesUnderProject(name)
	if err != nil {
		return err
	}
	if pipelinesAreUnfinished {
		return errors.Default.New("There are unfinished pipelines in the current project. It cannot be deleted at this time.")
	}
	err = deleteProjectBlueprint(name)
	if err != nil {
		return err
	}
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil || err != nil {
			err = tx.Rollback()
			if err != nil {
				logger.Error(err, "DeleteProject: failed to rollback")
			}
		}
	}()
	err = tx.Delete(&models.Project{}, dal.Where("name = ?", name))
	if err != nil {
		return errors.Default.Wrap(err, "error deleting project")
	}
	err = tx.Delete(&crossdomain.ProjectMapping{}, dal.Where("project_name = ?", name))
	if err != nil {
		return errors.Default.Wrap(err, "error deleting project")
	}
	err = tx.Delete(&models.ProjectMetricSetting{}, dal.Where("project_name = ?", name))
	if err != nil {
		return errors.Default.Wrap(err, "error deleting project metric setting")
	}
	err = tx.Delete(&crossdomain.ProjectPrMetric{}, dal.Where("project_name = ?", name))
	if err != nil {
		return errors.Default.Wrap(err, "error deleting project PR metric")
	}
	err = tx.Delete(&crossdomain.ProjectIncidentDeploymentRelationship{}, dal.Where("project_name = ?", name))
	if err != nil {
		return errors.Default.Wrap(err, "error deleting project Issue metric")
	}
	return tx.Commit()
}

func deleteProjectBlueprint(projectName string) errors.Error {
	bp, err := bpManager.GetDbBlueprintByProjectName(projectName)
	if err != nil {
		if !db.IsErrorNotFound(err) {
			return errors.Default.Wrap(err, fmt.Sprintf("error finding blueprint associated with project %s", projectName))
		}
	} else {
		err = bpManager.DeleteBlueprint(bp.ID)
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("error deleting blueprint associated with project %s", projectName))
		}
	}
	return nil
}

func getProjectByName(tx dal.Dal, name string, additionalClauses ...dal.Clause) (*models.Project, errors.Error) {
	project := &models.Project{}
	err := tx.First(project, append([]dal.Clause{dal.Where("name = ?", name)}, additionalClauses...)...)
	if err != nil {
		if tx.IsErrorNotFound(err) {
			return nil, errors.NotFound.Wrap(err, fmt.Sprintf("could not find project [%s] in DB", name))
		}
		return nil, errors.Default.Wrap(err, "error getting project from DB")
	}
	return project, nil
}

func refreshProjectMetrics(tx dal.Transaction, projectInput *models.ApiInputProject) errors.Error {
	err := tx.Delete(&models.ProjectMetricSetting{}, dal.Where("project_name = ?", projectInput.Name))
	if err != nil {
		return err
	}

	for _, baseMetric := range projectInput.Metrics {
		err = tx.Create(&models.ProjectMetricSetting{
			BaseProjectMetricSetting: models.BaseProjectMetricSetting{
				ProjectName: projectInput.Name,
				BaseMetric:  *baseMetric,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func makeProjectOutput(project *models.Project, withLastPipeline bool) (*models.ApiOutputProject, errors.Error) {
	projectOutput := &models.ApiOutputProject{}
	projectOutput.Project = *project
	// load project metrics
	projectMetrics := make([]models.ProjectMetricSetting, 0)
	err := db.All(&projectMetrics, dal.Where("project_name = ?", projectOutput.Name))
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to load project metrics")
	}
	// convert metric to api output
	if len(projectMetrics) > 0 {
		baseMetrics := make([]*models.BaseMetric, len(projectMetrics))
		for i, projectMetric := range projectMetrics {
			baseMetric := projectMetric.BaseMetric
			baseMetrics[i] = &baseMetric
		}
		projectOutput.Metrics = baseMetrics
	}

	// load blueprint
	projectOutput.Blueprint, err = GetBlueprintByProjectName(projectOutput.Name)
	if err != nil {
		return nil, errors.Default.Wrap(err, "Error to get blueprint by project")
	}
	if projectOutput.Blueprint != nil {
		if err := SanitizeBlueprint(projectOutput.Blueprint); err != nil {
			return nil, errors.Convert(err)
		}
	}
	if withLastPipeline {
		if projectOutput.Blueprint == nil {
			logger.Warn(fmt.Errorf("blueprint is nil"), "want to get latest pipeline, but blueprint is nil")
		} else {
			pipelines, pipelinesCount, err := GetPipelines(&PipelineQuery{
				BlueprintId: projectOutput.Blueprint.ID,
				Pagination: Pagination{
					PageSize: 1,
					Page:     1,
				},
			}, true)
			if err != nil {
				logger.Error(err, "GetPipelines, blueprint id: %d", projectOutput.Blueprint.ID)
				return nil, errors.Default.Wrap(err, "Error to get pipeline by blueprint id")
			}
			if pipelinesCount > 0 {
				projectOutput.LastPipeline = pipelines[0]
			}
		}
	}
	return projectOutput, err
}
