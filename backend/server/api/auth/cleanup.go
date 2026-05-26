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
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"
)

const (
	sessionCleanupInterval = 24 * time.Hour
	// Keep expired rows around as a soft audit trail before pruning. 30d
	// covers any reasonable "who logged in last month" question without
	// letting the table grow unbounded.
	sessionRetentionAfterExpiry = 30 * 24 * time.Hour
)

// startSessionCleanup deletes long-expired session rows on a daily timer.
// First sweep runs after one interval, not at boot, so deploys don't pay
// the cleanup cost during the noisy startup window.
func startSessionCleanup(ctx context.Context, db dal.Dal, logger log.Logger) {
	go func() {
		t := time.NewTicker(sessionCleanupInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				cutoff := time.Now().Add(-sessionRetentionAfterExpiry)
				if err := db.Delete(&AuthSession{}, dal.Where("expires_at < ?", cutoff)); err != nil {
					logger.Error(err, "auth: session cleanup")
					continue
				}
				logger.Info("auth: pruned auth_sessions rows expired before %s", cutoff.Format(time.RFC3339))
			}
		}
	}()
}
