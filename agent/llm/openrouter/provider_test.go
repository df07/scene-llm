package openrouter

import (
	"testing"
)

func TestProvider_Name(t *testing.T) {
	provider := &Provider{}
	if provider.Name() != "openrouter" {
		t.Errorf("Expected provider name 'openrouter', got '%s'", provider.Name())
	}
}

func TestProvider_SupportsVision(t *testing.T) {
	provider := &Provider{}
	if !provider.SupportsVision() {
		t.Error("OpenRouter provider should support vision")
	}
}

func TestProvider_SupportsThinking(t *testing.T) {
	provider := &Provider{}
	if !provider.SupportsThinking() {
		t.Error("OpenRouter provider should support thinking")
	}
}

func TestProvider_ListModels(t *testing.T) {
	provider := &Provider{}
	models := provider.ListModels()

	if len(models) == 0 {
		t.Fatal("Expected at least one model in the curated list")
	}

	// Verify all models have required fields
	for _, model := range models {
		if model.ID == "" {
			t.Errorf("Model has empty ID: %+v", model)
		}
		if model.DisplayName == "" {
			t.Errorf("Model %s has empty DisplayName", model.ID)
		}
		if model.Provider != "openrouter" {
			t.Errorf("Model %s has wrong provider: %s", model.ID, model.Provider)
		}
		if model.ContextWindow <= 0 {
			t.Errorf("Model %s has invalid context window: %d", model.ID, model.ContextWindow)
		}
	}

	// Verify we have some popular models (excluding Anthropic/Google - available via direct providers)
	expectedModels := map[string]bool{
		"openai/gpt-4o":                     false,
		"openai/o1":                         false,
		"x-ai/grok-beta":                    false,
		"deepseek/deepseek-chat":            false,
		"deepseek/deepseek-r1":              false,
		"qwen/qwen-2.5-72b-instruct":        false,
		"meta-llama/llama-3.3-70b-instruct": false,
		"mistralai/mistral-large":           false,
	}

	for _, model := range models {
		if _, exists := expectedModels[model.ID]; exists {
			expectedModels[model.ID] = true
		}
	}

	for modelID, found := range expectedModels {
		if !found {
			t.Errorf("Expected model %s not found in curated list", modelID)
		}
	}
}
