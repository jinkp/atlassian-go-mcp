# Confluence Module Specification

## Purpose

Confluence Cloud read/write/search capability — pages, spaces, comments, CQL. 12 tools across MCP + CLI + REST surfaces. V2 API (`/wiki/api/v2/`) for CRUD; v1 (`/wiki/rest/api/search`) for CQL search. Same BasicAuth credentials as Jira (no new config).

## Cross-Cutting Requirements

### Requirement: Three-Surface Exposure

Each of the 12 tools MUST be accessible from MCP (tool registration in `tools_confluence.go`), CLI (`atlassian confluence <command>`), and REST API (`/confluence/*` route). Read tools MUST NOT require `ENABLE_WRITE`.

### Requirement: Error Mapping — Sentinel Errors

The confluence package MUST define its own sentinel errors mirroring jira's convention: `ErrNotFound` (404), `ErrUnauthorized` (401/403), `ErrRateLimit` (429), `ErrConflict` (409 — version conflict on page update). All 12 tools MUST map HTTP status codes to these sentinels.

### Requirement: Storage-Body Pass-Through

Body content MUST use Confluence **storage format** (XHTML), NOT ADF. The system MUST accept the caller's string verbatim as the `{representation:"storage", value:"<xhtml>"}` payload. A plain-text convenience wrapper MAY be offered but is NOT required by this spec.

### Requirement: JSON Output Convention

All JSON output MUST use `snake_case` keys. Empty slices MUST serialize as `[]`, never `null`.

### Requirement: Cursor Pagination

List/collection endpoints MUST expose `limit` (optional) and MAY expose `cursor` (optional). When more results exist, the response MUST include `next_cursor`. When no more results exist, `next_cursor` MUST be empty or omitted.

### Requirement: No New Credentials

The confluence module MUST reuse `cfg.BaseURL` + existing BasicAuth (`ATLASSIAN_EMAIL` / `ATLASSIAN_TOKEN`). No new environment variables SHALL be introduced.

---

## Read Tools (7)

### Requirement: get_confluence_page

GET `/wiki/api/v2/pages/{id}`. Input: `page_id` (required), `body_format` (optional, default `"storage"`). Output: `{id, title, space_id, status, version_number, body, created_at, web_url}`.

#### Scenario: Happy path
- GIVEN a valid page ID
- WHEN `get_confluence_page` is called
- THEN the response contains the page with all specified fields

#### Scenario: Not found
- GIVEN a page ID that does not exist
- WHEN `get_confluence_page` is called
- THEN the system MUST return `ErrNotFound`

#### Scenario: Unauthorized
- GIVEN invalid credentials
- WHEN `get_confluence_page` is called
- THEN the system MUST return `ErrUnauthorized`

### Requirement: get_pages_in_space

GET `/wiki/api/v2/spaces/{id}/pages`. Input: `space_id` (required), `limit`/`cursor` (optional). Output: `{results:[{id, title, status, space_id}], next_cursor}`.

#### Scenario: Happy path with pagination
- GIVEN a space with pages exceeding `limit`
- WHEN called with `limit=2`
- THEN response contains 2 results and a non-empty `next_cursor`

#### Scenario: Empty space
- GIVEN a space with no pages
- WHEN called
- THEN `results` MUST be `[]`

#### Scenario: Space not found
- GIVEN an invalid space ID
- WHEN called
- THEN the system MUST return `ErrNotFound`

### Requirement: get_confluence_spaces

GET `/wiki/api/v2/spaces`. Input: `limit`/`cursor` (optional), `keys`/`type` filters (optional). Output: `{results:[{id, key, name, type, status}], next_cursor}`.

#### Scenario: Happy path
- GIVEN a Confluence instance with spaces
- WHEN `get_confluence_spaces` is called
- THEN the response contains an array of space objects

#### Scenario: Unauthorized
- GIVEN invalid credentials
- WHEN called
- THEN the system MUST return `ErrUnauthorized`

### Requirement: get_page_descendants

GET `/wiki/api/v2/pages/{id}/descendants`. Input: `page_id` (required), `limit`/`cursor` (optional). Output: array of `{id, title, status, type}`.

#### Scenario: Happy path
- GIVEN a page with child pages
- WHEN called
- THEN response contains descendant refs

#### Scenario: Page not found
- GIVEN an invalid page ID
- WHEN called
- THEN MUST return `ErrNotFound`

### Requirement: get_page_footer_comments

GET `/wiki/api/v2/pages/{id}/footer-comments`. Input: `page_id` (required), `limit`/`cursor` (optional). Output: array of `{id, body, created_at, version_number}`.

#### Scenario: Happy path
- GIVEN a page with footer comments
- WHEN called
- THEN response contains comment objects

#### Scenario: No comments
- GIVEN a page with no footer comments
- WHEN called
- THEN MUST return `[]`

### Requirement: get_page_inline_comments

GET `/wiki/api/v2/pages/{id}/inline-comments`. Input: `page_id` (required), `limit`/`cursor` (optional). Output: array of `{id, body, created_at, version_number}` (same shape as footer comments).

#### Scenario: Happy path
- GIVEN a page with inline comments
- WHEN called
- THEN response contains inline comment objects

#### Scenario: No inline comments
- GIVEN a page with no inline comments
- WHEN called
- THEN MUST return `[]`

### Requirement: get_comment_children

GET `/wiki/api/v2/footer-comments/{id}/children`. Input: `comment_id` (required), `limit`/`cursor` (optional). Output: array of child comment objects.

