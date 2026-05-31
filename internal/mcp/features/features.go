// Package features provides feature-flag gating for MCP tool registration.
// It parses a comma-separated module+access string (e.g. "jira,agile-read")
// into a FeatureSet that the MCP server uses to conditionally register tools.
package features

import "strings"

// AccessLevel is a bitmask of read and/or write access.
type AccessLevel int

const (
	AccessNone      AccessLevel = 0
	AccessRead      AccessLevel = 1
	AccessWrite     AccessLevel = 2
	AccessReadWrite AccessLevel = 3
)

// Module name constants.
const (
	ModuleJira     = "jira"
	ModuleAgile    = "agile"
	ModuleGoals    = "goals"
	ModuleMetrics  = "metrics"
	ModuleReleases = "releases"
	ModuleProjects = "projects"
	ModuleTeams    = "teams"
)

// allModules defines the canonical order used for String() output.
var allModules = []string{
	ModuleJira, ModuleAgile, ModuleGoals, ModuleMetrics,
	ModuleReleases, ModuleProjects, ModuleTeams,
}

// moduleToolCounts maps module → [readCount, writeCount].
// jira:     get_jira_issue, search_jira_issues, get_jira_transitions, get_jira_epics (4 read)
//           create_jira_issue, update_jira_issue, transition_jira_issue (3 write)
// agile:    get_jira_boards, get_jira_sprints, get_active_sprint, get_sprint_issues (4 read)
//           create_sprint, update_sprint, move_issues_to_sprint, move_issues_to_epic (4 write)
// goals:    get_site_id, get_goal, search_goals (3 read)
//           update_goal_status, create_goal, edit_goal (3 write)
// metrics:  get_goal_metrics (1 read)
//           create_metric, update_metric_value, update_metric_target (3 write)
// releases: search_releases, get_release, get_release_issues (3 read)
//           create_release, update_release (2 write)
// projects: list_projects, get_project, search_projects (3 read)
//           update_project (1 write)
// teams:    search_teams, get_team, get_team_members (3 read)
//           (0 write)
var moduleToolCounts = map[string][2]int{
	ModuleJira:     {4, 3},
	ModuleAgile:    {4, 4},
	ModuleGoals:    {3, 3},
	ModuleMetrics:  {1, 3},
	ModuleReleases: {3, 2},
	ModuleProjects: {3, 1},
	ModuleTeams:    {3, 0},
}

// FeatureSet holds the enabled modules and their access levels.
type FeatureSet struct {
	modules map[string]AccessLevel
}

// Parse converts a comma-separated feature flag string into a FeatureSet.
//
//   - "" or "all"          → all 7 modules at AccessReadWrite
//   - "jira"               → jira=AccessReadWrite
//   - "jira-read"          → jira=AccessRead
//   - "jira-write"         → jira=AccessWrite
//   - "jira,agile-read"    → jira=RW, agile=Read
//   - "jira-read,jira-write" → jira=AccessReadWrite (accumulated)
//   - Unknown tokens are silently ignored.
//   - Module names are case-sensitive (lowercase only).
func Parse(s string) *FeatureSet {
	s = strings.TrimSpace(s)
	if s == "" || s == "all" {
		return &FeatureSet{modules: allEnabled()}
	}
	fs := &FeatureSet{modules: make(map[string]AccessLevel)}
	for _, token := range strings.Split(s, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		var module string
		var level AccessLevel
		switch {
		case strings.HasSuffix(token, "-read"):
			module = strings.TrimSuffix(token, "-read")
			level = AccessRead
		case strings.HasSuffix(token, "-write"):
			module = strings.TrimSuffix(token, "-write")
			level = AccessWrite
		default:
			module = token
			level = AccessReadWrite
		}
		if !isKnownModule(module) {
			continue // silently ignore unknown tokens
		}
		// accumulate: "jira-read,jira-write" → AccessReadWrite
		fs.modules[module] |= level
	}
	return fs
}

func allEnabled() map[string]AccessLevel {
	m := make(map[string]AccessLevel, len(allModules))
	for _, mod := range allModules {
		m[mod] = AccessReadWrite
	}
	return m
}

func isKnownModule(m string) bool {
	for _, mod := range allModules {
		if mod == m {
			return true
		}
	}
	return false
}

// IsEnabled returns true if the given module is enabled for the requested access type.
// A nil receiver (nil *FeatureSet) returns true for everything — equivalent to "all".
func (f *FeatureSet) IsEnabled(module string, write bool) bool {
	if f == nil {
		return true
	}
	level, ok := f.modules[module]
	if !ok {
		return false
	}
	if write {
		return level&AccessWrite != 0
	}
	return level&AccessRead != 0
}

// EnabledToolCount returns the total number of tools that would be registered
// given this FeatureSet. A nil receiver returns the total tool count (37).
func (f *FeatureSet) EnabledToolCount() int {
	if f == nil {
		return totalTools()
	}
	count := 0
	for mod, level := range f.modules {
		counts, ok := moduleToolCounts[mod]
		if !ok {
			continue
		}
		if level&AccessRead != 0 {
			count += counts[0]
		}
		if level&AccessWrite != 0 {
			count += counts[1]
		}
	}
	return count
}

func totalTools() int {
	n := 0
	for _, c := range moduleToolCounts {
		n += c[0] + c[1]
	}
	return n
}

// String reconstructs the --enable flag value from this FeatureSet.
// Returns "all" if all 7 modules are at AccessReadWrite.
// Returns "" if no modules are enabled.
// A nil receiver returns "all".
func (f *FeatureSet) String() string {
	if f == nil {
		return "all"
	}
	// Check if all modules enabled at RW
	if len(f.modules) == len(allModules) {
		allRW := true
		for _, mod := range allModules {
			if f.modules[mod] != AccessReadWrite {
				allRW = false
				break
			}
		}
		if allRW {
			return "all"
		}
	}
	var parts []string
	for _, mod := range allModules {
		level, ok := f.modules[mod]
		if !ok || level == AccessNone {
			continue
		}
		switch level {
		case AccessReadWrite:
			parts = append(parts, mod)
		case AccessRead:
			parts = append(parts, mod+"-read")
		case AccessWrite:
			parts = append(parts, mod+"-write")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}
