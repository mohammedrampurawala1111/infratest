# ALB + EC2 Example

This example demonstrates deploying a complete application stack with:
- VPC with public subnets
- Multiple EC2 instances running a web application
- Application Load Balancer (ALB) distributing traffic
- Target group with health checks
- HTTP health check validation

## What it creates

- 1 VPC (10.0.0.0/16)
- 2 Public subnets
- 1 Internet Gateway
- 2 EC2 instances (configurable)
- 1 Application Load Balancer
- 1 Target Group
- Security groups for ALB and EC2

## Prerequisites

- Terraform installed
- AWS credentials configured (or use `--localstack` flag)
- For LocalStack: Docker running with LocalStack container

## Running with AWS

```bash
cd examples/alb-ec2
infratest run flow.yaml
```

## Running with LocalStack

First, start LocalStack:

```bash
docker run -d -p 4566:4566 localstack/localstack
```

Then run the test:

```bash
cd examples/alb-ec2
infratest run flow.yaml --localstack
```

## Expected Output

The test will:
1. Initialize and plan Terraform
2. Apply the infrastructure (VPC, subnets, ALB, EC2 instances)
3. Validate all resources exist with correct attributes
4. Wait for EC2 instances to be healthy
5. Perform HTTP health check on ALB endpoint
6. Generate HTML and JSON reports
7. Clean up all resources

## Customization

Edit `terraform/variables.tf` to:
- Change AWS region
- Adjust number of EC2 instances (`instance_count`)
- Change instance type (`instance_type`)

## Notes

- The EC2 instances run a simple Python HTTP server on port 80
- Health check endpoint: `/health`
- The test waits up to 2.5 minutes (10 retries × 15s) for instances to be ready
- With LocalStack, some features may have limitations

