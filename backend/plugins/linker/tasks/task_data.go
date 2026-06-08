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

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

type LinkerOptions struct {
	PrToIssueRegexp string `json:"prToIssueRegexp"`
	ProjectName     string `json:"projectName"`
}

type LinkerTaskData struct {
	Options         *LinkerOptions
	PrToIssueRegexp *regexp.Regexp
}

func DecodeAndValidateTaskOptions(options map[string]interface{}) (*LinkerOptions, errors.Error) {
	var op LinkerOptions
	err := helper.Decode(options, &op, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error decoding linker task options")
	}
	op.PrToIssueRegexp = strings.TrimSpace(op.PrToIssueRegexp)
	if op.PrToIssueRegexp == "" {
		return nil, errors.BadInput.New("prToIssueRegexp is required")
	}
	return &op, nil
}
