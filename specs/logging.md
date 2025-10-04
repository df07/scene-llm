# Logging and Transparency Specification

## User Stories

### 1. User Understanding and Transparency
1. As a user, I should be able to understand what the LLM is doing to the scene and why.
   - 1a. The LLM should explain to me what it is changing and why
   - 1b. I should be able to inspect the exact changes it made, although the details don't all have to be presented on initial display.

### 2. Developer Debugging
2. As a developer, I should be able to debug issues in production.
   - 2a. I should see terse, actionable logs on the server for all tool calls and failures
   - 2b. I should be able to trace the full conversation flow and tool call sequence
   - 2c. I should be able to identify performance bottlenecks and LLM usage patterns

### 3. User Experience Enhancement
3. As a user, I want to see what's happening in real-time without being overwhelmed.
   - 3a. I should see all tool calls and failures with simple summaries: "Updating sphere color"
   - 3b. I should be able to expand to see technical details if I'm curious
   - 3c. The interface should feel responsive and show progress during operations

## Technical Requirements

### Event System Enhancement
- **Current**: Single `ToolCallEvent` with limited `Shapes []ShapeRequest` data
- **Required**: Rich `ToolCallEvent` with operation type, target, before/after states, and results

### Server Logging
- **Format**: Structured logs with consistent format
- **Verbosity**: Terse by default, detailed on demand
- **Context**: Include session ID and timing information

### Client Display
- **Progressive Disclosure**: Simple summaries with expandable details
- **Real-time Updates**: Show tool calls as they happen
- **Error Handling**: Clear error messages and recovery suggestions

## UI Mockups

### Basic Tool Call Display
```
U: Make a blue sphere
A: I'll create a blue sphere for you.
   🔧 Created shape: blue_sphere [+]

U: Change it to red
A: I'll change the blue sphere to red.
   🔧 Updated shape: blue_sphere → red_sphere [+]

U: Remove it
A: I'll remove the red sphere from the scene.
   🔧 Removed shape: red_sphere [+]
```

### Expanded Tool Call Details
```
🔧 Updated shape: blue_sphere → red_sphere [-]

Function: update_shape
Target: blue_sphere
Status: ✓ Success
Duration: 3ms

Changes:
  ├─ id: "blue_sphere" → "red_sphere"
  └─ color: [0.0, 0.0, 1.0] → [1.0, 0.0, 0.0]

Raw Function Call:
{
  "name": "update_shape",
  "arguments": {
    "id": "blue_sphere",
    "properties": {
      "color": [1.0, 0.0, 0.0]
    }
  }
}
```

### Error Handling
```
U: Make it bigger
A: I'll increase the sphere size.
   🔧 Updated shape: red_sphere [+]
   ❌ Error: Shape 'red_sphere' not found

On expansion:
🔧 Updated shape: red_sphere [-]
❌ Error: Shape 'red_sphere' not found
Function: update_shape
Target: red_sphere
Status: ❌ Failed
Error: Shape with ID 'red_sphere' not found

Available shapes: (none)
Suggestion: Try creating a shape first
```

### Multiple Operations
```
U: Add a red cube and a green sphere
A: I'll add both shapes to the scene.
   🔧 Created shape: red_cube [+]
   🔧 Created shape: green_sphere [+]
```

## Server Log Mockups

### Basic Tool Call Logs
```
2025-01-24 15:42:18 INFO  [session:abc123] Tool call: create_shape (blue_sphere)
2025-01-24 15:42:19 INFO  [session:abc123] Tool call: update_shape (blue_sphere)
2025-01-24 15:42:20 INFO  [session:abc123] Tool call: remove_shape (blue_sphere)
```

### Error Logs
```
2025-01-24 15:43:15 INFO  [session:abc123] Tool call: update_shape (red_sphere)
2025-01-24 15:43:16 ERROR [session:abc123] Tool call FAIL: Shape not found (id: red_sphere)
```

## Implementation Details

### Enhanced ToolCallEvent Structure

