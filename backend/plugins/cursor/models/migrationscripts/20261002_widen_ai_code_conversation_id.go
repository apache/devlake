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

var _ plugin.MigrationScript = (*widenCursorAiCodeConversationId)(nil)

type widenCursorAiCodeConversationId struct{}

// Cursor commit-detail groups can carry conversation ids longer than 64
// characters (observed 91, including a tool-call id with an embedded newline).
// conversation_id is the primary key on conversations, so the alter keeps NOT NULL.
// On range annotations it is a nullable secondary index, so nullability stays open.
func (*widenCursorAiCodeConversationId) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	columns := []struct {
		tableName  string
		columnType string
	}{
		{"_tool_cursor_ai_code_conversations", "varchar(255) NOT NULL"},
		{"_tool_cursor_ai_code_range_annotations", "varchar(255)"},
	}
	for _, column := range columns {
		if err := db.ModifyColumnType(column.tableName, "conversation_id", column.columnType); err != nil {
			return err
		}
	}
	return nil
}

func (*widenCursorAiCodeConversationId) Version() uint64 {
	return 20261002131500
}

func (*widenCursorAiCodeConversationId) Name() string {
	return "widen cursor ai code conversation_id columns to varchar(255)"
}
