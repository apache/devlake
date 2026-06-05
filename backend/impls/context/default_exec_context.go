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

package context

import (
	gocontext "context"
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/plugin"
	"sync"
	"sync/atomic"
)

// shared by TaskContext and SubTaskContext
type defaultExecContext struct {
	context.BasicRes
	ctx      gocontext.Context
	name     string
	data     interface{}
	total    int
	current  int64
	mu       sync.Mutex
	progress chan plugin.RunningProgress
}

func newDefaultExecContext(
	ctx gocontext.Context,
	basicRes context.BasicRes,
	name string,
	data interface{},
	progress chan plugin.RunningProgress,
) *defaultExecContext {
	return &defaultExecContext{
		BasicRes: basicRes,
		ctx:      ctx,
		name:     name,
		data:     data,
		progress: progress,
	}
}

func (c *defaultExecContext) GetName() string {
	return c.name
}

func (c *defaultExecContext) GetContext() gocontext.Context {
	return c.ctx
}

func (c *defaultExecContext) GetData() interface{} {
	return c.data
}

func (c *defaultExecContext) SetProgress(progressType plugin.ProgressType, current int, total int) {
	c.current = int64(current)
	c.total = total

	if c.progress != nil {
		c.progress <- plugin.RunningProgress{
			Type:    progressType,
			Current: current,
			Total:   total,
		}
	}
}

func (c *defaultExecContext) IncProgress(progressType plugin.ProgressType, quantity int) {
	atomic.AddInt64(&c.current, int64(quantity))
	current := c.current
	if c.progress != nil {
		c.progress <- plugin.RunningProgress{
			Type:    progressType,
			Current: int(current),
			Total:   c.total,
		}
		// subtask progress may go too fast, remove old messages because they don't matter any more
		if progressType == plugin.SubTaskSetProgress {
			for len(c.progress) > 1 {
				<-c.progress
			}
		}
	}
}

func (c *defaultExecContext) GetProgress() int {
	return int(atomic.LoadInt64(&c.current))
}

func (c *defaultExecContext) fork(name string) *defaultExecContext {
	return newDefaultExecContext(
		c.ctx,
		c.BasicRes.NestedLogger(name),
		name,
		c.data,
		c.progress,
	)
}
