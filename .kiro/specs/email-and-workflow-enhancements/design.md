# Design Document

## Overview

This design implements email and workflow enhancements for the customer contact management system by integrating Typeform surveys for feedback collection. The solution includes three main components:

1. **Enhanced Approval Emails** - Configurable FQDN with customer-filtered approval links
2. **Typeform Survey Integration** - Automated survey creation and embedding for completed items
3. **Webhook Processing** - Real-time survey response collection via API Gateway and Lambda

The design follows the existing architecture patterns in the codebase, using Go with AWS SDK v2, structured logging with slog, and idempotent operations with proper error handling.

## Architecture

### High-Level Component Diagram

```
┌─────────────────┐
│  SES Email      │
│  Templates      │
└────────┬────────┘
         │
         ├──> Approval Emails (with customer-filtered links)
         └──> Completion Emails (with Typeform survey links + QR codes)
                      │
                      v
         ┌────────────────────────┐
         │  Typeform Create API   │
         │  (Survey Generation)   │
         └────────────────────────┘
                      │
                      v
         ┌────────────────────────┐
         │  Frontend Portal       │
         │  - Survey Tab          │
         │  - Typeform Embed SDK  │
         └────────────────────────┘
                      │
                      v
         ┌────────────────────────┐
         │  Typeform Webhooks     │
         └────────────────────────┘
                      │
                      v
         ┌────────────────────────┐
         │  API Gateway           │
         │  + Lambda Handler      │
         └────────────────────────┘
                      │
                      v
         ┌────────────────────────┐
         │  S3 Storage            │
         │  (Survey Results)      │
         └────────────────────────┘
```

### Configuration Architecture

```
config.json
├── base_fqdn: "https://contact.ccoe.hearst.com/"
└── typeform_workspace_id: "xxx"

AWS Systems Manager Parameter Store
├── /ccoe/typeform/api_key (encrypted)
└── /ccoe/typeform/webhook_secret (encrypted)
```

## Components and Interfaces

### 1. Configuration Management

**File**: `internal/config/config.go`

Add new configuration fields to the existing `Config` struct:

```go
type Config struct {
    // ... existing fields ...
    
    // Email and Survey Configuration
    BaseFQDN            string `json:"base_fqdn"`
    TypeformWorkspaceID string `json:"typeform_workspace_id"`
}
```

**Default Values**:
- `BaseFQDN`: `"https://contact-manager.ccoe.hearst.com/"` (current default)
- `TypeformWorkspaceID`: Empty string (must be configured)

**Secrets Management**:
Typeform secrets are injected via environment variables at runtime:
- `TYPEFORM_API_KEY` - Typeform API key
- `TYPEFORM_WEBHOOK_SECRET` - Webhook signature secret

Note: These environment variables are populated from AWS Systems Manager Parameter Store during deployment/initialization, but the application simply reads them from the environment.

```go
func LoadTypeformSecrets() (*TypeformSecrets, error) {
    apiKey := os.Getenv("TYPEFORM_API_KEY")
    webhookSecret := os.Getenv("TYPEFORM_WEBHOOK_SECRET")
    
    if apiKey == "" || webhookSecret == "" {
        return nil, fmt.Errorf("missing required Typeform environment variables")
    }
    
    return &TypeformSecrets{
        APIKey:        apiKey,
        WebhookSecret: webhookSecret,
    }, nil
}
```

### 2. Enhanced Email Templates

**File**: `internal/ses/operations.go` (extend existing)

#### Approval Email Enhancement

Modify the existing `SendApprovalRequest` function to include customer-filtered links:

```go
func SendApprovalRequest(
    sesClient *sesv2.Client,
    topicName string,
    metadata string,
    htmlTemplate string,
    senderEmail string,
    baseFQDN string,
    dryRun bool,
) error
```

**Email Template Variables**:
- `{{.ApprovalLink}}` - Generated as `{baseFQDN}/approvals.html?customer_code={{.CustomerCode}}&object_id={{.ChangeID}}`
- `{{.CustomerCode}}` - Extracted from metadata
- `{{.ChangeID}}` - Change/announcement identifier (CHG-*, INN-*, etc.)

#### Completion Email with Survey

New function for completion emails with Typeform integration:

```go
func SendCompletionEmailWithSurvey(
    sesClient *sesv2.Client,
    topicName string,
    metadata string,
    surveyURL string,
    qrCodeData string,
    senderEmail string,
    dryRun bool,
) error
```

