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
	"net/http"
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
)

// QuayClient wraps a Quay.io API client for listing artifacts/tags
// Similar to GCSBucket for Openshift CI
type QuayClient struct {
	baseURL    string
	httpClient *http.Client
	logger     log.Logger
}

// QuayTag represents a tag from Quay.io API
type QuayTag struct {
	Name           string  `json:"name"`
	Reversion      bool    `json:"reversion"`
	StartTS        int64   `json:"start_ts"` // Unix timestamp - used for date filtering
	EndTS          *int64  `json:"end_ts,omitempty"`
	LastModified   string  `json:"last_modified"` // RFC 1123 format string (not used for filtering, stored as string to avoid parsing issues)
	Expiration     *string `json:"expiration,omitempty"`
	DockerImageID  string  `json:"docker_image_id"`
	IsManifestList bool    `json:"is_manifest_list"`
	ManifestDigest string  `json:"manifest_digest"`
	Size           *int64  `json:"size,omitempty"`
}

// QuayTagsResponse represents the response from Quay.io tags API
type QuayTagsResponse struct {
	Tags          []QuayTag `json:"tags"`
	Page          int       `json:"page"`
	HasAdditional bool      `json:"has_additional"`
	NextPage      string    `json:"next_page,omitempty"`
}

// NewQuayClient creates a new Quay.io API client
//
// Parameters:
//   - ctx: Context for the operation
//   - logger: Logger for output
//
// Returns:
//   - *QuayClient: The Quay.io client instance
//   - errors.Error: Any error encountered during client creation
func NewQuayClient(ctx context.Context, logger log.Logger) (*QuayClient, errors.Error) {
	return &QuayClient{
		baseURL:    "https://quay.io",
		httpClient: &http.Client{},
		logger:     logger,
	}, nil
}

// ListTags lists all tags for a repository with optional date filtering
//
// Parameters:
//   - ctx: Context for the operation
//   - org: Organization name
//   - repo: Repository name
//   - since: Optional start date (from sync policy)
//   - until: Optional end date (from sync policy)
//
// Returns:
//   - []QuayTag: List of tags matching the date range
//   - errors.Error: Any error encountered during listing
func (c *QuayClient) ListTags(ctx context.Context, org, repo string, since, until *time.Time) ([]QuayTag, errors.Error) {
	var allTags []QuayTag
	page := 1
	hasMore := true

	// Quay.io API endpoint: /api/v1/repository/{org}/{repo}/tag/
	baseURL := fmt.Sprintf("%s/api/v1/repository/%s/%s/tag/", c.baseURL, org, repo)
	apiURL := baseURL

	for hasMore {
		// Build request with pagination
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to create request")
		}

		// If we're not using NextPage (first iteration or fallback), add page parameter manually
		if page > 1 && !strings.Contains(apiURL, "?") {
			q := req.URL.Query()
			q.Set("page", fmt.Sprintf("%d", page))
			req.URL.RawQuery = q.Encode()
		}

		c.logger.Debug("Fetching tags from Quay.io", "url", req.URL.String(), "page", page)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to fetch tags from Quay.io")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, errors.Default.New(fmt.Sprintf("Quay.io API returned status %d for tags", resp.StatusCode))
		}

		var tagsResponse QuayTagsResponse
		if err := json.NewDecoder(resp.Body).Decode(&tagsResponse); err != nil {
			return nil, errors.Default.Wrap(err, "failed to parse tags response")
		}

		// Filter tags by date range
		for _, tag := range tagsResponse.Tags {
			// Convert start_ts (Unix timestamp) to time.Time
			tagTime := time.Unix(tag.StartTS, 0)

			// Apply date filters
			if since != nil && tagTime.Before(*since) {
				c.logger.Debug("Skipping tag (before since date)", "tag", tag.Name, "tag_time", tagTime, "since", *since)
				continue
			}
			if until != nil && tagTime.After(*until) {
				c.logger.Debug("Skipping tag (after until date)", "tag", tag.Name, "tag_time", tagTime, "until", *until)
				continue
			}

			allTags = append(allTags, tag)
		}

		// Check if there are more pages
		hasMore = tagsResponse.HasAdditional
		if hasMore {
			if tagsResponse.NextPage != "" {
				// Use the next_page query string directly - it already contains pagination info
				apiURL = baseURL + "?" + tagsResponse.NextPage
			} else {
				// Fallback: increment page number if NextPage URL not provided
				page++
				apiURL = baseURL
			}
		}
	}

	c.logger.Info("Listed tags from Quay.io", "org", org, "repo", repo, "total_tags", len(allTags), "since", since, "until", until)
	return allTags, nil
}

// GetTagByName gets a specific tag by name
//
// Parameters:
//   - ctx: Context for the operation
//   - org: Organization name
//   - repo: Repository name
//   - tagName: Tag name
//
// Returns:
//   - *QuayTag: The tag if found, nil otherwise
//   - errors.Error: Any error encountered during fetching
func (c *QuayClient) GetTagByName(ctx context.Context, org, repo, tagName string) (*QuayTag, errors.Error) {
	apiURL := fmt.Sprintf("%s/api/v1/repository/%s/%s/tag/%s", c.baseURL, org, repo, tagName)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to fetch tag from Quay.io")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Tag not found
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Default.New(fmt.Sprintf("Quay.io API returned status %d for tag %s", resp.StatusCode, tagName))
	}

	var tag QuayTag
	if err := json.NewDecoder(resp.Body).Decode(&tag); err != nil {
		return nil, errors.Default.Wrap(err, "failed to parse tag response")
	}

	return &tag, nil
}
