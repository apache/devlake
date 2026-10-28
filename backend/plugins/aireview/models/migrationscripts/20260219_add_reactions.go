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

var _ plugin.MigrationScript = (*addReactions)(nil)

type addReactions struct{}

func (script *addReactions) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	err := db.AutoMigrate(&aiReviewAddReactions20260219{})
	if err != nil {
		return errors.Default.Wrap(err, "failed to add reaction columns to _tool_aireview_reviews")
	}

	return nil
}

func (script *addReactions) Version() uint64 {
	return 20260219000002
}

func (script *addReactions) Name() string {
	return "aireview add developer reaction columns"
}

type aiReviewAddReactions20260219 struct {
	ReactionsTotalCount int `gorm:"default:0"`
	ReactionsThumbsUp   int `gorm:"default:0"`
	ReactionsThumbsDown int `gorm:"default:0"`
}

func (aiReviewAddReactions20260219) TableName() string {
	return "_tool_aireview_reviews"
}
