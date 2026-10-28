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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/agentready/models"
)

const maxTreeResponseSize = 5 << 20
// Upstream convention: each submissions/{org}/{repo}/ directory contains timestamped
// assessment files plus an assessment-latest.json symlink pointing to the most recent one.
// We only collect the symlink to avoid duplicate DB rows per repo. The GitHub Contents API
// follows symlinks transparently, so no special handling is needed.
const latestAssessmentFilename = "assessment-latest.json"

var CollectSubmissionsMeta = plugin.SubTaskMeta{
	Name:             "collectSubmissions",
	EntryPoint:       CollectSubmissions,
	EnabledByDefault: true,
	Description:      "Fetch assessment JSON files from the submissions repository for a specific scope",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

type SubmissionEntry struct {
	Org      string
	Repo     string
	Filename string
	TreePath string
}

type githubTreeResponse struct {
	SHA       string            `json:"sha"`
	Tree      []GithubTreeEntry `json:"tree"`
	Truncated bool              `json:"truncated"`
}

type GithubTreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
}

type githubConnLookup struct {
	ID       uint64 `gorm:"primaryKey;column:id"`
	Endpoint string `gorm:"column:endpoint"`
	Token    string `gorm:"column:token;serializer:encdec"`
}

func (githubConnLookup) TableName() string { return "_tool_github_connections" }

type collectorAssessmentJSON struct {
	Repository struct {
		CommitHash string `json:"commit_hash"`
	} `json:"repository"`
}

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

func CollectSubmissions(taskCtx plugin.SubTaskContext) errors.Error {
	ctx := taskCtx.GetContext()
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AgentReadyTaskData)
	conn := data.Connection

	var ghConn githubConnLookup
	if err := db.First(&ghConn, dal.Where("id = ?", conn.GitHubConnectionId)); err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("GitHub connection %d not found", conn.GitHubConnectionId))
	}

	endpoint := ghConn.Endpoint
	if endpoint == "" {
		endpoint = "https://api.github.com"
	}

	submissionsPath := conn.SubmissionsPath
	if submissionsPath == "" {
		submissionsPath = "submissions"
	}

	treeResp, fetchErr := FetchGithubTree(ctx, endpoint, conn.SubmissionsRepo, conn.Branch, ghConn.Token)
	if fetchErr != nil {
		return errors.Default.Wrap(fetchErr, "failed to fetch submissions tree")
	}

	if treeResp.Truncated {
		logger.Warn(nil, "Submissions repo tree is truncated; some entries may be missing")
	}

	allEntries := ParseSubmissionEntries(treeResp.Tree, submissionsPath)

	scopeFullName := data.Options.FullName
	var entries []SubmissionEntry
	for _, e := range allEntries {
		entryFullName := fmt.Sprintf("%s/%s", e.Org, e.Repo)
		if entryFullName == scopeFullName {
			entries = append(entries, e)
		}
	}

	logger.Info("Found %d assessment files for scope %s", len(entries), scopeFullName)
	taskCtx.SetProgress(0, len(entries))

	now := time.Now()
	for _, entry := range entries {
		rawJSON, fetchErr := FetchGithubAssessment(ctx, endpoint, conn.SubmissionsRepo, entry.TreePath, conn.Branch, ghConn.Token)
		if fetchErr != nil {
			logger.Warn(nil, "Failed to fetch submission %s: %v", entry.TreePath, fetchErr)
			taskCtx.IncProgress(1)
			continue
		}
		if rawJSON == "" {
			taskCtx.IncProgress(1)
			continue
		}

		var partial collectorAssessmentJSON
		if jsonErr := json.Unmarshal([]byte(rawJSON), &partial); jsonErr != nil {
			logger.Warn(nil, "Failed to parse submission JSON %s: %v", entry.TreePath, jsonErr)
			taskCtx.IncProgress(1)
			continue
		}

		commitHash := partial.Repository.CommitHash
		if commitHash == "" {
			logger.Warn(nil, "Submission %s has no commit_hash, skipping", entry.TreePath)
			taskCtx.IncProgress(1)
			continue
		}

		repoId := scopeFullName
		assessment := &models.AgentReadyAssessment{
			Id:           fmt.Sprintf("%s:%s", repoId, commitHash),
			RepoId:       repoId,
			RepoName:     scopeFullName,
			ConnectionId: conn.ID,
			Provider:     "github",
			CollectedAt:  now,
			RawJSON:      rawJSON,
		}

		if saveErr := db.CreateOrUpdate(assessment); saveErr != nil {
			logger.Warn(saveErr, "Failed to save submission assessment for %s", entry.TreePath)
		}
		taskCtx.IncProgress(1)
	}

	return nil
}

