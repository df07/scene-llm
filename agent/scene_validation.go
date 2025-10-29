package agent

import (
	"fmt"
	"strings"
)

// ValidationErrors is a custom error type that holds multiple validation errors
type ValidationErrors []string

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	if len(ve) == 1 {
		return ve[0]
	}
	return fmt.Sprintf("%d validation errors: %s", len(ve), strings.Join(ve, "; "))
}

// validateShapeProperties validates that a shape has the required properties for its type
func validateShapeProperties(shape ShapeRequest) error {
	var errors ValidationErrors
	zero := 0.0
	one := 1.0

	validateStringRequired(&errors, shape.ID, "shape ID")
	validateStringRequired(&errors, shape.Type, "shape type")

	if shape.Properties == nil {
		errors = append(errors, "shape properties cannot be nil")
		return errors // Can't validate further without properties
	}

	switch shape.Type {
	case "sphere":
		validateVec3PropertyRequired(&errors, shape.Properties, "center", nil, nil, "sphere", shape.ID)
		validatePositiveFloatRequired(&errors, shape.Properties, "radius", "sphere", shape.ID)

	case "box":
		validateVec3PropertyRequired(&errors, shape.Properties, "center", nil, nil, "box", shape.ID)
		validateVec3PropertyRequired(&errors, shape.Properties, "dimensions", &zero, nil, "box", shape.ID)

	case "quad":
		validateVec3PropertyRequired(&errors, shape.Properties, "corner", nil, nil, "quad", shape.ID)
		validateVec3PropertyRequired(&errors, shape.Properties, "u", nil, nil, "quad", shape.ID)
		validateVec3PropertyRequired(&errors, shape.Properties, "v", nil, nil, "quad", shape.ID)

	case "disc":
		validateVec3PropertyRequired(&errors, shape.Properties, "center", nil, nil, "disc", shape.ID)
		validateVec3PropertyRequired(&errors, shape.Properties, "normal", nil, nil, "disc", shape.ID)
		validatePositiveFloatRequired(&errors, shape.Properties, "radius", "disc", shape.ID)

	case "cylinder":
		validateVec3PropertyRequired(&errors, shape.Properties, "base_center", nil, nil, "cylinder", shape.ID)
		validateVec3PropertyRequired(&errors, shape.Properties, "top_center", nil, nil, "cylinder", shape.ID)
		validatePositiveFloatRequired(&errors, shape.Properties, "radius", "cylinder", shape.ID)
		validateBoolPropertyRequired(&errors, shape.Properties, "capped", "cylinder", shape.ID)

	case "cone":
		validateVec3PropertyRequired(&errors, shape.Properties, "base_center", nil, nil, "cone", shape.ID)
		validateVec3PropertyRequired(&errors, shape.Properties, "top_center", nil, nil, "cone", shape.ID)
		validatePositiveFloatRequired(&errors, shape.Properties, "base_radius", "cone", shape.ID)
		validateNonNegativeFloatRequired(&errors, shape.Properties, "top_radius", "cone", shape.ID)
		validateBoolPropertyRequired(&errors, shape.Properties, "capped", "cone", shape.ID)

		// Validate that base_radius > top_radius (cone constraint)
		if baseRadius, ok := extractFloat(shape.Properties, "base_radius"); ok {
			if topRadius, ok := extractFloat(shape.Properties, "top_radius"); ok {
				if baseRadius <= topRadius {
					errors = append(errors, fmt.Sprintf("cone '%s' base_radius (%.2f) must be greater than top_radius (%.2f)", shape.ID, baseRadius, topRadius))
				}
			}
		}

	case "":
		// Already handled above
	default:
		errors = append(errors, fmt.Sprintf("unsupported shape type '%s' for shape '%s'", shape.Type, shape.ID))
	}

	// Validate color if present (optional property)
	validateVec3PropertyOptional(&errors, shape.Properties, "color", &zero, &one, "shape", shape.ID)

	// Validate material if present (optional property)
	if mat, ok := extractMaterial(shape.Properties); ok {
		validateMaterial(&errors, mat, shape.ID)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// validateLightProperties validates a light's structure and properties
func validateLightProperties(light LightRequest) error {
	var errors ValidationErrors
	zero := 0.0
	maxAngle := 180.0

	validateStringRequired(&errors, light.ID, "light ID")
	validateStringRequired(&errors, light.Type, "light type")

	if light.Properties == nil {
		errors = append(errors, fmt.Sprintf("light properties cannot be nil for light '%s'", light.ID))
		return errors // can't validate further without Properties
	}

	// Validate type-specific properties
	switch light.Type {
	case "point_spot_light":
		validateVec3PropertyRequired(&errors, light.Properties, "center", nil, nil, "point_spot_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "emission", &zero, nil, "point_spot_light", light.ID)
		validateVec3PropertyOptional(&errors, light.Properties, "direction", nil, nil, "point_spot_light", light.ID)
		validateFloatPropertyOptional(&errors, light.Properties, "cutoff_angle", &zero, &maxAngle, "point_spot_light", light.ID, "cutoff_angle must be between 0 and 180 degrees")
		validateFloatPropertyOptional(&errors, light.Properties, "falloff_exponent", &zero, nil, "point_spot_light", light.ID, "")

	case "area_quad_light":
		validateVec3PropertyRequired(&errors, light.Properties, "corner", nil, nil, "area_quad_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "u", nil, nil, "area_quad_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "v", nil, nil, "area_quad_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "emission", &zero, nil, "area_quad_light", light.ID)

	case "disc_spot_light":
		// Required: center, normal, radius, emission
		validateVec3PropertyRequired(&errors, light.Properties, "center", nil, nil, "disc_spot_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "normal", nil, nil, "disc_spot_light", light.ID)
		validatePositiveFloatRequired(&errors, light.Properties, "radius", "disc_spot_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "emission", &zero, nil, "disc_spot_light", light.ID)

	case "area_sphere_light":
		// Required: center, radius, emission
		validateVec3PropertyRequired(&errors, light.Properties, "center", nil, nil, "area_sphere_light", light.ID)
		validatePositiveFloatRequired(&errors, light.Properties, "radius", "area_sphere_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "emission", &zero, nil, "area_sphere_light", light.ID)

	case "area_disc_spot_light":
		// Required: center, normal, radius, emission, cutoff_angle, falloff_exponent
		validateVec3PropertyRequired(&errors, light.Properties, "center", nil, nil, "area_disc_spot_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "normal", nil, nil, "area_disc_spot_light", light.ID)
		validatePositiveFloatRequired(&errors, light.Properties, "radius", "area_disc_spot_light", light.ID)
		validateVec3PropertyRequired(&errors, light.Properties, "emission", &zero, nil, "area_disc_spot_light", light.ID)
		validateFloatPropertyRequired(&errors, light.Properties, "cutoff_angle", &zero, &maxAngle, "area_disc_spot_light", light.ID, "cutoff_angle must be between 0 and 180 degrees")
		validateFloatPropertyRequired(&errors, light.Properties, "falloff_exponent", &zero, nil, "area_disc_spot_light", light.ID, "")

	case "":
		// Already handled above
	default:
		errors = append(errors, fmt.Sprintf("unsupported light type '%s' for light '%s'", light.Type, light.ID))
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// validateMaterial validates material properties
func validateMaterial(errors *ValidationErrors, mat map[string]interface{}, shapeID string) {
	// Material type is required
	matType, ok := mat["type"].(string)
	if !ok {
		*errors = append(*errors, fmt.Sprintf("shape '%s' material must have a 'type' field", shapeID))
		return
	}

	// Helper variables for range validation
	zero := 0.0
	one := 1.0
	minRefractiveIndex := 1.0

	switch matType {
	case "lambertian":
		validateVec3PropertyRequired(errors, mat, "albedo", &zero, &one, matType+" material", shapeID)

	case "metal":
		validateVec3PropertyRequired(errors, mat, "albedo", &zero, &one, matType+" material", shapeID)
		validateFloatPropertyRequired(errors, mat, "fuzz", &zero, &one, matType+" material", shapeID, "")

	case "dielectric":
		validateFloatPropertyRequired(errors, mat, "refractive_index", &minRefractiveIndex, nil, matType+" material", shapeID, "")

	default:
		*errors = append(*errors, fmt.Sprintf("shape '%s' has unsupported material type '%s' (supported: lambertian, metal, dielectric)", shapeID, matType))
	}
}

// Camera validation helpers

// validateVec3Required validates that a Vec3 ([]float64) is non-nil and has exactly 3 elements
func validateVec3Required(errors *ValidationErrors, vec []float64, fieldName string) {
	if vec == nil {
		*errors = append(*errors, fmt.Sprintf("%s must be provided", fieldName))
		return
	}
	if len(vec) != 3 {
		*errors = append(*errors, fmt.Sprintf("%s must have exactly 3 values", fieldName))
		return
	}
}

// validateVec3NotEqual validates that two Vec3 arrays are not identical
func validateVec3NotEqual(errors *ValidationErrors, vec1, vec2 []float64, name1, name2 string) {
	if vec1 != nil && vec2 != nil && len(vec1) == 3 && len(vec2) == 3 {
		if vec1[0] == vec2[0] && vec1[1] == vec2[1] && vec1[2] == vec2[2] {
			*errors = append(*errors, fmt.Sprintf("%s and %s cannot be the same point", name1, name2))
		}
	}
}

// validateFloatRangeInclusive validates that a float is within an inclusive range [min, max]
func validateFloatRangeInclusive(errors *ValidationErrors, value, min, max float64, fieldName string) {
	if value < min || value > max {
		*errors = append(*errors, fmt.Sprintf("%s must be in range [%.1f, %.1f]", fieldName, min, max))
	}
}

// validateFloatRangeExclusive validates that a float is within an exclusive range (min < value < max)
func validateFloatRangeExclusive(errors *ValidationErrors, value, min, max float64, fieldName string) {
	if value <= min || value >= max {
		*errors = append(*errors, fmt.Sprintf("%s must be in range (%.1f, %.1f)", fieldName, min, max))
	}
}

// Shape and light validation helpers (for property bags)

// validateVec3PropertyRequired validates a required Vec3 property in a property bag
func validateVec3PropertyRequired(errors *ValidationErrors, properties map[string]interface{}, key string, minVal, maxVal *float64, objType, objID string) {
	val, ok := properties[key].([]interface{})
	if !ok {
		*errors = append(*errors, fmt.Sprintf("%s '%s' requires '%s' property", objType, objID, key))
		return
	}
	if len(val) != 3 {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must have exactly 3 values", objType, objID, key))
		return
	}

	// Convert []interface{} to []float64 and validate types
	fieldName := fmt.Sprintf("%s '%s' %s", objType, objID, key)
	for i, v := range val {
		f, ok := v.(float64)
		if !ok {
			*errors = append(*errors, fmt.Sprintf("%s[%d] must be a number", fieldName, i))
			return
		}
		// Validate range if specified
		if minVal != nil && f < *minVal {
			*errors = append(*errors, fmt.Sprintf("%s[%d] must be >= %.1f", fieldName, i, *minVal))
		}
		if maxVal != nil && f > *maxVal {
			*errors = append(*errors, fmt.Sprintf("%s[%d] must be <= %.1f", fieldName, i, *maxVal))
		}
	}
}

// validateVec3PropertyOptional validates an optional Vec3 property in a property bag (only if present)
func validateVec3PropertyOptional(errors *ValidationErrors, properties map[string]interface{}, key string, minVal, maxVal *float64, objType, objID string) {
	if !hasProperty(properties, key) {
		return // Property is optional and not present
	}
	// If present, validate it
	validateVec3PropertyRequired(errors, properties, key, minVal, maxVal, objType, objID)
}

// validateFloatPropertyRequired validates a required float property with optional range and custom error message for constraint violations
func validateFloatPropertyRequired(errors *ValidationErrors, properties map[string]interface{}, key string, minVal, maxVal *float64, objType, objID string, constraintErrMsg string) {
	if !hasProperty(properties, key) {
		*errors = append(*errors, fmt.Sprintf("%s '%s' requires '%s' property", objType, objID, key))
		return
	}

	val, ok := properties[key].(float64)
	if !ok {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be a number", objType, objID, key))
		return
	}

	// Validate range if specified
	if constraintErrMsg != "" && minVal != nil && maxVal != nil && (val < *minVal || val > *maxVal) {
		// Use custom error message
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s", objType, objID, constraintErrMsg))
		return
	}
	if minVal != nil && val < *minVal {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be >= %.1f", objType, objID, key, *minVal))
	}
	if maxVal != nil && val > *maxVal {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be <= %.1f", objType, objID, key, *maxVal))
	}
}

// validateFloatPropertyOptional validates an optional float property (only if present)
func validateFloatPropertyOptional(errors *ValidationErrors, properties map[string]interface{}, key string, minVal, maxVal *float64, objType, objID string, constraintErrMsg string) {
	if !hasProperty(properties, key) {
		return // Property is optional and not present
	}
	// If present, validate it
	validateFloatPropertyRequired(errors, properties, key, minVal, maxVal, objType, objID, constraintErrMsg)
}

// validatePositiveFloatRequired validates a required positive float property (> 0)
func validatePositiveFloatRequired(errors *ValidationErrors, properties map[string]interface{}, key string, objType, objID string) {
	if !hasProperty(properties, key) {
		*errors = append(*errors, fmt.Sprintf("%s '%s' requires '%s' property", objType, objID, key))
		return
	}

	val, ok := properties[key].(float64)
	if !ok {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be a number", objType, objID, key))
		return
	}

	if val <= 0 {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be positive", objType, objID, key))
	}
}

// validateNonNegativeFloatRequired validates a required non-negative float property (>= 0)
func validateNonNegativeFloatRequired(errors *ValidationErrors, properties map[string]interface{}, key string, objType, objID string) {
	if !hasProperty(properties, key) {
		*errors = append(*errors, fmt.Sprintf("%s '%s' requires '%s' property", objType, objID, key))
		return
	}

	val, ok := properties[key].(float64)
	if !ok {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be a number", objType, objID, key))
		return
	}

	if val < 0 {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be non-negative", objType, objID, key))
	}
}

// validateBoolPropertyRequired validates a required boolean property
func validateBoolPropertyRequired(errors *ValidationErrors, properties map[string]interface{}, key string, objType, objID string) {
	if !hasProperty(properties, key) {
		*errors = append(*errors, fmt.Sprintf("%s '%s' requires '%s' property", objType, objID, key))
		return
	}

	_, ok := properties[key].(bool)
	if !ok {
		*errors = append(*errors, fmt.Sprintf("%s '%s' %s must be a boolean", objType, objID, key))
		return
	}
}

// validateStringRequired validates that a string is non-empty
func validateStringRequired(errors *ValidationErrors, value string, fieldName string) {
	if value == "" {
		*errors = append(*errors, fmt.Sprintf("%s cannot be empty", fieldName))
	}
}

// Helper functions for extracting properties from map[string]interface{}

// extractFloatArray extracts a float array of specified length from properties
func extractFloatArray(properties map[string]interface{}, key string, length int) ([]float64, bool) {
	if val, ok := properties[key].([]interface{}); ok && len(val) == length {
		result := make([]float64, length)
		for i, v := range val {
			if f, ok := v.(float64); ok {
				result[i] = f
			} else {
				return nil, false
			}
		}
		return result, true
	}
	return nil, false
}

// extractFloat extracts a single float value from properties
func extractFloat(properties map[string]interface{}, key string) (float64, bool) {
	if val, ok := properties[key].(float64); ok {
		return val, true
	}
	return 0, false
}

// extractString extracts a string value from properties
func extractString(properties map[string]interface{}, key string) (string, bool) {
	if val, ok := properties[key].(string); ok {
		return val, true
	}
	return "", false
}

// hasProperty checks if a property exists in the map
func hasProperty(properties map[string]interface{}, key string) bool {
	_, exists := properties[key]
	return exists
}

// extractMaterial extracts material specification from shape properties
// Returns (materialMap, exists)
func extractMaterial(properties map[string]interface{}) (map[string]interface{}, bool) {
	if mat, ok := properties["material"].(map[string]interface{}); ok {
		return mat, true
	}
	return nil, false
}
