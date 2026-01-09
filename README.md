# Infratest

A declarative YAML-driven E2E infrastructure testing framework for Terraform.

## Overview

Infratest validates infrastructure deployments by:
- Running Terraform commands in user-provided directories
- Capturing and parsing Terraform state for resource inventory
- Validating completeness (expected vs actual resources)
- Running basic runtime checks (HTTP health checks)
- Cleaning up reliably
- Producing simple reports (HTML and JSON)

## Prerequisites

- Go 1.21 or later
- Terraform installed and available in your PATH

**Important**: Infratest does NOT bundle or install Terraform. You must have Terraform installed separately. If Terraform is not found in your PATH, Infratest will fail early with a clear error message.

## Installation

```bash
go build -o infratest .
```

Or install directly:

```bash
go install github.com/infratest/infratest@latest
```

## Usage

```bash
infratest run path/to/flow.yaml [--debug] [--localstack]
```

### Flags

- `--debug`: Enable debug output
- `--localstack`: Use LocalStack for AWS (development) - currently a placeholder for future implementation

## YAML Flow Syntax

```yaml
name: basic-vpc-test
description: Deploy simple VPC and verify resources

working_dir: ./terraform/vpc   # relative or absolute path where terraform files live

environment:
  provider: aws
  # No credentials here — use existing AWS SDK defaults / env vars

steps:
  - name: init-and-plan
    type: terraform
    commands:
      - terraform init
      - terraform plan -out=plan.tfplan

  - name: apply
    type: terraform
    command: terraform apply -auto-approve plan.tfplan

  - name: inventory-check
    type: terraform-inventory
    after: apply
    expected:
      resources:
        - type: aws_vpc
          min_count: 1
          max_count: 1
        - type: aws_subnet
          min_count: 2
        - type: aws_internet_gateway
          min_count: 1
    fail_on_extra: true
    fail_on_missing: true

  - name: http-health-check
    type: http
    url: "http://${output.alb_dns}/health"   # support terraform output interpolation
    expected_status: 200
    retries: 8
    delay: 10s

  - name: destroy
    type: terraform
    command: terraform destroy -auto-approve
    when: always   # always, on-success, on-failure

reporting:
  output: ./reports/${name}-$(date +%Y%m%d-%H%M%S).html
  formats: [html, json]
```

## Step Types

### terraform

Executes Terraform commands. Supports both `command` (single command) and `commands` (multiple commands executed sequentially).

```yaml
- name: apply
  type: terraform
  command: terraform apply -auto-approve
```

### terraform-inventory

Validates that the expected resources exist in Terraform state.

```yaml
- name: inventory-check
  type: terraform-inventory
  after: apply
  expected:
    resources:
      - type: aws_vpc
        min_count: 1
        max_count: 1
  fail_on_extra: true
  fail_on_missing: true
```

### http

Performs HTTP health checks with retry logic.

```yaml
- name: http-health-check
  type: http
  url: "http://${output.alb_dns}/health"
  expected_status: 200
  retries: 8
  delay: 10s
```

## Step Conditions

Use the `when` field to control when steps execute:

- `always`: Always execute, even if previous steps failed
- `on-success`: Only execute if all previous steps succeeded
- `on-failure`: Only execute if a previous step failed

## Output Interpolation

Terraform outputs can be interpolated in commands and URLs using `${output.KEY}` syntax:

```yaml
url: "http://${output.alb_dns}/health"
```

## Reporting

Reports are generated in HTML and/or JSON format. The output path supports:
- `${name}` - Flow name
- `$(date +%Y%m%d-%H%M%S)` - Current date/time

## Examples

See `examples/` directory for example flow files.

## Development

```bash
# Run tests
go test ./...

# Build
go build -o infratest .

# Run with debug
./infratest run examples/basic-vpc.yaml --debug
```

## License

MIT

