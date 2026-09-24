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
	"reflect"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/grafana_irm/models"
	"github.com/apache/devlake/plugins/grafana_irm/models/raw"
)

const RAW_INCIDENTS_TABLE = "grafana_irm_incidents"

const incidentsPageSize = 100

var _ plugin.SubTaskEntryPoint = CollectIncidents

// queryIncidentsResponse is the envelope IncidentsService.QueryIncidents
// returns; verified live, see grafana_irm_plan.md §3.1/§10.1.
type queryIncidentsResponse struct {
	Incidents []json.RawMessage `json:"incidents"`
	Cursor    struct {
		NextValue string `json:"nextValue"`
		HasMore   bool   `json:"hasMore"`
	} `json:"cursor"`
}

// getIncidentResponse is IncidentsService.GetIncident's envelope.
type getIncidentResponse struct {
	Incident json.RawMessage `json:"incident"`
}

// simplifiedIncident is the minimal shape read back from our own tool table
// to drive the "unfinished details" half's input iterator below. UpdatedDate
// is our last-known modifiedTime for this incident, used to skip
// re-inserting a raw row when GetIncident shows nothing has actually changed
// (see unfinishedDetailsHeader/incidentUnchanged below).
type simplifiedIncident struct {
	Id          string
	UpdatedDate time.Time
}

// knownModifiedHeader carries simplifiedIncident.UpdatedDate from
// unfinishedDetailsHeader (set on the outgoing request) through to that
// half's ResponseParser (read off the matching response) so the two can be
// compared per-request. This is smuggled through an HTTP header rather than
// shared state because ResponseParser only receives *http.Response, not the
// request that produced it or its originating input — but net/http
// guarantees res.Request is the exact request that was sent, headers
// included, which makes this correlation safe under this collector's
// concurrent workers (each request/response pair carries its own copy,
// nothing is shared across goroutines). Sent to Grafana's API too; harmless,
// since it silently ignores headers it doesn't recognize (verified live,
// same as it does for unrecognized JSON fields — see grafana_irm_plan.md
// §14.3).
const knownModifiedHeader = "X-Devlake-Known-Modified"

var CollectIncidentsMeta = plugin.SubTaskMeta{
	Name:             "collectIncidents",
	EntryPoint:       CollectIncidents,
	EnabledByDefault: true,
	Description:      "Collect Grafana IRM incidents: new/recently-changed by list, plus a refresh of still-open ones a date-range list can't catch",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{RAW_INCIDENTS_TABLE},
}

