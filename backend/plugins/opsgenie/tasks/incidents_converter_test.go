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
	"testing"
	"time"

	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/plugins/opsgenie/models"
	"github.com/stretchr/testify/assert"
)

func TestGetStatus(t *testing.T) {
	cases := map[models.IncidentStatus]string{
		models.IncidentStatusOpen:     ticket.IN_PROGRESS,
		models.IncidentStatusResolved: ticket.DONE,
		models.IncidentStatusClosed:   ticket.DONE,
	}
	for status, expected := range cases {
		t.Run(string(status), func(t *testing.T) {
			assert.Equal(t, expected, getStatus(&models.Incident{Status: status}))
		})
	}
}

func TestGetTimes(t *testing.T) {
	created := time.Date(2023, 9, 5, 18, 20, 0, 0, time.UTC)
	updated := created.Add(90 * time.Minute)

	for _, status := range []models.IncidentStatus{models.IncidentStatusResolved, models.IncidentStatusClosed} {
		t.Run(string(status), func(t *testing.T) {
			leadTime, resolutionDate := getTimes(&models.Incident{Status: status, CreatedDate: created, UpdatedDate: updated})
			if assert.NotNil(t, resolutionDate) && assert.NotNil(t, leadTime) {
				assert.Equal(t, updated, *resolutionDate)
				assert.Equal(t, uint(90), *leadTime)
			}
		})
	}

	t.Run("open", func(t *testing.T) {
		leadTime, resolutionDate := getTimes(&models.Incident{Status: models.IncidentStatusOpen, CreatedDate: created, UpdatedDate: updated})
		assert.Nil(t, leadTime)
		assert.Nil(t, resolutionDate)
	})
}
