<!--
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
-->
# Apache DevLake - AI Coding Agent Instructions

Apache DevLake ingests data from DevOps tools (GitHub, GitLab, Jira, Jenkins, etc.), transforms it into standardized domain models, and enables metrics/dashboards via Grafana.

## Architecture

Three-layer data model: **Raw** (`_raw_*`) → **Tool** (`_tool_*`) → **Domain** (standardized tables in `backend/core/models/domainlayer/`).

Key components: `backend/` (Go server + plugins), `config-ui/` (React frontend), `grafana/` (dashboards).

## Owned Plugins

This is a fork of Apache DevLake. An **owned plugin** is identified by having an
`AGENTS.md` file in its directory (`backend/plugins/<name>/AGENTS.md`).
Read the plugin's `AGENTS.md` before making changes.

To add a new owned plugin, create `backend/plugins/<name>/AGENTS.md` with
build/test commands, layout, and conventions for that plugin.

## Upstream Divergence

This fork occasionally modifies files from upstream Apache DevLake. All such
modifications are tracked in [`docs/upstream-diffs.md`](docs/upstream-diffs.md).

**Before modifying any file outside an owned plugin directory:**

1. Check `docs/upstream-diffs.md` — if the file is listed, follow the noted constraints.
2. If introducing a new divergence, add an entry to `docs/upstream-diffs.md` with:
   files changed, reason, upstream status, upstream PR link, owner, and rebase notes.
3. Use `Upstream-Status:` trailer in commit messages for upstream-divergent changes.

## Quick Reference

```bash
make build            # Build plugins + server
make dev              # Build + run
make unit-test        # Unit tests
make lint             # golangci-lint (from backend/)
```

For local environment setup (podman, MySQL, service URLs): see [docs/local-dev.md](docs/local-dev.md).

## Common Pitfalls

- Forgetting to add models to `GetTablesInfo()` fails `plugins/table_info_test.go`
- Migration scripts must be added to `All()` in `migrationscripts/register.go`
- API changes require `make swag` to update Swagger docs
- No cross-plugin imports (`plugins/X` must not import `plugins/Y`)
- Apache 2.0 license header required on all new `.go` files
