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

package migrationscripts

import (
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addDiffMatching)(nil)

type addDiffMatching struct{}

func (script *addDiffMatching) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	if err := db.AutoMigrate(&findingDiffMatching20260414{}); err != nil {
		return errors.Default.Wrap(err, "failed to migrate _tool_aireview_findings for diff matching")
	}
	if err := db.AutoMigrate(&reviewDiffAccepted20260414{}); err != nil {
		return errors.Default.Wrap(err, "failed to migrate _tool_aireview_reviews for diff accepted")
	}
	return nil
}

func (script *addDiffMatching) Version() uint64 {
	return 20260414000001
}

func (script *addDiffMatching) Name() string {
	return "aireview add diff-based suggestion matching fields"
}

type findingDiffMatching20260414 struct {
	SuggestionDiffMatched  bool    `gorm:"default:false"`
	SuggestionMatchMethod  string  `gorm:"type:varchar(50)"`
	SuggestionMatchScore   float64 `gorm:"default:0"`
	SuggestionLinesMatched int     `gorm:"default:0"`
	SuggestionLinesTotal   int     `gorm:"default:0"`
	MatchedCommitSha       string  `gorm:"type:varchar(40)"`
	MatchedFilePath        string  `gorm:"type:varchar(500)"`
}

func (findingDiffMatching20260414) TableName() string {
	return "_tool_aireview_findings"
}

type reviewDiffAccepted20260414 struct {
	SuggestionsDiffAccepted  int     `gorm:"default:0"`
	SuggestionsDiffAcceptPct float64 `gorm:"default:0"`
}

func (reviewDiffAccepted20260414) TableName() string {
	return "_tool_aireview_reviews"
}
