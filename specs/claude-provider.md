# Specification: Claude Provider Implementation

## Overview

Add support for Anthropic's Claude models to the scene-llm application using the official `anthropic-sdk-go`. This will enable users to select Claude models (Sonnet, Opus, Haiku) alongside existing Gemini models.

## Goals

1. Implement `LLMProvider` interface for Claude/Anthropic
2. Support all Claude model variants (Sonnet, Opus, Haiku)
3. Enable tool/function calling for scene generation
4. Support vision capabilities (image inputs)
5. Support extended thinking mode (where available)
6. Maintain consistent behavior with existing Gemini provider

## Non-Goals

- Streaming support (not required for current use case)
- Batch API support
- Prompt caching (can be added later as optimization)
- Computer use tools (specialized capability not needed)

## Architecture

### Package Structure

```
agent/llm/claude/
├── provider.go       # LLMProvider implementation
├── translation.go    # Internal ↔ Claude SDK format translation
├── provider_test.go  # Unit tests
└── translation_test.go
```

### Dependencies

- Add `github.com/anthropics/anthropic-sdk-go` to go.mod
- Requires `ANTHROPIC_API_KEY` environment variable

## Implementation Details

### 1. Provider Implementation (`provider.go`)

```go
type Provider struct {
    client *anthropic.Client
}

func NewProvider(ctx context.Context, apiKey string) (*Provider, error)
func (p *Provider) GenerateContent(ctx, model, messages, tools) (*llm.Response, error)
func (p *Provider) ListModels() []llm.ModelInfo
func (p *Provider) Name() string
func (p *Provider) SupportsVision() bool
func (p *Provider) SupportsThinking() bool
func (p *Provider) Close() error
```

**Key Considerations:**

- **Client Initialization**: Use `option.WithAPIKey()` for configuration
- **Model Selection**: Accept full model IDs (e.g., `claude-sonnet-4.5-20250929`)
- **Error Handling**: Wrap SDK errors with "Claude API error: %w" for consistency
- **Context Support**: Pass context through for cancellation support

### 2. Translation Layer (`translation.go`)

Required translation functions to convert between internal LLM types and Claude SDK types:

#### Message Translation

- `FromInternalMessages([]llm.Message) -> []anthropic.MessageParam`
- `ToInternalMessage(anthropic.Message) -> llm.Message`

**Role Mapping:**
```
Internal -> Claude
"user"      -> anthropic.RoleUser
"assistant" -> anthropic.RoleAssistant
```

**Note:** Claude uses system prompts separately, not as messages. Handle system messages by:
- Extracting first message if role="system"
- Passing to `SystemPrompt` parameter in request
- All subsequent messages must alternate user/assistant

#### Content/Part Translation

- `FromInternalPart(llm.Part) -> anthropic.MessageParamContentUnion`
- `ToInternalPart(anthropic.ContentBlock) -> llm.Part`

**Part Type Mapping:**
```
Internal              -> Claude
PartTypeText          -> anthropic.NewTextBlock()
PartTypeFunctionCall  -> Not in user messages (Claude doesn't support this)
PartTypeFunctionResponse -> anthropic.NewToolResultBlock()
PartTypeImage         -> anthropic.NewImageBlock()
```

**Claude -> Internal:**
```
ContentBlockTypeText      -> PartTypeText
ContentBlockTypeToolUse   -> PartTypeFunctionCall
```

**Thinking Support:**
- Claude uses `anthropic.BetaExtendedThinking` for extended reasoning
- Set `Type: anthropic.MessageParamContentBlockThinkingBlock` for thinking prompts
- Response thinking appears as `ContentBlockTypeThinking`
- Map to internal `Part.Thought = true`

#### Tool Translation

- `FromInternalTools([]llm.Tool) -> []anthropic.ToolUnionParam`
- `FromInternalSchema(*llm.Schema) -> anthropic.ToolParam`

