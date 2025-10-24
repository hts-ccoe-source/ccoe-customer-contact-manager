# Requirements Document

## Introduction

This feature adds new multi-customer SES CLI actions (with `-all` suffix) that read customer configurations from `config.json` and execute SES management operations across all customers concurrently using their respective SES role ARNs. This brings consistency with the existing `import-aws-contact-all` pattern and enables centralized SES management across multiple customer accounts while maintaining backward compatibility with existing single-customer actions.

## Glossary

- **SES**: Amazon Simple Email Service - AWS email sending service
- **Customer**: An organizational unit with its own AWS account and SES configuration
- **SES Role ARN**: IAM role ARN used to assume permissions in a customer's AWS account for SES operations
- **Contact List**: SES v2 contact list containing email addresses and topic subscriptions
- **Topic**: A subscription category within an SES contact list (e.g., "aws-calendar", "aws-announce")
- **CLI**: Command Line Interface - the ccoe-customer-contact-manager executable
- **config.json**: Main configuration file containing customer mappings and SES role ARNs
- **SESConfig.json**: Configuration file defining SES topics and subscription settings
- **SubscriptionConfig.json**: Configuration file mapping topics to email addresses for bulk operations
- **Dry-run**: Preview mode that shows what would be done without making actual changes
- **Credential Manager**: Component that manages AWS credentials and role assumptions per customer

## Requirements

### Requirement 1

**User Story:** As a platform administrator, I want to manage SES topics across all customer accounts with a single command, so that I can ensure consistent topic configuration without manually operating on each customer.

#### Acceptance Criteria

1. WHEN the administrator executes `manage-topic-all` action, THE CLI SHALL read all customers from config.json and execute the topic management operation for each customer concurrently using their respective SES role ARN
2. WHEN a customer's SES role ARN is not configured in config.json, THE CLI SHALL skip that customer and log a warning message
3. WHEN the administrator uses the `--dry-run` flag with `manage-topic-all`, THE CLI SHALL preview all changes across all customers without making actual modifications
4. WHEN processing multiple customers, THE CLI SHALL display progress indicators showing which customer is currently being processed
5. WHEN the existing `manage-topic` action is used, THE CLI SHALL continue to operate on a single customer as before (backward compatibility)

### Requirement 2

**User Story:** As a platform administrator, I want to view SES contact list information across all customer accounts, so that I can audit and verify configurations without switching between accounts.

#### Acceptance Criteria

1. WHEN the administrator executes `describe-list-all` action, THE CLI SHALL retrieve and display contact list information from all customers in config.json concurrently
2. WHEN the administrator executes `list-contacts-all` action, THE CLI SHALL retrieve and display all contacts from all customers in config.json concurrently
3. WHEN the administrator executes `describe-topics-all` action, THE CLI SHALL retrieve and display all topic information from all customers in config.json concurrently
4. WHEN displaying multi-customer results, THE CLI SHALL clearly label which customer each result belongs to
5. WHEN a customer's SES operation fails, THE CLI SHALL continue processing remaining customers and report all failures at the end

### Requirement 3

**User Story:** As a platform administrator, I want the CLI to handle errors gracefully when operating on multiple customers, so that a failure in one customer doesn't prevent operations on other customers.

#### Acceptance Criteria

1. WHEN an SES operation fails for one customer, THE CLI SHALL log the error with customer context and continue processing remaining customers
2. WHEN all customers have been processed, THE CLI SHALL display a summary showing successful operations, failed operations, and skipped customers
3. WHEN a customer's SES role ARN cannot be assumed, THE CLI SHALL log the authentication failure and continue with remaining customers
4. WHEN the config.json file is missing or invalid, THE CLI SHALL display a clear error message and exit before attempting any operations
5. WHEN no customers are configured in config.json, THE CLI SHALL display an informative message and exit gracefully

### Requirement 4

**User Story:** As a platform administrator, I want to control concurrency when operating on multiple customers, so that I can balance performance with API rate limits and system resources.

#### Acceptance Criteria

1. WHEN processing multiple customers, THE CLI SHALL support a `--max-customer-concurrency` flag to control how many customers are processed in parallel
2. WHEN the `--max-customer-concurrency` flag is not specified, THE CLI SHALL default to processing all customers concurrently (one goroutine per customer)
3. WHEN the `--max-customer-concurrency` flag is set to a value lower than the number of customers, THE CLI SHALL limit concurrent processing to that number
4. WHEN the `--max-customer-concurrency` flag is set to a value higher than the number of customers, THE CLI SHALL ignore the limit and process all customers concurrently
5. WHEN processing customers concurrently, THE CLI SHALL ensure thread-safe logging and result aggregation
6. WHEN processing customers concurrently, THE CLI SHALL buffer log messages per customer and flush them as a block to prevent interleaved output
7. WHEN displaying buffered logs, THE CLI SHALL clearly separate each customer's logs with visual boundaries and customer identification

### Requirement 5

**User Story:** As a platform administrator, I want backward compatibility with existing single-customer operations, so that existing scripts and workflows continue to function without modification.

#### Acceptance Criteria

1. WHEN the administrator uses existing single-customer actions (without `-all` suffix), THE CLI SHALL operate only on the specified customer using existing behavior
2. WHEN the administrator provides a `-customer-code` flag with any SES action, THE CLI SHALL operate only on that specific customer (existing behavior)
3. WHEN the administrator provides a `-ses-role-arn` flag with single-customer actions, THE CLI SHALL use that role ARN instead of reading from config.json (existing behavior)
4. WHEN the administrator uses new `-all` actions, THE CLI SHALL require config.json to be present and SHALL NOT accept `-customer-code` or `-ses-role-arn` flags
5. WHEN the administrator uses legacy command patterns, THE CLI SHALL continue to support them without breaking changes

### Requirement 6

**User Story:** As a platform administrator, I want clear documentation of which SES actions have multi-customer variants, so that I understand the capabilities and can use them effectively.

#### Acceptance Criteria

1. WHEN the administrator runs `ses -action help`, THE CLI SHALL display all available actions including new `-all` variants
2. WHEN the administrator uses a `-all` action without config.json present, THE CLI SHALL display a clear error message indicating config.json is required
3. WHEN the administrator views the README documentation, THE documentation SHALL clearly list all `-all` actions and their usage
4. WHEN the administrator uses `--dry-run` with `-all` actions, THE CLI SHALL display example output showing what would be executed for each customer
5. WHEN the administrator provides invalid flags for `-all` actions (like `-customer-code`), THE CLI SHALL display helpful error messages explaining that `-all` actions operate on all customers from config.json
