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
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/q_dev/models"
	"github.com/aws/aws-sdk-go/service/s3"
)

func NewQDevS3Client(taskCtx plugin.TaskContext, connection *models.QDevConnection) (*QDevS3Client, errors.Error) {
	// Create AWS session
	sess, err := newAWSSession(connection, connection.Region)
	if err != nil {
		return nil, errors.Convert(err)
	}

	// 创建S3服务客户端
	s3Client := s3.New(sess)

	return &QDevS3Client{
		S3:     s3Client,
		Bucket: connection.Bucket,
	}, nil
}
