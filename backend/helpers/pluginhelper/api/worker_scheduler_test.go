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
	"context"
	"testing"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/unithelper"

	"github.com/stretchr/testify/assert"
)

func TestWorkerSchedulerQpsControl(t *testing.T) {
	// assuming we want 2 requests per second
	testChannel := make(chan int, 100)
	ctx, cancel := context.WithCancel(context.Background())
	tickInterval, _ := CalcTickInterval(2, 1*time.Second)
	s, _ := NewWorkerScheduler(ctx, 5, tickInterval, unithelper.DummyLogger())
	defer s.Release()
	for i := 1; i <= 5; i++ {
		t := i
		s.SubmitBlocking(func() errors.Error {
			testChannel <- t
			return nil
		})
	}
	// after 1 second, 2 requerts should be issued
	time.Sleep(1200 * time.Millisecond)
	if len(testChannel) < 2 {
		t.Fatal(`worker not start`)
	}
	if len(testChannel) > 2 {
		t.Fatal(`worker run too fast`)
	}
	// after 2 seconds, 4 requests should be issued
	time.Sleep(time.Second)
	if len(testChannel) < 4 {
		t.Fatal(`worker not run after a second`)
	}
	if len(testChannel) > 4 {
		t.Fatal(`worker run too fast after a second`)
	}
	assert.Nil(t, s.WaitAsync())
	if len(testChannel) != 5 {
		t.Fatal(`worker not wait until finish`)
	}
	cancel()
}

// A failed task must be reported by the WaitAsync call that waits for it, not
// by a later one: a stateful collector saves its state when WaitAsync
// returns nil.
func TestWorkerSchedulerWaitAsyncReturnsTaskError(t *testing.T) {
	for i := 0; i < 1000; i++ {
		s, err := NewWorkerScheduler(context.Background(), 1, time.Microsecond, unithelper.DummyLogger())
		assert.Nil(t, err)
		s.SubmitBlocking(func() errors.Error {
			return errors.Default.New("request failed")
		})
		if !assert.NotNil(t, s.WaitAsync(), "run %d: WaitAsync returned before the task error was recorded", i) {
			s.Release()
			return
		}
		s.Release()
	}
}
