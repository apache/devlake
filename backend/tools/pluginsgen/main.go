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

// Command pluginsgen generates plugins/all, the package that links the Go
// plugins into the server binary.
//
// It scans plugins/*/impl for the type implementing plugin.PluginMeta and emits
// one import plus one plugin.MustRegister call per plugin. Keeping the registry
// generated means adding a plugin needs no edit to a shared list, and CI can
// assert that the checked-in file matches the tree.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const modulePath = "github.com/apache/devlake"

// licenseHeader is emitted verbatim; license-eye checks generated files too.
const licenseHeader = `/*
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

`

// pluginMetaMethods are the methods declared by plugin.PluginMeta. Requiring
// all of them keeps the scan from picking an unrelated Name method declared on
// some other type in the same package.
var pluginMetaMethods = []string{"Name", "Description", "RootPkgPath"}

type plugin struct {
	name  string // directory name, e.g. gh-copilot
	alias string // import alias, e.g. gh_copilot
	typ   string // type implementing plugin.PluginMeta
}

func main() {
	root := flag.String("root", ".", "backend module root")
	out := flag.String("o", "plugins/all/all.go", "output file, relative to root")
	subset := flag.String("plugins", os.Getenv("DEVLAKE_PLUGINS"), "comma-separated plugin names to include (default: $DEVLAKE_PLUGINS, otherwise all)")
	flag.Parse()

	if err := run(*root, *out, *subset); err != nil {
		fmt.Fprintf(os.Stderr, "pluginsgen: %v\n", err)
		os.Exit(1)
	}
}

func run(root, out, subset string) error {
	plugins, err := scan(root, subset)
	if err != nil {
		return err
	}

	src, err := render(plugins)
	if err != nil {
		return err
	}

	target := out
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, out)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, src, 0o644)
}

// scan returns the plugins to link, sorted by name.
func scan(root, subset string) ([]plugin, error) {
	dirs, err := filepath.Glob(filepath.Join(root, "plugins", "*", "impl"))
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no plugins found under %s/plugins/*/impl", root)
	}

	selected, explicit := parseSubset(subset)

	var plugins []plugin
	for _, dir := range dirs {
		name := filepath.Base(filepath.Dir(dir))
		if !selected(name) {
			continue
		}
		typ, err := pluginMetaType(dir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		plugins = append(plugins, plugin{
			name:  name,
			alias: strings.ReplaceAll(name, "-", "_"),
			typ:   typ,
		})
	}

	// An explicit selection may legitimately match nothing ("none" builds a
	// core with no compiled-in plugins); an unfiltered scan must not.
	if len(plugins) == 0 && !explicit {
		return nil, fmt.Errorf("no plugin found under %s/plugins/*/impl", root)
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].name < plugins[j].name })
	return plugins, nil
}

// parseSubset builds the selection predicate from DEVLAKE_PLUGINS. An empty
// value selects every plugin; "none" selects none, mirroring build-plugins.sh.
// The second result reports whether a selection was requested at all.
func parseSubset(subset string) (func(string) bool, bool) {
	subset = strings.TrimSpace(subset)
	switch subset {
	case "":
		return func(string) bool { return true }, false
	case "none":
		return func(string) bool { return false }, true
	}

	wanted := map[string]struct{}{}
	for _, name := range strings.Split(subset, ",") {
		if name = strings.TrimSpace(name); name != "" {
			wanted[name] = struct{}{}
		}
	}
	return func(name string) bool {
		_, ok := wanted[name]
		return ok
	}, true
}

// pluginMetaType finds the type in dir whose method set covers
// plugin.PluginMeta.
func pluginMetaType(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	methods := map[string]map[string]struct{}{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return "", err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			receiver := receiverName(fn.Recv.List[0].Type)
			if receiver == "" {
				continue
			}
			if methods[receiver] == nil {
				methods[receiver] = map[string]struct{}{}
			}
			methods[receiver][fn.Name.Name] = struct{}{}
		}
	}

	var found []string
	for receiver, declared := range methods {
		implements := true
		for _, method := range pluginMetaMethods {
			if _, ok := declared[method]; !ok {
				implements = false
				break
			}
		}
		if implements {
			found = append(found, receiver)
		}
	}

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("no type implements plugin.PluginMeta")
	default:
		sort.Strings(found)
		return "", fmt.Errorf("several types implement plugin.PluginMeta: %s", strings.Join(found, ", "))
	}
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.IndexExpr: // generic receiver, e.g. Foo[T]
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	}
	return ""
}

func render(plugins []plugin) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(licenseHeader)
	buf.WriteString("//go:build !devlake_external_plugins\n\n")
	buf.WriteString("// Code generated by tools/pluginsgen. DO NOT EDIT.\n\n")
	buf.WriteString("// Package all links the Go plugins into the server binary.\n")
	buf.WriteString("//\n")
	buf.WriteString("// Importing this package registers every plugin listed below, so the plugin\n")
	buf.WriteString("// set becomes a build-time decision. Build with the devlake_external_plugins\n")
	buf.WriteString("// tag to leave it empty and load plugins from PLUGIN_DIR as shared objects\n")
	buf.WriteString("// instead.\n")
	buf.WriteString("package all\n\n")

	if len(plugins) == 0 {
		src, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("formatting generated source: %w", err)
		}
		return src, nil
	}

	buf.WriteString("import (\n")
	buf.WriteString("\t\"github.com/apache/devlake/core/plugin\"\n\n")
	for _, p := range plugins {
		fmt.Fprintf(&buf, "\t%s \"%s/plugins/%s/impl\"\n", p.alias, modulePath, p.name)
	}
	buf.WriteString(")\n\n")
	buf.WriteString("func init() {\n")
	for _, p := range plugins {
		fmt.Fprintf(&buf, "\tplugin.MustRegister(%s.%s{})\n", p.alias, p.typ)
	}
	buf.WriteString("}\n")

	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting generated source: %w", err)
	}
	return src, nil
}
