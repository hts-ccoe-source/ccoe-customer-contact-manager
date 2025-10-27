# BuildSpec Implementation Summary

## Overview
Created AWS CodeBuild buildspec files for building and deploying the Typeform webhook Lambda function and all Lambda functions in the project.

## Files Created

### 1. `buildspec-webhook.yml`
**Purpose**: Dedicated buildspec for the Typeform webhook Lambda function

**Features**:
- Builds Go webhook Lambda for ARM64/Graviton architecture
- Creates deployment ZIP package
- Uploads to S3 (optional, via `LAMBDA_ARTIFACT_BUCKET`)
- Auto-updates Lambda function (optional, via `WEBHOOK_FUNCTION_NAME`)
- Includes build caching for faster subsequent builds

**Environment Variables**:
- `LAMBDA_ARTIFACT_BUCKET` (optional) - S3 bucket for artifacts
- `WEBHOOK_FUNCTION_NAME` (optional) - Lambda function name for auto-deployment
- `AWS_DEFAULT_REGION` - AWS region

**Build Process**:
1. Install Go 1.21
2. Set version info from git tags
3. Build webhook Lambda using `make build-webhook-lambda`
4. Create ZIP package
5. Upload to S3 (if configured)
6. Update Lambda function (if configured)

**Artifacts**:
- `ccoe-customer-contact-manager-webhook-lambda.zip`

### 2. `buildspec-all-lambdas.yml`
**Purpose**: Unified buildspec for building all Lambda functions

**Lambda Functions Built**:
1. Backend Golang Lambda (main.go) - S3 event processor
2. Webhook Golang Lambda (cmd/webhook) - Typeform webhook receiver
3. Upload Node.js Lambda (lambda/upload_lambda) - Frontend API
4. SAML Auth Lambda@Edge (lambda/saml_auth) - Authentication

**Features**:
- Builds all Lambda functions in one build
- Supports both Go and Node.js runtimes
- Uploads all packages to S3 (optional)
- Auto-updates all Lambda functions (optional)
- Comprehensive caching for Go and Node.js dependencies

**Environment Variables**:
- `LAMBDA_ARTIFACT_BUCKET` (optional) - S3 bucket for artifacts
- `BACKEND_FUNCTION_NAME` (optional) - Backend Lambda function name
- `WEBHOOK_FUNCTION_NAME` (optional) - Webhook Lambda function name
- `UPLOAD_FUNCTION_NAME` (optional) - Upload Lambda function name
- `SAML_FUNCTION_NAME` (optional) - SAML Lambda function name
- `AWS_DEFAULT_REGION` - AWS region

**Build Process**:
1. Install Go 1.21 and Node.js 18
2. Set version info from git tags
3. Build all Lambda functions using `make package-all-lambdas`
4. Upload all packages to S3 (if configured)
5. Update all Lambda functions (if configured)

**Artifacts**:
- `ccoe-customer-contact-manager-lambda.zip` (Backend)
- `ccoe-customer-contact-manager-webhook-lambda.zip` (Webhook)
- `lambda/upload_lambda/upload-metadata-lambda.zip` (Upload)
- `lambda/saml_auth/lambda-edge-samlify.zip` (SAML)

### 3. `docs/BUILDSPEC_GUIDE.md`
**Purpose**: Comprehensive documentation for all buildspec files

**Contents**:
- Detailed description of each buildspec file
- Environment variable configuration
- AWS CodeBuild project setup instructions
- Local testing commands
- CI/CD pipeline recommendations
- Deployment strategies
- Troubleshooting guide
- Best practices

## Key Features

### Version Management
All buildspecs include automatic version management:
- Uses git tags for version numbers
- Falls back to commit hash for dev builds
- Includes build timestamp and git commit in metadata

### Artifact Management
Flexible artifact handling:
- **Local artifacts**: ZIP files in build directory
- **S3 upload**: Optional upload to S3 bucket with versioning
- **Direct deployment**: Optional auto-update of Lambda functions

### Caching
Optimized build caching:
- Go module cache: `/go/pkg/mod/**/*`
- Go build cache: `/root/.cache/go-build/**/*`
- Node.js modules: `lambda/*/node_modules/**/*`

