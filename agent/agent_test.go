package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/df07/scene-llm/agent/llm"
	"github.com/df07/scene-llm/agent/llm/gemini"
	"google.golang.org/genai"
)

// MockProvider implements llm.LLMProvider for testing
type MockProvider struct {
	Responses []*genai.GenerateContentResponse
	CallCount int
}

func (m *MockProvider) GenerateContent(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	if m.CallCount >= len(m.Responses) {
		// Return empty response when we run out
		return &llm.Response{
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Done"}},
		}, nil
	}

	response := m.Responses[m.CallCount]
	m.CallCount++

	// Convert genai response to internal format
	return gemini.ToInternalResponse(response)
}

func (m *MockProvider) ListModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "mock-model", DisplayName: "Mock Model", Provider: "mock"}}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) SupportsVision() bool {
	return false
}

func (m *MockProvider) SupportsThinking() bool {
	return false
}

// NewMockResponse creates a mock response with text and optional function calls
func NewMockResponse(text string, functionCalls ...*genai.FunctionCall) *genai.GenerateContentResponse {
	parts := []*genai.Part{}

	if text != "" {
		parts = append(parts, &genai.Part{Text: text})
	}

	for i, fc := range functionCalls {
		// Add ID if not present
		if fc.ID == "" {
			fc.ID = fmt.Sprintf("call_%d", i)
		}
		parts = append(parts, &genai.Part{FunctionCall: fc})
	}

	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role:  "model",
					Parts: parts,
				},
			},
		},
	}
}

// TestAgenticLoopSingleTurn tests the loop terminates when LLM responds without tool calls
func TestAgenticLoopSingleTurn(t *testing.T) {
	events := make(chan AgentEvent, 100)

	mockProvider := &MockProvider{
		Responses: []*genai.GenerateContentResponse{
			// First response: create a shape
			NewMockResponse("I'll create a red sphere for you.", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":   "red_sphere",
					"type": "sphere",
					"properties": map[string]any{
						"center": []any{0.0, 1.0, 0.0},
						"radius": 1.0,
						"material": map[string]any{
							"type":   "lambertian",
							"albedo": []any{0.8, 0.1, 0.1},
						},
					},
				},
			}),
			// Second response: no tool calls, signals completion
			NewMockResponse("Done! I've created the red sphere."),
		},
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Create a red sphere"}},
		},
	}

	_, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM twice (once with tool call, once without)
	if mockProvider.CallCount != 2 {
		t.Errorf("Expected 2 LLM calls, got %d", mockProvider.CallCount)
	}

	// Check that scene has the shape
	if len(agent.sceneManager.state.Shapes) != 1 {
		t.Errorf("Expected 1 shape, got %d", len(agent.sceneManager.state.Shapes))
	}

	if agent.sceneManager.state.Shapes[0].ID != "red_sphere" {
		t.Errorf("Expected shape ID 'red_sphere', got '%s'", agent.sceneManager.state.Shapes[0].ID)
	}

	close(events)
}

// TestAgenticLoopMultiTurn tests the loop continues through multiple turns
func TestAgenticLoopMultiTurn(t *testing.T) {
	events := make(chan AgentEvent, 100)

	mockProvider := &MockProvider{
		Responses: []*genai.GenerateContentResponse{
			// Turn 1: Create sphere
			NewMockResponse("Creating sphere...", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":   "sphere1",
					"type": "sphere",
					"properties": map[string]any{
						"center": []any{0.0, 1.0, 0.0},
						"radius": 1.0,
					},
				},
			}),
			// Turn 2: Update sphere (after seeing result)
			NewMockResponse("Now making it bigger...", &genai.FunctionCall{
				Name: "update_shape",
				Args: map[string]any{
					"id": "sphere1",
					"updates": map[string]any{
						"properties": map[string]any{
							"radius": 2.0,
						},
					},
				},
			}),
			// Turn 3: Done
			NewMockResponse("Perfect! The sphere is now complete."),
		},
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Create a sphere and make it big"}},
		},
	}

	_, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM 3 times
	if mockProvider.CallCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockProvider.CallCount)
	}

	// Check final state
	if len(agent.sceneManager.state.Shapes) != 1 {
		t.Errorf("Expected 1 shape, got %d", len(agent.sceneManager.state.Shapes))
	}

	// Check radius was updated
	shape := agent.sceneManager.state.Shapes[0]
	radius, ok := extractFloat(shape.Properties, "radius")
	if !ok {
		t.Fatal("radius property not found")
	}
	if radius != 2.0 {
		t.Errorf("Expected radius 2.0, got %f. Full properties: %+v", radius, shape.Properties)
	}

	close(events)
}

