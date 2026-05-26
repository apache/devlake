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
	"context"
	"sync"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"
)

// revocationRefreshInterval is how often the cache reloads the revoked-jti
// set from the database. Revocations propagate within this window.
const revocationRefreshInterval = 30 * time.Second

// revocationCache holds the set of revoked session jtis. The MVP keeps it
// fully in memory because the set is tiny (only sessions that were
// explicitly revoked, pruned on natural expiry) and per-request DB lookups
// would otherwise dominate the auth middleware's hot path.
type revocationCache struct {
	mu      sync.RWMutex
	revoked map[string]struct{}
}

func newRevocationCache() *revocationCache {
	return &revocationCache{revoked: map[string]struct{}{}}
}

func (r *revocationCache) IsRevoked(jti string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.revoked[jti]
	return ok
}

func (r *revocationCache) Add(jti string) {
	r.mu.Lock()
	r.revoked[jti] = struct{}{}
	r.mu.Unlock()
}

// Refresh reloads the revoked set from the DB while preserving any Adds
// that landed concurrently with the DB read. The race we're guarding:
// Logout calls RevokeSession then revoked.Add; if Refresh's DB read happens
// between those two steps, the new map would be missing the jti. Carrying
// forward "additions since the snapshot" plugs that gap.
func (r *revocationCache) Refresh(db dal.Dal) error {
	r.mu.RLock()
	before := make(map[string]struct{}, len(r.revoked))
	for j := range r.revoked {
		before[j] = struct{}{}
	}
	r.mu.RUnlock()

	jtis, err := ListRevoked(db)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	next := make(map[string]struct{}, len(jtis))
	for _, j := range jtis {
		next[j] = struct{}{}
	}
	// Anything in the current set that wasn't in the snapshot was Added
	// during the DB read; keep it. Anything in both `before` and the
	// current set but absent from the DB result has expired naturally and
	// is safe to drop.
	for j := range r.revoked {
		if _, wasBefore := before[j]; !wasBefore {
			next[j] = struct{}{}
		}
	}
	r.revoked = next
	return nil
}

// startRefresher loads the cache once synchronously, then refreshes it on a
// timer. The synchronous first load means we never accept revoked sessions
// in a startup window.
func startRefresher(ctx context.Context, cache *revocationCache, db dal.Dal, logger log.Logger) {
	if err := cache.Refresh(db); err != nil {
		logger.Error(err, "auth: initial revocation cache load failed")
	}
	go func() {
		t := time.NewTicker(revocationRefreshInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := cache.Refresh(db); err != nil {
					logger.Error(err, "auth: revocation cache refresh failed")
				}
			}
		}
	}()
}
