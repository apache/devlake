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

package codequality

import (
	"github.com/apache/incubator-devlake/core/models/domainlayer"
)

type CqFileMetrics struct {
	domainlayer.DomainEntity
	ProjectKey                          string `gorm:"index;type:varchar(500)"` //domain project key
	FileName                            string `gorm:"type:varchar(2000)"`
	FilePath                            string
	FileLanguage                        string `gorm:"type:varchar(20)"`
	CodeSmells                          int
	SqaleIndex                          int
	SqaleRating                         float64
	Bugs                                int
	ReliabilityRating                   string `gorm:"type:varchar(20)"`
	Vulnerabilities                     int
	SecurityRating                      string `gorm:"type:varchar(20)"`
	SecurityHotspots                    int
	SecurityHotspotsReviewed            float64
	SecurityReviewRating                string `gorm:"type:varchar(20)"`
	Ncloc                               int
	UncoveredLines                      int
	Coverage                            float64
	LinesToCover                        int
	DuplicatedLinesDensity              float64
	DuplicatedBlocks                    int
	DuplicatedFiles                     int
	DuplicatedLines                     int
	EffortToReachMaintainabilityRatingA int
	Complexity                          int
	CognitiveComplexity                 int
	NumOfLines                          int
}

func (CqFileMetrics) TableName() string {
	return "cq_file_metrics"
}
