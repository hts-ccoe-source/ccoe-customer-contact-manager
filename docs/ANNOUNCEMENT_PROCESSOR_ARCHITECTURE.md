# Announcement Processor Architecture

## Overview

This document describes the new announcement processing architecture that separates announcement handling from change management, ensuring data integrity and preventing the "Untitled announcement" bug.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    S3 Event Notification                     │
│                  (object_type: announcement_*)               │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              internal/lambda/handlers.go                     │
│                                                              │
│  ProcessS3Event()                                            │
│    ├─ Extract customer code from S3 key                     │
│    ├─ Check object_type field                               │
│    └─ Route to handleAnnouncementEventNew()                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│        internal/lambda/announcement_handlers.go              │
│                                                              │
│  handleAnnouncementEventNew()                                │
│    ├─ downloadAnnouncementFromS3()                          │
│    │    └─ Parse as AnnouncementMetadata (NO CONVERSION)    │
│    ├─ validateAnnouncement()                                │
│    └─ Create AnnouncementProcessor                          │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│        internal/processors/announcement_processor.go         │
│                                                              │
│  AnnouncementProcessor.ProcessAnnouncement()                 │
│    ├─ Switch on announcement.Status                         │
│    ├─ "submitted" → handleSubmitted()                       │
│    ├─ "approved" → handleApproved()                         │
│    ├─ "cancelled" → handleCancelled()                       │
│    └─ "completed" → handleCompleted()                       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│          internal/ses/announcement_templates.go              │
│                                                              │
│  Email Template Functions                                    │
│    ├─ GetAnnouncementTemplate()                             │
│    ├─ GetAnnouncementApprovalRequestTemplate()              │
│    ├─ GetAnnouncementCancellationTemplate()                 │
│    └─ GetAnnouncementCompletionTemplate()                   │
│                                                              │
│  Type-Specific Templates                                     │
│    ├─ getCICTemplate()                                      │
│    ├─ getFinOpsTemplate()                                   │
│    ├─ getInnerSourceTemplate()                              │
│    └─ getGenericTemplate()                                  │
└─────────────────────────────────────────────────────────────┘
```

## Key Components

### 1. Announcement Handlers (`internal/lambda/announcement_handlers.go`)

**Purpose**: Entry point for announcement processing from S3 events.

**Key Functions**:
- `handleAnnouncementEventNew()`: Main handler that downloads and validates announcements
- `downloadAnnouncementFromS3()`: Downloads and parses AnnouncementMetadata from S3
- `validateAnnouncement()`: Validates announcement structure
- Status-specific handlers: `handleAnnouncementSubmitted()`, `handleAnnouncementApproved()`, etc.

**Critical Design Decision**: Announcements are parsed directly as `AnnouncementMetadata` and never converted to `ChangeMetadata`.

### 2. Announcement Processor (`internal/processors/announcement_processor.go`)

**Purpose**: Core business logic for announcement processing.

**Structure**:
```go
type AnnouncementProcessor struct {
    S3Client   *s3.Client
    SESClient  *sesv2.Client
    GraphToken string
    Config     *types.Config
}
```

**Key Methods**:
- `ProcessAnnouncement()`: Routes to status-specific handlers
- `handleSubmitted()`: Sends approval request emails
- `handleApproved()`: Schedules meetings (if needed) and sends announcement emails
- `handleCancelled()`: Cancels meetings and sends cancellation emails
- `handleCompleted()`: Sends completion emails

**Email Functions**:
- `sendApprovalRequest()`
- `sendAnnouncementEmails()`
- `sendCancellationEmail()`
- `sendCompletionEmail()`

**Helper Functions**:
- `convertToAnnouncementData()`: Converts AnnouncementMetadata to AnnouncementData for templates
- `getTopicNameForAnnouncementType()`: Maps announcement types to SES topics
- `sendEmailViaSES()`: Sends emails using SES topic management

### 3. Email Templates (`internal/ses/announcement_templates.go`)

**Purpose**: Type-specific email templates for announcements.

**Data Structure**:
```go
type AnnouncementData struct {
    AnnouncementID   string
    AnnouncementType string
    Title            string
    Summary          string
    Content          string
    Customers        []string
    MeetingMetadata  *types.MeetingMetadata
    Attachments      []string
    Author           string
    PostedDate       time.Time
    CreatedBy        string
    CreatedAt        time.Time
}
```

**Template Functions**:
- `GetAnnouncementTemplate()`: Returns template for approved announcements
- `GetAnnouncementApprovalRequestTemplate()`: Returns template for approval requests
- `GetAnnouncementCancellationTemplate()`: Returns template for cancellations
- `GetAnnouncementCompletionTemplate()`: Returns template for completions

**Type-Specific Templates**:
- CIC (Cloud Innovator Community): Blue theme
- FinOps: Green theme
- InnerSource: Purple theme
- Generic: Blue theme

## Data Flow

### Approved Announcement Flow

```
1. S3 Event (object_type: "announcement_cic", status: "approved")
   ↓
