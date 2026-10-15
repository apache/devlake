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
	"regexp"
	"strings"

	"github.com/apache/devlake/core/errors"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

type DoraApiParams struct {
	ProjectName string
}

type DoraOptions struct {
	Tasks       []string `json:"tasks,omitempty"`
	Since       string
	ProjectName string  `json:"projectName"`
	ScopeId     *string `json:"scopeId,omitempty"`
	// PrHeadRefPattern is a Go RE2 regex; when set, only merged pull requests whose
	// source branch (head_ref) matches it are counted by calculateChangeLeadTime.
	PrHeadRefPattern string `json:"prHeadRefPattern,omitempty"`
}

type DoraTaskData struct {
	Options                         *DoraOptions
	DisableIssueToIncidentGenerator bool
	PrHeadRefRegexp                 *regexp.Regexp
}

func DecodeAndValidateTaskOptions(options map[string]interface{}) (*DoraOptions, errors.Error) {
	var op DoraOptions
	err := helper.Decode(options, &op, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error decoding DORA task options")
	}

	return &op, nil
}

// CompilePrHeadRefPattern compiles the prHeadRefPattern option.
// A blank pattern means no filtering and yields a nil regexp.
func CompilePrHeadRefPattern(pattern string) (*regexp.Regexp, errors.Error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid prHeadRefPattern")
	}
	return re, nil
}

// MatchPrHeadRef reports whether a pull request source branch matches the pattern.
// A nil regexp matches everything. The "refs/heads/" prefix is ignored.
func MatchPrHeadRef(re *regexp.Regexp, headRef string) bool {
	if re == nil {
		return true
	}
	return re.MatchString(strings.TrimPrefix(headRef, "refs/heads/"))
}
