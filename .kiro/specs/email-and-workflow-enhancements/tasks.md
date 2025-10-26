# Implementation Plan

- [x] 1. Set up infrastructure foundation
  - Create Parameter Store entries for Typeform credentials
  - Deploy default placeholder logo to S3
  - _Requirements: 9, 10_

- [x] 1.1 Create Parameter Store entries
  - Add `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_API_TOKEN` with encrypted value
  - Add `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_WEBHOOK_SECRET` with encrypted value
  - _Requirements: 9_

- [x] 1.2 Deploy default logo asset
  - Create default placeholder logo image
  - Upload to S3 at `assets/images/default-logo.png`
  - Verify accessibility from portal
  - _Requirements: 10_

- [x] 2. Implement Typeform client package
  - Create Go package for Typeform API interactions
  - Implement survey creation with logo embedding
  - Implement webhook signature validation
  - _Requirements: 4, 6, 9_

- [x] 2.1 Create Typeform client structure
  - Create `internal/typeform/client.go` with API client
  - Implement authentication with Personal Access Token
  - Add error handling and logging
  - _Requirements: 9_

- [x] 2.2 Implement survey templates
  - Create `internal/typeform/templates.go`
  - Define survey templates for each type (change, CIC, InnerSource, FinOps, general)
  - Include NPS question and "Was this excellent?" question
  - Configure hidden fields (user_login, customer_code, year, quarter, event_type, event_subtype)
  - _Requirements: 4_

- [x] 2.3 Implement survey creation logic
  - Create `internal/typeform/create.go`
  - Implement logo retrieval from S3 with fallback to default
  - Implement base64 encoding for logo embedding
  - Implement Typeform Create API call
  - Store survey form definition in S3 at `surveys/forms/{customer_code}/{object_id}/{timestamp}-{survey_id}.json`
  - Update S3 object metadata with survey ID and URL
  - _Requirements: 4, 10_

- [x] 2.4 Implement webhook validation
  - Create `internal/typeform/webhook.go`
  - Implement HMAC-SHA256 signature validation
  - Return 401 Unauthorized for invalid signatures
  - _Requirements: 6_

- [x] 3. Enhance Golang Lambda with create-form action
  - Add survey creation trigger on 'completed' workflow state
  - Integrate Typeform client package
  - Handle errors gracefully without blocking workflow
  - _Requirements: 4_

- [x] 3.1 Add create-form action handler
  - Detect 'completed' workflow state in SQS event processing
  - Extract metadata (customer_code, object_id, event_type, event_subtype, year, quarter)
  - Call Typeform client to create survey
  - Log success/failure appropriately
  - _Requirements: 4_

- [x] 3.2 Enhance email generation with survey links
  - Update email templates to include survey links
  - Generate survey page URLs with parameters
  - Include QR codes for survey access
  - _Requirements: 3_

- [x] 3.3 Update approval email generation
  - Add customer_code and object_id parameters to approval URLs
  - Format: `{BASE_FQDN}/approvals.html?customerCode={code}&objectId={id}`
  - _Requirements: 1, 2_

- [x] 4. Create API Gateway webhook endpoint
  - Define Terraform configuration for API Gateway
  - Create REST API with POST /webhooks/typeform endpoint
  - Configure Lambda proxy integration
  - _Requirements: 7_

- [x] 4.1 Define Terraform resources
  - Create API Gateway REST API resource
  - Define /webhooks/typeform resource and POST method
  - Configure Lambda proxy integration
  - Set up CloudWatch logging
  - _Requirements: 7_

- [x] 4.2 Configure Lambda permissions
  - Add IAM role for webhook Lambda
  - Grant S3 read/write permissions for survey results
  - Grant Parameter Store read permissions
  - Add API Gateway invoke permissions
  - _Requirements: 7_

- [x] 5. Implement webhook Lambda function
  - Create new Go Lambda to process Typeform webhooks
  - Validate HMAC signatures
  - Store survey results in S3
  - _Requirements: 6, 8_

- [x] 5.1 Create webhook Lambda handler
  - Create `lambda/webhook/main.go`
  - Parse webhook payload
  - Call Typeform webhook validation
  - Extract hidden fields from response
  - _Requirements: 6_

- [x] 5.2 Implement survey results storage
  - Store results at `surveys/results/{customer_code}/{year}/{quarter}/{timestamp}-{survey_id}.json`
  - Use S3 native ETag support for caching
  - Implement retry logic with exponential backoff
  - _Requirements: 6, 8_

