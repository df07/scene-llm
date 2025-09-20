package server

import (
	"fmt"
	"log"
	"net/http"
)

// Server handles web requests for the scene LLM
type Server struct {
	port int
}

// NewServer creates a new web server
func NewServer(port int) *Server {
	return &Server{port: port}
}

// Start starts the web server
func (s *Server) Start() error {
	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("static/")))

	// API endpoints
	http.HandleFunc("/api/health", s.handleHealth)

	// Start server
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok", "service": "scene-llm", "auto_reload": "working!"}`))
}
