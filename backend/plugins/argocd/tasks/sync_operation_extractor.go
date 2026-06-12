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
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/argocd/models"
)

var _ plugin.SubTaskEntryPoint = ExtractSyncOperations

var ExtractSyncOperationsMeta = plugin.SubTaskMeta{
	Name:             "extractSyncOperations",
	EntryPoint:       ExtractSyncOperations,
	EnabledByDefault: true,
	Description:      "Extract sync operations from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{RAW_SYNC_OPERATION_TABLE},
	ProductTables:    []string{models.ArgocdSyncOperation{}.TableName()},
}

// ArgocdApiSyncSource represents a single source in a multi-source ArgoCD application.
type ArgocdApiSyncSource struct {
	RepoURL string `json:"repoURL"`
	Chart   string `json:"chart"`
}

type ArgocdApiSyncOperation struct {
	// For history entries
	ID              int64      `json:"id"`
	Revision        string     `json:"revision"`
	Revisions       []string   `json:"revisions"` // multi-source apps populate this instead of revision
	DeployedAt      time.Time  `json:"deployedAt"`
	DeployStartedAt *time.Time `json:"deployStartedAt"`
	Source          struct {
		RepoURL string `json:"repoURL"`
	} `json:"source"`
	Sources     []ArgocdApiSyncSource `json:"sources"` // multi-source apps populate this instead of source
	InitiatedBy struct {
		Username  string `json:"username"`
		Automated bool   `json:"automated"`
	} `json:"initiatedBy"`
	Metadata  ArgocdApiSyncOperationMetadata `json:"metadata"`
	Operation ArgocdApiSyncOperationDetails  `json:"operation"`

	// For operationState (current operation)
	Phase      string     `json:"phase"` // Succeeded, Failed, Error, Running, Terminating
	Message    string     `json:"message"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	SyncResult struct {
		Revision  string                      `json:"revision"`
		Revisions []string                    `json:"revisions"` // multi-source apps
		Resources []ArgocdApiSyncResourceItem `json:"resources"`
	} `json:"syncResult"`
}

type ArgocdApiSyncResourceItem struct {
	Group     string   `json:"group"`
	Version   string   `json:"version"`
	Kind      string   `json:"kind"`
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Message   string   `json:"message"`
	Images    []string `json:"images"`
}

type ArgocdApiSyncOperationMetadata struct {
	Images    []string                                 `json:"images"`
	Resources []ArgocdApiSyncOperationMetadataResource `json:"resources"`
}

type ArgocdApiSyncOperationMetadataResource struct {
	Images []string `json:"images"`
}

type ArgocdApiSyncOperationDetails struct {
	Metadata ArgocdApiSyncOperationMetadata `json:"metadata"`
	Sync     ArgocdApiSyncOperationSync     `json:"sync"`
}

type ArgocdApiSyncOperationSync struct {
	Resources []ArgocdApiSyncResourceItem `json:"resources"`
}

func ExtractSyncOperations(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*ArgocdTaskData)

	var summaryImages []string
	application := &models.ArgocdApplication{}
	db := taskCtx.GetDal()
	if err := db.First(
		application,
		dal.Where("connection_id = ? AND name = ?", data.Options.ConnectionId, data.Options.ApplicationName),
	); err != nil {
		if !db.IsErrorNotFound(err) {
			return errors.Default.Wrap(err, "error loading argocd application for summary images")
		}
	} else {
		summaryImages = application.SummaryImages
	}
	summaryImages = normalizeImages(summaryImages)

	revisionImageCache := make(map[string][]string)
	revisionRecords := make([]models.ArgocdRevisionImage, 0)
	err := db.All(
		&revisionRecords,
		dal.Where("connection_id = ? AND application_name = ?", data.Options.ConnectionId, data.Options.ApplicationName),
	)
	if err != nil && !db.IsErrorNotFound(err) {
		return errors.Default.Wrap(err, "error loading argocd revision images")
	}
	for _, record := range revisionRecords {
		if record.Revision == "" {
			continue
		}
		normalized := normalizeImages(record.Images)
		if len(normalized) == 0 {
			continue
		}
		revisionImageCache[record.Revision] = normalized
	}

	revisionDirty := make(map[string][]string)

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: RAW_SYNC_OPERATION_TABLE,
			Params: models.ArgocdApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.ApplicationName,
			},
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiOp ArgocdApiSyncOperation
			err := json.Unmarshal(row.Data, &apiOp)
			if err != nil {
				return nil, errors.Default.Wrap(err, "error unmarshaling sync operation")
			}

			syncOp := &models.ArgocdSyncOperation{
				ConnectionId:    data.Options.ConnectionId,
				ApplicationName: data.Options.ApplicationName,
				NoPKModel:       common.NewNoPKModel(),
			}

			normalize := func(t time.Time) *time.Time {
				if t.IsZero() {
					return nil
				}
				return &t
			}
			normalizePtr := func(t *time.Time) *time.Time {
				if t == nil {
					return nil
				}
				if t.IsZero() {
					return nil
				}
				return t
			}

			isOperationState := apiOp.Phase != ""

			// operationState always represents the same sync as the latest history entry
			// (when phase=Succeeded). Skip it to avoid the duplicate DeploymentId.
			if isOperationState && isRedundantOperationState(apiOp.Phase) {
				return nil, nil
			}

			// For multi-source apps ArgoCD sets revisions[] instead of revision. Resolve
			// the single commit SHA we care about before deciding whether to skip this entry.
			if apiOp.Revision == "" {
				apiOp.Revision = resolveMultiSourceRevision(apiOp.Revisions, apiOp.Sources)
			}
			if !isOperationState && apiOp.DeployedAt.IsZero() && apiOp.Revision == "" {
				return nil, nil
			}

			// Resolve the git repo URL at extraction time so the convertor can set
			// cicd_deployment_commits.repo_url correctly even when
			// _tool_argocd_applications.repo_url is empty (e.g. for multi-source apps
			// whose collectApplications subtask was skipped due to state caching).
			syncOp.RepoURL = resolveGitRepoURL(apiOp.Source.RepoURL, apiOp.Sources)

			if isOperationState {
				start := normalize(apiOp.StartedAt)
				if start != nil {
					syncOp.DeploymentId = start.Unix()
				} else {
					syncOp.DeploymentId = time.Now().Unix()
				}
				// Prefer the top-level resolved revision; fall back to syncResult.
				syncOp.Revision = apiOp.Revision
				if syncOp.Revision == "" {
					syncOp.Revision = resolveMultiSourceRevision(apiOp.SyncResult.Revisions, apiOp.Sources)
				}
				if syncOp.Revision == "" {
					syncOp.Revision = apiOp.SyncResult.Revision
				}
				syncOp.StartedAt = start
				syncOp.FinishedAt = normalizePtr(apiOp.FinishedAt)
				syncOp.Phase = apiOp.Phase
				syncOp.Message = apiOp.Message
				syncOp.ResourcesCount = len(apiOp.SyncResult.Resources)
			} else {
				deployedAt := normalize(apiOp.DeployedAt)
				if deployedAt != nil {
					syncOp.DeploymentId = deployedAt.Unix()
				} else {
					syncOp.DeploymentId = time.Now().Unix()
				}

				syncOp.Revision = apiOp.Revision
				syncOp.FinishedAt = deployedAt
				start := normalizePtr(apiOp.DeployStartedAt)
				if start == nil {
					start = deployedAt
				}
				syncOp.StartedAt = start
				syncOp.Phase = "Succeeded"

				if apiOp.InitiatedBy.Automated {
					syncOp.InitiatedBy = "automated"
				} else {
					syncOp.InitiatedBy = apiOp.InitiatedBy.Username
				}
			}

			syncOp.Kind = extractPrimaryDeploymentKind(apiOp.SyncResult.Resources)

			payloadImages := collectContainerImages(&apiOp)
			images := copyStringSlice(payloadImages)

			if len(images) > 0 {
				if syncOp.Revision != "" {
					cached := revisionImageCache[syncOp.Revision]
					if !stringSlicesEqual(cached, images) {
						revisionImageCache[syncOp.Revision] = copyStringSlice(images)
						revisionDirty[syncOp.Revision] = copyStringSlice(images)
					}
				}
			} else if syncOp.Revision != "" {
				if cached := revisionImageCache[syncOp.Revision]; len(cached) > 0 {
					images = copyStringSlice(cached)
				} else if len(summaryImages) > 0 {
					// Fallback: use application summary images and cache for this revision.
					images = copyStringSlice(summaryImages)
					revisionImageCache[syncOp.Revision] = copyStringSlice(images)
					revisionDirty[syncOp.Revision] = copyStringSlice(images)
				}
			}

			if len(images) > 0 {
				syncOp.ContainerImages = images
			}

			results := []interface{}{syncOp}
			if syncOp.Revision != "" {
				if dirtyImages, ok := revisionDirty[syncOp.Revision]; ok && len(dirtyImages) > 0 {
					revision := &models.ArgocdRevisionImage{
						ConnectionId:    syncOp.ConnectionId,
						ApplicationName: syncOp.ApplicationName,
						Revision:        syncOp.Revision,
						Images:          copyStringSlice(dirtyImages),
					}
					results = append(results, revision)
					delete(revisionDirty, syncOp.Revision)
				}
			}

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}

// extractPrimaryDeploymentKind identifies the primary deployment resource kind from the resources list.
// It prioritizes actual deployment resources over supporting resources like Services or Ingresses.
func extractPrimaryDeploymentKind(resources []ArgocdApiSyncResourceItem) string {
	// Priority order for deployment resources
	priorityKinds := []string{
		"Rollout",     // Argo Rollouts
		"Deployment",  // Standard K8s Deployment
		"StatefulSet", // Stateful applications
		"DaemonSet",   // Node-level deployments
		"ReplicaSet",  // Direct ReplicaSet management
		"Job",         // Batch jobs
		"CronJob",     // Scheduled jobs
	}

	for _, priorityKind := range priorityKinds {
		for _, resource := range resources {
			if resource.Kind == priorityKind {
				return priorityKind
			}
		}
	}

	for _, resource := range resources {
		if resource.Kind != "" {
			return resource.Kind
		}
	}

	return ""
}

func collectContainerImages(apiOp *ArgocdApiSyncOperation) []string {
	if apiOp == nil {
		return nil
	}

	var collected []string
	appendAll := func(images []string) {
		if len(images) == 0 {
			return
		}
		collected = append(collected, images...)
	}
	appendMetadataResources := func(resources []ArgocdApiSyncOperationMetadataResource) {
		for _, r := range resources {
			appendAll(r.Images)
		}
	}
	appendResourceItems := func(resources []ArgocdApiSyncResourceItem) {
		for _, r := range resources {
			appendAll(r.Images)
		}
	}

	appendAll(apiOp.Metadata.Images)
	appendMetadataResources(apiOp.Metadata.Resources)
	appendAll(apiOp.Operation.Metadata.Images)
	appendMetadataResources(apiOp.Operation.Metadata.Resources)
	appendResourceItems(apiOp.Operation.Sync.Resources)
	appendResourceItems(apiOp.SyncResult.Resources)

	return normalizeImages(collected)
}

func normalizeImages(images []string) []string {
	if len(images) == 0 {
		return nil
	}
	uniq := make(map[string]struct{}, len(images))
	for _, image := range images {
		trimmed := strings.TrimSpace(image)
		if trimmed == "" {
			continue
		}
		uniq[trimmed] = struct{}{}
	}
	if len(uniq) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(uniq))
	for image := range uniq {
		normalized = append(normalized, image)
	}
	sort.Strings(normalized)
	return normalized
}

func copyStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	dup := make([]string, len(values))
	copy(dup, values)
	return dup
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// resolveMultiSourceRevision picks the git commit SHA from a multi-source ArgoCD
// application's revisions slice. ArgoCD multi-source apps store one revision per
// source: Helm chart sources carry a semver tag while git sources carry a 40-hex
// commit SHA. We prefer the first git-hosted source (github.com / gitlab.com /
// bitbucket.org) and fall back to any entry that looks like a 40-character hex SHA.
//
// Single-source apps already populate the top-level "revision" field, so this
// function is only called when that field is empty.
func resolveMultiSourceRevision(revisions []string, sources []ArgocdApiSyncSource) string {
	if len(revisions) == 0 {
		return ""
	}

	// Pass 1: prefer a revision whose corresponding source is a git hosting service.
	for i, rev := range revisions {
		if i >= len(sources) {
			break
		}
		repoURL := sources[i].RepoURL
		if isGitHostedURL(repoURL) && isCommitSHA(rev) {
			return rev
		}
	}

	// Pass 2: accept any revision that looks like a full commit SHA regardless of
	// source type (covers self-hosted Gitea / Forgejo / etc.).
	for _, rev := range revisions {
		if isCommitSHA(rev) {
			return rev
		}
	}

	return ""
}

// isGitHostedURL returns true when the URL belongs to a known git hosting service
// or is clearly not a Helm chart registry.
func isGitHostedURL(repoURL string) bool {
	if repoURL == "" {
		return false
	}
	gitHosts := []string{
		"github.com",
		"gitlab.com",
		"bitbucket.org",
		"dev.azure.com",
		"ssh.dev.azure.com",
		"gitea.",
		"forgejo.",
	}
	lower := strings.ToLower(repoURL)
	for _, host := range gitHosts {
		if strings.Contains(lower, host) {
			return true
		}
	}
	// Any https/ssh git URL that is not a chart registry (gs://, oci://, https://*.azurecr.io, etc.)
	chartPrefixes := []string{"gs://", "oci://", "s3://"}
	for _, pfx := range chartPrefixes {
		if strings.HasPrefix(lower, pfx) {
			return false
		}
	}
	// .git suffix is a strong signal
	return strings.HasSuffix(strings.TrimSpace(repoURL), ".git")
}

// resolveGitRepoURL returns the best git repository URL from a sync operation's
// source metadata. For single-source apps the source.repoURL is used directly.
// For multi-source apps (sources[]) the first URL that matches a known git
// hosting service is preferred; if none match the heuristic, the first non-chart
// HTTPS/SSH URL is used as a fallback so that cicd_deployment_commits.repo_url
// is never left as the deployment-name placeholder.
//
// This is called during extractSyncOperations which always runs, providing
// reliable repo_url population even when extractApplications is skipped due
// to the collector state cache.
func resolveGitRepoURL(singleSourceURL string, sources []ArgocdApiSyncSource) string {
	// Single-source app: use the URL directly.
	if singleSourceURL != "" {
		return singleSourceURL
	}

	// Multi-source app: pass 1 — prefer a known git host.
	for _, src := range sources {
		if isGitHostedURL(src.RepoURL) {
			return src.RepoURL
		}
	}

	// Pass 2 — fall back to the first non-chart URL (covers self-hosted instances
	// not in the known-host list, e.g. on-prem GitLab with a custom domain).
	chartPrefixes := []string{"gs://", "oci://", "s3://"}
	for _, src := range sources {
		if src.RepoURL == "" {
			continue
		}
		lower := strings.ToLower(src.RepoURL)
		isChart := false
		for _, pfx := range chartPrefixes {
			if strings.HasPrefix(lower, pfx) {
				isChart = true
				break
			}
		}
		if !isChart {
			return src.RepoURL
		}
	}

	return ""
}

// isCommitSHA returns true for a 40-character lowercase hexadecimal string,
// which is the standard representation of a Git commit SHA-1.
func isCommitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isRedundantOperationState reports whether an operationState entry should be
// skipped because it duplicates the most recent history entry.
//
// ArgoCD exposes a completed sync in two places:
//   - status.history[]     — authoritative log, one entry per sync
//   - status.operationState — mirrors the last sync while it is current
//
// When phase is Succeeded the operationState is always already present in
// history[]. Emitting it would produce a second ArgocdSyncOperation whose
// DeploymentId differs by one second (startedAt vs deployedAt), creating a
// duplicate deployment in DORA metrics.
//
// In-flight (Running, Terminating) and terminal-failure (Failed, Error) states
// are not yet — or may never be — written to history, so they must be kept.
func isRedundantOperationState(phase string) bool {
	return phase == "Succeeded"
}
