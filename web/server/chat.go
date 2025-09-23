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
	"os"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
	"google.golang.org/genai"
)

// ChatSession represents an ongoing conversation
type ChatSession struct {
	ID       string           `json:"id"`
	Messages []*genai.Content `json:"messages"`
	Scene    *SceneState      `json:"scene"`
}

// SceneState represents the current 3D scene
type SceneState struct {
	Shapes []ShapeRequest `json:"shapes"`
	Camera CameraInfo     `json:"camera"`
}

// ShapeRequest represents the LLM's shape creation request
type ShapeRequest struct {
	Type     string     `json:"type"`
	Position [3]float64 `json:"position"`
	Size     float64    `json:"size"`
	Color    [3]float64 `json:"color"`
}

// CameraInfo represents camera information
type CameraInfo struct {
	Position [3]float64 `json:"position"`
	LookAt   [3]float64 `json:"look_at"`
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
			Scene: &SceneState{
				Shapes: []ShapeRequest{},
				Camera: CameraInfo{
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
	// Send thinking indicator
	s.broadcastToSession(session.ID, SSEChatEvent{
		Type: "thinking",
		Data: "🤖 Processing your request...",
	})

	// Get API key
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Printf("GOOGLE_API_KEY not set for session %s", session.ID)
		s.broadcastToSession(session.ID, SSEChatEvent{
			Type: "error",
			Data: "API key not configured",
		})
		return
	}

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("Failed to create Gemini client for session %s: %v", session.ID, err)
		s.broadcastToSession(session.ID, SSEChatEvent{
			Type: "error",
			Data: fmt.Sprintf("Failed to create LLM client: %v", err),
		})
		return
	}

	// Define the create_shape function for the LLM
	createShapeFunc := &genai.FunctionDeclaration{
		Name:        "create_shape",
		Description: "Create a 3D shape in the scene",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"type": {
					Type:        genai.TypeString,
					Enum:        []string{"sphere", "box"},
					Description: "The type of shape to create",
				},
				"position": {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeNumber},
					Description: "The position of the shape as [x, y, z]",
				},
				"size": {
					Type:        genai.TypeNumber,
					Description: "The size of the shape (radius for sphere, side length for cube)",
				},
				"color": {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeNumber},
					Description: "RGB color values as [r, g, b] where each value is between 0.0 and 1.0",
				},
			},
			Required: []string{"type", "position", "size", "color"},
		},
	}

	// Create tools
	tools := []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{createShapeFunc}}}

	// Build conversation context including current scene state
	s.mutex.RLock()
	messages := make([]*genai.Content, len(session.Messages))
	copy(messages, session.Messages)
	currentScene := session.Scene
	s.mutex.RUnlock()

	// Add system context about current scene
	sceneContext := "Current scene state: "
	if len(currentScene.Shapes) == 0 {
		sceneContext += "empty scene with no objects."
	} else {
		sceneContext += fmt.Sprintf("%d shapes: ", len(currentScene.Shapes))
		for i, shape := range currentScene.Shapes {
			sceneContext += fmt.Sprintf("%d) %s at [%.1f,%.1f,%.1f] size %.1f color [%.1f,%.1f,%.1f]",
				i+1, shape.Type, shape.Position[0], shape.Position[1], shape.Position[2],
				shape.Size, shape.Color[0], shape.Color[1], shape.Color[2])
			if i < len(currentScene.Shapes)-1 {
				sceneContext += ", "
			}
		}
	}

	// Add scene context to the first user message instead of using a system role
	contextualizedMessages := make([]*genai.Content, len(messages))
	copy(contextualizedMessages, messages)

	// Prepend context to the latest user message
	if len(contextualizedMessages) > 0 {
		lastMessage := contextualizedMessages[len(contextualizedMessages)-1]
		if lastMessage.Role == "user" && len(lastMessage.Parts) > 0 {
			// Prepend context to the user's message
			originalText := lastMessage.Parts[0].Text
			contextualText := fmt.Sprintf("Context: You are a 3D scene assistant. %s\n\nUser request: %s", sceneContext, originalText)
			lastMessage.Parts[0].Text = contextualText
		}
	}

	fullConversation := contextualizedMessages

	// Generate content with function calling
	result, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", fullConversation, &genai.GenerateContentConfig{
		Tools: tools,
	})
	if err != nil {
		log.Printf("Failed to generate content for session %s: %v", session.ID, err)
		s.broadcastToSession(session.ID, SSEChatEvent{
			Type: "error",
			Data: fmt.Sprintf("LLM generation failed: %v", err),
		})
		return
	}

	// Process the response
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		log.Printf("No response from LLM for session %s", session.ID)
		s.broadcastToSession(session.ID, SSEChatEvent{
			Type: "error",
			Data: "No response from LLM",
		})
		return
	}

	var assistantMessage *genai.Content
	var functionCalls []ShapeRequest
	hasTextResponse := false

	for _, part := range result.Candidates[0].Content.Parts {
		// Handle function calls
		if part.FunctionCall != nil && part.FunctionCall.Name == "create_shape" {
			var shapeRequest ShapeRequest
			args := part.FunctionCall.Args

			if typeVal, ok := args["type"].(string); ok {
				shapeRequest.Type = typeVal
			}
			if posVal, ok := args["position"].([]interface{}); ok && len(posVal) == 3 {
				for i, v := range posVal {
					if f, ok := v.(float64); ok {
						shapeRequest.Position[i] = f
					}
				}
			}
			if sizeVal, ok := args["size"].(float64); ok {
				shapeRequest.Size = sizeVal
			}
			if colorVal, ok := args["color"].([]interface{}); ok && len(colorVal) == 3 {
				for i, v := range colorVal {
					if f, ok := v.(float64); ok {
						shapeRequest.Color[i] = f
					}
				}
			}

			functionCalls = append(functionCalls, shapeRequest)
		}

		// Handle text response
		if part.Text != "" {
			if assistantMessage == nil {
				assistantMessage = &genai.Content{
					Parts: []*genai.Part{},
					Role:  "model",
				}
			}
			assistantMessage.Parts = append(assistantMessage.Parts, &genai.Part{Text: part.Text})
			hasTextResponse = true
		}
	}

	// Broadcast LLM text response
	if hasTextResponse && assistantMessage != nil {
		s.broadcastToSession(session.ID, SSEChatEvent{
			Type: "llm_response",
			Data: assistantMessage.Parts[0].Text,
		})

		// Add to conversation history
		s.mutex.Lock()
		session.Messages = append(session.Messages, assistantMessage)
		s.mutex.Unlock()
	}

	// Update scene with function calls
	if len(functionCalls) > 0 {
		s.mutex.Lock()
		session.Scene.Shapes = append(session.Scene.Shapes, functionCalls...)
		// Update camera to look at the first shape with proper distance
		if len(functionCalls) > 0 {
			firstShape := functionCalls[0]
			cameraDistance := firstShape.Size*3 + 5
			session.Scene.Camera.Position = [3]float64{firstShape.Position[0], firstShape.Position[1], firstShape.Position[2] + cameraDistance}
			session.Scene.Camera.LookAt = firstShape.Position
		}
		s.mutex.Unlock()

		// Broadcast function calls
		s.broadcastToSession(session.ID, SSEChatEvent{
			Type: "function_calls",
			Data: functionCalls,
		})

		// Render updated scene
		s.renderAndBroadcastScene(session)
	}

	// Send completion signal
	s.broadcastToSession(session.ID, SSEChatEvent{
		Type: "complete",
		Data: "Processing finished",
	})
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
func (s *Server) createSceneWithShapes(shapes []ShapeRequest) *scene.Scene {
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
