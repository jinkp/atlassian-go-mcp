# Jira Module Specification

This is the source of truth for the jira module, documenting all current capabilities.

## ADDED Requirements (Phase 1)

### Requirement: Account Lookup

The system MUST provide `lookup_jira_account_id` (READ) to search Jira users by name or email via `GET /rest/api/3/user/search?query={q}`. Input: `query` (required), `max_results` (optional, default 10 — kept small to reduce MCP token footprint). Output: array of `{account_id, display_name, email, active}`. The `email` field MAY be empty when the user's privacy settings hide it. Empty results MUST return `[]`, not null.

#### Scenario: Successful lookup by display name

- GIVEN a valid Jira connection
- WHEN `lookup_jira_account_id` is called with query `"Jane"`
- THEN the response is an array of matching users with `account_id`, `display_name`, `email`, and `active` fields
- AND the array MAY be empty if no users match

#### Scenario: Privacy-hidden email

- GIVEN a user whose email is hidden by Atlassian privacy settings
- WHEN the user appears in lookup results
- THEN the `email` field MUST be an empty string, not null or omitted

#### Scenario: Empty result set

- GIVEN a query that matches no Jira users
- WHEN `lookup_jira_account_id` is called
- THEN the response MUST be an empty array `[]`

#### Scenario: Unauthorized credentials

- GIVEN invalid or expired credentials
- WHEN `lookup_jira_account_id` is called
- THEN the system MUST return `ErrUnauthorized`

---

### Requirement: Add Comment to Issue

The system MUST provide `add_comment_to_issue` (WRITE) to post a comment on a Jira issue via `POST /rest/api/3/issue/{key}/comment`. Input: `issue_key` (required), `body` (required, plain text — converted to ADF via `plainTextToADF()`). Output: `{id, created, author}`. The tool MUST call `WriteGuardCheck()` and emit an audit log entry before executing. The tool MUST NOT be registered when write access is disabled.

#### Scenario: Successfully add a comment

- GIVEN a valid issue key and write access enabled
- WHEN `add_comment_to_issue` is called with `issue_key="PROJ-1"` and `body="Looks good"`
- THEN the Jira API receives an ADF-formatted body
- AND the response contains the comment `id`, `created` timestamp, and `author`

#### Scenario: Issue not found

- GIVEN an issue key that does not exist
- WHEN `add_comment_to_issue` is called
- THEN the system MUST return `ErrNotFound`

#### Scenario: Write access disabled

- GIVEN `ENABLE_WRITE` is not set or false
- WHEN `add_comment_to_issue` is called
- THEN `WriteGuardCheck()` MUST return an error before any API call is made

---

### Requirement: Get Issue Comments

The system MUST provide `get_issue_comments` (READ) to list comments on a Jira issue via `GET /rest/api/3/issue/{key}/comment`. Input: `issue_key` (required), `max_results` (optional). Output: array of `{id, author, body, created, updated}`. Empty comment lists MUST serialize as `[]`.

#### Scenario: Retrieve comments for an issue

- GIVEN an issue with two comments
- WHEN `get_issue_comments` is called with `issue_key="PROJ-1"`
- THEN the response is an array of 2 comment objects with `id`, `author`, `body`, `created`, `updated`

#### Scenario: Issue with no comments

- GIVEN an issue with zero comments
- WHEN `get_issue_comments` is called
- THEN the response MUST be `[]`

#### Scenario: Issue not found

- GIVEN an issue key that does not exist
- WHEN `get_issue_comments` is called
- THEN the system MUST return `ErrNotFound`

---

### Requirement: Link Issues

The system MUST provide `link_issues` (WRITE) to create a link between two issues via `POST /rest/api/3/issueLink`. Input: `inward_issue` (required key), `outward_issue` (required key), `link_type` (required — name e.g. "Blocks"). Output: success confirmation (API returns 201 with no body). MUST call `WriteGuardCheck()` and emit audit log. MUST NOT be registered when write access is disabled.

#### Scenario: Successfully link two issues

- GIVEN two valid issue keys and a valid link type name
- WHEN `link_issues` is called with `inward_issue="PROJ-1"`, `outward_issue="PROJ-2"`, `link_type="Blocks"`
- THEN the system creates the link and returns a success confirmation

