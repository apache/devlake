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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	lakeErrors "github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/unithelper"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	transport.sleep = func(_ context.Context, d time.Duration) error {
		*sleeps = append(*sleeps, d)
		return nil
	}
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

func TestRetryTransportNeverRetriesCancellation(t *testing.T) {
	transport, fake, _ := newTestTransport(
		fakeStep{err: context.Canceled},
		fakeStep{status: 200},
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(req)
	require.Error(t, err)
	assert.Equal(t, 1, fake.calls, "a cancelled request is terminal, never retried")
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

// Cancellation during a backoff wait must abort the wait promptly — the
// default sleep observes the request context, it is not a bare time.Sleep.
func TestRetryTransportBackoffSleepObservesCancellation(t *testing.T) {
	fake := &fakeRoundTripper{steps: []fakeStep{{status: 504}, {status: 200}}}
	transport := newRetryTransport(fake) // real sleepContext, real backoff (1s+)
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err = transport.RoundTrip(req)
	require.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), 500*time.Millisecond,
		"cancellation interrupts the backoff wait instead of sleeping it out")
	assert.Equal(t, 1, fake.calls, "no second attempt after cancellation")
}

// The async client's workers share ONE transport instance: RoundTrip must be
// safe for concurrent use (the jitter source is the concurrency-safe
// math/rand/v2 global — run with -race to prove it).
func TestRetryTransportConcurrentRoundTrips(t *testing.T) {
	var calls atomic.Int64
	flaky := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		status := 200
		if calls.Add(1)%2 == 0 { // every second call fails -> jitter path under concurrency
			status = 500
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("{}")),
			Request:    req,
		}, nil
	})
	transport := newRetryTransport(flaky)
	transport.sleep = func(context.Context, time.Duration) error { return nil }

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
			assert.NoError(t, err)
			res, err := transport.RoundTrip(req)
			assert.NoError(t, err)
			if res != nil && res.Body != nil {
				_ = res.Body.Close()
			}
		}()
	}
	wg.Wait()
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// The assembled client has ONE retry owner: the transport. A persistently
// failing endpoint must see exactly maxAttempts requests — not maxAttempts
// multiplied by the framework's own retry loop, which NewYoutrackApiClient
// disables explicitly.
func TestAssembledClientRetriesOnlyViaTransport(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	taskCtx := new(mockplugin.TaskContext)
	taskCtx.On("GetContext").Return(context.Background())
	taskCtx.On("GetConfig", mock.Anything).Return("")
	taskCtx.On("GetConfigReader").Return(viper.New())
	taskCtx.On("GetLogger").Return(unithelper.DummyLogger())

	connection := &models.YoutrackConnection{}
	connection.Endpoint = server.URL + "/"
	connection.Token = "perm:dummy"

	client, err := NewYoutrackApiClient(taskCtx, connection)
	require.NoError(t, err)
	defer client.Release()

	var handlerCalls atomic.Int64
	client.DoGetAsync("issues", nil, nil, func(res *http.Response) lakeErrors.Error {
		handlerCalls.Add(1)
		return nil
	})
	// Note: the scheduler publishes a task's terminal error via a panic
	// handler that can land after WaitAsync unblocks (framework trait), so
	// the deterministic assertions here are on the wire, not the return.
	_ = client.WaitAsync()
	assert.Equal(t, int64(3), hits.Load(),
		"exactly the transport's maxAttempts hit the server — the framework retry loop is disabled, not compounded")
	assert.Zero(t, handlerCalls.Load(), "the handler is never invoked for a failed request")
}
