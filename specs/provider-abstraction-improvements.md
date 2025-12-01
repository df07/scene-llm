# Provider Abstraction Improvements - Refined Analysis

## Executive Summary

After deep analysis of the current implementation and three major LLM providers (Gemini, Claude, OpenAI), I recommend **three critical changes** to our abstraction before adding Claude support:

1. **Separate system prompt from conversation** - Prevents awkward injection into user messages
2. **Add tool call IDs** - Required by Claude and OpenAI, enables proper tracking
3. **Use type-safe constants** - Improves code quality and prevents bugs

**Estimated effort:** 6-8 hours of refactoring before Claude implementation

## Current Implementation Analysis

### How We Use the Abstraction Today

**System Prompt Injection** (`agent.go:396-434`):
```go
// CURRENT APPROACH: Inject system prompt into FIRST user message
systemPrompt := `You are an autonomous 3D scene creation assistant...`
contextualText := fmt.Sprintf(systemPrompt, sceneContext, originalText)
lastMessage.Parts[0] = llm.Part{Type: llm.PartTypeText, Text: contextualText}
```

**Problems:**
- System prompt appears in conversation history
- Gets duplicated on every turn if we're not careful
- Doesn't match Claude's API model (system is separate parameter)
- Makes conversation history harder to reason about

**Provider Call** (`agent.go:79`):
```go
response, err := a.provider.GenerateContent(ctx, a.modelID, messages, tools)
```

**Tool Execution** (`agent.go:196`):
```go
// We generate our own IDs
toolCallID := fmt.Sprintf("%s_%d", operation.ToolName(), startTime.UnixNano())
```

**Problem:** These IDs aren't connected to the LLM's tool call IDs

## Critical Issues Discovered

### 1. System Messages Handling ⚠️ CRITICAL

**Current Problem:**
- We inject system prompt into the first user message's text
- This is a hack that works for Gemini but won't work for Claude
- System prompt gets embedded in conversation history
- Can't easily update system context without duplicating it

**How Each Provider Handles System Prompts:**

| Provider | System Prompt Handling |
|----------|----------------------|
| **Gemini** | Accepts system role messages in conversation |
| **Claude** | Separate `system` parameter (NOT in messages array) |
| **OpenAI** | Accepts system role messages in conversation |

**Proposed Solution:**

```go
type GenerateRequest struct {
    Model        string
    SystemPrompt string    // Separate from conversation
    Messages     []Message // No system messages here
    Tools        []Tool
}

GenerateContent(ctx context.Context, req *GenerateRequest) (*Response, error)
```

**Rationale:**
- Extensible for future parameters (temperature, max_tokens, top_p, etc.)
- Clear separation of system vs conversation
- Matches pattern used by major SDKs
- Prevents bloated method signatures

**Implementation:**
1. Agent builds system prompt with scene context
2. Agent passes system prompt in `GenerateRequest.SystemPrompt`
3. **Gemini provider**: Converts to first message with role="model" or prepends system message
4. **Claude provider**: Uses `System` parameter directly
5. **OpenAI provider**: Converts to first message with role="system"

### 2. Tool Call ID Tracking ⚠️ CRITICAL

**Current Problem:**
- Claude and OpenAI **require** tool call IDs to match results
- Our `FunctionCall` and `FunctionResponse` have no ID field
- Agent generates its own IDs for events, not connected to LLM IDs
- Can't properly track which response goes with which call

**How Each Provider Handles Tool IDs:**

| Provider | Tool Call IDs |
|----------|--------------|
| **Gemini** | Optional (not enforced) |
| **Claude** | **Required** - must match `tool_use.id` with `tool_result.tool_use_id` |
| **OpenAI** | **Required** - must match `tool_calls[].id` with `tool_call_id` |

**Proposed Solution:**

```go
type FunctionCall struct {
    ID        string                 // LLM-generated ID (e.g., "toolu_01A09...")
    Name      string
    Arguments map[string]interface{}
}

type FunctionResponse struct {
    ID       string                 // Matches FunctionCall.ID
    Name     string                 // Function name (for clarity/debugging)
    Response map[string]interface{}
}
```

**Benefits:**
- ✅ Claude provider can use IDs directly (required)
- ✅ OpenAI provider can use IDs directly (required)
- ✅ Better debugging - can trace call → execution → response
- ✅ Proper event correlation in UI
- ✅ Future-proof for other providers

**Migration:**
- Gemini provider: Generate IDs if not provided by SDK (fallback to timestamp)
- Agent: Use IDs from response instead of generating timestamp-based IDs
- Events: Include tool call ID for better UI tracking

### 3. Type-Safe Role Constants

**Current Problem:**
- Roles are magic strings: `"user"`, `"assistant"`, `"function"`
- Easy to typo ("asistant" vs "assistant")
- No autocomplete or compile-time checking
- `"function"` role is Gemini-specific, doesn't map to Claude/OpenAI

**Proposed Solution:**

