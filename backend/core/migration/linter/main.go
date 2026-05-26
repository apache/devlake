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

package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

var moduleName = ""

func init() {
	// prepare the module name
	line := firstLineFromFile("go.mod")
	moduleName = strings.Split(line, " ")[1]
}

func firstLineFromFile(path string) string {
	inFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer inFile.Close()

	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		return scanner.Text()
	}
	panic(fmt.Errorf("empty file: %s", path))
}

const (
	LINT_ERROR   = "error"
	LINT_WARNING = "warning"
)

type LintMessage struct {
	Level  string
	File   string
	Line   int
	Col    int
	EndCol int
	Title  string
	Msg    string
}

func lintMigrationScript(file string, allowedPkgs map[string]bool) []LintMessage {
	msgs := make([]LintMessage, 0)
	src, err := os.ReadFile(file)
	if err != nil {
		msgs = append(msgs, LintMessage{
			Level: LINT_ERROR,
			File:  file,
			Title: "Error reading file",
			Msg:   err.Error(),
		})
		return msgs
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		msgs = append(msgs, LintMessage{
			Level: LINT_ERROR,
			File:  file,
			Title: "Error parsing file",
			Msg:   err.Error(),
		})
		return msgs
	}
	// ast.Print(fset, f)
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			importedPkgName, err := strconv.Unquote(x.Path.Value)
			if err != nil {
				panic(err)
			}
			// it is ok to use subpackages
			filePkgName := path.Join(moduleName, path.Dir(file))
			if strings.HasPrefix(importedPkgName, filePkgName) {
				return true
			}
			// it is ok to use external libs, their behaviors are considered stable
			if !strings.HasPrefix(importedPkgName, moduleName) {
				return true
			}
			// it is ok if the package is whitelisted
			if allowedPkgs[importedPkgName] {
				return true
			}
			// we have a problem
			// migration scripts are Immutable, meaning their behaviors should not be changed over time
			// Relying on other packages may break the constraint and cause unexpected side-effects.
			// You may add the package to the whitelist by the -a option if you are sure it is OK
			pos := fset.Position(n.Pos())
			msgs = append(msgs, LintMessage{
				Level:  LINT_WARNING,
				File:   file,
				Title:  "Package not allowed",
				Msg:    fmt.Sprintf("%s imports forbidden package %s", file, x.Path.Value),
				Line:   pos.Line,
				Col:    pos.Column,
				EndCol: pos.Column + len(x.Path.Value),
			})
		}
		return true
	})
	return msgs
}

func main() {
	cmd := &cobra.Command{Use: "migration script linter"}
	prefix := cmd.Flags().StringP("prefix", "p", "", "path prefix if your go.mod resides in a subfolder")
	allowedPkg := cmd.Flags().StringArrayP(
		"allowed-pkg",
		"a",
		[]string{
			"github.com/apache/incubator-devlake/core/config",
			"github.com/apache/incubator-devlake/core/context",
			"github.com/apache/incubator-devlake/core/dal",
			"github.com/apache/incubator-devlake/core/errors",
			"github.com/apache/incubator-devlake/helpers/migrationhelper",
			"github.com/apache/incubator-devlake/core/models/migrationscripts/archived",
			"github.com/apache/incubator-devlake/core/plugin",
			"github.com/apache/incubator-devlake/helpers/pluginhelper/api",
		},
		"package that allowed to be used in a migration script. e.g.: github.com/apache/incubator-devlake/core/context",
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		allowedPkgs := make(map[string]bool, len(*allowedPkg))
		for _, p := range *allowedPkg {
			allowedPkgs[p] = true
		}
		warningTpl, err := template.New("warning").Parse("::warning file={{ .File }},line={{ .Line }},col={{ .Col }},endColumn={{ .EndCol }}::{{ .Msg }}")
		if err != nil {
			panic(err)
		}
		errorTpl, err := template.New("error").Parse("::error file={{ .File }},line={{ .Line }},endLine={{ .Col }},title={{ .Title }}::{{ .Msg }}")
		if err != nil {
			panic(err)
		}
		localTpl, err := template.New("local").Parse("{{ .Level }}: {{ .Msg }}\n\t{{ .File }}:{{ .Line }}:{{ .Col }}")
		if err != nil {
			panic(err)
		}
		exitCode := 0
		for _, file := range args {
			msgs := lintMigrationScript(file, allowedPkgs)
			if len(msgs) == 0 {
				continue
			}
			for _, msg := range msgs {
				var tpl *template.Template
				if *prefix != "" {
					// github actions need root relative path for annotation to show up in the PR
					msg.File = path.Join(*prefix, file)
					tpl = errorTpl
					if msg.Level == LINT_WARNING {
						tpl = warningTpl
					}
				} else {
					// we are running locally in the `backend` folder, use another format to make fixing easier
					tpl = localTpl
				}
				err = tpl.Execute(os.Stderr, msg)
				if err != nil {
					panic(err)
				}
				os.Stderr.WriteString("\n")
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	}
	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