**Schema Translation:**
```go
// Convert our Schema to Claude's InputSchema
// Claude expects standard JSON Schema format
type: object, array, string, number, integer, boolean
properties: map[string]Schema
items: Schema (for arrays)
enum: []string
required: []string
```

#### Response Translation

- `ToInternalResponse(anthropic.Message) -> (*llm.Response, error)`

**Stop Reason Mapping:**
```
Claude                 -> Internal
StopReasonEndTurn      -> "stop"
StopReasonMaxTokens    -> "max_tokens"
StopReasonToolUse      -> "tool_use"
StopReasonStopSequence -> "stop"
```

### 3. Model Listing

Claude doesn't provide a dynamic model listing API. Use a hardcoded list of current models:

```go
func (p *Provider) ListModels() []llm.ModelInfo {
    return []llm.ModelInfo{
        {
            ID:            "claude-sonnet-4.5-20250929",
            DisplayName:   "Claude Sonnet 4.5",
            Provider:      "anthropic",
            Vision:        true,
            Thinking:      true,
            ContextWindow: 200000,
        },
        {
            ID:            "claude-opus-4-20250514",
            DisplayName:   "Claude Opus 4",
            Provider:      "anthropic",
            Vision:        true,
            Thinking:      true,
            ContextWindow: 200000,
        },
        {
            ID:            "claude-haiku-4-20250514",
            DisplayName:   "Claude Haiku 4",
            Provider:      "anthropic",
            Vision:        true,
            Thinking:      false,
            ContextWindow: 200000,
        },
        // Add older versions as needed
    }
}
```

**Note:** Update this list when new Claude models are released. Consider adding a build-time or runtime check for new models.

### 4. Configuration & Initialization

**Environment Variables:**
- `ANTHROPIC_API_KEY` - Required for Claude provider

**Registry Integration:**
- Modify `web/server/server.go` to initialize Claude provider if API key is present
- Add to registry alongside Gemini provider
- Models from both providers appear in dropdown

```go
// In server initialization
if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
    claudeProvider, err := claude.NewProvider(ctx, apiKey)
    if err == nil {
        registry.Add(claudeProvider)
    }
}
```

## Key Differences: Claude vs Gemini

### 1. System Prompts

**Gemini:** System messages are part of the conversation history
**Claude:** System prompt is a separate parameter in the request

**Solution:** Extract system message from conversation and pass separately:
```go
var systemPrompt string
messageParams := []anthropic.MessageParam{}

for _, msg := range messages {
    if msg.Role == "system" {
        systemPrompt += msg.Parts[0].Text + "\n"
    } else {
        messageParams = append(messageParams, fromInternalMessage(msg))
    }
}
```

### 2. Message Alternation

**Gemini:** Allows consecutive messages from same role
**Claude:** Requires strict user/assistant alternation

**Solution:** Merge consecutive messages from same role:
```go
// If current message has same role as previous, merge parts
// into previous message instead of creating new message
```

### 3. Function Calling Format

**Gemini:** Uses `FunctionCall` and `FunctionResponse` parts
**Claude:** Uses `ToolUse` content blocks and `ToolResult` blocks

**Solution:** Translation layer handles conversion:
- Internal `FunctionCall` -> Claude `ToolUse` (in assistant messages)
- Internal `FunctionResponse` -> Claude `ToolResult` (in user messages)

### 4. Image Handling

**Gemini:** Accepts raw bytes with MIME type via `InlineData`
**Claude:** Uses `anthropic.NewImageBlock()` with base64 encoding

**Solution:**
```go
import "encoding/base64"

func imagePartToClaude(img *llm.ImageData) anthropic.MessageParamContentUnion {
    return anthropic.MessageParamContentUnion{
        OfImageBlock: &anthropic.MessageParamContentImageBlock{
            Source: anthropic.MessageParamContentImageBlockSource{
                Type: anthropic.ImageBlockSourceTypeBase64,
                MediaType: anthropic.ImageBlockSourceMediaType(img.MIMEType),
                Data: base64.StdEncoding.EncodeToString(img.Data),
            },
        },
    }
}
```