// TestAgenticLoopTurnLimit tests that loop stops at max turns
func TestAgenticLoopTurnLimit(t *testing.T) {
	events := make(chan AgentEvent, 100)

	// Create 15 responses that all have tool calls (exceeds limit of 10)
	responses := make([]*genai.GenerateContentResponse, 15)
	for i := 0; i < 15; i++ {
		responses[i] = NewMockResponse("Creating another shape...", &genai.FunctionCall{
			Name: "create_shape",
			Args: map[string]any{
				"id":   "shape" + string(rune('0'+i)),
				"type": "sphere",
				"properties": map[string]any{
					"center": []any{float64(i), 0.0, 0.0},
					"radius": 1.0,
				},
			},
		})
	}

	mockProvider := &MockProvider{
		Responses: responses,
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Create many spheres"}},
		},
	}

	_, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should stop at 10 turns
	if mockProvider.CallCount != 10 {
		t.Errorf("Expected exactly 10 LLM calls (turn limit), got %d", mockProvider.CallCount)
	}

	// Should have emitted a turn limit message
	foundTurnLimitMessage := false
	for event := range events {
		if resp, ok := event.(ResponseEvent); ok {
			if len(resp.Text) > 0 && resp.Text[0:7] == "Reached" {
				foundTurnLimitMessage = true
				break
			}
		}
	}

	if !foundTurnLimitMessage {
		t.Error("Expected turn limit message but didn't find one")
	}

	close(events)
}

// TestAgenticLoopErrorRecovery tests that errors are sent back to LLM
func TestAgenticLoopErrorRecovery(t *testing.T) {
	events := make(chan AgentEvent, 100)

	mockProvider := &MockProvider{
		Responses: []*genai.GenerateContentResponse{
			// Turn 1: Try to create sphere with missing property
			NewMockResponse("Creating sphere...", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":         "sphere1",
					"type":       "sphere",
					"properties": map[string]any{
						// Missing center and radius
					},
				},
			}),
			// Turn 2: Retry with correct properties (after seeing error)
			NewMockResponse("Let me fix that...", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":   "sphere1",
					"type": "sphere",
					"properties": map[string]any{
						"center": []any{0.0, 1.0, 0.0},
						"radius": 1.0,
					},
				},
			}),
			// Turn 3: Done
			NewMockResponse("Fixed! Sphere created successfully."),
		},
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Create a sphere"}},
		},
	}

	_, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM 3 times
	if mockProvider.CallCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockProvider.CallCount)
	}

	// Final scene should have the shape (after retry)
	if len(agent.sceneManager.state.Shapes) != 1 {
		t.Errorf("Expected 1 shape after successful retry, got %d", len(agent.sceneManager.state.Shapes))
	}

	close(events)
}

// TestAgenticLoopMixedSuccessFailure tests multiple tool calls with some succeeding and some failing
func TestAgenticLoopMixedSuccessFailure(t *testing.T) {
	events := make(chan AgentEvent, 100)

	mockProvider := &MockProvider{
		Responses: []*genai.GenerateContentResponse{
			// Turn 1: Multiple tool calls - one succeeds, one fails
			NewMockResponse("Creating two spheres...",
				&genai.FunctionCall{
					Name: "create_shape",
					Args: map[string]any{
						"id":   "good_sphere",
						"type": "sphere",
						"properties": map[string]any{
							"center": []any{0.0, 1.0, 0.0},
							"radius": 1.0,
						},
					},
				},
				&genai.FunctionCall{
					Name: "create_shape",
					Args: map[string]any{
						"id":   "bad_sphere",
						"type": "sphere",
						"properties": map[string]any{
							// Missing center - will fail
							"radius": 1.0,
						},
					},
				},
			),
			// Turn 2: Fix the failed one after seeing the error
			NewMockResponse("Let me fix the second sphere...", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":   "bad_sphere",
					"type": "sphere",
					"properties": map[string]any{
						"center": []any{2.0, 1.0, 0.0},
						"radius": 1.0,
					},
				},
			}),
			// Turn 3: Done
			NewMockResponse("Both spheres created successfully!"),
		},
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Create two spheres"}},
		},
	}

	_, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM 3 times
	if mockProvider.CallCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockProvider.CallCount)
	}

	// Final scene should have both shapes (one from turn 1, one from turn 2)
	if len(agent.sceneManager.state.Shapes) != 2 {
		t.Errorf("Expected 2 shapes after retry, got %d", len(agent.sceneManager.state.Shapes))
	}

	// Verify both shapes exist
	if agent.sceneManager.FindShape("good_sphere") == nil {
		t.Error("Expected to find 'good_sphere'")
	}
	if agent.sceneManager.FindShape("bad_sphere") == nil {
		t.Error("Expected to find 'bad_sphere' after retry")
	}

	close(events)
}

