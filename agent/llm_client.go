package agent

import (
	"context"

	"google.golang.org/genai"
)

// LLMClient is an interface for LLM operations
type LLMClient interface {
	GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

// GeminiClient wraps the actual Gemini client
type GeminiClient struct {
	client *genai.Client
}

func (g *GeminiClient) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	return g.client.Models.GenerateContent(ctx, model, contents, config)
}

// MockLLMClient allows injecting pre-defined responses for testing
type MockLLMClient struct {
	// Responses is a queue of responses to return
	Responses []*genai.GenerateContentResponse
	// CallCount tracks how many times GenerateContent was called
	CallCount int
}

func (m *MockLLMClient) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	if m.CallCount >= len(m.Responses) {
		// Return empty response when we run out
		return &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role:  "model",
						Parts: []*genai.Part{{Text: "Done"}},
					},
				},
			},
		}, nil
	}

	response := m.Responses[m.CallCount]
	m.CallCount++
	return response, nil
}

// NewMockResponse creates a mock response with text and optional function calls
func NewMockResponse(text string, functionCalls ...*genai.FunctionCall) *genai.GenerateContentResponse {
	parts := []*genai.Part{}

	if text != "" {
		parts = append(parts, &genai.Part{Text: text})
	}

	for _, fc := range functionCalls {
		parts = append(parts, &genai.Part{FunctionCall: fc})
	}

	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role:  "model",
					Parts: parts,
				},
			},
		},
	}
}
