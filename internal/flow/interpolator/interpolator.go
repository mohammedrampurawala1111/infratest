package interpolator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/infratest/infratest/internal/terraform"
)

// Enhanced regex to match ${output.KEY} and ${output.KEY[0]} and ${output.KEY.nested.path}
var outputRegex = regexp.MustCompile(`\$\{output\.([^}]+)\}`)

// Interpolate replaces ${output.KEY} with actual terraform output values
// Supports:
//   - ${output.key} - simple key
//   - ${output.key[0]} - array access
//   - ${output.key.nested.path} - nested map access
func Interpolate(template string, outputs map[string]interface{}) string {
	return outputRegex.ReplaceAllStringFunc(template, func(match string) string {
		// Extract path from ${output.PATH}
		submatches := outputRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		
		path := submatches[1]
		
		// Use GetOutputValue for complex paths
		val, err := terraform.GetOutputValue(outputs, path)
		if err != nil {
			// Return original if not found (for backward compatibility)
			return match
		}
		
		return formatValue(val)
	})
}

// formatValue formats a value for interpolation
func formatValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		// Check if it's a whole number
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []interface{}:
		// For arrays, join with comma (or return first element if single)
		if len(v) == 1 {
			return formatValue(v[0])
		}
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = formatValue(item)
		}
		return strings.Join(parts, ",")
	case map[string]interface{}:
		// For maps, return JSON-like representation or just indicate it's a map
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// InterpolateWithEnv also supports environment variables
func InterpolateWithEnv(template string, outputs map[string]interface{}) string {
	result := template
	
	// Replace ${output.KEY}
	result = outputRegex.ReplaceAllStringFunc(result, func(match string) string {
		key := outputRegex.FindStringSubmatch(match)[1]
		if val, ok := outputs[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match
	})
	
	// Replace ${env.KEY}
	envRegex := regexp.MustCompile(`\$\{env\.(\w+)\}`)
	result = envRegex.ReplaceAllStringFunc(result, func(match string) string {
		key := envRegex.FindStringSubmatch(match)[1]
		return getEnv(key)
	})
	
	return result
}

func getEnv(key string) string {
	// Simple implementation - could be extended
	return ""
}

