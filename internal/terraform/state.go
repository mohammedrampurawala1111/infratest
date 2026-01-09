package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Resource represents a Terraform resource from state
type Resource struct {
	Type       string
	ID         string
	Name       string
	Address    string
	Attributes map[string]interface{}
}

// State represents Terraform state structure
type State struct {
	Values StateValues `json:"values"`
}

// StateValues contains the root module
type StateValues struct {
	RootModule StateRootModule `json:"root_module"`
}

// StateRootModule contains resources
type StateRootModule struct {
	Resources []StateResource `json:"resources"`
}

// StateResource represents a resource in Terraform state
type StateResource struct {
	Address string                 `json:"address"`
	Mode    string                 `json:"mode"` // "managed" or "data"
	Type    string                 `json:"type"`
	Name    string                 `json:"name"`
	Values  map[string]interface{} `json:"values"`
}

// GetState reads and parses Terraform state
func GetState(workingDir string) (*State, error) {
	// Use terraform show -json to get state
	cmd := exec.Command("terraform", "show", "-json")
	cmd.Dir = workingDir
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read terraform state: %w", err)
	}

	var state State
	if err := json.Unmarshal(output, &state); err != nil {
		return nil, fmt.Errorf("failed to parse terraform state: %w", err)
	}

	return &state, nil
}

// GetResources extracts all resources from state
func (s *State) GetResources() []Resource {
	var resources []Resource
	for _, sr := range s.Values.RootModule.Resources {
		// Only include managed resources, skip data sources
		if sr.Mode != "managed" {
			continue
		}

		id := ""
		attributes := make(map[string]interface{})
		if sr.Values != nil {
			if idVal, ok := sr.Values["id"].(string); ok {
				id = idVal
			}
			// Copy all attributes
			for k, v := range sr.Values {
				attributes[k] = v
			}
		}

		resources = append(resources, Resource{
			Type:       sr.Type,
			ID:         id,
			Name:       sr.Name,
			Address:    sr.Address,
			Attributes: attributes,
		})
	}
	return resources
}

// GetResourcesByType returns resources filtered by type
func (s *State) GetResourcesByType(resourceType string) []Resource {
	var filtered []Resource
	for _, r := range s.GetResources() {
		if r.Type == resourceType {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// GetOutputs reads Terraform outputs (uses ParseOutputs for consistency)
func GetOutputs(workingDir string) (map[string]interface{}, error) {
	return ParseOutputs(workingDir)
}