**Email Template Variables**:
- `{{.SurveyURL}}` - Typeform survey link
- `{{.QRCodeData}}` - Base64-encoded QR code image
- `{{.ObjectType}}` - "change" or "announcement"
- `{{.ObjectID}}` - CHG-*, INN-*, etc.

**QR Code Generation**:
Use Go library `github.com/skip2/go-qrcode` for QR code generation:

```go
func GenerateQRCode(url string) (string, error) {
    png, err := qrcode.Encode(url, qrcode.Medium, 256)
    if err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(png), nil
}
```

### 3. Typeform Integration

**New Package**: `internal/typeform/`

#### Client Interface

```go
type TypeformClient struct {
    APIKey      string
    WorkspaceID string
    HTTPClient  *http.Client
}

func NewTypeformClient(apiKey, workspaceID string) *TypeformClient
```

#### Survey Creation

```go
type SurveyRequest struct {
    ObjectType string // "change" or "announcement"
    ObjectID   string // CHG-001, INN-002, etc.
    CustomerCode string
}

type SurveyResponse struct {
    FormID    string
    FormURL   string
    CreatedAt time.Time
}

func (c *TypeformClient) CreateSurvey(req SurveyRequest) (*SurveyResponse, error)
```

**Typeform Create API Request Structure**:

```json
{
  "title": "Feedback: {{ObjectType}} {{ObjectID}}",
  "workspace": {
    "href": "https://api.typeform.com/workspaces/{{WorkspaceID}}"
  },
  "fields": [
    {
      "title": "How likely are you to recommend our service? (0-10)",
      "type": "opinion_scale",
      "properties": {
        "start_at_one": false,
        "steps": 11,
        "labels": {
          "left": "Not at all likely",
          "right": "Extremely likely"
        }
      },
      "validations": {
        "required": true
      }
    },
    {
      "title": "Was this {{ObjectType}} excellent?",
      "type": "yes_no",
      "validations": {
        "required": true
      }
    }
  ],
  "hidden": [
    "customer_code",
    "object_type",
    "object_id"
  ]
}
```

**Hidden Fields**: Pass customer_code, object_type, and object_id as URL parameters for tracking.

#### Survey Metadata Storage

Store survey metadata in S3 in a dedicated surveys key prefix:

**S3 Key**: `surveys/metadata/{customer_code}/{object_id}/{survey_id}.json`

```json
{
  "form_id": "abc123",
  "form_url": "https://form.typeform.com/to/abc123",
  "created_at": "2025-10-15T10:30:00Z",
  "object_type": "change",
  "object_id": "CHG-001",
  "customer_code": "CUSTOMER123"
}
```

### 4. Frontend Survey Tab

**New File**: `html/assets/js/survey-tab.js`

#### Survey Tab Component

```javascript
class SurveyTab {
    constructor(customerCode, objectId) {
        this.customerCode = customerCode;
        this.objectId = objectId;
        this.surveyMetadata = null;
    }
    
    async loadSurveyMetadata() {
        // Fetch survey metadata from S3
        // Note: Need to list objects with prefix to find the survey_id
        const prefix = `surveys/metadata/${this.customerCode}/${this.objectId}/`;
        const objects = await s3Client.listObjects(prefix);
        if (objects.length > 0) {
            this.surveyMetadata = await s3Client.getObject(objects[0].key);
        }
    }
    
    embedSurvey() {
        // Use Typeform Embed SDK
        const { createWidget } = window.typeformEmbed;
        
        createWidget(this.surveyMetadata.form_id, {
            container: document.getElementById('survey-container'),
            hidden: {
                customer_code: this.customerCode,
                object_type: this.surveyMetadata.object_type,
                object_id: this.objectId
            },
            onSubmit: () => {
                this.showConfirmation();
            }
        });
    }
}
```

#### HTML Structure

Add to existing detail pages (e.g., `approvals.html`, `my-changes.html`):

```html
<div class="tabs">
    <button class="tab-button" data-tab="details">Details</button>
    <button class="tab-button" data-tab="timeline">Timeline</button>
    <button class="tab-button" data-tab="survey">Survey</button>
</div>

<div id="survey-tab" class="tab-content">
    <div id="survey-container"></div>
</div>

<script src="https://embed.typeform.com/next/embed.js"></script>
<script src="assets/js/survey-tab.js"></script>
```

### 5. Webhook Handler

**New Lambda Function**: `lambda/typeform_webhook/`

#### Lambda Handler Structure

