# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**scene-llm** is a web-based tool for generating 3D scenes using Large Language Models. Users interact with an LLM through a chat interface to create and modify scenes, which are then rendered using the [go-progressive-raytracer](https://github.com/df07/go-progressive-raytracer).

### Core Concept
- **Natural Language Scene Creation**: "Create a snowman" → LLM uses tools to build spheres, materials, lighting
- **Interactive Modifications**: "Make the light softer" → LLM adjusts scene parameters
- **Real-time Preview**: Immediate visual feedback via progressive raytracing

## Architecture

```
scene-llm/
├── agent/                    # LLM agent with agentic loop, function calling, error recovery
├── web/
│   ├── main.go               # Web server entry point
│   ├── server/               # HTTP handlers, WebSocket, LLM chat integration
│   └── static/               # HTML/JS/CSS for chat interface
├── specs/                    # Design specifications
└── go.mod                    # Imports go-progressive-raytracer as dependency
```

## Build Commands

```bash
# Run web server with auto-reload
cd web && air

# Or build and run manually
cd web && go build -o scene-server main.go && ./scene-server

# Run tests
go test ./...
```

## Testing Guidelines

- **Always run tests before committing**: Use `go test ./...` to ensure all tests pass
- **Add tests for new features**: When adding new shape types or helper functions, include comprehensive test cases covering edge cases and error conditions
- **Test both success and failure paths**: Validate that invalid inputs are properly handled and return appropriate errors
- **Avoid external LLM calls in tests**: Tests can call agent code but should never make actual LLM API calls (this adds cost and makes tests non-deterministic). Use mock function calls or direct method testing instead.

## Code Structure

### Agent Package (`agent/`)

**Data Flow**: LLM Function Call → Parse → Request → Execute → Event

**Key Files**:
- `tools.go` - Tool declarations, request types, parsing (converts LLM calls to requests)
- `agent.go` - Agentic loop, executes requests with validation
- `scene.go` - Scene state management, validation logic
- `events.go` - Event types for streaming updates to frontend

**Type Hierarchy**:
1. **Raw Data** (`ShapeRequest`, `LightRequest`) - Extracted from LLM function call args
2. **Tool Requests** (`CreateShapeRequest`, `UpdateShapeRequest`, etc.) - Structured requests ready for execution
   - All embed `BaseToolRequest` with `ToolType` and `Id` fields
   - Implement `ToolRequest` interface for polymorphic handling
3. **Events** (`ToolCallEvent`, `ThinkingEvent`, etc.) - Streamed to frontend via WebSocket

**Validation Strategy**:
- **Parsing** (`tools.go`): Extract values, return zero values for missing/malformed data
- **Execution** (`agent.go` → `scene.go`): Validate all fields, return errors for invalid data
- Centralized validation catches all malformed LLM input

**Request vs Event**:
- **Request**: Unvalidated LLM intention (what the LLM wants to do)
- **Event**: Validated result after execution (what actually happened)

### Web Package (`web/`)

- `main.go` - Server entry point
- `server/` - HTTP handlers, WebSocket, LLM chat integration, event streaming
- `static/` - Frontend HTML/JS/CSS

## Technical Approach

1. **Scene Schema**: Custom JSON format optimized for LLM function calls
2. **LLM Integration**: Function calling to manipulate scene elements (create_shape, set_camera, etc.)
3. **Scene Management**: In-memory scene state with validation and conversion to raytracer format
4. **Progressive Rendering**: Real-time tile streaming for immediate visual feedback
5. **Multi-Provider Support**: Pluggable LLM providers to optimize cost/capability