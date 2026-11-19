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

package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/webhook/models"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var openedAt = time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)

// mockCloseIssueDb sets up the webhook api package with a mocked database that
// holds webhook connection 1 and an open incident DLK-1234 created at openedAt.
// It returns the issue and incident as they are passed to Update.
func mockCloseIssueDb(t *testing.T) (*ticket.Issue, *ticket.Incident) {
	updatedIssue := &ticket.Issue{}
	updatedIncident := &ticket.Incident{}
	tx := new(mockdal.Transaction)
	tx.On("First", mock.AnythingOfType("*ticket.Issue"), mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*ticket.Issue) = ticket.Issue{
			DomainEntity:   domainlayer.DomainEntity{Id: "webhook:1:DLK-1234"},
			IssueKey:       "DLK-1234",
			Type:           ticket.INCIDENT,
			Status:         ticket.IN_PROGRESS,
			OriginalStatus: "firing",
			CreatedDate:    &openedAt,
		}
	}).Return(nil)
	tx.On("First", mock.AnythingOfType("*ticket.Incident"), mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*ticket.Incident) = ticket.Incident{
			DomainEntity:   domainlayer.DomainEntity{Id: "webhook:1:DLK-1234"},
			IncidentKey:    "DLK-1234",
			Status:         ticket.IN_PROGRESS,
			OriginalStatus: "firing",
			CreatedDate:    &openedAt,
		}
	}).Return(nil)
	tx.On("Update", mock.AnythingOfType("*ticket.Issue"), mock.Anything).Run(func(args mock.Arguments) {
		*updatedIssue = *args.Get(0).(*ticket.Issue)
	}).Return(nil)
	tx.On("Update", mock.AnythingOfType("*ticket.Incident"), mock.Anything).Run(func(args mock.Arguments) {
		*updatedIncident = *args.Get(0).(*ticket.Incident)
	}).Return(nil)
	tx.On("UnlockTables").Return(nil)
	tx.On("Commit").Return(nil)

	res := unithelper.DummyBasicRes(func(mockDal *mockdal.Dal) {
		mockDal.On("First", mock.AnythingOfType("*models.WebhookConnection"), mock.Anything).Run(func(args mock.Arguments) {
			args.Get(0).(*models.WebhookConnection).ID = 1
		}).Return(nil)
		mockDal.On("Begin").Return(tx)
	})
	prevBasicRes, prevLogger, prevVld, prevConnectionHelper := basicRes, logger, vld, connectionHelper
	basicRes = res
	logger = res.GetLogger()
	vld = validator.New()
	connectionHelper = api.NewConnectionHelper(res, vld, pluginName)
	t.Cleanup(func() {
		basicRes, logger, vld, connectionHelper = prevBasicRes, prevLogger, prevVld, prevConnectionHelper
	})
	return updatedIssue, updatedIncident
}

func TestCloseIssueByBodyUsesResolutionDateAndOriginalStatus(t *testing.T) {
	updatedIssue, updatedIncident := mockCloseIssueDb(t)

	output, err := CloseIssueByBody(&plugin.ApiResourceInput{
		Params: map[string]string{"connectionId": "1"},
		Body: map[string]interface{}{
			"issueKey":       "DLK-1234",
			"resolutionDate": "2026-06-13T10:00:00Z",
			"originalStatus": "resolved",
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, output.Status)

	resolvedAt := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	if assert.NotNil(t, updatedIssue.ResolutionDate) {
		assert.True(t, resolvedAt.Equal(*updatedIssue.ResolutionDate), "issue resolution date: %v", *updatedIssue.ResolutionDate)
	}
	assert.Equal(t, ticket.DONE, updatedIssue.Status)
	assert.Equal(t, "resolved", updatedIssue.OriginalStatus)
	if assert.NotNil(t, updatedIssue.LeadTimeMinutes) {
		assert.Equal(t, uint(60), *updatedIssue.LeadTimeMinutes)
	}

	if assert.NotNil(t, updatedIncident.ResolutionDate) {
		assert.True(t, resolvedAt.Equal(*updatedIncident.ResolutionDate), "incident resolution date: %v", *updatedIncident.ResolutionDate)
	}
	assert.Equal(t, ticket.DONE, updatedIncident.Status)
	assert.Equal(t, "resolved", updatedIncident.OriginalStatus)
	if assert.NotNil(t, updatedIncident.LeadTimeMinutes) {
		assert.Equal(t, uint(60), *updatedIncident.LeadTimeMinutes)
	}
}

func TestCloseIssueWithoutResolutionDateUsesNow(t *testing.T) {
	updatedIssue, updatedIncident := mockCloseIssueDb(t)

	before := time.Now()
	output, err := CloseIssue(&plugin.ApiResourceInput{
		Params: map[string]string{"connectionId": "1", "issueKey": "DLK-1234"},
	})
	after := time.Now()
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, output.Status)

	for _, resolutionDate := range []*time.Time{updatedIssue.ResolutionDate, updatedIncident.ResolutionDate} {
		if assert.NotNil(t, resolutionDate) {
			assert.False(t, resolutionDate.Before(before) || resolutionDate.After(after), "resolution date: %v", *resolutionDate)
		}
	}
	assert.Equal(t, ticket.DONE, updatedIssue.Status)
	assert.Equal(t, "", updatedIssue.OriginalStatus)
	assert.Equal(t, ticket.DONE, updatedIncident.Status)
	assert.Equal(t, "", updatedIncident.OriginalStatus)
}

func TestCloseIssueByBodyWithResolutionDateBeforeCreatedDate(t *testing.T) {
	updatedIssue, updatedIncident := mockCloseIssueDb(t)

	output, err := CloseIssueByBody(&plugin.ApiResourceInput{
		Params: map[string]string{"connectionId": "1"},
		Body: map[string]interface{}{
			"issueKey":       "DLK-1234",
			"resolutionDate": "2026-06-13T08:00:00Z",
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, output.Status)

	// a negative duration must not wrap around to a huge lead time
	assert.Nil(t, updatedIssue.LeadTimeMinutes)
	assert.Nil(t, updatedIncident.LeadTimeMinutes)
}
