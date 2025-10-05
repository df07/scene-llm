# LLM Lighting Interface Specification

## Overview

The lighting system will use a **unified light interface** similar to our shape system, where all lights have a consistent structure but different type-specific properties. This aligns with the raytracer's architecture and provides an intuitive interface for the LLM.

## Core Light Structure

Following the established shape pattern, lights use a consistent request structure:

### LightRequest Structure
```go
type LightRequest struct {
    ID         string                 `json:"id"`
    Type       string                 `json:"type"`
    Properties map[string]interface{} `json:"properties"`
}
```

### JSON Format
```json
{
  "id": "unique_light_identifier",
  "type": "light_type",
  "properties": {
    // Type-specific properties
  }
}
```

### Scene State Integration
```go
type SceneState struct {
    Shapes []ShapeRequest `json:"shapes"`
    Lights []LightRequest `json:"lights"`  // Added for light management
    Camera CameraInfo     `json:"camera"`
}
```

## Supported Light Types

### 1. **Point Lights**

#### Point Spot Light
```json
{
  "id": "main_spotlight",
  "type": "point_spot_light",
  "properties": {
    "center": [x, y, z],             // Light position
    "target": [x, y, z],             // Direction target point
    "emission": [r, g, b],           // Light color/intensity (0.0-10.0+)
    "cone_angle": 30.0,              // Total cone angle in degrees
    "falloff_angle": 5.0             // Falloff transition angle in degrees
  }
}
```

### 2. **Area Lights** (Shape-based)

#### Rectangular Area Light
```json
{
  "id": "window_light",
  "type": "area_quad_light",
  "properties": {
    "corner": [x, y, z],             // Corner position
    "u": [x, y, z],                  // First edge vector
    "v": [x, y, z],                  // Second edge vector
    "emission": [r, g, b]            // Light color/intensity
  }
}
```

#### Circular Area Light
```json
{
  "id": "ceiling_light",
  "type": "disc_spot_light",
  "properties": {
    "center": [x, y, z],             // Center position
    "normal": [x, y, z],             // Surface normal direction
    "radius": 2.0,                   // Disc radius
    "emission": [r, g, b]            // Light color/intensity
  }
}
```

#### Spherical Area Light
```json
{
  "id": "bulb_light",
  "type": "area_sphere_light",
  "properties": {
    "center": [x, y, z],             // Center position
    "radius": 0.5,                   // Sphere radius
    "emission": [r, g, b]            // Light color/intensity
  }
}
```

#### Disc Spot Light (Area + Directional)
```json
{
  "id": "stage_light",
  "type": "area_disc_spot_light",
  "properties": {
    "center": [x, y, z],             // Light position
    "target": [x, y, z],             // Direction target point
    "emission": [r, g, b],           // Light color/intensity
    "cone_angle": 45.0,              // Total cone angle in degrees
    "falloff_angle": 10.0,           // Falloff transition angle in degrees
    "radius": 1.0                    // Light source disc radius
  }
}
```

### 3. **Infinite Lights** (Environment)

#### Uniform Environment Light
```json
{
  "id": "ambient_light",
  "type": "infinite_uniform_light",
  "properties": {
    "emission": [r, g, b]            // Uniform light color/intensity
  }
}
```

#### Gradient Sky Light
```json
{
  "id": "sky_light",
  "type": "infinite_gradient_light",
  "properties": {
    "top_color": [r, g, b],          // Sky color (zenith)
    "bottom_color": [r, g, b]        // Horizon color
  }
}
```

## LLM Tool Functions

Following the established shape tool pattern, lights use separate tools for each operation:

### Primary Tools

#### `create_light`
```json
{
  "name": "create_light",
  "description": "Create a new light in the scene with a unique ID",
  "parameters": {
    "id": "string // Unique identifier for the light",
    "type": "point_spot_light|area_quad_light|disc_spot_light|area_sphere_light|area_disc_spot_light|infinite_uniform_light|infinite_gradient_light",
    "properties": "object // Type-specific properties"
  }
}
```

#### `update_light`
```json
{
  "name": "update_light",
  "description": "Update an existing light by ID. Can update the light's ID, type, or any properties like emission, position, angles, etc.",
  "parameters": {
    "id": "string // ID of the light to update",
    "updates": "object // Object containing fields to update"
  }
}
```

#### `remove_light`
```json
{
  "name": "remove_light",
  "description": "Remove a light from the scene by its ID",
  "parameters": {
    "id": "string // ID of the light to remove"
  }
}
```