// TestMultipleTextParts tests that multiple text parts are concatenated
func TestMultipleTextParts(t *testing.T) {
	events := make(chan AgentEvent, 100)

	// Create a response with multiple text parts manually
	mockProvider := &MockProvider{
		Responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Role: "model",
							Parts: []*genai.Part{
								{Text: "First part. "},
								{Text: "Second part. "},
								{Text: "Third part."},
							},
						},
					},
				},
			},
		},
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Test multiple parts"}},
		},
	}

	_, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Collect events
	var responseEvents []ResponseEvent
	done := false
	for !done {
		select {
		case event := <-events:
			if re, ok := event.(ResponseEvent); ok {
				responseEvents = append(responseEvents, re)
			}
			if _, ok := event.(CompleteEvent); ok {
				done = true
			}
		default:
			done = true
		}
	}

	// Should have emitted 3 separate response events
	if len(responseEvents) != 3 {
		t.Fatalf("Expected 3 response events, got %d", len(responseEvents))
	}

	// Check each part was emitted separately
	if responseEvents[0].Text != "First part. " {
		t.Errorf("Expected first part %q, got %q", "First part. ", responseEvents[0].Text)
	}
	if responseEvents[1].Text != "Second part. " {
		t.Errorf("Expected second part %q, got %q", "Second part. ", responseEvents[1].Text)
	}
	if responseEvents[2].Text != "Third part." {
		t.Errorf("Expected third part %q, got %q", "Third part.", responseEvents[2].Text)
	}

	close(events)
}

func TestRenderSceneEmptyScene(t *testing.T) {
	events := make(chan AgentEvent, 100)
	agent := NewWithProvider(events, &MockProvider{}, "mock-model")

	// Create a render_scene request without any shapes
	req := &RenderSceneRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "render_scene"},
	}

	result := agent.executeToolRequests(req, "test_call_1")

	// Should fail with empty scene error
	if result.Success {
		t.Fatal("Expected render_scene to fail on empty scene")
	}

	if len(result.Errors) == 0 {
		t.Fatal("Expected error messages")
	}

	expectedError := "cannot render empty scene - add shapes first"
	if result.Errors[0] != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, result.Errors[0])
	}

	// Should have emitted a start event and a completion event
	var startEvents []ToolCallStartEvent
	var toolEvents []ToolCallEvent
	for len(events) > 0 {
		event := <-events
		if se, ok := event.(ToolCallStartEvent); ok {
			startEvents = append(startEvents, se)
		}
		if te, ok := event.(ToolCallEvent); ok {
			toolEvents = append(toolEvents, te)
		}
	}

	// Should have 1 start event and 1 completion event (with error)
	if len(startEvents) != 1 {
		t.Errorf("Expected 1 ToolCallStartEvent, got %d", len(startEvents))
	}
	if len(toolEvents) != 1 {
		t.Errorf("Expected 1 ToolCallEvent, got %d", len(toolEvents))
	}

	close(events)
}

