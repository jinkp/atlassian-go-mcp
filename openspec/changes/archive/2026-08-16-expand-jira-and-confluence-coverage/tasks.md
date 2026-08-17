# Tasks: Expand Jira Coverage — Phase 1

## Phase 1: Service Layer Foundation

Domain models and service methods that all surfaces depend on.

- [x] 1.1 Add domain types to `internal/atlassian/jira/models.go`: `User`, `Comment`, `IssueLinkType`, `AddWorklogRequest`, `Worklog`, `IssueTypeMeta` structs + API response wrapper types (e.g., `userSearchResponse`, `commentResponse`, etc.)
- [x] 1.2 Implement `adfToPlainText()` function in `internal/atlassian/jira/models.go` — walks ADF JSON, extracts `text` node values, returns plain text string (handles nested `content` arrays)
- [x] 1.3 Add 7 new methods to `Service` interface in `internal/atlassian/jira/service.go`: `LookupAccountID`, `AddComment`, `GetComments`, `LinkIssues`, `GetIssueLinkTypes`, `AddWorklog`, `GetIssueTypeMetadata`
- [x] 1.4 Implement all 7 service methods in `JiraService` struct in `internal/atlassian/jira/service.go` — build correct HTTP request, call Jira API v3, decode response, map status codes (404→NotFound, 401→Unauthorized, 429→RateLimit), return domain model
- [x] 1.5 Add table-driven test cases to `internal/atlassian/jira/service_test.go` for all 7 methods — test success, 404, 401, 429 scenarios per spec; mock `HTTPDoer` with `httptest.NewServer`

**Acceptance**: All 7 service methods callable with correct ADF conversion for `AddComment`/`AddWorklog`; error mapping tested per spec scenarios.

---

## Phase 2: MCP Tool Handlers & Registration

Wire service methods into MCP protocol layer.

- [x] 2.1 Create `internal/mcp/tools_jira_extra.go` — define 7 tool handler functions (`ToolLookupJiraAccountID`, `ToolAddCommentToIssue`, `ToolGetIssueComments`, `ToolLinkIssues`, `ToolGetIssueLinkTypes`, `ToolAddWorklog`, `ToolGetIssueTypeMetadata`)
- [x] 2.2 Implement read tool handlers (4): parse MCP input, call service method, serialize domain model to snake_case JSON output structs, return via `mcp.NewToolResultText()`
- [x] 2.3 Implement write tool handlers (3): call `WriteGuardCheck()` first, then service method, call `audit.Logger.Log()`, serialize result to JSON
- [x] 2.4 Register 4 read tools in `internal/mcp/server.go` inside existing jira-read feature block; register 3 write tools in jira-write block — use `s.AddTool()` with tool names per spec
- [x] 2.5 Update `internal/mcp/features/features.go`: change `ModuleJira` count from `{4, 3}` to `{8, 6}` to reflect 4 new reads + 3 new writes
- [x] 2.6 Create `internal/mcp/tools_jira_extra_test.go` — test each tool handler with mocked `jira.Service` interface; verify write guards + audit logging for write tools

**Acceptance**: MCP tools callable and verified in tests; all 3 surfaces (MCP/REST/CLI) counted in features; `WriteGuardCheck` enforced for write tools.

---

## Phase 3: REST API Handlers & Routes

Expose service layer via HTTP.

- [x] 3.1 Add 7 handler methods to `internal/api/handlers/jira.go`: `SearchUsers`, `AddComment`, `GetComments`, `LinkIssues`, `GetIssueLinkTypes`, `AddWorklog`, `GetIssueTypeMetadata` — parse query/path params, call service, write JSON response
- [x] 3.2 Register all write handlers with `api.WriteGuardCheck()` middleware before API call; ensure audit logging
- [x] 3.3 Register 7 new routes in `cmd/api/main.go`: `GET /jira/users/search`, `POST /jira/issues/{key}/comments`, `GET /jira/issues/{key}/comments`, `POST /jira/issues/links`, `GET /jira/issues/link-types`, `POST /jira/issues/{key}/worklogs`, `GET /jira/projects/{key}/issue-types`
- [x] 3.4 Expand `mockJiraService` in `internal/api/handlers/jira_test.go` with 7 new function fields matching all new service methods
- [x] 3.5 Add table-driven test cases to `internal/api/handlers/jira_test.go` for all 7 handlers — success, 404, 401, 429 paths per spec

**Acceptance**: All handlers callable via HTTP; `WriteGuardCheck` verified; test coverage for success + error scenarios.

---

## Phase 4: CLI Commands & Wiring

Expose service layer via command-line.

- [x] 4.1 Create `cmd/atlassian/jira/users.go` — `jira users search --query <q> [--max-results N]` command, call `LookupAccountID`, print formatted table or JSON
- [x] 4.2 Create `cmd/atlassian/jira/comment.go` — two commands: `jira comment add <KEY> <body>` (calls `AddComment`) and `jira comment list <KEY>` (calls `GetComments`)
- [x] 4.3 Create `cmd/atlassian/jira/link.go` — two commands: `jira link <INWARD> <OUTWARD> --type <name>` (calls `LinkIssues`) and `jira link-types` (calls `GetIssueLinkTypes`)
- [x] 4.4 Create `cmd/atlassian/jira/worklog.go` — `jira worklog add <KEY> --time-spent <s> [--comment <c>] [--started <t>]` command, call `AddWorklog`, handle optional fields
- [x] 4.5 Create `cmd/atlassian/jira/issuetypes.go` — `jira issue-types <PROJECT-KEY>` command, call `GetIssueTypeMetadata`, print formatted table or JSON
- [x] 4.6 Update `cmd/atlassian/jira/root.go` — add `AddCommand()` calls for all 5 new command files
- [x] 4.7 Add smoke-level tests to each command file following pattern in `create_test.go` (mock service, capture stdout)

**Acceptance**: All 5 CLI commands wired and callable; smoke tests pass per existing pattern.

---

## Phase 5: Verification

Final validation and quality checks.

- [x] 5.1 Run `go test ./...` — ALL packages `ok`, 0 FAIL (service, MCP, REST, CLI layers + no regressions)
- [x] 5.2 Run `go vet ./...` — clean, no type-safety issues
- [x] 5.3 Run `gofmt -l .` — new code matches existing file style; repo has PRE-EXISTING gofmt drift (Go pre-1.19 doc-comment format, confirmed via git stash on untouched files) — NOT reformatted here (belongs in a separate style commit)
- [ ] 5.4 Manual smoke test against a live Atlassian site (MCP + CLI + REST) — DEFERRED: requires live credentials; automated tests cover happy + error paths per spec

**Acceptance**: All tests green; `go vet` clean; new code style-consistent with repo; 3 surfaces implemented & unit/integration-tested. Live e2e smoke deferred (needs real creds).