### Convenience Tool: `set_environment_lighting`
```json
{
  "name": "set_environment_lighting",
  "description": "Set the background/environment lighting for the scene",
  "parameters": {
    "type": "gradient|uniform|none",
    "top_color": "[r,g,b] // For gradient",
    "bottom_color": "[r,g,b] // For gradient",
    "emission": "[r,g,b] // For uniform"
  }
}
```

## Design Principles

### 1. **Semantic Property Names**
- Use `center` for all light positions (consistent with shapes)
- Use `corner` for quad light corners (consistent with quad shapes)
- Use `emission` for all light intensity/color (consistent with raytracer)
- Follow same property naming conventions as shapes

### 2. **Intuitive Parameters**
- **Angles in degrees** (more intuitive than radians for LLM)
- **RGB values 0.0-10.0+** (0-1 for typical colors, >1 for bright lights)
- **Target points** instead of direction vectors (easier for LLM to reason about)

### 3. **Area Light Integration**
- Area lights automatically register as both lights AND shapes
- Use familiar geometric parameters (quad = corner+u+v, disc = center+normal+radius)
- No separate shape creation needed - light creation handles everything

### 4. **Lighting Scenarios**

#### **Natural Indoor Scene**
```
- area_quad_light: Large window providing soft daylight
- area_sphere_light: Table lamp bulbs
- infinite_gradient_light: Outdoor sky visible through windows
```

#### **Studio/Stage Setup**
```
- area_disc_spot_light: Focused stage spotlights
- area_quad_light: Large softbox lighting
- point_spot_light: Accent lighting for details
```

#### **Outdoor Scene**
```
- infinite_gradient_light: Sky with sun/atmosphere
- area_sphere_light: Sun as distant bright sphere
- disc_spot_light: Moon or artificial light sources
```

#### **Architectural Visualization**
```
- area_quad_light: Ceiling panels, window openings
- disc_spot_light: Recessed ceiling lights
- infinite_uniform_light: Even ambient lighting
```

### 5. **LLM Interaction Examples**

**User**: *"Add a warm ceiling light above the table"*
**LLM Tool Call**:
```json
{
  "name": "create_light",
  "parameters": {
    "id": "ceiling_light_01",
    "type": "disc_spot_light",
    "properties": {
      "center": [0, 3, 0],
      "normal": [0, -1, 0],
      "radius": 0.8,
      "emission": [2.5, 2.2, 1.8]  // Warm white
    }
  }
}
```

**User**: *"Make the lighting softer"*
**LLM Tool Call**: Use `update_light` to reduce emission intensities and/or increase area light sizes

**User**: *"Add dramatic spotlighting from the side"*
**LLM Tool Call**:
```json
{
  "name": "create_light",
  "parameters": {
    "id": "dramatic_spot",
    "type": "point_spot_light",
    "properties": {
      "center": [5, 2, 0],
      "target": [0, 0, 0],
      "emission": [8.0, 6.0, 4.0],  // Bright warm light
      "cone_angle": 25.0,
      "falloff_angle": 8.0
    }
  }
}
```

## Implementation Benefits

1. **Familiar Interface**: Mirrors our successful shape system
2. **Complete Coverage**: Supports all raytracer light types
3. **Natural Language Friendly**: Intuitive parameters and naming
4. **Flexible Scenarios**: Handles simple to complex lighting setups
5. **Future Extensible**: Easy to add new light types or properties
6. **Type Safety**: Clear validation rules for each light type

## Technical Implementation Notes

### Raytracer Integration
- Area lights use embedded geometry (quad, disc, sphere) for hit testing
- All lights use `material.NewEmissive(emission)` for light emission
- Area lights register in both `scene.Lights[]` and `scene.Shapes[]` arrays
- Infinite lights require scene bounds preprocessing

### Property Validation
- Position/center/corner: 3-element float arrays
- Directions/normals: 3-element float arrays (will be normalized)
- Emission/colors: 3-element float arrays (can exceed 1.0 for bright lights)
- Radii: positive float values
- Angles: degrees (converted to radians internally)

### State Management
- Lights stored similarly to shapes with ID-based management
- Support add/update/remove operations
- Environment lights replace existing infinite lights
- Scene re-rendering triggered on light changes

This specification provides the LLM with powerful, intuitive lighting control while leveraging all the raytracer's advanced lighting capabilities, particularly the sophisticated area lighting system.