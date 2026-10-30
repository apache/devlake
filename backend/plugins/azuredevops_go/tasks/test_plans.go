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
	"net/url"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/qa"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
)

func init() {
	RegisterSubtaskMeta(&CollectApiTestPlansMeta)
	RegisterSubtaskMeta(&ExtractApiTestPlansMeta)
	RegisterSubtaskMeta(&CollectApiTestCasesMeta)
	RegisterSubtaskMeta(&ExtractApiTestCasesMeta)
	RegisterSubtaskMeta(&ConvertTestCasesMeta)
	RegisterSubtaskMeta(&CollectApiTestRunsMeta)
	RegisterSubtaskMeta(&ExtractApiTestRunsMeta)
	RegisterSubtaskMeta(&CollectApiTestResultsMeta)
	RegisterSubtaskMeta(&ExtractApiTestResultsMeta)
	RegisterSubtaskMeta(&ConvertTestResultsMeta)
}

const (
	RawTestPlanTable   = "azuredevops_go_api_test_plans"
	RawTestCaseTable   = "azuredevops_go_api_test_cases"
	RawTestRunTable    = "azuredevops_go_api_test_runs"
	RawTestResultTable = "azuredevops_go_api_test_results"
)

var CollectApiTestPlansMeta = plugin.SubTaskMeta{
	Name:             "collectApiTestPlans",
	EntryPoint:       CollectApiTestPlans,
	EnabledByDefault: true,
	Description:      "Collect Azure Test Plans.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	ProductTables:    []string{RawTestPlanTable},
}

var ExtractApiTestPlansMeta = plugin.SubTaskMeta{
	Name:             "extractApiTestPlans",
	EntryPoint:       ExtractApiTestPlans,
	EnabledByDefault: true,
	Description:      "Extract Azure Test Plans into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{RawTestPlanTable},
	ProductTables:    []string{models.AzuredevopsTestPlan{}.TableName()},
}

var CollectApiTestCasesMeta = plugin.SubTaskMeta{
	Name:             "collectApiTestCases",
	EntryPoint:       CollectApiTestCases,
	EnabledByDefault: true,
	Description:      "Collect test cases for each Azure Test Plan.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{models.AzuredevopsTestPlan{}.TableName()},
	ProductTables:    []string{RawTestCaseTable},
}

var ExtractApiTestCasesMeta = plugin.SubTaskMeta{
	Name:             "extractApiTestCases",
	EntryPoint:       ExtractApiTestCases,
	EnabledByDefault: true,
	Description:      "Extract Azure test cases into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{RawTestCaseTable},
	ProductTables:    []string{models.AzuredevopsTestCase{}.TableName()},
}

var ConvertTestCasesMeta = plugin.SubTaskMeta{
	Name:             "convertTestCases",
	EntryPoint:       ConvertTestCases,
	EnabledByDefault: true,
	Description:      "Convert Azure test cases into QA test cases.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{models.AzuredevopsTestCase{}.TableName()},
	ProductTables:    []string{(&qa.QaTestCase{}).TableName(), (&qa.QaProject{}).TableName()},
}

var CollectApiTestRunsMeta = plugin.SubTaskMeta{
	Name:             "collectApiTestRuns",
	EntryPoint:       CollectApiTestRuns,
	EnabledByDefault: true,
	Description:      "Collect Azure test runs.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	ProductTables:    []string{RawTestRunTable},
}

var ExtractApiTestRunsMeta = plugin.SubTaskMeta{
	Name:             "extractApiTestRuns",
	EntryPoint:       ExtractApiTestRuns,
	EnabledByDefault: true,
	Description:      "Extract Azure test runs into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{RawTestRunTable},
	ProductTables:    []string{models.AzuredevopsTestRun{}.TableName()},
}

var CollectApiTestResultsMeta = plugin.SubTaskMeta{
	Name:             "collectApiTestResults",
	EntryPoint:       CollectApiTestResults,
	EnabledByDefault: true,
	Description:      "Collect results for each Azure test run.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{models.AzuredevopsTestRun{}.TableName()},
	ProductTables:    []string{RawTestResultTable},
}

var ExtractApiTestResultsMeta = plugin.SubTaskMeta{
	Name:             "extractApiTestResults",
	EntryPoint:       ExtractApiTestResults,
	EnabledByDefault: true,
	Description:      "Extract Azure test results into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{RawTestResultTable},
	ProductTables:    []string{models.AzuredevopsTestResult{}.TableName()},
}

var ConvertTestResultsMeta = plugin.SubTaskMeta{
	Name:             "convertTestResults",
	EntryPoint:       ConvertTestResults,
	EnabledByDefault: true,
	Description:      "Convert Azure test results into QA test case executions.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_QUALITY},
	DependencyTables: []string{models.AzuredevopsTestResult{}.TableName(), models.AzuredevopsTestCase{}.TableName()},
	ProductTables:    []string{(&qa.QaTestCaseExecution{}).TableName()},
}

