package ui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	// Colors for different message types
	StepName   = color.New(color.FgBlue, color.Bold)
	Success    = color.New(color.FgGreen, color.Bold)
	Failure    = color.New(color.FgRed, color.Bold)
	Warning    = color.New(color.FgYellow)
	Info       = color.New(color.FgCyan)
	Debug      = color.New(color.FgMagenta)
	Timestamp  = color.New(color.FgHiBlack)
	
	// Status indicators
	OK    = color.New(color.FgGreen).Sprint("✓")
	FAIL  = color.New(color.FgRed).Sprint("✗")
	SKIP  = color.New(color.FgYellow).Sprint("⊘")
)

// PrintStep prints a step name with formatting
func PrintStep(stepNum, totalSteps int, stepName string) {
	StepName.Printf("Step %d/%d: %s", stepNum, totalSteps, stepName)
}

// PrintSuccess prints a success message
func PrintSuccess(message string) {
	Success.Println(message)
}

// PrintFailure prints a failure message
func PrintFailure(message string) {
	Failure.Println(message)
}

// PrintWarning prints a warning message
func PrintWarning(message string) {
	Warning.Println(message)
}

// PrintInfo prints an info message
func PrintInfo(message string) {
	Info.Println(message)
}

// PrintDebug prints a debug message (only if debug is enabled)
func PrintDebug(debug bool, format string, args ...interface{}) {
	if debug {
		Debug.Printf("[DEBUG] "+format, args...)
	}
}

// PrintError prints an error message to stderr
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s %s\n", FAIL, Failure.Sprintf(format, args...))
}

// PrintProgress prints progress with status and duration
func PrintProgress(stepNum, totalSteps int, stepName, status string, duration string) {
	StepName.Printf("Step %d/%d: %s", stepNum, totalSteps, stepName)
	fmt.Print(" [")
	if status == "OK" {
		Success.Print(status)
	} else if status == "FAIL" {
		Failure.Print(status)
	} else {
		fmt.Print(status)
	}
	fmt.Printf("] %s\n", duration)
}

// DisableColors disables all color output (useful for non-TTY)
func DisableColors() {
	color.NoColor = true
}