2. handleAnnouncementEventNew()
   ├─ Downloads announcement from S3
   ├─ Parses as AnnouncementMetadata
   └─ Validates structure
   ↓
3. AnnouncementProcessor.ProcessAnnouncement()
   ├─ Checks status: "approved"
   └─ Calls handleApproved()
   ↓
4. handleApproved()
   ├─ If IncludeMeeting: scheduleMeeting()
   └─ sendAnnouncementEmails()
   ↓
5. sendAnnouncementEmails()
   ├─ convertToAnnouncementData()
   ├─ GetAnnouncementTemplate("cic", data)
   └─ sendEmailViaSES()
   ↓
6. Email sent to SES topic subscribers
```

### Cancelled Announcement Flow

```
1. S3 Event (object_type: "announcement_finops", status: "cancelled")
   ↓
2. handleAnnouncementEventNew()
   ↓
3. AnnouncementProcessor.ProcessAnnouncement()
   ├─ Checks status: "cancelled"
   └─ Calls handleCancelled()
   ↓
4. handleCancelled()
   ├─ If MeetingMetadata exists: cancelMeeting()
   └─ sendCancellationEmail()
   ↓
5. sendCancellationEmail()
   ├─ convertToAnnouncementData()
   ├─ GetAnnouncementCancellationTemplate("finops", data)
   └─ sendEmailViaSES()
   ↓
6. Cancellation email sent to SES topic subscribers
```

## Key Design Principles

### 1. No Data Conversion

**Problem**: Previous implementation converted `AnnouncementMetadata` to `ChangeMetadata`, causing data loss.

**Solution**: Announcements remain as `AnnouncementMetadata` throughout their entire lifecycle.

```go
// OLD (BROKEN) - Don't do this
var announcement types.AnnouncementMetadata
json.Unmarshal(data, &announcement)
metadata := convertToChangeMetadata(announcement) // DATA LOSS HERE

// NEW (CORRECT) - Do this
var announcement types.AnnouncementMetadata
json.Unmarshal(data, &announcement)
processor.ProcessAnnouncement(ctx, customerCode, &announcement, bucket, key)
```

### 2. Separation of Concerns

- **Handlers**: Route events and validate data
- **Processor**: Business logic and orchestration
- **Templates**: Email rendering and formatting

### 3. Type Safety

All functions work with `*types.AnnouncementMetadata` directly:

```go
func (p *AnnouncementProcessor) ProcessAnnouncement(
    ctx context.Context,
    customerCode string,
    announcement *types.AnnouncementMetadata,  // NOT ChangeMetadata
    s3Bucket, s3Key string,
) error
```

### 4. Idempotency

All operations are designed to be idempotent:
- Email sending uses SES topic management
- Meeting scheduling checks for existing meetings
- S3 updates use conditional writes

## SES Topic Mapping

Announcement types map to SES topics:

| Announcement Type | SES Topic Pattern |
|------------------|-------------------|
| `cic` | `{customer-code}-cloud-innovator-community` |
| `finops` | `{customer-code}-finops` |
| `innersource` | `{customer-code}-innersource-guild` |
| `general` | `{customer-code}-general-announcements` |

Example: For customer "hts" with announcement type "cic", emails are sent to topic "hts-cloud-innovator-community".

## Error Handling

### Retryable Errors

- SES throttling errors
- Temporary S3 access issues
- Network timeouts

### Non-Retryable Errors

- Invalid announcement structure
- Missing required fields
- Unknown customer codes
- Malformed JSON

### Error Classification

```go
if err := validateAnnouncement(announcement); err != nil {
    // Non-retryable - bad data
    return fmt.Errorf("invalid announcement: %w", err)
}

