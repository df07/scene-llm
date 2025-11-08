package gemini

import (
	"fmt"

	"github.com/df07/scene-llm/agent/llm"
	"google.golang.org/genai"
)

// ToInternalMessages converts genai.Content messages to internal llm.Message format
func ToInternalMessages(contents []*genai.Content) []llm.Message {
	messages := make([]llm.Message, len(contents))
	for i, content := range contents {
		messages[i] = ToInternalMessage(content)
	}
	return messages
}

// ToInternalMessage converts a single genai.Content to llm.Message
func ToInternalMessage(content *genai.Content) llm.Message {
	parts := make([]llm.Part, len(content.Parts))
	for i, part := range content.Parts {
		parts[i] = ToInternalPart(part)
	}

	// Map genai role to internal role
	role := content.Role
	if role == "model" {
		role = "assistant"
	}

	return llm.Message{
		Role:  role,
		Parts: parts,
	}
}

// ToInternalPart converts a genai.Part to llm.Part
func ToInternalPart(part *genai.Part) llm.Part {
	// Text part
	if part.Text != "" {
		return llm.Part{
			Type:    llm.PartTypeText,
			Text:    part.Text,
			Thought: part.Thought,
		}
	}

	// Function call
	if part.FunctionCall != nil {
		return llm.Part{
			Type: llm.PartTypeFunctionCall,
			FunctionCall: &llm.FunctionCall{
				Name:      part.FunctionCall.Name,
				Arguments: part.FunctionCall.Args,
			},
		}
	}

	// Function response
	if part.FunctionResponse != nil {
		return llm.Part{
			Type: llm.PartTypeFunctionResponse,
			FunctionResp: &llm.FunctionResponse{
				Name:     part.FunctionResponse.Name,
				Response: part.FunctionResponse.Response,
			},
		}
	}

	// Image data (inline)
	if part.InlineData != nil {
		return llm.Part{
			Type: llm.PartTypeImage,
			ImageData: &llm.ImageData{
				Data:     part.InlineData.Data,
				MIMEType: part.InlineData.MIMEType,
			},
		}
	}

	// Unknown part type - return empty text part
	return llm.Part{
		Type: llm.PartTypeText,
		Text: "",
	}
}

// FromInternalMessages converts internal llm.Message format to genai.Content
func FromInternalMessages(messages []llm.Message) []*genai.Content {
	contents := make([]*genai.Content, len(messages))
	for i, msg := range messages {
		contents[i] = FromInternalMessage(msg)
	}
	return contents
}

// FromInternalMessage converts a single llm.Message to genai.Content
func FromInternalMessage(msg llm.Message) *genai.Content {
	parts := make([]*genai.Part, len(msg.Parts))
	for i, part := range msg.Parts {
		parts[i] = FromInternalPart(part)
	}

	// Map internal role to genai role
	role := msg.Role
	if role == "assistant" {
		role = "model"
	}

	return &genai.Content{
		Role:  role,
		Parts: parts,
	}
}

// FromInternalPart converts an llm.Part to genai.Part
func FromInternalPart(part llm.Part) *genai.Part {
	switch part.Type {
	case llm.PartTypeText:
		return &genai.Part{
			Text:    part.Text,
			Thought: part.Thought,
		}

	case llm.PartTypeFunctionCall:
		if part.FunctionCall == nil {
			return &genai.Part{}
		}
		return &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Arguments,
			},
		}

	case llm.PartTypeFunctionResponse:
		if part.FunctionResp == nil {
			return &genai.Part{}
		}
		return &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     part.FunctionResp.Name,
				Response: part.FunctionResp.Response,
			},
		}

	case llm.PartTypeImage:
		if part.ImageData == nil {
			return &genai.Part{}
		}
		return &genai.Part{
			InlineData: &genai.Blob{
				Data:     part.ImageData.Data,
				MIMEType: part.ImageData.MIMEType,
			},
		}

	default:
		return &genai.Part{Text: ""}
	}
}

// ToInternalTools converts genai.FunctionDeclaration to internal llm.Tool format
func ToInternalTools(declarations []*genai.FunctionDeclaration) []llm.Tool {
	tools := make([]llm.Tool, len(declarations))
	for i, decl := range declarations {
		tools[i] = ToInternalTool(decl)
	}
	return tools
}

