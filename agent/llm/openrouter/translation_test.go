package openrouter

import (
	"testing"

	"github.com/df07/scene-llm/agent/llm"
)

func TestSchemaToMap(t *testing.T) {
	schema := &llm.Schema{
		Type:        llm.TypeObject,
		Description: "Test schema",
		Properties: map[string]*llm.Schema{
			"name": {
				Type:        llm.TypeString,
				Description: "Name field",
			},
			"age": {
				Type:        llm.TypeInteger,
				Description: "Age field",
			},
		},
		Required: []string{"name"},
	}

	result := schemaToMap(schema)

	if result["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", result["type"])
	}

	if result["description"] != "Test schema" {
		t.Errorf("Expected description 'Test schema', got %v", result["description"])
	}

	props, ok := result["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	if len(props) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(props))
	}

	required, ok := result["required"].([]string)
	if !ok {
		t.Fatal("Required should be a string slice")
	}

	if len(required) != 1 || required[0] != "name" {
		t.Errorf("Expected required=['name'], got %v", required)
	}
}

func TestFromInternalTools(t *testing.T) {
	tools := []llm.Tool{
		{
			Name:        "test_function",
			Description: "A test function",
			Parameters: &llm.Schema{
				Type: llm.TypeObject,
				Properties: map[string]*llm.Schema{
					"param1": {
						Type:        llm.TypeString,
						Description: "First parameter",
					},
				},
				Required: []string{"param1"},
			},
		},
	}

	orTools := FromInternalTools(tools)

	if len(orTools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(orTools))
	}

	tool := orTools[0]

	if tool.Type != "function" {
		t.Errorf("Expected type 'function', got %s", tool.Type)
	}

	if tool.Function.Name != "test_function" {
		t.Errorf("Expected name 'test_function', got %s", tool.Function.Name)
	}

	if tool.Function.Description != "A test function" {
		t.Errorf("Expected description 'A test function', got %s", tool.Function.Description)
	}
}

func TestFromInternalMessages_WithSystemPrompt(t *testing.T) {
	messages := []llm.Message{
		{
			Role: llm.RoleUser,
			Parts: []llm.Part{
				{Type: llm.PartTypeText, Text: "Hello"},
			},
		},
	}

	systemPrompt := "You are a helpful assistant"
	orMessages := FromInternalMessages(messages, systemPrompt)

	if len(orMessages) != 2 {
		t.Fatalf("Expected 2 messages (system + user), got %d", len(orMessages))
	}

	if orMessages[0].Role != "system" {
		t.Errorf("First message should be system, got %s", orMessages[0].Role)
	}

	if orMessages[0].Content.Text != systemPrompt {
		t.Errorf("System message content incorrect, got %s", orMessages[0].Content.Text)
	}

	if orMessages[1].Role != "user" {
		t.Errorf("Second message should be user, got %s", orMessages[1].Role)
	}

	if orMessages[1].Content.Text != "Hello" {
		t.Errorf("User message content incorrect, got %s", orMessages[1].Content.Text)
	}
}

func TestFromInternalMessages_FunctionCall(t *testing.T) {
	messages := []llm.Message{
		{
			Role: llm.RoleAssistant,
			Parts: []llm.Part{
				{Type: llm.PartTypeText, Text: "I'll help you with that"},
				{
					Type: llm.PartTypeFunctionCall,
					FunctionCall: &llm.FunctionCall{
						ID:   "call_123",
						Name: "test_tool",
						Arguments: map[string]interface{}{
							"param": "value",
						},
					},
				},
			},
		},
	}

	orMessages := FromInternalMessages(messages, "")

	if len(orMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(orMessages))
	}

	msg := orMessages[0]

	if msg.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %s", msg.Role)
	}

	if msg.Content.Text != "I'll help you with that" {
		t.Errorf("Expected content 'I'll help you with that', got %s", msg.Content.Text)
	}

	if len(msg.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(msg.ToolCalls))
	}

	toolCall := msg.ToolCalls[0]

	if toolCall.ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got %s", toolCall.ID)
	}

	if toolCall.Function.Name != "test_tool" {
		t.Errorf("Expected function name 'test_tool', got %s", toolCall.Function.Name)
	}
}

func TestJoinTextParts(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "empty",
			parts:    []string{},
			expected: "",
		},
		{
			name:     "single part",
			parts:    []string{"hello"},
			expected: "hello",
		},
		{
			name:     "multiple parts",
			parts:    []string{"hello", "world"},
			expected: "hello\nworld",
		},
		{
			name:     "three parts",
			parts:    []string{"one", "two", "three"},
			expected: "one\ntwo\nthree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinTextParts(tt.parts)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
