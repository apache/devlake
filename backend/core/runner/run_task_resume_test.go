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

package runner

import (
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRunSubtaskCompletionMarker(t *testing.T) {
	for _, mode := range []string{"success", "error", "panic", "start-write-error", "end-write-error"} {
		t.Run(mode, func(t *testing.T) {
			var records []models.Subtask
			var db *mockdal.Dal
			failure := errors.Default.New("injected failure")
			res := unithelper.DummyBasicRes(func(d *mockdal.Dal) {
				db = d
				var startErr errors.Error
				if mode == "start-write-error" {
					startErr = failure
				}
				d.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					records = append(records, *args.Get(0).(*models.Subtask))
				}).Return(startErr).Once()
				if mode != "start-write-error" && mode != "panic" {
					var endErr errors.Error
					if mode == "end-write-error" {
						endErr = failure
					}
					d.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
						records = append(records, *args.Get(0).(*models.Subtask))
					}).Return(endErr).Once()
				}
			})
			for _, expectation := range db.ExpectedCalls {
				if expectation.Method == "AllTables" {
					expectation.Maybe()
				}
			}
			called := false
			invoke := func() errors.Error {
				return runSubtask(res, unithelper.DummySubTaskContext(db), 1, 1, func(plugin.SubTaskContext) errors.Error {
					called = true
					if mode == "panic" {
						panic("injected panic")
					}
					if mode == "error" {
						return failure
					}
					return nil
				})
			}
			if mode == "panic" {
				require.Panics(t, func() { _ = invoke() })
			} else {
				err := invoke()
				require.Equal(t, mode != "success", err != nil)
			}
			require.Equal(t, mode != "start-write-error", called)
			require.Nil(t, records[0].FinishedAt)
			if mode == "error" {
				require.Nil(t, records[1].FinishedAt)
				require.True(t, records[1].IsFailed)
			}
			if mode == "success" {
				require.NotNil(t, records[1].FinishedAt)
				require.False(t, records[1].IsFailed)
			}
			db.AssertExpectations(t)
		})
	}
}
