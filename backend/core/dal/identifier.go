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

package dal

import (
	"fmt"
	"regexp"

	"github.com/apache/incubator-devlake/core/errors"
)

// validIdentifierRegex matches valid SQL identifiers: alphanumeric, underscores, and dots (for schema.table)
var validIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// ValidateTableName checks that a table name is a safe SQL identifier to prevent SQL injection.
func ValidateTableName(name string) errors.Error {
	if name == "" {
		return errors.Default.New("table name must not be empty")
	}
	if !validIdentifierRegex.MatchString(name) {
		return errors.Default.New(fmt.Sprintf("invalid table name: %q", name))
	}
	return nil
}

// ValidateColumnName checks that a column name is a safe SQL identifier to prevent SQL injection.
func ValidateColumnName(name string) errors.Error {
	if name == "" {
		return errors.Default.New("column name must not be empty")
	}
	if !validIdentifierRegex.MatchString(name) {
		return errors.Default.New(fmt.Sprintf("invalid column name: %q", name))
	}
	return nil
}
