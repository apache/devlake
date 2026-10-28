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
	"fmt"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
)

type dedupRawTables struct{}

// dedupOneTable deduplicates a raw table by keeping only one row per unique
// (params, input) combination (the one with the highest id). It uses a
// create-new / atomic-rename strategy to avoid long-running deletes and
// heavy row-level locks on production.
func dedupOneTable(basicRes context.BasicRes, table string) errors.Error {
	db := basicRes.GetDal()
	logger := basicRes.GetLogger()

	if !db.HasTable(table) {
		logger.Info("[dedup-raw] table %s does not exist, skipping", table)
		return nil
	}

	newTable := table + "_dedup"
	oldTable := table + "_old"

	// Clean up any leftover temp tables from a previous interrupted run.
	// These DROPs use IF EXISTS so the only real failure mode is permissions/locks.
	if dropErr := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", newTable)); dropErr != nil {
		logger.Warn(dropErr, "[dedup-raw] failed to drop leftover %s", newTable)
	}
	if dropErr := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", oldTable)); dropErr != nil {
		logger.Warn(dropErr, "[dedup-raw] failed to drop leftover %s", oldTable)
	}

	logger.Info("[dedup-raw] deduplicating %s — creating clean copy", table)

	// 1. Create a new table with the same schema (no data)
	err := db.Exec(fmt.Sprintf("CREATE TABLE `%s` LIKE `%s`", newTable, table))
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to create %s", newTable))
	}

	// 2. Copy only unique rows — keep the latest (MAX id) per (params, input)
	err = db.Exec(fmt.Sprintf(`
		INSERT INTO %s
		SELECT t.* FROM %s t
		INNER JOIN (
			SELECT MAX(id) AS max_id
			FROM %s
			GROUP BY params, CAST(input AS CHAR)
		) keep ON t.id = keep.max_id
	`, newTable, table, table))
	if err != nil {
		if cleanupErr := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", newTable)); cleanupErr != nil {
			logger.Warn(cleanupErr, "[dedup-raw] also failed to clean up %s after INSERT error", newTable)
		}
		return errors.Default.Wrap(err, fmt.Sprintf("failed to copy unique rows into %s", newTable))
	}

	// 3. Atomic rename: original → backup, new → original
	err = db.Exec(fmt.Sprintf("RENAME TABLE `%s` TO `%s`, `%s` TO `%s`",
		table, oldTable, newTable, table))
	if err != nil {
		if cleanupErr := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", newTable)); cleanupErr != nil {
			logger.Warn(cleanupErr, "[dedup-raw] also failed to clean up %s after RENAME error", newTable)
		}
		return errors.Default.Wrap(err, fmt.Sprintf("failed to rename tables for %s", table))
	}

	// 4. Drop the old bloated table to reclaim disk space
	err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", oldTable))
	if err != nil {
		logger.Warn(err, "[dedup-raw] could not drop %s — clean up manually", oldTable)
	}

	logger.Info("[dedup-raw] %s: dedup complete", table)
	return nil
}

func (u *dedupRawTables) Up(basicRes context.BasicRes) errors.Error {
	tables := []string{
		"_raw_codecov_api_commit_coverages",
		"_raw_codecov_api_comparisons",
		"_raw_codecov_api_commit_totals",
	}
	for _, t := range tables {
		if err := dedupOneTable(basicRes, t); err != nil {
			return err
		}
	}
	return nil
}

func (*dedupRawTables) Version() uint64 {
	return 20260622000000
}

func (*dedupRawTables) Name() string {
	return "Codecov deduplicate raw API tables to fix converter performance"
}
