package reporting

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// FlowInfo contains flow metadata for reporting
type FlowInfo struct {
	Name        string
	Description string
	WorkingDir  string
}

// StepResultInfo contains step result data for reporting
type StepResultInfo struct {
	StepName   string
	StepType   string
	Success    bool
	Output     string
	Error      error
	Duration   time.Duration
	Resources  []ResourceInfo
	HTTPStatus int
}

// ResourceInfo contains resource data for reporting
type ResourceInfo struct {
	Type string
	ID   string
}

// GenerateHTMLReport creates an HTML report
func GenerateHTMLReport(f FlowInfo, results []StepResultInfo, outputPath string, outputs map[string]interface{}) error {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Infratest Report - ` + f.Name + `</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #4CAF50; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .summary { background: #f9f9f9; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .step { margin: 15px 0; padding: 15px; border-left: 4px solid #ddd; background: #fafafa; border-radius: 4px; }
        .step.success { border-left-color: #4CAF50; }
        .step.failure { border-left-color: #f44336; }
        .step-header { font-weight: bold; font-size: 1.1em; margin-bottom: 10px; }
        .step-type { color: #666; font-size: 0.9em; }
        .step-duration { color: #888; font-size: 0.85em; }
        .error { color: #f44336; background: #ffebee; padding: 10px; border-radius: 4px; margin-top: 10px; }
        .output { background: #263238; color: #aed581; padding: 10px; border-radius: 4px; font-family: monospace; font-size: 0.9em; overflow-x: auto; margin-top: 10px; }
        .resources { margin-top: 10px; }
        .resource { display: inline-block; background: #e3f2fd; padding: 5px 10px; margin: 5px; border-radius: 3px; font-size: 0.9em; }
        .status-badge { display: inline-block; padding: 3px 8px; border-radius: 3px; font-size: 0.85em; font-weight: bold; margin-left: 10px; }
        .status-success { background: #4CAF50; color: white; }
        .status-failure { background: #f44336; color: white; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Infratest Report: ` + escapeHTML(f.Name) + `</h1>
        <div class="summary">
            <p><strong>Description:</strong> ` + escapeHTML(f.Description) + `</p>
            <p><strong>Working Directory:</strong> ` + escapeHTML(f.WorkingDir) + `</p>
            <p><strong>Generated:</strong> ` + time.Now().Format(time.RFC3339) + `</p>
        </div>
`

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

	html += fmt.Sprintf(`
        <h2>Summary</h2>
        <div class="summary">
            <p><strong>Total Steps:</strong> %d</p>
            <p><strong>Successful:</strong> <span style="color: #4CAF50;">%d</span></p>
            <p><strong>Failed:</strong> <span style="color: #f44336;">%d</span></p>
            <p><strong>Total Duration:</strong> %s</p>
        </div>
        <h2>Terraform Outputs</h2>
`, len(results), successCount, failureCount, totalDuration.Round(time.Millisecond))
	
	// Add outputs table if available
	if outputs != nil && len(outputs) > 0 {
		html += `        <div class="summary">
            <table style="width: 100%%; border-collapse: collapse;">
                <thead>
                    <tr style="background: #f0f0f0;">
                        <th style="padding: 10px; text-align: left; border: 1px solid #ddd;">Output Name</th>
                        <th style="padding: 10px; text-align: left; border: 1px solid #ddd;">Value</th>
                    </tr>
                </thead>
                <tbody>
`
		for key, val := range outputs {
			valueStr := formatOutputValue(val)
			html += fmt.Sprintf(`                    <tr>
                        <td style="padding: 10px; border: 1px solid #ddd; font-weight: bold;">%s</td>
                        <td style="padding: 10px; border: 1px solid #ddd; font-family: monospace;">%s</td>
                    </tr>
`, escapeHTML(key), escapeHTML(valueStr))
		}
		html += `                </tbody>
            </table>
        </div>
`
	} else {
		html += `        <div class="summary">
            <p><em>No outputs available</em></p>
        </div>
`
	}

	html += `
        <h2>Step Results</h2>
`

	// Add step results
	for _, result := range results {
		statusClass := "success"
		statusBadge := `<span class="status-badge status-success">SUCCESS</span>`
		if !result.Success {
			statusClass = "failure"
			statusBadge = `<span class="status-badge status-failure">FAILED</span>`
		}

		html += fmt.Sprintf(`
        <div class="step %s">
            <div class="step-header">
                %s %s
            </div>
            <div class="step-type">Type: %s</div>
            <div class="step-duration">Duration: %s</div>
`, statusClass, escapeHTML(result.StepName), statusBadge, escapeHTML(result.StepType), result.Duration.Round(time.Millisecond))

		if result.Error != nil {
			html += fmt.Sprintf(`            <div class="error">Error: %s</div>`, escapeHTML(result.Error.Error()))
		} else if !result.Success {
			html += fmt.Sprintf(`            <div class="error">Step failed</div>`)
		}

		if result.Output != "" {
			html += fmt.Sprintf(`            <div class="output">%s</div>`, escapeHTML(result.Output))
		}

		if len(result.Resources) > 0 {
			html += `            <div class="resources">`
			for _, r := range result.Resources {
				html += fmt.Sprintf(`<span class="resource">%s: %s</span>`, escapeHTML(r.Type), escapeHTML(r.ID))
			}
			html += `            </div>`
		}

		if result.HTTPStatus > 0 {
			html += fmt.Sprintf(`            <div>HTTP Status: %d</div>`, result.HTTPStatus)
		}

		html += `        </div>`
	}

	html += `
    </div>
</body>
</html>`

	return os.WriteFile(outputPath, []byte(html), 0644)
}

func formatOutputValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = fmt.Sprintf("%v", item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		parts := make([]string, 0)
		for k, v := range v {
			parts = append(parts, fmt.Sprintf("%s: %v", k, v))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func escapeHTML(s string) string {
	var escaped strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			escaped.WriteString("&lt;")
		case '>':
			escaped.WriteString("&gt;")
		case '&':
			escaped.WriteString("&amp;")
		case '"':
			escaped.WriteString("&quot;")
		case '\'':
			escaped.WriteString("&#39;")
		default:
			escaped.WriteRune(r)
		}
	}
	return escaped.String()
}

