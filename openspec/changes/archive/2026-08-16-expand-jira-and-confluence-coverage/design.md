# Design: Expand Jira Coverage — Phase 1 (7 New Jira Tools)

## Technical Approach

Extend the existing three-surface hexagonal architecture (service → MCP/REST/CLI) with 7 new Jira operations. All additions follow the established raw-API-response → domain-model → JSON-output pipeline. The 4 read tools extend the read block in `server.go`; the 3 write tools extend the write block and call `WriteGuardCheck()` + audit log before any API call. No new packages, no new credentials.

## Architecture Decisions

| # | Decision | Choice | Rejected | Rationale |
|---|----------|--------|----------|-----------|
| 1 | New file vs. extend `tools.go` | New **`internal/mcp/tools_jira_extra.go`** | Appending to `tools.go` | `tools.go` is already 300 lines; domain cohesion within file, same package `mcpserver` so no import cycle |
| 2 | ADF → plain text for `get_issue_comments` | **Minimal recursive extractor** `adfToPlainText()` in `models.go` | Return raw ADF JSON / rely on caller | Raw ADF is useless to MCP consumers; a 10-line walker covering `text` nodes covers 95% of real comments |
| 3 | `createmeta` response shape | **Decode `values []` with fallback to `issueTypes []`** — try `values` key first, fall back to top-level `issueTypes` | Hardcode one key | Jira Cloud returns `values`; Server/DC returns `issueTypes`; single decode struct with two optional fields handles both |
| 4 | `time_spent` parsing for worklog | **Pass-through string** to Jira API as-is | Pre-parse to seconds | Jira parses "3h 30m", "2h", "30m" natively; pre-parsing adds risk of divergence with Jira's own rules; user gets natural syntax |

## Data Flow

### Write tool sequence (all 3 write tools)

```
MCP/CLI/REST caller
  → WriteGuardCheck()          ← returns error if ENABLE_WRITE != "true"
  → jira.Service.XxxMethod()  ← builds ADF body where needed, calls Jira API
  → audit.Logger.Log()        ← records outcome (success or error) after call
  → serialise domain model to snake_case JSON
  → mcp.NewToolResultText / api.RespondJSON / fmt.Fprintln
```

### Read tool sequence (all 4 read tools)

```
MCP/CLI/REST caller
  → jira.Service.XxxMethod()  ← GET Jira REST API v3
  → switch(statusCode)        ← map 404/401/429 to sentinel errors
  → decode raw APIResponse    → ToModel() → domain struct
  → serialise to snake_case JSON
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/atlassian/jira/models.go` | Modify | Add 5 new domain types + API response structs + `adfToPlainText()` |
| `internal/atlassian/jira/service.go` | Modify | Add 7 new methods to `Service` interface + `JiraService` impl |
| `internal/mcp/tools_jira_extra.go` | **Create** | 7 MCP tool handlers (ToolLookupJiraAccountID, ToolAddCommentToIssue, ToolGetIssueComments, ToolLinkIssues, ToolGetIssueLinkTypes, ToolAddWorklog, ToolGetIssueTypeMetadata) |
| `internal/mcp/server.go` | Modify | Register 4 new read tools in jira-read block; 3 new write tools in jira-write block |
| `internal/mcp/features/features.go` | Modify | `moduleToolCounts[ModuleJira]` → `{8, 6}` |
| `internal/api/handlers/jira.go` | Modify | 7 new handler methods |
| `cmd/api/main.go` | Modify | Register 7 new routes |
| `cmd/atlassian/jira/comment.go` | **Create** | `jira comment add <key> <body>` + `jira comment list <key>` |
| `cmd/atlassian/jira/link.go` | **Create** | `jira link <inward> <outward> --type <name>` + `jira link-types` |
| `cmd/atlassian/jira/worklog.go` | **Create** | `jira worklog add <key> --time-spent <s> [--comment <c>] [--started <t>]` |
| `cmd/atlassian/jira/users.go` | **Create** | `jira users search --query <q> [--max-results N]` |
| `cmd/atlassian/jira/issuetypes.go` | **Create** | `jira issue-types <project-key>` |
| `cmd/atlassian/jira/root.go` | Modify | Add `AddCommand()` calls for all new sub-commands |
| `internal/atlassian/jira/service_test.go` | Modify | New table-driven cases for 7 methods |
| `internal/mcp/tools_jira_extra_test.go` | **Create** | Handler tests per tool (mock service, mock httptest) |
| `internal/api/handlers/jira_test.go` | Modify | New cases + expand `mockJiraService` with 7 new func fields |

