package mcpserver

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	confluence "github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
)

// serverVersion is reported via the MCP protocol. Set via SetVersion before calling StartServer.
var serverVersion = "dev"

// SetVersion sets the version string reported by the MCP server.
// Call this from main() before StartServer.
func SetVersion(v string) {
	serverVersion = v
}

// ConfigFromEnv reads the three required Atlassian env vars and returns a client.Config.
// Returns a descriptive error naming the missing variable if any is absent.
func ConfigFromEnv() (client.Config, error) {
	baseURL := os.Getenv("ATLASSIAN_BASE_URL")
	if baseURL == "" {
		return client.Config{}, fmt.Errorf("ATLASSIAN_BASE_URL is required but not set")
	}
	email := os.Getenv("ATLASSIAN_EMAIL")
	if email == "" {
		return client.Config{}, fmt.Errorf("ATLASSIAN_EMAIL is required but not set")
	}
	token := os.Getenv("ATLASSIAN_TOKEN")
	if token == "" {
		return client.Config{}, fmt.Errorf("ATLASSIAN_TOKEN is required but not set")
	}

	return client.Config{
		BaseURL:  baseURL,
		Email:    email,
		APIToken: token,
	}, nil
}

// WriteGuardCheck returns nil when ENABLE_WRITE=true, or an error otherwise.
// When blocked, a diagnostic line is written to stderr so operators can see rejected attempts.
// Future write tools MUST call this and return the error result via mcp.NewToolResultError.
func WriteGuardCheck() error {
	if os.Getenv("ENABLE_WRITE") == "true" {
		return nil
	}
	fmt.Fprintln(os.Stderr, "[atlassian-mcp] BLOCKED: write operation rejected (ENABLE_WRITE is not set to \"true\")")
	return errors.New("write operations disabled: set ENABLE_WRITE=true to enable write tools")
}

// LogStartupDiagnostics writes a configuration summary to w (typically os.Stderr).
// Includes enabled modules, access levels, write guard status, and tool count.
// Safe to call with a nil FeatureSet.
func LogStartupDiagnostics(w io.Writer, fs *features.FeatureSet) {
	writeGuard := "disabled"
	if os.Getenv("ENABLE_WRITE") == "true" {
		writeGuard = "enabled"
	}
	total := features.TotalToolCount()
	enabled := fs.EnabledToolCount()
	fmt.Fprintf(w, "[atlassian-mcp] version: %s\n", serverVersion)
	fmt.Fprintf(w, "[atlassian-mcp] modules: %s\n", fs.Diagnostics())
	fmt.Fprintf(w, "[atlassian-mcp] write guard: %s (ENABLE_WRITE=%s)\n", writeGuard, os.Getenv("ENABLE_WRITE"))
	fmt.Fprintf(w, "[atlassian-mcp] tools: %d/%d registered\n", enabled, total)
}

