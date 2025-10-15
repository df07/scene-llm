package agent

import (
	"regexp"
	"strings"
	"testing"
)

// Helper function to compare CameraInfo structs (since slices can't be compared with ==)
func cameraEqual(a, b CameraInfo) bool {
	if len(a.Center) != len(b.Center) || len(a.LookAt) != len(b.LookAt) {
		return false
	}
	for i := range a.Center {
		if a.Center[i] != b.Center[i] {
			return false
		}
	}
	for i := range a.LookAt {
		if a.LookAt[i] != b.LookAt[i] {
			return false
		}
	}
	return a.VFov == b.VFov && a.Aperture == b.Aperture
}

func TestNewSceneManager(t *testing.T) {
	sm := NewSceneManager()

	if sm == nil {
		t.Fatal("NewSceneManager() returned nil")
	}

	state := sm.GetState()
	if state == nil {
		t.Fatal("GetState() returned nil")
	}

	if len(state.Shapes) != 0 {
		t.Errorf("Expected empty scene, got %d shapes", len(state.Shapes))
	}

	expectedCamera := CameraInfo{
		Center:   []float64{0, 0, 5},
		LookAt:   []float64{0, 0, 0},
		VFov:     45.0,
		Aperture: 0.0,
	}
	if !cameraEqual(state.Camera, expectedCamera) {
		t.Errorf("Expected camera %+v, got %+v", expectedCamera, state.Camera)
	}
}

func TestAddShapes(t *testing.T) {
	sm := NewSceneManager()

	shapes := []ShapeRequest{
		{
			ID:   "red_sphere",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{1.0, 2.0, 3.0},
				"radius": 2.0,
				"color":  []interface{}{1.0, 0.0, 0.0},
			},
		},
		{
			ID:   "green_box",
			Type: "box",
			Properties: map[string]interface{}{
				"center":     []interface{}{4.0, 5.0, 6.0},
				"dimensions": []interface{}{1.5, 1.5, 1.5},
				"color":      []interface{}{0.0, 1.0, 0.0},
			},
		},
	}

	err := sm.AddShapes(shapes)
	if err != nil {
		t.Fatalf("AddShapes() returned error: %v", err)
	}

	state := sm.GetState()
	if len(state.Shapes) != 2 {
		t.Errorf("Expected 2 shapes, got %d", len(state.Shapes))
	}

	// Check that shapes were added correctly
	if state.Shapes[0].ID != "red_sphere" || state.Shapes[0].Type != "sphere" {
		t.Errorf("First shape not added correctly: %+v", state.Shapes[0])
	}
	if state.Shapes[1].ID != "green_box" || state.Shapes[1].Type != "box" {
		t.Errorf("Second shape not added correctly: %+v", state.Shapes[1])
	}
}

func TestAddShapesUpdatesCamera(t *testing.T) {
	sm := NewSceneManager()

	shape := ShapeRequest{
		ID:   "test_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{10.0, 20.0, 30.0},
			"radius": 5.0,
			"color":  []interface{}{1.0, 0.0, 0.0},
		},
	}

	err := sm.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("AddShapes() returned error: %v", err)
	}

	state := sm.GetState()

	// Camera should be positioned relative to the first shape
	// expectedDistance := radius*3 + 5 = 5.0*3 + 5 = 20
	expectedCamera := CameraInfo{
		Center:   []float64{10, 20, 50}, // shape position Z + 20
		LookAt:   []float64{10, 20, 30}, // shape position
		VFov:     45.0,
		Aperture: 0.0,
	}

	if !cameraEqual(state.Camera, expectedCamera) {
		t.Errorf("Expected camera %+v, got %+v", expectedCamera, state.Camera)
	}
}

func TestAddEmptyShapes(t *testing.T) {
	sm := NewSceneManager()

	err := sm.AddShapes([]ShapeRequest{})
	if err != nil {
		t.Errorf("AddShapes() with empty slice returned error: %v", err)
	}

	if sm.GetShapeCount() != 0 {
		t.Errorf("Expected 0 shapes after adding empty slice, got %d", sm.GetShapeCount())
	}
}

func TestBuildContextEmptyScene(t *testing.T) {
	sm := NewSceneManager()

	context := sm.BuildContext()
	expected := "Current scene state: empty scene with no objects."

	if context != expected {
		t.Errorf("Expected context '%s', got '%s'", expected, context)
	}
}

func TestBuildContextWithShapes(t *testing.T) {
	sm := NewSceneManager()

	shapes := []ShapeRequest{
		{
			ID:   "test_sphere",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{1.5, 2.7, 3.2},
				"radius": 2.5,
				"color":  []interface{}{0.8, 0.2, 0.4},
			},
		},
		{
			ID:   "test_box",
			Type: "box",
			Properties: map[string]interface{}{
				"center":     []interface{}{-1.0, 0.0, 1.0},
				"dimensions": []interface{}{1.0, 1.0, 1.0},
				"color":      []interface{}{0.0, 1.0, 0.5},
			},
		},
	}

	sm.AddShapes(shapes)
	context := sm.BuildContext()

	// Check that context contains expected elements
	if !strings.Contains(context, "2 shapes") {
		t.Errorf("Context should contain '2 shapes', got: %s", context)
	}

	if !strings.Contains(context, "sphere (ID: test_sphere) at [1.5,2.7,3.2]") {
		t.Errorf("Context should contain sphere info with ID, got: %s", context)
	}

	if !strings.Contains(context, "box (ID: test_box) at [-1.0,0.0,1.0]") {
		t.Errorf("Context should contain box info with ID, got: %s", context)
	}

	if !strings.Contains(context, "size 2.5") {
		t.Errorf("Context should contain size info, got: %s", context)
	}
}