```go
package main

import (
    "context"
    "encoding/json"
    "log/slog"
    
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
)

type WebhookHandler struct {
    s3Client      *s3.Client
    webhookSecret string
    logger        *slog.Logger
}

func (h *WebhookHandler) HandleRequest(
    ctx context.Context,
    request events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error)
```

#### Webhook Signature Validation

```go
func (h *WebhookHandler) validateSignature(
    payload []byte,
    signature string,
) bool {
    mac := hmac.New(sha256.New, []byte(h.webhookSecret))
    mac.Write(payload)
    expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
```

#### Webhook Payload Processing

**Typeform Webhook Payload Structure**:

```json
{
  "event_id": "01H...",
  "event_type": "form_response",
  "form_response": {
    "form_id": "abc123",
    "token": "xyz789",
    "submitted_at": "2025-10-15T10:30:00Z",
    "hidden": {
      "customer_code": "CUSTOMER123",
      "object_type": "change",
      "object_id": "CHG-001"
    },
    "answers": [
      {
        "field": {
          "id": "field1",
          "type": "opinion_scale"
        },
        "type": "number",
        "number": 9
      },
      {
        "field": {
          "id": "field2",
          "type": "yes_no"
        },
        "type": "boolean",
        "boolean": true
      }
    ]
  }
}
```

#### S3 Storage Logic

```go
func (h *WebhookHandler) storeResponse(
    ctx context.Context,
    response TypeformResponse,
) error {
    // Extract metadata
    customerCode := response.FormResponse.Hidden["customer_code"]
    objectID := response.FormResponse.Hidden["object_id"]
    
    // Generate S3 key using standardized RFC3339 timestamp format
    surveyID := response.FormResponse.FormID
    timestamp := time.Now().Format(time.RFC3339)
    key := fmt.Sprintf(
        "surveys/results/%s/%s/%s/%s-%s.json",
        customerCode,
        objectID,
        surveyID,
        timestamp,
        response.FormResponse.Token,
    )
    
    // Store with idempotency check
    return h.putObjectIdempotent(ctx, key, response)
}
```

**Idempotency Implementation**:
- Use response token as unique identifier in the filename
- List objects with prefix `surveys/results/{customer_code}/{object_id}/{survey_id}/` and check if any filename contains the response token
- If token already exists in any filename, skip write and return success (idempotent)
- Otherwise, write new file with timestamp and token in filename

### 6. API Gateway Configuration

**Infrastructure**: Terraform or CloudFormation

```hcl
resource "aws_api_gateway_rest_api" "typeform_webhook" {
  name        = "typeform-webhook-api"
  description = "API Gateway for Typeform webhook processing"
}

resource "aws_api_gateway_resource" "webhook" {
  rest_api_id = aws_api_gateway_rest_api.typeform_webhook.id
  parent_id   = aws_api_gateway_rest_api.typeform_webhook.root_resource_id
  path_part   = "webhook"
}

resource "aws_api_gateway_method" "post" {
  rest_api_id   = aws_api_gateway_rest_api.typeform_webhook.id
  resource_id   = aws_api_gateway_resource.webhook.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "lambda" {
  rest_api_id = aws_api_gateway_rest_api.typeform_webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.post.http_method
  
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.webhook_handler.invoke_arn
}
```

## Data Models

### Survey Metadata

```go
type SurveyMetadata struct {
    FormID       string    `json:"form_id"`
    FormURL      string    `json:"form_url"`
    CreatedAt    time.Time `json:"created_at"`
    ObjectType   string    `json:"object_type"`
    ObjectID     string    `json:"object_id"`
    CustomerCode string    `json:"customer_code"`
}
```

### Survey Response

```go
type SurveyResponse struct {
    EventID      string                 `json:"event_id"`
    EventType    string                 `json:"event_type"`
    FormResponse FormResponseData       `json:"form_response"`
    ReceivedAt   time.Time              `json:"received_at"`
}

type FormResponseData struct {
    FormID      string            `json:"form_id"`
    Token       string            `json:"token"`
    SubmittedAt time.Time         `json:"submitted_at"`
    Hidden      map[string]string `json:"hidden"`
    Answers     []Answer          `json:"answers"`
}

type Answer struct {
    Field   FieldInfo   `json:"field"`
    Type    string      `json:"type"`
    Number  *int        `json:"number,omitempty"`
    Boolean *bool       `json:"boolean,omitempty"`
    Text    *string     `json:"text,omitempty"`
}

type FieldInfo struct {
    ID   string `json:"id"`
    Type string `json:"type"`
}
```

