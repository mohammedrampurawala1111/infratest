# Simple VPC Example

This example demonstrates deploying a basic VPC with public subnets and an internet gateway.

## What it creates

- 1 VPC (10.0.0.0/16)
- 2 Public subnets (10.0.1.0/24, 10.0.2.0/24)
- 1 Internet Gateway
- Route table and associations

## Prerequisites

- Terraform installed
- AWS credentials configured (or use `--localstack` flag)
- For LocalStack: Docker running with LocalStack container

## Running with AWS

```bash
cd examples/simple-vpc
infratest run flow.yaml
```

## Running with LocalStack

First, start LocalStack:

```bash
docker run -d -p 4566:4566 localstack/localstack
```

Then run the test:

```bash
cd examples/simple-vpc
infratest run flow.yaml --localstack
```

## Expected Output

The test will:
1. Initialize and plan Terraform
2. Apply the infrastructure
3. Validate resources exist with correct attributes
4. Generate HTML and JSON reports
5. Clean up resources

## Customization

Edit `terraform/variables.tf` to change the AWS region.

