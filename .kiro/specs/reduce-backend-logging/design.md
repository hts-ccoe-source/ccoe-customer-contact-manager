# Design Document: Reduce Backend Logging

## Overview

This design reduces verbose logging in the Go Lambda backend to improve CloudWatch log readability while maintaining essential error, warning, and critical operation logs. The approach focuses on removing debug-level success messages, emoji characters, and redundant logging while preserving all information needed for troubleshooting.

## Current Logging State

The codebase has **two logging systems**:

1. **`log.Printf` (518 calls)** - Used throughout Lambda handler, processors, and most operations
   - Contains emoji characters (📧, ✅, ❌, ⚠️, etc.)
   - Verbose debug messages
   - No structured logging
   - Used in: main.go Lambda handler, announcement_processor.go, meetings.go, handlers.go

2. **`slog` (55 calls)** - Used only in specific CLI commands
   - Supports JSON and text output formats
   - Structured logging with key-value pairs
   - Used in: SES domain configuration, deliverability configuration
   - Already clean (no emoji, good structure)

**Strategy:** Migrate all `log.Printf` calls to `slog` for consistency and structured logging.

## Architecture

### Current State

The backend Lambda has excessive logging including:
- Individual message processing logs for each SQS message
- Detailed parsing logs for successful S3 event parsing
- Customer code extraction logs for every extraction
- Emoji characters in log messages (📧, ✅, ❌, ⚠️, etc.)
- Decorative separators and formatting
- Verbose success messages for routine operations

### Target State

Streamlined logging with:
- Single summary line per Lambda invocation
- Error and warning logs with full context
- Critical operation logs (meeting scheduling, email sending)
- Clean, parseable log format without emoji
- Consistent error prefixes for log analysis tools

## Migration Strategy: log.Printf → slog

### Step 1: Initialize slog in Lambda Handler

Add slog initialization at the start of the Lambda handler:

```go
// Setup structured logging
// Default to JSON for Lambda (CloudWatch), text for CLI (human-readable)
isLambda := os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
logFormat := os.Getenv("LOG_FORMAT")

var handler slog.Handler
if logFormat == "text" || (!isLambda && logFormat == "") {
    // CLI mode: default to text (human-readable)
    handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
} else {
    // Lambda mode: default to JSON (CloudWatch)
    handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
}
logger := slog.New(handler)
slog.SetDefault(logger) // Set as default logger
```

### Step 2: Pass Logger to Components

Update function signatures to accept `*slog.Logger`:
- `AnnouncementProcessor` - add logger field
- `ProcessS3Event` - accept logger parameter
- Other processors and handlers

### Step 3: Convert log.Printf to slog

**Migration Patterns:**

**Errors:**
```go
// Before:
log.Printf("❌ Failed to process: %v", err)

// After:
logger.Error("failed to process", "error", err)
```

**Warnings:**
```go
// Before:
log.Printf("⚠️  Warning: No subscribers found")

// After:
logger.Warn("no subscribers found")
```

**Info (Critical Operations Only):**
```go
// Before:
log.Printf("✅ Meeting scheduled: %s", meetingID)

// After:
logger.Info("meeting scheduled", "meeting_id", meetingID)
```

**Debug (Remove Most):**
```go
// Before:
log.Printf("📧 Sending email to %d recipients", count)

// After (if keeping):
logger.Debug("sending email", "recipient_count", count)

// Or remove entirely if too verbose
```

### Logging Levels & Reduction Strategy

**Goal: Reduce 518 log statements to ~50-100 essential logs**

**Use slog.Error (Keep All):**
- All error conditions with context
- Failed operations (API calls, S3, SES, Graph API)
- Exceptions and panics
- **Example:** `logger.Error("failed to send email", "error", err, "customer", code)`

**Use slog.Warn (Keep Selectively):**
- Event loop prevention discards
- Missing/invalid configuration
- Fallback behaviors
- Partial failures (some emails sent, some failed)
- **Example:** `logger.Warn("no subscribers found", "topic", topicName)`

**Use slog.Info (Keep Only Critical Operations):**
- Lambda invocation start/end summary (ONE log per invocation)
- Meeting scheduled/cancelled (with meeting_id)
- Email sent (with type, recipient_count, customer)
- Major state transitions (submitted → approved)
- **Example:** `logger.Info("meeting scheduled", "meeting_id", id, "customer", code)`
- **Target:** ~10-20 Info logs per Lambda invocation

**Use slog.Debug (Minimal, for troubleshooting):**
- S3 event parsing details (only if needed)
- Customer code extraction (only if needed)
- **Default:** Set log level to Info in production, Debug only when troubleshooting
- **Target:** ~5-10 Debug logs per invocation

