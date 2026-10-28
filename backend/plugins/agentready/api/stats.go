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
	"net/http"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/agentready/models"
)

type AggResult struct {
	Total    int64   `gorm:"column:total"`
	AvgScore float64 `gorm:"column:avg_score"`
}

type CertCount struct {
	CertificationLevel string `gorm:"column:certification_level" json:"certificationLevel"`
	Count              int64  `gorm:"column:count" json:"count"`
}

func GetStats(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	db := basicRes.GetDal()

	connectionId, pErr := parseConnectionId(input.Params)
	if pErr != nil {
		return nil, errors.BadInput.Wrap(pErr, "invalid connectionId")
	}

	var baseClauses []dal.Clause

	if projectName := input.Query.Get("projectName"); projectName != "" {
		baseClauses = []dal.Clause{
			dal.From("_tool_agentready_assessments a"),
			dal.Join("JOIN project_mapping pm ON a.repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ? AND a.connection_id = ?", projectName, models.ProjectMappingTable, connectionId),
		}
	} else {
		baseClauses = []dal.Clause{
			dal.From(&models.AgentReadyAssessment{}),
			dal.Where("connection_id = ?", connectionId),
		}
	}
	baseClauses = append(baseClauses, dal.Where("id != ''"))

	aggClauses := make([]dal.Clause, len(baseClauses))
	copy(aggClauses, baseClauses)
	aggClauses = append(aggClauses,
		dal.Select("COUNT(*) as total, AVG(overall_score) as avg_score"),
	)

	var agg AggResult
	err := db.First(&agg, aggClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get assessment stats")
	}

	var certCounts []CertCount
	certClauses := make([]dal.Clause, len(baseClauses))
	copy(certClauses, baseClauses)
	certClauses = append(certClauses,
		dal.Select("certification_level, COUNT(*) as count"),
		dal.Groupby("certification_level"),
	)
	err = db.All(&certCounts, certClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get certification distribution")
	}

	certDist := make(map[string]int, len(certCounts))
	for _, c := range certCounts {
		certDist[c.CertificationLevel] = int(c.Count)
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"totalAssessments":          agg.Total,
			"averageScore":              agg.AvgScore,
			"certificationDistribution": certDist,
		},
		Status: http.StatusOK,
	}, nil
}
