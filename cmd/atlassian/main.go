// Command atlassian is a CLI for Atlassian (Jira, Agile, Goals) operations.
// It reads ATLASSIAN_BASE_URL, ATLASSIAN_EMAIL, and ATLASSIAN_TOKEN from
// the environment and fails with a clear message if any are missing.
package main

import (
	"context"
	"fmt"
	"os"

	atlcliagile "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/agile"
	atlcligoals "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/cmd/atlassian/jira"
	atlclient "github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	agilesvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	goalssvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	jirasvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/spf13/cobra"
)

func main() {
	root := buildRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "atlassian",
		Short: "CLI for Atlassian (Jira, Agile, Goals) operations",
		// PersistentPreRunE validates env vars before any sub-command runs.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return validateEnv()
		},
	}

	// Build the client from env vars. We do this lazily in PersistentPreRunE
	// so validation happens before any command runs.
	cfg, err := configFromEnv()
	if err != nil {
		// This will be caught by PersistentPreRunE on first command execution.
		// We wire nil-safe services so cobra can at least show --help.
	}

	var (
		svc      jirasvc.Service
		agileSvc agilesvc.AgileService
		goalsSvc goalssvc.GoalsService
	)

	if err == nil {
		c, clientErr := atlclient.NewClient(cfg)
		if clientErr == nil {
			svc = jirasvc.NewService(c, cfg.BaseURL)
			agileSvc = agilesvc.NewService(c, cfg.BaseURL)
			goalsSvc = goalssvc.NewService(c, cfg.BaseURL)
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

	// Jira subgroup
	jiraRoot := jira.NewJiraCmd()
	jiraRoot.AddCommand(jira.NewGetCmd(svc))
	jiraRoot.AddCommand(jira.NewSearchCmd(svc))
	jiraRoot.AddCommand(jira.NewCreateCmd(svc))
	jiraRoot.AddCommand(jira.NewUpdateCmd(svc))
	jiraRoot.AddCommand(jira.NewTransitionsCmd(svc))
	jiraRoot.AddCommand(jira.NewTransitionCmd(svc))
	root.AddCommand(jiraRoot)

	// Agile subgroup
	agileRoot := atlcliagile.NewAgileCmd()
	atlcliagile.RegisterCommands(agileRoot, agileSvc)
	root.AddCommand(agileRoot)

	// Goals subgroup
	goalsRoot := atlcligoals.NewGoalsCmd()
	atlcligoals.RegisterCommands(goalsRoot, goalsSvc)
	root.AddCommand(goalsRoot)

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
