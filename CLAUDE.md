# Common AI Assistant Rules

These rules apply to all repositories using centralized AI rules management.

---

## 1. Golden Rules

1. **One message = all related operations**

   - Batch reads/writes, shell commands, todos, and agent spawns in a single message.

2. **No working files in repo root**

   - Root is only for repo metadata and config: `AGENTS.md`, `CLAUDE.md`, `TODO.md`, `README.md`, `LICENSE`, `.gitignore`, dependency files, tooling configs.
   - All code, docs, tests, notes, experiments, and scripts go into subfolders.

3. **Planning lives in `TODO.md` at root**

   - Use `TODO.md` for high-level plans.
   - Keep it short and actionable.
   - Remove steps once they're done (this file should always reflect the current plan, not history).

4. **Keep instructions lean**

   - Enough structure to be consistent.
   - No walls of text; use lists and checklists.

5. **Prefer editing over creating**
   - Edit existing files rather than creating new ones when possible.
   - Only create new files when a clear responsibility or context is missing.

---

## 2. Repository Structure

### 2.1 Root (allowed files)

Root directory may contain only:

- `CLAUDE.md` – AI/human collaboration rules
- `TODO.md` – current plan / task list
- `README.md` – short project overview
- `LICENSE`, `.gitignore`, `.editorconfig`
- Tooling configs and dependency files (language-specific)
- No code, no docs content, no tests, no random notes.

### 2.2 Common subfolders

Use these by default (adapt per project, especially for language-specific alternatives):

- `src/` – production source code
- `tests/` – all tests (unit, integration, etc.)
- `docs/` – internally facing documentation
- `docs/adr/` – Architecture Decision Records (ADRs)
- `docs/research/` – research notes, content libraries, reusable frameworks
- `config/` – configuration files (YAML, JSON, env templates)
- `scripts/` or `tools/` – automation, utility scripts, and small code tools
- `examples/` – example usage code or demo projects

**Infrastructure projects** (adapt naming as needed):

- `pulumi/` – Pulumi infra code
- `kubernetes/` – manifests + Kustomize overlays

**Presentation projects**:

- `<presentation-name>/`
  - `speaker-notes.md`
  - `slides.pptx`
  - Optional `README.md`
- Shared resources under `docs/`

**Knowledge projects**

For projects exposing documentation through `mkdoc`, externally facing documentation goes to:

- `knowledge/`

---

## 3. Documentation & ADRs

### 3.1 Documentation rules

- All internal documentation files go under `docs/` or a specific subfolder.
- Avoid creating new root-level `.md` files except: `README.md`, `TODO.md`

### 3.2 ADRs (Architecture Decision Records)

- Location: `docs/adr/`
- File name: `NNNN-title-with-hyphens.md` (e.g., `0001-infrastructure-as-code-with-pulumi.md`)
- Each ADR contains: **Status**, **Context**, **Decision**, **Consequences**.
- Maintain `docs/adr/README.md` as an index.

Use an ADR when:

- Architecture significantly changes
- Tooling or core tech stack changes
- Security, reliability, or cost decisions with long-term impact

---

## 4. Naming Conventions

### 4.1 General rules

- Names should be descriptive, not cute
- Stable over time (avoid date prefixes; use ADRs for history)
- See language-specific profiles for file naming conventions

### 4.2 Internal Documentation files

- Project overviews: `docs/overview.md`, `docs/architecture.md`
- Guides: `docs/<topic>.md`
- Research: `docs/research/<topic>.md`

---

## 5. AI Assistant & Human Collaboration

### 5.1 When AI changes code

- Read relevant existing files before editing.
- Keep imports at top, respect code patterns.
- When adding features:
  - Consider security and consistency first.
  - Update docs where relevant.
  - Add or update tests.
- When fixing bugs:
  - Understand root cause first.
  - Add regression tests.

### 5.2 When AI creates docs

