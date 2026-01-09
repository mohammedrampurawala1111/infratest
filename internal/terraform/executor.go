package terraform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/infratest/infratest/internal/ui"
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

// Execute runs a terraform command (without context, for backward compatibility)
func (e *Executor) Execute(command string) (string, error) {
	return e.ExecuteWithContext(context.Background(), command)
}

// ExecuteWithContext runs a terraform command with context support
func (e *Executor) ExecuteWithContext(ctx context.Context, command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Remove 'terraform' prefix if present
	if parts[0] == "terraform" {
		parts = parts[1:]
	}

	cmd := exec.CommandContext(ctx, "terraform", parts...)
	cmd.Dir = e.workingDir
	cmd.Env = os.Environ()
	
	// Suppress cost warnings if LocalStack is being used
	if os.Getenv("AWS_ENDPOINT_URL") != "" {
		// Add TF_IN_AUTOMATION to suppress interactive prompts
		cmd.Env = append(cmd.Env, "TF_IN_AUTOMATION=true")
	}

	ui.PrintDebug(e.debug, "Running: terraform %s", strings.Join(parts, " "))
	ui.PrintDebug(e.debug, "Working directory: %s", e.workingDir)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Check if context was cancelled
	if ctx.Err() != nil {
		return outputStr, fmt.Errorf("command cancelled: %w", ctx.Err())
	}

	if err != nil {
		// Enhanced error reporting
		exitError, ok := err.(*exec.ExitError)
		exitCode := -1
		if ok {
			exitCode = exitError.ExitCode()
		}

		// Show colored error output
		if e.debug {
			fmt.Println()
			color.New(color.FgRed).Printf("✗ Terraform command failed (exit code: %d)\n", exitCode)
			color.New(color.FgYellow).Printf("Command: terraform %s\n", strings.Join(parts, " "))
			color.New(color.FgRed).Printf("Error output:\n")
			fmt.Print(outputStr)
			fmt.Println()
		}

		return outputStr, fmt.Errorf("terraform command failed (exit code: %d): %w\nOutput:\n%s", exitCode, err, outputStr)
	}

	if e.debug {
		// Show success output in debug mode
		if len(outputStr) > 0 {
			color.New(color.FgGreen).Printf("✓ Command succeeded\n")
			if len(outputStr) < 500 {
				// Only show full output if it's short
				fmt.Print(outputStr)
			} else {
				// Show first and last lines for long output
				lines := strings.Split(outputStr, "\n")
				if len(lines) > 10 {
					fmt.Println(strings.Join(lines[:5], "\n"))
					fmt.Println("... (output truncated) ...")
					fmt.Println(strings.Join(lines[len(lines)-5:], "\n"))
				} else {
					fmt.Print(outputStr)
				}
			}
		}
	}

	return outputStr, nil
}

// ExecuteMultiple runs multiple terraform commands sequentially (without context, for backward compatibility)
func (e *Executor) ExecuteMultiple(commands []string) (string, error) {
	return e.ExecuteMultipleWithContext(context.Background(), commands)
}

// ExecuteMultipleWithContext runs multiple terraform commands sequentially with context support
func (e *Executor) ExecuteMultipleWithContext(ctx context.Context, commands []string) (string, error) {
	var allOutput strings.Builder
	for i, cmd := range commands {
		// Check context cancellation before each command
		select {
		case <-ctx.Done():
			return allOutput.String(), fmt.Errorf("execution cancelled: %w", ctx.Err())
		default:
		}

		ui.PrintDebug(e.debug, "Command %d/%d: %s", i+1, len(commands), cmd)
		output, err := e.ExecuteWithContext(ctx, cmd)
		allOutput.WriteString(output)
		allOutput.WriteString("\n")
		if err != nil {
			return allOutput.String(), fmt.Errorf("command %d/%d failed: %w", i+1, len(commands), err)
		}
	}
	return allOutput.String(), nil
}

