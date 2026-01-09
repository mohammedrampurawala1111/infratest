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
		ui.PrintWarning("\n⚠️  Interrupt received. Running cleanup steps...")
	} else {
		ui.PrintInfo("\n🧹 Running cleanup steps...")
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
	
	cleanupExecuted := 0
	for _, step := range flow.Steps {
		if step.When == "always" && !executed[step.Name] {
			select {
			case <-cleanupCtx.Done():
				return fmt.Errorf("cleanup timeout after %v", cm.timeout)
			default:
			}
			
			ui.PrintInfo(fmt.Sprintf("  Running cleanup step: %s", step.Name))
			// Create a temporary executor context for cleanup
			// We need to execute the step, so we'll use the executor's internal method
			// For now, we'll execute it directly through the flow executor
			err := cm.executor.ExecuteStepWithContext(cleanupCtx, step, stepMap, executed)
			if err != nil {
				ui.PrintError("Cleanup step %s failed: %v", step.Name, err)
				// Continue with other cleanup steps
			} else {
				cleanupExecuted++
			}
			executed[step.Name] = true
		}
	}
	
	if cleanupExecuted > 0 {
		ui.PrintSuccess(fmt.Sprintf("✓ Cleanup completed (%d step(s))", cleanupExecuted))
	} else {
		ui.PrintInfo("No cleanup steps to run")
	}
	
	return nil
}

func (cm *CleanupManager) monitorSignals() {
	select {
	case sig := <-cm.cleanupCh:
		cm.interrupted = true
		ui.PrintWarning(fmt.Sprintf("\n⚠️  Received signal: %v", sig))
		ui.PrintWarning("Cancelling operations and running cleanup...")
		cm.cancel()
		
		// Run cleanup
		if err := cm.RunCleanup(); err != nil {
			ui.PrintError("Cleanup failed: %v", err)
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

