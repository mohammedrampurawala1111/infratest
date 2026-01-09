package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/infratest/infratest/internal/flow"
	"github.com/infratest/infratest/internal/flow/interpolator"
	"github.com/infratest/infratest/internal/reporting"
	"github.com/infratest/infratest/internal/ui"
	"github.com/spf13/cobra"
)

var (
	debug          bool
	localstack     bool
	localstackEndpoint string
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
	runCmd.Flags().StringVar(&localstackEndpoint, "localstack-endpoint", "http://localhost:4566", "LocalStack endpoint URL (only used with --localstack)")
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

	// Setup LocalStack environment if enabled
	if localstack {
		// Check if YAML provides endpoint override
		endpoint := localstackEndpoint
		if f.Environment.Endpoint != "" {
			endpoint = f.Environment.Endpoint
			ui.PrintInfo(fmt.Sprintf("🔧 Using endpoint from YAML: %s", endpoint))
		}
		
		// Check if LocalStack is reachable
		if err := checkLocalStackAvailability(endpoint); err != nil {
			ui.PrintWarning(fmt.Sprintf("⚠️  LocalStack not detected at %s", endpoint))
			showLocalStackStartInstructions(endpoint)
			return fmt.Errorf("LocalStack not available: %w", err)
		}
		
		setupLocalStackEnv(endpoint)
		ui.PrintInfo(fmt.Sprintf("🔧 LocalStack mode enabled (endpoint: %s)", endpoint))
	}
	
	// Show debug information at startup
	if debug {
		fmt.Println()
		color.New(color.FgMagenta, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		color.New(color.FgMagenta, color.Bold).Printf("  [DEBUG MODE ENABLED]\n")
		color.New(color.FgMagenta, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		color.New(color.FgCyan).Printf("Flow File: %s\n", flowPath)
		color.New(color.FgCyan).Printf("Working Directory: %s\n", f.WorkingDir)
		color.New(color.FgCyan).Printf("Total Steps: %d\n", len(f.Steps))
		if localstack {
			color.New(color.FgCyan).Printf("LocalStack: enabled\n")
		}
		color.New(color.FgCyan).Printf("Cleanup Timeout: %v\n", cleanupTimeout)
		color.New(color.FgMagenta, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Println()
	}

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
		
		// Run cleanup (manual instructions shown if it fails)
		if err := cleanupMgr.RunCleanup(); err != nil {
			// Manual destroy instructions are already shown in RunCleanup
			// Just return the error
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

// showErrorDetails shows detailed error information with timeline
func showErrorDetails(executor *flow.Executor, err error) {
	results := executor.GetResults()
	f := executor.GetFlow()
	
	fmt.Println()
	color.New(color.FgRed, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	color.New(color.FgRed, color.Bold).Printf("  ❌ FLOW EXECUTION FAILED\n")
	color.New(color.FgRed, color.Bold).Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Println()
	
	// Show timeline of steps
	color.New(color.FgCyan, color.Bold).Printf("Execution Timeline:\n")
	fmt.Println()
	
	for i, step := range f.Steps {
		stepNum := i + 1
		// Find result for this step
		var result *flow.StepResult
		for j := range results {
			if results[j].StepName == step.Name {
				result = &results[j]
				break
			}
		}
		
		if result != nil && result.Success {
			// Show successful step with green checkmark
			color.New(color.FgGreen).Printf("  ✓ Step %d/%d: %s", stepNum, len(f.Steps), step.Name)
			color.New(color.FgHiBlack).Printf(" [%s]\n", result.Duration.Round(time.Second))
		} else if result != nil && !result.Success {
			// Show failed step with red X
			color.New(color.FgRed, color.Bold).Printf("  ✗ Step %d/%d: %s", stepNum, len(f.Steps), step.Name)
			if result != nil {
				color.New(color.FgHiBlack).Printf(" [%s]\n", result.Duration.Round(time.Second))
			} else {
				fmt.Println()
			}
		} else {
			// Step not executed yet
			color.New(color.FgHiBlack).Printf("  ⊘ Step %d/%d: %s", stepNum, len(f.Steps), step.Name)
			color.New(color.FgHiBlack).Printf(" [not executed]\n")
		}
	}
	
	fmt.Println()
	
	// Show failing step details
	for _, r := range results {
		if !r.Success {
			color.New(color.FgRed, color.Bold).Printf("Failed Step: %s\n", r.StepName)
			color.New(color.FgYellow).Printf("Type: %s\n", r.StepType)
			
			if r.Error != nil {
				fmt.Println()
				color.New(color.FgRed).Printf("Error: %v\n", r.Error)
			}
			
			// Output is already shown by terraform executor with full formatting
			// Only show here if it's not a terraform step (e.g., HTTP, inventory)
			if r.StepType != "terraform" && r.Output != "" {
				fmt.Println()
				color.New(color.FgYellow).Printf("Output:\n")
				fmt.Print(indentOutput(r.Output))
				fmt.Println()
			}
		}
	}
	
	fmt.Println()
}

func indentOutput(output string) string {
	lines := strings.Split(output, "\n")
	indented := make([]string, len(lines))
	for i, line := range lines {
		indented[i] = "    " + line
	}
	return strings.Join(indented, "\n")
}

// setupLocalStackEnv configures environment variables for LocalStack
func setupLocalStackEnv(endpoint string) {
	os.Setenv("AWS_ENDPOINT_URL", endpoint)
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	os.Setenv("AWS_REGION", "us-east-1")
	
	// Skip cost warnings and other AWS SDK warnings
	os.Setenv("TF_LOG", "")
	os.Setenv("TF_LOG_PATH", "")
	
	// Suppress Terraform cost estimation warnings
	os.Setenv("TF_IN_AUTOMATION", "true")
}

// checkLocalStackAvailability checks if LocalStack is reachable at the given endpoint
func checkLocalStackAvailability(endpoint string) error {
	// Try to connect to LocalStack health endpoint
	// LocalStack typically exposes a health check at /_localstack/health
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	
	healthURL := strings.TrimSuffix(endpoint, "/") + "/_localstack/health"
	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("cannot connect to LocalStack: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("LocalStack health check returned status %d", resp.StatusCode)
	}
	
	return nil
}

// showLocalStackStartInstructions shows helpful instructions for starting LocalStack
func showLocalStackStartInstructions(endpoint string) {
	fmt.Println()
	ui.PrintInfo("To start LocalStack, run one of the following:")
	fmt.Println()
	
	ui.PrintInfo("Option 1: Docker (Recommended)")
	fmt.Printf("  docker run -d -p 4566:4566 localstack/localstack\n")
	fmt.Println()
	
	ui.PrintInfo("Option 2: LocalStack CLI")
	fmt.Printf("  localstack start\n")
	fmt.Println()
	
	ui.PrintInfo("Option 3: Docker Compose")
	fmt.Printf("  # Create docker-compose.yml:\n")
	fmt.Printf("  version: '3.8'\n")
	fmt.Printf("  services:\n")
	fmt.Printf("    localstack:\n")
	fmt.Printf("      image: localstack/localstack\n")
	fmt.Printf("      ports:\n")
	fmt.Printf("        - \"4566:4566\"\n")
	fmt.Printf("      environment:\n")
	fmt.Printf("        - SERVICES=ec2,s3,vpc,iam,sts\n")
	fmt.Printf("  docker-compose up -d\n")
	fmt.Println()
	
	ui.PrintInfo("After starting LocalStack, verify it's running:")
	fmt.Printf("  curl %s/_localstack/health\n", endpoint)
	fmt.Println()
	
	ui.PrintWarning("Note: If LocalStack is running on a different endpoint, use:")
	fmt.Printf("  --localstack-endpoint <your-endpoint>\n")
	fmt.Println()
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
			err = reporting.GenerateHTMLReport(flowInfo, stepResults, outputPath, outputs)
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

