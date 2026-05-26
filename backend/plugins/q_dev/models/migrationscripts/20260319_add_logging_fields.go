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
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
	"github.com/apache/incubator-devlake/plugins/q_dev/models/migrationscripts/archived"
)

var _ = (*addLoggingFields)(nil)

type addLoggingFields struct{}

func (*addLoggingFields) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&archived.QDevChatLog{},
		&archived.QDevCompletionLog{},
	)
}

func (*addLoggingFields) Version() uint64 {
	return 20260319000001
}

func (*addLoggingFields) Name() string {
	return "Add code_reference_count, web_link_count, has_followup_prompts to chat_log; left/right_context_length to completion_log"
}
