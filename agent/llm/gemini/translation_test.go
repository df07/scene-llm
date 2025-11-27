package gemini

import (
	"testing"

	"github.com/df07/scene-llm/agent/llm"
	"google.golang.org/genai"
)

func TestToInternalPart_Text(t *testing.T) {
	genaiPart := &genai.Part{
		Text:    "Hello, world!",
		Thought: false,
	}

	result := ToInternalPart(genaiPart)

	if result.Type != llm.PartTypeText {
		t.Errorf("Expected type %v, got %v", llm.PartTypeText, result.Type)
	}
	if result.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got '%s'", result.Text)
	}
	if result.Thought != false {
		t.Errorf("Expected Thought=false, got true")
	}
}

func TestToInternalPart_ThinkingText(t *testing.T) {
	genaiPart := &genai.Part{
		Text:    "Let me think...",
		Thought: true,
	}

	result := ToInternalPart(genaiPart)

	if result.Type != llm.PartTypeText {
		t.Errorf("Expected type %v, got %v", llm.PartTypeText, result.Type)
	}
	if result.Thought != true {
		t.Errorf("Expected Thought=true, got false")
	}
}

func TestToInternalPart_ThinkingTextDetectedByPrefix(t *testing.T) {
	// Test that we detect thinking even when SDK doesn't set Thought field
	testCases := []struct {
		text     string
		expected bool
	}{
		{"thought: analyzing the scene", true},
		{"Thought: let me consider this", true},
		{"THOUGHT: planning approach", true},
		{"  thought: with leading whitespace", true},
		{"\tthought: with tab", true},
		{"Not a thought token", false},
		{"This contains the word thought but doesn't start with it", false},
	}

	for _, tc := range testCases {
		genaiPart := &genai.Part{
			Text:    tc.text,
			Thought: false, // SDK didn't set the field
		}

		result := ToInternalPart(genaiPart)

		if result.Thought != tc.expected {
			t.Errorf("For text '%s': expected Thought=%v, got %v", tc.text, tc.expected, result.Thought)
		}
	}
}

func TestToInternalPart_FunctionCall(t *testing.T) {
	genaiPart := &genai.Part{
		FunctionCall: &genai.FunctionCall{
			Name: "create_shape",
			Args: map[string]interface{}{
				"id":   "sphere1",
				"type": "sphere",
			},
		},
	}

	result := ToInternalPart(genaiPart)

	if result.Type != llm.PartTypeFunctionCall {
		t.Errorf("Expected type %v, got %v", llm.PartTypeFunctionCall, result.Type)
	}
	if result.FunctionCall == nil {
		t.Fatal("FunctionCall is nil")
	}
	if result.FunctionCall.Name != "create_shape" {
		t.Errorf("Expected name 'create_shape', got '%s'", result.FunctionCall.Name)
	}
	if len(result.FunctionCall.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(result.FunctionCall.Arguments))
	}
}

func TestToInternalPart_FunctionResponse(t *testing.T) {
	genaiPart := &genai.Part{
		FunctionResponse: &genai.FunctionResponse{
			Name: "create_shape",
			Response: map[string]interface{}{
				"success": true,
				"id":      "sphere1",
			},
		},
	}

	result := ToInternalPart(genaiPart)

	if result.Type != llm.PartTypeFunctionResponse {
		t.Errorf("Expected type %v, got %v", llm.PartTypeFunctionResponse, result.Type)
	}
	if result.FunctionResp == nil {
		t.Fatal("FunctionResp is nil")
	}
	if result.FunctionResp.Name != "create_shape" {
		t.Errorf("Expected name 'create_shape', got '%s'", result.FunctionResp.Name)
	}
}

func TestToInternalPart_Image(t *testing.T) {
	imageData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
	genaiPart := &genai.Part{
		InlineData: &genai.Blob{
			Data:     imageData,
			MIMEType: "image/png",
		},
	}

	result := ToInternalPart(genaiPart)

	if result.Type != llm.PartTypeImage {
		t.Errorf("Expected type %v, got %v", llm.PartTypeImage, result.Type)
	}
	if result.ImageData == nil {
		t.Fatal("ImageData is nil")
	}
	if result.ImageData.MIMEType != "image/png" {
		t.Errorf("Expected MIME type 'image/png', got '%s'", result.ImageData.MIMEType)
	}
	if len(result.ImageData.Data) != len(imageData) {
		t.Errorf("Expected %d bytes, got %d", len(imageData), len(result.ImageData.Data))
	}
}

func TestFromInternalPart_Text(t *testing.T) {
	internalPart := llm.Part{
		Type:    llm.PartTypeText,
		Text:    "Hello",
		Thought: true,
	}

	result := FromInternalPart(internalPart)

	if result.Text != "Hello" {
		t.Errorf("Expected text 'Hello', got '%s'", result.Text)
	}
	if result.Thought != true {
		t.Errorf("Expected Thought=true, got false")
	}
}

func TestFromInternalPart_FunctionCall(t *testing.T) {
	internalPart := llm.Part{
		Type: llm.PartTypeFunctionCall,
		FunctionCall: &llm.FunctionCall{
			Name: "test_function",
			Arguments: map[string]interface{}{
				"arg1": "value1",
			},
		},
	}

	result := FromInternalPart(internalPart)

	if result.FunctionCall == nil {
		t.Fatal("FunctionCall is nil")
	}
	if result.FunctionCall.Name != "test_function" {
		t.Errorf("Expected name 'test_function', got '%s'", result.FunctionCall.Name)
	}
}

