package agent

import (
	"fmt"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// SceneState represents the current 3D scene state
type SceneState struct {
	Shapes []ShapeRequest `json:"shapes"`
	Lights []LightRequest `json:"lights"`
	Camera CameraInfo     `json:"camera"`
}

// CameraInfo represents camera information
type CameraInfo struct {
	Center   []float64 `json:"center"`
	LookAt   []float64 `json:"look_at"`
	VFov     float64   `json:"vfov"`     // Vertical field of view in degrees
	Aperture float64   `json:"aperture"` // Lens aperture for depth of field
}

// Helper functions for extracting properties from map[string]interface{}

// extractFloatArray extracts a float array of specified length from properties
func extractFloatArray(properties map[string]interface{}, key string, length int) ([]float64, bool) {
	if val, ok := properties[key].([]interface{}); ok && len(val) == length {
		result := make([]float64, length)
		for i, v := range val {
			if f, ok := v.(float64); ok {
				result[i] = f
			} else {
				return nil, false
			}
		}
		return result, true
	}
	return nil, false
}

// extractFloat extracts a single float value from properties
func extractFloat(properties map[string]interface{}, key string) (float64, bool) {
	if val, ok := properties[key].(float64); ok {
		return val, true
	}
	return 0, false
}

// validateFloatArray validates that a property is a float array of specified length with optional range check
func validateFloatArray(properties map[string]interface{}, key string, length int, minVal, maxVal *float64, shapeID string) error {
	val, ok := properties[key].([]interface{})
	if !ok {
		return fmt.Errorf("%s '%s' %s must be an array", getShapeType(properties), shapeID, key)
	}
	if len(val) != length {
		return fmt.Errorf("%s '%s' %s must have exactly %d values", getShapeType(properties), shapeID, key, length)
	}
	for i, v := range val {
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%s '%s' %s[%d] must be a number", getShapeType(properties), shapeID, key, i)
		}
		if minVal != nil && f < *minVal {
			return fmt.Errorf("%s '%s' %s[%d] must be >= %.1f", getShapeType(properties), shapeID, key, i, *minVal)
		}
		if maxVal != nil && f > *maxVal {
			return fmt.Errorf("%s '%s' %s[%d] must be <= %.1f", getShapeType(properties), shapeID, key, i, *maxVal)
		}
	}
	return nil
}

// extractString extracts a string value from properties
func extractString(properties map[string]interface{}, key string) (string, bool) {
	if val, ok := properties[key].(string); ok {
		return val, true
	}
	return "", false
}

// hasProperty checks if a property exists in the map
func hasProperty(properties map[string]interface{}, key string) bool {
	_, exists := properties[key]
	return exists
}

// validateRequiredProperty validates that a required property exists
func validateRequiredProperty(properties map[string]interface{}, key, shapeType, shapeID string) error {
	if !hasProperty(properties, key) {
		return fmt.Errorf("%s '%s' requires '%s' property", shapeType, shapeID, key)
	}
	return nil
}

// getShapeType extracts shape type from validation context (helper for error messages)
func getShapeType(properties map[string]interface{}) string {
	if typeVal, ok := properties["type"].(string); ok {
		return typeVal
	}
	return "shape"
}

// extractMaterial extracts material specification from shape properties
// Returns (materialMap, exists)
func extractMaterial(properties map[string]interface{}) (map[string]interface{}, bool) {
	if mat, ok := properties["material"].(map[string]interface{}); ok {
		return mat, true
	}
	return nil, false
}

// validateMaterialFloatArray validates a material float array property with required check
func validateMaterialFloatArray(mat map[string]interface{}, propName, matType, shapeID string, length int, min, max *float64) error {
	if !hasProperty(mat, propName) {
		return fmt.Errorf("shape '%s' %s material requires '%s' property", shapeID, matType, propName)
	}

	// Validate the array directly to give proper material-specific error messages
	arr, ok := mat[propName].([]interface{})
	if !ok || len(arr) != length {
		elemDesc := "values"
		if length == 3 {
			elemDesc = "[r,g,b] array"
		}
		return fmt.Errorf("shape '%s' %s material '%s' must be %s", shapeID, matType, propName, elemDesc)
	}

	// Validate array values
	for i, v := range arr {
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("shape '%s' %s material %s[%d] must be a number", shapeID, matType, propName, i)
		}
		if min != nil && f < *min {
			return fmt.Errorf("shape '%s' %s material %s[%d] must be in range [%.1f,%.1f]", shapeID, matType, propName, i, *min, *max)
		}
		if max != nil && f > *max {
			return fmt.Errorf("shape '%s' %s material %s[%d] must be in range [%.1f,%.1f]", shapeID, matType, propName, i, *min, *max)
		}
	}
	return nil
}

// validateMaterialFloatProperty validates a required material float property with optional range
func validateMaterialFloatProperty(mat map[string]interface{}, propName, matType, shapeID string, min, max *float64) error {
	if !hasProperty(mat, propName) {
		return fmt.Errorf("shape '%s' %s material requires '%s' property", shapeID, matType, propName)
	}
	val, ok := mat[propName].(float64)
	if !ok {
		return fmt.Errorf("shape '%s' %s material '%s' must be a number", shapeID, matType, propName)
	}
	if min != nil && max != nil && (val < *min || val > *max) {
		return fmt.Errorf("shape '%s' %s material '%s' must be in range [%.1f,%.1f]", shapeID, matType, propName, *min, *max)
	}
	if min != nil && val < *min {
		return fmt.Errorf("shape '%s' %s material '%s' must be >= %.1f", shapeID, matType, propName, *min)
	}
	if max != nil && val > *max {
		return fmt.Errorf("shape '%s' %s material '%s' must be <= %.1f", shapeID, matType, propName, *max)
	}
	return nil
}