- [x] 5.3 Build and package webhook Lambda
  - Update Makefile with webhook Lambda build target
  - Create deployment package
  - _Requirements: 7_

- [x] 6. Enhance portal approvals page
  - Add URL parameter parsing
  - Implement automatic filtering by customer code
  - Implement automatic modal opening for object ID
  - Add customer logo display
  - _Requirements: 1, 10_

- [x] 6.1 Implement URL parameter handling
  - Add JavaScript to parse customerCode and objectId parameters
  - Filter approvals list by customer code
  - Auto-open modal for specific object ID
  - _Requirements: 1_

- [x] 6.2 Add customer logo display
  - Fetch customer logo from S3 (`customers/{customer_code}/logo.{ext}`)
  - Fallback to default logo (`assets/images/default-logo.png`)
  - Display logo prominently on page
  - _Requirements: 1, 10_

- [x] 7. Create portal survey page
  - Create new surveys.html page
  - Integrate Typeform Embed SDK
  - Implement inline and popup embed modes
  - _Requirements: 3, 5_

- [x] 7.1 Create survey page HTML structure
  - Create `html/surveys.html`
  - Add container for inline survey embed
  - Include Typeform Embed SDK script
  - _Requirements: 5_

- [x] 7.2 Implement inline embed mode
  - Use Typeform createWidget for portal browsing
  - Pass hidden fields (user_login, customer_code, year, quarter, event_type, event_subtype)
  - Retrieve survey metadata from S3
  - _Requirements: 5_

- [x] 7.3 Implement popup embed with autoclose
  - Detect survey ID parameter from URL
  - Use Typeform createPopup with autoClose: 2000
  - Pass hidden fields
  - Auto-open popup on page load
  - _Requirements: 3, 5_

- [x] 7.4 Add survey list view
  - Fetch survey forms from S3 for customer
  - Display list of available surveys
  - Filter by customer code
  - Use ETag caching for performance
  - _Requirements: 5, 8_

- [ ] 8. Deploy and configure infrastructure
  - Apply Terraform changes
  - Configure Typeform webhook URL
  - Verify all components
  - _Requirements: All_

- [ ] 8.1 Deploy Terraform infrastructure
  - Apply Terraform configuration for API Gateway
  - Deploy webhook Lambda
  - Verify Parameter Store references
  - _Requirements: 7, 9_

- [ ] 8.2 Configure Typeform webhooks
  - Log into Typeform account
  - Configure webhook URL to API Gateway endpoint
  - Set webhook secret to match Parameter Store value
  - Test webhook delivery
  - _Requirements: 6_

- [ ] 8.3 Deploy updated Lambdas
  - Build and deploy Golang Lambda with create-form action
  - Verify environment variables
  - Test SQS event processing
  - _Requirements: 4_

- [ ] 8.4 Deploy portal updates
  - Upload updated HTML/JS to S3
  - Invalidate CloudFront cache
  - Verify page loads and functionality
  - _Requirements: 1, 3, 5_

- [ ] 9. End-to-end testing
  - Test complete approval email flow
  - Test complete survey creation and submission flow
  - Verify data storage and retrieval
  - _Requirements: All_

- [ ] 9.1 Test approval email flow
  - Create test announcement/change
  - Verify approval email generation
  - Click email link and verify page filtering
  - Verify modal auto-opens for specific object
  - Verify customer logo displays
  - _Requirements: 1, 2, 10_

- [ ] 9.2 Test survey creation flow
  - Mark test object as completed
  - Verify survey created via Typeform API
  - Verify survey form stored in S3
  - Verify object metadata updated
  - Verify completion email includes survey link
  - _Requirements: 3, 4, 8_

- [ ] 9.3 Test survey submission flow
  - Access survey via email link
  - Submit survey responses
  - Verify webhook received and validated
  - Verify results stored in S3 with correct structure
  - Verify autoclose behavior
  - _Requirements: 5, 6, 8_

- [ ] 9.4 Test logo handling
  - Test with customer-specific logo
  - Test with missing logo (fallback to default)
  - Verify logo in Typeform survey
  - Verify logo on approvals page
  - _Requirements: 10_

- [ ] 9.5 Test error scenarios
  - Test Typeform API failure (verify workflow continues)
  - Test invalid webhook signature (verify 401 response)
  - Test S3 storage failure (verify retry logic)
  - Test missing hidden fields (verify graceful handling)
  - _Requirements: 4, 6, 8_
