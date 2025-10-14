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

	operation := parseSetEnvironmentLightingRequest(functionCall)
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
			expectNil:  false,
			expectType: "gradient", // Parser doesn't check function name, just extracts args
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
			operation := parseSetEnvironmentLightingRequest(tt.functionCall)

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
	err := sm.SetEnvironmentLighting("gradient", []float64{1.0, 0.5, 0.0}, []float64{0.0, 0.5, 1.0}, []float64{0.0, 0.0, 0.0})
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

// Tests for positioned lights

func TestAddLights(t *testing.T) {
	sm := NewSceneManager()

	// Test valid point spot light
	pointLight := LightRequest{
		ID:   "main_light",
		Type: "point_spot_light",
		Properties: map[string]interface{}{
			"center":    []interface{}{0.0, 2.0, 0.0},
			"direction": []interface{}{0.0, -1.0, 0.0},
			"emission":  []interface{}{1.0, 1.0, 1.0},
		},
	}

	err := sm.AddTypedLights([]LightRequest{pointLight})
	if err != nil {
		t.Errorf("Failed to add valid point light: %v", err)
	}

	if len(sm.state.Lights) != 1 {
		t.Errorf("Expected 1 light, got %d", len(sm.state.Lights))
	}

	// Test duplicate ID error
	err = sm.AddTypedLights([]LightRequest{pointLight})
	if err == nil {
		t.Error("Expected error for duplicate light ID")
	}

	// Test empty light list
	err = sm.AddTypedLights([]LightRequest{})
	if err != nil {
		t.Errorf("Empty light list should not error: %v", err)
	}
}

func TestFindLight(t *testing.T) {
	sm := NewSceneManager()

	// Add a test light
	testLight := LightRequest{
		ID:   "test_light",
		Type: "area_sphere_light",
		Properties: map[string]interface{}{
			"center":   []interface{}{1.0, 1.0, 1.0},
			"radius":   2.0,
			"emission": []interface{}{0.8, 0.6, 0.4},
		},
	}

	sm.AddTypedLights([]LightRequest{testLight})

	// Test finding existing light
	found := sm.FindLight("test_light")
	if found == nil {
		t.Error("Failed to find existing light")
	} else if found.ID != "test_light" {
		t.Errorf("Found wrong light: %s", found.ID)
	}

	// Test finding non-existent light
	notFound := sm.FindLight("nonexistent")
	if notFound != nil {
		t.Error("Found non-existent light")
	}
}

func TestUpdateLight(t *testing.T) {
	sm := NewSceneManager()

	// Add a test light
	testLight := LightRequest{
		ID:   "test_light",
		Type: "point_spot_light",
		Properties: map[string]interface{}{
			"center":    []interface{}{0.0, 0.0, 0.0},
			"direction": []interface{}{0.0, -1.0, 0.0},
			"emission":  []interface{}{1.0, 1.0, 1.0},
		},
	}
	sm.AddTypedLights([]LightRequest{testLight})

	// Test updating properties
	updates := map[string]interface{}{
		"properties": map[string]interface{}{
			"emission": []interface{}{2.0, 1.0, 0.5},
		},
	}

	err := sm.UpdateLight("test_light", updates)
	if err != nil {
		t.Errorf("Failed to update light properties: %v", err)
	}

	// Verify the update
	updated := sm.FindLight("test_light")
	if updated == nil {
		t.Error("Light disappeared after update")
	}

	// Test updating ID
	idUpdate := map[string]interface{}{
		"id": "renamed_light",
	}
	err = sm.UpdateLight("test_light", idUpdate)
	if err != nil {
		t.Errorf("Failed to rename light: %v", err)
	}

	// Verify old ID is gone and new ID exists
	if sm.FindLight("test_light") != nil {
		t.Error("Old light ID still exists after rename")
	}
	if sm.FindLight("renamed_light") == nil {
		t.Error("New light ID not found after rename")
	}

	// Test updating non-existent light
	err = sm.UpdateLight("nonexistent", updates)
	if err == nil {
		t.Error("Expected error updating non-existent light")
	}
}

func TestRemoveLight(t *testing.T) {
	sm := NewSceneManager()

	// Add test lights
	lights := []LightRequest{
		{
			ID:   "light1",
			Type: "point_spot_light",
			Properties: map[string]interface{}{
				"center":    []interface{}{0.0, 0.0, 0.0},
				"direction": []interface{}{0.0, -1.0, 0.0},
				"emission":  []interface{}{1.0, 1.0, 1.0},
			},
		},
		{
			ID:   "light2",
			Type: "area_sphere_light",
			Properties: map[string]interface{}{
				"center":   []interface{}{1.0, 1.0, 1.0},
				"radius":   1.0,
				"emission": []interface{}{0.5, 0.5, 0.5},
			},
		},
	}
	sm.AddTypedLights(lights)

	if len(sm.state.Lights) != 2 {
		t.Errorf("Expected 2 lights, got %d", len(sm.state.Lights))
	}

	// Test removing existing light
	err := sm.RemoveLight("light1")
	if err != nil {
		t.Errorf("Failed to remove light: %v", err)
	}

	if len(sm.state.Lights) != 1 {
		t.Errorf("Expected 1 light after removal, got %d", len(sm.state.Lights))
	}

	if sm.FindLight("light1") != nil {
		t.Error("Removed light still exists")
	}

	// Test removing non-existent light
	err = sm.RemoveLight("nonexistent")
	if err == nil {
		t.Error("Expected error removing non-existent light")
	}
}

