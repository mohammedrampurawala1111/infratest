package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ParseOutputs parses terraform output -json into a map with proper type handling
func ParseOutputs(workingDir string) (map[string]interface{}, error) {
	cmd := exec.Command("terraform", "output", "-json")
	cmd.Dir = workingDir
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read terraform outputs: %w", err)
	}

	var outputs map[string]interface{}
	if err := json.Unmarshal(output, &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse terraform outputs: %w", err)
	}

	// Extract values from output structure
	result := make(map[string]interface{})
	for key, val := range outputs {
		if outputMap, ok := val.(map[string]interface{}); ok {
			if value, exists := outputMap["value"]; exists {
				result[key] = value
			}
		}
	}

	return result, nil
}

// GetOutputValue retrieves a nested value from outputs using dot notation or array access
// Examples:
//   - "alb_dns" -> outputs["alb_dns"]
//   - "instance_ids[0]" -> outputs["instance_ids"].([]interface{})[0]
//   - "config.database.host" -> outputs["config"].(map[string]interface{})["database"].(map[string]interface{})["host"]
func GetOutputValue(outputs map[string]interface{}, path string) (interface{}, error) {
	if outputs == nil {
		return nil, fmt.Errorf("outputs map is nil")
	}

	// Handle array access like "key[0]"
	if idx := findArrayIndex(path); idx >= 0 {
		key := path[:idx]
		index := parseArrayIndex(path[idx:])
		if index < 0 {
			return nil, fmt.Errorf("invalid array index in path: %s", path)
		}

		val, err := GetOutputValue(outputs, key)
		if err != nil {
			return nil, err
		}

		arr, ok := val.([]interface{})
		if !ok {
			return nil, fmt.Errorf("path %s is not an array", key)
		}

		if index >= len(arr) {
			return nil, fmt.Errorf("array index %d out of bounds for path %s (length: %d)", index, key, len(arr))
		}

		return arr[index], nil
	}

	// Handle dot notation for nested maps
	parts := splitPath(path)
	current := outputs

	for i, part := range parts {
		val, exists := current[part]
		if !exists {
			return nil, fmt.Errorf("output path not found: %s (missing: %s)", path, part)
		}

		// If this is the last part, return the value
		if i == len(parts)-1 {
			return val, nil
		}

		// Otherwise, continue navigating
		next, ok := val.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("path %s is not a map at segment: %s", path, part)
		}
		current = next
	}

	return nil, fmt.Errorf("unexpected error parsing path: %s", path)
}

// findArrayIndex finds the position of '[' in the path
func findArrayIndex(path string) int {
	for i, r := range path {
		if r == '[' {
			return i
		}
	}
	return -1
}

// parseArrayIndex extracts the index from "[123]"
func parseArrayIndex(s string) int {
	if len(s) < 3 || s[0] != '[' || s[len(s)-1] != ']' {
		return -1
	}
	var idx int
	if _, err := fmt.Sscanf(s, "[%d]", &idx); err != nil {
		return -1
	}
	return idx
}

// splitPath splits a dot-notation path into parts
func splitPath(path string) []string {
	var parts []string
	var current strings.Builder
	
	for _, r := range path {
		if r == '.' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}
	
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	
	return parts
}

