package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/logan-hendricks-pwc/loan-processor/internal/loan"
)

// Server holds the dependencies shared by the HTTP handlers.
// Add stores, clients, and config here as the app grows.
type Server struct {
	router *mux.Router
	engine loan.DecisionEngine
}

func New(engine loan.DecisionEngine) *Server {
	s := &Server{router: mux.NewRouter(), engine: engine}
	s.routes()
	return s
}

func (s *Server) Router() *mux.Router {
	return s.router
}

// routes is the single place every route is registered.
func (s *Server) routes() {
	// recoverPanic must wrap logging (registered first = outermost) so a
	// panic recovered here still lets the logging middleware's deferred
	// access-log line fire.
	s.router.Use(recoverPanic)
	s.router.Use(logging)

	s.router.HandleFunc("/healthz", s.handleHealth()).Methods(http.MethodGet)

	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/loan-applications", s.handleCreateLoanApplication()).Methods(http.MethodPost)
}
