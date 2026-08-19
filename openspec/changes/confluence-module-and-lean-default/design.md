# Design: Confluence Module (Phase A)

## Technical Approach

New domain package `internal/atlassian/confluence/` mirrors the `jira` package exactly: `Service` interface + `ConfluenceService` struct + `models.go` with raw API types, domain types, and `ToModel` converters. The same `client.HTTPDoer` + `BasicAuth` transport is reused — no new credentials. Endpoints use `cfg.BaseURL + "/wiki/api/v2/..."` (v2 for CRUD) and `cfg.BaseURL + "/wiki/rest/api/search"` (v1 for CQL). Tools are registered in a new `tools_confluence.go` gated by `features.ModuleConfluence`. CLI tree is `atlassian confluence ...`. REST routes are `/confluence/*` wired in `cmd/api/main.go`.

---

## Architecture Decisions

| # | Decision | Choice | Alternatives | Rationale |
|---|----------|--------|--------------|-----------|
| 1 | Body format | Storage/XHTML pass-through with optional `plainTextToStorage(s string) string` helper | ADF converter | Confluence v2 expects `{representation:"storage",value:"<xhtml>"}`. ADF is Jira-only. Caller controls content. Tiny helper wraps plain text in `<p>...</p>` for convenience; use is optional. |
| 2 | Update page version | Service fetches current version internally when `version_number` omitted (extra GET then PUT) | Require caller to supply version | Single tool call UX. Service-level GET hides API complexity. 409 on stale version maps to `ErrConflict`. Trade-off: one extra API round-trip. |
| 3 | Cursor extraction | Parse `cursor=` query param from response body `_links.next` URL | Parse `Link` response header | Body-based is simpler (no header parsing), works even when body is fully buffered, aligns with Confluence API docs. Helper: `extractCursor(links confLinks) string` parses `_links.next`. |
| 4 | BaseURL + `/wiki` prefix | Confluence service builds `s.baseURL + "/wiki/api/v2/..."` and `s.baseURL + "/wiki/rest/api/..."` | Separate `wikiBaseURL` field | `cfg.BaseURL` is the site root (e.g. `https://x.atlassian.net`). Jira appends `/rest/api/3`. Confluence appends `/wiki/...`. Same `baseURL` field, different prefix per service. Confirmed from `NewService(doer, baseURL)` pattern. |
| 5 | MCP file split | Single `tools_confluence.go` (all 12 handlers) | Split read/write files | Bitbucket uses a single file; 12 tools is manageable. Jira split happened because Phase 2 added mid-development. Start single; split only if >~400 lines. |

---

## Service Layer — `internal/atlassian/confluence/`

### Endpoints Table

| Tool | Method | Path | Success |
|------|--------|------|---------|
| GetPage | GET | `/wiki/api/v2/pages/{id}?body-format=storage` | 200 |
| GetPagesInSpace | GET | `/wiki/api/v2/spaces/{id}/pages?limit=&cursor=` | 200 |
| GetSpaces | GET | `/wiki/api/v2/spaces?limit=&cursor=&keys=&type=` | 200 |
| GetPageDescendants | GET | `/wiki/api/v2/pages/{id}/descendants?limit=&cursor=` | 200 |
| GetFooterComments | GET | `/wiki/api/v2/pages/{id}/footer-comments?limit=&cursor=` | 200 |
| GetInlineComments | GET | `/wiki/api/v2/pages/{id}/inline-comments?limit=&cursor=` | 200 |
| GetCommentChildren | GET | `/wiki/api/v2/footer-comments/{id}/children?limit=&cursor=` | 200 |
| CreatePage | POST | `/wiki/api/v2/pages` | 200 |
| UpdatePage | PUT | `/wiki/api/v2/pages/{id}` | 200 |
| CreateFooterComment | POST | `/wiki/api/v2/footer-comments` | 200 |
| CreateInlineComment | POST | `/wiki/api/v2/inline-comments` | 200 |
| Search | GET | `/wiki/rest/api/search?cql=&limit=` | 200 |

### `Service` Interface (models.go)

