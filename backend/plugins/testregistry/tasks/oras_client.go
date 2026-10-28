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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
)

// ORASClient wraps the ORAS CLI tool for pulling OCI artifacts from Quay.io
// Similar to the qe-tools controller: https://github.com/konflux-ci/qe-tools/blob/main/pkg/oci/controller.go
// Uses the ORAS CLI tool (oras pull) to pull artifacts from Quay.io
type ORASClient struct {
	registryURL string
	repoPath    string
	loggingDir  string
	logger      log.Logger
	orasPath    string // Path to oras executable (default: "oras")
}

// NewORASClient creates a new ORAS client that uses the ORAS CLI tool
//
// Parameters:
//   - ctx: Context for the operation
//   - registryURL: Registry URL (e.g., "quay.io")
//   - repoPath: Repository path (e.g., "org/repo")
//   - loggingDir: Directory to store pulled artifacts (from LOGGING_DIR env var)
//   - logger: Logger for output
//
// Returns:
//   - *ORASClient: The ORAS client instance
//   - errors.Error: Any error encountered during client creation
func NewORASClient(ctx context.Context, registryURL, repoPath, loggingDir string, logger log.Logger) (*ORASClient, errors.Error) {
	if loggingDir == "" {
		// Fallback to LOGGING_DIR environment variable or default
		loggingDir = os.Getenv("LOGGING_DIR")
		if loggingDir == "" {
			loggingDir = "/app/logs"
		}
	}

	// Ensure logging directory exists
	if err := os.MkdirAll(loggingDir, 0755); err != nil {
		return nil, errors.Default.Wrap(err, "failed to create logging directory")
	}

	// Check if oras is available globally in PATH
	orasPath, err := exec.LookPath("oras")
	if err != nil {
		return nil, errors.Default.Wrap(err, "oras CLI not found in PATH. Please ensure ORAS CLI is installed.")
	}

	return &ORASClient{
		registryURL: registryURL,
		repoPath:    repoPath,
		loggingDir:  loggingDir,
		logger:      logger,
		orasPath:    orasPath,
	}, nil
}

// generateUUID generates a unique identifier using crypto/rand
// Returns a hex-encoded string (16 bytes = 32 hex characters)
func generateUUID() (string, errors.Error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Default.Wrap(err, "failed to generate UUID")
	}
	return hex.EncodeToString(bytes), nil
}

// PullArtifact pulls an OCI artifact from Quay.io using ORAS CLI and stores it in a unique tmp directory
//
// This method:
// 1. Generates a unique UUID for this artifact pull
// 2. Creates a tmp/{uuid} directory for storing the artifact
// 3. Uses `oras pull` command to pull the artifact from the registry
// 4. Returns the local path where artifacts were stored (tmp/{uuid})
//
// Parameters:
//   - ctx: Context for the operation
//   - ref: Artifact reference (tag, digest, or "latest")
//
// Returns:
//   - string: Local directory path where artifacts were stored (tmp/{uuid})
//   - errors.Error: Any error encountered during pull operation
func (c *ORASClient) PullArtifact(ctx context.Context, ref string) (string, errors.Error) {
	if ref == "" {
		ref = "latest"
	}

	// Generate unique UUID for this artifact pull
	uuid, err := generateUUID()
	if err != nil {
		return "", err
	}

	// Create unique directory for this artifact: tmp/{uuid}
	tmpBaseDir := filepath.Join(c.loggingDir, "tmp")
	artifactDir := filepath.Join(tmpBaseDir, uuid)
	if mkdirErr := os.MkdirAll(artifactDir, 0755); mkdirErr != nil {
		return "", errors.Default.Wrap(mkdirErr, "failed to create artifact directory")
	}

	// Build artifact reference
	artifactRef := fmt.Sprintf("%s/%s:%s", c.registryURL, c.repoPath, ref)

	c.logger.Info("Pulling OCI artifact using ORAS CLI", "artifact", artifactRef, "target", artifactDir, "uuid", uuid)

	// Execute oras pull command
	// oras pull quay.io/org/repo:tag -o /path/to/output
	cmd := exec.CommandContext(ctx, c.orasPath, "pull", artifactRef, "-o", artifactDir)

	// Capture output for logging
	output, execErr := cmd.CombinedOutput()
	if execErr != nil {
		outputStr := string(output)
		c.logger.Error(execErr, "failed to pull artifact with ORAS CLI", "artifact", artifactRef, "output", outputStr, "uuid", uuid)
		return "", errors.Default.Wrap(execErr, fmt.Sprintf("oras pull failed: %s", outputStr))
	}

	c.logger.Info("Successfully pulled OCI artifact", "artifact", artifactRef, "local_path", artifactDir, "uuid", uuid, "output", string(output))
	return artifactDir, nil
}