// NewAtlassianServer creates a configured MCPServer with Atlassian tools registered
// according to the provided FeatureSet. A nil FeatureSet enables all 79 tools (backward compat).
// log receives an entry for every write operation (after WriteGuardCheck passes).
func NewAtlassianServer(svc jira.Service, agileSvc agile.AgileService, goalsSvc goals.GoalsService, releasesSvc releases.ReleasesService, projectsSvc projects.ProjectsService, teamsSvc teams.TeamsService, bitbucketSvc bitbucket.BitbucketService, confluenceSvc confluence.Service, log audit.Logger, fs *features.FeatureSet) *server.MCPServer {
	s := server.NewMCPServer(
		"atlassian-mcp",
		serverVersion,
		server.WithToolCapabilities(true),
	)

	// --- JIRA READ (4 tools) ---
	if fs.IsEnabled(features.ModuleJira, false) {
		s.AddTool(
			mcp.NewTool(
				"get_jira_issue",
				mcp.WithDescription("Get a Jira issue by key"),
				mcp.WithString(
					"issue_key",
					mcp.Required(),
					mcp.Description("The Jira issue key, e.g. PROJ-123"),
				),
			),
			ToolGetJiraIssue(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"search_jira_issues",
				mcp.WithDescription("Search Jira issues with JQL"),
				mcp.WithString(
					"jql",
					mcp.Required(),
					mcp.Description("JQL query string, e.g. 'project = PROJ ORDER BY updated DESC'"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of results to return (default 50)"),
				),
			),
			ToolSearchJiraIssues(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_jira_transitions",
				mcp.WithDescription("List available workflow transitions for a Jira issue"),
				mcp.WithString(
					"issue_key",
					mcp.Required(),
					mcp.Description("Issue key, e.g. PROJ-123"),
				),
			),
			ToolGetJiraTransitions(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_jira_epics",
				mcp.WithDescription("List epics for a Jira project"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key, e.g. PROJ"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of epics to return (default 50)"),
				),
			),
			ToolGetJiraEpics(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"lookup_jira_account_id",
				mcp.WithDescription("Search Jira users by display name or email"),
				mcp.WithString(
					"query",
					mcp.Required(),
					mcp.Description("Search query — name or email fragment"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of users to return (default 10)"),
				),
			),
			ToolLookupJiraAccountID(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_issue_comments",
				mcp.WithDescription("List comments on a Jira issue"),
				mcp.WithString(
					"issue_key",
					mcp.Required(),
					mcp.Description("Issue key, e.g. PROJ-123"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of comments to return (default 50)"),
				),
			),
			ToolGetIssueComments(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_issue_link_types",
				mcp.WithDescription("List all available issue link types for this Jira instance"),
			),
			ToolGetIssueLinkTypes(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_issue_type_metadata",
				mcp.WithDescription("List valid issue types for a Jira project"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key, e.g. PROJ"),
				),
			),
			ToolGetIssueTypeMetadata(svc),
		)
	}

	// --- JIRA WRITE (3 tools) ---
	if fs.IsEnabled(features.ModuleJira, true) {
		s.AddTool(
			mcp.NewTool(
				"create_jira_issue",
				mcp.WithDescription("Create a new Jira issue"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key, e.g. PROJ"),
				),
				mcp.WithString(
					"issue_type",
					mcp.Required(),
					mcp.Description("Issue type name, e.g. Bug, Story, Task"),
				),
				mcp.WithString(
					"summary",
					mcp.Required(),
					mcp.Description("Issue summary/title"),
				),
				mcp.WithString(
					"description",
					mcp.Description("Issue description (plain text)"),
				),
				mcp.WithString(
					"assignee_id",
					mcp.Description("Assignee account ID"),
				),
				mcp.WithString(
					"priority",
					mcp.Description("Priority name, e.g. High, Medium, Low"),
				),
				mcp.WithString(
					"labels",
					mcp.Description("Comma-separated list of labels, e.g. 'backend,urgent'"),
				),
			),
			ToolCreateJiraIssue(svc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"update_jira_issue",
				mcp.WithDescription("Update fields of an existing Jira issue"),
				mcp.WithString(
					"issue_key",
					mcp.Required(),
					mcp.Description("Issue key to update, e.g. PROJ-123"),
				),
				mcp.WithString(
					"summary",
					mcp.Description("New summary"),
				),
				mcp.WithString(
					"description",
					mcp.Description("New description (plain text)"),
				),
				mcp.WithString(
					"assignee_id",
					mcp.Description("New assignee account ID"),
				),
				mcp.WithString(
					"priority",
					mcp.Description("New priority name"),
				),
			),
			ToolUpdateJiraIssue(svc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"transition_jira_issue",
				mcp.WithDescription("Apply a workflow transition to a Jira issue"),
				mcp.WithString(
					"issue_key",
					mcp.Required(),
					mcp.Description("Issue key, e.g. PROJ-123"),
				),
				mcp.WithString(
					"transition_id",
					mcp.Required(),
					mcp.Description("Transition ID from get_jira_transitions"),
				),
			),
			ToolTransitionJiraIssue(svc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"add_comment_to_issue",
				mcp.WithDescription("Add a comment to a Jira issue. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"issue_key",
					mcp.Required(),
					mcp.Description("Issue key, e.g. PROJ-123"),
				),
				mcp.WithString(
					"body",
					mcp.Required(),
					mcp.Description("Comment body (plain text)"),
				),
			),
			ToolAddCommentToIssue(svc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"link_issues",
				mcp.WithDescription("Create a link between two Jira issues. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"inward_issue",
					mcp.Required(),
					mcp.Description("Inward issue key, e.g. PROJ-1"),
				),
				mcp.WithString(
					"outward_issue",
					mcp.Required(),
					mcp.Description("Outward issue key, e.g. PROJ-2"),
				),
				mcp.WithString(
					"link_type",
					mcp.Required(),
					mcp.Description("Link type name, e.g. 'Blocks'"),
				),
			),
			ToolLinkIssues(svc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"add_worklog",
				mcp.WithDescription("Log time spent on a Jira issue. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"issue_key",
					mcp.Required(),
					mcp.Description("Issue key, e.g. PROJ-123"),
				),
				mcp.WithString(
					"time_spent",
					mcp.Required(),
					mcp.Description("Time spent, e.g. '3h 30m', '2h', '30m'"),
				),
				mcp.WithString(
					"comment",
					mcp.Description("Optional worklog comment (plain text)"),
				),
				mcp.WithString(
					"started",
					mcp.Description("Optional start time ISO 8601, e.g. '2026-08-16T10:00:00.000+0000'"),
				),
			),
			ToolAddWorklog(svc, log),
		)
	}

	// --- AGILE READ (4 tools) ---
	if fs.IsEnabled(features.ModuleAgile, false) {
		s.AddTool(
			mcp.NewTool(
				"get_jira_boards",
				mcp.WithDescription("List Jira Software boards for a project"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key, e.g. PROJ"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of boards to return (default 50)"),
				),
			),
			ToolGetJiraBoards(agileSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_jira_sprints",
				mcp.WithDescription("List sprints for a Jira board"),
				mcp.WithNumber(
					"board_id",
					mcp.Required(),
					mcp.Description("Board ID (number)"),
				),
				mcp.WithString(
					"state",
					mcp.Description("Sprint state filter: active, future, closed, or empty for all"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of sprints to return (default 50)"),
				),
			),
			ToolGetJiraSprints(agileSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_active_sprint",
				mcp.WithDescription("Get the currently active sprint for a board"),
				mcp.WithNumber(
					"board_id",
					mcp.Required(),
					mcp.Description("Board ID (number)"),
				),
			),
			ToolGetJiraActiveSprint(agileSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_sprint_issues",
				mcp.WithDescription("List issues in a sprint"),
				mcp.WithNumber(
					"sprint_id",
					mcp.Required(),
					mcp.Description("Sprint ID (number)"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of issues to return (default 50)"),
				),
			),
			ToolGetJiraSprintIssues(agileSvc),
		)
	}

	// --- AGILE WRITE (4 tools) ---
	if fs.IsEnabled(features.ModuleAgile, true) {
		s.AddTool(
			mcp.NewTool(
				"create_sprint",
				mcp.WithDescription("Create a new sprint on a Jira board. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"name",
					mcp.Required(),
					mcp.Description("Sprint name, e.g. 'Sprint 8'"),
				),
				mcp.WithNumber(
					"board_id",
					mcp.Required(),
					mcp.Description("Board ID to create the sprint on"),
				),
				mcp.WithString(
					"start_date",
					mcp.Description("Sprint start date ISO 8601: 2024-01-15T00:00:00.000Z"),
				),
				mcp.WithString(
					"end_date",
					mcp.Description("Sprint end date ISO 8601: 2024-01-29T00:00:00.000Z"),
				),
			),
			ToolCreateSprint(agileSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"update_sprint",
				mcp.WithDescription("Update a sprint — close it (state=closed) or rename/redate it"),
				mcp.WithNumber(
					"sprint_id",
					mcp.Required(),
					mcp.Description("Sprint ID from get_jira_sprints"),
				),
				mcp.WithString(
					"state",
					mcp.Description("New state: 'closed' to complete the sprint"),
				),
				mcp.WithString(
					"name",
					mcp.Description("New sprint name"),
				),
				mcp.WithString(
					"start_date",
					mcp.Description("Start date ISO 8601: 2024-01-15T00:00:00.000Z"),
				),
				mcp.WithString(
					"end_date",
					mcp.Description("End date ISO 8601: 2024-01-29T00:00:00.000Z"),
				),
			),
			ToolUpdateSprint(agileSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"move_issues_to_sprint",
				mcp.WithDescription("Move one or more issues into a sprint (max 50 per call)"),
				mcp.WithNumber(
					"sprint_id",
					mcp.Required(),
					mcp.Description("Target sprint ID"),
				),
				mcp.WithString(
					"issue_keys",
					mcp.Required(),
					mcp.Description("Comma-separated issue keys: 'PROJ-1,PROJ-2'"),
				),
			),
			ToolMoveIssuesToSprint(agileSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"move_issues_to_epic",
				mcp.WithDescription("Link one or more issues to an epic"),
				mcp.WithString(
					"epic_key",
					mcp.Required(),
					mcp.Description("Epic issue key: 'PROJ-100'"),
				),
				mcp.WithString(
					"issue_keys",
					mcp.Required(),
					mcp.Description("Comma-separated issue keys: 'PROJ-1,PROJ-2'"),
				),
			),
			ToolMoveIssuesToEpic(agileSvc, log),
		)
	}

	// --- GOALS READ (3 tools) ---
	if fs.IsEnabled(features.ModuleGoals, false) {
		s.AddTool(
			mcp.NewTool(
				"get_site_id",
				mcp.WithDescription("Resolve an Atlassian subdomain to its cloudId for use in Goals API calls"),
				mcp.WithString(
					"subdomain",
					mcp.Required(),
					mcp.Description("Atlassian subdomain, e.g. 'myorg' for myorg.atlassian.net"),
				),
			),
			ToolGetSiteID(goalsSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_goal",
				mcp.WithDescription("Get an Atlassian Goal by its ARI"),
				mcp.WithString(
					"goal_id",
					mcp.Required(),
					mcp.Description("Goal ARI, e.g. ari:cloud:townsquare:{siteId}:goal/{uuid}"),
				),
			),
			ToolGetGoal(goalsSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"search_goals",
				mcp.WithDescription("Search Atlassian Goals with JQL-like syntax"),
				mcp.WithString(
					"site_id",
					mcp.Required(),
					mcp.Description("cloudId from get_site_id"),
				),
				mcp.WithString(
					"search_string",
					mcp.Description("Goals search query, e.g. 'status = on_track AND owner = \"acctId\"'"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Max goals to return (default 25)"),
				),
				mcp.WithString(
					"cursor",
					mcp.Description("Pagination cursor from previous search_goals response"),
				),
			),
			ToolSearchGoals(goalsSvc),
		)
	}

	// --- GOALS WRITE (3 tools) ---
	if fs.IsEnabled(features.ModuleGoals, true) {
		s.AddTool(
			mcp.NewTool(
				"update_goal_status",
				mcp.WithDescription("Post a status check-in for an Atlassian Goal. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"goal_id",
					mcp.Required(),
					mcp.Description("Goal ARI"),
				),
				mcp.WithString(
					"status",
					mcp.Required(),
					mcp.Description("New status: on_track | off_track | at_risk"),
				),
				mcp.WithNumber(
					"score",
					mcp.Description("Progress score 0-100 (optional)"),
				),
				mcp.WithString(
					"summary",
					mcp.Description("Plain text update summary (optional)"),
				),
			),
			ToolUpdateGoalStatus(goalsSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"create_goal",
				mcp.WithDescription("Create a new Atlassian Goal. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"site_id",
					mcp.Required(),
					mcp.Description("cloudId from get_site_id"),
				),
				mcp.WithString(
					"name",
					mcp.Required(),
					mcp.Description("Goal name"),
				),
				mcp.WithString(
					"goal_type_id",
					mcp.Required(),
					mcp.Description("Goal type ARI, e.g. ari:cloud:goal:{siteId}:goal-type/{activationId}/{goalTypeId}"),
				),
				mcp.WithString(
					"target_date",
					mcp.Required(),
					mcp.Description("Target date YYYY-MM-DD"),
				),
				mcp.WithString(
					"confidence",
					mcp.Description("Date confidence: QUARTER (default), DAY, WEEK, MONTH, YEAR"),
				),
				mcp.WithString(
					"description",
					mcp.Description("Optional plain text description (wrapped to ADF internally)"),
				),
			),
			ToolCreateGoal(goalsSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"edit_goal",
				mcp.WithDescription("Edit structural fields of an Atlassian Goal (name, targetDate, isArchived). Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"goal_id",
					mcp.Required(),
					mcp.Description("Goal ARI, e.g. ari:cloud:townsquare:{siteId}:goal/{uuid}"),
				),
				mcp.WithString(
					"name",
					mcp.Description("New goal name (optional)"),
				),
				mcp.WithString(
					"target_date",
					mcp.Description("New target date YYYY-MM-DD (optional)"),
				),
				mcp.WithString(
					"confidence",
					mcp.Description("Date confidence: QUARTER (default), DAY, WEEK, MONTH, YEAR (optional, only used when target_date is set)"),
				),
				mcp.WithString(
					"archive",
					mcp.Description("Archive or unarchive: 'true' to archive, 'false' to unarchive (optional)"),
				),
			),
			ToolEditGoal(goalsSvc, log),
		)
	}

	// --- METRICS READ (1 tool) ---
	if fs.IsEnabled(features.ModuleMetrics, false) {
		s.AddTool(
			mcp.NewTool(
				"get_goal_metrics",
				mcp.WithDescription("List all metric targets (metrics) for an Atlassian Goal"),
				mcp.WithString(
					"goal_id",
					mcp.Required(),
					mcp.Description("Goal ARI, e.g. ari:cloud:townsquare:{siteId}:goal/{uuid}"),
				),
			),
			ToolGetGoalMetrics(goalsSvc),
		)
	}

	// --- METRICS WRITE (3 tools) ---
	if fs.IsEnabled(features.ModuleMetrics, true) {
		s.AddTool(
			mcp.NewTool(
				"create_metric",
				mcp.WithDescription("Create a new metric and attach it to an Atlassian Goal. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"goal_id",
					mcp.Required(),
					mcp.Description("Goal ARI"),
				),
				mcp.WithString(
					"name",
					mcp.Required(),
					mcp.Description("Metric name, e.g. 'Revenue'"),
				),
				mcp.WithString(
					"metric_type",
					mcp.Required(),
					mcp.Description("Metric type: CURRENCY | NUMERIC | PERCENTAGE"),
				),
				mcp.WithString(
					"start_value",
					mcp.Required(),
					mcp.Description("Start value (numeric), e.g. '0'"),
				),
				mcp.WithString(
					"target_value",
					mcp.Required(),
					mcp.Description("Target value (numeric), e.g. '100'"),
				),
				mcp.WithString(
					"initial_value",
					mcp.Required(),
					mcp.Description("Initial current value (numeric), e.g. '0'"),
				),
			),
			ToolCreateMetric(goalsSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"update_metric_value",
				mcp.WithDescription("Add a value datapoint to an existing metric. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"metric_id",
					mcp.Required(),
					mcp.Description("Metric ID"),
				),
				mcp.WithString(
					"value",
					mcp.Required(),
					mcp.Description("Numeric value to record, e.g. '75'"),
				),
				mcp.WithString(
					"time",
					mcp.Description("Timestamp ISO 8601, e.g. '2024-01-15T00:00:00Z' (optional, defaults to now)"),
				),
			),
			ToolUpdateMetricValue(goalsSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"update_metric_target",
				mcp.WithDescription("Update start, current, and/or target values of a MetricTarget. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"metric_target_id",
					mcp.Required(),
					mcp.Description("MetricTarget ID"),
				),
				mcp.WithString(
					"current_value",
					mcp.Description("New current value (numeric string, optional)"),
				),
				mcp.WithString(
					"start_value",
					mcp.Description("New start value (numeric string, optional)"),
				),
				mcp.WithString(
					"target_value",
					mcp.Description("New target value (numeric string, optional)"),
				),
			),
			ToolUpdateMetricTarget(goalsSvc, log),
		)
	}

	// --- RELEASES READ (3 tools) ---
	if fs.IsEnabled(features.ModuleReleases, false) {
		s.AddTool(
			mcp.NewTool(
				"search_releases",
				mcp.WithDescription("List all Jira releases (versions) for a project"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key, e.g. PROJ"),
				),
			),
			ToolSearchReleases(releasesSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_release",
				mcp.WithDescription("Get a Jira release (version) by its ID"),
				mcp.WithString(
					"release_id",
					mcp.Required(),
					mcp.Description("Release ID, e.g. 10001"),
				),
			),
			ToolGetRelease(releasesSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_release_issues",
				mcp.WithDescription("Get issue counts for a Jira release (fix version and affects version)"),
				mcp.WithString(
					"release_id",
					mcp.Required(),
					mcp.Description("Release ID, e.g. 10001"),
				),
			),
			ToolGetReleaseIssues(releasesSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"validate_release_for_deploy",
				mcp.WithDescription("Run deploy-readiness validation rules against the issues linked to a release (via fixVersion)"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key, e.g. PROJ"),
				),
				mcp.WithString(
					"release_name",
					mcp.Required(),
					mcp.Description("Release (fix version) name, e.g. 'v1.0.0'"),
				),
				mcp.WithString(
					"rules",
					mcp.Description("Comma-separated rule names to run (optional). Defaults to all_issues_done,no_critical_bugs_open,no_blocking_issues,min_issues_count"),
				),
			),
			ToolValidateReleaseForDeploy(svc),
		)

		s.AddTool(
			mcp.NewTool(
				"generate_release_notes",
				mcp.WithDescription("Generate Markdown release notes grouped by issue type for a release (via fixVersion)"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key, e.g. PROJ"),
				),
				mcp.WithString(
					"release_name",
					mcp.Required(),
					mcp.Description("Release (fix version) name, e.g. 'v1.0.0'"),
				),
			),
			ToolGenerateReleaseNotes(svc),
		)
	}

	// --- RELEASES WRITE (2 tools) ---
	if fs.IsEnabled(features.ModuleReleases, true) {
		s.AddTool(
			mcp.NewTool(
				"create_release",
				mcp.WithDescription("Create a new Jira release (version). Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"project_id",
					mcp.Required(),
					mcp.Description("Project ID (numeric string), e.g. '10000'"),
				),
				mcp.WithString(
					"name",
					mcp.Required(),
					mcp.Description("Release name, e.g. 'v1.0.0'"),
				),
				mcp.WithString(
					"description",
					mcp.Description("Release description (optional)"),
				),
				mcp.WithString(
					"start_date",
					mcp.Description("Start date YYYY-MM-DD (optional)"),
				),
				mcp.WithString(
					"release_date",
					mcp.Description("Release date YYYY-MM-DD (optional)"),
				),
			),
			ToolCreateRelease(releasesSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"update_release",
				mcp.WithDescription("Update a Jira release (version). Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"release_id",
					mcp.Required(),
					mcp.Description("Release ID, e.g. 10001"),
				),
				mcp.WithString(
					"name",
					mcp.Description("New release name (optional)"),
				),
				mcp.WithString(
					"description",
					mcp.Description("New description (optional)"),
				),
				mcp.WithString(
					"released",
					mcp.Description("Mark as released: 'true' or 'false' (optional)"),
				),
				mcp.WithString(
					"archived",
					mcp.Description("Archive the release: 'true' or 'false' (optional)"),
				),
				mcp.WithString(
					"release_date",
					mcp.Description("New release date YYYY-MM-DD (optional)"),
				),
			),
			ToolUpdateRelease(releasesSvc, log),
		)
	}

	// --- PROJECTS READ (3 tools) ---
	if fs.IsEnabled(features.ModuleProjects, false) {
		s.AddTool(
			mcp.NewTool(
				"list_projects",
				mcp.WithDescription("List all Jira projects"),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of projects to return (default 50)"),
				),
			),
			ToolListProjects(projectsSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_project",
				mcp.WithDescription("Get a Jira project by key or ID"),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key or ID, e.g. PROJ or 10000"),
				),
			),
			ToolGetProject(projectsSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"search_projects",
				mcp.WithDescription("Search Jira projects by name or key"),
				mcp.WithString(
					"query",
					mcp.Description("Search query (optional — omit to list all)"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of results to return (default 50)"),
				),
			),
			ToolSearchProjects(projectsSvc),
		)
	}

	// --- PROJECTS WRITE (1 tool) ---
	if fs.IsEnabled(features.ModuleProjects, true) {
		s.AddTool(
			mcp.NewTool(
				"update_project",
				mcp.WithDescription("Update a Jira project. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"project_key",
					mcp.Required(),
					mcp.Description("Project key or ID, e.g. PROJ"),
				),
				mcp.WithString(
					"name",
					mcp.Description("New project name (optional)"),
				),
				mcp.WithString(
					"description",
					mcp.Description("New project description (optional)"),
				),
				mcp.WithString(
					"lead",
					mcp.Description("New lead account ID (optional)"),
				),
			),
			ToolUpdateProject(projectsSvc, log),
		)
	}

	// --- TEAMS READ (3 tools) ---
	if fs.IsEnabled(features.ModuleTeams, false) {
		s.AddTool(
			mcp.NewTool(
				"search_teams",
				mcp.WithDescription("List or search Atlassian teams in the organization"),
				mcp.WithString(
					"query",
					mcp.Description("Optional search query to filter teams by name"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of teams to return (default 50)"),
				),
			),
			ToolSearchTeams(teamsSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_team",
				mcp.WithDescription("Get an Atlassian team by its ID"),
				mcp.WithString(
					"team_id",
					mcp.Required(),
					mcp.Description("Team ID (UUID)"),
				),
			),
			ToolGetTeam(teamsSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_team_members",
				mcp.WithDescription("List members of an Atlassian team"),
				mcp.WithString(
					"team_id",
					mcp.Required(),
					mcp.Description("Team ID (UUID)"),
				),
				mcp.WithNumber(
					"max_results",
					mcp.Description("Maximum number of members to return (default 50)"),
				),
			),
			ToolGetTeamMembers(teamsSvc),
		)
	}
	// TEAMS has no write tools currently.

	// --- BITBUCKET READ (12 tools) ---
	if fs.IsEnabled(features.ModuleBitbucket, false) {
		wsDesc := mcp.WithString("workspace", mcp.Description("Bitbucket workspace slug (overrides BITBUCKET_WORKSPACE env)"))
		repoDesc := mcp.WithString("repo", mcp.Description("Repository slug (overrides BITBUCKET_REPO env)"))
		prIDDesc := mcp.WithNumber("pr_id", mcp.Required(), mcp.Description("Pull request ID"))

		s.AddTool(
			mcp.NewTool("bb_list_repos",
				mcp.WithDescription("List Bitbucket repositories in a workspace"),
				wsDesc,
			),
			ToolBitbucketListRepos(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_list_prs",
				mcp.WithDescription("List pull requests for a Bitbucket repository"),
				wsDesc, repoDesc,
				mcp.WithString("state", mcp.Description("PR state filter: OPEN, MERGED, DECLINED, SUPERSEDED (default OPEN)")),
			),
			ToolBitbucketListPRs(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_get_pr",
				mcp.WithDescription("Get details of a specific Bitbucket pull request"),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketGetPR(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_pr_comments",
				mcp.WithDescription("List comments on a Bitbucket pull request"),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketPRComments(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_pr_commits",
				mcp.WithDescription("List commits in a Bitbucket pull request"),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketPRCommits(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_pr_files",
				mcp.WithDescription("List files changed in a Bitbucket pull request (diffstat)"),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketPRFiles(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_pr_diff",
				mcp.WithDescription("Get the raw diff for a Bitbucket pull request"),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketPRDiff(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_pr_checks",
				mcp.WithDescription("Get build/pipeline status checks for a Bitbucket pull request"),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketPRChecks(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_pr_reviewers",
				mcp.WithDescription("List reviewers of a Bitbucket pull request"),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketPRReviewers(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_list_branches",
				mcp.WithDescription("List branches in a Bitbucket repository"),
				wsDesc, repoDesc,
			),
			ToolBitbucketListBranches(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_stale_branches",
				mcp.WithDescription("List Bitbucket branches with no commits in the last N days"),
				wsDesc, repoDesc,
				mcp.WithNumber("days", mcp.Description("Number of days to consider stale (default 30)")),
			),
			ToolBitbucketStaleBranches(bitbucketSvc),
		)

		s.AddTool(
			mcp.NewTool("bb_list_pipelines",
				mcp.WithDescription("List recent pipelines for a Bitbucket repository"),
				wsDesc, repoDesc,
			),
			ToolBitbucketListPipelines(bitbucketSvc),
		)
	}

	// --- BITBUCKET WRITE (6 tools) ---
	if fs.IsEnabled(features.ModuleBitbucket, true) {
		wsDesc := mcp.WithString("workspace", mcp.Description("Bitbucket workspace slug (overrides BITBUCKET_WORKSPACE env)"))
		repoDesc := mcp.WithString("repo", mcp.Description("Repository slug (overrides BITBUCKET_REPO env)"))
		prIDDesc := mcp.WithNumber("pr_id", mcp.Required(), mcp.Description("Pull request ID"))

		s.AddTool(
			mcp.NewTool("bb_create_pr",
				mcp.WithDescription("Create a Bitbucket pull request. Requires ENABLE_WRITE=true."),
				wsDesc, repoDesc,
				mcp.WithString("title", mcp.Required(), mcp.Description("Pull request title")),
				mcp.WithString("source", mcp.Required(), mcp.Description("Source branch name")),
				mcp.WithString("destination", mcp.Required(), mcp.Description("Destination branch name")),
				mcp.WithString("description", mcp.Description("Pull request description (optional)")),
				mcp.WithString("reviewers", mcp.Description("Comma-separated reviewer nicknames or {uuid} values (optional)")),
				mcp.WithBoolean("close_source_branch", mcp.Description("Close source branch after merge (optional)")),
			),
			ToolBitbucketCreatePR(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_comment_pr",
				mcp.WithDescription("Add a comment to a Bitbucket pull request. Requires ENABLE_WRITE=true."),
				wsDesc, repoDesc, prIDDesc,
				mcp.WithString("message", mcp.Required(), mcp.Description("Comment text to post")),
			),
			ToolBitbucketCommentPR(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_update_pr",
				mcp.WithDescription("Update a Bitbucket pull request (title/description/destination/reviewers). Requires ENABLE_WRITE=true. At least one optional field must be provided."),
				wsDesc, repoDesc, prIDDesc,
				mcp.WithString("title", mcp.Description("New pull request title (optional)")),
				mcp.WithString("description", mcp.Description("New pull request description (optional)")),
				mcp.WithString("destination", mcp.Description("New destination branch name (optional)")),
				mcp.WithString("reviewers", mcp.Description("Comma-separated reviewer nicknames or {uuid} values; replaces existing reviewers (optional)")),
			),
			ToolBitbucketUpdatePR(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_approve_pr",
				mcp.WithDescription("Approve a Bitbucket pull request. Requires ENABLE_WRITE=true. Idempotent."),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketApprovePR(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_decline_pr",
				mcp.WithDescription("Decline (reject) a Bitbucket pull request. Requires ENABLE_WRITE=true."),
				wsDesc, repoDesc, prIDDesc,
			),
			ToolBitbucketDeclinePR(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_merge_pr",
				mcp.WithDescription("Merge a Bitbucket pull request. Requires ENABLE_WRITE=true."),
				wsDesc, repoDesc, prIDDesc,
				mcp.WithString("strategy", mcp.Description("Merge strategy: merge_commit, squash, fast_forward, squash_fast_forward, rebase_fast_forward, rebase_merge (optional)")),
				mcp.WithString("message", mcp.Description("Custom merge commit message (optional)")),
			),
			ToolBitbucketMergePR(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_run_pipeline",
				mcp.WithDescription("Trigger a Bitbucket pipeline for a branch. Requires ENABLE_WRITE=true."),
				wsDesc, repoDesc,
				mcp.WithString("branch", mcp.Required(), mcp.Description("Branch to run the pipeline on")),
			),
			ToolBitbucketRunPipeline(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_create_pr_task",
				mcp.WithDescription("Create a task on a Bitbucket pull request. Requires ENABLE_WRITE=true."),
				wsDesc, repoDesc, prIDDesc,
				mcp.WithString("message", mcp.Required(), mcp.Description("Task description text")),
			),
			ToolBitbucketCreatePRTask(bitbucketSvc, log),
		)

		s.AddTool(
			mcp.NewTool("bb_resolve_pr_task",
				mcp.WithDescription("Resolve a task on a Bitbucket pull request. Requires ENABLE_WRITE=true."),
				wsDesc, repoDesc, prIDDesc,
				mcp.WithNumber("task_id", mcp.Required(), mcp.Description("Task ID to resolve")),
			),
			ToolBitbucketResolvePRTask(bitbucketSvc, log),
		)
	}

	// --- CONFLUENCE READ (8 tools incl. search) ---
	if fs.IsEnabled(features.ModuleConfluence, false) {
		s.AddTool(
			mcp.NewTool(
				"get_confluence_page",
				mcp.WithDescription("Get a Confluence page by ID"),
				mcp.WithString(
					"page_id",
					mcp.Required(),
					mcp.Description("The Confluence page ID"),
				),
				mcp.WithString(
					"body_format",
					mcp.Description("Body format to return (default 'storage' = XHTML)"),
				),
			),
			ToolGetConfluencePage(confluenceSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_pages_in_space",
				mcp.WithDescription("List pages in a Confluence space"),
				mcp.WithString(
					"space_id",
					mcp.Required(),
					mcp.Description("The Confluence space ID"),
				),
				mcp.WithNumber(
					"limit",
					mcp.Description("Maximum number of pages to return"),
				),
				mcp.WithString(
					"cursor",
					mcp.Description("Pagination cursor from a previous response"),
				),
			),
			ToolGetPagesInSpace(confluenceSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_confluence_spaces",
				mcp.WithDescription("List Confluence spaces"),
				mcp.WithNumber(
					"limit",
					mcp.Description("Maximum number of spaces to return"),
				),
				mcp.WithString(
					"cursor",
					mcp.Description("Pagination cursor from a previous response"),
				),
				mcp.WithString(
					"keys",
					mcp.Description("Comma-separated space keys to filter by (optional)"),
				),
				mcp.WithString(
					"type",
					mcp.Description("Space type filter, e.g. 'global' or 'personal' (optional)"),
				),
			),
			ToolGetConfluenceSpaces(confluenceSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_page_descendants",
				mcp.WithDescription("List descendant pages of a Confluence page"),
				mcp.WithString(
					"page_id",
					mcp.Required(),
					mcp.Description("The Confluence page ID"),
				),
				mcp.WithNumber(
					"limit",
					mcp.Description("Maximum number of descendants to return"),
				),
				mcp.WithString(
					"cursor",
					mcp.Description("Pagination cursor from a previous response"),
				),
			),
			ToolGetPageDescendants(confluenceSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_page_footer_comments",
				mcp.WithDescription("List footer comments on a Confluence page"),
				mcp.WithString(
					"page_id",
					mcp.Required(),
					mcp.Description("The Confluence page ID"),
				),
				mcp.WithNumber(
					"limit",
					mcp.Description("Maximum number of comments to return"),
				),
				mcp.WithString(
					"cursor",
					mcp.Description("Pagination cursor from a previous response"),
				),
			),
			ToolGetPageFooterComments(confluenceSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_page_inline_comments",
				mcp.WithDescription("List inline comments on a Confluence page"),
				mcp.WithString(
					"page_id",
					mcp.Required(),
					mcp.Description("The Confluence page ID"),
				),
				mcp.WithNumber(
					"limit",
					mcp.Description("Maximum number of comments to return"),
				),
				mcp.WithString(
					"cursor",
					mcp.Description("Pagination cursor from a previous response"),
				),
			),
			ToolGetPageInlineComments(confluenceSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"get_comment_children",
				mcp.WithDescription("List child (reply) comments of a Confluence comment"),
				mcp.WithString(
					"comment_id",
					mcp.Required(),
					mcp.Description("The Confluence comment ID"),
				),
				mcp.WithNumber(
					"limit",
					mcp.Description("Maximum number of children to return"),
				),
				mcp.WithString(
					"cursor",
					mcp.Description("Pagination cursor from a previous response"),
				),
			),
			ToolGetCommentChildren(confluenceSvc),
		)

		s.AddTool(
			mcp.NewTool(
				"search_confluence",
				mcp.WithDescription("Search Confluence content with CQL (Confluence Query Language)"),
				mcp.WithString(
					"cql",
					mcp.Required(),
					mcp.Description("CQL query string, e.g. 'type=page AND space=DEV'"),
				),
				mcp.WithNumber(
					"limit",
					mcp.Description("Maximum number of results to return"),
				),
			),
			ToolSearchConfluence(confluenceSvc),
		)
	}

	// --- CONFLUENCE WRITE (4 tools) ---
	if fs.IsEnabled(features.ModuleConfluence, true) {
		s.AddTool(
			mcp.NewTool(
				"create_confluence_page",
				mcp.WithDescription("Create a new Confluence page. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"space_id",
					mcp.Required(),
					mcp.Description("The Confluence space ID where the page will be created"),
				),
				mcp.WithString(
					"title",
					mcp.Required(),
					mcp.Description("Page title"),
				),
				mcp.WithString(
					"body",
					mcp.Required(),
					mcp.Description("Page body in Confluence storage format (XHTML)"),
				),
				mcp.WithString(
					"parent_id",
					mcp.Description("Parent page ID (optional)"),
				),
				mcp.WithString(
					"status",
					mcp.Description("Page status (optional, default 'current')"),
				),
			),
			ToolCreateConfluencePage(confluenceSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"update_confluence_page",
				mcp.WithDescription("Update an existing Confluence page. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"page_id",
					mcp.Required(),
					mcp.Description("The Confluence page ID to update"),
				),
				mcp.WithString(
					"title",
					mcp.Required(),
					mcp.Description("New page title"),
				),
				mcp.WithString(
					"body",
					mcp.Required(),
					mcp.Description("New page body in Confluence storage format (XHTML)"),
				),
				mcp.WithString(
					"status",
					mcp.Description("Page status (optional, default 'current')"),
				),
				mcp.WithNumber(
					"version_number",
					mcp.Description("Page version number (optional — omit to auto-increment from current version)"),
				),
			),
			ToolUpdateConfluencePage(confluenceSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"create_footer_comment",
				mcp.WithDescription("Create a footer comment on a Confluence page. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"page_id",
					mcp.Required(),
					mcp.Description("The Confluence page ID"),
				),
				mcp.WithString(
					"body",
					mcp.Required(),
					mcp.Description("Comment body in Confluence storage format (XHTML)"),
				),
				mcp.WithString(
					"parent_comment_id",
					mcp.Description("Parent comment ID for threaded replies (optional)"),
				),
			),
			ToolCreateFooterComment(confluenceSvc, log),
		)

		s.AddTool(
			mcp.NewTool(
				"create_inline_comment",
				mcp.WithDescription("Create an inline comment anchored to selected text on a Confluence page. Requires ENABLE_WRITE=true."),
				mcp.WithString(
					"page_id",
					mcp.Required(),
					mcp.Description("The Confluence page ID"),
				),
				mcp.WithString(
					"body",
					mcp.Required(),
					mcp.Description("Comment body in Confluence storage format (XHTML)"),
				),
				mcp.WithString(
					"text_selection",
					mcp.Required(),
					mcp.Description("The exact text on the page that the comment anchors to (required)"),
				),
				mcp.WithNumber(
					"text_selection_match_count",
					mcp.Description("Total number of times the selected text appears on the page (optional)"),
				),
				mcp.WithNumber(
					"text_selection_match_index",
					mcp.Description("Which occurrence of the selected text to anchor to, 0-based (optional)"),
				),
			),
			ToolCreateInlineComment(confluenceSvc, log),
		)
	}

	return s
}

// StartServer wires jira.Service, agile.AgileService, goals.GoalsService, releases.ReleasesService,
// projects.ProjectsService, teams.TeamsService, bitbucket.BitbucketService, confluence.Service,
// an audit.Logger, and a FeatureSet into the MCP server and starts the stdio loop.
// Blocks until the server exits. Call from cmd/mcp after setting log.SetOutput(os.Stderr)
// to guarantee stdout discipline.
// A nil FeatureSet enables all 79 tools.
func StartServer(svc jira.Service, agileSvc agile.AgileService, goalsSvc goals.GoalsService, releasesSvc releases.ReleasesService, projectsSvc projects.ProjectsService, teamsSvc teams.TeamsService, bitbucketSvc bitbucket.BitbucketService, confluenceSvc confluence.Service, auditLog audit.Logger, fs *features.FeatureSet) error {
	s := NewAtlassianServer(svc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, bitbucketSvc, confluenceSvc, auditLog, fs)
	return server.ServeStdio(s)
}
