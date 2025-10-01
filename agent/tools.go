package agent

import (
	"google.golang.org/genai"
)

// Helper functions for extracting values from function call arguments

// extractStringArg extracts a string argument from function call args
func extractStringArg(args map[string]interface{}, key string) (string, bool) {
	if val, ok := args[key].(string); ok {
		return val, true
	}
	return "", false
}

// extractMapArg extracts a map argument from function call args
func extractMapArg(args map[string]interface{}, key string) (map[string]interface{}, bool) {
	if val, ok := args[key].(map[string]interface{}); ok {
		return val, true
	}
	return nil, false
}

// ShapeRequest represents a shape creation/update request from the LLM
type ShapeRequest struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// LightRequest represents a light creation/update request from the LLM
type LightRequest struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// SceneState represents the current 3D scene state
type SceneState struct {
	Shapes []ShapeRequest `json:"shapes"`
	Lights []LightRequest `json:"lights"`
	Camera CameraInfo     `json:"camera"`
}

// CameraInfo represents camera information
type CameraInfo struct {
	Position [3]float64 `json:"position"`
	LookAt   [3]float64 `json:"look_at"`
}

// createShapeToolDeclaration returns the function declaration for shape creation
func createShapeToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "create_shape",
		Description: "Create a 3D shape in the scene with a unique ID",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"id": {
					Type:        genai.TypeString,
					Description: "Unique identifier for the shape (e.g., 'blue_sphere', 'main_building')",
				},
				"type": {
					Type:        genai.TypeString,
					Enum:        []string{"sphere", "box", "quad", "disc"},
					Description: "The type of shape to create",
				},
				"properties": {
					Type:        genai.TypeObject,
					Description: "Shape-specific properties including optional material. For sphere: {center: [x,y,z], radius: number, material?: {...}}. For box: {center: [x,y,z], dimensions: [w,h,d], rotation?: [x,y,z], material?: {...}}. For quad: {corner: [x,y,z], u: [x,y,z], v: [x,y,z], material?: {...}}. For disc: {center: [x,y,z], normal: [x,y,z], radius: number, material?: {...}}. Material defaults to gray lambertian if not specified. Materials: Lambertian {type: 'lambertian', albedo: [r,g,b]}, Metal {type: 'metal', albedo: [r,g,b], fuzz: 0.0-1.0}, Dielectric {type: 'dielectric', refractive_index: number (1.0=air, 1.33=water, 1.5=glass, 2.4=diamond)}",
				},
			},
			Required: []string{"id", "type", "properties"},
		},
	}
}

// updateShapeToolDeclaration returns the function declaration for shape updating
func updateShapeToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "update_shape",
		Description: "Update an existing shape by ID. Can update the shape's ID, type, or any properties like color, position, size, etc.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"id": {
					Type:        genai.TypeString,
					Description: "ID of the shape to update",
				},
				"updates": {
					Type:        genai.TypeObject,
					Description: "Object containing fields to update. Examples: {\"id\": \"new_name\"} to rename, {\"properties\": {\"position\": [1, 2, 3]}} to move shape, {\"properties\": {\"material\": {\"type\": \"metal\", \"albedo\": [0.9, 0.9, 0.9], \"fuzz\": 0.1}}} to make metallic, {\"properties\": {\"material\": {\"type\": \"dielectric\", \"refractive_index\": 1.5}}} to make glass. Only specified fields will be updated.",
				},
			},
			Required: []string{"id", "updates"},
		},
	}
}

// removeShapeToolDeclaration returns the function declaration for shape removal
func removeShapeToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "remove_shape",
		Description: "Remove a shape from the scene by its ID",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"id": {
					Type:        genai.TypeString,
					Description: "ID of the shape to remove",
				},
			},
			Required: []string{"id"},
		},
	}
}

// createLightToolDeclaration returns the function declaration for light creation
func createLightToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "create_light",
		Description: "Create a positioned light in the scene with a unique ID",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"id": {
					Type:        genai.TypeString,
					Description: "Unique identifier for the light (e.g., 'main_light', 'ceiling_lamp')",
				},
				"type": {
					Type:        genai.TypeString,
					Enum:        []string{"point_spot_light", "area_quad_light", "area_disc_light", "area_sphere_light", "area_disc_spot_light"},
					Description: "The type of light to create",
				},
				"properties": {
					Type:        genai.TypeObject,
					Description: "Light-specific properties. For point_spot_light: {center: [x,y,z], emission: [r,g,b], direction: [x,y,z] (optional), cutoff_angle: degrees (optional), falloff_exponent: number (optional)}. For area_quad_light: {corner: [x,y,z], u: [x,y,z], v: [x,y,z], emission: [r,g,b]}. For area_disc_light: {center: [x,y,z], normal: [x,y,z], radius: number, emission: [r,g,b]}. For area_sphere_light: {center: [x,y,z], radius: number, emission: [r,g,b]}. For area_disc_spot_light: {center: [x,y,z], normal: [x,y,z], radius: number, emission: [r,g,b], cutoff_angle: degrees, falloff_exponent: number}",
				},
			},
			Required: []string{"id", "type", "properties"},
		},
	}
}

