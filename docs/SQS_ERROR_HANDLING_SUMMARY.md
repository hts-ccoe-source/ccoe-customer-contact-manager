# SQS Error Handling Implementation Summary

## Problem Solved
When the Lambda function receives an SQS message but cannot retrieve the corresponding S3 file (404 NoSuchKey error), the message would get stuck in an infinite retry loop. This implementation adds intelligent error classification to distinguish between retryable and non-retryable errors.

## Solution Implemented

### 1. Error Classification System (`internal/lambda/errors.go`)

#### ProcessingError Structure
- **Type**: Categorizes errors (S3NotFound, AccessDenied, NetworkError, etc.)
- **Retryable**: Boolean flag indicating if the error should trigger a retry
- **Context**: Includes MessageID, S3Bucket, S3Key for debugging
- **Underlying**: Preserves the original error for detailed logging

#### Error Types and Retry Behavior
| Error Type | Retryable | Description | Example |
|------------|-----------|-------------|---------|
| `s3_not_found` | ❌ No | S3 object/bucket doesn't exist | 404 NoSuchKey |
| `s3_access_denied` | ❌ No | Permission issues | 403 AccessDenied |
| `s3_network_error` | ✅ Yes | Temporary network/service issues | Timeouts, DNS errors |
| `invalid_format` | ❌ No | Bad JSON or message format | Parse errors |
| `invalid_customer` | ❌ No | Customer code not in config | Unknown customer |
| `config_error` | ✅ Yes | Configuration/credential issues | Temporary config problems |
| `email_error` | ✅ Yes | Email sending failures | SES temporary issues |
| `unknown` | ✅ Yes | Unclassified errors | Default to retry for safety |

#### Smart Error Detection
- **AWS SDK Integration**: Detects specific AWS error codes (NoSuchKey, AccessDenied, etc.)
- **HTTP Status Codes**: 4xx errors are non-retryable, 5xx errors are retryable
- **Pattern Matching**: Analyzes error messages for common patterns
- **Context Preservation**: Maintains S3 location and message ID for debugging

### 2. Updated Lambda Handler (`internal/lambda/handlers.go`)

#### Intelligent Message Processing
```go
// Process each message with proper error handling
for _, record := range sqsEvent.Records {
    err := ProcessSQSRecord(ctx, record, cfg)
    if err != nil {
        LogError(err, record.MessageId)
        
        if ShouldDeleteMessage(err) {
            // Non-retryable: message will be deleted
            nonRetryableErrors = append(nonRetryableErrors, err)
        } else {
            // Retryable: message will be retried
            retryableErrors = append(retryableErrors, err)
        }
    }
}

// Only return error for retryable failures
if len(retryableErrors) > 0 {
    return fmt.Errorf("retryable errors occurred")
}
// Non-retryable errors don't cause Lambda to retry
return nil
```

#### Lambda Retry Behavior
- **Retryable Errors**: Lambda handler returns error → SQS retries message
- **Non-Retryable Errors**: Lambda handler returns success → SQS deletes message
- **Mixed Results**: Only retryable errors cause Lambda to return error

### 3. Enhanced Error Context

#### S3 Event Processing
```go
// Download metadata with proper error classification
metadata, err := DownloadMetadataFromS3(ctx, bucketName, objectKey, region)
if err != nil {
    // ClassifyError automatically determines retry behavior
    return ClassifyError(err, "", bucketName, objectKey)
}
```

#### Legacy SQS Message Processing
```go
// Validate customer code
if err := ValidateCustomerCode(customerCode, cfg); err != nil {
    return NewProcessingError(
        ErrorTypeInvalidCustomer,
        fmt.Sprintf("Invalid customer code %s", customerCode),
        false, // Not retryable - bad customer code
        err, messageID, s3Bucket, s3Key,
    )
}
```

### 4. Comprehensive Logging