func TestClearScene(t *testing.T) {
	sm := NewSceneManager()

	// Add some shapes first
	shapes := []ShapeRequest{
		{
			ID:   "test_sphere",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{1.0, 2.0, 3.0},
				"radius": 1.0,
				"color":  []interface{}{1.0, 0.0, 0.0},
			},
		},
	}
	sm.AddShapes(shapes)

	if sm.GetShapeCount() != 1 {
		t.Errorf("Expected 1 shape before clear, got %d", sm.GetShapeCount())
	}

	// Clear the scene
	sm.ClearScene()

	if sm.GetShapeCount() != 0 {
		t.Errorf("Expected 0 shapes after clear, got %d", sm.GetShapeCount())
	}

	// Check camera reset to default
	state := sm.GetState()
	expectedCamera := CameraInfo{
		Center:   []float64{0, 0, 5},
		LookAt:   []float64{0, 0, 0},
		VFov:     45.0,
		Aperture: 0.0,
	}
	if !cameraEqual(state.Camera, expectedCamera) {
		t.Errorf("Expected camera reset to %+v, got %+v", expectedCamera, state.Camera)
	}
}

func TestGetStateReturnsImmutableCopy(t *testing.T) {
	sm := NewSceneManager()

	// Add a shape
	shape := ShapeRequest{
		ID:   "test_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{1.0, 2.0, 3.0},
			"radius": 1.0,
			"color":  []interface{}{1.0, 0.0, 0.0},
		},
	}
	sm.AddShapes([]ShapeRequest{shape})

	// Get state twice
	state1 := sm.GetState()
	state2 := sm.GetState()

	// Modify first state's properties
	if state1.Shapes[0].Properties != nil {
		state1.Shapes[0].Properties["radius"] = 999.0
	}

	// Second state should be unaffected
	if radius, ok := extractFloat(state2.Shapes[0].Properties, "radius"); !ok || radius != 1.0 {
		t.Errorf("GetState() should return independent copies, but modification affected other copy")
	}

	// Original scene should be unaffected
	originalState := sm.GetState()
	if radius, ok := extractFloat(originalState.Shapes[0].Properties, "radius"); !ok || radius != 1.0 {
		t.Errorf("GetState() should return copies, but modification affected original scene")
	}
}

func TestGetShapeCount(t *testing.T) {
	sm := NewSceneManager()

	if sm.GetShapeCount() != 0 {
		t.Errorf("Expected 0 shapes initially, got %d", sm.GetShapeCount())
	}

	shapes := []ShapeRequest{
		{
			ID:   "sphere1",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{0.0, 0.0, 0.0},
				"radius": 1.0,
				"color":  []interface{}{1.0, 0.0, 0.0},
			},
		},
		{
			ID:   "box1",
			Type: "box",
			Properties: map[string]interface{}{
				"center":     []interface{}{1.0, 1.0, 1.0},
				"dimensions": []interface{}{2.0, 2.0, 2.0},
				"color":      []interface{}{0.0, 1.0, 0.0},
			},
		},
		{
			ID:   "sphere2",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{2.0, 2.0, 2.0},
				"radius": 1.5,
				"color":  []interface{}{0.0, 0.0, 1.0},
			},
		},
	}

	sm.AddShapes(shapes)

	if sm.GetShapeCount() != 3 {
		t.Errorf("Expected 3 shapes after adding, got %d", sm.GetShapeCount())
	}

	sm.ClearScene()

	if sm.GetShapeCount() != 0 {
		t.Errorf("Expected 0 shapes after clear, got %d", sm.GetShapeCount())
	}
}

// Tests for new transformation features

func TestFindShape(t *testing.T) {
	sm := NewSceneManager()

	shapes := []ShapeRequest{
		{
			ID:   "blue_sphere",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{0.0, 0.0, 0.0},
				"radius": 1.0,
				"color":  []interface{}{0.0, 0.0, 1.0},
			},
		},
		{
			ID:   "red_box",
			Type: "box",
			Properties: map[string]interface{}{
				"center":     []interface{}{1.0, 1.0, 1.0},
				"dimensions": []interface{}{2.0, 2.0, 2.0},
				"color":      []interface{}{1.0, 0.0, 0.0},
			},
		},
	}

	sm.AddShapes(shapes)

	// Test finding existing shapes
	foundSphere := sm.FindShape("blue_sphere")
	if foundSphere == nil {
		t.Error("Expected to find blue_sphere, got nil")
	} else if foundSphere.ID != "blue_sphere" || foundSphere.Type != "sphere" {
		t.Errorf("Found wrong shape: %+v", foundSphere)
	}

	foundBox := sm.FindShape("red_box")
	if foundBox == nil {
		t.Error("Expected to find red_box, got nil")
	} else if foundBox.ID != "red_box" || foundBox.Type != "box" {
		t.Errorf("Found wrong shape: %+v", foundBox)
	}

	// Test finding non-existent shape
	notFound := sm.FindShape("nonexistent")
	if notFound != nil {
		t.Errorf("Expected nil for non-existent shape, got %+v", notFound)
	}
}

