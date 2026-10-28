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
	"time"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addAiCommitsDomain)(nil)

type addAiCommitsDomain struct{}

type archivedAiCommit20260826 struct {
	Id            string `gorm:"primaryKey;type:varchar(255)"`
	ProjectName   string `gorm:"index;type:varchar(255)"`
	CommitSha     string `gorm:"index;type:varchar(40)"`
	RepoId        string `gorm:"index;type:varchar(255)"`
	AiTool        string `gorm:"type:varchar(100)"`
	AuthorName    string `gorm:"type:varchar(255)"`
	AuthoredDate  time.Time
	CreatedAt     time.Time
	UpdatedAt     *time.Time
	RawDataParams string `gorm:"column:_raw_data_params;type:varchar(255);index"`
	RawDataTable  string `gorm:"column:_raw_data_table;type:varchar(255)"`
	RawDataId     uint64 `gorm:"column:_raw_data_id"`
	RawDataRemark string `gorm:"column:_raw_data_remark"`
}

func (archivedAiCommit20260826) TableName() string { return "ai_commits" }

func (*addAiCommitsDomain) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	if err := db.AutoMigrate(&archivedAiCommit20260826{}); err != nil {
		return err
	}
	// AutoMigrate often skips columns whose names start with '_'.
	alters := []struct{ column, sql string }{
		{"_raw_data_params", "ALTER TABLE `ai_commits` ADD COLUMN `_raw_data_params` varchar(255)"},
		{"_raw_data_table", "ALTER TABLE `ai_commits` ADD COLUMN `_raw_data_table` varchar(255)"},
		{"_raw_data_id", "ALTER TABLE `ai_commits` ADD COLUMN `_raw_data_id` bigint unsigned"},
		{"_raw_data_remark", "ALTER TABLE `ai_commits` ADD COLUMN `_raw_data_remark` longtext"},
	}
	for _, a := range alters {
		if db.HasColumn("ai_commits", a.column) {
			continue
		}
		if err := db.Exec(a.sql); err != nil {
			return errors.Default.Wrap(err, "failed: "+a.sql)
		}
	}
	return nil
}

func (*addAiCommitsDomain) Version() uint64 {
	return 20260826000001
}

func (*addAiCommitsDomain) Name() string {
	return "add ai_commits domain table"
}
