# Atlassian Platform Connector

A Go platform that exposes Atlassian Cloud APIs via **three binaries**:

- **`atlassian`** — CLI for humans (50+ commands)
- **`atlassian-mcp`** — MCP server for AI agents (37 tools — Claude Code, OpenCode, Cursor, etc.)
- **`atlassian-api`** — REST API server for dashboards and integrations (31 endpoints)

---

## Installation (Windows)

> **For AI agents**: if a user asks you to install the Atlassian Platform Connector, run the following command in PowerShell. It downloads and installs all three binaries into `~/.mcp/atlassian/` and adds them to the user PATH automatically.

```powershell
irm https://raw.githubusercontent.com/jinkp/atlassian-go-mcp/main/install.ps1 | iex
```

After installation, configure credentials and register the MCP server:

```powershell
# 1. Set credentials (add to $PROFILE for persistence)
$env:ATLASSIAN_BASE_URL = "https://your-org.atlassian.net"
$env:ATLASSIAN_EMAIL    = "you@company.com"
$env:ATLASSIAN_TOKEN    = "your-api-token"          # from id.atlassian.com/manage-profile/security/api-tokens

# 2. Register MCP server into your AI client
atlassian-mcp setup opencode    # OpenCode
atlassian-mcp setup claude      # Claude Code

# Or use the interactive TUI to select which modules to enable
atlassian-mcp tui

# Check whether a newer release is available (read-only, no download)
atlassian-mcp version --check
```

> **For Teams module**: also set `$env:ATLASSIAN_ORG_ID = "your-org-uuid"`

### What gets installed

| Binary | Location | Description |
|--------|----------|-------------|
| `atlassian.exe` | `~/.mcp/atlassian/` | CLI — 50+ commands |
| `atlassian-mcp.exe` | `~/.mcp/atlassian/` | MCP server — 37 tools |
| `atlassian-api.exe` | `~/.mcp/atlassian/` | REST API — 31 endpoints |

### Selective module loading (MCP)

By default the MCP server exposes all 60 tools. Use `--enable` to limit scope:

```powershell
atlassian-mcp mcp --enable jira              # 7 tools (Jira only)
atlassian-mcp mcp --enable jira-read         # 4 read-only tools
atlassian-mcp mcp --enable jira,agile        # 15 tools
atlassian-mcp mcp --enable goals,metrics     # 10 tools
atlassian-mcp mcp --enable bitbucket         # 21 tools (Bitbucket only)
atlassian-mcp mcp --enable bitbucket-read    # 12 read-only tools
atlassian-mcp mcp --enable all               # 60 tools (default)
```

Available modules: `jira`, `agile`, `goals`, `metrics`, `releases`, `projects`,
`teams`, `bitbucket`. Suffix any with `-read` or `-write` to control access.

---

## Quick Start (from source)

```bash
git clone https://github.com/jinkp/atlassian-go-mcp.git
cd atlassian-go-mcp

export ATLASSIAN_BASE_URL=https://your-org.atlassian.net
export ATLASSIAN_EMAIL=you@company.com
export ATLASSIAN_TOKEN=your-api-token

# Run CLI directly
go run ./cmd/atlassian jira search --jql "project=PROJ ORDER BY updated DESC"

# Build all three binaries
go build -o atlassian     ./cmd/atlassian/
go build -o atlassian-mcp ./cmd/mcp/
go build -o atlassian-api ./cmd/api/

# Register MCP into your AI client (one-time)
./atlassian-mcp setup opencode
./atlassian-mcp setup claude

# Start REST API
./atlassian-api --port 8080

# Run tests
go test ./...
```

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ATLASSIAN_BASE_URL` | Yes | `https://your-org.atlassian.net` |
| `ATLASSIAN_EMAIL` | Yes | Your Atlassian account email |
| `ATLASSIAN_TOKEN` | Yes | Jira API token from id.atlassian.com |
| `ATLASSIAN_ORG_ID` | For Teams | Organization UUID (required for `teams` commands) |
| `BITBUCKET_USERNAME` | For Bitbucket | Bitbucket account (usually your Atlassian email) |
| `BITBUCKET_API_TOKEN` | For Bitbucket | API token from id.atlassian.com |
| `BITBUCKET_WORKSPACE` | No | Default workspace slug (overridable per call) |
| `BITBUCKET_REPO` | No | Default repository slug (overridable per call) |
| `ENABLE_WRITE=true` | No | Enables write tools in MCP server (default: disabled) |

