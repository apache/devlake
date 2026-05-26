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
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&CollectRunsMeta)
}

const RAW_RUN_TABLE = "github_api_runs"

// PAGE_SIZE is below GitHub's 100-item max to avoid oversized response bodies (#3199).
const PAGE_SIZE = 30

// FILTERED_SEARCH_CAP is GitHub's per-query item cap for `/actions/runs` in filtered mode
// (`created=<from>..<to>`); exceeding it triggers HTTP 422. See #8842.
const FILTERED_SEARCH_CAP = 1000

// githubTimeLayout is the ISO8601 format GitHub expects in the `created` filter.
const githubTimeLayout = "2006-01-02T15:04:05Z"

// TimeWindow is an inclusive-both-ends range for the `/actions/runs` `created=<from>..<to>` query.
type TimeWindow struct {
	From time.Time
	To   time.Time
}

var CollectRunsMeta = plugin.SubTaskMeta{
	Name:             "Collect Workflow Runs",
	EntryPoint:       CollectRuns,
	EnabledByDefault: true,
	Description:      "Collect Runs data from Github action api, supports both timeFilter and diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{},
	ProductTables:    []string{RAW_RUN_TABLE},
}

// CollectRuns collects GitHub Workflow Runs into the raw table, working around the two
// pagination caps of `/actions/runs`: 40k items in unfiltered mode and 1000 items per
// filtered window. It probes each candidate window with `per_page=1`, bisects recursively
// until every leaf is under FILTERED_SEARCH_CAP, then feeds the leaves to a single
// ApiCollector so the raw table is truncated only once per fullsync.
func CollectRuns(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	logger := taskCtx.GetLogger()

	manager, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: GithubApiParams{
			ConnectionId: data.Options.ConnectionId,
			Name:         data.Options.Name,
		},
		Table: RAW_RUN_TABLE,
	})
	if err != nil {
		return err
	}

	// Normalize both bounds to second precision so `created=<from>..<to>` queries and the
	// persisted LatestSuccessStart share the same boundary; without this, every incremental
	// sync would re-fetch up to 1s of overlap. For incremental syncs we also advance
	// `windowStart` past the previously collected second (inclusive-both-ends), while
	// fullsync + TimeAfter keeps the user-specified bound inclusive.
	createdAfter := manager.GetSince()
	sinceSource := "state_since"
	syncPolicy := taskCtx.TaskContext().SyncPolicy()
	if createdAfter == nil {
		sinceSource = "none"
	}
	if createdAfter == nil && (syncPolicy == nil || !syncPolicy.FullSync) {
		fallbackSince, err := loadLatestRunUpdatedAt(taskCtx, data.Options.ConnectionId, data.Options.GithubId)
		if err != nil {
			return err
		}
		if fallbackSince != nil {
			createdAfter = fallbackSince
			sinceSource = "tool_runs_fallback"
			logger.Info("cicd_run_collector: collector state missing; bootstrapping since from existing _tool_github_runs at %s", fallbackSince.UTC().Format(time.RFC3339))
		} else {
			logger.Debug("cicd_run_collector: collector state missing and no _tool_github_runs timestamp found")
		}
	}
	untilPtr := manager.GetUntil()
	*untilPtr = untilPtr.Truncate(time.Second)
	until := *untilPtr

	var windowStart time.Time
	if createdAfter != nil {
		windowStart = createdAfter.Truncate(time.Second)
		if manager.IsIncremental() {
			windowStart = windowStart.Add(time.Second)
		}
	} else {
		// 2018-01-01 conservatively predates GitHub Actions' late-2019 GA.
		windowStart = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
		sinceSource = "epoch_fullsync"
	}

	logger.Info("cicd_run_collector: collecting workflow runs in [%s, %s] (incremental=%v, since_source=%s)",
		windowStart.Format(githubTimeLayout),
		until.Format(githubTimeLayout),
		manager.IsIncremental(),
		sinceSource)

	leafWindows, err := newLeafWindowBuilder(taskCtx, data).build(windowStart, until)
	if err != nil {
		return err
	}
	logger.Info("cicd_run_collector: built %d leaf windows for collection", len(leafWindows))

	if err := registerCollectorForLeafWindows(manager, data.ApiClient, leafWindows); err != nil {
		return err
	}

	return manager.Execute()
}

