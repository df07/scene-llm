package agent

import (
	"testing"

	"google.golang.org/genai"
)

func TestSetEnvironmentLighting(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name         string
		lightingType string
		topColor     []float64
		bottomColor  []float64
		emission     []float64
		shouldError  bool
	}{
		{
			name:         "gradient lighting",
			lightingType: "gradient",
			topColor:     []float64{0.5, 0.7, 1.0},
			bottomColor:  []float64{1.0, 1.0, 1.0},
			shouldError:  false,
		},
		{
			name:         "uniform lighting",
			lightingType: "uniform",
			emission:     []float64{0.8, 0.8, 0.8},
			shouldError:  false,
		},
		{
			name:         "no lighting",
			lightingType: "none",
			shouldError:  false,
		},
		{
			name:         "invalid lighting type",
			lightingType: "invalid",
			shouldError:  true,
		},
		{
			name:         "gradient missing bottom color",
			lightingType: "gradient",
			topColor:     []float64{0.5, 0.7, 1.0},
			shouldError:  true,
		},
		{
			name:         "uniform missing emission",
			lightingType: "uniform",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear lights before each test
			sm.removeEnvironmentLights()

			err := sm.SetEnvironmentLighting(tt.lightingType, tt.topColor, tt.bottomColor, tt.emission)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.name, err)
				}

				// Verify lights were added correctly
				switch tt.lightingType {
				case "gradient":
					if len(sm.state.Lights) != 1 {
						t.Errorf("Expected 1 light for gradient, got %d", len(sm.state.Lights))
					}
					if sm.state.Lights[0].Type != "infinite_gradient_light" {
						t.Errorf("Expected infinite_gradient_light, got %s", sm.state.Lights[0].Type)
					}
				case "uniform":
					if len(sm.state.Lights) != 1 {
						t.Errorf("Expected 1 light for uniform, got %d", len(sm.state.Lights))
					}
					if sm.state.Lights[0].Type != "infinite_uniform_light" {
						t.Errorf("Expected infinite_uniform_light, got %s", sm.state.Lights[0].Type)
					}
				case "none":
					if len(sm.state.Lights) != 0 {
						t.Errorf("Expected 0 lights for none, got %d", len(sm.state.Lights))
					}
				}
			}
		})
	}
}

func TestSetEnvironmentLightingToolCall(t *testing.T) {
	// Test that tool call parsing works correctly
	functionCall := &genai.FunctionCall{
		Name: "set_environment_lighting",
		Args: map[string]interface{}{
			"type":         "gradient",
			"top_color":    []interface{}{0.5, 0.7, 1.0},
			"bottom_color": []interface{}{1.0, 1.0, 1.0},
		},
	}

	operation := parseSetEnvironmentLightingOperation(functionCall)
	if operation == nil {
		t.Fatal("Failed to parse set_environment_lighting operation")
	}

	if operation.LightingType != "gradient" {
		t.Errorf("Expected lighting type 'gradient', got '%s'", operation.LightingType)
	}

	if len(operation.TopColor) != 3 {
		t.Errorf("Expected top_color length 3, got %d", len(operation.TopColor))
	}

	if len(operation.BottomColor) != 3 {
		t.Errorf("Expected bottom_color length 3, got %d", len(operation.BottomColor))
	}

	// Test execution
	sm := NewSceneManager()
	err := sm.SetEnvironmentLighting(operation.LightingType, operation.TopColor, operation.BottomColor, operation.Emission)
	if err != nil {
		t.Errorf("Failed to execute environment lighting operation: %v", err)
	}

	// Verify scene conversion works
	scene, err := sm.ToRaytracerScene()
	if err != nil {
		t.Errorf("Failed to convert scene: %v", err)
	}
	if scene == nil {
		t.Error("Scene conversion returned nil")
	}
}

