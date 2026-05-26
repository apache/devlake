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
	"reflect"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
)

// Iterator FIXME ...
type Iterator interface {
	HasNext() bool
	Fetch() (interface{}, errors.Error)
	Close() errors.Error
}

// DalCursorIterator FIXME ...
type DalCursorIterator struct {
	db        dal.Dal
	cursor    dal.Rows
	elemType  reflect.Type
	batchSize int
}

// NewDalCursorIterator FIXME ...
func NewDalCursorIterator(db dal.Dal, cursor dal.Rows, elemType reflect.Type) (*DalCursorIterator, errors.Error) {
	return NewBatchedDalCursorIterator(db, cursor, elemType, -1)
}

// NewBatchedDalCursorIterator FIXME ...
func NewBatchedDalCursorIterator(db dal.Dal, cursor dal.Rows, elemType reflect.Type, batchSize int) (*DalCursorIterator, errors.Error) {
	return &DalCursorIterator{
		db:        db,
		cursor:    cursor,
		elemType:  elemType,
		batchSize: batchSize,
	}, nil
}

// HasNext increments the row cursor. If we're at the end, it'll return false.
func (c *DalCursorIterator) HasNext() bool {
	return c.cursor.Next()
}

// Fetch if batching is disabled, it'll read a single row, otherwise it'll read as many rows up to the batch size, and the
// runtime return type will be []interface{}. Note, HasNext needs to have been called before invoking this.
func (c *DalCursorIterator) Fetch() (interface{}, errors.Error) {
	if c.batchSize > 0 {
		return c.batchedFetch()
	}
	if c.batchSize != -1 {
		panic("invalid batch size")
	}
	elem := reflect.New(c.elemType).Interface()
	err := c.db.Fetch(c.cursor, elem)
	if err != nil {
		return nil, err
	}
	return elem, nil
}

func (c *DalCursorIterator) batchedFetch() (interface{}, errors.Error) {
	var elems []interface{}
	for i := 1; ; i++ {
		elem := reflect.New(c.elemType).Interface()
		err := c.cursor.Scan(elem)
		if err != nil {
			return nil, errors.Convert(err)
		}
		elems = append(elems, elem)
		if i == c.batchSize || !c.HasNext() {
			break
		}
	}
	return elems, nil
}

// Close iterator
func (c *DalCursorIterator) Close() errors.Error {
	return errors.Convert(c.cursor.Close())
}

var _ Iterator = (*DalCursorIterator)(nil)

// DateIterator FIXME ...
type DateIterator struct {
	startTime time.Time
	endTime   time.Time
	Days      int
	Current   int
}

// DatePair FIXME ...
type DatePair struct {
	PairStartTime time.Time
	PairEndTime   time.Time
}

// HasNext FIXME ...
func (c *DateIterator) HasNext() bool {
	return c.Current < c.Days
}

// Fetch FIXME ...
func (c *DateIterator) Fetch() (interface{}, errors.Error) {
	c.Current++
	return &DatePair{
		PairStartTime: c.startTime.AddDate(0, 0, c.Current),
		PairEndTime:   c.endTime.AddDate(0, 0, c.Current),
	}, nil
}

// Close iterator
func (c *DateIterator) Close() errors.Error {
	return nil
}

// NewDateIterator FIXME ...
func NewDateIterator(days int) (*DateIterator, errors.Error) {
	endTime := time.Now().Truncate(24 * time.Hour)
	return &DateIterator{
		startTime: endTime.AddDate(0, 0, -days-1),
		endTime:   endTime.AddDate(0, 0, -days),
		Days:      days,
		Current:   0,
	}, nil
}

// QueueIterator implements Iterator based on Queue
type QueueIterator struct {
	queue *Queue
}

// HasNext increments the row cursor. If we're at the end, it'll return false.
func (q *QueueIterator) HasNext() bool {
	return q.queue.GetCount() > 0
}

// Fetch current item
func (q *QueueIterator) Fetch() (interface{}, errors.Error) {
	node := q.queue.Pull()
	if node == nil {
		return nil, nil
	} else {
		return node.Data(), nil
	}
}

// Push a data into queue
func (q *QueueIterator) Push(data interface{}) {
	q.queue.Push(NewQueueIteratorNode(data))
}

// Close releases resources
func (q *QueueIterator) Close() errors.Error {
	q.queue.CleanWithOutLock()
	return nil
}

// NewQueueIterator creates a new QueueIterator
func NewQueueIterator() *QueueIterator {
	return &QueueIterator{
		queue: NewQueue(),
	}
}
