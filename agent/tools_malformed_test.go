package agent

import (
	"testing"

	"google.golang.org/genai"
)

// TestMalformedLLMInput tests that malformed/missing data from LLM is handled gracefully
func TestMalformedLLMInput(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name        string
		call        *genai.FunctionCall
		expectError bool
		errorMsg    string
	}{
		{
			name: "create_shape with empty ID",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "",
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": 1.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "shape ID cannot be empty",
		},
		{
			name: "create_shape with empty type",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": 1.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "shape type cannot be empty",
		},
		{
			name: "create_shape with nil properties",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":         "test",
					"type":       "sphere",
					"properties": nil,
				},
			},
			expectError: true,
			errorMsg:    "shape properties cannot be nil",
		},
		{
			name: "create_shape with missing ID",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": 1.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "shape ID cannot be empty",
		},
		{
			name: "update_shape with empty ID",
			call: &genai.FunctionCall{
				Name: "update_shape",
				Args: map[string]interface{}{
					"id": "",
					"updates": map[string]interface{}{
						"properties": map[string]interface{}{
							"color": []interface{}{1.0, 0.0, 0.0},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "shape with ID '' not found",
		},
		{
			name: "remove_shape with empty ID",
			call: &genai.FunctionCall{
				Name: "remove_shape",
				Args: map[string]interface{}{
					"id": "",
				},
			},
			expectError: true,
			errorMsg:    "shape with ID '' not found",
		},
		{
			name: "sphere with wrong array length for center",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0}, // Only 2 elements
						"radius": 1.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "center must have exactly 3 values",
		},
		{
			name: "sphere with mixed types in array",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{1.0, "invalid", 3.0},
						"radius": 1.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "center[1] must be a number",
		},
		{
			name: "sphere with negative radius",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": -1.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "radius must be positive",
		},
		{
			name: "box with negative dimensions",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "box",
					"properties": map[string]interface{}{
						"center":     []interface{}{0.0, 0.0, 0.0},
						"dimensions": []interface{}{1.0, -1.0, 1.0},
					},
				},
			},
			expectError: true,
			errorMsg:    "dimensions[1] must be >= 0.0",
		},
		{
			name: "lambertian material with missing albedo",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": 1.0,
						"material": map[string]interface{}{
							"type": "lambertian",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "lambertian material requires 'albedo' property",
		},
		{
			name: "metal material with fuzz out of range",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": 1.0,
						"material": map[string]interface{}{
							"type":   "metal",
							"albedo": []interface{}{1.0, 1.0, 1.0},
							"fuzz":   1.5, // > 1.0
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "fuzz' must be in range [0.0,1.0]",
		},
		{
			name: "dielectric with refractive_index too low",
			call: &genai.FunctionCall{
				Name: "create_shape",
				Args: map[string]interface{}{
					"id":   "test",
					"type": "sphere",
					"properties": map[string]interface{}{
						"center": []interface{}{0.0, 0.0, 0.0},
						"radius": 1.0,
						"material": map[string]interface{}{
							"type":             "dielectric",
							"refractive_index": 0.5, // < 1.0
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "refractive_index' must be >= 1.0",
		},
		{
			name: "set_camera with missing center",
			call: &genai.FunctionCall{
				Name: "set_camera",
				Args: map[string]interface{}{
					"look_at":  []interface{}{0.0, 0.0, 0.0},
					"vfov":     45.0,
					"aperture": 0.0,
				},
			},
			expectError: true,
			errorMsg:    "camera center must be provided",
		},
		{
			name: "set_camera with missing look_at",
			call: &genai.FunctionCall{
				Name: "set_camera",
				Args: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 5.0},
					"vfov":     45.0,
					"aperture": 0.0,
				},
			},
			expectError: true,
			errorMsg:    "camera look_at must be provided",
		},
		{
			name: "set_camera with wrong length center",
			call: &genai.FunctionCall{
				Name: "set_camera",
				Args: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0}, // Only 2 elements
					"look_at":  []interface{}{1.0, 1.0, 1.0},
					"vfov":     45.0,
					"aperture": 0.0,
				},
			},
			expectError: true,
			errorMsg:    "camera center must have exactly 3 values",
		},
		{
			name: "set_camera with center == look_at",
			call: &genai.FunctionCall{
				Name: "set_camera",
				Args: map[string]interface{}{
					"center":   []interface{}{1.0, 2.0, 3.0},
					"look_at":  []interface{}{1.0, 2.0, 3.0},
					"vfov":     45.0,
					"aperture": 0.0,
				},
			},
			expectError: true,
			errorMsg:    "cannot be the same point",
		},
		{
			name: "set_camera with vfov out of range (too high)",
			call: &genai.FunctionCall{
				Name: "set_camera",
				Args: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 5.0},
					"look_at":  []interface{}{0.0, 0.0, 0.0},
					"vfov":     180.0,
					"aperture": 0.0,
				},
			},
			expectError: true,
			errorMsg:    "vfov must be in range",
		},
		{
			name: "set_camera with negative aperture",
			call: &genai.FunctionCall{
				Name: "set_camera",
				Args: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 5.0},
					"look_at":  []interface{}{0.0, 0.0, 0.0},
					"vfov":     45.0,
					"aperture": -0.5,
				},
			},
			expectError: true,
			errorMsg:    "aperture must be in range",
		},
		{
			name: "create_light with empty ID",
			call: &genai.FunctionCall{
				Name: "create_light",
				Args: map[string]interface{}{
					"id":   "",
					"type": "point_spot_light",
					"properties": map[string]interface{}{
						"center":    []interface{}{0.0, 1.0, 0.0},
						"direction": []interface{}{0.0, -1.0, 0.0},
						"emission":  []interface{}{1.0, 1.0, 1.0},
					},
				},
			},
			expectError: true,
			errorMsg:    "light ID cannot be empty",
		},
		{
			name: "point_spot_light with negative emission",
			call: &genai.FunctionCall{
				Name: "create_light",
				Args: map[string]interface{}{
					"id":   "test_light",
					"type": "point_spot_light",
					"properties": map[string]interface{}{
						"center":    []interface{}{0.0, 1.0, 0.0},
						"direction": []interface{}{0.0, -1.0, 0.0},
						"emission":  []interface{}{-1.0, 1.0, 1.0},
					},
				},
			},
			expectError: true,
			errorMsg:    "emission[0] must be >= 0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the operation
			op := parseToolRequestFromFunctionCall(tt.call)
			if op == nil {
				t.Fatal("Expected operation to be non-nil")
			}

			// Execute based on operation type
			var err error
			switch typedOp := op.(type) {
			case *CreateShapeRequest:
				err = sm.AddShapes([]ShapeRequest{typedOp.Shape})
			case *UpdateShapeRequest:
				err = sm.UpdateShape(typedOp.Id, typedOp.Updates)
			case *RemoveShapeRequest:
				err = sm.RemoveShape(typedOp.Id)
			case *SetCameraRequest:
				err = sm.SetCamera(typedOp.Camera)
			case *CreateLightRequest:
				// AddTypedLights handles parsing, validation, and adding (matching agent.go behavior)
				err = sm.AddTypedLights([]LightRequest{typedOp.Light})
			default:
				t.Fatalf("Unexpected operation type: %T", op)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got no error", tt.errorMsg)
				} else if err.Error() != tt.errorMsg && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