#### Scenario: Invalid issue key

- GIVEN an inward or outward issue key that does not exist
- WHEN `link_issues` is called
- THEN the system MUST return `ErrNotFound`

#### Scenario: Write access disabled

- GIVEN `ENABLE_WRITE` is not set
- WHEN `link_issues` is called
- THEN `WriteGuardCheck()` MUST block the operation

---

### Requirement: Get Issue Link Types

The system MUST provide `get_issue_link_types` (READ) to list all available issue link types via `GET /rest/api/3/issueLinkType`. Input: none. Output: array of `{id, name, inward, outward}`.

#### Scenario: Retrieve all link types

- GIVEN a valid Jira connection
- WHEN `get_issue_link_types` is called
- THEN the response is an array of link type objects with `id`, `name`, `inward`, and `outward` fields

#### Scenario: Unauthorized

- GIVEN invalid credentials
- WHEN `get_issue_link_types` is called
- THEN the system MUST return `ErrUnauthorized`

---

### Requirement: Add Worklog

The system MUST provide `add_worklog` (WRITE) to log time spent on an issue via `POST /rest/api/3/issue/{key}/worklog`. Input: `issue_key` (required), `time_spent` (required — e.g. `"3h 30m"`), `comment` (optional, plain text → ADF), `started` (optional, ISO 8601 timestamp). Output: `{id, time_spent_seconds, started, author}`. MUST call `WriteGuardCheck()` and emit audit log. MUST NOT be registered when write access is disabled.

#### Scenario: Successfully add a worklog entry

- GIVEN a valid issue key and write access enabled
- WHEN `add_worklog` is called with `issue_key="PROJ-1"` and `time_spent="2h"`
- THEN the response contains `id`, `time_spent_seconds`, `started`, and `author`

#### Scenario: Worklog with optional comment and start time

- GIVEN a valid issue key
- WHEN `add_worklog` is called with `time_spent="1h 30m"`, `comment="Sprint review"`, and `started="2026-08-16T10:00:00.000+0000"`
- THEN the comment is converted to ADF before sending
- AND `started` is forwarded to the API as-is

#### Scenario: Issue not found

- GIVEN an issue key that does not exist
- WHEN `add_worklog` is called
- THEN the system MUST return `ErrNotFound`

---

### Requirement: Get Issue Type Metadata

The system MUST provide `get_issue_type_metadata` (READ) to list valid issue types for a project via `GET /rest/api/3/issue/createmeta/{projectKey}/issuetypes`. Input: `project_key` (required). Output: array of `{id, name, description, subtask}`.

#### Scenario: Retrieve issue types for a project

- GIVEN a valid project key
- WHEN `get_issue_type_metadata` is called with `project_key="PROJ"`
- THEN the response is an array of issue type objects with `id`, `name`, `description`, and `subtask` (bool)

#### Scenario: Project not found

- GIVEN a project key that does not exist
- WHEN `get_issue_type_metadata` is called
- THEN the system MUST return `ErrNotFound`

#### Scenario: Unauthorized

- GIVEN invalid credentials
- WHEN `get_issue_type_metadata` is called
- THEN the system MUST return `ErrUnauthorized`

---

## Cross-Cutting Requirements

### Requirement: Module Tool Count Update

After adding these 7 tools, `moduleToolCounts[jira]` MUST become `{8, 6}` (was `{4, 3}`): 4 new reads + 3 new writes.

### Requirement: Error Mapping Consistency

All 7 new tools MUST reuse existing sentinel errors: `404 → ErrNotFound`, `401/403 → ErrUnauthorized`, `429 → ErrRateLimit`. Rate-limited requests MUST return `ErrRateLimit`.

### Requirement: JSON Output Convention

All JSON output MUST use `snake_case` keys. Empty slices MUST serialize as `[]`, never `null`.

### Requirement: Three-Surface Exposure

Each of the 7 tools MUST be accessible from MCP (tool registration), CLI (`atlassian jira <command>`), and REST API (`/jira/*` route). Read tools MUST NOT require `ENABLE_WRITE`.
