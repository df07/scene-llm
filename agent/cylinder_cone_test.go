package agent

import (
	"strings"
	"testing"
)

func TestCylinderValidation(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name        string
		cylinder    ShapeRequest
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid cylinder capped",
			cylinder: ShapeRequest{
				ID:   "test_cylinder",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"radius":      0.5,
					"capped":      true,
				},
			},
			shouldError: false,
		},
		{
			name: "valid cylinder uncapped",
			cylinder: ShapeRequest{
				ID:   "test_cylinder_uncapped",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"base_center": []interface{}{1.0, 1.0, 1.0},
					"top_center":  []interface{}{2.0, 3.0, 4.0},
					"radius":      1.5,
					"capped":      false,
				},
			},
			shouldError: false,
		},
		{
			name: "cylinder missing base_center",
			cylinder: ShapeRequest{
				ID:   "bad_cylinder",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"top_center": []interface{}{0.0, 2.0, 0.0},
					"radius":     0.5,
					"capped":     true,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'base_center' property",
		},
		{
			name: "cylinder missing top_center",
			cylinder: ShapeRequest{
				ID:   "bad_cylinder",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"radius":      0.5,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'top_center' property",
		},
		{
			name: "cylinder missing radius",
			cylinder: ShapeRequest{
				ID:   "bad_cylinder",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'radius' property",
		},
		{
			name: "cylinder missing capped",
			cylinder: ShapeRequest{
				ID:   "bad_cylinder",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"radius":      0.5,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'capped' property",
		},
		{
			name: "cylinder with negative radius",
			cylinder: ShapeRequest{
				ID:   "bad_cylinder",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"radius":      -0.5,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "must be positive",
		},
		{
			name: "cylinder with invalid capped type",
			cylinder: ShapeRequest{
				ID:   "bad_cylinder",
				Type: "cylinder",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"radius":      0.5,
					"capped":      "yes", // Should be bool
				},
			},
			shouldError: true,
			errorMsg:    "must be a boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.AddShapes([]ShapeRequest{tt.cylinder})

			if tt.shouldError && err == nil {
				t.Errorf("Expected error, but got none")
			} else if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if tt.shouldError && err != nil {
				// Check that error message contains expected text
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			}
		})
	}
}

func TestConeValidation(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name        string
		cone        ShapeRequest
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid pointed cone",
			cone: ShapeRequest{
				ID:   "test_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 1.0,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  0.0,
					"capped":      true,
				},
			},
			shouldError: false,
		},
		{
			name: "valid frustum cone",
			cone: ShapeRequest{
				ID:   "test_frustum",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{1.0, 1.0, 1.0},
					"base_radius": 1.5,
					"top_center":  []interface{}{1.0, 3.0, 1.0},
					"top_radius":  0.5,
					"capped":      true,
				},
			},
			shouldError: false,
		},
		{
			name: "valid uncapped frustum",
			cone: ShapeRequest{
				ID:   "test_uncapped_frustum",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 2.0,
					"top_center":  []interface{}{0.0, 5.0, 0.0},
					"top_radius":  1.0,
					"capped":      false,
				},
			},
			shouldError: false,
		},
		{
			name: "cone missing base_center",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_radius": 1.0,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  0.0,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'base_center' property",
		},
		{
			name: "cone missing top_center",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 1.0,
					"top_radius":  0.0,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'top_center' property",
		},
		{
			name: "cone missing base_radius",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  0.0,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'base_radius' property",
		},
		{
			name: "cone missing top_radius",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 1.0,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'top_radius' property",
		},
		{
			name: "cone missing capped",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 1.0,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  0.0,
				},
			},
			shouldError: true,
			errorMsg:    "requires 'capped' property",
		},
		{
			name: "cone with negative base_radius",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": -1.0,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  0.0,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "must be positive",
		},
		{
			name: "cone with negative top_radius",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 1.0,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  -0.5,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "must be non-negative",
		},
		{
			name: "cone with base_radius <= top_radius",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 0.5,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  1.0,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "base_radius",
		},
		{
			name: "cone with base_radius equal to top_radius",
			cone: ShapeRequest{
				ID:   "bad_cone",
				Type: "cone",
				Properties: map[string]interface{}{
					"base_center": []interface{}{0.0, 0.0, 0.0},
					"base_radius": 1.0,
					"top_center":  []interface{}{0.0, 2.0, 0.0},
					"top_radius":  1.0,
					"capped":      true,
				},
			},
			shouldError: true,
			errorMsg:    "base_radius",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.AddShapes([]ShapeRequest{tt.cone})

			if tt.shouldError && err == nil {
				t.Errorf("Expected error, but got none")
			} else if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if tt.shouldError && err != nil {
				// Check that error message contains expected text
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			}
		})
	}
}

func TestCylinderAndConeSceneConversion(t *testing.T) {
	sm := NewSceneManager()

	// Add a cylinder and a cone
	shapes := []ShapeRequest{
		{
			ID:   "test_cylinder",
			Type: "cylinder",
			Properties: map[string]interface{}{
				"base_center": []interface{}{0.0, 0.0, 0.0},
				"top_center":  []interface{}{0.0, 2.0, 0.0},
				"radius":      0.5,
				"capped":      true,
				"material": map[string]interface{}{
					"type":   "lambertian",
					"albedo": []interface{}{0.8, 0.2, 0.2},
				},
			},
		},
		{
			ID:   "test_cone",
			Type: "cone",
			Properties: map[string]interface{}{
				"base_center": []interface{}{2.0, 0.0, 0.0},
				"base_radius": 1.0,
				"top_center":  []interface{}{2.0, 3.0, 0.0},
				"top_radius":  0.0,
				"capped":      true,
				"material": map[string]interface{}{
					"type":   "metal",
					"albedo": []interface{}{0.9, 0.9, 0.9},
					"fuzz":   0.1,
				},
			},
		},
	}

	err := sm.AddShapes(shapes)
	if err != nil {
		t.Fatalf("AddShapes() returned error: %v", err)
	}

	// Try to convert to raytracer scene
	raytracerScene, err := sm.ToRaytracerScene()
	if err != nil {
		t.Fatalf("ToRaytracerScene() returned error: %v", err)
	}

	if raytracerScene == nil {
		t.Fatal("ToRaytracerScene() returned nil scene")
	}

	// Check that both shapes were added
	if len(raytracerScene.Shapes) != 2 {
		t.Errorf("Expected 2 shapes in raytracer scene, got %d", len(raytracerScene.Shapes))
	}
}
