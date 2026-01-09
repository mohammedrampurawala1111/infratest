# LocalStack Example

This example demonstrates how to use **infratest** with **LocalStack** to test infrastructure without an AWS account or any costs.

## What This Example Shows

- ✅ How to run infrastructure tests locally with LocalStack
- ✅ No AWS credentials or account needed
- ✅ Fast, free testing environment
- ✅ Same Terraform code works with both LocalStack and real AWS

## Prerequisites

- Terraform installed
- Docker installed (for running LocalStack)
- **No AWS account or credentials needed!**

## Quick Start

### 1. Start LocalStack

```bash
docker run -d -p 4566:4566 localstack/localstack
```

Verify it's running:
```bash
curl http://localhost:4566/_localstack/health
```

### 2. Run the Test

```bash
cd examples/localstack-example
infratest run flow.yaml --localstack
```

That's it! The `--localstack` flag automatically:
- Sets `AWS_ENDPOINT_URL=http://localhost:4566`
- Configures test credentials
- Suppresses cost warnings
- Validates LocalStack is running

## What Gets Created

- 1 VPC (10.0.0.0/16)
- 2 Public subnets (10.0.1.0/24, 10.0.2.0/24)
- 1 Internet Gateway
- Route table and associations

## Custom Endpoint

If LocalStack is running on a different port:

**Via Flag:**
```bash
infratest run flow.yaml --localstack --localstack-endpoint http://localhost:5000
```

**Via YAML:**
```yaml
environment:
  provider: aws
  endpoint: "http://localhost:5000"  # Overrides flag
```

## Expected Output

```
📋 Flow: localstack-vpc-test
   Test VPC deployment using LocalStack (no AWS account needed)
📁 Working directory: ./terraform
📊 Steps: 4

🔧 LocalStack mode enabled (endpoint: http://localhost:4566)

🚀 Starting flow execution...

Step 1/4: init-and-plan ... [OK] 8s
Step 2/4: apply ... [OK] 12s
Step 3/4: inventory-check ... [OK] 1s
Step 4/4: destroy ... [OK] 5s

📄 Generating reports...

✅ Flow executed successfully!
```

## Benefits of Using LocalStack

- 💰 **No costs** - Test freely without AWS charges
- ⚡ **Fast** - Local execution is much faster than real AWS
- 🔒 **Safe** - No risk of affecting production resources
- 🚀 **CI/CD friendly** - Run tests in containers without AWS setup
- 🧪 **Offline** - Work without internet connection

## Troubleshooting

**LocalStack not detected:**
```
⚠️  LocalStack not detected at http://localhost:4566
```

Make sure LocalStack is running:
```bash
docker ps | grep localstack
docker logs <container-id>
```

**Different port:**
```bash
infratest run flow.yaml --localstack --localstack-endpoint http://localhost:5000
```

**Terraform errors:**
If you see errors about missing services, make sure LocalStack has the required services enabled:
```bash
docker run -d -p 4566:4566 \
  -e SERVICES=ec2,s3,vpc,iam,sts \
  localstack/localstack
```

## Next Steps

- Try modifying the Terraform code and re-running the test
- Experiment with different resources
- Compare execution time with real AWS (when you have credentials)
- Use this as a template for your own LocalStack tests

## See Also

- [Main README](../../README.md) - Full documentation
- [Simple VPC Example](../simple-vpc/) - Example without LocalStack
- [LocalStack Documentation](https://docs.localstack.cloud/)

