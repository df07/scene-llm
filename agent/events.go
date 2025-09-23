package agent

import "github.com/df07/go-progressive-raytracer/pkg/scene"

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

type ToolCallEvent struct {
	Shapes []ShapeRequest `json:"shapes"`
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

func NewToolCallEvent(shapes []ShapeRequest) ToolCallEvent {
	return ToolCallEvent{Shapes: shapes}
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
