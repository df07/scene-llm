package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
	"github.com/df07/scene-llm/agent"
	"google.golang.org/genai"
)

// ChatSession represents an ongoing conversation with persistent agent state
type ChatSession struct {
	ID       string           `json:"id"`
	Messages []*genai.Content `json:"messages"`
	Agent    *agent.Agent     `json:"-"` // Agent with persistent SceneManager
}

// ChatMessage represents a chat message request
type ChatMessage struct {
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message"`
}

// ChatResponse represents the immediate response to a chat message
type ChatResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// SSEChatEvent represents events sent via SSE
type SSEChatEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// generateSessionID creates a new random session ID
func generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

// getOrCreateSession gets an existing session or creates a new one
func (s *Server) getOrCreateSession(sessionID string) *ChatSession {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if sessionID == "" {
		sessionID = generateSessionID()
	}

	session, exists := s.sessions[sessionID]
	if !exists {
		// Create agent for this session (this will create a persistent SceneManager)
		ag, err := agent.New(nil) // We'll set the events channel later per message
		if err != nil {
			log.Printf("Failed to create agent for session %s: %v", sessionID, err)
			return nil
		}

		session = &ChatSession{
			ID:       sessionID,
			Messages: []*genai.Content{},
			Agent:    ag,
		}
		s.sessions[sessionID] = session
	}

	return session
}

// setSSEHeaders sets the required headers for Server-Sent Events
func (s *Server) setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")
}

// sendSSEEvent sends an SSE event to the client
func (s *Server) sendSSEEvent(w http.ResponseWriter, eventType string, data interface{}) error {
	event := SSEChatEvent{
		Type: eventType,
		Data: data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", jsonData)
	if err != nil {
		return err
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

// addSSEClient adds a new SSE client for a session
func (s *Server) addSSEClient(sessionID string, client chan SSEChatEvent) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.sseClients[sessionID] == nil {
		s.sseClients[sessionID] = make(map[chan SSEChatEvent]bool)
	}
	s.sseClients[sessionID][client] = true
}

// removeSSEClient removes an SSE client
func (s *Server) removeSSEClient(sessionID string, client chan SSEChatEvent) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if clients := s.sseClients[sessionID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(s.sseClients, sessionID)
		}
	}
	close(client)
}

// broadcastToSession sends an SSE event to all clients of a session
func (s *Server) broadcastToSession(sessionID string, event SSEChatEvent) {
	s.clientMutex.RLock()
	clients := s.sseClients[sessionID]
	s.clientMutex.RUnlock()

	if clients == nil {
		return
	}

	for client := range clients {
		select {
		case client <- event:
		default:
			// Client channel is full, remove it
			go s.removeSSEClient(sessionID, client)
		}
	}
}

// handleChat handles incoming chat messages
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		response := ChatResponse{Status: "error", Error: "Method not allowed"}
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse request
	var chatMsg ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&chatMsg); err != nil {
		response := ChatResponse{Status: "error", Error: "Invalid JSON"}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if chatMsg.Message == "" {
		response := ChatResponse{Status: "error", Error: "Message cannot be empty"}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get or create session
	session := s.getOrCreateSession(chatMsg.SessionID)

	// Add user message to conversation history
	s.mutex.Lock()
	userMessage := &genai.Content{
		Parts: []*genai.Part{{Text: chatMsg.Message}},
		Role:  "user",
	}
	session.Messages = append(session.Messages, userMessage)
	s.mutex.Unlock()

	// Return immediate acknowledgment with session ID
	response := ChatResponse{
		SessionID: session.ID,
		Status:    "processing",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	// Process the message asynchronously (this will stream results via SSE)
	go s.processMessage(session, chatMsg.Message)
}

// handleChatStream handles SSE connections for real-time chat updates
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	s.setSSEHeaders(w)

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		s.sendSSEEvent(w, "error", "Session ID required")
		return
	}

	// Get session
	s.mutex.RLock()
	session, exists := s.sessions[sessionID]
	s.mutex.RUnlock()

	if !exists {
		s.sendSSEEvent(w, "error", "Session not found")
		return
	}

	// Create client channel and add to session
	clientChan := make(chan SSEChatEvent, 10)
	s.addSSEClient(sessionID, clientChan)
	defer s.removeSSEClient(sessionID, clientChan)

	// Send initial connection state
	s.sendSSEEvent(w, "connection_state", map[string]interface{}{
		"message_count": len(session.Messages),
	})

	// Listen for events and connection close
	ctx := r.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-clientChan:
			if err := s.sendSSEEvent(w, event.Type, event.Data); err != nil {
				return // Connection closed
			}
		case <-ticker.C:
			s.sendSSEEvent(w, "ping", map[string]string{"status": "alive"})
		}
	}
}

