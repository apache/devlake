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
		new(addInitTables),
		new(addGithubRunsTable),
		new(addGithubJobsTable),
		new(addGithubPipelineTable),
		new(deleteGithubPipelineTable),
		new(addHeadRepoIdFieldInGithubPr),
		new(addEnableGraphqlForConnection),
		new(addTransformationRule20221124),
		new(concatOwnerAndName),
		new(addStdTypeToIssue221230),
		new(addConnectionIdToTransformationRule),
		new(addEnvToRunAndJob),
		new(addGithubCommitAuthorInfo),
		new(fixRunNameToText),
		new(addGithubMultiAuth),
		new(renameTr2ScopeConfig),
		new(addGithubIssueAssignee),
		new(addFullName),
		new(addRawParamTableForScope),
		new(addDeploymentTable),
		new(modifyGithubMilestone),
		new(addEnvNamePattern),
		new(modifyIssueTypeLength),
		new(addWorkflowDisplayTitle),
		new(addReleaseTable),
		new(addReleaseCommitSha),
		new(addMergedByToPr),
		new(restructReviewer),
		new(addIsDraftToPr),
		new(changeIssueComponentType),
		new(addIndexToGithubJobs),
		new(addRefreshTokenFields),
		new(modifyTokenExpiresAtToNullable),
		new(addPrSizeExcludedFileExtensions),
	}
}
