package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"log"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/scene-llm/agent/llm"
	"github.com/df07/scene-llm/agent/llm/gemini"
)

// Agent handles LLM conversations and tool execution
type Agent struct {
	provider     llm.LLMProvider // LLM provider interface
	modelID      string          // Model ID (e.g., "gemini-2.5-flash")
	events       chan<- AgentEvent
	sceneManager *SceneManager
}

// NewWithProvider creates an agent using the new provider interface
func NewWithProvider(events chan<- AgentEvent, provider llm.LLMProvider, modelID string) *Agent {
	return &Agent{
		provider:     provider,
		modelID:      modelID,
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
// Returns the updated conversation history including assistant responses and function calls
func (a *Agent) ProcessMessage(ctx context.Context, conversation []llm.Message) ([]llm.Message, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("agent has no provider - use NewWithProvider")
	}
	const maxTurns = 10

	// Send processing event
	a.events <- NewProcessingEvent("🤖 Processing your request...")

	// Build scene context from our internal scene manager
	sceneContext := a.sceneManager.BuildContext()

	// Add scene context to the latest message
	conversation = a.addSceneContext(conversation, sceneContext)

	// Get tool declarations (convert from genai format to internal format)
	genaiTools := getAllToolDeclarations()
	tools := gemini.ToInternalTools(genaiTools)

	// Work with conversation directly (already in internal format)
	messages := conversation

	// Agentic loop
	turnCount := 0
	for {
		// Check turn limit
		if turnCount >= maxTurns {
			a.events <- NewResponseEvent(fmt.Sprintf("Reached maximum turn limit (%d turns). Send a message to continue.", maxTurns))
			a.events <- NewCompleteEvent()
			return messages, nil
		}

		// Generate content using provider
		response, err := a.provider.GenerateContent(ctx, a.modelID, messages, tools)
		if err != nil {
			log.Printf("Failed to generate content: %v", err)
			// Check if this is a context cancellation
			if errors.Is(err, context.Canceled) {
				return messages, context.Canceled
			}
			a.events <- NewErrorEvent(fmt.Errorf("LLM generation failed: %w", err))
			return messages, err
		}

		// Check for empty response
		if len(response.Parts) == 0 {
			log.Printf("No response from LLM")
			a.events <- NewErrorEvent(fmt.Errorf("no response from LLM"))
			return messages, fmt.Errorf("no response from LLM")
		}

		var functionCalls []*llm.FunctionCall
		var hasToolRequests bool

		// Process response parts
		for _, part := range response.Parts {
			if part.Type == llm.PartTypeFunctionCall && part.FunctionCall != nil {
				functionCalls = append(functionCalls, part.FunctionCall)
			} else if part.Type == llm.PartTypeText && part.Text != "" {
				a.events <- ResponseEvent{Text: part.Text, Thought: part.Thought}
			}
		}

		// Append assistant's response to conversation
		messages = append(messages, llm.Message{
			Role:  "assistant",
			Parts: response.Parts,
		})

		// If no function calls, we're done
		if len(functionCalls) == 0 {
			break
		}

		// Execute function calls and collect results
		var functionResponses []llm.Part
		for _, fc := range functionCalls {
			operation := parseToolRequestFromFunctionCall(fc)
			if operation != nil {
				hasToolRequests = true
				toolResult := a.executeToolRequests(operation)

				// Convert result to internal format
				resultMap := make(map[string]interface{})
				if toolResult.Success {
					resultMap["success"] = true
					resultMap["result"] = toolResult.Result
				} else {
					resultMap["success"] = false
					resultMap["errors"] = toolResult.Errors
				}

				functionResponses = append(functionResponses, llm.Part{
					Type: llm.PartTypeFunctionResponse,
					FunctionResp: &llm.FunctionResponse{
						Name:     fc.Name,
						Response: resultMap,
					},
				})

				// Handle render_scene image
				if renderReq, ok := operation.(*RenderSceneRequest); ok && renderReq.RenderedImage != nil {
					functionResponses = append(functionResponses, llm.Part{
						Type: llm.PartTypeImage,
						ImageData: &llm.ImageData{
							Data:     renderReq.RenderedImage,
							MIMEType: "image/png",
						},
					})
				}
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
			hasToolRequests = false
		}

		// Append function responses
		if len(functionResponses) > 0 {
			messages = append(messages, llm.Message{
				Role:  "function",
				Parts: functionResponses,
			})
		}

		turnCount++
	}

	// Send completion event
	a.events <- NewCompleteEvent()
	return messages, nil
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
	toolCallID := fmt.Sprintf("%s_%d", operation.ToolName(), startTime.UnixNano())
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
	case *RenderSceneRequest:
		// Emit start event to show "Rendering..." in UI
		a.events <- NewToolCallStartEvent(toolCallID, operation)

		// Get scene for rendering
		raytracerScene, sceneErr := a.sceneManager.ToRaytracerScene()
		if sceneErr != nil {
			err = fmt.Errorf("failed to create scene: %w", sceneErr)
			break
		}

		if len(raytracerScene.Shapes) == 0 {
			err = fmt.Errorf("cannot render empty scene - add shapes first")
			break
		}

		log.Printf("[render_scene] Scene has %d shapes, camera at %v looking at %v",
			len(raytracerScene.Shapes),
			raytracerScene.CameraConfig.Center,
			raytracerScene.CameraConfig.LookAt)

		// Render at same size as user preview (400x300) with high quality (500 samples)
		config := renderer.DefaultProgressiveConfig()
		config.MaxPasses = 1
		config.MaxSamplesPerPixel = 500

		// Use the scene's default dimensions (400x300) - don't modify them

		logger := renderer.NewDefaultLogger()
		integ := integrator.NewPathTracingIntegrator(raytracerScene.SamplingConfig)

		raytracer, renderErr := renderer.NewProgressiveRaytracer(raytracerScene, config, integ, logger)
		if renderErr != nil {
			err = fmt.Errorf("failed to create raytracer: %w", renderErr)
			break
		}

		// Render (synchronous - this takes several seconds)
		resultImg, _, renderErr := raytracer.RenderPass(1, nil)
		if renderErr != nil {
			err = fmt.Errorf("render failed: %w", renderErr)
			break
		}

		// Encode as PNG
		var buf bytes.Buffer
		if encodeErr := png.Encode(&buf, resultImg); encodeErr != nil {
			err = fmt.Errorf("failed to encode image: %w", encodeErr)
			break
		}

		// Store image in request
		op.RenderedImage = buf.Bytes()

		// Return success with metadata
		result = map[string]interface{}{
			"shape_count":       len(raytracerScene.Shapes),
			"samples_per_pixel": 500,
			"width":             raytracerScene.SamplingConfig.Width,
			"height":            raytracerScene.SamplingConfig.Height,
			"render_time_ms":    time.Since(startTime).Milliseconds(),
		}
	case *GetSceneStateRequest:
		// Get the complete scene state as JSON
		sceneState := a.sceneManager.GetSceneState()

		// Store in request for potential use
		op.SceneState = sceneState

		// Return the scene state
		result = sceneState
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

	// Create tool call event with image data if this is a render_scene request
	toolEvent := NewToolCallEvent(toolCallID, operation, success, errorMsg, duration)
	if renderReq, ok := operation.(*RenderSceneRequest); ok && renderReq.RenderedImage != nil {
		toolEvent.RenderedImage = renderReq.RenderedImage
	}
	a.events <- toolEvent

	// Return structured result (for LLM feedback)
	if success {
		return ToolResult{Success: true, Result: result}
	}
	return ToolResult{Success: false, Errors: errors}
}

// addSceneContext prepends scene context to the latest user message
func (a *Agent) addSceneContext(conversation []llm.Message, sceneContext string) []llm.Message {
	if len(conversation) == 0 {
		return conversation
	}

	// Make a copy to avoid modifying the original
	contextualizedConversation := make([]llm.Message, len(conversation))
	copy(contextualizedConversation, conversation)

	// Add context to the latest user message
	lastMessage := &contextualizedConversation[len(contextualizedConversation)-1]
	if lastMessage.Role == "user" && len(lastMessage.Parts) > 0 {
		originalText := lastMessage.Parts[0].Text

		systemPrompt := `You are an autonomous 3D scene creation assistant with vision capabilities. Your job is to help users create and modify 3D scenes using raytracing.

AVAILABLE TOOLS:
You have access to tools for creating, updating, and removing shapes and lights. Each tool call will return a JSON result showing you what happened.
You can make multiple tool calls in a single response.

AUTOMATIC RENDERING:
The user sees an automatically rendered preview after each tool call. You do NOT need to render the scene for the user.

VISUAL VERIFICATION (render_scene tool):
You have vision and can see rendered images. Use the render_scene tool to verify your work meets the user's request. The rendered image will be sent to you and you can analyze it visually to check colors, materials, lighting, composition, and overall appearance. This is expensive (500 samples, ~3-5 seconds), so use it strategically - typically once after completing major work or when the user asks you to verify something specific.

WORKFLOW:
1. Explain to the user what you're doing as you work
2. Call tools to create/modify the scene
3. Review tool results - if there are errors, retry with corrections
4. Call render_scene to verify the visual result matches the user's request
5. If the render looks wrong, make corrections and verify again
6. When satisfied with the visual result, provide a final response (text only, no tool calls) to signal completion

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
		lastMessage.Parts[0] = llm.Part{Type: llm.PartTypeText, Text: contextualText}
	}

	return contextualizedConversation
}

// Close cleans up the agent resources
func (a *Agent) Close() error {
	return nil
}