- Put them under `docs/` or `knowledge` depending on audience – not root.
- For presentations: slides are visual anchors, speaker notes have full narrative.

### 5.3 When humans work

- Keep `TODO.md` current.
- Use ADRs for major decisions.
- Keep repo structure clean.
- Prefer small, focused commits with clear messages.

---

## 6. Pre-Action Checklists

### 6.1 Before creating a new file

- [ ] Does an existing file already cover this responsibility?
- [ ] Am I putting it in the right folder (`src/`, `tests/`, `docs/`, etc.)?
- [ ] Is the name clear and consistent with conventions?
- [ ] Is this going to live longer than one session?

### 6.2 Before committing

- [ ] No working files in root (only allowed meta files).
- [ ] `TODO.md` is up to date (completed steps removed).
- [ ] New decisions captured in `docs/adr/` if needed.
- [ ] Tests updated or added where appropriate.
- [ ] Repo structure still matches conventions.

---

## 7. JIRA Integration (DPROD Project)

### 7.1 Issue Types

| Type         | When to Use                                     |
| ------------ | ----------------------------------------------- |
| **Epic**     | Large feature or initiative                     |
| **Story**    | User-facing functionality                       |
| **Task**     | Technical/implementation work                   |
| **Spike**    | Research, discovery, PoC with uncertain outcome |
| **Sub-task** | Breakdown of Story/Task                         |

### 7.2 Required Fields

| Field            | Field ID               | Required For       | Example                 |
| ---------------- | ---------------------- | ------------------ | ----------------------- |
| Epic Name        | `customfield_12311141` | Epic               | `"User Authentication"` |
| Epic Link        | `customfield_12311140` | Story, Task, Spike | `"DPROD-876"`           |
| Git Pull Request | `customfield_12310220` | Any (optional)     | MR/PR URL               |

### 7.3 Story Points

Story points cannot be set during issue creation. Update them after the issue is created:

```python
jira_update_issue(
    issue_key="DPROD-123",
    fields={},
    additional_fields={
        "customfield_12310243": 3  # Story Points field
    }
)
```

### 7.4 Labels

Always include `ai-generated` label when creating issues via AI:

```python
additional_fields={"labels": ["ai-generated"]}
```

### 7.5 Priority Mapping

| Priority | When to Use                                 |
| -------- | ------------------------------------------- |
| Blocker  | Blocking release or other work              |
| Critical | Production issues, security vulnerabilities |
| Major    | High-value features, P1 items               |
| Normal   | Standard work, P2 items                     |
| Minor    | Nice-to-have, P3 items                      |

### 7.6 JIRA Markup

| Markdown      | JIRA Markup   |
| ------------- | ------------- |
| `**bold**`    | `*bold*`      |
| `# Heading`   | `h1. Heading` |
| `## Heading`  | `h2. Heading` |
| `- bullet`    | `* bullet`    |
| `[text](url)` | `[text\|url]` |

---

## 8. Security

- Never commit secrets or credentials
- Validate user input at system boundaries
- Follow principle of least privilege
- Log errors with context but without sensitive data

---

## 9. Commit Style

Use conventional commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

When AI assists with commits, include Co-Authored-By trailer:

```
feat: add user authentication

Implement JWT-based authentication for API endpoints.

Co-Authored-By: <AI Assistant> <noreply@example.com>
```

# Go Profile

Rules for Go development.

---

## 10. Code Organization

- Use `internal/` for private packages; add `pkg/` only if you intentionally support external imports as a stable public API.
- One package per directory; split across multiple files freely when it improves clarity (organize by responsibility, avoid "god files").
- Keep `main.go` minimal – delegate to internal packages.

```
project/
├── cmd/
│   └── myapp/
│       └── main.go
├── internal/
│   ├── config/
│   ├── server/
│   └── storage/
├── pkg/           # Only if public API needed
├── go.mod
└── go.sum
```

---

## 11. Naming Conventions