func loadLatestRunUpdatedAt(taskCtx plugin.SubTaskContext, connectionId uint64, repoId int) (*time.Time, errors.Error) {
	db := taskCtx.GetDal()
	latest := &models.GithubRun{}
	err := db.First(
		latest,
		dal.Where("connection_id = ? AND repo_id = ? AND github_updated_at IS NOT NULL", connectionId, repoId),
		dal.Orderby("github_updated_at DESC"),
		dal.Limit(1),
	)
	if err != nil {
		if db.IsErrorNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if latest.GithubUpdatedAt == nil {
		return nil, nil
	}
	fallback := latest.GithubUpdatedAt.UTC()
	return &fallback, nil
}

// buildRunsQuery assembles the filtered-mode query for a single leaf TimeWindow.
// Shared between registerCollectorForLeafWindows and tests.
func buildRunsQuery(reqData *helper.RequestData) (url.Values, errors.Error) {
	w, ok := reqData.Input.(*TimeWindow)
	if !ok || w == nil {
		return nil, errors.Default.New("cicd_run_collector: Input is not *TimeWindow")
	}
	q := url.Values{}
	q.Set("created", fmt.Sprintf("%s..%s",
		w.From.UTC().Format(githubTimeLayout),
		w.To.UTC().Format(githubTimeLayout)))
	q.Set("page", fmt.Sprintf("%d", reqData.Pager.Page))
	q.Set("per_page", fmt.Sprintf("%d", reqData.Pager.Size))
	return q, nil
}

// registerCollectorForLeafWindows wires a single ApiCollector whose Input iterator feeds the
// leaf TimeWindows.
func registerCollectorForLeafWindows(
	manager *helper.StatefulApiCollector,
	apiClient helper.RateLimitedApiClient,
	leafWindows []TimeWindow,
) errors.Error {
	iterator := helper.NewQueueIterator()
	for i := range leafWindows {
		w := leafWindows[i]
		iterator.Push(&w)
	}
	return manager.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   apiClient,
		Input:       iterator,
		UrlTemplate: "repos/{{ .Params.Name }}/actions/runs",
		Query:       buildRunsQuery,
		PageSize:    PAGE_SIZE,
		Concurrency: 10,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			body := &GithubRawRunsResult{}
			if err := helper.UnmarshalResponse(res, body); err != nil {
				return nil, err
			}
			if len(body.WorkflowRuns) == 0 {
				return nil, nil
			}
			// Range is already bounded in filtered mode; only keep completed runs.
			filtered := make([]json.RawMessage, 0, len(body.WorkflowRuns))
			for _, run := range body.WorkflowRuns {
				if run.Status != "completed" {
					continue
				}
				runJSON, err := json.Marshal(run)
				if err != nil {
					return nil, errors.Convert(err)
				}
				filtered = append(filtered, json.RawMessage(runJSON))
			}
			return filtered, nil
		},
	})
}

// probeFunc signature matches defaultProbeTotalCount so tests can inject a fake.
type probeFunc func(taskCtx plugin.SubTaskContext, data *GithubTaskData, from, to time.Time) (int, bool, errors.Error)

// leafWindowBuilder recursively bisects a [from, to] range until every leaf window fits under
// FILTERED_SEARCH_CAP. The probe function is a field so tests can inject a fake without
// mutating package-level state.
type leafWindowBuilder struct {
	taskCtx plugin.SubTaskContext
	data    *GithubTaskData
	probe   probeFunc
	logger  log.Logger
}

func newLeafWindowBuilder(taskCtx plugin.SubTaskContext, data *GithubTaskData) *leafWindowBuilder {
	return &leafWindowBuilder{
		taskCtx: taskCtx,
		data:    data,
		probe:   defaultProbeTotalCount,
		logger:  taskCtx.GetLogger(),
	}
}