```go
package confluence

import (
    "context"
    "errors"
    "net/url"
)

var (
    ErrNotFound     = errors.New("confluence: not found")
    ErrUnauthorized = errors.New("confluence: unauthorized")
    ErrRateLimit    = errors.New("confluence: rate limited")
    ErrConflict     = errors.New("confluence: version conflict")
)

type Service interface {
    // Read (7)
    GetPage(ctx context.Context, pageID string, bodyFormat string) (*Page, error)
    GetPagesInSpace(ctx context.Context, spaceID string, limit int, cursor string) (*PageList, error)
    GetSpaces(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*SpaceList, error)
    GetPageDescendants(ctx context.Context, pageID string, limit int, cursor string) (*PageRefList, error)
    GetFooterComments(ctx context.Context, pageID string, limit int, cursor string) (*CommentList, error)
    GetInlineComments(ctx context.Context, pageID string, limit int, cursor string) (*CommentList, error)
    GetCommentChildren(ctx context.Context, commentID string, limit int, cursor string) (*CommentList, error)
    // Write (4)
    CreatePage(ctx context.Context, req CreatePageRequest) (*Page, error)
    UpdatePage(ctx context.Context, req UpdatePageRequest) (*Page, error)
    CreateFooterComment(ctx context.Context, req CreateCommentRequest) (*Comment, error)
    CreateInlineComment(ctx context.Context, req CreateInlineCommentRequest) (*Comment, error)
    // Search (1)
    SearchContent(ctx context.Context, cql string, limit int) ([]SearchResult, error)
}
```

### Domain Types (models.go)

```go
// Domain types
type Page struct {
    ID            string
    Title         string
    SpaceID       string
    Status        string
    VersionNumber int
    Body          string // storage/XHTML value
    CreatedAt     time.Time
    WebURL        string
}

type PageRef struct { ID, Title, Status, Type string }
type Space  struct { ID, Key, Name, Type, Status string }
type Comment struct {
    ID            string
    Body          string
    VersionNumber int
    CreatedAt     time.Time
}
type SearchResult struct {
    ContentID string
    Title     string
    Type      string
    SpaceKey  string
    Excerpt   string
}

// Paginated list types
type PageList    struct { Results []Page;    NextCursor string }
type PageRefList struct { Results []PageRef; NextCursor string }
type SpaceList   struct { Results []Space;   NextCursor string }
type CommentList struct { Results []Comment; NextCursor string }

// Request types
type CreatePageRequest struct {
    SpaceID  string
    Title    string
    Body     string // storage/XHTML; pass-through
    ParentID string // optional
    Status   string // default "current"
}
type UpdatePageRequest struct {
    PageID        string
    Title         string
    Body          string
    Status        string
    VersionNumber *int // nil = fetch internally
}
type CreateCommentRequest struct {
    PageID          string
    Body            string
    ParentCommentID string // optional, for replies
}
type CreateInlineCommentRequest struct {
    PageID                   string
    Body                     string
    TextSelection            string // required
    TextSelectionMatchCount  int    // optional
    TextSelectionMatchIndex  int    // optional
}
```

### Helpers (models.go)

```go
// plainTextToStorage wraps plain text in minimal Confluence storage XHTML.
// Use this when the caller wants to post plain text without hand-writing XHTML.
func plainTextToStorage(text string) string {
    return "<p>" + template.HTMLEscapeString(text) + "</p>"
}

// extractCursor parses the `cursor=` query param from _links.next.
// Returns "" when no next page exists.
func extractCursor(next string) string {
    if next == "" { return "" }
    u, err := url.Parse(next)
    if err != nil { return "" }
    return u.Query().Get("cursor")
}
```

### Raw API Response Structs (models.go, unexported)

```go
// _links envelope present on list responses
type confLinks struct { Next string `json:"next"` }

// Page raw response
type pageAPIResponse struct {
    ID      string `json:"id"`
    Title   string `json:"title"`
    SpaceID string `json:"spaceId"`
    Status  string `json:"status"`
    Version struct { Number int `json:"number"` } `json:"version"`
    Body    struct {
        Storage struct { Value string `json:"value"` } `json:"storage"`
    } `json:"body"`
    Links struct { WebUI string `json:"webui"` } `json:"_links"`
    CreatedAt string `json:"createdAt"`
}
func (r pageAPIResponse) ToPage() Page { ... }

// pageListAPIResponse wraps results + _links
type pageListAPIResponse struct {
    Results []pageAPIResponse `json:"results"`
    Links   confLinks         `json:"_links"`
}

// Similar shapes for spaceAPIResponse, pageRefAPIResponse, commentAPIResponse, searchItemAPIResponse
```

