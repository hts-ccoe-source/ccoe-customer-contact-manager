# Implementation Plan

- [ ] 1. Add configuration support for base FQDN and environment variable loading
  - Add `BaseFQDN` and `TypeformWorkspaceID` fields to Config struct in `internal/config/config.go`
  - Set default value for `BaseFQDN` to current default URL
  - Create `LoadTypeformSecrets` function to load secrets from environment variables
  - Read `TYPEFORM_API_KEY` and `TYPEFORM_WEBHOOK_SECRET` environment variables
  - Create `TypeformSecrets` struct to hold loaded secrets
  - Validate that required environment variables are present
  - Update `config.json` example with `base_fqdn` and `typeform_workspace_id` fields
  - _Requirements: 2.1, 2.2, 2.3, 2.4_

- [ ] 2. Enhance approval email generation with customer-filtered links
  - Modify `SendApprovalRequest` function in `internal/ses/operations.go` to accept `baseFQDN` parameter
  - Extract customer code and object ID (CHG-*, INN-*, etc.) from metadata JSON
  - Generate approval URL with format: `{baseFQDN}/approvals.html?customer_code={customerCode}&object_id={objectID}`
  - Update email template to include the generated approval link
  - Update CLI command in `main.go` to pass `baseFQDN` from config
  - _Requirements: 1.1, 1.2, 1.4, 1.5_

- [ ] 3. Create Typeform integration package
- [ ] 3.1 Implement Typeform client structure
  - Create new package `internal/typeform/`
  - Implement `TypeformClient` struct with `APIKey`, `WorkspaceID`, and `HTTPClient` fields
  - Implement `NewTypeformClient` constructor function accepting API key and workspace ID
  - _Requirements: 4.1_

- [ ] 3.2 Implement survey creation functionality
  - Create `SurveyRequest` and `SurveyResponse` structs
  - Implement `CreateSurvey` method that calls Typeform Create API
  - Build JSON request with NPS question (0-10 scale) and yes/no excellence question
  - Include hidden fields for customer_code, object_type, and object_id
  - Handle API errors gracefully and return structured response
  - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [ ] 3.3 Implement survey metadata storage
  - Create `SurveyMetadata` struct
  - Implement function to store survey metadata in S3 at `surveys/metadata/{customer_code}/{object_id}/{survey_id}.json`
  - Use existing S3 client patterns from the codebase
  - _Requirements: 4.4_

- [ ] 3.4 Implement rate limiting for Typeform API
  - Use existing `RateLimiter` pattern from `internal/ses/operations.go`
  - Configure for 60 requests per minute (Typeform API limit)
  - _Requirements: 4.5_

- [ ] 4. Implement QR code generation for survey links
  - Add `github.com/skip2/go-qrcode` dependency to `go.mod`
  - Create `GenerateQRCode` function in `internal/typeform/` package
  - Generate 256x256 PNG QR code from survey URL
  - Return base64-encoded image data for email embedding
  - Handle generation errors gracefully
  - _Requirements: 3.2_

- [ ] 5. Create completion email with survey integration
  - Implement `SendCompletionEmailWithSurvey` function in `internal/ses/operations.go`
  - Call Typeform API to create survey when change/announcement is completed
  - Generate QR code for survey URL
  - Build email template with survey URL and embedded QR code image
  - Include object type (change/announcement) and object ID in email
  - Log errors if Typeform API fails but continue with email without survey
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 4.5_

- [ ] 6. Add CLI command for sending completion emails
  - Add new CLI command in `main.go` for `send-completion-email`
  - Accept parameters: topic name, metadata JSON, sender email, dry-run flag
  - Load workspace ID from config.json and secrets from environment variables at initialization
  - Call `SendCompletionEmailWithSurvey` function
  - Support both manual execution and Lambda wrapper modes
  - _Requirements: 3.1, 4.1_

- [ ] 7. Implement webhook handler Lambda function
- [ ] 7.1 Create Lambda function structure
  - Create new directory `lambda/typeform_webhook/`
  - Create `main.go` with Lambda handler entry point
  - Implement `WebhookHandler` struct with S3 client, webhook secret, and logger
  - Implement `HandleRequest` method accepting API Gateway proxy request
  - _Requirements: 7.1, 7.2_

- [ ] 7.2 Implement webhook signature validation
  - Implement `validateSignature` method using HMAC-SHA256
  - Extract signature from request headers
  - Compare with computed signature from payload
  - Return 401 Unauthorized for invalid signatures
  - Log security events for failed validations
  - _Requirements: 6.2, 7.3_

- [ ] 7.3 Implement webhook payload processing
  - Create structs for Typeform webhook payload: `TypeformWebhook`, `FormResponseData`, `Answer`, `FieldInfo`
  - Parse JSON payload from request body
  - Extract hidden fields: customer_code, object_type, object_id
  - Extract survey answers (NPS score and excellence question)
  - Return 400 Bad Request for malformed payloads
  - _Requirements: 6.1, 6.3_