func CollectApiTestPlans(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	return collectContinuation(
		&api.RawDataSubTaskArgs{Ctx: taskCtx, Table: RawTestPlanTable, Options: data.Options},
		data.ApiClient,
		adoScopePrefix(data)+"/_apis/testplan/plans",
		apiVersionQuery("7.1"),
	)
}

func CollectApiTestRuns(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	return collectContinuation(
		&api.RawDataSubTaskArgs{Ctx: taskCtx, Table: RawTestRunTable, Options: data.Options},
		data.ApiClient,
		adoScopePrefix(data)+"/_apis/test/runs",
		apiVersionQuery("7.1"),
	)
}

func apiVersionQuery(version string) func(reqData *api.RequestData) (url.Values, errors.Error) {
	return func(reqData *api.RequestData) (url.Values, errors.Error) {
		query := url.Values{}
		query.Set("api-version", version)
		return query, nil
	}
}

type testCaseInput struct {
	PlanId  int
	SuiteId int
}

type testResultInput struct {
	RunId  int
	PlanId int
}

func CollectApiTestCases(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	var plans []models.AzuredevopsTestPlan
	err := taskCtx.GetDal().All(&plans, dal.Where("connection_id = ? AND project_id = ? AND root_suite_id <> 0", data.Options.ConnectionId, data.Options.ProjectId))
	if err != nil {
		return err
	}
	iterator := api.NewQueueIterator()
	for _, plan := range plans {
		iterator.Push(testCaseInput{PlanId: plan.AzuredevopsId, SuiteId: plan.RootSuiteId})
	}
	rawArgs := &api.RawDataSubTaskArgs{Ctx: taskCtx, Table: RawTestCaseTable, Options: data.Options}
	collector, err := api.NewStatefulApiCollector(*rawArgs)
	if err != nil {
		return err
	}
	err = collector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: *rawArgs,
		ApiClient:          data.ApiClient,
		Input:              iterator,
		UrlTemplate:        adoScopePrefix(data) + "/_apis/testplan/plans/{{ .Input.PlanId }}/Suites/{{ .Input.SuiteId }}/TestCase",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("isRecursive", "true")
			query.Set("api-version", "7.1")
			return query, nil
		},
		ResponseParser: ParseRawMessageFromValue,
		AfterResponse:  change203To401,
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