### Shared credentials file (homologated)

All Atlassian MCP tools (this connector and `bbk`) read from a **single shared
file**, so you configure credentials once:

```
~/.atlassian/credentials.env        # Windows: %USERPROFILE%\.atlassian\credentials.env
```

Override its location with `ATLASSIAN_SHARED_CONFIG`. Example contents:

```dotenv
# --- Jira / Confluence / Teams ---
ATLASSIAN_BASE_URL=https://your-org.atlassian.net
ATLASSIAN_EMAIL=you@company.com
JIRA_API_TOKEN=your-jira-api-token
ATLASSIAN_ORG_ID=your-org-uuid        # optional, Teams only
# --- Bitbucket ---
BITBUCKET_USERNAME=you@company.com
BITBUCKET_API_TOKEN=your-bitbucket-api-token
BITBUCKET_WORKSPACE=your-workspace     # optional default
BITBUCKET_REPO=your-repo               # optional default
```

Notes:
- The Jira token is stored as `JIRA_API_TOKEN` in the file but exported at runtime
  as `ATLASSIAN_TOKEN` (what the services expect). Both are accepted on read.
- Environment variables always **win** over the file.
- Writes are **merge-by-line**: saving Jira credentials never touches the
  `BITBUCKET_*` keys, and vice versa.
- On first run, credentials from the legacy `~/.mcp/atlassian/.env` are migrated
  automatically (non-destructively) into the shared file.

---

## Architecture

```
Atlassian Cloud
      │
      ├── Jira REST v3          /rest/api/3/
      ├── Agile REST v1.0       /rest/agile/1.0/
      ├── Goals GraphQL         /gateway/api/graphql
      ├── Releases REST v3      /rest/api/3/version
      ├── Projects REST v3      /rest/api/3/project
      └── Teams Public REST     api.atlassian.com/public/teams/v1/
                │
        internal/atlassian/
          ├── client/     HTTP client, BasicAuth, Idempotency-Key, Retry (429-only)
          ├── jira/       JiraService
          ├── agile/      AgileService
          ├── goals/      GoalsService (GraphQL)
          ├── releases/   ReleasesService
          ├── projects/   ProjectsService
          └── teams/      TeamsService
                │
    ┌─────────────────────────────┐
    │           │                 │
cmd/atlassian/ cmd/mcp/       cmd/api/
CLI for humans MCP for agents REST API
```

### Cross-cutting features
- **Audit log** — every write operation logged as JSON lines to stderr
- **`--dry-run`** — all CLI write commands print intent without executing
- **Idempotency-Key** — injected automatically on all POST requests
- **WriteGuard** — MCP write tools blocked unless `ENABLE_WRITE=true`

---

## CLI Reference

### Global flags
```bash
--output table|json|yaml   # output format (default: table)
--dry-run                  # print what would happen, don't execute
```

### Exit codes
| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Usage / user error |
| `2` | Auth / API error |
| `3` | Not found |

---

### Jira

```bash
# Read
atlassian jira get <KEY>
atlassian jira search --jql "project=PROJ AND status='In Progress'"

# Write
atlassian jira create --project PROJ --type Bug --summary "..." \
  [--description "..."] [--assignee <accountId>] [--priority High] [--labels "bug,urgent"]
atlassian jira update <KEY> [--summary "..."] [--description "..."] [--assignee <accountId>] [--priority Medium]
atlassian jira transitions <KEY>
atlassian jira transition <KEY> --transition-id 31
```

### Agile

```bash
# Read
atlassian agile boards --project PROJ
atlassian agile sprints --board-id 10 [--state active|future|closed]
atlassian agile sprint active --board-id 10
atlassian agile sprint issues --sprint-id 42

# Write
atlassian agile sprint create --board-id 10 --name "Sprint 9" [--start "..."] [--end "..."]
atlassian agile sprint update --sprint-id 42 [--name "..."] [--state closed]
atlassian agile move-to-sprint --sprint-id 42 --issues "PROJ-1,PROJ-2"
atlassian agile move-to-epic --epic-key PROJ-100 --issues "PROJ-1,PROJ-2"
```

