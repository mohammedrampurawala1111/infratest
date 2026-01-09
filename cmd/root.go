package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/infratest/infratest/internal/flow"
	"github.com/infratest/infratest/internal/flow/interpolator"
	"github.com/infratest/infratest/internal/reporting"
	"github.com/infratest/infratest/internal/ui"
	"github.com/spf13/cobra"
)

var (
	debug         bool
	localstack    bool
	cleanupTimeout time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "infratest",
	Short: "A declarative YAML-driven E2E infrastructure testing framework",
	Long: `Infratest is a declarative YAML-driven E2E infrastructure testing framework
that validates infrastructure deployments using Terraform.`,
}

var runCmd = &cobra.Command{
	Use:   "run [flow.yaml]",
	Short: "Run an infrastructure test flow",
	Long:  "Execute a test flow defined in a YAML file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flowPath := args[0]
		return executeFlow(flowPath)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output")
	runCmd.Flags().BoolVar(&localstack, "localstack", false, "Use LocalStack for AWS (development)")
	runCmd.Flags().DurationVar(&cleanupTimeout, "cleanup-timeout", 300*time.Second, "Timeout for cleanup operations")
}

func Execute() error {
	return rootCmd.Execute()
}

func executeFlow(flowPath string) error {
	// Early terraform binary check
	if err := checkTerraformBinary(); err != nil {
		return err
	}

	// Check if output is a TTY, disable colors if not
	if !isTerminal(os.Stdout) {
		ui.DisableColors()
	}

	// Parse flow
	f, err := flow.ParseFlow(flowPath)
	if err != nil {
		return fmt.Errorf("failed to parse flow: %w", err)
	}

	ui.PrintInfo(fmt.Sprintf("📋 Flow: %s", f.Name))
	if f.Description != "" {
		ui.PrintInfo(fmt.Sprintf("   %s", f.Description))
	}
	ui.PrintInfo(fmt.Sprintf("📁 Working directory: %s", f.WorkingDir))
	ui.PrintInfo(fmt.Sprintf("📊 Steps: %d", len(f.Steps)))
	fmt.Println()

	// Create executor
	executor, err := flow.NewExecutor(f, debug)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// Setup cleanup manager with panic recovery
	cleanupMgr := flow.NewCleanupManager(executor, cleanupTimeout, debug)
	cleanupMgr.Start()
	defer cleanupMgr.Stop()
	defer func() {
		if r := recover(); r != nil {
			// Cleanup manager will handle this, but we need to recover here too
			cleanupMgr.RunCleanup()
			panic(r)
		}
	}()

	// Execute flow with context
	ui.PrintInfo("🚀 Starting flow execution...")
	fmt.Println()
	
	if err := executor.ExecuteWithContext(cleanupMgr.Context()); err != nil {
		ui.PrintFailure(fmt.Sprintf("❌ Flow execution failed: %v", err))
		
		// Show error details
		showErrorDetails(executor, err)
		
		// Still generate report even on failure
		if err2 := generateReport(executor); err2 != nil {
			ui.PrintError("Failed to generate report: %v", err2)
		}
		
		// Run cleanup
		if err := cleanupMgr.RunCleanup(); err != nil {
			ui.PrintError("Cleanup failed: %v", err)
		}
		
		return err
	}

	// Generate report
	ui.PrintInfo("\n📄 Generating reports...")
	if err := generateReport(executor); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	ui.PrintSuccess("\n✅ Flow executed successfully!")
	return nil
}

// checkTerraformBinary checks if terraform is available in PATH
func checkTerraformBinary() error {
	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		ui.PrintError("Terraform binary not found in PATH")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Please install Terraform:\n")
		fmt.Fprintf(os.Stderr, "  - Visit: https://www.terraform.io/downloads\n")
		fmt.Fprintf(os.Stderr, "  - Or use a package manager:\n")
		fmt.Fprintf(os.Stderr, "    • macOS: brew install terraform\n")
		fmt.Fprintf(os.Stderr, "    • Linux: See https://learn.hashicorp.com/tutorials/terraform/install-cli\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "After installation, ensure 'terraform' is in your PATH:\n")
		fmt.Fprintf(os.Stderr, "  export PATH=$PATH:/path/to/terraform\n")
		return fmt.Errorf("terraform binary not found")
	}
	
	if debug {
		ui.PrintDebug(debug, "Terraform found at: %s", terraformPath)
	}
	
	return nil
}

