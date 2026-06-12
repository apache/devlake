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
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/apache/incubator-devlake/plugins/q_dev/models"
)

// newAWSSession creates an AWS session for the given connection and region.
// For access_key auth, static credentials are used; for iam_role, the default credential chain is used.
func newAWSSession(conn *models.QDevConnection, region string) (*session.Session, error) {
	cfg := &aws.Config{
		Region: aws.String(region),
	}
	if !conn.IsIAMRoleAuth() {
		cfg.Credentials = credentials.NewStaticCredentials(
			conn.AccessKeyId,
			conn.SecretAccessKey,
			"",
		)
	}
	return session.NewSession(cfg)
}
