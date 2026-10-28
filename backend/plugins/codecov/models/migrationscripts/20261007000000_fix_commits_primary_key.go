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
)

type fixCommitsPrimaryKey struct{}

// Up deduplicates _tool_codecov_commits and adds the missing composite
// primary key (connection_id, repo_id, commit_sha).
//
// Migration 20260615 dropped the old auto_increment id and called
// AutoMigrateTables, but MySQL's AutoMigrate does not reliably add a
// composite PK to an existing table. The result: no PK, so
// CreateOrUpdate always inserts, causing ~32x row duplication.
func (*fixCommitsPrimaryKey) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	logger := basicRes.GetLogger()

	// 1. Clean up leftover tables from a previous interrupted run (before any checks)
	if dropErr := db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_dedup`); dropErr != nil {
		logger.Warn(dropErr, "[fix-commits-pk] failed to drop leftover _dedup table")
	}
	if dropErr := db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_old`); dropErr != nil {
		logger.Warn(dropErr, "[fix-commits-pk] failed to drop leftover _old table")
	}

	// 2. Check if PK already exists (idempotent)
	rows, err := db.RawCursor(`
		SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = '_tool_codecov_commits'
		  AND CONSTRAINT_TYPE = 'PRIMARY KEY'
	`)
	if err == nil && rows != nil {
		defer rows.Close()
		if rows.Next() {
			var pkCount int
			if scanErr := rows.Scan(&pkCount); scanErr == nil && pkCount > 0 {
				logger.Info("[fix-commits-pk] PK already exists, skipping")
				return nil
			}
		}
	}

	// 3. Deduplicate — keep exactly one row per (connection_id, repo_id, commit_sha).
	// Uses ROW_NUMBER with a deterministic tie-breaker (updated_at DESC, created_at DESC)
	// to handle duplicates that share the same updated_at timestamp.
	logger.Info("[fix-commits-pk] Deduplicating _tool_codecov_commits")

	err = db.Exec(`CREATE TABLE _tool_codecov_commits_dedup LIKE _tool_codecov_commits`)
	if err != nil {
		return errors.Default.Wrap(err, "failed to create dedup table")
	}

	err = db.Exec(`
		INSERT INTO _tool_codecov_commits_dedup
			(created_at, updated_at, _raw_data_params, _raw_data_table,
			 _raw_data_id, _raw_data_remark, connection_id, repo_id,
			 commit_sha, branch, commit_timestamp, message, author, parent_sha)
		SELECT t.created_at, t.updated_at, t._raw_data_params, t._raw_data_table,
		       t._raw_data_id, t._raw_data_remark, t.connection_id, t.repo_id,
		       t.commit_sha, t.branch, t.commit_timestamp, t.message, t.author, t.parent_sha
		FROM (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY connection_id, repo_id, commit_sha
				ORDER BY updated_at DESC, created_at DESC
			) AS rn
			FROM _tool_codecov_commits
		) t WHERE t.rn = 1
	`)
	if err != nil {
		if dropErr := db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_dedup`); dropErr != nil {
			logger.Warn(dropErr, "[fix-commits-pk] also failed to clean up _dedup table after INSERT error")
		}
		return errors.Default.Wrap(err, "failed to copy unique rows")
	}

	// 4. Add PK on the replacement BEFORE swapping — validates uniqueness
	// and avoids exposing a keyless table if ALTER fails.
	err = db.Exec(`
		ALTER TABLE _tool_codecov_commits_dedup
		ADD PRIMARY KEY (connection_id, repo_id, commit_sha)
	`)
	if err != nil {
		if dropErr := db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_dedup`); dropErr != nil {
			logger.Warn(dropErr, "[fix-commits-pk] also failed to clean up _dedup table after PK error")
		}
		return errors.Default.Wrap(err, "failed to add primary key on dedup table")
	}

	// 5. Atomic rename: swap dedup into service
	err = db.Exec(`
		RENAME TABLE _tool_codecov_commits TO _tool_codecov_commits_old,
		             _tool_codecov_commits_dedup TO _tool_codecov_commits
	`)
	if err != nil {
		if dropErr := db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_dedup`); dropErr != nil {
			logger.Warn(dropErr, "[fix-commits-pk] also failed to clean up _dedup table after RENAME error")
		}
		return errors.Default.Wrap(err, "failed to rename tables")
	}

	// 6. Drop old bloated table
	if dropErr := db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_old`); dropErr != nil {
		logger.Warn(dropErr, "[fix-commits-pk] could not drop _old table — clean up manually")
	}

	logger.Info("[fix-commits-pk] Done — duplicates removed, PK added")
	return nil
}

func (*fixCommitsPrimaryKey) Version() uint64 {
	return 20261007000000
}

func (*fixCommitsPrimaryKey) Name() string {
	return "Codecov fix missing primary key on commits table and remove duplicates"
}
