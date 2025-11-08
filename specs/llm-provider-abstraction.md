# LLM Provider Abstraction

## Overview

Abstract the LLM provider implementation to support multiple models and providers (Gemini, Claude, OpenAI, etc.) with a unified interface. This will allow users to switch between models via a UI selector.

## Current State

### Tight Coupling Points
1. **Type Definitions**: All conversation/response types use `genai.Content`, `genai.Part`, `genai.FunctionCall`, etc.
2. **Tool Declarations**: Tools defined as `genai.FunctionDeclaration` with `genai.Schema`
3. **Client Interface**: `LLMClient` interface uses genai types in signature
4. **Agent Logic**: Direct references to genai structures for parsing responses
5. **Session Storage**: `ChatSession.Messages` is `[]*genai.Content`
6. **Model Hardcoded**: `"gemini-2.5-flash"` hardcoded in agent.go:93

### What Works
- There's already an `LLMClient` interface (agent/llm_client.go)
- Mock implementation exists for testing
- Agent uses interface, not direct client

## Goals

1. **Support Multiple Providers**:
   - Google Gemini (existing)
   - Anthropic Claude (via Messages API)
   - OpenAI (GPT-4, GPT-4o)

2. **UI Model Selector**:
   - Dropdown in chat interface
   - Persist selection in localStorage
   - Pass model choice to backend via API

3. **Provider-Agnostic Types**:
   - Define internal types for messages, function calls, tools
   - Each provider adapter translates to/from internal format

4. **Minimal Code Changes**:
   - Keep existing agent logic largely unchanged
   - Adapter pattern for provider-specific code

## Architecture

### Core Abstraction Layer

```go
// Internal message representation (provider-agnostic)
type Message struct {
    Role    string      // "user", "assistant", "function"
    Content []Part      // Message parts
}

type Part struct {
    Type         PartType     // text, function_call, function_response, image
    Text         string
    FunctionCall *FunctionCall
    FunctionResp *FunctionResponse
    ImageData    *ImageData
    Thought      bool         // For thinking tokens
}

type FunctionCall struct {
    Name      string
    Arguments map[string]interface{}
}

type FunctionResponse struct {
    Name     string
    Response map[string]interface{}
}

type ImageData struct {
    Data     []byte
    MIMEType string
}

// Tool definition (provider-agnostic)
type Tool struct {
    Name        string
    Description string
    Parameters  *Schema
}

type Schema struct {
    Type        SchemaType
    Description string
    Properties  map[string]*Schema
    Items       *Schema
    Enum        []string
    Required    []string
}
```

### Provider Interface

```go
type LLMProvider interface {
    // Generate content with function calling support
    GenerateContent(ctx context.Context, model string, messages []Message, tools []Tool) (*Response, error)

    // Get available models for this provider
    ListModels() []ModelInfo

    // Provider metadata
    Name() string
    SupportsVision() bool
    SupportsThinking() bool
}

type Response struct {
    Parts       []Part
    StopReason  string
}

type ModelInfo struct {
    ID          string  // "gemini-2.5-flash", "claude-3-5-sonnet-20241022"
    DisplayName string  // "Gemini 2.5 Flash", "Claude 3.5 Sonnet"
    Provider    string  // "google", "anthropic", "openai"
    Vision      bool
    Thinking    bool
    ContextWindow int
}
```

### Provider Implementations

```
agent/
  llm/
    provider.go          # LLMProvider interface, internal types
    registry.go          # Provider registry, model lookup
    gemini/
      client.go          # Gemini adapter
      translation.go     # genai <-> internal types
    anthropic/
      client.go          # Claude adapter
      translation.go     # Messages API <-> internal types
    openai/
      client.go          # OpenAI adapter
      translation.go     # Chat Completions <-> internal types
```

### Migration Strategy

#### Phase 1: Internal Type Definitions
- Define provider-agnostic message/tool types
- No behavioral changes yet

#### Phase 2: Gemini Adapter
- Create `agent/llm/gemini/` package
- Implement translation layer: genai ↔ internal types
- Wrap existing Gemini client
- Update Agent to use new types internally
- **Verify all tests pass**

#### Phase 3: Provider Registry
- Implement provider registration
- Model discovery and lookup
- Environment-based configuration (API keys)

