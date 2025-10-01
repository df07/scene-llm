# Materials System Specification

## Overview

This document specifies the material system for scene-llm, including how to create materials in the raytracer and the implementation plan for integrating materials into shape creation.

## Raytracer Material Constructors

Shapes receive materials at construction time. All shape constructors take a `material.Material` parameter:

- `geometry.NewSphere(center, radius, material)`
- `geometry.NewQuad(corner, u, v, material)`
- etc.

## Available Material Types

### 1. Lambertian (Diffuse) ✅

**Constructor**: `material.NewLambertian(albedo core.Vec3)`

**Properties**:
- `albedo: [r, g, b]` - Base color/reflectance (0-1 range)

**Use Cases**: Matte surfaces, clay, concrete, painted walls, paper

### 2. Metal (Specular) ✅

**Constructor**: `material.NewMetal(albedo core.Vec3, fuzz float64)`

**Properties**:
- `albedo: [r, g, b]` - Metal color (0-1 range)
- `fuzz: number` - Reflection fuzziness (0.0 = perfect mirror, 1.0 = very fuzzy)

**Use Cases**: Mirrors, polished metals, chrome, brushed steel

### 3. Dielectric (Glass/Transparent) 🔮 (Future)

**Constructor**: `material.NewDielectric(refractiveIndex float64)`

### 4. Emissive (Light Source) 💡 (Handled via lighting system)

## Material Integration with Shapes

Materials are specified inline as part of shape properties, following the same pattern as shapes and lights.

### Shape Schema with Material

```json
{
  "shapes": [
    {
      "id": "matte_sphere",
      "type": "sphere",
      "properties": {
        "position": [0, 1, 0],
        "radius": 1.0,
        "material": {
          "type": "lambertian",
          "albedo": [0.8, 0.1, 0.1]
        }
      }
    },
    {
      "id": "metal_sphere",
      "type": "sphere",
      "properties": {
        "position": [2, 1, 0],
        "radius": 1.0,
        "material": {
          "type": "metal",
          "albedo": [0.9, 0.9, 0.9],
          "fuzz": 0.1
        }
      }
    }
  ]
}
```

### create_shape Extension

The `create_shape` tool is extended to support material specification:

**Parameters**:
- `id: string` - Unique identifier for the shape
- `type: "sphere" | "quad" | "disc"` - Shape type
- `properties: object` - Shape-specific properties including optional material

**Example - Lambertian material**:
```javascript
create_shape({
  id: "red_sphere",
  type: "sphere",
  properties: {
    position: [0, 1, 0],
    radius: 1.0,
    material: {
      type: "lambertian",
      albedo: [0.8, 0.1, 0.1]
    }
  }
})
```

**Example - Metal material**:
```javascript
create_shape({
  id: "mirror_ball",
  type: "sphere",
  properties: {
    position: [2, 1, 0],
    radius: 1.0,
    material: {
      type: "metal",
      albedo: [0.95, 0.95, 0.95],
      fuzz: 0.0
    }
  }
})
```

**Example - Default behavior (no material specified)**:
```javascript
create_shape({
  id: "simple_sphere",
  type: "sphere",
  properties: {
    position: [0, 1, 0],
    radius: 1.0,
    color: [0.8, 0.1, 0.1]  // Creates default gray Lambertian if no material
  }
})
```

### update_shape Extension

Materials can be updated via `update_shape`:

**Example - Change material type**:
```javascript
update_shape({
  id: "red_sphere",
  updates: {
    material: {
      type: "metal",
      albedo: [0.9, 0.1, 0.1],
      fuzz: 0.2
    }
  }
})
```

## Implementation Plan

### Phase 1: Core Material Support (Current)

1. ✅ Research raytracer material API
2. ✅ Design material schema (inline with shapes)
3. ⏳ Extend `create_shape` tool to accept material parameter
4. ⏳ Extend `update_shape` tool to support material updates
5. ⏳ Add material property extraction and validation
6. ⏳ Update raytracer integration to create Lambertian and Metal materials
7. ⏳ Add default material behavior (gray Lambertian if not specified)
8. ⏳ Add comprehensive tests

### Phase 2: Advanced Materials (Future)

1. Dielectric (glass/transparent) material support
2. Texture support (if raytracer adds it)

## Validation Rules

### Lambertian Material
- `type` must be "lambertian"
- `albedo` must be [r, g, b] array with values in [0, 1] range

### Metal Material
- `type` must be "metal"
- `albedo` must be [r, g, b] array with values in [0, 1] range
- `fuzz` must be number in [0.0, 1.0] range

### Default Material
- If no material is specified in shape properties, create gray Lambertian: `{type: "lambertian", albedo: [0.5, 0.5, 0.5]}`
