# Future Enhancements

Quick reference for next features to implement.

## User Interface
- Stop button to interrupt LLM processing
- Save/load chat history
- Save/load scene models (JSON export/import)

## Scene Features
- Rotation tools for objects
- Translation/scaling operations
- Instance groups (create reusable object collections)
- Copy/clone groups with transformations

## Performance
- LLM context caching (Gemini caches, Claude prompt caching)
- Optimize long conversation histories

## Deployment
- Public hosting with rate limiting
- Quota management and usage tracking
- Display quota to users



LLM Feature Requests:
> Given the current primitives, creating a natural-looking curve like a smile is indeed challenging. If I could open a feature request, I would ask for a new shape primitive: arc.
> 
> This arc primitive would allow me to define a curved segment in 3D space, which would be perfect for a smile. Here's how I envision its properties:
> 
> * **type: 'arc'**
> * **properties: { center: [x, y, z], normal: [x, y, z], radius: number, start_angle: degrees, end_angle: degrees, thickness: number, material?: {...} }**
> 
> With this arc primitive, I could easily create a smile by specifying the center of the arc, a normal to define its plane (e.g., facing the camera), a radius for the curvature, start_angle and end_angle to define the extent of the smile, and a thickness to give it volume. This would allow for much more expressive and accurate curved shapes than trying to approximate them with boxes or cylinders.