func TestUpdateShape(t *testing.T) {
	sm := NewSceneManager()

	// Add initial shape
	shape := ShapeRequest{
		ID:   "blue_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{0.0, 0.0, 0.0},
			"radius": 1.0,
			"color":  []interface{}{0.0, 0.0, 1.0},
		},
	}
	sm.AddShapes([]ShapeRequest{shape})

	// Test updating color
	err := sm.UpdateShape("blue_sphere", map[string]interface{}{
		"properties": map[string]interface{}{
			"color": []interface{}{1.0, 0.0, 1.0}, // Change to purple
		},
	})
	if err != nil {
		t.Fatalf("UpdateShape failed: %v", err)
	}

	// Verify color was updated
	updated := sm.FindShape("blue_sphere")
	if updated == nil {
		t.Fatal("Shape disappeared after update")
	}
	if colorArray, ok := extractFloatArray(updated.Properties, "color", 3); !ok ||
		colorArray[0] != 1.0 || colorArray[1] != 0.0 || colorArray[2] != 1.0 {
		t.Errorf("Color not updated correctly: %+v", updated.Properties["color"])
	}

	// Test updating ID (renaming)
	err = sm.UpdateShape("blue_sphere", map[string]interface{}{
		"id": "purple_sphere",
	})
	if err != nil {
		t.Fatalf("UpdateShape ID change failed: %v", err)
	}

	// Verify ID was updated and old ID no longer exists
	if sm.FindShape("blue_sphere") != nil {
		t.Error("Old ID still exists after rename")
	}
	renamed := sm.FindShape("purple_sphere")
	if renamed == nil {
		t.Error("New ID not found after rename")
	}

	// Test updating non-existent shape
	err = sm.UpdateShape("nonexistent", map[string]interface{}{
		"properties": map[string]interface{}{
			"color": []interface{}{1.0, 1.0, 1.0},
		},
	})
	if err == nil {
		t.Error("Expected error when updating non-existent shape")
	}

	// Test ID conflict
	sm.AddShapes([]ShapeRequest{{
		ID:   "another_shape",
		Type: "box",
		Properties: map[string]interface{}{
			"center":     []interface{}{2.0, 2.0, 2.0},
			"dimensions": []interface{}{1.0, 1.0, 1.0},
		},
	}})

	err = sm.UpdateShape("purple_sphere", map[string]interface{}{
		"id": "another_shape", // Try to rename to existing ID
	})
	if err == nil {
		t.Error("Expected error when trying to rename to existing ID")
	}
}

func TestRemoveShape(t *testing.T) {
	// Set up initial shapes
	initialShapes := []ShapeRequest{
		{
			ID:   "shape1",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{0.0, 0.0, 0.0},
				"radius": 1.0,
			},
		},
		{
			ID:   "shape2",
			Type: "box",
			Properties: map[string]interface{}{
				"center":     []interface{}{1.0, 1.0, 1.0},
				"dimensions": []interface{}{1.0, 1.0, 1.0},
			},
		},
		{
			ID:   "shape3",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{2.0, 2.0, 2.0},
				"radius": 0.5,
			},
		},
	}

	tests := []struct {
		name             string
		removeID         string
		shouldError      bool
		expectedCount    int
		shouldStillExist []string
		shouldNotExist   []string
	}{
		{
			name:             "remove middle shape",
			removeID:         "shape2",
			shouldError:      false,
			expectedCount:    2,
			shouldStillExist: []string{"shape1", "shape3"},
			shouldNotExist:   []string{"shape2"},
		},
		{
			name:             "remove non-existent shape",
			removeID:         "nonexistent",
			shouldError:      true,
			expectedCount:    3, // Should remain unchanged
			shouldStillExist: []string{"shape1", "shape2", "shape3"},
			shouldNotExist:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSceneManager()
			sm.AddShapes(initialShapes)

			if sm.GetShapeCount() != 3 {
				t.Fatalf("Expected 3 shapes initially, got %d", sm.GetShapeCount())
			}

			err := sm.RemoveShape(tt.removeID)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
			} else if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.name, err)
			}

			if sm.GetShapeCount() != tt.expectedCount {
				t.Errorf("Expected %d shapes after %s, got %d", tt.expectedCount, tt.name, sm.GetShapeCount())
			}

			// Verify shapes that should still exist
			for _, id := range tt.shouldStillExist {
				if sm.FindShape(id) == nil {
					t.Errorf("Shape %s should still exist after %s", id, tt.name)
				}
			}

			// Verify shapes that should not exist
			for _, id := range tt.shouldNotExist {
				if sm.FindShape(id) != nil {
					t.Errorf("Shape %s should not exist after %s", id, tt.name)
				}
			}
		})
	}

	// Test sequential removal
	t.Run("sequential removal", func(t *testing.T) {
		sm := NewSceneManager()
		sm.AddShapes(initialShapes)

		// Remove shapes in sequence: shape1, then shape3
		removeOrder := []string{"shape1", "shape3"}
		expectedCounts := []int{2, 1}

		for i, id := range removeOrder {
			err := sm.RemoveShape(id)
			if err != nil {
				t.Fatalf("Failed to remove %s: %v", id, err)
			}

			if sm.GetShapeCount() != expectedCounts[i] {
				t.Errorf("Expected %d shapes after removing %s, got %d", expectedCounts[i], id, sm.GetShapeCount())
			}

			if sm.FindShape(id) != nil {
				t.Errorf("Shape %s should be removed", id)
			}
		}
	})
}

// Tests for helper functions

