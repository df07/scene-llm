package llm

import (
	"fmt"
)

// Registry manages available LLM providers and models
// Registry is initialized once at startup and then read-only.
// Go maps are safe for concurrent reads.
type Registry struct {
	providers map[string]LLMProvider // provider name -> provider
	models    map[string]string      // model ID -> provider name
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]LLMProvider),
		models:    make(map[string]string),
	}
}

// Add registers a provider and indexes its models
func (r *Registry) Add(provider LLMProvider) {
	r.providers[provider.Name()] = provider

	// Index all models from this provider
	for _, model := range provider.ListModels() {
		r.models[model.ID] = provider.Name()
	}
}

// GetProviderForModel returns the provider that serves a given model
func (r *Registry) GetProviderForModel(modelID string) (LLMProvider, error) {
	providerName, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not found", modelID)
	}

	return r.providers[providerName], nil
}

// ListModels returns all available model IDs with preferred models first
// The first model in the list is the recommended default
func (r *Registry) ListModels() []string {
	// Preferred models in order of preference
	preferredModels := []string{
		"gemini-2.5-flash",
		"gemini-1.5-flash",
		"gemini-2.5-pro",
		"gemini-1.5-pro",
	}

	var models []string

	// Add preferred models first if they exist
	for _, preferred := range preferredModels {
		if _, exists := r.models[preferred]; exists {
			models = append(models, preferred)
		}
	}

	// Add any other models
	for modelID := range r.models {
		found := false
		for _, preferred := range preferredModels {
			if modelID == preferred {
				found = true
				break
			}
		}
		if !found {
			models = append(models, modelID)
		}
	}

	return models
}