### `NewService`

```go
type ConfluenceService struct { doer client.HTTPDoer; baseURL string }
func NewService(doer client.HTTPDoer, baseURL string) *ConfluenceService {
    return &ConfluenceService{doer: doer, baseURL: baseURL}
}
```

### Update Page Flow (Decision 2)

```
UpdatePage(ctx, req):
  if req.VersionNumber == nil:
    current, err := s.GetPage(ctx, req.PageID, "")  // internal GET
    if err: return nil, err
    version = current.VersionNumber + 1
  else:
    version = *req.VersionNumber

  PUT /wiki/api/v2/pages/{id}
    body: {id, status, title, body:{representation:"storage",value}, version:{number: version}}

  switch resp.StatusCode:
    200     → decode + return
    404     → ErrNotFound
    401/403 → ErrUnauthorized
    409     → ErrConflict  // stale version
    429     → ErrRateLimit
```

---

## MCP Layer — `internal/mcp/tools_confluence.go`

**File**: Single file. Pattern: `ToolXxx(svc confluence.Service) server.ToolHandlerFunc` for reads; `ToolXxx(svc confluence.Service, log audit.Logger) server.ToolHandlerFunc` for writes.

**Output structs** (snake_case, all in file):

```go
type mcpPageJSON struct {
    ID            string `json:"id"`
    Title         string `json:"title"`
    SpaceID       string `json:"space_id"`
    Status        string `json:"status"`
    VersionNumber int    `json:"version_number"`
    Body          string `json:"body,omitempty"`
    CreatedAt     string `json:"created_at,omitempty"`
    WebURL        string `json:"web_url,omitempty"`
}
type mcpPageRefJSON  struct { ID, Title, Status, Type string `json:"..."` }
type mcpSpaceJSON    struct { ID, Key, Name, Type, Status string `json:"..."` }
type mcpCommentJSON  struct { ID string `json:"id"`; Body string `json:"body"`; VersionNumber int `json:"version_number"`; CreatedAt string `json:"created_at,omitempty"` }
type mcpSearchJSON   struct { ContentID, Title, Type, SpaceKey, Excerpt string `json:"..."` }
type mcpPageListJSON struct { Results []mcpPageRefJSON `json:"results"`; NextCursor string `json:"next_cursor,omitempty"` }
```

**Empty-slice rule**: use `make([]T, 0)` or `make([]T, len(src))` so `json.Marshal` emits `[]`.

**Registration blocks in `server.go`** (Phase B finalizes counts, but hooks go here):

```go
// --- CONFLUENCE READ (8 tools incl. search) ---
if fs.IsEnabled(features.ModuleConfluence, false) {
    s.AddTool(mcp.NewTool("get_confluence_page", ...), ToolGetConfluencePage(confluenceSvc))
    // ... 7 more read tools
}

// --- CONFLUENCE WRITE (4 tools) ---
if fs.IsEnabled(features.ModuleConfluence, true) {
    s.AddTool(mcp.NewTool("create_confluence_page", ...), ToolCreateConfluencePage(confluenceSvc, log))
    // ... 3 more write tools
}
```

`ModuleConfluence = "confluence"` is added to `features.go` in `allModules` and `moduleToolCounts` (Phase B sets counts `{8, 4}`). Phase B also sets `String()` / `allEnabled()` to include it.

---

## REST Layer — `internal/api/handlers/confluence.go`

```go
type ConfluenceHandler struct { svc confluence.Service; auditLog audit.Logger }
func NewConfluenceHandler(svc confluence.Service, auditLog audit.Logger) *ConfluenceHandler
```

**Methods**: `GetPage`, `GetPagesInSpace`, `GetSpaces`, `GetPageDescendants`, `GetFooterComments`, `GetInlineComments`, `GetCommentChildren`, `SearchContent`, `CreatePage`, `UpdatePage`, `CreateFooterComment`, `CreateInlineComment`.

