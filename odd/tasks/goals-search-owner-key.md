# Goals: honest search, owner filter, key lookup

Objective: make the Goals CLI/MCP search tell the truth, add owner-scoped goal retrieval, and accept human-readable goal keys instead of only ARIs.
Branch: `feat/goals-search-owner-key` | Delivery strategy: `ask-on-risk` | Test runner: `go test ./...`
Forecast: ~530 authored changed lines across 3 units (over the ~400 planning heuristic; slice decision deferred to the T1/T2 boundary).

## Specs

### S1 — `--query` / `search_string` must not silently return unfiltered results

Verified defect: the Atlassian Goals GraphQL `goals_search(searchString:)` argument is ignored by the API. Probed against tenant `softrestaurant-team` (cloudId `5669cd27-6784-477e-9400-6c1286436160`): the search strings `"Analytics"`, `"Roberto"`, `"status = on_track"` and `"zzzznotexist"` all returned byte-identical result sets.

Today `atlassian goals search --query "status = on_track"` and the MCP tool `search_goals(search_string: ...)` therefore return the full unfiltered goal list with a success exit code. `README.md:239` documents the flag as `--query "status = on_track"` (JQL-like), and `models.go:25` documents `SearchString` as "JQL-like Goals search syntax". Both claims are false.

Acceptance criteria:
- `SearchGoals` with a non-empty `SearchString` returns ONLY goals whose name or key matches the search string (case-insensitive substring).
- `SearchGoals` with an empty `SearchString` preserves the current behaviour exactly: one API page, unfiltered, `first = MaxResults`.
- Filtering is performed client-side over paginated API pages, because no server-side filter argument exists. Pagination is bounded by an explicit page budget so a large site cannot hang the caller.
- When the page budget is exhausted before `MaxResults` matches are collected, the result reports that the scan was incomplete rather than implying the list is exhaustive.
- The goal `key` (e.g. `ZQEAYBZU-633`) is requested in the GraphQL selection and exposed on the domain model, since it participates in matching.
- `README.md` and all flag/tool descriptions stop claiming JQL-like syntax and state plainly what is matched.

Applicable checks: `go test ./internal/atlassian/goals/... ./internal/mcp/... ./cmd/...`, plus a live probe against the real tenant proving a filtered search returns a strict subset.

### S2 — Retrieve the goals of a specific owner

User request, verbatim: "generar escenarios como: obtener los goals de un owner en especifico".

Verified capability: `graphStore_atlasGoalHasOwnerInverse(id:, first:, after:, queryContext:)` returns the goals owned by a user. Confirmed working against the real tenant for owner `712020:7bfdc0f9-20c9-41b1-900d-aab305ab0f22` (Roberto Dzul).

Three non-obvious requirements, each established empirically:
- the directive `@optIn(to: "GraphStoreAtlasGoalHasOwner")` must be placed ON THE FIELD; placing it on the operation fails with `MisplacedDirective`;
- a query context is mandatory, otherwise the API returns "Query context must not be null and should be a valid platform site or workspace ARI"; the `queryContext` argument set to `ari:cloud:platform::site/<cloudId>` works, so no HTTP header plumbing is required;
- the edge node is the union `GraphStoreSimplifiedAtlasGoalHasOwnerInverseUnion` with `possibleTypes: [TownsquareGoal]`, so an inline fragment `... on TownsquareGoal` is required.

Acceptance criteria:
- A service method returns an owner's goals with cursor pagination, accepting either a bare Atlassian account ID or a full `ari:cloud:identity::user/...` ARI.
- A CLI command exposes it and an MCP tool exposes it.
- An owner with no goals yields an empty result, not an error.
- `README.md` documents the command, the tool, and where to obtain the account ID.

Applicable checks: package tests plus a live probe proving every returned goal belongs to the requested owner.

### S3 — Accept human-readable goal keys, not only ARIs

