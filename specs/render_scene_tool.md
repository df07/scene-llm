# Render Scene Tool Specification

## Overview
Add a `render_scene` tool that allows the LLM to render the current scene at high quality and receive the rendered image for visual inspection. This enables the LLM to verify visual appearance, check lighting/materials, and debug rendering issues.

## Motivation
Currently, the LLM creates scenes "blind" - it can manipulate shapes, materials, and lighting but cannot see the visual results. This tool gives the LLM vision to:
- Verify that materials (metal, glass, diffuse) look correct
- Check lighting intensity and color accuracy
- Debug visual issues that aren't obvious from scene state alone
- Make aesthetic judgments about composition and appearance

## Design Decisions

### Trigger Mode: Explicit Only
The LLM must explicitly call `render_scene` - it does NOT trigger automatically after scene modifications.

**Rationale**: High quality rendering is expensive (500 samples/pixel, 3-10 seconds). We want the LLM to use this sparingly and deliberately.

### Quality: High Quality Only
Always renders at 500 samples per pixel (high quality mode).

**Rationale**: The LLM needs accurate visual information to make good decisions. Low quality previews might show incorrect lighting/materials.

### Image Size: Standard (400x300)
Render at the same resolution as the user-facing preview (400x300 pixels).

**Rationale**: Using the same resolution ensures the LLM sees exactly what the user sees. This avoids any rendering artifacts or differences that might occur at different resolutions.

### Execution: Synchronous
The tool blocks until rendering completes.

**Rationale**:
- LLM needs the image result to make decisions
- Simpler implementation than async
- Progress is shown to user via ToolCallEvent

### Image Format: Gemini InlineData
Use Gemini's native `InlineData` format with PNG blob.

**Rationale**: Most efficient format for Gemini API. The image is embedded directly in the conversation context.

## Implementation

### Tool Declaration
```go
Name: "render_scene"
Description: "Render the current scene at high quality and return the image for visual inspection. **WARNING: This is computationally expensive (500 samples/pixel, takes several seconds). Use sparingly - only when you need to verify visual appearance, check lighting/materials, or debug rendering issues."
Parameters: None
```

### Request Type
```go
type RenderSceneRequest struct {
    BaseToolRequest
    RenderedImage []byte `json:"rendered_image,omitempty"` // Populated after execution
}
```

### Execution Flow
1. Emit `ToolCallEvent` immediately (shows "Rendering scene..." in UI)
2. Convert scene state to raytracer scene
3. Validate scene is not empty
4. Create renderer with config:
   - Width: 100px
   - Height: 75px (4:3 aspect ratio)
   - Samples: 500 per pixel
5. Render synchronously (takes 3-10 seconds)
6. Encode as PNG
7. Store in `RenderSceneRequest.RenderedImage`
8. Return success with metadata

### LLM Response Format
The function response includes TWO parts in the "function" role message:
1. **FunctionResponse Part**: Structured JSON with metadata
   ```json
   {
     "success": true,
     "result": {
       "shape_count": 5,
       "samples_per_pixel": 500,
       "width": 100,
       "height": 75,
       "render_time_ms": 4523
     }
   }
   ```
2. **InlineData Part**: The PNG image
   ```go
   &genai.Part{
     InlineData: &genai.Blob{
       Data: renderedImage,
       MIMEType: "image/png",
       DisplayName: "rendered_scene.png"
     }
   }
   ```

### Frontend Display
The `ToolCallEvent` sent to the frontend includes the rendered image:
```go
type ToolCallEvent struct {
    Request       ToolRequest
    Success       bool
    Error         string
    Duration      int64
    Timestamp     time.Time
    RenderedImage []byte  // NEW: Image data for render_scene tool
}
```

The frontend can:
- Show "Rendering scene..." immediately when tool is called
- Update with completion status and duration
- Display the image in the details expando (shows what was sent to LLM)

## Token Usage Considerations
- 100x75 PNG image ≈ 5-15KB compressed
- Gemini vision tokens ≈ 258 tokens per image (fixed cost for small images)
- This is acceptable for occasional use
- Tool description warns LLM to use sparingly

## Error Handling
- **Empty scene**: Return error "Cannot render empty scene - add shapes first"
- **Render failure**: Return error with render error message
- **Encoding failure**: Return error "Failed to encode rendered image"

## Testing
- Test successful render with image data
- Test empty scene error
- Test that image is included in both ToolCallEvent and FunctionResponse
- Test that rendering is synchronous (blocks until complete)

## Future Enhancements (Not in Initial Implementation)
- Allow LLM to specify resolution (small/medium/large)
- Allow LLM to specify quality (draft/high)
- Support multiple viewpoints in one render
- Streaming/progressive rendering feedback