func TestRenderSceneWithShape(t *testing.T) {
	events := make(chan AgentEvent, 100)
	agent := NewWithProvider(events, &MockProvider{}, "mock-model")

	// Add a simple sphere to the scene
	shape := ShapeRequest{
		ID:   "test_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{0.0, 0.0, 0.0},
			"radius": 1.0,
			"material": map[string]interface{}{
				"type":   "lambertian",
				"albedo": []interface{}{0.8, 0.3, 0.3},
			},
		},
	}
	err := agent.sceneManager.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("Failed to add shape: %v", err)
	}

	// Add a light so we can actually see the sphere
	light := LightRequest{
		ID:   "test_light",
		Type: "point_spot_light",
		Properties: map[string]interface{}{
			"center":   []interface{}{2.0, 3.0, 2.0},
			"emission": []interface{}{10.0, 10.0, 10.0},
		},
	}
	err = agent.sceneManager.AddLights([]LightRequest{light})
	if err != nil {
		t.Fatalf("Failed to add light: %v", err)
	}

	// Create render_scene request
	req := &RenderSceneRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "render_scene"},
	}

	// Execute the render (this will actually render, but should be fast for 100x75)
	result := agent.executeToolRequests(req, "test_call_1")

	// Should succeed
	if !result.Success {
		t.Fatalf("Expected render_scene to succeed, got errors: %v", result.Errors)
	}

	// Check that we got metadata back
	resultMap, ok := result.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["shape_count"] != 1 {
		t.Errorf("Expected shape_count=1, got %v", resultMap["shape_count"])
	}
	if resultMap["samples_per_pixel"] != 500 {
		t.Errorf("Expected samples_per_pixel=500, got %v", resultMap["samples_per_pixel"])
	}
	if resultMap["width"] != 400 {
		t.Errorf("Expected width=400, got %v", resultMap["width"])
	}
	if resultMap["height"] != 300 {
		t.Errorf("Expected height=300, got %v", resultMap["height"])
	}

	// Check that the image was populated
	if req.RenderedImage == nil {
		t.Fatal("Expected RenderedImage to be populated")
	}

	if len(req.RenderedImage) == 0 {
		t.Fatal("Expected RenderedImage to have data")
	}

	// Verify it's a valid PNG by checking the header
	if len(req.RenderedImage) < 8 {
		t.Fatal("RenderedImage too small to be a valid PNG")
	}
	pngHeader := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	for i := 0; i < 8; i++ {
		if req.RenderedImage[i] != pngHeader[i] {
			t.Fatalf("Invalid PNG header at byte %d: expected %d, got %d", i, pngHeader[i], req.RenderedImage[i])
		}
	}

	t.Logf("Rendered PNG size: %d bytes", len(req.RenderedImage))

	// Check that we got start and completion events
	var startEvents []ToolCallStartEvent
	var toolEvents []ToolCallEvent
	for len(events) > 0 {
		event := <-events
		if se, ok := event.(ToolCallStartEvent); ok {
			startEvents = append(startEvents, se)
		}
		if te, ok := event.(ToolCallEvent); ok {
			toolEvents = append(toolEvents, te)
		}
	}

	// Should have 1 start event
	if len(startEvents) != 1 {
		t.Errorf("Expected 1 ToolCallStartEvent, got %d", len(startEvents))
	}

	// Should have 1 completion event
	if len(toolEvents) != 1 {
		t.Errorf("Expected 1 ToolCallEvent, got %d", len(toolEvents))
	}

	// The completion event should have the image
	if len(toolEvents) > 0 {
		if !toolEvents[0].Success {
			t.Error("Expected ToolCallEvent to be successful")
		}
		if len(toolEvents[0].RenderedImage) == 0 {
			t.Error("Expected ToolCallEvent to have RenderedImage data")
		}
	}

	close(events)
}

func TestRenderSceneToolParsing(t *testing.T) {
	// Test that render_scene function call is parsed correctly
	call := &genai.FunctionCall{
		Name: "render_scene",
		Args: map[string]interface{}{},
	}

	req := parseToolRequestFromFunctionCall(&llm.FunctionCall{Name: call.Name, Arguments: call.Args})
	if req == nil {
		t.Fatal("Expected non-nil request")
	}

	renderReq, ok := req.(*RenderSceneRequest)
	if !ok {
		t.Fatalf("Expected *RenderSceneRequest, got %T", req)
	}

	if renderReq.ToolName() != "render_scene" {
		t.Errorf("Expected tool name 'render_scene', got %q", renderReq.ToolName())
	}
}

