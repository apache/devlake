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

package pushapiaccess

import (
	"regexp"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
)

var tableNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

const internalTablePrefix = "_devlake_"

func ValidateTable(table string, allowedTables string) errors.Error {
	if !tableNameRegex.MatchString(table) {
		return errors.BadInput.New("table name invalid")
	}
	if strings.HasPrefix(table, internalTablePrefix) {
		return errors.Forbidden.New("writing internal tables via push API is forbidden")
	}

	allowlist := map[string]struct{}{}
	for _, t := range strings.Split(allowedTables, ",") {
		name := strings.TrimSpace(t)
		if name == "" || strings.HasPrefix(name, internalTablePrefix) {
			continue
		}
		allowlist[name] = struct{}{}
	}
	if len(allowlist) == 0 {
		return errors.Forbidden.New("push API is disabled unless PUSH_API_ALLOWED_TABLES is configured")
	}
	if _, ok := allowlist[table]; !ok {
		return errors.Forbidden.New("table name is not in the allowed list")
	}
	return nil
}