// processMessage processes a chat message and streams responses via SSE to all connected clients
func (s *Server) processMessage(session *ChatSession, message string) {
	// Create channel for agent events
	agentEvents := make(chan agent.AgentEvent, 10)

	// Use the persistent agent from the session
	ag := session.Agent
	if ag == nil {
		s.broadcastToSession(session.ID, SSEChatEvent{Type: "error", Data: "Session agent not initialized"})
		return
	}

	// Set the events channel for this message processing
	ag.SetEventsChannel(agentEvents)

	// Start agent processing in goroutine
	// Note: Agent maintains its scene state across messages
	go func() {
		if err := ag.ProcessMessage(context.Background(), session.Messages); err != nil {
			agentEvents <- agent.NewErrorEvent(err)
		}
		close(agentEvents)
	}()

	// Listen for events and handle them
	for event := range agentEvents {
		switch e := event.(type) {
		case agent.ResponseEvent:
			// Handle text response - add to conversation history
			// Add to conversation history
			s.mutex.Lock()
			assistantMessage := &genai.Content{
				Parts: []*genai.Part{{Text: e.Text}},
				Role:  "model",
			}
			session.Messages = append(session.Messages, assistantMessage)
			s.mutex.Unlock()
			// Broadcast the response
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Text})

		case agent.SceneRenderEvent:
			// Handle ready-to-render scene from agent
			s.renderAndBroadcastScene(session.ID, e.RaytracerScene)

		case agent.ThinkingEvent:
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Message})
		case agent.ErrorEvent:
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Message})
		case agent.CompleteEvent:
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Message})
		default:
			// Fallback for any unhandled event types
			s.broadcastToSession(session.ID, SSEChatEvent{Type: "unknown", Data: "Unknown event type"})
		}
	}
}

// renderAndBroadcastScene renders a raytracer scene and broadcasts to a specific session
func (s *Server) renderAndBroadcastScene(sessionID string, raytracerScene *scene.Scene) {
	if len(raytracerScene.Shapes) == 0 {
		return // No shapes to render
	}

	// Render the scene
	config := renderer.DefaultProgressiveConfig()
	config.MaxSamplesPerPixel = 10
	config.MaxPasses = 3

	logger := renderer.NewDefaultLogger()
	integrator := integrator.NewPathTracingIntegrator(raytracerScene.SamplingConfig)

	raytracer, err := renderer.NewProgressiveRaytracer(raytracerScene, config, integrator, logger)
	if err != nil {
		log.Printf("Failed to create raytracer for session %s: %v", sessionID, err)
		return
	}

	// Render
	result_img, _, err := raytracer.RenderPass(1, nil)
	if err != nil {
		log.Printf("Failed to render for session %s: %v", sessionID, err)
		return
	}

	// Encode image to base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, result_img); err != nil {
		log.Printf("Failed to encode image for session %s: %v", sessionID, err)
		return
	}

	imageBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Extract basic scene info for frontend (simplified representation)
	sceneInfo := map[string]interface{}{
		"shape_count":  len(raytracerScene.Shapes),
		"image_base64": imageBase64,
	}

	// Broadcast scene update with image
	s.broadcastToSession(sessionID, SSEChatEvent{
		Type: "scene_update",
		Data: sceneInfo,
	})

	log.Printf("Scene rendered for session %s - %d shapes", sessionID, len(raytracerScene.Shapes))
}
