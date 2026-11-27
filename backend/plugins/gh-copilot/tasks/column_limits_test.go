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
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestFitColumn(t *testing.T) {
	t.Run("short value is unchanged", func(t *testing.T) {
		require.Equal(t, "go", fitColumn(nil, "language", "go"))
		require.Equal(t, "", fitColumn(nil, "language", ""))
	})

	t.Run("value of exactly 50 characters is unchanged", func(t *testing.T) {
		v := strings.Repeat("a", maxKeyColumnLength)
		require.Equal(t, v, fitColumn(nil, "language", v))
	})

	t.Run("long value is cut to 50 characters", func(t *testing.T) {
		v := "update companies set AB1api2permissions=2 where companyid=69 and more text after it"
		got := fitColumn(nil, "language", v)
		require.Equal(t, v[:maxKeyColumnLength], got)
		require.Len(t, got, maxKeyColumnLength)
	})

	t.Run("very long value is cut to 50 characters", func(t *testing.T) {
		got := fitColumn(nil, "ide", strings.Repeat("x", 5000))
		require.Len(t, got, maxKeyColumnLength)
	})

	t.Run("multi-byte characters are not split", func(t *testing.T) {
		v := strings.Repeat("é", 80)
		got := fitColumn(nil, "language", v)
		require.True(t, utf8.ValidString(got))
		require.Equal(t, maxKeyColumnLength, utf8.RuneCountInString(got))
	})
}
