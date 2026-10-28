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
	"fmt"
	"regexp"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/pkg/gcshelper"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"
)

// GCSBucket wraps gcshelper.GCSBucket and adds testregistry-specific helpers
// for fetching JUnit XML artifacts from the Openshift CI public bucket.
type GCSBucket struct {
	*gcshelper.GCSBucket
	// bkt is needed for the JUnit artifact listing which uses the raw GCS API.
	bkt *storage.BucketHandle
}

// NewGCSBucketClient creates a new GCS client for the Openshift CI public bucket
// (test-platform-results-public).
func NewGCSBucketClient(ctx context.Context) (*GCSBucket, errors.Error) {
	inner, err := gcshelper.New(ctx, gcshelper.OpenshiftCIBucketName)
	if err != nil {
		return nil, err
	}
	// Access the underlying bucket handle via a thin accessor so that
	// GetJobJunitContent can build its own object iterator.
	bkt := inner.BucketHandle()
	return &GCSBucket{GCSBucket: inner, bkt: bkt}, nil
}

// maxJUnitFilesPerJob limits the number of JUnit files collected per job to
// prevent excessive memory usage.
const maxJUnitFilesPerJob = 50

// JUnitFile represents a single JUnit XML file fetched from GCS.
type JUnitFile struct {
	Content []byte
	Path    string
}

// GetJobJunitContent retrieves all matching JUnit XML files from GCS for a
// specific job. It iterates through all objects in the artifact directory and
// returns every file matching the regex pattern.
//
// Based on the quality-dashboard implementation:
// https://github.com/konflux-ci/quality-dashboard/blob/main/backend/pkg/connectors/gcs/gcs_authentication.go
func (b *GCSBucket) GetJobJunitContent(ctx context.Context, orgName, repoName, pullNumber, jobId, jobType, jobName string, fileName *regexp.Regexp) ([]JUnitFile, error) {
	query := &storage.Query{}

	switch jobType {
	case "presubmit":
		query.Prefix = fmt.Sprintf("pr-logs/pull/%s_%s/%s/%s/%s/artifacts", orgName, repoName, pullNumber, jobName, jobId)
	case "postsubmit":
		query.Prefix = fmt.Sprintf("logs/%s/%s/artifacts", jobName, jobId)
	case "periodic":
		query.Prefix = fmt.Sprintf("logs/%s/%s/artifacts", jobName, jobId)
	default:
		query.Prefix = fmt.Sprintf("logs/%s/%s/artifacts", jobName, jobId)
	}

	var results []JUnitFile

	it := b.bkt.Objects(ctx, query)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return results, fmt.Errorf("GCS listing interrupted: %w", err)
		}

		if fileName != nil && fileName.MatchString(obj.Name) {
			content, err := b.GetContent(ctx, obj.Name)
			if err != nil {
				continue
			}
			results = append(results, JUnitFile{Content: content, Path: obj.Name})
			if len(results) >= maxJUnitFilesPerJob {
				break
			}
		}
	}

	return results, nil
}
