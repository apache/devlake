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

import "time"

// LockingHistory is desgned for preventing multiple delake instances from sharing the same database which may cause
// problems like #3537, #3466. It works by the following step:
//
// 1. Each devlake insert a record to this table whie `Succeeded=false`
// 2. Then it should try to lock the LockingStub table
// 3. Update the record with `Succeeded=true` if it had obtained the lock successfully
//
// NOTE: it works IFF all devlake instances obey the principle described above, in other words, this mechanism can
// not prevent older versions from sharing the same database
type LockingHistory struct {
	ID        uint64 `gorm:"primaryKey" json:"id"`
	HostName  string
	Version   string
	Succeeded bool
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (LockingHistory) TableName() string {
	return "_devlake_locking_history"
}

// LockingStub does nothing but offer a locking target
type LockingStub struct {
	Stub string `gorm:"primaryKey;type:varchar(255)"`
}

func (LockingStub) TableName() string {
	return "_devlake_locking_stub"
}