func TestExtractFloatArray(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]interface{}
		key      string
		length   int
		expected []float64
		shouldOK bool
	}{
		{
			name:     "valid 3-element array",
			props:    map[string]interface{}{"center": []interface{}{1.0, 2.0, 3.0}},
			key:      "center",
			length:   3,
			expected: []float64{1.0, 2.0, 3.0},
			shouldOK: true,
		},
		{
			name:     "wrong length",
			props:    map[string]interface{}{"center": []interface{}{1.0, 2.0}},
			key:      "center",
			length:   3,
			expected: nil,
			shouldOK: false,
		},
		{
			name:     "non-float element",
			props:    map[string]interface{}{"center": []interface{}{1.0, "invalid", 3.0}},
			key:      "center",
			length:   3,
			expected: nil,
			shouldOK: false,
		},
		{
			name:     "missing key",
			props:    map[string]interface{}{"other": []interface{}{1.0, 2.0, 3.0}},
			key:      "center",
			length:   3,
			expected: nil,
			shouldOK: false,
		},
		{
			name:     "not an array",
			props:    map[string]interface{}{"center": 1.0},
			key:      "center",
			length:   3,
			expected: nil,
			shouldOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractFloatArray(tt.props, tt.key, tt.length)

			if ok != tt.shouldOK {
				t.Errorf("Expected ok=%v, got ok=%v", tt.shouldOK, ok)
			}

			if tt.shouldOK {
				if len(result) != len(tt.expected) {
					t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
				}
				for i, expected := range tt.expected {
					if result[i] != expected {
						t.Errorf("Expected result[%d]=%f, got %f", i, expected, result[i])
					}
				}
			} else if result != nil {
				t.Errorf("Expected nil result when ok=false, got %v", result)
			}
		})
	}
}

func TestExtractFloat(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]interface{}
		key      string
		expected float64
		shouldOK bool
	}{
		{
			name:     "valid float",
			props:    map[string]interface{}{"radius": 2.5},
			key:      "radius",
			expected: 2.5,
			shouldOK: true,
		},
		{
			name:     "missing key",
			props:    map[string]interface{}{"other": 2.5},
			key:      "radius",
			expected: 0,
			shouldOK: false,
		},
		{
			name:     "wrong type",
			props:    map[string]interface{}{"radius": "invalid"},
			key:      "radius",
			expected: 0,
			shouldOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractFloat(tt.props, tt.key)

			if ok != tt.shouldOK {
				t.Errorf("Expected ok=%v, got ok=%v", tt.shouldOK, ok)
			}

			if result != tt.expected {
				t.Errorf("Expected result=%f, got %f", tt.expected, result)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]interface{}
		key      string
		expected string
		shouldOK bool
	}{
		{
			name:     "valid string",
			props:    map[string]interface{}{"id": "test_shape"},
			key:      "id",
			expected: "test_shape",
			shouldOK: true,
		},
		{
			name:     "missing key",
			props:    map[string]interface{}{"other": "test"},
			key:      "id",
			expected: "",
			shouldOK: false,
		},
		{
			name:     "wrong type",
			props:    map[string]interface{}{"id": 123},
			key:      "id",
			expected: "",
			shouldOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractString(tt.props, tt.key)

			if ok != tt.shouldOK {
				t.Errorf("Expected ok=%v, got ok=%v", tt.shouldOK, ok)
			}

			if result != tt.expected {
				t.Errorf("Expected result=%s, got %s", tt.expected, result)
			}
		})
	}
}

func TestHasProperty(t *testing.T) {
	props := map[string]interface{}{
		"existing": "value",
		"nil_val":  nil,
	}

	if !hasProperty(props, "existing") {
		t.Error("Expected hasProperty to return true for existing key")
	}

	if !hasProperty(props, "nil_val") {
		t.Error("Expected hasProperty to return true for key with nil value")
	}

	if hasProperty(props, "missing") {
		t.Error("Expected hasProperty to return false for missing key")
	}
}

func TestBoxRotation(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name        string
		box         ShapeRequest
		shouldError bool
		description string
	}{
		{
			name: "box without rotation",
			box: ShapeRequest{
				ID:   "simple_box",
				Type: "box",
				Properties: map[string]interface{}{
					"center":     []interface{}{0.0, 0.0, 0.0},
					"dimensions": []interface{}{2.0, 1.0, 3.0},
					"color":      []interface{}{1.0, 0.0, 0.0},
				},
			},
			shouldError: false,
			description: "Box without rotation should use axis-aligned constructor",
		},
		{
			name: "box with rotation",
			box: ShapeRequest{
				ID:   "rotated_box",
				Type: "box",
				Properties: map[string]interface{}{
					"center":     []interface{}{1.0, 2.0, 3.0},
					"dimensions": []interface{}{2.0, 1.0, 3.0},
					"rotation":   []interface{}{0.5, 1.0, 0.0}, // radians
					"color":      []interface{}{0.0, 1.0, 0.0},
				},
			},
			shouldError: false,
			description: "Box with rotation should use rotated constructor",
		},
		{
			name: "box with invalid rotation format",
			box: ShapeRequest{
				ID:   "bad_rotation_box",
				Type: "box",
				Properties: map[string]interface{}{
					"center":     []interface{}{0.0, 0.0, 0.0},
					"dimensions": []interface{}{1.0, 1.0, 1.0},
					"rotation":   []interface{}{0.5, "invalid"}, // Wrong type and count
					"color":      []interface{}{0.0, 0.0, 1.0},
				},
			},
			shouldError: false, // Should fall back to axis-aligned box
			description: "Box with invalid rotation should fall back to axis-aligned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.AddShapes([]ShapeRequest{tt.box})

			if tt.shouldError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.description)
			} else if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.description, err)
			}

			// If successful, check that shape was added
			if !tt.shouldError && err == nil {
				found := sm.FindShape(tt.box.ID)
				if found == nil {
					t.Errorf("Shape %s was not added", tt.box.ID)
				}
			}

			// Clean up for next test
			sm.ClearScene()
		})
	}
}

