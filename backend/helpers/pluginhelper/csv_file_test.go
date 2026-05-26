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

package pluginhelper

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestExampleCsvFile(t *testing.T) {
	tmpPath := t.TempDir()
	filename := fmt.Sprintf(`%s/foobar.csv`, tmpPath)
	println(filename)

	writer, _ := NewCsvFileWriter(filename, []string{"id", "name", "json", "created_at"})
	writer.Write([]string{"123", "foobar", `{"url": "https://example.com"}`, "2022-05-05 09:56:43.438000000"})
	writer.Close()

	iter, _ := NewCsvFileIterator(filename)
	defer iter.Close()
	for iter.HasNext() {
		row := iter.Fetch()
		assert.Equal(t, row["name"], "foobar", "name not equal")
		assert.Equal(t, row["json"], `{"url": "https://example.com"}`, "json not equal")
	}
}

func TestWrongCsvPath(t *testing.T) {
	tmpPath := t.TempDir()
	filename := fmt.Sprintf(`%s/foobar.txt`, tmpPath)
	println(filename)

	_, err := NewCsvFileWriter(filename, []string{})
	if err == nil {
		t.Fatal("the code did not return error")
	}
}
