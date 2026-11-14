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

package tasks

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"strconv"
	"unicode"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/sonarqube/models"
)

func CreateRawDataSubTaskArgs(taskCtx plugin.SubTaskContext, rawTable string) (*api.RawDataSubTaskArgs, *SonarqubeTaskData) {
	data := taskCtx.GetData().(*SonarqubeTaskData)
	filteredData := *data
	filteredData.Options = &SonarqubeOptions{}
	*filteredData.Options = *data.Options
	var params = models.SonarqubeApiParams{
		ConnectionId: data.Options.ConnectionId,
		ProjectKey:   data.Options.ProjectKey,
	}
	rawDataSubTaskArgs := &api.RawDataSubTaskArgs{
		Ctx:    taskCtx,
		Params: params,
		Table:  rawTable,
	}
	return rawDataSubTaskArgs, &filteredData
}

func GetTotalPagesFromResponse(res *http.Response, args *api.ApiCollectorArgs) (int, errors.Error) {
	body := &SonarqubePagination{}
	err := api.UnmarshalResponse(res, body)
	if err != nil {
		return 0, err
	}
	pages := body.Paging.Total / args.PageSize
	if body.Paging.Total%args.PageSize > 0 {
		pages++
	}
	if pages > 100 {
		pages = 100
	}
	return pages, nil
}

type SonarqubePagination struct {
	Paging Paging `json:"paging"`
}
type Paging struct {
	PageIndex int `json:"pageIndex"`
	PageSize  int `json:"pageSize"`
	Total     int `json:"total"`
}

func generateId(hashCodeBlock hash.Hash, entity *models.SonarqubeIssueCodeBlock) {
	// the hash is shared by all code blocks of the subtask, so clear the previous input first
	hashCodeBlock.Reset()
	hashCodeBlock.Write([]byte(fmt.Sprintf("%s-%s-%d-%d-%d-%d-%s", entity.IssueKey, entity.Component, entity.StartLine, entity.EndLine, entity.StartOffset, entity.EndOffset, entity.Msg)))
	entity.Id = hex.EncodeToString(hashCodeBlock.Sum(nil))
}

func hashString(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

func convertTimeToMinutes(timeStr string) int {
	days := 0
	hours := 0
	minutes := 0

	var currentNum int
	var currentUnit string

	for i := 0; i < len(timeStr); i++ {
		c := timeStr[i]

		if unicode.IsDigit(rune(c)) {
			currentNum = currentNum*10 + int(c-'0')
		} else {
			currentUnit += string(c)
			if currentUnit == "d" {
				days = currentNum
			} else if currentUnit == "h" {
				hours = currentNum
			} else if currentUnit == "min" {
				minutes = currentNum
			} else {
				continue
			}
			currentNum = 0
			currentUnit = ""
		}
	}

	totalMinutes := days*8*60 + hours*60 + minutes
	return totalMinutes
}

var alphabetMap = map[string]string{
	"1.0": "A",
	"2.0": "B",
	"3.0": "C",
	"4.0": "D",
	"5.0": "E",
}

type fileMetricsResponse struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Qualifier string    `json:"qualifier"`
	Path      string    `json:"path"`
	Language  string    `json:"language"`
	Measures  []Measure `json:"measures"`
}
type Measure struct {
	Metric    string `json:"metric"`
	Value     string `json:"value"`
	BestValue bool   `json:"bestValue,omitempty"`
}

// As we have many metrics, we cannot
func setMetrics(fileMetrics *models.SonarqubeFileMetrics, metricsList []Measure) errors.Error {
	var err errors.Error
	for _, v := range metricsList {
		switch v.Metric {
		case "sqale_index":
			fileMetrics.SqaleIndex, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "sqale_rating":
			fileMetrics.SqaleRating, err = errors.Convert01(strconv.ParseFloat(v.Value, 32))
			if err != nil {
				return err
			}
		case "reliability_rating":
			fileMetrics.ReliabilityRating = alphabetMap[v.Value]
		case "security_rating":
			fileMetrics.SecurityRating = alphabetMap[v.Value]
		case "security_review_rating":
			fileMetrics.SecurityReviewRating = alphabetMap[v.Value]
		case "ncloc":
			fileMetrics.Ncloc, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "code_smells":
			fileMetrics.CodeSmells, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "bugs":
			fileMetrics.Bugs, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "vulnerabilities":
			fileMetrics.Vulnerabilities, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "security_hotspots":
			fileMetrics.SecurityHotspots, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "security_hotspots_reviewed":
			fileMetrics.SecurityHotspotsReviewed, err = errors.Convert01(strconv.ParseFloat(v.Value, 32))
			if err != nil {
				return err
			}
		case "uncovered_lines":
			fileMetrics.UncoveredLines, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "lines_to_cover":
			fileMetrics.LinesToCover, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "coverage":
			fileMetrics.Coverage, err = errors.Convert01(strconv.ParseFloat(v.Value, 32))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// As we have many metrics, we cannot
func setAdditionalMetrics(fileMetrics *models.SonarqubeAdditionalFileMetrics, metricsList []Measure) errors.Error {
	var err errors.Error
	for _, v := range metricsList {
		switch v.Metric {
		case "duplicated_blocks":
			fileMetrics.DuplicatedBlocks, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "duplicated_lines":
			fileMetrics.DuplicatedLines, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "duplicated_files":
			fileMetrics.DuplicatedFiles, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "duplicated_lines_density":
			fileMetrics.DuplicatedLinesDensity, err = errors.Convert01(strconv.ParseFloat(v.Value, 32))
			if err != nil {
				return err
			}
		case "complexity":
			fileMetrics.Complexity, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "cognitive_complexity":
			fileMetrics.CognitiveComplexity, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "effort_to_reach_maintainability_rating_a":
			fileMetrics.EffortToReachMaintainabilityRatingA, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}
		case "lines":
			fileMetrics.NumOfLines, err = errors.Convert01(strconv.Atoi(v.Value))
			if err != nil {
				return err
			}

		}
	}
	return nil
}