func CollectApiTestResults(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	var runs []models.AzuredevopsTestRun
	err := taskCtx.GetDal().All(&runs, dal.Where("connection_id = ? AND project_id = ?", data.Options.ConnectionId, data.Options.ProjectId))
	if err != nil {
		return err
	}
	iterator := api.NewQueueIterator()
	for _, run := range runs {
		iterator.Push(testResultInput{RunId: run.AzuredevopsId, PlanId: run.PlanId})
	}
	rawArgs := &api.RawDataSubTaskArgs{Ctx: taskCtx, Table: RawTestResultTable, Options: data.Options}
	collector, err := api.NewStatefulApiCollector(*rawArgs)
	if err != nil {
		return err
	}
	err = collector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: *rawArgs,
		ApiClient:          data.ApiClient,
		Input:              iterator,
		UrlTemplate:        adoScopePrefix(data) + "/_apis/test/runs/{{ .Input.RunId }}/results",
		Query:              apiVersionQuery("7.1"),
		ResponseParser:     ParseRawMessageFromValue,
		AfterResponse:      change203To401,
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

func ExtractApiTestPlans(taskCtx plugin.SubTaskContext) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawTestPlanTable)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				ID        int         `json:"id"`
				Name      string      `json:"name"`
				State     string      `json:"state"`
				URL       string      `json:"url"`
				StartDate interface{} `json:"startDate"`
				EndDate   interface{} `json:"endDate"`
				RootSuite struct {
					ID int `json:"id"`
				} `json:"rootSuite"`
			}
			if err := errors.Convert(json.Unmarshal(row.Data, &raw)); err != nil {
				return nil, err
			}
			return []interface{}{&models.AzuredevopsTestPlan{
				ConnectionId:  data.Options.ConnectionId,
				AzuredevopsId: raw.ID,
				OrgId:         data.Options.OrganizationId,
				ProjectId:     data.Options.ProjectId,
				Name:          raw.Name,
				State:         raw.State,
				RootSuiteId:   raw.RootSuite.ID,
				StartedDate:   parseAdoTime(raw.StartDate),
				EndedDate:     parseAdoTime(raw.EndDate),
				Url:           raw.URL,
			}}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func ExtractApiTestRuns(taskCtx plugin.SubTaskContext) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawTestRunTable)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				ID            int         `json:"id"`
				Name          string      `json:"name"`
				State         string      `json:"state"`
				URL           string      `json:"url"`
				StartedDate   interface{} `json:"startedDate"`
				CompletedDate interface{} `json:"completedDate"`
				TotalTests    int         `json:"totalTests"`
				PassedTests   int         `json:"passedTests"`
				Plan          struct {
					ID int `json:"id"`
				} `json:"plan"`
			}
			if err := errors.Convert(json.Unmarshal(row.Data, &raw)); err != nil {
				return nil, err
			}
			return []interface{}{&models.AzuredevopsTestRun{
				ConnectionId:  data.Options.ConnectionId,
				AzuredevopsId: raw.ID,
				OrgId:         data.Options.OrganizationId,
				ProjectId:     data.Options.ProjectId,
				PlanId:        raw.Plan.ID,
				Name:          raw.Name,
				State:         raw.State,
				StartedDate:   parseAdoTime(raw.StartedDate),
				CompletedDate: parseAdoTime(raw.CompletedDate),
				TotalTests:    raw.TotalTests,
				PassedTests:   raw.PassedTests,
				Url:           raw.URL,
			}}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func ExtractApiTestCases(taskCtx plugin.SubTaskContext) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawTestCaseTable)
	creators := loadTestCaseCreators(taskCtx, data)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				WorkItem struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
					URL  string `json:"url"`
				} `json:"workItem"`
			}
			if err := errors.Convert(json.Unmarshal(row.Data, &raw)); err != nil {
				return nil, err
			}
			var input testCaseInput
			_ = json.Unmarshal(row.Input, &input)
			workItemID := raw.WorkItem.ID
			if workItemID == 0 {
				workItemID = raw.ID
			}
			name := raw.WorkItem.Name
			if name == "" {
				name = raw.Name
			}
			creator := creators[workItemID]
			return []interface{}{&models.AzuredevopsTestCase{
				ConnectionId:  data.Options.ConnectionId,
				AzuredevopsId: workItemID,
				PlanId:        input.PlanId,
				OrgId:         data.Options.OrganizationId,
				ProjectId:     data.Options.ProjectId,
				Name:          name,
				WorkItemId:    workItemID,
				Url:           raw.WorkItem.URL,
				CreatedDate:   creator.Created,
				CreatorId:     creator.CreatorId,
			}}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

type testCaseCreator struct {
	Created   *time.Time
	CreatorId string
}

func loadTestCaseCreators(taskCtx plugin.SubTaskContext, data *AzuredevopsTaskData) map[int]testCaseCreator {
	creators := map[int]testCaseCreator{}
	rawSub, err := api.NewRawDataSubTask(api.RawDataSubTaskArgs{Ctx: taskCtx, Table: RawTestCaseTable, Options: data.Options})
	if err != nil {
		taskCtx.GetLogger().Warn(err, "skipping test case creator lookup")
		return creators
	}
	var rows []api.RawData
	if err = taskCtx.GetDal().All(&rows, dal.From(rawSub.GetTable()), dal.Where("params = ?", rawSub.GetParams())); err != nil {
		taskCtx.GetLogger().Warn(err, "skipping test case creator lookup")
		return creators
	}
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			ID       int `json:"id"`
			WorkItem struct {
				ID int `json:"id"`
			} `json:"workItem"`
		}
		if json.Unmarshal(row.Data, &raw) != nil {
			continue
		}
		id := raw.WorkItem.ID
		if id == 0 {
			id = raw.ID
		}
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, chunk := range chunkInts(ids, 200) {
		query := url.Values{}
		query.Set("ids", joinInts(chunk))
		query.Set("fields", "System.CreatedDate,System.CreatedBy")
		query.Set("api-version", "7.1")
		res, getErr := data.ApiClient.Get(adoOrgPrefix(data)+"_apis/wit/workitems", query, nil)
		if getErr != nil {
			taskCtx.GetLogger().Warn(getErr, "could not load test case work items")
			return creators
		}
		var body struct {
			Value []struct {
				ID     int                    `json:"id"`
				Fields map[string]interface{} `json:"fields"`
			} `json:"value"`
		}
		if getErr = readJSON(res, &body); getErr != nil {
			taskCtx.GetLogger().Warn(getErr, "could not load test case work items")
			return creators
		}
		for _, item := range body.Value {
			creatorID, _ := identityFromRef(item.Fields["System.CreatedBy"])
			creators[item.ID] = testCaseCreator{
				Created:   parseAdoTime(item.Fields["System.CreatedDate"]),
				CreatorId: creatorID,
			}
		}
	}
	return creators
}

