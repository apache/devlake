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

package parse

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	jiraKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
	rfeFilePattern = regexp.MustCompile(`(RHAIRFE-\d+)\.result\.md$`)
	totalPattern   = regexp.MustCompile(`(?m)\|\s*\*{0,2}Total\*{0,2}\s*\|\s*\*{0,2}(\d+)\s*/\s*(\d+)\*{0,2}\s*\|\s*\*{0,2}([A-Za-z]+)\*{0,2}`)
	titlePattern   = regexp.MustCompile(`(?m)^TITLE:\s*(.+?)\s*$`)
	rowPattern     = regexp.MustCompile(`(?m)\|\s*([^|\n]+?)\s*\|\s*(\d+)\s*/\s*(\d+)\s*\|`)
)

type pipelineFile struct {
	GeneratedAt string          `json:"generated_at"`
	Timestamp   string          `json:"timestamp"`
	RunID       string          `json:"run_id"`
	DryRun      bool            `json:"dry_run"`
	Strategies  []strategyEntry `json:"strategies"`
}

type strategyEntry struct {
	StratID        string      `json:"strat_id"`
	Title          string      `json:"title"`
	SourceRFE      string      `json:"source_rfe"`
	Status         string      `json:"status"`
	Recommendation string      `json:"recommendation"`
	Scores         *scoreBlock `json:"scores"`
}

type scoreBlock struct {
	Total        *float64 `json:"total"`
	Feasibility  *float64 `json:"feasibility"`
	Testability  *float64 `json:"testability"`
	Scope        *float64 `json:"scope"`
	Architecture *float64 `json:"architecture"`
}

type built struct {
	observations []Observation
	workItems    []WorkItem
	execution    *Execution
}

// Parse converts collected files into observations. Re-parsing the same blobs
// yields the same observation ids. Each observation keeps file.CollectedAt.
func Parse(cfg Config, files []File) DraftSet {
	set := DraftSet{}
	items := map[string]WorkItem{}
	execs := map[string]Execution{}
	for _, file := range files {
		if !Matches(cfg.Parser, file.Path) {
			continue
		}
		var result built
		var err error
		switch cfg.Parser {
		case ParserRFE:
			result, err = parseRFE(cfg, file)
		case ParserStrat:
			if isPipelineData(file.Path) {
				result, err = parsePipeline(cfg, file)
			} else {
				result, err = parseStratTask(cfg, file)
			}
		default:
			err = fmt.Errorf("unknown parser %q", cfg.Parser)
		}
		if err != nil {
			set.FileErrors = append(set.FileErrors, file.Path+": "+err.Error())
			continue
		}
		set.Observations = append(set.Observations, result.observations...)
		for _, item := range result.workItems {
			items[item.ID] = item
		}
		if result.execution != nil {
			execs[result.execution.ID] = *result.execution
		}
	}
	for _, item := range items {
		set.WorkItems = append(set.WorkItems, item)
	}
	for _, exec := range execs {
		set.Executions = append(set.Executions, exec)
	}
	return set
}

func parseRFE(cfg Config, file File) (built, error) {
	match := rfeFilePattern.FindStringSubmatch(file.Path)
	if match == nil {
		return built{}, fmt.Errorf("result file name has no RFE key")
	}
	key := match[1]
	subject := jiraWorkItem(key, "rfe")
	selector := "result"
	obsID := ObservationID(cfg.SourceID, file.BlobID, file.Path, selector, KindAssessment)
	total, scale, verdict := parseTotal(file.Body)
	ext := map[string]any{"title": parseTitle(file.Body), "criteria": parseCriteria(file.Body)}
	rawExt, err := json.Marshal(ext)
	if err != nil {
		return built{}, err
	}
	obs := baseObservation(cfg, file, obsID, KindAssessment, subject, "", "git-rfe-v0.1", "file_assessment", selector)
	obs.Extensions = string(rawExt)
	if total != nil {
		obs.AssessmentTotal = total
		obs.AssessmentScale = scale
		obs.RawVerdict = verdict
	}
	obs.Evidence = []Evidence{evidence(cfg, file, obsID, selector)}
	obs.Unknowns = standardUnknowns(obsID, false)
	if total == nil {
		obs.Unknowns = append(obs.Unknowns, unknown(obsID, "assessments[0].total", "the result file has no total score", "a Total row in the result table"))
	}
	if folder := batchFolderName(file.Path); folder != "" {
		entry := ClassifyTime(folder)
		entry.Type = "batch_folder"
		entry.AppliesTo = "batch"
		obs.Times = []TimeEntry{entry}
	}
	return built{observations: []Observation{obs}, workItems: []WorkItem{subject}}, nil
}