## Error Handling

### Email Sending Errors

- **Typeform API Failure**: Log error, continue with email without survey link
- **QR Code Generation Failure**: Log error, send email with URL only
- **SES Failure**: Retry with exponential backoff (existing pattern)

### Webhook Processing Errors

- **Invalid Signature**: Return 401 Unauthorized, log security event
- **Malformed Payload**: Return 400 Bad Request, log error
- **S3 Storage Failure**: Retry within Lambda timeout, return 500 if all retries fail
- **Duplicate Webhook**: Return 200 OK (idempotent operation)

### Error Response Codes

```go
const (
    StatusOK                  = 200
    StatusBadRequest          = 400
    StatusUnauthorized        = 401
    StatusInternalServerError = 500
)
```

## Testing Strategy

### Unit Tests

1. **Configuration Loading**
   - Test default FQDN value
   - Test configuration validation
   - Test missing required fields

2. **URL Generation**
   - Test approval link generation with customer code
   - Test survey URL generation
   - Test QR code generation

3. **Typeform Client**
   - Mock HTTP client for API calls
   - Test survey creation request formatting
   - Test error handling for API failures

4. **Webhook Handler**
   - Test signature validation (valid/invalid)
   - Test payload parsing
   - Test S3 key generation
   - Test idempotency logic

### Integration Tests

1. **Email Flow**
   - Send test approval email with customer-filtered link
   - Verify link format and parameters
   - Send test completion email with survey

2. **Typeform Integration**
   - Create test survey via API
   - Verify survey structure and hidden fields
   - Test survey embedding in frontend

3. **Webhook Processing**
   - Send test webhook payload to API Gateway
   - Verify Lambda invocation
   - Verify S3 storage of response
   - Test duplicate webhook handling

### Manual Testing

1. **Frontend Survey Tab**
   - Load survey tab in portal
   - Verify Typeform embed renders correctly
   - Submit test survey and verify confirmation

2. **End-to-End Flow**
   - Complete a change/announcement
   - Receive completion email with survey
   - Submit survey via email link
   - Verify webhook delivery and S3 storage
   - View survey in portal tab

## Security Considerations

### Webhook Security

- **Signature Validation**: HMAC-SHA256 signature verification for all webhook requests
- **HTTPS Only**: API Gateway enforces HTTPS
- **Secret Management**: Store webhook secret in AWS Systems Manager Parameter Store

### Secrets Management

- **Storage**: Typeform API key and webhook secret stored in AWS Systems Manager Parameter Store with encryption
- **Configuration**: Workspace ID stored in config.json (not sensitive)
- **Runtime**: Secrets injected as environment variables (`TYPEFORM_API_KEY`, `TYPEFORM_WEBHOOK_SECRET`) at application/Lambda startup
- **Application**: Code reads secrets from environment variables, not directly from SSM
- **Rotation**: Support key rotation by updating SSM parameters and restarting the application/Lambda
- **Deployment**: Infrastructure/deployment scripts handle loading SSM parameters into environment variables

### Data Privacy

- **PII Handling**: Survey responses may contain customer feedback
- **Access Control**: S3 bucket policies restrict access to authorized services
- **Encryption**: S3 server-side encryption enabled

## Performance Considerations

### Rate Limiting

- **Typeform API**: 60 requests per minute (Create API)
- **Implementation**: Use existing rate limiter pattern from `internal/ses/operations.go`

### Lambda Configuration

- **Memory**: 512 MB (sufficient for JSON processing)
- **Timeout**: 30 seconds (webhook processing should be fast)
- **Concurrency**: 10 concurrent executions (adjust based on volume)

### S3 Performance

- **Key Structure**: Optimized for retrieval by customer and object ID
- **Partitioning**: Natural partitioning by customer code
- **Listing**: Efficient prefix-based listing for object-specific queries

## Deployment Considerations

### Configuration Updates

1. Add new fields to `config.json`
2. Update configuration validation
3. Deploy updated Lambda backend
4. Deploy frontend changes
5. Configure Typeform webhook URL in Typeform dashboard

### Rollback Strategy

- **Email Templates**: Keep old templates as fallback
- **Lambda Versions**: Use Lambda versioning and aliases
- **Frontend**: Deploy behind feature flag if needed

### Monitoring

- **CloudWatch Metrics**: Lambda invocations, errors, duration
- **CloudWatch Logs**: Structured logging for debugging
- **Alarms**: Alert on webhook processing failures
- **Dashboard**: Track survey submission rates and response times
