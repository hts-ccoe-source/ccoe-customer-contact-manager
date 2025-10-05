# Upload Metadata Lambda Function

This directory contains the Lambda function for uploading metadata collected via the HTML S3 website.

## Files

- `upload-metadata-lambda.js` - Main Lambda function code
- `package.json` - Node.js dependencies and configuration
- `package-lock.json` - Locked dependency versions (generated)
- `upload-metadata-lambda.zip` - Pre-built deployment package (checked into git for Terraform Cloud)

## Dependencies

- `aws-sdk` - AWS SDK for JavaScript (S3, SQS operations)

## Deployment

This function is deployed using Terraform Cloud. The pre-built `upload-metadata-lambda.zip` file is checked into git and used by Terraform's Lambda function resource in `main.tf`.

## Development Workflow

### For Code Changes:

1. Make changes to `upload-metadata-lambda.js`
2. Update dependencies in `package.json` if needed
3. Build and commit the deployment package:

   ```bash
   cd upload_lambda
   npm install                    # Install/update dependencies
   make build                     # Create deployment package
   git add upload-metadata-lambda.zip
   git commit -m "Update upload metadata Lambda function"
   ```

4. Push to trigger Terraform Cloud deployment

### For Terraform Cloud:

- **Pre-built zip required**: Terraform Cloud uses the checked-in `upload-metadata-lambda.zip`
- **No build step in cloud**: Dependencies must be bundled locally before commit
- **Version control**: The zip file is tracked in git for reproducible deployments

## Build Commands

- `make build` - Install dependencies and create deployment package
- `make package` - Create package only (assumes dependencies installed)
- `make clean` - Remove build artifacts and node_modules
- `./build.sh` - Direct build script execution

## Architecture

This Lambda function handles:

- Metadata upload processing from the HTML S3 website
- Authentication validation via headers from Lambda@Edge
- User authorization for change management operations
- S3 operations for storing uploaded metadata
- SQS integration for processing notifications

## Integration with Lambda@Edge

The function expects authentication headers set by the Lambda@Edge SAML function:
- `x-user-email` - Authenticated user's email address
- `x-user-groups` - User's group memberships (optional)

## File Management

### Checked into Git:
- `upload-metadata-lambda.js` - Function source code
- `package.json` & `package-lock.json` - Dependency definitions
- `upload-metadata-lambda.zip` - **Pre-built deployment package** (required for Terraform Cloud)
- Build tools (`Makefile`, `build.sh`, `README.md`)

### Not Checked into Git:
- `node_modules/` - Raw dependencies (excluded via .gitignore)
- Temporary build artifacts