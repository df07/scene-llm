package claude

import (
	"testing"

	"github.com/df07/scene-llm/agent/llm"
)

func TestProvider_Name(t *testing.T) {
	// We can test provider methods that don't require API calls
	provider := &Provider{}

	if provider.Name() != "claude" {
		t.Errorf("Expected name 'claude', got '%s'", provider.Name())
	}
}

func TestProvider_SupportsVision(t *testing.T) {
	provider := &Provider{}

	if !provider.SupportsVision() {
		t.Error("Expected Claude to support vision")
	}
}

func TestProvider_SupportsThinking(t *testing.T) {
	provider := &Provider{}

	if provider.SupportsThinking() {
		t.Error("Expected Claude to not support thinking tokens")
	}
}

func TestProvider_ListModels(t *testing.T) {
	provider := &Provider{}

	models := provider.ListModels()

	if len(models) == 0 {
		t.Fatal("Expected at least one model")
	}

	// Verify all models have required fields
	for _, model := range models {
		if model.ID == "" {
			t.Error("Model missing ID")
		}
		if model.DisplayName == "" {
			t.Error("Model missing DisplayName")
		}
		if model.Provider != "claude" {
			t.Errorf("Expected provider 'claude', got '%s'", model.Provider)
		}
		if model.ContextWindow == 0 {
			t.Error("Model missing ContextWindow")
		}
	}

	// Verify expected models are present
	expectedModels := map[string]bool{
		"claude-sonnet-4-5-20250929": false,
		"claude-haiku-4-5-20251001":  false,
		"claude-opus-4-5-20251101":   false,
	}

	for _, model := range models {
		if _, exists := expectedModels[model.ID]; exists {
			expectedModels[model.ID] = true
		}
	}

	for modelID, found := range expectedModels {
		if !found {
			t.Errorf("Expected model '%s' not found in list", modelID)
		}
	}
}

func TestNewProvider_MissingAPIKey(t *testing.T) {
	// Save original env var
	originalKey := ""
	// Note: We can't actually unset the env var in tests, so we just test the error path
	// This test documents the expected behavior

	t.Skip("NewProvider requires real ANTHROPIC_API_KEY - run as integration test")

	provider, err := NewProvider()
	if err == nil {
		t.Error("Expected error when ANTHROPIC_API_KEY is not set")
	}
	if provider != nil {
		t.Error("Expected nil provider when API key is not set")
	}

	_ = originalKey // avoid unused variable warning
}

func TestProvider_GenerateContent_Interface(t *testing.T) {
	// Verify Provider implements LLMProvider interface
	var _ llm.LLMProvider = (*Provider)(nil)
}
