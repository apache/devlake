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

// ProwJobPull represents a pull request associated with a Prow job.
// Prow jobs can be triggered by pull requests, and this struct contains the PR details.
type ProwJobPull struct {
	// Number is the pull request number (e.g., 123)
	Number int `json:"number"`

	// Author is the GitHub username of the PR author
	Author string `json:"author"`

	// SHA is the git commit SHA of the PR head commit
	SHA string `json:"sha"`

	// Title is the pull request title
	Title string `json:"title"`
}

// ProwJobRefs represents git repository references (refs) used by a Prow job.
// Prow jobs clone and test code from specific git repositories and branches.
// This struct contains information about which repository and branch to use.
type ProwJobRefs struct {
	// Org is the GitHub organization name (e.g., "openshift")
	Org string `json:"org"`

	// Repo is the repository name within the organization (e.g., "release")
	Repo string `json:"repo"`

	// BaseRef is the base branch to test against (e.g., "master", "main", "release-4.10")
	BaseRef string `json:"base_ref"`

	// BaseSHA is the specific commit SHA of the base branch to test
	BaseSHA string `json:"base_sha"`

	// Pulls is the list of pull requests being tested (empty for periodic/postsubmit jobs)
	Pulls []ProwJobPull `json:"pulls"`

	// PathAlias is an optional path alias for the repository (used in GOPATH scenarios)
	PathAlias string `json:"path_alias"`
}

// ProwJobSpec represents the specification/configuration of a Prow job.
// It defines what the job should do, where it runs, and how it should be executed.
type ProwJobSpec struct {
	// Job is the name of the Prow job (e.g., "periodic-ci-openshift-release-master-test")
	Job string `json:"job"`

	// Refs are the primary git repository references (main repo to test)
	Refs *ProwJobRefs `json:"refs"`

	// ExtraRefs are additional git repositories needed (dependencies, test frameworks, etc.)
	ExtraRefs []*ProwJobRefs `json:"extra_refs"`

	// Type is the job type: "presubmit", "postsubmit", or "periodic"
	Type string `json:"type"`

	// Agent is the execution agent: "kubernetes" or "jenkins"
	Agent string `json:"agent"`

	// Cluster is the Kubernetes cluster name where the job runs (e.g., "build01", "build07")
	Cluster string `json:"cluster"`

	// Namespace is the Kubernetes namespace (typically "ci")
	Namespace string `json:"namespace"`

	// ReporterConfig is the configuration for reporting results (GCS, Slack, etc.)
	ReporterConfig map[string]interface{} `json:"reporter_config"`

	// Decorate indicates whether Prow should add metadata and artifacts (logs, test results) to the job
	Decorate bool `json:"decorate"`
}

// ProwJobStatus represents the current status and execution details of a Prow job.
// It tracks the job's lifecycle from creation to completion.
type ProwJobStatus struct {
	// State is the current state: "pending", "triggered", "success", "failure", "aborted", "error"
	State string `json:"state"`
	// StartTime is the ISO 8601 timestamp when the job started executing
	StartTime string `json:"startTime"`
	// PendingTime is the ISO 8601 timestamp when the job was created and pending
	PendingTime string `json:"pendingTime"`
	// CompletionTime is the ISO 8601 timestamp when the job finished (success or failure)
	CompletionTime string `json:"completionTime"`
	// URL is the URL to view the job in Prow UI (e.g., "https://prow.ci.openshift.org/view/gs/...")
	URL string `json:"url"`
	// BuildID is the unique build identifier (used for GCS paths and job identification)
	BuildID string `json:"build_id"`
	// PodName is the Kubernetes pod name running the job (for kubernetes agent jobs)
	PodName string `json:"pod_name"`
	// Description is a human-readable description of the current job state
	Description string `json:"description"`
}

// ProwJob represents a complete Prow job from the OpenShift CI Prow API.
// Prow is the CI/CD system used by OpenShift/Kubernetes projects.
// This struct matches the JSON structure returned by https://prow.ci.openshift.org/prowjobs.js
type ProwJob struct {
	// Spec contains the job specification (what to run, where, how)
	Spec ProwJobSpec `json:"spec"`
	// Status contains the job status (current state, timestamps, URLs)
	Status ProwJobStatus `json:"status"`
	// Labels are Kubernetes labels with metadata used for filtering and identification:
	//   - "prow.k8s.io/refs.org": GitHub organization
	//   - "prow.k8s.io/refs.repo": Repository name
	//   - "prow.k8s.io/type": Job type (presubmit/postsubmit/periodic)
	//   - "prow.k8s.io/job": Job name
	//   - "prow.k8s.io/build-id": Build identifier
	Labels map[string]string `json:"labels"`
}
