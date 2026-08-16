// Command atlassian is a CLI for Atlassian (Jira, Agile, Goals) operations.
// It reads ATLASSIAN_BASE_URL, ATLASSIAN_EMAIL, and ATLASSIAN_TOKEN from
// the environment and fails with a clear message if any are missing.
package main

import (
	"context"
	"fmt"
	"os"

	atlcliagile "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/agile"
	atlclibitbucket "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/bitbucket"
	atlcligoals "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/cmd/atlassian/jira"
	atlcliprojects "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/projects"
	atlclireleases "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/releases"
	atlcliteams "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	agilesvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	bitbucketsvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/bitbucket"
	atlclient "github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	projectssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	releasessvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	teamssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
	"github.com/jinkp/atlassian-go-mcp/internal/envstore"
	"github.com/spf13/cobra"
)

func main() {
	root := buildRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	auditLog := audit.NewJSONLogger(os.Stderr)
	var dryRun bool

	root := &cobra.Command{
		Use:   "atlassian",
		Short: "CLI for Atlassian (Jira, Agile, Goals) operations",
		// PersistentPreRunE validates env vars before any sub-command runs.
		// Bitbucket commands use a different credential set than the Jira APIs.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if isBitbucketCommand(cmd) {
				return validateBitbucketEnv()
			}
			return validateEnv()
		},
	}
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Print what would happen without executing write operations")

	// Build the client from env vars. We do this lazily in PersistentPreRunE
	// so validation happens before any command runs.
	cfg, err := configFromEnv()
	if err != nil {
		// This will be caught by PersistentPreRunE on first command execution.
		// We wire nil-safe services so cobra can at least show --help.
	}

	var (
		svc         jirasvc.Service
		agileSvc    agilesvc.AgileService
		goalsSvc    goalssvc.GoalsService
		releasesSvc releasessvc.ReleasesService
		projectsSvc projectssvc.ProjectsService
		teamsSvc    teamssvc.TeamsService
	)

	if err == nil {
		c, clientErr := atlclient.NewClient(cfg)
		if clientErr == nil {
			svc = jirasvc.NewService(c, cfg.BaseURL)
			agileSvc = agilesvc.NewService(c, cfg.BaseURL)
			goalsSvc = goalssvc.NewService(c, cfg.BaseURL)
			releasesSvc = releasessvc.NewService(c, cfg.BaseURL)
			projectsSvc = projectssvc.NewService(c, cfg.BaseURL)
			// ATLASSIAN_ORG_ID is required for teams commands only — validate lazily.
			orgID := os.Getenv("ATLASSIAN_ORG_ID")
			teamsSvc = teamssvc.NewService(c, orgID)
		}
	}

	// If env vars missing, services will be nil; PersistentPreRunE will exit(1)
	// before RunE is reached so nil services are safe.
	if svc == nil {
		svc = &nilService{}
	}
	if agileSvc == nil {
		agileSvc = &nilAgileService{}
	}
	if goalsSvc == nil {
		goalsSvc = &nilGoalsService{}
	}
	if releasesSvc == nil {
		releasesSvc = &nilReleasesService{}
	}
	if projectsSvc == nil {
		projectsSvc = &nilProjectsService{}
	}
	if teamsSvc == nil {
		teamsSvc = &nilTeamsService{}
	}

	// Jira subgroup
	jiraRoot := jira.NewJiraCmd()
	jiraRoot.AddCommand(jira.NewGetCmd(svc))
	jiraRoot.AddCommand(jira.NewSearchCmd(svc))
	jiraRoot.AddCommand(jira.NewCreateCmd(svc, auditLog, dryRun))
	jiraRoot.AddCommand(jira.NewUpdateCmd(svc, auditLog, dryRun))
	jiraRoot.AddCommand(jira.NewTransitionsCmd(svc))
	jiraRoot.AddCommand(jira.NewTransitionCmd(svc, auditLog, dryRun))
	root.AddCommand(jiraRoot)

	// Agile subgroup
	agileRoot := atlcliagile.NewAgileCmd()
	atlcliagile.RegisterCommands(agileRoot, agileSvc, auditLog, dryRun)
	root.AddCommand(agileRoot)

	// Goals subgroup
	goalsRoot := atlcligoals.NewGoalsCmd()
	atlcligoals.RegisterCommands(goalsRoot, goalsSvc, auditLog, dryRun)
	root.AddCommand(goalsRoot)

	// Releases subgroup
	releasesRoot := atlclireleases.NewReleasesCmd()
	atlclireleases.RegisterCommands(releasesRoot, releasesSvc, auditLog, dryRun)
	root.AddCommand(releasesRoot)

	// Projects subgroup
	projectsRoot := atlcliprojects.NewProjectsCmd()
	atlcliprojects.RegisterCommands(projectsRoot, projectsSvc, auditLog, dryRun)
	root.AddCommand(projectsRoot)

	// Teams subgroup
	teamsRoot := atlcliteams.NewTeamsCmd()
	atlcliteams.RegisterCommands(teamsRoot, teamsSvc)
	root.AddCommand(teamsRoot)

	// Bitbucket subgroup — uses its own host/credentials (BITBUCKET_*), loaded from
	// the shared ~/.atlassian/credentials.env. The client is always constructible
	// (only BaseURL is required); missing creds are caught by PersistentPreRunE.
	bbCreds := envstore.LoadBitbucket()
	if bbCreds.Workspace != "" && os.Getenv("BITBUCKET_WORKSPACE") == "" {
		_ = os.Setenv("BITBUCKET_WORKSPACE", bbCreds.Workspace)
	}
	if bbCreds.Repo != "" && os.Getenv("BITBUCKET_REPO") == "" {
		_ = os.Setenv("BITBUCKET_REPO", bbCreds.Repo)
	}
	var bitbucketSvc bitbucketsvc.BitbucketService
	if bbClient, bbErr := atlclient.NewClient(atlclient.Config{
		BaseURL:  bitbucketsvc.CloudBaseURL,
		Email:    bbCreds.Username,
		APIToken: bbCreds.APIToken,
	}); bbErr == nil {
		bitbucketSvc = bitbucketsvc.NewService(bbClient, bitbucketsvc.CloudBaseURL)
	}
	bitbucketRoot := atlclibitbucket.NewBitbucketCmd()
	atlclibitbucket.RegisterCommands(bitbucketRoot, bitbucketSvc, auditLog, dryRun)
	root.AddCommand(bitbucketRoot)

	return root
}