- Use short, concise variable names in small scopes (`i`, `err`, `ctx`).
- Use descriptive names for package-level and exported identifiers.
- Avoid stuttering: `config.Config` not `config.ConfigStruct`.
- Interfaces: single-method uses `-er` suffix (`Reader`, `Writer`, `Closer`).

```go
// Good
type Reader interface {
    Read(p []byte) (n int, err error)
}

// Avoid
type IReader interface {
    Read(p []byte) (n int, err error)
}
```

---

## 12. Error Handling

- Return errors, don't panic (except in `init()` for truly fatal issues).
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`.
- Use custom error types for errors that need programmatic handling.
- Check errors immediately after the call.
- Avoid logging inside libraries/internal packages by default; return errors and let `main` decide how to report them.

```go
// Good
result, err := doSomething()
if err != nil {
    return fmt.Errorf("processing item %s: %w", id, err)
}

// Avoid
result, err := doSomething()
if err != nil {
    log.Printf("error: %v", err)
    return err
}
```

---

## 13. Testing

- Table-driven tests for multiple cases.
- Prefer stdlib `testing`; use `testify` only when it materially improves clarity.
- Test file next to source: `foo.go` → `foo_test.go`.
- Use `testdata/` for fixtures.
- Use `t.TempDir()` for file-based tests.

```go
func TestProcess(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"empty input", "", "", true},
        {"valid input", "hello", "HELLO", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Process(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Process() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

## 14. Formatting & Linting

- `gofmt` / `goimports` on save.
- Run `golangci-lint` with default config.
- No `//nolint` without justification comment.

### golangci-lint Rules

**Octal literals** – Use `0o` prefix:
```go
os.WriteFile(path, data, 0o644)  // Good
os.WriteFile(path, data, 0644)   // Bad
```

**Import ordering** – Three groups separated by blank lines: stdlib, third-party, local. Fix with `goimports -local <module-path> -w .`
```go
import (
    "context"
    "fmt"

    "github.com/spf13/cobra"

    "myproject/internal/check"
)
```

**Unused parameters** – Rename to `_`:
```go
func handler(_ *cobra.Command, _ []string) error { return doWork() }
```

**Slice pre-allocation** – Pre-allocate when size is known:
```go
results := make([]string, 0, len(items))
```

**Explicit error handling** – Check or explicitly ignore with `_`:
```go
if err := tags.ForEach(fn); err != nil { return err }  // Check
_ = tags.ForEach(fn)                                    // Explicit ignore
defer func() { _ = resource.Cleanup() }()               // Cleanup in defer
```

**No os.Exit after defer** – Separate execution from exit handling so defers run.

**String comparisons** – Use `strings.EqualFold` for case-insensitive:
```go
strings.EqualFold(tag, "v"+version)  // Good
strings.ToLower(tag) == "v"+version  // Bad
```

**Empty string checks** – Use direct comparison:
```go
if s != "" { }   // Good
if len(s) > 0 { } // Bad
```

```bash
gofmt -w .
golangci-lint run ./...
```

---

## 15. Dependencies

- Prefer stdlib over third-party when reasonable.
- Vendor dependencies (`go mod vendor`) only if you need offline/hermetic builds or policy requires it.
- Minimize dependency count.

---

## 16. Concurrency

- Prefer mutexes for shared state; use channels for coordination/signaling and ownership handoff.
- Use `context.Context` for cancellation/timeouts.
- Document goroutine ownership and lifecycle.

```go
// Good - context for cancellation
func fetchData(ctx context.Context, url string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    // ...
}
```

---

## 17. I/O & Determinism

- Prefer `io/fs` (`fs.FS`, `os.DirFS`) abstractions to keep file I/O testable.
- Deterministic output: stable ordering, stable formatting, consistent newlines across OSes.
- Use `\n` (LF) for all generated files.

---

## 18. Documentation

- Document exported functions, types, and packages.
- Start comments with the name being documented.
- Keep comments concise but informative.

```go
// Config holds the application configuration.
// It is loaded from environment variables or a config file.
type Config struct {
    // Port is the HTTP server port.
    Port int
    // Debug enables debug logging.
    Debug bool
}

// NewConfig creates a Config from environment variables.
func NewConfig() (*Config, error) {
    // ...
}
```

---

## 19. Common Commands

```bash
# Initialize module
go mod init example.com/myapp

# Build
go build ./...

# Test
go test ./...
go test -v -race -coverprofile=coverage.out ./...

# Lint
golangci-lint run ./...

# Format
gofmt -w .
goimports -w .
```

# Claude-Specific Rules

Rules specific to Claude Code assistant.

---

## 20. Output Format

- Keep responses concise and actionable.
- Use code blocks with appropriate language tags.
- Prefer editing existing files over creating new ones.
- Use GitHub-flavored markdown for formatting.

---

## 21. Tool Usage

- Use Read tool before editing files.
- Use Grep/Glob for searching, not bash grep/find.
- Make atomic, focused changes.
- Batch related operations in a single message.

---

## 22. Claude Flow & Multi-Agent Coordination

Core principle: **Claude Flow coordinates, Claude Code executes.**

### 22.1 Golden rule for agents

- **"1 message = all related operations"**
  - For a given task, one message should:
    - Spawn all agents (via Task tool)
    - Batch all todos (TodoWrite)
    - Batch all file operations
    - Batch all shell commands

### 22.2 Roles

**MCP / Claude Flow tools:**
- Swarm+coordination: `swarm_init`, `agent_spawn`, `task_orchestrate`, etc.
- Setup topology, memory patterns, coordination – **not** actual file work.

**Claude Code:**
- Runs the real work:
  - `Task(...)` for agents (`coder`, `tester`, `reviewer`, `planner`, etc.)
  - File operations (read, write, edit)
  - Bash commands
  - Git operations
  - Package management
  - Test runs

### 22.3 Execution pattern

For a non-trivial task:

1. (Optional) Use Claude Flow MCP to configure coordination.
2. In one Claude Code message:
   - Spawn all relevant agents with clear roles.
   - Batch all planned file reads/writes/edits.
   - Batch all shell commands.
   - Batch all todos in one `TodoWrite` call (5–10+ items).
3. Respect folder structure and root rules:
   - Never create code, tests, or docs in root (except `TODO.md`, `README.md`, `CLAUDE.md`).

---

## 23. Presentation Workflow

When creating presentations using Claude Flow:

### 23.1 Structure

```
<presentation-name>/
├── speaker-notes.md    # Full narrative (2-3× slide detail)
├── slides.pptx         # Visual anchors, minimal text
└── README.md           # Optional: abstract, audience, duration
```

### 23.2 Process

1. **Research phase**: Gather content from knowledge base, docs, research folder.
2. **Outline phase**: Create structure with key points per slide.
3. **Speaker notes**: Write full narrative in markdown.
4. **Slides**: Create visual deck (can be manual or via tooling).
5. **Review**: Cross-check notes against slides for consistency.

### 23.3 Agent coordination

For complex presentations, coordinate agents:
- `researcher`: Gathers relevant content
- `writer`: Creates speaker notes
- `reviewer`: Checks consistency and flow

---

## 24. Task Tool Best Practices

When using the Task tool to spawn agents:

- Provide clear, specific prompts.
- Specify what the agent should do (research vs. implement).
- Use appropriate agent types:
  - `Explore`: For codebase exploration
  - `Plan`: For implementation planning
  - `Bash`: For command execution
  - `general-purpose`: For multi-step tasks

```
Task(
  subagent_type="Explore",
  prompt="Find all files that handle user authentication",
  description="Find auth files"
)
```

---

## 25. Memory & Context

- Use TodoWrite to track progress across turns.
- Reference earlier context when resuming work.
- Keep TODO.md updated for persistence across sessions.
- Use clear markers for work-in-progress sections.

# DevLake Local Development (AI Reference)

Quick reference for AI agents working on this project.

## Environment

- **Container runtime**: podman (not docker)
- **Compose file**: `docker-compose-dev.yml`
- **Database**: MySQL 8, credentials `merico:merico`, database `lake`
- **Test database**: `lake_test` (auto-created via `scripts/mysql-init.sql`)
- **MySQL root**: `root:admin` (for admin operations)
- **Go version**: 1.21+

## Service URLs

| Service | URL |
|---------|-----|
| Config UI | http://localhost:4000 |
| DevLake API | http://localhost:8080 |
| Grafana | http://localhost:4000/grafana/ |
| MySQL | localhost:3306 |

## Custom CA (CEE GitLab)

For internal services using Red Hat CA:

```bash
# Export Red Hat IT Root CA (macOS)
security find-certificate -a -p -c "Red Hat IT Root CA" /Library/Keychains/System.keychain > ./custom-ca.crt

# Set in .env
CA_CERT_FILE=./custom-ca.crt

# Start with custom CA
podman compose -f docker-compose-dev.yml -f docker-compose-custom-ca.yml up -d
```

## Common Commands

```bash
# Start all services
podman compose -f docker-compose-dev.yml up -d

# Start just MySQL (for local Go dev)
podman compose -f docker-compose-dev.yml up -d mysql

# Stop services
podman compose -f docker-compose-dev.yml down

# Rebuild and restart devlake container
podman compose -f docker-compose-dev.yml up -d --build devlake

# View logs
podman compose -f docker-compose-dev.yml logs -f devlake
```

## Building (from backend/)

```bash
cd backend
make go-dep      # Install dependencies
make build       # Build everything (plugins + server)
make build-plugin # Build plugins only
make run         # Run server
make dev         # Build and run
```

## Testing

```bash
cd backend

# All tests (unit + e2e)
make test

# Unit tests only
make unit-test

# E2E tests (requires MySQL running with lake_test)
export E2E_DB_URL="mysql://merico:merico@localhost:3306/lake_test?charset=utf8mb4&parseTime=True"
make e2e-test

# Specific plugin tests
go test ./plugins/aireview/... -v

# Lint and format
make lint
make fmt
```

## Database

```bash
# Connect to MySQL
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake

# Check migrations
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e \
  "SELECT * FROM _devlake_migration_history ORDER BY created_at DESC LIMIT 10;"

# Trigger migration after schema changes
curl -s http://localhost:4000/api/proceed-db-migration
```

## Plugin Verification

```bash
# Check plugin is loaded
curl -s http://localhost:4000/api/plugins | jq '.[] | select(.plugin == "aireview")'

# Check aireview tables
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e \
  "SHOW TABLES LIKE '_tool_aireview%';"
```

## Before Running Commands

1. Ensure podman machine is running: `podman machine start`
2. Check if services are already running: `podman ps`
3. For E2E tests, MySQL must be running with `lake_test` database

## If E2E tests fail with "Access denied for lake_test"

Existing MySQL volumes may not have `lake_test`. Fix with:

```bash
podman compose -f docker-compose-dev.yml exec mysql mysql -uroot -padmin -e "
  CREATE DATABASE IF NOT EXISTS lake_test;
  GRANT ALL PRIVILEGES ON lake_test.* TO 'merico'@'%';
  FLUSH PRIVILEGES;
"
```

## Project Conventions

- Plugin code: `backend/plugins/<plugin-name>/`
- Tests next to source: `foo.go` → `foo_test.go`
- E2E test fixtures: `e2e/raw_tables/*.csv`
- Migrations: `models/migrationscripts/`

Also follow the rules in [AGENTS.md](./AGENTS.md) — plugin ownership and upstream
divergence tracking.