## Interfaces / Contracts

### Service interface additions (`service.go`)

```go
// New methods added to Service interface:
LookupAccountID(ctx context.Context, query string, maxResults int) ([]User, error)
AddComment(ctx context.Context, key string, body string) (*Comment, error)
GetComments(ctx context.Context, key string, maxResults int) ([]Comment, error)
LinkIssues(ctx context.Context, inward, outward, linkTypeName string) error
GetIssueLinkTypes(ctx context.Context) ([]IssueLinkType, error)
AddWorklog(ctx context.Context, key string, req AddWorklogRequest) (*Worklog, error)
GetIssueTypeMetadata(ctx context.Context, projectKey string) ([]IssueTypeMeta, error)
```

### New domain types (`models.go`)

```go
type User struct {
    AccountID   string
    DisplayName string
    Email       string
    Active      bool
}

type Comment struct {
    ID      string
    Author  string
    Body    string    // plain text extracted from ADF
    Created time.Time
    Updated time.Time
}

type IssueLinkType struct {
    ID      string
    Name    string
    Inward  string
    Outward string
}

type AddWorklogRequest struct {
    TimeSpent string            // passed through as-is, e.g. "3h 30m"
    Comment   string            // optional plain text → ADF
    Started   string            // optional ISO 8601; forwarded as-is
}

type Worklog struct {
    ID               string
    TimeSpentSeconds int
    Started          time.Time
    Author           string
}

type IssueTypeMeta struct {
    ID       string
    Name     string
    Desc     string
    Subtask  bool
}
```

### ADF plain-text extractor (new, in `models.go`)

```go
// adfToPlainText walks ADF JSON and concatenates all "text" node values.
// Handles nested content arrays. Returns "" on malformed input.
func adfToPlainText(adf map[string]interface{}) string
```

### MCP output structs (`tools_jira_extra.go`)

```go
type userJSON struct {
    AccountID   string `json:"account_id"`
    DisplayName string `json:"display_name"`
    Email       string `json:"email"`
    Active      bool   `json:"active"`
}
type commentJSON struct {
    ID      string `json:"id"`
    Author  string `json:"author"`
    Body    string `json:"body"`
    Created string `json:"created,omitempty"`
    Updated string `json:"updated,omitempty"`
}
type issueLinkTypeJSON struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Inward  string `json:"inward"`
    Outward string `json:"outward"`
}
type worklogJSON struct {
    ID               string `json:"id"`
    TimeSpentSeconds int    `json:"time_spent_seconds"`
    Started          string `json:"started,omitempty"`
    Author           string `json:"author"`
}
type issueTypeMetaJSON struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Desc    string `json:"description"`
    Subtask bool   `json:"subtask"`
}
```

### REST API routes (additions to `cmd/api/main.go`)

```
GET  /jira/users/search?query=&maxResults=     → jiraH.SearchUsers
POST /jira/issues/{key}/comments               → jiraH.AddComment
GET  /jira/issues/{key}/comments               → jiraH.GetComments
POST /jira/issues/links                        → jiraH.LinkIssues
GET  /jira/issues/link-types                   → jiraH.GetIssueLinkTypes
POST /jira/issues/{key}/worklogs               → jiraH.AddWorklog
GET  /jira/projects/{key}/issue-types          → jiraH.GetIssueTypeMetadata
```

### CLI command tree additions (`root.go`)

```
atlassian jira users search --query <q> [--max-results N]
atlassian jira comment add <KEY> <body>
atlassian jira comment list <KEY>
atlassian jira link <INWARD> <OUTWARD> --type <name>
atlassian jira link-types
atlassian jira worklog add <KEY> --time-spent <s> [--comment <c>] [--started <t>]
atlassian jira issue-types <PROJECT-KEY>
```