// validateEnv checks that all required environment variables are set.
func validateEnv() error {
	required := []string{"ATLASSIAN_BASE_URL", "ATLASSIAN_EMAIL", "ATLASSIAN_TOKEN"}
	for _, key := range required {
		if os.Getenv(key) == "" {
			fmt.Fprintf(os.Stderr, "error: %s is required\n", key)
			os.Exit(1)
		}
	}
	return nil
}

// isBitbucketCommand reports whether cmd belongs to the "bitbucket" subgroup.
func isBitbucketCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "bitbucket" {
			return true
		}
	}
	return false
}

// validateBitbucketEnv checks that Bitbucket credentials are available (env vars
// or the shared ~/.atlassian/credentials.env). Exits(1) with a clear message otherwise.
func validateBitbucketEnv() error {
	creds := envstore.LoadBitbucket()
	if creds.Username == "" {
		fmt.Fprintln(os.Stderr, "error: BITBUCKET_USERNAME is required (env or ~/.atlassian/credentials.env)")
		os.Exit(1)
	}
	if creds.APIToken == "" {
		fmt.Fprintln(os.Stderr, "error: BITBUCKET_API_TOKEN is required (env or ~/.atlassian/credentials.env)")
		os.Exit(1)
	}
	return nil
}

// configFromEnv constructs a client.Config from environment variables.
func configFromEnv() (atlclient.Config, error) {
	baseURL := os.Getenv("ATLASSIAN_BASE_URL")
	email := os.Getenv("ATLASSIAN_EMAIL")
	token := os.Getenv("ATLASSIAN_TOKEN")

	if baseURL == "" || email == "" || token == "" {
		return atlclient.Config{}, fmt.Errorf("missing required environment variables")
	}

	return atlclient.Config{
		BaseURL:  baseURL,
		Email:    email,
		APIToken: token,
	}, nil
}

// --- nilService: no-op Jira service for --help without credentials ---
// Never reached because PersistentPreRunE exits(1) first.

type nilService struct{}

func (n *nilService) GetIssue(_ context.Context, _ string) (*jirasvc.Issue, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilService) SearchIssues(_ context.Context, _ string, _ int) (*jirasvc.SearchResult, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilService) CreateIssue(_ context.Context, _ jirasvc.CreateIssueRequest) (*jirasvc.CreateIssueResponse, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilService) UpdateIssue(_ context.Context, _ string, _ jirasvc.UpdateIssueRequest) error {
	return fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilService) GetTransitions(_ context.Context, _ string) ([]jirasvc.Transition, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilService) TransitionIssue(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("service not initialized: missing env vars")
}

