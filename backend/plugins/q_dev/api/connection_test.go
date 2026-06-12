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

package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apache/incubator-devlake/plugins/q_dev/models"
)

func TestValidateConnection_Success(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AuthType:            models.AuthTypeAccessKey,
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			RateLimitPerHour:    20000,
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	err := validateConnection(connection)
	assert.NoError(t, err)
}

func TestValidateConnection_IAMRoleSuccess(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AuthType: models.AuthTypeIAMRole,
			Region:   "us-east-1",
			Bucket:   "my-q-dev-bucket",
		},
	}

	err := validateConnection(connection)
	assert.NoError(t, err)
}

func TestValidateConnection_IAMRoleNoCredentialsRequired(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AuthType:        models.AuthTypeIAMRole,
			AccessKeyId:     "", // Should not be required
			SecretAccessKey: "", // Should not be required
			Region:          "us-east-1",
			Bucket:          "my-q-dev-bucket",
		},
	}

	err := validateConnection(connection)
	assert.NoError(t, err)
}

func TestValidateConnection_DefaultsToAccessKey(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AuthType:        "", // Should default to access_key
			AccessKeyId:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:          "us-east-1",
			Bucket:          "my-q-dev-bucket",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.NoError(t, err)
	assert.Equal(t, models.AuthTypeAccessKey, connection.AuthType)
}

func TestValidateConnection_InvalidAuthType(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AuthType: "invalid",
			Region:   "us-east-1",
			Bucket:   "my-q-dev-bucket",
		},
	}

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AuthType must be")
}

func TestValidateConnection_MissingAccessKeyId(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AuthType:            models.AuthTypeAccessKey,
			AccessKeyId:         "", // Missing
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AccessKeyId is required")
}

func TestValidateConnection_MissingSecretAccessKey(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AuthType:            models.AuthTypeAccessKey,
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "", // Missing
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SecretAccessKey is required")
}

func TestValidateConnection_MissingRegion(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "", // Missing
			Bucket:              "my-q-dev-bucket",
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Region is required")
}

func TestValidateConnection_MissingBucket(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "", // Missing
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Bucket is required")
}

func TestValidateConnection_EmptyIdentityStoreOk(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			IdentityStoreId:     "",
			IdentityStoreRegion: "",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.NoError(t, err)
}

func TestValidateConnection_IdentityStoreRegionWithoutId(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			IdentityStoreId:     "",
			IdentityStoreRegion: "us-east-1",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IdentityStoreRegion")
}

func TestValidateConnection_IdentityStoreIdWithoutRegion(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IdentityStoreId provided but IdentityStoreRegion is empty")
}

func TestValidateConnection_InvalidRateLimit(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			RateLimitPerHour:    -1, // Invalid
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RateLimitPerHour must be positive")
}

func TestValidateConnection_DefaultRateLimit(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			RateLimitPerHour:    0, // Should get default value
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	normalizeConnection(connection)
	err := validateConnection(connection)
	assert.NoError(t, err)
	assert.Equal(t, 20000, connection.RateLimitPerHour) // Should be set to default
}

func TestConnectionRequestBody_Serialization(t *testing.T) {
	// Test that the connection can be properly serialized/deserialized with new fields
	original := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			RateLimitPerHour:    20000,
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(original)
	assert.NoError(t, err)

	// Deserialize from JSON
	var deserialized models.QDevConnection
	err = json.Unmarshal(jsonData, &deserialized)
	assert.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, original.AccessKeyId, deserialized.AccessKeyId)
	assert.Equal(t, original.SecretAccessKey, deserialized.SecretAccessKey)
	assert.Equal(t, original.Region, deserialized.Region)
	assert.Equal(t, original.Bucket, deserialized.Bucket)
	assert.Equal(t, original.RateLimitPerHour, deserialized.RateLimitPerHour)
	assert.Equal(t, original.IdentityStoreId, deserialized.IdentityStoreId)
	assert.Equal(t, original.IdentityStoreRegion, deserialized.IdentityStoreRegion)
}

func TestConnectionSanitization_PreservesIdentityStore(t *testing.T) {
	connection := &models.QDevConnection{
		QDevConn: models.QDevConn{
			AccessKeyId:         "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:              "us-east-1",
			Bucket:              "my-q-dev-bucket",
			RateLimitPerHour:    20000,
			IdentityStoreId:     "d-1234567890",
			IdentityStoreRegion: "us-west-2",
		},
	}

	sanitized := connection.Sanitize()

	// Secret should be sanitized
	assert.NotEqual(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", sanitized.SecretAccessKey)

	// Identity Store fields should be preserved
	assert.Equal(t, "d-1234567890", sanitized.IdentityStoreId)
	assert.Equal(t, "us-west-2", sanitized.IdentityStoreRegion)

	// Other fields should be preserved
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", sanitized.AccessKeyId)
	assert.Equal(t, "us-east-1", sanitized.Region)
	assert.Equal(t, "my-q-dev-bucket", sanitized.Bucket)
	assert.Equal(t, 20000, sanitized.RateLimitPerHour)
}
