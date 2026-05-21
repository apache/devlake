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
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// getTotalPagesFromResponse parses the GitHub Link header to determine the last page number.
// This is used for paginated list endpoints (teams, team members).
func getTotalPagesFromResponse(res *http.Response, _ *helper.ApiCollectorArgs) (int, errors.Error) {
	link := res.Header.Get("Link")
	if link == "" {
		return 0, nil
	}
	pagePattern := regexp.MustCompile(`page=(\d+)`)
	relPattern := regexp.MustCompile(`rel="([a-z]+)"`)
	for _, part := range strings.Split(link, ",") {
		relMatch := relPattern.FindStringSubmatch(part)
		if len(relMatch) < 2 || relMatch[1] != "last" {
			continue
		}
		pageMatch := pagePattern.FindStringSubmatch(part)
		if len(pageMatch) < 2 {
			continue
		}
		last, err := strconv.Atoi(pageMatch[1])
		if err != nil {
			return 0, errors.Default.Wrap(err, "failed to parse last page")
		}
		return last, nil
	}
	return 0, nil
}
