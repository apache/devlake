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

package parse

import "strings"

// TreeEntry is one GitLab repository tree item.
type TreeEntry struct {
	ID   string
	Path string
	Type string
	Mode string
}

func hasSegment(path, segment string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}

func underPrefix(path, prefix string) bool {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return true
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func skippedPath(path string) bool {
	return hasSegment(path, "current")
}

func isPipelineData(path string) bool {
	return strings.HasSuffix(path, "/pipeline-data.json")
}

func isStratTask(path string) bool {
	return strings.Contains(path, "/strat-tasks/") && strings.HasSuffix(path, ".md")
}

// Matches reports whether this parser owns the path.
func Matches(parser, path string) bool {
	if path == "" || skippedPath(path) {
		return false
	}
	switch parser {
	case ParserRFE:
		return strings.HasSuffix(path, ".result.md")
	case ParserStrat:
		return isPipelineData(path) || isStratTask(path)
	default:
		return false
	}
}

// BatchDir is the timestamped directory that contains a strategy artifact.
func BatchDir(path string) string {
	if i := strings.Index(path, "/strat-tasks/"); i >= 0 {
		return path[:i]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i]
	}
	return ""
}

// SelectPaths keeps parser-owned blobs under prefix and drops strat-tasks
// in a batch that also has pipeline-data.json.
func SelectPaths(parser, prefix string, entries []TreeEntry) []TreeEntry {
	selected := make([]TreeEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Type != "" && entry.Type != "blob" {
			continue
		}
		if entry.Mode == "120000" {
			continue
		}
		if !underPrefix(entry.Path, prefix) || !Matches(parser, entry.Path) {
			continue
		}
		selected = append(selected, entry)
	}
	if parser != ParserStrat {
		return selected
	}
	batches := map[string]bool{}
	for _, entry := range selected {
		if isPipelineData(entry.Path) {
			batches[BatchDir(entry.Path)] = true
		}
	}
	filtered := make([]TreeEntry, 0, len(selected))
	for _, entry := range selected {
		if isStratTask(entry.Path) && batches[BatchDir(entry.Path)] {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
