package llm

import (
	"fmt"
	"sort"
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

// ListModels returns all available model IDs sorted reverse alphabetically
// This works across multiple providers and puts newer versions first (2.5 before 2.0)
func (r *Registry) ListModels() []string {
	// Collect all model IDs
	var models []string
	for modelID := range r.models {
		models = append(models, modelID)
	}

	// Sort reverse alphabetically (Z to A)
	// This naturally puts gemini-2.5 before gemini-2.0
	sort.Slice(models, func(i, j int) bool {
		return models[i] > models[j]
	})

	return models
}

// ListModelsGrouped returns all available models grouped by provider
// Models within each provider are sorted reverse alphabetically
func (r *Registry) ListModelsGrouped() map[string][]ModelInfo {
	grouped := make(map[string][]ModelInfo)

	// Collect models from each provider
	for providerName, provider := range r.providers {
		models := provider.ListModels()

		// Sort models reverse alphabetically (newer versions first)
		sort.Slice(models, func(i, j int) bool {
			return models[i].ID > models[j].ID
		})

		grouped[providerName] = models
	}

	return grouped
}
