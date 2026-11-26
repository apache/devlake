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

package e2e

import (
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/org/api"
	"gorm.io/gorm/schema"
)

type uploadSnapshot struct {
	table        schema.Tabler
	csvRelPath   string
	targetFields []string
}

// A failed upload must keep the existing rows, and a later upload must still work.
// Each too_long file ends with a row whose value has 300 characters. The column is
// varchar(255), so MySQL in strict mode (the default) and PostgreSQL reject the row.
func TestCsvUploadFailureChangesNothing(t *testing.T) {
	tests := []struct {
		name        string
		upload      func(h *api.Handlers, input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error)
		validFile   string
		tooLongFile string
		snapshots   []uploadSnapshot
	}{
		{
			name:        "teams",
			upload:      (*api.Handlers).CreateTeam,
			validFile:   "./raw_tables/upload_teams.csv",
			tooLongFile: "./raw_tables/upload_teams_too_long.csv",
			snapshots: []uploadSnapshot{
				{crossdomain.Team{}, "./snapshot_tables/upload_teams.csv", []string{"id", "name", "alias", "parent_id", "sorting_index"}},
			},
		},
		{
			name:        "users",
			upload:      (*api.Handlers).CreateUser,
			validFile:   "./raw_tables/upload_users.csv",
			tooLongFile: "./raw_tables/upload_users_too_long.csv",
			snapshots: []uploadSnapshot{
				{crossdomain.User{}, "./snapshot_tables/upload_users.csv", []string{"id", "name", "email"}},
				{crossdomain.TeamUser{}, "./snapshot_tables/upload_team_users.csv", []string{"team_id", "user_id"}},
			},
		},
		{
			name:        "user account mapping",
			upload:      (*api.Handlers).CreateUserAccountMapping,
			validFile:   "./raw_tables/upload_user_account_mapping.csv",
			tooLongFile: "./raw_tables/upload_user_account_mapping_too_long.csv",
			snapshots: []uploadSnapshot{
				{crossdomain.UserAccount{}, "./snapshot_tables/upload_user_accounts.csv", []string{"account_id", "user_id"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataflowTester, handlers := newUploadHandlers(t)
			verify := func() {
				for _, s := range tt.snapshots {
					dataflowTester.VerifyTable(s.table, s.csvRelPath, s.targetFields)
				}
			}
			for _, s := range tt.snapshots {
				dataflowTester.FlushTabler(s.table)
			}

			if _, err := tt.upload(handlers, uploadInput(t, tt.validFile)); err != nil {
				t.Fatal(err)
			}
			verify()

			if _, err := tt.upload(handlers, uploadInput(t, tt.tooLongFile)); err == nil {
				t.Error("the upload with a value that is too long must return an error")
			}
			verify()

			if _, err := tt.upload(handlers, uploadInput(t, tt.validFile)); err != nil {
				t.Fatalf("an upload after a failed upload must work: %v", err)
			}
			verify()
		})
	}
}
