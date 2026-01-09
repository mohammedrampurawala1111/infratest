package flow

import "time"

// Flow represents the complete test flow configuration
type Flow struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	WorkingDir  string      `yaml:"working_dir"`
	Environment Environment `yaml:"environment"`
	Steps       []Step      `yaml:"steps"`
	Reporting   Reporting   `yaml:"reporting"`
}

// Environment configuration
type Environment struct {
	Provider string `yaml:"provider"`
}

// Step represents a single step in the flow
type Step struct {
	Name    string            `yaml:"name"`
	Type    string            `yaml:"type"`
	After   string            `yaml:"after,omitempty"`
	When    string            `yaml:"when,omitempty"` // always, on-success, on-failure
	Command string            `yaml:"command,omitempty"`
	Commands []string         `yaml:"commands,omitempty"`
	
	// Terraform inventory step fields (legacy format)
	Expected       *ExpectedResources `yaml:"expected,omitempty"`
	FailOnExtra    bool               `yaml:"fail_on_extra,omitempty"`
	FailOnMissing  bool               `yaml:"fail_on_missing,omitempty"`
	
	// Advanced inventory format (new)
	ExpectedResources map[string]ResourceMatchConfig `yaml:"expected_resources,omitempty"`
	
	// HTTP step fields
	URL            string        `yaml:"url,omitempty"`
	ExpectedStatus int          `yaml:"expected_status,omitempty"`
	Retries        int          `yaml:"retries,omitempty"`
	Delay          string       `yaml:"delay,omitempty"`
}

// ExpectedResources defines what resources should exist
type ExpectedResources struct {
	Resources []ExpectedResource `yaml:"resources"`
}

// ExpectedResource defines expected resource type and count
type ExpectedResource struct {
	Type     string `yaml:"type"`
	MinCount int    `yaml:"min_count,omitempty"`
	MaxCount int    `yaml:"max_count,omitempty"`
}

// ResourceMatchConfig defines advanced resource matching configuration (for new inventory format)
type ResourceMatchConfig struct {
	Count      *int                  `yaml:"count,omitempty"`
	MinCount   *int                  `yaml:"min_count,omitempty"`
	MaxCount   *int                  `yaml:"max_count,omitempty"`
	Attributes map[string]interface{} `yaml:"attributes,omitempty"`
}

// Reporting configuration
type Reporting struct {
	Output  string   `yaml:"output"`
	Formats []string `yaml:"formats"`
}

// StepResult represents the result of executing a step
type StepResult struct {
	StepName   string
	StepType   string
	Success    bool
	Output     string
	Error      error
	Duration   time.Duration
	Resources  []Resource
	HTTPStatus int
}

// Resource represents a Terraform resource
type Resource struct {
	Type string
	ID   string
}