### Goals

```bash
# Get site ID first (required for search/create)
atlassian goals site-id --subdomain myorg

# Read
atlassian goals get "ari:cloud:townsquare:abc:goal/xyz"
atlassian goals search --site-id <cloudId> [--query "status = on_track"] [--max-results 25]

# Write
atlassian goals create --site-id <cloudId> --name "Grow MRR 20%" \
  --type-id "ari:cloud:goal:<siteId>:goal-type/..." --target-date 2026-12-31 \
  [--confidence QUARTER] [--description "..."]
atlassian goals update --goal-id "ari:..." --status on_track [--score 75] [--summary "..."]
atlassian goals edit "ari:..." [--name "..."] [--target-date 2026-12-31] [--archive]

# Metrics
atlassian goals metrics "ari:..."
atlassian goals metric-create --goal-id "ari:..." --name "MRR Growth" \
  --type PERCENTAGE --start 0 --target 20 --initial 8
atlassian goals metric-value --metric-id "ari:..." --value 12.5 [--time 2024-01-15T00:00:00Z]
atlassian goals metric-target --metric-target-id "ari:..." [--current 12.5] [--target 25]
```

### Releases

```bash
atlassian releases list --project PROJ
atlassian releases get <release-id>
atlassian releases issues <release-id>
atlassian releases create --project-id 10000 --name "v1.0.0" [--description "..."] [--release-date 2024-03-01]
atlassian releases update <release-id> [--name "..."] [--released] [--archived] [--release-date 2024-03-01]
```

### Projects

```bash
atlassian projects list [--max-results 50]
atlassian projects get <KEY>
atlassian projects search --query "backend"
atlassian projects update <KEY> [--name "..."] [--description "..."] [--lead <accountId>]
```

### Teams

```bash
# Requires ATLASSIAN_ORG_ID env var
atlassian teams list [--query "engineering"] [--max-results 50]
atlassian teams get <team-id>
atlassian teams members <team-id> [--max-results 50]
```

### Bitbucket

```bash
# Requires BITBUCKET_USERNAME + BITBUCKET_API_TOKEN (shared file or env)
# --workspace / --repo default to BITBUCKET_WORKSPACE / BITBUCKET_REPO

# Read
atlassian bitbucket repos [--workspace <ws>]
atlassian bitbucket branches | stale-branches [--days 30]
atlassian bitbucket pipeline list
atlassian bitbucket pr list [--state OPEN]
atlassian bitbucket pr get|comments|commits|files|diff|checks|reviewers <id>

# Write
atlassian bitbucket pr create --title <t> --source <branch> --dest <branch>
atlassian bitbucket pr comment <id> --body <text>
atlassian bitbucket pr update <id> [--title ... --description ...]
atlassian bitbucket pr approve|decline <id>
atlassian bitbucket pr merge <id> [--strategy merge_commit|squash|fast_forward] [--message <m>]
atlassian bitbucket pr task add <id> --body <text>
atlassian bitbucket pr task resolve <id> <taskId>
atlassian bitbucket pipeline run --branch <branch>

# Local-only convenience (not exposed via MCP/REST)
atlassian bitbucket pr checkout <id>    # git fetch + checkout the PR branch
atlassian bitbucket pr open <id>        # open the PR in your browser
```

---

## MCP Server — 60 Tools

```bash
# Start server
./atlassian-mcp mcp

# Enable write operations
ENABLE_WRITE=true ./atlassian-mcp mcp
```

### Jira Read (2)
| Tool | Args | Description |
|------|------|-------------|
| `get_jira_issue` | `issue_key` (req) | Get issue by key |
| `search_jira_issues` | `jql` (req), `max_results` (opt, default 50) | Search with JQL |

### Jira Write (4) — `ENABLE_WRITE=true`
| Tool | Args | Description |
|------|------|-------------|
| `create_jira_issue` | `project_key`, `issue_type`, `summary` (req); `description`, `assignee_id`, `priority`, `labels` (opt) | Create issue |
| `update_jira_issue` | `issue_key` (req); `summary`, `description`, `assignee_id`, `priority` (opt) | Update fields |
| `get_jira_transitions` | `issue_key` (req) | List workflow transitions |
| `transition_jira_issue` | `issue_key`, `transition_id` (req) | Apply transition |

