# Atlassian Platform Connector

A Go platform that exposes Atlassian Cloud APIs (Jira, Agile, Goals) via two binaries:

- **`atlassian`** — CLI for humans
- **`atlassian-mcp`** — MCP server for AI agents (Claude Code, OpenCode, Cursor, etc.)

---

## Quick Start

```bash
git clone https://github.com/jinkp/atlassian-go-mcp.git
cd atlassian-go-mcp

export ATLASSIAN_BASE_URL=https://your-org.atlassian.net
export ATLASSIAN_EMAIL=you@company.com
export ATLASSIAN_TOKEN=your-api-token

# CLI
go run ./cmd/atlassian jira search --jql "project=PROJ ORDER BY updated DESC"

# Build both binaries
go build -o atlassian ./cmd/atlassian/
go build -o atlassian-mcp ./cmd/mcp/

# Register MCP into your AI client (one-time)
./atlassian-mcp setup opencode
./atlassian-mcp setup claude

# Tests
go test ./...
```

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ATLASSIAN_BASE_URL` | Yes | `https://your-org.atlassian.net` |
| `ATLASSIAN_EMAIL` | Yes | Your Atlassian account email |
| `ATLASSIAN_TOKEN` | Yes | API token from id.atlassian.com |
| `ENABLE_WRITE=true` | No | Enables write tools in MCP (default: disabled) |

---

## Architecture

```
Atlassian Cloud
      │
      ├── Jira REST v3        /rest/api/3/
      ├── Agile REST v1.0     /rest/agile/1.0/
      └── Goals GraphQL       /gateway/api/graphql
                │
        internal/atlassian/
          ├── client/     HTTP client, BasicAuth, Retry (429-only)
          ├── jira/       JiraService
          ├── agile/      AgileService
          └── goals/      GoalsService (GraphQL)
                │
    ┌───────────────────────┐
    │                       │
cmd/atlassian/          cmd/mcp/
CLI for humans          MCP server for AI agents
```

---

## CLI Reference

### Jira

```bash
# Read
atlassian jira get <KEY>
atlassian jira search --jql "project=PROJ AND status='In Progress'" [--output table|json|yaml]

# Write
atlassian jira create \
  --project PROJ \
  --type Bug \
  --summary "Login fails on Safari" \
  [--description "Steps to reproduce..."] \
  [--assignee <accountId>] \
  [--priority High] \
  [--labels "bug,urgent"]

atlassian jira update PROJ-123 \
  [--summary "..."] \
  [--description "..."] \
  [--assignee <accountId>] \
  [--priority Medium]

atlassian jira transitions PROJ-123
atlassian jira transition PROJ-123 --transition-id 31
```

### Agile

```bash
# Boards & Sprints (read)
atlassian agile boards --project PROJ
atlassian agile sprints --board-id 10 [--state active|future|closed]
atlassian agile sprint active --board-id 10
atlassian agile sprint issues --sprint-id 42 [--output json]

# Sprint management (write)
atlassian agile sprint create \
  --board-id 10 \
  --name "Sprint 9" \
  [--start "2024-01-15T00:00:00.000Z"] \
  [--end "2024-01-29T00:00:00.000Z"]

atlassian agile sprint update --sprint-id 42 [--name "..."] [--state closed]

atlassian agile move-to-sprint --sprint-id 42 --issues "PROJ-1,PROJ-2,PROJ-3"
atlassian agile move-to-epic --epic-key PROJ-100 --issues "PROJ-1,PROJ-2"
```

### Goals

```bash
# Requires cloud ID — get it first
atlassian goals site-id --subdomain myorg

# Read
atlassian goals get "ari:cloud:townsquare:abc:goal/xyz"
atlassian goals search \
  --site-id <cloudId> \
  [--query "status = on_track"] \
  [--max-results 25]

# Write
atlassian goals create \
  --site-id <cloudId> \
  --name "Grow MRR 20% by Q4" \
  --type-id "ari:cloud:goal:<siteId>:goal-type/..." \
  --target-date 2026-12-31 \
  [--confidence QUARTER] \
  [--description "..."]

atlassian goals update \
  --goal-id "ari:cloud:townsquare:..." \
  --status on_track \
  [--score 75] \
  [--summary "Q3 on track, 75% complete"]
```

### Output formats

All commands support `--output table|json|yaml` (default: `table`).

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Usage / user error |
| `2` | Auth / API error |
| `3` | Not found |

---

## MCP Server — 20 Tools

Start the MCP server for your AI client after running `setup`.

### Jira Read

| Tool | Args | Description |
|------|------|-------------|
| `get_jira_issue` | `issue_key` (req) | Get issue by key |
| `search_jira_issues` | `jql` (req), `max_results` (opt, default 50) | Search issues with JQL |

### Jira Write *(requires `ENABLE_WRITE=true`)*

| Tool | Args | Description |
|------|------|-------------|
| `create_jira_issue` | `project_key`, `issue_type`, `summary` (req); `description`, `assignee_id`, `priority`, `labels` (opt) | Create an issue |
| `update_jira_issue` | `issue_key` (req); `summary`, `description`, `assignee_id`, `priority` (opt) | Update issue fields |
| `get_jira_transitions` | `issue_key` (req) | List available workflow transitions |
| `transition_jira_issue` | `issue_key`, `transition_id` (req) | Apply a workflow transition |

### Agile Read