func TestGetSceneStateWithEmptyScene(t *testing.T) {
	events := make(chan AgentEvent, 100)
	agent := NewWithProvider(events, &MockProvider{}, "mock-model")

	req := &GetSceneStateRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "get_scene_state"},
	}

	result := agent.executeToolRequests(req, "test_call_1")

	if !result.Success {
		t.Fatalf("Expected success, got errors: %v", result.Errors)
	}

	resultMap, ok := result.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result.Result)
	}

	// Check that scene state has expected fields
	if _, ok := resultMap["shapes"]; !ok {
		t.Error("Expected 'shapes' field in scene state")
	}
	if _, ok := resultMap["lights"]; !ok {
		t.Error("Expected 'lights' field in scene state")
	}
	if _, ok := resultMap["camera"]; !ok {
		t.Error("Expected 'camera' field in scene state")
	}

	// Check that SceneState was populated in the request
	if req.SceneState == nil {
		t.Error("Expected SceneState to be populated in request")
	}
}

func TestGetSceneStateWithShapesAndLights(t *testing.T) {
	events := make(chan AgentEvent, 100)
	agent := NewWithProvider(events, &MockProvider{}, "mock-model")

	// Add a shape
	shape := ShapeRequest{
		ID:   "test_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{0.0, 1.0, 0.0},
			"radius": 1.0,
			"material": map[string]interface{}{
				"type":   "lambertian",
				"albedo": []interface{}{0.8, 0.1, 0.1},
			},
		},
	}
	err := agent.sceneManager.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("Failed to add shape: %v", err)
	}

	// Add a light
	light := LightRequest{
		ID:   "test_light",
		Type: "point_spot_light",
		Properties: map[string]interface{}{
			"center":   []interface{}{5.0, 5.0, 5.0},
			"emission": []interface{}{10.0, 10.0, 10.0},
		},
	}
	err = agent.sceneManager.AddLights([]LightRequest{light})
	if err != nil {
		t.Fatalf("Failed to add light: %v", err)
	}

	// Get scene state
	req := &GetSceneStateRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "get_scene_state"},
	}

	result := agent.executeToolRequests(req, "test_call_1")

	if !result.Success {
		t.Fatalf("Expected success, got errors: %v", result.Errors)
	}

	resultMap, ok := result.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result.Result)
	}

	// Check shapes
	shapes, ok := resultMap["shapes"].([]ShapeRequest)
	if !ok {
		t.Fatalf("Expected shapes to be []ShapeRequest, got %T", resultMap["shapes"])
	}
	if len(shapes) != 1 {
		t.Errorf("Expected 1 shape, got %d", len(shapes))
	}
	if len(shapes) > 0 && shapes[0].ID != "test_sphere" {
		t.Errorf("Expected shape ID 'test_sphere', got %q", shapes[0].ID)
	}

	// Check lights
	lights, ok := resultMap["lights"].([]LightRequest)
	if !ok {
		t.Fatalf("Expected lights to be []LightRequest, got %T", resultMap["lights"])
	}
	if len(lights) != 1 {
		t.Errorf("Expected 1 light, got %d", len(lights))
	}
	if len(lights) > 0 && lights[0].ID != "test_light" {
		t.Errorf("Expected light ID 'test_light', got %q", lights[0].ID)
	}

	// Check camera is present
	_, ok = resultMap["camera"].(CameraInfo)
	if !ok {
		t.Errorf("Expected camera to be CameraInfo, got %T", resultMap["camera"])
	}
}

func TestGetSceneStateToolParsing(t *testing.T) {
	call := &genai.FunctionCall{
		Name: "get_scene_state",
		Args: map[string]any{},
	}

	req := parseToolRequestFromFunctionCall(&llm.FunctionCall{Name: call.Name, Arguments: call.Args})
	if req == nil {
		t.Fatal("Expected non-nil request")
	}

	getSceneReq, ok := req.(*GetSceneStateRequest)
	if !ok {
		t.Fatalf("Expected *GetSceneStateRequest, got %T", req)
	}

	if getSceneReq.ToolName() != "get_scene_state" {
		t.Errorf("Expected tool name 'get_scene_state', got %q", getSceneReq.ToolName())
	}
}

