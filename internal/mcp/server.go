package mcpserver

import (
	"errors"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
)

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
// Future write tools MUST call this and return the error result via mcp.NewToolResultError.
func WriteGuardCheck() error {
	if os.Getenv("ENABLE_WRITE") == "true" {
		return nil
	}
	return errors.New("write operations disabled: set ENABLE_WRITE=true to enable write tools")
}

// NewAtlassianServer creates a configured MCPServer with Atlassian tools registered
// according to the provided FeatureSet. A nil FeatureSet enables all 37 tools (backward compat).
// log receives an entry for every write operation (after WriteGuardCheck passes).
func NewAtlassianServer(svc jira.Service, agileSvc agile.AgileService, goalsSvc goals.GoalsService, releasesSvc releases.ReleasesService, projectsSvc projects.ProjectsService, teamsSvc teams.TeamsService, log audit.Logger, fs *features.FeatureSet) *server.MCPServer {
	s := server.NewMCPServer(
		"atlassian-platform-connector",
		"1.0.0",
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

	return s
}

// StartServer wires jira.Service, agile.AgileService, goals.GoalsService, releases.ReleasesService,
// projects.ProjectsService, teams.TeamsService, an audit.Logger, and a FeatureSet into the MCP server
// and starts the stdio loop. Blocks until the server exits. Call from cmd/mcp after setting
// log.SetOutput(os.Stderr) to guarantee stdout discipline.
// A nil FeatureSet enables all 37 tools.
func StartServer(svc jira.Service, agileSvc agile.AgileService, goalsSvc goals.GoalsService, releasesSvc releases.ReleasesService, projectsSvc projects.ProjectsService, teamsSvc teams.TeamsService, auditLog audit.Logger, fs *features.FeatureSet) error {
	s := NewAtlassianServer(svc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, auditLog, fs)
	return server.ServeStdio(s)
}
