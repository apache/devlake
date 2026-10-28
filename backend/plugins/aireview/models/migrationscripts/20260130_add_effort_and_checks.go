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

var _ plugin.MigrationScript = (*addEffortAndChecks)(nil)

type addEffortAndChecks struct{}

func (script *addEffortAndChecks) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Add effort_rating column to _tool_aireview_reviews
	err := db.AutoMigrate(&aiReviewAddColumns20260130{})
	if err != nil {
		return errors.Default.Wrap(err, "failed to add new columns to _tool_aireview_reviews")
	}

	return nil
}

func (script *addEffortAndChecks) Version() uint64 {
	return 20260130000001
}

func (script *addEffortAndChecks) Name() string {
	return "aireview add effort rating and pre-merge check columns"
}

// aiReviewAddColumns20260130 represents the table structure with new columns
type aiReviewAddColumns20260130 struct {
	EffortRating               int `gorm:"default:0"`
	PreMergeChecksPassed       int `gorm:"default:0"`
	PreMergeChecksFailed       int `gorm:"default:0"`
	PreMergeChecksInconclusive int `gorm:"default:0"`
}

func (aiReviewAddColumns20260130) TableName() string {
	return "_tool_aireview_reviews"
}
