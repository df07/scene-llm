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
├── main.go                    # CLI entry point
├── web/
│   ├── main.go               # Web server entry point
│   ├── server/               # HTTP handlers, WebSocket, LLM chat integration
│   └── static/               # HTML/JS/CSS for chat interface
├── pkg/                      # Packages (to be created as needed)
└── go.mod                    # Imports go-progressive-raytracer as dependency
```

## Build Commands

```bash
# Build CLI tool
go build -o scene-llm main.go

# Build web server
cd web && go build -o scene-server main.go

# Run web server with auto-reload (from web directory)
cd web && air

# Run tests
go test ./...
```

## Technical Approach

1. **Scene Schema**: Custom JSON format optimized for LLM function calls
2. **LLM Integration**: Function calling to manipulate scene elements (add_sphere, set_lighting, etc.)
3. **Scene Management**: In-memory scene state with conversion to raytracer format
4. **Progressive Rendering**: Real-time tile streaming for immediate visual feedback
5. **Multi-Provider Support**: Pluggable LLM providers to optimize cost/capability