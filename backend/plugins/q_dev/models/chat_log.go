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
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

// QDevChatLog stores parsed data from GenerateAssistantResponse logging events
type QDevChatLog struct {
	common.NoPKModel
	ConnectionId        uint64    `gorm:"primaryKey"`
	ScopeId             string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	RequestId           string    `gorm:"primaryKey;type:varchar(255)" json:"requestId"`
	UserId              string    `gorm:"index;type:varchar(255)" json:"userId"`
	DisplayName         string    `gorm:"type:varchar(255)" json:"displayName"`
	Timestamp           time.Time `gorm:"index" json:"timestamp"`
	ChatTriggerType     string    `gorm:"type:varchar(50)" json:"chatTriggerType"`
	HasCustomization    bool      `json:"hasCustomization"`
	ConversationId      string    `gorm:"type:varchar(255)" json:"conversationId"`
	UtteranceId         string    `gorm:"type:varchar(255)" json:"utteranceId"`
	ModelId             string    `gorm:"type:varchar(100)" json:"modelId"`
	PromptLength        int       `json:"promptLength"`
	ResponseLength      int       `json:"responseLength"`
	OpenFileCount       int       `json:"openFileCount"`
	ActiveFileName      string    `gorm:"type:varchar(512)" json:"activeFileName"`
	ActiveFileExtension string    `gorm:"type:varchar(50)" json:"activeFileExtension"`
	HasSteering         bool      `json:"hasSteering"`
	IsSpecMode          bool      `json:"isSpecMode"`
	CodeReferenceCount  int       `json:"codeReferenceCount"`
	WebLinkCount        int       `json:"webLinkCount"`
	HasFollowupPrompts  bool      `json:"hasFollowupPrompts"`
}

func (QDevChatLog) TableName() string {
	return "_tool_q_dev_chat_log"
}
