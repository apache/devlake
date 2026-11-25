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
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func issueRows(prefix string, n int) []string {
	rows := []string{issueHeader}
	for i := 1; i <= n; i++ {
		rows = append(rows, fmt.Sprintf("%s:%d,https://example.com/%s/%d,%d,title,BUG,new,2024-01-01 00:00:00", prefix, i, prefix, i, i))
	}
	return rows
}

// An import that is still reading its file must not block writes to the issues and board links
// of other boards. This guards the lock scope of the import transaction, not the original bug:
// it also passes on the code before the fix, which used no transaction.
func TestImportDoesNotLockOtherBoards(t *testing.T) {
	dataflowTester, svc := newImportTester(t)
	if dataflowTester.Dal.Dialect() != "mysql" {
		t.Skip("checks InnoDB locks, and uses innodb_lock_wait_timeout")
	}
	err := svc.ImportIssue("board-a", csvFile(issueRows("a", 3)...), false)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.ImportIssue("board-b", csvFile(issueRows("b", 3)...), false)
	if err != nil {
		t.Fatal(err)
	}

	reader, writer := io.Pipe()
	// Close the pipe even if the test fails, so that the import releases the import lock.
	t.Cleanup(func() { _ = writer.CloseWithError(io.ErrUnexpectedEOF) })
	done := make(chan error, 1)
	go func() { done <- svc.ImportIssue("board-a", reader, false) }()
	// No sleep is needed: Write returns only after the reader has consumed the bytes. The CSV
	// reader first reads in NewCsvFileIteratorFromFile, which runs after the deletes, and the
	// header plus one row is smaller than its 4096-byte buffer, so one read consumes them.
	_, writeErr := writer.Write([]byte(strings.Join(issueRows("a", 1), "\n") + "\n"))
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	statements := []string{
		"UPDATE issues SET title = 'changed' WHERE id = 'b:1'",
		"INSERT INTO board_issues (board_id, issue_id) VALUES ('board-b', 'b:0')",
		"INSERT INTO board_issues (board_id, issue_id) VALUES ('board-b', 'b:9')",
		"INSERT INTO board_issues (board_id, issue_id) VALUES ('board-a2', 'z:1')",
	}
	tx := dataflowTester.Db.Begin()
	setErr := tx.Exec("SET innodb_lock_wait_timeout = 2").Error
	if setErr != nil {
		t.Fatal(setErr)
	}
	var statementErrs []error
	for _, statement := range statements {
		statementErrs = append(statementErrs, tx.Exec(statement).Error)
	}
	tx.Rollback()
	closeErr := writer.Close()
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if importErr := <-done; importErr != nil {
		t.Fatal(importErr)
	}
	for i, statementErr := range statementErrs {
		if statementErr != nil {
			t.Errorf("an import of board-a must not lock %q: %v", statements[i], statementErr)
		}
	}
}

// Two imports that save the same authors in a different order must both succeed.
// This guards the import lock, not the original bug: it also passes on the code before the fix,
// which used no transaction. Without the import lock, the two transactions deadlock on accounts.
func TestConcurrentImportsWithSharedAuthors(t *testing.T) {
	_, svc := newImportTester(t)
	err := svc.ImportIssue("board-a", csvFile(issueRows("a", 1)...), false)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.ImportIssue("board-b", csvFile(issueRows("b", 1)...), false)
	if err != nil {
		t.Fatal(err)
	}
	changelogs := func(issueId string, reverse bool) []string {
		rows := []string{"id,issue_id,author_name,field_name,original_from_value,original_to_value,created_date"}
		for i := 0; i < 200; i++ {
			author := i % 40
			if reverse {
				author = 39 - author
			}
			rows = append(rows, fmt.Sprintf("%s-%d,%s,author%d,status,Open,Done,2024-01-01 00:00:00", issueId, i, issueId, author))
		}
		return rows
	}
	for round := 0; round < 3; round++ {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, issueId := range []string{"a:1", "b:1"} {
			wg.Add(1)
			go func(i int, issueId string) {
				defer wg.Done()
				board := "board-" + issueId[:1]
				if err := svc.ImportIssueChangelog(board, csvFile(changelogs(issueId, i == 1)...), true); err != nil {
					errs[i] = err
				}
			}(i, issueId)
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
	}
}