// isTerminal checks if the file descriptor is a terminal
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// showErrorDetails shows detailed error information
func showErrorDetails(executor *flow.Executor, err error) {
	results := executor.GetResults()
	
	fmt.Println()
	ui.PrintFailure("Error Details:")
	fmt.Println()
	
	// Show successful steps
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	
	if successCount > 0 {
		ui.PrintInfo(fmt.Sprintf("✓ Completed steps: %d", successCount))
		for _, r := range results {
			if r.Success {
				fmt.Printf("  • %s (%s)\n", r.StepName, r.Duration.Round(time.Second))
			}
		}
		fmt.Println()
	}
	
	// Show failing step details
	for _, r := range results {
		if !r.Success {
			ui.PrintFailure(fmt.Sprintf("✗ Failed step: %s", r.StepName))
			if r.Error != nil {
				fmt.Printf("  Error: %v\n", r.Error)
			}
			if r.Output != "" && debug {
				fmt.Printf("  Output:\n%s\n", indentOutput(r.Output))
			}
		}
	}
}

func indentOutput(output string) string {
	lines := strings.Split(output, "\n")
	indented := make([]string, len(lines))
	for i, line := range lines {
		indented[i] = "    " + line
	}
	return strings.Join(indented, "\n")
}

func generateReport(executor *flow.Executor) error {
	f := executor.GetFlow()
	results := executor.GetResults()
	outputs := executor.GetOutputs()

	// Skip if reporting not configured
	if f.Reporting.Output == "" || len(f.Reporting.Formats) == 0 {
		if debug {
			fmt.Println("[DEBUG] Reporting not configured, skipping report generation")
		}
		return nil
	}

	// Extract module name from working directory
	moduleName := extractModuleName(f.WorkingDir)
	if debug {
		fmt.Printf("[DEBUG] Extracted module name: %s\n", moduleName)
	}

	// Interpolate report output path
	outputPath := interpolator.Interpolate(f.Reporting.Output, outputs)
	
	// Replace ${name} with flow name
	outputPath = strings.ReplaceAll(outputPath, "${name}", f.Name)
	
	// Replace ${module} with module name
	outputPath = strings.ReplaceAll(outputPath, "${module}", moduleName)
	
	// Replace date/time placeholders (simple implementation)
	now := time.Now()
	outputPath = strings.ReplaceAll(outputPath, "$(date +%Y%m%d-%H%M%S)", now.Format("20060102-150405"))

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	// Convert flow and results to reporting types
	flowInfo := reporting.FlowInfo{
		Name:        f.Name,
		Description: f.Description,
		WorkingDir:  f.WorkingDir,
	}

	stepResults := make([]reporting.StepResultInfo, len(results))
	for i, r := range results {
		resources := make([]reporting.ResourceInfo, len(r.Resources))
		for j, res := range r.Resources {
			resources[j] = reporting.ResourceInfo{
				Type: res.Type,
				ID:   res.ID,
			}
		}
		stepResults[i] = reporting.StepResultInfo{
			StepName:   r.StepName,
			StepType:   r.StepType,
			Success:    r.Success,
			Output:     r.Output,
			Error:      r.Error,
			Duration:   r.Duration,
			Resources:  resources,
			HTTPStatus: r.HTTPStatus,
		}
	}

	for _, format := range f.Reporting.Formats {
		var err error
		switch format {
		case "html":
			err = reporting.GenerateHTMLReport(flowInfo, stepResults, outputPath)
		case "json":
			jsonPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".json"
			err = reporting.GenerateJSONReport(flowInfo, stepResults, jsonPath)
		default:
			return fmt.Errorf("unsupported report format: %s", format)
		}

		if err != nil {
			return fmt.Errorf("failed to generate %s report: %w", format, err)
		}
	}

	return nil
}

// extractModuleName extracts the module name from the working directory path
// Examples:
//   ./terraform/vpc -> vpc
//   ./terraform/ec2/ec2 -> ec2
//   terraform/vpc -> vpc
//   /absolute/path/terraform/vpc -> vpc
func extractModuleName(workingDir string) string {
	// Normalize path separators
	normalized := strings.ReplaceAll(workingDir, "\\", "/")
	
	// Remove leading/trailing slashes
	normalized = strings.Trim(normalized, "/")
	
	// Split by path separator
	parts := strings.Split(normalized, "/")
	
	// Look for common patterns
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		// Skip empty parts and common directory names
		if part != "" && part != "terraform" && part != "." && part != ".." {
			return part
		}
	}
	
	// Fallback: use the last non-empty part
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	
	// Ultimate fallback
	return "default"
}

