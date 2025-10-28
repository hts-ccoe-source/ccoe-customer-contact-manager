# Design Document: Per-Customer Additional Contacts

## Overview

This feature enables management of additional contacts (people without AWS Access) on a per-customer basis. Additional contacts are email addresses subscribed to specific topics that receive notifications through AWS SES. The system follows the existing architectural pattern: HTML → Frontend Node API → S3 → SQS → Golang Backend, with workflow state management (draft → submitted → approved → completed) and email uniqueness validation across customer lists.

## Architecture

### High-Level Flow

```
┌─────────────────┐
│  HTML Pages     │
│  - View         │
│  - Add Contacts │
└────────┬────────┘
         │ POST /api/contacts/*
         ▼
┌─────────────────┐
│ Frontend Node   │
│ API Lambda      │
│ (Function URL)  │
└────────┬────────┘
         │ Write to S3
         ▼
┌─────────────────┐
│ S3 Bucket       │
│ drafts/         │
│ customers/      │
│ archive/        │
└────────┬────────┘
         │ S3 Event
         ▼
┌─────────────────┐
│ SQS Queue       │
│ (per-customer)  │
└────────┬────────┘
         │ Pop message
         ▼
┌─────────────────┐
│ Golang Backend  │
│ Lambda          │
│ - Validate      │
│ - Add to SES    │
│ - Update S3     │
└─────────────────┘
```

### S3 Key Structure

```
drafts/{TACT-GUID}.json                    # Draft contact imports
customers/{customer-code}/{TACT-GUID}.json # Ephemeral trigger for import event (deleted after processing)
archive/{TACT-GUID}.json                   # Authoritative copy (created on submit, updated on completion)
```

### S3 Object Metadata

All contact import objects use S3 metadata fields:
- `x-amz-meta-status`: Current workflow status (draft, submitted, approved, completed)
- `x-amz-meta-version`: Version number for optimistic locking
- `x-amz-meta-object-id`: TACT-GUID identifier
- `x-amz-meta-completed-at`: Timestamp when processing completed
- `x-amz-meta-completed-by`: User who completed the import
- `x-amz-meta-object-type`: Set to "contact_import"

## Components and Interfaces

### 1. HTML Frontend

#### View Additional Contacts Page (`html/view-contacts.html`)

**Purpose**: Display all additional contacts for selected customer(s)

**Features**:
- Customer selector dropdown (multi-select)
- Contact list table with columns:
  - Email Address
  - Customer
  - Topics (comma-separated)
  - Date Added
  - Status
- Filter by topic
- Sort by email, customer, or date
- Empty state with "Add Contacts" CTA
- Pagination for large lists

**API Calls**:
- `GET /contacts?customers={codes}` - Retrieve contacts for customer(s)
- `GET /contacts/topics` - Get available topics

#### Add Additional Contacts Page (`html/add-contacts.html`)

**Purpose**: Add new email addresses as additional contacts

**Features**:
- Customer selector (single customer per import)
- Topic selector (multi-select checkboxes)
- Three input methods:
  1. **Text Input**: Textarea accepting comma, semicolon, or newline-separated emails
  2. **CSV Upload**: File input with template download link
  3. **Excel Upload**: File input with template download link
- Template download links for CSV and Excel formats
- Email validation preview before submission
- Submit button creates draft → transitions to submitted (duplicate detection happens in backend)

**Template Files**:
- `templates/contact-import-template.csv`
- `templates/contact-import-template.xlsx`

**Template Columns**:
- Customer (customer code)
- Email Address
- Topics (comma-separated)

**API Calls**:
- `POST /contacts/drafts` - Create draft import
- `POST /contacts/{TACT-GUID}/submit` - Submit draft for processing

### 2. Frontend Node API Lambda

**New Endpoints**:

#### `GET /contacts`
**Query Parameters**:
- `customers`: Comma-separated customer codes
- `topic`: Optional topic filter
- `status`: Optional status filter (default: all)

**Response**:
```json
{
  "success": true,
  "contacts": [
    {
      "email": "user@example.com",
      "customer": "hts",
      "topics": ["announce", "calendar"],
      "dateAdded": "2025-10-27T14:00:00Z",
      "status": "active"
    }
  ]
}
```

#### `GET /contacts/topics`
**Response**:
```json
{
  "success": true,
  "topics": [
    {
      "name": "announce",
      "displayName": "Change Announcements",
      "description": "Announce what/why/when for CCOE Changes"
    }
  ]
}
```

#### `POST /contacts/drafts`
**Request Body**:
```json
{
  "customer": "hts",
  "contacts": [
    {
      "email": "user1@example.com",
      "topics": ["announce", "calendar"]
    },
    {
      "email": "user2@example.com",
      "topics": ["announce"]
    }
  ],
  "source": "web_form|csv_upload|excel_upload"
}
```

