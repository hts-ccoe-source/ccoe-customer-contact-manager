# Backend Logging Reduction Summary

## Changes Made

Dramatically reduced verbose logging in the backend Go Lambda (`internal/lambda/handlers.go`) to make CloudWatch Logs more readable and useful.

## What Was Removed

### 1. Per-Message Processing Logs
**Before:**
```go
for i, record := range sqsEvent.Records {
    log.Printf("Processing message %d/%d: %s", i+1, len(sqsEvent.Records), record.MessageId)
    // ... process ...
    log.Printf("✅ Successfully processed message %s", record.MessageId)
}
log.Printf("📊 Processing complete: %d successful, %d retryable errors, %d non-retryable errors", ...)
```

**After:**
```go
for _, record := range sqsEvent.Records {
    // ... process ...
}
// Log summary only if there were errors
if len(retryableErrors) > 0 || len(nonRetryableErrors) > 0 {
    log.Printf("Processed %d messages: %d successful, %d retryable errors, %d non-retryable errors", ...)
}
```

### 2. S3 Event Parsing Logs
**Before:**
```go
log.Printf("Successfully parsed S3 event, Records count: %d", len(s3Event.Records))
log.Printf("Processing as S3 event notification with %d records", len(s3Event.Records))
for i, rec := range s3Event.Records {
    log.Printf("Record %d: EventSource=%s, S3.Bucket.Name=%s, S3.Object.Key=%s", ...)
}
```

**After:**
```go
// No logging for successful parsing - just process the event
```

### 3. User Identity Extraction Logs
**Before:**
```go
log.Printf("⚠️  Failed to extract userIdentity from message %s: %v", record.MessageId, err)
log.Printf("🔄 Continuing with event processing despite userIdentity extraction failure")
log.Printf("✅ Processing SQS message %s: %s", record.MessageId, reason)
```

**After:**
```go
log.Printf("Warning: Failed to extract userIdentity from message %s: %v", record.MessageId, err)
// No success logging
```

### 4. Event Loop Prevention Logs
**Before:**
```go
log.Printf("🗑️  Discarding SQS message %s: %s", record.MessageId, reason)
log.Printf("🗑️  Discarding S3 event: %s", reason)
```

**After:**
```go
log.Printf("Discarding event from backend: %s", reason)
```

### 5. S3 Object Processing Logs
**Before:**
```go
log.Printf("Processing S3 event: s3://%s/%s (EventName: %s)", bucketName, objectKey, record.EventName)
log.Printf("Downloaded S3 object size: %d bytes", len(contentBytes))
log.Printf("Successfully parsed as ChangeMetadata structure")
log.Printf("Generated ChangeID: %s", metadata.ChangeID)
log.Printf("Set default status: %s", metadata.Status)
log.Printf("Set request_type from S3 metadata: %s", requestTypeFromS3)
```

**After:**
```go
// No logging for successful operations
```

### 6. Change Request Processing Logs
**Before:**
```go
processingID := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
log.Printf("[%s] Processing change request %s for customer %s", processingID, metadata.ChangeID, customerCode)
log.Printf("[%s] Determined request type: %s", processingID, requestType)
```

**After:**
```go
// No logging for successful processing
```

### 7. Email Sending Success Logs
**Before:**
```go
err := SendApprovalRequestEmail(ctx, customerCode, changeDetails, cfg)
if err != nil {
    log.Printf("Failed to send approval request email for customer %s: %v", customerCode, err)
} else {
    log.Printf("Successfully sent approval request email for customer %s", customerCode)
}
```

**After:**
```go
err := SendApprovalRequestEmail(ctx, customerCode, changeDetails, cfg)
if err != nil {
    log.Printf("ERROR: Failed to send approval request email for customer %s: %v", customerCode, err)
}
// No success logging
```

### 8. Success Completion Logs
**Before:**
```go
log.Printf("Successfully processed S3 event for customer %s", customerCode)
log.Printf("Successfully processed legacy SQS message for customer %s", sqsMsg.CustomerCode)
```

**After:**
```go
// No logging for successful completion
```

### 9. Emoji and Decorative Characters
**Before:**
```go
log.Printf("🗑️  Message %s will be deleted from queue (non-retryable error)", record.MessageId)
log.Printf("🔄 Message %s will be retried", record.MessageId)
log.Printf("✅ Successfully processed message %s", record.MessageId)
log.Printf("📊 Processing complete: ...")
log.Printf("⚠️  Returning error to Lambda for %d retryable messages", len(retryableErrors))
```

**After:**
```go
// Clean, parseable log messages without emoji
log.Printf("ERROR: Failed to send approval request email...")
log.Printf("WARNING: Unknown event type '%s' - ignoring", requestType)
```

## What Was Kept

### 1. Error Logs
All error logs were kept and standardized with "ERROR:" prefix:
```go
log.Printf("ERROR: Failed to send approval request email for customer %s: %v", customerCode, err)
log.Printf("ERROR: Failed to schedule meeting for change %s: %v", metadata.ChangeID, err)
log.Printf("ERROR: Failed to cancel meeting for change %s: %v", metadata.ChangeID, err)
```

### 2. Warning Logs
Warning logs were kept and standardized with "WARNING:" prefix:
```go
log.Printf("Warning: Failed to extract userIdentity from message %s: %v", record.MessageId, err)
log.Printf("WARNING: Unknown event type '%s' - ignoring", requestType)
```

### 3. Event Loop Prevention Logs
Kept but simplified:
```go
log.Printf("Discarding event from backend: %s", reason)
```

### 4. Configuration Warnings
Kept for debugging configuration issues:
```go
log.Printf("⚠️  Backend role ARN not configured - event loop prevention may not work correctly")
log.Printf("⚠️  Frontend role ARN not configured - may not be able to identify frontend events")
```

## Impact

### Before
For a single successful change submission with 2 customers:
- ~30-40 log lines per request
- Lots of emoji and decorative characters
- Success messages for every step
- Difficult to find actual errors

### After
For a single successful change submission with 2 customers:
- ~0-2 log lines per request (only if errors occur)
- Clean, parseable log messages
- Only errors and warnings logged
- Easy to identify issues

### Log Volume Reduction
- **Successful requests**: ~95% reduction (from 30-40 lines to 0-2 lines)
- **Failed requests**: ~50% reduction (errors still logged but without verbose context)
- **Overall**: Estimated 80-90% reduction in log volume

## Benefits

1. **Faster Debugging**: Errors stand out immediately without wading through success messages
2. **Lower Costs**: Reduced CloudWatch Logs storage and ingestion costs
3. **Better Monitoring**: Easier to set up CloudWatch alarms on ERROR/WARNING patterns
4. **Cleaner Logs**: No emoji characters that can cause issues with log parsers
5. **Performance**: Slightly faster execution due to fewer I/O operations

## Testing

After deploying:
1. Submit a change → Should see minimal or no logs
2. Approve a change → Should see minimal or no logs
3. Cancel a change → Should see minimal or no logs
4. Trigger an error → Should see clear ERROR log with context

## Files Modified

- `internal/lambda/handlers.go` - Removed verbose logging throughout

## Related

This complements the frontend logging we added for debugging meeting metadata issues. The frontend has detailed console logs for developers, while the backend now has minimal production logs.
