# Proposal: Confluence Module and Lean Default

## Intent

Close the largest coverage gap vs. official Atlassian Rovo MCP: introduce a 12-tool Confluence module (we have zero today) and wire a lean default install so the expanded tool count does not bloat every user's token budget without opt-in.

## Scope

### In Scope

**Phase A — Confluence module (12 tools, new domain package, all 3 surfaces)**

| Group | Tools |
|-------|-------|
| Read (7) | `get_confluence_page`, `get_pages_in_space`, `get_confluence_spaces`, `get_page_descendants`, `get_page_footer_comments`, `get_page_inline_comments`, `get_comment_children` |
| Write (4) | `create_confluence_page`, `update_confluence_page`, `create_footer_comment`, `create_inline_comment` |
| Search (1) | `search_confluence` (CQL via v1 `/wiki/rest/api/search`) |

New package `internal/atlassian/confluence/` (service.go + models.go + tests).
New `internal/mcp/tools_confluence.go`. New `cmd/atlassian/confluence/` CLI group.
New `internal/api/handlers/confluence.go` + routes in `cmd/api/main.go`.

**Phase B — Feature-flag wiring + lean default**
- Add `ModuleConfluence` to `internal/mcp/features/features.go`; counts `{8, 4}` (8 read incl. search, 4 write).
- `defaultModules` in TUI gets a confluence row with `Enabled: false`.
- `--enable all` still includes Confluence; fresh setup writes an explicit list that excludes it.
- Update `allModules`, `moduleToolCounts`, `Diagnostics`, TUI `model.go`/`connector.go`.

### Out of Scope
- OAuth 2.1 / new credentials (same API-token BasicAuth)
- Confluence attachments, labels, whiteboards, databases, custom content, blog posts
- Storage-format editor helpers (accept XHTML body as-is; plain-text wrapper is a design decision)
- Jira changes (Phase 1 already delivered)

## Capabilities

### New Capabilities
- `confluence`: Confluence Cloud read/write/search via MCP + CLI + REST (pages, spaces, comments, CQL)

### Modified Capabilities
- `feature-flags`: add `ModuleConfluence`; lean default-install profile excludes Confluence; `--enable all` still includes it

## Approach

Reuse `client.NewClient` (same BasicAuth, same `cfg.BaseURL` with `/wiki` path prefix — no new credentials). Confluence domain package mirrors the established `service.go + models.go` pattern. Page/space/comment CRUD via v2 (`/wiki/api/v2/`); CQL search stays on v1 (`/wiki/rest/api/search`). Write tools call `WriteGuardCheck()` + audit log. List endpoints use cursor pagination (Link header `next`). Page updates require `version.number = current + 1` (optimistic locking); design will decide: fetch version internally or require caller to pass it. Body is Confluence **storage format** (XHTML) — `plainTextToADF()` is NOT reusable; design decides body handling.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/atlassian/confluence/` | New | service.go, models.go, service_test.go |
| `internal/mcp/tools_confluence.go` | New | 12 MCP tool handlers |
| `internal/mcp/server.go` | Modified | Register confluence read/write blocks |
| `internal/mcp/features/features.go` | Modified | ModuleConfluence, counts, allModules, Diagnostics |
| `cmd/atlassian/confluence/` | New | CLI group (pages, spaces, comments, search) |
| `internal/api/handlers/confluence.go` | New | REST handlers |
| `cmd/api/main.go` | Modified | New confluence routes |
| `internal/tui/{model.go,connector.go}` | Modified | New module row, lean default (Enabled: false) |
| `cmd/mcp/main.go` | Modified | Lean default install profile, --enable help |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Storage (XHTML) body vs ADF — not reusable | Med | Design decides: pass-through + optional plain-text wrapper |
| Update page optimistic locking (version.number) | Med | Design decides: internal fetch or caller-supplied version |
| Lean-default breaks `--enable all` semantics | Med | Investigate setup write path in design; contract: `all` always includes Confluence |
| Cursor pagination differs from Jira offset | Low | Expose `limit` + optional `cursor` param uniformly |
| Tool count grows to ~79 total | Low | Module gating + lean default keeps MCP context lean |

## Rollback Plan

Purely additive. Rollback = delete `internal/atlassian/confluence/`, remove confluence registration from `server.go`, revert `features.go` (remove `ModuleConfluence`, revert counts/allModules), remove TUI row. No data migrations. Confluence disabled at runtime via `--enable` flag without any code changes.

## Success Criteria

- [ ] Confluence module exposes exactly 12 tools across MCP + CLI + REST
- [ ] Fresh install default EXCLUDES Confluence; `--enable all` and `--enable confluence` include it
- [ ] No new credentials required — same API token covers Confluence
- [ ] `go test ./...` passes; no regressions on existing tools
- [ ] CQL search works via v1; page CRUD and comments via v2
- [ ] Update page enforces `version.number` increment (no silent clobber)
