# Multi-Customer Email Distribution Architecture

## Introduction

This spec defines the requirements for a multi-customer email distribution system that supports change management and other notification use cases across multiple customer AWS Organizations in a Managed Service Provider (MSP) environment. The system includes a complete web portal for change creation and management, Lambda-based backend services, and a comprehensive Go CLI (AWS Alternate Contact Manager) that handles SES email management, SQS message processing, alternate contact management, and Identity Center integration. The architecture supports ~30 customer organizations while maintaining proper isolation and security boundaries.

The intent is for the CCOE team to use the <https://change-management.ccoe.hearst.com> website (static html, css, and javascript hosted on a custom domain with a valid auto-renewing SSL Certificate) to write a Change Control request once and be able to select which customer(s) it applies to.  The front-end creates a `json` message that is placed into an S3 bucket.  The bucket has notifications enabled and when the frontend saves the `json` at a key prefix, one sqs queue for each customer gets a message. The main Go Lang (CLI and Lambda) backend process gets the event from sqs, gets the corresponding `json` object from s3, then the backend assumes a new Role inside each customer own SES-enabled AWS Organization member Account (`common-nonprod` currently for `htsnonprod`) and uses the customers own List Management service to send templated emails.

## Requirements

### Requirement 1: Customer Isolation and Security

**User Story:** As a Managed Service Provider, I want each customer to have their own isolated SES contact lists and email management with proper authentication and authorization, so that customers cannot access or interfere with each other's email subscriptions.

#### Acceptance Criteria

1. WHEN a change affects multiple customers THEN each customer's emails SHALL be sent from their own organization's SES account using customer-specific IAM roles
2. WHEN a customer manages their subscriptions THEN they SHALL only have access to their own organization's contact lists through role-based access control
3. WHEN email sending occurs THEN it SHALL use the appropriate customer's AWS credentials and SES configuration from the consolidated config.json
4. IF a customer's SES service is unavailable THEN other customers' email delivery SHALL NOT be affected
5. WHEN users access the web portal THEN they SHALL be authenticated via AWS Identity Center SAML integration
6. WHEN users submit changes THEN their identity SHALL be validated and logged for audit purposes
7. WHEN Lambda functions process requests THEN they SHALL validate user permissions through Lambda@Edge authentication headers

### Requirement 2: Web Portal Change Management

**User Story:** As a change manager, I want to create, edit, and manage change requests through a comprehensive web portal that supports multi-customer distribution, so that I can efficiently manage changes across all customer organizations.

#### Acceptance Criteria

1. WHEN I access the web portal THEN I SHALL see a dashboard with statistics, recent changes, and quick actions
2. WHEN I create a new change THEN I SHALL be able to select multiple customer organizations using checkboxes with select-all/clear-all functionality
3. WHEN I fill out the change form THEN I SHALL provide required fields including title, customers, implementation plan, schedule, and rollback plan
4. WHEN I submit a change THEN the system SHALL generate a unique change ID and upload metadata to S3 for each selected customer plus archive storage
5. WHEN I save a draft THEN it SHALL be stored locally and optionally on the server for later editing
6. WHEN I view my changes THEN I SHALL see separate tabs for drafts, submitted changes, and all changes with filtering capabilities
7. WHEN I search for changes THEN I SHALL be able to filter by status, date range, customer, and text search
8. WHEN I edit an existing change THEN the system SHALL increment the version number and preserve change history

### Requirement 3: Lambda Backend Services

**User Story:** As a system architect, I want robust Lambda-based backend services that handle authentication, metadata management, and S3 operations, so that the web portal has reliable API endpoints for all operations.

#### Acceptance Criteria

1. WHEN users access protected endpoints THEN Lambda@Edge SHALL validate SAML authentication and set user context headers
2. WHEN changes are uploaded THEN the enhanced metadata Lambda SHALL process uploads, validate data, and store to multiple S3 locations
3. WHEN users request change data THEN the Lambda SHALL provide CRUD operations for changes, drafts, and search functionality
4. WHEN S3 uploads occur THEN the system SHALL upload to customers/{code}/ prefixes for each customer plus archive/ for permanent storage
5. WHEN API calls are made THEN they SHALL include proper CORS headers and error handling
6. WHEN authentication fails THEN the system SHALL return appropriate HTTP status codes and redirect to Identity Center
7. WHEN multiple customers are selected THEN the Lambda SHALL create parallel uploads with progress tracking and error isolation

