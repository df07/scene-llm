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
	"sync"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
	"google.golang.org/genai"
)

// Server handles web requests for the scene LLM
type Server struct {
	port        int
	sessions    map[string]*ChatSession
	sseClients  map[string]map[chan SSEChatEvent]bool // sessionID -> clients
	mutex       sync.RWMutex
	clientMutex sync.RWMutex
}

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
	http.HandleFunc("/api/test-render", s.handleTestRender)
	http.HandleFunc("/api/test-llm", s.handleTestLLM)
	http.HandleFunc("/api/generate-scene", s.handleGenerateScene)
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

// handleTestRender creates a simple test scene and renders it
func (s *Server) handleTestRender(w http.ResponseWriter, r *http.Request) {
	// Create a simple test scene with a sphere
	testScene := createTestScene()

	// Render the scene
	config := renderer.DefaultProgressiveConfig()
	config.MaxSamplesPerPixel = 10 // Keep it fast for testing
	config.MaxPasses = 3

	logger := renderer.NewDefaultLogger()
	integrator := integrator.NewPathTracingIntegrator(testScene.SamplingConfig)

	raytracer, err := renderer.NewProgressiveRaytracer(testScene, config, integrator, logger)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create raytracer: %v", err), http.StatusInternalServerError)
		return
	}

	// Render a single pass
	result, _, err := raytracer.RenderPass(1, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to render: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the image as PNG
	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, result); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode image: %v", err), http.StatusInternalServerError)
		return
	}
}

// createTestScene creates a simple scene with a sphere, light, and camera
func createTestScene() *scene.Scene {
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

	// Camera
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

	// Create a red sphere
	redMaterial := material.NewLambertian(core.NewVec3(0.8, 0.2, 0.2))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, 0), 1.0, redMaterial)

	// Create scene
	testScene := &scene.Scene{
		Camera:         camera,
		Shapes:         []geometry.Shape{sphere},
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Add a simple light
	testScene.AddUniformInfiniteLight(core.NewVec3(0.5, 0.7, 1.0))

	return testScene
}

