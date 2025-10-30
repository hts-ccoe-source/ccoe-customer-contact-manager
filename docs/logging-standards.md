# Logging Standards

## Overview

This project uses Go's structured logging package (`log/slog`) for all logging. This document defines standards for what, when, and how to log.

## Quick Reference

| Level | When to Use | Example |
|-------|-------------|---------|
| **Error** | Operation failures, exceptions, errors that need attention | `logger.Error("failed to send email", "error", err, "customer", code)` |
| **Warn** | Unexpected conditions that don't prevent operation | `logger.Warn("no subscribers found", "topic", topicName)` |
| **Info** | Critical operations and summary statistics | `logger.Info("meeting scheduled", "meeting_id", id, "attendees", count)` |
| **Debug** | Detailed troubleshooting (disabled by default) | `logger.Debug("processing step", "step", "validate", "data", data)` |

## Core Principles

### 1. Use Structured Logging

✅ **DO**: Use key-value pairs
```go
logger.Error("failed to process customer", 
    "error", err,
    "customer", customerCode,
    "change_id", changeID)
```

❌ **DON'T**: Use printf-style formatting
```go
logger.Error("failed to process customer %s: %v", customerCode, err)
```

### 2. Log Sparingly

- **Target**: 80%+ reduction from verbose logging
- **Focus**: Errors, warnings, and critical operations only
- **Avoid**: Step-by-step progress logs, success confirmations for routine operations

### 3. Use Summary Statistics

Instead of logging every operation, track metrics in `ExecutionSummary` and log once at the end:

```go
// Don't log each operation
// logger.Info("email sent") ❌

// Instead, track in summary
if summary := SummaryFromContext(ctx); summary != nil {
    summary.RecordEmailSent(recipientCount)
}

// Summary logged once at end of Lambda execution ✅
```

### 4. No Emoji in Logs

❌ **DON'T**: Use emoji characters
```go
log.Printf("✅ Success!")
log.Printf("❌ Failed!")
```

✅ **DO**: Use plain text
```go
logger.Info("operation successful")
logger.Error("operation failed", "error", err)
```

## Log Levels in Detail

### Error Level

**When to use:**
- Operation failures that prevent completion
- Exceptions and errors that need investigation
- Data validation failures
- External service failures (S3, SES, Graph API)

**What to include:**
- Error object: `"error", err`
- Context: customer code, change ID, meeting ID, etc.
- Operation being attempted

**Examples:**
```go
// S3 operation failure
logger.Error("failed to download from S3",
    "error", err,
    "bucket", bucket,
    "key", key)

// Email sending failure
logger.Error("failed to send email",
    "error", err,
    "customer", customerCode,
    "topic", topicName,
    "recipient_count", len(recipients))

// Meeting creation failure
logger.Error("failed to create meeting",
    "error", err,
    "change_id", changeID,
    "customers", customers)
```

### Warn Level

**When to use:**
- Unexpected but non-fatal conditions
- Missing optional data
- Fallback behavior triggered
- Empty result sets that might indicate issues

**What to include:**
- Condition that triggered warning
- Context for investigation
- What action was taken (if any)

**Examples:**
```go
// No subscribers found
logger.Warn("no contacts subscribed to topic",
    "topic", topicName,
    "customer", customerCode)

// Missing optional data
logger.Warn("survey URL not found in metadata",
    "change_id", changeID)

// Fallback behavior
logger.Warn("failed to load from S3, using fallback",
    "error", err,
    "bucket", bucket,
    "key", key)
```

### Info Level

**When to use:**
- Critical operations completed successfully
- Meeting scheduled/cancelled
- Email sent (with recipient count)
- Change request state transitions
- Lambda execution summary

**What to include:**
- Operation completed
- Key identifiers (meeting ID, change ID, customer code)
- Counts (recipients, attendees, messages processed)

