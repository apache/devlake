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

const (
	patchChangeContext  = ' '
	patchChangeAddition = '+'
	patchChangeDeletion = '-'

	// patchRecordMarker introduces the commit sha in the `git log --format`
	// output. A patch line can never start with it: every patch content line
	// starts with ' ', '+' or '-'.
	patchRecordMarker = "\x01"
)

type patchLine struct {
	Origin  byte
	OldLine int
	NewLine int
	HunkNum int
}

type patchFile struct {
	OldPath string
	NewPath string
	Lines   []patchLine
}

type commitPatch struct {
	Sha   string
	Files []patchFile
}

type patchParser struct {
	commit  *commitPatch
	current *patchFile

	oldLine int
	newLine int
	hunkNum int

	inHunk bool
}

// parsePatchStream parses `git log --format=%x01%H -p ...` output.
func parsePatchStream(reader io.Reader, fn func(*commitPatch) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 16*1024*1024)

	var parser *patchParser
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, patchRecordMarker) {
			if err := flushPatch(parser, fn); err != nil {
				return err
			}
			parser = &patchParser{commit: &commitPatch{Sha: line[len(patchRecordMarker):]}}
			continue
		}
		if parser == nil {
			continue
		}
		if err := parser.line(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flushPatch(parser, fn)
}

func flushPatch(parser *patchParser, fn func(*commitPatch) error) error {
	if parser == nil || parser.commit == nil {
		return nil
	}
	parser.endFile()
	commit := parser.commit
	parser.commit = nil
	return fn(commit)
}

func (p *patchParser) line(line string) error {
	switch {
	case strings.HasPrefix(line, "diff --git "):
		p.endFile()
		oldPath, newPath, err := parseDiffGitPaths(strings.TrimPrefix(line, "diff --git "))
		if err != nil {
			return err
		}
		p.current = &patchFile{OldPath: oldPath, NewPath: newPath}
		p.inHunk = false
		return nil
	case strings.HasPrefix(line, "--- "):
		if p.current == nil {
			p.current = &patchFile{}
		}
		p.current.OldPath = parsePatchPath(strings.TrimPrefix(line, "--- "))
		return nil
	case strings.HasPrefix(line, "+++ "):
		if p.current == nil {
			p.current = &patchFile{}
		}
		p.current.NewPath = parsePatchPath(strings.TrimPrefix(line, "+++ "))
		return nil
	case strings.HasPrefix(line, "@@ "):
		return p.hunkHeader(line)
	}

	if !p.inHunk {
		return nil
	}

	switch line[0] {
	case patchChangeContext:
		p.appendLine(patchChangeContext, p.oldLine, p.newLine)
		p.oldLine++
		p.newLine++
	case patchChangeAddition:
		p.appendLine(patchChangeAddition, -1, p.newLine)
		p.newLine++
	case patchChangeDeletion:
		p.appendLine(patchChangeDeletion, p.oldLine, -1)
		p.oldLine++
	case '\\':
		// "\ No newline at end of file"
	}
	return nil
}

func (p *patchParser) appendLine(origin byte, oldLine, newLine int) {
	if p.current == nil {
		return
	}
	p.current.Lines = append(p.current.Lines, patchLine{
		Origin:  origin,
		OldLine: oldLine,
		NewLine: newLine,
		HunkNum: p.hunkNum,
	})
}

func (p *patchParser) hunkHeader(line string) error {
	// @@ -oldStart[,oldCount] +newStart[,newCount] @@ optional heading
	end := strings.Index(line[3:], " @@")
	if end < 0 {
		return fmt.Errorf("unexpected hunk header %q", line)
	}
	fields := strings.SplitN(line[3:3+end], " ", 2)
	if len(fields) != 2 {
		return fmt.Errorf("unexpected hunk header %q", line)
	}
	oldStart, err := parseHunkRange(fields[0])
	if err != nil {
		return fmt.Errorf("unexpected hunk header %q: %w", line, err)
	}
	newStart, err := parseHunkRange(fields[1])
	if err != nil {
		return fmt.Errorf("unexpected hunk header %q: %w", line, err)
	}
	p.oldLine = oldStart
	p.newLine = newStart
	p.hunkNum++
	p.inHunk = true
	return nil
}