#### Scenario: Happy path
- GIVEN a comment with replies
- WHEN called
- THEN response contains child comments

#### Scenario: Comment not found
- GIVEN an invalid comment ID
- WHEN called
- THEN MUST return `ErrNotFound`

---

## Write Tools (4)

Write tools MUST call `WriteGuardCheck()` (MCP) / go through `WriteGuardMiddleware` (REST) / respect `--dry-run` (CLI). Write tools MUST emit an audit log entry before executing. Write tools MUST NOT be registered when write access is disabled.

### Requirement: create_confluence_page

POST `/wiki/api/v2/pages`. Input: `space_id` (required), `title` (required), `body` (required — storage/XHTML string), `parent_id` (optional), `status` (optional, default `"current"`). Output: `{id, title, space_id, version_number, web_url}`.

#### Scenario: Happy path
- GIVEN valid space ID, title, and XHTML body with write access enabled
- WHEN `create_confluence_page` is called
- THEN the page is created and response contains `id`, `title`, `space_id`, `version_number`, `web_url`

#### Scenario: Space not found
- GIVEN an invalid space ID
- WHEN called
- THEN MUST return `ErrNotFound`

#### Scenario: Write access disabled
- GIVEN `ENABLE_WRITE` is not set or false
- WHEN called
- THEN `WriteGuardCheck()` MUST block before any API call

#### Scenario: Unauthorized
- GIVEN invalid credentials
- WHEN called
- THEN MUST return `ErrUnauthorized`

### Requirement: update_confluence_page

PUT `/wiki/api/v2/pages/{id}`. Input: `page_id` (required), `title` (required), `body` (required — storage/XHTML), `status` (required, default `"current"`), `version_number` (optional). The system MUST enforce version increment: when `version_number` is not supplied, the system MUST fetch the current page version and send `current + 1`. When `version_number` IS supplied, the system MUST use it as-is. A 409 from the API MUST map to `ErrConflict` with a clear version-conflict message.

#### Scenario: Happy path — version supplied
- GIVEN a valid page ID, title, body, and `version_number=3`
- WHEN `update_confluence_page` is called
- THEN the API receives version 3 and the response contains the updated page

#### Scenario: Version omitted — auto-increment
- GIVEN a valid page ID and no `version_number` supplied
- WHEN `update_confluence_page` is called
- THEN the system MUST first GET the page, read current version, and PUT with `current + 1`

#### Scenario: Version conflict (409)
- GIVEN a stale `version_number`
- WHEN `update_confluence_page` is called
- THEN the API returns 409 and the system MUST return `ErrConflict`

#### Scenario: Page not found
- GIVEN an invalid page ID
- WHEN called
- THEN MUST return `ErrNotFound`

#### Scenario: Write access disabled
- GIVEN `ENABLE_WRITE` is not set
- WHEN called
- THEN `WriteGuardCheck()` MUST block before any API call

### Requirement: create_footer_comment

POST `/wiki/api/v2/footer-comments`. Input: `page_id` (required), `body` (required — storage/XHTML), `parent_comment_id` (optional — for threaded replies). Output: `{id, body, version_number}`.

#### Scenario: Happy path — top-level comment
- GIVEN a valid page ID and body
- WHEN `create_footer_comment` is called without `parent_comment_id`
- THEN a top-level footer comment is created

#### Scenario: Happy path — reply to comment
- GIVEN a valid page ID, body, and `parent_comment_id`
- WHEN called
- THEN a reply comment is created under the parent

#### Scenario: Page not found
- GIVEN an invalid page ID
- WHEN called
- THEN MUST return `ErrNotFound`

#### Scenario: Write access disabled
- GIVEN `ENABLE_WRITE` is not set
- WHEN called
- THEN `WriteGuardCheck()` MUST block

### Requirement: create_inline_comment

POST `/wiki/api/v2/inline-comments`. Input: `page_id` (required), `body` (required — storage/XHTML), `text_selection` (required — the selected text the comment anchors to), `text_selection_match_count` (optional — total matches of text on page), `text_selection_match_index` (optional — which occurrence, 0-based). Inline comments MUST have an anchor; the `text_selection` field is REQUIRED. Output: `{id, body}`.

#### Scenario: Happy path
- GIVEN a valid page ID, body, and `text_selection="deploy pipeline"`
- WHEN `create_inline_comment` is called
- THEN the inline comment is created anchored to the selected text

#### Scenario: Missing anchor — validation error
- GIVEN a valid page ID and body but NO `text_selection`
- WHEN called
- THEN the system MUST return a validation error before making any API call

#### Scenario: Page not found
- GIVEN an invalid page ID
- WHEN called
- THEN MUST return `ErrNotFound`

#### Scenario: Write access disabled
- GIVEN `ENABLE_WRITE` is not set
- WHEN called
- THEN `WriteGuardCheck()` MUST block

---

## Search Tool (1)

### Requirement: search_confluence

GET `/wiki/rest/api/search?cql={cql}`. Note: this uses v1 API, not v2. Input: `cql` (required), `limit` (optional). Output: array of `{content_id, title, type, space_key, excerpt}`.

#### Scenario: Happy path
- GIVEN a valid CQL query `type=page AND space=DEV`
- WHEN `search_confluence` is called
- THEN the response is an array of matching content with `content_id`, `title`, `type`, `space_key`, `excerpt`

#### Scenario: No results
- GIVEN a CQL query that matches nothing
- WHEN called
- THEN the response MUST be `[]`

#### Scenario: Unauthorized
- GIVEN invalid credentials
- WHEN called
- THEN MUST return `ErrUnauthorized`
