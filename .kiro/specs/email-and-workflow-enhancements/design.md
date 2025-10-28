# Design Document

## Overview

This design document outlines the technical approach for enhancing the customer contact management system with improved approval workflows, customer logo management, and Typeform survey integration. The solution leverages existing AWS infrastructure including S3, Lambda, API Gateway, and SQS while introducing new components for survey creation, webhook handling, and enhanced UI features.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         User Interactions                            │
├─────────────────────────────────────────────────────────────────────┤
│  Email Links → Portal (Approvals/Surveys) → Typeform Embed          │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      CloudFront + S3 (Portal)                        │
├─────────────────────────────────────────────────────────────────────┤
│  • HTML/CSS/JS Assets                                                │
│  • Customer Logos (customers/{code}/logo.{ext})                      │
│  • Default Logo (assets/images/default-logo.png)                     │
│  • Survey Forms (surveys/forms/{code}/{id}/{ts}-{sid}.json)          │
│  • Survey Results (surveys/results/{code}/{id}/{ts}-{sid}.json)      │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Backend Processing Layer                          │
├─────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────┐    ┌──────────────────┐    ┌───────────────┐ │
│  │  Upload Lambda   │    │  Golang Lambda   │    │ Webhook Lambda│ │
│  │  (Node.js)       │    │  (Go)            │    │ (Go)          │ │
│  │  • API Handler   │    │  • SQS Processor │    │ • HMAC Verify │ │
│  │  • Write to S3   │    │  • create-form   │    │ • Store Results│ │
│  │                  │    │  • Email Gen     │    │               │ │
│  └──────────────────┘    └──────────────────┘    └───────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      External Services                               │
├─────────────────────────────────────────────────────────────────────┤
│  • Typeform Create API (Form Creation)                              │
│  • Typeform Webhooks (Response Collection)                          │
│  • SES (Email Delivery)                                              │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Interactions

1. **Approval Email Flow**:
   - Upload Lambda writes object to S3
   - S3 event → SQS → Golang Lambda
   - Golang Lambda generates approval emails with deep links
   - Links include customer code and object ID parameters
   - Emails contain text links only (no embedded images)

2. **Survey Creation Flow**:
   - S3 event → SQS → Golang Lambda
   - Lambda detects 'completed' workflow state
   - Executes `create-form` action
   - Retrieves customer logo, base64 encodes it
   - Calls Typeform Create API with logo
   - Stores survey metadata in S3 object metadata
   - Stores survey form definition in S3

3. **Survey Response Flow**:
   - User submits survey via Typeform
   - Typeform sends webhook to API Gateway
   - Webhook Lambda validates HMAC signature
   - Extracts response data
   - Stores in S3 with ETag support

4. **Portal Survey Display Flow**:
   - User accesses portal survey page
   - JavaScript loads Typeform Embed SDK
   - Inline mode for browsing, popup with autoclose for email links
   - Retrieves survey forms from S3 using ETags for caching

## Components and Interfaces

### 1. Frontend Components

#### 1.1 Approvals Page Enhancement

**File**: `html/approvals.html`

**New Features**:
- URL parameter parsing for `customerCode` and `objectId`
- Automatic filtering by customer code
- Automatic modal opening for specific object
- Customer logo display

**JavaScript Functions**:
```javascript
// Parse URL parameters
function getUrlParams() {
  const params = new URLSearchParams(window.location.search);
  return {
    customerCode: params.get('customerCode'),
    objectId: params.get('objectId')
  };
}

// Load and display customer logo
async function loadCustomerLogo(customerCode) {
  const logoUrl = `/customers/${customerCode}/logo.png`;
  // Fallback to default logo if not found
  const defaultLogo = '/assets/images/default-logo.png';
  // Implementation details...
}

// Auto-open modal for specific object
function openObjectModal(objectId) {
  // Implementation details...
}
```

#### 1.2 Survey Page Component

**File**: `html/surveys.html` (new)

**Features**:
- Typeform Embed SDK integration
- Inline embed mode for portal browsing
- Popup mode with autoclose for email links
- Survey ID parameter handling