### Agile Read (5)
| Tool | Args | Description |
|------|------|-------------|
| `get_jira_boards` | `project_key` (req), `max_results` (opt) | List boards |
| `get_jira_sprints` | `board_id` (req), `state` (opt), `max_results` (opt) | List sprints |
| `get_active_sprint` | `board_id` (req) | Get active sprint |
| `get_sprint_issues` | `sprint_id` (req), `max_results` (opt) | Issues in sprint |
| `get_jira_epics` | `project_key` (req) | List epics |

### Agile Write (4) — `ENABLE_WRITE=true`
| Tool | Args | Description |
|------|------|-------------|
| `create_sprint` | `name`, `board_id` (req); `start_date`, `end_date` (opt) | Create sprint |
| `update_sprint` | `sprint_id` (req); `state`, `name`, `start_date`, `end_date` (opt) | Update/close sprint |
| `move_issues_to_sprint` | `sprint_id` (req), `issue_keys` (req, comma-sep, max 50) | Move issues to sprint |
| `move_issues_to_epic` | `epic_key` (req), `issue_keys` (req, comma-sep) | Link issues to epic |

### Goals Read (3)
| Tool | Args | Description |
|------|------|-------------|
| `get_site_id` | `subdomain` (req, e.g. `myorg`) | Get Atlassian cloud ID |
| `get_goal` | `goal_id` (req, ARI) | Get goal by ID |
| `search_goals` | `site_id` (req), `search_string` (opt), `max_results` (opt), `cursor` (opt) | Search goals |
| `get_goal_metrics` | `goal_id` (req) | List metric targets for a goal |

### Goals Write (5) — `ENABLE_WRITE=true`
| Tool | Args | Description |
|------|------|-------------|
| `update_goal_status` | `goal_id`, `status` (req: `on_track`/`off_track`/`at_risk`); `score` (opt), `summary` (opt) | Post check-in |
| `create_goal` | `site_id`, `name`, `goal_type_id`, `target_date` (req); `confidence` (opt), `description` (opt) | Create goal |
| `edit_goal` | `goal_id` (req); `name`, `target_date`, `confidence`, `archive` (opt) | Edit goal fields |
| `create_metric` | `goal_id`, `name`, `metric_type`, `start_value`, `target_value`, `initial_value` (req) | Create metric |
| `update_metric_value` | `metric_id` (req), `value` (req); `time` (opt) | Record metric value |
| `update_metric_target` | `metric_target_id` (req); `current_value`, `start_value`, `target_value` (opt) | Update targets |

### Releases Read (3)
| Tool | Args | Description |
|------|------|-------------|
| `search_releases` | `project_key` (req) | List releases for project |
| `get_release` | `release_id` (req) | Get release by ID |
| `get_release_issues` | `release_id` (req) | Issue counts for release |

### Releases Write (2) — `ENABLE_WRITE=true`
| Tool | Args | Description |
|------|------|-------------|
| `create_release` | `project_id`, `name` (req); `description`, `start_date`, `release_date` (opt) | Create release |
| `update_release` | `release_id` (req); `name`, `description`, `release_date`, `released`, `archived` (opt) | Update release |

### Projects Read (3)
| Tool | Args | Description |
|------|------|-------------|
| `list_projects` | `max_results` (opt, default 50) | List all projects |
| `get_project` | `project_key` (req) | Get project by key |
| `search_projects` | `query` (opt), `max_results` (opt) | Search projects |

### Projects Write (1) — `ENABLE_WRITE=true`
| Tool | Args | Description |
|------|------|-------------|
| `update_project` | `project_key` (req); `name`, `description`, `lead` (opt) | Update project |

### Teams Read (3)
| Tool | Args | Description |
|------|------|-------------|
| `search_teams` | `query` (opt), `max_results` (opt, default 50) | Search teams |
| `get_team` | `team_id` (req) | Get team by ID |
| `get_team_members` | `team_id` (req), `max_results` (opt, default 50) | List team members |