// setEnvironmentLightingToolDeclaration returns the function declaration for environment lighting
func setEnvironmentLightingToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "set_environment_lighting",
		Description: "Set the background/environment lighting for the scene. This replaces any existing environment lighting.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"type": {
					Type:        genai.TypeString,
					Enum:        []string{"gradient", "uniform", "none"},
					Description: "Type of environment lighting",
				},
				"top_color": {
					Type:        genai.TypeArray,
					Description: "RGB color for gradient top/zenith [r,g,b] (0.0-10.0+). Required for gradient type.",
					Items: &genai.Schema{
						Type: genai.TypeNumber,
					},
				},
				"bottom_color": {
					Type:        genai.TypeArray,
					Description: "RGB color for gradient bottom/horizon [r,g,b] (0.0-10.0+). Required for gradient type.",
					Items: &genai.Schema{
						Type: genai.TypeNumber,
					},
				},
				"emission": {
					Type:        genai.TypeArray,
					Description: "RGB emission color [r,g,b] (0.0-10.0+). Required for uniform type.",
					Items: &genai.Schema{
						Type: genai.TypeNumber,
					},
				},
			},
			Required: []string{"type"},
		},
	}
}

// updateLightToolDeclaration returns the function declaration for light updating
func updateLightToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "update_light",
		Description: "Update an existing light by ID. Can update the light's ID, type, or any properties like emission, position, size, etc.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"id": {
					Type:        genai.TypeString,
					Description: "ID of the light to update",
				},
				"updates": {
					Type:        genai.TypeObject,
					Description: "Object containing fields to update. Examples: {\"id\": \"new_name\"} to rename, {\"properties\": {\"emission\": [2.0, 1.0, 0.5]}} to change emission to warm orange, {\"properties\": {\"center\": [1, 2, 3]}} to move light. Only specified fields will be updated.",
				},
			},
			Required: []string{"id", "updates"},
		},
	}
}

// removeLightToolDeclaration returns the function declaration for light removal
func removeLightToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "remove_light",
		Description: "Remove a light from the scene by its ID",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"id": {
					Type:        genai.TypeString,
					Description: "ID of the light to remove",
				},
			},
			Required: []string{"id"},
		},
	}
}

// getAllToolDeclarations returns all available tool declarations
func getAllToolDeclarations() []*genai.FunctionDeclaration {
	return []*genai.FunctionDeclaration{
		createShapeToolDeclaration(),
		updateShapeToolDeclaration(),
		removeShapeToolDeclaration(),
		createLightToolDeclaration(),
		updateLightToolDeclaration(),
		removeLightToolDeclaration(),
		setEnvironmentLightingToolDeclaration(),
	}
}

// parseShapeFromFunctionCall extracts a ShapeRequest from a function call
func parseShapeFromFunctionCall(call *genai.FunctionCall) *ShapeRequest {
	if call.Name != "create_shape" {
		return nil
	}

	var shape ShapeRequest
	args := call.Args

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		shape.ID = id
	}

	// Extract type
	if shapeType, ok := extractStringArg(args, "type"); ok {
		shape.Type = shapeType
	}

	// Extract properties as-is (let SceneManager validate them)
	if props, ok := extractMapArg(args, "properties"); ok {
		shape.Properties = props
	}

	return &shape
}

// ShapeUpdate represents an update request for an existing shape
type ShapeUpdate struct {
	ID      string                 `json:"id"`      // ID of shape to update
	Updates map[string]interface{} `json:"updates"` // Fields to update
}

// parseUpdateFromFunctionCall extracts a ShapeUpdate from an update_shape function call
func parseUpdateFromFunctionCall(call *genai.FunctionCall) *ShapeUpdate {
	if call.Name != "update_shape" {
		return nil
	}

	var update ShapeUpdate
	args := call.Args

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		update.ID = id
	}

	// Extract updates
	if updates, ok := extractMapArg(args, "updates"); ok {
		update.Updates = updates
	}

	return &update
}

// parseRemoveFromFunctionCall extracts shape ID from a remove_shape function call
func parseRemoveFromFunctionCall(call *genai.FunctionCall) string {
	if call.Name != "remove_shape" {
		return ""
	}

	args := call.Args
	if id, ok := extractStringArg(args, "id"); ok {
		return id
	}

	return ""
}

// New parsing functions that create ToolOperation objects

// parseToolOperationFromFunctionCall creates a ToolOperation from any function call
func parseToolOperationFromFunctionCall(call *genai.FunctionCall) ToolOperation {
	switch call.Name {
	case "create_shape":
		return parseCreateShapeOperation(call)
	case "update_shape":
		return parseUpdateShapeOperation(call)
	case "remove_shape":
		return parseRemoveShapeOperation(call)
	case "create_light":
		return parseCreateLightOperation(call)
	case "update_light":
		return parseUpdateLightOperation(call)
	case "remove_light":
		return parseRemoveLightOperation(call)
	case "set_environment_lighting":
		return parseSetEnvironmentLightingOperation(call)
	default:
		return nil
	}
}

