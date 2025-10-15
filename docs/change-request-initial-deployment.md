Every aspect of the solution is implemented and managed by Terraform Cloud. The implementation plan at a high-level is to plan and apply changes to the following three Terraform Cloud Workspaces:

a, https://app.terraform.io/app/hearsttech/workspaces/hts-aws-com-std-app-orchestration-email-distro-prod-use1
b, https://app.terraform.io/app/hearsttech/workspaces/hts-aws-com-std-app-cicd-prod-email-distribution-use1
c, another per-customer terraform cloud workspace to be created that will deploy the IAM Role and Policy

The solution is deployed across multiple AWS accounts with distinct responsibilities:

- **hts-aws-com-std-app-prod** (Account ID: 730335533660): Main orchestration account hosting the web application, S3 storage, and governance ECS cluster
Terraform Cloud Workspace: a, https://app.terraform.io/app/hearsttech/workspaces/hts-aws-com-std-app-orchestration-email-distro-prod-use1
Components:
Web Application: CloudFront distribution serving static website from S3
S3 Bucket: Stores change request metadata and attachments
ECS Cluster: Runs the governance service container for processing workflows
Lambda Functions: Handles S3 upload events and metadata processing
SQS Queues: Message queuing for asynchronous processing
IAM Roles: Cross-account assume role permissions to access customer accounts

- **hts-aws-com-std-app-cicd-prod** (Account ID: 173748160886): CI/CD account containing build pipelines for all solution components
Terraform Cloud Workspace: b, https://app.terraform.io/app/hearsttech/workspaces/hts-aws-com-std-app-cicd-prod-email-distribution-use1
Components:
CodePipeline: Orchestrates the build and deployment process
CodeBuild: Compiles Go applications, builds Docker images, packages Lambda functions
ECR Repository: Stores Docker images for ECS deployment
S3 Buckets: Stores build artifacts and deployment packages
IAM Roles: Cross-account deployment permissions

- **$CUSTOMER-aws-com-std-app-common-prod (per customer)**: Customer-specific accounts for SES email delivery and IAM role management
Terraform Cloud Workspace: c, another per-customer terraform cloud workspace to be created that will deploy the IAM Role and Policy
Components:
SES (Simple Email Service): Sends emails to customer users
IAM Role: Allows hts-aws-com-std-app-prod to assume role for operations
IAM Policy: Grants permissions for Identity Center queries and SES operations
Identity Center Integration: Access to customer's user directory








Testing:
# CCOE Customer Contact Manager - Testing Session

This directory contains comprehensive tests for the CCOE Customer Contact Manager application.

## Test Categories

### 1. Configuration Tests
- Validate all JSON configuration files
- Test configuration loading and parsing
- Verify customer code mappings

### 2. AWS Infrastructure Tests
- Test AWS credentials and permissions
- Validate SQS queue access
- Test S3 bucket operations
- Verify SES configuration

### 3. Application Mode Tests
- Test update mode with dry-run
- Test SQS message processing
- Test validation mode
- Test multi-customer operations

### 4. Integration Tests
- End-to-end workflow testing
- Multi-customer email distribution
- Error handling and recovery

## Running Tests

Execute tests in order:

```bash
# 1. Configuration validation
./test-config.sh

# 2. AWS infrastructure validation
./test-aws-infrastructure.sh

# 3. Application functionality
./test-app-modes.sh

# 4. Integration testing
./test-integration.sh
```

## Test Results

Results are logged to `test-results/` directory with timestamps.