// CollectIncidents is a single FinalizableApiCollector subtask (grafana_irm_plan.md §15;
// replaces the earlier two-subtask CollectIncidents+RefreshOpenIncidents split, which caused
// three real bugs — §14.1/§14.2/§14.4 — all traced to having two independent ApiCollectors
// each independently deciding whether to wipe the raw table, with no coordination between
// them). This mirrors pagerduty's CollectUnfinishedDetails pattern, the framework's own
// blessed answer to this exact "list new/changed, then separately refresh still-open ones"
// shape:
//   - CollectNewRecordsByList: the main pass. `IncidentsQuery.DateFrom`/`DateTo` were verified
//     live to have no filtering effect at all, so incremental filtering goes through
//     `queryString`'s `declared:`/`resolved:` date-range syntax instead — verified live to
//     combine correctly with a bare `isdrill:false` term via `or(...)` (§10.1). Real incidents
//     only; drills are never synced (§11).
//   - CollectUnfinishedDetails: `queryString` only supports `declared:`/`started:`/`resolved:`/
//     `ended:` date ranges (verified live; `modified:`/`updated:` are not valid properties
//     there), so a status change, label edit, or role assignment on an incident that's already
//     synced and still open would never be picked up by the list pass alone. This re-fetches
//     (via GetIncident) every incident our own tool table still has as non-resolved from a
//     prior sync — but per NewStatefulApiCollectorForFinalizableEntity's own implementation,
//     it is never even constructed when the run isn't incremental (a full sync), because the
//     list pass's own unrestricted query already covers everything then. That's exactly what
//     §14.4 needed and didn't have under the old two-subtask design.
func CollectIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GrafanaIrmTaskData)
	db := taskCtx.GetDal()

	args := api.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Options: data.Options,
		Table:   RAW_INCIDENTS_TABLE,
	}

	// FinalizableApiCollectorCommonArgs.RequestBody doesn't receive
	// createdAfter the way Query/Header do (grafana_irm_plan.md §15.2 — an
	// asymmetry nothing else needed until now, not a deliberate constraint:
	// every other FinalizableApiCollector plugin filters via Query on a REST
	// GET, never RequestBody). Since createdAfter is a single constant for
	// the whole run, not a per-request value, we read it ourselves via a
	// second, independent, read-only CollectorStateManager against the same
	// (table, params) state row NewStatefulApiCollectorForFinalizableEntity
	// will also read below. Safe because NewCollectorStateManager only
	// reads — state is only ever written in .Close() — and this instance's
	// Close() is never called; only the framework's own manager (returned
	// below, closed via collector.Execute()) does that.
	rawDataSubTask, err := api.NewRawDataSubTask(args)
	if err != nil {
		return err
	}
	stateManager, err := api.NewCollectorStateManager(taskCtx, taskCtx.TaskContext().SyncPolicy(), rawDataSubTask.GetTable(), rawDataSubTask.GetParams())
	if err != nil {
		return err
	}
	queryString := buildIncidentsQueryString(stateManager.GetSince(), stateManager.GetUntil())

	var lastCursor string
	var lastHasMore bool

	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: args,
		ApiClient:          data.Client,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize: incidentsPageSize,
			GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
				// lastCursor/lastHasMore are set in ResponseParser below and read
				// from that closure rather than prevPageResponse.Body here: the
				// body is a single-read stream and is already drained by the time
				// this hook fires (same constraint incidentio's collector notes).
				if !lastHasMore || lastCursor == "" {
					return nil, api.ErrFinishCollect
				}
				return lastCursor, nil
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				Method:      http.MethodPost,
				UrlTemplate: "api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.QueryIncidents",
				RequestBody: func(reqData *api.RequestData) map[string]interface{} {
					query := map[string]interface{}{
						"limit":          reqData.Pager.Size,
						"orderDirection": "ASC",
						"queryString":    queryString,
					}
					body := map[string]interface{}{"query": query}
					if cursor, ok := reqData.CustomData.(string); ok && cursor != "" {
						body["cursor"] = map[string]interface{}{"nextValue": cursor}
					}
					return body
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					envelope := &queryIncidentsResponse{}
					if err := api.UnmarshalResponse(res, envelope); err != nil {
						return nil, err
					}
					lastCursor = envelope.Cursor.NextValue
					lastHasMore = envelope.Cursor.HasMore
					return envelope.Incidents, nil
				},
			},
		},
		CollectUnfinishedDetails: &api.FinalizableApiCollectorDetailArgs{
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				Method:      http.MethodPost,
				UrlTemplate: "api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.GetIncident",
				RequestBody: unfinishedDetailsRequestBody,
				Header:      unfinishedDetailsHeader,
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					envelope := &getIncidentResponse{}
					if err := api.UnmarshalResponse(res, envelope); err != nil {
						return nil, err
					}
					incident := &raw.Incident{}
					if err := errors.Convert(json.Unmarshal(envelope.Incident, incident)); err != nil {
						return nil, err
					}
					if incidentUnchanged(incident.ModifiedTime, res.Request.Header.Get(knownModifiedHeader)) {
						return []json.RawMessage{}, nil
					}
					return []json.RawMessage{envelope.Incident}, nil
				},
			},
			// Deliberately connection-wide, NOT filtered to this scope's label: the
			// tool tables are shared across a connection's scopes, and this pass is
			// what keeps every open incident's labels current. That matters because
			// scope membership is recomputed from those labels at convert time — if
			// an incident is relabelled out of this scope, neither scope's list pass
			// would re-fetch it (relabelling changes no declared/resolved date), so
			// this refresh is the only thing that notices.
			BuildInputIterator: func() (api.Iterator, errors.Error) {
				cursor, err := db.Cursor(
					dal.Select("id, updated_date"),
					dal.From(&models.Incident{}),
					dal.Where("connection_id = ? AND status != ?", data.Options.ConnectionId, "resolved"),
				)
				if err != nil {
					return nil, err
				}
				return api.NewDalCursorIterator(db, cursor, reflect.TypeOf(simplifiedIncident{}))
			},
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

