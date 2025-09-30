# CloudFront + Lambda@Edge Deployment Guide

## Prerequisites

1. **ACM Certificate**: You need an SSL certificate in `us-east-1` for your domain
2. **Domain Name**: Configure your desired domain (e.g., `change-management.hearst.com`)
3. **JWT Secret**: Your SSO JWT secret or public key for token verification

## Step 1: Add Terraform Resources

Copy the contents of `cloudfront-terraform.tf` to your main Terraform file:

```
/Users/steven.craig/OneDrive - Hearst/hearst/terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/main.tf
```

## Step 2: Set Terraform Variables

Add these variables to your `terraform.tfvars` or define them in your Terraform configuration:

```hcl
# Add to terraform.tfvars
domain_name = "change-management.hearst.com"
certificate_arn = "arn:aws:acm:us-east-1:730335533660:certificate/your-cert-id"
jwt_secret = "your-sso-jwt-secret-or-public-key"
```

## Step 3: Copy Required Files

Ensure these files are in your Terraform directory:

- `lambda-edge-auth.js`
- `aws-credentials-api.js`
- `html/` directory with all website files

## S3 Bucket Structure

The S3 bucket contains both website files and customer data:

```
s3://bucket-name/
├── index.html                    # Website files (deployed from html/)
├── create-change.html
├── my-changes.html
├── view-changes.html
├── search-changes.html
├── edit-change.html
├── assets/                       # Website assets
│   ├── css/
│   └── js/
├── customers/                    # Customer-specific change files
│   ├── customer-a/
│   ├── customer-b/
│   └── ...
├── drafts/                       # Draft changes
├── archive/                      # Permanent change archive
└── lambda/                       # Lambda function code
```

**⚠️ Important**: Never use `aws s3 sync --delete` as it would remove customer data!

## Step 4: Deploy with Terraform

```bash
cd /Users/steven.craig/OneDrive\ -\ Hearst/hearst/terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/

# Plan the deployment
terraform plan

# Apply the changes
terraform apply
```

## Step 5: Deploy Enhanced Lambda Function

The multi-page portal requires an enhanced Lambda function that supports multiple API endpoints. Replace the current upload Lambda with the enhanced version:

### API Endpoints Required

| Method | Path | Description | Frontend Usage |
|--------|------|-------------|----------------|
| `POST` | `/upload` | Submit new change | create-change.html |
| `GET` | `/changes` | Get all changes | view-changes.html |
| `GET` | `/changes/{id}` | Get specific change | view-changes.html (modal) |
| `GET` | `/my-changes` | Get user's changes | my-changes.html |
| `GET` | `/drafts` | Get user's drafts | my-changes.html |
| `GET` | `/drafts/{id}` | Get specific draft | create-change.html (load) |
| `POST` | `/drafts` | Save draft | create-change.html (save) |
| `POST` | `/changes/search` | Search changes | search-changes.html |

### Deploy Enhanced Lambda

```bash
# Backup current function
cp upload-metadata-lambda.js upload-metadata-lambda.js.backup

# Deploy enhanced version
cp enhanced-metadata-lambda.js upload-metadata-lambda.js

# Update Lambda function
aws lambda update-function-code \
    --function-name your-lambda-function-name \
    --zip-file fileb://lambda-deployment-package.zip
```

### Update CloudFront Behaviors

Add these path patterns to your CloudFront distribution to route API calls to Lambda:

```yaml
# Add to cloudfront-auth-stack.yaml
CacheBehaviors:
  - PathPattern: '/changes*'
    TargetOriginId: LambdaOrigin
    ViewerProtocolPolicy: redirect-to-https
    CachePolicyId: 4135ea2d-6df8-44a3-9df3-4b5a84be39ad  # CachingDisabled
  - PathPattern: '/my-changes'
    TargetOriginId: LambdaOrigin
    ViewerProtocolPolicy: redirect-to-https
    CachePolicyId: 4135ea2d-6df8-44a3-9df3-4b5a84be39ad  # CachingDisabled
  - PathPattern: '/drafts*'
    TargetOriginId: LambdaOrigin
    ViewerProtocolPolicy: redirect-to-https
    CachePolicyId: 4135ea2d-6df8-44a3-9df3-4b5a84be39ad  # CachingDisabled
```

## Step 6: Upload HTML Files to S3

```bash
# Upload the updated HTML files
# Note: Do NOT use --delete flag as it would remove customer archives
aws s3 sync html/ s3://hts-prod-ccoe-change-management-metadata/
```

```bash
# break the cache
export CLOUDFRONT_DISTRIBUTION_ID=E3DIDLE5N99NVJ
aws cloudfront create-invalidation --distribution-id $CLOUDFRONT_DISTRIBUTION_ID --paths "/*.html" "/assets/*"
```

## Step 6: Configure DNS

Point your domain to the CloudFront distribution:

```
change-management.hearst.com -> d1234567890123.cloudfront.net
```

## Step 7: Configure SSO Integration

Update `lambda-edge-auth.js` with your specific SSO configuration:

1. **JWT Verification**: Replace the JWT verification logic with your SSO provider
2. **Authorization Logic**: Update `isAuthorizedUser()` function for your requirements
3. **Redirect URL**: Update the SSO login URL in `generateAuthResponse()`

## Step 8: Test the Deployment

1. **Access the URL**: `https://change-management.hearst.com`
2. **Verify Authentication**: Should redirect to your SSO login
3. **Test Form Submission**: Fill out the form and verify S3 upload works
4. **Check ECS Task**: Verify the ECS task is triggered by S3 events

## Architecture Overview

```
User Browser
    ↓ (HTTPS)
CloudFront Distribution
    ↓ (Lambda@Edge Auth)
S3 Static Website
    ↓ (API Call)
API Gateway
    ↓ (Lambda)
AWS Credentials API
    ↓ (STS AssumeRole)
Temporary AWS Credentials
    ↓ (S3 Upload)
S3 Bucket
    ↓ (S3 Event)
SQS Queue
    ↓ (ECS Task)
Contact Manager CLI
```

## Security Features

- ✅ **No Public Access**: All access requires authentication
- ✅ **Lambda@Edge Authentication**: Validates users at CloudFront edge
- ✅ **Temporary Credentials**: Short-lived AWS credentials via STS
- ✅ **Role-Based Access**: Users must have appropriate groups/roles
- ✅ **HTTPS Only**: All traffic encrypted in transit
- ✅ **Origin Access Control**: S3 bucket only accessible via CloudFront

## Troubleshooting

### Common Issues

1. **Lambda@Edge Deployment**: Takes 15-30 minutes to propagate globally
2. **Certificate Issues**: ACM certificate must be in `us-east-1` region
3. **DNS Propagation**: Can take up to 48 hours for DNS changes
4. **CORS Issues**: API Gateway needs proper CORS configuration for browser requests

### Logs to Check

- **CloudFront Logs**: For authentication issues
- **Lambda@Edge Logs**: In CloudWatch (multiple regions)
- **API Gateway Logs**: For credentials API issues
- **ECS Task Logs**: For contact manager execution

## Next Steps

1. **Monitor Usage**: Set up CloudWatch dashboards
2. **Set Up Alerts**: Configure alarms for failures
3. **Backup Strategy**: Ensure S3 bucket has versioning enabled
4. **Security Review**: Regular audit of IAM roles and permissions
