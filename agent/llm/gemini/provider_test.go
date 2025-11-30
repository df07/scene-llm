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

// TestProvider_ListModels is skipped because it requires a real API call to Gemini.
// ListModels queries the Gemini API dynamically and cannot be tested without valid credentials.
// Integration tests with real API credentials should be run separately.
func TestProvider_ListModels(t *testing.T) {
	t.Skip("ListModels requires real Gemini API credentials - run as integration test")
}

func TestProvider_Close(t *testing.T) {
	provider := &Provider{}

	err := provider.Close()
	if err != nil {
		t.Errorf("Expected Close to return nil, got error: %v", err)
	}
}
