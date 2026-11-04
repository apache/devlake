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
	"testing"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
)

type tokenColumnCall struct {
	table, column, columnType string
}

type tokenColumnRecordingDal struct {
	dal.Dal
	calls []tokenColumnCall
}

func (d *tokenColumnRecordingDal) ModifyColumnType(table, column, columnType string) errors.Error {
	d.calls = append(d.calls, tokenColumnCall{table, column, columnType})
	return nil
}

type tokenColumnBasicRes struct {
	context.BasicRes
	database dal.Dal
}

func (r *tokenColumnBasicRes) GetDal() dal.Dal {
	return r.database
}

func TestChangeConnectionTokenToText(t *testing.T) {
	database := new(tokenColumnRecordingDal)
	script := new(changeConnectionTokenToText20261104)

	if err := script.Up(&tokenColumnBasicRes{database: database}); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	want := tokenColumnCall{"_tool_jira_connections", "token", "text"}
	if len(database.calls) != 1 || database.calls[0] != want {
		t.Fatalf("ModifyColumnType calls = %#v, want [%#v]", database.calls, want)
	}

	for _, registered := range All() {
		if registered.Version() == script.Version() {
			return
		}
	}
	t.Fatalf("migration version %d is not registered", script.Version())
}
