package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/df07/scene-llm/agent/llm"
	"github.com/df07/scene-llm/agent/llm/claude"
	"github.com/df07/scene-llm/agent/llm/gemini"
)

// Server handles web requests for the scene LLM
type Server struct {
	port        int
	registry    *llm.Registry
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

// noCacheMiddleware adds no-cache headers to prevent browser caching during development
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// initializeProviders initializes the LLM provider registry from environment variables
func (s *Server) initializeProviders() error {
	ctx := context.Background()
	s.registry = llm.NewRegistry()

	// Try to add Gemini provider
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
		provider, err := gemini.NewProvider(ctx, apiKey)
		if err != nil {
			log.Printf("Warning: Failed to initialize Gemini provider: %v", err)
		} else {
			s.registry.Add(provider)
			log.Printf("Initialized Gemini provider")
		}
	}

	// Try to add Claude provider
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		provider, err := claude.NewProvider()
		if err != nil {
			log.Printf("Warning: Failed to initialize Claude provider: %v", err)
		} else {
			s.registry.Add(provider)
			log.Printf("Initialized Claude provider")
		}
	}

	// Validate at least one provider is available
	if len(s.registry.ListModels()) == 0 {
		return fmt.Errorf("no LLM providers available - set GOOGLE_API_KEY or ANTHROPIC_API_KEY environment variable")
	}

	log.Printf("Available models: %v", s.registry.ListModels())
	return nil
}

// Start starts the web server
func (s *Server) Start() error {
	// Initialize provider registry
	if err := s.initializeProviders(); err != nil {
		return err
	}

	// Serve static files with no-cache headers for development
	fs := http.FileServer(http.Dir("static/"))
	http.Handle("/", noCacheMiddleware(fs))

	// API endpoints
	http.HandleFunc("/api/health", s.handleHealth)
	http.HandleFunc("/api/models", s.handleModels)
	http.HandleFunc("/api/chat", s.handleChat)
	http.HandleFunc("/api/chat/stream", s.handleChatStream)
	http.HandleFunc("/api/chat/interrupt", s.handleInterrupt)
	http.HandleFunc("/api/render", s.handleRender)

	// Start server
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok", "service": "scene-llm"}`))
}

// handleModels returns the list of available models
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models := s.registry.ListModels()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Simple JSON array response
	response := "["
	for i, model := range models {
		if i > 0 {
			response += ","
		}
		response += fmt.Sprintf(`"%s"`, model)
	}
	response += "]"

	w.Write([]byte(response))
}
