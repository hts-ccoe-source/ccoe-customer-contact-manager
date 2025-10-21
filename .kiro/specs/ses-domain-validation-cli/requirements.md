# Requirements Document

## Introduction

This feature adds additional CLI functionality to the already extensive capabilities of the golang backend to configure SES domain validation resources in customer AWS accounts and manage corresponding Route53 DNS records in a centralized DNS account. The solution addresses the limitation of Terraform Cloud where cross-organization output sharing is not possible, requiring a programmatic approach to coordinate SES setup across multiple customer accounts.

## Glossary

- **CLI**: Command-line interface mode of the golang backend application
- **SES**: Amazon Simple Email Service - AWS email sending service
- **SESV2**: Amazon Simple Email Service API Version 2
- **DKIM**: DomainKeys Identified Mail - email authentication method
- **Route53**: AWS DNS web service
- **Customer Account**: Individual AWS accounts in the app-common tier for each customer organization
- **DNS Account**: Centralized AWS account in the production governance organization that manages Route53 hosted zones
- **Email Identity**: SESV2 resource representing a verified email address or domain for sending emails (supports both email addresses and domains with integrated DKIM configuration)
- **Verification Token**: AWS-generated token used to verify domain ownership via DNS TXT record
- **DKIM Tokens**: AWS-generated tokens (set of 3) used to configure DKIM signing via DNS CNAME records
- **Hosted Zone**: Route53 resource containing DNS records for a domain
- **IAM Role Assumption**: Process of temporarily assuming credentials from another AWS account

## Requirements

### Requirement 1

**User Story:** As a platform administrator, I want to configure SES domain validation resources in customer accounts via CLI, so that I can enable email sending capabilities without manual AWS console operations.

#### Acceptance Criteria

1. WHEN the CLI executes with the SES configuration command, THE CLI SHALL create an SESV2 email identity resource for `ccoe@hearst.com` in the target customer account
2. WHEN the CLI executes with the SES configuration command, THE CLI SHALL create an SESV2 email identity resource for `ccoe.hearst.com` domain in the target customer account
3. WHEN the CLI executes with the SES configuration command, THE CLI SHALL configure DKIM signing attributes for the domain email identity in the target customer account
4. WHERE the `--dry-run` flag is provided, THE CLI SHALL display the SES resources that would be created without making actual changes
5. WHEN SES resource creation completes successfully, THE CLI SHALL retrieve and output the domain verification token and three DKIM tokens for DNS configuration

### Requirement 2

**User Story:** As a platform administrator, I want to configure Route53 DNS records for SES validation in the centralized DNS account, so that domain ownership verification and DKIM signing are properly configured.

#### Acceptance Criteria

1. WHEN the CLI executes with the Route53 configuration command, THE CLI SHALL assume an IAM role in the DNS account to obtain credentials
2. WHEN creating DNS records, THE CLI SHALL create three CNAME records for DKIM tokens in the format `{token}._domainkey.{domain}` pointing to `{token}.dkim.amazonses.com`
3. WHEN creating DNS records, THE CLI SHALL create one TXT record for domain verification in the format `_amazonses.{domain}` containing the verification token
4. WHEN creating DNS records, THE CLI SHALL set a TTL value of 600 seconds for all created records
5. WHERE the `--dry-run` flag is provided, THE CLI SHALL display the Route53 records that would be created without making actual changes

### Requirement 3

**User Story:** As a platform administrator, I want to process multiple customer organizations in a single CLI execution with proper validation, so that I can efficiently configure SES and DNS for all customers while catching configuration errors early.

#### Acceptance Criteria

1. WHEN the CLI processes multiple organizations, THE CLI SHALL iterate through each organization configuration sequentially
2. WHEN processing an organization for Route53 operations, THE CLI SHALL validate that exactly three DKIM tokens are present before creating DNS records
3. WHEN processing an organization for Route53 operations, THE CLI SHALL validate that a verification token is present before creating DNS records
4. IF an organization configuration is invalid, THEN THE CLI SHALL log a warning message and continue processing remaining organizations
5. WHEN all organizations are processed, THE CLI SHALL output a summary of successful and failed operations

