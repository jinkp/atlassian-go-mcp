# Tasks: Confluence Module and Lean Default

## Block 1: Confluence Service Layer (Foundation)

- [x] 1.1 Create `internal/atlassian/confluence/models.go`: sentinel errors (ErrNotFound, ErrUnauthorized, ErrRateLimit, ErrConflict) + domain types (Page, PageRef, Space, Comment, SearchResult + pagination list types)
- [x] 1.2 Add raw API response structs to `models.go`: pageAPIResponse, spaceAPIResponse, commentAPIResponse, searchItemAPIResponse + ToModel converters + confLinks envelope
- [x] 1.3 Add helpers to `models.go`: `plainTextToStorage(text string) string` (wraps text in `<p>…</p>`) + `extractCursor(next string) string` (parses cursor from URL)
- [x] 1.4 Create `internal/atlassian/confluence/service.go`: Service interface + ConfluenceService struct + NewService factory
- [x] 1.5 Implement all 12 Service methods in `service.go`: GetPage, GetPagesInSpace, GetSpaces, GetPageDescendants, GetFooterComments, GetInlineComments, GetCommentChildren, CreatePage, UpdatePage, CreateFooterComment, CreateInlineComment, SearchContent
- [x] 1.6 Implement UpdatePage version-fetch logic in `service.go`: when version_number omitted, internal GET → current+1; 409 → ErrConflict mapping
- [x] 1.7 Create `internal/atlassian/confluence/service_test.go`: table-driven tests for all 12 methods covering happy path + 404/401/409/429 error codes + UpdatePage auto-increment + cursor extraction

## Block 2: MCP Layer

- [x] 2.1 Create `internal/mcp/tools_confluence.go`: output structs (mcpPageJSON, mcpSpaceJSON, mcpCommentJSON, mcpSearchJSON, mcpPageListJSON, etc.) all snake_case
- [x] 2.2 Implement 8 read tool handlers in `tools_confluence.go`: ToolGetConfluencePage, ToolGetPagesInSpace, ToolGetSpaces, ToolGetPageDescendants, ToolGetFooterComments, ToolGetInlineComments, ToolGetCommentChildren, ToolSearchConfluence
- [x] 2.3 Implement 4 write tool handlers in `tools_confluence.go`: ToolCreateConfluencePage, ToolUpdateConfluencePage, ToolCreateFooterComment, ToolCreateInlineComment (with WriteGuardCheck + audit.Log)
- [x] 2.4 Add validation to ToolCreateInlineComment: require text_selection field; error before API call if missing
- [x] 2.5 Ensure empty-slice rule in `tools_confluence.go`: use `make([]T, 0)` for empty results so json.Marshal emits `[]` not `null`
- [x] 2.6 Add ModuleConfluence to `internal/mcp/features/features.go`: const ModuleConfluence = "confluence" + add to allModules + add {8,4} to moduleToolCounts
- [x] 2.7 Update `internal/mcp/server.go`: extend NewAtlassianServer + StartServer signatures to accept confluenceSvc parameter
- [x] 2.8 Register read tools in `internal/mcp/server.go`: add confluence-read block gated by fs.IsEnabled(ModuleConfluence, false) for 8 tools (incl. search)
- [x] 2.9 Register write tools in `internal/mcp/server.go`: add confluence-write block gated by fs.IsEnabled(ModuleConfluence, true) for 4 tools
- [x] 2.10 Create `internal/mcp/tools_confluence_test.go`: handler tests with mock Service + WriteGuardCheck tests for 4 write tools
- [x] 2.11 Update `internal/mcp/features/features_test.go`: verify ModuleConfluence in allModules + verify counts {8,4}
- [x] 2.12 Update `internal/mcp/server_test.go`: update total tool count assertions (67 → 79) + confluence count (0 → 12)

## Block 3: REST API Layer

