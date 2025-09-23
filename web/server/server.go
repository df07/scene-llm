package server

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

// Server handles web requests for the scene LLM
type Server struct {
	port        int
	sessions    map[string]*ChatSession
	sseClients  map[string]map[chan SSEChatEvent]bool // sessionID -> clients
	mutex       sync.RWMutex
	clientMutex sync.RWMutex
}

// NewServer creates a new web server
func NewServer(port int) *Server {
	return &Server{
		port:       port,
		sessions:   make(map[string]*ChatSession),
		sseClients: make(map[string]map[chan SSEChatEvent]bool),
	}
}

// Start starts the web server
func (s *Server) Start() error {
	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("static/")))

	// API endpoints
	http.HandleFunc("/api/health", s.handleHealth)
	http.HandleFunc("/api/chat", s.handleChat)
	http.HandleFunc("/api/chat/stream", s.handleChatStream)

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