// validateMaterial validates material properties
func validateMaterial(mat map[string]interface{}, shapeID string) error {
	// Material type is required
	matType, ok := mat["type"].(string)
	if !ok {
		return fmt.Errorf("shape '%s' material must have a 'type' field", shapeID)
	}

	// Helper variables for range validation
	zero := 0.0
	one := 1.0
	minRefractiveIndex := 1.0

	switch matType {
	case "lambertian":
		// Validate albedo (required [r,g,b] array in [0,1])
		if err := validateMaterialFloatArray(mat, "albedo", matType, shapeID, 3, &zero, &one); err != nil {
			return err
		}

	case "metal":
		// Validate albedo (required [r,g,b] array in [0,1])
		if err := validateMaterialFloatArray(mat, "albedo", matType, shapeID, 3, &zero, &one); err != nil {
			return err
		}
		// Validate fuzz (required float in [0,1])
		if err := validateMaterialFloatProperty(mat, "fuzz", matType, shapeID, &zero, &one); err != nil {
			return err
		}

	case "dielectric":
		// Validate refractive_index (required float >= 1.0)
		if err := validateMaterialFloatProperty(mat, "refractive_index", matType, shapeID, &minRefractiveIndex, nil); err != nil {
			return err
		}

	default:
		return fmt.Errorf("shape '%s' has unsupported material type '%s' (supported: lambertian, metal, dielectric)", shapeID, matType)
	}

	return nil
}

// SceneManager handles all scene state and operations
type SceneManager struct {
	state *SceneState
}

