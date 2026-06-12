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

package oidchelper

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func resetFederatedAssertionCache() {
	wifMu.Lock()
	defer wifMu.Unlock()
	wifAssertion = ""
	wifExpires = time.Time{}
}

func TestFederatedAssertionMissingEnv(t *testing.T) {
	resetFederatedAssertionCache()
	t.Setenv(AzureFederatedTokenFileEnv, "")
	if _, err := FederatedAssertion(); err == nil {
		t.Fatal("expected error when env var is unset")
	}
}

func TestFederatedAssertionReadsAndCaches(t *testing.T) {
	resetFederatedAssertionCache()
	var reads atomic.Int32
	wifReadFile = func(string) ([]byte, error) {
		reads.Add(1)
		return []byte("token-content"), nil
	}
	t.Cleanup(func() { wifReadFile = osReadFileForTests })

	t.Setenv(AzureFederatedTokenFileEnv, "/fake/path")

	for i := 0; i < 5; i++ {
		got, err := FederatedAssertion()
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if got != "token-content" {
			t.Fatalf("got %q", got)
		}
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("expected 1 file read across 5 calls, got %d", got)
	}
}

func TestFederatedAssertionRefreshesAfterTTL(t *testing.T) {
	resetFederatedAssertionCache()
	var counter atomic.Int32
	wifReadFile = func(string) ([]byte, error) {
		counter.Add(1)
		return []byte("v"), nil
	}
	t.Cleanup(func() { wifReadFile = osReadFileForTests })

	t.Setenv(AzureFederatedTokenFileEnv, "/fake/path")

	if _, err := FederatedAssertion(); err != nil {
		t.Fatalf("first read: %v", err)
	}
	// Force the cache stale.
	wifMu.Lock()
	wifExpires = time.Now().Add(-time.Second)
	wifMu.Unlock()

	if _, err := FederatedAssertion(); err != nil {
		t.Fatalf("second read: %v", err)
	}
	if got := counter.Load(); got != 2 {
		t.Fatalf("expected 2 file reads, got %d", got)
	}
}

func TestFederatedAssertionPropagatesReadError(t *testing.T) {
	resetFederatedAssertionCache()
	wifReadFile = func(string) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { wifReadFile = osReadFileForTests })

	t.Setenv(AzureFederatedTokenFileEnv, "/fake/path")
	if _, err := FederatedAssertion(); err == nil {
		t.Fatal("expected error from underlying read")
	}
}

// osReadFileForTests restores the real os.ReadFile after each test that
// monkey-patches wifReadFile. Defined here to avoid an import-cycle.
var osReadFileForTests = func(name string) ([]byte, error) {
	return nil, errors.New("real os.ReadFile not stubbed in this test")
}