- [ ] 7.4 Implement S3 storage for survey responses
  - Generate S3 key using format: `surveys/results/{customer_code}/{object_id}/{survey_id}/{timestamp}-{token}.json`
  - Implement idempotent storage by listing objects with prefix `surveys/results/{customer_code}/{object_id}/{survey_id}/` and checking if token exists in any filename
  - If token found in existing filename, return 200 OK without writing (idempotent)
  - If token not found, write new object with timestamp and token in filename
  - Retry S3 operations with exponential backoff within Lambda timeout
  - Return 200 OK on success, 500 on failure after retries
  - _Requirements: 6.4, 6.5, 8.1, 8.2, 8.3, 8.4, 7.4, 7.5, 7.6_

- [ ] 7.5 Add structured logging to webhook handler
  - Use `slog` for structured logging
  - Log webhook receipt, validation results, processing steps, and S3 storage
  - Include customer code, object ID, and event ID in log context
  - Log errors with appropriate severity levels
  - _Requirements: 7.5_

- [ ] 7.6 Create Lambda build and deployment scripts
  - Create `Makefile` in `lambda/typeform_webhook/` directory
  - Add build script to compile Go binary for Lambda
  - Add deployment script to package and upload Lambda function
  - Document required environment variables (`TYPEFORM_API_KEY`, `TYPEFORM_WEBHOOK_SECRET`)
  - Document how deployment scripts inject SSM parameters as environment variables
  - Include instructions in README.md
  - _Requirements: 7.1_

- [ ] 8. Create API Gateway infrastructure
  - Create Terraform configuration file `terraform/typeform-webhook-api.tf`
  - Define REST API resource for webhook endpoint
  - Configure POST method at `/webhook` path
  - Set up Lambda integration with proxy mode
  - Configure Lambda permissions for API Gateway invocation
  - Deploy API Gateway stage
  - _Requirements: 7.1, 7.2_

- [ ] 9. Implement frontend survey tab
- [ ] 9.1 Create survey tab JavaScript module
  - Create new file `html/assets/js/survey-tab.js`
  - Implement `SurveyTab` class with constructor accepting customer code and object ID
  - Implement `loadSurveyMetadata` method to fetch survey metadata from S3 at `surveys/metadata/{customer_code}/{object_id}/{survey_id}.json`
  - Implement `embedSurvey` method using Typeform Embed SDK
  - Implement `showConfirmation` method for post-submission feedback
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ] 9.2 Add survey tab to approvals page
  - Update `html/approvals.html` to include survey tab button
  - Add survey tab content container with ID `survey-container`
  - Include Typeform Embed SDK script tag
  - Include `survey-tab.js` script tag
  - Initialize survey tab when approval details are loaded
  - Pass customer code from URL parameter to survey tab
  - _Requirements: 5.1, 5.2, 5.3_

- [ ] 9.3 Add survey tab to my-changes page
  - Update `html/my-changes.html` to include survey tab button
  - Add survey tab content container
  - Include necessary script tags
  - Initialize survey tab when change details are loaded
  - _Requirements: 5.1, 5.2, 5.3_

- [ ] 9.4 Add survey tab to announcements page
  - Update `html/announcements.html` to include survey tab button
  - Add survey tab content container
  - Include necessary script tags
  - Initialize survey tab when announcement details are loaded
  - _Requirements: 5.1, 5.2, 5.3_

- [ ] 10. Update Lambda backend to trigger survey creation
  - Modify `internal/lambda/handlers.go` to detect when changes/announcements are completed
  - Call Typeform API to create survey when status changes to completed
  - Store survey metadata in S3
  - Trigger completion email with survey link
  - Handle errors gracefully without blocking the completion workflow
  - _Requirements: 4.1, 4.5_

- [ ] 11. Configure Typeform webhook in Typeform dashboard
  - Document the API Gateway webhook URL
  - Create instructions for configuring webhook in Typeform dashboard
  - Include webhook secret setup instructions
  - Add to deployment documentation
  - _Requirements: 6.1_

- [ ] 12. Add integration for closed experiences
  - Update status change handlers to detect when experiences are closed
  - Generate Typeform survey links for closed experiences
  - Include survey links in notification emails or UI
  - _Requirements: 3.1a_

- [ ] 13. Update documentation
  - Document new `base_fqdn` and `typeform_workspace_id` configuration fields in README.md
  - Add Typeform integration setup guide
  - Document required environment variables (`TYPEFORM_API_KEY`, `TYPEFORM_WEBHOOK_SECRET`)
  - Document SSM Parameter Store setup for storing secrets (`/ccoe/typeform/api_key`, `/ccoe/typeform/webhook_secret`)
  - Document how deployment scripts inject SSM parameters as environment variables
  - Document webhook endpoint configuration
  - Add troubleshooting section for common issues
  - Document survey tab usage for end users
  - _Requirements: 2.1, 2.2, 2.3, 2.4_
