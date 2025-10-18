# Archive Update API Reference

## Overview

This document describes the API for updating archive objects with processing metadata and meeting information in the multi-customer email distribution system.

## Core Concepts

### Modification Entries

Modification entries track the history of changes to a change request. Each entry includes:
- `timestamp`: When the modification occurred
- `user_id`: Who made the modification (backend role ARN or user ID)
- `modification_type`: Type of modification (processed, meeting_scheduled, etc.)
- `customer_code`: (Optional) Which customer processed the change
- `meeting_metadata`: (Optional) Meeting details for meeting_scheduled type

### Storage Locations

- **Archive**: `archive/{changeId}.json` - Single source of truth, permanent storage
- **Triggers**: `customers/{code}/{changeId}.json` - Transient trigger files, deleted after processing

### Meeting Metadata Storage

Meeting metadata is stored at TWO levels:
1. **Root level** (for easy access): `meeting_id`, `join_url`
2. **Modification entry** (for audit trail): Full `MeetingMetadata` object

## API Functions

### 1. UpdateArchiveWithProcessingMetadata

Updates the archive with a "processed" modification entry after successful email delivery.

**Function Signature:**
```go
func (s *S3UpdateManager) UpdateArchiveWithProcessingMetadata(
    ctx context.Context,
    bucket string,
    key string,
    customerCode string,
) error
```

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `bucket`: S3 bucket name (e.g., "metadata-bucket")
- `key`: S3 object key (e.g., "archive/CHG-12345.json")
- `customerCode`: Customer code that processed the change (e.g., "customer-a")

**Returns:**
- `error`: nil on success, error on failure

**Example Usage:**
```go
s3Manager := NewS3UpdateManager("us-east-1")

err := s3Manager.UpdateArchiveWithProcessingMetadata(
    context.Background(),
    "4cm-prod-ccoe-change-management-metadata",
    "archive/CHG-12345.json",
    "customer-a",
)
if err != nil {
    log.Printf("Failed to update archive: %v", err)
    return err
}
```

**What It Does:**
1. Loads the change object from S3 with ETag
2. Creates a "processed" modification entry with customer code
3. Adds the entry to the modifications array
4. Saves the updated object back to S3 with optimistic locking
5. Retries up to 3 times on ETag mismatch (concurrent modification)

**Error Handling:**
- Returns error if context is cancelled
- Returns error if bucket or key is empty
- Returns error if customer code is empty
- Returns error if S3 operations fail
- Retries automatically on ETag mismatch

---

### 2. UpdateArchiveWithMeetingAndProcessing

Updates the archive with both meeting metadata and processing status in a single atomic operation.

**Function Signature:**
```go
func (s *S3UpdateManager) UpdateArchiveWithMeetingAndProcessing(
    ctx context.Context,
    bucket string,
    key string,
    customerCode string,
    meetingMetadata *types.MeetingMetadata,
) error
```

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `bucket`: S3 bucket name
- `key`: S3 object key (archive path)
- `customerCode`: Customer code that processed the change
- `meetingMetadata`: Meeting details from Microsoft Graph API

**Returns:**
- `error`: nil on success, error on failure

**Example Usage:**
```go
meetingMetadata := &types.MeetingMetadata{
    MeetingID: "AAMkAGI1...meeting-id",
    JoinURL:   "https://teams.microsoft.com/l/meetup/...",
    StartTime: "2025-10-20T10:00:00Z",
    EndTime:   "2025-10-20T11:00:00Z",
    Subject:   "Change Implementation Meeting",
    Organizer: "ccoe@hearst.com",
    Attendees: []string{"user1@hearst.com", "user2@hearst.com"},
}

err := s3Manager.UpdateArchiveWithMeetingAndProcessing(
    context.Background(),
    "4cm-prod-ccoe-change-management-metadata",
    "archive/CHG-12345.json",
    "customer-a",
    meetingMetadata,
)
if err != nil {
    log.Printf("Failed to update archive with meeting: %v", err)
    return err
}
```

