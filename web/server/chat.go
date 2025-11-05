package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
	"github.com/df07/scene-llm/agent"
	"google.golang.org/genai"
)

// ChatSession represents an ongoing conversation with persistent agent state
type ChatSession struct {
	ID       string             `json:"id"`
	Messages []*genai.Content   `json:"messages"`
	Agent    *agent.Agent       `json:"-"` // Agent with persistent SceneManager
	cancel   context.CancelFunc // Function to cancel ongoing processing
	mutex    sync.Mutex         // Protects cancel function
}

// ChatMessage represents a chat message request
type ChatMessage struct {
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message"`
	Quality   string `json:"quality,omitempty"` // Render quality: "draft" or "high"
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
		// Only close if the client is still in the map (not already removed)
		if _, exists := clients[client]; exists {
			delete(clients, client)
			if len(clients) == 0 {
				delete(s.sseClients, sessionID)
			}
			close(client)
		}
	}
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
	if session == nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

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

	// Parse quality setting (default to draft if not specified)
	quality := agent.QualityDraft
	if chatMsg.Quality == "high" {
		quality = agent.QualityHigh
	}

	// Process the message asynchronously (this will stream results via SSE)
	go s.processMessage(session, chatMsg.Message, quality)
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
		case event, ok := <-clientChan:
			if !ok {
				return // Channel closed
			}
			if err := s.sendSSEEvent(w, event.Type, event.Data); err != nil {
				return // Connection closed
			}
		case <-ticker.C:
			s.sendSSEEvent(w, "ping", map[string]string{"status": "alive"})
		}
	}
}

// processMessage processes a chat message and streams responses via SSE to all connected clients
func (s *Server) processMessage(session *ChatSession, message string, quality agent.RenderQuality) {
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

	// Create a cancellable context for this processing
	ctx, cancel := context.WithCancel(context.Background())

	// Store the cancel function in the session
	session.mutex.Lock()
	session.cancel = cancel
	session.mutex.Unlock()

	// Start agent processing in goroutine
	// Note: Agent maintains its scene state across messages
	go func() {
		defer func() {
			// Clear the cancel function when processing completes
			session.mutex.Lock()
			session.cancel = nil
			session.mutex.Unlock()
		}()

		if err := ag.ProcessMessage(ctx, session.Messages); err != nil {
			// Check if the error is due to cancellation
			if errors.Is(err, context.Canceled) {
				agentEvents <- agent.NewErrorEvent(fmt.Errorf("processing interrupted by user"))
			} else {
				agentEvents <- agent.NewErrorEvent(err)
			}
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
			// Broadcast the response (send whole event to include thought field)
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e})

		case agent.SceneRenderEvent:
			// Handle ready-to-render scene from agent (use quality from message)
			s.renderAndBroadcastScene(session.ID, e.RaytracerScene, quality)

		case agent.ToolCallStartEvent:
			// Handle tool call start events
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e})

		case agent.ToolCallEvent:
			// Handle tool call events with logging and broadcasting
			s.handleToolCallEvent(session.ID, e)

		case agent.ProcessingEvent:
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
func (s *Server) renderAndBroadcastScene(sessionID string, raytracerScene *scene.Scene, quality agent.RenderQuality) {
	if len(raytracerScene.Shapes) == 0 {
		return // No shapes to render
	}

	// Broadcast render start event
	s.broadcastToSession(sessionID, SSEChatEvent{
		Type: "render_start",
		Data: map[string]interface{}{
			"quality": string(quality),
		},
	})

	// Render the scene with appropriate config based on quality
	config := renderer.DefaultProgressiveConfig()
	config.MaxPasses = 1
	if quality == agent.QualityHigh {
		config.MaxSamplesPerPixel = 500
	} else {
		config.MaxSamplesPerPixel = 10
	}

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
		"quality":      string(quality),
	}

	// Broadcast scene update with image
	s.broadcastToSession(sessionID, SSEChatEvent{
		Type: "scene_update",
		Data: sceneInfo,
	})

	log.Printf("Scene rendered for session %s - %d shapes", sessionID, len(raytracerScene.Shapes))
}

// InterruptRequest represents a request to interrupt LLM processing
type InterruptRequest struct {
	SessionID string `json:"session_id"`
}

// handleInterrupt handles requests to interrupt ongoing LLM processing
func (s *Server) handleInterrupt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// Parse request
	var interruptReq InterruptRequest
	if err := json.NewDecoder(r.Body).Decode(&interruptReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if interruptReq.SessionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "session_id is required"})
		return
	}

	// Find session
	s.mutex.RLock()
	session, exists := s.sessions[interruptReq.SessionID]
	s.mutex.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Session not found"})
		return
	}

	// Cancel ongoing processing if any
	session.mutex.Lock()
	if session.cancel != nil {
		session.cancel()
		session.mutex.Unlock()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "interrupted"})
		return
	}
	session.mutex.Unlock()

	// No processing to cancel
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "not_processing"})
}

// RenderRequest represents a request to re-render the scene
type RenderRequest struct {
	SessionID string `json:"session_id"`
	Quality   string `json:"quality"`
}

// handleRender handles requests to re-render the current scene with different quality
func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// Parse request
	var renderReq RenderRequest
	if err := json.NewDecoder(r.Body).Decode(&renderReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Get session
	s.mutex.RLock()
	session, exists := s.sessions[renderReq.SessionID]
	s.mutex.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Session not found"})
		return
	}

	// Parse quality setting
	quality := agent.QualityDraft
	if renderReq.Quality == "high" {
		quality = agent.QualityHigh
	}

	// Get current scene from agent's scene manager
	raytracerScene, err := session.Agent.GetSceneManager().ToRaytracerScene()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate scene"})
		return
	}

	// Render and broadcast the scene
	go s.renderAndBroadcastScene(renderReq.SessionID, raytracerScene, quality)

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "rendering"})
}

// handleToolCallEvent processes tool call events with logging and client broadcast
func (s *Server) handleToolCallEvent(sessionID string, event agent.ToolCallEvent) {
	// Log to server with terse format as specified in our spec
	if event.Success {
		log.Printf("INFO  [session:%s] Tool call: %s (%s)",
			sessionID, event.Request.ToolName(), event.Request.Target())
	} else {
		log.Printf("INFO  [session:%s] Tool call: %s (%s)",
			sessionID, event.Request.ToolName(), event.Request.Target())
		log.Printf("ERROR [session:%s] Tool call FAILED", sessionID)
		log.Printf("      %s", event.Error)
	}

	// Broadcast the event to the client (the client will handle display formatting)
	s.broadcastToSession(sessionID, SSEChatEvent{
		Type: event.EventType(), // "function_calls"
		Data: event,             // Send the entire ToolCallEvent structure
	})
}
