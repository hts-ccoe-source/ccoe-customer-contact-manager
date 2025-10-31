# SAML Authentication Lambda@Edge Function

This directory contains the Lambda@Edge function for SAML authentication with AWS Identity Center.

## Files

- `lambda-edge-samlify.js` - Main Lambda@Edge function code
- `package.json` - Node.js dependencies and configuration
- `package-lock.json` - Locked dependency versions
- `lambda-edge-samlify.zip` - Pre-built deployment package (checked into git for Terraform Cloud)

## Dependencies

- `samlify` - SAML 2.0 library for Node.js
- `@authenio/samlify-node-xmllint` - XML validation for samlify

## Deployment

This function is deployed using Terraform Cloud. The pre-built `lambda-edge-samlify.zip` file is checked into git and used by Terraform's `archive_file` data source in `main.tf`.

### Lambda@Edge Deployment Configuration

#### Terraform Configuration Example

```hcl
# Lambda@Edge function for SAML authentication
resource "aws_lambda_function" "saml_auth" {
  filename         = "lambda-edge-samlify.zip"
  function_name    = "saml-auth-edge"
  role            = aws_iam_role.lambda_edge_role.arn
  handler         = "lambda-edge-samlify.handler"
  runtime         = "nodejs18.x"
  publish         = true  # Required for Lambda@Edge
  
  # Lambda@Edge constraints
  timeout     = 5      # Max 5 seconds for viewer-request
  memory_size = 128    # Min 128MB, max 10240MB
  
  # Session timeout configuration (optional - uses defaults if not set)
  environment {
    variables = {
      SESSION_IDLE_TIMEOUT_MS      = "10800000"  # 3 hours
      SESSION_ABSOLUTE_MAX_MS      = "43200000"  # 12 hours
      SESSION_REFRESH_THRESHOLD_MS = "600000"    # 10 minutes
      SESSION_COOKIE_MAX_AGE       = "10800"     # 3 hours
    }
  }
}

# CloudFront distribution with Lambda@Edge association
resource "aws_cloudfront_distribution" "main" {
  # ... other configuration ...
  
  default_cache_behavior {
    # ... other settings ...
    
    lambda_function_association {
      event_type   = "viewer-request"
      lambda_arn   = aws_lambda_function.saml_auth.qualified_arn
      include_body = true
    }
  }
}
```

#### CloudFormation Configuration Example

```yaml
Resources:
  SAMLAuthFunction:
    Type: AWS::Lambda::Function
    Properties:
      FunctionName: saml-auth-edge
      Runtime: nodejs18.x
      Handler: lambda-edge-samlify.handler
      Role: !GetAtt LambdaEdgeRole.Arn
      Code:
        S3Bucket: my-deployment-bucket
        S3Key: lambda-edge-samlify.zip
      Timeout: 5
      MemorySize: 128
      Environment:
        Variables:
          SESSION_IDLE_TIMEOUT_MS: "10800000"
          SESSION_ABSOLUTE_MAX_MS: "43200000"
          SESSION_REFRESH_THRESHOLD_MS: "600000"
          SESSION_COOKIE_MAX_AGE: "10800"
  
  SAMLAuthVersion:
    Type: AWS::Lambda::Version
    Properties:
      FunctionName: !Ref SAMLAuthFunction
  
  CloudFrontDistribution:
    Type: AWS::CloudFront::Distribution
    Properties:
      DistributionConfig:
        DefaultCacheBehavior:
          LambdaFunctionAssociations:
            - EventType: viewer-request
              LambdaFunctionARN: !Ref SAMLAuthVersion
              IncludeBody: true
```

### Lambda@Edge Constraints

This function meets all Lambda@Edge requirements:

| Constraint | Limit | Current Usage | Status |
|------------|-------|---------------|--------|
| **Package Size** | 50 MB (compressed) | ~2.7 MB | ✅ Well within limit |
| **Uncompressed Size** | No specific limit | ~9.5 MB | ✅ Acceptable |
| **Memory** | 128 MB - 10,240 MB | 128 MB | ✅ Minimum sufficient |
| **Timeout** | 5 seconds (viewer-request) | < 50ms typical | ✅ Well within limit |
| **Environment Variables** | Supported | 4 variables | ✅ Supported |
| **Layers** | Not supported | None used | ✅ N/A |
| **Dead Letter Queues** | Not supported | None used | ✅ N/A |
| **VPC** | Not supported | None used | ✅ N/A |

**Verification**: Run `./verify-lambda-edge-constraints.sh` to verify all constraints before deployment.

#### Performance Characteristics

- **Cold Start**: ~100-150ms (includes module loading and config validation)
- **Warm Execution**: ~20-50ms (session validation and cookie operations)
- **Memory Usage**: ~40-60 MB (well below 128 MB limit)
- **Package Size**: ~50 KB compressed (2% of 1 MB limit)

