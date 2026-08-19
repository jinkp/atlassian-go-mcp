# Feature Flags Specification

## Purpose

Module gating and lean-default install profile. Controls which Atlassian tool modules are enabled in fresh installs vs. opt-in expansions. All tool modules initially disabled; only Jira + Bitbucket auto-enabled. New modules start with Enabled: false and must be explicitly enabled via `--enable all` or `--enable <module-name>`.

## Overview

The system uses feature flags to manage tool module registration across three surfaces (MCP, REST API, CLI) and two install profiles:
- **Lean Default** (initial install): Excludes large module sets (Confluence) to minimize token budget bloat. Includes only Jira + Bitbucket + core tools (~67 tools).
- **Full Profile** (`--enable all`): Includes all available modules, including Confluence (~79 tools).

## Cross-Cutting Requirements

### Requirement: Module Gating

Each module (e.g., `ModuleConfluence`, `ModuleAgile`, `ModuleMetrics`) MUST have an entry in the feature-flags system. Each entry MUST track:
- `name` (e.g., "confluence", "agile", "metrics")
- `enabled` (boolean, default based on profile)
- `read_tool_count` (total read/search tools in the module)
- `write_tool_count` (total write tools in the module)

### Requirement: Lean Default Profile

Fresh install writes an explicit module list with Confluence and other large modules set to `enabled: false`. Core modules (Jira, Bitbucket) default to `enabled: true`. This ensures new users do not incur token cost for tools they do not use.

### Requirement: --enable all Includes All Modules

The `--enable all` flag MUST result in a profile where all available modules (including Confluence) are set to `enabled: true`. This MUST override the lean-default behavior.

### Requirement: --enable <module-name> Granular Control

The `--enable <module-name>` flag (e.g., `--enable confluence`) MUST enable only the specified module, leaving others at their default (lean-default) state.

### Requirement: Tool Count Accounting

The system MUST maintain accurate counts of:
- `allModules`: list of all known modules
- `moduleToolCounts`: map of module name → {read_count, write_count}
- `Diagnostics.TotalTools`: sum of all enabled tools across all modules

---

## Lean Default Profile

### Requirement: Initial Install Behavior

When user runs the MCP server for the first time (fresh install, no profile file):
1. System generates a profile with all modules listed
2. Confluence (and other explicitly large modules) set to `enabled: false`
3. Core modules (Jira, Bitbucket) set to `enabled: true`
4. Profile is persisted to avoid re-initialization

#### Scenario: First run generates lean profile
- GIVEN a fresh MCP server install (no prior profile)
- WHEN the server starts
- THEN a default profile is written with Confluence disabled
- AND Jira + Bitbucket enabled
- AND total tool count ≈ 67

#### Scenario: Fresh install respects ENABLE_WRITE
- GIVEN a fresh install with `ENABLE_WRITE=false`
- WHEN the server starts
- THEN write tools are not registered, even for enabled modules

### Requirement: --enable all Includes Confluence

When user explicitly passes `--enable all`:
1. The profile is rewritten with all modules set to `enabled: true`
2. Confluence is now included in tool registration
3. Total tool count is ≈ 79

#### Scenario: --enable all adds Confluence
- GIVEN a lean-default install (Confluence disabled)
- WHEN user passes `--enable all`
- THEN the profile is updated to enable all modules
- AND Confluence tools become available
- AND total tool count ≈ 79

### Requirement: --enable confluence Enables Only Confluence

When user passes `--enable confluence`:
1. Only the Confluence module's `enabled` flag is set to true
2. Other large modules (if any) remain disabled
3. Tool count increases by ~12 (8 read + 4 write for Confluence)

#### Scenario: Granular enable
- GIVEN a lean-default install
- WHEN user passes `--enable confluence`
- THEN only Confluence is enabled
- AND tool count increases to ~75 (67 + 8 confluence-read)

---

## Module Definitions

### Confluence Module

**Name**: `confluence`
**Read tools**: 8 (7 page/space/comment reads + 1 search)
**Write tools**: 4 (create page, update page, create footer comment, create inline comment)
**Default enabled in lean profile**: false
**Default enabled with --enable all**: true

### Jira Module

**Name**: `jira`
**Read tools**: 35
**Write tools**: 8
**Default enabled in lean profile**: true

### Bitbucket Module

**Name**: `bitbucket`
**Read tools**: 12
**Write tools**: 4
**Default enabled in lean profile**: true

### Agile Module

**Name**: `agile`
**Read tools**: 4
**Write tools**: 2
**Default enabled in lean profile**: false
**Default enabled with --enable all**: true

### Additional Modules

Metrics, Releases, Projects, Teams, Goals modules follow the same pattern — all disabled by default, enabled with `--enable all`.

---

## Tool Count Accounting

### Requirement: Total Tool Count Calculation

**Lean default (fresh install)**:
- Jira: 35 + 8 = 43 tools
- Bitbucket: 12 + 4 = 16 tools
- Confluence: 0 (disabled)
- Other modules: 0 (disabled)
- **Total: ~67 tools**

**Full profile (--enable all)**:
- Jira: 43 tools
- Bitbucket: 16 tools
- Confluence: 8 + 4 = 12 tools
- Agile: 4 + 2 = 6 tools
- Metrics, Releases, Projects, Teams, Goals: ~12 additional tools
- **Total: ~79+ tools**

### Requirement: Diagnostics Output