**JavaScript Integration**:
```javascript
// Inline embed for portal browsing
function embedSurveyInline(surveyId, metadata) {
  const { createWidget } = window.tf;
  createWidget(surveyId, {
    container: document.getElementById('survey-container'),
    medium: 'portal-inline',
    hidden: {
      user_login: getCurrentUserLogin(),
      customer_code: metadata.customerCode,
      year: metadata.year,
      quarter: metadata.quarter,
      event_type: metadata.eventType,
      event_subtype: metadata.eventSubtype
    }
  });
}

// Popup with autoclose for email links
function embedSurveyPopup(surveyId, metadata) {
  const { createPopup } = window.tf;
  const { open } = createPopup(surveyId, {
    medium: 'email-link',
    autoClose: 2000,
    hidden: {
      user_login: getCurrentUserLogin(),
      customer_code: metadata.customerCode,
      year: metadata.year,
      quarter: metadata.quarter,
      event_type: metadata.eventType,
      event_subtype: metadata.eventSubtype
    }
  });
  open();
}
```

### 2. Backend Components

#### 2.1 Upload Lambda (No Changes Required)

**File**: `lambda/upload_lambda/upload-metadata-lambda.js`

**Current Behavior**:
- Receives API requests from portal
- Writes metadata objects to S3
- Triggers S3 events that flow to SQS

**Note**: Email generation is handled by the Golang Lambda, not the Upload Lambda.

#### 2.2 Golang Lambda - Survey Creation

**File**: `internal/typeform/create.go` (new)

**Package Structure**:
```
internal/
  typeform/
    create.go       # Survey creation logic
    templates.go    # Survey templates by type
    client.go       # Typeform API client
    webhook.go      # Webhook signature validation
```

**Core Functions**:

```go
package typeform

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
)

// SurveyType represents the type of survey to create
type SurveyType string

const (
    SurveyTypeChange       SurveyType = "change"
    SurveyTypeCIC          SurveyType = "cic"
    SurveyTypeInnerSource  SurveyType = "innersource"
    SurveyTypeFinOps       SurveyType = "finops"
    SurveyTypeGeneral      SurveyType = "general"
)

// CreateFormRequest represents the Typeform Create API request
type CreateFormRequest struct {
    Title      string                 `json:"title"`
    Type       string                 `json:"type,omitempty"`
    Theme      *Theme                 `json:"theme,omitempty"`
    Fields     []Field                `json:"fields"`
    Hidden     []string               `json:"hidden,omitempty"`
}

// Hidden fields to capture in all surveys:
// - user_login: User's login/email
// - customer_code: Customer identifier
// - year: Year of the event/experience
// - quarter: Quarter of the event/experience (Q1, Q2, Q3, Q4)
// - event_type: Type of event (change, announcement)
// - event_subtype: Subtype (cic, innersource, finops, general)

// Theme represents the form theme with logo
type Theme struct {
    Logo *Logo `json:"logo,omitempty"`
}

// Logo represents the base64-encoded logo
type Logo struct {
    Image string `json:"image"` // base64-encoded image data
}

// CreateSurvey creates a Typeform survey for a completed object
func CreateSurvey(ctx context.Context, customerCode, objectID string, surveyType SurveyType) (*SurveyResponse, error) {
    // 1. Retrieve customer logo from S3
    logoData, err := getCustomerLogo(ctx, customerCode)
    if err != nil {
        log.Printf("Failed to get customer logo, using default: %v", err)
        logoData, _ = getDefaultLogo(ctx)
    }
    
    // 2. Base64 encode logo
    logoBase64 := base64.StdEncoding.EncodeToString(logoData)
    
    // 3. Get survey template for type
    template := getSurveyTemplate(surveyType)
    
    // 4. Build create request
    request := CreateFormRequest{
        Title: fmt.Sprintf("%s Feedback - %s", surveyType, objectID),
        Type: "form",
        Theme: &Theme{
            Logo: &Logo{
                Image: logoBase64,
            },
        },
        Fields: template.Fields,
        Hidden: []string{
            "user_login",
            "customer_code",
            "year",
            "quarter",
            "event_type",
            "event_subtype",
        },
    }
    
    // 5. Call Typeform Create API
    response, err := callTypeformCreateAPI(ctx, request)
    if err != nil {
        return nil, fmt.Errorf("failed to create survey: %w", err)
    }
    
    // 6. Store survey form in S3
    if err := storeSurveyForm(ctx, customerCode, objectID, response); err != nil {
        log.Printf("Failed to store survey form: %v", err)
    }
    
    // 7. Update S3 object metadata with survey info
    if err := updateObjectMetadata(ctx, objectID, response.ID, response.URL); err != nil {
        log.Printf("Failed to update object metadata: %v", err)
    }
    
    return response, nil
}

// getCustomerLogo retrieves customer logo from S3
func getCustomerLogo(ctx context.Context, customerCode string) ([]byte, error) {
    key := fmt.Sprintf("customers/%s/logo.png", customerCode)
    return getS3Object(ctx, key)
}

// getDefaultLogo retrieves default placeholder logo
func getDefaultLogo(ctx context.Context) ([]byte, error) {
    return getS3Object(ctx, "assets/images/default-logo.png")
}

// storeSurveyForm stores the survey form definition in S3
func storeSurveyForm(ctx context.Context, customerCode, objectID string, survey *SurveyResponse) error {
    timestamp := time.Now().Unix()
    key := fmt.Sprintf("surveys/forms/%s/%s/%d-%s.json", customerCode, objectID, timestamp, survey.ID)
    
    data, err := json.Marshal(survey)
    if err != nil {
        return err
    }
    
    return putS3Object(ctx, key, data)
}
```

