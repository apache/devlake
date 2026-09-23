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
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRoundTripper answers with the queued responses/errors in order.
type fakeRoundTripper struct {
	steps []fakeStep
	calls int
}

type fakeStep struct {
	status  int
	headers map[string]string
	err     error
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	step := f.steps[f.calls]
	f.calls++
	if step.err != nil {
		return nil, step.err
	}
	res := &http.Response{
		StatusCode: step.status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("{}")),
		Request:    req,
	}
	for k, v := range step.headers {
		res.Header.Set(k, v)
	}
	return res, nil
}

func newTestTransport(steps ...fakeStep) (*retryTransport, *fakeRoundTripper, *[]time.Duration) {
	fake := &fakeRoundTripper{steps: steps}
	sleeps := &[]time.Duration{}
	transport := newRetryTransport(fake)
	transport.random = rand.New(rand.NewSource(42))
	transport.sleep = func(d time.Duration) { *sleeps = append(*sleeps, d) }
	return transport, fake, sleeps
}

func TestRetryTransportPassesThroughSuccess(t *testing.T) {
	transport, fake, sleeps := newTestTransport(fakeStep{status: 200})
	res, err := transport.RoundTrip(&http.Request{})
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, 1, fake.calls)
	assert.Empty(t, *sleeps)
}

func TestRetryTransportRetries429And5xx(t *testing.T) {
	for _, status := range []int{429, 500, 502, 503, 504} {
		transport, fake, _ := newTestTransport(
			fakeStep{status: status},
			fakeStep{status: 200},
		)
		res, err := transport.RoundTrip(&http.Request{})
		require.NoError(t, err)
		assert.Equal(t, 200, res.StatusCode, "status %d must be retried", status)
		assert.Equal(t, 2, fake.calls)
	}
}

func TestRetryTransportDoesNotRetry4xx(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422} {
		transport, fake, sleeps := newTestTransport(fakeStep{status: status})
		res, err := transport.RoundTrip(&http.Request{})
		require.NoError(t, err)
		assert.Equal(t, status, res.StatusCode, "status %d must NOT be retried", status)
		assert.Equal(t, 1, fake.calls)
		assert.Empty(t, *sleeps)
	}
}

func TestRetryTransportRetriesNetworkErrors(t *testing.T) {
	transport, fake, _ := newTestTransport(
		fakeStep{err: errors.New("connection reset by peer")},
		fakeStep{status: 200},
	)
	res, err := transport.RoundTrip(&http.Request{})
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, 2, fake.calls)
}

func TestRetryTransportHonoursRetryAfter(t *testing.T) {
	transport, _, sleeps := newTestTransport(
		fakeStep{status: 429, headers: map[string]string{"Retry-After": "17"}},
		fakeStep{status: 200},
	)
	res, err := transport.RoundTrip(&http.Request{})
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
	require.Len(t, *sleeps, 1)
	assert.Equal(t, 17*time.Second, (*sleeps)[0], "Retry-After in seconds is honoured verbatim")
}

func TestRetryTransportCapsRetryAfterAtMaxDelay(t *testing.T) {
	t.Run("seconds", func(t *testing.T) {
		transport, _, sleeps := newTestTransport(
			fakeStep{status: 429, headers: map[string]string{"Retry-After": "3600"}},
			fakeStep{status: 200},
		)
		_, err := transport.RoundTrip(&http.Request{})
		require.NoError(t, err)
		require.Len(t, *sleeps, 1)
		assert.Equal(t, transport.maxDelay, (*sleeps)[0], "a huge Retry-After never parks a worker beyond maxDelay")
	})
	t.Run("http-date", func(t *testing.T) {
		far := time.Now().Add(2 * time.Hour).UTC().Format(http.TimeFormat)
		transport, _, sleeps := newTestTransport(
			fakeStep{status: 503, headers: map[string]string{"Retry-After": far}},
			fakeStep{status: 200},
		)
		_, err := transport.RoundTrip(&http.Request{})
		require.NoError(t, err)
		require.Len(t, *sleeps, 1)
		assert.Equal(t, transport.maxDelay, (*sleeps)[0])
	})
}

func TestRetryTransportBackoffGrowsWithJitterAndCaps(t *testing.T) {
	transport, fake, sleeps := newTestTransport(
		fakeStep{status: 500},
		fakeStep{status: 500},
		fakeStep{status: 500},
		fakeStep{status: 500},
	)
	transport.maxAttempts = 4
	transport.baseDelay = 10 * time.Second
	transport.maxDelay = 25 * time.Second
	res, err := transport.RoundTrip(&http.Request{})
	require.NoError(t, err)
	assert.Equal(t, 500, res.StatusCode)
	assert.Equal(t, 4, fake.calls)
	require.Len(t, *sleeps, 3)
	// backoff base 10s/20s/40s with ±25% jitter, capped at 25s
	assert.GreaterOrEqual(t, (*sleeps)[0], 7500*time.Millisecond)
	assert.LessOrEqual(t, (*sleeps)[0], 12500*time.Millisecond)
	assert.GreaterOrEqual(t, (*sleeps)[1], 15*time.Second)
	assert.LessOrEqual(t, (*sleeps)[1], 25*time.Second)
	assert.Equal(t, 25*time.Second, (*sleeps)[2], "third backoff hits the cap")
}

func TestRetryTransportStopsAtMaxAttempts(t *testing.T) {
	transport, fake, sleeps := newTestTransport(
		fakeStep{status: 503},
		fakeStep{status: 503},
		fakeStep{status: 503},
		fakeStep{status: 200},
	)
	res, err := transport.RoundTrip(&http.Request{})
	require.NoError(t, err)
	assert.Equal(t, 503, res.StatusCode, "the last response is returned when attempts run out")
	assert.Equal(t, 3, fake.calls, "maxAttempts=3 means 3 calls, never the 4th")
	assert.Len(t, *sleeps, 2)
}
