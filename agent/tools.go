package agent

import (
	"google.golang.org/genai"
)

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
					Description: "Shape-specific properties. For sphere: {position: [x,y,z], radius: number, color: [r,g,b]}. For box: {position: [x,y,z], dimensions: [w,h,d], color: [r,g,b]}",
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
	if idVal, ok := args["id"].(string); ok {
		shape.ID = idVal
	}

	// Extract type
	if typeVal, ok := args["type"].(string); ok {
		shape.Type = typeVal
	}

	// Extract properties as-is (let SceneManager validate them)
	if propsVal, ok := args["properties"].(map[string]interface{}); ok {
		shape.Properties = propsVal
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
	if idVal, ok := args["id"].(string); ok {
		update.ID = idVal
	}

	// Extract updates
	if updatesVal, ok := args["updates"].(map[string]interface{}); ok {
		update.Updates = updatesVal
	}

	return &update
}

// parseRemoveFromFunctionCall extracts shape ID from a remove_shape function call
func parseRemoveFromFunctionCall(call *genai.FunctionCall) string {
	if call.Name != "remove_shape" {
		return ""
	}

	args := call.Args
	if idVal, ok := args["id"].(string); ok {
		return idVal
	}

	return ""
}