#### Deployment Checklist

Before deploying to Lambda@Edge:

- [ ] Package size is under 1 MB compressed
- [ ] Function timeout is set to 5 seconds or less
- [ ] Memory is set to at least 128 MB
- [ ] Function is published (versioned) - required for CloudFront association
- [ ] IAM role has Lambda@Edge trust policy
- [ ] Environment variables are set (or defaults are acceptable)
- [ ] CloudWatch log groups are configured in all regions
- [ ] Function has been tested in us-east-1 (required region for Lambda@Edge)

## Development Workflow

### For Code Changes:

1. Make changes to `lambda-edge-samlify.js`
2. Update dependencies in `package.json` if needed
3. Build and commit the deployment package:

   ```bash
   cd saml_auth
   npm install                    # Install/update dependencies
   make build                     # Create deployment package
   git add lambda-edge-samlify.zip
   git commit -m "Update Lambda@Edge function"
   ```

4. Push to trigger Terraform Cloud deployment

### For Terraform Cloud:

- **Pre-built zip required**: Terraform Cloud uses the checked-in `lambda-edge-samlify.zip`
- **No build step in cloud**: Dependencies must be bundled locally before commit
- **Version control**: The zip file is tracked in git for reproducible deployments

## Build Commands

- `make build` - Install dependencies and create deployment package
- `make package` - Create package only (assumes dependencies installed)
- `make clean` - Remove build artifacts and node_modules
- `./build.sh` - Direct build script execution

## Architecture

This Lambda@Edge function handles:

- SAML AuthnRequest generation
- SAML Response validation (simplified for performance)
- Session management via secure cookies
- User authorization based on email domain

### CloudFront Integration Requirements

**CRITICAL**: When using this Lambda@Edge function with CloudFront cache behaviors that forward requests to origins (like Lambda Function URLs), you **MUST** configure an Origin Request Policy that forwards custom headers.

#### Required Configuration:

```terraform
ordered_cache_behavior {
  path_pattern             = "/upload*"
  target_origin_id         = "LambdaUploadOrigin"
  # ... other settings ...
  
  # REQUIRED: Forward authentication headers added by Lambda@Edge
  origin_request_policy_id = "88a5eaf4-2fd4-4709-b370-b4c650ea3fcf" # CORS-S3Origin
  
  lambda_function_association {
    event_type   = "viewer-request"
    lambda_arn   = aws_lambda_function.auth_lambda_function.qualified_arn
    include_body = true
  }
}
```

#### Why This Is Required:

1. **Lambda@Edge adds headers**: After SAML validation, the function adds:
   - `x-user-email: user@hearst.com`
   - `x-authenticated: true`
   - `x-user-groups: group1,group2`

2. **CloudFront doesn't forward by default**: Without an origin request policy, CloudFront strips custom headers when forwarding to origins

3. **Downstream services need headers**: Lambda functions, APIs, and other services rely on these authentication headers

#### Common Issues:

- **401 Unauthorized errors**: If headers aren't forwarded, downstream services can't validate authentication
- **Missing user context**: Services won't know who the authenticated user is
- **Silent failures**: Requests may succeed but lack user information

#### Recommended Policies:

- **CORS-S3Origin** (`88a5eaf4-2fd4-4709-b370-b4c650ea3fcf`): Forwards all headers (recommended for authenticated endpoints)
- **Custom policy**: Create specific policy to forward only authentication headers for better performance

## SAML Configuration

The SAML metadata is embedded directly in `lambda-edge-samlify.js`:

### Identity Provider (IdP) Metadata

- **AWS Identity Center**: Configured for the Hearst production environment
- **Entity ID**: `https://portal.sso.us-east-1.amazonaws.com/saml/assertion/NzQ4OTA2OTEyNDY5X2lucy00NGQ2M2ZjOGM2OWUyNGJl`
- **SSO URL**: AWS Identity Center SAML endpoint
- **Certificate**: X.509 certificate for signature validation (embedded in metadata)

### Service Provider (SP) Metadata

- **Entity ID**: `https://change-management.ccoe.hearst.com`
- **ACS URL**: `https://change-management.ccoe.hearst.com/saml/acs`
- **NameID Format**: Email address format
- **Binding**: HTTP-POST for assertion consumer service

### Configuration Notes

- Metadata is currently hardcoded in the JavaScript for Lambda@Edge performance
- For production enhancement, see the caching strategy in `.kiro/specs/lambda-edge-saml-validation/`
- Certificate validation is simplified in the current implementation

For the full production SAML validation implementation, see the spec in `.kiro/specs/lambda-edge-saml-validation/`.

## Session Management and Monitoring

