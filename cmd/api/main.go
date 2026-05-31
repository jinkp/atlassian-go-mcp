// Command atlassian-api exposes all Atlassian MCP operations as a JSON REST API.
//
// Usage:
//
//	atlassian-api --port 8080           # Start the REST API server (default port)
//	atlassian-api --port 9000           # Start on a custom port
//	atlassian-api --read-only           # Start in read-only mode (blocks all writes)
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/api"
	"github.com/jinkp/atlassian-go-mcp/internal/api/handlers"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/client"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	readOnly := flag.Bool("read-only", false, "Disable all write operations regardless of X-Enable-Write header")
	flag.Parse()

	log.SetOutput(os.Stderr)

	// Read required environment variables.
	baseURL := os.Getenv("ATLASSIAN_BASE_URL")
	if baseURL == "" {
		log.Fatal("ATLASSIAN_BASE_URL is required but not set")
	}
	email := os.Getenv("ATLASSIAN_EMAIL")
	if email == "" {
		log.Fatal("ATLASSIAN_EMAIL is required but not set")
	}
	token := os.Getenv("ATLASSIAN_TOKEN")
	if token == "" {
		log.Fatal("ATLASSIAN_TOKEN is required but not set")
	}

	// Build the authenticated HTTP client.
	httpClient, err := client.NewClient(client.Config{
		BaseURL:  baseURL,
		Email:    email,
		APIToken: token,
	})
	if err != nil {
		log.Fatalf("building HTTP client: %v", err)
	}

	// Wire all services.
	jiraSvc := jira.NewService(httpClient, baseURL)
	agileSvc := agile.NewService(httpClient, baseURL)
	goalsSvc := goals.NewService(httpClient, baseURL)
	releasesSvc := releases.NewService(httpClient, baseURL)
	projectsSvc := projects.NewService(httpClient, baseURL)

	// Teams uses a different API base URL and requires ATLASSIAN_ORG_ID.
	// An empty orgID is allowed at startup — requests to /teams will fail at call time.
	orgID := os.Getenv("ATLASSIAN_ORG_ID")
	teamsSvc := teams.NewService(httpClient, orgID)

	auditLog := audit.NewJSONLogger(os.Stderr)

	// Register the route registrar — this is the only place that imports both api and api/handlers,
	// which avoids the import cycle between the two packages.
	api.RegisterRoutes(func(mux *http.ServeMux, s *api.Server) {
		health := handlers.NewHealthHandler()
		mux.Handle("GET /health", health)

		jiraH := handlers.NewJiraHandler(s.JiraSvc(), s.AuditLog())
		mux.HandleFunc("GET /jira/issues/{key}", jiraH.GetIssue)
		mux.HandleFunc("GET /jira/issues", jiraH.SearchIssues)
		mux.HandleFunc("POST /jira/issues", jiraH.CreateIssue)
		mux.HandleFunc("PUT /jira/issues/{key}", jiraH.UpdateIssue)
		mux.HandleFunc("GET /jira/issues/{key}/transitions", jiraH.GetTransitions)
		mux.HandleFunc("POST /jira/issues/{key}/transitions", jiraH.TransitionIssue)

		agileH := handlers.NewAgileHandler(s.AgileSvc(), s.AuditLog())
		mux.HandleFunc("GET /agile/boards", agileH.GetBoards)
		mux.HandleFunc("GET /agile/boards/{boardId}/sprints", agileH.GetSprints)
		mux.HandleFunc("GET /agile/boards/{boardId}/sprints/active", agileH.GetActiveSprint)
		mux.HandleFunc("GET /agile/sprints/{sprintId}/issues", agileH.GetSprintIssues)
		mux.HandleFunc("POST /agile/sprints", agileH.CreateSprint)
		mux.HandleFunc("PUT /agile/sprints/{sprintId}", agileH.UpdateSprint)
		mux.HandleFunc("POST /agile/sprints/{sprintId}/issues", agileH.MoveIssuesToSprint)

		goalsH := handlers.NewGoalsHandler(s.GoalsSvc(), s.AuditLog())
		mux.HandleFunc("GET /goals/site-id", goalsH.GetSiteID)
		mux.HandleFunc("GET /goals", goalsH.SearchGoals)
		mux.HandleFunc("GET /goals/{goalId}", goalsH.GetGoal)
		mux.HandleFunc("POST /goals", goalsH.CreateGoal)
		mux.HandleFunc("PUT /goals/{goalId}/status", goalsH.UpdateGoalStatus)
		mux.HandleFunc("PUT /goals/{goalId}", goalsH.EditGoal)

		releasesH := handlers.NewReleasesHandler(s.ReleasesSvc(), s.AuditLog())
		mux.HandleFunc("GET /releases", releasesH.GetReleases)
		mux.HandleFunc("GET /releases/{releaseId}", releasesH.GetRelease)
		mux.HandleFunc("GET /releases/{releaseId}/issues", releasesH.GetReleaseIssues)
		mux.HandleFunc("POST /releases", releasesH.CreateRelease)
		mux.HandleFunc("PUT /releases/{releaseId}", releasesH.UpdateRelease)

		projectsH := handlers.NewProjectsHandler(s.ProjectsSvc(), s.AuditLog())
		mux.HandleFunc("GET /projects", projectsH.SearchProjects)
		mux.HandleFunc("GET /projects/{key}", projectsH.GetProject)
		mux.HandleFunc("PUT /projects/{key}", projectsH.UpdateProject)

		teamsH := handlers.NewTeamsHandler(s.TeamsSvc())
		mux.HandleFunc("GET /teams", teamsH.GetTeams)
		mux.HandleFunc("GET /teams/{teamId}", teamsH.GetTeam)
		mux.HandleFunc("GET /teams/{teamId}/members", teamsH.GetTeamMembers)
	})

	srv := api.NewServer(jiraSvc, agileSvc, goalsSvc, releasesSvc, projectsSvc, teamsSvc, auditLog, *readOnly, *port)
	log.Printf("atlassian-api: listening on :%d (read-only=%v)", *port, *readOnly)
	log.Fatal(srv.Start())
}