// ToInternalTool converts a single genai.FunctionDeclaration to llm.Tool
func ToInternalTool(decl *genai.FunctionDeclaration) llm.Tool {
	var schema *llm.Schema
	if decl.Parameters != nil {
		schema = ToInternalSchema(decl.Parameters)
	}

	return llm.Tool{
		Name:        decl.Name,
		Description: decl.Description,
		Parameters:  schema,
	}
}

// genaiTypeToInternal converts genai.Type to llm.SchemaType
func genaiTypeToInternal(t genai.Type) llm.SchemaType {
	switch t {
	case genai.TypeObject:
		return llm.TypeObject
	case genai.TypeArray:
		return llm.TypeArray
	case genai.TypeString:
		return llm.TypeString
	case genai.TypeNumber:
		return llm.TypeNumber
	case genai.TypeInteger:
		return llm.TypeInteger
	case genai.TypeBoolean:
		return llm.TypeBoolean
	default:
		return llm.TypeString // Default fallback
	}
}

// internalTypeToGenai converts llm.SchemaType to genai.Type
func internalTypeToGenai(t llm.SchemaType) genai.Type {
	switch t {
	case llm.TypeObject:
		return genai.TypeObject
	case llm.TypeArray:
		return genai.TypeArray
	case llm.TypeString:
		return genai.TypeString
	case llm.TypeNumber:
		return genai.TypeNumber
	case llm.TypeInteger:
		return genai.TypeInteger
	case llm.TypeBoolean:
		return genai.TypeBoolean
	default:
		return genai.TypeString // Default fallback
	}
}

// ToInternalSchema converts genai.Schema to llm.Schema
func ToInternalSchema(s *genai.Schema) *llm.Schema {
	if s == nil {
		return nil
	}

	schema := &llm.Schema{
		Type:        genaiTypeToInternal(s.Type),
		Description: s.Description,
		Enum:        s.Enum,
		Required:    s.Required,
	}

	// Convert properties
	if len(s.Properties) > 0 {
		schema.Properties = make(map[string]*llm.Schema)
		for name, prop := range s.Properties {
			schema.Properties[name] = ToInternalSchema(prop)
		}
	}

	// Convert items (for arrays)
	if s.Items != nil {
		schema.Items = ToInternalSchema(s.Items)
	}

	return schema
}

// FromInternalTools converts internal llm.Tool to genai.FunctionDeclaration
func FromInternalTools(tools []llm.Tool) []*genai.FunctionDeclaration {
	declarations := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		declarations[i] = FromInternalTool(tool)
	}
	return declarations
}

// FromInternalTool converts a single llm.Tool to genai.FunctionDeclaration
func FromInternalTool(tool llm.Tool) *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  FromInternalSchema(tool.Parameters),
	}
}

// FromInternalSchema converts llm.Schema to genai.Schema
func FromInternalSchema(s *llm.Schema) *genai.Schema {
	if s == nil {
		return nil
	}

	schema := &genai.Schema{
		Type:        internalTypeToGenai(s.Type),
		Description: s.Description,
		Enum:        s.Enum,
		Required:    s.Required,
	}

	// Convert properties
	if len(s.Properties) > 0 {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range s.Properties {
			schema.Properties[name] = FromInternalSchema(prop)
		}
	}

	// Convert items (for arrays)
	if s.Items != nil {
		schema.Items = FromInternalSchema(s.Items)
	}

	return schema
}

// ToInternalResponse converts genai.GenerateContentResponse to llm.Response
func ToInternalResponse(resp *genai.GenerateContentResponse) (*llm.Response, error) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("no response candidates")
	}

	candidate := resp.Candidates[0]
	parts := make([]llm.Part, len(candidate.Content.Parts))
	for i, part := range candidate.Content.Parts {
		parts[i] = ToInternalPart(part)
	}

	// Map finish reason to stop reason
	stopReason := "stop"
	if candidate.FinishReason != "" {
		stopReason = string(candidate.FinishReason)
	}

	return &llm.Response{
		Parts:      parts,
		StopReason: stopReason,
	}, nil
}
