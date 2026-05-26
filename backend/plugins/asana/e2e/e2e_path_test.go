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

package e2e

import (
	"path/filepath"
	"runtime"
)

// e2eDir returns the directory of the asana e2e package so CSV paths work when
// tests run from backend/ (CI) or from the e2e directory.
func e2eDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func rawTablePath(name string) string {
	return filepath.Join(e2eDir(), "raw_tables", name)
}

func snapshotPath(name string) string {
	return filepath.Join(e2eDir(), "snapshot_tables", name)
}