Goals carry a short key (`ZQEAYBZU-633`) and `goals_byKey(containerId:, goalKey:)` exists. Today every read path requires a ~90-character ARI, e.g. `atlassian goals get "ari:cloud:townsquare:abc:goal/xyz"`.

Acceptance criteria:
- A goal can be fetched by key given a site ID.
- The existing ARI-based lookup keeps working unchanged.
- An unknown key produces a clear not-found error, not a nil dereference.
- `README.md` shows the key-based form.

Applicable checks: package tests plus a live probe fetching a real key observed in this tenant.

### S4 — Out of scope, recorded so it is not lost

`Goal.Phase` (`models.go:14`) is dead code: never requested in any GraphQL selection, never assigned in `toGoal()`, so it always serialises as `""`. Observed `status.value` values in the live tenant are `on_track`, `off_track`, `pending`, `archived`. Removing or populating the field is a separate output-contract change and is NOT part of this feature.

Also available and unused, deliberately deferred: `goals_goalTypes(containerId)` (would remove the README instruction at line 608 to fetch `goal_type_id` manually from the UI), `goals_statusSummary(containerId, startDate, endDate)`, `goals_fullHierarchySearch(childrenOf:)`, `goals_byIds`, `goals_metricSearch`, `goals_directoryViewSearch`.

## Tasks

| ID | Specs | Route | Status | Evidence |
| --- | --- | --- | --- | --- |
| T1 | S1 | inline | done | RED observed (missing `Key`/`ScanIncomplete`/`maxFilterScanPages`, then 3 failing CLI behaviours) → GREEN. `go test ./...` all packages ok, `go vet ./...` clean. Live differential check vs `main` recorded in L4. Risk: none. |
| T2 | S2 | inline | done | RED observed (missing `GoalsByOwnerRequest`/`GetGoalsByOwner`, then the MCP/CLI behaviours) → GREEN. `go test ./...` all packages ok, `go vet ./...` clean. Live verification recorded in L7. Risk: none. |
| T3 | S3 | inline | done | RED observed (missing `GetGoalByKey`, then the routing behaviours) → GREEN. `go test ./...` all packages ok, `go vet ./...` clean. Live verification recorded in L10. Risk: none. |

Routing rationale: all three tasks edit `internal/atlassian/goals/service.go`, `internal/atlassian/goals/models.go` and `README.md`. The edit surfaces are NOT disjoint, so the Writer trigger does not apply and parallel writers are unsafe. The parent writes inline, sequentially, one work-unit commit per task.

## Log

### L1 — Original request (verbatim, 2026-10-29)

"ayudame a mejorar el cli y mcp de atlasian para los goals, en esta maquina tengo configurada las llaves compartidas para realizar pruebas de busquedas, etc. me explico generar escenarios como: obtener los goals de un owner en especifico, etc."

### L2 — Scope decision (2026-10-29)

Presented four scope options after the live API probe. User chose "Bug + filtro por owner + key": fix the ignored `--query` first, then add owner filtering via graphStore, then accept keys alongside ARIs. The full Goals package (goal-types, status-summary, hierarchy, email lookup) was explicitly not chosen and is recorded in S4.

### L3 — Investigation evidence (2026-10-29)

Method: GraphQL introspection of the `Query` type plus live queries against `https://softrestaurant-team.atlassian.net/gateway/api/graphql` using the shared credentials in `~/.atlassian/credentials.env`, which the user confirmed are configured for search testing.

- `goals_search` accepts only `containerId` (required), `searchString`, `after`, `first`, `sort: [TownsquareGoalSortEnum]`. There is no filter input object, so no server-side status, owner or team filter exists on that query.
- `searchString` is inert: four different values returned identical pages (see S1).
- Owner filtering is reachable only through the graphStore edge query (see S2).
- Tooling gotcha: PowerShell 5.1 `Invoke-RestMethod` fails on these GraphQL responses with "No parameterless constructor defined for type 'System.String'". Use `Invoke-WebRequest -UseBasicParsing` and `ConvertFrom-Json` on `.Content`.
- The gateway warns that anonymous operations will be banned; all existing Go queries are already named, so no action is required.

