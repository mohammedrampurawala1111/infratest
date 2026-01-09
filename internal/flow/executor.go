package flow

import (
	"fmt"
	"time"

	"github.com/infratest/infratest/internal/flow/interpolator"
	"github.com/infratest/infratest/internal/http"
	"github.com/infratest/infratest/internal/terraform"
)

// Executor runs a flow
type Executor struct {
	flow       *Flow
	executor   *terraform.Executor
	results    []StepResult
	outputs    map[string]interface{}
	debug      bool
}

// NewExecutor creates a new flow executor
func NewExecutor(flow *Flow, debug bool) (*Executor, error) {
	executor, err := terraform.NewExecutor(flow.WorkingDir, debug)
	if err != nil {
		return nil, err
	}

	return &Executor{
		flow:     flow,
		executor: executor,
		results:  make([]StepResult, 0),
		outputs:  make(map[string]interface{}),
		debug:    debug,
	}, nil
}

// Execute runs all steps in the flow
func (e *Executor) Execute() error {
	stepMap := make(map[string]*Step)
	for i := range e.flow.Steps {
		stepMap[e.flow.Steps[i].Name] = &e.flow.Steps[i]
	}

	executed := make(map[string]bool)
	hasFailure := false
	
	for _, step := range e.flow.Steps {
		// Check if step should run based on 'when' condition
		if step.When == "on-success" && hasFailure {
			if e.debug {
				fmt.Printf("[DEBUG] Skipping step %s (when: on-success, but previous step failed)\n", step.Name)
			}
			continue
		}
		if step.When == "on-failure" && !hasFailure {
			if e.debug {
				fmt.Printf("[DEBUG] Skipping step %s (when: on-failure, but no previous failure)\n", step.Name)
			}
			continue
		}

		err := e.executeStep(step, stepMap, executed)
		executed[step.Name] = true

		if err != nil {
			hasFailure = true
			// Check if we should continue based on 'when' condition
			if step.When == "always" {
				// Continue even on error
				if e.debug {
					fmt.Printf("[DEBUG] Step %s failed but continuing (when: always)\n", step.Name)
				}
				continue
			}
			return err
		}
	}

	return nil
}

func (e *Executor) executeStep(step Step, stepMap map[string]*Step, executed map[string]bool) error {
	// Check dependencies
	if step.After != "" {
		if !executed[step.After] {
			return fmt.Errorf("step %s depends on %s which hasn't been executed", step.Name, step.After)
		}
	}

	if e.debug {
		fmt.Printf("[DEBUG] Executing step: %s (type: %s)\n", step.Name, step.Type)
	}

	start := time.Now()
	result := StepResult{
		StepName: step.Name,
		StepType: step.Type,
	}

	var err error
	var output string

	switch step.Type {
	case "terraform":
		output, err = e.executeTerraformStep(step)
		result.Output = output
		result.Success = err == nil

	case "terraform-inventory":
		resources, err2 := e.executeInventoryStep(step)
		result.Resources = resources
		result.Success = err2 == nil
		err = err2

	case "http":
		status, err2 := e.executeHTTPStep(step)
		result.HTTPStatus = status
		result.Success = err2 == nil
		err = err2

	default:
		err = fmt.Errorf("unknown step type: %s", step.Type)
	}

	result.Duration = time.Since(start)
	result.Error = err
	e.results = append(e.results, result)

	if err != nil {
		return fmt.Errorf("step %s failed: %w", step.Name, err)
	}

	return nil
}

func (e *Executor) executeTerraformStep(step Step) (string, error) {
	// Refresh outputs before each terraform step
	outputs, err := terraform.GetOutputs(e.flow.WorkingDir)
	if err == nil {
		e.outputs = outputs
	}

	if step.Command != "" {
		// Interpolate terraform outputs in command
		cmd := interpolator.Interpolate(step.Command, e.outputs)
		return e.executor.Execute(cmd)
	}

	if len(step.Commands) > 0 {
		// Interpolate commands
		interpolated := make([]string, len(step.Commands))
		for i, cmd := range step.Commands {
			interpolated[i] = interpolator.Interpolate(cmd, e.outputs)
		}
		return e.executor.ExecuteMultiple(interpolated)
	}

	return "", fmt.Errorf("no command or commands specified for terraform step")
}

