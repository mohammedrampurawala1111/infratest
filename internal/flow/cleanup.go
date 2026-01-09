package flow

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/infratest/infratest/internal/ui"
)

// CleanupManager handles cleanup operations with signal handling
type CleanupManager struct {
	executor   *Executor
	ctx        context.Context
	cancel     context.CancelFunc
	cleanupCh  chan os.Signal
	timeout    time.Duration
	debug      bool
	interrupted bool
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(executor *Executor, timeout time.Duration, debug bool) *CleanupManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	cm := &CleanupManager{
		executor:  executor,
		ctx:       ctx,
		cancel:    cancel,
		cleanupCh: make(chan os.Signal, 1),
		timeout:   timeout,
		debug:     debug,
	}
	
	// Setup signal handling
	signal.Notify(cm.cleanupCh, os.Interrupt, syscall.SIGTERM)
	
	return cm
}

// Start starts the cleanup manager and sets up panic recovery
func (cm *CleanupManager) Start() {
	// Setup panic recovery
	defer cm.recoverPanic()
	
	// Monitor for signals in a goroutine
	go cm.monitorSignals()
}

// Stop stops the cleanup manager
func (cm *CleanupManager) Stop() {
	cm.cancel()
	signal.Stop(cm.cleanupCh)
}

// Context returns the context
func (cm *CleanupManager) Context() context.Context {
	return cm.ctx
}

// RunCleanup runs cleanup steps (steps with when: always)
func (cm *CleanupManager) RunCleanup() error {
	if cm.interrupted {
		ui.PrintWarning("\n⚠️  Cleanup triggered by interrupt (SIGINT/SIGTERM) — attempting destroy...")
		ui.PrintWarning(fmt.Sprintf("   Cleanup timeout: %v", cm.timeout))
	} else {
		ui.PrintInfo("\n🧹 Running cleanup steps...")
		ui.PrintInfo(fmt.Sprintf("   Cleanup timeout: %v", cm.timeout))
	}
	
	// Create a context with timeout for cleanup
	cleanupCtx, cancel := context.WithTimeout(context.Background(), cm.timeout)
	defer cancel()
	
	// Find and execute cleanup steps
	flow := cm.executor.GetFlow()
	stepMap := make(map[string]*Step)
	for i := range flow.Steps {
		stepMap[flow.Steps[i].Name] = &flow.Steps[i]
	}
	
	executed := make(map[string]bool)
	results := cm.executor.GetResults()
	for _, result := range results {
		executed[result.StepName] = true
	}
	
	var cleanupSteps []*Step
	for i := range flow.Steps {
		if flow.Steps[i].When == "always" && !executed[flow.Steps[i].Name] {
			cleanupSteps = append(cleanupSteps, &flow.Steps[i])
		}
	}
	
	if len(cleanupSteps) == 0 {
		ui.PrintInfo("No cleanup steps to run")
		return nil
	}
	
	cleanupExecuted := 0
	var cleanupErrors []string
	
	for _, step := range cleanupSteps {
		// Check timeout before each step
		select {
		case <-cleanupCtx.Done():
			ui.PrintError("Cleanup timeout after %v", cm.timeout)
			cm.showManualDestroyInstructions(flow.WorkingDir, cleanupErrors)
			return fmt.Errorf("cleanup timeout after %v", cm.timeout)
		default:
		}
		
		// Calculate remaining time
		deadline, ok := cleanupCtx.Deadline()
		remainingTime := "unknown"
		if ok {
			remainingTime = time.Until(deadline).Round(time.Second).String()
		}
		ui.PrintInfo(fmt.Sprintf("  Running cleanup step: %s (timeout: %s remaining)", step.Name, remainingTime))
		
		// Execute cleanup step with timeout context
		err := cm.executor.ExecuteStepWithContext(cleanupCtx, *step, stepMap, executed)
		if err != nil {
			errorMsg := fmt.Sprintf("Cleanup step '%s' failed: %v", step.Name, err)
			ui.PrintError(errorMsg)
			cleanupErrors = append(cleanupErrors, errorMsg)
			// Continue with other cleanup steps
		} else {
			cleanupExecuted++
		}
		executed[step.Name] = true
	}
	
	if len(cleanupErrors) > 0 {
		ui.PrintWarning(fmt.Sprintf("\n⚠️  Cleanup completed with %d error(s)", len(cleanupErrors)))
		cm.showManualDestroyInstructions(flow.WorkingDir, cleanupErrors)
		return fmt.Errorf("cleanup failed: %d step(s) failed", len(cleanupErrors))
	}
	
	if cleanupExecuted > 0 {
		ui.PrintSuccess(fmt.Sprintf("✓ Cleanup completed successfully (%d step(s))", cleanupExecuted))
	}
	
	return nil
}

// showManualDestroyInstructions shows instructions for manual cleanup
func (cm *CleanupManager) showManualDestroyInstructions(workingDir string, errors []string) {
	fmt.Println()
	ui.PrintWarning("═══════════════════════════════════════════════════════════")
	ui.PrintWarning("⚠️  CLEANUP FAILED - Manual intervention required")
	ui.PrintWarning("═══════════════════════════════════════════════════════════")
	fmt.Println()
	
	if len(errors) > 0 {
		ui.PrintFailure("Failed cleanup steps:")
		for i, err := range errors {
			fmt.Printf("  %d. %s\n", i+1, err)
		}
		fmt.Println()
	}
	
	ui.PrintInfo("To manually destroy resources, run:")
	fmt.Printf("  cd %s\n", workingDir)
	fmt.Printf("  terraform destroy -auto-approve\n")
	fmt.Println()
	
	ui.PrintInfo("Or if using LocalStack:")
	fmt.Printf("  cd %s\n", workingDir)
	fmt.Printf("  AWS_ENDPOINT_URL=http://localhost:4566 terraform destroy -auto-approve\n")
	fmt.Println()
	
	ui.PrintWarning("═══════════════════════════════════════════════════════════")
}

func (cm *CleanupManager) monitorSignals() {
	select {
	case sig := <-cm.cleanupCh:
		cm.interrupted = true
		sigName := "SIGINT"
		if sig == syscall.SIGTERM {
			sigName = "SIGTERM"
		}
		ui.PrintWarning(fmt.Sprintf("\n⚠️  Received signal: %s (%v)", sigName, sig))
		ui.PrintWarning("Cancelling operations and running cleanup...")
		cm.cancel()
		
		// Run cleanup with timeout
		if err := cm.RunCleanup(); err != nil {
			ui.PrintError("Cleanup failed: %v", err)
			// Manual instructions already shown in RunCleanup
		}
		
		os.Exit(130) // Standard exit code for SIGINT
	case <-cm.ctx.Done():
		return
	}
}

func (cm *CleanupManager) recoverPanic() {
	if r := recover(); r != nil {
		ui.PrintError("⚠️  Panic occurred: %v", r)
		
		if cm.debug {
			// Print stack trace in debug mode
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			fmt.Fprintf(os.Stderr, "\nStack trace:\n%s\n", buf[:n])
		}
		
		// Attempt cleanup
		ui.PrintWarning("Attempting cleanup after panic...")
		if err := cm.RunCleanup(); err != nil {
			ui.PrintError("Cleanup after panic failed: %v", err)
		}
		
		panic(r) // Re-panic to maintain original behavior
	}
}

