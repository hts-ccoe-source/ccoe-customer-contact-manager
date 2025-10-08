# Multi-Customer Email Distribution Architecture

## Introduction

This spec defines the requirements for a multi-customer email distribution system that supports change management and other notification use cases across multiple customer AWS Organizations in a Managed Service Provider (MSP) environment. The system includes a complete web portal for change creation and management, Lambda-based backend services, and a comprehensive Go CLI (CCOE Customer Contact Manager) that handles SES email management, SQS message processing, alternate contact management, and Identity Center integration. The architecture supports ~30 customer organizations while maintaining proper isolation and security boundaries.

The intent is for the CCOE team to use the <https://change-management.ccoe.hearst.com> website (static html, css, and javascript hosted on a custom domain with a valid auto-renewing SSL Certificate) to write a Change Control request once and be able to select which customer(s) it applies to.  The front-end creates a `json` message that is placed into an S3 bucket.  The bucket has notifications enabled and when the frontend saves the `json` at a key prefix, one sqs queue for each customer gets a message. The main Go Lang (CLI and Lambda) backend process gets the event from sqs, gets the corresponding `json` object from s3, then the backend assumes a new Role inside each customer own SES-enabled AWS Organization member Account (`common-nonprod` currently for `htsnonprod`) and uses the customers own List Management service to send templated emails.

For the change management portal, drafts should not generate email messages. Once a CR is submitted, it should send notifications to the appropriate SES topic (`aws-announce` for announcements, `aws-approval` for approval requests) for all the customers who were selected.


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

### Requirement 2: CCOE Change Management Portal Business Logic

**User Story:** As a CCOE team member, I want to use the change-management.ccoe.hearst.com website to write a Change Control request once and select which customer(s) it applies to, so that I can efficiently distribute change notifications across multiple customer organizations.

#### Acceptance Criteria

1. WHEN I access https://change-management.ccoe.hearst.com THEN I SHALL see a static HTML/CSS/JavaScript website hosted on a custom domain with valid auto-renewing SSL certificate
2. WHEN I create a new change request THEN I SHALL be able to select multiple customer organizations using checkboxes with select-all/clear-all functionality
3. WHEN I fill out the change form THEN I SHALL provide required fields including title, customers, implementation plan, schedule, and rollback plan
4. WHEN I save a draft THEN it SHALL be stored locally and optionally on the server WITHOUT generating any email messages
5. WHEN I submit a change request THEN the frontend SHALL create a JSON message and place it into an S3 bucket at customer-specific key prefixes
6. WHEN a change is submitted THEN it SHALL send notifications to the appropriate SES topic based on change status for all selected customers
7. WHEN I view my changes THEN I SHALL see separate tabs for drafts, submitted changes, and all changes with filtering capabilities
8. WHEN I search for changes THEN I SHALL be able to filter by status, date range, customer, and text search

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

### Requirement 4: S3 Event Notification and SQS Processing Business Logic

**User Story:** As a system architect, I want S3 bucket notifications to trigger customer-specific SQS queues when JSON messages are saved, so that each customer's Go backend process can independently handle their change notifications.

#### Acceptance Criteria

1. WHEN the frontend saves JSON at customer-specific key prefixes (customers/{code}/) THEN S3 bucket notifications SHALL be enabled to trigger one SQS queue for each customer
2. WHEN the frontend saves JSON at the drafts/ key prefix THEN S3 bucket notifications SHALL NOT be triggered and no SQS messages SHALL be generated
3. WHEN SQS messages are received THEN the main Go Lang backend process (CLI and Lambda) SHALL get the event from SQS
4. WHEN processing SQS events THEN the backend SHALL get the corresponding JSON object from S3 using the event metadata
5. WHEN processing change requests THEN the backend SHALL assume a new Role inside each customer's own SES-enabled AWS Organization member account
6. WHEN sending emails THEN the system SHALL use the customer's own List Management service (SES) to send templated emails
7. WHEN processing for htsnonprod customer THEN the system SHALL use the `common-nonprod` account for SES operations
8. WHEN multiple changes occur simultaneously THEN each customer's SQS queue SHALL process independently without conflicts