### Session Configuration

The Lambda@Edge function implements a sliding window session refresh mechanism with dual timeout controls:

- **Idle Timeout**: 3 hours (configurable via `SESSION_IDLE_TIMEOUT_MS`)
- **Absolute Maximum**: 12 hours (configurable via `SESSION_ABSOLUTE_MAX_MS`)
- **Refresh Threshold**: 10 minutes (configurable via `SESSION_REFRESH_THRESHOLD_MS`)
- **Cookie Max-Age**: 3 hours (configurable via `SESSION_COOKIE_MAX_AGE`)

#### Environment Variables

The following environment variables can be set to customize session timeout behavior:

| Variable | Default | Description | Valid Range |
|----------|---------|-------------|-------------|
| `SESSION_IDLE_TIMEOUT_MS` | `10800000` (3 hours) | Time in milliseconds before an idle session expires | > 0, must be < ABSOLUTE_MAX_MS |
| `SESSION_ABSOLUTE_MAX_MS` | `43200000` (12 hours) | Maximum session duration regardless of activity | > IDLE_TIMEOUT_MS |
| `SESSION_REFRESH_THRESHOLD_MS` | `600000` (10 minutes) | Time before expiration when refresh is triggered | > 0, must be < IDLE_TIMEOUT_MS |
| `SESSION_COOKIE_MAX_AGE` | `10800` (3 hours) | Browser cookie max-age in seconds | > 0, should match idle timeout |

#### Configuration Examples

**Default Configuration (3 hour idle, 12 hour max):**
```bash
# No environment variables needed - uses defaults
SESSION_IDLE_TIMEOUT_MS=10800000      # 3 hours
SESSION_ABSOLUTE_MAX_MS=43200000      # 12 hours
SESSION_REFRESH_THRESHOLD_MS=600000   # 10 minutes
SESSION_COOKIE_MAX_AGE=10800          # 3 hours
```

**Extended Configuration (8 hour idle, 24 hour max):**
```bash
SESSION_IDLE_TIMEOUT_MS=28800000      # 8 hours
SESSION_ABSOLUTE_MAX_MS=86400000      # 24 hours
SESSION_REFRESH_THRESHOLD_MS=1800000  # 30 minutes
SESSION_COOKIE_MAX_AGE=28800          # 8 hours
```

**Strict Configuration (1 hour idle, 4 hour max):**
```bash
SESSION_IDLE_TIMEOUT_MS=3600000       # 1 hour
SESSION_ABSOLUTE_MAX_MS=14400000      # 4 hours
SESSION_REFRESH_THRESHOLD_MS=300000   # 5 minutes
SESSION_COOKIE_MAX_AGE=3600           # 1 hour
```

#### Configuration Validation

The function validates configuration at startup and will fail to deploy if:
- Any timeout value is zero or negative
- `ABSOLUTE_MAX_MS` is less than or equal to `IDLE_TIMEOUT_MS`
- `REFRESH_THRESHOLD_MS` is greater than or equal to `IDLE_TIMEOUT_MS`
- `COOKIE_MAX_AGE` is zero or negative

Configuration validation errors are logged to CloudWatch and will prevent the Lambda from starting.

### Session Lifecycle Events

The function logs structured session lifecycle events for monitoring:

1. **Session Creation** (`🆕 New session created`) - When a user successfully authenticates
2. **Session Validation** (`✅ Valid session found`) - When an existing session is validated
3. **Session Refresh** (`🔄 Session refresh triggered`) - When a session cookie is refreshed
4. **Session Expiration** (`❌ Session validation failed`) - When a session expires (idle or absolute)
5. **Session Errors** (`❌ Session cookie parse error`, `❌ Session validation error`) - When session processing fails

### CloudWatch Logs

Lambda@Edge logs are distributed across CloudWatch log groups in multiple regions. To find your logs:

1. **Log Group Pattern**: `/aws/lambda/us-east-1.<function-name>`
2. **Regions**: Check all regions where CloudFront has edge locations (primarily us-east-1, but can be others)
3. **Log Retention**: Configure retention period in CloudWatch settings

### CloudWatch Insights Queries

For detailed session analysis queries and monitoring dashboards, see [MONITORING.md](./MONITORING.md).

## File Management

### Checked into Git:
- `lambda-edge-samlify.js` - Function source code
- `package.json` & `package-lock.json` - Dependency definitions
- `lambda-edge-samlify.zip` - **Pre-built deployment package** (required for Terraform Cloud)
- Build tools (`Makefile`, `build.sh`, `README.md`)
- `MONITORING.md` - CloudWatch monitoring and observability guide

### Not Checked into Git:
- `node_modules/` - Raw dependencies (excluded via .gitignore)
- Temporary build artifacts