// TestConversationHistoryPreserved verifies that ProcessMessage returns complete conversation history
// including user messages, assistant responses, function calls, and function responses
func TestConversationHistoryPreserved(t *testing.T) {
	events := make(chan AgentEvent, 100)

	mockProvider := &MockProvider{
		Responses: []*genai.GenerateContentResponse{
			// First response: LLM calls create_shape
			NewMockResponse("I'll create a sphere.", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":   "sphere1",
					"type": "sphere",
					"properties": map[string]any{
						"center": []any{0.0, 0.0, 0.0},
						"radius": 1.0,
						"material": map[string]any{
							"type":   "lambertian",
							"albedo": []any{0.8, 0.2, 0.2},
						},
					},
				},
			}),
			// Second response: LLM responds with text (no tool calls)
			NewMockResponse("Done! The sphere has been created."),
		},
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	// Initial conversation with one user message
	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Create a red sphere"}},
		},
	}

	// Process the message
	updatedConversation, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Verify conversation structure:
	// [0] user message (original)
	// [1] assistant message with text + function call
	// [2] function response
	// [3] assistant message with text only (completion)

	if len(updatedConversation) != 4 {
		t.Fatalf("Expected 4 messages in conversation, got %d", len(updatedConversation))
	}

	// Check message 0: original user message
	if updatedConversation[0].Role != llm.RoleUser {
		t.Errorf("Message 0: expected role 'user', got %q", updatedConversation[0].Role)
	}

	// Check message 1: assistant response with function call
	if updatedConversation[1].Role != llm.RoleAssistant {
		t.Errorf("Message 1: expected role 'assistant', got %q", updatedConversation[1].Role)
	}

	// Should have both text and function call parts
	hasText := false
	hasFunctionCall := false
	for _, part := range updatedConversation[1].Parts {
		if part.Type == llm.PartTypeText && part.Text != "" {
			hasText = true
		}
		if part.Type == llm.PartTypeFunctionCall {
			hasFunctionCall = true
		}
	}
	if !hasText {
		t.Error("Message 1: expected text part in assistant response")
	}
	if !hasFunctionCall {
		t.Error("Message 1: expected function call part in assistant response")
	}

	// Check message 2: function response
	if updatedConversation[2].Role != llm.RoleUser {
		t.Errorf("Message 2: expected role 'user' (function responses), got %q", updatedConversation[2].Role)
	}

	// Should have function response parts
	hasFunctionResponse := false
	for _, part := range updatedConversation[2].Parts {
		if part.Type == llm.PartTypeFunctionResponse {
			hasFunctionResponse = true
			// Verify it has the result
			if part.FunctionResp == nil {
				t.Error("Message 2: function response part missing FunctionResp")
			}
		}
	}
	if !hasFunctionResponse {
		t.Error("Message 2: expected function response part")
	}

	// Check message 3: final assistant response (text only, no function calls)
	if updatedConversation[3].Role != llm.RoleAssistant {
		t.Errorf("Message 3: expected role 'assistant', got %q", updatedConversation[3].Role)
	}

	// Verify no function calls in final message (signals completion)
	for _, part := range updatedConversation[3].Parts {
		if part.Type == llm.PartTypeFunctionCall {
			t.Error("Message 3: unexpected function call in completion message")
		}
	}

	// Drain events
	close(events)
	for range events {
	}
}