**What It Does:**
1. Validates meeting metadata structure
2. Loads the change object from S3 with ETag
3. Creates a "meeting_scheduled" modification entry
4. Sets root-level meeting fields (meeting_id, join_url)
5. Creates a "processed" modification entry
6. Saves the updated object back to S3 with optimistic locking
7. Retries up to 3 times on ETag mismatch

**Error Handling:**
- Returns error if meeting metadata is nil or invalid
- Returns error if customer code is empty
- Validates meeting metadata format (ISO 8601 timestamps)
- Retries automatically on concurrent modifications

---

### 3. UpdateChangeObjectWithMeetingMetadata

Updates the archive with meeting metadata only (no processing entry).

**Function Signature:**
```go
func (s *S3UpdateManager) UpdateChangeObjectWithMeetingMetadata(
    ctx context.Context,
    bucket string,
    key string,
    meetingMetadata *types.MeetingMetadata,
) error
```

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `bucket`: S3 bucket name
- `key`: S3 object key
- `meetingMetadata`: Meeting details

**Returns:**
- `error`: nil on success, error on failure

**Example Usage:**
```go
meetingMetadata := &types.MeetingMetadata{
    MeetingID: "meeting-123",
    JoinURL:   "https://teams.microsoft.com/...",
    StartTime: "2025-10-20T10:00:00Z",
    EndTime:   "2025-10-20T11:00:00Z",
    Subject:   "Implementation Meeting",
}

err := s3Manager.UpdateChangeObjectWithMeetingMetadata(
    context.Background(),
    "metadata-bucket",
    "archive/CHG-12345.json",
    meetingMetadata,
)
```

---

## ModificationManager API

### CreateProcessedEntry

Creates a "processed" modification entry with customer tracking.

**Function Signature:**
```go
func (m *ModificationManager) CreateProcessedEntry(customerCode string) (types.ModificationEntry, error)
```

**Example Usage:**
```go
modManager := NewModificationManager()

entry, err := modManager.CreateProcessedEntry("customer-a")
if err != nil {
    log.Printf("Failed to create processed entry: %v", err)
    return err
}

// Entry structure:
// {
//   "timestamp": "2025-10-17T22:00:00Z",
//   "user_id": "arn:aws:iam::123456789012:role/backend-lambda-role",
//   "modification_type": "processed",
//   "customer_code": "customer-a"
// }
```

---

### AddProcessedToChange

Adds a "processed" modification entry to change metadata.

**Function Signature:**
```go
func (m *ModificationManager) AddProcessedToChange(
    changeMetadata *types.ChangeMetadata,
    customerCode string,
) error
```

**Example Usage:**
```go
modManager := NewModificationManager()

err := modManager.AddProcessedToChange(changeMetadata, "customer-a")
if err != nil {
    log.Printf("Failed to add processed entry: %v", err)
    return err
}
```

---

## Integration with SQS Processing

### Typical Workflow

```go
func ProcessSQSMessage(event S3Event) error {
    ctx := context.Background()
    
    // Extract customer code and change ID from S3 key
    customerCode := extractCustomerCode(event.S3.Object.Key)
    changeId := extractChangeId(event.S3.Object.Key)
    
    // Step 1: Check if trigger still exists (idempotency)
    triggerKey := fmt.Sprintf("customers/%s/%s.json", customerCode, changeId)
    exists, err := s3Client.HeadObject(triggerKey)
    if !exists {
        log.Info("Trigger already processed, skipping")
        return nil
    }
    
    // Step 2: Load from archive (single source of truth)
    archiveKey := fmt.Sprintf("archive/%s.json", changeId)
    changeData, err := s3Client.GetObject(archiveKey)
    if err != nil {
        return fmt.Errorf("failed to load from archive: %w", err)
    }
    
    // Step 3: Process the change (send emails, schedule meetings)
    results, err := processChange(changeData, customerCode)
    if err != nil {
        return fmt.Errorf("failed to process change: %w", err)
    }
    
    // Step 4: Update archive with processing metadata
    s3Manager := NewS3UpdateManager("us-east-1")
    
    if results.MeetingCreated {
        // Update with both meeting and processing metadata
        err = s3Manager.UpdateArchiveWithMeetingAndProcessing(
            ctx,
            bucketName,
            archiveKey,
            customerCode,
            results.MeetingMetadata,
        )
    } else {
        // Update with processing metadata only
        err = s3Manager.UpdateArchiveWithProcessingMetadata(
            ctx,
            bucketName,
            archiveKey,
            customerCode,
        )
    }
    
    if err != nil {
        // CRITICAL: Delete trigger but do NOT acknowledge SQS message
        _ = s3Client.DeleteObject(triggerKey)
        return fmt.Errorf("failed to update archive: %w", err)
    }
    
    // Step 5: Delete trigger (cleanup)
    err = s3Client.DeleteObject(triggerKey)
    if err != nil {
        log.Warn("Failed to delete trigger, but processing complete")
    }
    
    // Step 6: Acknowledge SQS message (only after successful archive update)
    return nil
}
```

