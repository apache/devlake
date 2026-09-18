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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQueueIterator(t *testing.T) {
	it := NewQueueIterator()
	it.Push("a")
	it.Push("b")
	require.True(t, it.HasNext())
	folderRaw, err := it.Fetch()
	require.NoError(t, err)
	data := folderRaw.(string)
	require.Equal(t, "a", data)
	require.True(t, it.HasNext())
	folderRaw, err = it.Fetch()
	require.NoError(t, err)
	data = folderRaw.(string)
	require.Equal(t, "b", data)
	require.False(t, it.HasNext())
}

func TestQueue(t *testing.T) {
	q := NewQueue()
	q.Push(NewQueueIteratorNode("a"))
	q.Push(NewQueueIteratorNode("b"))

	require.Equal(t, q.GetCount(), int64(2))
	folderRaw := q.PullWithWorkingBlock()
	data, ok := folderRaw.Data().(string)
	require.True(t, ok)
	require.Equal(t, "a", data)
	require.Equal(t, q.GetCount(), int64(1))
	folderRaw = q.PullWithWorkingBlock()
	data, ok = folderRaw.Data().(string)
	require.True(t, ok)
	require.Equal(t, "b", data)
	require.Equal(t, q.GetCount(), int64(0))

	empty := make(chan struct{})
	waited := make(chan struct{})
	go func() {
		defer close(empty)
		require.Equal(t, q.GetCountWithWorkingBlock(), int64(1))
		data, ok := q.PullWithWorkingBlock().Data().(string)
		require.True(t, ok)
		require.Equal(t, data, "c")
		dataNode := q.PullWithWorkingBlock()
		require.Equal(t, dataNode, nil)
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		q.Push(NewQueueIteratorNode("c"))
		close(waited)
		q.Finish(3)
	}()

	select {
	case <-empty:
	case <-time.After(5 * time.Second):
		t.Fatal("queue consumer did not finish")
	}

	select {
	case <-waited:
	default:
		t.Fatal("queue producer did not finish")
	}
}
