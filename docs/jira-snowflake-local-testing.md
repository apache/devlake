# jira_snowflake Plugin — Local Testing Guide

## Prerequisites

- Go 1.21+
- podman + podman-compose
- Access to the Snowflake account (`JIRA_DB.CLOUDRHAI_MARTS`)
- Snowflake role with `SELECT` on that schema

---

## Step 1 — Verify Snowflake access

Before starting DevLake, confirm you can reach Snowflake from your machine.

Create a throwaway Go file (outside the repo):

```go
// /tmp/check_snowflake.go
package main

import (
	"database/sql"
	"fmt"
	"log"

	sf "github.com/snowflakedb/gosnowflake"
)

func main() {
	cfg := &sf.Config{
		Account:       "YOUR_ACCOUNT",   // e.g. "myorg-myaccount" (not the full URL)
		User:          "YOUR_USER",
		Role:          "YOUR_ROLE",
		Warehouse:     "YOUR_WAREHOUSE",
		Database:      "JIRA_DB",
		Schema:        "CLOUDRHAI_MARTS",
		Authenticator: sf.AuthTypeExternalBrowser, // opens browser for SSO
	}
	dsn, err := sf.DSN(cfg)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var n int
	row := db.QueryRow("SELECT COUNT(*) FROM JIRA_ISSUE_NON_PII")
	if err := row.Scan(&n); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("OK — %d issues visible\n", n)
}
```

Run it from `backend/` where `gosnowflake` is already in `go.mod`:

```bash
cd backend
go run /tmp/check_snowflake.go
# A browser window opens for SSO — log in once, then the script prints:
# OK — NNNN issues visible
```

If you see `OK — NNNN issues visible`, your credentials and role are working.

**Common errors:**

| Error | Fix |
|---|---|
| `account is empty` | Use the account identifier (e.g. `myorg-myaccount`), not the full `*.snowflakecomputing.com` URL |
| `Object does not exist or not authorized` | Your role lacks `SELECT` on `JIRA_DB.CLOUDRHAI_MARTS` — ask a Snowflake admin |
| Browser window never opens | You may be running inside a container or headless SSH session — run locally |

---

## Step 2 — Start MySQL

```bash
podman machine start
podman compose -f docker-compose-dev.yml up -d mysql
```

---

## Step 3 — Configure `.env`

Create or edit `.env` in the repo root:

```env
# Disable auth for local testing (skip bearer token on every API call)
AUTH_ENABLED=false

# Required: any 32+ char string works for local dev — replace with your own value
ENCRYPTION_SECRET=<generate-a-random-32-char-string-here>

DB_URL=mysql://merico:merico@127.0.0.1:3306/lake?charset=utf8mb4&parseTime=True

# Generate ENCRYPTION_SECRET with:
# openssl rand -hex 16
```

---

## Step 4 — Build and run DevLake (plugin only)

Browser-based SSO (`externalbrowser` auth) only works when DevLake runs **natively on your desktop**, not inside a container (the browser pop-up cannot reach a container process).

Build only the `jira_snowflake` plugin and start the server:

```bash
cd backend
DEVLAKE_PLUGINS=jira_snowflake DISABLED_REMOTE_PLUGINS=true ENV_FILE=../.env make build-plugin run
```

- `DEVLAKE_PLUGINS=jira_snowflake` — only compile this plugin (much faster than building all plugins)
- `DISABLED_REMOTE_PLUGINS=true` — skip loading remote/dynamic plugins
- `ENV_FILE=../.env` — point to the `.env` in the repo root

Verify the plugin loaded:

```bash
curl -s http://localhost:8080/plugins | jq '.[] | select(.plugin == "jira_snowflake")'
```

Trigger DB migrations (creates `_tool_jira_snowflake_connections` table):

```bash
curl -s http://localhost:8080/proceed-db-migration | jq .
```

Verify the table exists:

```bash
podman compose -f docker-compose-dev.yml exec mysql \
  mysql -umerico -pmerico lake -e "SHOW TABLES LIKE '_tool_jira_snowflake%';"
```

---

## Step 5 — Create a connection