### Requirement 5: Customer-Specific SES Email Template Processing

**User Story:** As an email administrator, I want the Go backend to use each customer's own SES List Management service to send templated emails, so that change notifications are delivered using customer-specific email configurations and contact lists.

#### Acceptance Criteria

1. WHEN processing change requests THEN the Go backend SHALL assume customer-specific IAM roles to access their SES services
2. WHEN sending change notifications THEN the system SHALL select the appropriate SES topic based on change status: `aws-announce` for announcements and `aws-approval` for approval requests
3. WHEN managing contact lists THEN the CLI SHALL support create-list, describe-list, delete-list, and list-contacts actions per customer
4. WHEN managing topics THEN the CLI SHALL support topic expansion with group prefixes (aws-, wiz-) and manage-topic for configuration updates
5. WHEN managing subscriptions THEN the CLI SHALL support bulk subscribe/unsubscribe operations with dry-run capabilities per customer
6. WHEN managing suppression lists THEN the CLI SHALL support suppress and unsuppress actions for bounce and complaint handling per customer
7. WHEN sending templated emails THEN the system SHALL use customer-specific email templates and branding from their SES configuration
8. WHEN determining email recipients THEN the system SHALL select SES topics based on change status: `aws-announce` for change announcements, `aws-approval` for approval requests, and other topics as defined by business rules

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

### Requirement 7: Draft vs. Submitted Change Request Business Logic

**User Story:** As a CCOE team member, I want clear separation between draft and submitted change requests, so that I can work on changes without triggering email notifications until I'm ready to submit them.

#### Acceptance Criteria

1. WHEN I save a change as draft THEN the system SHALL NOT generate any email messages, SQS notifications, or S3 event triggers
2. WHEN I save a draft THEN it SHALL be stored locally in browser storage and optionally on the server in the drafts/ S3 prefix WITHOUT triggering any SQS events
3. WHEN I submit a change request THEN it SHALL transition from draft status to submitted status and trigger the full notification workflow
4. WHEN a change is submitted THEN it SHALL automatically send notifications to the appropriate SES topic based on change status for all selected customers
5. WHEN I edit a submitted change THEN it SHALL create a new version while preserving the original submission
6. WHEN viewing changes THEN I SHALL clearly see the distinction between draft and submitted changes in the interface
7. WHEN a draft exists THEN I SHALL be able to load it, continue editing, and either save as draft again or submit it
8. WHEN S3 event notifications are configured THEN they SHALL ONLY apply to the customers/{code}/ prefix and SHALL NOT apply to the drafts/ prefix

### Requirement 8: Configuration Management and Operational Excellence

**User Story:** As a system administrator, I want consolidated configuration management and comprehensive operational capabilities, so that I can efficiently manage the multi-customer system with proper monitoring and error handling.

#### Acceptance Criteria

1. WHEN configuring the system THEN all settings SHALL be consolidated in a single config.json file with customer mappings, SES roles, and SQS queue ARNs
2. WHEN configuring customer mappings THEN each customer SHALL have their own SES role ARN pointing to their SES-enabled AWS Organization member account
3. WHEN configuring htsnonprod customer THEN it SHALL point to the `common-nonprod` account for SES operations
4. WHEN validating configurations THEN the system SHALL provide validation commands for customer codes, S3 events, and system connectivity
5. WHEN monitoring operations THEN the system SHALL provide detailed logging, progress tracking, and status reporting for all operations
6. WHEN errors occur THEN they SHALL be isolated to the affected customer with detailed error messages and automatic retry mechanisms
7. WHEN managing lifecycle policies THEN S3 SHALL automatically clean up operational files in customers/ prefixes after 30 days while preserving archive/ permanently