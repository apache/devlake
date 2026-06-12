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

package auth

import (
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
)

// AuthSession is the persisted record backing one signed session JWT. It is
// what makes server-side revocation possible: the JWT carries the jti, and a
// row here can be flagged revoked to block reuse before the token's natural
// expiry.
type AuthSession struct {
	Jti        string     `gorm:"primaryKey;type:varchar(36)" json:"jti"`
	Sub        string     `gorm:"type:varchar(255);index" json:"sub"`
	Email      string     `gorm:"type:varchar(255)" json:"email"`
	Name       string     `gorm:"type:varchar(255)" json:"name"`
	IssuedAt   time.Time  `json:"issuedAt"`
	ExpiresAt  time.Time  `gorm:"index" json:"expiresAt"`
	RevokedAt  *time.Time `gorm:"index" json:"revokedAt,omitempty"`
	LastSeenAt time.Time  `json:"lastSeenAt"`
}

func (AuthSession) TableName() string { return "auth_sessions" }

func CreateSession(db dal.Dal, s *AuthSession) errors.Error {
	return db.Create(s)
}

// RevokeSession marks the session row revoked. No-op if no row matches.
func RevokeSession(db dal.Dal, jti string) errors.Error {
	now := time.Now()
	return db.UpdateColumn(&AuthSession{}, "revoked_at", now, dal.Where("jti = ?", jti))
}

// UpdateLastSeen records that the session was used. Throttled by the caller
// (see Service.bumpLastSeen) so this isn't a per-request DB write.
func UpdateLastSeen(db dal.Dal, jti string, at time.Time) errors.Error {
	return db.UpdateColumn(&AuthSession{}, "last_seen_at", at, dal.Where("jti = ?", jti))
}

// ListRevoked returns the jti of every still-relevant revoked session,
// excluding ones already past their natural expiry.
func ListRevoked(db dal.Dal) ([]string, errors.Error) {
	var rows []AuthSession
	err := db.All(&rows,
		dal.Select("jti"),
		dal.Where("revoked_at IS NOT NULL AND expires_at > ?", time.Now()),
	)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Jti
	}
	return out, nil
}
