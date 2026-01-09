package terraform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	// Enhanced debug output
	if e.debug {
		fmt.Println()
		color.New(color.FgMagenta, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		color.New(color.FgMagenta, color.Bold).Printf("  [DEBUG] Executing Terraform Command\n")
		color.New(color.FgMagenta, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		
		// Show full command
		fullCommand := fmt.Sprintf("terraform %s", strings.Join(parts, " "))
		color.New(color.FgCyan).Printf("Command: %s\n", fullCommand)
		color.New(color.FgCyan).Printf("Working Directory: %s\n", e.workingDir)
		
		// Show relevant environment variables
		color.New(color.FgCyan).Printf("Environment Variables:\n")
		relevantEnvVars := []string{
			"AWS_ENDPOINT_URL", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
			"AWS_DEFAULT_REGION", "AWS_REGION", "TF_IN_AUTOMATION", "TF_LOG",
		}
		for _, key := range relevantEnvVars {
			if val := os.Getenv(key); val != "" {
				if key == "AWS_SECRET_ACCESS_KEY" {
					color.New(color.FgHiBlack).Printf("  %s=***hidden***\n", key)
				} else {
					color.New(color.FgHiBlack).Printf("  %s=%s\n", key, val)
				}
			}
		}
		
		// Show all env vars if specifically requested (very verbose)
		if os.Getenv("INFRATEST_DEBUG_ENV") == "true" {
			color.New(color.FgCyan).Printf("All Environment Variables:\n")
			for _, env := range cmd.Env {
				if strings.Contains(env, "SECRET") || strings.Contains(env, "PASSWORD") {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) == 2 {
						color.New(color.FgHiBlack).Printf("  %s=***hidden***\n", parts[0])
					}
				} else {
					color.New(color.FgHiBlack).Printf("  %s\n", env)
				}
			}
		}
		
		color.New(color.FgMagenta, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Println()
	} else {
		ui.PrintDebug(e.debug, "Running: terraform %s", strings.Join(parts, " "))
		ui.PrintDebug(e.debug, "Working directory: %s", e.workingDir)
	}

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

		// Always show colored error output on failure (not just in debug)
		fmt.Println()
		color.New(color.FgRed, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		color.New(color.FgRed, color.Bold).Printf("  ✗ TERRAFORM COMMAND FAILED\n")
		color.New(color.FgRed, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		
		color.New(color.FgYellow).Printf("Command: ")
		color.New(color.FgWhite, color.Bold).Printf("terraform %s\n", strings.Join(parts, " "))
		color.New(color.FgYellow).Printf("Working Directory: ")
		color.New(color.FgWhite).Printf("%s\n", e.workingDir)
		color.New(color.FgYellow).Printf("Exit Code: ")
		color.New(color.FgRed, color.Bold).Printf("%d\n", exitCode)
		
		fmt.Println()
		color.New(color.FgRed, color.Bold).Printf("Full Output (stdout + stderr):\n")
		color.New(color.FgHiBlack).Printf("────────────────────────────────────────────────────────────\n")
		
		// Print output with syntax highlighting for common error patterns
		printColoredOutput(outputStr)
		
		color.New(color.FgHiBlack).Printf("────────────────────────────────────────────────────────────\n")
		fmt.Println()
		
		// Show suggested fixes
		suggestFixes(exitCode, outputStr, e.workingDir)

		return outputStr, fmt.Errorf("terraform command failed (exit code: %d): %w", exitCode, err)
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

// printColoredOutput prints terraform output with syntax highlighting for common patterns
func printColoredOutput(output string) {
	lines := strings.Split(output, "\n")
	
	errorPattern := regexp.MustCompile(`(?i)(error|failed|failure)`)
	warningPattern := regexp.MustCompile(`(?i)(warning|deprecated)`)
	successPattern := regexp.MustCompile(`(?i)(success|complete|created|updated)`)
	
	for _, line := range lines {
		if errorPattern.MatchString(line) {
			color.New(color.FgRed).Println(line)
		} else if warningPattern.MatchString(line) {
			color.New(color.FgYellow).Println(line)
		} else if successPattern.MatchString(line) {
			color.New(color.FgGreen).Println(line)
		} else {
			fmt.Println(line)
		}
	}
}

// suggestFixes provides helpful suggestions based on error patterns
func suggestFixes(exitCode int, output string, workingDir string) {
	color.New(color.FgCyan, color.Bold).Printf("💡 Suggested Fixes:\n")
	fmt.Println()
	
	outputLower := strings.ToLower(output)
	
	// Check for common error patterns and suggest fixes
	suggestions := []string{}
	
	if strings.Contains(outputLower, "terraform") && strings.Contains(outputLower, "not found") {
		suggestions = append(suggestions, "Terraform not found in PATH")
		color.New(color.FgYellow).Printf("  → Install Terraform:\n")
		color.New(color.FgWhite).Printf("     • macOS: brew install terraform\n")
		color.New(color.FgWhite).Printf("     • Linux: See https://learn.hashicorp.com/tutorials/terraform/install-cli\n")
		color.New(color.FgWhite).Printf("     • Windows: https://www.terraform.io/downloads\n")
		fmt.Println()
	}
	
	if strings.Contains(outputLower, "authentication") || strings.Contains(outputLower, "credentials") {
		suggestions = append(suggestions, "AWS authentication issue")
		color.New(color.FgYellow).Printf("  → Configure AWS credentials:\n")
		color.New(color.FgWhite).Printf("     • Run: aws configure\n")
		color.New(color.FgWhite).Printf("     • Or set: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY\n")
		color.New(color.FgWhite).Printf("     • Or use: --localstack flag for local testing\n")
		fmt.Println()
	}
	
	if strings.Contains(outputLower, "provider") && strings.Contains(outputLower, "not found") {
		suggestions = append(suggestions, "Terraform provider issue")
		color.New(color.FgYellow).Printf("  → Initialize Terraform providers:\n")
		color.New(color.FgWhite).Printf("     cd %s\n", workingDir)
		color.New(color.FgWhite).Printf("     terraform init\n")
		fmt.Println()
	}
	
	if strings.Contains(outputLower, "state") && (strings.Contains(outputLower, "lock") || strings.Contains(outputLower, "locked")) {
		suggestions = append(suggestions, "Terraform state locked")
		color.New(color.FgYellow).Printf("  → Unlock Terraform state:\n")
		color.New(color.FgWhite).Printf("     cd %s\n", workingDir)
		color.New(color.FgWhite).Printf("     terraform force-unlock <lock-id>\n")
		color.New(color.FgWhite).Printf("     (Find lock-id in the error message above)\n")
		fmt.Println()
	}
	
	if strings.Contains(outputLower, "permission denied") || strings.Contains(outputLower, "access denied") {
		suggestions = append(suggestions, "Permission issue")
		color.New(color.FgYellow).Printf("  → Check permissions:\n")
		color.New(color.FgWhite).Printf("     • Verify AWS IAM permissions\n")
		color.New(color.FgWhite).Printf("     • Check file/directory permissions\n")
		fmt.Println()
	}
	
	if strings.Contains(outputLower, "localstack") || strings.Contains(outputLower, "connection refused") {
		if os.Getenv("AWS_ENDPOINT_URL") != "" {
			suggestions = append(suggestions, "LocalStack connection issue")
			color.New(color.FgYellow).Printf("  → Start LocalStack:\n")
			color.New(color.FgWhite).Printf("     docker run -d -p 4566:4566 localstack/localstack\n")
			color.New(color.FgWhite).Printf("     curl http://localhost:4566/_localstack/health\n")
			fmt.Println()
		}
	}
	
	if strings.Contains(outputLower, "plan file") && strings.Contains(outputLower, "not found") {
		suggestions = append(suggestions, "Terraform plan file missing")
		color.New(color.FgYellow).Printf("  → Regenerate plan:\n")
		color.New(color.FgWhite).Printf("     cd %s\n", workingDir)
		color.New(color.FgWhite).Printf("     terraform plan -out=plan.tfplan\n")
		fmt.Println()
	}
	
	// Generic suggestion if no specific pattern matched
	if len(suggestions) == 0 {
		color.New(color.FgYellow).Printf("  → Review the error output above\n")
		color.New(color.FgYellow).Printf("  → Check Terraform documentation: https://www.terraform.io/docs\n")
		color.New(color.FgYellow).Printf("  → Run with --debug flag for more details:\n")
		color.New(color.FgWhite).Printf("     infratest run <flow.yaml> --debug\n")
		fmt.Println()
	}
	
	color.New(color.FgHiBlack).Printf("────────────────────────────────────────────────────────────\n")
	fmt.Println()
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

