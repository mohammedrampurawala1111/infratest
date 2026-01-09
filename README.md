# Infratest

![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8?style=flat-square&logo=go)
![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)
![Terraform](https://img.shields.io/badge/terraform-1.0+-623CE4?style=flat-square&logo=terraform)

> A declarative YAML-driven E2E infrastructure testing framework for Terraform

Infratest validates infrastructure deployments by running Terraform commands, validating resource state, performing runtime checks, and generating comprehensive reports—all defined in simple YAML files.

## ✨ Features

- 🚀 **Declarative YAML flows** - Define tests in simple, readable YAML
- 🔍 **Advanced inventory validation** - Check resource counts, attributes, and wildcard matching
- 🌐 **HTTP health checks** - Validate endpoints with retry logic
- 📊 **Rich reporting** - HTML and JSON reports with outputs and resource diffs
- 🎨 **Beautiful console output** - Colored progress indicators and clear error messages
- 🛡️ **Signal handling** - Graceful cleanup on interrupt (SIGINT/SIGTERM)
- 🐳 **LocalStack support** - Test locally without AWS costs
- 🔧 **Output interpolation** - Use Terraform outputs in URLs and commands

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/infratest/infratest.git
cd infratest

# Build
go build -o infratest .

# Or install directly
go install github.com/infratest/infratest@latest
```

### Prerequisites

- **Go 1.21+** - For building from source
- **Terraform 1.0+** - Must be installed and in your PATH
- **AWS credentials** - Configured via environment variables or AWS CLI (unless using LocalStack)

### Your First Test (5 minutes)

```bash
# 1. Try the simple VPC example
cd examples/simple-vpc
infratest run flow.yaml

# 2. Or use LocalStack (no AWS account needed)
docker run -d -p 4566:4566 localstack/localstack
infratest run flow.yaml --localstack
```

That's it! The tool will:
- ✅ Deploy infrastructure
- ✅ Validate resources
- ✅ Generate reports
- ✅ Clean up automatically

## 📖 Usage

```bash
infratest run path/to/flow.yaml [flags]
```

### Flags

- `--debug` - Enable verbose debug output
- `--localstack` - Use LocalStack instead of real AWS
- `--localstack-endpoint URL` - Override LocalStack endpoint (default: http://localhost:4566)
- `--cleanup-timeout duration` - Timeout for cleanup operations (default: 5m)

### Example Output

```
📋 Flow: simple-vpc
   Deploy a simple VPC with subnets and internet gateway
📁 Working directory: ./terraform
📊 Steps: 4

🚀 Starting flow execution...

Step 1/4: init-and-plan ... [OK] 12s
Step 2/4: apply ... [OK] 45s
Step 3/4: inventory-check ... [OK] 2s
Step 4/4: destroy ... [OK] 18s

📄 Generating reports...

✅ Flow executed successfully!
```

**Screenshot/Video**: Add a screenshot of the console output and HTML report to `docs/screenshots/` directory.

## 🐳 LocalStack Usage

Test your infrastructure locally without AWS costs!

### Start LocalStack

**Option 1: Docker (Recommended)**
```bash
docker run -d -p 4566:4566 localstack/localstack
```

**Option 2: LocalStack CLI**
```bash
localstack start
```

### Run Tests with LocalStack

```bash
infratest run flow.yaml --localstack
```

### Custom Endpoint

```bash
infratest run flow.yaml --localstack --localstack-endpoint http://localhost:4566
```

### What LocalStack Provides

- ✅ Full AWS API compatibility
- ✅ No AWS costs
- ✅ Fast iteration
- ✅ Offline development
- ⚠️ Some advanced features may have limitations

## 📝 YAML Flow Syntax

### Basic Example

```yaml
name: my-test
description: Test my infrastructure

working_dir: ./terraform

environment:
  provider: aws

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
    expected_resources:
      aws_vpc.main:
        count: 1
        attributes:
          cidr_block: "10.0.0.0/16"
      aws_subnet.public:
        min_count: 2
        max_count: 3

  - name: http-health-check
    type: http
    url: "http://${output.alb_dns}/health"
    expected_status: 200
    retries: 8
    delay: 10s

  - name: destroy
    type: terraform
    command: terraform destroy -auto-approve
    when: always

reporting:
  output: ./reports/${module}/${name}-$(date +%Y%m%d-%H%M%S).html
  formats: [html, json]
```

## 🔧 Step Types

### terraform

Execute Terraform commands. Supports both single `command` and multiple `commands`.

```yaml
- name: apply
  type: terraform
  command: terraform apply -auto-approve
```

### terraform-inventory

Validate resources with advanced matching:

**Legacy Format** (still supported):
```yaml
- name: inventory-check
  type: terraform-inventory
  expected:
    resources:
      - type: aws_vpc
        min_count: 1
        max_count: 1
```

**Advanced Format** (recommended):
```yaml
- name: inventory-check
  type: terraform-inventory
  expected_resources:
    aws_vpc.main:                    # Exact name match
      count: 1
      attributes:
        cidr_block: "10.0.0.0/16"
        tags.Name: "my-vpc"
    aws_subnet.*:                    # Wildcard matching
      min_count: 2
      max_count: 4
      attributes:
        map_public_ip_on_launch: true
```

### http

Perform HTTP health checks with retry logic:

```yaml
- name: health-check
  type: http
  url: "http://${output.alb_dns}/health"
  expected_status: 200
  retries: 8
  delay: 10s
```

## 🎯 Advanced Features

### Output Interpolation

Use Terraform outputs in URLs and commands:

```yaml
# Simple output
url: "http://${output.alb_dns}/health"

# Array access
url: "http://${output.instance_ips[0]}:3000"

# Nested paths
url: "http://${output.config.database.host}:5432"
```

### Step Conditions

Control when steps execute:

- `always` - Always execute (useful for cleanup)
- `on-success` - Only if previous steps succeeded
- `on-failure` - Only if a previous step failed

```yaml
- name: cleanup
  type: terraform
  command: terraform destroy -auto-approve
  when: always  # Always run, even on failure
```

### Module-wise Reports

Reports are automatically organized by module:

```
reports/
├── vpc/
│   └── simple-vpc-20260109-161308.html
└── ec2/
    └── ec2-test-20260109-162300.html
```

## 📚 Examples

We provide complete, ready-to-run examples:

### 1. Simple VPC (`examples/simple-vpc/`)

Basic VPC with subnets and internet gateway.

```bash
cd examples/simple-vpc
infratest run flow.yaml
```

### 2. ALB + EC2 (`examples/alb-ec2/`)

Complete application stack with load balancer and EC2 instances.

```bash
cd examples/alb-ec2
infratest run flow.yaml
```

### 3. EC2 Test (`examples/ec2-test.yaml`)

EC2 instance with web server and health checks.

```bash
infratest run examples/ec2-test.yaml
```

Each example includes:
- ✅ Complete Terraform code
- ✅ Flow YAML file
- ✅ README with instructions
- ✅ Works with both AWS and LocalStack

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test ./internal/flow/...
```

## 🛠️ Development

```bash
# Build
go build -o infratest .

# Run with debug
./infratest run examples/simple-vpc/flow.yaml --debug

# Format code
go fmt ./...

# Lint (if you have golangci-lint)
golangci-lint run
```

## 📊 Report Features

HTML reports include:
- ✅ Flow summary (steps, duration, success/failure)
- ✅ Terraform outputs table
- ✅ Step-by-step results with colored status
- ✅ Resource inventory
- ✅ Error details with full output

JSON reports provide machine-readable format for CI/CD integration.

## 🤝 Contributing

Contributions are welcome! Here's how you can help:

1. **Report bugs** - Open an issue with details
2. **Suggest features** - Share your ideas
3. **Submit PRs** - Fix bugs or add features
4. **Improve docs** - Help others learn

### Development Guidelines

- Follow Go best practices
- Add tests for new features
- Update documentation
- Keep commits focused and clear

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI
- Inspired by [Terratest](https://terratest.gruntwork.io/)
- Uses [LocalStack](https://localstack.cloud/) for local AWS testing

---

**Ready to test your infrastructure?** Start with `examples/simple-vpc/` and see how easy it is! 🚀
