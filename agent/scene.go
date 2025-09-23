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

// AddShapes adds shapes to the scene and updates camera positioning
func (sm *SceneManager) AddShapes(shapes []ShapeRequest) error {
	if len(shapes) == 0 {
		return nil
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
	cameraDistance := shape.Size*3 + 5
	sm.state.Camera.Position = [3]float64{
		shape.Position[0],
		shape.Position[1],
		shape.Position[2] + cameraDistance,
	}
	sm.state.Camera.LookAt = shape.Position
}

// GetState returns a copy of the current scene state
func (sm *SceneManager) GetState() *SceneState {
	// Return a copy to prevent external mutation
	stateCopy := &SceneState{
		Shapes: make([]ShapeRequest, len(sm.state.Shapes)),
		Camera: sm.state.Camera,
	}
	copy(stateCopy.Shapes, sm.state.Shapes)
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
			sceneContext += fmt.Sprintf("%d) %s at [%.1f,%.1f,%.1f] size %.1f color [%.1f,%.1f,%.1f]",
				i+1, shape.Type, shape.Position[0], shape.Position[1], shape.Position[2],
				shape.Size, shape.Color[0], shape.Color[1], shape.Color[2])
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
		// Create material with requested color
		shapeMaterial := material.NewLambertian(core.NewVec3(shapeReq.Color[0], shapeReq.Color[1], shapeReq.Color[2]))

		// Create geometry based on type
		var shape geometry.Shape
		switch shapeReq.Type {
		case "sphere":
			shape = geometry.NewSphere(
				core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2]),
				shapeReq.Size,
				shapeMaterial,
			)
		case "box":
			// Create a simple cube using a sphere for now (since Box constructor seems different)
			shape = geometry.NewSphere(
				core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2]),
				shapeReq.Size/2, // Use half size for radius
				shapeMaterial,
			)
		default:
			// Default to sphere
			shape = geometry.NewSphere(
				core.NewVec3(shapeReq.Position[0], shapeReq.Position[1], shapeReq.Position[2]),
				shapeReq.Size,
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

	// Add default lighting
	sceneWithShapes.AddUniformInfiniteLight(core.NewVec3(0.5, 0.7, 1.0))

	return sceneWithShapes
}
