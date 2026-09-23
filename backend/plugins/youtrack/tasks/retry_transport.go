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

package tasks

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// retryTransport is the plugin's single retry-policy owner: it
// retries 429 and 5xx responses (incl. the observed 504 on expensive
// queries) and network-level failures with exponential backoff + jitter,
// honouring `Retry-After` when present. The framework async client's own
// retry loop (any status >= 400 on a fixed tick) is explicitly disabled in
// NewYoutrackApiClient so the two never multiply each other's attempts.
// Requests are all GETs (no bodies to rewind). Backoff sleeps observe the
// request context, so pipeline cancellation interrupts a wait promptly, and
// the jitter draws from math/rand/v2's concurrency-safe global source —
// async workers share one transport instance.
type retryTransport struct {
	base        http.RoundTripper
	maxAttempts int           // total attempts including the first
	baseDelay   time.Duration // backoff = baseDelay * 2^(attempt-1), +/-25% jitter, capped
	maxDelay    time.Duration
	sleep       func(ctx context.Context, d time.Duration) error // injectable for tests
}

func newRetryTransport(base http.RoundTripper) *retryTransport {
	return &retryTransport{
		base:        base,
		maxAttempts: 3,
		baseDelay:   1 * time.Second,
		maxDelay:    30 * time.Second,
		sleep:       sleepContext,
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var res *http.Response
	var err error
	for attempt := 1; ; attempt++ {
		res, err = t.base.RoundTrip(req)
		retry := false
		if err != nil {
			// cancellation is terminal, never a retry signal
			if req.Context().Err() != nil {
				return res, err
			}
			// network-level failure (timeout, reset, temporary DNS): retry
			retry = true
		} else if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
			retry = true
		}
		if !retry || attempt >= t.maxAttempts {
			return res, err
		}
		wait := t.waitFor(res, attempt)
		if res != nil && res.Body != nil {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}
		if sleepErr := t.sleep(req.Context(), wait); sleepErr != nil {
			return nil, sleepErr
		}
	}
}

// waitFor computes the sleep before the next attempt: Retry-After when the
// server sent one, else exponential backoff with +/-25% jitter. Both are
// capped at maxDelay: the ApiClient has no request timeout, so an unbounded
// Retry-After would park an async worker inside RoundTrip indefinitely.
func (t *retryTransport) waitFor(res *http.Response, attempt int) time.Duration {
	if res != nil {
		if retryAfter := res.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
				return min(time.Duration(seconds)*time.Second, t.maxDelay)
			}
			if date, err := http.ParseTime(retryAfter); err == nil {
				if wait := time.Until(date); wait > 0 {
					return min(wait, t.maxDelay)
				}
			}
		}
	}
	wait := t.baseDelay << (attempt - 1)
	jitter := 0.75 + rand.Float64()*0.5
	wait = time.Duration(float64(wait) * jitter)
	if wait > t.maxDelay {
		wait = t.maxDelay
	}
	return wait
}

// sleepContext waits for d or until ctx is done, whichever comes first.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