#### 2.3 Golang Lambda - Webhook Handler

**File**: `internal/typeform/webhook.go`

**Core Functions**:

```go
package typeform

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "time"
)

// WebhookPayload represents the Typeform webhook payload
type WebhookPayload struct {
    EventID      string                 `json:"event_id"`
    EventType    string                 `json:"event_type"`
    FormResponse FormResponse           `json:"form_response"`
}

// FormResponse represents the survey response data
type FormResponse struct {
    FormID      string                 `json:"form_id"`
    Token       string                 `json:"token"`
    SubmittedAt string                 `json:"submitted_at"`
    Hidden      map[string]string      `json:"hidden"`
    Answers     []Answer               `json:"answers"`
}

// ValidateWebhookSignature validates the HMAC signature
func ValidateWebhookSignature(payload []byte, signature string, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    expectedSignature := hex.EncodeToString(mac.Sum(nil))
    
    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// HandleWebhook processes incoming Typeform webhooks
func HandleWebhook(ctx context.Context, payload []byte, signature string) error {
    // 1. Validate signature
    secret := os.Getenv("TYPEFORM_WEBHOOK_SECRET")
    if !ValidateWebhookSignature(payload, signature, secret) {
        return fmt.Errorf("invalid webhook signature")
    }
    
    // 2. Parse payload
    var webhook WebhookPayload
    if err := json.Unmarshal(payload, &webhook); err != nil {
        return fmt.Errorf("failed to parse webhook payload: %w", err)
    }
    
    // 3. Extract metadata from hidden fields
    userLogin := webhook.FormResponse.Hidden["user_login"]
    customerCode := webhook.FormResponse.Hidden["customer_code"]
    year := webhook.FormResponse.Hidden["year"]
    quarter := webhook.FormResponse.Hidden["quarter"]
    eventType := webhook.FormResponse.Hidden["event_type"]
    eventSubtype := webhook.FormResponse.Hidden["event_subtype"]
    
    // 4. Store survey results in S3
    timestamp := time.Now().Unix()
    // Use customer_code and year/quarter for organization
    key := fmt.Sprintf("surveys/results/%s/%s/%s/%d-%s.json", 
        customerCode, year, quarter, timestamp, webhook.FormResponse.FormID)
    
    if err := putS3Object(ctx, key, payload); err != nil {
        return fmt.Errorf("failed to store survey results: %w", err)
    }
    
    return nil
}
```

### 3. Survey Templates

**File**: `internal/typeform/templates.go`

