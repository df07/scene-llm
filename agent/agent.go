package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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
	var hasShapeOperations bool

	// Process all parts of the response
	for _, part := range result.Candidates[0].Content.Parts {
		// Handle function calls
		if part.FunctionCall != nil {
			operation := parseToolOperationFromFunctionCall(part.FunctionCall)
			if operation != nil {
				hasShapeOperations = true
				a.executeToolOperation(operation)
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

	// Emit scene render event if any operations were performed
	if hasShapeOperations {
		raytracerScene := a.sceneManager.ToRaytracerScene()
		a.events <- NewSceneRenderEvent(raytracerScene)
	}

	// Send completion event
	a.events <- NewCompleteEvent()
	return nil
}

// executeToolOperation executes a tool operation and emits appropriate events
func (a *Agent) executeToolOperation(operation ToolOperation) {
	startTime := time.Now()
	var err error

	switch op := operation.(type) {
	case *CreateShapeOperation:
		err = a.sceneManager.AddShapes([]ShapeRequest{op.Shape})
	case *UpdateShapeOperation:
		// Capture before state
		if beforeShape := a.sceneManager.FindShape(op.ID); beforeShape != nil {
			op.Before = beforeShape
		}

		err = a.sceneManager.UpdateShape(op.ID, op.Updates)

		// Capture after state if successful
		if err == nil {
			if afterShape := a.sceneManager.FindShape(op.ID); afterShape != nil {
				op.After = afterShape
			}
		}
	case *RemoveShapeOperation:
		// Capture shape before removal
		if beforeShape := a.sceneManager.FindShape(op.ID); beforeShape != nil {
			op.RemovedShape = beforeShape
		}

		err = a.sceneManager.RemoveShape(op.ID)
	}

	// Calculate duration
	duration := time.Since(startTime).Milliseconds()

	// Emit ToolCallEvent
	var errorMsg string
	success := err == nil
	if err != nil {
		errorMsg = err.Error()
	}

	a.events <- NewToolCallEvent(operation, success, errorMsg, duration)
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
