package claude

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/df07/scene-llm/agent/llm"
)

// ToInternalResponse converts Claude SDK response to internal format
func ToInternalResponse(resp *anthropic.Message) (*llm.Response, error) {
	parts := make([]llm.Part, 0)

	for _, block := range resp.Content {
		part := ToInternalContentBlock(block)
		parts = append(parts, part)
	}

	return &llm.Response{
		Parts:      parts,
		StopReason: string(resp.StopReason),
	}, nil
}

// ToInternalContentBlock converts a Claude content block to internal Part
func ToInternalContentBlock(block anthropic.ContentBlockUnion) llm.Part {
	switch block.Type {
	case "text":
		return llm.Part{
			Type: llm.PartTypeText,
			Text: block.Text,
		}
	case "tool_use":
		// Parse the Input field (it's json.RawMessage)
		var input map[string]interface{}
		if err := json.Unmarshal(block.Input, &input); err != nil {
			input = make(map[string]interface{})
		}
		return llm.Part{
			Type: llm.PartTypeFunctionCall,
			FunctionCall: &llm.FunctionCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: input,
			},
		}
	default:
		// Unknown block type - return empty text part
		return llm.Part{Type: llm.PartTypeText, Text: ""}
	}
}

// FromInternalMessages converts internal messages to Claude format
func FromInternalMessages(messages []llm.Message) []anthropic.MessageParam {
	claudeMessages := make([]anthropic.MessageParam, 0, len(messages))

	for _, msg := range messages {
		claudeMsg := FromInternalMessage(msg)
		claudeMessages = append(claudeMessages, claudeMsg)
	}

	return claudeMessages
}

// FromInternalMessage converts a single internal message to Claude format
func FromInternalMessage(msg llm.Message) anthropic.MessageParam {
	role := anthropic.MessageParamRole(msg.Role)

	// Convert parts to Claude content blocks
	content := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		block := FromInternalPart(part)
		content = append(content, block)
	}

	return anthropic.MessageParam{
		Role:    role,
		Content: content,
	}
}

// FromInternalPart converts internal Part to Claude ContentBlock
func FromInternalPart(part llm.Part) anthropic.ContentBlockParamUnion {
	switch part.Type {
	case llm.PartTypeText:
		return anthropic.NewTextBlock(part.Text)

	case llm.PartTypeFunctionResponse:
		if part.FunctionResp == nil {
			return anthropic.ContentBlockParamUnion{}
		}
		// Convert response map to JSON string
		jsonBytes, err := json.Marshal(part.FunctionResp.Response)
		content := string(jsonBytes)
		if err != nil {
			content = fmt.Sprintf("%v", part.FunctionResp.Response)
		}
		return anthropic.NewToolResultBlock(
			part.FunctionResp.ID,
			content,
			false, // isError
		)

	case llm.PartTypeImage:
		if part.ImageData == nil {
			return anthropic.ContentBlockParamUnion{}
		}
		// Claude expects base64 encoded images as strings
		encodedData := base64.StdEncoding.EncodeToString(part.ImageData.Data)
		return anthropic.NewImageBlockBase64(
			part.ImageData.MIMEType,
			encodedData,
		)

	case llm.PartTypeFunctionCall:
		// Function calls appear in assistant messages as tool_use blocks
		// When echoing conversation history, we need to include them
		if part.FunctionCall == nil {
			return anthropic.ContentBlockParamUnion{}
		}
		// Create a tool use block
		return anthropic.NewToolUseBlock(
			part.FunctionCall.ID,
			part.FunctionCall.Arguments,
			part.FunctionCall.Name,
		)

	default:
		return anthropic.ContentBlockParamUnion{}
	}
}

// schemaToMap converts llm.Schema to map[string]interface{}
func schemaToMap(schema *llm.Schema) map[string]interface{} {
	if schema == nil {
		return make(map[string]interface{})
	}

	result := make(map[string]interface{})
	result["type"] = string(schema.Type)

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if schema.Properties != nil {
		props := make(map[string]interface{})
		for name, propSchema := range schema.Properties {
			props[name] = schemaToMap(propSchema)
		}
		result["properties"] = props
	}

	if schema.Items != nil {
		result["items"] = schemaToMap(schema.Items)
	}

	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	return result
}

// FromInternalTools converts internal tool definitions to Claude format
func FromInternalTools(tools []llm.Tool) []anthropic.ToolUnionParam {
	claudeTools := make([]anthropic.ToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		// Convert the schema to the format Claude expects
		schemaMap := schemaToMap(tool.Parameters)

		// Create ToolInputSchemaParam
		inputSchema := anthropic.ToolInputSchemaParam{
			Properties: schemaMap["properties"],
		}
		if required, ok := schemaMap["required"].([]string); ok {
			inputSchema.Required = required
		}
		// Add extra fields
		inputSchema.ExtraFields = make(map[string]any)
		for k, v := range schemaMap {
			if k != "type" && k != "properties" && k != "required" {
				inputSchema.ExtraFields[k] = v
			}
		}

		claudeTool := anthropic.ToolUnionParamOfTool(inputSchema, tool.Name)
		if tool.Description != "" {
			claudeTool.OfTool.Description = anthropic.String(tool.Description)
		}
		claudeTools = append(claudeTools, claudeTool)
	}

	return claudeTools
}

// ToInternalMessage converts Claude message to internal format (for conversation history)
func ToInternalMessage(msg anthropic.Message) llm.Message {
	parts := make([]llm.Part, 0, len(msg.Content))
	for _, block := range msg.Content {
		parts = append(parts, ToInternalContentBlock(block))
	}

	var role llm.Role
	if msg.Role == "assistant" {
		role = llm.RoleAssistant
	} else {
		role = llm.RoleUser
	}

	return llm.Message{
		Role:  role,
		Parts: parts,
	}
}