```go
package typeform

// SurveyTemplate defines the structure for each survey type
type SurveyTemplate struct {
    Fields []Field
}

// Field represents a Typeform field
type Field struct {
    Type       string                 `json:"type"`
    Title      string                 `json:"title"`
    Properties map[string]interface{} `json:"properties,omitempty"`
}

// getSurveyTemplate returns the template for a given survey type
func getSurveyTemplate(surveyType SurveyType) SurveyTemplate {
    switch surveyType {
    case SurveyTypeChange:
        return getChangeTemplate()
    case SurveyTypeCIC:
        return getCICTemplate()
    case SurveyTypeInnerSource:
        return getInnerSourceTemplate()
    case SurveyTypeFinOps:
        return getFinOpsTemplate()
    case SurveyTypeGeneral:
        return getGeneralTemplate()
    default:
        return getGeneralTemplate()
    }
}

// getChangeTemplate returns the survey template for changes
func getChangeTemplate() SurveyTemplate {
    return SurveyTemplate{
        Fields: []Field{
            {
                Type:  "opinion_scale",
                Title: "How likely are you to recommend this change to a colleague?",
                Properties: map[string]interface{}{
                    "start_at_one": false,
                    "steps":        11,
                    "labels": map[string]string{
                        "left":   "Not at all likely",
                        "right":  "Extremely likely",
                    },
                },
            },
            {
                Type:  "yes_no",
                Title: "Was this change excellent?",
            },
            {
                Type:  "long_text",
                Title: "What could we improve about this change?",
                Properties: map[string]interface{}{
                    "description": "Optional feedback",
                },
            },
        },
    }
}

// Similar templates for CIC, InnerSource, FinOps, and General...
```

## Data Models

### S3 Object Metadata

**Enhanced Metadata Fields**:
```json
{
  "customer_code": "ACME",
  "object_id": "change-12345",
  "workflow_state": "completed",
  "meeting_url": "https://meet.google.com/abc-defg-hij",
  "meeting_id": "abc-defg-hij",
  "survey_id": "HLjqXS5W",
  "survey_url": "https://form.typeform.com/to/HLjqXS5W",
  "survey_created_at": "2025-10-25T12:00:00Z"
}
```

### Survey Form Storage

**S3 Key**: `surveys/forms/{customer_code}/{object_id}/{timestamp}-{survey_id}.json`

**Content**:
```json
{
  "id": "HLjqXS5W",
  "title": "Change Feedback - change-12345",
  "type": "form",
  "workspace": {
    "href": "https://api.typeform.com/workspaces/abc123"
  },
  "_links": {
    "display": "https://form.typeform.com/to/HLjqXS5W"
  },
  "theme": {
    "href": "https://api.typeform.com/themes/xyz789"
  },
  "fields": [...],
  "hidden": ["customer_code", "object_id"],
  "created_at": "2025-10-25T12:00:00Z"
}
```

### Survey Results Storage

**S3 Key**: `surveys/results/{customer_code}/{year}/{quarter}/{timestamp}-{survey_id}.json`

**Content**:
```json
{
  "event_id": "01HGWQR...",
  "event_type": "form_response",
  "form_response": {
    "form_id": "HLjqXS5W",
    "token": "abc123def456",
    "submitted_at": "2025-10-25T14:30:00Z",
    "hidden": {
      "user_login": "john.doe@hearst.com",
      "customer_code": "ACME",
      "year": "2025",
      "quarter": "Q4",
      "event_type": "change",
      "event_subtype": "general"
    },
    "answers": [
      {
        "type": "number",
        "number": 9,
        "field": {
          "id": "nps_question",
          "type": "opinion_scale"
        }
      },
      {
        "type": "boolean",
        "boolean": true,
        "field": {
          "id": "excellent_question",
          "type": "yes_no"
        }
      }
    ]
  }
}
```

### Customer Logo Storage

**S3 Key Structure**:
- Customer logos: `customers/{customer_code}/logo.{png|jpg|jpeg|gif|svg}`
- Default logo: `assets/images/default-logo.png`

**Supported Formats**: PNG, JPG, JPEG, GIF, SVG

**Size Limits**: Max 2MB per logo (optimized for web display)

## Error Handling

### 1. Logo Retrieval Failures

**Scenario**: Customer logo not found in S3

**Handling**:
```go
func getCustomerLogoWithFallback(ctx context.Context, customerCode string) ([]byte, error) {
    // Try customer-specific logo
    logoData, err := getCustomerLogo(ctx, customerCode)
    if err != nil {
        log.Printf("Customer logo not found for %s, using default: %v", customerCode, err)
        // Fallback to default logo
        logoData, err = getDefaultLogo(ctx)
        if err != nil {
            return nil, fmt.Errorf("failed to get default logo: %w", err)
        }
    }
    return logoData, nil
}
```

### 2. Typeform API Failures

**Scenario**: Typeform Create API returns error

