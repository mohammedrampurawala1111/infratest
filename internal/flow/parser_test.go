package flow

import (
	"os"
	"testing"
)

func TestParseFlow(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `name: test-flow
description: Test flow
working_dir: ./terraform
environment:
  provider: aws
steps:
  - name: init
    type: terraform
    command: terraform init
reporting:
  output: ./reports/test.html
  formats: [html]
`

	tmpFile, err := os.CreateTemp("", "test-flow-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the flow
	flow, err := ParseFlow(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse flow: %v", err)
	}

	// Validate parsed content
	if flow.Name != "test-flow" {
		t.Errorf("Expected name 'test-flow', got '%s'", flow.Name)
	}

	if flow.Description != "Test flow" {
		t.Errorf("Expected description 'Test flow', got '%s'", flow.Description)
	}

	if flow.WorkingDir != "./terraform" {
		t.Errorf("Expected working_dir './terraform', got '%s'", flow.WorkingDir)
	}

	if flow.Environment.Provider != "aws" {
		t.Errorf("Expected provider 'aws', got '%s'", flow.Environment.Provider)
	}

	if len(flow.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(flow.Steps))
	}

	if flow.Steps[0].Name != "init" {
		t.Errorf("Expected step name 'init', got '%s'", flow.Steps[0].Name)
	}

	if flow.Steps[0].Type != "terraform" {
		t.Errorf("Expected step type 'terraform', got '%s'", flow.Steps[0].Type)
	}

	if flow.Reporting.Output != "./reports/test.html" {
		t.Errorf("Expected output './reports/test.html', got '%s'", flow.Reporting.Output)
	}

	if len(flow.Reporting.Formats) != 1 || flow.Reporting.Formats[0] != "html" {
		t.Errorf("Expected format [html], got %v", flow.Reporting.Formats)
	}
}

func TestParseFlowWithAdvancedInventory(t *testing.T) {
	yamlContent := `name: advanced-test
working_dir: ./terraform
environment:
  provider: aws
steps:
  - name: inventory-check
    type: terraform-inventory
    expected_resources:
      aws_vpc.main:
        count: 1
        attributes:
          cidr_block: "10.0.0.0/16"
      aws_subnet.*:
        min_count: 2
reporting:
  output: ./reports/test.html
  formats: [html]
`

	tmpFile, err := os.CreateTemp("", "test-advanced-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	flow, err := ParseFlow(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse flow: %v", err)
	}

	if len(flow.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(flow.Steps))
	}

	step := flow.Steps[0]
	if len(step.ExpectedResources) != 2 {
		t.Fatalf("Expected 2 expected resources, got %d", len(step.ExpectedResources))
	}

	// Check aws_vpc.main
	vpcMatch, exists := step.ExpectedResources["aws_vpc.main"]
	if !exists {
		t.Fatal("Expected 'aws_vpc.main' in expected_resources")
	}
	if vpcMatch.Count == nil || *vpcMatch.Count != 1 {
		t.Errorf("Expected count 1, got %v", vpcMatch.Count)
	}
	if vpcMatch.Attributes["cidr_block"] != "10.0.0.0/16" {
		t.Errorf("Expected cidr_block '10.0.0.0/16', got %v", vpcMatch.Attributes["cidr_block"])
	}

	// Check aws_subnet.*
	subnetMatch, exists := step.ExpectedResources["aws_subnet.*"]
	if !exists {
		t.Fatal("Expected 'aws_subnet.*' in expected_resources")
	}
	if subnetMatch.MinCount == nil || *subnetMatch.MinCount != 2 {
		t.Errorf("Expected min_count 2, got %v", subnetMatch.MinCount)
	}
}

func TestValidateFlow(t *testing.T) {
	tests := []struct {
		name    string
		flow    *Flow
		wantErr bool
	}{
		{
			name: "valid flow",
			flow: &Flow{
				Name:       "test",
				WorkingDir: "./terraform",
				Steps:      []Step{{Name: "test", Type: "terraform"}},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			flow: &Flow{
				WorkingDir: "./terraform",
				Steps:      []Step{{Name: "test", Type: "terraform"}},
			},
			wantErr: true,
		},
		{
			name: "missing working_dir",
			flow: &Flow{
				Name:  "test",
				Steps: []Step{{Name: "test", Type: "terraform"}},
			},
			wantErr: true,
		},
		{
			name: "no steps",
			flow: &Flow{
				Name:       "test",
				WorkingDir: "./terraform",
				Steps:      []Step{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlow(tt.flow)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFlow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