type adoTestResult struct {
	ID                  int         `json:"id"`
	Outcome             string      `json:"outcome"`
	State               string      `json:"state"`
	URL                 string      `json:"url"`
	StartedDate         interface{} `json:"startedDate"`
	CompletedDate       interface{} `json:"completedDate"`
	DurationInMs        *float64    `json:"durationInMs"`
	TestCaseTitle       string      `json:"testCaseTitle"`
	TestCaseReferenceId int         `json:"testCaseReferenceId"`
	TestCase            struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"testCase"`
	TestPlan struct {
		ID string `json:"id"`
	} `json:"testPlan"`
	RunBy         map[string]interface{} `json:"runBy"`
	LastUpdatedBy map[string]interface{} `json:"lastUpdatedBy"`
	Owner         map[string]interface{} `json:"owner"`
}

func ExtractApiTestResults(taskCtx plugin.SubTaskContext) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawTestResultTable)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var raw adoTestResult
			if err := errors.Convert(json.Unmarshal(row.Data, &raw)); err != nil {
				return nil, err
			}
			var input testResultInput
			_ = json.Unmarshal(row.Input, &input)
			caseID := asInt(raw.TestCase.ID)
			if caseID == 0 {
				caseID = raw.TestCaseReferenceId
			}
			planID := asInt(raw.TestPlan.ID)
			if planID == 0 {
				planID = input.PlanId
			}
			title := raw.TestCaseTitle
			if title == "" {
				title = raw.TestCase.Name
			}
			executor, _ := identityFromRef(firstIdentity(raw.RunBy, raw.LastUpdatedBy, raw.Owner))
			durationMs := 0
			if raw.DurationInMs != nil {
				durationMs = int(*raw.DurationInMs)
			}
			return []interface{}{&models.AzuredevopsTestResult{
				ConnectionId:  data.Options.ConnectionId,
				AzuredevopsId: raw.ID,
				RunId:         input.RunId,
				OrgId:         data.Options.OrganizationId,
				ProjectId:     data.Options.ProjectId,
				PlanId:        planID,
				TestCaseId:    caseID,
				TestCaseTitle: title,
				Outcome:       raw.Outcome,
				State:         raw.State,
				StartedDate:   parseAdoTime(raw.StartedDate),
				CompletedDate: parseAdoTime(raw.CompletedDate),
				DurationMs:    durationMs,
				Url:           raw.URL,
				CreatorId:     executor,
			}}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func firstIdentity(values ...map[string]interface{}) interface{} {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func ConvertTestCases(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	caseIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsTestCase{})
	projectID := projectBoardID(data)
	qaProject := &qa.QaProject{
		DomainEntityExtended: domainlayer.DomainEntityExtended{Id: projectID},
		Name:                 data.Options.ProjectId,
	}
	return convertProjectRows(taskCtx, models.AzuredevopsTestCase{}.TableName(), &models.AzuredevopsTestCase{}, func(row interface{}) ([]interface{}, errors.Error) {
		testCase := row.(*models.AzuredevopsTestCase)
		var created time.Time
		if testCase.CreatedDate != nil {
			created = *testCase.CreatedDate
		}
		return []interface{}{
			qaProject,
			&qa.QaTestCase{
				DomainEntityExtended: domainlayer.DomainEntityExtended{
					Id: caseIdGen.Generate(data.Options.ConnectionId, testCase.AzuredevopsId, testCase.PlanId),
				},
				Name:        testCase.Name,
				Type:        "functional",
				CreateTime:  created,
				CreatorId:   testCase.CreatorId,
				QaProjectId: projectID,
			},
		}, nil
	})
}

func ConvertTestResults(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	caseIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsTestCase{})
	resultIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsTestResult{})
	projectID := projectBoardID(data)
	return convertProjectRows(taskCtx, models.AzuredevopsTestResult{}.TableName(), &models.AzuredevopsTestResult{}, func(row interface{}) ([]interface{}, errors.Error) {
		result := row.(*models.AzuredevopsTestResult)
		if result.TestCaseId == 0 || result.PlanId == 0 {
			return nil, nil
		}
		var started, finished time.Time
		if result.StartedDate != nil {
			started = *result.StartedDate
		}
		if result.CompletedDate != nil {
			finished = *result.CompletedDate
		}
		return []interface{}{&qa.QaTestCaseExecution{
			DomainEntityExtended: domainlayer.DomainEntityExtended{
				Id: resultIdGen.Generate(data.Options.ConnectionId, result.RunId, result.AzuredevopsId),
			},
			QaProjectId:  projectID,
			QaTestCaseId: caseIdGen.Generate(data.Options.ConnectionId, result.TestCaseId, result.PlanId),
			CreateTime:   started,
			StartTime:    started,
			FinishTime:   finished,
			CreatorId:    result.CreatorId,
			Status:       mapTestOutcome(result.Outcome),
		}}, nil
	})
}
