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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/identitystore"

	"github.com/apache/incubator-devlake/plugins/q_dev/models"
)

// IdentityStoreAPI interface for AWS Identity Store operations
// This allows for easier testing with mocks
type IdentityStoreAPI interface {
	DescribeUser(input *identitystore.DescribeUserInput) (*identitystore.DescribeUserOutput, error)
}

// QDevIdentityClient wraps AWS Identity Store client for user display name resolution
type QDevIdentityClient struct {
	IdentityStore IdentityStoreAPI
	StoreId       string
	Region        string
}

// NewQDevIdentityClient creates a new Identity Store client for the given connection
// Returns nil if Identity Store is not configured (empty ID or region)
func NewQDevIdentityClient(connection *models.QDevConnection) (*QDevIdentityClient, error) {
	// Return nil if Identity Store is not configured
	if connection.IdentityStoreId == "" || connection.IdentityStoreRegion == "" {
		return nil, nil
	}

	// Create AWS session with Identity Store region and credentials
	sess, err := newAWSSession(connection, connection.IdentityStoreRegion)
	if err != nil {
		return nil, err
	}

	return &QDevIdentityClient{
		IdentityStore: identitystore.New(sess),
		StoreId:       connection.IdentityStoreId,
		Region:        connection.IdentityStoreRegion,
	}, nil
}

// ResolveUserDisplayName resolves a user ID to a human-readable display name
// Returns the display name if found, otherwise returns the original userId as fallback
func (client *QDevIdentityClient) ResolveUserDisplayName(userId string) (string, error) {
	// Check if client or IdentityStore is nil
	if client == nil || client.IdentityStore == nil {
		return userId, nil
	}

	input := &identitystore.DescribeUserInput{
		IdentityStoreId: aws.String(client.StoreId),
		UserId:          aws.String(userId),
	}

	result, err := client.IdentityStore.DescribeUser(input)
	if err != nil {
		// Return userId as fallback on error, but still return the error for logging
		return userId, err
	}

	// Check if DisplayName exists and is not empty
	if result.DisplayName != nil && *result.DisplayName != "" {
		return *result.DisplayName, nil
	}

	// Fallback to userId if no display name available
	return userId, nil
}