if err := sendEmail(); err != nil {
    // Retryable - temporary issue
    return fmt.Errorf("failed to send email: %w", err)
}
```

## Testing Strategy

### Unit Tests

- Template rendering with various data
- Announcement validation
- Data conversion functions

### Integration Tests

- End-to-end announcement lifecycle
- Email delivery verification
- Meeting scheduling integration

### Test Data

```json
{
  "object_type": "announcement_cic",
  "announcement_id": "CIC-2025-001",
  "announcement_type": "cic",
  "title": "Test Announcement",
  "summary": "Test summary",
  "content": "Test content",
  "customers": ["hts", "cds"],
  "include_meeting": true,
  "status": "approved",
  "modifications": []
}
```

## Monitoring

### CloudWatch Metrics

- Announcement processing count by type
- Email delivery success rate
- Meeting scheduling success rate
- Error rates by error type

### CloudWatch Logs

Key log patterns to monitor:

```
📢 Processing announcement event for customer
✅ Announcement processing completed
❌ Failed to process announcement
📧 Sending announcement emails
```

### Alerts

- High error rate (>5% of announcements)
- Email delivery failures
- Meeting scheduling failures
- "Untitled announcement" occurrences (should be zero)

## Migration Path

### Phase 1: Deploy New Code (Completed)

- ✅ Create `AnnouncementProcessor`
- ✅ Create `announcement_handlers.go`
- ✅ Update routing in `handlers.go`
- ✅ Add new email template functions

### Phase 2: Cleanup (In Progress)

- ✅ Create cleanup script
- ✅ Document cleanup process
- ⏳ Run cleanup script (dry-run)
- ⏳ Run cleanup script (actual deletion)

### Phase 3: Verification

- Verify no announcement objects remain
- Deploy new Lambda code
- Create test announcements
- Verify email delivery
- Monitor for issues

### Phase 4: Monitoring

- Watch CloudWatch Logs
- Monitor error rates
- Check for "Untitled announcement" reports
- Verify meeting scheduling works

## Troubleshooting

### "Untitled announcement" Bug

**Symptom**: Announcements show "Untitled announcement" in emails

**Root Cause**: Conversion from AnnouncementMetadata to ChangeMetadata loses the `title` field

**Fix**: Ensure announcements are processed as AnnouncementMetadata throughout

**Verification**:
```bash
# Check CloudWatch Logs for conversion
grep "Convert announcement to ChangeMetadata" /aws/lambda/your-function

# Should return no results after fix
```

### Missing Announcement Fields

**Symptom**: Emails missing summary, content, or other fields

**Root Cause**: Data loss during conversion or incomplete AnnouncementMetadata

**Fix**: Verify announcement structure in S3:
```bash
aws s3api get-object \
  --bucket your-bucket \
  --key customers/hts/CIC-2025-001.json \
  /dev/stdout | jq .
```

### Email Not Sent

**Symptom**: Announcement approved but no email received

**Root Cause**: SES topic misconfiguration or no subscribers

**Fix**: Check SES topic and subscribers:
```bash
# List topics
aws sesv2 list-contact-lists

# List contacts in topic
aws sesv2 list-contacts --contact-list-name main-contact-list
```

## Related Documentation

- [Announcement Cleanup Guide](./ANNOUNCEMENT_CLEANUP_GUIDE.md)
- [Object Model Enhancement Spec](../.kiro/specs/object-model-enhancement/)
- [Frontend Display Enhancements Spec](../.kiro/specs/frontend-display-enhancements/)
- [SES Operations](../internal/ses/operations.go)

## Future Enhancements

1. **Meeting Scheduling**: Implement Microsoft Graph API integration in AnnouncementProcessor
2. **S3 Save**: Implement SaveAnnouncementToS3() to update meeting metadata
3. **Attachment Handling**: Add S3 presigned URLs for attachments
4. **Rich Content**: Support HTML content in announcements
5. **Scheduling**: Support scheduled announcement delivery
