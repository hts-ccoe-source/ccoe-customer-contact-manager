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

## File Management

### Checked into Git:
- `lambda-edge-samlify.js` - Function source code
- `package.json` & `package-lock.json` - Dependency definitions
- `lambda-edge-samlify.zip` - **Pre-built deployment package** (required for Terraform Cloud)
- Build tools (`Makefile`, `build.sh`, `README.md`)

### Not Checked into Git:
- `node_modules/` - Raw dependencies (excluded via .gitignore)
- Temporary build artifacts