// build recursively bisects [from, to] until every leaf has total_count < FILTERED_SEARCH_CAP
// (or the window is a single-second bucket that cannot be split further). Empty windows are
// dropped.
//
// Boundary policy (non-overlapping, full coverage at second precision):
//   - left:  created=<from>..<mid>
//   - right: created=<mid+1s>..<to>
//
// Bisection is done on integer Unix seconds because GitHub's `created` filter is
// second-precision; a single-second bucket (from.Unix() == to.Unix()) is the smallest
// indivisible unit.
func (b *leafWindowBuilder) build(from, to time.Time) ([]TimeWindow, errors.Error) {
	if !from.Before(to) && !from.Equal(to) {
		return nil, nil
	}

	total, is422, err := b.probe(b.taskCtx, b.data, from, to)
	if err != nil {
		return nil, err
	}

	if total == 0 && !is422 {
		return nil, nil
	}

	if total >= FILTERED_SEARCH_CAP || is422 {
		fromSec := from.UTC().Unix()
		toSec := to.UTC().Unix()
		if fromSec == toSec {
			return nil, errors.Default.New(fmt.Sprintf(
				"cicd_run_collector: %d runs within a single 1-second bucket at %s; cannot bisect further. "+
					"Filtered GitHub search caps at %d items per window, so some runs would be missed. "+
					"Refusing to advance collector state.",
				total, from.UTC().Format(time.RFC3339), FILTERED_SEARCH_CAP,
			))
		}
		if b.logger != nil {
			b.logger.Debug("cicd_run_collector: bisecting [%s, %s] (total=%d, is422=%v)",
				from.Format(githubTimeLayout),
				to.Format(githubTimeLayout),
				total, is422)
		}
		midSec := (fromSec + toSec) / 2
		leftTo := time.Unix(midSec, 0).UTC()
		rightFrom := leftTo.Add(time.Second)
		left, err := b.build(from, leftTo)
		if err != nil {
			return nil, err
		}
		right, err := b.build(rightFrom, to)
		if err != nil {
			return nil, err
		}
		return append(left, right...), nil
	}

	return []TimeWindow{{From: from, To: to}}, nil
}

// defaultProbeTotalCount issues a filtered-mode GET with per_page=1 to learn total_count
// (or detect 422) cheaply. It runs under SubmitBlocking so it shares the rate-limit budget
// with the main collector; DoGetAsync is avoided because that path errors on >=400 before
// the callback, which would hide the 422 we use as a bisection signal.
func defaultProbeTotalCount(
	taskCtx plugin.SubTaskContext,
	data *GithubTaskData,
	from time.Time,
	to time.Time,
) (int, bool, errors.Error) {
	q := url.Values{}
	q.Set("per_page", "1")
	q.Set("page", "1")
	q.Set("created", fmt.Sprintf("%s..%s",
		from.UTC().Format(githubTimeLayout),
		to.UTC().Format(githubTimeLayout)))
	path := fmt.Sprintf("repos/%s/actions/runs", data.Options.Name)

	var total int
	var is422 bool
	var innerErr errors.Error
	data.ApiClient.SubmitBlocking(func() errors.Error {
		res, getErr := data.ApiClient.Get(path, q, nil)
		// If a sibling subtask installed an AfterResponse hook that maps 422 ->
		// helper.ErrIgnoreAndContinue, ApiClient.Do already closed res.Body before returning
		// the sentinel (api_client.go L389). Recover the 422 signal here without double-closing.
		// Sentinel comparison uses `==` to stay consistent with every other ErrIgnoreAndContinue
		// call site in devlake (api_client.go:389, api_async_client.go:165, etc.).
		if getErr == helper.ErrIgnoreAndContinue && res != nil && res.StatusCode == http.StatusUnprocessableEntity {
			is422 = true
			return nil
		}
		if getErr != nil {
			innerErr = getErr
			return nil
		}
		if res.StatusCode == http.StatusUnprocessableEntity {
			if res.Body != nil {
				_ = res.Body.Close()
			}
			is422 = true
			return nil
		}
		body := &GithubRawRunsResult{}
		if e := helper.UnmarshalResponse(res, body); e != nil {
			innerErr = e
			return nil
		}
		total = body.TotalCount
		return nil
	})
	if err := data.ApiClient.WaitAsync(); err != nil {
		return 0, false, err
	}
	if innerErr != nil {
		return 0, false, innerErr
	}
	return total, is422, nil
}

