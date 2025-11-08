package gemini

import (
	"context"
	"fmt"

	"github.com/df07/scene-llm/agent/llm"
	"google.golang.org/genai"
)

// Provider implements the llm.LLMProvider interface for Google Gemini
type Provider struct {
	client *genai.Client
}

// NewProvider creates a new Gemini provider with the given API key
func NewProvider(ctx context.Context, apiKey string) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &Provider{client: client}, nil
}

// GenerateContent generates a response from the LLM with optional tool support
func (p *Provider) GenerateContent(ctx context.Context, model string, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	// Convert internal format to genai format
	genaiMessages := FromInternalMessages(messages)

	// Prepare config
	config := &genai.GenerateContentConfig{}

	// Add tools if provided
	if len(tools) > 0 {
		genaiTools := FromInternalTools(tools)
		config.Tools = []*genai.Tool{{FunctionDeclarations: genaiTools}}
	}

	// Call Gemini API
	resp, err := p.client.Models.GenerateContent(ctx, model, genaiMessages, config)
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	// Convert response back to internal format
	return ToInternalResponse(resp)
}

// ListModels returns the models available from Gemini
func (p *Provider) ListModels() []llm.ModelInfo {
	return []llm.ModelInfo{
		{
			ID:            "gemini-2.5-flash",
			DisplayName:   "Gemini 2.5 Flash",
			Provider:      "google",
			Vision:        true,
			Thinking:      true,
			ContextWindow: 1000000, // 1M tokens
		},
		{
			ID:            "gemini-2.5-pro",
			DisplayName:   "Gemini 2.5 Pro",
			Provider:      "google",
			Vision:        true,
			Thinking:      true,
			ContextWindow: 2000000, // 2M tokens
		},
		{
			ID:            "gemini-1.5-flash",
			DisplayName:   "Gemini 1.5 Flash",
			Provider:      "google",
			Vision:        true,
			Thinking:      false,
			ContextWindow: 1000000,
		},
		{
			ID:            "gemini-1.5-pro",
			DisplayName:   "Gemini 1.5 Pro",
			Provider:      "google",
			Vision:        true,
			Thinking:      false,
			ContextWindow: 2000000,
		},
	}
}

// Name returns the provider's name
func (p *Provider) Name() string {
	return "google"
}

// SupportsVision returns true since Gemini supports image inputs
func (p *Provider) SupportsVision() bool {
	return true
}

// SupportsThinking returns true since Gemini 2.5+ supports extended reasoning
func (p *Provider) SupportsThinking() bool {
	return true
}

// Close cleans up the provider resources
func (p *Provider) Close() error {
	// genai.Client doesn't have a Close method
	return nil
}