### Requirement 4: SQS Message Processing and Email Distribution

**User Story:** As a DevOps engineer, I want automated SQS message processing that triggers customer-specific email notifications, so that change notifications are delivered promptly without manual intervention.

#### Acceptance Criteria

1. WHEN S3 objects are created in customers/{code}/ prefixes THEN S3 events SHALL trigger customer-specific SQS queues
2. WHEN SQS messages are received THEN the Go CLI SHALL process them using the process-sqs-message action
3. WHEN processing SQS messages THEN the system SHALL extract customer codes from S3 keys and download metadata from S3
4. WHEN sending emails THEN the system SHALL use customer-specific SES roles and contact lists from the consolidated configuration
5. WHEN multiple changes occur simultaneously THEN the system SHALL handle concurrent processing without conflicts
6. WHEN a customer's email delivery fails THEN other customers' deliveries SHALL continue successfully
7. WHEN processing messages THEN the system SHALL support both S3 event notifications and legacy SQS message formats

### Requirement 5: SES Contact List Management

**User Story:** As an email administrator, I want comprehensive SES contact list management capabilities through the Go CLI, so that I can manage email subscriptions, topics, and suppression lists for each customer organization.

#### Acceptance Criteria

1. WHEN managing contact lists THEN the CLI SHALL support create-list, describe-list, delete-list, and list-contacts actions
2. WHEN managing contacts THEN the CLI SHALL support add-contact, remove-contact, and describe-contact operations
3. WHEN managing topics THEN the CLI SHALL support topic expansion with group prefixes and manage-topic for configuration updates
4. WHEN managing subscriptions THEN the CLI SHALL support bulk subscribe/unsubscribe operations with dry-run capabilities
5. WHEN managing suppression lists THEN the CLI SHALL support suppress and unsuppress actions for bounce and complaint handling
6. WHEN assuming customer roles THEN the CLI SHALL use customer-specific SES role ARNs from the configuration
7. WHEN performing operations THEN the CLI SHALL provide detailed progress reporting and error handling with automatic backups

### Requirement 6: Identity Center Integration and User Management

**User Story:** As a user administrator, I want integration with AWS Identity Center for user management and automated contact imports, so that I can efficiently manage user access and email subscriptions based on group memberships.

#### Acceptance Criteria

1. WHEN listing Identity Center users THEN the CLI SHALL support both single user and bulk user operations with rate limiting
2. WHEN retrieving group memberships THEN the CLI SHALL provide user-centric and group-centric views with CCOE cloud group filtering
3. WHEN importing contacts THEN the CLI SHALL automatically subscribe users to topics based on their Identity Center group memberships
4. WHEN processing imports THEN the CLI SHALL support dry-run mode and provide detailed progress reporting
5. WHEN managing concurrent operations THEN the CLI SHALL use configurable concurrency limits and requests-per-second rate limiting
6. WHEN role assumption is required THEN the CLI SHALL support management account role ARNs for cross-account Identity Center access
7. WHEN data files exist THEN the CLI SHALL auto-detect Identity Center instance IDs to reduce required parameters

### Requirement 7: Configuration Management and Operational Excellence

**User Story:** As a system administrator, I want consolidated configuration management and comprehensive operational capabilities, so that I can efficiently manage the multi-customer system with proper monitoring and error handling.

#### Acceptance Criteria

1. WHEN configuring the system THEN all settings SHALL be consolidated in a single config.json file with customer mappings, SES roles, and SQS queue ARNs
2. WHEN validating configurations THEN the system SHALL provide validation commands for customer codes, S3 events, and system connectivity
3. WHEN monitoring operations THEN the system SHALL provide detailed logging, progress tracking, and status reporting for all operations
4. WHEN errors occur THEN they SHALL be isolated to the affected customer with detailed error messages and automatic retry mechanisms
5. WHEN performing bulk operations THEN the system SHALL support dry-run modes and provide comprehensive summaries
6. WHEN managing lifecycle policies THEN S3 SHALL automatically clean up operational files in customers/ prefixes after 30 days while preserving archive/ permanently
7. WHEN troubleshooting issues THEN the system SHALL provide help commands, validation tools, and detailed error diagnostics