The system MUST expose a `Diagnostics()` function that returns:
```json
{
  "total_tools": 67,
  "enabled_modules": ["jira", "bitbucket"],
  "disabled_modules": ["confluence", "agile", "metrics", "releases", "projects", "teams", "goals"],
  "module_breakdown": {
    "jira": { "read": 35, "write": 8 },
    "bitbucket": { "read": 12, "write": 4 },
    "confluence": { "read": 8, "write": 4, "enabled": false }
  }
}
```

---

## Flag Parsing

### Requirement: --enable Flag Parsing

The `--enable` flag accepts:
- `all` (enables all modules)
- `<module-name>` (e.g., `confluence`, `agile`, `metrics` — enables only that module)
- Multiple flags allowed: `--enable confluence --enable agile`

#### Scenario: Parsing --enable all
- GIVEN command-line argument `--enable all`
- WHEN the MCP server parses flags
- THEN all modules are marked enabled in the profile

#### Scenario: Parsing --enable confluence
- GIVEN command-line argument `--enable confluence`
- WHEN the MCP server parses flags
- THEN only the confluence module is marked enabled

#### Scenario: Parsing multiple --enable flags
- GIVEN command-line arguments `--enable confluence --enable agile`
- WHEN the MCP server parses flags
- THEN both confluence and agile modules are marked enabled

### Requirement: --enable Help Text

The help text for the `--enable` flag MUST clearly document:
1. What modules are available
2. That the lean-default behavior excludes Confluence
3. How to enable modules (one at a time or all at once)

Example:

```
--enable <module>
  Enable optional tool modules. Options:
    all              — enable all modules (confluence, agile, metrics, releases, projects, teams, goals)
    confluence       — enable Confluence tools (pages, spaces, comments, search)
    agile            — enable Agile tools (boards, sprints, backlog)
    metrics          — enable Metrics tools
    releases         — enable Releases tools
    projects         — enable Projects tools
    teams            — enable Teams tools
    goals            — enable Goals tools
    
  Default (fresh install): only Jira + Bitbucket enabled (~67 tools)
  Fresh install writes explicit profile excluding Confluence to minimize token usage.
```

---

## API & Data Model

### Requirement: Feature-Flags Data Structure

```go
type ModuleConfig struct {
    Name       string
    Enabled    bool
    ReadCount  int
    WriteCount int
}

type Profile struct {
    Modules map[string]ModuleConfig
}
```

### Requirement: IsEnabled Check

The system MUST provide a function to check if a module is enabled:

```go
func IsEnabled(module string, requiresWrite bool) bool
```

- `module`: module name (e.g., "confluence")
- `requiresWrite`: if true, also checks that write access is enabled globally (`ENABLE_WRITE=true`)
- Returns: true if module is enabled, false otherwise

#### Scenario: IsEnabled for read tools
- GIVEN `IsEnabled("confluence", false)` on a lean-default profile
- THEN the result is false

#### Scenario: IsEnabled for write tools
- GIVEN `IsEnabled("confluence", true)` on a lean-default profile
- THEN the result is false, even if write access is globally enabled

#### Scenario: IsEnabled after --enable confluence
- GIVEN `IsEnabled("confluence", false)` after `--enable confluence`
- THEN the result is true
- AND `IsEnabled("agile", false)` remains false

---

## Persistence

### Requirement: Profile File Location

The feature-flags profile is persisted to:
```
$XDG_CONFIG_HOME/atlassian-go-mcp/profile.yaml
  or
~/.config/atlassian-go-mcp/profile.yaml
```

Format (YAML):
```yaml
schema: "1.0"
created_at: "2026-08-19T12:00:00Z"
modules:
  jira:
    enabled: true
    read_count: 35
    write_count: 8
  bitbucket:
    enabled: true
    read_count: 12
    write_count: 4
  confluence:
    enabled: false
    read_count: 8
    write_count: 4
  agile:
    enabled: false
    read_count: 4
    write_count: 2
```

### Requirement: Profile Initialization

On first run:
1. If profile file does not exist, create it with lean-default settings
2. If `--enable all` is passed, update the profile to enable all modules
3. If `--enable <module>` is passed, update the profile to enable only that module
4. Write the updated profile back to disk

#### Scenario: First run creates lean profile
- GIVEN no existing profile file
- WHEN the MCP server starts
- THEN a profile.yaml is created in the config directory
- AND it contains the lean-default configuration

---

## Testing

### Requirement: Feature-Flags Unit Tests

Test scenarios:
1. Lean-default profile initialization
2. `--enable all` parsing and tool count update
3. `--enable <module>` granular enable
4. `IsEnabled()` logic with various flags
5. Tool registration/deregistration based on feature flags
6. Diagnostics output accuracy

#### Scenario: Test lean-default profile
- GIVEN a fresh profile
- WHEN we query module counts
- THEN Jira + Bitbucket are enabled, Confluence is not
- AND total tool count is ~67

#### Scenario: Test --enable all updates counts
- GIVEN a lean-default profile
- WHEN we call ParseFlags(["--enable", "all"])
- THEN Confluence and other modules become enabled
- AND total tool count is ~79

#### Scenario: Test tool registration respects flags
- GIVEN a disabled Confluence module
- WHEN we call `IsEnabled("confluence", false)`
- THEN the MCP server does NOT register Confluence tools
- AND Confluence tools are not callable

---

## Traceability

This spec is sourced from:
- **Proposal**: `proposal.md` sections "Feature-flag wiring + lean default"
- **Design**: Phase B architecture decision #2 (lean default + ModuleConfluence)
- **Implemented in**: `internal/mcp/features/features.go`, `cmd/mcp/main.go`, `internal/tui/model.go`, `internal/tui/connector.go`
