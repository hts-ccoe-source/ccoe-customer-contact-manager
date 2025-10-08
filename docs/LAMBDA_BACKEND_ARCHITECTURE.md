# Lambda Backend Architecture

## Overview

This document describes the new secure Lambda backend architecture for the Change Management Portal, which eliminates the need for users to have direct AWS credentials.

## Architecture Diagram

```
User Browser → CloudFront → Lambda@Edge (Auth) → Lambda Function → S3 + SQS
     ↓              ↓              ↓                    ↓           ↓
  HTML/JS      Static Files   User Headers        S3 Uploads   Notifications
```

## Components

### 1. Lambda@Edge Authentication (`lambda-edge-samlify.js`)
- **Purpose**: Authenticates users via SAML SSO
- **Location**: CloudFront edge locations
- **Responsibilities**:
  - SAML authentication with Identity Center
  - Session management via secure cookies
  - Adds user headers (`x-user-email`, `x-user-groups`) to requests
  - Routes `/api/*` requests to Lambda backend

### 2. Lambda Backend Function (`upload-metadata-lambda.js`)
- **Purpose**: Handles metadata uploads securely
- **Location**: AWS Lambda (us-east-1)
- **Responsibilities**:
  - Validates user authentication headers
  - Authorizes users for change management operations
  - Uploads metadata to S3 buckets (customer + archive)
  - Sends SQS notifications
  - Returns upload results to frontend

### 3. CloudFront Distribution
- **Purpose**: Routes requests and provides authentication
- **Behaviors**:
  - `*.html` → S3 static files (with Lambda@Edge auth)
  - `/api/*` → Lambda Function URL (with Lambda@Edge auth)

### 4. HTML Frontend (`html/` directory)
- **Purpose**: User interface for change requests
- **Changes**:
  - Removed AWS SDK dependency
  - Calls `/api/upload-metadata` instead of direct S3
  - Displays results from Lambda response

## Security Model

### Authentication Flow
1. **User accesses site** → CloudFront serves HTML
2. **Lambda@Edge checks session** → Redirects to SAML if needed
3. **SAML authentication** → Identity Center validates user
4. **Session created** → Secure cookie with user info
5. **Subsequent requests** → Lambda@Edge adds user headers

### Authorization
- **Domain validation**: Must be `@hearst.com` email
- **Group membership**: Optional group-based permissions
- **Request validation**: Lambda validates all inputs
- **S3 permissions**: Lambda has minimal S3 upload permissions only

### Credentials Management
- **Users**: No AWS credentials needed
- **Lambda@Edge**: No AWS API access (stateless)
- **Lambda Function**: IAM role with minimal S3/SQS permissions
- **S3 Access**: Only through Lambda function, not direct user access

## Deployment

### Prerequisites
- AWS CLI configured with appropriate permissions
- CloudFormation access
- Lambda deployment permissions

### Steps

1. **Deploy Lambda Backend**:
   ```bash
   ./deploy-lambda-backend.sh
   ```

2. **Update CloudFront Stack**:
   ```bash
   aws cloudformation update-stack \
       --stack-name change-management-cloudfront \
       --template-body file://cloudfront-auth-stack.yaml \
       --parameters ParameterKey=LambdaFunctionUrl,ParameterValue=<lambda-domain> \
       --capabilities CAPABILITY_IAM \
       --region us-east-1
   ```

3. **Test Lambda Function**:
   ```bash
   ./test-lambda-backend.sh
   ```

4. **Deploy Updated HTML**:
   - Upload `html/` directory to S3
   - Invalidate CloudFront cache

## API Endpoints

### POST /api/upload-metadata

**Request Headers**:
- `Content-Type: application/json`
- `x-user-email`: User email (added by Lambda@Edge)
- `x-user-groups`: User groups (added by Lambda@Edge)

**Request Body**:
```json
{
  "changeTitle": "Example Change",
  "customers": ["hts", "cds"],
  "changeReason": "Business justification",
  "implementationPlan": "Step-by-step plan",
  "testPlan": "Testing approach",
  "customerImpact": "Expected impact",
  "rollbackPlan": "Rollback procedure",
  "implementationBeginDate": "2024-01-01",
  "implementationBeginTime": "10:00",
  "implementationEndDate": "2024-01-01",
  "implementationEndTime": "11:00",
  "timezone": "America/New_York"
}
```

**Response**:
```json
{
  "success": true,
  "changeId": "CHG-2024-01-01T10-00-00-abc123",
  "uploadResults": [
    {
      "customer": "HTS Prod",
      "success": true,
      "s3Key": "customers/hts/CHG-2024-01-01T10-00-00-abc123.json",
      "bucket": "4cm-prod-ccoe-change-management-metadata"
    }
  ],
  "summary": {
    "total": 3,
    "successful": 3,
    "failed": 0
  }
}
```

## Error Handling

### Authentication Errors
- **401 Unauthorized**: No user email header
- **403 Forbidden**: Invalid domain or insufficient permissions

### Validation Errors
- **400 Bad Request**: Missing required fields
- **400 Bad Request**: Invalid customer selection

### Upload Errors
- **500 Internal Server Error**: S3 upload failures
- **Partial Success**: Some uploads succeed, others fail

## Benefits

### Security
- ✅ No AWS credentials in browser
- ✅ Centralized authentication via SAML
- ✅ Minimal IAM permissions for Lambda
- ✅ Request validation and authorization

### User Experience
- ✅ Seamless authentication flow
- ✅ Error messages appear at bottom of form
- ✅ Detailed upload status and retry options
- ✅ No AWS SDK complexity in frontend

### Maintainability
- ✅ Centralized upload logic in Lambda
- ✅ Easy to add validation rules
- ✅ Comprehensive logging and monitoring
- ✅ Testable backend components

## Monitoring

### CloudWatch Logs
- **Lambda@Edge**: Authentication events and errors
- **Lambda Function**: Upload operations and results
- **CloudFront**: Request routing and caching

### Metrics
- **Authentication success/failure rates**
- **Upload success/failure rates**
- **Response times**
- **Error rates by type**

## Troubleshooting

### Common Issues

1. **"Authentication required" error**
   - Check Lambda@Edge is adding user headers
   - Verify SAML session is valid

2. **"Upload failed" errors**
   - Check Lambda function logs in CloudWatch
   - Verify S3 bucket permissions
   - Check SQS queue configuration

3. **CORS errors**
   - Verify CloudFront behaviors are configured correctly
   - Check Lambda Function URL CORS settings

### Debug Steps

1. **Check CloudFront logs** for request routing
2. **Check Lambda@Edge logs** for authentication
3. **Check Lambda function logs** for upload operations
4. **Test Lambda directly** using test scripts
5. **Verify S3 permissions** for Lambda role

## Future Enhancements

- **Enhanced group-based authorization**
- **Audit logging to dedicated S3 bucket**
- **Integration with ServiceNow API**
- **Real-time notifications via WebSocket**
- **Approval workflow integration**