### Architecture Support
Built for AWS Graviton (ARM64):
- 20% cost savings vs x86_64
- Better price/performance ratio
- Lower latency

## Integration with Existing Infrastructure

### Makefile Integration
Buildspecs use existing Makefile targets:
- `make build-webhook-lambda` - Build webhook Lambda
- `make package-all-lambdas` - Build all Lambda functions
- Consistent build process between local and CI/CD

### S3 Upload Paths
Organized S3 structure:
```
s3://bucket/
├── backend-lambda/
│   ├── ccoe-customer-contact-manager-lambda-{commit}.zip
│   └── ccoe-customer-contact-manager-lambda-latest.zip
├── webhook-lambda/
│   ├── ccoe-customer-contact-manager-webhook-lambda-{commit}.zip
│   └── ccoe-customer-contact-manager-webhook-lambda-latest.zip
├── upload-lambda/
│   ├── upload-metadata-lambda-{commit}.zip
│   └── upload-metadata-lambda-latest.zip
└── saml-lambda/
    ├── lambda-edge-samlify-{commit}.zip
    └── lambda-edge-samlify-latest.zip
```

### Terraform Integration
Buildspecs designed to work with Terraform:
- Uploads to S3 for Terraform to reference
- Consistent naming for Terraform data sources
- Version tracking for rollback capability

## Usage Recommendations

### Development Workflow
1. **Local Testing**: Use Makefile to test builds locally
2. **Push to Git**: Commit and push changes
3. **CodeBuild**: Automatic build triggered by git push
4. **S3 Upload**: Artifacts uploaded to S3
5. **Terraform Deploy**: Use Terraform to deploy from S3

### CI/CD Pipeline Options

**Option 1: Separate Pipelines (Recommended)**
- Create separate CodeBuild projects for each Lambda
- Use `buildspec-webhook.yml` for webhook Lambda
- Faster builds, independent deployments

**Option 2: Unified Pipeline**
- Create one CodeBuild project for all Lambdas
- Use `buildspec-all-lambdas.yml`
- Simpler configuration, synchronized deployments

**Option 3: Conditional Builds**
- Use git diff to detect changed files
- Build only affected Lambda functions
- Most efficient for monorepo

## Required IAM Permissions

### CodeBuild Service Role
The CodeBuild service role needs these permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "lambda:UpdateFunctionCode",
        "lambda:GetFunction"
      ],
      "Resource": "arn:aws:lambda:*:*:function:ccoe-*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:PutObjectAcl"
      ],
      "Resource": "arn:aws:s3:::your-lambda-artifacts-bucket/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::your-lambda-artifacts-bucket",
        "arn:aws:s3:::your-lambda-artifacts-bucket/*"
      ]
    }
  ]
}
```

## Testing

### Local Testing
```bash
# Test webhook Lambda build
make build-webhook-lambda
make package-webhook-lambda

# Test all Lambda builds
make package-all-lambdas

# Verify artifacts
ls -lh *.zip
ls -lh bin/
```

### CodeBuild Testing
1. Create test CodeBuild project
2. Set environment variables
3. Trigger manual build
4. Verify artifacts in S3
5. Check Lambda function updated (if configured)

## Next Steps

1. **Create CodeBuild Projects**: Set up CodeBuild projects in AWS
2. **Configure Environment Variables**: Set required environment variables
3. **Test Builds**: Run test builds to verify configuration
4. **Integrate with Git**: Set up webhooks for automatic builds
5. **Monitor Builds**: Set up CloudWatch alarms for build failures
6. **Document Deployment**: Update deployment documentation

## Related Files

- `Makefile` - Local build commands
- `buildspec.yml` - Original Docker/ECS buildspec
- `cmd/webhook/main.go` - Webhook Lambda source code
- `docs/WEBHOOK_INFRASTRUCTURE.md` - Webhook Lambda documentation
- `docs/BUILDSPEC_GUIDE.md` - Comprehensive buildspec documentation

## Notes

- All buildspecs use AWS CodeBuild standard image 7.0
- Go 1.21 and Node.js 18 are the required runtime versions
- ARM64/Graviton is the default architecture for cost savings
- Build caching significantly speeds up subsequent builds
- Version information is automatically extracted from git tags
- Artifacts are versioned by commit hash for traceability
