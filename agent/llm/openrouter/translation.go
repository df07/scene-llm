package openrouter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/df07/scene-llm/agent/llm"
	"github.com/revrost/go-openrouter"
)

// FromInternalMessages converts internal messages to OpenRouter format
func FromInternalMessages(messages []llm.Message, systemPrompt string) []openrouter.ChatCompletionMessage {
	orMessages := make([]openrouter.ChatCompletionMessage, 0, len(messages)+1)

	// Add system prompt as first message if provided
	if systemPrompt != "" {
		orMessages = append(orMessages, openrouter.SystemMessage(systemPrompt))
	}

	// Convert each message
	for _, msg := range messages {
		orMsg := fromInternalMessage(msg)
		if orMsg.Role != "" {
			orMessages = append(orMessages, orMsg)
		}
	}

	return orMessages
}

// fromInternalMessage converts a single internal message to OpenRouter format
func fromInternalMessage(msg llm.Message) openrouter.ChatCompletionMessage {
	orMsg := openrouter.ChatCompletionMessage{
		Role: string(msg.Role),
	}

	// Process parts to build content
	var textParts []string
	var toolCalls []openrouter.ToolCall
	var hasImage bool
	var imageURL string

	for _, part := range msg.Parts {
		switch part.Type {
		case llm.PartTypeText:
			textParts = append(textParts, part.Text)

		case llm.PartTypeFunctionCall:
			if part.FunctionCall != nil {
				// Marshal arguments to JSON
				argsJSON, err := json.Marshal(part.FunctionCall.Arguments)
				if err != nil {
					continue
				}

				toolCalls = append(toolCalls, openrouter.ToolCall{
					ID:   part.FunctionCall.ID,
					Type: "function",
					Function: openrouter.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				})
			}

		case llm.PartTypeFunctionResponse:
			// Function responses go in separate "tool" role message
			if part.FunctionResp != nil {
				respJSON, err := json.Marshal(part.FunctionResp.Response)
				if err != nil {
					continue
				}
				return openrouter.ChatCompletionMessage{
					Role: "tool",
					Content: openrouter.Content{
						Text: string(respJSON),
					},
					ToolCallID: part.FunctionResp.ID,
				}
			}

		case llm.PartTypeImage:
			if part.ImageData != nil && !hasImage {
				// Convert to base64 data URL
				b64 := base64.StdEncoding.EncodeToString(part.ImageData.Data)
				imageURL = fmt.Sprintf("data:%s;base64,%s", part.ImageData.MIMEType, b64)
				hasImage = true
			}
		}
	}

	// Build content based on what we have
	if hasImage && len(textParts) > 0 {
		// Multimodal message with text and image
		orMsg.Content = openrouter.Content{
			Multi: []openrouter.ChatMessagePart{
				{
					Type: "text",
					Text: joinTextParts(textParts),
				},
				{
					Type: "image_url",
					ImageURL: &openrouter.ChatMessageImageURL{
						URL: imageURL,
					},
				},
			},
		}
	} else if hasImage {
		// Image only
		orMsg.Content = openrouter.Content{
			Multi: []openrouter.ChatMessagePart{
				{
					Type: "image_url",
					ImageURL: &openrouter.ChatMessageImageURL{
						URL: imageURL,
					},
				},
			},
		}
	} else {
		// Text only
		orMsg.Content = openrouter.Content{
			Text: joinTextParts(textParts),
		}
	}

	// Add tool calls if any
	if len(toolCalls) > 0 {
		orMsg.ToolCalls = toolCalls
	}

	return orMsg
}

// joinTextParts joins text parts with newlines
func joinTextParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "\n"
		}
		result += part
	}
	return result
}

// FromInternalTools converts internal tool definitions to OpenRouter format
func FromInternalTools(tools []llm.Tool) []openrouter.Tool {
	orTools := make([]openrouter.Tool, 0, len(tools))

	for _, tool := range tools {
		orTool := openrouter.Tool{
			Type: "function",
			Function: &openrouter.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  schemaToMap(tool.Parameters),
			},
		}
		orTools = append(orTools, orTool)
	}

	return orTools
}

// schemaToMap converts a Schema to a map for JSON serialization
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

// ToInternalResponse converts OpenRouter response to internal format
func ToInternalResponse(resp openrouter.ChatCompletionResponse) (*llm.Response, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	parts := make([]llm.Part, 0)

	// Extract text content
	if choice.Message.Content.Text != "" {
		parts = append(parts, llm.Part{
			Type: llm.PartTypeText,
			Text: choice.Message.Content.Text,
		})
	}

	// Extract tool calls
	if len(choice.Message.ToolCalls) > 0 {
		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Type != "function" {
				continue
			}

			// Parse arguments JSON
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				// If parsing fails, skip this tool call
				continue
			}

			parts = append(parts, llm.Part{
				Type: llm.PartTypeFunctionCall,
				FunctionCall: &llm.FunctionCall{
					ID:        toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: args,
				},
			})
		}
	}

	// Map finish reason
	stopReason := "stop"
	switch choice.FinishReason {
	case "stop":
		stopReason = "stop"
	case "length":
		stopReason = "max_tokens"
	case "tool_calls":
		stopReason = "tool_use"
	}

	return &llm.Response{
		Parts:      parts,
		StopReason: stopReason,
	}, nil
}