// ListArtifacts lists available artifacts (tags) in the Quay.io repository
// Uses Quay.io REST API since ORAS CLI doesn't have a direct tag listing command
//
// Parameters:
//   - ctx: Context for the operation
//
// Returns:
//   - []string: List of available artifact tags/refs
//   - errors.Error: Any error encountered during listing
func (c *ORASClient) ListArtifacts(ctx context.Context) ([]string, errors.Error) {
	// Quay.io API endpoint for listing tags
	tagsURL := fmt.Sprintf("https://quay.io/api/v1/repository/%s/tag/", c.repoPath)

	// Use curl or http client to fetch tags
	// For simplicity, we'll use exec with curl (or we could use http.Client)
	cmd := exec.CommandContext(ctx, "curl", "-s", tagsURL)

	output, err := cmd.Output()
	if err != nil {
		// Fallback: return "latest" if we can't list tags
		c.logger.Warn(err, "failed to list tags from Quay.io API, will use 'latest'", "url", tagsURL)
		return []string{"latest"}, nil
	}

	// Simple JSON parsing (we could use encoding/json but this is simpler for now)
	// For production, we should properly parse the JSON
	outputStr := string(output)
	var tagList []string

	// Simple extraction of tag names from JSON
	// This is a basic implementation - should use proper JSON parsing
	if strings.Contains(outputStr, `"name"`) {
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, `"name"`) && strings.Contains(line, ":") {
				parts := strings.Split(line, `"name":`)
				if len(parts) > 1 {
					namePart := strings.TrimSpace(parts[1])
					namePart = strings.Trim(namePart, `"`)
					namePart = strings.Trim(namePart, `,`)
					namePart = strings.TrimSpace(namePart)
					if namePart != "" && !strings.Contains(namePart, "{") {
						tagList = append(tagList, namePart)
					}
				}
			}
		}
	}

	// Fallback to "latest" if no tags found
	if len(tagList) == 0 {
		c.logger.Info("No tags found or failed to parse, using 'latest'", "output_preview", outputStr[:min(200, len(outputStr))])
		return []string{"latest"}, nil
	}

	return tagList, nil
}

// GetArtifactContent retrieves the content of a file from a pulled artifact
//
// Parameters:
//   - ctx: Context for the operation
//   - artifactPath: Local path to the artifact (from PullArtifact)
//   - filePath: Relative path to the file within the artifact
//
// Returns:
//   - []byte: The file content
//   - errors.Error: Any error encountered during retrieval
func (c *ORASClient) GetArtifactContent(ctx context.Context, artifactPath, filePath string) ([]byte, errors.Error) {
	fullPath := filepath.Join(artifactPath, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("failed to read file %s", fullPath))
	}
	return content, nil
}

// ExtractArtifactFiles lists all files extracted from an OCI artifact
// ORAS CLI extracts files automatically to the output directory
//
// Parameters:
//   - ctx: Context for the operation
//   - artifactPath: Local path to the artifact (from PullArtifact)
//   - targetDir: Directory where files were extracted (same as artifactPath typically)
//
// Returns:
//   - []string: List of extracted file paths
//   - errors.Error: Any error encountered during listing
func (c *ORASClient) ExtractArtifactFiles(ctx context.Context, artifactPath, targetDir string) ([]string, errors.Error) {
	var extractedFiles []string

	// Walk the artifact directory and collect all files
	err := filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Continue on error
		}

		if !info.IsDir() {
			// Get relative path from targetDir
			relPath, err := filepath.Rel(targetDir, path)
			if err != nil {
				return err
			}
			extractedFiles = append(extractedFiles, relPath)
		}
		return nil
	})

	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to walk artifact directory")
	}

	c.logger.Info("Found extracted files in artifact", "artifact_path", artifactPath, "file_count", len(extractedFiles))
	return extractedFiles, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
