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

package domaininfo

import (
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/codequality"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/qa"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
)

func GetDomainTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		// code
		&code.AiReview{},
		&code.AiCommit{},
		&code.AiFailurePrediction{},
		&code.AiPredictionMetrics{},
		&code.Commit{},
		&code.CommitFile{},
		&code.CommitFileComponent{},
		&code.CommitParent{},
		&code.Component{},
		&code.CommitLineChange{},
		&code.PullRequest{},
		&code.PullRequestComment{},
		&code.PullRequestCommit{},
		&code.PullRequestLabel{},
		&code.PullRequestReviewer{},
		&code.PullRequestAssignee{},
		&code.Ref{},
		&code.CommitsDiff{},
		&code.RefCommit{},
		&code.RefsPrCherrypick{},
		&code.Repo{},
		&code.RepoCommit{},
		&code.RepoLanguage{},
		&code.RepoSnapshot{},
		// codequality
		&codequality.CqFileMetrics{},
		&codequality.CqIssueCodeBlock{},
		&codequality.CqIssue{},
		&codequality.CqIssueImpact{},
		&codequality.CqProject{},
		&codequality.CqProjectMetricsHistory{},
		// crossdomain
		&crossdomain.Account{},
		&crossdomain.BoardRepo{},
		&crossdomain.IssueCommit{},
		&crossdomain.IssueRepoCommit{},
		&crossdomain.ProjectMapping{},
		&crossdomain.ProjectIncidentDeploymentRelationship{},
		&crossdomain.ProjectPrMetric{},
		&crossdomain.PullRequestIssue{},
		&crossdomain.RefsIssuesDiffs{},
		&crossdomain.Team{},
		&crossdomain.TeamUser{},
		&crossdomain.User{},
		&crossdomain.UserAccount{},
		// devops
		&devops.CICDPipeline{},
		&devops.CICDTask{},
		&devops.CicdDeploymentCommit{},
		&devops.CicdDeploymentSubproject{},
		&devops.CiCDPipelineCommit{},
		&devops.CicdScope{},
		&devops.CICDDeployment{},
		&devops.CicdRelease{},
		// didgen no table
		// ticket
		&ticket.Board{},
		&ticket.BoardIssue{},
		&ticket.BoardSprint{},
		&ticket.Issue{},
		&ticket.IssueChangelogs{},
		&ticket.IssueComment{},
		&ticket.IssueLabel{},
		&ticket.IssueWorklog{},
		&ticket.Sprint{},
		&ticket.SprintIssue{},
		&ticket.IssueAssignee{},
		&ticket.IssueRelationship{},
		&ticket.IssueCustomArrayField{},
		&ticket.Incident{},
		&ticket.IncidentAssignee{},
		// qa
		&qa.QaProject{},
		&qa.QaApi{},
		&qa.QaTestCase{},
		&qa.QaTestCaseExecution{},
	}
}
