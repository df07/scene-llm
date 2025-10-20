package agent

import (
	"testing"
	"time"
)

// Test the executeToolRequest method without requiring Google API
func TestExecuteToolRequestWithoutAPI(t *testing.T) {
	// Create agent components manually to avoid API requirement
	events := make(chan AgentEvent, 10)
	sceneManager := NewSceneManager()

	agent := &Agent{
		sceneManager: sceneManager,
		events:       events,
	}

	// Test create operation
	t.Run("CreateShape", func(t *testing.T) {
		operation := &CreateShapeRequest{
			BaseToolRequest: BaseToolRequest{ToolType: "create_shape"},
			Shape: ShapeRequest{
				ID:   "test_sphere",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"color":  []interface{}{1.0, 0.0, 0.0},
				},
			},
		}

		// Execute the operation
		agent.executeToolRequests(operation)

		// Check that a ToolCallEvent was emitted
		select {
		case event := <-events:
			toolEvent, ok := event.(ToolCallEvent)
			if !ok {
				t.Fatalf("Expected ToolCallEvent, got %T", event)
			}

			if !toolEvent.Success {
				t.Errorf("Expected operation to succeed, got error: %s", toolEvent.Error)
			}

			if toolEvent.Request.ToolName() != "create_shape" {
				t.Errorf("Expected ToolName to be 'create_shape', got '%s'", toolEvent.Request.ToolName())
			}

			if toolEvent.Duration < 0 {
				t.Error("Expected Duration to be non-negative")
			}

		case <-time.After(100 * time.Millisecond):
			t.Fatal("No event received within timeout")
		}

		// Verify the shape was actually added to the scene
		if sceneManager.GetShapeCount() != 1 {
			t.Errorf("Expected 1 shape in scene, got %d", sceneManager.GetShapeCount())
		}
	})

	// Test update operation
	t.Run("UpdateShape", func(t *testing.T) {
		operation := &UpdateShapeRequest{
			BaseToolRequest: BaseToolRequest{ToolType: "update_shape", Id: "test_sphere"},
			Updates: map[string]interface{}{
				"properties": map[string]interface{}{
					"color": []interface{}{0.0, 1.0, 0.0}, // Change to green
				},
			},
		}

		// Execute the operation
		agent.executeToolRequests(operation)

		// Check that a ToolCallEvent was emitted
		select {
		case event := <-events:
			toolEvent, ok := event.(ToolCallEvent)
			if !ok {
				t.Fatalf("Expected ToolCallEvent, got %T", event)
			}

			if !toolEvent.Success {
				t.Errorf("Expected operation to succeed, got error: %s", toolEvent.Error)
			}

			// Check that before/after state was captured
			updateOp, ok := toolEvent.Request.(*UpdateShapeRequest)
			if !ok {
				t.Fatalf("Expected UpdateShapeRequest, got %T", toolEvent.Request)
			}

			if updateOp.Before == nil {
				t.Error("Expected Before state to be captured")
			}

			if updateOp.After == nil {
				t.Error("Expected After state to be captured")
			}

		case <-time.After(100 * time.Millisecond):
			t.Fatal("No event received within timeout")
		}
	})

	// Test remove operation
	t.Run("RemoveShape", func(t *testing.T) {
		operation := &RemoveShapeRequest{
			BaseToolRequest: BaseToolRequest{ToolType: "remove_shape", Id: "test_sphere"},
		}

		// Execute the operation
		agent.executeToolRequests(operation)

		// Check that a ToolCallEvent was emitted
		select {
		case event := <-events:
			toolEvent, ok := event.(ToolCallEvent)
			if !ok {
				t.Fatalf("Expected ToolCallEvent, got %T", event)
			}

			if !toolEvent.Success {
				t.Errorf("Expected operation to succeed, got error: %s", toolEvent.Error)
			}

			// Check that removed shape was captured
			removeOp, ok := toolEvent.Request.(*RemoveShapeRequest)
			if !ok {
				t.Fatalf("Expected RemoveShapeRequest, got %T", toolEvent.Request)
			}

			if removeOp.RemovedShape == nil {
				t.Error("Expected RemovedShape to be captured")
			}

		case <-time.After(100 * time.Millisecond):
			t.Fatal("No event received within timeout")
		}

		// Verify the shape was actually removed from the scene
		if sceneManager.GetShapeCount() != 0 {
			t.Errorf("Expected 0 shapes in scene after removal, got %d", sceneManager.GetShapeCount())
		}
	})

	// Test error case
	t.Run("ErrorCase", func(t *testing.T) {
		operation := &UpdateShapeRequest{
			BaseToolRequest: BaseToolRequest{ToolType: "update_shape", Id: "nonexistent_shape"},
			Updates: map[string]interface{}{
				"properties": map[string]interface{}{
					"color": []interface{}{0.0, 1.0, 0.0},
				},
			},
		}

		// Execute the operation
		agent.executeToolRequests(operation)

		// Check that a failed ToolCallEvent was emitted
		select {
		case event := <-events:
			toolEvent, ok := event.(ToolCallEvent)
			if !ok {
				t.Fatalf("Expected ToolCallEvent, got %T", event)
			}

			if toolEvent.Success {
				t.Error("Expected operation to fail, but it succeeded")
			}

			if toolEvent.Error == "" {
				t.Error("Expected error message to be set")
			}

		case <-time.After(100 * time.Millisecond):
			t.Fatal("No event received within timeout")
		}
	})
}

func TestToolCallEventCreation(t *testing.T) {
	operation := &CreateShapeRequest{
		Shape: ShapeRequest{
			ID:   "test_sphere",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{0.0, 0.0, 0.0},
				"radius": 1.0,
			},
		},
	}

	event := NewToolCallEvent("test_id", operation, true, "", 25)

	if event.Request != operation {
		t.Error("Expected Request to match the provided operation")
	}

	if !event.Success {
		t.Error("Expected Success to be true")
	}

	if event.Error != "" {
		t.Errorf("Expected Error to be empty, got '%s'", event.Error)
	}

	if event.Duration != 25 {
		t.Errorf("Expected Duration to be 25, got %d", event.Duration)
	}

	if event.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}

	if event.EventType() != "function_calls" {
		t.Errorf("Expected EventType to be 'function_calls', got '%s'", event.EventType())
	}
}
