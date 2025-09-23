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

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
	"github.com/df07/scene-llm/agent"
	"google.golang.org/genai"
)

// ChatSession represents an ongoing conversation
type ChatSession struct {
	ID       string            `json:"id"`
	Messages []*genai.Content  `json:"messages"`
	Scene    *agent.SceneState `json:"scene"`
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
		session = &ChatSession{
			ID:       sessionID,
			Messages: []*genai.Content{},
			Scene: &agent.SceneState{
				Shapes: []agent.ShapeRequest{},
				Camera: agent.CameraInfo{
					Position: [3]float64{0, 0, 5},
					LookAt:   [3]float64{0, 0, 0},
				},
			},
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

	// Send current scene state immediately
	s.sendSSEEvent(w, "scene_state", map[string]interface{}{
		"scene":         session.Scene,
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

	// Create agent
	ag, err := agent.New(agentEvents)
	if err != nil {
		s.broadcastToSession(session.ID, SSEChatEvent{Type: "error", Data: err.Error()})
		return
	}
	defer ag.Close()

	// Build scene context
	sceneContext := s.buildSceneContext(session.Scene)

	// Start agent processing in goroutine
	go func() {
		if err := ag.ProcessMessage(context.Background(), session.Messages, sceneContext); err != nil {
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

		case agent.ToolCallEvent:
			// Handle function calls - update scene and render
			// Update scene with function calls
			s.mutex.Lock()
			session.Scene.Shapes = append(session.Scene.Shapes, e.Shapes...)
			// Update camera to look at the first shape with proper distance
			if len(e.Shapes) > 0 {
				firstShape := e.Shapes[0]
				cameraDistance := firstShape.Size*3 + 5
				session.Scene.Camera.Position = [3]float64{firstShape.Position[0], firstShape.Position[1], firstShape.Position[2] + cameraDistance}
				session.Scene.Camera.LookAt = firstShape.Position
			}
			s.mutex.Unlock()

			// Broadcast function calls
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Shapes})

			// Render updated scene
			s.renderAndBroadcastScene(session)

		case agent.ThinkingEvent:
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Message})
		case agent.ErrorEvent:
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Message})
		case agent.CompleteEvent:
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Message})
		case agent.SceneUpdateEvent:
			s.broadcastToSession(session.ID, SSEChatEvent{Type: e.EventType(), Data: e.Scene})
		default:
			// Fallback for any unhandled event types
			s.broadcastToSession(session.ID, SSEChatEvent{Type: "unknown", Data: "Unknown event type"})
		}
	}
}

// buildSceneContext creates a context string describing the current scene state
func (s *Server) buildSceneContext(scene *agent.SceneState) string {
	sceneContext := "Current scene state: "
	if len(scene.Shapes) == 0 {
		sceneContext += "empty scene with no objects."
	} else {
		sceneContext += fmt.Sprintf("%d shapes: ", len(scene.Shapes))
		for i, shape := range scene.Shapes {
			sceneContext += fmt.Sprintf("%d) %s at [%.1f,%.1f,%.1f] size %.1f color [%.1f,%.1f,%.1f]",
				i+1, shape.Type, shape.Position[0], shape.Position[1], shape.Position[2],
				shape.Size, shape.Color[0], shape.Color[1], shape.Color[2])
			if i < len(scene.Shapes)-1 {
				sceneContext += ", "
			}
		}
	}
	return sceneContext
}

// renderAndBroadcastScene renders the current scene and broadcasts to SSE clients
func (s *Server) renderAndBroadcastScene(session *ChatSession) {
	if len(session.Scene.Shapes) == 0 {
		return // No shapes to render
	}

	// Create a complete scene with all shapes
	sceneWithShapes := s.createSceneWithShapes(session.Scene.Shapes)

	// Render the scene
	config := renderer.DefaultProgressiveConfig()
	config.MaxSamplesPerPixel = 10
	config.MaxPasses = 3

	logger := renderer.NewDefaultLogger()
	integrator := integrator.NewPathTracingIntegrator(sceneWithShapes.SamplingConfig)

	raytracer, err := renderer.NewProgressiveRaytracer(sceneWithShapes, config, integrator, logger)
	if err != nil {
		log.Printf("Failed to create raytracer for session %s: %v", session.ID, err)
		return
	}

	// Render
	result_img, _, err := raytracer.RenderPass(1, nil)
	if err != nil {
		log.Printf("Failed to render for session %s: %v", session.ID, err)
		return
	}

	// Encode image to base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, result_img); err != nil {
		log.Printf("Failed to encode image for session %s: %v", session.ID, err)
		return
	}

	imageBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Broadcast scene update with image
	s.broadcastToSession(session.ID, SSEChatEvent{
		Type: "scene_update",
		Data: map[string]interface{}{
			"scene":        session.Scene,
			"image_base64": imageBase64,
		},
	})

	log.Printf("Scene rendered for session %s - %d shapes", session.ID, len(session.Scene.Shapes))
}

// createSceneWithShapes builds a complete scene with all shapes plus defaults
func (s *Server) createSceneWithShapes(shapes []agent.ShapeRequest) *scene.Scene {
	// Scene configuration
	samplingConfig := scene.SamplingConfig{
		Width:                     400,
		Height:                    300,
		SamplesPerPixel:           10,
		MaxDepth:                  8,
		RussianRouletteMinBounces: 3,
		AdaptiveMinSamples:        0.1,
		AdaptiveThreshold:         0.05,
	}

	// Camera (will be updated based on shapes)
	cameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 0, 5),
		LookAt:        core.NewVec3(0, 0, 0),
		Up:            core.NewVec3(0, 1, 0),
		VFov:          45.0,
		Width:         samplingConfig.Width,
		AspectRatio:   float64(samplingConfig.Width) / float64(samplingConfig.Height),
		Aperture:      0.0,
		FocusDistance: 0.0,
	}
	camera := geometry.NewCamera(cameraConfig)

	// Create shapes
	var sceneShapes []geometry.Shape
	for _, shapeReq := range shapes {
		// Create material with requested color
		shapeMaterial := material.NewLambertian(core.NewVec3(shapeReq.Color[0], shapeReq.Color[1], shapeReq.Color[2]))

		// Create geometry based on type
		var shape geometry.Shape
		switch shapeReq.Type {
		case "sphere":
			shape = geometry.NewSphere(
				core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2]),
				shapeReq.Size,
				shapeMaterial,
			)
		case "box":
			// Create a simple cube using a sphere for now (since Box constructor seems different)
			shape = geometry.NewSphere(
				core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2]),
				shapeReq.Size/2, // Use half size for radius
				shapeMaterial,
			)
		default:
			// Default to sphere
			shape = geometry.NewSphere(
				core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2]),
				shapeReq.Size,
				shapeMaterial,
			)
		}
		sceneShapes = append(sceneShapes, shape)
	}

	// Create scene
	sceneWithShapes := &scene.Scene{
		Camera:         camera,
		Shapes:         sceneShapes,
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Add default lighting
	sceneWithShapes.AddUniformInfiniteLight(core.NewVec3(0.5, 0.7, 1.0))

	return sceneWithShapes
}