func TestToInternalMessage(t *testing.T) {
	genaiContent := &genai.Content{
		Role: "model",
		Parts: []*genai.Part{
			{Text: "Hello"},
			{FunctionCall: &genai.FunctionCall{Name: "test", Args: map[string]interface{}{}}},
		},
	}

	result := ToInternalMessage(genaiContent)

	if result.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", result.Role)
	}
	if len(result.Parts) != 2 {
		t.Errorf("Expected 2 parts, got %d", len(result.Parts))
	}
}

func TestFromInternalMessage(t *testing.T) {
	internalMsg := llm.Message{
		Role: "assistant",
		Parts: []llm.Part{
			{Type: llm.PartTypeText, Text: "Response"},
		},
	}

	result := FromInternalMessage(internalMsg)

	if result.Role != "model" {
		t.Errorf("Expected role 'model', got '%s'", result.Role)
	}
	if len(result.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(result.Parts))
	}
}

func TestToInternalSchema(t *testing.T) {
	genaiSchema := &genai.Schema{
		Type:        genai.TypeObject,
		Description: "Test schema",
		Properties: map[string]*genai.Schema{
			"name": {
				Type:        genai.TypeString,
				Description: "Name field",
			},
			"count": {
				Type: genai.TypeInteger,
			},
		},
		Required: []string{"name"},
	}

	result := ToInternalSchema(genaiSchema)

	if result.Type != llm.TypeObject {
		t.Errorf("Expected type %v, got %v", llm.TypeObject, result.Type)
	}
	if result.Description != "Test schema" {
		t.Errorf("Expected description 'Test schema', got '%s'", result.Description)
	}
	if len(result.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(result.Properties))
	}
	if len(result.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(result.Required))
	}
	if result.Properties["name"].Type != llm.TypeString {
		t.Errorf("Expected name property type %v, got %v", llm.TypeString, result.Properties["name"].Type)
	}
}

func TestFromInternalSchema(t *testing.T) {
	internalSchema := &llm.Schema{
		Type: llm.TypeArray,
		Items: &llm.Schema{
			Type: llm.TypeString,
			Enum: []string{"a", "b", "c"},
		},
	}

	result := FromInternalSchema(internalSchema)

	if result.Type != genai.TypeArray {
		t.Errorf("Expected type %v, got %v", genai.TypeArray, result.Type)
	}
	if result.Items == nil {
		t.Fatal("Items is nil")
	}
	if result.Items.Type != genai.TypeString {
		t.Errorf("Expected items type %v, got %v", genai.TypeString, result.Items.Type)
	}
	if len(result.Items.Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(result.Items.Enum))
	}
}

func TestToInternalTool(t *testing.T) {
	genaiDecl := &genai.FunctionDeclaration{
		Name:        "create_shape",
		Description: "Create a shape",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"id": {Type: genai.TypeString},
			},
		},
	}

	result := ToInternalTool(genaiDecl)

	if result.Name != "create_shape" {
		t.Errorf("Expected name 'create_shape', got '%s'", result.Name)
	}
	if result.Description != "Create a shape" {
		t.Errorf("Expected description 'Create a shape', got '%s'", result.Description)
	}
	if result.Parameters == nil {
		t.Fatal("Parameters is nil")
	}
	if result.Parameters.Type != llm.TypeObject {
		t.Errorf("Expected parameters type %v, got %v", llm.TypeObject, result.Parameters.Type)
	}
}

func TestFromInternalTool(t *testing.T) {
	internalTool := llm.Tool{
		Name:        "test_tool",
		Description: "Test description",
		Parameters: &llm.Schema{
			Type:       llm.TypeObject,
			Properties: map[string]*llm.Schema{},
		},
	}

	result := FromInternalTool(internalTool)

	if result.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", result.Name)
	}
	if result.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", result.Description)
	}
	if result.Parameters == nil {
		t.Fatal("Parameters is nil")
	}
}

func TestRoundTrip_Message(t *testing.T) {
	// Test that converting from internal -> genai -> internal preserves data
	original := llm.Message{
		Role: "user",
		Parts: []llm.Part{
			{Type: llm.PartTypeText, Text: "Hello"},
			{
				Type: llm.PartTypeFunctionCall,
				FunctionCall: &llm.FunctionCall{
					Name:      "test",
					Arguments: map[string]interface{}{"key": "value"},
				},
			},
		},
	}

	genaiMsg := FromInternalMessage(original)
	result := ToInternalMessage(genaiMsg)

	if result.Role != original.Role {
		t.Errorf("Role changed: %s -> %s", original.Role, result.Role)
	}
	if len(result.Parts) != len(original.Parts) {
		t.Errorf("Parts count changed: %d -> %d", len(original.Parts), len(result.Parts))
	}
}

func TestToInternalResponse(t *testing.T) {
	genaiResp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Response text"},
					},
				},
				FinishReason: "STOP",
			},
		},
	}

	result, err := ToInternalResponse(genaiResp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(result.Parts))
	}
	if result.Parts[0].Text != "Response text" {
		t.Errorf("Expected text 'Response text', got '%s'", result.Parts[0].Text)
	}
	if result.StopReason != "STOP" {
		t.Errorf("Expected stop reason 'STOP', got '%s'", result.StopReason)
	}
}

func TestToInternalResponse_NoCandidates(t *testing.T) {
	genaiResp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{},
	}

	_, err := ToInternalResponse(genaiResp)
	if err == nil {
		t.Error("Expected error for empty candidates, got nil")
	}
}
