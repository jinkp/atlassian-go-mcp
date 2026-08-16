package releases

import (
	"fmt"

	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
)

// DefaultDoneStatuses is used when no done statuses are configured.
var DefaultDoneStatuses = []string{"Done", "Closed", "Resolved"}

// DefaultCriticalLabels is used when no critical labels are configured.
var DefaultCriticalLabels = []string{"critical"}

// DefaultRules is the rule set evaluated when the caller does not specify one.
var DefaultRules = []string{
	"all_issues_done",
	"no_critical_bugs_open",
	"no_blocking_issues",
	"min_issues_count",
}

// ValidationResult holds the outcome of a deploy-readiness validation run.
type ValidationResult struct {
	Ready    bool     `json:"ready"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// validationRule checks a set of issues and appends errors/warnings to the result.
type validationRule func(issues []jira.Issue, result *ValidationResult)

// ValidationEngine holds registered rules plus the configured done statuses and
// critical labels used to evaluate deploy-readiness for a release.
type ValidationEngine struct {
	rules          map[string]validationRule
	doneStatuses   map[string]bool
	criticalLabels map[string]bool
}

// NewValidationEngine creates an engine with the given done statuses and critical
// labels. Empty slices fall back to the package defaults.
func NewValidationEngine(doneStatuses, criticalLabels []string) *ValidationEngine {
	if len(doneStatuses) == 0 {
		doneStatuses = DefaultDoneStatuses
	}
	if len(criticalLabels) == 0 {
		criticalLabels = DefaultCriticalLabels
	}
	e := &ValidationEngine{
		rules:          make(map[string]validationRule),
		doneStatuses:   toSet(doneStatuses),
		criticalLabels: toSet(criticalLabels),
	}
	e.registerBuiltins()
	return e
}

// Evaluate runs the named rules against the given issues and returns the result.
// Unknown rule names produce a warning instead of failing the run.
func (e *ValidationEngine) Evaluate(issues []jira.Issue, ruleNames []string) *ValidationResult {
	if len(ruleNames) == 0 {
		ruleNames = DefaultRules
	}
	result := &ValidationResult{Ready: true}
	for _, name := range ruleNames {
		rule, ok := e.rules[name]
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("unknown rule: %s", name))
			continue
		}
		rule(issues, result)
	}
	return result
}

// registerBuiltins registers all built-in validation rules.
func (e *ValidationEngine) registerBuiltins() {
	e.rules["all_issues_done"] = e.ruleAllIssuesDone
	e.rules["no_critical_bugs_open"] = e.ruleNoCriticalBugsOpen
	e.rules["no_blocking_issues"] = e.ruleNoBlockingIssues
	e.rules["min_issues_count"] = e.ruleMinIssuesCount
	e.rules["status_check"] = e.ruleStatusCheck
	e.rules["custom_jql"] = e.ruleCustomJQL
}

// ruleAllIssuesDone fails if any issue is not in a done status.
func (e *ValidationEngine) ruleAllIssuesDone(issues []jira.Issue, result *ValidationResult) {
	var open []string
	for _, iss := range issues {
		if !e.doneStatuses[iss.Status] {
			open = append(open, iss.Key)
		}
	}
	if len(open) > 0 {
		result.Ready = false
		result.Errors = append(result.Errors, fmt.Sprintf("all_issues_done: %d issue(s) not done: %v", len(open), open))
	}
}

// ruleNoCriticalBugsOpen fails if any open Bug has a critical label.
func (e *ValidationEngine) ruleNoCriticalBugsOpen(issues []jira.Issue, result *ValidationResult) {
	for _, iss := range issues {
		if iss.IssueType != "Bug" {
			continue
		}
		if e.doneStatuses[iss.Status] {
			continue
		}
		for _, label := range iss.Labels {
			if e.criticalLabels[label] {
				result.Ready = false
				result.Errors = append(result.Errors, fmt.Sprintf("no_critical_bugs_open: %s is an open critical bug", iss.Key))
				break
			}
		}
	}
}

// ruleNoBlockingIssues fails if any open issue has a "blocker" label.
func (e *ValidationEngine) ruleNoBlockingIssues(issues []jira.Issue, result *ValidationResult) {
	for _, iss := range issues {
		if e.doneStatuses[iss.Status] {
			continue
		}
		for _, label := range iss.Labels {
			if label == "blocker" {
				result.Ready = false
				result.Errors = append(result.Errors, fmt.Sprintf("no_blocking_issues: %s has blocking label", iss.Key))
				break
			}
		}
	}
}

// ruleMinIssuesCount fails if the release has no issues at all.
func (e *ValidationEngine) ruleMinIssuesCount(issues []jira.Issue, result *ValidationResult) {
	if len(issues) == 0 {
		result.Ready = false
		result.Errors = append(result.Errors, "min_issues_count: release has no issues linked")
	}
}

// ruleStatusCheck warns if any issue has a non-standard status
// (neither a configured done status nor a recognised in-flight status).
func (e *ValidationEngine) ruleStatusCheck(issues []jira.Issue, result *ValidationResult) {
	known := map[string]bool{"In Progress": true, "To Do": true, "Open": true, "Reopened": true}
	for _, iss := range issues {
		s := iss.Status
		if !e.doneStatuses[s] && !known[s] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("status_check: %s has unusual status %q", iss.Key, s))
		}
	}
}

// ruleCustomJQL is a placeholder — custom JQL requires runtime evaluation against
// Jira. In this offline validation it produces an informational warning.
func (e *ValidationEngine) ruleCustomJQL(_ []jira.Issue, result *ValidationResult) {
	result.Warnings = append(result.Warnings, "custom_jql: requires Jira query execution (not supported in offline validation)")
}

func toSet(values []string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}