func (e *Executor) executeInventoryStep(step Step) ([]Resource, error) {
	if step.Expected == nil {
		return nil, fmt.Errorf("expected resources not specified")
	}

	// Get current state
	state, err := terraform.GetState(e.flow.WorkingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform state: %w", err)
	}

	allResources := state.GetResources()
	if e.debug {
		fmt.Printf("[DEBUG] Found %d managed resources in state\n", len(allResources))
		for _, r := range allResources {
			fmt.Printf("[DEBUG]   - %s (id: %s)\n", r.Type, r.ID)
		}
	}

	var foundResources []Resource

	// Validate each expected resource type
	for _, expected := range step.Expected.Resources {
		resources := state.GetResourcesByType(expected.Type)
		count := len(resources)

		if expected.MinCount > 0 && count < expected.MinCount {
			if step.FailOnMissing {
				return nil, fmt.Errorf("resource type %s: expected at least %d, found %d", expected.Type, expected.MinCount, count)
			}
		}

		if expected.MaxCount > 0 && count > expected.MaxCount {
			if step.FailOnExtra {
				return nil, fmt.Errorf("resource type %s: expected at most %d, found %d", expected.Type, expected.MaxCount, count)
			}
		}

		// If no min/max specified, just check existence
		if expected.MinCount == 0 && expected.MaxCount == 0 && count == 0 {
			if step.FailOnMissing {
				return nil, fmt.Errorf("resource type %s: expected but not found", expected.Type)
			}
		}

		for _, r := range resources {
			foundResources = append(foundResources, Resource{
				Type: r.Type,
				ID:   r.ID,
			})
		}
	}

	// Check for extra resources if fail_on_extra is true
	if step.FailOnExtra {
		expectedTypes := make(map[string]bool)
		for _, expected := range step.Expected.Resources {
			expectedTypes[expected.Type] = true
		}

		for _, r := range allResources {
			if !expectedTypes[r.Type] {
				return nil, fmt.Errorf("unexpected resource found: %s (id: %s)", r.Type, r.ID)
			}
		}
	}

	return foundResources, nil
}

func (e *Executor) executeHTTPStep(step Step) (int, error) {
	// Refresh outputs before HTTP step to ensure we have the latest values
	outputs, err := terraform.GetOutputs(e.flow.WorkingDir)
	if err == nil {
		e.outputs = outputs
		if e.debug {
			fmt.Printf("[DEBUG] Refreshed terraform outputs:\n")
			for k, v := range e.outputs {
				fmt.Printf("[DEBUG]   %s = %v\n", k, v)
			}
		}
	} else if e.debug {
		fmt.Printf("[DEBUG] Warning: failed to refresh outputs: %v\n", err)
	}

	// Interpolate URL with terraform outputs
	url := interpolator.Interpolate(step.URL, e.outputs)
	
	if e.debug {
		fmt.Printf("[DEBUG] Original URL template: %s\n", step.URL)
		fmt.Printf("[DEBUG] Interpolated URL: %s\n", url)
	}

	// Parse delay
	delay, err := time.ParseDuration(step.Delay)
	if err != nil {
		delay = 10 * time.Second // default
	}

	retries := step.Retries
	if retries == 0 {
		retries = 3 // default
	}

	status, err := http.CheckWithRetry(url, step.ExpectedStatus, retries, delay, e.debug)
	return status, err
}

// GetFlow returns the flow configuration
func (e *Executor) GetFlow() *Flow {
	return e.flow
}

// GetOutputs returns the terraform outputs
func (e *Executor) GetOutputs() map[string]interface{} {
	return e.outputs
}

// GetResults returns all step results
func (e *Executor) GetResults() []StepResult {
	return e.results
}

