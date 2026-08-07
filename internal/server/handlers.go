package server

import (
	"encoding/json"
	"log"
	"net/http"
)

func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// respondJSON writes v as a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// respondError writes a JSON error body with the given status code.
func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// FieldError describes one invalid field in a request body.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// respondFieldErrors writes the 400 body shape used for validation
// failures: a summary plus every offending field, so a client can fix all
// of them in one round trip.
func respondFieldErrors(w http.ResponseWriter, summary string, fields []FieldError) {
	respondJSON(w, http.StatusBadRequest, map[string]any{
		"error":  summary,
		"fields": fields,
	})
}
