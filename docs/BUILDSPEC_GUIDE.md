# BuildSpec Guide

This document describes the AWS CodeBuild buildspec files available in this project for building and deploying Lambda functions.

## Available BuildSpec Files

### 1. `buildspec.yml` - Docker/ECS Build
**Purpose**: Builds Docker images for ECS/container deployments

**Use Case**: 
- Building the backend Golang Lambda as a Docker container
- Deploying to ECS or ECR

**Environment Variables**:
- `SERVICE_NAME` - Name of the service (e.g., "ccoe-customer-contact-manager")
- `ECR_URL_BASE` - Base URL for ECR repository
- `AWS_DEFAULT_REGION` - AWS region for deployment

**Artifacts**: Docker images pushed to ECR

---

### 2. `buildspec-webhook.yml` - Webhook Lambda Build
**Purpose**: Builds the Typeform webhook Lambda function (Go)

**Use Case**:
- Building the webhook Lambda that receives Typeform survey responses
- Deploying webhook Lambda to AWS Lambda

**Environment Variables**:
- `LAMBDA_ARTIFACT_BUCKET` (optional) - S3 bucket for storing Lambda packages
- `WEBHOOK_FUNCTION_NAME` (optional) - Lambda function name for automatic deployment
- `AWS_DEFAULT_REGION` - AWS region for deployment

**Build Process**:
1. Builds Go binary for ARM64/Graviton architecture
2. Creates deployment ZIP package
3. Uploads to S3 (if bucket specified)
4. Updates Lambda function (if function name specified)

**Artifacts**:
- `ccoe-customer-contact-manager-webhook-lambda.zip`

**S3 Upload Paths** (if `LAMBDA_ARTIFACT_BUCKET` is set):
- `s3://$LAMBDA_ARTIFACT_BUCKET/webhook-lambda/ccoe-customer-contact-manager-webhook-lambda-{commit}.zip`
- `s3://$LAMBDA_ARTIFACT_BUCKET/webhook-lambda/ccoe-customer-contact-manager-webhook-lambda-latest.zip`

---

### 3. `buildspec-all-lambdas.yml` - All Lambda Functions Build
**Purpose**: Builds all Lambda functions in one build

**Use Case**:
- Complete CI/CD pipeline for all Lambda functions
- Deploying multiple Lambda functions at once
- Release builds

**Lambda Functions Built**:
1. **Backend Golang Lambda** (`main.go`) - Processes S3 events via SQS
2. **Webhook Golang Lambda** (`cmd/webhook`) - Receives Typeform webhooks
3. **Upload Node.js Lambda** (`lambda/upload_lambda`) - Frontend API with Function URL
4. **SAML Auth Lambda@Edge** (`lambda/saml_auth`) - CloudFront authentication

**Environment Variables**:
- `LAMBDA_ARTIFACT_BUCKET` (optional) - S3 bucket for storing Lambda packages
- `BACKEND_FUNCTION_NAME` (optional) - Backend Lambda function name
- `WEBHOOK_FUNCTION_NAME` (optional) - Webhook Lambda function name
- `UPLOAD_FUNCTION_NAME` (optional) - Upload Lambda function name
- `SAML_FUNCTION_NAME` (optional) - SAML Lambda@Edge function name
- `AWS_DEFAULT_REGION` - AWS region for deployment

**Build Process**:
1. Installs Go 1.21 and Node.js 18
2. Runs `make package-all-lambdas` to build all functions
3. Uploads all packages to S3 (if bucket specified)
4. Updates Lambda functions (if function names specified)

**Artifacts**:
- `ccoe-customer-contact-manager-lambda.zip` (Backend)
- `ccoe-customer-contact-manager-webhook-lambda.zip` (Webhook)
- `lambda/upload_lambda/upload-metadata-lambda.zip` (Upload API)
- `lambda/saml_auth/lambda-edge-samlify.zip` (SAML Auth)

**S3 Upload Paths** (if `LAMBDA_ARTIFACT_BUCKET` is set):
- Backend: `s3://$LAMBDA_ARTIFACT_BUCKET/backend-lambda/`
- Webhook: `s3://$LAMBDA_ARTIFACT_BUCKET/webhook-lambda/`
- Upload: `s3://$LAMBDA_ARTIFACT_BUCKET/upload-lambda/`
- SAML: `s3://$LAMBDA_ARTIFACT_BUCKET/saml-lambda/`

---

## AWS CodeBuild Project Setup

### Creating a CodeBuild Project for Webhook Lambda

```bash
# Using AWS CLI
aws codebuild create-project \
  --name ccoe-webhook-lambda-build \
  --source type=GITHUB,location=https://github.com/your-org/your-repo.git \
  --artifacts type=S3,location=your-artifact-bucket \
  --environment type=LINUX_CONTAINER,image=aws/codebuild/standard:7.0,computeType=BUILD_GENERAL1_SMALL \
  --service-role arn:aws:iam::ACCOUNT_ID:role/CodeBuildServiceRole \
  --buildspec buildspec-webhook.yml
```

### Environment Variables Configuration

Set these in your CodeBuild project:

**For Webhook Lambda Build**:
```
LAMBDA_ARTIFACT_BUCKET=your-lambda-artifacts-bucket
WEBHOOK_FUNCTION_NAME=ccoe-webhook-lambda
AWS_DEFAULT_REGION=us-east-1
```

