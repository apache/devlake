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

type cursorConversationIdColumnCall struct {
	tableName  string
	columnName string
	columnType string
}

type cursorConversationIdRecordingDal struct {
	dal.Dal
	calls []cursorConversationIdColumnCall
}

func (d *cursorConversationIdRecordingDal) ModifyColumnType(tableName, columnName, columnType string) errors.Error {
	d.calls = append(d.calls, cursorConversationIdColumnCall{tableName, columnName, columnType})
	return nil
}

type cursorConversationIdBasicRes struct {
	context.BasicRes
	database dal.Dal
}

func (r *cursorConversationIdBasicRes) GetDal() dal.Dal {
	return r.database
}

func TestWidenCursorAiCodeConversationId(t *testing.T) {
	database := new(cursorConversationIdRecordingDal)
	script := new(widenCursorAiCodeConversationId)

	if err := script.Up(&cursorConversationIdBasicRes{database: database}); err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	want := []cursorConversationIdColumnCall{
		{"_tool_cursor_ai_code_conversations", "conversation_id", "varchar(255) NOT NULL"},
		{"_tool_cursor_ai_code_range_annotations", "conversation_id", "varchar(255)"},
	}
	if len(database.calls) != len(want) {
		t.Fatalf("ModifyColumnType calls = %#v, want %#v", database.calls, want)
	}
	for i := range want {
		if database.calls[i] != want[i] {
			t.Fatalf("ModifyColumnType calls = %#v, want %#v", database.calls, want)
		}
	}
	if script.Version() != 20261002131500 {
		t.Fatalf("Version() = %d, want 20261002131500", script.Version())
	}
	if script.Name() != "widen cursor ai code conversation_id columns to varchar(255)" {
		t.Fatalf("Name() = %q, want %q", script.Name(), "widen cursor ai code conversation_id columns to varchar(255)")
	}

	for _, registeredScript := range All() {
		if registeredScript.Version() == script.Version() {
			return
		}
	}
	t.Fatal("migration version 20261002131500 is not registered")
}
