package reporting

import (
	"encoding/json"
	"os"
	"time"
)

// Report represents the complete test report
type Report struct {
	Flow      FlowInfo       `json:"flow"`
	Summary   Summary        `json:"summary"`
	Steps     []StepReport   `json:"steps"`
	Generated time.Time      `json:"generated"`
}

// Summary contains test summary
type Summary struct {
	TotalSteps    int           `json:"total_steps"`
	Successful    int           `json:"successful"`
	Failed        int           `json:"failed"`
	TotalDuration time.Duration `json:"total_duration"`
}

// StepReport represents a step result in the report
type StepReport struct {
	Name      string        `json:"name"`
	Type      string        `json:"type"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	Output    string        `json:"output,omitempty"`
	Resources []Resource    `json:"resources,omitempty"`
	HTTPStatus int          `json:"http_status,omitempty"`
}

// Resource represents a resource in the report
type Resource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// GenerateJSONReport creates a JSON report
func GenerateJSONReport(f FlowInfo, results []StepResultInfo, outputPath string) error {
	// Calculate summary
	successCount := 0
	failureCount := 0
	totalDuration := time.Duration(0)
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failureCount++
		}
		totalDuration += r.Duration
	}

	// Convert results
	stepReports := make([]StepReport, len(results))
	for i, r := range results {
		sr := StepReport{
			Name:     r.StepName,
			Type:     r.StepType,
			Success:  r.Success,
			Duration: r.Duration,
			Output:   r.Output,
		}

		if r.Error != nil {
			sr.Error = r.Error.Error()
		}

		if len(r.Resources) > 0 {
			sr.Resources = make([]Resource, len(r.Resources))
			for j, res := range r.Resources {
				sr.Resources[j] = Resource{
					Type: res.Type,
					ID:   res.ID,
				}
			}
		}

		if r.HTTPStatus > 0 {
			sr.HTTPStatus = r.HTTPStatus
		}

		stepReports[i] = sr
	}

	report := Report{
		Flow: FlowInfo{
			Name:        f.Name,
			Description: f.Description,
			WorkingDir:  f.WorkingDir,
		},
		Summary: Summary{
			TotalSteps:    len(results),
			Successful:    successCount,
			Failed:        failureCount,
			TotalDuration: totalDuration,
		},
		Steps:     stepReports,
		Generated: time.Now(),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

