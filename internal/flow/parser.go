package flow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseFlow reads and parses a YAML flow file
func ParseFlow(path string) (*Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file: %w", err)
	}

	var flow Flow
	if err := yaml.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := validateFlow(&flow); err != nil {
		return nil, fmt.Errorf("invalid flow: %w", err)
	}

	return &flow, nil
}

func validateFlow(flow *Flow) error {
	if flow.Name == "" {
		return fmt.Errorf("flow name is required")
	}
	if flow.WorkingDir == "" {
		return fmt.Errorf("working_dir is required")
	}
	if len(flow.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}
	return nil
}