| Tool | Args | Description |
|------|------|-------------|
| `get_jira_boards` | `project_key` (req), `max_results` (opt) | List boards for a project |
| `get_jira_sprints` | `board_id` (req), `state` (opt), `max_results` (opt) | List sprints for a board |
| `get_active_sprint` | `board_id` (req) | Get the active sprint |
| `get_sprint_issues` | `sprint_id` (req), `max_results` (opt) | Issues in a sprint |
| `get_jira_epics` | `project_key` (req) | List epics for a project |

### Agile Write *(requires `ENABLE_WRITE=true`)*

| Tool | Args | Description |
|------|------|-------------|
| `create_sprint` | `name`, `board_id` (req); `start_date`, `end_date` (opt, ISO 8601) | Create a new sprint |
| `update_sprint` | `sprint_id` (req); `state`, `name`, `start_date`, `end_date` (opt) | Update or close a sprint |
| `move_issues_to_sprint` | `sprint_id` (req), `issue_keys` (req, comma-sep, max 50) | Move issues into a sprint |
| `move_issues_to_epic` | `epic_key` (req), `issue_keys` (req, comma-sep) | Link issues to an epic |

### Goals Read

| Tool | Args | Description |
|------|------|-------------|
| `get_site_id` | `subdomain` (req, e.g. `myorg`) | Get Atlassian cloud ID |
| `get_goal` | `goal_id` (req, ARI format) | Get goal by ID |
| `search_goals` | `site_id` (req), `search_string` (opt), `max_results` (opt, default 25), `cursor` (opt) | Search goals |

### Goals Write *(requires `ENABLE_WRITE=true`)*

| Tool | Args | Description |
|------|------|-------------|
| `update_goal_status` | `goal_id`, `status` (req: `on_track`/`off_track`/`at_risk`); `score` (opt, 0-100), `summary` (opt) | Post a check-in |
| `create_goal` | `site_id`, `name`, `goal_type_id`, `target_date` (req); `confidence` (opt, default `QUARTER`), `description` (opt) | Create a goal |

---

## Typical Agent Workflows

### Full sprint cycle

```
1. get_jira_boards      → find board_id for project
2. get_active_sprint    → get current sprint_id
3. get_sprint_issues    → see what's in the sprint
4. search_jira_issues   → find backlog candidates
5. move_issues_to_sprint → load sprint with issues
6. update_sprint        → close sprint (state=closed)
7. create_sprint        → open next sprint
```

### Goal health check & update

```
1. get_site_id          → get cloudId for your org
2. search_goals         → find goals by status/owner
3. get_goal             → inspect a specific goal
4. update_goal_status   → post check-in with score
```

### Bug triage

```
1. search_jira_issues   → find High priority open bugs
2. create_jira_issue    → create new bug if missing
3. get_jira_transitions → discover transition IDs
4. transition_jira_issue → move to In Progress
```

---

## Guardrails

| Feature | Behavior |
|---------|----------|
| Write protection | All MCP write tools require `ENABLE_WRITE=true` — safe by default for AI agents |
| Rate limiting | Retry on HTTP 429 only — with exponential backoff and `Retry-After` header support |
| Token safety | API tokens are never logged or printed to stdout |
| ADF handling | Plain-text descriptions are automatically wrapped to Atlassian Document Format |
| Idempotency | Move operations are idempotent — duplicate issue keys are silently ignored by Jira |

---

## Project Structure

```
atlassian-go-mcp/
├── cmd/
│   ├── atlassian/               # CLI binary
│   │   ├── main.go              # Root command, env validation, service wiring
│   │   ├── jira/                # get, search, create, update, transitions, transition
│   │   ├── agile/               # boards, sprints, sprint-*, move-to-*
│   │   └── goals/               # site-id, get, search, create, update
│   └── mcp/
│       └── main.go              # MCP stdio server (20 tools)
├── internal/
│   ├── atlassian/
│   │   ├── client/              # HTTP client, BasicAuthTransport, RetryTransport
│   │   ├── jira/                # Jira REST v3 — full CRUD + transitions
│   │   ├── agile/               # Jira Agile REST v1.0 — boards, sprints, moves
│   │   └── goals/               # Atlassian Goals GraphQL — list, get, create, update
│   ├── mcp/                     # Tool handlers, WriteGuardCheck, server registration
│   ├── opencode/                # JSON-merge config writer for opencode.json
│   ├── claude/                  # JSON-merge config writer for .claude.json
│   └── output/                  # Formatters: table / json / yaml
├── go.mod
├── go.sum
└── mvp.txt                      # Original product vision
```

---

## Known Limitations

- **`atlassian agile epics`** — not yet available as a CLI command (no `GetEpics` service method; use `search_jira_issues` with JQL `issuetype=Epic` as a workaround)
- **Goal metrics CRUD** — deferred; the Atlassian Goals metrics API has no public documentation at this time. Will be added after schema discovery via `__schema` introspection
- **`goal_type_id`** for `create_goal` — this is a per-tenant ARI. Obtain it from your Atlassian admin or the Goals UI. Format: `ari:cloud:goal:<siteId>:goal-type/<activationId>/<goalTypeId>`
- **Date format** for sprints and goals: ISO 8601 — `2024-01-15T00:00:00.000Z`
- **Assignee** for Jira requires `accountId` (not display name or email). Use the Jira user search API or find it in profile URLs
