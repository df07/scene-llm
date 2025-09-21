package server

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"os"

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
	http.HandleFunc("/api/test-render", s.handleTestRender)
	http.HandleFunc("/api/test-llm", s.handleTestLLM)

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
