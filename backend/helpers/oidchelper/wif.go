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
	"fmt"
	"os"
	"sync"
	"time"
)

// AzureFederatedTokenFileEnv is the env var the azure-workload-identity
// webhook sets on the pod, pointing at the projected SA token file.
const AzureFederatedTokenFileEnv = "AZURE_FEDERATED_TOKEN_FILE"

// wifCacheTTL is the read-cache lifetime for the federated SA token.
// Kubernetes rotates SA tokens at ~80% of their TTL (1h minimum), so 10
// minutes is well inside the safe window.
const wifCacheTTL = 10 * time.Minute

var (
	wifMu        sync.RWMutex
	wifAssertion string
	wifExpires   time.Time
	// wifReadFile is overridable from tests so they can stub the file read
	// without touching the real filesystem.
	wifReadFile = os.ReadFile
)

// FederatedAssertion returns the workload-identity SA token to use as the
// `client_assertion` in an OIDC code exchange against Microsoft Entra.
// Reads the file pointed to by AZURE_FEDERATED_TOKEN_FILE and caches the
// result for wifCacheTTL.
func FederatedAssertion() (string, error) {
	file := os.Getenv(AzureFederatedTokenFileEnv)
	if file == "" {
		return "", fmt.Errorf("%s env var not set (is the azure-workload-identity webhook installed and the pod labeled)", AzureFederatedTokenFileEnv)
	}

	wifMu.RLock()
	if wifExpires.After(time.Now()) {
		assertion := wifAssertion
		wifMu.RUnlock()
		return assertion, nil
	}
	wifMu.RUnlock()

	wifMu.Lock()
	defer wifMu.Unlock()
	// Double-check under the write lock in case a peer goroutine already
	// refreshed the assertion.
	if wifExpires.After(time.Now()) {
		return wifAssertion, nil
	}
	content, err := wifReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file, err)
	}
	wifAssertion = string(content)
	wifExpires = time.Now().Add(wifCacheTTL)
	return wifAssertion, nil
}