### Bitbucket Read (12)
`bb_list_repos`, `bb_list_prs`, `bb_get_pr`, `bb_get_pr_comments`,
`bb_get_pr_commits`, `bb_get_pr_files`, `bb_get_pr_diff`, `bb_get_pr_checks`,
`bb_get_pr_reviewers`, `bb_list_branches`, `bb_list_stale_branches`,
`bb_list_pipelines` — all take optional `workspace`/`repo` (fall back to env).

### Bitbucket Write (9) — `ENABLE_WRITE=true`
`bb_create_pr`, `bb_comment_pr`, `bb_update_pr`, `bb_approve_pr`,
`bb_decline_pr`, `bb_merge_pr`, `bb_create_task`, `bb_resolve_task`,
`bb_run_pipeline`.

---

## REST API — 52 Endpoints

```bash
./atlassian-api --port 8080 [--read-only]

# Write operations require header:
# X-Enable-Write: true
```

### Routes

```
GET  /health

# Jira
GET  /jira/issues/{key}
GET  /jira/issues?jql=...&maxResults=50
POST /jira/issues                          (X-Enable-Write: true)
PUT  /jira/issues/{key}                    (X-Enable-Write: true)
GET  /jira/issues/{key}/transitions
POST /jira/issues/{key}/transitions        (X-Enable-Write: true)

# Agile
GET  /agile/boards?project=...
GET  /agile/boards/{boardId}/sprints?state=...
GET  /agile/boards/{boardId}/sprints/active
GET  /agile/sprints/{sprintId}/issues
POST /agile/sprints                        (X-Enable-Write: true)
PUT  /agile/sprints/{sprintId}             (X-Enable-Write: true)
POST /agile/sprints/{sprintId}/issues      (X-Enable-Write: true)

# Goals
GET  /goals/site-id?subdomain=...
GET  /goals?siteId=...&query=...
GET  /goals/{goalId}
POST /goals                                (X-Enable-Write: true)
PUT  /goals/{goalId}/status               (X-Enable-Write: true)
PUT  /goals/{goalId}                      (X-Enable-Write: true)

# Releases
GET  /releases?project=...
GET  /releases/{releaseId}
GET  /releases/{releaseId}/issues
POST /releases                             (X-Enable-Write: true)
PUT  /releases/{releaseId}                 (X-Enable-Write: true)

# Projects
GET  /projects?query=...
GET  /projects/{key}
PUT  /projects/{key}                       (X-Enable-Write: true)

# Teams
GET  /teams?query=...
GET  /teams/{teamId}
GET  /teams/{teamId}/members

# Bitbucket
GET  /bitbucket/repos
GET  /bitbucket/pullrequests
GET  /bitbucket/pullrequests/{id}
GET  /bitbucket/pullrequests/{id}/comments
GET  /bitbucket/pullrequests/{id}/commits
GET  /bitbucket/pullrequests/{id}/files
GET  /bitbucket/pullrequests/{id}/diff
GET  /bitbucket/pullrequests/{id}/checks
GET  /bitbucket/pullrequests/{id}/reviewers
GET  /bitbucket/branches
GET  /bitbucket/branches/stale
GET  /bitbucket/pipelines
POST /bitbucket/pullrequests                       (X-Enable-Write: true)
POST /bitbucket/pullrequests/{id}/comments         (X-Enable-Write: true)
PUT  /bitbucket/pullrequests/{id}                   (X-Enable-Write: true)
POST /bitbucket/pullrequests/{id}/approve           (X-Enable-Write: true)
POST /bitbucket/pullrequests/{id}/decline           (X-Enable-Write: true)
POST /bitbucket/pullrequests/{id}/merge             (X-Enable-Write: true)
POST /bitbucket/pullrequests/{id}/tasks             (X-Enable-Write: true)
PUT  /bitbucket/pullrequests/{id}/tasks/{taskId}    (X-Enable-Write: true)
POST /bitbucket/pipelines                           (X-Enable-Write: true)
```

### Error format
```json
{"error": "resource not found", "code": "NOT_FOUND"}
```

---

## Typical Agent Workflows

### Full sprint cycle
```
1. get_jira_boards        → find board_id for project
2. get_active_sprint      → get current sprint_id
3. get_sprint_issues      → see what's in the sprint
4. search_jira_issues     → find backlog candidates
5. move_issues_to_sprint  → load sprint with issues
6. update_sprint          → close sprint (state=closed)
7. create_sprint          → open next sprint
```