```go
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type Message struct {
    Role  Role
    Parts []Part
}
```

**Note:** Remove `"function"` role - not used by agent, only exists for providers internally

**Benefits:**
- ✅ Type safety with autocomplete
- ✅ Compile-time checking
- ✅ Self-documenting code
- ✅ Consistent across providers

## Recommended Changes - Final List

### ✅ MUST HAVE (Before Claude)

1. **System Prompt Separation** - Use `GenerateRequest` struct with `SystemPrompt` field
2. **Tool Call IDs** - Add `ID` field to `FunctionCall` and `FunctionResponse`
3. **Role Constants** - Replace string roles with `type Role string` + constants

### 🤔 SHOULD HAVE (Can defer to later)

4. **StopReason Constants** - Type-safe stop reasons (currently just strings)
5. **Request Configuration** - Add temperature, max_tokens, etc. to GenerateRequest

### ❌ NOT NEEDED NOW

- Message alternation helpers (let providers handle their own quirks)
- Streaming support (not needed for current use case)
- Prompt caching configuration (provider-specific optimization)

## Final Proposed API

```go
package llm

import "context"

// === TYPES ===

type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type Message struct {
    Role  Role   // Type-safe role
    Parts []Part
}

type FunctionCall struct {
    ID        string                 // ⭐ NEW: LLM-generated ID
    Name      string
    Arguments map[string]interface{}
}

type FunctionResponse struct {
    ID       string                 // ⭐ NEW: Matches FunctionCall.ID
    Name     string
    Response map[string]interface{}
}

// === REQUEST ===

type GenerateRequest struct {
    Model        string    // Model ID (e.g., "claude-sonnet-4.5-20250929")
    SystemPrompt string    // ⭐ NEW: Separate system prompt
    Messages     []Message // Conversation history (no system messages)
    Tools        []Tool    // Available tools

    // Optional configuration (can add later)
    // MaxTokens   *int
    // Temperature *float64
}

// === PROVIDER INTERFACE ===

type LLMProvider interface {
    GenerateContent(ctx context.Context, req *GenerateRequest) (*Response, error) // ⭐ CHANGED signature
    ListModels() []ModelInfo
    Name() string
    SupportsVision() bool
    SupportsThinking() bool
}
```

## Implementation Plan

### Step 1: Update Type Definitions (1 hour)

**File:** `agent/llm/provider.go`

```go
// Add Role type
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

// Add GenerateRequest
type GenerateRequest struct {
    Model        string
    SystemPrompt string
    Messages     []Message
    Tools        []Tool
}

// Update Message
type Message struct {
    Role  Role   // Changed from string
    Parts []Part
}

// Update FunctionCall
type FunctionCall struct {
    ID        string  // NEW
    Name      string
    Arguments map[string]interface{}
}

// Update FunctionResponse
type FunctionResponse struct {
    ID       string  // NEW
    Name     string
    Response map[string]interface{}
}

// Update interface
type LLMProvider interface {
    GenerateContent(ctx context.Context, req *GenerateRequest) (*Response, error)  // Changed
    // ... other methods unchanged
}
```

### Step 2: Update Gemini Provider (2 hours)

**File:** `agent/llm/gemini/provider.go`

```go
func (p *Provider) GenerateContent(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
    // Convert system prompt to message if provided
    messages := req.Messages
    if req.SystemPrompt != "" {
        systemMsg := llm.Message{
            Role: llm.RoleUser,  // Gemini doesn't have system role, use user
            Parts: []llm.Part{
                {Type: llm.PartTypeText, Text: req.SystemPrompt},
            },
        }
        messages = append([]llm.Message{systemMsg}, messages...)
    }

    // Convert to genai format
    genaiMessages := FromInternalMessages(messages)

    // ... rest of implementation
}
```

**File:** `agent/llm/gemini/translation.go`

```go
// Update to handle Role type
func FromInternalMessage(msg llm.Message) *genai.Content {
    role := string(msg.Role)  // Convert Role to string
    if role == string(llm.RoleAssistant) {
        role = "model"  // Gemini uses "model" instead of "assistant"
    }
    // ... rest of translation
}

// Update to generate IDs if missing
func ToInternalPart(part *genai.Part) llm.Part {
    if part.FunctionCall != nil {
        // Generate ID if Gemini doesn't provide one
        id := part.FunctionCall.ID
        if id == "" {
            id = fmt.Sprintf("call_%d", time.Now().UnixNano())
        }

        return llm.Part{
            Type: llm.PartTypeFunctionCall,
            FunctionCall: &llm.FunctionCall{
                ID:        id,  // Include ID
                Name:      part.FunctionCall.Name,
                Arguments: part.FunctionCall.Args,
            },
        }
    }
    // ... rest
}
```

### Step 3: Update Agent (2 hours)

**File:** `agent/agent.go`