---

## Best Practices

### 1. Always Update Archive Before Deleting Trigger

```go
// CORRECT: Update archive first, then delete trigger
err := s3Manager.UpdateArchiveWithProcessingMetadata(ctx, bucket, archiveKey, customerCode)
if err != nil {
    // Delete trigger but don't ack SQS - allows retry
    _ = s3Client.DeleteObject(triggerKey)
    return err
}
_ = s3Client.DeleteObject(triggerKey)
```

### 2. Use Context with Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := s3Manager.UpdateArchiveWithProcessingMetadata(ctx, bucket, key, customerCode)
```

### 3. Handle Concurrent Modifications

The functions automatically retry on ETag mismatch (up to 3 times). No additional handling needed:

```go
// Automatic retry on concurrent modification
err := s3Manager.UpdateArchiveWithProcessingMetadata(ctx, bucket, key, customerCode)
// Will retry automatically if another process modified the object
```

### 4. Validate Meeting Metadata

```go
if err := meetingMetadata.ValidateMeetingMetadata(); err != nil {
    return fmt.Errorf("invalid meeting metadata: %w", err)
}

err := s3Manager.UpdateArchiveWithMeetingAndProcessing(ctx, bucket, key, customerCode, meetingMetadata)
```

### 5. Track Multiple Customer Processing

```go
// Each customer processes independently
for _, customerCode := range affectedCustomers {
    err := s3Manager.UpdateArchiveWithProcessingMetadata(
        ctx,
        bucket,
        archiveKey,
        customerCode,
    )
    if err != nil {
        log.Printf("Failed to update for %s: %v", customerCode, err)
        // Continue processing other customers
    }
}
```

---

## Error Handling

### Common Errors

1. **Context Cancelled**: Operation timed out or was cancelled
   ```go
   if ctx.Err() != nil {
       return fmt.Errorf("context cancelled: %w", ctx.Err())
   }
   ```

2. **ETag Mismatch**: Concurrent modification detected (automatically retried)
   ```go
   // Handled automatically by the functions
   // No additional code needed
   ```

3. **S3 Access Denied**: Insufficient permissions
   ```go
   // Check IAM role has s3:GetObject and s3:PutObject permissions
   ```

4. **Invalid Meeting Metadata**: Validation failed
   ```go
   if err := meetingMetadata.ValidateMeetingMetadata(); err != nil {
       return fmt.Errorf("invalid meeting metadata: %w", err)
   }
   ```

---

## Monitoring and Logging

All functions provide comprehensive logging:

```
📝 Updating archive with processing metadata: customer=customer-a, key=archive/CHG-12345.json
🔄 Updating change object with modification entry (optimistic locking): type=processed
📋 Loaded change object: CHG-12345 (ETag: "abc123", modifications: 2)
📝 Added modification entry to change CHG-12345 (total entries: 3)
✅ Successfully updated archive with processing metadata for customer customer-a
```

Monitor these logs for:
- Processing success/failure rates
- ETag mismatch frequency (indicates high concurrency)
- Archive update latency
- Customer-specific processing issues

---

## Testing

See `internal/lambda/archive_update_test.go` for comprehensive test examples covering:
- Single customer processing
- Multiple customer processing
- Meeting metadata handling
- Validation scenarios
- Error cases

Run tests:
```bash
go test ./internal/lambda -run "Archive|Processed|Modification" -v
```
