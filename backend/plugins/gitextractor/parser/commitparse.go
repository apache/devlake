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
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// parsedCommit is the metadata of one commit object, as stored by git.
type parsedCommit struct {
	Sha       string
	Parents   []string
	Author    parsedSignature
	Committer parsedSignature
	Message   string
}

type parsedSignature struct {
	Name  string
	Email string
	When  time.Time
}

// parseCommitObject parses the raw content of a commit object.
func parseCommitObject(sha string, content []byte) (*parsedCommit, error) {
	commit := &parsedCommit{Sha: sha}

	header := content
	if idx := bytes.Index(content, []byte("\n\n")); idx >= 0 {
		header = content[:idx]
		commit.Message = string(content[idx+2:])
	}

	for _, line := range strings.Split(string(header), "\n") {
		if line == "" {
			continue
		}
		if line[0] == ' ' {
			// continuation of a multi-line header such as gpgsig, not used here
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		switch key {
		case "parent":
			commit.Parents = append(commit.Parents, value)
		case "author":
			signature, err := parseSignature(value)
			if err != nil {
				return nil, fmt.Errorf("commit %s: %w", sha, err)
			}
			commit.Author = signature
		case "committer":
			signature, err := parseSignature(value)
			if err != nil {
				return nil, fmt.Errorf("commit %s: %w", sha, err)
			}
			commit.Committer = signature
		}
	}
	return commit, nil
}

// parseSignature parses "Name <email> <unix timestamp> <timezone>".
func parseSignature(value string) (parsedSignature, error) {
	var signature parsedSignature

	tzSpace := strings.LastIndexByte(value, ' ')
	if tzSpace < 0 {
		return signature, fmt.Errorf("invalid signature %q", value)
	}
	timezone := value[tzSpace+1:]
	rest := value[:tzSpace]

	tsSpace := strings.LastIndexByte(rest, ' ')
	if tsSpace < 0 {
		return signature, fmt.Errorf("invalid signature %q", value)
	}
	seconds, err := strconv.ParseInt(rest[tsSpace+1:], 10, 64)
	if err != nil {
		return signature, fmt.Errorf("invalid timestamp in signature %q", value)
	}
	identity := rest[:tsSpace]

	if lt := strings.LastIndexByte(identity, '<'); lt >= 0 {
		if gt := strings.LastIndexByte(identity[lt:], '>'); gt >= 0 {
			signature.Email = identity[lt+1 : lt+gt]
			signature.Name = strings.TrimSpace(identity[:lt])
		} else {
			signature.Name = strings.TrimSpace(identity)
		}
	} else {
		signature.Name = strings.TrimSpace(identity)
	}

	signature.When = time.Unix(seconds, 0).In(parseTimezone(timezone))
	return signature, nil
}

// parseTimezone converts a git timezone such as "+0800" into a fixed zone so
// the original offset of the commit date is preserved.
func parseTimezone(timezone string) *time.Location {
	if len(timezone) != 5 || (timezone[0] != '+' && timezone[0] != '-') {
		return time.UTC
	}
	hours, errH := strconv.Atoi(timezone[1:3])
	minutes, errM := strconv.Atoi(timezone[3:5])
	if errH != nil || errM != nil {
		return time.UTC
	}
	offset := hours*3600 + minutes*60
	if timezone[0] == '-' {
		offset = -offset
	}
	return time.FixedZone(timezone, offset)
}

// readCatFileObject reads one object from `git cat-file --batch` output.
// It returns io.EOF when the stream is exhausted.
func readCatFileObject(reader *bufio.Reader) (string, []byte, error) {
	header, err := reader.ReadString('\n')
	if err == io.EOF && header == "" {
		return "", nil, io.EOF
	}
	if err != nil && err != io.EOF {
		return "", nil, err
	}

	fields := strings.Fields(header)
	if len(fields) != 3 {
		return "", nil, fmt.Errorf("unexpected cat-file header %q", strings.TrimSpace(header))
	}
	if fields[1] != "commit" {
		return "", nil, fmt.Errorf("object %s is a %s, not a commit", fields[0], fields[1])
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", nil, fmt.Errorf("unexpected cat-file size in %q", strings.TrimSpace(header))
	}

	content := make([]byte, size)
	if _, err := io.ReadFull(reader, content); err != nil {
		return "", nil, err
	}
	if _, err := reader.Discard(1); err != nil { // trailing newline
		return "", nil, err
	}
	return fields[0], content, nil
}
