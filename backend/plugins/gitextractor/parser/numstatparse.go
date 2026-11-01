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

package parser

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const numstatRecordMarker = '\x01'

type commitFileStat struct {
	Path      string
	Additions int
	Deletions int
}

type commitStats struct {
	Sha   string
	Files []commitFileStat
}

// parseNumstatStream parses the output of
//
//	git log --format=%x01%H --numstat --no-renames -z ...
//
// Each commit is introduced by a \x01<sha> segment; every following NUL
// terminated segment is "additions<TAB>deletions<TAB>path". Paths may contain
// tabs, spaces and newlines, but never NUL, so splitting on NUL is safe.
func parseNumstatStream(reader io.Reader, fn func(*commitStats) error) error {
	buffered := bufio.NewReaderSize(reader, 256*1024)

	var current *commitStats
	flush := func() error {
		if current == nil {
			return nil
		}
		if err := fn(current); err != nil {
			return err
		}
		current = nil
		return nil
	}

	for {
		segment, err := buffered.ReadString(0)
		if err != nil && err != io.EOF {
			return err
		}
		// ReadString returns data without the delimiter on EOF
		segment = strings.TrimSuffix(segment, "\x00")

		switch {
		case segment == "":
			// nothing to do; covers a trailing NUL
		case segment[0] == numstatRecordMarker:
			if err := flush(); err != nil {
				return err
			}
			current = &commitStats{Sha: segment[1:]}
		default:
			if current == nil {
				return fmt.Errorf("numstat entry %q before any commit", segment)
			}
			entry := strings.TrimPrefix(segment, "\n")
			stat, err := parseNumstatEntry(entry)
			if err != nil {
				return fmt.Errorf("commit %s: %w", current.Sha, err)
			}
			current.Files = append(current.Files, stat)
		}

		if err == io.EOF {
			break
		}
	}
	return flush()
}

func parseNumstatEntry(entry string) (commitFileStat, error) {
	var stat commitFileStat
	parts := strings.SplitN(entry, "\t", 3)
	if len(parts) != 3 {
		return stat, fmt.Errorf("unexpected numstat entry %q", entry)
	}
	stat.Path = parts[2]

	additions, err := parseNumstatCount(parts[0])
	if err != nil {
		return stat, err
	}
	deletions, err := parseNumstatCount(parts[1])
	if err != nil {
		return stat, err
	}
	stat.Additions = additions
	stat.Deletions = deletions
	return stat, nil
}

// parseNumstatCount parses a numstat count. Binary files are reported as "-"
// and count as zero changed lines.
func parseNumstatCount(value string) (int, error) {
	if value == "-" {
		return 0, nil
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("unexpected numstat count %q", value)
	}
	return count, nil
}
