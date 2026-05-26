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
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/log"
	"github.com/stretchr/testify/require"
)

type captureLogger struct {
	warnings []string
}

func (l *captureLogger) IsLevelEnabled(level log.LogLevel) bool { return true }
func (l *captureLogger) Printf(format string, a ...interface{}) {}
func (l *captureLogger) Log(level log.LogLevel, format string, a ...interface{}) {
}
func (l *captureLogger) Debug(format string, a ...interface{}) {}
func (l *captureLogger) Info(format string, a ...interface{})  {}
func (l *captureLogger) Warn(err error, format string, a ...interface{}) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, a...))
}
func (l *captureLogger) Error(err error, format string, a ...interface{}) {}
func (l *captureLogger) Nested(name string) log.Logger                    { return l }
func (l *captureLogger) GetConfig() *log.LoggerConfig                     { return &log.LoggerConfig{} }
func (l *captureLogger) SetStream(config *log.LoggerStreamConfig)         {}

func TestHandleGitHubRetryAfterSleepsAndReturnsError(t *testing.T) {
	res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
	res.Header.Set("Retry-After", "10")

	logger := &captureLogger{}
	var slept time.Duration

	err := handleGitHubRetryAfter(
		res,
		logger,
		func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		func(d time.Duration) { slept = d },
	)

	require.NotNil(t, err)
	require.Equal(t, 10*time.Second, slept)
	require.Len(t, logger.warnings, 1)
	require.Contains(t, logger.warnings[0], "sleeping")
}

func TestHandleGitHubRetryAfterNoopOnNon429(t *testing.T) {
	res := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}
	var slept time.Duration
	logger := &captureLogger{}

	err := handleGitHubRetryAfter(res, logger, nil, func(d time.Duration) { slept = d })
	require.Nil(t, err)
	require.Equal(t, time.Duration(0), slept)
	require.Len(t, logger.warnings, 0)
}

func TestGitHubApiVersionHeaderValue(t *testing.T) {
	require.Equal(t, "2026-03-10", gitHubApiVersion,
		"GitHub API version header was changed; verify the new version is supported and update this test")
}
