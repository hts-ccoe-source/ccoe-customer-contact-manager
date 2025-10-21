# Development Standards

## project-specific information you **always need**

There are four main components inside this project:

1. SAML authentication with Identity Center via CloudFront Lambda at Edge `./lambda/saml_auth` built with `make package-saml-lambda` with CloudWatch logs: ``
1. the frontend node js api lambda with function URL `./lambda/upload_labmda` built with `make package-upload-lambda` with CloudWatch logs: `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-api`
1. the web portal UI `./html` creates an s3 object event built and deployed with `./deploy-website.sh`
1. the 'backend golang lambda' processes the s3 event via sqs `./main.go` and `./internal/` built with `make package-golang-lambda` with CloudWatch logs: `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-backend`
1. the 'backend golang lambda' also has a CLI mode that will execute inside an ESC 'governance' cluster and import contacts into SES topic lists based on AWS Identity Center roles `./SESConfig.json`

## Language and Framework Preferences

- **Always first choice**: Write Go Lang using the AWS SDK v2 for Go
- Use good internal Go Lang subdirectory structure
- Include Makefile in project root

## Project Structure

- Keep as few files in the project root as possible
- When possible, work with existing files rather than creating new ones
- Put markdown summaries into a `./summaries` subdirectory

## Execution Modes

All applications must support two main modes of execution:

- Manual execution via CLI options
- Lambda-wrapper for event-driven workflows

## Logging

- Support structured logging with `slog`
- Feature both JSON and text output types

## Performance and Reliability

- All operations support full concurrency
- All operations support full API rate limiting with exponential backoff and retries
- All write/put/update/create/delete operations are always fully idempotent

## Operation Patterns

### Dry Run Support

- Read operations never implement `--dry-run`
- All write/put/update/create/delete operations always implement `--dry-run` to see what would happen without actually doing it

### Processing Patterns

- All operations can process one object or all objects
- When processing all objects, the code calls the process-one-object function multiple times
