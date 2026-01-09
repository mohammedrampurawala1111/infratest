package interpolator

import (
	"fmt"
	"regexp"
)

var outputRegex = regexp.MustCompile(`\$\{output\.(\w+)\}`)

// Interpolate replaces ${output.KEY} with actual terraform output values
func Interpolate(template string, outputs map[string]interface{}) string {
	return outputRegex.ReplaceAllStringFunc(template, func(match string) string {
		// Extract key from ${output.KEY}
		key := outputRegex.FindStringSubmatch(match)[1]
		if val, ok := outputs[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// Return original if not found
		return match
	})
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

