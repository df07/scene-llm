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
type ThinkingEvent struct {
	Message string `json:"message"`
}

func (e ThinkingEvent) EventType() string { return "thinking" }

type ResponseEvent struct {
	Text string `json:"text"`
}

func (e ResponseEvent) EventType() string { return "llm_response" }

// Legacy ToolCallEvent - keeping temporarily for backward compatibility
type LegacyToolCallEvent struct {
	Shapes []ShapeRequest `json:"shapes"`
}

func (e LegacyToolCallEvent) EventType() string { return "function_calls" }

// New ToolCallEvent using ToolOperation
type ToolCallEvent struct {
	Operation ToolOperation `json:"operation"` // The tool operation that was attempted
	Success   bool          `json:"success"`   // Operation result
	Error     string        `json:"error,omitempty"`
	Duration  int64         `json:"duration"`  // Operation duration in ms
	Timestamp time.Time     `json:"timestamp"` // When the operation occurred
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
func NewThinkingEvent(message string) ThinkingEvent {
	return ThinkingEvent{Message: message}
}

func NewResponseEvent(text string) ResponseEvent {
	return ResponseEvent{Text: text}
}

// Legacy helper - keeping for backward compatibility
func NewLegacyToolCallEvent(shapes []ShapeRequest) LegacyToolCallEvent {
	return LegacyToolCallEvent{Shapes: shapes}
}

// New helper for creating ToolCallEvent with ToolOperation
func NewToolCallEvent(operation ToolOperation, success bool, errorMsg string, duration int64) ToolCallEvent {
	return ToolCallEvent{
		Operation: operation,
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

// ToolOperation interface - describes what the LLM wanted to do
type ToolOperation interface {
	ToolName() string // "create_shape", "update_shape", "remove_shape"
	Target() string   // Shape ID being operated on (if applicable), empty otherwise
}

// Concrete tool operations - pure data structures describing LLM intentions
type CreateShapeOperation struct {
	Shape    ShapeRequest `json:"shape"`
	ToolType string       `json:"tool_name"` // For JSON serialization
}

func (op CreateShapeOperation) ToolName() string { return "create_shape" }
func (op CreateShapeOperation) Target() string   { return op.Shape.ID }

type UpdateShapeOperation struct {
	ID       string                 `json:"id"`
	Updates  map[string]interface{} `json:"updates"`
	Before   *ShapeRequest          `json:"before,omitempty"` // Populated by agent after execution
	After    *ShapeRequest          `json:"after,omitempty"`  // Populated by agent after execution
	ToolType string                 `json:"tool_name"`        // For JSON serialization
}

func (op UpdateShapeOperation) ToolName() string { return "update_shape" }
func (op UpdateShapeOperation) Target() string   { return op.ID }

type RemoveShapeOperation struct {
	ID           string        `json:"id"`
	RemovedShape *ShapeRequest `json:"removed_shape,omitempty"` // Populated by agent after execution
	ToolType     string        `json:"tool_name"`               // For JSON serialization
}

func (op RemoveShapeOperation) ToolName() string { return "remove_shape" }
func (op RemoveShapeOperation) Target() string   { return op.ID }
