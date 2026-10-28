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
	"github.com/apache/devlake/helpers/migrationhelper"
)

// codecovCommit20260615 is a frozen snapshot of models.CodecovCommit as of 2026-06-15.
// Do not update this struct — create a new migration instead.
type codecovCommit20260615 struct {
	// common.NoPKModel fields
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId uint64 `gorm:"primaryKey;type:bigint"`
	RepoId       string `gorm:"primaryKey;type:varchar(200);index"`
	CommitSha    string `gorm:"primaryKey;type:varchar(64);index"`
	// fields
	Branch          string     `gorm:"type:varchar(100)"`
	CommitTimestamp *time.Time `gorm:"index"`
	Message         string     `gorm:"type:text"`
	Author          string     `gorm:"type:varchar(255)"`
	ParentSha       string     `gorm:"type:varchar(64)"`
}

func (codecovCommit20260615) TableName() string {
	return "_tool_codecov_commits"
}

type fixCommitsDedupAndDropId struct{}

// Up deduplicates _tool_codecov_commits and ensures the primary key is
// (connection_id, repo_id, commit_sha) without an auto_increment id column.
//
// In production the table has an id AUTO_INCREMENT column that prevents
// ON DUPLICATE KEY UPDATE from firing. In dev databases the id column may
// not exist. This migration handles both cases.
func (u *fixCommitsDedupAndDropId) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	logger := basicRes.GetLogger()

	// Check if the id column exists by trying to select it
	idExists := true
	err := db.Exec(`SELECT id FROM _tool_codecov_commits LIMIT 0`)
	if err != nil {
		idExists = false
		logger.Info("_tool_codecov_commits has no id column, skipping id-based dedup")
	}

	if idExists {
		logger.Info("_tool_codecov_commits has id column — deduplicating and dropping it")

		err = db.Exec(`
			DELETE t1 FROM _tool_codecov_commits t1
			INNER JOIN _tool_codecov_commits t2
			ON t1.connection_id = t2.connection_id
				AND t1.repo_id = t2.repo_id
				AND t1.commit_sha = t2.commit_sha
				AND t1.id < t2.id
		`)
		if err != nil {
			return err
		}

		err = db.Exec(`ALTER TABLE _tool_codecov_commits MODIFY id BIGINT UNSIGNED NOT NULL`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE _tool_codecov_commits DROP PRIMARY KEY`)
		if err != nil {
			return err
		}
		err = db.Exec(`ALTER TABLE _tool_codecov_commits DROP COLUMN id`)
		if err != nil {
			return err
		}
	}

	// Use AutoMigrate to ensure the table matches the current model definition,
	// which has PK on (connection_id, repo_id, commit_sha) and no id column.
	return migrationhelper.AutoMigrateTables(basicRes, &codecovCommit20260615{})
}

func (*fixCommitsDedupAndDropId) Version() uint64 {
	return 20260615000000
}

func (*fixCommitsDedupAndDropId) Name() string {
	return "Codecov fix commits table: deduplicate rows and drop auto_increment id to enable upserts"
}