func TestToRaytracerSceneWithRotation(t *testing.T) {
	sm := NewSceneManager()

	// Add a box with rotation
	rotatedBox := ShapeRequest{
		ID:   "rotated_box",
		Type: "box",
		Properties: map[string]interface{}{
			"center":     []interface{}{1.0, 2.0, 3.0},
			"dimensions": []interface{}{2.0, 1.0, 3.0},
			"rotation":   []interface{}{0.5, 1.0, 0.0}, // radians
			"color":      []interface{}{0.8, 0.2, 0.4},
		},
	}

	err := sm.AddShapes([]ShapeRequest{rotatedBox})
	if err != nil {
		t.Fatalf("Failed to add rotated box: %v", err)
	}

	// Convert to raytracer scene
	scene, err := sm.ToRaytracerScene()
	if err != nil {
		t.Fatalf("ToRaytracerScene() returned error: %v", err)
	}

	if scene == nil {
		t.Fatal("ToRaytracerScene() returned nil")
	}

	if len(scene.Shapes) != 1 {
		t.Errorf("Expected 1 shape in raytracer scene, got %d", len(scene.Shapes))
	}

	// Test that scene can be created without errors
	// (The actual raytracer functionality is tested by the raytracer library itself)
}

func TestQuadAndDiscCreation(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name  string
		shape ShapeRequest
	}{
		{
			name: "simple quad",
			shape: ShapeRequest{
				ID:   "test_quad",
				Type: "quad",
				Properties: map[string]interface{}{
					"corner": []interface{}{-1.0, -1.0, 0.0},
					"u":      []interface{}{2.0, 0.0, 0.0},
					"v":      []interface{}{0.0, 2.0, 0.0},
					"color":  []interface{}{0.8, 0.6, 0.4},
				},
			},
		},
		{
			name: "simple disc",
			shape: ShapeRequest{
				ID:   "test_disc",
				Type: "disc",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"normal": []interface{}{0.0, 0.0, 1.0},
					"radius": 1.5,
					"color":  []interface{}{0.9, 0.2, 0.3},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.AddShapes([]ShapeRequest{tt.shape})
			if err != nil {
				t.Fatalf("Failed to add %s: %v", tt.shape.Type, err)
			}

			// Verify shape was added
			found := sm.FindShape(tt.shape.ID)
			if found == nil {
				t.Errorf("Shape %s was not added", tt.shape.ID)
			}

			// Test scene conversion
			scene, err := sm.ToRaytracerScene()
			if err != nil {
				t.Fatalf("ToRaytracerScene() returned error: %v", err)
			}
			if scene == nil {
				t.Fatal("ToRaytracerScene() returned nil")
			}

			if len(scene.Shapes) != 1 {
				t.Errorf("Expected 1 shape in scene, got %d", len(scene.Shapes))
			}

			// Clean up
			sm.ClearScene()
		})
	}
}

// Tests for shape validation using table-driven tests

func TestValidateShapeProperties(t *testing.T) {
	tests := []struct {
		name        string
		shape       ShapeRequest
		shouldError bool
	}{
		{
			name: "valid sphere",
			shape: ShapeRequest{
				ID:   "valid_sphere",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 1.0, 2.0},
					"radius": 1.5,
					"color":  []interface{}{0.8, 0.2, 0.4},
				},
			},
			shouldError: false,
		},
		{
			name: "valid box",
			shape: ShapeRequest{
				ID:   "valid_box",
				Type: "box",
				Properties: map[string]interface{}{
					"center":     []interface{}{1.0, 2.0, 3.0},
					"dimensions": []interface{}{2.0, 1.5, 3.0},
					"color":      []interface{}{0.1, 0.9, 0.3},
				},
			},
			shouldError: false,
		},
		{
			name: "valid quad",
			shape: ShapeRequest{
				ID:   "valid_quad",
				Type: "quad",
				Properties: map[string]interface{}{
					"corner": []interface{}{0.0, 0.0, 0.0},
					"u":      []interface{}{2.0, 0.0, 0.0},
					"v":      []interface{}{0.0, 2.0, 0.0},
					"color":  []interface{}{0.8, 0.8, 0.2},
				},
			},
			shouldError: false,
		},
		{
			name: "valid disc",
			shape: ShapeRequest{
				ID:   "valid_disc",
				Type: "disc",
				Properties: map[string]interface{}{
					"center": []interface{}{1.0, 1.0, 1.0},
					"normal": []interface{}{0.0, 1.0, 0.0},
					"radius": 1.5,
					"color":  []interface{}{0.9, 0.1, 0.7},
				},
			},
			shouldError: false,
		},
		{
			name: "empty ID",
			shape: ShapeRequest{
				ID:   "",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
				},
			},
			shouldError: true,
		},
		{
			name: "empty type",
			shape: ShapeRequest{
				ID:   "test_shape",
				Type: "",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
				},
			},
			shouldError: true,
		},
		{
			name: "nil properties",
			shape: ShapeRequest{
				ID:         "test_shape",
				Type:       "sphere",
				Properties: nil,
			},
			shouldError: true,
		},
		{
			name: "unsupported shape type",
			shape: ShapeRequest{
				ID:   "triangle",
				Type: "triangle",
				Properties: map[string]interface{}{
					"center":   []interface{}{0.0, 0.0, 0.0},
					"vertices": []interface{}{},
				},
			},
			shouldError: true,
		},
		{
			name: "sphere without radius",
			shape: ShapeRequest{
				ID:   "incomplete_sphere",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"color":  []interface{}{1.0, 0.0, 0.0},
				},
			},
			shouldError: true,
		},
		{
			name: "sphere without position",
			shape: ShapeRequest{
				ID:   "incomplete_sphere2",
				Type: "sphere",
				Properties: map[string]interface{}{
					"radius": 1.0,
					"color":  []interface{}{1.0, 0.0, 0.0},
				},
			},
			shouldError: true,
		},
		{
			name: "box without dimensions",
			shape: ShapeRequest{
				ID:   "incomplete_box",
				Type: "box",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"color":  []interface{}{1.0, 0.0, 0.0},
				},
			},
			shouldError: true,
		},
		{
			name: "invalid position format",
			shape: ShapeRequest{
				ID:   "bad_position",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, "invalid"}, // Wrong type and count
					"radius": 1.0,
				},
			},
			shouldError: true,
		},
		{
			name: "negative radius",
			shape: ShapeRequest{
				ID:   "negative_sphere",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": -1.0,
				},
			},
			shouldError: true,
		},
		{
			name: "invalid color range",
			shape: ShapeRequest{
				ID:   "bad_color",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"color":  []interface{}{1.5, 0.0, 0.0}, // Color > 1.0
				},
			},
			shouldError: true,
		},
		{
			name: "quad without corner",
			shape: ShapeRequest{
				ID:   "incomplete_quad",
				Type: "quad",
				Properties: map[string]interface{}{
					"u": []interface{}{1.0, 0.0, 0.0},
					"v": []interface{}{0.0, 1.0, 0.0},
				},
			},
			shouldError: true,
		},
		{
			name: "quad without u vector",
			shape: ShapeRequest{
				ID:   "incomplete_quad2",
				Type: "quad",
				Properties: map[string]interface{}{
					"corner": []interface{}{0.0, 0.0, 0.0},
					"v":      []interface{}{0.0, 1.0, 0.0},
				},
			},
			shouldError: true,
		},
		{
			name: "disc without center",
			shape: ShapeRequest{
				ID:   "incomplete_disc",
				Type: "disc",
				Properties: map[string]interface{}{
					"normal": []interface{}{0.0, 1.0, 0.0},
					"radius": 1.0,
				},
			},
			shouldError: true,
		},
		{
			name: "disc with negative radius",
			shape: ShapeRequest{
				ID:   "negative_disc",
				Type: "disc",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"normal": []interface{}{0.0, 1.0, 0.0},
					"radius": -1.0,
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSceneManager()
			err := sm.AddShapes([]ShapeRequest{tt.shape})

			if tt.shouldError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
			} else if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.name, err)
			}
		})
	}

	// Test duplicate ID separately since it requires existing shapes
	t.Run("duplicate ID", func(t *testing.T) {
		sm := NewSceneManager()

		// Add first shape
		first := ShapeRequest{
			ID:   "existing_shape",
			Type: "sphere",
			Properties: map[string]interface{}{
				"center": []interface{}{0.0, 0.0, 0.0},
				"radius": 1.0,
			},
		}
		err := sm.AddShapes([]ShapeRequest{first})
		if err != nil {
			t.Fatalf("Failed to add first shape: %v", err)
		}

		// Try to add duplicate ID
		duplicate := ShapeRequest{
			ID:   "existing_shape", // Same ID
			Type: "box",
			Properties: map[string]interface{}{
				"center":     []interface{}{1.0, 1.0, 1.0},
				"dimensions": []interface{}{1.0, 1.0, 1.0},
			},
		}
		err = sm.AddShapes([]ShapeRequest{duplicate})
		if err == nil {
			t.Error("Expected error for duplicate ID, but got none")
		}
	})
}

