package agent

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

// Agent handles LLM conversations and tool execution
type Agent struct {
	client *genai.Client
	events chan<- AgentEvent
}

// New creates a new agent that will send events to the provided channel
func New(events chan<- AgentEvent) (*Agent, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable not set")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &Agent{
		client: client,
		events: events,
	}, nil
}

// ProcessMessage handles a conversation turn and emits events
func (a *Agent) ProcessMessage(ctx context.Context, conversation []*genai.Content, sceneContext string) error {
	// Send thinking event
	a.events <- NewThinkingEvent("🤖 Processing your request...")

	// Add scene context to the latest message
	contextualizedConversation := a.addSceneContext(conversation, sceneContext)

	// Prepare tools
	tools := []*genai.Tool{{FunctionDeclarations: getAllToolDeclarations()}}

	// Generate content with function calling
	result, err := a.client.Models.GenerateContent(ctx, "gemini-2.5-flash", contextualizedConversation, &genai.GenerateContentConfig{
		Tools: tools,
	})
	if err != nil {
		log.Printf("Failed to generate content: %v", err)
		a.events <- NewErrorEvent(fmt.Errorf("LLM generation failed: %w", err))
		return err
	}

	// Process the response
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		log.Printf("No response from LLM")
		a.events <- NewErrorEvent(fmt.Errorf("no response from LLM"))
		return fmt.Errorf("no response from LLM")
	}

	var textResponse string
	var functionCalls []ShapeRequest

	// Process all parts of the response
	for _, part := range result.Candidates[0].Content.Parts {
		// Handle function calls
		if part.FunctionCall != nil {
			if shape := parseShapeFromFunctionCall(part.FunctionCall); shape != nil {
				functionCalls = append(functionCalls, *shape)
			}
		}

		// Handle text response
		if part.Text != "" {
			textResponse = part.Text
		}
	}

	// Emit text response if we have one
	if textResponse != "" {
		a.events <- NewResponseEvent(textResponse)
	}

	// Emit function calls if we have any
	if len(functionCalls) > 0 {
		a.events <- NewToolCallEvent(functionCalls)
	}

	// Send completion event
	a.events <- NewCompleteEvent()
	return nil
}

// addSceneContext prepends scene context to the latest user message
func (a *Agent) addSceneContext(conversation []*genai.Content, sceneContext string) []*genai.Content {
	if len(conversation) == 0 {
		return conversation
	}

	// Make a copy to avoid modifying the original
	contextualizedConversation := make([]*genai.Content, len(conversation))
	copy(contextualizedConversation, conversation)

	// Add context to the latest user message
	lastMessage := contextualizedConversation[len(contextualizedConversation)-1]
	if lastMessage.Role == "user" && len(lastMessage.Parts) > 0 {
		originalText := lastMessage.Parts[0].Text
		contextualText := fmt.Sprintf("Context: You are a 3D scene assistant. %s\n\nUser request: %s", sceneContext, originalText)

		// Create a new part with the contextualized text
		lastMessage.Parts[0] = &genai.Part{Text: contextualText}
	}

	return contextualizedConversation
}

// Close cleans up the agent resources
func (a *Agent) Close() error {
	// genai.Client doesn't have a Close method in this version
	return nil
}