**Examples:**
```go
// Meeting scheduled
logger.Info("meeting scheduled",
    "meeting_id", meetingID,
    "change_id", changeID,
    "attendee_count", len(attendees))

// Email sent
logger.Info("email sent",
    "type", "approval_request",
    "customer", customerCode,
    "recipient_count", recipientCount)

// Lambda execution summary (logged once at end)
logger.Info("lambda execution complete",
    "duration_ms", summary.DurationMs(),
    "total_messages", summary.TotalMessages,
    "successful", summary.SuccessfulMessages,
    "customers", summary.CustomersProcessed,
    "emails_sent", summary.EmailsSent,
    "meetings_scheduled", summary.MeetingsScheduled,
    // ... all summary metrics
)
```

### Debug Level

**When to use:**
- Detailed troubleshooting during development
- Step-by-step operation tracing
- Data inspection

**Note**: Debug logs are disabled by default in production. Enable only for troubleshooting.

**Examples:**
```go
logger.Debug("parsing metadata",
    "raw_data", string(data),
    "customer", customerCode)

logger.Debug("filtering recipients",
    "before", len(allRecipients),
    "after", len(filteredRecipients),
    "restricted", restrictedRecipients)
```

## What NOT to Log

### ❌ Don't Log These

1. **Routine success messages**
   ```go
   // DON'T
   logger.Info("customer code extracted successfully")
   logger.Info("S3 event parsed successfully")
   logger.Info("metadata validated successfully")
   ```

2. **Step-by-step progress**
   ```go
   // DON'T
   logger.Info("step 1: loading metadata")
   logger.Info("step 2: validating data")
   logger.Info("step 3: processing customer")
   ```

3. **Verbose operation details**
   ```go
   // DON'T
   logger.Info("found 5 recipients")
   logger.Info("filtering recipients")
   logger.Info("filtered out 2 recipients")
   logger.Info("sending to 3 recipients")
   ```
   
   Instead, use summary statistics:
   ```go
   // DO
   summary.RecordEmailFiltering(5, 3, 2)
   summary.RecordEmailSent(3)
   ```

4. **Sensitive information**
   ```go
   // DON'T
   logger.Info("email addresses", "emails", emailList)
   logger.Info("access token", "token", token)
   ```

5. **Duplicate information**
   ```go
   // DON'T - already tracked in summary
   logger.Info("message processed successfully")
   logger.Info("customer processed successfully")
   ```

## Structured Logging Best Practices

### 1. Use Consistent Field Names

| Data | Field Name | Example |
|------|------------|---------|
| Error | `"error"` | `"error", err` |
| Customer Code | `"customer"` | `"customer", customerCode` |
| Change ID | `"change_id"` | `"change_id", changeID` |
| Meeting ID | `"meeting_id"` | `"meeting_id", meetingID` |
| S3 Bucket | `"bucket"` | `"bucket", bucketName` |
| S3 Key | `"key"` | `"key", objectKey` |
| Topic Name | `"topic"` | `"topic", topicName` |
| Count | `"count"` or specific like `"recipient_count"` | `"recipient_count", len(recipients)` |

### 2. Group Related Fields

```go
// Good - related fields together
logger.Error("failed to update S3 object",
    "error", err,
    "bucket", bucket,
    "key", key,
    "customer", customerCode)
```

### 3. Use Appropriate Types

```go
// Good - use native types
logger.Info("meeting scheduled",
    "meeting_id", meetingID,           // string
    "attendee_count", len(attendees),  // int
    "start_time", startTime,           // time.Time
    "customers", customerCodes)        // []string

// Avoid string conversion when not needed
logger.Info("count", "value", strconv.Itoa(count)) // ❌
logger.Info("count", "value", count)                // ✅
```

### 4. Include Context for Troubleshooting

Always include enough context to understand what failed:

```go
// Insufficient context ❌
logger.Error("operation failed", "error", err)

// Good context ✅
logger.Error("failed to send approval request email",
    "error", err,
    "customer", customerCode,
    "change_id", changeID,
    "topic", topicName)
```

## Passing Logger Through Functions

### Standard Pattern