### What to DELETE Entirely (Don't Convert)

**Remove ~400+ of the 518 log statements:**
- ❌ "Processing customer X" - redundant with summary
- ❌ "Starting processing" / "Completed processing" - use summary instead
- ❌ Individual SQS message processing logs
- ❌ Successful S3 event parsing details
- ❌ Customer code extraction success logs
- ❌ "Sending email to topic X" - keep only final "email sent" with count
- ❌ "Retrieved N contacts" - too verbose
- ❌ "Assuming role X" - only log if it fails
- ❌ "Loading config" - only log if it fails
- ❌ Step-by-step processing logs
- ❌ All emoji characters
- ❌ Decorative separators
- ❌ Redundant success messages

### Consolidation Examples

**Before (5 logs):**
```go
log.Printf("📧 Processing announcement for customer %s", code)
log.Printf("✅ Announcement %s is approved", id)
log.Printf("📅 Scheduling meeting for announcement %s", id)
log.Printf("✅ Successfully scheduled meeting %s", meetingID)
log.Printf("📧 Sending announcement emails for %s", id)
```

**After (2 logs):**
```go
logger.Info("processing announcement", "id", id, "customer", code, "status", "approved")
logger.Info("meeting scheduled", "meeting_id", meetingID, "announcement_id", id)
// Email log happens in sendEmail function
```

**Before (3 logs):**
```go
log.Printf("🔐 Customer %s: Assuming SES role: %s", code, roleArn)
log.Printf("📥 Customer %s: Importing contacts", code)
log.Printf("✅ Customer %s: Successfully imported contacts", code)
```

**After (0 logs in success case, 1 log if error):**
```go
// Only log if error occurs:
logger.Error("failed to import contacts", "customer", code, "error", err)
// Success is implied by absence of error + summary log
```

## Implementation Approach

### Phase 1: Identify Log Statements

Search for all `log.Printf()` and `slog.*()` calls in:
- `main.go`
- `internal/lambda/handlers.go`
- `internal/processors/announcement_processor.go`
- `internal/ses/meetings.go`
- `internal/ses/operations.go`

### Phase 2: Categorize Logs

For each log statement, determine:
1. Is it an error? → KEEP
2. Is it a warning? → KEEP
3. Is it a critical operation? → KEEP
4. Is it a debug/success message? → REMOVE
5. Does it contain emoji? → REMOVE emoji, keep message if needed

### Phase 3: Apply Changes

**Remove entirely:**
- Debug success messages
- Verbose processing logs
- Redundant status updates

**Clean up (remove emoji, keep message):**
- Error logs: `❌ Failed to...` → `ERROR: Failed to...`
- Warning logs: `⚠️ Warning:...` → `WARNING: ...`
- Info logs: `✅ Success:...` → `INFO: ...` (if keeping)

**Add summary logging:**
- At end of Lambda handler, log single summary line

## Log Format Standards

### Error Format
```
ERROR: {operation} failed: {reason}
```

### Warning Format
```
WARNING: {condition}: {details}
```

### Info Format (Critical Operations Only)
```
INFO: {operation}: {key_details}
```

### Summary Format
```
SUMMARY: Lambda complete: processed={count} errors={count} duration={ms}ms
```

## Concurrent Logging Considerations

### CustomerLogBuffer Pattern

The codebase uses a `CustomerLogBuffer` pattern for concurrent customer processing to keep logs grouped by customer:

```go
type CustomerLogBuffer struct {
    mu           sync.Mutex
    customerCode string
    logs         []string
}
```

**Functions using buffered logging:**
- `processCustomer()` - Concurrent customer import processing
- `ImportAllAWSContactsWithLogger()` - SES import with custom logger

**Important:** When cleaning up logs in these functions:
1. Keep the buffered logging pattern intact
2. Remove emoji from buffered log messages
3. Reduce verbosity but maintain customer grouping
4. Don't break the `logBuffer.Printf()` and `logBuffer.Flush()` pattern

### Other Concurrent Operations

The codebase uses `internal/concurrent` package for multi-customer operations:
- `ProcessCustomersConcurrently()` - Generic concurrent processor
- `AggregateResults()` - Result aggregation
- `DisplaySummary()` - Summary display

These already have good summary logging - keep as-is.

## Files to Modify

### Primary Files
1. **main.go** - Main Lambda handler, SQS processing, concurrent customer processing
2. **internal/lambda/handlers.go** - Request handlers
3. **internal/processors/announcement_processor.go** - Announcement processing
4. **internal/ses/meetings.go** - Meeting operations
5. **internal/ses/operations.go** - SES operations

