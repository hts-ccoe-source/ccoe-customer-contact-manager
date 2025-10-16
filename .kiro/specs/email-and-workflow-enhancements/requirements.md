# Requirements Document

## Introduction

This feature enhances the customer contact management system by improving approval email workflows and integrating Typeform surveys for customer feedback collection. The enhancement includes configurable email links that direct users to filtered approval views, links that direct users to TypeForm surveys when experiences are closed, embedded surveys within the portal, and real-time survey result collection via webhooks. This will enable better user experience through direct navigation to relevant approvals and provide valuable feedback data for continuous improvement.

## Requirements

### Requirement 1

**User Story:** As an approver, I want to receive approval emails with a direct link to my customer-specific approvals page, so that I can quickly access and review only the items relevant to my organization.

#### Acceptance Criteria

1. WHEN an approval email is generated THEN the system SHALL include a URL with the customer code as a parameter
2. WHEN the customer code parameter is present in the URL THEN the approvals page SHALL automatically filter to show only that customer's approvals
3. IF the base FQDN needs to be changed THEN the system SHALL support configuration updates without code changes
4. WHEN the base FQDN is configured THEN the system SHALL use this value for all generated approval email links
5. WHEN the approval email is sent THEN the link SHALL be clearly visible and actionable within the email template

### Requirement 2

**User Story:** As a system administrator, I want the base FQDN for approval links to be configurable, so that I can easily update the domain (e.g., to "https://contact.ccoe.hearst.com/") without modifying code.

#### Acceptance Criteria

1. WHEN the system initializes THEN it SHALL load the base FQDN from a configuration source
2. IF the base FQDN is not configured THEN the system SHALL use a sensible default value
3. WHEN the configuration is updated THEN the system SHALL apply the new base FQDN to all subsequently generated links
4. WHEN generating approval URLs THEN the system SHALL concatenate the base FQDN with the appropriate path and customer code parameter

### Requirement 3

**User Story:** As a customer, I want to provide feedback on completed announcements and changes through a survey, so that the organization can improve their services.

#### Acceptance Criteria

1. WHEN an announcement or change is completed THEN the system SHALL include a Typeform survey link in the completion email
1a. WHEN an announcement or change is closed THEN the system SHALL provide links that direct users to Typeform surveys
2. WHEN the completion email is generated THEN it SHALL include both a clickable URL and a QR code for the survey
3. WHEN the survey is accessed THEN it SHALL contain a Net Promoter Score (NPS) question
4. WHEN the survey is accessed THEN it SHALL contain a question asking "Was this [announcement/change] excellent?"
5. IF possible THEN the survey SHALL be embedded directly in the email with interactive buttons or score line

### Requirement 4

**User Story:** As a system administrator, I want surveys to be automatically created via the Typeform Create API when announcements or changes are completed, so that feedback collection is automated and consistent.

#### Acceptance Criteria

1. WHEN an announcement or change is marked as completed THEN the system SHALL call the Typeform Create API
2. WHEN creating a survey THEN the system SHALL include an NPS question with a 0-10 scale
3. WHEN creating a survey THEN the system SHALL include a yes/no question "Was this [announcement/change] excellent?"
4. WHEN the survey is created THEN the system SHALL store the survey ID and URL for reference
5. IF the Typeform API call fails THEN the system SHALL log the error and continue without blocking the completion workflow

### Requirement 5

**User Story:** As a customer, I want to access surveys directly within the portal under a survey tab for my customer's items, so that I can provide feedback without leaving the application.

#### Acceptance Criteria

1. WHEN viewing a customer contact item THEN the system SHALL display a new "Survey" tab
2. WHEN the Survey tab is selected THEN the system SHALL embed the Typeform survey using the Typeform Embed SDK
3. WHEN the embedded survey loads THEN it SHALL use the customer code to filter and display only surveys relevant to that customer
4. WHEN the survey is embedded THEN it SHALL be fully functional and allow submission within the portal
5. WHEN the survey is submitted THEN the user SHALL receive appropriate confirmation feedback

### Requirement 6

**User Story:** As a system administrator, I want survey responses to be collected in real-time via Typeform webhooks, so that I can analyze feedback data immediately.

#### Acceptance Criteria

1. WHEN a survey is submitted THEN Typeform SHALL send a webhook notification to the system
2. WHEN the webhook is received THEN the system SHALL validate the webhook signature for security
3. WHEN the webhook payload is validated THEN the system SHALL extract the survey response data
4. WHEN survey response data is extracted THEN the system SHALL store it in S3 under a results key prefix
5. WHEN storing survey results THEN the system SHALL organize them using the S3 key structure `surveys/results/{customer_code}/{object_id}/{timestamp}-{survey_id}.json` for easy retrieval by customer and contact object

### Requirement 7

**User Story:** As a developer, I want the webhook endpoint to be implemented using API Gateway and Lambda, so that the system can scale automatically and handle webhook requests reliably.

#### Acceptance Criteria

1. WHEN the system is deployed THEN it SHALL include an API Gateway endpoint for Typeform webhooks
2. WHEN a webhook request is received THEN API Gateway SHALL route it to a Lambda function
3. WHEN the Lambda function processes the webhook THEN it SHALL handle errors gracefully and return appropriate HTTP status codes
4. WHEN the Lambda function succeeds THEN it SHALL return a 200 status code to Typeform
5. IF the Lambda function fails THEN it SHALL return an appropriate error status code and log the failure
6. WHEN processing webhook data THEN the Lambda function SHALL support idempotent operations to handle duplicate webhook deliveries

### Requirement 8

**User Story:** As a system administrator, I want survey results to be stored in a structured format in S3, so that they can be easily processed and displayed in the portal later.

#### Acceptance Criteria

1. WHEN storing survey results THEN the system SHALL use a consistent S3 key structure (e.g., `surveys/results/{customer_code}/{object_id}/{survey_id}/{timestamp}-{token}.json`)
2. WHEN storing survey results THEN the system SHALL include all response data, metadata, and timestamps
3. WHEN storing survey results THEN the system SHALL ensure the data is in JSON format for easy parsing
4. IF S3 storage fails THEN the system SHALL retry with exponential backoff within Lambda execution time limits and log the error