func TestShapeWithLambertianMaterial(t *testing.T) {
	sm := NewSceneManager()

	shape := ShapeRequest{
		ID:   "matte_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{0.0, 1.0, 0.0},
			"radius": 1.0,
			"material": map[string]interface{}{
				"type":   "lambertian",
				"albedo": []interface{}{0.8, 0.1, 0.1},
			},
		},
	}

	err := sm.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("AddShapes() with lambertian material failed: %v", err)
	}

	state := sm.GetState()
	if len(state.Shapes) != 1 {
		t.Fatalf("Expected 1 shape, got %d", len(state.Shapes))
	}

	// Verify material is preserved
	mat, ok := extractMaterial(state.Shapes[0].Properties)
	if !ok {
		t.Fatal("Material not found in shape properties")
	}

	matType, _ := mat["type"].(string)
	if matType != "lambertian" {
		t.Errorf("Expected material type 'lambertian', got '%s'", matType)
	}

	albedo, ok := extractFloatArray(mat, "albedo", 3)
	if !ok {
		t.Fatal("Albedo not found or invalid")
	}
	if albedo[0] != 0.8 || albedo[1] != 0.1 || albedo[2] != 0.1 {
		t.Errorf("Expected albedo [0.8, 0.1, 0.1], got %v", albedo)
	}
}

func TestShapeWithMetalMaterial(t *testing.T) {
	sm := NewSceneManager()

	shape := ShapeRequest{
		ID:   "mirror_ball",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{2.0, 1.0, 0.0},
			"radius": 1.0,
			"material": map[string]interface{}{
				"type":   "metal",
				"albedo": []interface{}{0.9, 0.9, 0.9},
				"fuzz":   0.1,
			},
		},
	}

	err := sm.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("AddShapes() with metal material failed: %v", err)
	}

	state := sm.GetState()
	if len(state.Shapes) != 1 {
		t.Fatalf("Expected 1 shape, got %d", len(state.Shapes))
	}

	// Verify material is preserved
	mat, ok := extractMaterial(state.Shapes[0].Properties)
	if !ok {
		t.Fatal("Material not found in shape properties")
	}

	matType, _ := mat["type"].(string)
	if matType != "metal" {
		t.Errorf("Expected material type 'metal', got '%s'", matType)
	}

	albedo, ok := extractFloatArray(mat, "albedo", 3)
	if !ok {
		t.Fatal("Albedo not found or invalid")
	}
	if albedo[0] != 0.9 || albedo[1] != 0.9 || albedo[2] != 0.9 {
		t.Errorf("Expected albedo [0.9, 0.9, 0.9], got %v", albedo)
	}

	fuzz, ok := extractFloat(mat, "fuzz")
	if !ok {
		t.Fatal("Fuzz not found or invalid")
	}
	if fuzz != 0.1 {
		t.Errorf("Expected fuzz 0.1, got %f", fuzz)
	}
}