**For All Lambdas Build**:
```
LAMBDA_ARTIFACT_BUCKET=your-lambda-artifacts-bucket
BACKEND_FUNCTION_NAME=ccoe-backend-lambda
WEBHOOK_FUNCTION_NAME=ccoe-webhook-lambda
UPLOAD_FUNCTION_NAME=ccoe-upload-lambda
SAML_FUNCTION_NAME=ccoe-saml-auth-lambda
AWS_DEFAULT_REGION=us-east-1
```

---

## Local Testing

You can test the build process locally using the Makefile:

### Build Webhook Lambda Only
```bash
make build-webhook-lambda
make package-webhook-lambda
```

### Build All Lambdas
```bash
make package-all-lambdas
```

### Build Individual Lambdas
```bash
# Backend Golang Lambda
make package-golang-lambda

# Upload Node.js Lambda
make package-upload-lambda

# SAML Auth Lambda
make package-saml-lambda

# Webhook Lambda
make package-webhook-lambda
```

---

## CI/CD Pipeline Recommendations

### Option 1: Separate Pipelines (Recommended)
Create separate CodeBuild projects for each Lambda function:
- Faster builds (only build what changed)
- Independent deployment cycles
- Easier troubleshooting

**Use**:
- `buildspec-webhook.yml` for webhook Lambda pipeline
- Similar buildspecs for other Lambda functions

### Option 2: Unified Pipeline
Create one CodeBuild project that builds all Lambda functions:
- Simpler configuration
- Ensures all functions are in sync
- Good for release builds

**Use**:
- `buildspec-all-lambdas.yml` for unified pipeline

### Option 3: Conditional Builds
Use a single buildspec with conditional logic based on changed files:
- Most efficient for monorepo
- Requires more complex buildspec logic

---

## Deployment Strategies

### Strategy 1: Direct Lambda Update
Set `WEBHOOK_FUNCTION_NAME` environment variable in CodeBuild:
- Automatically updates Lambda function after build
- Fast deployment
- No manual steps

### Strategy 2: S3 + Manual Deploy
Set only `LAMBDA_ARTIFACT_BUCKET`:
- Uploads package to S3
- Deploy manually or via separate pipeline
- More control over deployment timing

### Strategy 3: S3 + Terraform
Upload to S3, then use Terraform to deploy:
- Infrastructure as Code
- Version controlled deployments
- Recommended for production

```hcl
# Terraform example
resource "aws_lambda_function" "webhook" {
  function_name = "ccoe-webhook-lambda"
  s3_bucket     = "your-lambda-artifacts-bucket"
  s3_key        = "webhook-lambda/ccoe-customer-contact-manager-webhook-lambda-latest.zip"
  handler       = "bootstrap"
  runtime       = "provided.al2"
  architectures = ["arm64"]
  # ... other configuration
}
```

---

## Architecture Notes

### ARM64/Graviton Support
All Go Lambda functions are built for ARM64 (Graviton) by default:
- Better price/performance ratio
- Lower latency
- 20% cost savings vs x86_64

To build for x86_64 instead, modify the buildspec:
```yaml
- make build-webhook-lambda-x86 VERSION=$VERSION
```

### Caching
BuildSpec files include caching for:
- Go module cache: `/go/pkg/mod/**/*`
- Go build cache: `/root/.cache/go-build/**/*`
- Node.js modules: `lambda/*/node_modules/**/*`

This speeds up subsequent builds significantly.

---

## Troubleshooting

### Build Fails: "go: command not found"
- Ensure `runtime-versions: golang: 1.21` is in install phase
- Use CodeBuild image `aws/codebuild/standard:7.0` or later

### Build Fails: "make: command not found"
- Make is included in standard CodeBuild images
- If using custom image, install make in install phase

### Lambda Update Fails: "AccessDeniedException"
- Ensure CodeBuild service role has `lambda:UpdateFunctionCode` permission
- Add policy:
```json
{
  "Effect": "Allow",
  "Action": [
    "lambda:UpdateFunctionCode",
    "lambda:GetFunction"
  ],
  "Resource": "arn:aws:lambda:*:*:function:ccoe-*"
}
```

### S3 Upload Fails: "Access Denied"
- Ensure CodeBuild service role has S3 write permissions
- Add policy:
```json
{
  "Effect": "Allow",
  "Action": [
    "s3:PutObject",
    "s3:PutObjectAcl"
  ],
  "Resource": "arn:aws:s3:::your-lambda-artifacts-bucket/*"
}
```

---

## Best Practices

1. **Use Separate Buildspecs**: Create focused buildspecs for each Lambda function
2. **Version Artifacts**: Include commit hash in S3 object keys for versioning
3. **Cache Dependencies**: Enable caching to speed up builds
4. **Test Locally First**: Use Makefile to test builds before pushing to CodeBuild
5. **Monitor Build Times**: Optimize build process if builds take >5 minutes
6. **Use ARM64**: Leverage Graviton for cost savings and performance
7. **Automate Testing**: Add test phase before deployment
8. **Tag Releases**: Use git tags for production deployments

---

## Related Documentation

- [Makefile Documentation](../Makefile) - Local build commands
- [Webhook Lambda Documentation](WEBHOOK_INFRASTRUCTURE.md) - Webhook Lambda details
- [Deployment Guide](DEPLOYMENT.md) - Full deployment instructions