### Files with Buffered Logging (Special Care)
6. **main.go** - `processCustomer()` function (uses CustomerLogBuffer)
7. **internal/ses/operations.go** - `ImportAllAWSContactsWithLogger()` (uses custom logger)

### Secondary Files (if needed)
- Any other files with excessive logging

## Summary Statistics Design

### Critical Requirement: No Information Loss

For every log statement we delete, we must ensure the information is captured in summary statistics or is truly unnecessary.

### Summary Data Structure

```go
type LambdaExecutionSummary struct {
    ProcessedCount       int
    ErrorCount           int
    EmailsSent           int
    EmailsFiltered       int      // Emails blocked by restricted_recipients
    MeetingsScheduled    int
    MeetingsCancelled    int
    MeetingAttendeesTotal    int  // Total attendees across all meetings
    MeetingAttendeesFiltered int  // Attendees blocked by restricted_recipients
    MeetingManualAttendees   int  // Additional manual attendees added
    CustomersProcessed   []string
    DurationMs           int64
    Errors               []string // Brief error descriptions
}
```

### Key Statistics to Track

**Email Filtering:**
- Total email recipients before filtering
- Emails blocked by `restricted_recipients`
- Emails actually sent
- **Example log:** `logger.Info("emails sent", "sent", 5, "filtered", 15, "total_before_filter", 20)`

**Meeting Attendees:**
- Total attendees from topic subscribers
- Attendees blocked by `restricted_recipients`
- Manual attendees added (from `Attendees` field)
- Final attendee count in meeting
- **Example log:** `logger.Info("meeting scheduled", "meeting_id", id, "attendees_total", 10, "attendees_filtered", 5, "manual_attendees", 2, "final_attendees", 7)`

### Verification Process

For each deleted log:
1. **Identify what information it contained**
2. **Determine if information is needed for troubleshooting**
3. **If needed, add to summary statistics**
4. **If not needed, document why it's safe to delete**
5. **Test that summary provides equivalent troubleshooting capability**

### Example Mapping

| Deleted Log | Summary Field | Verification |
|------------|---------------|--------------|
| `"Processing customer X"` | `CustomersProcessed` | Count matches |
| `"Email sent to N recipients"` | `EmailsSent` | Sum matches |
| `"Filtered out N recipients"` | `EmailsFiltered` | Sum matches |
| `"Meeting scheduled: ID"` | `MeetingsScheduled` | Count matches |
| `"Meeting attendees: N"` | `MeetingAttendeesTotal` | Sum matches |
| `"Filtered out N attendees"` | `MeetingAttendeesFiltered` | Sum matches |
| `"Added manual attendee"` | `MeetingManualAttendees` | Count matches |
| `"Failed to send email"` | `ErrorCount`, `Errors[]` | All errors captured |

## Testing Strategy

### Manual Testing
1. Deploy to non-prod environment
2. Trigger various workflows (change, announcement, meeting)
3. Review CloudWatch logs for:
   - Reduced log volume
   - All errors still logged
   - Critical operations still logged
   - No emoji characters
   - Clean, parseable format
   - **Summary statistics match expected values**

### Validation Checklist
- [ ] Errors are still logged with full context
- [ ] Warnings are still logged
- [ ] Meeting operations are logged
- [ ] Email operations are logged
- [ ] No emoji in logs
- [ ] Log volume reduced by 80%+
- [ ] **Summary statistics are accurate and complete**
- [ ] **Can still troubleshoot issues from logs + summary**
- [ ] **No information loss compared to old logs**

## Migration Path

1. Create feature branch
2. Apply logging changes file by file
3. Test each file's changes
4. Deploy to non-prod
5. Monitor logs for 24 hours
6. Verify no critical information lost
7. Deploy to production

## Rollback Plan

If issues are discovered:
1. Revert commit
2. Redeploy previous version
3. Review what information was needed
4. Adjust logging strategy
5. Re-apply changes

## Success Metrics

- **Log statement count reduced from 518 to ~50-100** (90% reduction)
- **Log volume reduced by 80%+** (measured by CloudWatch log bytes)
- **All errors still captured** (verify with error rate metrics)
- **Critical operations still visible** (meeting scheduled, email sent)
- **Structured logging** (all logs use slog with key-value pairs)
- **No emoji characters** (clean, parseable logs)
- **Improved log readability** (developer feedback)
- **Faster log analysis** (time to find relevant logs)
- **Proper log levels** (Error/Warn/Info/Debug used correctly)