### Goal + metrics workflow
```
1. get_site_id            → get cloudId for your org
2. search_goals           → find goals by status/owner
3. get_goal_metrics       → see current metric values
4. update_metric_value    → record new actual value
5. update_goal_status     → post check-in (score + summary)
```

### Release management
```
1. search_releases        → list open releases
2. get_release_issues     → see issue counts by status
3. update_release         → mark as released (released=true)
4. create_release         → open next version
```

### Bug triage
```
1. search_jira_issues     → find High priority open bugs
2. create_jira_issue      → create new bug
3. get_jira_transitions   → discover transition IDs
4. transition_jira_issue  → move to In Progress
```

---

## Guardrails

| Feature | Behavior |
|---------|----------|
| Write protection (MCP) | All write tools require `ENABLE_WRITE=true` — safe by default for AI agents |
| Write protection (API) | POST/PUT blocked unless `X-Enable-Write: true` header — or `--read-only` flag |
| `--dry-run` (CLI) | All write commands print intent without executing — exit 0 |
| Idempotency-Key | Injected automatically on every POST request |
| Rate limiting | Retry on HTTP 429 only — exponential backoff + `Retry-After` header |
| Audit log | Every write operation logged as JSON line to stderr |
| Token safety | API tokens never logged or printed to stdout |
| ADF handling | Plain-text descriptions auto-wrapped to Atlassian Document Format |

---

## Project Structure

```
atlassian-go-mcp/
├── cmd/
│   ├── atlassian/               # CLI binary
│   │   ├── main.go
│   │   ├── jira/                # get, search, create, update, transitions, transition
│   │   ├── agile/               # boards, sprints, sprint-*, move-to-*
│   │   ├── goals/               # site-id, get, search, create, update, edit, metrics
│   │   ├── releases/            # list, get, issues, create, update
│   │   ├── projects/            # list, get, search, update
│   │   └── teams/               # list, get, members
│   ├── mcp/
│   │   └── main.go              # MCP stdio server (37 tools)
│   └── api/
│       └── main.go              # REST API server (31 endpoints)
├── internal/
│   ├── atlassian/
│   │   ├── client/              # HTTP, BasicAuth, IdempotencyTransport, RetryTransport
│   │   ├── jira/                # Jira REST v3 — full CRUD + transitions
│   │   ├── agile/               # Jira Agile REST v1.0 — boards, sprints, moves
│   │   ├── goals/               # Goals GraphQL — 10 methods incl. metrics
│   │   ├── releases/            # Jira Versions REST v3
│   │   ├── projects/            # Jira Projects REST v3
│   │   └── teams/               # Atlassian Teams Public REST API
│   ├── api/
│   │   ├── server.go            # Server struct, Start()
│   │   ├── router.go            # Route registration
│   │   ├── middleware.go        # Recover, Logging, WriteGuard
│   │   ├── respond.go           # JSON helpers, error codes
│   │   └── handlers/            # Domain handlers (jira, agile, goals, etc.)
│   ├── mcp/                     # Tool handlers, WriteGuardCheck, server
│   ├── audit/                   # JSONLogger — writes to stderr
│   ├── opencode/                # JSON-merge config writer for opencode.json
│   ├── claude/                  # JSON-merge config writer for .claude.json
│   └── output/                  # Formatters: table / json / yaml
├── go.mod
├── go.sum
└── mvp.txt
```

---

## Notes

- **`atlassian agile epics` CLI** — not yet implemented (use `search_jira_issues --jql "issuetype=Epic"`)
- **Teams** requires `ATLASSIAN_ORG_ID` env var (org UUID from your Atlassian admin)
- **`goal_type_id`** is per-tenant ARI — obtain from Goals UI or Atlassian admin
- **Assignee** in Jira requires `accountId` (not email/display name)
- **Date format** for sprints/goals: ISO 8601 — `2024-01-15T00:00:00.000Z`
- **Metric types**: `CURRENCY` | `NUMERIC` | `PERCENTAGE`
- **REST API `/goals/{goalId}/metrics`** — metrics endpoints not yet exposed in REST API (CLI and MCP have full metrics support)
- **Windows only** — Linux/macOS builds not yet available; build from source with `go build`
