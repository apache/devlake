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
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// gitRunner runs read-only git commands against one local repository.
//
// Output and Stream both run the git command in a way that never writes to the
// repository and never needs a working tree, so bare clones are supported.
type gitRunner struct {
	dir string
}

func newGitRunner(dir string) *gitRunner {
	return &gitRunner{dir: dir}
}

// Output runs git and returns stdout. Stderr is reported in the error.
func (r *gitRunner) Output(ctx context.Context, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := r.command(ctx, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("git %s failed: %s", strings.Join(sanitizeArgs(args), " "), errorMessage(stderr.Bytes(), err))
	}
	return stdout.Bytes(), nil
}

// OutputLines runs git and returns its stdout lines with the trailing newline
// removed from each line.
func (r *gitRunner) OutputLines(ctx context.Context, args ...string) ([]string, error) {
	out, err := r.Output(ctx, args...)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// Stream runs git and returns its stdout. The caller must read to EOF and then
// call the returned wait function; closing the reader early kills git.
func (r *gitRunner) Stream(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
	cmd := r.command(ctx, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdout, func() error {
		err := cmd.Wait()
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("git %s failed: %s", strings.Join(sanitizeArgs(args), " "), errorMessage(stderr.Bytes(), err))
	}, nil
}

// Pipe runs git with stdin and returns a reader over its stdout.
func (r *gitRunner) Pipe(ctx context.Context, stdin []byte, args ...string) (io.ReadCloser, func() error, error) {
	cmd := r.command(ctx, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdout, func() error {
		err := cmd.Wait()
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("git %s failed: %s", strings.Join(sanitizeArgs(args), " "), errorMessage(stderr.Bytes(), err))
	}, nil
}

func (r *gitRunner) command(ctx context.Context, args ...string) *exec.Cmd {
	fullArgs := append([]string{"-c", "core.quotePath=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Dir = r.dir
	cmd.Env = append(cmd.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_PAGER=cat",
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)
	return cmd
}

func errorMessage(stderr []byte, err error) string {
	if msg := strings.TrimSpace(string(stderr)); msg != "" {
		return msg
	}
	return err.Error()
}

func splitLines(out []byte) []string {
	text := strings.TrimSuffix(string(out), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func scanLines(reader io.Reader, fn func(line string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		if err := fn(scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}
