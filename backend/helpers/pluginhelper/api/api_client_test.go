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

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/core/errors"
	implcontext "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/impls/logruslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApiClientBlackList(t *testing.T) {
	for _, tc := range []struct {
		Name      string
		Pattern   string
		Endpoints []string
		Err       errors.Error
	}{
		{
			Name:    "Blocked Internal IP Addresses",
			Pattern: "10.0.0.1/16",
			Endpoints: []string{
				"https://10.0.0.1",
				"http://10.0.0.254",
				"http://10.0.254.1",
				"https://10.0.254.254",
			},
			Err: ErrHostNotAllowed,
		},
		{
			Name:    "Allowed Public IP Addresses",
			Pattern: "10.0.0.1/16",
			Endpoints: []string{
				"http://10.1.0.1",
				"http://10.1.0.254",
				"http://10.1.254.1",
				"http://10.1.254.254",
			},
			Err: nil,
		},
		{
			Name:    "Blocked IPv6 loopback",
			Pattern: "::1/128",
			Endpoints: []string{
				"http://[::1]",
			},
			Err: ErrHostNotAllowed,
		},
		{
			Name:    "Blocked IPv6 Unique Local Addresses IP Addresses",
			Pattern: "fc00::/7",
			Endpoints: []string{
				"http://[fdf8:f53b:82e4::53]",
			},
			Err: ErrHostNotAllowed,
		},
		{
			Name:    "Blocked IPv6 Link-Local IP Addresses",
			Pattern: "fe80::/10",
			Endpoints: []string{
				"http://[fe80::200:5aee:feaa:20a2]",
			},
			Err: ErrHostNotAllowed,
		},
		{
			Name:    "Allowed IPv6 Public IP Addresses",
			Pattern: "fe80::/10",
			Endpoints: []string{
				"http://[2001:0002:6c::430]",
			},
			Err: nil,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			for _, endpoint := range tc.Endpoints {
				err := checkCidrBlacklist(tc.Pattern, endpoint, logruslog.Global)
				assert.Equal(t, tc.Err, err, "pattern %s and endpoint %s should return %v, but got %v", tc.Pattern, endpoint, tc.Err, err)
			}
		})
	}
}

// TestNewApiClientWithProxyDoesNotProbeEndpoint is a regression test for
// apache/devlake#8632: creating a client with a proxy must not send a request
// to the endpoint root. Jira behind SSO redirects that root to a login page,
// which made client creation fail with "stopped after 10 redirects".
func TestNewApiClientWithProxyDoesNotProbeEndpoint(t *testing.T) {
	var proxied atomic.Int32
	// Behaves like a proxy in front of an SSO-protected server: every request
	// is redirected back to itself, so any probe would end in a redirect loop.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Add(1)
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
	}))
	defer proxy.Close()

	br := implcontext.NewDefaultBasicRes(config.GetConfig(), logruslog.Global, nil)
	client, err := NewApiClient(context.Background(), "http://jira.example.invalid/rest/", nil, 0, proxy.URL, br)

	require.Nil(t, err)
	require.NotNil(t, client)
	assert.Equal(t, int32(0), proxied.Load(), "NewApiClient must not send requests through the proxy")

	// Requests made later still go through the proxy.
	_, _ = client.Get("api/2/serverInfo", nil, nil)
	assert.Greater(t, proxied.Load(), int32(0), "requests should be routed through the proxy")
}
