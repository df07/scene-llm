# Agentic Loop Specification

## Overview

This document specifies the agentic loop architecture for scene-llm, enabling the LLM to autonomously think, plan, execute tools, validate results, and recover from errors until it completes the user's request.

## System Prompt

The LLM receives the following system instruction to guide its behavior in the agentic loop:

```
You are an autonomous 3D scene creation assistant. Your job is to help users create and modify 3D scenes using raytracing.

AVAILABLE TOOLS:
You have access to tools for creating, updating, and removing shapes and lights. Each tool call will return a JSON result showing you what happened.

WORKFLOW:
1. Explain to the user what you're doing as you work
2. Call tools to create/modify the scene
3. Review tool results - if there are errors, retry with corrections
4. Iterate until the scene matches the user's request
5. When satisfied, provide a final response (text only, no tool calls) to signal completion

TOOL RESULTS:
- Success: {"success": true, "result": {<full object>}}
- Error: {"success": false, "error": "<error message>"}

The results show the complete state of each object, including any defaults that were applied. Use these to track what's in the scene and validate your work.

CURRENT SCENE:
%s

USER REQUEST:
%s
```

**Implementation Note**: This instruction is prepended to the first user message. The `%s` placeholders are replaced with:
1. Current scene state (from `SceneManager.BuildContext()`)
2. User's actual request text

## Current Architecture (Call-and-Response)

Currently, scene-llm operates in a single-turn mode:
1. User sends message
2. LLM responds with tool calls
3. System executes all tool calls
4. Done (no feedback loop)

**Limitations:**
- No error recovery (LLM never sees tool execution results)
- No iterative refinement (can't validate and adjust)
- No autonomous completion (LLM can't determine when it's done)

## New Architecture (Agentic Loop)

### Flow

```
User sends message
    ↓
┌─→ LLM processes (with full conversation history)
│   ↓
│   LLM responds (text and/or tool calls)
│   ↓
│   Execute tool calls → Append results to history
│   ↓
│   Has tool calls? ──Yes→ Turn count < limit? ──Yes─┐
│   │                                               │
│   No                                              No
│   ↓                                               ↓
└─  Return final response                    Notify user of limit
                                             Wait for next message
```

### Turn Limit

- **Maximum turns per user message**: 10 (configurable)
- **Turn definition**: Each LLM call (after initial user message) counts as one turn
- **When limit reached**:
  - Complete current tool execution safely
  - Stop the loop
  - Send message to user: "Reached maximum turn limit (N turns). Send a message to continue."
  - Preserve tool results in conversation history
  - Wait for next user message (any message resumes the loop)

### Termination Conditions

The agentic loop terminates when:
1. **LLM sends response with no tool calls** (with or without text) → Success
2. **Turn limit reached** → Pause, wait for user input
3. **User interrupts** → Pause, wait for user input

### Conversation History

- **Format**: Append-only list of messages
- **Contents**: User messages, assistant messages, tool calls, tool results
- **Persistence**: Full history maintained (no compression/summarization for now)
- **Structure** (following Gemini API format):
  ```
  [
    {role: "user", parts: [{text: "Create a red sphere"}]},
    {role: "model", parts: [{functionCall: {...}}, {text: "..."}]},
    {role: "function", parts: [{functionResponse: {...}}]},
    {role: "model", parts: [{text: "Created the sphere. Anything else?"}]}
  ]
  ```

### Tool Call Results

Tool results are appended to conversation history after each execution.

**Success format:**
```json
{
  "name": "create_shape",
  "response": {
    "success": true,
    "result": {
      "id": "red_sphere",
      "type": "sphere",
      "properties": {
        "center": [0, 1, 0],
        "radius": 1.0,
        "material": {
          "type": "lambertian",
          "albedo": [0.8, 0.1, 0.1]
        }
      }
    }
  }
}
```

**Error format:**
```json
{
  "name": "create_shape",
  "response": {
    "success": false,
    "error": "shape 'red_sphere' requires 'center' property"
  }
}
```

**Rationale:**
- JSON is compact and precise (better than human-readable descriptions)
- Full object includes defaults applied by system
- LLM can easily parse and understand what happened
- Error messages enable autonomous recovery

### Error Recovery

- Errors are returned as tool results (see above)
- No automatic retry logic (let LLM decide what to do)
- LLM sees error in history and can:
  - Retry with corrected parameters
  - Try a different approach
  - Ask user for clarification
  - Give up and explain the problem

### User Interruption

**Behavior:**
- User can interrupt at any time via WebSocket cancel message
- Current tool execution completes safely (no partial state)
- Tool results are preserved in conversation history
- Loop stops and waits for next user message

**Implementation:**
- WebSocket sends cancel signal
- Server sets cancellation flag
- After current tool execution completes, check flag
- If cancelled: stop loop, don't call LLM again, wait for user

**Resume:**
- User sends any message
- Loop continues from where it left off (with full history)

## Implementation Phases

### Phase 1: Core Loop ⏳
1. Modify LLM call handler to loop while tool calls present
2. Implement turn counter and limit check
3. Add tool result formatting (JSON success/error)
4. Append tool results to conversation history
5. Update conversation history structure
6. Add termination logic (no tool calls = done)
7. Test with simple multi-turn scenarios

### Phase 2: Interruption ⏳
1. Add cancellation flag to conversation context
2. Check cancellation between iterations
3. Handle WebSocket cancel messages
4. Preserve state on interruption
5. Test interrupt/resume flows

### Phase 3: Turn Limit Handling ⏳
1. Implement turn limit check
2. Add user notification on limit reached
3. Test limit scenarios

### Phase 4: Error Recovery ⏳
1. Format error responses as tool results
2. Test LLM error recovery behavior
3. Refine error messages for clarity

## Future Enhancements

### Visual Feedback (Future)
- Add explicit `render_preview` tool call
- LLM can request to see current render
- Multimodal model can analyze image and adjust scene
- Only called when needed (expensive operation)

### Context Compression (Future)
- When history gets very long, summarize old interactions
- Keep recent history + summary of past
- Preserve critical information (current scene state, user preferences)

### Streaming Improvements (Future)
- Stream LLM text responses to user in real-time
- Show "processing..." indicator during tool execution
- Progressive rendering during multi-turn loops

## Migration Notes

**Backward Compatibility:**
- Existing single-turn behavior is a special case (LLM returns no tool calls on first response)
- Current frontend code needs minimal changes (already handles streaming responses)
- Tool execution code needs refactoring to return structured results

**Breaking Changes:**
- Tool call result format changes (need to wrap in success/error structure)
- Conversation history structure changes (need migration for any persisted conversations)

## Open Questions

1. Should we persist conversation history to disk/database for recovery after crashes?
2. Should there be different turn limits for different user tiers (free vs paid)?
3. How do we handle very long conversations (thousands of messages)?
4. Should we add telemetry/logging for turn counts and error rates?