func TestValidateLightProperties(t *testing.T) {
	tests := []struct {
		name        string
		light       LightRequest
		shouldError bool
	}{
		{
			name: "valid point spot light",
			light: LightRequest{
				ID:   "test_point",
				Type: "point_spot_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 2.0, 0.0},
					"emission": []interface{}{1.0, 1.0, 1.0},
				},
			},
			shouldError: false,
		},
		{
			name: "valid area quad light",
			light: LightRequest{
				ID:   "test_quad",
				Type: "area_quad_light",
				Properties: map[string]interface{}{
					"corner":   []interface{}{-1.0, 2.0, -1.0},
					"u":        []interface{}{2.0, 0.0, 0.0},
					"v":        []interface{}{0.0, 0.0, 2.0},
					"emission": []interface{}{0.8, 0.8, 0.8},
				},
			},
			shouldError: false,
		},
		{
			name: "valid area disc light",
			light: LightRequest{
				ID:   "test_disc",
				Type: "disc_spot_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 3.0, 0.0},
					"normal":   []interface{}{0.0, -1.0, 0.0},
					"radius":   1.5,
					"emission": []interface{}{1.0, 0.8, 0.6},
				},
			},
			shouldError: false,
		},
		{
			name: "valid area sphere light",
			light: LightRequest{
				ID:   "test_sphere",
				Type: "area_sphere_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 4.0, 0.0},
					"radius":   0.5,
					"emission": []interface{}{2.0, 2.0, 2.0},
				},
			},
			shouldError: false,
		},
		{
			name: "valid area disc spot light",
			light: LightRequest{
				ID:   "test_disc_spot",
				Type: "area_disc_spot_light",
				Properties: map[string]interface{}{
					"center":           []interface{}{0.0, 3.0, 0.0},
					"normal":           []interface{}{0.0, -1.0, 0.0},
					"radius":           0.3,
					"emission":         []interface{}{3.0, 3.0, 3.0},
					"cutoff_angle":     30.0,
					"falloff_exponent": 2.0,
				},
			},
			shouldError: false,
		},
		{
			name: "empty ID",
			light: LightRequest{
				ID:   "",
				Type: "point_spot_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 0.0},
					"emission": []interface{}{1.0, 1.0, 1.0},
				},
			},
			shouldError: true,
		},
		{
			name: "missing required property",
			light: LightRequest{
				ID:   "test_invalid",
				Type: "point_spot_light",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					// Missing emission and direction
				},
			},
			shouldError: true,
		},
		{
			name: "point_spot_light missing direction",
			light: LightRequest{
				ID:   "test_no_direction",
				Type: "point_spot_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 0.0},
					"emission": []interface{}{1.0, 1.0, 1.0},
					// Missing direction - should error with AddTypedLights
				},
			},
			shouldError: true,
		},
		{
			name: "invalid cutoff angle",
			light: LightRequest{
				ID:   "test_invalid_angle",
				Type: "point_spot_light",
				Properties: map[string]interface{}{
					"center":       []interface{}{0.0, 0.0, 0.0},
					"emission":     []interface{}{1.0, 1.0, 1.0},
					"cutoff_angle": 200.0, // Invalid: > 180
				},
			},
			shouldError: true,
		},
		{
			name: "negative radius",
			light: LightRequest{
				ID:   "test_negative_radius",
				Type: "area_sphere_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 0.0},
					"radius":   -1.0,
					"emission": []interface{}{1.0, 1.0, 1.0},
				},
			},
			shouldError: true,
		},
		{
			name: "unsupported light type",
			light: LightRequest{
				ID:   "test_unsupported",
				Type: "invalid_light_type",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 0.0},
					"emission": []interface{}{1.0, 1.0, 1.0},
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Special case: "point_spot_light missing direction" tests AddTypedLights
			// which has stricter validation than validateLightProperties
			if tt.name == "point_spot_light missing direction" {
				sm := NewSceneManager()
				err := sm.AddTypedLights([]LightRequest{tt.light})
				if tt.shouldError && err == nil {
					t.Errorf("Expected error for %s, but got none", tt.name)
				}
				if !tt.shouldError && err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.name, err)
				}
				return
			}

			err := validateLightProperties(tt.light)
			if tt.shouldError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.name, err)
			}
		})
	}
}