// buildIncidentsQueryString implements the incremental-sync recipe verified
// live against the real API (grafana_irm_plan.md §10.1/§10.2). since/until
// come from the framework's own collector state tracking; a nil since means
// a full sync, so no date restriction is applied. A connection has exactly
// one scope covering its whole incident stream (§4.2), so there is no
// per-scope filter term to add here.
func buildIncidentsQueryString(since, until *time.Time) string {
	terms := []string{"isdrill:false"}
	if since != nil && until != nil {
		from := since.UTC().Format(time.RFC3339)
		to := until.UTC().Format(time.RFC3339)
		terms = append(terms, fmt.Sprintf("or(declared:%s,%s resolved:%s,%s)", from, to, from, to))
	}
	return strings.Join(terms, " ")
}

// unfinishedDetailsRequestBody reads the current input row off reqData.
// NewDalCursorIterator (used by CollectUnfinishedDetails' BuildInputIterator
// above) hands back *simplifiedIncident, not simplifiedIncident: it
// constructs each row via reflect.New, which always yields a pointer.
// Asserting the value type here panicked at runtime with "interface
// conversion: interface {} is *tasks.simplifiedIncident, not
// tasks.simplifiedIncident" the first time this path ran on a live pipeline
// with an unresolved incident already synced (§14.1) — neither unit nor e2e
// tests exercised this iterator before that.
func unfinishedDetailsRequestBody(reqData *api.RequestData) map[string]interface{} {
	input := reqData.Input.(*simplifiedIncident)
	return map[string]interface{}{"incidentID": input.Id}
}

// unfinishedDetailsHeader stamps this request with the incident's last-known
// modifiedTime, so ResponseParser can tell whether GetIncident's response
// actually changed anything (see knownModifiedHeader's doc comment).
// FinalizableApiCollectorCommonArgs.Header receives createdAfter (unlike
// RequestBody — see CollectIncidents' doc comment on that asymmetry); it's
// unused here since this comparison only cares about this one incident's own
// prior state, not the run's overall time window.
func unfinishedDetailsHeader(reqData *api.RequestData, _ *time.Time) (http.Header, errors.Error) {
	input := reqData.Input.(*simplifiedIncident)
	h := http.Header{}
	h.Set(knownModifiedHeader, input.UpdatedDate.UTC().Format(time.RFC3339Nano))
	return h, nil
}

// incidentUnchanged reports whether fetchedModifiedTime (the wire value from
// a fresh GetIncident call, RFC3339 with sub-second precision) represents the
// same instant as knownModified (this incident's last-known UpdatedDate,
// round-tripped through unfinishedDetailsHeader). Comparison truncates both
// sides to millisecond precision: UpdatedDate has already lost precision
// below that through MySQL's datetime(3) column (see models/incident.go), so
// comparing at full nanosecond precision would report "changed" every time
// even when nothing was.
//
// Fails safe: any parse failure (a missing/malformed header, or a malformed
// fetchedModifiedTime) returns false — "treat as changed, keep the row" —
// never "assume unchanged" on a header this function can't make sense of.
func incidentUnchanged(fetchedModifiedTime string, knownModified string) bool {
	if knownModified == "" {
		return false
	}
	fetched, err := parseIncidentTime(fetchedModifiedTime)
	if err != nil {
		return false
	}
	known, parseErr := time.Parse(time.RFC3339Nano, knownModified)
	if parseErr != nil {
		return false
	}
	return fetched.Truncate(time.Millisecond).Equal(known.Truncate(time.Millisecond))
}
