# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**scene-llm** is a web-based tool for generating 3D scenes using Large Language Models. Users interact with an LLM through a chat interface to create and modify scenes, which are then rendered using the [go-progressive-raytracer](https://github.com/df07/go-progressive-raytracer).

### Core Concept
- **Natural Language Scene Creation**: "Create a snowman" → LLM uses tools to build spheres, materials, lighting
- **Interactive Modifications**: "Make the light softer" → LLM adjusts scene parameters
- **Real-time Preview**: Immediate visual feedback via progressive raytracing

## Build Commands

```bash
# Run web server with auto-reload (requires GOOGLE_API_KEY env var)
cd web && air

# Run tests
go test ./...
```

## Testing Guidelines

- **Always run tests before committing**: Use `go test ./...` to ensure all tests pass
- **Add tests for new features**: When adding new shape types or helper functions, include comprehensive test cases covering edge cases and error conditions
- **Test both success and failure paths**: Validate that invalid inputs are properly handled and return appropriate errors
- **Avoid external LLM calls in tests**: Tests can call agent code but should never make actual LLM API calls (this adds cost and makes tests non-deterministic). Use mock function calls or direct method testing instead.

## Code Structure

**Data Flow**: LLM Function Call → Parse → Request → Execute → Event

**Type Hierarchy**:
1. **Raw Data** (`ShapeRequest`, `LightRequest`) - Extracted from LLM function call args
2. **Tool Requests** (`CreateShapeRequest`, `UpdateShapeRequest`, etc.) - Structured requests ready for execution
3. **Events** (`ToolCallEvent`, `ProcessingEvent`, etc.) - Streamed to frontend via SSE

**Validation Strategy**:
- **Parsing** (`tools.go`): Extract values, return zero values for missing/malformed data
- **Execution** (`scene.go`): Validate all fields, return errors for invalid data
- Centralized validation catches all malformed LLM input

**Key Files**:
- `agent/tools.go` - Tool declarations, request types, parsing
- `agent/agent.go` - Agentic loop, executes requests with validation
- `agent/scene.go` - Scene state management, validation logic
- `agent/events.go` - Event types for streaming updates to frontend
- `web/server/` - HTTP handlers, SSE, LLM chat integration

## Key Implementation Details

### LLM Provider
- **Current**: Google Gemini (`gemini-2.5-flash`) via `google.golang.org/genai`
- **Agentic Loop**: Max 10 turns with retry logic for network errors
- **System Prompt**: Dynamically generated with current scene context

### Supported Shapes & Materials
**Shapes**: sphere, box, quad, disc
**Materials**: lambertian (diffuse), metal (reflective), dielectric (glass/transparent)
**Lights**: point_spot, area_quad, disc_spot, area_sphere, area_disc_spot, infinite environment lights

### Property Bags & IDs
- **Property Bags**: Shapes/lights use `map[string]interface{}` for flexibility across different types and partial updates
- **IDs**: User-defined semantic strings (e.g., "blue_sphere"), must be unique, validated on create/update

### Frontend Communication
- **Protocol**: Server-Sent Events (SSE), not WebSocket
- **Events**: processing, llm_response, function_calls, scene_render, error, complete
- **Rendering**: Draft mode (10 samples) or High quality (500 samples)
- **Sessions**: Persist agent + SceneManager state across messages

### Error Handling Pattern
- **Parsing**: Permissive - returns zero values for missing data
- **Validation**: Strict - all validation in `scene.go`, returns `ValidationErrors`
- **LLM Feedback**: Tool failures return structured errors so LLM can retry