```go
// Function signature includes logger
func ProcessCustomer(ctx context.Context, customerCode string, logger *slog.Logger) error {
    logger.Info("processing customer", "customer", customerCode)
    
    // Pass logger to called functions
    err := SendEmail(ctx, customerCode, logger)
    if err != nil {
        logger.Error("failed to send email", "error", err, "customer", customerCode)
        return err
    }
    
    return nil
}
```

### Using slog.Default()

When logger is not available in context:

```go
func HelperFunction() error {
    logger := slog.Default()
    logger.Info("operation complete")
    return nil
}
```

## ExecutionSummary Pattern

### Overview

Instead of logging every operation, track metrics in `ExecutionSummary` and log once at the end of Lambda execution.

### Structure

```go
type ExecutionSummary struct {
    // Timing
    StartTime time.Time
    EndTime   time.Time
    
    // Message Processing
    TotalMessages      int
    SuccessfulMessages int
    RetryableErrors    int
    NonRetryableErrors int
    DiscardedEvents    int
    
    // Customer Processing
    CustomersProcessed []string
    
    // Email Statistics
    EmailsSent         int
    EmailsFiltered     int
    EmailsBeforeFilter int
    EmailErrors        int
    
    // Meeting Statistics
    MeetingsScheduled  int
    MeetingsCancelled  int
    MeetingsUpdated    int
    MeetingErrors      int
    TotalAttendees     int
    FilteredAttendees  int
    ManualAttendees    int
    FinalAttendeeCount int
    
    // S3 Operations
    S3Downloads int
    S3Uploads   int
    S3Deletes   int
    S3Errors    int
    
    // Change Request Processing
    ApprovalRequests int
    ApprovedChanges  int
    CompletedChanges int
    CancelledChanges int
    
    // Error Details
    ErrorMessages []string
}
```

### Using ExecutionSummary

#### 1. Initialize in Handler

```go
func Handler(ctx context.Context, sqsEvent events.SQSEvent) error {
    logger := slog.Default()
    
    // Initialize summary
    summary := NewExecutionSummary()
    summary.TotalMessages = len(sqsEvent.Records)
    
    // Store in context
    ctx = ContextWithSummary(ctx, summary)
    
    // ... process messages ...
    
    // Finalize and log summary
    summary.Finalize()
    logger.Info("lambda execution complete",
        "duration_ms", summary.DurationMs(),
        "total_messages", summary.TotalMessages,
        "successful", summary.SuccessfulMessages,
        // ... all metrics ...
    )
    
    return nil
}
```

#### 2. Track Operations in Functions

```go
func SendEmail(ctx context.Context, recipients []string) error {
    // ... send email ...
    
    // Track in summary
    if summary := SummaryFromContext(ctx); summary != nil {
        summary.RecordEmailSent(len(recipients))
    }
    
    return nil
}

func ScheduleMeeting(ctx context.Context, attendees []string) error {
    // ... create meeting ...
    
    // Track in summary
    if summary := SummaryFromContext(ctx); summary != nil {
        summary.RecordMeetingScheduled(len(attendees))
    }
    
    return nil
}
```

#### 3. Track Errors

```go
func ProcessMessage(ctx context.Context, msg string) error {
    err := doSomething()
    if err != nil {
        // Track error in summary
        if summary := SummaryFromContext(ctx); summary != nil {
            if isRetryable(err) {
                summary.RecordRetryableError(err)
            } else {
                summary.RecordNonRetryableError(err)
            }
        }
        return err
    }
    
    // Track success
    if summary := SummaryFromContext(ctx); summary != nil {
        summary.RecordSuccess()
    }
    
    return nil
}
```

### Adding New Metrics

When you need to track a new operation:

1. **Add field to ExecutionSummary struct**
   ```go
   type ExecutionSummary struct {
       // ... existing fields ...
       NewOperationCount int
   }
   ```

2. **Add tracking method**
   ```go
   func (s *ExecutionSummary) RecordNewOperation() {
       s.NewOperationCount++
   }
   ```

3. **Update summary logging**
   ```go
   logger.Info("lambda execution complete",
       // ... existing fields ...
       "new_operation_count", summary.NewOperationCount)
   ```