### 5. Thinking/Reasoning

**Gemini:** Extended thinking via `Thought` field on parts
**Claude:** Extended thinking via `anthropic.BetaExtendedThinking` header and thinking blocks

**Solution:**
- Set `Betas: []anthropic.BetaExtendedThinking` in request params for thinking-enabled models
- Map `ContentBlockTypeThinking` responses to `Part.Thought = true`

## Error Handling

Follow existing error handling patterns from Gemini provider:

1. Wrap all SDK errors: `fmt.Errorf("Claude API error: %w", err)`
2. Return errors without sending error events (let caller handle)
3. Handle context cancellation gracefully
4. Provide meaningful error messages for common issues:
   - Invalid API key: "Authentication failed"
   - Rate limits: Parse and preserve retry-after information
   - Model not found: "Model %s not available"

## Testing Strategy

### Unit Tests

1. **Provider Tests** (`provider_test.go`):
   - `TestProvider_Name()` - Returns "anthropic"
   - `TestProvider_SupportsVision()` - Returns true
   - `TestProvider_SupportsThinking()` - Returns true
   - `TestProvider_ListModels()` - Returns expected models
   - `TestProvider_Close()` - No errors

2. **Translation Tests** (`translation_test.go`):
   - Round-trip message conversion
   - System prompt extraction
   - Message alternation merging
   - Tool definition conversion
   - Image data encoding/decoding
   - Thinking block handling
   - Stop reason mapping

### Integration Tests

**Note:** Skip by default (require real API key):
```go
func TestProvider_GenerateContent(t *testing.T) {
    if os.Getenv("ANTHROPIC_API_KEY") == "" {
        t.Skip("ANTHROPIC_API_KEY not set")
    }
    // Test actual API call
}
```

## Migration & Rollout

### Phase 1: Implementation
1. Add anthropic-sdk-go dependency
2. Implement provider.go with basic message generation
3. Implement translation.go with message/tool conversion
4. Add unit tests

### Phase 2: Integration
1. Update server initialization to add Claude provider to registry
2. Test with simple prompts (no tools)
3. Test with tool calling (scene generation)
4. Test with vision inputs (if applicable)

### Phase 3: Polish
1. Add integration tests
2. Update documentation
3. Add model selection persistence
4. Performance testing and optimization

## Open Questions

1. **Model Updates:** How do we keep the hardcoded model list updated?
   - Option A: Manual updates when new models release
   - Option B: Add a configuration file for model definitions
   - **Recommendation:** Start with manual updates, move to config file if becomes a maintenance burden

2. **Default Model:** Which Claude model should be default?
   - **Recommendation:** Claude Sonnet 4.5 (good balance of capability and cost)

3. **System Prompt Handling:** Should we enforce a single system message or concatenate multiple?
   - **Recommendation:** Concatenate multiple system messages with newlines for flexibility

4. **Thinking Mode:** Should thinking be always enabled or model-specific?
   - **Recommendation:** Enable for models that support it (check `SupportsThinking()`)

## Success Criteria

- [ ] User can select Claude models from dropdown
- [ ] Claude models generate valid responses to text prompts
- [ ] Claude models can call tools (scene manipulation functions)
- [ ] Tool responses are properly formatted and handled
- [ ] Error messages are user-friendly and actionable
- [ ] All unit tests pass
- [ ] Integration tests pass with real API key
- [ ] No regression in Gemini provider functionality

## Future Enhancements

1. **Prompt Caching:** Add support for Claude's prompt caching to reduce costs for repeated system prompts
2. **Streaming:** Implement streaming responses for better UX
3. **Model Comparison:** Add UI to compare outputs from different providers side-by-side
4. **Cost Tracking:** Track API usage and estimated costs per provider
5. **Extended Thinking UI:** Better visualization of thinking/reasoning process
