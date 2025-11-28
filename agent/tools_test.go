package agent

import (
	"testing"

	"github.com/df07/scene-llm/agent/llm"
	"google.golang.org/genai"
)

// TestToolSchemasValid verifies that all tool schemas are properly formed
// with required fields like Items for array types
func TestToolSchemasValid(t *testing.T) {
	tools := getAllTools()

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			// Validate the tool has a name and description
			if tool.Name == "" {
				t.Error("Tool name is empty")
			}
			if tool.Description == "" {
				t.Error("Tool description is empty")
			}

			// Validate parameters schema
			if tool.Parameters != nil {
				validateSchema(t, tool.Name, "parameters", tool.Parameters)
			}
		})
	}
}

// validateSchema recursively validates a schema structure
func validateSchema(t *testing.T, toolName, path string, schema *llm.Schema) {
	// If this is an array type, it MUST have Items defined
	if schema.Type == llm.TypeArray {
		if schema.Items == nil {
			t.Errorf("Tool %s: schema at %s is TypeArray but missing Items field (required by Gemini API)", toolName, path)
		} else {
			// Recursively validate the Items schema
			validateSchema(t, toolName, path+".items", schema.Items)
		}
	}

	// If this is an object type, validate all properties
	if schema.Type == llm.TypeObject && schema.Properties != nil {
		for propName, propSchema := range schema.Properties {
			validateSchema(t, toolName, path+"."+propName, propSchema)
		}
	}
}

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