// TestMultiTurnConversationHistory verifies that conversation history is properly maintained
// across multiple user messages in a session
func TestMultiTurnConversationHistory(t *testing.T) {
	events := make(chan AgentEvent, 100)

	mockProvider := &MockProvider{
		Responses: []*genai.GenerateContentResponse{
			// Turn 1: Create sphere
			NewMockResponse("Creating a red sphere.", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":   "sphere1",
					"type": "sphere",
					"properties": map[string]any{
						"center": []any{0.0, 0.0, 0.0},
						"radius": 1.0,
						"material": map[string]any{
							"type":   "lambertian",
							"albedo": []any{0.8, 0.2, 0.2},
						},
					},
				},
			}),
			NewMockResponse("Done! Created a red sphere."),

			// Turn 2: Create another shape (should have context from turn 1)
			NewMockResponse("Creating a blue cube next to the sphere.", &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]any{
					"id":   "cube1",
					"type": "box",
					"properties": map[string]any{
						"center": []any{3.0, 0.0, 0.0},
						"size":   []any{1.0, 1.0, 1.0},
						"material": map[string]any{
							"type":   "lambertian",
							"albedo": []any{0.2, 0.2, 0.8},
						},
					},
				},
			}),
			NewMockResponse("Done! Added a blue cube."),

			// Turn 3: Update existing shape (referencing previous turns)
			NewMockResponse("Making the sphere bigger.", &genai.FunctionCall{
				Name: "update_shape",
				Args: map[string]any{
					"id": "sphere1",
					"properties": map[string]any{
						"radius": 2.0,
					},
				},
			}),
			NewMockResponse("Done! The sphere is now bigger."),
		},
	}

	agent := NewWithProvider(events, mockProvider, "mock-model")

	// Turn 1: First user message
	conversation := []llm.Message{
		{
			Role:  llm.RoleUser,
			Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Create a red sphere"}},
		},
	}

	conversation, err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}

	// After turn 1, should have:
	// [0] user: "Create a red sphere"
	// [1] assistant: text + create_shape call
	// [2] function: create_shape response
	// [3] assistant: "Done! Created a red sphere."

	if len(conversation) != 4 {
		t.Errorf("After turn 1: expected 4 messages, got %d", len(conversation))
	}

	// Turn 2: Add another user message
	conversation = append(conversation, llm.Message{
		Role:  "user",
		Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Now add a blue cube"}},
	})

	conversation, err = agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}

	// After turn 2, should have previous 4 + new 4:
	// [4] user: "Now add a blue cube"
	// [5] assistant: text + create_shape call
	// [6] function: create_shape response
	// [7] assistant: "Done! Added a blue cube."

	if len(conversation) != 8 {
		t.Errorf("After turn 2: expected 8 messages, got %d", len(conversation))
	}

	// Verify turn 2 messages have correct structure
	if conversation[4].Role != llm.RoleUser {
		t.Errorf("Message 4: expected role 'user', got %q", conversation[4].Role)
	}
	if conversation[5].Role != llm.RoleAssistant {
		t.Errorf("Message 5: expected role 'assistant', got %q", conversation[5].Role)
	}
	if conversation[6].Role != llm.RoleUser {
		t.Errorf("Message 6: expected role 'user' (function responses), got %q", conversation[6].Role)
	}
	if conversation[7].Role != llm.RoleAssistant {
		t.Errorf("Message 7: expected role 'assistant', got %q", conversation[7].Role)
	}

	// Turn 3: Update previous shape (tests that context is preserved)
	conversation = append(conversation, llm.Message{
		Role:  "user",
		Parts: []llm.Part{{Type: llm.PartTypeText, Text: "Make the sphere bigger"}},
	})

	conversation, err = agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("Turn 3 failed: %v", err)
	}

	// After turn 3, should have 8 + 4 = 12 messages
	if len(conversation) != 12 {
		t.Errorf("After turn 3: expected 12 messages, got %d", len(conversation))
	}

	// Verify the update_shape function call is in the conversation
	foundUpdateCall := false
	for i := 8; i < len(conversation); i++ {
		if conversation[i].Role == llm.RoleAssistant {
			for _, part := range conversation[i].Parts {
				if part.Type == llm.PartTypeFunctionCall && part.FunctionCall != nil {
					if part.FunctionCall.Name == "update_shape" {
						foundUpdateCall = true
						// Verify it references the correct shape ID from turn 1
						if id, ok := part.FunctionCall.Arguments["id"].(string); ok {
							if id != "sphere1" {
								t.Errorf("Expected update to reference 'sphere1', got %q", id)
							}
						}
					}
				}
			}
		}
	}

	if !foundUpdateCall {
		t.Error("Expected to find update_shape function call in turn 3")
	}

	// Verify all original user messages are preserved (count messages with text parts, not function responses)
	originalUserMessageCount := 0
	for _, msg := range conversation {
		if msg.Role == llm.RoleUser {
			// Check if this is an original user message (has text parts) vs function response
			for _, part := range msg.Parts {
				if part.Type == llm.PartTypeText {
					originalUserMessageCount++
					break
				}
			}
		}
	}
	if originalUserMessageCount != 3 {
		t.Errorf("Expected 3 original user messages, got %d", originalUserMessageCount)
	}

	// Drain events
	close(events)
	for range events {
	}
}
