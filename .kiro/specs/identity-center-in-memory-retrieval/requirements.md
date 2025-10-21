# Requirements Document

## Introduction

Enhance the CLI functionality to retrieve AWS Identity Center user and group membership information in-memory and pass it directly to the `import-aws-contact-all` command, eliminating the dependency on pre-generated JSON files.

## Glossary

- **Identity Center**: AWS IAM Identity Center (formerly AWS SSO) service that manages user identities and group memberships
- **CLI**: Command Line Interface for the CCOE Customer Contact Manager
- **SES**: Amazon Simple Email Service used for managing contact lists
- **Identity Center Role**: IAM role in the management account that has permissions to read Identity Center data
- **SES Role**: IAM role in each customer account that has permissions to manage SES contacts (stored as `ses_role_arn` in customer mappings)
- **In-Memory Processing**: Retrieving and processing data without writing intermediate files to disk
- **Idempotency**: The ability to execute the same operation multiple times without changing the result beyond the initial application

## Requirements

### Requirement 0

**User Story:** As a system administrator, I want to configure the Identity Center role ARN in the customer mapping configuration, so that each customer can have its own Identity Center access configuration.

#### Acceptance Criteria

1. THE customer mapping configuration SHALL support an optional `identity_center_role_arn` field for each customer
2. WHEN an Identity Center action is specified, WHERE the `identity_center_role_arn` field is present in the customer configuration, THE CLI SHALL use it to retrieve Identity Center data for that customer
3. WHEN an Identity Center action is specified, WHERE the `identity_center_role_arn` field is absent, THE CLI SHALL fall back to loading data from JSON files
4. WHEN the Identity Center role is assumed for an Identity Center action, THE CLI SHALL automatically discover the Identity Center instance ID from the account
5. WHEN discovering the Identity Center instance ID, THE CLI SHALL validate that exactly one Identity Center instance exists in the account

### Requirement 1

**User Story:** As a system administrator, I want to retrieve Identity Center data and import contacts to customer accounts concurrently, so that bulk imports complete efficiently without managing intermediate JSON files.

#### Acceptance Criteria

1. WHEN the `import-aws-contact-all` command is executed, THE CLI SHALL read the Identity Center role ARN from either the `--identity-center-role-arn` CLI flag or the customer mapping configuration
2. WHERE both the CLI flag and configuration are provided, THE CLI SHALL prioritize the CLI flag value
3. WHEN processing each customer concurrently, THE CLI SHALL assume the Identity Center role for that customer
4. WHEN the Identity Center role is assumed, THE CLI SHALL retrieve Identity Center user and group membership data in-memory
5. WHEN Identity Center data is retrieved for a customer, THE CLI SHALL assume the SES role for that same customer
6. WHEN the SES role is assumed, THE CLI SHALL import contacts using the in-memory Identity Center data
7. THE Identity Center role assumption and SES role assumption SHALL be atomic operations within each customer's processing
8. WHERE the Identity Center role ARN is not provided via CLI flag or configuration for a customer, THE CLI SHALL fall back to loading data from existing JSON files for that customer

### Requirement 2

**User Story:** As a developer, I want the Identity Center retrieval functions to return structured data, so that the data can be used by other functions without file I/O.

#### Acceptance Criteria

1. THE `HandleIdentityCenterUserListing` function SHALL return a slice of user data structures instead of only writing to files
2. THE `HandleIdentityCenterGroupMembership` function SHALL return a slice of group membership data structures instead of only writing to files
3. WHEN these functions are called, THE CLI SHALL support both returning data and optionally writing to files for backward compatibility
4. THE returned data structures SHALL match the format currently used in the JSON files
5. WHEN an error occurs during retrieval, THE functions SHALL return a descriptive error

### Requirement 3

**User Story:** As a system administrator, I want the import process to work seamlessly with both in-memory and file-based data sources, so that existing workflows are not disrupted.

#### Acceptance Criteria

1. THE `ImportAllAWSContacts` function SHALL accept an optional parameter for in-memory Identity Center data
2. WHERE in-memory data is provided, THE function SHALL use it directly without attempting to load files
3. WHERE in-memory data is not provided, THE function SHALL load data from JSON files as it currently does
4. THE function SHALL validate that the provided data (whether in-memory or from files) contains the required fields
5. WHEN processing users, THE function SHALL produce identical results regardless of data source
6. THE function SHALL support idempotent operations by checking existing contacts before adding or updating
7. WHEN a contact already exists with the same topics, THE function SHALL skip the update operation

### Requirement 4

**User Story:** As a system administrator, I want clear logging and error messages during in-memory retrieval, so that I can troubleshoot issues effectively.

#### Acceptance Criteria

1. WHEN processing each customer begins, THE CLI SHALL log the customer code and which roles will be assumed
2. WHEN in-memory retrieval begins for a customer, THE CLI SHALL log that it is retrieving Identity Center data via API
3. WHEN in-memory retrieval completes for a customer, THE CLI SHALL log the number of users and group memberships retrieved
4. IF in-memory retrieval fails for a customer, THE CLI SHALL log a descriptive error message and continue processing other customers
5. THE CLI SHALL log whether it is using in-memory data or file-based data for each customer's import operation
6. WHEN all customers are processed, THE CLI SHALL log a summary including total customers, successes, failures, and skipped

### Requirement 5

**User Story:** As a developer, I want the code to follow existing patterns and conventions, so that it is maintainable and consistent with the rest of the codebase.

#### Acceptance Criteria

1. THE new functionality SHALL use direct STS role assumption for both Identity Center and SES roles
2. THE new functionality SHALL respect the `--dry-run` flag for testing without making changes
3. THE new functionality SHALL support the existing `--max-concurrency` and `--requests-per-second` flags
4. THE code SHALL follow Go best practices including error handling and structured logging
5. THE implementation SHALL reuse existing helper functions where applicable

### Requirement 6

**User Story:** As a system administrator, I want concurrent processing of multiple customers, so that bulk imports complete quickly.

#### Acceptance Criteria

1. WHEN processing multiple customers, THE CLI SHALL process customer imports concurrently up to the `--max-concurrency` limit
2. WHEN processing a single customer, THE CLI SHALL perform Identity Center role assumption, data retrieval, SES role assumption, and contact import as atomic sequential operations
3. WHEN a customer's processing fails at any step, THE CLI SHALL log the error and continue processing other customers
4. THE CLI SHALL collect and report results from all customer imports including successes, failures, and skipped customers
5. WHEN rate limiting is configured, THE CLI SHALL apply rate limits within each customer's processing to avoid throttling
