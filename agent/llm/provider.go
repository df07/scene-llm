package llm

import "context"

// Role represents the role of a message in a conversation
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// PartType represents the type of content in a message part
type PartType string

const (
	PartTypeText             PartType = "text"
	PartTypeFunctionCall     PartType = "function_call"
	PartTypeFunctionResponse PartType = "function_response"
	PartTypeImage            PartType = "image"
)

// Message represents a provider-agnostic message in a conversation
type Message struct {
	Role  Role // Type-safe role (user or assistant)
	Parts []Part
}

// Part represents a single piece of content within a message
type Part struct {
	Type         PartType
	Text         string
	FunctionCall *FunctionCall
	FunctionResp *FunctionResponse
	ImageData    *ImageData
	Thought      bool // For thinking tokens (extended reasoning)
}

// FunctionCall represents a request to call a function/tool
type FunctionCall struct {
	ID        string // LLM-generated ID for tracking
	Name      string
	Arguments map[string]interface{}
}

// FunctionResponse represents the result of a function/tool execution
type FunctionResponse struct {
	ID       string // Matches the FunctionCall.ID
	Name     string
	Response map[string]interface{}
}

// ImageData represents image content in a message
type ImageData struct {
	Data     []byte
	MIMEType string
}

// Tool represents a function/tool definition for the LLM
type Tool struct {
	Name        string
	Description string
	Parameters  *Schema
}

// SchemaType represents JSON schema types
type SchemaType string

const (
	TypeObject  SchemaType = "object"
	TypeArray   SchemaType = "array"
	TypeString  SchemaType = "string"
	TypeNumber  SchemaType = "number"
	TypeInteger SchemaType = "integer"
	TypeBoolean SchemaType = "boolean"
)

// Schema represents a JSON schema for tool parameters
type Schema struct {
	Type        SchemaType
	Description string
	Properties  map[string]*Schema
	Items       *Schema  // For array types
	Enum        []string // For enum values
	Required    []string // Required property names (only for object type)
}

// Response represents the LLM's response to a generation request
type Response struct {
	Parts      []Part
	StopReason string // "stop", "max_tokens", "tool_use", etc.
}

// ModelInfo provides metadata about an available model
type ModelInfo struct {
	ID            string // Unique identifier (e.g., "gemini-2.5-flash")
	DisplayName   string // User-friendly name (e.g., "Gemini 2.5 Flash")
	Provider      string // Provider name (e.g., "google", "anthropic", "openai")
	Vision        bool   // Supports image input
	Thinking      bool   // Supports extended reasoning/thinking
	ContextWindow int    // Maximum context window in tokens
}

// GenerateRequest contains all parameters for generating LLM content
type GenerateRequest struct {
	Model        string    // Model ID to use
	SystemPrompt string    // System prompt (separate from conversation)
	Messages     []Message // Conversation history
	Tools        []Tool    // Available tools
}

// LLMProvider defines the interface that all LLM providers must implement
type LLMProvider interface {
	// GenerateContent generates a response from the LLM with optional tool support
	GenerateContent(ctx context.Context, req *GenerateRequest) (*Response, error)

	// ListModels returns the models available from this provider
	ListModels() []ModelInfo

	// Name returns the provider's name (e.g., "google", "anthropic")
	Name() string

	// SupportsVision returns true if this provider supports image inputs
	SupportsVision() bool

	// SupportsThinking returns true if this provider supports extended reasoning
	SupportsThinking() bool
}