```go
type ToolCallEvent struct {
    Operation ToolOperation `json:"operation"`      // The tool operation that was attempted
    Success   bool          `json:"success"`        // Operation result
    Error     string        `json:"error,omitempty"` // Error message if failed
    Duration  int64         `json:"duration"`       // Operation duration in ms
    Timestamp time.Time     `json:"timestamp"`      // When the operation occurred
}

// ToolOperation interface - describes what the LLM wanted to do
type ToolOperation interface {
    ToolName() string // "create_shape", "update_shape", "remove_shape"
    Target() string   // Shape ID being operated on (if applicable), empty otherwise
}

// Concrete tool operations - pure data structures describing LLM intentions
type CreateShapeOperation struct {
    Shape ShapeRequest `json:"shape"`
}

func (op CreateShapeOperation) ToolName() string { return "create_shape" }
func (op CreateShapeOperation) Target() string { return op.Shape.ID }

type UpdateShapeOperation struct {
    ID      string                 `json:"id"`
    Updates map[string]interface{} `json:"updates"`
    Before  *ShapeRequest         `json:"before,omitempty"` // Populated by agent after execution
    After   *ShapeRequest         `json:"after,omitempty"`  // Populated by agent after execution
}

func (op UpdateShapeOperation) ToolName() string { return "update_shape" }
func (op UpdateShapeOperation) Target() string { return op.TargetId }

type RemoveShapeOperation struct {
    ID           string        `json:"id"`
    RemovedShape *ShapeRequest `json:"removed_shape,omitempty"` // Populated by agent after execution
}

func (op RemoveShapeOperation) ToolName() string { return "remove_shape" }
func (op RemoveShapeOperation) Target() string { return op.TargetId }
```

### Tool Operation Examples

**create_shape:**
```json
{
  "operation": {
    "shape": {
      "id": "blue_sphere",
      "type": "sphere",
      "properties": {"position": [0,0,0], "radius": 1.0, "color": [0,0,1]}
    }
  },
  "success": true,
  "duration": 2,
  "timestamp": "2025-01-24T15:42:18Z"
}
```

**update_shape:**
```json
{
  "operation": {
    "id": "blue_sphere",
    "updates": {"id": "red_sphere", "properties": {"color": [1,0,0]}},
    "before": {"id": "blue_sphere", "type": "sphere", "properties": {"color": [0,0,1]}},
    "after": {"id": "red_sphere", "type": "sphere", "properties": {"color": [1,0,0]}}
  },
  "success": true,
  "duration": 3,
  "timestamp": "2025-01-24T15:42:19Z"
}
```

**remove_shape:**
```json
{
  "operation": {
    "id": "red_sphere",
    "removed_shape": {
      "id": "red_sphere",
      "type": "sphere",
      "properties": {"position": [0,0,0], "radius": 1.0, "color": [1,0,0]}
    }
  },
  "success": true,
  "duration": 1,
  "timestamp": "2025-01-24T15:42:20Z"
}
```

### Event Flow

1. **Agent Processing**: When agent processes LLM function calls:
   - Parse LLM function calls into `ToolOperation` objects
   - Execute operations via SceneManager (keeping execution separate from data)
   - Populate before/after state in operation objects
   - Emit `ToolCallEvent` wrapping the `ToolOperation` with success/failure

2. **Server Logging**: Web server receives events and:
   - Logs with terse format using `operation.ToolName()` and `operation.Target()`
   - Example: `Tool call: update_shape (blue_sphere)`
   - On failure: Additional ERROR log with details
   - Includes session context and timing

3. **Client Display**: Server forwards events to client via SSE:
   - UI extracts operation data to generate display strings in preferred language
   - Shows simple summary with `[+]` expandable technical details
   - Handles success and error states with appropriate styling

### Integration Points

**New Types (agent/events.go):**
- Add `ToolOperation` interface and concrete operation types
- Update `ToolCallEvent` to wrap `ToolOperation` objects
- Remove old property bag approach

**Agent Changes (agent/agent.go):**
- Create tool operation parsing functions (LLM function calls → `ToolOperation`)
- Execute operations via existing SceneManager methods
- Populate before/after state in operation objects after execution
- Emit unified `ToolCallEvent` for all tool types

**Server Changes (web/server/chat.go):**
- Update event handling to work with `ToolOperation` interface
- Use `operation.ToolName()` and `operation.Target()` for logging
- Forward operation-based events to SSE clients

**Client Changes (web/static/chat.js):**
- Update to handle operation-based event structure
- Create display logic that formats operations for user consumption
- Support multiple languages/display formats as needed