**Response**:
```json
{
  "success": true,
  "draftId": "TACT-abc123",
  "s3Key": "drafts/TACT-abc123.json",
  "emailCount": 2
}
```

#### `POST /contacts/{TACT-GUID}/submit`
**Request Body**:
```json
{
  "draftId": "TACT-abc123"
}
```

**Response**:
```json
{
  "success": true,
  "importId": "TACT-abc123",
  "status": "submitted",
  "message": "Contact import submitted for processing"
}
```

**Implementation Details**:
- Extract user identity from `x-user-email` header (added by Lambda@Edge)
- Generate contact import ID using UUID v4 format with `TACT-` prefix (same pattern as announcements): `TACT-xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`
- Write JSON to S3 with appropriate metadata
- For submit operation: copy from `drafts/` to `customers/{customer}/` and `archive/`
- Trigger S3 events for backend processing

### 3. Golang Backend Lambda

**New Handler**: `ProcessContactImport`

**Responsibilities**:
1. Detect contact import objects by `x-amz-meta-object-type: contact_import`
2. Check if trigger still exists (idempotency - Transient Trigger Pattern)
3. Load authoritative data from `archive/{TACT-GUID}.json`
4. Load all existing contacts from all customers (once per import)
5. Validate email uniqueness (except htsnonprod)
6. Add valid contacts to SES contact lists
7. Subscribe contacts to specified topics and respect topic OPT_IN/OPT_OUT settings from SESConfig.json
8. Update archive object with processing results
9. Delete trigger from `customers/{customer-code}/`

**New Package**: `internal/contacts/additional_contacts.go`

**Key Functions**:

```go
// ProcessContactImport handles contact import workflow
func ProcessContactImport(ctx context.Context, bucket, key string, cfg *types.Config) error

// LoadAllExistingContacts retrieves all additional contacts from all customers
func LoadAllExistingContacts(ctx context.Context, bucket string, cfg *types.Config) (map[string]string, error)

// ValidateEmailUniqueness checks if emails are unique across customers
func ValidateEmailUniqueness(emails []string, customer string, existing map[string]string) ([]string, []EmailDuplicate, error)

// AddContactsToSES adds validated contacts to SES contact list
func AddContactsToSES(ctx context.Context, customer string, emails []string, topics []string, cfg *types.Config) error

// SubscribeContactsToTopics subscribes contacts to specified topics
func SubscribeContactsToTopics(ctx context.Context, sesClient *sesv2.Client, listName string, emails []string, topics []string) error
```

**Error Handling**:
- Retryable: SES API throttling, temporary network errors
- Non-retryable: Invalid email format, uniqueness violations, invalid customer code
- Partial success: Track successful and failed emails separately

**Idempotency**:
- Check if contact already exists in SES before adding
- Get all contacts from all customers first once and compare to contact
- Skip already-subscribed contacts for topics

## Data Models

### ContactImport (S3 JSON Object)

```json
{
  "object_type": "contact_import",
  "import_id": "TACT-abc123",
  "customer": "hts",
  "contacts": [
    {
      "email": "user1@example.com",
      "topics": ["announce", "calendar"]
    },
    {
      "email": "user2@example.com",
      "topics": ["announce"]
    }
  ],
  "source": "web_form",
  "status": "submitted",
  "version": 1,
  "created_at": "2025-10-27T14:00:00Z",
  "created_by": "user@hearst.com",
  "submitted_at": "2025-10-27T14:05:00Z",
  "submitted_by": "user@hearst.com",
  "processing_results": {
    "successful": [
      {
        "email": "user1@example.com",
        "topics": ["announce", "calendar"]
      }
    ],
    "failed": [
      {
        "email": "user2@example.com",
        "topics": ["announce"],
        "reason": "duplicate_in_customer_cds"
      }
    ],
    "processed_at": "2025-10-27T14:10:00Z"
  }
}
```

### EmailDuplicate (Go Struct)

```go
type EmailDuplicate struct {
    Email            string `json:"email"`
    ExistingCustomer string `json:"existing_customer"`
    CanOverride      bool   `json:"can_override"`
}
```

### ContactImportMetadata (Go Struct)

```go
type ContactImportMetadata struct {
    ObjectType        string              `json:"object_type"`
    ImportID          string              `json:"import_id"`
    Customer          string              `json:"customer"`
    Contacts          []ContactEntry      `json:"contacts"`
    Source            string              `json:"source"`
    Status            string              `json:"status"`
    Version           int                 `json:"version"`
    CreatedAt         time.Time           `json:"created_at"`
    CreatedBy         string              `json:"created_by"`
    SubmittedAt       *time.Time          `json:"submitted_at,omitempty"`
    SubmittedBy       string              `json:"submitted_by,omitempty"`
    ProcessingResults *ProcessingResults  `json:"processing_results,omitempty"`
}

type ContactEntry struct {
    Email  string   `json:"email"`
    Topics []string `json:"topics"`
}

type ProcessingResults struct {
    Successful  []ContactEntry        `json:"successful"`
    Failed      []FailedContactEntry  `json:"failed"`
    ProcessedAt time.Time             `json:"processed_at"`
}

type FailedContactEntry struct {
    Email  string   `json:"email"`
    Topics []string `json:"topics"`
    Reason string   `json:"reason"`
}
```

