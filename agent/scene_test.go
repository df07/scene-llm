package agent

import (
	"strings"
	"testing"
)

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
		Position: [3]float64{0, 0, 5},
		LookAt:   [3]float64{0, 0, 0},
	}
	if state.Camera != expectedCamera {
		t.Errorf("Expected camera %+v, got %+v", expectedCamera, state.Camera)
	}
}

func TestAddShapes(t *testing.T) {
	sm := NewSceneManager()

	shapes := []ShapeRequest{
		{
			Type:     "sphere",
			Position: [3]float64{1, 2, 3},
			Size:     2.0,
			Color:    [3]float64{1.0, 0.0, 0.0},
		},
		{
			Type:     "box",
			Position: [3]float64{4, 5, 6},
			Size:     1.5,
			Color:    [3]float64{0.0, 1.0, 0.0},
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
	for i, expectedShape := range shapes {
		if state.Shapes[i] != expectedShape {
			t.Errorf("Shape %d: expected %+v, got %+v", i, expectedShape, state.Shapes[i])
		}
	}
}

func TestAddShapesUpdatesCamera(t *testing.T) {
	sm := NewSceneManager()

	shape := ShapeRequest{
		Type:     "sphere",
		Position: [3]float64{10, 20, 30},
		Size:     5.0,
		Color:    [3]float64{1.0, 0.0, 0.0},
	}

	err := sm.AddShapes([]ShapeRequest{shape})
	if err != nil {
		t.Fatalf("AddShapes() returned error: %v", err)
	}

	state := sm.GetState()

	// Camera should be positioned relative to the first shape
	// expectedDistance := shape.Size*3 + 5 = 5.0*3 + 5 = 20
	expectedCamera := CameraInfo{
		Position: [3]float64{10, 20, 50}, // shape.Position[2] + 20
		LookAt:   [3]float64{10, 20, 30}, // shape.Position
	}

	if state.Camera != expectedCamera {
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
			Type:     "sphere",
			Position: [3]float64{1.5, 2.7, 3.2},
			Size:     2.5,
			Color:    [3]float64{0.8, 0.2, 0.4},
		},
		{
			Type:     "box",
			Position: [3]float64{-1.0, 0.0, 1.0},
			Size:     1.0,
			Color:    [3]float64{0.0, 1.0, 0.5},
		},
	}

	sm.AddShapes(shapes)
	context := sm.BuildContext()

	// Check that context contains expected elements
	if !strings.Contains(context, "2 shapes") {
		t.Errorf("Context should contain '2 shapes', got: %s", context)
	}

	if !strings.Contains(context, "sphere at [1.5,2.7,3.2]") {
		t.Errorf("Context should contain sphere info, got: %s", context)
	}

	if !strings.Contains(context, "box at [-1.0,0.0,1.0]") {
		t.Errorf("Context should contain box info, got: %s", context)
	}

	if !strings.Contains(context, "size 2.5") {
		t.Errorf("Context should contain size info, got: %s", context)
	}
}

func TestClearScene(t *testing.T) {
	sm := NewSceneManager()

	// Add some shapes first
	shapes := []ShapeRequest{
		{Type: "sphere", Position: [3]float64{1, 2, 3}, Size: 1.0, Color: [3]float64{1, 0, 0}},
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
		Position: [3]float64{0, 0, 5},
		LookAt:   [3]float64{0, 0, 0},
	}
	if state.Camera != expectedCamera {
		t.Errorf("Expected camera reset to %+v, got %+v", expectedCamera, state.Camera)
	}
}

func TestGetStateReturnsImmutableCopy(t *testing.T) {
	sm := NewSceneManager()

	// Add a shape
	shape := ShapeRequest{
		Type:     "sphere",
		Position: [3]float64{1, 2, 3},
		Size:     1.0,
		Color:    [3]float64{1, 0, 0},
	}
	sm.AddShapes([]ShapeRequest{shape})

	// Get state twice
	state1 := sm.GetState()
	state2 := sm.GetState()

	// Modify first state
	state1.Shapes[0].Size = 999.0

	// Second state should be unaffected
	if state2.Shapes[0].Size != 1.0 {
		t.Errorf("GetState() should return independent copies, but modification affected other copy")
	}

	// Original scene should be unaffected
	if sm.GetState().Shapes[0].Size != 1.0 {
		t.Errorf("GetState() should return copies, but modification affected original scene")
	}
}

func TestGetShapeCount(t *testing.T) {
	sm := NewSceneManager()

	if sm.GetShapeCount() != 0 {
		t.Errorf("Expected 0 shapes initially, got %d", sm.GetShapeCount())
	}

	shapes := []ShapeRequest{
		{Type: "sphere", Position: [3]float64{0, 0, 0}, Size: 1.0, Color: [3]float64{1, 0, 0}},
		{Type: "box", Position: [3]float64{1, 1, 1}, Size: 2.0, Color: [3]float64{0, 1, 0}},
		{Type: "sphere", Position: [3]float64{2, 2, 2}, Size: 1.5, Color: [3]float64{0, 0, 1}},
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