// --- nilAgileService: no-op Agile service for --help without credentials ---

type nilAgileService struct{}

func (n *nilAgileService) GetBoards(_ context.Context, _ string, _ int) ([]agilesvc.Board, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilAgileService) GetSprints(_ context.Context, _ int, _ string, _ int) ([]agilesvc.Sprint, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilAgileService) GetSprintIssues(_ context.Context, _ int, _ int) (*agilesvc.SprintIssueResult, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilAgileService) UpdateSprint(_ context.Context, _ int, _ agilesvc.UpdateSprintRequest) (*agilesvc.Sprint, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilAgileService) MoveIssuesToSprint(_ context.Context, _ int, _ []string) error {
	return fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilAgileService) MoveIssuesToEpic(_ context.Context, _ string, _ []string) error {
	return fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilAgileService) CreateSprint(_ context.Context, _ agilesvc.CreateSprintRequest) (*agilesvc.Sprint, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

// --- nilGoalsService: no-op Goals service for --help without credentials ---

type nilGoalsService struct{}

func (n *nilGoalsService) GetSiteID(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilGoalsService) GetGoal(_ context.Context, _ string) (*goalssvc.Goal, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilGoalsService) SearchGoals(_ context.Context, _ goalssvc.SearchGoalsRequest) (*goalssvc.GoalSearchResult, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilGoalsService) UpdateGoalStatus(_ context.Context, _ goalssvc.UpdateGoalStatusRequest) error {
	return fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilGoalsService) CreateGoal(_ context.Context, _ goalssvc.CreateGoalRequest) (*goalssvc.CreateGoalResult, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

func (n *nilGoalsService) EditGoal(_ context.Context, _ goalssvc.EditGoalRequest) (*goalssvc.Goal, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

func (n *nilGoalsService) GetGoalMetrics(_ context.Context, _ string) ([]goalssvc.MetricTarget, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

func (n *nilGoalsService) CreateMetric(_ context.Context, _ goalssvc.CreateMetricRequest) (*goalssvc.MetricTarget, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

func (n *nilGoalsService) UpdateMetricValue(_ context.Context, _ goalssvc.UpdateMetricValueRequest) (*goalssvc.MetricValue, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

func (n *nilGoalsService) UpdateMetricTarget(_ context.Context, _ goalssvc.UpdateMetricTargetRequest) error {
	return fmt.Errorf("service not initialized: missing env vars")
}

// --- nilReleasesService: no-op Releases service for --help without credentials ---

type nilReleasesService struct{}

func (n *nilReleasesService) GetReleases(_ context.Context, _ string) ([]releasessvc.Release, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilReleasesService) GetRelease(_ context.Context, _ string) (*releasessvc.Release, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilReleasesService) GetReleaseIssueCounts(_ context.Context, _ string) (*releasessvc.ReleaseIssueCounts, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilReleasesService) CreateRelease(_ context.Context, _ releasessvc.CreateReleaseRequest) (*releasessvc.Release, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilReleasesService) UpdateRelease(_ context.Context, _ string, _ releasessvc.UpdateReleaseRequest) (*releasessvc.Release, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

// --- nilProjectsService: no-op Projects service for --help without credentials ---

type nilProjectsService struct{}

func (n *nilProjectsService) GetProjects(_ context.Context, _ int) ([]projectssvc.Project, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilProjectsService) GetProject(_ context.Context, _ string) (*projectssvc.Project, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilProjectsService) SearchProjects(_ context.Context, _ projectssvc.SearchProjectsRequest) (*projectssvc.SearchProjectsResult, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilProjectsService) UpdateProject(_ context.Context, _ string, _ projectssvc.UpdateProjectRequest) (*projectssvc.Project, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}

// --- nilTeamsService: no-op Teams service for --help without credentials ---
// Never reached because PersistentPreRunE exits(1) first.

type nilTeamsService struct{}

func (n *nilTeamsService) GetTeams(_ context.Context, _ string, _ int) (*teamssvc.TeamSearchResult, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilTeamsService) GetTeam(_ context.Context, _ string) (*teamssvc.Team, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
func (n *nilTeamsService) GetTeamMembers(_ context.Context, _ string, _ int) ([]teamssvc.TeamMember, error) {
	return nil, fmt.Errorf("service not initialized: missing env vars")
}