**Handling**:
```go
func CreateSurvey(ctx context.Context, customerCode, objectID string, surveyType SurveyType) (*SurveyResponse, error) {
    response, err := callTypeformCreateAPI(ctx, request)
    if err != nil {
        // Log error but don't block workflow
        log.Printf("Failed to create survey for %s/%s: %v", customerCode, objectID, err)
        
        // Send alert to administrators
        sendAlert(ctx, fmt.Sprintf("Survey creation failed: %v", err))
        
        // Return error but allow workflow to continue
        return nil, fmt.Errorf("survey creation failed: %w", err)
    }
    return response, nil
}
```

### 3. Webhook Signature Validation Failures

**Scenario**: Invalid HMAC signature on webhook

**Handling**:
```go
func HandleWebhook(ctx context.Context, payload []byte, signature string) (int, error) {
    if !ValidateWebhookSignature(payload, signature, secret) {
        log.Printf("Invalid webhook signature received")
        return 401, fmt.Errorf("unauthorized: invalid signature")
    }
    // Continue processing...
}
```

### 4. S3 Storage Failures

**Scenario**: Failed to store survey form or results

**Handling**:
```go
func storeSurveyFormWithRetry(ctx context.Context, key string, data []byte) error {
    maxRetries := 3
    backoff := time.Second
    
    for i := 0; i < maxRetries; i++ {
        err := putS3Object(ctx, key, data)
        if err == nil {
            return nil
        }
        
        log.Printf("S3 storage attempt %d failed: %v", i+1, err)
        
        if i < maxRetries-1 {
            time.Sleep(backoff)
            backoff *= 2 // Exponential backoff
        }
    }
    
    return fmt.Errorf("failed to store after %d retries", maxRetries)
}
```

## Testing Strategy

### 1. Unit Tests

**Frontend**:
- URL parameter parsing
- Logo loading with fallback
- Modal auto-opening
- Typeform embed initialization

**Backend**:
- Logo retrieval and base64 encoding
- Survey template generation
- HMAC signature validation
- S3 storage operations

### 2. Integration Tests

**Survey Creation Flow**:
1. Trigger 'completed' workflow state
2. Verify SQS message processing
3. Verify Typeform API call
4. Verify S3 storage of form definition
5. Verify metadata update

**Webhook Flow**:
1. Send test webhook with valid signature
2. Verify signature validation
3. Verify S3 storage of results
4. Send webhook with invalid signature
5. Verify rejection with 401

### 3. End-to-End Tests

**Approval Email Flow**:
1. Generate approval email
2. Click email link
3. Verify page loads with correct filter
4. Verify modal opens for specific object
5. Verify customer logo displays

**Survey Submission Flow**:
1. Access survey via email link
2. Submit survey responses
3. Verify webhook received
4. Verify results stored in S3
5. Verify autoclose behavior

## Configuration

### Environment Variables

**Upload Lambda**:
```
BASE_FQDN=https://contact.ccoe.hearst.com
S3_BUCKET=4cm-prod-ccoe-change-management-metadata
```

**Golang Lambda**:
```
TYPEFORM_API_TOKEN=<runtime value from encrypted Parameter Store>
TYPEFORM_WEBHOOK_SECRET=<runtime value from encrypted Parameter Store>
S3_BUCKET=4cm-prod-ccoe-change-management-metadata
BASE_FQDN=https://contact.ccoe.hearst.com
```

**Webhook Lambda**:
```
TYPEFORM_WEBHOOK_SECRET=<runtime value from encrypted Parameter Store>
S3_BUCKET=4cm-prod-ccoe-change-management-metadata
```

**Note**: Typeform credentials are stored in encrypted SSM Parameter Store and injected as environment variables at Lambda runtime via Terraform configuration, consistent with existing Microsoft Graph API credential management.

## Deployment Considerations

### 1. Infrastructure Changes

**Deployment Method**: All infrastructure changes will be deployed via Terraform

**New Resources**:
- **API Gateway REST API**: New HTTP endpoint to receive Typeform webhook POST requests
  - Endpoint: `POST /webhooks/typeform`
  - Integration: Lambda proxy integration to webhook handler
  - Purpose: Capture survey response webhooks from Typeform
- **Webhook Lambda function**: New Go Lambda to process webhook payloads
  - Environment variables for Typeform credentials (sourced from encrypted Parameter Store)
