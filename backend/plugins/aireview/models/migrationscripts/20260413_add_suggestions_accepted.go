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

var _ plugin.MigrationScript = (*addSuggestionsAccepted)(nil)

type addSuggestionsAccepted struct{}

// Up adds suggestions_accepted column to _tool_aireview_reviews.
func (script *addSuggestionsAccepted) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	if err := db.AutoMigrate(&reviewSuggestionsAccepted20260413{}); err != nil {
		return errors.Default.Wrap(err, "failed to migrate _tool_aireview_reviews for suggestions_accepted")
	}
	return nil
}

func (script *addSuggestionsAccepted) Version() uint64 {
	return 20260413000001
}

func (script *addSuggestionsAccepted) Name() string {
	return "aireview add suggestions_accepted to reviews"
}

type reviewSuggestionsAccepted20260413 struct {
	SuggestionsAccepted int `gorm:"default:0"`
}

func (reviewSuggestionsAccepted20260413) TableName() string {
	return "_tool_aireview_reviews"
}