func TestSetEnvironmentLightingToolParsing(t *testing.T) {
	tests := []struct {
		name         string
		functionCall *genai.FunctionCall
		expectNil    bool
		expectType   string
	}{
		{
			name: "uniform lighting",
			functionCall: &genai.FunctionCall{
				Name: "set_environment_lighting",
				Args: map[string]interface{}{
					"type":     "uniform",
					"emission": []interface{}{0.8, 0.8, 0.8},
				},
			},
			expectNil:  false,
			expectType: "uniform",
		},
		{
			name: "none lighting",
			functionCall: &genai.FunctionCall{
				Name: "set_environment_lighting",
				Args: map[string]interface{}{
					"type": "none",
				},
			},
			expectNil:  false,
			expectType: "none",
		},
		{
			name: "wrong function name",
			functionCall: &genai.FunctionCall{
				Name: "wrong_function",
				Args: map[string]interface{}{
					"type": "gradient",
				},
			},
			expectNil: true,
		},
		{
			name: "malformed color arrays",
			functionCall: &genai.FunctionCall{
				Name: "set_environment_lighting",
				Args: map[string]interface{}{
					"type":         "gradient",
					"top_color":    []interface{}{"not", "a", "number"},
					"bottom_color": []interface{}{1.0, 1.0, 1.0},
				},
			},
			expectNil:  false,
			expectType: "gradient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation := parseSetEnvironmentLightingOperation(tt.functionCall)

			if tt.expectNil {
				if operation != nil {
					t.Errorf("Expected nil operation, got %+v", operation)
				}
			} else {
				if operation == nil {
					t.Fatal("Expected operation, got nil")
				}
				if operation.LightingType != tt.expectType {
					t.Errorf("Expected type %s, got %s", tt.expectType, operation.LightingType)
				}
			}
		})
	}
}

func TestLightReplacement(t *testing.T) {
	sm := NewSceneManager()

	// Add gradient lighting
	err := sm.SetEnvironmentLighting("gradient", []float64{1.0, 0.5, 0.0}, []float64{0.0, 0.5, 1.0}, nil)
	if err != nil {
		t.Fatalf("Failed to set gradient lighting: %v", err)
	}

	if len(sm.state.Lights) != 1 {
		t.Errorf("Expected 1 light after gradient, got %d", len(sm.state.Lights))
	}

	// Replace with uniform lighting
	err = sm.SetEnvironmentLighting("uniform", nil, nil, []float64{0.9, 0.9, 0.9})
	if err != nil {
		t.Fatalf("Failed to set uniform lighting: %v", err)
	}

	if len(sm.state.Lights) != 1 {
		t.Errorf("Expected 1 light after uniform replacement, got %d", len(sm.state.Lights))
	}

	if sm.state.Lights[0].Type != "infinite_uniform_light" {
		t.Errorf("Expected uniform light type, got %s", sm.state.Lights[0].Type)
	}

	// Remove all lighting
	err = sm.SetEnvironmentLighting("none", nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to remove lighting: %v", err)
	}

	if len(sm.state.Lights) != 0 {
		t.Errorf("Expected 0 lights after removal, got %d", len(sm.state.Lights))
	}
}

func TestSceneConversionWithLights(t *testing.T) {
	tests := []struct {
		name         string
		lightingType string
		topColor     []float64
		bottomColor  []float64
		emission     []float64
	}{
		{
			name:         "gradient lighting",
			lightingType: "gradient",
			topColor:     []float64{1.0, 0.8, 0.6},
			bottomColor:  []float64{0.2, 0.4, 0.8},
		},
		{
			name:         "uniform lighting",
			lightingType: "uniform",
			emission:     []float64{0.7, 0.7, 0.7},
		},
		{
			name:         "no lighting (default fallback)",
			lightingType: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSceneManager()

			// Set lighting
			err := sm.SetEnvironmentLighting(tt.lightingType, tt.topColor, tt.bottomColor, tt.emission)
			if err != nil {
				t.Fatalf("Failed to set lighting: %v", err)
			}

			// Convert to raytracer scene
			scene, err := sm.ToRaytracerScene()
			if err != nil {
				t.Fatalf("Failed to convert scene: %v", err)
			}

			if scene == nil {
				t.Fatal("Scene conversion returned nil")
			}

			// Verify scene has proper structure
			if scene.Camera == nil {
				t.Error("Scene missing camera")
			}
		})
	}
}

func TestEnvironmentLightingValidation(t *testing.T) {
	sm := NewSceneManager()

	// Test negative color values
	err := sm.SetEnvironmentLighting("gradient", []float64{-1.0, 0.5, 1.0}, []float64{1.0, 1.0, 1.0}, nil)
	if err == nil {
		t.Error("Expected error for negative color values")
	}

	// Test wrong array length
	err = sm.SetEnvironmentLighting("gradient", []float64{1.0, 0.5}, []float64{1.0, 1.0, 1.0}, nil)
	if err == nil {
		t.Error("Expected error for wrong array length")
	}

	// Test nil arrays where required
	err = sm.SetEnvironmentLighting("gradient", nil, []float64{1.0, 1.0, 1.0}, nil)
	if err == nil {
		t.Error("Expected error for missing top_color")
	}

	err = sm.SetEnvironmentLighting("uniform", nil, nil, nil)
	if err == nil {
		t.Error("Expected error for missing emission")
	}
}