func TestLightToolParsing(t *testing.T) {
	// Test create_light parsing
	createCall := &genai.FunctionCall{
		Name: "create_light",
		Args: map[string]interface{}{
			"id":   "test_light",
			"type": "point_spot_light",
			"properties": map[string]interface{}{
				"center":    []interface{}{0.0, 2.0, 0.0},
				"direction": []interface{}{0.0, -1.0, 0.0},
				"emission":  []interface{}{1.0, 1.0, 1.0},
			},
		},
	}

	createOp := parseCreateLightRequest(createCall)
	if createOp == nil {
		t.Fatal("Failed to parse create_light operation")
	}
	if createOp.Light.ID != "test_light" {
		t.Errorf("Expected light ID 'test_light', got '%s'", createOp.Light.ID)
	}
	if createOp.Light.Type != "point_spot_light" {
		t.Errorf("Expected light type 'point_spot_light', got '%s'", createOp.Light.Type)
	}

	// Test update_light parsing
	updateCall := &genai.FunctionCall{
		Name: "update_light",
		Args: map[string]interface{}{
			"id": "test_light",
			"updates": map[string]interface{}{
				"properties": map[string]interface{}{
					"emission": []interface{}{2.0, 1.0, 0.5},
				},
			},
		},
	}

	updateOp := parseUpdateLightRequest(updateCall)
	if updateOp == nil {
		t.Fatal("Failed to parse update_light operation")
	}
	if updateOp.Id != "test_light" {
		t.Errorf("Expected update ID 'test_light', got '%s'", updateOp.Id)
	}

	// Test remove_light parsing
	removeCall := &genai.FunctionCall{
		Name: "remove_light",
		Args: map[string]interface{}{
			"id": "test_light",
		},
	}

	removeOp := parseRemoveLightRequest(removeCall)
	if removeOp == nil {
		t.Fatal("Failed to parse remove_light operation")
	}
	if removeOp.Id != "test_light" {
		t.Errorf("Expected remove ID 'test_light', got '%s'", removeOp.Id)
	}

	// Note: Parsers no longer check function names (redundant since caller checks)
	// They will parse any args passed to them
}

func TestSceneConversionWithPositionedLights(t *testing.T) {
	tests := []struct {
		name      string
		lightType string
		light     LightRequest
	}{
		{
			name:      "point spot light",
			lightType: "point_spot_light",
			light: LightRequest{
				ID:   "test_point",
				Type: "point_spot_light",
				Properties: map[string]interface{}{
					"center":    []interface{}{0.0, 2.0, 0.0},
					"direction": []interface{}{0.0, -1.0, 0.0},
					"emission":  []interface{}{1.0, 1.0, 1.0},
				},
			},
		},
		{
			name:      "area quad light",
			lightType: "area_quad_light",
			light: LightRequest{
				ID:   "test_quad",
				Type: "area_quad_light",
				Properties: map[string]interface{}{
					"corner":   []interface{}{-1.0, 2.0, -1.0},
					"u":        []interface{}{2.0, 0.0, 0.0},
					"v":        []interface{}{0.0, 0.0, 2.0},
					"emission": []interface{}{0.8, 0.8, 0.8},
				},
			},
		},
		{
			name:      "area disc light",
			lightType: "disc_spot_light",
			light: LightRequest{
				ID:   "test_disc",
				Type: "disc_spot_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 3.0, 0.0},
					"normal":   []interface{}{0.0, -1.0, 0.0},
					"radius":   1.5,
					"emission": []interface{}{1.0, 0.8, 0.6},
				},
			},
		},
		{
			name:      "area sphere light",
			lightType: "area_sphere_light",
			light: LightRequest{
				ID:   "test_sphere",
				Type: "area_sphere_light",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 4.0, 0.0},
					"radius":   0.5,
					"emission": []interface{}{2.0, 2.0, 2.0},
				},
			},
		},
		{
			name:      "area disc spot light",
			lightType: "area_disc_spot_light",
			light: LightRequest{
				ID:   "test_disc_spot",
				Type: "area_disc_spot_light",
				Properties: map[string]interface{}{
					"center":           []interface{}{0.0, 3.0, 0.0},
					"normal":           []interface{}{0.0, -1.0, 0.0},
					"radius":           0.3,
					"emission":         []interface{}{3.0, 3.0, 3.0},
					"cutoff_angle":     30.0,
					"falloff_exponent": 2.0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSceneManager()

			// Add the light
			err := sm.AddTypedLights([]LightRequest{tt.light})
			if err != nil {
				t.Fatalf("Failed to add %s: %v", tt.lightType, err)
			}

			// Convert to raytracer scene
			scene, err := sm.ToRaytracerScene()
			if err != nil {
				t.Fatalf("Failed to convert scene with %s: %v", tt.lightType, err)
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
