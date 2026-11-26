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
	"github.com/apache/devlake/core/log"
)

// maxKeyColumnLength is the size of the varchar columns that hold the ide and
// language names. They are part of the primary key, so they cannot grow.
const maxKeyColumnLength = 50

// fitColumn cuts value to at most maxKeyColumnLength characters, so one
// unexpected value from the API cannot fail the whole collection with
// "Data too long for column". It cuts on a character boundary and logs a
// warning when it changes the value.
func fitColumn(logger log.Logger, field, value string) string {
	runes := []rune(value)
	if len(runes) <= maxKeyColumnLength {
		return value
	}
	cut := string(runes[:maxKeyColumnLength])
	if logger != nil {
		logger.Warn(nil, "%s value is longer than %d characters, truncating it to %q", field, maxKeyColumnLength, cut)
	}
	return cut
}
