package agent

import (
	"testing"

	"google.golang.org/genai"
)

func TestParseCreateShapeOperation(t *testing.T) {
	// Test create_shape function call parsing
	call := &genai.FunctionCall{
		Name: "create_shape",
		Args: map[string]interface{}{
			"id":   "test_sphere",
			"type": "sphere",
			"properties": map[string]interface{}{
				"position": []interface{}{1.0, 2.0, 3.0},
				"radius":   1.5,
				"color":    []interface{}{1.0, 0.0, 0.0},
			},
		},
	}

	operation := parseCreateShapeOperation(call)
	if operation == nil {
		t.Fatal("Expected operation to be non-nil")
	}

	if operation.ToolName() != "create_shape" {
		t.Errorf("Expected ToolName() to be 'create_shape', got '%s'", operation.ToolName())
	}

	if operation.Target() != "test_sphere" {
		t.Errorf("Expected Target() to be 'test_sphere', got '%s'", operation.Target())
	}

	if operation.Shape.ID != "test_sphere" {
		t.Errorf("Expected Shape.ID to be 'test_sphere', got '%s'", operation.Shape.ID)
	}

	if operation.Shape.Type != "sphere" {
		t.Errorf("Expected Shape.Type to be 'sphere', got '%s'", operation.Shape.Type)
	}
}

func TestParseUpdateShapeOperation(t *testing.T) {
	// Test update_shape function call parsing
	call := &genai.FunctionCall{
		Name: "update_shape",
		Args: map[string]interface{}{
			"id": "test_sphere",
			"updates": map[string]interface{}{
				"properties": map[string]interface{}{
					"color": []interface{}{0.0, 1.0, 0.0},
				},
			},
		},
	}

	operation := parseUpdateShapeOperation(call)
	if operation == nil {
		t.Fatal("Expected operation to be non-nil")
	}

	if operation.ToolName() != "update_shape" {
		t.Errorf("Expected ToolName() to be 'update_shape', got '%s'", operation.ToolName())
	}

	if operation.Target() != "test_sphere" {
		t.Errorf("Expected Target() to be 'test_sphere', got '%s'", operation.Target())
	}

	if operation.ID != "test_sphere" {
		t.Errorf("Expected ID to be 'test_sphere', got '%s'", operation.ID)
	}

	if operation.Updates == nil {
		t.Error("Expected Updates to be non-nil")
	}
}

func TestParseRemoveShapeOperation(t *testing.T) {
	// Test remove_shape function call parsing
	call := &genai.FunctionCall{
		Name: "remove_shape",
		Args: map[string]interface{}{
			"id": "test_sphere",
		},
	}

	operation := parseRemoveShapeOperation(call)
	if operation == nil {
		t.Fatal("Expected operation to be non-nil")
	}

	if operation.ToolName() != "remove_shape" {
		t.Errorf("Expected ToolName() to be 'remove_shape', got '%s'", operation.ToolName())
	}

	if operation.Target() != "test_sphere" {
		t.Errorf("Expected Target() to be 'test_sphere', got '%s'", operation.Target())
	}

	if operation.ID != "test_sphere" {
		t.Errorf("Expected ID to be 'test_sphere', got '%s'", operation.ID)
	}
}

func TestParseToolOperationFromFunctionCall(t *testing.T) {
	tests := []struct {
		name           string
		functionCall   *genai.FunctionCall
		expectedType   string
		expectedTarget string
	}{
		{
			name: "create_shape",
			functionCall: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "blue_sphere",
					"type": "sphere",
					"properties": map[string]interface{}{
						"position": []interface{}{0.0, 0.0, 0.0},
						"radius":   1.0,
					},
				},
			},
			expectedType:   "create_shape",
			expectedTarget: "blue_sphere",
		},
		{
			name: "update_shape",
			functionCall: &genai.FunctionCall{
				Name: "update_shape",
				Args: map[string]interface{}{
					"id": "blue_sphere",
					"updates": map[string]interface{}{
						"properties": map[string]interface{}{
							"color": []interface{}{1.0, 0.0, 0.0},
						},
					},
				},
			},
			expectedType:   "update_shape",
			expectedTarget: "blue_sphere",
		},
		{
			name: "remove_shape",
			functionCall: &genai.FunctionCall{
				Name: "remove_shape",
				Args: map[string]interface{}{
					"id": "blue_sphere",
				},
			},
			expectedType:   "remove_shape",
			expectedTarget: "blue_sphere",
		},
		{
			name: "unknown_function",
			functionCall: &genai.FunctionCall{
				Name: "unknown_function",
				Args: map[string]interface{}{},
			},
			expectedType:   "",
			expectedTarget: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation := parseToolOperationFromFunctionCall(tt.functionCall)

			if tt.expectedType == "" {
				if operation != nil {
					t.Errorf("Expected operation to be nil for unknown function, got %T", operation)
				}
				return
			}

			if operation == nil {
				t.Fatal("Expected operation to be non-nil")
			}

			if operation.ToolName() != tt.expectedType {
				t.Errorf("Expected ToolName() to be '%s', got '%s'", tt.expectedType, operation.ToolName())
			}

			if operation.Target() != tt.expectedTarget {
				t.Errorf("Expected Target() to be '%s', got '%s'", tt.expectedTarget, operation.Target())
			}
		})
	}
}
