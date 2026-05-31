package api

import (
	"fmt"
	"net/http"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/agile"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	jira "github.com/jinkp/atlassian-go-mcp/internal/atlassian/jira"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/projects"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/releases"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/teams"
)

// Server holds all service dependencies and configuration for the REST API server.
type Server struct {
	jiraSvc     jira.Service
	agileSvc    agile.AgileService
	goalsSvc    goals.GoalsService
	releasesSvc releases.ReleasesService
	projectsSvc projects.ProjectsService
	teamsSvc    teams.TeamsService
	auditLog    audit.Logger
	readOnly    bool
	port        int
}

// NewServer constructs a Server with the provided services and configuration.
// readOnly=true disables all write operations regardless of the X-Enable-Write header.
// port is the TCP port to listen on (default 8080).
func NewServer(
	jiraSvc jira.Service,
	agileSvc agile.AgileService,
	goalsSvc goals.GoalsService,
	releasesSvc releases.ReleasesService,
	projectsSvc projects.ProjectsService,
	teamsSvc teams.TeamsService,
	auditLog audit.Logger,
	readOnly bool,
	port int,
) *Server {
	return &Server{
		jiraSvc:     jiraSvc,
		agileSvc:    agileSvc,
		goalsSvc:    goalsSvc,
		releasesSvc: releasesSvc,
		projectsSvc: projectsSvc,
		teamsSvc:    teamsSvc,
		auditLog:    auditLog,
		readOnly:    readOnly,
		port:        port,
	}
}

// JiraSvc returns the Jira service — used by cmd/api to wire handlers.
func (s *Server) JiraSvc() jira.Service { return s.jiraSvc }

// AgileSvc returns the Agile service — used by cmd/api to wire handlers.
func (s *Server) AgileSvc() agile.AgileService { return s.agileSvc }

// GoalsSvc returns the Goals service — used by cmd/api to wire handlers.
func (s *Server) GoalsSvc() goals.GoalsService { return s.goalsSvc }

// ReleasesSvc returns the Releases service — used by cmd/api to wire handlers.
func (s *Server) ReleasesSvc() releases.ReleasesService { return s.releasesSvc }

// ProjectsSvc returns the Projects service — used by cmd/api to wire handlers.
func (s *Server) ProjectsSvc() projects.ProjectsService { return s.projectsSvc }

// TeamsSvc returns the Teams service — used by cmd/api to wire handlers.
func (s *Server) TeamsSvc() teams.TeamsService { return s.teamsSvc }

// AuditLog returns the audit logger — used by cmd/api to wire handlers.
func (s *Server) AuditLog() audit.Logger { return s.auditLog }

// Start begins serving HTTP requests on the configured port.
// It blocks until the server is stopped or an error occurs.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	registerRoutes(mux, s)

	// Build middleware chain: Recover → Logging → WriteGuard → mux
	handler := RecoverMiddleware(
		LoggingMiddleware(
			WriteGuardMiddleware(s.readOnly, mux),
		),
	)

	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, handler)
}