Write methods call `h.auditLog.Log(...)` before returning. Write protection is handled globally by `WriteGuardMiddleware` — no per-handler guard needed.

**Routes in `cmd/api/main.go`**:

```
confluenceH := handlers.NewConfluenceHandler(confluenceSvc, auditLog)
// Read
GET /confluence/pages/{pageId}
GET /confluence/spaces/{spaceId}/pages
GET /confluence/spaces
GET /confluence/pages/{pageId}/descendants
GET /confluence/pages/{pageId}/footer-comments
GET /confluence/pages/{pageId}/inline-comments
GET /confluence/comments/{commentId}/children
GET /confluence/search
// Write (guarded by X-Enable-Write header via WriteGuardMiddleware)
POST /confluence/pages
PUT  /confluence/pages/{pageId}
POST /confluence/footer-comments
POST /confluence/inline-comments
```

---

## CLI Layer — `cmd/atlassian/confluence/`

**Command tree**:

```
atlassian confluence
  page
    get  <PAGE_ID>            → GetPage
    list <SPACE_ID>           → GetPagesInSpace
    descendants <PAGE_ID>     → GetPageDescendants
    create                    → CreatePage (--space-id, --title, --body, --parent-id) [write]
    update <PAGE_ID>          → UpdatePage (--title, --body, --version) [write]
  spaces                      → GetSpaces
  comment
    footer  <PAGE_ID>         → GetFooterComments
    inline  <PAGE_ID>         → GetInlineComments
    children <COMMENT_ID>     → GetCommentChildren
    add-footer <PAGE_ID>      → CreateFooterComment [write]
    add-inline <PAGE_ID>      → CreateInlineComment [write]
  search <CQL>                → SearchContent
```

**`root.go`**:

```go
func NewConfluenceCmd() *cobra.Command { ... }
func RegisterCommands(root *cobra.Command, svc confluence.Service, auditLog audit.Logger, dryRun bool) { ... }
```

**Wiring in `cmd/atlassian/main.go`** (after existing service block):

```go
confluenceSvc = confluencepkg.NewService(c, cfg.BaseURL)
// nil guard:
if confluenceSvc == nil { confluenceSvc = &nilConfluenceService{} }
confluenceRoot := atlcliconfluence.NewConfluenceCmd()
atlcliconfluence.RegisterCommands(confluenceRoot, confluenceSvc, auditLog, dryRun)
root.AddCommand(confluenceRoot)
```

`nilConfluenceService` implements all 12 `Service` methods returning `fmt.Errorf("service not initialized: missing env vars")`.

---

## Wiring

### `cmd/mcp/main.go`
```go
import confluencesvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
confluenceSvc := confluencesvc.NewService(httpClient, cfg.BaseURL)
// Pass confluenceSvc to mcpserver.StartServer (signature extended)
```

### `cmd/api/main.go`
```go
confluenceSvc := confluence.NewService(httpClient, baseURL)
// Pass to NewConfluenceHandler and register routes
```

### `internal/mcp/server.go`
- `NewAtlassianServer` gains a `confluenceSvc confluence.Service` parameter.
- `StartServer` gains the same parameter.

---

## Write-Flow Sequence (create_confluence_page)

```
MCP client
  │ call create_confluence_page
  ▼
ToolCreateConfluencePage
  │ WriteGuardCheck() → error if ENABLE_WRITE≠"true"
  │ req.RequireString("space_id"), "title", "body"
  │ svc.CreatePage(ctx, CreatePageRequest{...})
  ▼
ConfluenceService.CreatePage
  │ marshal JSON body: {spaceId, title, status:"current", body:{representation:"storage",value:...}}
  │ POST baseURL+"/wiki/api/v2/pages"
  │ switch statusCode → 200 decode | 404 ErrNotFound | 401/403 ErrUnauthorized | 429 ErrRateLimit
  │ pageAPIResponse.ToPage() → Page{}
  ▼
ToolCreateConfluencePage
  │ audit.Log(...)
  │ json.Marshal(mcpPageJSON{...})
  │ mcp.NewToolResultText(...)
  ▼
MCP client ← result
```

