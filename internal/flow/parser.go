package flow

import (
	"fmt"
	"os"
	"path/filepath"

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

	// Resolve working_dir relative to the flow file's directory
	flowFileDir := filepath.Dir(path)
	if !filepath.IsAbs(flow.WorkingDir) {
		// If working_dir is relative, make it relative to the flow file
		flow.WorkingDir = filepath.Join(flowFileDir, flow.WorkingDir)
	}
	// Clean the path to remove any ".." or "." components
	flow.WorkingDir = filepath.Clean(flow.WorkingDir)

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