- [x] 3.1 Create `internal/api/handlers/confluence.go`: ConfluenceHandler struct + NewConfluenceHandler factory
- [x] 3.2 Implement 12 handler methods in `confluence.go`: GetPage, GetPagesInSpace, GetSpaces, GetPageDescendants, GetFooterComments, GetInlineComments, GetCommentChildren, SearchContent, CreatePage, UpdatePage, CreateFooterComment, CreateInlineComment
- [x] 3.3 Add audit.Log calls in write handlers (CreatePage, UpdatePage, CreateFooterComment, CreateInlineComment)
- [x] 3.4 Add `ConfluenceSvc()` accessor to `internal/api/server.go` following existing pattern (e.g., JiraSvc)
- [x] 3.5 Register routes in `cmd/api/main.go`: 8 read routes (GET /confluence/*) + 4 write routes (POST/PUT /confluence/*) under global WriteGuardMiddleware
- [x] 3.6 Create `internal/api/handlers/confluence_test.go`: route handler tests with httptest + mock Service
- [x] 3.7 Update test assertions in relevant handler tests to reflect new route count (no count assertions existed — N/A)

## Block 4: CLI Layer

- [x] 4.1 Create `cmd/atlassian/confluence/root.go`: NewConfluenceCmd() + RegisterCommands() function
- [x] 4.2 Create `cmd/atlassian/confluence/page.go`: page get, page list (GetPagesInSpace), page descendants, page create, page update commands
- [x] 4.3 Create `cmd/atlassian/confluence/spaces.go`: spaces list command
- [x] 4.4 Create `cmd/atlassian/confluence/comment.go`: comment footer, comment inline, comment children, comment add-footer, comment add-inline commands
- [x] 4.5 Create `cmd/atlassian/confluence/search.go`: search CQL command
- [x] 4.6 Create nilConfluenceService stub in `cmd/atlassian/main.go` implementing all 12 Service methods returning error
- [x] 4.7 Wire confluenceSvc in `cmd/atlassian/main.go`: import confluence pkg, instantiate service, pass to RegisterCommands
- [x] 4.8 Add confluence subcommand to root in `cmd/atlassian/main.go`
- [x] 4.9 Create smoke tests for write commands in respective `*.go` files (dry-run mode tests)

## Block 5: Feature Flags + Lean Default

- [x] 5.1 Ensure ModuleConfluence constant + counts complete in `internal/mcp/features/features.go` (from Block 2.6, validate here)
- [x] 5.2 Update `allEnabled()` in `features/features.go` to include ModuleConfluence in "all" profile
- [x] 5.3 Add confluence row to `internal/tui/model.go` defaultModules with Enabled: false
- [x] 5.4 Update `internal/tui/connector.go` to gate confluence module registration by feature flag
- [x] 5.5 Investigate `cmd/mcp/main.go` setup write path: ensure fresh install writes lean-default profile (excluding confluence) while `--enable all` includes confluence
- [x] 5.6 Update `--enable` flag help text in `cmd/mcp/main.go` to clarify lean-default behavior
- [x] 5.7 Update Diagnostics in `features/features.go` to reference ModuleConfluence
- [x] 5.8 Verify feature-flags tests in `features_test.go`: assert `Parse("all")` includes confluence but lean/default excludes it

## Block 6: Verification & Integration

- [x] 6.1 Run `go test ./...` across all layers; verify no regressions on existing 67 tools
- [x] 6.2 Verify total tool count = 79 (67 existing + 12 confluence); verify confluence count = 12 in assertions
- [x] 6.3 Run `go vet ./...` for type checking; confirm no errors
- [x] 6.4 Verify three-surface cross-cutting requirement: each of 12 tools present in MCP (tools_confluence.go) + CLI (confluence/\*.go) + REST (handlers/confluence.go + routes)
- [x] 6.5 Verify design decisions baked into code: (1) storage XHTML pass-through + plainTextToStorage helper (2) UpdatePage optimistic locking with internal GET when version omitted (3) cursor extraction from response body _links.next
- [x] 6.6 Verify ConfluenceSvc() accessor added to `internal/api/server.go` + signature extended (no breaking changes to existing services)
- [x] 6.7 Spot-check error mapping: validate sentinel errors used consistently (404→ErrNotFound, 401/403→ErrUnauthorized, 409→ErrConflict, 429→ErrRateLimit)
- [x] 6.8 Confirm lint compliance (`go vet`); do NOT run `go build` per project rules
- [x] 6.9 Do NOT commit; await orchestrator instruction for archival phase

---

## Execution Notes

- **Dependency order**: Block 1 (Service) → Block 2 (MCP) → Block 3 (REST) → Block 4 (CLI) → Block 5 (Feature flags). Block 6 runs last.
- **Three-surface rule**: Each tool must be callable from all 3 surfaces. Use Ctrl+F to verify tool name appears in tools_confluence.go, confluence/\*.go, and handlers/confluence.go.
- **Testing**: Table-driven per Block 1 convention; httptest mocks for HTTP layers; mock Service interface for handler tests.
- **No build/commit**: Project rule states "never build after changes". Test compilation via `go test ./...` is sufficient verification.
- **Empty slices**: Strictly enforce `[]` in JSON output for empty lists; never `null`.
- **Lean default**: Block 5 finalizes the feature-flag wiring so fresh installs exclude Confluence by default, but `--enable all` and `--enable confluence` include it.

Total tasks: **51**  
Suggested session budget: ~8 hours (6–7 tasks per session, bottom-up by dependency).