func parseHunkRange(value string) (int, error) {
	value = strings.TrimPrefix(value, "-")
	value = strings.TrimPrefix(value, "+")
	if idx := strings.IndexByte(value, ','); idx >= 0 {
		value = value[:idx]
	}
	start, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return start, nil
}

func (p *patchParser) endFile() {
	if p.current != nil {
		// Report the same path on both sides for additions and deletions, so the
		// commit file and line change rows always name the file.
		if p.current.OldPath == "" {
			p.current.OldPath = p.current.NewPath
		}
		if p.current.NewPath == "" {
			p.current.NewPath = p.current.OldPath
		}
		p.commit.Files = append(p.commit.Files, *p.current)
		p.current = nil
	}
	p.inHunk = false
	p.hunkNum = 0
}

// parsePatchPath strips the a/ or b/ prefix and unquotes git's C-style path
// quoting. /dev/null becomes the empty path.
func parsePatchPath(value string) string {
	if value == "/dev/null" {
		return ""
	}
	return stripGitPathPrefix(unquoteGitPath(value))
}

// parseDiffGitPaths extracts both paths from "a/<path> b/<path>".
//
// Without rename detection both sides are the same path, so finding the split
// point is unambiguous. Quoted paths are parsed as two quoted tokens.
func parseDiffGitPaths(value string) (string, string, error) {
	if strings.HasPrefix(value, `"`) {
		first, rest, err := readQuotedGitPath(value)
		if err != nil {
			return "", "", fmt.Errorf("invalid diff header %q: %w", value, err)
		}
		rest = strings.TrimPrefix(rest, " ")
		second, _, err := readQuotedGitPath(rest)
		if err != nil {
			return "", "", fmt.Errorf("invalid diff header %q: %w", value, err)
		}
		return stripGitPathPrefix(first), stripGitPathPrefix(second), nil
	}
	for i := 2; i+3 <= len(value); i++ {
		if value[i:i+3] != " b/" {
			continue
		}
		oldPath := value[2:i]
		newPath := value[i+3:]
		if oldPath == newPath {
			return oldPath, newPath, nil
		}
	}
	return "", "", fmt.Errorf("invalid diff header %q", value)
}

func stripGitPathPrefix(value string) string {
	if len(value) > 2 && (value[0] == 'a' || value[0] == 'b') && value[1] == '/' {
		return value[2:]
	}
	return value
}
func readQuotedGitPath(value string) (string, string, error) {
	if !strings.HasPrefix(value, `"`) {
		return "", value, fmt.Errorf("expected a quoted path")
	}
	for i := 1; i < len(value); i++ {
		if value[i] == '\\' {
			i++
			continue
		}
		if value[i] == '"' {
			return unquoteGitPath(value[:i+1]), value[i+1:], nil
		}
	}
	return "", value, fmt.Errorf("unterminated quoted path")
}

// unquoteGitPath decodes git's C-style quoted paths, e.g. "a/\303\274ber".
func unquoteGitPath(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	var out strings.Builder
	for i := 1; i < len(value)-1; i++ {
		char := value[i]
		if char != '\\' {
			out.WriteByte(char)
			continue
		}
		i++
		if i >= len(value)-1 {
			break
		}
		switch value[i] {
		case 'a':
			out.WriteByte('\a')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'v':
			out.WriteByte('\v')
		case '\\', '"':
			out.WriteByte(value[i])
		case '0', '1', '2', '3', '4', '5', '6', '7':
			end := i + 3
			if end > len(value)-1 {
				end = len(value) - 1
			}
			code, err := strconv.ParseUint(value[i:end], 8, 8)
			if err != nil {
				out.WriteByte(value[i])
				continue
			}
			out.WriteByte(byte(code))
			i = end - 1
		default:
			out.WriteByte(value[i])
		}
	}
	return out.String()
}
