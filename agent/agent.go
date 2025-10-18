package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

// Agent handles LLM conversations and tool execution
type Agent struct {
	llmClient    LLMClient
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
		llmClient:    &GeminiClient{client: client},
		events:       events,
		sceneManager: NewSceneManager(),
	}, nil
}

// NewWithMockLLM creates an agent with a mock LLM client for testing
func NewWithMockLLM(events chan<- AgentEvent, mockClient LLMClient) *Agent {
	return &Agent{
		llmClient:    mockClient,
		events:       events,
		sceneManager: NewSceneManager(),
	}
}

// SetEventsChannel sets the events channel for this agent
func (a *Agent) SetEventsChannel(events chan<- AgentEvent) {
	a.events = events
}

// GetSceneManager returns the scene manager for this agent
func (a *Agent) GetSceneManager() *SceneManager {
	return a.sceneManager
}

// ProcessMessage handles a conversation with agentic loop and emits events
func (a *Agent) ProcessMessage(ctx context.Context, conversation []*genai.Content) error {
	const maxTurns = 10

	// Send processing event
	a.events <- NewProcessingEvent("🤖 Processing your request...")

	// Build scene context from our internal scene manager
	sceneContext := a.sceneManager.BuildContext()

	// Add scene context to the latest message (only for initial user message)
	contextualizedConversation := a.addSceneContext(conversation, sceneContext)

	// Prepare tools
	tools := []*genai.Tool{{FunctionDeclarations: getAllToolDeclarations()}}

	// Agentic loop
	turnCount := 0
	for {
		// Check turn limit
		if turnCount >= maxTurns {
			a.events <- NewResponseEvent(fmt.Sprintf("Reached maximum turn limit (%d turns). Send a message to continue.", maxTurns))
			a.events <- NewCompleteEvent()
			return nil
		}

		// Generate content with function calling and retry logic
		result, err := a.generateContentWithRetry(ctx, "gemini-2.5-flash", contextualizedConversation, &genai.GenerateContentConfig{
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

		var functionCalls []*genai.FunctionCall
		var hasToolRequests bool

		// Collect all parts from the response
		for _, part := range result.Candidates[0].Content.Parts {
			if part.FunctionCall != nil {
				functionCalls = append(functionCalls, part.FunctionCall)
			} else if part.Text != "" {
				// Emit each text part as a separate response event
				a.events <- NewResponseEvent(part.Text)
			} else {
				// Log unexpected part types
				log.Printf("WARNING: Received unexpected part type from LLM (not FunctionCall or Text)")
			}
		}

		// If no tool calls, we're done (LLM signaled completion)
		if len(functionCalls) == 0 {
			break
		}

		// Execute tool calls and collect results
		var functionResponses []*genai.Part
		for _, fc := range functionCalls {
			operation := parseToolRequestFromFunctionCall(fc)
			if operation != nil {
				hasToolRequests = true
				toolResult := a.executeToolRequests(operation)

				// Convert ToolResult to map for FunctionResponse
				resultMap := make(map[string]any)
				if toolResult.Success {
					resultMap["success"] = true
					resultMap["result"] = toolResult.Result
				} else {
					resultMap["success"] = false
					resultMap["errors"] = toolResult.Errors
				}

				// Create function response for conversation history
				functionResponses = append(functionResponses, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     fc.Name,
						Response: resultMap,
					},
				})
			}
		}

		// Emit scene render event if any operations were performed
		if hasToolRequests {
			raytracerScene, err := a.sceneManager.ToRaytracerScene()
			if err != nil {
				a.events <- NewErrorEvent(fmt.Errorf("failed to create scene: %w", err))
			} else {
				a.events <- NewSceneRenderEvent(raytracerScene)
			}
			hasToolRequests = false // Reset for next iteration
		}

		// Append assistant's response to conversation
		contextualizedConversation = append(contextualizedConversation, result.Candidates[0].Content)

		// Append function responses to conversation
		if len(functionResponses) > 0 {
			contextualizedConversation = append(contextualizedConversation, &genai.Content{
				Role:  "function",
				Parts: functionResponses,
			})
		}

		// Increment turn counter
		turnCount++
	}

	// Send completion event
	a.events <- NewCompleteEvent()
	return nil
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Errors  []string    `json:"errors,omitempty"`
}

