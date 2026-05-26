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
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"net/url"
)

var _ plugin.MigrationScript = (*addIndexToGithubJobs)(nil)

type addIndexToGithubJobs struct{}

func (script *addIndexToGithubJobs) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	dbUrl := basicRes.GetConfig("DB_URL")
	if dbUrl == "" {
		return errors.BadInput.New("DB_URL is required")
	}
	u, errParse := url.Parse(dbUrl)
	if errParse != nil {
		return errors.Convert(errParse)
	}
	if u.Scheme == "mysql" {
		sql := "ALTER TABLE `_tool_github_jobs` ADD INDEX `idx_repo_id_connection_id` (`repo_id`, `connection_id`)"
		if err := db.Exec(sql); err != nil {
			return err
		}
	} else if u.Scheme == "postgres" || u.Scheme == "postgresql" {
		sql := "CREATE INDEX IF NOT EXISTS idx_repo_id_connection_id ON _tool_github_jobs (repo_id, connection_id)"
		if err := db.Exec(sql); err != nil {
			return err
		}
	}
	return nil
}

func (*addIndexToGithubJobs) Version() uint64 {
	return 20240821160000
}

func (*addIndexToGithubJobs) Name() string {
	return "add index to _tool_github_jobs"
}
