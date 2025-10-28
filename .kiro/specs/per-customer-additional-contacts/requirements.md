# Requirements Document

## Introduction

This feature enables management of additional contacts (people without AWS Access) on a per-customer basis. Additional contacts are stored in S3, contain email addresses subscribed to particular topics, and are managed through dedicated web interfaces. The system ensures email address uniqueness across customer lists (except for the htsnonprod customer) and follows the existing architectural pattern of HTML → Frontend Node API → S3 → SQS → Golang Backend.

## Glossary

- **Additional Contact**: An email address representing a person without AWS Access who should receive notifications for a specific customer
- **Customer List**: A collection of additional contacts associated with a specific customer identifier
- **Topic Subscription**: The association between an additional contact and specific notification topics they should receive
- **Frontend Node API**: The Node.js Lambda function with function URL that handles API requests from the HTML interface
- **Golang Backend**: The Go Lambda function that processes S3 events via SQS and performs business logic operations
- **SES Contact List**: AWS Simple Email Service managed list of contacts for email distribution
- **Email Uniqueness Validation**: The process ensuring an email address belongs to only one customer list (except htsnonprod)
- **Contact Import**: The process of adding one or more email addresses to a customer's additional contacts list via CSV, Excel, or web form

## Requirements

### Requirement 1

**User Story:** As a customer manager, I want to view all additional contacts for a specific customer, so that I can see who is receiving notifications without AWS Access

#### Acceptance Criteria

1. WHEN the user navigates to the view additional contacts page, THE System SHALL display a list of all additional contacts for the selected customer
2. WHILE displaying the contact list, THE System SHALL show the email address and subscribed topics for each contact
3. THE System SHALL retrieve additional contact data from the customer-specific S3 key prefix
4. WHEN no additional contacts exist for a customer, THE System SHALL display an empty state with a call-to-action to add contacts
5. THE System SHALL support filtering and sorting of the displayed contact list

### Requirement 2

**User Story:** As a customer manager, I want to add new email addresses as additional contacts through multiple input methods, so that I can efficiently onboard contacts regardless of data format

#### Acceptance Criteria

1. THE System SHALL provide a web interface accepting per-customer, per-topic comma-separated, semicolon-separated, or carriage-return-separated email addresses
2. THE System SHALL accept CSV file uploads containing per-customer email addresses and topic subscriptions
3. THE System SHALL accept Excel file uploads containing per-customer email addresses and topic subscriptions
4. WHEN processing uploaded files, THE System SHALL validate that required columns (customer, email address, topics) are present
5. THE System SHALL provide example template CSV and Excel download links on the add contacts page
6. THE System SHALL parse and normalize email addresses from all input methods before validation

### Requirement 3

**User Story:** As a system administrator, I want to ensure email address uniqueness across customer lists, so that contacts do not receive duplicate notifications from multiple customers

#### Acceptance Criteria

1. WHEN new email addresses are submitted, THE System SHALL retrieve all existing additional contacts from all customer lists once before validation
2. IF an email address already exists in a different customer list, THEN THE System SHALL reject the addition and return an error message
3. WHERE the customer is htsnonprod, THE System SHALL allow email addresses that exist in other customer lists
4. THE System SHALL perform email uniqueness validation before adding contacts to SES
5. WHEN validation fails, THE System SHALL provide clear error messages indicating which email addresses are duplicates and their existing customer associations

### Requirement 4

**User Story:** As a developer, I want the additional contacts workflow to follow the existing architectural pattern, so that the system remains consistent and maintainable

#### Acceptance Criteria

1. WHEN the HTML interface submits contact data, THE Frontend Node API SHALL receive the request at a new endpoint
2. THE Frontend Node API SHALL write the contact data to a customer-specific S3 key prefix
3. WHEN S3 receives the new object, THE System SHALL generate an event into the customer-specific SQS queue
4. THE Golang Backend SHALL pop the SQS message and process the contact addition request
5. THE Golang Backend SHALL perform email uniqueness validation across all customer lists before adding contacts

### Requirement 5

**User Story:** As a customer manager, I want added contacts to be automatically subscribed to SES topic lists, so that they receive appropriate notifications

#### Acceptance Criteria

1. WHEN the Golang Backend validates a new contact, THE System SHALL add the email address to the mapped customer SES contact list
2. THE System SHALL subscribe the contact to the specified topics in SES
3. IF SES operations fail, THEN THE System SHALL retry with exponential backoff
4. THE System SHALL maintain idempotency for all SES contact addition operations
5. WHEN contact addition completes successfully, THE System SHALL update the S3 object with the final status

### Requirement 6

**User Story:** As a customer manager, I want to receive clear feedback on contact import operations, so that I know which contacts were added successfully and which failed

#### Acceptance Criteria

1. WHEN contact import completes, THE System SHALL return a summary showing successful additions and failures
2. THE System SHALL provide specific error messages for each failed email address
3. THE System SHALL indicate whether failures were due to uniqueness violations, invalid email formats, or SES errors
4. THE System SHALL log all contact addition operations for audit purposes
5. WHEN partial success occurs, THE System SHALL complete all successful additions and report all failures

### Requirement 7

**User Story:** As a customer manager, I want contact import operations to follow a workflow state progression, so that I can review and approve contact additions before they are finalized

#### Acceptance Criteria

1. WHEN a contact import is initiated, THE System SHALL generate a unique TACT-GUID identifier for the import operation
2. THE System SHALL create the import request object at S3 key `drafts/{TACT-GUID}.json` with status field set to "draft"
3. WHEN the user submits the import, THE System SHALL update the status field to "submitted" and move the object from `drafts/` to `customers/{customer-code}/{TACT-GUID}.json` for each relevant customer and save one copy to `archive/{TACT-GUID}.json`
4. WHEN the import is approved, THE System SHALL update the status field to "approved" on all copies
5. WHEN the Golang Backend completes processing, THE System SHALL update the status field to "completed" on all copies

### Requirement 8

**User Story:** As a system administrator, I want contact data stored securely in S3 with proper structure, so that the system can efficiently retrieve and manage contacts

#### Acceptance Criteria

1. THE System SHALL store additional contact import requests in S3 using workflow state-based key prefixes
2. THE System SHALL store contact data in JSON format with TACT-GUID, email addresses, topics, customer identifier, and metadata fields
3. THE System SHALL use S3 object metadata to track status, version, completion timestamps, and user identity
4. THE System SHALL include x-amz-meta-status, x-amz-meta-version, x-amz-meta-completed-at, x-amz-meta-completed-by, and x-amz-meta-object-id metadata fields
5. THE System SHALL support concurrent read and write operations on contact data
6. THE System SHALL maintain data consistency when multiple contact import operations occur simultaneously
