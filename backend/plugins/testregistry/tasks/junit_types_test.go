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

	mocklog "github.com/apache/devlake/mocks/core/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetJUnitRegex(t *testing.T) {
	t.Run("empty pattern returns default", func(t *testing.T) {
		regex, err := GetJUnitRegex("", nil)
		assert.Nil(t, err)
		assert.Equal(t, JUnitRegexpSearch, regex)
	})

	t.Run("valid custom pattern returns compiled regex", func(t *testing.T) {
		regex, err := GetJUnitRegex(`test-.*\.xml`, nil)
		assert.Nil(t, err)
		assert.NotNil(t, regex)
		assert.True(t, regex.MatchString("test-results.xml"))
		assert.False(t, regex.MatchString("other.json"))
	})

	t.Run("cached pattern returns same object", func(t *testing.T) {
		pattern := `cached-pattern-[0-9]+\.xml`
		first, err1 := GetJUnitRegex(pattern, nil)
		second, err2 := GetJUnitRegex(pattern, nil)
		assert.Nil(t, err1)
		assert.Nil(t, err2)
		assert.Same(t, first, second)
	})

	t.Run("invalid pattern returns error and default", func(t *testing.T) {
		mockLogger := new(mocklog.Logger)
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

		regex, err := GetJUnitRegex(`[invalid`, mockLogger)
		assert.NotNil(t, err)
		assert.Equal(t, JUnitRegexpSearch, regex)
	})

	t.Run("default regex matches expected patterns", func(t *testing.T) {
		regex, _ := GetJUnitRegex("", nil)
		assert.True(t, regex.MatchString("devlake-abc123.xml"))
		assert.True(t, regex.MatchString("e2e-test.xml"))
		assert.True(t, regex.MatchString("qd-report-results.junit"))
		assert.False(t, regex.MatchString("random-file.txt"))
	})
}

func TestGetJUnitRegexOrDefault(t *testing.T) {
	t.Run("empty pattern returns default", func(t *testing.T) {
		regex := GetJUnitRegexOrDefault("", nil)
		assert.Equal(t, JUnitRegexpSearch, regex)
	})

	t.Run("valid pattern returns compiled regex", func(t *testing.T) {
		regex := GetJUnitRegexOrDefault(`custom-.*\.xml`, nil)
		assert.NotNil(t, regex)
		assert.True(t, regex.MatchString("custom-test.xml"))
	})

	t.Run("invalid pattern returns default without error", func(t *testing.T) {
		mockLogger := new(mocklog.Logger)
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

		regex := GetJUnitRegexOrDefault(`[broken`, mockLogger)
		assert.Equal(t, JUnitRegexpSearch, regex)
	})
}