func TestMaterialValidation(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name        string
		shape       ShapeRequest
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid lambertian",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":   "lambertian",
						"albedo": []interface{}{0.5, 0.5, 0.5},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid metal",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":   "metal",
						"albedo": []interface{}{0.9, 0.9, 0.9},
						"fuzz":   0.5,
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing material type",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"albedo": []interface{}{0.5, 0.5, 0.5},
					},
				},
			},
			expectError: true,
			errorMatch:  "must have a 'type' field",
		},
		{
			name: "unsupported material type",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":   "emissive",
						"albedo": []interface{}{0.5, 0.5, 0.5},
					},
				},
			},
			expectError: true,
			errorMatch:  "unsupported material type",
		},
		{
			name: "lambertian missing albedo",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type": "lambertian",
					},
				},
			},
			expectError: true,
			errorMatch:  "requires 'albedo' property",
		},
		{
			name: "metal missing fuzz",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":   "metal",
						"albedo": []interface{}{0.9, 0.9, 0.9},
					},
				},
			},
			expectError: true,
			errorMatch:  "requires 'fuzz' property",
		},
		{
			name: "metal fuzz out of range",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":   "metal",
						"albedo": []interface{}{0.9, 0.9, 0.9},
						"fuzz":   1.5,
					},
				},
			},
			expectError: true,
			errorMatch:  "must be in range",
		},
		{
			name: "albedo out of range",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":   "lambertian",
						"albedo": []interface{}{1.5, 0.5, 0.5},
					},
				},
			},
			expectError: true,
			errorMatch:  "must be in range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.AddShapes([]ShapeRequest{tt.shape})
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMatch) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMatch, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
			// Clear shapes for next test
			sm.state.Shapes = []ShapeRequest{}
		})
	}
}

func TestUpdateShapeMaterial(t *testing.T) {
	sm := NewSceneManager()

	// Add a shape with lambertian material
	shape := ShapeRequest{
		ID:   "test_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{0.0, 1.0, 0.0},
			"radius": 1.0,
			"material": map[string]interface{}{
				"type":   "lambertian",
				"albedo": []interface{}{0.8, 0.1, 0.1},
			},
		},
	}

	err := sm.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("AddShapes() failed: %v", err)
	}

	// Update material to metal
	err = sm.UpdateShape("test_sphere", map[string]interface{}{
		"properties": map[string]interface{}{
			"material": map[string]interface{}{
				"type":   "metal",
				"albedo": []interface{}{0.9, 0.9, 0.9},
				"fuzz":   0.2,
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateShape() failed: %v", err)
	}

	// Verify material was updated
	state := sm.GetState()
	mat, ok := extractMaterial(state.Shapes[0].Properties)
	if !ok {
		t.Fatal("Material not found after update")
	}

	matType, _ := mat["type"].(string)
	if matType != "metal" {
		t.Errorf("Expected material type 'metal', got '%s'", matType)
	}

	fuzz, ok := extractFloat(mat, "fuzz")
	if !ok {
		t.Fatal("Fuzz not found after update")
	}
	if fuzz != 0.2 {
		t.Errorf("Expected fuzz 0.2, got %f", fuzz)
	}
}

func TestShapeWithDielectricMaterial(t *testing.T) {
	sm := NewSceneManager()

	shape := ShapeRequest{
		ID:   "glass_sphere",
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{0.0, 1.0, 0.0},
			"radius": 1.0,
			"material": map[string]interface{}{
				"type":             "dielectric",
				"refractive_index": 1.5,
			},
		},
	}

	err := sm.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("AddShapes() with dielectric material failed: %v", err)
	}

	state := sm.GetState()
	if len(state.Shapes) != 1 {
		t.Fatalf("Expected 1 shape, got %d", len(state.Shapes))
	}

	// Verify material is preserved
	mat, ok := extractMaterial(state.Shapes[0].Properties)
	if !ok {
		t.Fatal("Material not found in shape properties")
	}

	matType, _ := mat["type"].(string)
	if matType != "dielectric" {
		t.Errorf("Expected material type 'dielectric', got '%s'", matType)
	}

	refractiveIndex, ok := extractFloat(mat, "refractive_index")
	if !ok {
		t.Fatal("Refractive index not found or invalid")
	}
	if refractiveIndex != 1.5 {
		t.Errorf("Expected refractive_index 1.5, got %f", refractiveIndex)
	}
}

func TestDielectricMaterialValidation(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name        string
		shape       ShapeRequest
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid dielectric glass",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":             "dielectric",
						"refractive_index": 1.5,
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid dielectric water",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":             "dielectric",
						"refractive_index": 1.33,
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid dielectric diamond",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":             "dielectric",
						"refractive_index": 2.4,
					},
				},
			},
			expectError: false,
		},
		{
			name: "dielectric missing refractive_index",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type": "dielectric",
					},
				},
			},
			expectError: true,
			errorMatch:  "requires 'refractive_index' property",
		},
		{
			name: "dielectric refractive_index too low",
			shape: ShapeRequest{
				ID:   "test",
				Type: "sphere",
				Properties: map[string]interface{}{
					"center": []interface{}{0.0, 0.0, 0.0},
					"radius": 1.0,
					"material": map[string]interface{}{
						"type":             "dielectric",
						"refractive_index": 0.5,
					},
				},
			},
			expectError: true,
			errorMatch:  "must be >= 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.AddShapes([]ShapeRequest{tt.shape})
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMatch) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMatch, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
			// Clear shapes for next test
			sm.state.Shapes = []ShapeRequest{}
		})
	}
}

