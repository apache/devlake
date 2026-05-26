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

// This script lists GitHub App installations and tests the token refresh flow.
//
// Usage:
//
//	GITHUB_APP_ID=123456 GITHUB_APP_PEM="$(cat private-key.pem)" go run ./plugins/github/token/cmd/test_refresh/
//
// Or if the key is in a file:
//
//	GITHUB_APP_ID=123456 GITHUB_APP_PEM_FILE=/path/to/private-key.pem go run ./plugins/github/token/cmd/test_refresh/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type installation struct {
	ID      int `json:"id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
	AppID int `json:"app_id"`
}

type installationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func main() {
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		fatal("GITHUB_APP_ID env var is required")
	}

	pemData := os.Getenv("GITHUB_APP_PEM")
	if pemData == "" {
		pemFile := os.Getenv("GITHUB_APP_PEM_FILE")
		if pemFile == "" {
			fatal("Set GITHUB_APP_PEM (contents) or GITHUB_APP_PEM_FILE (path)")
		}
		data, err := os.ReadFile(pemFile)
		if err != nil {
			fatal("failed to read PEM file: %v", err)
		}
		pemData = string(data)
	}

	// Step 1: Create a JWT signed with the app's private key
	fmt.Println("=== Step 1: Creating JWT from App ID and private key ===")
	jwtToken, err := createJWT(appID, pemData)
	if err != nil {
		fatal("failed to create JWT: %v", err)
	}
	fmt.Printf("JWT created (first 20 chars): %s...\n\n", jwtToken[:20])

	// Step 2: List installations
	fmt.Println("=== Step 2: Listing installations for this app ===")
	installations, err := listInstallations(jwtToken)
	if err != nil {
		fatal("failed to list installations: %v", err)
	}
	if len(installations) == 0 {
		fatal("no installations found for this app")
	}
	for _, inst := range installations {
		fmt.Printf("  Installation ID: %d  Account: %s\n", inst.ID, inst.Account.Login)
	}
	fmt.Println()

	// Step 3: Get an installation token for the first installation
	inst := installations[0]
	fmt.Printf("=== Step 3: Minting installation token for %s (ID: %d) ===\n", inst.Account.Login, inst.ID)
	token1, err := getInstallationToken(jwtToken, inst.ID)
	if err != nil {
		fatal("failed to get installation token: %v", err)
	}
	fmt.Printf("Token 1: %s...  Expires: %s\n\n", token1.Token[:10], token1.ExpiresAt.Format(time.RFC3339))

	// Step 4: Make an API call with the token to verify it works
	fmt.Println("=== Step 4: Verifying token works (GET /installation/repositories) ===")
	err = verifyToken(token1.Token)
	if err != nil {
		fatal("token verification failed: %v", err)
	}
	fmt.Printf("Token is valid and working.\n\n")

	// Step 5: Simulate the refresh flow — mint a second token (as our refreshFn would)
	fmt.Println("=== Step 5: Simulating token refresh (minting a second token) ===")
	jwtToken2, err := createJWT(appID, pemData)
	if err != nil {
		fatal("failed to create second JWT: %v", err)
	}
	token2, err := getInstallationToken(jwtToken2, inst.ID)
	if err != nil {
		fatal("failed to get second installation token: %v", err)
	}
	fmt.Printf("Token 2: %s...  Expires: %s\n", token2.Token[:10], token2.ExpiresAt.Format(time.RFC3339))

	if token1.Token == token2.Token {
		fmt.Println("Note: Both tokens are identical (GitHub may cache short-lived tokens)")
	} else {
		fmt.Println("Tokens are different — refresh produced a new token.")
	}
	fmt.Println()

	// Step 6: Verify the new token works
	fmt.Println("=== Step 6: Verifying refreshed token works ===")
	err = verifyToken(token2.Token)
	if err != nil {
		fatal("refreshed token verification failed: %v", err)
	}
	fmt.Println("Refreshed token is valid and working.")

	fmt.Println("\n=== All steps passed. The token refresh flow works correctly. ===")
	fmt.Printf("\nFor reference, your Installation ID is: %d\n", inst.ID)
}

func createJWT(appID, pemData string) (string, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(pemData))
	if err != nil {
		return "", fmt.Errorf("invalid PEM key: %w", err)
	}

	now := time.Now().Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now,
		"exp": now + (10 * 60), // 10 minutes
		"iss": appID,
	})

	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}
	return signed, nil
}

func listInstallations(jwtToken string) ([]installation, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/app/installations", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result []installation
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

func getInstallationToken(jwtToken string, installationID int) (*installationToken, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var token installationToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	return &token, nil
}

func verifyToken(token string) error {
	req, err := http.NewRequest("GET", "https://api.github.com/installation/repositories?per_page=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	fmt.Printf("  HTTP 200 OK (X-RateLimit-Remaining: %s)\n", resp.Header.Get("X-RateLimit-Remaining"))
	return nil
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}
