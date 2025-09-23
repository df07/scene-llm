package agent

import (
	"google.golang.org/genai"
)

// ShapeRequest represents a shape creation request from the LLM
type ShapeRequest struct {
	Type     string     `json:"type"`
	Position [3]float64 `json:"position"`
	Size     float64    `json:"size"`
	Color    [3]float64 `json:"color"`
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
		Description: "Create a 3D shape in the scene",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"type": {
					Type:        genai.TypeString,
					Enum:        []string{"sphere", "box"},
					Description: "The type of shape to create",
				},
				"position": {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeNumber},
					Description: "The position of the shape as [x, y, z]",
				},
				"size": {
					Type:        genai.TypeNumber,
					Description: "The size of the shape (radius for sphere, side length for cube)",
				},
				"color": {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeNumber},
					Description: "RGB color values as [r, g, b] where each value is between 0.0 and 1.0",
				},
			},
			Required: []string{"type", "position", "size", "color"},
		},
	}
}

// getAllToolDeclarations returns all available tool declarations
func getAllToolDeclarations() []*genai.FunctionDeclaration {
	return []*genai.FunctionDeclaration{
		createShapeToolDeclaration(),
		// Future tools can be added here
	}
}

// parseShapeFromFunctionCall extracts a ShapeRequest from a function call
func parseShapeFromFunctionCall(call *genai.FunctionCall) *ShapeRequest {
	if call.Name != "create_shape" {
		return nil
	}

	var shape ShapeRequest
	args := call.Args

	if typeVal, ok := args["type"].(string); ok {
		shape.Type = typeVal
	}
	if posVal, ok := args["position"].([]interface{}); ok && len(posVal) == 3 {
		for i, v := range posVal {
			if f, ok := v.(float64); ok {
				shape.Position[i] = f
			}
		}
	}
	if sizeVal, ok := args["size"].(float64); ok {
		shape.Size = sizeVal
	}
	if colorVal, ok := args["color"].([]interface{}); ok && len(colorVal) == 3 {
		for i, v := range colorVal {
			if f, ok := v.(float64); ok {
				shape.Color[i] = f
			}
		}
	}

	return &shape
}