// executeToolRequests executes a tool operation and returns structured result
func (a *Agent) executeToolRequests(operation ToolRequest) ToolResult {
	startTime := time.Now()
	var err error
	var result interface{}

	switch op := operation.(type) {
	case *CreateShapeRequest:
		err = a.sceneManager.AddShapes([]ShapeRequest{op.Shape})
		if err == nil {
			// Return the created shape
			result = op.Shape
		}
	case *UpdateShapeRequest:
		// Capture before state
		if beforeShape := a.sceneManager.FindShape(op.Id); beforeShape != nil {
			op.Before = beforeShape
		}

		err = a.sceneManager.UpdateShape(op.Id, op.Updates)

		// Capture after state if successful
		if err == nil {
			if afterShape := a.sceneManager.FindShape(op.Id); afterShape != nil {
				op.After = afterShape
				result = afterShape
			}
		}
	case *RemoveShapeRequest:
		// Capture shape before removal
		if beforeShape := a.sceneManager.FindShape(op.Id); beforeShape != nil {
			op.RemovedShape = beforeShape
		}

		err = a.sceneManager.RemoveShape(op.Id)
		if err == nil {
			result = map[string]string{"id": op.Id, "status": "removed"}
		}
	case *SetEnvironmentLightingRequest:
		err = a.sceneManager.SetEnvironmentLighting(op.LightingType, op.TopColor, op.BottomColor, op.Emission)
		if err == nil {
			result = map[string]interface{}{
				"lighting_type": op.LightingType,
				"top_color":     op.TopColor,
				"bottom_color":  op.BottomColor,
				"emission":      op.Emission,
			}
		}
	case *CreateLightRequest:
		err = a.sceneManager.AddLights([]LightRequest{op.Light})
		if err == nil {
			result = op.Light
		}
	case *UpdateLightRequest:
		// Capture before state
		if beforeLight := a.sceneManager.FindLight(op.Id); beforeLight != nil {
			op.Before = beforeLight
		}

		err = a.sceneManager.UpdateLight(op.Id, op.Updates)

		// Capture after state if successful
		if err == nil {
			if afterLight := a.sceneManager.FindLight(op.Id); afterLight != nil {
				op.After = afterLight
				result = afterLight
			}
		}
	case *RemoveLightRequest:
		// Capture light before removal
		if beforeLight := a.sceneManager.FindLight(op.Id); beforeLight != nil {
			op.RemovedLight = beforeLight
		}

		err = a.sceneManager.RemoveLight(op.Id)
		if err == nil {
			result = map[string]string{"id": op.Id, "status": "removed"}
		}
	case *SetCameraRequest:
		err = a.sceneManager.SetCamera(op.Camera)
		if err == nil {
			result = op.Camera
		}
	}

	// Calculate duration
	duration := time.Since(startTime).Milliseconds()

	// Emit ToolCallEvent (for UI display)
	var errorMsg string
	var errors []string
	success := err == nil
	if err != nil {
		errorMsg = err.Error()
		// Check if error is ValidationErrors to extract individual errors
		if validationErrs, ok := err.(ValidationErrors); ok {
			errors = []string(validationErrs)
		} else {
			errors = []string{errorMsg}
		}
	}

	a.events <- NewToolCallEvent(operation, success, errorMsg, duration)

	// Return structured result (for LLM feedback)
	if success {
		return ToolResult{Success: true, Result: result}
	}
	return ToolResult{Success: false, Errors: errors}
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

		systemPrompt := `You are an autonomous 3D scene creation assistant. Your job is to help users create and modify 3D scenes using raytracing.

AVAILABLE TOOLS:
You have access to tools for creating, updating, and removing shapes and lights. Each tool call will return a JSON result showing you what happened.
You can make multiple tool calls in a single response.

WORKFLOW:
1. Explain to the user what you're doing as you work
2. Call tools to create/modify the scene
3. Review tool results - if there are errors, retry with corrections
4. Iterate until the scene matches the user's request
5. When satisfied, provide a final response (text only, no tool calls) to signal completion

TOOL RESULTS:
- Success: {"success": true, "result": {<full object>}}
- Error: {"success": false, "error": "<error message>"}

The results show the complete state of each object, including any defaults that were applied. Use these to track what's in the scene and validate your work.

CURRENT SCENE:
%s

USER REQUEST:
%s`

		contextualText := fmt.Sprintf(systemPrompt, sceneContext, originalText)

		// Create a new part with the contextualized text
		lastMessage.Parts[0] = &genai.Part{Text: contextualText}
	}

	return contextualizedConversation
}

// generateContentWithRetry wraps the GenerateContent call with retry logic for transient errors
func (a *Agent) generateContentWithRetry(ctx context.Context, model string, conversation []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	const maxRetries = 3
	const baseDelay = 1 * time.Second

	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err := a.llmClient.GenerateContent(ctx, model, conversation, config)
		if err == nil {
			if attempt > 0 {
				log.Printf("Gemini API call succeeded on attempt %d", attempt+1)
			}
			return result, nil
		}

		lastErr = err
		errStr := strings.ToLower(err.Error())

		// Check if this is a transient network error worth retrying
		isRetryable := strings.Contains(errStr, "connection reset by peer") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "temporary failure") ||
			strings.Contains(errStr, "network error")

		if !isRetryable || attempt == maxRetries-1 {
			// Don't retry for non-network errors or on final attempt
			break
		}

		delay := baseDelay * time.Duration(1<<uint(attempt)) // Exponential backoff: 1s, 2s, 4s
		log.Printf("Gemini API call failed (attempt %d/%d): %v. Retrying in %v...", attempt+1, maxRetries, err, delay)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	return nil, lastErr
}

// Close cleans up the agent resources
func (a *Agent) Close() error {
	// genai.Client doesn't have a Close method in this version
	return nil
}