// handleTestLLM tests the LLM integration with a simple query
func (s *Server) handleTestLLM(w http.ResponseWriter, r *http.Request) {
	// Get API key from environment
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		http.Error(w, "GOOGLE_API_KEY environment variable not set", http.StatusInternalServerError)
		return
	}

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create Gemini client: %v", err), http.StatusInternalServerError)
		return
	}

	// Simple test query
	parts := []*genai.Part{
		{Text: "What is 2 + 2? Please respond with just the number and nothing else."},
	}

	result, err := client.Models.GenerateContent(ctx, "gemini-1.5-flash", []*genai.Content{{Parts: parts}}, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate content: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract response text
	var responseText string
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		if part := result.Candidates[0].Content.Parts[0]; part.Text != "" {
			responseText = part.Text
		}
	}

	// Return JSON response
	response := map[string]interface{}{
		"status":   "ok",
		"query":    "What is 2 + 2?",
		"response": responseText,
		"model":    "gemini-1.5-flash",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ShapeRequest represents the LLM's shape creation request
type ShapeRequest struct {
	Type     string     `json:"type"`
	Position [3]float64 `json:"position"`
	Size     float64    `json:"size"`
	Color    [3]float64 `json:"color"`
}

// SceneResponse represents the full response from the generate-scene endpoint
type SceneResponse struct {
	Status      string       `json:"status"`
	Prompt      string       `json:"prompt"`
	LLMResponse LLMDebugInfo `json:"llm_response"`
	Scene       *SceneInfo   `json:"scene,omitempty"`
	ImageBase64 string       `json:"image_base64,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// LLMDebugInfo contains debugging information about the LLM conversation
type LLMDebugInfo struct {
	Model          string      `json:"model"`
	FunctionCalled bool        `json:"function_called"`
	FunctionName   string      `json:"function_name,omitempty"`
	FunctionArgs   interface{} `json:"function_args,omitempty"`
	TextResponse   string      `json:"text_response,omitempty"`
	RawCandidates  interface{} `json:"raw_candidates,omitempty"`
}

// SceneInfo represents the current scene state
type SceneInfo struct {
	Shapes []ShapeRequest `json:"shapes"`
	Camera CameraInfo     `json:"camera"`
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

// handleGenerateScene takes a text prompt and generates a scene using LLM
func (s *Server) handleGenerateScene(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := &SceneResponse{
		Status: "error",
		LLMResponse: LLMDebugInfo{
			Model: "gemini-2.5-flash",
		},
	}

	if r.Method != http.MethodPost {
		response.Error = "Method not allowed"
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse request
	var request struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response.Error = "Invalid JSON"
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	response.Prompt = request.Prompt

	// Get API key
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		response.Error = "GOOGLE_API_KEY environment variable not set"
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		response.Error = fmt.Sprintf("Failed to create Gemini client: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
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

	// Create prompt
	prompt := fmt.Sprintf("Create a single 3D shape based on this description: %s. Use the create_shape function to specify the shape.", request.Prompt)
	parts := []*genai.Part{{Text: prompt}}

	// Generate content with function calling
	result, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", []*genai.Content{{Parts: parts}}, &genai.GenerateContentConfig{
		Tools: tools,
	})
	if err != nil {
		response.Error = fmt.Sprintf("Failed to generate content: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Store debug information about the LLM response
	response.LLMResponse.RawCandidates = result.Candidates

	// Parse function call response
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		response.Error = "No response from LLM"
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	var shapeRequest ShapeRequest
	functionFound := false

	for _, part := range result.Candidates[0].Content.Parts {
		// Check for function calls
		if part.FunctionCall != nil && part.FunctionCall.Name == "create_shape" {
			response.LLMResponse.FunctionCalled = true
			response.LLMResponse.FunctionName = part.FunctionCall.Name
			response.LLMResponse.FunctionArgs = part.FunctionCall.Args

			// Parse function call arguments
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
			functionFound = true
		}

		// Also capture any text response
		if part.Text != "" {
			response.LLMResponse.TextResponse = part.Text
		}
	}

	if !functionFound {
		response.Error = "LLM did not call create_shape function"
		response.Status = "no_function_call"
		w.WriteHeader(http.StatusOK) // Change to 200 so we can see the debug info
		json.NewEncoder(w).Encode(response)
		return
	}

	// Build scene with the LLM's shape plus defaults
	sceneWithShape := createSceneWithShape(shapeRequest)

	// Create scene info for response
	cameraPos := [3]float64{shapeRequest.Position[0], shapeRequest.Position[1], shapeRequest.Position[2] + 5}
	response.Scene = &SceneInfo{
		Shapes: []ShapeRequest{shapeRequest},
		Camera: CameraInfo{
			Position: cameraPos,
			LookAt:   shapeRequest.Position,
		},
	}

	// Render the scene
	config := renderer.DefaultProgressiveConfig()
	config.MaxSamplesPerPixel = 10
	config.MaxPasses = 3

	logger := renderer.NewDefaultLogger()
	integrator := integrator.NewPathTracingIntegrator(sceneWithShape.SamplingConfig)

	raytracer, err := renderer.NewProgressiveRaytracer(sceneWithShape, config, integrator, logger)
	if err != nil {
		response.Error = fmt.Sprintf("Failed to create raytracer: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Render
	result_img, _, err := raytracer.RenderPass(1, nil)
	if err != nil {
		response.Error = fmt.Sprintf("Failed to render: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Encode image to base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, result_img); err != nil {
		response.Error = fmt.Sprintf("Failed to encode image: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response.ImageBase64 = base64.StdEncoding.EncodeToString(buf.Bytes())
	response.Status = "success"

	// Return successful JSON response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// createSceneWithShape builds a complete scene with the LLM's shape plus defaults
func createSceneWithShape(shapeReq ShapeRequest) *scene.Scene {
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

	// Shape position for camera to look at
	shapePosition := core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2])

	// Camera position - back away from the shape along Z axis, accounting for shape size
	cameraDistance := shapeReq.Size*3 + 5 // At least 3x the shape size plus 5 units
	cameraPosition := core.NewVec3(shapePosition.X, shapePosition.Y, shapePosition.Z+cameraDistance)

	// Camera (look at the shape)
	cameraConfig := geometry.CameraConfig{
		Center:        cameraPosition,
		LookAt:        shapePosition,
		Up:            core.NewVec3(0, 1, 0),
		VFov:          45.0,
		Width:         samplingConfig.Width,
		AspectRatio:   float64(samplingConfig.Width) / float64(samplingConfig.Height),
		Aperture:      0.0,
		FocusDistance: 0.0,
	}
	camera := geometry.NewCamera(cameraConfig)

	// Use the color from the LLM request
	shapeColor := core.NewVec3(shapeReq.Color[0], shapeReq.Color[1], shapeReq.Color[2])
	shapeMaterial := material.NewLambertian(shapeColor)

	// Create shape based on LLM request
	var shape geometry.Shape
	position := core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2])

	switch shapeReq.Type {
	case "sphere":
		shape = geometry.NewSphere(position, shapeReq.Size, shapeMaterial)
	case "box":
		// Create a box with the size as the side length
		min := core.NewVec3(position.X-shapeReq.Size/2, position.Y-shapeReq.Size/2, position.Z-shapeReq.Size/2)
		max := core.NewVec3(position.X+shapeReq.Size/2, position.Y+shapeReq.Size/2, position.Z+shapeReq.Size/2)
		shape = geometry.NewBox(min, max, core.NewVec3(0, 0, 0), shapeMaterial)
	default:
		// Default to sphere
		shape = geometry.NewSphere(position, shapeReq.Size, shapeMaterial)
	}

	// Create scene
	sceneWithShape := &scene.Scene{
		Camera:         camera,
		Shapes:         []geometry.Shape{shape},
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Add default lighting
	sceneWithShape.AddUniformInfiniteLight(core.NewVec3(0.5, 0.7, 1.0))

	return sceneWithShape
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

	// Use the first shape to build a scene (for now, we'll enhance this later)
	firstShape := session.Scene.Shapes[0]
	sceneWithShape := createSceneWithShape(firstShape)

	// Render the scene
	config := renderer.DefaultProgressiveConfig()
	config.MaxSamplesPerPixel = 10
	config.MaxPasses = 3

	logger := renderer.NewDefaultLogger()
	integrator := integrator.NewPathTracingIntegrator(sceneWithShape.SamplingConfig)

	raytracer, err := renderer.NewProgressiveRaytracer(sceneWithShape, config, integrator, logger)
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
