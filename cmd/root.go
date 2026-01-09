package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/infratest/infratest/internal/flow"
	"github.com/infratest/infratest/internal/flow/interpolator"
	"github.com/infratest/infratest/internal/reporting"
	"github.com/spf13/cobra"
)

var (
	debug     bool
	localstack bool
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
}

func Execute() error {
	return rootCmd.Execute()
}

func executeFlow(flowPath string) error {
	// Parse flow
	f, err := flow.ParseFlow(flowPath)
	if err != nil {
		return fmt.Errorf("failed to parse flow: %w", err)
	}

	// Create executor
	executor, err := flow.NewExecutor(f, debug)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// Execute flow
	if debug {
		fmt.Println("[DEBUG] Starting flow execution...")
	}
	if err := executor.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Flow execution failed: %v\n", err)
		// Still generate report even on failure
		if err2 := generateReport(executor); err2 != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate report: %v\n", err2)
		}
		return err
	}

	// Generate report
	if debug {
		fmt.Println("[DEBUG] Generating reports...")
	}
	if err := generateReport(executor); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	fmt.Println("Flow executed successfully!")
	return nil
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