func parsePipeline(cfg Config, file File) (built, error) {
	var doc pipelineFile
	if err := json.Unmarshal([]byte(file.Body), &doc); err != nil {
		return built{}, fmt.Errorf("pipeline-data.json: %w", err)
	}
	result := built{}
	var executionID string
	if doc.Timestamp != "" || doc.GeneratedAt != "" {
		folder := batchFolderName(file.Path)
		runID := folder
		if runID == "" {
			runID = doc.Timestamp
		}
		exec := Execution{
			ID:               ID(cfg.SourceID, "execution", BatchDir(file.Path)),
			SourceID:         cfg.SourceID,
			StationID:        cfg.StationID,
			SourceRunID:      runID,
			RunIDType:        "batch_folder",
			PipelineIDStatus: "unknown",
			BatchPath:        BatchDir(file.Path),
		}
		result.execution = &exec
		executionID = exec.ID
	}
	seen := map[string]WorkItem{}
	for _, strategy := range doc.Strategies {
		if strategy.StratID == "" {
			result.observations = append(result.observations, Observation{})
			continue
		}
		subject := jiraWorkItem(strategy.StratID, "strategy")
		seen[subject.ID] = subject
		selector := "/strategies/" + strategy.StratID
		obsID := ObservationID(cfg.SourceID, file.BlobID, file.Path, selector, KindProcessing)
		rawResult := strategy.Recommendation
		if rawResult == "" {
			rawResult = strategy.Status
		}
		ext := map[string]any{
			"title":           strategy.Title,
			"status":          strategy.Status,
			"recommendation":  strategy.Recommendation,
			"reported_run_id": doc.RunID,
			"dry_run":         doc.DryRun,
		}
		if strategy.Scores != nil {
			ext["criteria"] = map[string]any{
				"feasibility":  strategy.Scores.Feasibility,
				"testability":  strategy.Scores.Testability,
				"scope":        strategy.Scores.Scope,
				"architecture": strategy.Scores.Architecture,
			}
		}
		rawExt, err := json.Marshal(ext)
		if err != nil {
			return built{}, err
		}
		obs := baseObservation(cfg, file, obsID, KindProcessing, subject, executionID, "git-strat-v0.1", "batch_membership_known", selector)
		obs.RawResult = rawResult
		obs.Extensions = string(rawExt)
		obs.Evidence = []Evidence{evidence(cfg, file, obsID, selector)}
		obs.Unknowns = standardUnknowns(obsID, executionID != "")
		if strategy.Recommendation == "" {
			obs.Unknowns = append(obs.Unknowns, unknown(obsID, "processing_result.recommendation", "the strategy entry has no recommendation", "a recommendation field on the strategy"))
		}
		if doc.RunID == "" || doc.RunID == GenericRunID {
			obs.Unknowns = append(obs.Unknowns, unknown(obsID, "run_ref.reported_run_id", "source run_id does not identify this batch", "a producer pipeline id"))
		}
		if strategy.Scores != nil && strategy.Scores.Total != nil {
			total := *strategy.Scores.Total
			obs.AssessmentTotal = &total
		}
		if strategy.SourceRFE != "" {
			linked := jiraWorkItem(strategy.SourceRFE, "rfe")
			seen[linked.ID] = linked
			obs.Links = []Link{{
				ID:             ID(obsID, "link", linked.ID),
				ObservationID:  obsID,
				Role:           "input",
				Relation:       "source_work_item",
				WorkItemID:     linked.ID,
				EvidenceStatus: "recorded",
			}}
		}
		stamp := doc.Timestamp
		if stamp == "" {
			stamp = doc.GeneratedAt
		}
		if stamp != "" {
			entry := ClassifyTime(stamp)
			entry.Type = "source_generated"
			entry.AppliesTo = "batch"
			obs.Times = append(obs.Times, entry)
		}
		result.observations = append(result.observations, obs)
	}
	// Drop the placeholder used when strat_id is empty.
	kept := result.observations[:0]
	for _, obs := range result.observations {
		if obs.ID != "" {
			kept = append(kept, obs)
		}
	}
	result.observations = kept
	for _, item := range seen {
		result.workItems = append(result.workItems, item)
	}
	return result, nil
}