// parseCreateShapeOperation creates a CreateShapeOperation from a create_shape function call
func parseCreateShapeOperation(call *genai.FunctionCall) *CreateShapeOperation {
	if call.Name != "create_shape" {
		return nil
	}

	var shape ShapeRequest
	args := call.Args

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		shape.ID = id
	}

	// Extract type
	if shapeType, ok := extractStringArg(args, "type"); ok {
		shape.Type = shapeType
	}

	// Extract properties as-is (let SceneManager validate them)
	if props, ok := extractMapArg(args, "properties"); ok {
		shape.Properties = props
	}

	return &CreateShapeOperation{
		Shape:    shape,
		ToolType: "create_shape",
	}
}

// parseUpdateShapeOperation creates an UpdateShapeOperation from an update_shape function call
func parseUpdateShapeOperation(call *genai.FunctionCall) *UpdateShapeOperation {
	if call.Name != "update_shape" {
		return nil
	}

	args := call.Args
	operation := &UpdateShapeOperation{
		ToolType: "update_shape",
	}

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		operation.ID = id
	}

	// Extract updates
	if updates, ok := extractMapArg(args, "updates"); ok {
		operation.Updates = updates
	}

	return operation
}

// parseRemoveShapeOperation creates a RemoveShapeOperation from a remove_shape function call
func parseRemoveShapeOperation(call *genai.FunctionCall) *RemoveShapeOperation {
	if call.Name != "remove_shape" {
		return nil
	}

	args := call.Args
	operation := &RemoveShapeOperation{
		ToolType: "remove_shape",
	}

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		operation.ID = id
	}

	return operation
}

// parseSetEnvironmentLightingOperation creates a SetEnvironmentLightingOperation from a set_environment_lighting function call
func parseSetEnvironmentLightingOperation(call *genai.FunctionCall) *SetEnvironmentLightingOperation {
	if call.Name != "set_environment_lighting" {
		return nil
	}

	args := call.Args
	operation := &SetEnvironmentLightingOperation{
		ToolType: "set_environment_lighting",
	}

	// Extract lighting type
	if lightingType, ok := extractStringArg(args, "type"); ok {
		operation.LightingType = lightingType
	}

	// Extract optional color arrays
	if topColorInterface, ok := args["top_color"].([]interface{}); ok {
		var topColor []float64
		for _, val := range topColorInterface {
			if f, ok := val.(float64); ok {
				topColor = append(topColor, f)
			}
		}
		operation.TopColor = topColor
	}

	if bottomColorInterface, ok := args["bottom_color"].([]interface{}); ok {
		var bottomColor []float64
		for _, val := range bottomColorInterface {
			if f, ok := val.(float64); ok {
				bottomColor = append(bottomColor, f)
			}
		}
		operation.BottomColor = bottomColor
	}

	if emissionInterface, ok := args["emission"].([]interface{}); ok {
		var emission []float64
		for _, val := range emissionInterface {
			if f, ok := val.(float64); ok {
				emission = append(emission, f)
			}
		}
		operation.Emission = emission
	}

	return operation
}

// parseCreateLightOperation creates a CreateLightOperation from a create_light function call
func parseCreateLightOperation(call *genai.FunctionCall) *CreateLightOperation {
	if call.Name != "create_light" {
		return nil
	}

	var light LightRequest
	args := call.Args

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		light.ID = id
	}

	// Extract type
	if lightType, ok := extractStringArg(args, "type"); ok {
		light.Type = lightType
	}

	// Extract properties as-is (let SceneManager validate them)
	if props, ok := extractMapArg(args, "properties"); ok {
		light.Properties = props
	}

	return &CreateLightOperation{
		Light:    light,
		ToolType: "create_light",
	}
}

// parseUpdateLightOperation creates an UpdateLightOperation from an update_light function call
func parseUpdateLightOperation(call *genai.FunctionCall) *UpdateLightOperation {
	if call.Name != "update_light" {
		return nil
	}

	args := call.Args
	operation := &UpdateLightOperation{
		ToolType: "update_light",
	}

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		operation.ID = id
	}

	// Extract updates
	if updates, ok := extractMapArg(args, "updates"); ok {
		operation.Updates = updates
	}

	return operation
}

// parseRemoveLightOperation creates a RemoveLightOperation from a remove_light function call
func parseRemoveLightOperation(call *genai.FunctionCall) *RemoveLightOperation {
	if call.Name != "remove_light" {
		return nil
	}

	args := call.Args
	operation := &RemoveLightOperation{
		ToolType: "remove_light",
	}

	// Extract ID
	if id, ok := extractStringArg(args, "id"); ok {
		operation.ID = id
	}

	return operation
}