#### Phase 4: Additional Providers
- Implement Anthropic adapter
- Implement OpenAI adapter (if desired)
- Test each independently

#### Phase 5: UI Integration
- Add model selector dropdown
- API endpoint to list available models
- Pass model selection to /api/chat
- Store selection in session

## API Changes

### GET /api/models
Returns available models based on configured API keys.

```json
{
  "models": [
    {
      "id": "gemini-2.5-flash",
      "display_name": "Gemini 2.5 Flash",
      "provider": "google",
      "vision": true,
      "thinking": true
    },
    {
      "id": "claude-3-5-sonnet-20241022",
      "display_name": "Claude 3.5 Sonnet",
      "provider": "anthropic",
      "vision": true,
      "thinking": true
    }
  ],
  "default": "gemini-2.5-flash"
}
```

### POST /api/chat (updated)
Add optional `model` field:

```json
{
  "session_id": "abc123",
  "message": "Create a sphere",
  "quality": "draft",
  "model": "claude-3-5-sonnet-20241022"  // NEW
}
```

If not specified, use session's current model or default.

## UI Changes

### Model Selector
Add to chat header alongside theme/quality switchers:

```html
<div class="model-switcher">
    <select id="modelSelect" class="model-select">
        <option value="gemini-2.5-flash">Gemini 2.5 Flash</option>
        <option value="claude-3-5-sonnet-20241022">Claude 3.5 Sonnet</option>
    </select>
</div>
```

- Fetch available models on page load
- Persist selection in localStorage
- Send with each message
- Show provider icon/badge

## Configuration

### Environment Variables
```bash
# Current
GOOGLE_API_KEY=...

# New (backward compatible)
GOOGLE_API_KEY=...
ANTHROPIC_API_KEY=...
OPENAI_API_KEY=...
```

### Default Model
- If only one provider configured → use that provider's default
- If multiple → prefer Gemini for backward compatibility
- Allow override via env var: `DEFAULT_MODEL=claude-3-5-sonnet-20241022`

## Testing Strategy

1. **Unit Tests**: Each adapter with mock HTTP clients
2. **Integration Tests**: Agent with each provider (using real API keys in CI)
3. **Regression Tests**: Ensure existing Gemini flow unchanged
4. **Cross-Provider Tests**: Same prompt to different providers, verify tool calls work

## Risks & Considerations

### Function Calling Differences
- **Gemini**: Native support, well-documented
- **Claude**: Tool use via Messages API, slightly different format
- **OpenAI**: Function calling deprecated, use tools instead

**Mitigation**: Adapter layer handles format differences transparently

### Thinking Tokens
- Gemini 2.5+ has native thinking support
- Claude 3.5+ has thinking support (different format)
- OpenAI doesn't have thinking tokens

**Mitigation**: `Part.Thought` field, providers set appropriately

### Vision Support
- All major models support vision
- Image formats may differ (base64, URLs)

**Mitigation**: Unified `ImageData` type, adapters handle encoding

### Conversation History
- Each provider has different limits
- Different tokenization

**Mitigation**: Store in internal format, adapter handles truncation if needed

### Cost Differences
- Different pricing models
- Need to track usage per provider

**Future Enhancement**: Add usage tracking, display costs to user

## Implementation Checklist

- [ ] Define internal types (Message, Part, Tool, Schema)
- [ ] Create LLMProvider interface
- [ ] Implement Gemini adapter with translation layer
- [ ] Update Agent to use internal types
- [ ] Update ChatSession to use internal types
- [ ] Create provider registry
- [ ] Implement model discovery
- [ ] Add /api/models endpoint
- [ ] Update /api/chat to accept model parameter
- [ ] Add Claude adapter (anthropic Messages API)
- [ ] Add OpenAI adapter (optional)
- [ ] Add UI model selector
- [ ] Persist model selection
- [ ] Update tests
- [ ] Update documentation (CLAUDE.md, README.md)

## Future Enhancements

- **Model-specific prompts**: Optimize system prompts per provider
- **Automatic fallback**: If one provider fails, try another
- **Cost tracking**: Show estimated costs per session
- **Rate limiting**: Handle provider-specific rate limits
- **Streaming responses**: Support streaming for faster UX
- **Local models**: Support Ollama, llama.cpp for local inference
