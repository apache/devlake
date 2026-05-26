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

package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
)

type resetS3FileMetaProcessed struct{}

func (*resetS3FileMetaProcessed) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Reset processed flag so data will be re-extracted with the new
	// dedup-safe composite-PK schema on next pipeline run
	err := db.UpdateColumn(
		"_tool_q_dev_s3_file_meta",
		"processed", false,
		dal.Where("1 = 1"),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to reset s3_file_meta processed flag")
	}

	return nil
}

func (*resetS3FileMetaProcessed) Version() uint64 {
	return 20260228000002
}

func (*resetS3FileMetaProcessed) Name() string {
	return "Reset s3_file_meta processed flag to re-extract data with dedup-safe schema"
}
