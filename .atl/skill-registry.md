# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.

## User Skills

| Trigger | Skill | Path |
|---------|-------|------|
| When writing Go tests, using teatest, or adding test coverage | go-testing | C:\Users\isai_\.config\opencode\skills\go-testing\SKILL.md |
| When building a CLI tool usable as MCP server from OpenCode or Claude Code | self-registering-mcp | C:\Users\isai_\.config\opencode\skills\self-registering-mcp\SKILL.md |
| When user asks to create a new skill, add agent instructions, or document patterns for AI | skill-creator | C:\Users\isai_\.config\opencode\skills\skill-creator\SKILL.md |
| When user says "judgment day", "judgment-day", "review adversarial", "dual review", "juzgar", "que lo juzguen" | judgment-day | C:\Users\isai_\.config\opencode\skills\judgment-day\SKILL.md |
| When creating a GitHub issue, reporting a bug, or requesting a feature | issue-creation | C:\Users\isai_\.config\opencode\skills\issue-creation\SKILL.md |
| When creating a pull request, opening a PR, or preparing changes for review | branch-pr | C:\Users\isai_\.config\opencode\skills\branch-pr\SKILL.md |
| When user asks "how do I do X", "find a skill for X", or wants to extend capabilities | find-skills | C:\Users\isai_\.agents\skills\find-skills\SKILL.md |
| When building 1-click launchers or apps with launchers using Pinokio | gepeto | C:\Users\isai_\.agents\skills\gepeto\SKILL.md |
| When discovering, launching, or using apps and tools for the current task | pinokio | C:\Users\isai_\.agents\skills\pinokio\SKILL.md |
| When user wants runtime-evidence debugging with traces or logs via OpenTelemetry | motel-debug | C:\Users\isai_\.agents\skills\motel-debug\SKILL.md |

## Compact Rules

Pre-digested rules per skill. Delegators copy matching blocks into sub-agent prompts as `## Project Standards (auto-resolved)`.

### go-testing
- Use table-driven tests with `[]struct{ name, input, expected, wantErr }` pattern
- Run each case with `t.Run(tt.name, func(t *testing.T){...})` — never loop without subtests
- Test Bubbletea model state by calling `m.Update(tea.KeyMsg{...})` directly for unit tests
- Use `teatest.NewTestModel(t, m)` + `tm.Send()` for interactive TUI flow tests
- Use golden files in `testdata/*.golden` for visual output; update with `-update` flag
- Commands: `go test ./...`, `go test -cover ./...`, `go test -run TestName`, `go test -short ./...`
- Never use `testify` if stdlib `testing` suffices — prefer standard error messages with `t.Errorf`
- Mock dependencies via interfaces, never via monkey-patching; use `t.TempDir()` for file ops

### self-registering-mcp
- MCP `<tool> mcp` subcommand MUST NOT write to stdout before `server.ServeStdio()` — stdout is owned by MCP transport
- Redirect all logs to stderr: `log.SetOutput(os.Stderr)` inside the mcp command
- Tool handlers wrap existing service functions — zero business logic in tools; never panic, use `mcp.NewToolResultError(err.Error())`
- MCP tools cannot use Cobra context — inline env/config/git fallback chain for workspace resolution
- OpenCode config key: `mcp.<name>` with `{"type":"local","command":"<bin>","args":["mcp"]}`; Claude Code key: `mcpServers.<name>`
- JSON merge: read existing file into `map[string]json.RawMessage`, merge only the target key, never overwrite unrelated keys
- Use `github.com/mark3labs/mcp-go/server` for server and tool registration

### skill-creator
- Only create a skill for reusable patterns, not one-off tasks or trivially documented things
- Required frontmatter: `name`, `description` (with Trigger:), `license: Apache-2.0`, `metadata.author`, `metadata.version`
- Skill directory: `skills/{skill-name}/SKILL.md`; optional `assets/` for templates, `references/` for local doc links
- `references/` MUST point to local file paths, never web URLs
- Naming: generic → `{technology}`, project-specific → `{project}-{component}`, workflow → `{action}-{target}`
- Critical Patterns section is mandatory — put the most important rules first
- After creating, register the skill in `AGENTS.md` with trigger, name, and path

