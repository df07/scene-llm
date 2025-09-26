package agent

import (
	"fmt"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// SceneManager handles all scene state and operations
type SceneManager struct {
	state *SceneState
}

// NewSceneManager creates a new scene manager with default scene
func NewSceneManager() *SceneManager {
	return &SceneManager{
		state: &SceneState{
			Shapes: []ShapeRequest{},
			Camera: CameraInfo{
				Position: [3]float64{0, 0, 5},
				LookAt:   [3]float64{0, 0, 0},
			},
		},
	}
}

// validateShapeProperties validates that a shape has the required properties for its type
func validateShapeProperties(shape ShapeRequest) error {
	if shape.ID == "" {
		return fmt.Errorf("shape ID cannot be empty")
	}

	if shape.Type == "" {
		return fmt.Errorf("shape type cannot be empty")
	}

	if shape.Properties == nil {
		return fmt.Errorf("shape properties cannot be nil")
	}

	switch shape.Type {
	case "sphere":
		// Sphere requires position and radius
		if _, hasPos := shape.Properties["position"]; !hasPos {
			return fmt.Errorf("sphere '%s' requires 'position' property", shape.ID)
		}
		if _, hasRadius := shape.Properties["radius"]; !hasRadius {
			return fmt.Errorf("sphere '%s' requires 'radius' property", shape.ID)
		}

		// Validate position is an array of 3 numbers
		if posVal, ok := shape.Properties["position"].([]interface{}); ok {
			if len(posVal) != 3 {
				return fmt.Errorf("sphere '%s' position must have exactly 3 coordinates", shape.ID)
			}
			for i, coord := range posVal {
				if _, ok := coord.(float64); !ok {
					return fmt.Errorf("sphere '%s' position[%d] must be a number", shape.ID, i)
				}
			}
		} else {
			return fmt.Errorf("sphere '%s' position must be an array", shape.ID)
		}

		// Validate radius is a positive number
		if radiusVal, ok := shape.Properties["radius"].(float64); ok {
			if radiusVal <= 0 {
				return fmt.Errorf("sphere '%s' radius must be positive", shape.ID)
			}
		} else {
			return fmt.Errorf("sphere '%s' radius must be a number", shape.ID)
		}

	case "box":
		// Box requires position and dimensions
		if _, hasPos := shape.Properties["position"]; !hasPos {
			return fmt.Errorf("box '%s' requires 'position' property", shape.ID)
		}
		if _, hasDims := shape.Properties["dimensions"]; !hasDims {
			return fmt.Errorf("box '%s' requires 'dimensions' property", shape.ID)
		}

		// Validate position is an array of 3 numbers
		if posVal, ok := shape.Properties["position"].([]interface{}); ok {
			if len(posVal) != 3 {
				return fmt.Errorf("box '%s' position must have exactly 3 coordinates", shape.ID)
			}
			for i, coord := range posVal {
				if _, ok := coord.(float64); !ok {
					return fmt.Errorf("box '%s' position[%d] must be a number", shape.ID, i)
				}
			}
		} else {
			return fmt.Errorf("box '%s' position must be an array", shape.ID)
		}

		// Validate dimensions is an array of 3 positive numbers
		if dimsVal, ok := shape.Properties["dimensions"].([]interface{}); ok {
			if len(dimsVal) != 3 {
				return fmt.Errorf("box '%s' dimensions must have exactly 3 values [width, height, depth]", shape.ID)
			}
			for i, dim := range dimsVal {
				if dimNum, ok := dim.(float64); ok {
					if dimNum <= 0 {
						return fmt.Errorf("box '%s' dimensions[%d] must be positive", shape.ID, i)
					}
				} else {
					return fmt.Errorf("box '%s' dimensions[%d] must be a number", shape.ID, i)
				}
			}
		} else {
			return fmt.Errorf("box '%s' dimensions must be an array", shape.ID)
		}

	default:
		return fmt.Errorf("unsupported shape type '%s' for shape '%s'", shape.Type, shape.ID)
	}

	// Validate color if present (optional property)
	if colorVal, exists := shape.Properties["color"]; exists {
		if colorArray, ok := colorVal.([]interface{}); ok {
			if len(colorArray) != 3 {
				return fmt.Errorf("shape '%s' color must have exactly 3 values [r, g, b]", shape.ID)
			}
			for i, component := range colorArray {
				if colorNum, ok := component.(float64); ok {
					if colorNum < 0 || colorNum > 1 {
						return fmt.Errorf("shape '%s' color[%d] must be between 0 and 1", shape.ID, i)
					}
				} else {
					return fmt.Errorf("shape '%s' color[%d] must be a number", shape.ID, i)
				}
			}
		} else {
			return fmt.Errorf("shape '%s' color must be an array", shape.ID)
		}
	}

	return nil
}

// AddShapes adds shapes to the scene and updates camera positioning
func (sm *SceneManager) AddShapes(shapes []ShapeRequest) error {
	if len(shapes) == 0 {
		return nil
	}

	// Validate unique IDs and shape properties
	for _, newShape := range shapes {
		// Validate shape properties
		if err := validateShapeProperties(newShape); err != nil {
			return err
		}

		// Check for duplicate IDs
		if sm.FindShape(newShape.ID) != nil {
			return fmt.Errorf("shape with ID '%s' already exists", newShape.ID)
		}
	}

	// Add shapes to scene
	sm.state.Shapes = append(sm.state.Shapes, shapes...)

	// Update camera to look at the first new shape with proper distance
	firstShape := shapes[0]
	sm.updateCameraForShape(firstShape)

	return nil
}

// updateCameraForShape positions camera to view a shape optimally
func (sm *SceneManager) updateCameraForShape(shape ShapeRequest) {
	// Extract position from properties
	var position [3]float64
	var size float64 = 2.0 // default size

	if props := shape.Properties; props != nil {
		// Try to get position
		if posVal, ok := props["position"].([]interface{}); ok && len(posVal) == 3 {
			for i, v := range posVal {
				if f, ok := v.(float64); ok {
					position[i] = f
				}
			}
		}

		// Try to get size (radius for sphere, dimensions for box, etc.)
		if sizeVal, ok := props["radius"].(float64); ok {
			size = sizeVal
		} else if sizeVal, ok := props["size"].(float64); ok {
			size = sizeVal
		} else if dimsVal, ok := props["dimensions"].([]interface{}); ok && len(dimsVal) >= 1 {
			if f, ok := dimsVal[0].(float64); ok {
				size = f // Use first dimension as representative size
			}
		}
	}

	cameraDistance := size*3 + 5
	sm.state.Camera.Position = [3]float64{
		position[0],
		position[1],
		position[2] + cameraDistance,
	}
	sm.state.Camera.LookAt = position
}

// GetState returns a deep copy of the current scene state
func (sm *SceneManager) GetState() *SceneState {
	// Return a deep copy to prevent external mutation
	stateCopy := &SceneState{
		Shapes: make([]ShapeRequest, len(sm.state.Shapes)),
		Camera: sm.state.Camera,
	}

	// Deep copy each shape including its properties map
	for i, shape := range sm.state.Shapes {
		stateCopy.Shapes[i] = ShapeRequest{
			ID:         shape.ID,
			Type:       shape.Type,
			Properties: make(map[string]interface{}),
		}

		// Deep copy the properties map
		if shape.Properties != nil {
			for key, value := range shape.Properties {
				stateCopy.Shapes[i].Properties[key] = value
			}
		}
	}

	return stateCopy
}

// BuildContext creates a context string describing the current scene state
func (sm *SceneManager) BuildContext() string {
	sceneContext := "Current scene state: "
	if len(sm.state.Shapes) == 0 {
		sceneContext += "empty scene with no objects."
	} else {
		sceneContext += fmt.Sprintf("%d shapes: ", len(sm.state.Shapes))
		for i, shape := range sm.state.Shapes {
			// Extract properties using helper function
			position, size, color := extractShapeProperties(shape)

			sceneContext += fmt.Sprintf("%s) %s (ID: %s) at [%.1f,%.1f,%.1f] size %.1f color [%.1f,%.1f,%.1f]",
				fmt.Sprintf("%d", i+1), shape.Type, shape.ID, position[0], position[1], position[2],
				size, color[0], color[1], color[2])
			if i < len(sm.state.Shapes)-1 {
				sceneContext += ", "
			}
		}
	}
	return sceneContext
}

// ClearScene resets the scene to empty state
func (sm *SceneManager) ClearScene() {
	sm.state.Shapes = []ShapeRequest{}
	sm.state.Camera = CameraInfo{
		Position: [3]float64{0, 0, 5},
		LookAt:   [3]float64{0, 0, 0},
	}
}

// GetShapeCount returns the number of shapes in the scene
func (sm *SceneManager) GetShapeCount() int {
	return len(sm.state.Shapes)
}

// FindShape finds a shape by ID, returns nil if not found
func (sm *SceneManager) FindShape(id string) *ShapeRequest {
	for i := range sm.state.Shapes {
		if sm.state.Shapes[i].ID == id {
			return &sm.state.Shapes[i]
		}
	}
	return nil
}

// UpdateShape updates an existing shape by ID
func (sm *SceneManager) UpdateShape(id string, updates map[string]interface{}) error {
	// Find the shape
	for i := range sm.state.Shapes {
		if sm.state.Shapes[i].ID == id {
			shape := &sm.state.Shapes[i]

			// Apply updates
			if newID, ok := updates["id"].(string); ok && newID != "" {
				// Check if new ID already exists (and it's not the current shape)
				if newID != shape.ID && sm.FindShape(newID) != nil {
					return fmt.Errorf("shape with ID '%s' already exists", newID)
				}
				shape.ID = newID
			}

			if newType, ok := updates["type"].(string); ok {
				shape.Type = newType
			}

			if newProps, ok := updates["properties"].(map[string]interface{}); ok {
				// Merge properties (replace existing ones with new values)
				if shape.Properties == nil {
					shape.Properties = make(map[string]interface{})
				}
				for key, value := range newProps {
					shape.Properties[key] = value
				}
			}

			return nil
		}
	}

	return fmt.Errorf("shape with ID '%s' not found", id)
}

// RemoveShape removes a shape by ID
func (sm *SceneManager) RemoveShape(id string) error {
	for i := range sm.state.Shapes {
		if sm.state.Shapes[i].ID == id {
			// Remove shape by slicing
			sm.state.Shapes = append(sm.state.Shapes[:i], sm.state.Shapes[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("shape with ID '%s' not found", id)
}

// extractShapeProperties extracts common properties from a shape's Properties map
func extractShapeProperties(shape ShapeRequest) (position [3]float64, size float64, color [3]float64) {
	// Default values
	position = [3]float64{0, 0, 0}
	size = 1.0
	color = [3]float64{0.5, 0.5, 0.5} // Gray default

	if shape.Properties == nil {
		return
	}

	// Extract position
	if posVal, ok := shape.Properties["position"].([]interface{}); ok && len(posVal) >= 3 {
		for i := 0; i < 3 && i < len(posVal); i++ {
			if f, ok := posVal[i].(float64); ok {
				position[i] = f
			}
		}
	}

	// Extract size/radius
	if sizeVal, ok := shape.Properties["radius"].(float64); ok {
		size = sizeVal
	} else if sizeVal, ok := shape.Properties["size"].(float64); ok {
		size = sizeVal
	} else if dimsVal, ok := shape.Properties["dimensions"].([]interface{}); ok && len(dimsVal) >= 1 {
		if f, ok := dimsVal[0].(float64); ok {
			size = f // Use first dimension as representative size
		}
	}

	// Extract color
	if colorVal, ok := shape.Properties["color"].([]interface{}); ok && len(colorVal) >= 3 {
		for i := 0; i < 3 && i < len(colorVal); i++ {
			if f, ok := colorVal[i].(float64); ok {
				color[i] = f
			}
		}
	}

	return
}

// ToRaytracerScene converts the scene state to a raytracer scene
func (sm *SceneManager) ToRaytracerScene() *scene.Scene {
	// Scene configuration
	samplingConfig := scene.SamplingConfig{
		Width:                     400,
		Height:                    300,
		SamplesPerPixel:           10,
		MaxDepth:                  8,
		RussianRouletteMinBounces: 3,
		AdaptiveMinSamples:        0.1,
		AdaptiveThreshold:         0.05,
	}

	// Camera using our scene's camera settings
	cameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(sm.state.Camera.Position[0], sm.state.Camera.Position[1], sm.state.Camera.Position[2]),
		LookAt:        core.NewVec3(sm.state.Camera.LookAt[0], sm.state.Camera.LookAt[1], sm.state.Camera.LookAt[2]),
		Up:            core.NewVec3(0, 1, 0),
		VFov:          45.0,
		Width:         samplingConfig.Width,
		AspectRatio:   float64(samplingConfig.Width) / float64(samplingConfig.Height),
		Aperture:      0.0,
		FocusDistance: 0.0,
	}
	camera := geometry.NewCamera(cameraConfig)

	// Create shapes
	var sceneShapes []geometry.Shape
	for _, shapeReq := range sm.state.Shapes {
		// Extract properties using helper function
		position, size, color := extractShapeProperties(shapeReq)

		// Create material with requested color
		shapeMaterial := material.NewLambertian(core.NewVec3(color[0], color[1], color[2]))

		// Create geometry based on type
		var shape geometry.Shape
		switch shapeReq.Type {
		case "sphere":
			shape = geometry.NewSphere(
				core.NewVec3(position[0], position[1], position[2]),
				size,
				shapeMaterial,
			)
		case "box":
			// Create a simple cube using a sphere for now (since Box constructor seems different)
			shape = geometry.NewSphere(
				core.NewVec3(position[0], position[1], position[2]),
				size/2, // Use half size for radius
				shapeMaterial,
			)
		default:
			// Default to sphere
			shape = geometry.NewSphere(
				core.NewVec3(position[0], position[1], position[2]),
				size,
				shapeMaterial,
			)
		}
		sceneShapes = append(sceneShapes, shape)
	}

	// Create scene
	sceneWithShapes := &scene.Scene{
		Camera:         camera,
		Shapes:         sceneShapes,
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Add default sky gradient lighting (blue to white)
	sceneWithShapes.AddGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white horizon)
	)

	return sceneWithShapes
}
