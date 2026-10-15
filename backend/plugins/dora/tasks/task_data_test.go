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

	"github.com/apache/devlake/core/errors"
	"github.com/stretchr/testify/assert"
)

func TestCompilePrHeadRefPattern(t *testing.T) {
	for _, pattern := range []string{"", "   "} {
		re, err := CompilePrHeadRefPattern(pattern)
		assert.Nil(t, err)
		assert.Nil(t, re)
	}

	re, err := CompilePrHeadRefPattern("^(feat")
	assert.Nil(t, re)
	if assert.NotNil(t, err) {
		assert.Equal(t, errors.BadInput, err.GetType())
	}

	re, err = CompilePrHeadRefPattern(" ^(feat|fix)/ ")
	assert.Nil(t, err)
	if assert.NotNil(t, re) {
		assert.Equal(t, "^(feat|fix)/", re.String())
	}
}

func TestMatchPrHeadRef(t *testing.T) {
	assert.True(t, MatchPrHeadRef(nil, ""))
	assert.True(t, MatchPrHeadRef(nil, "chore/a"))

	re, err := CompilePrHeadRefPattern("^(feat|fix)/")
	assert.Nil(t, err)
	assert.True(t, MatchPrHeadRef(re, "feat/a"))
	assert.True(t, MatchPrHeadRef(re, "refs/heads/fix/b"))
	assert.False(t, MatchPrHeadRef(re, "chore/a"))
	assert.False(t, MatchPrHeadRef(re, ""))
	assert.False(t, MatchPrHeadRef(re, "FEAT/x"))

	re, err = CompilePrHeadRefPattern("(?i)^feat/")
	assert.Nil(t, err)
	assert.True(t, MatchPrHeadRef(re, "FEAT/x"))
}