func TestSetCamera(t *testing.T) {
	sm := NewSceneManager()

	// Test valid camera configuration
	newCamera := CameraInfo{
		Center:   []float64{10, 5, 15},
		LookAt:   []float64{0, 0, 0},
		VFov:     60.0,
		Aperture: 0.1,
	}

	err := sm.SetCamera(newCamera)
	if err != nil {
		t.Fatalf("SetCamera() with valid config returned error: %v", err)
	}

	state := sm.GetState()
	if !cameraEqual(state.Camera, newCamera) {
		t.Errorf("Camera not updated correctly. Expected %+v, got %+v", newCamera, state.Camera)
	}
}

func TestSetCameraValidation(t *testing.T) {
	sm := NewSceneManager()

	tests := []struct {
		name         string
		camera       CameraInfo
		expectError  bool
		errorPattern string
	}{
		{
			name: "valid camera",
			camera: CameraInfo{
				Center:   []float64{1, 2, 3},
				LookAt:   []float64{0, 0, 0},
				VFov:     45.0,
				Aperture: 0.0,
			},
			expectError: false,
		},
		{
			name: "invalid vfov - too low",
			camera: CameraInfo{
				Center:   []float64{1, 2, 3},
				LookAt:   []float64{0, 0, 0},
				VFov:     0.0,
				Aperture: 0.0,
			},
			expectError:  true,
			errorPattern: `vfov.*range`,
		},
		{
			name: "invalid vfov - too high",
			camera: CameraInfo{
				Center:   []float64{1, 2, 3},
				LookAt:   []float64{0, 0, 0},
				VFov:     180.0,
				Aperture: 0.0,
			},
			expectError:  true,
			errorPattern: `vfov.*range`,
		},
		{
			name: "invalid vfov - negative",
			camera: CameraInfo{
				Center:   []float64{1, 2, 3},
				LookAt:   []float64{0, 0, 0},
				VFov:     -10.0,
				Aperture: 0.0,
			},
			expectError:  true,
			errorPattern: `vfov.*range`,
		},
		{
			name: "invalid aperture - negative",
			camera: CameraInfo{
				Center:   []float64{1, 2, 3},
				LookAt:   []float64{0, 0, 0},
				VFov:     45.0,
				Aperture: -0.5,
			},
			expectError:  true,
			errorPattern: `aperture.*range`,
		},
		{
			name: "wide angle camera",
			camera: CameraInfo{
				Center:   []float64{0, 10, 0},
				LookAt:   []float64{0, 0, 0},
				VFov:     120.0,
				Aperture: 0.2,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.SetCamera(tt.camera)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorPattern != "" {
					matched, regexErr := regexp.MatchString(tt.errorPattern, err.Error())
					if regexErr != nil {
						t.Fatalf("Invalid regex pattern '%s': %v", tt.errorPattern, regexErr)
					}
					if !matched {
						t.Errorf("Expected error matching pattern '%s', got: %v", tt.errorPattern, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestSetCameraMultipleErrors(t *testing.T) {
	sm := NewSceneManager()

	// Create a camera with multiple validation errors
	badCamera := CameraInfo{
		Center:   []float64{1.0}, // Wrong length
		LookAt:   nil,            // Missing
		VFov:     200.0,          // Out of range
		Aperture: -1.0,           // Negative
	}

	err := sm.SetCamera(badCamera)
	if err == nil {
		t.Fatal("Expected error for invalid camera, got none")
	}

	// Check if it's a ValidationErrors type
	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors type, got %T", err)
	}

	// Should have 4 errors
	if len(validationErrs) != 4 {
		t.Errorf("Expected 4 validation errors, got %d: %v", len(validationErrs), validationErrs)
	}

	// Verify the error message includes count
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
	t.Logf("Error message: %s", errMsg)
}

func TestValidateShapeMultipleErrors(t *testing.T) {
	sm := NewSceneManager()

	// Create a sphere with multiple validation errors
	badSphere := ShapeRequest{
		ID:   "", // Empty ID
		Type: "sphere",
		Properties: map[string]interface{}{
			"center": []interface{}{1.0}, // Wrong length
			// Missing radius
			"color": []interface{}{2.0, 0.5, 0.5}, // Color out of range
		},
	}

	err := sm.AddShapes([]ShapeRequest{badSphere})
	if err == nil {
		t.Fatal("Expected error for invalid shape, got none")
	}

	// Check if it's a ValidationErrors type
	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors type, got %T", err)
	}

	// Should have at least 4 errors (empty ID, wrong center length, missing radius, color out of range)
	if len(validationErrs) < 4 {
		t.Errorf("Expected at least 4 validation errors, got %d: %v", len(validationErrs), validationErrs)
	}

	t.Logf("Error message: %s", err.Error())
}

func TestValidateLightMultipleErrors(t *testing.T) {
	sm := NewSceneManager()

	// Create a point_spot_light with multiple validation errors
	badLight := LightRequest{
		ID:   "", // Empty ID
		Type: "point_spot_light",
		Properties: map[string]interface{}{
			"center": []interface{}{1.0, 2.0}, // Wrong length
			// Missing emission
			"cutoff_angle":     200.0, // Out of range
			"falloff_exponent": -5.0,  // Negative
		},
	}

	err := sm.AddLights([]LightRequest{badLight})
	if err == nil {
		t.Fatal("Expected error for invalid light, got none")
	}

	// Check if it's a ValidationErrors type
	validationErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors type, got %T", err)
	}

	// Should have at least 5 errors (empty ID, wrong center length, missing emission, invalid cutoff_angle, negative falloff_exponent)
	if len(validationErrs) < 5 {
		t.Errorf("Expected at least 5 validation errors, got %d: %v", len(validationErrs), validationErrs)
	}

	t.Logf("Error message: %s", err.Error())
}
