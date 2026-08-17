# Proposal: Expand Jira and Confluence Coverage

---

## ARCHIVE-NOTE

**Archived**: 2026-08-16

**Status**: Phase 1 (Jira 7 tools) DELIVERED & VERIFIED ✅

**Phase Breakdown**:
- ✅ **Phase 1 (Jira)**: 7 tools (`lookup_jira_account_id`, `add_comment_to_issue`, `get_issue_comments`, `link_issues`, `get_issue_link_types`, `add_worklog`, `get_issue_type_metadata`) implemented across MCP + CLI + REST. All tests passing. All scenarios verified.
- 🔄 **Phases 2–3 (Confluence + lean-default wiring)**: Deferred to a NEW separate SDD change. The full design and requirements remain in this proposal for future reference.

**For Next Phase**: Create a new SDD change to implement Confluence module (Phase 2) and update feature-flags + lean default wiring (Phase 3). The design and full scope are documented here under "Scope" and "Affected Areas".

---

## Intent

Close the coverage gap vs. the official Atlassian Rovo MCP: add 7 missing Jira operations (parity with official's 14 tools) and introduce a first-class Confluence module (12 tools). Token footprint is controlled via a **lean default install** that excludes Confluence unless the user opts in.

## Scope

### In Scope

**Phase 1 — Jira fine-grained tools** (jira module: 7 → 14 tools, across MCP + CLI + REST)
- `lookup_jira_account_id` (read) — user search by name/email
- `add_comment_to_issue` (write) — ADF body via existing `plainTextToADF()`
- `get_issue_comments` (read)
- `link_issues` (write) — POST issueLink
- `get_issue_link_types` (read)
- `add_worklog` (write)
- `get_issue_type_metadata` (read)

**Phase 2 — Confluence module** (12 tools, new domain package, across MCP + CLI + REST)

| Group | Tools (7 read + 4 write + 1 search) |
|-------|--------------------------------------|
| Read | get_confluence_page, get_pages_in_space, get_confluence_spaces, get_page_descendants, get_page_footer_comments, get_page_inline_comments, get_comment_children |
| Write | create_confluence_page, update_confluence_page, create_footer_comment, create_inline_comment |
| Search | search_confluence (CQL via REST v1) |

**Phase 3 — Wiring & lean default**
- `ModuleConfluence` in `features.go`; counts `{8,4}` (read/write)
- TUI: new module row, `Enabled: false` in `defaultModules`
- `--enable all` includes Confluence; fresh install writes an explicit list excluding it
- Update flag help, diagnostics, and `allModules` slice

### Out of Scope
- Jira Service Management, Compass, Teamwork Graph
- OAuth 2.1 (keep API-token BasicAuth)
- Confluence attachments, labels, whiteboards, databases
- Remote issue links

## Capabilities

### New Capabilities
- `confluence`: Cloud read/write/search (pages, spaces, comments, CQL) via MCP + CLI + REST

### Modified Capabilities
- `jira`: add comment, worklog, account lookup, issue linking, link-type lookup, issue-type metadata
- `feature-flags`: add `ModuleConfluence`; lean default excludes Confluence; `all` still includes it

## Approach

Reuse existing `client.NewClient` (BasicAuth, same credentials) — no new auth setup. Confluence domain package follows the established service/models pattern. Confluence v2 API for page/space/comment CRUD; v1 REST for CQL search. Jira additions extend `internal/atlassian/jira/service.go`. All write tools go through `WriteGuardCheck()` + audit log. Each phase is independently mergeable.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/atlassian/jira/{service.go,models.go}` | Modified | 7 new service methods |
| `internal/atlassian/confluence/` | New | service.go, models.go, tests |
| `internal/mcp/{server.go,tools_confluence.go}` | Modified/New | Confluence MCP registration |
| `cmd/atlassian/jira/*` | Modified | New CLI sub-commands |
| `cmd/atlassian/confluence/*` | New | CLI group for Confluence |
| `internal/api/handlers/{jira.go,confluence.go}` | Modified/New | REST handlers + routes |
| `internal/mcp/features/features.go` | Modified | ModuleConfluence, updated counts |
| `internal/tui/{model.go,connector.go}` | Modified | New module row, lean default |
| `cmd/mcp/main.go` | Modified | Flag help, default install profile |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Confluence v2 vs v1 CQL split | Med | Validate endpoints in design phase via Context7/docs |
| Lean default confuses users expecting `all` | Low | TUI shows toggle state; `--help` documents lean default |
| Token footprint grows for `all` users | Low | Lean default; per-module `--enable`; clear docs |
| Tool confusion at ~79 total tools | Med | Module gating; clear naming convention |

## Rollback Plan

Each phase is additive. Rollback per phase: remove the module registration block in `server.go` + delete the domain package (Confluence) or comment out new tool registrations (Jira). `features.go` counts revert. No data migrations. Confluence can be disabled at runtime via `--enable` without code changes.

## Dependencies

- Confluence Cloud API v2 (pages, spaces, comments) and v1 (CQL search) — publicly documented, no new credentials
- Existing `internal/atlassian/client` transport layer (reused as-is)

## Success Criteria

- [ ] `jira` module exposes exactly 14 tools; `confluence` module exposes 12
- [ ] Fresh install default excludes Confluence; `--enable all` includes it
- [ ] All new tools work across MCP + CLI + REST
- [ ] `go test ./...` passes; no regressions on existing tools
- [ ] No new credentials required — same API token covers Confluence
- [ ] `lookup_jira_account_id` and `add_comment_to_issue` are P1 (unblocks existing create/update flows)
