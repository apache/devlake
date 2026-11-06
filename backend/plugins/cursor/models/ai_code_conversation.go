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

package models

import (
	"github.com/apache/devlake/core/models/common"
)

// CursorAiCodeConversation stores conversation metadata referenced by commit-detail
// range annotations from GET /analytics/ai-code/commits/:commitHash (Enterprise API).
type CursorAiCodeConversation struct {
	ConnectionId   uint64 `gorm:"primaryKey" json:"connectionId"`
	ScopeId        string `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	ConversationId string `gorm:"primaryKey;type:varchar(255)" json:"conversationId"`

	Title          string `gorm:"type:varchar(512)" json:"title"`
	Tldr           string `gorm:"type:text" json:"tldr"`
	Overview       string `gorm:"type:text" json:"overview"`
	SummaryBullets string `gorm:"type:text" json:"summaryBullets"`

	common.NoPKModel
}

func (CursorAiCodeConversation) TableName() string {
	return "_tool_cursor_ai_code_conversations"
}