// NewSceneManager creates a new scene manager with default scene
func NewSceneManager() *SceneManager {
	return &SceneManager{
		state: &SceneState{
			Shapes: []ShapeRequest{},
			Lights: []LightRequest{},
			Camera: CameraInfo{
				Center:   []float64{0, 0, 5},
				LookAt:   []float64{0, 0, 0},
				VFov:     45.0,
				Aperture: 0.0,
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
		// Validate required properties exist
		if err := validateRequiredProperty(shape.Properties, "center", "sphere", shape.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(shape.Properties, "radius", "sphere", shape.ID); err != nil {
			return err
		}

		// Validate center is an array of 3 numbers
		if err := validateFloatArray(shape.Properties, "center", 3, nil, nil, shape.ID); err != nil {
			return err
		}

		// Validate radius is a positive number
		if radius, ok := extractFloat(shape.Properties, "radius"); ok {
			if radius <= 0 {
				return fmt.Errorf("sphere '%s' radius must be positive", shape.ID)
			}
		} else {
			return fmt.Errorf("sphere '%s' radius must be a number", shape.ID)
		}

	case "box":
		// Validate required properties exist
		if err := validateRequiredProperty(shape.Properties, "center", "box", shape.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(shape.Properties, "dimensions", "box", shape.ID); err != nil {
			return err
		}

		// Validate center is an array of 3 numbers
		if err := validateFloatArray(shape.Properties, "center", 3, nil, nil, shape.ID); err != nil {
			return err
		}

		// Validate dimensions is an array of 3 positive numbers
		zero := 0.0
		if err := validateFloatArray(shape.Properties, "dimensions", 3, &zero, nil, shape.ID); err != nil {
			return err
		}

	case "quad":
		// Validate required properties exist
		if err := validateRequiredProperty(shape.Properties, "corner", "quad", shape.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(shape.Properties, "u", "quad", shape.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(shape.Properties, "v", "quad", shape.ID); err != nil {
			return err
		}

		// Validate corner, u, and v are arrays of 3 numbers
		if err := validateFloatArray(shape.Properties, "corner", 3, nil, nil, shape.ID); err != nil {
			return err
		}
		if err := validateFloatArray(shape.Properties, "u", 3, nil, nil, shape.ID); err != nil {
			return err
		}
		if err := validateFloatArray(shape.Properties, "v", 3, nil, nil, shape.ID); err != nil {
			return err
		}

	case "disc":
		// Validate required properties exist
		if err := validateRequiredProperty(shape.Properties, "center", "disc", shape.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(shape.Properties, "normal", "disc", shape.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(shape.Properties, "radius", "disc", shape.ID); err != nil {
			return err
		}

		// Validate center and normal are arrays of 3 numbers
		if err := validateFloatArray(shape.Properties, "center", 3, nil, nil, shape.ID); err != nil {
			return err
		}
		if err := validateFloatArray(shape.Properties, "normal", 3, nil, nil, shape.ID); err != nil {
			return err
		}

		// Validate radius is a positive number
		if radius, ok := extractFloat(shape.Properties, "radius"); ok {
			if radius <= 0 {
				return fmt.Errorf("disc '%s' radius must be positive", shape.ID)
			}
		} else {
			return fmt.Errorf("disc '%s' radius must be a number", shape.ID)
		}

	default:
		return fmt.Errorf("unsupported shape type '%s' for shape '%s'", shape.Type, shape.ID)
	}

	// Validate color if present (optional property)
	if hasProperty(shape.Properties, "color") {
		zero := 0.0
		one := 1.0
		if err := validateFloatArray(shape.Properties, "color", 3, &zero, &one, shape.ID); err != nil {
			return err
		}
	}

	// Validate material if present (optional property)
	if mat, ok := extractMaterial(shape.Properties); ok {
		if err := validateMaterial(mat, shape.ID); err != nil {
			return err
		}
	}

	return nil
}

// validateLightProperties validates a light's structure and properties
func validateLightProperties(light LightRequest) error {
	// Validate basic fields
	if light.ID == "" {
		return fmt.Errorf("light ID cannot be empty")
	}
	if light.Type == "" {
		return fmt.Errorf("light type cannot be empty for light '%s'", light.ID)
	}
	if light.Properties == nil {
		return fmt.Errorf("light properties cannot be nil for light '%s'", light.ID)
	}

	// Validate type-specific properties
	switch light.Type {
	case "point_spot_light":
		// Required: center, emission
		// Optional: direction, cutoff_angle, falloff_exponent
		if err := validateRequiredProperty(light.Properties, "center", "point_spot_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "emission", "point_spot_light", light.ID); err != nil {
			return err
		}

		// Validate center is array of 3 numbers
		if err := validateFloatArray(light.Properties, "center", 3, nil, nil, light.ID); err != nil {
			return err
		}

		// Validate emission is array of 3 positive numbers
		zero := 0.0
		if err := validateFloatArray(light.Properties, "emission", 3, &zero, nil, light.ID); err != nil {
			return err
		}

		// Validate optional direction (if present)
		if hasProperty(light.Properties, "direction") {
			if err := validateFloatArray(light.Properties, "direction", 3, nil, nil, light.ID); err != nil {
				return err
			}
		}

		// Validate optional cutoff_angle (if present)
		if hasProperty(light.Properties, "cutoff_angle") {
			if angle, ok := extractFloat(light.Properties, "cutoff_angle"); ok {
				if angle <= 0 || angle > 180 {
					return fmt.Errorf("point_spot_light '%s' cutoff_angle must be between 0 and 180 degrees", light.ID)
				}
			} else {
				return fmt.Errorf("point_spot_light '%s' cutoff_angle must be a number", light.ID)
			}
		}

		// Validate optional falloff_exponent (if present)
		if hasProperty(light.Properties, "falloff_exponent") {
			if exponent, ok := extractFloat(light.Properties, "falloff_exponent"); ok {
				if exponent < 0 {
					return fmt.Errorf("point_spot_light '%s' falloff_exponent must be >= 0", light.ID)
				}
			} else {
				return fmt.Errorf("point_spot_light '%s' falloff_exponent must be a number", light.ID)
			}
		}

	case "area_quad_light":
		// Required: corner, u, v, emission
		if err := validateRequiredProperty(light.Properties, "corner", "area_quad_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "u", "area_quad_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "v", "area_quad_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "emission", "area_quad_light", light.ID); err != nil {
			return err
		}

		// Validate all are arrays of 3 numbers
		if err := validateFloatArray(light.Properties, "corner", 3, nil, nil, light.ID); err != nil {
			return err
		}
		if err := validateFloatArray(light.Properties, "u", 3, nil, nil, light.ID); err != nil {
			return err
		}
		if err := validateFloatArray(light.Properties, "v", 3, nil, nil, light.ID); err != nil {
			return err
		}

		// Validate emission is array of 3 positive numbers
		zero := 0.0
		if err := validateFloatArray(light.Properties, "emission", 3, &zero, nil, light.ID); err != nil {
			return err
		}

	case "area_disc_light":
		// Required: center, normal, radius, emission
		if err := validateRequiredProperty(light.Properties, "center", "area_disc_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "normal", "area_disc_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "radius", "area_disc_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "emission", "area_disc_light", light.ID); err != nil {
			return err
		}

		// Validate center and normal are arrays of 3 numbers
		if err := validateFloatArray(light.Properties, "center", 3, nil, nil, light.ID); err != nil {
			return err
		}
		if err := validateFloatArray(light.Properties, "normal", 3, nil, nil, light.ID); err != nil {
			return err
		}

		// Validate radius is a positive number
		if radius, ok := extractFloat(light.Properties, "radius"); ok {
			if radius <= 0 {
				return fmt.Errorf("area_disc_light '%s' radius must be positive", light.ID)
			}
		} else {
			return fmt.Errorf("area_disc_light '%s' radius must be a number", light.ID)
		}

		// Validate emission is array of 3 positive numbers
		zero := 0.0
		if err := validateFloatArray(light.Properties, "emission", 3, &zero, nil, light.ID); err != nil {
			return err
		}

	case "area_sphere_light":
		// Required: center, radius, emission
		if err := validateRequiredProperty(light.Properties, "center", "area_sphere_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "radius", "area_sphere_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "emission", "area_sphere_light", light.ID); err != nil {
			return err
		}

		// Validate center is array of 3 numbers
		if err := validateFloatArray(light.Properties, "center", 3, nil, nil, light.ID); err != nil {
			return err
		}

		// Validate radius is a positive number
		if radius, ok := extractFloat(light.Properties, "radius"); ok {
			if radius <= 0 {
				return fmt.Errorf("area_sphere_light '%s' radius must be positive", light.ID)
			}
		} else {
			return fmt.Errorf("area_sphere_light '%s' radius must be a number", light.ID)
		}

		// Validate emission is array of 3 positive numbers
		zero := 0.0
		if err := validateFloatArray(light.Properties, "emission", 3, &zero, nil, light.ID); err != nil {
			return err
		}

	case "area_disc_spot_light":
		// Required: center, normal, radius, emission, cutoff_angle, falloff_exponent
		if err := validateRequiredProperty(light.Properties, "center", "area_disc_spot_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "normal", "area_disc_spot_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "radius", "area_disc_spot_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "emission", "area_disc_spot_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "cutoff_angle", "area_disc_spot_light", light.ID); err != nil {
			return err
		}
		if err := validateRequiredProperty(light.Properties, "falloff_exponent", "area_disc_spot_light", light.ID); err != nil {
			return err
		}

		// Validate center and normal are arrays of 3 numbers
		if err := validateFloatArray(light.Properties, "center", 3, nil, nil, light.ID); err != nil {
			return err
		}
		if err := validateFloatArray(light.Properties, "normal", 3, nil, nil, light.ID); err != nil {
			return err
		}

		// Validate radius is a positive number
		if radius, ok := extractFloat(light.Properties, "radius"); ok {
			if radius <= 0 {
				return fmt.Errorf("area_disc_spot_light '%s' radius must be positive", light.ID)
			}
		} else {
			return fmt.Errorf("area_disc_spot_light '%s' radius must be a number", light.ID)
		}

		// Validate emission is array of 3 positive numbers
		zero := 0.0
		if err := validateFloatArray(light.Properties, "emission", 3, &zero, nil, light.ID); err != nil {
			return err
		}

		// Validate cutoff_angle
		if angle, ok := extractFloat(light.Properties, "cutoff_angle"); ok {
			if angle <= 0 || angle > 180 {
				return fmt.Errorf("area_disc_spot_light '%s' cutoff_angle must be between 0 and 180 degrees", light.ID)
			}
		} else {
			return fmt.Errorf("area_disc_spot_light '%s' cutoff_angle must be a number", light.ID)
		}

		// Validate falloff_exponent
		if exponent, ok := extractFloat(light.Properties, "falloff_exponent"); ok {
			if exponent < 0 {
				return fmt.Errorf("area_disc_spot_light '%s' falloff_exponent must be >= 0", light.ID)
			}
		} else {
			return fmt.Errorf("area_disc_spot_light '%s' falloff_exponent must be a number", light.ID)
		}

	default:
		return fmt.Errorf("unsupported light type '%s' for light '%s'", light.Type, light.ID)
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
		// Try to get position (try different property names)
		if centerArray, ok := extractFloatArray(props, "center", 3); ok {
			copy(position[:], centerArray) // For spheres, boxes, discs
		} else if cornerArray, ok := extractFloatArray(props, "corner", 3); ok {
			copy(position[:], cornerArray) // For quads
		}

		// Try to get size (radius for sphere/disc, dimensions for box, edge vectors for quad)
		if radius, ok := extractFloat(props, "radius"); ok {
			size = radius
		} else if sizeVal, ok := extractFloat(props, "size"); ok {
			size = sizeVal
		} else if dimsArray, ok := extractFloatArray(props, "dimensions", 3); ok {
			size = dimsArray[0] // Use first dimension as representative size
		} else if uArray, ok := extractFloatArray(props, "u", 3); ok {
			// For quads, use the length of the u vector as representative size
			size = math.Sqrt(uArray[0]*uArray[0] + uArray[1]*uArray[1] + uArray[2]*uArray[2])
		}
	}

	cameraDistance := size*3 + 5
	sm.state.Camera.Center = []float64{
		position[0],
		position[1],
		position[2] + cameraDistance,
	}
	sm.state.Camera.LookAt = []float64{position[0], position[1], position[2]}
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
			// Extract properties directly
			var center [3]float64
			var size float64 = 1.0
			var color [3]float64 = [3]float64{0.5, 0.5, 0.5}

			// Extract center (or corner for quads)
			if centerArray, ok := extractFloatArray(shape.Properties, "center", 3); ok {
				copy(center[:], centerArray)
			} else if cornerArray, ok := extractFloatArray(shape.Properties, "corner", 3); ok {
				copy(center[:], cornerArray) // Use corner as position for display
			}

			// Extract size/radius
			if radius, ok := extractFloat(shape.Properties, "radius"); ok {
				size = radius
			} else if dimsArray, ok := extractFloatArray(shape.Properties, "dimensions", 3); ok {
				size = dimsArray[0] // Use first dimension as representative size
			}

			// Extract color
			if colorArray, ok := extractFloatArray(shape.Properties, "color", 3); ok {
				copy(color[:], colorArray)
			}

			sceneContext += fmt.Sprintf("%s) %s (ID: %s) at [%.1f,%.1f,%.1f] size %.1f color [%.1f,%.1f,%.1f]",
				fmt.Sprintf("%d", i+1), shape.Type, shape.ID, center[0], center[1], center[2],
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
		Center:   []float64{0, 0, 5},
		LookAt:   []float64{0, 0, 0},
		VFov:     45.0,
		Aperture: 0.0,
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

// AddLights adds lights to the scene
func (sm *SceneManager) AddLights(lights []LightRequest) error {
	if len(lights) == 0 {
		return nil
	}

	// Validate unique IDs and light properties
	for _, newLight := range lights {
		// Validate light properties
		if err := validateLightProperties(newLight); err != nil {
			return err
		}

		// Check for ID uniqueness
		if sm.FindLight(newLight.ID) != nil {
			return fmt.Errorf("light with ID '%s' already exists", newLight.ID)
		}
	}

	// Add all lights if validation passes
	sm.state.Lights = append(sm.state.Lights, lights...)
	return nil
}

// FindLight returns a light by its ID, or nil if not found
func (sm *SceneManager) FindLight(id string) *LightRequest {
	for i := range sm.state.Lights {
		if sm.state.Lights[i].ID == id {
			return &sm.state.Lights[i]
		}
	}
	return nil
}

// UpdateLight updates an existing light with the provided changes
func (sm *SceneManager) UpdateLight(id string, updates map[string]interface{}) error {
	light := sm.FindLight(id)
	if light == nil {
		return fmt.Errorf("light with ID '%s' not found", id)
	}

	// Apply updates to the light
	for key, value := range updates {
		switch key {
		case "id":
			newID, ok := value.(string)
			if !ok {
				return fmt.Errorf("new ID must be a string")
			}

			// Check that new ID is unique (unless it's the same as current)
			if newID != light.ID && sm.FindLight(newID) != nil {
				return fmt.Errorf("light with ID '%s' already exists", newID)
			}

			light.ID = newID

		case "type":
			newType, ok := value.(string)
			if !ok {
				return fmt.Errorf("light type must be a string")
			}
			light.Type = newType

		case "properties":
			newProps, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("properties must be an object")
			}

			// Update properties by merging
			if light.Properties == nil {
				light.Properties = make(map[string]interface{})
			}
			for propKey, propValue := range newProps {
				light.Properties[propKey] = propValue
			}

		default:
			return fmt.Errorf("unknown field '%s' for light update", key)
		}
	}

	// Validate the updated light
	if err := validateLightProperties(*light); err != nil {
		return fmt.Errorf("updated light validation failed: %w", err)
	}

	return nil
}

// RemoveLight removes a light from the scene by its ID
func (sm *SceneManager) RemoveLight(id string) error {
	for i := range sm.state.Lights {
		if sm.state.Lights[i].ID == id {
			// Remove light by slicing
			sm.state.Lights = append(sm.state.Lights[:i], sm.state.Lights[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("light with ID '%s' not found", id)
}

// SetCamera updates the camera configuration
func (sm *SceneManager) SetCamera(camera CameraInfo) error {
	// Validate center is provided and has correct length
	if camera.Center == nil {
		return fmt.Errorf("camera center must be provided")
	}
	if len(camera.Center) != 3 {
		return fmt.Errorf("camera center must have exactly 3 values")
	}

	// Validate lookAt is provided and has correct length
	if camera.LookAt == nil {
		return fmt.Errorf("camera look_at must be provided")
	}
	if len(camera.LookAt) != 3 {
		return fmt.Errorf("camera look_at must have exactly 3 values")
	}

	// Validate center and lookAt are different
	if camera.Center[0] == camera.LookAt[0] &&
		camera.Center[1] == camera.LookAt[1] &&
		camera.Center[2] == camera.LookAt[2] {
		return fmt.Errorf("camera center and look_at cannot be the same point")
	}

	// Validate vfov is in reasonable range
	if camera.VFov <= 0 || camera.VFov >= 180 {
		return fmt.Errorf("vfov must be between 0 and 180 degrees")
	}

	// Validate aperture is non-negative
	if camera.Aperture < 0 {
		return fmt.Errorf("aperture must be >= 0")
	}

	// Update camera state
	sm.state.Camera = camera
	return nil
}

// SetEnvironmentLighting sets the background/environment lighting for the scene
func (sm *SceneManager) SetEnvironmentLighting(lightingType string, topColor, bottomColor, emission []float64) error {
	// Validate lighting type
	switch lightingType {
	case "gradient":
		if len(topColor) != 3 || len(bottomColor) != 3 {
			return fmt.Errorf("gradient lighting requires both top_color and bottom_color as [r,g,b] arrays")
		}
		// Validate color values
		for i, c := range topColor {
			if c < 0 {
				return fmt.Errorf("top_color[%d] must be >= 0", i)
			}
		}
		for i, c := range bottomColor {
			if c < 0 {
				return fmt.Errorf("bottom_color[%d] must be >= 0", i)
			}
		}

		// Remove any existing environment lights and add gradient
		sm.removeEnvironmentLights()

		// Convert to interface{} arrays for storage
		topColorInterface := make([]interface{}, len(topColor))
		for i, v := range topColor {
			topColorInterface[i] = v
		}
		bottomColorInterface := make([]interface{}, len(bottomColor))
		for i, v := range bottomColor {
			bottomColorInterface[i] = v
		}

		sm.state.Lights = append(sm.state.Lights, LightRequest{
			ID:   "environment_gradient",
			Type: "infinite_gradient_light",
			Properties: map[string]interface{}{
				"top_color":    topColorInterface,
				"bottom_color": bottomColorInterface,
			},
		})

	case "uniform":
		if len(emission) != 3 {
			return fmt.Errorf("uniform lighting requires emission as [r,g,b] array")
		}
		// Validate emission values
		for i, e := range emission {
			if e < 0 {
				return fmt.Errorf("emission[%d] must be >= 0", i)
			}
		}

		// Remove any existing environment lights and add uniform
		sm.removeEnvironmentLights()

		// Convert to interface{} array for storage
		emissionInterface := make([]interface{}, len(emission))
		for i, v := range emission {
			emissionInterface[i] = v
		}

		sm.state.Lights = append(sm.state.Lights, LightRequest{
			ID:   "environment_uniform",
			Type: "infinite_uniform_light",
			Properties: map[string]interface{}{
				"emission": emissionInterface,
			},
		})

	case "none":
		// Remove all environment lights
		sm.removeEnvironmentLights()

	default:
		return fmt.Errorf("unsupported environment lighting type: %s", lightingType)
	}

	return nil
}

// removeEnvironmentLights removes all infinite lights from the scene
func (sm *SceneManager) removeEnvironmentLights() {
	filtered := make([]LightRequest, 0, len(sm.state.Lights))
	for _, light := range sm.state.Lights {
		if light.Type != "infinite_gradient_light" && light.Type != "infinite_uniform_light" {
			filtered = append(filtered, light)
		}
	}
	sm.state.Lights = filtered
}

// addLightsToScene adds all lights from the scene state to the raytracer scene
func (sm *SceneManager) addLightsToScene(raytracerScene *scene.Scene) error {
	// If no lights are defined, add default gradient lighting
	if len(sm.state.Lights) == 0 {
		raytracerScene.AddGradientInfiniteLight(
			core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
			core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white horizon)
		)
		return nil
	}

	// Add lights from scene state
	for _, lightReq := range sm.state.Lights {
		err := sm.addLightToScene(raytracerScene, lightReq)
		if err != nil {
			return fmt.Errorf("failed to add light '%s': %w", lightReq.ID, err)
		}
	}

	return nil
}

// addLightToScene adds a single light to the raytracer scene
func (sm *SceneManager) addLightToScene(raytracerScene *scene.Scene, lightReq LightRequest) error {
	switch lightReq.Type {
	case "infinite_gradient_light":
		// Extract top and bottom colors
		topColor, ok := extractFloatArray(lightReq.Properties, "top_color", 3)
		if !ok {
			return fmt.Errorf("gradient light requires top_color property")
		}
		bottomColor, ok := extractFloatArray(lightReq.Properties, "bottom_color", 3)
		if !ok {
			return fmt.Errorf("gradient light requires bottom_color property")
		}

		raytracerScene.AddGradientInfiniteLight(
			core.NewVec3(topColor[0], topColor[1], topColor[2]),
			core.NewVec3(bottomColor[0], bottomColor[1], bottomColor[2]),
		)

	case "infinite_uniform_light":
		// Extract emission color
		emission, ok := extractFloatArray(lightReq.Properties, "emission", 3)
		if !ok {
			return fmt.Errorf("uniform light requires emission property")
		}

		raytracerScene.AddUniformInfiniteLight(
			core.NewVec3(emission[0], emission[1], emission[2]),
		)

	case "point_spot_light":
		// Extract required properties
		center, ok := extractFloatArray(lightReq.Properties, "center", 3)
		if !ok {
			return fmt.Errorf("point_spot_light requires center property")
		}
		emission, ok := extractFloatArray(lightReq.Properties, "emission", 3)
		if !ok {
			return fmt.Errorf("point_spot_light requires emission property")
		}

		// Extract optional properties
		direction, hasDirection := extractFloatArray(lightReq.Properties, "direction", 3)
		cutoffAngle, hasCutoff := extractFloat(lightReq.Properties, "cutoff_angle")
		falloffExponent, hasFalloff := extractFloat(lightReq.Properties, "falloff_exponent")

		// Set defaults for optional parameters
		if !hasDirection {
			direction = []float64{0, -1, 0} // Default downward direction
		}
		if !hasCutoff {
			cutoffAngle = 45.0 // Default 45 degree cone
		}
		if !hasFalloff {
			falloffExponent = 5.0 // Default sharp falloff
		}

		// Calculate target point from center and direction
		to := core.NewVec3(
			center[0]+direction[0],
			center[1]+direction[1],
			center[2]+direction[2],
		)

		raytracerScene.AddPointSpotLight(
			core.NewVec3(center[0], center[1], center[2]),
			to,
			core.NewVec3(emission[0], emission[1], emission[2]),
			cutoffAngle,
			falloffExponent,
			0.0, // Point light has no radius
		)

	case "area_quad_light":
		// Extract required properties
		corner, ok := extractFloatArray(lightReq.Properties, "corner", 3)
		if !ok {
			return fmt.Errorf("area_quad_light requires corner property")
		}
		u, ok := extractFloatArray(lightReq.Properties, "u", 3)
		if !ok {
			return fmt.Errorf("area_quad_light requires u property")
		}
		v, ok := extractFloatArray(lightReq.Properties, "v", 3)
		if !ok {
			return fmt.Errorf("area_quad_light requires v property")
		}
		emission, ok := extractFloatArray(lightReq.Properties, "emission", 3)
		if !ok {
			return fmt.Errorf("area_quad_light requires emission property")
		}

		raytracerScene.AddQuadLight(
			core.NewVec3(corner[0], corner[1], corner[2]),
			core.NewVec3(u[0], u[1], u[2]),
			core.NewVec3(v[0], v[1], v[2]),
			core.NewVec3(emission[0], emission[1], emission[2]),
		)

	case "area_disc_light":
		// For now, we'll create a disc light using spot light with wide angle
		// Extract required properties
		center, ok := extractFloatArray(lightReq.Properties, "center", 3)
		if !ok {
			return fmt.Errorf("area_disc_light requires center property")
		}
		normal, ok := extractFloatArray(lightReq.Properties, "normal", 3)
		if !ok {
			return fmt.Errorf("area_disc_light requires normal property")
		}
		radius, ok := extractFloat(lightReq.Properties, "radius")
		if !ok {
			return fmt.Errorf("area_disc_light requires radius property")
		}
		emission, ok := extractFloatArray(lightReq.Properties, "emission", 3)
		if !ok {
			return fmt.Errorf("area_disc_light requires emission property")
		}

		// Calculate target point from center and normal
		to := core.NewVec3(
			center[0]+normal[0],
			center[1]+normal[1],
			center[2]+normal[2],
		)

		// Use AddSpotLight with wide angle (170 degrees) to simulate disc area light
		raytracerScene.AddSpotLight(
			core.NewVec3(center[0], center[1], center[2]),
			to,
			core.NewVec3(emission[0], emission[1], emission[2]),
			170.0, // Wide cone angle to approximate disc area light
			2.0,   // Gentle falloff
			radius,
		)

	case "area_sphere_light":
		// Extract required properties
		center, ok := extractFloatArray(lightReq.Properties, "center", 3)
		if !ok {
			return fmt.Errorf("area_sphere_light requires center property")
		}
		radius, ok := extractFloat(lightReq.Properties, "radius")
		if !ok {
			return fmt.Errorf("area_sphere_light requires radius property")
		}
		emission, ok := extractFloatArray(lightReq.Properties, "emission", 3)
		if !ok {
			return fmt.Errorf("area_sphere_light requires emission property")
		}

		raytracerScene.AddSphereLight(
			core.NewVec3(center[0], center[1], center[2]),
			radius,
			core.NewVec3(emission[0], emission[1], emission[2]),
		)

	case "area_disc_spot_light":
		// Extract required properties
		center, ok := extractFloatArray(lightReq.Properties, "center", 3)
		if !ok {
			return fmt.Errorf("area_disc_spot_light requires center property")
		}
		normal, ok := extractFloatArray(lightReq.Properties, "normal", 3)
		if !ok {
			return fmt.Errorf("area_disc_spot_light requires normal property")
		}
		radius, ok := extractFloat(lightReq.Properties, "radius")
		if !ok {
			return fmt.Errorf("area_disc_spot_light requires radius property")
		}
		emission, ok := extractFloatArray(lightReq.Properties, "emission", 3)
		if !ok {
			return fmt.Errorf("area_disc_spot_light requires emission property")
		}
		cutoffAngle, ok := extractFloat(lightReq.Properties, "cutoff_angle")
		if !ok {
			return fmt.Errorf("area_disc_spot_light requires cutoff_angle property")
		}
		falloffExponent, ok := extractFloat(lightReq.Properties, "falloff_exponent")
		if !ok {
			return fmt.Errorf("area_disc_spot_light requires falloff_exponent property")
		}

		// Calculate target point from center and normal
		to := core.NewVec3(
			center[0]+normal[0],
			center[1]+normal[1],
			center[2]+normal[2],
		)

		raytracerScene.AddSpotLight(
			core.NewVec3(center[0], center[1], center[2]),
			to,
			core.NewVec3(emission[0], emission[1], emission[2]),
			cutoffAngle,
			falloffExponent,
			radius,
		)

	default:
		return fmt.Errorf("unsupported light type: %s", lightReq.Type)
	}

	return nil
}

// ToRaytracerScene converts the scene state to a raytracer scene
func (sm *SceneManager) ToRaytracerScene() (*scene.Scene, error) {
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
		Center:        core.NewVec3(sm.state.Camera.Center[0], sm.state.Camera.Center[1], sm.state.Camera.Center[2]),
		LookAt:        core.NewVec3(sm.state.Camera.LookAt[0], sm.state.Camera.LookAt[1], sm.state.Camera.LookAt[2]),
		Up:            core.NewVec3(0, 1, 0),
		VFov:          sm.state.Camera.VFov,
		Width:         samplingConfig.Width,
		AspectRatio:   float64(samplingConfig.Width) / float64(samplingConfig.Height),
		Aperture:      sm.state.Camera.Aperture,
		FocusDistance: 0.0, // TODO: add focus distance control
	}
	camera := geometry.NewCamera(cameraConfig)

	// Create shapes
	var sceneShapes []geometry.Shape
	for _, shapeReq := range sm.state.Shapes {
		// Extract common properties
		var size float64 = 1.0 // Default size

		// Extract size/radius (used for default values)
		if radius, ok := extractFloat(shapeReq.Properties, "radius"); ok {
			size = radius
		} else if dimsArray, ok := extractFloatArray(shapeReq.Properties, "dimensions", 3); ok {
			size = dimsArray[0] // Use first dimension as representative size
		}

		// Create material from shape properties
		var shapeMaterial material.Material
		if mat, hasMaterial := extractMaterial(shapeReq.Properties); hasMaterial {
			// Extract material from shape properties
			matType, _ := mat["type"].(string)
			switch matType {
			case "lambertian":
				albedo, _ := extractFloatArray(mat, "albedo", 3)
				shapeMaterial = material.NewLambertian(core.NewVec3(albedo[0], albedo[1], albedo[2]))
			case "metal":
				albedo, _ := extractFloatArray(mat, "albedo", 3)
				fuzz, _ := extractFloat(mat, "fuzz")
				shapeMaterial = material.NewMetal(core.NewVec3(albedo[0], albedo[1], albedo[2]), fuzz)
			case "dielectric":
				refractiveIndex, _ := extractFloat(mat, "refractive_index")
				shapeMaterial = material.NewDielectric(refractiveIndex)
			default:
				// Unknown material type - use default gray Lambertian
				shapeMaterial = material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
			}
		} else {
			// No material specified - use default gray Lambertian
			shapeMaterial = material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
		}

		// Create geometry based on type
		var shape geometry.Shape
		switch shapeReq.Type {
		case "sphere":
			// Extract center
			var center [3]float64
			if centerArray, ok := extractFloatArray(shapeReq.Properties, "center", 3); ok {
				copy(center[:], centerArray)
			}

			shape = geometry.NewSphere(
				core.NewVec3(center[0], center[1], center[2]),
				size,
				shapeMaterial,
			)
		case "box":
			// Extract center
			var center [3]float64
			if centerArray, ok := extractFloatArray(shapeReq.Properties, "center", 3); ok {
				copy(center[:], centerArray)
			}

			// Extract dimensions
			var dimensions [3]float64
			if dimsArray, ok := extractFloatArray(shapeReq.Properties, "dimensions", 3); ok {
				// Convert to half-extents
				dimensions[0] = dimsArray[0] / 2.0
				dimensions[1] = dimsArray[1] / 2.0
				dimensions[2] = dimsArray[2] / 2.0
			}

			// Check for optional rotation (in radians)
			var rotation [3]float64
			hasRotation := false
			if rotArray, ok := extractFloatArray(shapeReq.Properties, "rotation", 3); ok {
				copy(rotation[:], rotArray)
				hasRotation = true
			}

			if hasRotation {
				shape = geometry.NewBox(
					core.NewVec3(center[0], center[1], center[2]),
					core.NewVec3(dimensions[0], dimensions[1], dimensions[2]),
					core.NewVec3(rotation[0], rotation[1], rotation[2]),
					shapeMaterial,
				)
			} else {
				shape = geometry.NewAxisAlignedBox(
					core.NewVec3(center[0], center[1], center[2]),
					core.NewVec3(dimensions[0], dimensions[1], dimensions[2]),
					shapeMaterial,
				)
			}
		case "quad":
			// Extract corner, u, and v vectors
			var corner, u, v [3]float64
			if cornerArray, ok := extractFloatArray(shapeReq.Properties, "corner", 3); ok {
				copy(corner[:], cornerArray)
			}

			if uArray, ok := extractFloatArray(shapeReq.Properties, "u", 3); ok {
				copy(u[:], uArray)
			} else {
				// Default u vector (right direction)
				u = [3]float64{size, 0, 0}
			}

			if vArray, ok := extractFloatArray(shapeReq.Properties, "v", 3); ok {
				copy(v[:], vArray)
			} else {
				// Default v vector (up direction)
				v = [3]float64{0, size, 0}
			}

			shape = geometry.NewQuad(
				core.NewVec3(corner[0], corner[1], corner[2]),
				core.NewVec3(u[0], u[1], u[2]),
				core.NewVec3(v[0], v[1], v[2]),
				shapeMaterial,
			)
		case "disc":
			// Extract center, normal, and radius
			var center, normal [3]float64
			var radius float64

			if centerArray, ok := extractFloatArray(shapeReq.Properties, "center", 3); ok {
				copy(center[:], centerArray)
			}

			if normalArray, ok := extractFloatArray(shapeReq.Properties, "normal", 3); ok {
				copy(normal[:], normalArray)
			} else {
				// Default normal (up direction)
				normal = [3]float64{0, 1, 0}
			}

			if r, ok := extractFloat(shapeReq.Properties, "radius"); ok {
				radius = r
			}

			shape = geometry.NewDisc(
				core.NewVec3(center[0], center[1], center[2]),
				core.NewVec3(normal[0], normal[1], normal[2]),
				radius,
				shapeMaterial,
			)
		default:
			return nil, fmt.Errorf("unsupported shape type: %s", shapeReq.Type)
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

	// Add lights from scene state
	err := sm.addLightsToScene(sceneWithShapes)
	if err != nil {
		return nil, fmt.Errorf("failed to add lights to scene: %w", err)
	}

	return sceneWithShapes, nil
}
