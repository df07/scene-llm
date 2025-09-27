# Shape System Specification

## Overview

This document specifies the shape system for scene-llm, including available geometry types, their properties, and implementation details for the raytracer integration.

## Available Geometry Types

Based on exploration of the go-progressive-raytracer library, the following shapes are available:

### 1. Sphere ✅ (Implemented)
- **Constructor**: `geometry.NewSphere(center, radius, material)`
- **Properties Required**:
  - `position: [x, y, z]` - Center point
  - `radius: number` - Sphere radius
  - `color: [r, g, b]` - RGB color (0-1 range)
- **Status**: Working correctly

### 2. Box 🔴 (Needs Fix)
- **Constructors**:
  - `geometry.NewAxisAlignedBox(center, size, material)` - Recommended
  - `geometry.NewBox(center, size, rotation, material)` - With rotation
- **Properties Required**:
  - `position: [x, y, z]` - Center point
  - `dimensions: [width, height, depth]` - Full dimensions
  - `color: [r, g, b]` - RGB color
- **Key Implementation Details**:
  - Constructor expects **half-extents**, not full dimensions
  - Must convert: `dimensions/2` before passing to constructor
  - Current bug: renders as sphere instead of box
- **Status**: Currently broken - renders as sphere

### 3. Quad 🆕 (Recommended Addition)
- **Constructor**: `geometry.NewQuad(corner, u, v, material)`
- **Use Cases**: Walls, floors, planes, rectangular surfaces
- **Properties Design**:
  - `position: [x, y, z]` - Corner position
  - `size: [width, height]` - Quad dimensions
  - `normal: [x, y, z]` - Surface normal (optional, default: [0,1,0])
  - `color: [r, g, b]` - RGB color

### 4. Disc 🆕 (Recommended Addition)
- **Constructor**: `geometry.NewDisc(center, normal, radius, material)`
- **Use Cases**: Circular surfaces, coins, table tops, manholes
- **Properties Design**:
  - `position: [x, y, z]` - Center position
  - `radius: number` - Disc radius
  - `normal: [x, y, z]` - Surface normal (optional, default: [0,1,0])
  - `color: [r, g, b]` - RGB color

### 5. Triangle 🆕 (Advanced)
- **Constructor**: `geometry.NewTriangle(v0, v1, v2, material)`
- **Use Cases**: Custom geometry, architectural details
- **Properties Design**:
  - `vertices: [[x,y,z], [x,y,z], [x,y,z]]` - Three vertices
  - `color: [r, g, b]` - RGB color

### 6. TriangleMesh 🆕 (Advanced)
- **Constructor**: `geometry.NewTriangleMesh(vertices, faces, material, options)`
- **Use Cases**: Complex 3D models, imported geometry
- **Properties Design**:
  - `vertices: [[x,y,z], ...]` - Array of vertices
  - `faces: [i1,i2,i3, ...]` - Triangle indices (groups of 3)
  - `color: [r, g, b]` - Default color

## Implementation Priority

### Phase 1: Fix Existing (Immediate)
- ✅ Sphere (working)
- 🔴 **Box (critical bug fix needed)**

### Phase 2: Core Shapes (Recommended)
- 🆕 Quad (very useful for environments)
- 🆕 Disc (useful for details)

### Phase 3: Advanced Shapes (Future)
- 🆕 Triangle
- 🆕 TriangleMesh

## Current Box Bug

**Location**: `/home/dfull/code/scene-llm/agent/scene.go:421-427`

**Current Code** (incorrect):
```go
case "box":
    // Create a simple cube using a sphere for now (since Box constructor seems different)
    shape = geometry.NewSphere(
        core.NewVec3(position[0], position[1], position[2]),
        size/2, // Use half size for radius
        shapeMaterial,
    )
```

**Should Be**:
```go
case "box":
    // Extract dimensions from properties
    var dimensions [3]float64
    if dimsVal, ok := shapeReq.Properties["dimensions"].([]interface{}); ok && len(dimsVal) == 3 {
        for i, dim := range dimsVal {
            if f, ok := dim.(float64); ok {
                dimensions[i] = f / 2.0 // Convert to half-extents
            }
        }
    } else {
        // Fallback to uniform cube using size
        dimensions = [3]float64{size/2, size/2, size/2}
    }

    shape = geometry.NewAxisAlignedBox(
        core.NewVec3(position[0], position[1], position[2]),
        core.NewVec3(dimensions[0], dimensions[1], dimensions[2]),
        shapeMaterial,
    )
```

## Material System

All shapes use Lambertian (diffuse) materials:
```go
material.NewLambertian(core.NewVec3(r, g, b))
```

Where `r`, `g`, `b` are color values in range [0, 1].

## LLM Tool Integration

Current tool definition supports sphere and box:
```go
Description: "Shape-specific properties. For sphere: {position: [x,y,z], radius: number, color: [r,g,b]}. For box: {position: [x,y,z], dimensions: [w,h,d], color: [r,g,b]}"
```

When adding new shapes, update:
1. Tool description in `agent/tools.go`
2. Validation in `agent/scene.go`
3. Rendering logic in `agent/scene.go`

## Property Validation

Each shape type has specific validation requirements:

- **Sphere**: `position[3]`, `radius > 0`, `color[3]`
- **Box**: `position[3]`, `dimensions[3] > 0`, `color[3]`
- **Future shapes**: Define validation for each new type

## Design Principles

1. **Flexible Properties**: Use `map[string]interface{}` for shape-specific properties
2. **Type Safety**: Validate all properties with clear error messages
3. **Consistent Interface**: All shapes follow same pattern (position, size/dimensions, color)
4. **Raytracer Integration**: Direct mapping to go-progressive-raytracer constructors
5. **LLM Friendly**: Clear, simple property names that LLMs can understand and use correctly

## Testing Requirements

For each shape type, ensure:
1. **Validation Tests**: Invalid properties are caught
2. **Rendering Tests**: Shape appears correctly in output
3. **LLM Integration Tests**: Tool calls create expected shapes
4. **Property Update Tests**: Shapes can be modified after creation

## Files to Modify

When adding new shapes:
- `agent/tools.go` - Update tool descriptions
- `agent/scene.go` - Add validation and rendering logic
- `agent/scene_test.go` - Add tests for new shapes
- Update this specification document