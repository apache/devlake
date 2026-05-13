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

package unithelper

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	fs2 "io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
)

type TableInfoChecker struct {
	tables               map[string]struct{}
	tablePrefix          string
	ignoredTables        []string
	count                int
	validatePluginsCount bool
	ignoredPackages      []string
}

type TableInfoCheckerConfig struct {
	TablePrefix         string
	ValidatePluginCount bool
	IgnoreTables        []string
}

type parsedPackage struct {
	name  string
	files map[string]*ast.File
}

func NewTableInfoChecker(cfg TableInfoCheckerConfig) *TableInfoChecker {
	return &TableInfoChecker{
		tables:               make(map[string]struct{}),
		tablePrefix:          cfg.TablePrefix,
		validatePluginsCount: cfg.ValidatePluginCount,
		ignoredTables:        cfg.IgnoreTables,
		ignoredPackages:      []string{"migrationscripts", "archived"},
	}
}

// The FeedIn function iterates through the model definitions,
// identifies all the tables by parsing the source files,
// and then compares them with the output obtained from the GetTablesInfo function.
// If the plugin has no models directory, pass in the root directory of the plugin
func (checker *TableInfoChecker) FeedIn(modelsDir string, f func() []dal.Tabler, additionalIgnorablePackages ...string) {
	checker.count++
	packs, err := checker.parseDirRecursively(modelsDir, additionalIgnorablePackages...)
	if err != nil {
		panic(err)
	}
	funcs := checker.getTableNameFuncs(packs)
	for _, fun := range funcs {
		s := checker.getTableName(fun)
		if s == "" {
			continue //exclude models whose tables are not declared as constants
		}
		if strings.HasPrefix(s, checker.tablePrefix) {
			checker.tables[s] = struct{}{}
		}
	}
	for _, tb := range checker.ignoredTables {
		delete(checker.tables, tb)
	}
	for _, tabler := range f() {
		delete(checker.tables, tabler.TableName())
	}
}

func (checker *TableInfoChecker) Verify() errors.Error {
	if checker.validatePluginsCount {
		err := checker.ensureCoverage()
		if err != nil {
			return err
		}
	}
	for _, tb := range checker.ignoredTables {
		delete(checker.tables, tb)
	}
	if len(checker.tables) == 0 {
		return nil
	}
	tableNames := make([]string, 0, len(checker.tables))
	sb := strings.Builder{}
	_, _ = sb.WriteString("The following tables are not returned by the TablesInfo method\n")
	for t := range checker.tables {
		tableNames = append(tableNames, t)
	}
	// sort the table names so that the tables of the same plugin are together
	sort.Strings(tableNames)
	_, _ = sb.WriteString(strings.Join(tableNames, "\n"))
	return errors.Default.New(sb.String())
}

func (checker *TableInfoChecker) parseDirRecursively(modelsDir string, additionalIgnorablePackages ...string) (map[string]*parsedPackage, error) {
	packagesMap := make(map[string]*parsedPackage)
	ignorablePackages := append(checker.ignoredPackages, additionalIgnorablePackages...)
	err := filepath.WalkDir(modelsDir, func(path string, d fs2.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		packs, err := parsePackagesInDir(path, 0)
		if err != nil {
			return err
		}
		for packageName, packageObj := range packs {
			if slices.Contains(ignorablePackages, packageName) {
				return fs2.SkipDir
			}
			if _, ok := packagesMap[packageName]; ok {
				return fmt.Errorf("package %s is duplicated across directories", packageName)
			}
			packagesMap[packageName] = packageObj
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return packagesMap, nil
}

func parsePackagesInDir(dir string, mode parser.Mode) (map[string]*parsedPackage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	packs := make(map[string]*parsedPackage)
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		filename := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fileSet, filename, nil, mode)
		if err != nil {
			return nil, err
		}
		packageName := file.Name.Name
		pack := packs[packageName]
		if pack == nil {
			pack = &parsedPackage{
				name:  packageName,
				files: make(map[string]*ast.File),
			}
			packs[packageName] = pack
		}
		pack.files[filename] = file
	}
	return packs, nil
}

func (checker *TableInfoChecker) getTableNameFuncs(pks map[string]*parsedPackage) []*ast.FuncDecl {
	var funcs []*ast.FuncDecl
	for _, pack := range pks {
		for _, f := range pack.files {
			for _, d := range f.Decls {
				if fn, isFn := d.(*ast.FuncDecl); isFn && fn.Name.String() == "TableName" {
					funcs = append(funcs, fn)
				}
			}
		}
	}
	return funcs
}

func (checker *TableInfoChecker) getTableName(fn *ast.FuncDecl) string {
	for _, stmt := range fn.Body.List {
		if rs, isReturn := stmt.(*ast.ReturnStmt); isReturn {
			if lit, ok := rs.Results[0].(*ast.BasicLit); ok {
				return strings.Trim(lit.Value, `"`)
			}
		}
	}
	return ""
}

func (checker *TableInfoChecker) ensureCoverage() errors.Error {
	packagesFound := 0
	err := filepath.WalkDir(".", func(path string, d fs2.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." || !d.IsDir() {
			return nil
		}
		if strings.Count(path, string(os.PathSeparator)) != 0 {
			return fs2.SkipDir
		}
		packs, err := parsePackagesInDir(path, parser.PackageClauseOnly)
		if err != nil {
			return err
		}
		for _, pk := range packs {
			if pk.name == "main" {
				packagesFound++
				return fs2.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return errors.Default.WrapRaw(err)
	}
	if checker.count != packagesFound {
		return errors.Default.New(fmt.Sprintf("Number of actual plugins (%d) and tested plugins (%d) don't match", packagesFound, checker.count))
	}
	return nil
}