## Error Handling

### Frontend Validation Errors
- **Empty email list**: "Please provide at least one email address"
- **Invalid email format**: "Invalid email format: {email}"
- **No customer selected**: "Please select a customer"
- **No topics selected**: "Please select at least one topic"

### Backend Validation Errors
- **Duplicate email**: "Email {email} already exists in customer {customer}"
- **Invalid customer code**: "Customer code {code} not found"
- **SES contact list not found**: "SES contact list not configured for customer {customer}"

### SES Operation Errors
- **Throttling**: Retry with exponential backoff (retryable)
- **Contact already exists**: Skip and continue (non-error)
- **Invalid email address**: Mark as failed, continue with others (non-retryable)
- **Topic not found**: Mark as failed, continue with others (non-retryable)

### Partial Success Handling
When some emails succeed and others fail:
1. Complete all successful operations
2. Update S3 object with detailed results
3. Set status to "completed_with_errors"
4. Return summary with counts of successful/failed

## Testing Strategy

### Unit Tests

**Frontend Node API**:
- TACT-GUID generation uniqueness
- Email parsing from different formats (comma, semicolon, newline)
- CSV/Excel file parsing
- S3 key generation for different workflow states
- User identity extraction from headers

**Golang Backend**:
- Email uniqueness validation logic
- htsnonprod exception handling
- SES contact addition idempotency
- Topic subscription logic
- Partial success result aggregation

### Integration Tests

**End-to-End Workflow**:
1. Create draft via API
2. Verify draft object in S3
3. Submit draft
4. Verify objects copied to customers/ and contacts/
5. Verify SQS message sent
6. Process via backend
7. Verify contacts added to SES
8. Verify topics subscribed
9. Verify archive object created

**Email Uniqueness**:
1. Add contact to customer A
2. Attempt to add same contact to customer B (should fail)
3. Attempt to add same contact to htsnonprod (should succeed)

**Partial Success**:
1. Submit import with mix of valid and duplicate emails
2. Verify valid emails added to SES
3. Verify duplicate emails marked as failed
4. Verify status set to "completed_with_errors"

### Manual Testing

**UI Testing**:
- Test all three input methods (text, CSV, Excel)
- Verify template downloads work
- Test customer and topic selectors
- Verify validation messages display correctly
- Test empty states and error states

**SES Verification**:
- Verify contacts appear in SES console
- Verify topic subscriptions are correct
- Send test email to topic and verify delivery

## Security Considerations

### Authentication
- All API endpoints require valid `x-user-email` header from Lambda@Edge
- User must have `@hearst.com` domain

### Authorization
- Users can only add contacts for customers they have access to
- Future enhancement: Role-based restrictions per customer

### Email Validation
- Validate email format using RFC 5322 regex
- Prevent email injection attacks
- Sanitize all user inputs

### Data Privacy
- Email addresses are PII - handle appropriately
- Log operations without exposing full email addresses
- Implement data retention policy for archived imports

## Performance Considerations

### Batch Operations
- Load all existing contacts once per import (not per email)
- Use SES batch operations where available
- Process emails concurrently with goroutines (limit: 10 concurrent)

### Caching
- Cache SES contact list names per customer
- Cache topic configurations from SESConfig.json
- Cache customer mappings from config.json

### Rate Limiting
- Implement exponential backoff for SES API calls
- Respect SES rate limits (1 TPS for CreateContact)
- Use AWS SDK built-in retry logic

## Monitoring and Logging

### CloudWatch Metrics
- Contact imports created (by customer)
- Contact imports completed (by customer)
- Contact imports failed (by customer)
- Emails added to SES (by customer)
- Duplicate email rejections (by customer)

### CloudWatch Logs
- Log each contact import with import_id
- Log validation failures with reasons
- Log SES operations (success/failure)
- Log partial success details

### Alarms
- High failure rate for contact imports (> 50%)
- SES API throttling errors
- Duplicate email rate spike (> 20%)

## Future Enhancements

### Phase 2
- Bulk contact removal
- Contact update (change topics)
- Contact export to CSV
- Audit log of all contact operations

### Phase 3
- Self-service contact management portal
- Email verification workflow (double opt-in)
- Contact import approval workflow
- Scheduled contact list synchronization

### Phase 4
- Integration with Identity Center for automatic contact sync
- Contact segmentation by attributes
- A/B testing for email campaigns
- Analytics dashboard for contact engagement