### judgment-day
- Read skill registry first (`mem_search("skill-registry")` → `.atl/skill-registry.md`) and inject matching compact rules into ALL judge and fix-agent prompts
- Launch Judge A + Judge B in parallel via `delegate` (async) — never sequential, never do the review yourself
- Judges classify every WARNING as `(real)` (normal user can trigger) or `(theoretical)` (requires contrived scenario) — theoretical WARNINGs are INFO only, do NOT fix or re-judge
- Round 1: present verdict table, ASK user before fixing. Round 2+: re-judge only for confirmed CRITICALs
- APPROVED criteria: 0 confirmed CRITICALs + 0 confirmed real WARNINGs — theoretical warnings and suggestions may remain
- After 2 iterations with remaining issues: ASK user whether to continue or escalate

### issue-creation
- MUST use a template (bug_report.yml or feature_request.yml) — blank issues are disabled
- Issues get `status:needs-review` automatically; maintainer MUST add `status:approved` before any PR opens
- Check for duplicates before creating; questions go to Discussions, not issues
- Bug report requires: pre-flight checks, description, steps to reproduce, expected/actual behavior, OS, agent/client, shell
- Feature request requires: pre-flight checks, problem description, proposed solution, affected area

### branch-pr
- Every PR MUST link an approved issue (`Closes #N`, `Fixes #N`, or `Resolves #N`)
- Branch name regex: `^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)\/[a-z0-9._-]+$`
- PR MUST have exactly one `type:*` label matching the commit type
- Conventional commit format: `type(scope): description` — no `Co-Authored-By` trailers
- Run `shellcheck scripts/*.sh` before opening any PR that touches shell scripts
- All automated checks (PR Validation + Shellcheck) must pass before merge

### find-skills
- Check skills.sh leaderboard first before running `npx skills find`
- Prefer skills with 1K+ installs; treat anything under 100 with skepticism
- Verify source reputation (official orgs > unknown authors) and GitHub stars before recommending
- Install command: `npx skills add <package>`; browse at https://skills.sh/

### gepeto
- Always resolve `PINOKIO_HOME` before creating or editing any launcher files — never silently fall back to current workspace
- Check `system/examples` folder for matching example scripts before writing any Pinokio script — always imitate, never assume syntax
- Web UI URL capture: always use `on: [{event: "/(http:\\/\\/[0-9.:]+)/", done: true}]` and set via `local.set {url: "{{input.event[1]}}"}`
- When fixing or debugging, check the `logs` folder first before anything else
- Mandatory 6-step execution checklist: AGENTS Snapshot → Destination Resolution → Example Lock-in → Pre-flight → Mid-task Verify → Exit Checklist

### pinokio
- Use `pterm` for all Pinokio control-plane operations — never manually call launcher CLIs unless user explicitly asks
- Resolve pterm path from `~/.pinokio/config.json` → control-plane API → `which pterm`; on Windows prefer `.cmd` or `.ps1` sibling
- Search order: `pterm search "<query>" --mode balanced --min-match 2 --limit 8`; fallback to `--mode broad`
- Do not run update commands from this skill
- `pterm status`, `pterm run`, `pterm open`, `pterm logs`, `pterm which`, `pterm download` are the primary lifecycle commands

### motel-debug
- Check `GET /api/health` first; if unreachable run `motel start` (daemon, not TUI) then recheck
- Generate 3-5 specific hypotheses BEFORE touching any code
- Wrap ALL debug instrumentation in `#region motel debug` / `#endregion motel debug` markers
- Tag every debug point with `debug.session`, `debug.hypothesis`, `debug.step`, `debug.label` attributes
- Remove ALL motel instrumentation before committing — these are temporary debug blocks only

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| AGENTS.md | C:\Users\isai_\.config\opencode\AGENTS.md | User-level agent config (system prompt) |

No project-level convention files found (AGENTS.md, CLAUDE.md, .cursorrules, GEMINI.md) in project root.
