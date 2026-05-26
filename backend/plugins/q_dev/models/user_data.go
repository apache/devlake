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

// QDevUserData 存储从CSV中提取的原始数据
type QDevUserData struct {
	common.NoPKModel
	ConnectionId uint64    `gorm:"primaryKey"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	UserId       string    `gorm:"primaryKey;type:varchar(255)" json:"userId"`
	Date         time.Time `gorm:"primaryKey;type:date" json:"date"`
	DisplayName  string    `gorm:"type:varchar(255)" json:"displayName"`

	CodeReview_FindingsCount             int
	CodeReview_SucceededEventCount       int
	InlineChat_AcceptanceEventCount      int
	InlineChat_AcceptedLineAdditions     int
	InlineChat_AcceptedLineDeletions     int
	InlineChat_DismissalEventCount       int
	InlineChat_DismissedLineAdditions    int
	InlineChat_DismissedLineDeletions    int
	InlineChat_RejectedLineAdditions     int
	InlineChat_RejectedLineDeletions     int
	InlineChat_RejectionEventCount       int
	InlineChat_TotalEventCount           int
	Inline_AICodeLines                   int
	Inline_AcceptanceCount               int
	Inline_SuggestionsCount              int
	Chat_AICodeLines                     int
	Chat_MessagesInteracted              int
	Chat_MessagesSent                    int
	CodeFix_AcceptanceEventCount         int
	CodeFix_AcceptedLines                int
	CodeFix_GeneratedLines               int
	CodeFix_GenerationEventCount         int
	CodeReview_FailedEventCount          int
	Dev_AcceptanceEventCount             int
	Dev_AcceptedLines                    int
	Dev_GeneratedLines                   int
	Dev_GenerationEventCount             int
	DocGeneration_AcceptedFileUpdates    int
	DocGeneration_AcceptedFilesCreations int
	DocGeneration_AcceptedLineAdditions  int
	DocGeneration_AcceptedLineUpdates    int
	DocGeneration_EventCount             int
	DocGeneration_RejectedFileCreations  int
	DocGeneration_RejectedFileUpdates    int
	DocGeneration_RejectedLineAdditions  int
	DocGeneration_RejectedLineUpdates    int
	TestGeneration_AcceptedLines         int
	TestGeneration_AcceptedTests         int
	TestGeneration_EventCount            int
	TestGeneration_GeneratedLines        int
	TestGeneration_GeneratedTests        int
	Transformation_EventCount            int
	Transformation_LinesGenerated        int
	Transformation_LinesIngested         int
}

func (QDevUserData) TableName() string {
	return "_tool_q_dev_user_data"
}
