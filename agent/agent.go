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
	client       *genai.Client
	events       chan<- AgentEvent
	sceneManager *SceneManager
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
		client:       client,
		events:       events,
		sceneManager: NewSceneManager(),
	}, nil
}

// SetEventsChannel sets the events channel for this agent
func (a *Agent) SetEventsChannel(events chan<- AgentEvent) {
	a.events = events
}

// ProcessMessage handles a conversation turn and emits events
func (a *Agent) ProcessMessage(ctx context.Context, conversation []*genai.Content) error {
	// Send thinking event
	a.events <- NewThinkingEvent("🤖 Processing your request...")

	// Build scene context from our internal scene manager
	sceneContext := a.sceneManager.BuildContext()

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
	var createShapes []ShapeRequest
	var hasShapeOperations bool

	// Process all parts of the response
	for _, part := range result.Candidates[0].Content.Parts {
		// Handle function calls
		if part.FunctionCall != nil {
			switch part.FunctionCall.Name {
			case "create_shape":
				if shape := parseShapeFromFunctionCall(part.FunctionCall); shape != nil {
					createShapes = append(createShapes, *shape)
					hasShapeOperations = true
				}
			case "update_shape":
				if update := parseUpdateFromFunctionCall(part.FunctionCall); update != nil {
					err := a.sceneManager.UpdateShape(update.ID, update.Updates)
					if err != nil {
						a.events <- NewErrorEvent(err)
						return err
					}
					hasShapeOperations = true
				}
			case "remove_shape":
				if id := parseRemoveFromFunctionCall(part.FunctionCall); id != "" {
					err := a.sceneManager.RemoveShape(id)
					if err != nil {
						a.events <- NewErrorEvent(err)
						return err
					}
					hasShapeOperations = true
				}
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

	// Apply shape operations to scene manager and emit scene update
	if len(createShapes) > 0 || hasShapeOperations {
		// Add new shapes to scene manager if any
		if len(createShapes) > 0 {
			err := a.sceneManager.AddShapes(createShapes)
			if err != nil {
				a.events <- NewErrorEvent(fmt.Errorf("failed to add shapes to scene: %w", err))
				return err
			}
		}

		// Convert to raytracer scene and emit render event
		raytracerScene := a.sceneManager.ToRaytracerScene()
		a.events <- NewSceneRenderEvent(raytracerScene)
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
