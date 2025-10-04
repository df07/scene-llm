package agent

import (
	"google.golang.org/genai"
)

// ------------------------------------------------------------
// Request types - raw data from LLM function calls
// ------------------------------------------------------------

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

// ------------------------------------------------------------
// Tool request types - structured requests for execution
// ------------------------------------------------------------

type BaseToolRequest struct {
	ToolType string `json:"tool_name"`    // For JSON serialization
	Id       string `json:"id,omitempty"` // Optional target ID (shape/light ID, or special like "camera")
}

// ToolName returns the tool type from the BaseToolRequest
func (op BaseToolRequest) ToolName() string { return op.ToolType }

// Target returns the target ID from the BaseToolRequest
func (op BaseToolRequest) Target() string { return op.Id }

// Concrete tool requests - pure data structures describing LLM intentions
type CreateShapeRequest struct {
	BaseToolRequest
	Shape ShapeRequest `json:"shape"`
}

type UpdateShapeRequest struct {
	BaseToolRequest
	Updates map[string]interface{} `json:"updates"`
	Before  *ShapeRequest          `json:"before,omitempty"` // Populated by agent after execution
	After   *ShapeRequest          `json:"after,omitempty"`  // Populated by agent after execution
}

type RemoveShapeRequest struct {
	BaseToolRequest
	RemovedShape *ShapeRequest `json:"removed_shape,omitempty"` // Populated by agent after execution
}

type SetEnvironmentLightingRequest struct {
	BaseToolRequest
	LightingType string    `json:"lighting_type"`
	TopColor     []float64 `json:"top_color,omitempty"`
	BottomColor  []float64 `json:"bottom_color,omitempty"`
	Emission     []float64 `json:"emission,omitempty"`
}

type CreateLightRequest struct {
	BaseToolRequest
	Light LightRequest `json:"light"`
}

type UpdateLightRequest struct {
	BaseToolRequest
	Updates map[string]interface{} `json:"updates"`
	Before  *LightRequest          `json:"before,omitempty"` // Populated by agent after execution
	After   *LightRequest          `json:"after,omitempty"`  // Populated by agent after execution
}

type RemoveLightRequest struct {
	BaseToolRequest
	RemovedLight *LightRequest `json:"removed_light,omitempty"` // Populated by agent after execution
}

type SetCameraRequest struct {
	BaseToolRequest
	Camera CameraInfo `json:"camera"`
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
		setCameraToolDeclaration(),
	}
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

func setCameraToolDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "set_camera",
		Description: "Set camera position and properties for viewing the scene",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"center": {
					Type:        genai.TypeArray,
					Description: "Camera position as [x, y, z]",
					Items: &genai.Schema{
						Type: genai.TypeNumber,
					},
				},
				"look_at": {
					Type:        genai.TypeArray,
					Description: "Point the camera looks at as [x, y, z]",
					Items: &genai.Schema{
						Type: genai.TypeNumber,
					},
				},
				"vfov": {
					Type:        genai.TypeNumber,
					Description: "Vertical field of view in degrees (default: 45.0)",
				},
				"aperture": {
					Type:        genai.TypeNumber,
					Description: "Lens aperture for depth of field effect (0.0 = no blur, default: 0.0)",
				},
			},
		},
	}
}

// ------------------------------------------------------------
// Parsing functions - convert LLM function calls to requests
// ------------------------------------------------------------

// parseToolRequestFromFunctionCall creates a ToolRequest from any function call
func parseToolRequestFromFunctionCall(call *genai.FunctionCall) ToolRequest {
	switch call.Name {
	case "create_shape":
		return parseCreateShapeRequest(call)
	case "update_shape":
		return parseUpdateShapeRequest(call)
	case "remove_shape":
		return parseRemoveShapeRequest(call)
	case "create_light":
		return parseCreateLightRequest(call)
	case "update_light":
		return parseUpdateLightRequest(call)
	case "remove_light":
		return parseRemoveLightRequest(call)
	case "set_environment_lighting":
		return parseSetEnvironmentLightingRequest(call)
	case "set_camera":
		return parseSetCameraRequest(call)
	default:
		return nil
	}
}

// parseCreateShapeRequest creates a CreateShapeRequest from a create_shape function call
func parseCreateShapeRequest(call *genai.FunctionCall) *CreateShapeRequest {
	shape := extractShapeRequest(call.Args)

	return &CreateShapeRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "create_shape", Id: shape.ID},
		Shape:           shape,
	}
}

// parseUpdateShapeRequest creates an UpdateShapeRequest from an update_shape function call
func parseUpdateShapeRequest(call *genai.FunctionCall) *UpdateShapeRequest {
	id, _ := extractStringArg(call.Args, "id")
	updates, _ := extractMapArg(call.Args, "updates")

	return &UpdateShapeRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "update_shape", Id: id},
		Updates:         updates,
	}
}