### `moduleToolCounts` update

```go
// features.go — was {4, 3}
ModuleJira: {8, 6},
// Comment: 4 existing read + 4 new read = 8; 3 existing write + 3 new write = 6
```

### MCP server registration blocks (`server.go`)

```go
// Inside if fs.IsEnabled(features.ModuleJira, false) { ... }
// Add after existing 4 read tools:
s.AddTool(mcp.NewTool("lookup_jira_account_id", ...), ToolLookupJiraAccountID(svc))
s.AddTool(mcp.NewTool("get_issue_comments", ...), ToolGetIssueComments(svc))
s.AddTool(mcp.NewTool("get_issue_link_types", ...), ToolGetIssueLinkTypes(svc))
s.AddTool(mcp.NewTool("get_issue_type_metadata", ...), ToolGetIssueTypeMetadata(svc))

// Inside if fs.IsEnabled(features.ModuleJira, true) { ... }
// Add after existing 3 write tools:
s.AddTool(mcp.NewTool("add_comment_to_issue", ...), ToolAddCommentToIssue(svc, log))
s.AddTool(mcp.NewTool("link_issues", ...), ToolLinkIssues(svc, log))
s.AddTool(mcp.NewTool("add_worklog", ...), ToolAddWorklog(svc, log))
```

### Jira API endpoints used

| Method | Endpoint | Success code |
|--------|----------|-------------|
| GET | `/rest/api/3/user/search?query={q}&maxResults={n}` | 200 (array) |
| POST | `/rest/api/3/issue/{key}/comment` | 201 — returns comment JSON |
| GET | `/rest/api/3/issue/{key}/comment?maxResults={n}` | 200 — `{comments:[…]}` |
| POST | `/rest/api/3/issueLink` | 201 — empty body |
| GET | `/rest/api/3/issueLinkType` | 200 — `{issueLinkTypes:[…]}` |
| POST | `/rest/api/3/issue/{key}/worklog` | 201 — returns worklog JSON |
| GET | `/rest/api/3/issue/createmeta/{key}/issuetypes` | 200 — try `values[]`, fallback `issueTypes[]` |

## Testing Strategy

| Layer | Test File | What to Test |
|-------|-----------|-------------|
| Service unit | `internal/atlassian/jira/service_test.go` | 7 new methods; mock `HTTPDoer` via `httptest.NewServer`; table cases: success, 404, 401, 429, 400 |
| MCP handlers | `internal/mcp/tools_jira_extra_test.go` (new) | Each `ToolXxx` func; mock `jira.Service` interface; verify `WriteGuardCheck` path for write tools |
| REST handlers | `internal/api/handlers/jira_test.go` | Expand `mockJiraService` with 7 new func fields; table cases per new route |
| CLI | `cmd/atlassian/jira/` | No strict TDD; smoke-level tests matching `create_test.go` pattern (mock service, capture stdout) |

## Migration / Rollout

No migration required. All additions are purely additive. Rollback: remove new `s.AddTool` calls from `server.go`, remove new routes from `cmd/api/main.go`, revert `moduleToolCounts` to `{4, 3}`. No data is created server-side by the integration itself.

## Open Questions

- [ ] **`get_issue_comments` pagination**: The spec says `max_results` optional. The Jira Cloud `GET /issue/{key}/comment` also supports `startAt`. Should we expose `startAt` now or defer cursor-based pagination to a future iteration? (Recommendation: defer — add only `maxResults` query param for Phase 1.)
- [ ] **`lookup_jira_account_id` max_results default**: Spec says 50. Confirm this is acceptable vs. a smaller default (e.g. 10) to reduce token footprint in MCP context.
- [ ] **Worklog `started` validation**: Should the service validate that `started` is a valid ISO 8601 string before sending, or pass through and let Jira return a 400? (Recommendation: pass-through — same pattern as `time_spent`; Jira error message is descriptive enough.)
