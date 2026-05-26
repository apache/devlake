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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"

	"github.com/panjf2000/ants/v2"
)

// CalcTickInterval calculates tick interval for number of works to be
// executed in specified duration
func CalcTickInterval(numOfWorks int, duration time.Duration) (time.Duration, errors.Error) {
	if numOfWorks <= 0 {
		return 0, errors.Default.New("numOfWorks must be greater than 0")
	}
	if duration <= 0 {
		return 0, errors.Default.New("duration must be greater than 0")
	}
	return duration / time.Duration(numOfWorks), nil
}

// WorkerScheduler runs asynchronous tasks in parallel with throttling support
type WorkerScheduler struct {
	waitGroup    sync.WaitGroup
	pool         *ants.Pool
	ticker       *time.Ticker
	workerErrors []error
	ctx          context.Context
	mu           sync.Mutex
	counter      int32
	logger       log.Logger
	tickInterval time.Duration
}

//var callframeEnabled = os.Getenv("ASYNC_CF") == "true"

// NewWorkerScheduler creates a WorkerScheduler
func NewWorkerScheduler(
	ctx context.Context,
	numOfWorkers int,
	tickInterval time.Duration,
	logger log.Logger,
) (*WorkerScheduler, errors.Error) {
	s := &WorkerScheduler{
		ctx:          ctx,
		logger:       logger,
		tickInterval: tickInterval,
		ticker:       time.NewTicker(tickInterval),
	}
	pool, err := ants.NewPool(numOfWorkers, ants.WithPanicHandler(func(i interface{}) {
		s.checkError(i)
	}))
	if err != nil {
		return nil, errors.Convert(err)
	}
	s.pool = pool
	return s, nil
}

// SubmitBlocking enqueues a async task to ants, the task will be executed in future when timing is right.
// It doesn't return error because it wouldn't be any when with a Blocking semantic, returned error does nothing but
// causing confusion, more often, people thought it is returned by the task.
// Since it is async task, the callframes would not be available for production mode, you can export Environment
// Variable ASYNC_CF=true to enable callframes capturing when debugging.
// IMPORTANT: do NOT call SubmitBlocking inside the async task, it is likely to cause a deadlock, call
// SubmitNonBlocking instead when number of tasks is relatively small.
func (s *WorkerScheduler) SubmitBlocking(task func() errors.Error) {
	if s.HasError() {
		return
	}
	s.waitGroup.Add(1)
	s.checkError(s.pool.Submit(func() {
		defer s.waitGroup.Done()

		id := atomic.AddInt32(&s.counter, 1)
		s.logger.Debug("schedulerJob >>> %d started", id)
		defer s.logger.Debug("schedulerJob <<< %d ended", id)

		if s.HasError() {
			return
		}

		// normal error
		select {
		case <-s.ctx.Done():
			panic(s.ctx.Err())
		case <-s.ticker.C:
			err := task()
			if err != nil {
				panic(err)
			}
		}
	}))
}

/*
func (s *WorkerScheduler) gatherCallFrames() string {
	cf := "set Environment Variable ASYNC_CF=true to enable callframes capturing"
	if callframeEnabled {
		cf = utils.GatherCallFrames(1)
	}
	return cf
}
*/

func (s *WorkerScheduler) appendError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workerErrors = append(s.workerErrors, err)
}

func (s *WorkerScheduler) checkError(err interface{}) {
	if err == nil {
		return
	}
	switch e := err.(type) {
	case error:
		s.appendError(e)
	default:
		s.appendError(fmt.Errorf("%v", e))
	}
}

// HasError return if any error occurred
func (s *WorkerScheduler) HasError() bool {
	return len(s.workerErrors) > 0
}

// NextTick enqueues task in a NonBlocking manner, you should only call this method within task submitted by
// SubmitBlocking method
// IMPORTANT: do NOT call this method with a huge number of tasks, it is likely to eat up all available memory
func (s *WorkerScheduler) NextTick(task func() errors.Error) {
	// to make sure task will be enqueued
	s.waitGroup.Add(1)
	go func() {
		defer s.waitGroup.Done()
		s.checkError(task())
	}()
}

// Wait blocks current go-routine until all workers returned
func (s *WorkerScheduler) WaitAsync() errors.Error {
	s.waitGroup.Wait()
	if len(s.workerErrors) > 0 {
		for _, err := range s.workerErrors {
			if errors.Is(err, context.Canceled) {
				return errors.Default.Wrap(err, "task canceled")
			}
		}
		return errors.Default.Combine(s.workerErrors)
	}
	return nil
}

// Reset stops a WorkScheduler and resets its period to the specified duration.
func (s *WorkerScheduler) Reset(interval time.Duration) {
	s.tickInterval = interval
	s.ticker.Reset(interval)
}

// GetTickInterval returns current tick interval of the WorkScheduler
func (s *WorkerScheduler) GetTickInterval() time.Duration {
	return s.tickInterval
}

// Release resources
func (s *WorkerScheduler) Release() {
	s.waitGroup.Wait()
	s.pool.Release()
	if s.ticker != nil {
		s.ticker.Stop()
	}
}
