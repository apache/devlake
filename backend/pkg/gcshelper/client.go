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

// Package gcshelper provides a lightweight Google Cloud Storage client for
// reading objects from public GCS buckets, primarily for Openshift CI / Prow
// test result retrieval.
package gcshelper

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"github.com/apache/devlake/core/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// OpenshiftCIBucketName is the anonymously readable GCS bucket that stores
// Prow job artifacts. OpenShift CI cut over from test-platform-results to
// test-platform-results-public around 2026-09-17; the old bucket no longer
// allows unauthenticated listing.
const OpenshiftCIBucketName = "test-platform-results-public"

// maxFileSize caps individual GCS object reads at 10 MB.
const maxFileSize = 10 * 1024 * 1024

// HistoryStore is the minimal interface required for fetching Prow job metadata
// from a GCS-like store. Implementations include *GCSBucket (real GCS) and
// test fakes.
type HistoryStore interface {
	// ListSubdirectories returns the immediate child "directory" prefixes one
	// level below the given prefix (using "/" as the GCS delimiter).
	ListSubdirectories(ctx context.Context, prefix string) ([]string, error)
	// ReadFile retrieves the full content of the object at path.
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// GCSBucket wraps a Google Cloud Storage bucket handle.
type GCSBucket struct {
	client *storage.Client
	bkt    *storage.BucketHandle
}

// New creates a GCSBucket client for the given bucket using unauthenticated
// (public) access.
func New(ctx context.Context, bucketName string) (*GCSBucket, errors.Error) {
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create GCS client")
	}
	return &GCSBucket{
		client: client,
		bkt:    client.Bucket(bucketName),
	}, nil
}

// Close releases resources held by the GCS client.
func (b *GCSBucket) Close() error {
	return b.client.Close()
}

// ListSubdirectories returns the immediate child "directory" prefixes one level
// below the given prefix. It uses "/" as the GCS delimiter so that only direct
// children are returned, not all recursive objects.
func (b *GCSBucket) ListSubdirectories(ctx context.Context, prefix string) ([]string, error) {
	query := &storage.Query{Prefix: prefix, Delimiter: "/"}
	var subdirs []string
	it := b.bkt.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return subdirs, fmt.Errorf("GCS listing failed for %s: %w", prefix, err)
		}
		if attrs.Prefix != "" {
			subdirs = append(subdirs, attrs.Prefix)
		}
	}
	return subdirs, nil
}

// ReadFile retrieves the full content of the GCS object at path.
func (b *GCSBucket) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, err := b.GetContent(ctx, path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// GetContent retrieves the content of a GCS object, capped at maxFileSize.
func (b *GCSBucket) GetContent(ctx context.Context, path string) ([]byte, errors.Error) {
	if path == "" {
		return nil, errors.BadInput.New("missing path to GCS content")
	}

	obj := b.bkt.Object(path)
	objAttrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error reading GCS attributes")
	}
	obj = obj.Generation(objAttrs.Generation)
	if objAttrs.Size > maxFileSize {
		return nil, errors.Default.New(fmt.Sprintf("GCS object too large (%d bytes, limit %d)", objAttrs.Size, maxFileSize))
	}

	gcsReader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error reading GCS content")
	}
	defer gcsReader.Close()

	content, err := io.ReadAll(io.LimitReader(gcsReader, maxFileSize))
	if err != nil {
		return nil, errors.Default.Wrap(err, "error reading GCS content")
	}
	return content, nil
}

// BucketHandle returns the underlying *storage.BucketHandle for callers that
// need raw GCS API access beyond what HistoryStore provides.
func (b *GCSBucket) BucketHandle() *storage.BucketHandle {
	return b.bkt
}

// ContentExists checks whether an object exists at the given GCS path.
func (b *GCSBucket) ContentExists(ctx context.Context, path string) bool {
	if path == "" {
		return false
	}
	_, err := b.bkt.Object(path).Attrs(ctx)
	return err == nil
}