---

## Data Flow

```
MCP / CLI / REST handler
        │
        ▼
confluence.Service  (interface)
        │
        ▼
ConfluenceService.doer  (client.HTTPDoer = *client.Client)
        │  BasicAuthTransport → IdempotencyTransport → RetryTransport
        ▼
Confluence Cloud API
  /wiki/api/v2/*  (CRUD)
  /wiki/rest/api/search  (CQL)
        │
        ▼
raw JSON → pageAPIResponse.ToPage() → Page → mcpPageJSON / REST response
```

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/atlassian/confluence/service.go` | Create | `ConfluenceService`, `NewService`, 12 method implementations |
| `internal/atlassian/confluence/models.go` | Create | Sentinel errors, domain types, raw API structs, ToModel converters, `plainTextToStorage`, `extractCursor` |
| `internal/atlassian/confluence/service_test.go` | Create | Table-driven tests per method, httptest mocks |
| `internal/mcp/tools_confluence.go` | Create | 12 MCP tool handlers + output structs |
| `internal/mcp/tools_confluence_test.go` | Create | Handler tests with mock Service |
| `internal/mcp/server.go` | Modify | Add `confluenceSvc` param, add read/write registration blocks gated by `ModuleConfluence` |
| `internal/mcp/features/features.go` | Modify | Add `ModuleConfluence = "confluence"`, add to `allModules`, add to `moduleToolCounts` with `{8,4}` |
| `internal/api/handlers/confluence.go` | Create | `ConfluenceHandler` + 12 handler methods |
| `internal/api/handlers/confluence_test.go` | Create | Handler tests with httptest |
| `internal/api/server.go` | Modify | Add `ConfluenceSvc()` accessor |
| `cmd/api/main.go` | Modify | Wire `confluenceSvc`, register 12 routes |
| `cmd/atlassian/confluence/root.go` | Create | `NewConfluenceCmd()`, `RegisterCommands()` |
| `cmd/atlassian/confluence/page.go` | Create | `page get`, `page list`, `page descendants`, `page create`, `page update` |
| `cmd/atlassian/confluence/spaces.go` | Create | `spaces` (list) |
| `cmd/atlassian/confluence/comment.go` | Create | `comment footer/inline/children/add-footer/add-inline` |
| `cmd/atlassian/confluence/search.go` | Create | `search` |
| `cmd/atlassian/main.go` | Modify | Import confluence pkg, wire `confluenceSvc`, add `nilConfluenceService`, add subgroup |
| `cmd/mcp/main.go` | Modify | Import confluence pkg, wire `confluenceSvc`, pass to `StartServer` |

---

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Service | All 12 methods: happy path, 404→ErrNotFound, 401→ErrUnauthorized, 409→ErrConflict, 429→ErrRateLimit; UpdatePage auto-version fetch; cursor extraction | Table-driven, `httptest.NewServer`, mock JSON responses |
| MCP handlers | Tool handler reads params, calls service, serializes output; WriteGuardCheck blocks writes; empty slices → `[]` | Mock `confluence.Service`, assert `mcp.CallToolResult.Content` |
| REST handlers | GET routes decode query params + call service; POST/PUT routes decode body, audit log; WriteGuardMiddleware returns 403 when header missing | `httptest.NewRecorder`, mock service |
| CLI | dry-run prints without calling service; write commands call service + audit | cobra with `cmd.SetOut`, mock service |

Test files: `service_test.go`, `tools_confluence_test.go`, `handlers/confluence_test.go`.

---

## Open Questions

- [ ] **`api.Server` accessor**: Does `internal/api/server.go` need a `ConfluenceSvc()` method, or should routes be wired differently? (Recommendation: follow existing pattern — add `ConfluenceSvc()` accessor matching `JiraSvc()`, `AgileSvc()`, etc.)
- [ ] **`cmd/api/main.go` server constructor**: The `api.NewServer(...)` call currently takes 8 positional args. Adding `confluenceSvc` makes it 9. Recommend extending `api.Server` to hold the service and expose accessor — same pattern as existing services.

Both questions are low-risk implementation details resolvable during `sdd-tasks` without blocking design.