type GithubRawRunsResult struct {
	TotalCount   int `json:"total_count"`
	WorkflowRuns []struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		NodeID           string `json:"node_id"`
		HeadBranch       string `json:"head_branch"`
		HeadSha          string `json:"head_sha"`
		Path             string `json:"path"`
		DisplayTitle     string `json:"display_title"`
		RunNumber        int    `json:"run_number"`
		Event            string `json:"event"`
		Status           string `json:"status"`
		Conclusion       string `json:"conclusion"`
		WorkflowID       int    `json:"workflow_id"`
		CheckSuiteID     int64  `json:"check_suite_id"`
		CheckSuiteNodeID string `json:"check_suite_node_id"`
		URL              string `json:"url"`
		HTMLURL          string `json:"html_url"`
		PullRequests     []struct {
			URL    string `json:"url"`
			ID     int    `json:"id"`
			Number int    `json:"number"`
			Head   struct {
				Ref  string `json:"ref"`
				Sha  string `json:"sha"`
				Repo struct {
					ID   int    `json:"id"`
					URL  string `json:"url"`
					Name string `json:"name"`
				} `json:"repo"`
			} `json:"head"`
			Base struct {
				Ref  string `json:"ref"`
				Sha  string `json:"sha"`
				Repo struct {
					ID   int    `json:"id"`
					URL  string `json:"url"`
					Name string `json:"name"`
				} `json:"repo"`
			} `json:"base"`
		} `json:"pull_requests"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Actor     struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			NodeID            string `json:"node_id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"actor"`
		RunAttempt          int       `json:"run_attempt"`
		ReferencedWorkflows []any     `json:"referenced_workflows"`
		RunStartedAt        time.Time `json:"run_started_at"`
		TriggeringActor     struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			NodeID            string `json:"node_id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"triggering_actor"`
		JobsURL            string `json:"jobs_url"`
		LogsURL            string `json:"logs_url"`
		CheckSuiteURL      string `json:"check_suite_url"`
		ArtifactsURL       string `json:"artifacts_url"`
		CancelURL          string `json:"cancel_url"`
		RerunURL           string `json:"rerun_url"`
		PreviousAttemptURL any    `json:"previous_attempt_url"`
		WorkflowURL        string `json:"workflow_url"`
		HeadCommit         struct {
			ID        string    `json:"id"`
			TreeID    string    `json:"tree_id"`
			Message   string    `json:"message"`
			Timestamp time.Time `json:"timestamp"`
			Author    struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
			Committer struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"committer"`
		} `json:"head_commit"`
		Repository struct {
			ID       int    `json:"id"`
			NodeID   string `json:"node_id"`
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Private  bool   `json:"private"`
			Owner    struct {
				Login             string `json:"login"`
				ID                int    `json:"id"`
				NodeID            string `json:"node_id"`
				AvatarURL         string `json:"avatar_url"`
				GravatarID        string `json:"gravatar_id"`
				URL               string `json:"url"`
				HTMLURL           string `json:"html_url"`
				FollowersURL      string `json:"followers_url"`
				FollowingURL      string `json:"following_url"`
				GistsURL          string `json:"gists_url"`
				StarredURL        string `json:"starred_url"`
				SubscriptionsURL  string `json:"subscriptions_url"`
				OrganizationsURL  string `json:"organizations_url"`
				ReposURL          string `json:"repos_url"`
				EventsURL         string `json:"events_url"`
				ReceivedEventsURL string `json:"received_events_url"`
				Type              string `json:"type"`
				SiteAdmin         bool   `json:"site_admin"`
			} `json:"owner"`
			HTMLURL          string `json:"html_url"`
			Description      string `json:"description"`
			Fork             bool   `json:"fork"`
			URL              string `json:"url"`
			ForksURL         string `json:"forks_url"`
			KeysURL          string `json:"keys_url"`
			CollaboratorsURL string `json:"collaborators_url"`
			TeamsURL         string `json:"teams_url"`
			HooksURL         string `json:"hooks_url"`
			IssueEventsURL   string `json:"issue_events_url"`
			EventsURL        string `json:"events_url"`
			AssigneesURL     string `json:"assignees_url"`
			BranchesURL      string `json:"branches_url"`
			TagsURL          string `json:"tags_url"`
			BlobsURL         string `json:"blobs_url"`
			GitTagsURL       string `json:"git_tags_url"`
			GitRefsURL       string `json:"git_refs_url"`
			TreesURL         string `json:"trees_url"`
			StatusesURL      string `json:"statuses_url"`
			LanguagesURL     string `json:"languages_url"`
			StargazersURL    string `json:"stargazers_url"`
			ContributorsURL  string `json:"contributors_url"`
			SubscribersURL   string `json:"subscribers_url"`
			SubscriptionURL  string `json:"subscription_url"`
			CommitsURL       string `json:"commits_url"`
			GitCommitsURL    string `json:"git_commits_url"`
			CommentsURL      string `json:"comments_url"`
			IssueCommentURL  string `json:"issue_comment_url"`
			ContentsURL      string `json:"contents_url"`
			CompareURL       string `json:"compare_url"`
			MergesURL        string `json:"merges_url"`
			ArchiveURL       string `json:"archive_url"`
			DownloadsURL     string `json:"downloads_url"`
			IssuesURL        string `json:"issues_url"`
			PullsURL         string `json:"pulls_url"`
			MilestonesURL    string `json:"milestones_url"`
			NotificationsURL string `json:"notifications_url"`
			LabelsURL        string `json:"labels_url"`
			ReleasesURL      string `json:"releases_url"`
			DeploymentsURL   string `json:"deployments_url"`
		} `json:"repository"`
		HeadRepository struct {
			ID       int    `json:"id"`
			NodeID   string `json:"node_id"`
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Private  bool   `json:"private"`
			Owner    struct {
				Login             string `json:"login"`
				ID                int    `json:"id"`
				NodeID            string `json:"node_id"`
				AvatarURL         string `json:"avatar_url"`
				GravatarID        string `json:"gravatar_id"`
				URL               string `json:"url"`
				HTMLURL           string `json:"html_url"`
				FollowersURL      string `json:"followers_url"`
				FollowingURL      string `json:"following_url"`
				GistsURL          string `json:"gists_url"`
				StarredURL        string `json:"starred_url"`
				SubscriptionsURL  string `json:"subscriptions_url"`
				OrganizationsURL  string `json:"organizations_url"`
				ReposURL          string `json:"repos_url"`
				EventsURL         string `json:"events_url"`
				ReceivedEventsURL string `json:"received_events_url"`
				Type              string `json:"type"`
				SiteAdmin         bool   `json:"site_admin"`
			} `json:"owner"`
			HTMLURL          string `json:"html_url"`
			Description      string `json:"description"`
			Fork             bool   `json:"fork"`
			URL              string `json:"url"`
			ForksURL         string `json:"forks_url"`
			KeysURL          string `json:"keys_url"`
			CollaboratorsURL string `json:"collaborators_url"`
			TeamsURL         string `json:"teams_url"`
			HooksURL         string `json:"hooks_url"`
			IssueEventsURL   string `json:"issue_events_url"`
			EventsURL        string `json:"events_url"`
			AssigneesURL     string `json:"assignees_url"`
			BranchesURL      string `json:"branches_url"`
			TagsURL          string `json:"tags_url"`
			BlobsURL         string `json:"blobs_url"`
			GitTagsURL       string `json:"git_tags_url"`
			GitRefsURL       string `json:"git_refs_url"`
			TreesURL         string `json:"trees_url"`
			StatusesURL      string `json:"statuses_url"`
			LanguagesURL     string `json:"languages_url"`
			StargazersURL    string `json:"stargazers_url"`
			ContributorsURL  string `json:"contributors_url"`
			SubscribersURL   string `json:"subscribers_url"`
			SubscriptionURL  string `json:"subscription_url"`
			CommitsURL       string `json:"commits_url"`
			GitCommitsURL    string `json:"git_commits_url"`
			CommentsURL      string `json:"comments_url"`
			IssueCommentURL  string `json:"issue_comment_url"`
			ContentsURL      string `json:"contents_url"`
			CompareURL       string `json:"compare_url"`
			MergesURL        string `json:"merges_url"`
			ArchiveURL       string `json:"archive_url"`
			DownloadsURL     string `json:"downloads_url"`
			IssuesURL        string `json:"issues_url"`
			PullsURL         string `json:"pulls_url"`
			MilestonesURL    string `json:"milestones_url"`
			NotificationsURL string `json:"notifications_url"`
			LabelsURL        string `json:"labels_url"`
			ReleasesURL      string `json:"releases_url"`
			DeploymentsURL   string `json:"deployments_url"`
		} `json:"head_repository"`
	} `json:"workflow_runs"`
}