// parseRemoveShapeRequest creates a RemoveShapeRequest from a remove_shape function call
func parseRemoveShapeRequest(call *genai.FunctionCall) *RemoveShapeRequest {
	id, _ := extractStringArg(call.Args, "id")

	return &RemoveShapeRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "remove_shape", Id: id},
	}
}

// parseSetEnvironmentLightingRequest creates a SetEnvironmentLightingRequest from a set_environment_lighting function call
func parseSetEnvironmentLightingRequest(call *genai.FunctionCall) *SetEnvironmentLightingRequest {
	lightingType, _ := extractStringArg(call.Args, "type")
	topColor, _ := extractFloatArrayArg(call.Args, "top_color")
	bottomColor, _ := extractFloatArrayArg(call.Args, "bottom_color")
	emission, _ := extractFloatArrayArg(call.Args, "emission")

	return &SetEnvironmentLightingRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "set_environment_lighting"},
		LightingType:    lightingType,
		TopColor:        topColor,
		BottomColor:     bottomColor,
		Emission:        emission,
	}
}

// parseCreateLightRequest creates a CreateLightRequest from a create_light function call
func parseCreateLightRequest(call *genai.FunctionCall) *CreateLightRequest {
	light := extractLightRequest(call.Args)

	return &CreateLightRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "create_light"},
		Light:           light,
	}
}

// parseUpdateLightRequest creates an UpdateLightRequest from an update_light function call
func parseUpdateLightRequest(call *genai.FunctionCall) *UpdateLightRequest {
	id, _ := extractStringArg(call.Args, "id")
	updates, _ := extractMapArg(call.Args, "updates")

	return &UpdateLightRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "update_light", Id: id},
		Updates:         updates,
	}
}

// parseRemoveLightRequest creates a RemoveLightRequest from a remove_light function call
func parseRemoveLightRequest(call *genai.FunctionCall) *RemoveLightRequest {
	id, _ := extractStringArg(call.Args, "id")

	return &RemoveLightRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "remove_light", Id: id},
	}
}

func parseSetCameraRequest(call *genai.FunctionCall) *SetCameraRequest {
	center, _ := extractFloatArrayArg(call.Args, "center")
	lookAt, _ := extractFloatArrayArg(call.Args, "look_at")
	vfov, hasVFov := extractFloatArg(call.Args, "vfov")
	aperture, _ := extractFloatArg(call.Args, "aperture")

	// Apply defaults for optional parameters
	if !hasVFov || vfov == 0 {
		vfov = 45.0
	}
	// aperture defaults to 0.0 (already handled by zero value)

	return &SetCameraRequest{
		BaseToolRequest: BaseToolRequest{ToolType: "set_camera"},
		Camera: CameraInfo{
			Center:   center,
			LookAt:   lookAt,
			VFov:     vfov,
			Aperture: aperture,
		},
	}
}

// ------------------------------------------------------------
// Helper functions
// ------------------------------------------------------------

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

func extractFloatArg(args map[string]interface{}, key string) (float64, bool) {
	if val, ok := args[key].(float64); ok {
		return val, true
	}
	return 0, false
}

func extractFloatArrayArg(args map[string]interface{}, key string) ([]float64, bool) {
	// Handle []float64 directly
	if val, ok := args[key].([]float64); ok {
		return val, true
	}

	// Handle []interface{} (from JSON/function calls)
	if val, ok := args[key].([]interface{}); ok {
		result := make([]float64, len(val))
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

func extractFloat3ArrayArg(args map[string]interface{}, key string) ([3]float64, bool) {
	// Handle [3]float64 directly
	if val, ok := args[key].([3]float64); ok {
		return val, true
	}

	// Handle []interface{} (from JSON/function calls)
	if val, ok := args[key].([]interface{}); ok && len(val) == 3 {
		var result [3]float64
		for i, v := range val {
			if f, ok := v.(float64); ok {
				result[i] = f
			} else {
				return [3]float64{}, false
			}
		}
		return result, true
	}

	return [3]float64{}, false
}

func extractShapeRequest(args map[string]interface{}) ShapeRequest {
	shape := ShapeRequest{}
	shape.ID, _ = extractStringArg(args, "id")
	shape.Type, _ = extractStringArg(args, "type")
	shape.Properties, _ = extractMapArg(args, "properties")
	return shape
}

func extractLightRequest(args map[string]interface{}) LightRequest {
	light := LightRequest{}
	light.ID, _ = extractStringArg(args, "id")
	light.Type, _ = extractStringArg(args, "type")
	light.Properties, _ = extractMapArg(args, "properties")
	return light
}
