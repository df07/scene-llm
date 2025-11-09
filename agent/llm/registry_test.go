package llm

import (
	"context"
	"testing"
)

// MockProvider for testing
type MockProvider struct {
	name   string
	models []ModelInfo
}

func (m *MockProvider) GenerateContent(ctx context.Context, model string, messages []Message, tools []Tool) (*Response, error) {
	return &Response{
		Parts:      []Part{{Type: PartTypeText, Text: "mock response"}},
		StopReason: "stop",
	}, nil
}

func (m *MockProvider) ListModels() []ModelInfo {
	return m.models
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) SupportsVision() bool {
	return false
}

func (m *MockProvider) SupportsThinking() bool {
	return false
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}
}

func TestAdd(t *testing.T) {
	registry := NewRegistry()

	provider := &MockProvider{
		name: "test",
		models: []ModelInfo{
			{ID: "model1", Provider: "test"},
			{ID: "model2", Provider: "test"},
		},
	}

	registry.Add(provider)

	// Verify models are indexed
	models := registry.ListModels()
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}

func TestGetProviderForModel(t *testing.T) {
	registry := NewRegistry()

	provider := &MockProvider{
		name: "test",
		models: []ModelInfo{
			{ID: "model1", Provider: "test"},
		},
	}

	registry.Add(provider)

	// Existing model
	p, err := registry.GetProviderForModel("model1")
	if err != nil {
		t.Fatalf("GetProviderForModel failed: %v", err)
	}
	if p.Name() != "test" {
		t.Errorf("Expected provider 'test', got '%s'", p.Name())
	}

	// Non-existing model
	_, err = registry.GetProviderForModel("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent model")
	}
}

func TestMultipleProviders(t *testing.T) {
	registry := NewRegistry()

	provider1 := &MockProvider{
		name: "provider1",
		models: []ModelInfo{
			{ID: "model1", Provider: "provider1"},
			{ID: "model2", Provider: "provider1"},
		},
	}
	provider2 := &MockProvider{
		name: "provider2",
		models: []ModelInfo{
			{ID: "model3", Provider: "provider2"},
		},
	}

	registry.Add(provider1)
	registry.Add(provider2)

	models := registry.ListModels()
	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	// Verify correct provider for each model
	p1, _ := registry.GetProviderForModel("model1")
	if p1.Name() != "provider1" {
		t.Errorf("Expected provider1 for model1, got %s", p1.Name())
	}

	p2, _ := registry.GetProviderForModel("model3")
	if p2.Name() != "provider2" {
		t.Errorf("Expected provider2 for model3, got %s", p2.Name())
	}
}
