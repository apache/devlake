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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apache/devlake/plugins/aisdlc/tasks/parse"
)

const (
	maxBodyBytes = 8 << 20
	maxTreePages = 2000
)

// GitlabProject is a project the token can see.
type GitlabProject struct {
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
}

type gitlabTreeEntry struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Type string `json:"type"`
	Mode string `json:"mode"`
}

type gitlabFileMeta struct {
	BlobID   string `json:"blob_id"`
	FilePath string `json:"file_path"`
}

type gitlabCommit struct {
	ID string `json:"id"`
}

// GitlabClient calls the GitLab API with either a bearer token or a private token.
type GitlabClient struct {
	Base   string
	HTTP   *http.Client
	Header http.Header
}

// NewGitlabClient probes bearer authentication and falls back to PRIVATE-TOKEN.
func NewGitlabClient(ctx context.Context, endpoint, token string) (*GitlabClient, error) {
	if token == "" {
		return nil, fmt.Errorf("gitlab token is empty")
	}
	client := &GitlabClient{
		Base: strings.TrimRight(endpoint, "/"),
		HTTP: &http.Client{Timeout: 60 * time.Second},
	}
	if client.Base == "" {
		return nil, fmt.Errorf("gitlab endpoint is empty")
	}
	bearer := http.Header{}
	bearer.Set("Authorization", "Bearer "+token)
	status, err := client.probe(ctx, bearer)
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK {
		client.Header = bearer
		return client, nil
	}
	private := http.Header{}
	private.Set("PRIVATE-TOKEN", token)
	status, err = client.probe(ctx, private)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("gitlab authentication failed with status %d", status)
	}
	client.Header = private
	return client, nil
}

func (c *GitlabClient) probe(ctx context.Context, header http.Header) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/user", nil)
	if err != nil {
		return 0, err
	}
	req.Header = header.Clone()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, nil
}

// WebBase strips the API prefix so evidence links open in the GitLab UI.
func WebBase(endpoint string) string {
	base := strings.TrimRight(endpoint, "/")
	base = strings.TrimSuffix(base, "/api/v4")
	return strings.TrimRight(base, "/")
}

func encodeGitlabPath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "%2F")
}

func (c *GitlabClient) get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	u := c.Base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = c.Header.Clone()
	return c.HTTP.Do(req)
}

func readJSON(resp *http.Response, dest any) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gitlab status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode gitlab response: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// NextPage reads GitLab's X-Next-Page header.
func NextPage(header http.Header) string {
	return header.Get("X-Next-Page")
}

// GetProject loads one project by path.
func (c *GitlabClient) GetProject(ctx context.Context, projectPath string) (*GitlabProject, error) {
	resp, err := c.get(ctx, "/projects/"+encodeGitlabPath(projectPath), nil)
	if err != nil {
		return nil, err
	}
	var project GitlabProject
	if err := readJSON(resp, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// BranchCommit returns the commit sha at ref.
func (c *GitlabClient) BranchCommit(ctx context.Context, projectPath, ref string) (string, error) {
	resp, err := c.get(ctx, "/projects/"+encodeGitlabPath(projectPath)+"/repository/commits/"+encodeGitlabPath(ref), nil)
	if err != nil {
		return "", err
	}
	var commit gitlabCommit
	if err := readJSON(resp, &commit); err != nil {
		return "", err
	}
	if commit.ID == "" {
		return "", fmt.Errorf("gitlab commit response has no id")
	}
	return commit.ID, nil
}

// ListProjects returns projects visible to the token.
func (c *GitlabClient) ListProjects(ctx context.Context, search, page string) ([]GitlabProject, string, error) {
	query := url.Values{}
	query.Set("membership", "true")
	query.Set("simple", "true")
	query.Set("per_page", "50")
	if search != "" {
		query.Set("search", search)
	}
	if page != "" {
		query.Set("page", page)
	}
	resp, err := c.get(ctx, "/projects", query)
	if err != nil {
		return nil, "", err
	}
	next := NextPage(resp.Header)
	var projects []GitlabProject
	if err := readJSON(resp, &projects); err != nil {
		return nil, "", err
	}
	return projects, next, nil
}

// ListBlobs returns parser-owned blobs under prefix.
// ref is a commit SHA when CollectFiles calls this, so the tree matches GetRaw.
func (c *GitlabClient) ListBlobs(ctx context.Context, projectPath, ref, prefix, parser string) ([]parse.TreeEntry, error) {
	prefix = strings.Trim(prefix, "/")
	if prefix != "" && strings.Contains(pathBase(prefix), ".") {
		entry, err := c.fileEntry(ctx, projectPath, ref, prefix)
		if err != nil {
			return nil, err
		}
		return parse.SelectPaths(parser, prefix, []parse.TreeEntry{entry}), nil
	}
	page := "1"
	var entries []parse.TreeEntry
	for i := 0; i < maxTreePages && page != ""; i++ {
		query := url.Values{}
		query.Set("ref", ref)
		query.Set("per_page", "100")
		query.Set("page", page)
		query.Set("recursive", "true")
		if prefix != "" {
			query.Set("path", prefix)
		}
		resp, err := c.get(ctx, "/projects/"+encodeGitlabPath(projectPath)+"/repository/tree", query)
		if err != nil {
			return nil, err
		}
		next := NextPage(resp.Header)
		var batch []gitlabTreeEntry
		if err := readJSON(resp, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, item := range batch {
			entries = append(entries, parse.TreeEntry{ID: item.ID, Path: item.Path, Type: item.Type, Mode: item.Mode})
		}
		if next == "" || next == page {
			break
		}
		page = next
	}
	return parse.SelectPaths(parser, prefix, entries), nil
}

func (c *GitlabClient) fileEntry(ctx context.Context, projectPath, ref, filePath string) (parse.TreeEntry, error) {
	query := url.Values{}
	query.Set("ref", ref)
	resp, err := c.get(ctx, "/projects/"+encodeGitlabPath(projectPath)+"/repository/files/"+encodeGitlabPath(filePath), query)
	if err != nil {
		return parse.TreeEntry{}, err
	}
	var meta gitlabFileMeta
	if err := readJSON(resp, &meta); err != nil {
		return parse.TreeEntry{}, err
	}
	pathValue := meta.FilePath
	if pathValue == "" {
		pathValue = filePath
	}
	return parse.TreeEntry{ID: meta.BlobID, Path: pathValue, Type: "blob", Mode: "100644"}, nil
}

// GetRaw downloads a file body at ref.
func (c *GitlabClient) GetRaw(ctx context.Context, projectPath, ref, filePath string) (string, error) {
	query := url.Values{}
	query.Set("ref", ref)
	resp, err := c.get(ctx, "/projects/"+encodeGitlabPath(projectPath)+"/repository/files/"+encodeGitlabPath(filePath)+"/raw", query)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gitlab raw status %d for %s", resp.StatusCode, filePath)
	}
	return string(body), nil
}

func pathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