```bash
curl -s -X POST http://localhost:8080/plugins/jira_snowflake/connections \
  -H 'Content-Type: application/json' \
  -d '{
    "name":      "my-snowflake-jira",
    "account":   "YOUR_ACCOUNT",
    "user":      "YOUR_USER",
    "authType":  "externalbrowser",
    "database":  "JIRA_DB",
    "schema":    "CLOUDRHAI_MARTS",
    "warehouse": "YOUR_WAREHOUSE",
    "role":      "YOUR_ROLE"
  }' | jq .
```

Note the `id` in the response — that is your `connectionId`.

> For **key-pair auth** (production / CI), use `"authType": "keypair"` and
> add `"privateKey": "-----BEGIN PRIVATE KEY-----\n..."`.
> Key-pair auth works inside containers and does not require a browser.

---

## Step 6 — Run a pipeline

You need:
- `connectionId` from Step 5
- `boardId`: the numeric Jira board ID (visible in the Jira board URL: `.../boards/NNN`)
- `projectKeys`: the list of Jira project keys that board covers (e.g. `["KONFLUX"]`)

```bash
curl -s -X POST http://localhost:8080/pipelines \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "jira-snowflake-test",
    "plan": [[{
      "plugin":  "jira_snowflake",
      "options": {
        "connectionId": 1,
        "boardId":      1,
        "projectKeys":  ["KONFLUX"]
      }
    }]]
  }' | jq '{id, status}'
```

Poll the pipeline until it completes:

```bash
PIPELINE_ID=<id from above>
while true; do
  curl -s http://localhost:8080/pipelines/$PIPELINE_ID | jq '{status, message}'
  sleep 5
done
```

A successful run ends with `"status": "TASK_COMPLETED"`.

---

## Step 7 — Verify results in MySQL

```bash
MYSQL="podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e"

# Row counts — tool layer and domain layer should match
$MYSQL "SELECT
  (SELECT COUNT(*) FROM _tool_jira_issues)     AS tool_issues,
  (SELECT COUNT(*) FROM issues)                AS domain_issues,
  (SELECT COUNT(*) FROM _tool_jira_changelogs) AS tool_changelogs,
  (SELECT COUNT(*) FROM issue_changelogs)       AS domain_changelogs\G"

# Status distribution
$MYSQL "SELECT std_type AS type, std_status AS status, COUNT(*) AS n
        FROM _tool_jira_issues
        GROUP BY std_type, std_status
        ORDER BY n DESC
        LIMIT 15;"

# Sample issues
$MYSQL "SELECT issue_key, priority_name, std_type, std_status
        FROM _tool_jira_issues LIMIT 10;"
```

**Expected results:**
- `tool_issues` and `domain_issues` counts should be equal (or domain slightly higher if rows remain from a prior run with a different board).
- `std_status` values should be `TODO`, `IN_PROGRESS`, or `DONE` — never `indeterminate`.
- `std_type` values should be `FEATURE`, `BUG`, `TASK`, `SUB-TASK`, etc.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Plugin not listed at `/plugins` | Build step skipped or failed | Re-run `make build-plugin run` and check for compile errors |
| `ENCRYPTION_SECRET` error on startup | `.env` missing or key too short | Set a 32+ char hex string in `.env` |
| `401 Unauthorized` on API calls | Auth enabled | Set `AUTH_ENABLED=false` in `.env` |
| `invalid identifier 'X'` in pipeline logs | Column/table name mismatch | Run `DESCRIBE TABLE JIRA_DB.CLOUDRHAI_MARTS.<TABLE>;` in Snowflake to verify column names |
| `Object 'X' does not exist or not authorized` | Table not replicated or role lacks access | See "Not available" table list in `AGENTS.md`; check Snowflake role grants |
| Browser pop-up doesn't open | Running inside a container | Run DevLake natively with `make run`, not via `podman compose` |
| Pipeline ends with `TASK_FAILED` | Check `message` field | `curl -s http://localhost:8080/pipelines/$ID | jq .message` for the full error |
| `tool_issues` populated but `domain_issues` = 0 | `convertIssues` subtask failed | Check pipeline subtask logs; ensure the subtask is enabled |
| Status shows `indeterminate` | `STATUSCATEGORY` value unexpected | The `CASE` in `sync_issues.go` maps `'2'`/`'3'`/`'4'`; any other value falls through to `indeterminate` |