### L4 — T1 evidence: differential check against `main` (2026-10-29)

Built two binaries — `main` (dc3739f) in a scratch git worktree under the system temp dir, and the branch — and ran both against the live site, comparing stdout.

- `goals search --site-id <site> --max-results 3 -o json`, unfiltered: the ONLY difference is three added `"key": "..."` lines. Same goals, same order, every other field byte-identical. No regression.
- `goals search --query "Analytics" --max-results 5 -o json`: `main` returned 5 goals of which 4 contain no "Analytics" at all ("ciclos de sincronización", "registros validados", "Llamadas analizadas", "Tickets analizados") — it returned the first 5 alphabetically and presented them as search results. The branch returns 5 goals that all contain "analytics", including goals outside the first alphabetical page, which also proves the pagination walk works.
- `goals get <ari> -o json`: only difference is the added `"key"` line.
- `goals site-id --subdomain softrestaurant-team`: byte-identical.
- `--query "zzzznotexist"`: `main` returned goals; the branch returns "No goals found" with no incompleteness warning, i.e. the scan genuinely exhausted this site inside the page budget.
- `--query "ZQEAYBZU-987"`: returns exactly that one goal, so key matching works end to end.

Also confirmed live that `"phase": ""` is emitted for every goal, which is the S4 dead-field finding.

### L5 — T1 self-correction found during verification (2026-10-29)

The first implementation reintroduced the very defect it was fixing, one layer up: the CLI printed the flat `No goals found` even when `ScanIncomplete` was true, and `NextCursor` was unusable because the search command had no `--cursor` flag to resume with. A budget-truncated scan proves nothing about the pages it never read, so reporting it as "nothing found" is the same unsupported claim in a different place.

Fixed with three failing tests first: an incomplete scan with no matches must not print the definitive message and must surface the cursor; an incomplete scan with matches must still warn; a complete scan with no matches may say so plainly. The warning is written to stderr, with a test asserting it stays out of stdout, so `-o json` remains parseable. Added `--cursor` and a test proving it reaches the service.

### L6 — Risk assessment for T1 (2026-10-29)

`Risk: none`. Reasoning against the high-risk list: `key` on `Goal` and `scan_incomplete` on `GoalSearchResult` are added optional fields, and `--cursor` is an added flag — both explicitly excluded from the "contracts others consume" item. `GetGoal` shares `toGoal()` and therefore also gained `key`, which the request did not mention, but an added optional field is not a break. The one genuine behaviour change on a consumed contract — `--query` / `search_goals.search_string` now filtering instead of silently returning everything — is the explicitly requested fix, and requested changes are not high risk by themselves. No migration, credential, concurrency, delivery or untested surface is touched.

### L7 — T2 evidence: owner filtering verified live (2026-10-29)

- `goals by-owner --site-id <site> --owner 712020:7bfdc0f9-... --max-results 5 -o json` returned 5 goals, all 5 owned by exactly one person (Roberto Dzul). Distinct owner count in the output: 1.
- A second owner (`712020:2eee3493-...`, Andres Mena Dorantes) returned a disjoint set of goals, so the filter actually varies with the argument.
- The bare account ID and the full `ari:cloud:identity::user/...` form return identical goal IDs.
- An owner who owns nothing prints "No goals found for that owner" and exits 0.
- Omitting `--owner` exits 1 with `required flag(s) "owner" not set`; the service is never called.

Cursor gotcha found while verifying: `next_cursor` is a KMS-encrypted opaque token that the API regenerates on every request, so two otherwise identical responses differ in that field. Comparing whole responses byte-for-byte across requests gives a false negative; compare goal IDs instead.

