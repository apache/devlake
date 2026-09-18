<!--
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements. See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# GraphQL crash-resume coverage (#9142)

## Reproduce

Run `bash backend/test/graphql-resume-crash.sh` from the repository root.
For PostgreSQL, run `DEVLAKE_CRASH_DRIVER=postgres bash backend/test/graphql-resume-crash.sh`.

Only Docker is required. The script uses Go 1.26, MySQL 8.4.11 or PostgreSQL
18.4, and generates missing mocks with mockery v3.7.4. Test binaries enable
the race detector. Databases are disposable, use no published ports, and
only containers created by the script are removed. Logs remain in the
temporary directory printed on exit. Upstream HTTP responses are fixtures;
no production database or live GitHub/Linear endpoint is modified.

The local validation on September 18, rebased onto upstream main at
`a0cc75183badb3d7b370942c96ea563fba26bb7b`, passed 36 checks per database and
14 SIGKILLs per database (exit 137, not OOM). Eight related packages passed
`go test -race -count=1`. These are local results, not a claim that the
GitHub Actions jobs have passed.

## Audit matrix

| Area | Implementation and evidence |
| --- | --- |
| Actual Jobs path | Production queue/runner executes CollectJobs then ExtractJobs in batch, page, and extractor-restart scenarios. |
| Batch members and tail | Three inputs with batch size two; verify every Job ID, RunID, RepoId, ConnectionId and Conclusion, including the final singleton batch. |
| Multiple rows/pages/crashes | Fail the second SQL row of a page; collect three pages per input; SIGKILL the same Jobs task twice. |
| Collector registration | Test automatic checkpoint indices; actual PR/Issue collectors exercise two nested collectors. |
| Nested collectors | A completes, B fails, then resume; preserve A's raw rows and verify another rerun does not duplicate them. |
| Incremental watermark | LatestSuccessStart remains anchored to the original Task.BeganAt. Incremental runs retain existing raw records. |
| Crash windows | SIGKILL during initialization, an intermediate/final page transaction, completion transaction, and after collection/before extraction. Inject state-manager and runner-status write failures separately. |
| Recovery from errors | Recreate collectors after SQL/parser failures and lost commit acknowledgements; compare exact raw IDs. |
| Iterators | Real SQL cursors; Rows.Err, Fetch/Close/marshal failures, empty input, duplicate input, and cancellation. Validate/stage the input before modifying raw data. |
| Page boundaries | Reject nil/error pageInfo and empty/unchanged next cursors. Cover ErrFinishCollect with and without records. |
| Partial GraphQL errors | Unknown errors cannot complete durable collection, even with IgnoreQueryErrors=true. |
| Async cancellation | Cover HTTP/retry/rate-limit/NextTick cancellation; multi-input Jobs crash tests run with the race detector. |
| API and runner | RESUME_PIPELINES=true preserves task identity; false marks interrupted work FAILED and the rerun API creates a new task. Skip completed collection when resuming extraction. |
| Task-less compatibility | Full/incremental Execute, empty input, and ErrFinishCollect on real databases; no durable checkpoints for callers without task IDs. |
| Identity and interference | Isolate table/params/subtask/index; reject overlapping raw scopes, changed input/order/page or batch settings, changed executable/plugin code, and tasks superseded by newer work. |
| Database failures | Initialization/completion delete/save failures; partial raw writes; checkpoint/commit failures; successful commit with lost acknowledgement. Raw and checkpoint updates share a transaction. |
| Database/migration compatibility | Run the same suite on MySQL/PostgreSQL, including production migrations and restart with existing data. Compare migration-snapshot and runtime-model field types/tags. |
| Other callers | Interrupt and resume real GitHub CollectPrs/CollectIssues and Linear CollectIssues. Completed list work is not repeated; unfinished detail/page work resumes. |
| Assertions | Compare raw IDs, all batch members, requests against committed pages, extracted Job fields, task/subtask uniqueness, and pipeline progress rather than counts alone. |
| Continuous validation | Add a MySQL/PostgreSQL matrix to the unit-test workflow and preserve logs as artifacts. |
| Hot-path cost | Pure parsers add the inserted row count without scanning all accumulated raw rows per page. Recount only for transactional parsers that may delete rows. Test append, empty, deletion, and failed-commit accounting. |

Additional fixes covered by these tests:

- Distinguish SQL cursor failure from EOF.
- Do not record success FinishedAt on subtask error/panic; propagate status-write failures.
- Do not create duplicate subtask rows after restart.
- Run Issue parser cleanup inside the raw/checkpoint transaction.
- Delete checkpoints through both scope-deletion helpers; test table discovery and deletion predicates.
- Reject raw-count mismatch instead of silently trusting stale checkpoints.
- Stabilize Jobs ordering for identical update timestamps and order PR/Issue/Account/Linear child inputs.
- Synchronize the existing Queue test's completion flags with channels so race validation is meaningful.

## Resume contract and limitations

- Resume requires the same persisted task, code, input and page/batch settings,
  with its database intact. Detected changes fail safely; use a new task (rerun)
  instead of reusing an incompatible checkpoint.
- Input payloads are staged to a temporary file. Fixed-size duplicate-detection
  hashes are retained in memory for each input.
- Raw integrity checking compares counts. It is not a checksum capable of
  detecting manual same-count rewrites.
- This does not promise a remote GitHub/Linear snapshot: cursors may expire and
  upstream result sets may change. Upstream errors leave checkpoints intact
  rather than reporting success.
- Arbitrary environment/connection settings are not all fingerprinted. Changing
  endpoints or collection conditions between attempts requires a new task.
- Local fixtures do not replace live-service load testing or testing every
  combination of every plugin. The per-attempt subtask progress display is not
  restored to a cumulative pre-crash page count; persisted cursors/raw data,
  not that UI counter, determine what is refetched.