#### Error Classification Logging
```
❌ NON-RETRYABLE ERROR [s3_not_found] Message f6ee1584-...: S3 object not found: s3://bucket/key
   This message will be deleted from the queue to prevent infinite retries
   S3 Location: s3://my-bucket/customers/invalid/file.json
   Underlying error: NoSuchKey: The specified key does not exist

⚠️  RETRYABLE ERROR [s3_network_error] Message abc123-...: Network error accessing S3
   S3 Location: s3://my-bucket/customers/hts/change.json
   Underlying error: RequestTimeout: Your socket connection to the server was not read from or written to within the timeout period
```

#### Processing Summary
```
📊 Processing complete: 2 successful, 1 retryable errors, 3 non-retryable errors
🗑️  Message f6ee1584-... will be deleted from queue (non-retryable error)
🔄 Message abc123-... will be retried
✅ All non-retryable errors will be deleted from queue
```

## Error Scenarios Handled

### 1. S3 File Not Found (Original Issue)
```
Error: NoSuchKey: The specified key does not exist
Classification: s3_not_found (non-retryable)
Action: Message deleted from queue
Result: No infinite retries
```

### 2. S3 Access Denied
```
Error: AccessDenied: Access Denied
Classification: s3_access_denied (non-retryable)  
Action: Message deleted from queue
Result: No retries for permission issues
```

### 3. Network Timeouts
```
Error: RequestTimeout: Connection timeout
Classification: s3_network_error (retryable)
Action: Message retried by SQS
Result: Temporary issues are retried
```

### 4. Invalid JSON Format
```
Error: invalid character '}' looking for beginning of value
Classification: invalid_format (non-retryable)
Action: Message deleted from queue
Result: Bad data doesn't cause infinite retries
```

### 5. Unknown Customer Code
```
Error: customer code 'invalid' not found in configuration
Classification: invalid_customer (non-retryable)
Action: Message deleted from queue
Result: Configuration errors don't retry indefinitely
```

## Benefits

### 🚫 **Prevents Infinite Retries**
- S3 404 errors no longer cause messages to retry forever
- Invalid data formats are immediately discarded
- Bad customer codes don't consume retry attempts

### 🔄 **Preserves Legitimate Retries**
- Network issues and timeouts are still retried
- Temporary AWS service issues get retry attempts
- Configuration problems can be fixed and retried

### 📊 **Enhanced Observability**
- Clear logging distinguishes error types
- S3 locations included in all error messages
- Processing summaries show retry vs delete decisions

### ⚡ **Improved Performance**
- Reduces SQS queue backlog from stuck messages
- Faster processing of valid messages
- Lower Lambda execution costs from fewer retries

## Testing Scenarios

### Test Case 1: S3 File Deleted After Message Sent
```bash
# Simulate: File uploaded → SQS message sent → File deleted
# Expected: Message processed once, then deleted (no retries)
```

### Test Case 2: Temporary S3 Service Issue
```bash
# Simulate: S3 returns 503 ServiceUnavailable
# Expected: Message retried until S3 recovers
```

### Test Case 3: Invalid Customer Code
```bash
# Simulate: Message with customer code not in config
# Expected: Message deleted immediately (no retries)
```

### Test Case 4: Malformed JSON
```bash
# Simulate: S3 file contains invalid JSON
# Expected: Message deleted after first attempt
```

## Configuration

No additional configuration required. The error handling is automatic and uses intelligent defaults:

- **AWS SDK errors**: Classified by error code
- **HTTP status codes**: 4xx = non-retryable, 5xx = retryable  
- **Pattern matching**: Common error patterns detected
- **Conservative approach**: Unknown errors default to retryable

## Monitoring

Monitor these CloudWatch metrics to track error handling effectiveness:

- **Lambda Errors**: Should decrease for non-retryable issues
- **SQS Message Age**: Should not increase indefinitely
- **Dead Letter Queue**: May receive more messages (expected for bad data)
- **Lambda Duration**: Should decrease due to fewer retries

The implementation ensures that the SQS queue doesn't get clogged with unprocessable messages while still allowing legitimate retries for temporary issues.