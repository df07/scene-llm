package gemini

import (
	"testing"
)

func TestProvider_Name(t *testing.T) {
	provider := &Provider{}

	if provider.Name() != "google" {
		t.Errorf("Expected name 'google', got '%s'", provider.Name())
	}
}

func TestProvider_SupportsVision(t *testing.T) {
	provider := &Provider{}

	if !provider.SupportsVision() {
		t.Error("Expected SupportsVision to return true")
	}
}

func TestProvider_SupportsThinking(t *testing.T) {
	provider := &Provider{}

	if !provider.SupportsThinking() {
		t.Error("Expected SupportsThinking to return true")
	}
}

func TestProvider_ListModels(t *testing.T) {
	provider := &Provider{}

	models := provider.ListModels()

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Verify Gemini 2.5 Flash is in the list
	foundFlash := false
	for _, model := range models {
		if model.ID == "gemini-2.5-flash" {
			foundFlash = true
			if model.DisplayName != "Gemini 2.5 Flash" {
				t.Errorf("Expected display name 'Gemini 2.5 Flash', got '%s'", model.DisplayName)
			}
			if model.Provider != "google" {
				t.Errorf("Expected provider 'google', got '%s'", model.Provider)
			}
			if !model.Vision {
				t.Error("Expected Gemini 2.5 Flash to support vision")
			}
			if !model.Thinking {
				t.Error("Expected Gemini 2.5 Flash to support thinking")
			}
			if model.ContextWindow != 1000000 {
				t.Errorf("Expected context window 1000000, got %d", model.ContextWindow)
			}
		}
	}

	if !foundFlash {
		t.Error("Expected to find gemini-2.5-flash in model list")
	}
}

func TestProvider_Close(t *testing.T) {
	provider := &Provider{}

	err := provider.Close()
	if err != nil {
		t.Errorf("Expected Close to return nil, got error: %v", err)
	}
}
