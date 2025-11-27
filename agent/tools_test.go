package agent

import (
	"testing"

	"github.com/df07/scene-llm/agent/llm"
	"google.golang.org/genai"
)

func TestParseToolRequestFromFunctionCall(t *testing.T) {
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
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": 1.0,
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
			operation := parseToolRequestFromFunctionCall(&llm.FunctionCall{Name: tt.functionCall.Name, Arguments: tt.functionCall.Args})

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
