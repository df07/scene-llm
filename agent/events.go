package agent

import (
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// AgentEvent is the interface that all agent events implement
type AgentEvent interface {
	EventType() string
}

// Specific event types with type-safe data
type ProcessingEvent struct {
	Message string `json:"message"`
}

func (e ProcessingEvent) EventType() string { return "processing" }

type ResponseEvent struct {
	Text    string `json:"text"`
	Thought bool   `json:"thought,omitempty"` // True if this is a thinking token from the model
}

func (e ResponseEvent) EventType() string { return "llm_response" }

// ToolCallStartEvent indicates a tool has started executing
type ToolCallStartEvent struct {
	ID        string      `json:"id"`        // Unique ID for this tool call
	Request   ToolRequest `json:"request"`   // The tool request that is starting
	Timestamp time.Time   `json:"timestamp"` // When the tool started
}

func (e ToolCallStartEvent) EventType() string { return "function_call_start" }

// ToolCallEvent using ToolRequest (completion event)
type ToolCallEvent struct {
	ID            string      `json:"id"`      // Unique ID matching the start event
	Request       ToolRequest `json:"request"` // The tool request that was attempted
	Success       bool        `json:"success"` // Tool request result
	Error         string      `json:"error,omitempty"`
	Duration      int64       `json:"duration"`                 // Tool request duration in ms
	Timestamp     time.Time   `json:"timestamp"`                // When the tool request occurred
	RenderedImage []byte      `json:"rendered_image,omitempty"` // Image data for render_scene tool
}

func (e ToolCallEvent) EventType() string { return "function_calls" }

type SceneUpdateEvent struct {
	Scene *SceneState `json:"scene"`
}

func (e SceneUpdateEvent) EventType() string { return "scene_update" }

type SceneRenderEvent struct {
	RaytracerScene *scene.Scene `json:"-"` // Ready-to-render scene, not serialized
}

func (e SceneRenderEvent) EventType() string { return "scene_render" }

type ErrorEvent struct {
	Message string `json:"message"`
}

func (e ErrorEvent) EventType() string { return "error" }

type CompleteEvent struct {
	Message string `json:"message"`
}

func (e CompleteEvent) EventType() string { return "complete" }

// Helper functions for creating events
func NewProcessingEvent(message string) ProcessingEvent {
	return ProcessingEvent{Message: message}
}

func NewResponseEvent(text string) ResponseEvent {
	return ResponseEvent{Text: text}
}

// Helper for creating ToolCallStartEvent
func NewToolCallStartEvent(id string, request ToolRequest) ToolCallStartEvent {
	return ToolCallStartEvent{
		ID:        id,
		Request:   request,
		Timestamp: time.Now(),
	}
}

// Helper for creating ToolCallEvent with ToolRequest
func NewToolCallEvent(id string, request ToolRequest, success bool, errorMsg string, duration int64) ToolCallEvent {
	return ToolCallEvent{
		ID:        id,
		Request:   request,
		Success:   success,
		Error:     errorMsg,
		Duration:  duration,
		Timestamp: time.Now(),
	}
}

func NewSceneUpdateEvent(scene *SceneState) SceneUpdateEvent {
	return SceneUpdateEvent{Scene: scene}
}

func NewSceneRenderEvent(raytracerScene *scene.Scene) SceneRenderEvent {
	return SceneRenderEvent{RaytracerScene: raytracerScene}
}

func NewErrorEvent(err error) ErrorEvent {
	return ErrorEvent{Message: err.Error()}
}

func NewCompleteEvent() CompleteEvent {
	return CompleteEvent{Message: "Processing finished"}
}
