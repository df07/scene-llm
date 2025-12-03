package gemini

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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
func (p *Provider) GenerateContent(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	// Prepend system prompt as a user message if provided
	// Gemini doesn't have a separate system parameter, so we add it as the first message
	messages := req.Messages
	if req.SystemPrompt != "" {
		systemMsg := llm.Message{
			Role: llm.RoleUser,
			Parts: []llm.Part{
				{Type: llm.PartTypeText, Text: req.SystemPrompt},
			},
		}
		messages = append([]llm.Message{systemMsg}, messages...)
	}

	// Convert internal format to genai format
	genaiMessages := FromInternalMessages(messages)

	// Prepare config
	config := &genai.GenerateContentConfig{}

	// Add tools if provided
	if len(req.Tools) > 0 {
		genaiTools := FromInternalTools(req.Tools)
		config.Tools = []*genai.Tool{{FunctionDeclarations: genaiTools}}
	}

	// Call Gemini API
	resp, err := p.client.Models.GenerateContent(ctx, req.Model, genaiMessages, config)
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	// Convert response back to internal format
	return ToInternalResponse(resp)
}

// ListModels returns the models available from Gemini by querying the API
func (p *Provider) ListModels() []llm.ModelInfo {
	ctx := context.Background()

	// isAllowedModel checks if a model ID matches our patterns for conversational models
	isAllowedModel := func(modelID string) bool {
		// Must start with "gemini-"
		if !strings.HasPrefix(modelID, "gemini-") {
			return false
		}

		// Must contain either "flash" or "pro"
		if !strings.Contains(modelID, "flash") && !strings.Contains(modelID, "pro") {
			return false
		}

		// Exclude specialized models we don't want
		excludePatterns := []string{
			"image",    // Image generation models
			"tts",      // Text-to-speech models
			"computer", // Computer use models
			"robotics", // Robotics models
			"lite",     // Lite versions (less capable)
			"-latest",  // Aliases (we want specific versions)
		}

		for _, exclude := range excludePatterns {
			if strings.Contains(modelID, exclude) {
				return false
			}
		}

		// Exclude dated variants - match patterns like:
		// -MMDD, -MM-DD, -YYYYMMDD, -YYYY-MM-DD, -preview-MM-DD, -exp-MMDD, -001, -1219
		// Pattern: dash followed by digits (optionally with dashes) at the end
		datedPattern := regexp.MustCompile(`-\d{2,4}(-\d{2})?(-\d{2})?$`)
		if datedPattern.MatchString(modelID) {
			return false
		}

		return true
	}

	// Query the Gemini API for available models
	modelsPage, err := p.client.Models.List(ctx, nil)
	if err != nil {
		// Fallback to hardcoded list if API call fails
		return []llm.ModelInfo{
			{
				ID:            "gemini-2.0-flash-exp",
				DisplayName:   "Gemini 2.0 Flash (Experimental)",
				Provider:      "google",
				Vision:        true,
				Thinking:      true,
				ContextWindow: 1000000,
			},
		}
	}

	// Build a map of available models from the API response
	availableModels := make(map[string]*llm.ModelInfo)
	for _, model := range modelsPage.Items {
		// Only include models that support generateContent
		supportsGenerate := false
		for _, action := range model.SupportedActions {
			if action == "generateContent" {
				supportsGenerate = true
				break
			}
		}

		if !supportsGenerate {
			continue
		}

		// Extract model ID (remove "models/" prefix if present)
		modelID := model.Name
		if len(modelID) > 7 && modelID[:7] == "models/" {
			modelID = modelID[7:]
		}

		// Only include models matching our pattern
		if !isAllowedModel(modelID) {
			continue
		}

		availableModels[modelID] = &llm.ModelInfo{
			ID:            modelID,
			DisplayName:   model.DisplayName,
			Provider:      "google",
			Vision:        true,    // Most Gemini models support vision
			Thinking:      false,   // Conservative default
			ContextWindow: 1000000, // Default, actual may vary
		}
	}

	// Build result (sorting handled by registry)
	var result []llm.ModelInfo
	for _, modelInfo := range availableModels {
		result = append(result, *modelInfo)
	}

	return result
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
