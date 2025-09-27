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

// SceneState represents the current 3D scene state
type SceneState struct {
	Shapes []ShapeRequest `json:"shapes"`
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
					Enum:        []string{"sphere", "box"},
					Description: "The type of shape to create",
				},
				"properties": {
					Type:        genai.TypeObject,
					Description: "Shape-specific properties. For sphere: {position: [x,y,z], radius: number, color: [r,g,b]}. For box: {position: [x,y,z], dimensions: [w,h,d], color: [r,g,b], rotation: [x,y,z] (optional, degrees)}",
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
					Description: "Object containing fields to update. Examples: {\"id\": \"new_name\"} to rename, {\"properties\": {\"color\": [1.0, 0.0, 1.0]}} to change color to purple, {\"properties\": {\"position\": [1, 2, 3]}} to move shape. Only specified fields will be updated.",
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

// getAllToolDeclarations returns all available tool declarations
func getAllToolDeclarations() []*genai.FunctionDeclaration {
	return []*genai.FunctionDeclaration{
		createShapeToolDeclaration(),
		updateShapeToolDeclaration(),
		removeShapeToolDeclaration(),
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
