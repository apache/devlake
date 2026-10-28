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
	"net/http"
	"strconv"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/agentready/models"
)

func parseConnectionId(params map[string]string) (uint64, error) {
	raw, ok := params["connectionId"]
	if !ok || raw == "" {
		return 0, fmt.Errorf("missing connectionId")
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid connectionId %q: %w", raw, err)
	}
	if id == 0 {
		return 0, fmt.Errorf("connectionId must be > 0")
	}
	return id, nil
}

func GetAssessments(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	db := basicRes.GetDal()

	connectionId, pErr := parseConnectionId(input.Params)
	if pErr != nil {
		return nil, errors.BadInput.Wrap(pErr, "invalid connectionId")
	}

	page, _ := strconv.Atoi(input.Query.Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(input.Query.Get("pageSize"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var clauses []dal.Clause

	if projectName := input.Query.Get("projectName"); projectName != "" {
		clauses = []dal.Clause{
			dal.Select("DISTINCT a.*"),
			dal.From("_tool_agentready_assessments a"),
			dal.Join("JOIN project_mapping pm ON a.repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ? AND a.connection_id = ?", projectName, models.ProjectMappingTable, connectionId),
		}
	} else {
		clauses = []dal.Clause{
			dal.From(&models.AgentReadyAssessment{}),
			dal.Where("connection_id = ?", connectionId),
		}
		if repoId := input.Query.Get("repoId"); repoId != "" {
			clauses = append(clauses, dal.Where("repo_id = ?", repoId))
		}
	}

	if cert := input.Query.Get("certification"); cert != "" {
		clauses = append(clauses, dal.Where("certification_level = ?", cert))
	}

	countClauses := make([]dal.Clause, len(clauses))
	copy(countClauses, clauses)
	total, err := db.Count(countClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to count assessments")
	}

	clauses = append(clauses,
		dal.Orderby("assessed_at DESC"),
		dal.Limit(pageSize),
		dal.Offset(offset),
	)

	var assessments []models.AgentReadyAssessment
	err = db.All(&assessments, clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query assessments")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"assessments": assessments,
			"page":        page,
			"pageSize":    pageSize,
			"total":       total,
		},
		Status: http.StatusOK,
	}, nil
}

func GetAssessment(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	db := basicRes.GetDal()

	connectionId, pErr := parseConnectionId(input.Params)
	if pErr != nil {
		return nil, errors.BadInput.Wrap(pErr, "invalid connectionId")
	}

	id := input.Params["id"]
	var assessment models.AgentReadyAssessment
	err := db.First(&assessment, dal.Where("id = ? AND connection_id = ?", id, connectionId))
	if err != nil {
		if db.IsErrorNotFound(err) {
			return nil, errors.HttpStatus(http.StatusNotFound).Wrap(err, "assessment not found")
		}
		return nil, errors.Default.Wrap(err, "failed to get assessment")
	}

	var findings []models.AgentReadyFinding
	err = db.All(&findings,
		dal.From(&models.AgentReadyFinding{}),
		dal.Where("assessment_id = ? AND connection_id = ?", id, connectionId),
		dal.Orderby("tier ASC, status ASC"),
	)

	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query findings for assessment")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"assessment": assessment,
			"findings":   findings,
		},
		Status: http.StatusOK,
	}, nil
}

func GetAssessmentFindings(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	db := basicRes.GetDal()

	connectionId, pErr := parseConnectionId(input.Params)
	if pErr != nil {
		return nil, errors.BadInput.Wrap(pErr, "invalid connectionId")
	}

	assessmentId := input.Params["id"]

	clauses := []dal.Clause{
		dal.From(&models.AgentReadyFinding{}),
		dal.Where("assessment_id = ? AND connection_id = ?", assessmentId, connectionId),
	}

	if tier := input.Query.Get("tier"); tier != "" {
		clauses = append(clauses, dal.Where("tier = ?", tier))
	}
	if status := input.Query.Get("status"); status != "" {
		clauses = append(clauses, dal.Where("status = ?", status))
	}
	if category := input.Query.Get("category"); category != "" {
		clauses = append(clauses, dal.Where("category = ?", category))
	}

	clauses = append(clauses, dal.Orderby("tier ASC, status ASC"))

	var findings []models.AgentReadyFinding
	err := db.All(&findings, clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query findings")
	}

	return &plugin.ApiResourceOutput{
		Body:   findings,
		Status: http.StatusOK,
	}, nil
}
