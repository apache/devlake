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

package core

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	modulePrefix = "github.com/apache/devlake/"
	pluginPrefix = modulePrefix + "plugins/"
)

// frameworkRoots are the packages that must stay plugin-agnostic.
//
// The dependency direction is: plugins depend on core and helpers; core,
// helpers and server never depend on a concrete plugin. Plugins reach the
// framework through the interfaces in core/plugin, and the framework reaches
// plugins only through those same interfaces, resolved by name at runtime.
//
// server/ is in the list because it is the application, not the framework: it
// may link the generated registry (see allowedEdges) but must never reach for an
// individual plugin.
//
// The rule matters more now that plugins are compiled into the server binary by
// default: one framework→plugin import links that plugin, and everything it
// imports, into the core payload, defeats subset builds, and creates an import
// cycle for any plugin that needs the framework package doing the importing.
var frameworkRoots = []string{
	modulePrefix + "core/...",
	modulePrefix + "helpers/...",
	modulePrefix + "server/...",
}

// allowedEdges holds the only framework→plugin edges that are tolerated,
// keyed by the exact importing package. Every entry is a layering exception
// that has to be argued for; keep this map as close to empty as possible.
var allowedEdges = map[string]string{
	// The composition root links the generated plugin registry, which
	// blank-imports every plugin. It is the one place that is allowed to know
	// that plugins exist as concrete packages. It must never import a single
	// plugin directly, and no other package under server/ gets the same
	// privilege.
	modulePrefix + "server": modulePrefix + "plugins/all",
}

// TestFrameworkDoesNotImportPlugins fails when anything under core/, helpers/ or
// server/ imports a plugin package, including from _test.go files.
//
// Tests count because a test that needs a plugin is a design smell: it usually
// means the framework helper under test is only exercised through one concrete
// plugin's fixtures. Move the test next to that plugin instead, or give the
// helper a plugin-independent fixture.
func TestFrameworkDoesNotImportPlugins(t *testing.T) {
	args := []string{
		"list",
		"-f", `{{.ImportPath}}|{{join .Imports " "}}|{{join .TestImports " "}}|{{join .XTestImports " "}}`,
	}
	args = append(args, frameworkRoots...)

	out, err := exec.Command("go", args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("go list failed: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("go list failed: %v", err)
	}

	seen := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 4 {
			t.Fatalf("unexpected go list output line %q", line)
		}
		importer := fields[0]
		for _, group := range fields[1:] {
			for _, imported := range strings.Fields(group) {
				if !strings.HasPrefix(imported, pluginPrefix) {
					continue
				}
				if allowedEdges[importer] == imported {
					continue
				}
				seen[importer+" imports "+imported] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		trackFrameworkSources(t)
		return
	}

	violations := make([]string, 0, len(seen))
	for v := range seen {
		violations = append(violations, v)
	}
	sort.Strings(violations)

	t.Fatalf("the framework must not import plugins, found %d edge(s):\n\t%s",
		len(violations), strings.Join(violations, "\n\t"))
}

// trackFrameworkSources registers every framework source file with the test log.
//
// The test cache only knows about the package under test, not about the other
// packages this test inspects through `go list`. Without this, a plain
// `go test ./core` would replay a cached PASS after someone adds a plugin
// import in helpers/ or server/. Opening the files makes them part of the cache
// entry, so any change to them invalidates it.
func trackFrameworkSources(t *testing.T) {
	t.Helper()

	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return
	}
	moduleDir := filepath.Dir(strings.TrimSpace(string(out)))

	for _, dir := range []string{"core", "helpers", "server"} {
		_ = filepath.WalkDir(filepath.Join(moduleDir, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Best effort: the cache entry only needs the file to be opened.
			_, _ = os.ReadFile(path)
			return nil
		})
	}
}