- **SSM Parameter Store entries**: Encrypted parameters for Typeform credentials
  - `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_API_TOKEN`
  - `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_WEBHOOK_SECRET`

**Modified Resources**:
- Golang Lambda: Add `create-form` action
- S3 bucket: New prefixes for surveys and logos

### 2. Migration Steps

1. Deploy default logo to `assets/images/default-logo.png`
2. Store Typeform credentials in encrypted Parameter Store
3. Deploy infrastructure changes via Terraform (API Gateway, Webhook Lambda, Parameter Store references)
4. Deploy updated Golang Lambda with `create-form` action
5. Configure Typeform webhook URL to point to API Gateway endpoint
6. Deploy updated portal HTML/JS
7. Test end-to-end flows

### 3. Rollback Plan

- Revert Lambda function versions
- Disable webhook endpoint
- Restore previous HTML/JS versions
- Survey data remains in S3 (no data loss)

## Performance Considerations

### 1. Caching Strategy

**S3 ETags**:
- Survey forms and results use native S3 ETags
- Portal JavaScript uses conditional GET requests
- Reduces bandwidth and improves load times

**CloudFront**:
- Customer logos cached at edge locations
- Default logo cached with long TTL
- Survey page assets cached

### 2. Concurrency

**Golang Lambda**:
- Concurrent S3 operations for logo retrieval
- Concurrent Typeform API calls (if multiple surveys)
- Rate limiting with exponential backoff

**Webhook Lambda**:
- Handles concurrent webhook deliveries
- Idempotent operations for duplicate webhooks

### 3. Scalability

**API Gateway**:
- Auto-scales for webhook traffic
- Throttling configured to prevent abuse

**Lambda**:
- Concurrent execution limits configured
- Reserved concurrency for critical functions

## Security Considerations

### 1. Authentication

**Typeform API**:
- Personal Access Token injected via environment variables
- Token rotation managed through Parameter Store updates
- Minimum required scopes configured

**Webhook Validation**:
- HMAC-SHA256 signature verification
- Secret injected via environment variables
- Reject requests with invalid signatures

### 2. Authorization

**S3 Access**:
- Lambda execution roles with least privilege
- Separate permissions for read/write operations
- Customer logo access restricted by prefix

**API Gateway**:
- Webhook endpoint publicly accessible (validated by HMAC)
- Rate limiting to prevent abuse
- CloudWatch logging for audit trail

### 3. Data Protection

**Sensitive Data**:
- Survey responses may contain PII
- Stored encrypted at rest in S3
- Access logged in CloudTrail

**Secrets**:
- Typeform credentials injected via environment variables
- Rotation managed through Parameter Store updates
- Never logged or exposed in code

## Monitoring and Observability

### 1. CloudWatch Metrics

**Custom Metrics**:
- Survey creation success/failure rate
- Webhook processing latency
- Logo retrieval cache hit rate
- S3 storage operation duration

### 2. CloudWatch Logs

**Log Groups**:
- `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-backend`
- `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-api`
- `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-webhook`

**Log Insights Queries**:
```
# Survey creation failures
fields @timestamp, @message
| filter @message like /failed to create survey/
| sort @timestamp desc

# Webhook signature failures
fields @timestamp, @message
| filter @message like /invalid webhook signature/
| sort @timestamp desc
```

### 3. Alarms

**Critical Alarms**:
- Survey creation failure rate > 10%
- Webhook processing errors > 5%
- S3 storage failures > 1%
- Typeform API error rate > 5%

**Warning Alarms**:
- Logo retrieval failures (fallback to default)
- Webhook processing latency > 5s
- Lambda concurrent execution approaching limit

## Future Enhancements

### 1. Survey Analytics Dashboard

- Aggregate survey results by customer
- NPS score trending over time
- "Excellent" rating percentages
- Export to CSV/Excel

### 2. Survey Template Management

- Admin UI for creating custom templates
- A/B testing different survey questions
- Multi-language support

### 3. Advanced Logo Management

- Logo upload UI in portal
- Image optimization and resizing
- Multiple logo variants (light/dark mode)
- Logo versioning

### 4. Enhanced Webhook Processing

- Real-time notifications on survey submission
- Automatic follow-up actions based on responses
- Integration with analytics platforms
