package claude

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/df07/scene-llm/agent/llm"
)

// Provider implements the LLM provider interface for Anthropic Claude
type Provider struct {
	client anthropic.Client
}

// NewProvider creates a new Claude provider
// Requires ANTHROPIC_API_KEY environment variable
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &Provider{
		client: client,
	}, nil
}

// GenerateContent generates a response from Claude
func (p *Provider) GenerateContent(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	// Convert internal messages to Claude format
	claudeMessages := FromInternalMessages(req.Messages)

	// Build request parameters
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		Messages:  claudeMessages,
		MaxTokens: int64(4096), // Claude requires max_tokens
	}

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	// Add tools if provided
	if len(req.Tools) > 0 {
		claudeTools := FromInternalTools(req.Tools)
		params.Tools = claudeTools
	}

	// Call Claude API
	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("Claude API error: %w", err)
	}

	// Convert response to internal format
	return ToInternalResponse(resp)
}

// ListModels returns available Claude models
func (p *Provider) ListModels() []llm.ModelInfo {
	return []llm.ModelInfo{
		{
			ID:            "claude-sonnet-4-5-20250929",
			DisplayName:   "Claude Sonnet 4.5",
			Provider:      "claude",
			ContextWindow: 200000,
		},
		{
			ID:            "claude-haiku-4-5-20251001",
			DisplayName:   "Claude Haiku 4.5",
			Provider:      "claude",
			ContextWindow: 200000,
		},
		{
			ID:            "claude-opus-4-5-20251101",
			DisplayName:   "Claude Opus 4.5",
			Provider:      "claude",
			ContextWindow: 200000,
		},
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "claude"
}

// SupportsVision returns whether this provider supports image inputs
func (p *Provider) SupportsVision() bool {
	return true
}

// SupportsThinking returns whether this provider supports extended thinking
func (p *Provider) SupportsThinking() bool {
	return false // Claude doesn't have Gemini-style thinking tokens
}
