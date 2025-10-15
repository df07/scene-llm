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

// validateStringRequired validates that a string is non-empty
func validateStringRequired(errors *ValidationErrors, value string, fieldName string) {
	if value == "" {
		*errors = append(*errors, fmt.Sprintf("%s cannot be empty", fieldName))
	}
}