func ParseSubmissionEntries(tree []GithubTreeEntry, submissionsPath string) []SubmissionEntry {
	prefix := submissionsPath + "/"
	var entries []SubmissionEntry

	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		if !strings.HasPrefix(entry.Path, prefix) {
			continue
		}
		if !strings.HasSuffix(entry.Path, ".json") {
			continue
		}

		relPath := strings.TrimPrefix(entry.Path, prefix)
		parts := strings.SplitN(relPath, "/", 4)
		if len(parts) != 3 {
			continue
		}

		if parts[2] != latestAssessmentFilename {
			continue
		}

		entries = append(entries, SubmissionEntry{
			Org:      parts[0],
			Repo:     parts[1],
			Filename: parts[2],
			TreePath: entry.Path,
		})
	}

	return entries
}

func FetchGithubTree(ctx context.Context, endpoint, fullName, branch, token string, client ...*http.Client) (*githubTreeResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("a GitHub token is required to fetch the tree")
	}
	if branch == "" {
		return nil, fmt.Errorf("branch is required; resolve the default branch before calling FetchGithubTree")
	}

	hc := defaultHTTPClient
	if len(client) > 0 && client[0] != nil {
		hc = client[0]
	}

	endpoint = strings.TrimSuffix(endpoint, "/")
	apiURL := fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1", endpoint, fullName, url.PathEscape(branch))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating tree request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching tree from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256))
		if readErr != nil {
			return nil, fmt.Errorf("GitHub Trees API returned %d (body unreadable: %w)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("GitHub Trees API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTreeResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading tree response: %w", err)
	}
	if len(body) > maxTreeResponseSize {
		return nil, fmt.Errorf("tree response exceeds %d bytes limit", maxTreeResponseSize)
	}

	var treeResp githubTreeResponse
	if err := json.Unmarshal(body, &treeResp); err != nil {
		return nil, fmt.Errorf("decoding tree response: %w", err)
	}

	return &treeResp, nil
}

func FetchDefaultBranch(ctx context.Context, endpoint, fullName, token string, client ...*http.Client) (string, error) {
	if token == "" {
		return "", fmt.Errorf("a GitHub token is required to fetch the default branch")
	}

	hc := defaultHTTPClient
	if len(client) > 0 && client[0] != nil {
		hc = client[0]
	}

	endpoint = strings.TrimSuffix(endpoint, "/")
	apiURL := fmt.Sprintf("%s/repos/%s", endpoint, fullName)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256))
		if readErr != nil {
			return "", fmt.Errorf("GitHub API returned %d (body unreadable: %w)", resp.StatusCode, readErr)
		}
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		DefaultBranch string `json:"default_branch"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTreeResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	if len(body) > maxTreeResponseSize {
		return "", fmt.Errorf("response exceeds %d bytes limit", maxTreeResponseSize)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if result.DefaultBranch == "" {
		return "", fmt.Errorf("default_branch is empty in response")
	}

	return result.DefaultBranch, nil
}

const maxAssessmentSize = 10 << 20

func FetchGithubAssessment(ctx context.Context, endpoint, fullName, filePath, branch, token string, client ...*http.Client) (string, error) {
	hc := defaultHTTPClient
	if len(client) > 0 && client[0] != nil {
		hc = client[0]
	}

	endpoint = strings.TrimSuffix(endpoint, "/")
	apiURL := fmt.Sprintf("%s/repos/%s/contents/%s", endpoint, fullName, filePath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	if branch != "" {
		q := req.URL.Query()
		q.Set("ref", branch)
		req.URL.RawQuery = q.Encode()
	}
	if token == "" {
		return "", fmt.Errorf("a GitHub token is required to make requests")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256))
		if readErr != nil {
			return "", fmt.Errorf("GitHub API returned %d (body unreadable: %w)", resp.StatusCode, readErr)
		}
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAssessmentSize+1))
	if err != nil {
		return "", fmt.Errorf("reading GitHub response: %w", err)
	}
	if len(body) > maxAssessmentSize {
		return "", fmt.Errorf("GitHub response exceeds %d bytes limit", maxAssessmentSize)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decoding GitHub response: %w", err)
	}

	if result.Encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding: %s", result.Encoding)
	}

	cleaned := strings.ReplaceAll(result.Content, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("decoding base64 content: %w", err)
	}

	return string(decoded), nil
}
