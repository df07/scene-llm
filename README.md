# Scene LLM

Exploring agentic LLM patterns through 3D scene generation. Built as a natural language interface on top of [go-progressive-raytracer](https://github.com/df07/go-progressive-raytracer).

![Scene LLM creating a snowman](renders/web%20interface.png)

## What is this?

An experiment in building an agentic LLM system that can iteratively create 3D scenes. Features: 
- Agentic loop for scene creation (LLM sees tool results, can retry/refine autonomously)
- Function calling with validation and error recovery
- Streaming multi-turn conversations
- Progressive rendering and real-time feedback

## Quick Start

### Prerequisites

- Go 1.24+
- Google Gemini API key

### Running the Web Interface

```bash
# Set your API key
export GOOGLE_API_KEY=your_key_here

# Install and run with auto-reload
cd web
go install github.com/air-verse/air@latest
air

# Or build and run manually
go build -o scene-server main.go
./scene-server
```

Visit `http://localhost:8081` to start creating scenes.

## Architecture

- **LLM Agent** (`agent/`): Agentic loop with function calling, error recovery, and iterative refinement
- **Scene Manager** (`agent/scene.go`): Scene state management and validation
- **Web Server** (`web/server/`): WebSocket-based chat interface with streaming responses
- **Raytracer**: Uses [go-progressive-raytracer](https://github.com/df07/go-progressive-raytracer) for rendering


## Development

```bash
# Run tests
go test ./...

# Run agent tests with mock LLM
go test ./agent -v

# Build CLI tool
go build -o scene-llm main.go
```

## License

MIT
