package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/df07/scene-llm/agent/llm"
	"github.com/revrost/go-openrouter"
)

// Provider implements the llm.LLMProvider interface for OpenRouter
type Provider struct {
	client *openrouter.Client
}

// NewProvider creates a new OpenRouter provider with the given API key
func NewProvider(apiKey string) (*Provider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENROUTER_API_KEY not provided")
		}
	}

	client := openrouter.NewClient(apiKey)
	return &Provider{client: client}, nil
}

// GenerateContent generates a response from the LLM with optional tool support
func (p *Provider) GenerateContent(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	// Convert internal messages to OpenRouter format
	orMessages := FromInternalMessages(req.Messages, req.SystemPrompt)

	// Build request
	orRequest := openrouter.ChatCompletionRequest{
		Model:    req.Model,
		Messages: orMessages,
	}

	// Add tools if provided
	if len(req.Tools) > 0 {
		orRequest.Tools = FromInternalTools(req.Tools)

		// Debug: Log the tools being sent
		if len(orRequest.Tools) > 0 {
			toolsJSON, err := json.MarshalIndent(orRequest.Tools, "", "  ")
			if err == nil {
				log.Printf("[OpenRouter] Sending %d tools to model %s:\n%s", len(orRequest.Tools), req.Model, string(toolsJSON))
			}
		}
	}

	// Call OpenRouter API
	resp, err := p.client.CreateChatCompletion(ctx, orRequest)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API error: %w", err)
	}

	// Convert response back to internal format
	return ToInternalResponse(resp)
}

// ListModels returns a curated list of popular models available from OpenRouter
// Note: Excludes Anthropic and Google models since they're available via direct providers
func (p *Provider) ListModels() []llm.ModelInfo {
	return []llm.ModelInfo{
		// OpenAI
		{
			ID:            "openai/gpt-5.1",
			DisplayName:   "GPT-5.1",
			Provider:      "openrouter",
			Vision:        true,
			Thinking:      false,
			ContextWindow: 400000,
		},
		{
			ID:            "openai/gpt-oss-120b",
			DisplayName:   "GPT-OSS-120B",
			Provider:      "openrouter",
			Vision:        true,
			Thinking:      false,
			ContextWindow: 128000,
		},
		{
			ID:            "openai/o1",
			DisplayName:   "o1",
			Provider:      "openrouter",
			Vision:        false,
			Thinking:      true,
			ContextWindow: 200000,
		},
		// xAI
		{
			ID:            "x-ai/grok-beta",
			DisplayName:   "Grok Beta",
			Provider:      "openrouter",
			Vision:        true,
			Thinking:      false,
			ContextWindow: 131072,
		},
		// DeepSeek
		{
			ID:            "deepseek/deepseek-chat",
			DisplayName:   "DeepSeek V3",
			Provider:      "openrouter",
			Vision:        false,
			Thinking:      false,
			ContextWindow: 64000,
		},
		{
			ID:            "deepseek/deepseek-r1",
			DisplayName:   "DeepSeek R1",
			Provider:      "openrouter",
			Vision:        false,
			Thinking:      true,
			ContextWindow: 64000,
		},
		// Qwen (Alibaba)
		{
			ID:            "qwen/qwen-2.5-72b-instruct",
			DisplayName:   "Qwen 2.5 72B",
			Provider:      "openrouter",
			Vision:        false,
			Thinking:      false,
			ContextWindow: 131072,
		},
		// Meta
		{
			ID:            "meta-llama/llama-3.3-70b-instruct",
			DisplayName:   "Llama 3.3 70B",
			Provider:      "openrouter",
			Vision:        false,
			Thinking:      false,
			ContextWindow: 128000,
		},
		// Mistral
		{
			ID:            "mistralai/mistral-large",
			DisplayName:   "Mistral Large",
			Provider:      "openrouter",
			Vision:        false,
			Thinking:      false,
			ContextWindow: 128000,
		},
	}
}

// Name returns the provider's name
func (p *Provider) Name() string {
	return "openrouter"
}

// SupportsVision returns true since OpenRouter has models that support image inputs
func (p *Provider) SupportsVision() bool {
	return true
}

// SupportsThinking returns true since OpenRouter has reasoning models
func (p *Provider) SupportsThinking() bool {
	return true
}