func parseStratTask(cfg Config, file File) (built, error) {
	fields := frontMatter(file.Body)
	if len(fields) == 0 {
		return built{}, fmt.Errorf("strat task has no front matter")
	}
	stratID := fields["strat_id"]
	if stratID == "" {
		return built{}, fmt.Errorf("strat task has no strat_id")
	}
	subject := workItemFromTask(stratID, fields["jira_key"])
	selector := "frontmatter"
	obsID := ObservationID(cfg.SourceID, file.BlobID, file.Path, selector, KindProcessing)
	ext, err := json.Marshal(map[string]any{
		"title":    fields["title"],
		"status":   fields["status"],
		"strat_id": stratID,
		"jira_key": fields["jira_key"],
	})
	if err != nil {
		return built{}, err
	}
	obs := baseObservation(cfg, file, obsID, KindProcessing, subject, "", "git-strat-v0.1", "front_matter_only", selector)
	obs.RawResult = fields["status"]
	obs.Extensions = string(ext)
	obs.Evidence = []Evidence{evidence(cfg, file, obsID, selector)}
	obs.Unknowns = standardUnknowns(obsID, false)
	if fields["jira_key"] == "" {
		obs.Unknowns = append(obs.Unknowns, unknown(obsID, "subject_ref", "front matter jira_key is null", "a Jira key on the strategy task"))
	}
	items := []WorkItem{subject}
	if fields["source_rfe"] != "" {
		linked := jiraWorkItem(fields["source_rfe"], "rfe")
		items = append(items, linked)
		obs.Links = []Link{{
			ID:             ID(obsID, "link", linked.ID),
			ObservationID:  obsID,
			Role:           "input",
			Relation:       "source_work_item",
			WorkItemID:     linked.ID,
			EvidenceStatus: "recorded",
		}}
	}
	return built{observations: []Observation{obs}, workItems: items}, nil
}

func baseObservation(cfg Config, file File, id, kind string, subject WorkItem, executionID, adapter, completeness, selector string) Observation {
	return Observation{
		ID:            id,
		Kind:          kind,
		WorkItemID:    subject.ID,
		SubjectKey:    subject.ItemKey,
		ExecutionID:   executionID,
		Path:          file.Path,
		BlobID:        file.BlobID,
		CommitSHA:     file.CommitSHA,
		FieldSelector: selector,
		AdapterID:     adapter,
		Completeness:  completeness,
		CollectedAt:   file.CollectedAt,
		AttemptStatus: AttemptUnknown,
	}
}

func jiraWorkItem(key, kind string) WorkItem {
	return WorkItem{
		ID:             ID(JiraSystem, JiraInstance, kind, key),
		SourceSystem:   JiraSystem,
		SourceInstance: JiraInstance,
		EntityKind:     kind,
		ItemKey:        key,
		IdStatus:       IdStatusKeyOnly,
		MatchStatus:    MatchUnresolved,
	}
}