### Requirement 4

**User Story:** As a platform administrator, I want the CLI to support both individual and batch processing modes, so that I can configure a single customer or all customers based on operational needs.

#### Acceptance Criteria

1. WHEN the CLI is invoked with a specific customer identifier, THE CLI SHALL process only that customer's SES and DNS configuration
2. WHEN the CLI is invoked without a specific customer identifier, THE CLI SHALL process all customers defined in the configuration file
3. WHEN processing in batch mode, THE CLI SHALL continue processing remaining customers if one customer's configuration fails
4. WHEN processing completes, THE CLI SHALL return a non-zero exit code if any customer configuration failed
5. WHEN processing completes successfully for all customers, THE CLI SHALL return exit code zero

### Requirement 5

**User Story:** As a platform administrator, I want the CLI to implement idempotent operations with proper error handling, so that I can safely re-run commands without creating duplicate resources or encountering failures.

#### Acceptance Criteria

1. WHEN creating SES resources that already exist, THE CLI SHALL update existing resources rather than failing with a duplicate error
2. WHEN creating Route53 records that already exist, THE CLI SHALL use UPSERT action to update existing records
3. WHEN AWS API rate limits are encountered, THE CLI SHALL implement exponential backoff with retries up to a maximum of five attempts
4. IF AWS API operations fail after all retry attempts, THEN THE CLI SHALL log detailed error information including the AWS error code and message
5. WHEN Route53 changes exceed 1000 records, THE CLI SHALL split changes into multiple batches of 1000 records each

### Requirement 6

**User Story:** As a platform administrator, I want the CLI to load configuration from a JSON file, so that I can manage customer and DNS settings in a version-controlled format.

#### Acceptance Criteria

1. WHEN the CLI starts, THE CLI SHALL accept a `--config` flag specifying the path to a JSON configuration file
2. WHEN no `--config` flag is provided, THE CLI SHALL use a default configuration file path of `./SESConfig.json`
3. WHEN loading the configuration file, THE CLI SHALL validate that required fields are present including Route53 zone ID and zone name
4. IF the configuration file is missing or invalid JSON, THEN THE CLI SHALL return an error message and exit with a non-zero code
5. WHEN the configuration is loaded successfully, THE CLI SHALL parse organization entries including name, DKIM tokens, and verification tokens

### Requirement 7

**User Story:** As a platform administrator, I want the CLI to provide structured logging output, so that I can monitor operations and troubleshoot issues effectively.

#### Acceptance Criteria

1. WHEN the CLI executes operations, THE CLI SHALL output structured log messages using the slog package
2. WHEN processing each organization, THE CLI SHALL log the organization name being processed
3. WHEN creating each DNS record, THE CLI SHALL log the record name, type, and value
4. WHEN operations complete successfully, THE CLI SHALL output a success message with a count of created resources
5. WHERE the `--dry-run` flag is provided, THE CLI SHALL clearly indicate in log output that no changes were made

### Requirement 8

**User Story:** As a platform administrator, I want the CLI to integrate with the existing command structure, so that SES and Route53 operations follow consistent patterns with other CLI commands.

#### Acceptance Criteria

1. WHEN the CLI is invoked, THE CLI SHALL use the existing command category for SES operations
2. WHEN the CLI is invoked, THE CLI SHALL provide a new primary command category for Route53 operations
3. WHEN using SES commands, THE CLI SHALL support flags for AWS profile, region, and dry-run mode
4. WHEN using Route53 commands, THE CLI SHALL support flags for IAM role ARN to assume in the DNS account
5. WHEN using Route53 commands, THE CLI SHALL support flags for AWS profile, region, and dry-run mode
