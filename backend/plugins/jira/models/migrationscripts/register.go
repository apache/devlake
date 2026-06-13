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
	"github.com/apache/incubator-devlake/core/plugin"
)

// All return all the migration scripts
func All() []plugin.MigrationScript {
	return []plugin.MigrationScript{
		new(addSourceTable20220407),
		new(renameSourceTable20220505),
		new(addInitTables20220716),
		new(addTransformationRule20221116),
		new(addProjectName20221215),
		new(addJiraMultiAuth20230129),
		new(removeIssueStdStoryPoint),
		new(addCommitRepoPattern),
		new(expandRemotelinkUrl),
		new(addConnectionIdToTransformationRule),
		new(addChangeTotal20230412),
		new(expandRemotelinkSelfUrl),
		new(addDescAndComments),
		new(renameTr2ScopeConfig),
		new(addRepoUrl),
		new(addApplicationType),
		new(clearRepoPattern),
		new(addRawParamTableForScope),
		new(addIssueRelationship),
		new(dropIssueAllFields),
		new(modifyIssueRelationship),
		new(addComponents20230412),
		new(addFilterJQL),
		new(addWorklogToIssue),
		new(addSubtaskToIssue),
		new(addTmpAccountIdToJiraIssueChangelogItem),
		new(addIssueFieldTable),
		new(changeIssueComponentType),
		new(flushJiraIssues),
		new(updateScopeConfig),
		new(addFixVersions20250619),
		new(addBoardSubQuery20260613),
	}
}
