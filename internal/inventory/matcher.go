package inventory

import (
	"fmt"
	"regexp"
	"strings"
)

// ResourceMatch represents a resource match pattern
type ResourceMatch struct {
	Type      string                 // e.g., "aws_vpc"
	Name      string                 // e.g., "main" or ".*" for wildcard
	Count     *int                   // exact count
	MinCount  *int                   // minimum count
	MaxCount  *int                   // maximum count
	Attributes map[string]interface{} // attribute assertions
}

// MatchResult represents the result of matching resources
type MatchResult struct {
	Matched     bool
	Count       int
	Resources   []MatchedResource
	Issues      []string
	Mismatches  []AttributeMismatch
}

// MatchedResource represents a matched resource
type MatchedResource struct {
	Type      string
	Name      string
	ID        string
	Address   string
	Attributes map[string]interface{}
}

// AttributeMismatch represents an attribute that didn't match
type AttributeMismatch struct {
	Resource  string
	Attribute string
	Expected  interface{}
	Actual    interface{}
}

// Matcher matches resources against expected patterns
type Matcher struct {
	resources []Resource
}

// Resource represents a terraform resource from state
type Resource struct {
	Type      string
	Name      string
	Address   string
	ID        string
	Attributes map[string]interface{}
}

// NewMatcher creates a new matcher
func NewMatcher(resources []Resource) *Matcher {
	return &Matcher{
		resources: resources,
	}
}

// Match matches resources against expected patterns
func (m *Matcher) Match(expected map[string]ResourceMatch) (map[string]MatchResult, []string) {
	results := make(map[string]MatchResult)
	var globalIssues []string

	for pattern, match := range expected {
		result := m.matchPattern(pattern, match)
		results[pattern] = result
		
		if !result.Matched {
			globalIssues = append(globalIssues, result.Issues...)
		}
	}

	return results, globalIssues
}

func (m *Matcher) matchPattern(pattern string, match ResourceMatch) MatchResult {
	result := MatchResult{
		Resources:  []MatchedResource{},
		Issues:     []string{},
		Mismatches: []AttributeMismatch{},
	}

	// Build regex for type and name matching
	typeRegex := regexp.MustCompile("^" + regexp.QuoteMeta(match.Type) + "$")
	namePattern := match.Name
	if namePattern == "" {
		namePattern = ".*"
	}
	
	// Convert wildcard pattern to regex
	nameRegexStr := strings.ReplaceAll(regexp.QuoteMeta(namePattern), "\\*", ".*")
	nameRegex := regexp.MustCompile("^" + nameRegexStr + "$")

	// Find matching resources
	var matched []Resource
	for _, res := range m.resources {
		if typeRegex.MatchString(res.Type) && nameRegex.MatchString(res.Name) {
			matched = append(matched, res)
		}
	}

	result.Count = len(matched)

	// Check count constraints
	if match.Count != nil {
		if result.Count != *match.Count {
			result.Issues = append(result.Issues, fmt.Sprintf("expected exactly %d resources, found %d", *match.Count, result.Count))
		}
	}
	if match.MinCount != nil && result.Count < *match.MinCount {
		result.Issues = append(result.Issues, fmt.Sprintf("expected at least %d resources, found %d", *match.MinCount, result.Count))
	}
	if match.MaxCount != nil && result.Count > *match.MaxCount {
		result.Issues = append(result.Issues, fmt.Sprintf("expected at most %d resources, found %d", *match.MaxCount, result.Count))
	}

	// Check attributes for matched resources
	for _, res := range matched {
		matchedRes := MatchedResource{
			Type:       res.Type,
			Name:       res.Name,
			ID:         res.ID,
			Address:    res.Address,
			Attributes: res.Attributes,
		}

		// Validate attributes
		for attrPath, expectedVal := range match.Attributes {
			actualVal, err := m.getNestedAttribute(res.Attributes, attrPath)
			if err != nil {
				result.Mismatches = append(result.Mismatches, AttributeMismatch{
					Resource:  res.Address,
					Attribute: attrPath,
					Expected:  expectedVal,
					Actual:    nil,
				})
				result.Issues = append(result.Issues, fmt.Sprintf("%s: attribute %s not found", res.Address, attrPath))
				continue
			}

			if !m.valuesEqual(expectedVal, actualVal) {
				result.Mismatches = append(result.Mismatches, AttributeMismatch{
					Resource:  res.Address,
					Attribute: attrPath,
					Expected:  expectedVal,
					Actual:    actualVal,
				})
				result.Issues = append(result.Issues, fmt.Sprintf("%s: attribute %s mismatch - expected %v, got %v", res.Address, attrPath, expectedVal, actualVal))
			}
		}

		result.Resources = append(result.Resources, matchedRes)
	}

	result.Matched = len(result.Issues) == 0 && 
		(match.Count == nil || result.Count == *match.Count) &&
		(match.MinCount == nil || result.Count >= *match.MinCount) &&
		(match.MaxCount == nil || result.Count <= *match.MaxCount)

	return result
}

// getNestedAttribute gets a nested attribute using dot notation (e.g., "tags.Name")
func (m *Matcher) getNestedAttribute(attrs map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := attrs

	for i, part := range parts {
		val, exists := current[part]
		if !exists {
			return nil, fmt.Errorf("attribute not found: %s", path)
		}

		if i == len(parts)-1 {
			return val, nil
		}

		next, ok := val.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("attribute %s is not a map", strings.Join(parts[:i+1], "."))
		}
		current = next
	}

	return nil, fmt.Errorf("unexpected error")
}

// valuesEqual compares two values for equality
func (m *Matcher) valuesEqual(expected, actual interface{}) bool {
	// Handle string comparison (most common)
	if expStr, ok := expected.(string); ok {
		actStr := fmt.Sprintf("%v", actual)
		return expStr == actStr
	}

	// Handle numeric comparison
	if expNum, ok := expected.(float64); ok {
		if actNum, ok := actual.(float64); ok {
			return expNum == actNum
		}
		if actNum, ok := actual.(int); ok {
			return expNum == float64(actNum)
		}
	}

	// Handle bool comparison
	if expBool, ok := expected.(bool); ok {
		if actBool, ok := actual.(bool); ok {
			return expBool == actBool
		}
	}

	// Fallback to string comparison
	return fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", actual)
}

