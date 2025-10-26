# Requirements Document

## Introduction

This feature enhances the customer contact management system by improving approval email workflows and integrating Typeform surveys for customer feedback collection. The enhancement includes configurable email links that direct users to filtered approval views, links that direct users to TypeForm surveys when experiences are closed, embedded surveys within the portal, and real-time survey result collection via webhooks. This will enable better user experience through direct navigation to relevant approvals and provide valuable feedback data for continuous improvement.

## Requirements

### Requirement 1

**User Story:** As an approver, I want to receive approval emails with a direct link to my customer-specific approvals page, so that I can quickly access and review only the items relevant to my organization.

#### Acceptance Criteria

1. WHEN an approval email is generated THEN the system SHALL include a URL with the customer code as a parameter and the object ID as a parameter
2. WHEN the customer code parameter is present in the URL THEN the approvals page SHALL automatically filter to show only that customer's approvals
3. WHEN the object ID parameter is present in the URL THEN the approvals page SHALL automatically open the detail modal for that specific object
4. IF the base FQDN needs to be changed THEN the system SHALL support configuration updates without code changes
5. WHEN the base FQDN is configured THEN the system SHALL use this value for all generated approval email links
6. WHEN the approval email is sent THEN the link SHALL be clearly visible and actionable within the email template
7. WHEN a user accesses the approvals page with a customer code parameter THEN the system SHALL display the customer's logo image from the portal S3 bucket
8. WHEN the customer logo is displayed THEN it SHALL be positioned prominently on the approvals page
9. IF the customer logo image is not found THEN the system SHALL use the default placeholder logo from `assets/images/default-logo.png`

### Requirement 2

**User Story:** As a system administrator, I want the base FQDN for approval links to be configurable, so that I can easily update the domain (e.g., to "<https://contact.ccoe.hearst.com/>") without modifying code.

#### Acceptance Criteria

1. WHEN the system initializes THEN it SHALL load the base FQDN from a configuration source
2. IF the base FQDN is not configured THEN the system SHALL use a sensible default value
3. WHEN the configuration is updated THEN the system SHALL apply the new base FQDN to all subsequently generated links
4. WHEN generating approval URLs THEN the system SHALL concatenate the base FQDN with the appropriate path and customer code parameter

### Requirement 3

**User Story:** As a customer, I want to provide feedback on completed announcements and changes through a survey, so that the organization can improve their services.

#### Acceptance Criteria

1. WHEN an announcement or change is completed THEN the system SHALL include a Typeform survey link in the completion email
2. WHEN an announcement or change is closed THEN the system SHALL provide links that direct users to Typeform surveys
3. WHEN the completion email is generated THEN it SHALL include both a clickable URL and a QR code for the survey
4. WHEN the survey link in the email is clicked THEN it SHALL direct users to a portal survey page with the survey ID as a parameter
5. WHEN the portal survey page loads with a survey ID parameter THEN it SHALL use the Typeform autoclose feature to automatically close the survey after submission
6. WHEN the survey is accessed THEN it SHALL contain a Net Promoter Score (NPS) question
7. WHEN the survey is accessed THEN it SHALL contain a question asking "Was this [announcement/change] excellent?"

### Requirement 4

**User Story:** As a system administrator, I want surveys to be automatically created via the Typeform Create API when announcements or changes are completed, so that feedback collection is automated and consistent.

#### Acceptance Criteria

1. WHEN the backend golang lambda processes an SQS event THEN it SHALL recognize the 'completed' workflow state
2. WHEN the 'completed' workflow state is recognized THEN the backend golang lambda SHALL execute its new `create-form` action
3. WHEN the `create-form` action executes THEN the system SHALL call the Typeform Create API
4. WHEN creating a survey THEN the system SHALL determine the object type (change, CIC announcement, InnerSource announcement, FinOps announcement, or general announcement)
5. WHEN creating a survey for a change THEN the system SHALL use a survey template designed for change feedback
6. WHEN creating a survey for a CIC announcement THEN the system SHALL use a survey template designed for CIC feedback
7. WHEN creating a survey for an InnerSource announcement THEN the system SHALL use a survey template designed for InnerSource feedback
8. WHEN creating a survey for a FinOps announcement THEN the system SHALL use a survey template designed for FinOps feedback
9. WHEN creating a survey for a general announcement THEN the system SHALL use a survey template designed for general announcement feedback
10. WHEN creating a survey THEN the system SHALL include an NPS question with a 0-10 scale
11. WHEN creating a survey THEN the system SHALL include a yes/no question "Was this [object type] excellent?"
12. WHEN the survey is created THEN the system SHALL store the survey ID and URL in the primary S3 archive object metadata in a manner similar to the meeting scheduled URL and ID
13. IF the Typeform API call fails THEN the system SHALL log the error and continue without blocking the completion workflow
14. WHEN creating a survey THEN the system SHALL retrieve the customer's logo image from the portal S3 bucket
15. WHEN the customer logo image is retrieved THEN the system SHALL base64 encode it for inclusion in the Typeform Create API request
16. WHEN the Typeform Create API request is sent THEN it SHALL include the base64-encoded customer logo image in the form definition
17. IF the customer logo image is not found THEN the system SHALL use the default placeholder logo from `assets/images/default-logo.png`