func workItemFromTask(stratID, jiraKey string) WorkItem {
	if jiraKeyPattern.MatchString(jiraKey) {
		return jiraWorkItem(jiraKey, "strategy")
	}
	return WorkItem{
		ID:           ID("station", "strat-task", stratID),
		SourceSystem: "station",
		EntityKind:   "strategy",
		ItemKey:      stratID,
		IdStatus:     IdStatusUnconfirmed,
		MatchStatus:  MatchUnresolved,
	}
}

func evidence(cfg Config, file File, obsID, selector string) Evidence {
	return Evidence{
		ID:            ID(obsID, "evidence"),
		ObservationID: obsID,
		SourceID:      cfg.SourceID,
		URI:           blobURI(cfg, file),
		Revision:      file.BlobID,
		FieldSelector: selector,
	}
}

func blobURI(cfg Config, file File) string {
	base := strings.TrimRight(cfg.WebBase, "/")
	if base == "" {
		base = "https://gitlab.com"
	}
	ref := file.CommitSHA
	if ref == "" {
		ref = file.BlobID
	}
	return base + "/" + cfg.ProjectPath + "/-/blob/" + url.PathEscape(ref) + "/" + pathEscape(file.Path)
}

func pathEscape(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return path.Join(parts...)
}

func standardUnknowns(obsID string, hasExecution bool) []Unknown {
	unknowns := []Unknown{
		unknown(obsID, "attempt_ref", "no per-item execution id was found", "a producer item-level execution id"),
		unknown(obsID, "run_ref.pipeline_id", "GitLab pipeline id is not in the artifact", "a GitLab CI pipeline or job id"),
		unknown(obsID, "inputs[0].consumed_revision", "the exact input revision consumed by this run is not recorded", "an input manifest or snapshot hash"),
	}
	if !hasExecution {
		unknowns = append(unknowns, unknown(obsID, "run_ref", "the artifact does not identify a producer run", "a run id recorded by the producer"))
	}
	return unknowns
}

func unknown(obsID, field, reason, needed string) Unknown {
	return Unknown{
		ID:             ID(obsID, "unknown", field),
		ObservationID:  obsID,
		Field:          field,
		Reason:         reason,
		EvidenceNeeded: needed,
	}
}

func parseTitle(body string) string {
	match := titlePattern.FindStringSubmatch(body)
	if match == nil {
		return ""
	}
	return match[1]
}

func parseTotal(body string) (total *float64, scale *float64, verdict string) {
	match := totalPattern.FindStringSubmatch(body)
	if match == nil {
		return nil, nil, ""
	}
	totalVal, err1 := strconv.ParseFloat(match[1], 64)
	scaleVal, err2 := strconv.ParseFloat(match[2], 64)
	if err1 != nil || err2 != nil {
		return nil, nil, ""
	}
	return &totalVal, &scaleVal, match[3]
}

func parseCriteria(body string) []map[string]any {
	rows := rowPattern.FindAllStringSubmatch(body, -1)
	criteria := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		name := strings.Trim(row[1], "* ")
		if strings.EqualFold(name, "total") || strings.EqualFold(name, "criterion") {
			continue
		}
		score, err1 := strconv.Atoi(row[2])
		scale, err2 := strconv.Atoi(row[3])
		if err1 != nil || err2 != nil {
			continue
		}
		criteria = append(criteria, map[string]any{"name": name, "score": score, "scale": scale})
	}
	return criteria
}

func frontMatter(body string) map[string]string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	fields := map[string]string{}
	var key string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if key != "" {
				fields[key] = strings.TrimSpace(fields[key] + " " + strings.TrimSpace(line))
			}
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(name)
		fields[key] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	for name, value := range fields {
		if value == "null" || value == "~" {
			fields[name] = ""
		}
	}
	return fields
}

func batchFolderName(filePath string) string {
	dir := BatchDir(filePath)
	name := path.Base(dir)
	if folderPattern.MatchString(name) {
		return name
	}
	return ""
}
