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
