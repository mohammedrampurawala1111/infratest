package terraform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Executor handles Terraform command execution
type Executor struct {
	workingDir string
	debug      bool
}

// NewExecutor creates a new Terraform executor
func NewExecutor(workingDir string, debug bool) (*Executor, error) {
	// Check if terraform binary exists
	if _, err := exec.LookPath("terraform"); err != nil {
		return nil, fmt.Errorf("terraform binary not found in PATH. Please install Terraform and ensure it's available in your PATH")
	}

	// Resolve working directory
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("invalid working directory: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("working directory does not exist: %s", absPath)
	}

	return &Executor{
		workingDir: absPath,
		debug:      debug,
	}, nil
}

// Execute runs a terraform command
func (e *Executor) Execute(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Remove 'terraform' prefix if present
	if parts[0] == "terraform" {
		parts = parts[1:]
	}

	cmd := exec.Command("terraform", parts...)
	cmd.Dir = e.workingDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if e.debug {
		fmt.Printf("[DEBUG] terraform %s\n", strings.Join(parts, " "))
		fmt.Printf("[DEBUG] Output: %s\n", outputStr)
	}

	if err != nil {
		return outputStr, fmt.Errorf("terraform command failed: %w", err)
	}

	return outputStr, nil
}

// ExecuteMultiple runs multiple terraform commands sequentially
func (e *Executor) ExecuteMultiple(commands []string) (string, error) {
	var allOutput strings.Builder
	for _, cmd := range commands {
		output, err := e.Execute(cmd)
		allOutput.WriteString(output)
		allOutput.WriteString("\n")
		if err != nil {
			return allOutput.String(), err
		}
	}
	return allOutput.String(), nil
}

