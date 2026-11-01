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

## Project Overview
Apache DevLake is a dev data platform that ingests data from DevOps tools (GitHub, GitLab, Jira, Jenkins, etc.), transforms it into standardized domain models, and enables metrics/dashboards via Grafana.

## Architecture

### Three-Layer Data Model
1. **Raw Layer** (`_raw_*` tables): JSON data collected from APIs, stored for replay/debugging
2. **Tool Layer** (`_tool_*` tables): Plugin-specific models extracted from raw data
3. **Domain Layer** (standardized tables): Normalized models in [backend/core/models/domainlayer/](backend/core/models/domainlayer/) - CODE, TICKET, CICD, CODEREVIEW, CODEQUALITY, CROSS

### Key Components
- **backend/**: Go server + plugins (main codebase)
- **backend/python/**: Python plugin framework via RPC
- **config-ui/**: React frontend (TypeScript, Vite, Ant Design)
- **grafana/**: Dashboard definitions

## Plugin Development (Go)

### Plugin Structure
Each plugin in `backend/plugins/<name>/` follows this layout:
```
api/         # REST endpoints (connections, scopes, scope-configs)
impl/        # Plugin implementation (implements core interfaces)
models/      # Tool layer models + migrationscripts/
tasks/       # Collectors, Extractors, Converters
e2e/         # Integration tests with CSV fixtures
```

### Required Interfaces
See [backend/plugins/gitlab/impl/impl.go](backend/plugins/gitlab/impl/impl.go) for reference:
- `PluginMeta`: Name, Description, RootPkgPath
- `PluginTask`: SubTaskMetas(), PrepareTaskData()
- `PluginModel`: GetTablesInfo() - **must list all models or CI fails**
- `PluginMigration`: MigrationScripts() for DB schema evolution
- `PluginSource`: Connection(), Scope(), ScopeConfig()

### Subtask Pattern (Collector → Extractor → Converter)
```go
// 1. Register subtask in tasks/register.go via init()
func init() {
    RegisterSubtaskMeta(&CollectIssuesMeta)
}

// 2. Define dependencies for execution order
var CollectIssuesMeta = plugin.SubTaskMeta{
    Name:         "Collect Issues",
    Dependencies: []*plugin.SubTaskMeta{}, // or reference other metas
}
```

### API Collectors
- Use `helper.NewStatefulApiCollector` for incremental collection with time-based bookmarking
- See [backend/plugins/gitlab/tasks/issue_collector.go](backend/plugins/gitlab/tasks/issue_collector.go)

### Migration Scripts
- Located in `models/migrationscripts/`
- Register all scripts in `register.go`'s `All()` function
- Version format: `YYYYMMDD_description.go`

## Build & Development Commands

```bash
# From repo root
make dep              # Install Go + Python dependencies
make build            # Build plugins + server
make dev              # Build + run server
make godev            # Go-only dev (no Python plugins)
make unit-test        # Run all unit tests
make e2e-test         # Run E2E tests

# From backend/
make swag             # Regenerate Swagger docs (required after API changes)
make lint             # Run golangci-lint
```

### Running Locally
```bash
docker-compose -f docker-compose-dev.yml up mysql grafana  # Start deps
make dev                                                     # Run server on :8080
cd config-ui && yarn && yarn start                          # UI on :4000
```

## Testing

### Unit Tests
Place `*_test.go` files alongside source. Use mocks from `backend/mocks/`.

### E2E Tests for Plugins
Use CSV fixtures in `e2e/` directory. See [backend/test/helper/](backend/test/helper/) for the Go test client that can spin up an in-memory DevLake instance.

### Integration Testing
```go
helper.ConnectLocalServer(t, &helper.LocalClientConfig{
    ServerPort:   8080,
    DbURL:        "mysql://merico:merico@127.0.0.1:3306/lake",
    CreateServer: true,
    Plugins:      []plugin.PluginMeta{gitlab.Gitlab{}},
})
```

## Python Plugins
Located in `backend/python/plugins/`. Use Poetry for dependencies. See [backend/python/README.md](backend/python/README.md).

## Code Conventions
- Tool model table names: `_tool_<plugin>_<entity>` (e.g., `_tool_gitlab_issues`)
- Domain model IDs: Use `didgen.NewDomainIdGenerator` for consistent cross-plugin IDs
- All plugins must be independent - no cross-plugin imports
- Apache 2.0 license header required on all source files

## Common Pitfalls
- Forgetting to add models to `GetTablesInfo()` fails `plugins/table_info_test.go`
- Migration scripts must be added to `All()` in `register.go`
- API changes require running `make swag` to update Swagger docs
- `gitextractor` shells out to the `git` CLI; no git library or cgo is required