Adding a method to `GoalsService` touched five implementors — `nilGoalsService` in `cmd/atlassian/main.go` plus the four test mocks in `cmd/atlassian/goals/site_id_test.go`, `internal/api/handlers/goals_test.go`, `internal/mcp/tools_goals_test.go` and `internal/mcp/server_test.go`. The registered-tool count moved 79 → 80, which required updating the hardcoded expectations in `internal/mcp/features/features_test.go` and `internal/mcp/server_test.go` and the module count in `features.go` (`ModuleGoals` 3 → 4 read).

No REST handler was added. `internal/api` already omits the goal metrics endpoints (noted in `README.md`), so S2 scoped the surface to CLI plus MCP and the REST gap stays consistent rather than half-closed.

### L8 — Formatting baseline note (2026-10-29)

`gofmt -l` flags almost every file in this repository on Windows because the working tree is CRLF. Normalising line endings in a scratch copy and re-running gofmt showed that `internal/atlassian/goals/models.go` and `internal/mcp/tools_goals.go` are genuinely unformatted — and that both were ALREADY unformatted on `main` (verified against the base worktree), in blocks this work never touched: `metricTargetAPIItem` field alignment, the import order in `tools_goals.go`, and a trailing blank line. Those were left alone rather than reformatted, to keep the diff reviewable. The one block actually edited here, the `Goal` struct, was left gofmt-correct.

### L9 — Risk assessment for T2 (2026-10-29)

`Risk: none`. New method, new CLI command, new MCP tool: all additive. `GoalsService` gained a method, which is an internal interface with all five implementors updated in the same commit, not a published contract. No existing command's behaviour changed; the tool-count test updates are mechanical consequences of registering one new tool. No migration, credential, concurrency or delivery surface is touched, and the new code is covered by service, MCP and CLI tests plus live verification.

### L10 — T3 evidence: key lookup verified live (2026-10-29)

- `goals get ZQEAYBZU-987 --site-id <site> -o json` returns the goal, exit 0.
- The key form and the ARI form of the same goal produce byte-identical output.
- An unknown key exits 3 with `goals: no goal with key "ZQEAYBZU-99999" in site "<site>": issue not found`.
- A key with no `--site-id` exits 1 and explains that the argument was read as a key and a site is required, instead of attempting a doomed lookup.

Routing is by identifier shape: anything not prefixed `ari:` is a key. The ARI path is untouched, verified by the identical-output check above.

Two findings handled along the way:

- The API reports an unknown key as a GraphQL error with `extensions.statusCode: 404` and the prose message `Exception while fetching data (/goals_byKey) : Goal with key X does not exist for <cloudId>`, not as a null payload. `graphQLError` gained an `Extensions.StatusCode` field and a dedicated `firstGraphQLErrorIsNotFound` helper, so only this call maps 404 to not-found and no other method's error reporting changes. A test asserts that errors without extensions still surface their message, since that is the shape every other call sees.
- The shared `jira.ErrNotFound` renders as "issue not found", which is wrong for a goal and names nothing. The error is wrapped so `errors.Is` and the exit code 3 still hold while the message identifies the key and the site.

`get_goal`'s `goal_id` went from required to optional in the MCP schema, which is a relaxation: existing callers that send `goal_id` behave exactly as before, and `goal_id` wins when both identifiers are supplied. A test pins that precedence.

### L11 — Risk assessment for T3 (2026-10-29)

`Risk: none`. `GetGoalByKey`, the `--site-id` flag on `goals get`, and the `goal_key`/`site_id` arguments on `get_goal` are all additions. Relaxing a required MCP argument to optional cannot break a caller that was already supplying it. `graphQLError` gained a field, which is additive, and the new 404 mapping is reachable only from `GetGoalByKey`; a regression test covers the extension-less error shape all other methods rely on. The one behaviour change to an existing command is that `goals get` now accepts a non-ARI argument where it previously would have sent it to the ARI endpoint and failed — strictly more inputs accepted, none removed.

Feature complete: S1, S2 and S3 are all done and verified live. S4 remains deliberately out of scope.
