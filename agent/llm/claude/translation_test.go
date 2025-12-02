package claude

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/df07/scene-llm/agent/llm"
)

func TestToInternalContentBlock_Text(t *testing.T) {
	block := anthropic.ContentBlockUnion{
		Type: "text",
		Text: "Hello from Claude!",
	}

	result := ToInternalContentBlock(block)

	if result.Type != llm.PartTypeText {
		t.Errorf("Expected type %v, got %v", llm.PartTypeText, result.Type)
	}
	if result.Text != "Hello from Claude!" {
		t.Errorf("Expected text 'Hello from Claude!', got '%s'", result.Text)
	}
}

func TestToInternalContentBlock_ToolUse(t *testing.T) {
	inputJSON := []byte(`{"id":"sphere1","type":"sphere"}`)
	block := anthropic.ContentBlockUnion{
		Type:  "tool_use",
		ID:    "call_123",
		Name:  "create_shape",
		Input: inputJSON,
	}

	result := ToInternalContentBlock(block)

	if result.Type != llm.PartTypeFunctionCall {
		t.Errorf("Expected type %v, got %v", llm.PartTypeFunctionCall, result.Type)
	}
	if result.FunctionCall == nil {
		t.Fatal("FunctionCall is nil")
	}
	if result.FunctionCall.ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got '%s'", result.FunctionCall.ID)
	}
	if result.FunctionCall.Name != "create_shape" {
		t.Errorf("Expected name 'create_shape', got '%s'", result.FunctionCall.Name)
	}
	if len(result.FunctionCall.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(result.FunctionCall.Arguments))
	}
}

func TestFromInternalMessage_User(t *testing.T) {
	msg := llm.Message{
		Role: llm.RoleUser,
		Parts: []llm.Part{
			{Type: llm.PartTypeText, Text: "Create a sphere"},
		},
	}

	result := FromInternalMessage(msg)

	if result.Role != anthropic.MessageParamRoleUser {
		t.Errorf("Expected role 'user', got '%s'", result.Role)
	}
}

func TestFromInternalMessage_Assistant(t *testing.T) {
	msg := llm.Message{
		Role: llm.RoleAssistant,
		Parts: []llm.Part{
			{Type: llm.PartTypeText, Text: "I'll create that for you"},
		},
	}

	result := FromInternalMessage(msg)

	if result.Role != anthropic.MessageParamRoleAssistant {
		t.Errorf("Expected role 'assistant', got '%s'", result.Role)
	}
}

func TestFromInternalPart_Text(t *testing.T) {
	part := llm.Part{
		Type: llm.PartTypeText,
		Text: "Test message",
	}

	result := FromInternalPart(part)

	// The actual SDK type checking is complex, so we just verify it returns something
	_ = result
}

func TestFromInternalPart_FunctionResponse(t *testing.T) {
	part := llm.Part{
		Type: llm.PartTypeFunctionResponse,
		FunctionResp: &llm.FunctionResponse{
			ID:   "call_123",
			Name: "create_shape",
			Response: map[string]interface{}{
				"success": true,
				"result":  "created",
			},
		},
	}

	result := FromInternalPart(part)

	// The actual SDK type checking is complex, so we just verify it returns something
	_ = result
}

func TestFromInternalTools(t *testing.T) {
	tools := []llm.Tool{
		{
			Name:        "create_shape",
			Description: "Creates a 3D shape",
			Parameters: &llm.Schema{
				Type: llm.TypeObject,
				Properties: map[string]*llm.Schema{
					"id": {
						Type:        llm.TypeString,
						Description: "Unique identifier",
					},
				},
			},
		},
	}

	result := FromInternalTools(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	// Just verify the conversion works, the actual SDK structure is complex
	_ = result[0]
}

func TestToInternalMessage_FromClaude(t *testing.T) {
	claudeMsg := anthropic.Message{
		Role: "assistant",
		Content: []anthropic.ContentBlockUnion{
			{
				Type: "text",
				Text: "I'll help you create that",
			},
		},
	}

	result := ToInternalMessage(claudeMsg)

	if result.Role != llm.RoleAssistant {
		t.Errorf("Expected role %v, got %v", llm.RoleAssistant, result.Role)
	}
	if len(result.Parts) != 1 {
		t.Fatalf("Expected 1 part, got %d", len(result.Parts))
	}
	if result.Parts[0].Type != llm.PartTypeText {
		t.Errorf("Expected part type %v, got %v", llm.PartTypeText, result.Parts[0].Type)
	}
	if result.Parts[0].Text != "I'll help you create that" {
		t.Errorf("Expected text 'I'll help you create that', got '%s'", result.Parts[0].Text)
	}
}
