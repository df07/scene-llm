package agent

import (
	"context"
	"testing"

	"google.golang.org/genai"
)

// TestAgenticLoopSingleTurn tests the loop terminates when LLM responds without tool calls
func TestAgenticLoopSingleTurn(t *testing.T) {
	events := make(chan AgentEvent, 100)

	mockClient := &MockLLMClient{
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

	agent := NewWithMockLLM(events, mockClient)

	conversation := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Create a red sphere"}},
		},
	}

	err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM twice (once with tool call, once without)
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 LLM calls, got %d", mockClient.CallCount)
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

	mockClient := &MockLLMClient{
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

	agent := NewWithMockLLM(events, mockClient)

	conversation := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Create a sphere and make it big"}},
		},
	}

	err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM 3 times
	if mockClient.CallCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockClient.CallCount)
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

	mockClient := &MockLLMClient{
		Responses: responses,
	}

	agent := NewWithMockLLM(events, mockClient)

	conversation := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Create many spheres"}},
		},
	}

	err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should stop at 10 turns
	if mockClient.CallCount != 10 {
		t.Errorf("Expected exactly 10 LLM calls (turn limit), got %d", mockClient.CallCount)
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

	mockClient := &MockLLMClient{
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

	agent := NewWithMockLLM(events, mockClient)

	conversation := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Create a sphere"}},
		},
	}

	err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM 3 times
	if mockClient.CallCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockClient.CallCount)
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

	mockClient := &MockLLMClient{
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

	agent := NewWithMockLLM(events, mockClient)

	conversation := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Create two spheres"}},
		},
	}

	err := agent.ProcessMessage(context.Background(), conversation)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have called LLM 3 times
	if mockClient.CallCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockClient.CallCount)
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
	mockClient := &MockLLMClient{
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

	agent := NewWithMockLLM(events, mockClient)

	conversation := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Test multiple parts"}},
		},
	}

	err := agent.ProcessMessage(context.Background(), conversation)
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
	agent := NewWithMockLLM(events, &MockLLMClient{})

	// Create a render_scene request without any shapes
	req := &RenderSceneRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "render_scene"},
	}

	result := agent.executeToolRequests(req)

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
	agent := NewWithMockLLM(events, &MockLLMClient{})

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
	result := agent.executeToolRequests(req)

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

	req := parseToolRequestFromFunctionCall(call)
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
