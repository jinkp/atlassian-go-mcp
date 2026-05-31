package api

import (
	"net/http"
)

// RouteRegistrar is a function that registers routes on a mux for the given Server.
// It is provided by the caller (cmd/api) to avoid import cycles between api and api/handlers.
type RouteRegistrar func(mux *http.ServeMux, s *Server)

// registerRoutes is set at startup from cmd/api; it wires the handler sub-package routes.
// This indirection breaks the import cycle: api → api/handlers would be circular.
var registerRoutes RouteRegistrar = func(mux *http.ServeMux, s *Server) {
	// default no-op; overridden by RegisterRoutes() call from cmd/api/main.go
}

// RegisterRoutes sets the route registrar. Call this from cmd/api/main.go before Start().
func RegisterRoutes(fn RouteRegistrar) {
	registerRoutes = fn
}
