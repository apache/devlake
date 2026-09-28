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

// YoutrackAccount is a YouTrack user (tool layer), converted to
// crossdomain.Account. Accounts are derived inline from user refs (reporter,
// updater, assignee, author) in collected payloads — the plugin never pulls a
// full `/api/users` roster, so it works with least-privilege tokens. The
// converter skips the built-in guest account.
type YoutrackAccount struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Login        string `gorm:"type:varchar(255)" json:"login"`
	FullName     string `gorm:"type:varchar(255)" json:"fullName"`
	Email        string `gorm:"type:varchar(255)" json:"email"`
	AvatarUrl    string `gorm:"type:varchar(255)" json:"avatarUrl"`
	Guest        bool   `json:"guest"`
	common.NoPKModel
}

func (YoutrackAccount) TableName() string {
	return "_tool_youtrack_accounts"
}