4. **Update summary-metrics-mapping.md**
   - Document what logs this metric replaces
   - Document where it should be tracked

5. **Call tracking method in code**
   ```go
   if summary := SummaryFromContext(ctx); summary != nil {
       summary.RecordNewOperation()
   }
   ```

## CustomerLogBuffer Pattern

### When to Use

Use `CustomerLogBuffer` for concurrent operations that process multiple customers. This groups logs by customer for better readability.

### Pattern

```go
func ProcessCustomers(customers []string, logger *slog.Logger) {
    var wg sync.WaitGroup
    
    for _, customer := range customers {
        wg.Add(1)
        go func(code string) {
            defer wg.Done()
            
            // Create buffer for this customer's logs
            logBuffer := &CustomerLogBuffer{}
            
            // Log to buffer instead of directly
            logBuffer.Printf("Processing customer: %s", code)
            
            err := doWork(code)
            if err != nil {
                logBuffer.Printf("Error: %v", err)
            } else {
                logBuffer.Printf("Success")
            }
            
            // Flush all logs for this customer at once
            logBuffer.Flush(logger)
        }(customer)
    }
    
    wg.Wait()
}
```

### Rules for CustomerLogBuffer

1. **Use for concurrent operations only** - Don't use for sequential processing
2. **Keep logs minimal** - Even buffered logs should be sparse
3. **No emoji** - Remove emoji from buffered messages
4. **Flush at end** - Always call `Flush()` when done
5. **Include customer context** - Make it clear which customer the logs belong to

## Troubleshooting Guide

### Finding Errors

**CloudWatch Insights Query:**
```
fields @timestamp, @message, error, customer, change_id
| filter level = "ERROR"
| sort @timestamp desc
| limit 100
```

### Analyzing Summary Statistics

**Find Lambda executions with errors:**
```
fields @timestamp, total_messages, successful, retryable_errors, non_retryable_errors
| filter @message like /lambda execution complete/
| filter retryable_errors > 0 or non_retryable_errors > 0
| sort @timestamp desc
```

**Find executions with high email filtering:**
```
fields @timestamp, emails_before_filter, emails_filtered, emails_sent
| filter @message like /lambda execution complete/
| filter emails_filtered > 10
| sort @timestamp desc
```

**Find executions with meeting errors:**
```
fields @timestamp, meetings_scheduled, meeting_errors
| filter @message like /lambda execution complete/
| filter meeting_errors > 0
| sort @timestamp desc
```

### Common Issues

#### Issue: No logs appearing

**Check:**
1. Log level set correctly? (should be Info or Debug)
2. Logger initialized? (`slog.SetDefault(logger)`)
3. CloudWatch log group exists?

#### Issue: Summary metrics don't match expectations

**Check:**
1. All operations calling `Record*()` methods?
2. Summary passed through context correctly?
3. Check `summary-metrics-mapping.md` for correct tracking locations

#### Issue: Logs too verbose

**Check:**
1. Are you logging routine success messages? (remove them)
2. Are you logging step-by-step progress? (remove them)
3. Should this be tracked in summary instead?

## Migration Checklist

When adding new code or modifying existing code:

- [ ] Use `logger *slog.Logger` parameter, not `log.Printf()`
- [ ] Use structured logging with key-value pairs
- [ ] Log errors with context (customer, change_id, etc.)
- [ ] Track operations in ExecutionSummary instead of logging
- [ ] No emoji characters in log messages
- [ ] No routine success messages
- [ ] No step-by-step progress logs
- [ ] Include enough context for troubleshooting
- [ ] Use consistent field names
- [ ] Pass logger through function calls
- [ ] Update summary-metrics-mapping.md if adding new metrics

## References

- **Go slog documentation**: https://pkg.go.dev/log/slog
- **Summary metrics mapping**: `.kiro/specs/reduce-backend-logging/summary-metrics-mapping.md`
- **Summary verification report**: `.kiro/specs/reduce-backend-logging/summary-verification-report.md`
- **ExecutionSummary implementation**: `internal/lambda/summary.go`