### Requirement 5

**User Story:** As a customer, I want to access surveys directly within the portal under a survey tab for my customer's items, so that I can provide feedback without leaving the application.

#### Acceptance Criteria

1. WHEN viewing a customer contact item THEN the system SHALL display a new "Survey" tab
2. WHEN the Survey tab is selected THEN the system SHALL embed the Typeform survey using the Typeform Embed SDK inline mode
3. WHEN the embedded survey loads THEN it SHALL use the customer code to filter and display only surveys relevant to that customer
4. WHEN the survey is embedded THEN it SHALL be fully functional and allow submission within the portal
5. WHEN a user accesses the portal survey page via email link with a survey ID parameter THEN the system SHALL use the Typeform popup mode with autoclose enabled
6. WHEN the autoclose feature is enabled THEN the survey SHALL automatically close after submission with a configurable delay (e.g., 2000ms)
7. WHEN the survey is submitted THEN the user SHALL receive appropriate confirmation feedback

### Requirement 6

**User Story:** As a system administrator, I want survey responses to be collected in real-time via Typeform webhooks, so that I can analyze feedback data immediately.

#### Acceptance Criteria

1. WHEN a survey is submitted THEN Typeform SHALL send a webhook notification to the system
2. WHEN the webhook is received THEN the system SHALL validate the webhook signature using HMAC for security
3. WHEN validating the webhook signature THEN the system SHALL use the Typeform webhook secret to compute the HMAC signature
4. IF the webhook signature validation fails THEN the system SHALL reject the request and return a 401 Unauthorized status code
5. WHEN the webhook payload is validated THEN the system SHALL extract the survey response data
6. WHEN survey response data is extracted THEN the system SHALL store it in S3 under a results key prefix
7. WHEN storing survey results THEN the system SHALL organize them using the S3 key structure `surveys/results/{customer_code}/{object_id}/{timestamp}-{survey_id}.json` for easy retrieval by customer and contact object

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

**User Story:** As a system administrator, I want survey forms and results to be stored in a structured format in S3, so that they can be easily processed and displayed in the portal later.

#### Acceptance Criteria

1. WHEN a survey form is created THEN the system SHALL store it using the S3 key structure `surveys/forms/{customer_code}/{object_id}/{timestamp}-{survey_id}.json`
2. WHEN storing survey forms THEN the system SHALL include the complete form definition, survey ID, and URL
3. WHEN storing survey results THEN the system SHALL use the S3 key structure `surveys/results/{customer_code}/{object_id}/{timestamp}-{survey_id}.json`
4. WHEN storing survey results THEN the system SHALL include all response data, metadata, and timestamps
5. WHEN storing survey forms or results THEN the system SHALL ensure the data is in JSON format for easy parsing
6. WHEN storing survey forms or results in S3 THEN the system SHALL use native ETag support for caching in the same manner as other event object types
7. WHEN retrieving survey forms or results THEN the system SHALL leverage S3 ETags for efficient caching and conditional requests
8. IF S3 storage fails THEN the system SHALL retry with exponential backoff within Lambda execution time limits and log the error

### Requirement 9

**User Story:** As a system administrator, I want Typeform API authentication to be securely configured and managed, so that the system can create surveys and manage webhooks without exposing credentials.

#### Acceptance Criteria

1. WHEN the system makes Typeform API requests THEN it SHALL use a Personal Access Token in the Authorization header as `Bearer {token}`
2. WHEN the system is deployed THEN the Typeform Personal Access Token SHALL be stored in AWS Secrets Manager or environment variables
3. WHEN configuring the Typeform token THEN it SHALL have the minimum required scopes for creating forms and managing webhooks
4. WHEN the Typeform token is stored THEN it SHALL NOT be committed to source control or logged in plaintext
5. WHEN the Typeform API returns authentication errors THEN the system SHALL log the error and alert administrators

### Requirement 10

**User Story:** As a system administrator, I want to store and manage customer logo images in the portal S3 bucket, so that they can be used in both Typeform surveys and the approvals page.

#### Acceptance Criteria

1. WHEN a customer logo is uploaded THEN the system SHALL store it in the portal S3 bucket under the key structure `customers/{customer_code}/logo.{extension}`
2. WHEN storing customer logos THEN the system SHALL support common image formats (PNG, JPG, JPEG, GIF, SVG)
3. WHEN retrieving a customer logo THEN the system SHALL check for the existence of the image in S3
4. WHEN a customer logo is needed for Typeform creation THEN the system SHALL retrieve it from S3 and base64 encode it
5. WHEN a customer logo is needed for the approvals page THEN the system SHALL generate a presigned URL or serve it directly
6. WHEN a customer logo does not exist THEN the system SHALL use a default placeholder logo image
7. WHEN the placeholder logo is used THEN it SHALL be stored in S3 at `assets/images/default-logo.png` alongside the main site HTML assets
8. WHEN customer logos are stored THEN they SHALL be optimized for web display with reasonable file size limits