```go
func (a *Agent) ProcessMessage(ctx context.Context, conversation []llm.Message) ([]llm.Message, error) {
    // Build system prompt (extracted from addSceneContext logic)
    sceneContext := a.sceneManager.BuildContext()
    systemPrompt := buildSystemPrompt(sceneContext)

    // Get tools
    tools := getAllTools()

    // Build request
    req := &llm.GenerateRequest{
        Model:        a.modelID,
        SystemPrompt: systemPrompt,  // Separate!
        Messages:     conversation,  // No system prompt injection
        Tools:        tools,
    }

    // Call provider
    response, err := a.provider.GenerateContent(ctx, req)
    // ... rest
}

func buildSystemPrompt(sceneContext string) string {
    return fmt.Sprintf(`You are an autonomous 3D scene creation assistant...

CURRENT SCENE:
%s`, sceneContext)
}

// Remove addSceneContext() - no longer needed!
```

**Update tool execution to use IDs:**

```go
func (a *Agent) executeToolRequests(operation ToolRequest, callID string) ToolResult {
    // Use the callID from the function call
    // ... execute tool ...

    // Return response with ID
    return ToolResult{
        ID:      callID,  // Pass through the ID
        Success: true,
        Result:  result,
    }
}

// When building function response message:
for _, call := range functionCalls {
    result := a.executeToolRequests(req, call.ID)

    parts = append(parts, llm.Part{
        Type: llm.PartTypeFunctionResponse,
        FunctionResp: &llm.FunctionResponse{
            ID:       call.ID,  // Match the ID!
            Name:     call.Name,
            Response: result.Result,
        },
    })
}
```

### Step 4: Update Tests (2 hours)

**File:** `agent/agent_test.go`

```go
// Update MockProvider
func (m *MockProvider) GenerateContent(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
    // Update signature
    // ... mock logic
}

// Update test messages to use Role constants
msg := llm.Message{
    Role: llm.RoleUser,  // Not "user"
    Parts: []llm.Part{...},
}

// Update FunctionCalls to include IDs
call := &llm.FunctionCall{
    ID:        "call_001",
    Name:      "create_shape",
    Arguments: map[string]interface{}{...},
}
```

**File:** `agent/llm/gemini/translation_test.go`

Update all tests to use new types and check ID handling

### Step 5: Update Mock Provider (30 min)

**File:** `agent/llm/registry_test.go`

```go
func (m *MockProvider) GenerateContent(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
    return &llm.Response{
        Parts: []llm.Part{{
            Type: llm.PartTypeText,
            Text: fmt.Sprintf("System: %s\nMessage: %s", req.SystemPrompt, req.Messages[0].Parts[0].Text),
        }},
        StopReason: "stop",
    }, nil
}
```

## Testing Strategy

1. **Run existing tests** - Should fail initially (expected)
2. **Update test mocks** - Fix MockProvider signature
3. **Update test data** - Use Role constants, add IDs
4. **Run tests again** - Should pass
5. **Manual testing** - Test with Gemini in running app
6. **Verify** - System prompt works, tool calls have IDs

## Rollout Plan

### Phase 1: Refactoring (Day 1)
- [ ] Update type definitions
- [ ] Update Gemini provider
- [ ] Update Agent
- [ ] Fix all compilation errors
- [ ] Update tests

### Phase 2: Testing (Day 1-2)
- [ ] All unit tests pass
- [ ] Manual testing with Gemini
- [ ] Verify system prompt separation works
- [ ] Verify tool call IDs are preserved
- [ ] Check UI still works correctly

### Phase 3: Claude Implementation (Day 2-3)
- [ ] Implement Claude provider using new API
- [ ] Much simpler than it would have been!
- [ ] Add to registry
- [ ] Test with both providers

## Benefits Summary

**Before Refactoring:**
- ❌ System prompt injected into user messages (hacky)
- ❌ No tool call ID tracking
- ❌ Magic string roles
- ❌ Claude implementation would require workarounds

**After Refactoring:**
- ✅ Clean system prompt separation
- ✅ Proper tool call ID tracking
- ✅ Type-safe roles
- ✅ Claude implementation will be straightforward
- ✅ Future providers will be easier to add
- ✅ Better debugging and logging
- ✅ More maintainable codebase

## Risk Mitigation

**Risk:** Breaking existing Gemini functionality
**Mitigation:**
- Comprehensive test coverage
- Manual testing before committing
- Can revert if issues found

**Risk:** Time estimate too low
**Mitigation:**
- Well-scoped changes
- Step-by-step plan
- Can pause if needed

**Risk:** Unforeseen compatibility issues
**Mitigation:**
- Gemini provider abstracts away differences
- Test suite catches regressions
- Incremental rollout

## Conclusion

These refactorings are **essential** before adding Claude support. The current abstraction works for Gemini because we're hacking around its limitations, but Claude has different requirements that expose the weaknesses in our design.

The refactoring is well-scoped, low-risk, and will pay dividends immediately when implementing Claude and any future providers.

**Recommendation: Proceed with refactoring before Claude implementation.**
