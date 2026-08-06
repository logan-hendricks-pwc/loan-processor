package server

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Server holds the dependencies shared by the HTTP handlers.
// Add stores, clients, and config here as the app grows.
type Server struct {
	router *mux.Router
}

func New() *Server {
	s := &Server{router: mux.NewRouter()}
	s.routes()
	return s
}

func (s *Server) Router() *mux.Router {
	return s.router
}

// routes is the single place every route is registered.
func (s *Server) routes() {
	s.router.Use(logging)

	s.router.HandleFunc("/healthz", s.handleHealth()).Methods(http.MethodGet)

	api := s.router.PathPrefix("/api/v1").Subrouter()
	_ = api // register API routes here, e.g. api.HandleFunc("/loans", s.handleListLoans()).Methods(http.MethodGet)
}
