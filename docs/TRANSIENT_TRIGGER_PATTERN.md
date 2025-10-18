# Transient Trigger Pattern Architecture

## Overview

The Transient Trigger Pattern is a storage and processing architecture that separates operational triggers from permanent data storage. This pattern ensures a single source of truth while enabling efficient, idempotent event processing across multiple customer organizations.

## Core Concepts

### Single Source of Truth

The `archive/` prefix in S3 serves as the **only authoritative location** for change data:

- All backend processing loads data from `archive/{changeId}.json`
- All updates (meeting metadata, processing results) are written to `archive/`
- Frontend and backend never read from `customers/` prefix
- No data synchronization issues between multiple copies

### Transient Triggers

The `customers/{customer-code}/` prefix contains **temporary trigger files** that:

- Exist only to trigger S3 event notifications → SQS messages
- Are deleted immediately after successful backend processing
- Never serve as a data source (backend loads from `archive/`)
- Provide natural idempotency through existence checks

### Storage Architecture

```
s3://metadata-bucket/
├── archive/                    # Single source of truth (permanent)
│   └── {changeId}.json        # Authoritative change data
│
├── customers/                  # Transient triggers (temporary)
│   ├── hts/
│   │   └── {changeId}.json    # Deleted after processing
│   ├── htsnonprod/
│   │   └── {changeId}.json    # Deleted after processing
│   └── {customer-code}/
│       └── {changeId}.json    # Deleted after processing
│
└── drafts/                     # Draft storage (optional)
    └── {changeId}.json        # Working copies
```

## Processing Flow

### Frontend Upload Sequence

When a change is submitted (not saved as draft):

```
1. Upload to archive/{changeId}.json
   ↓
2. For each selected customer:
   Upload to customers/{customer-code}/{changeId}.json
   ↓
3. S3 event notifications trigger customer-specific SQS queues
   ↓
4. Remove from drafts/{changeId}.json (if exists)
```

**Critical**: Archive upload MUST succeed before creating any customer triggers.

### Backend Processing Sequence

When a backend process receives an SQS event:

```
1. Extract customer code and changeId from S3 event
   ↓
2. Idempotency check: Does customers/{customer-code}/{changeId}.json exist?
   - If NO: Skip processing (already handled)
   - If YES: Continue
   ↓
3. Load authoritative data from archive/{changeId}.json
   ↓
4. Process change (send emails, schedule meetings, etc.)
   ↓
5. Update archive/{changeId}.json with processing results
   ↓
6. Delete customers/{customer-code}/{changeId}.json (cleanup)
   ↓
7. Acknowledge SQS message (only after successful archive update)
```

## Idempotency Guarantees

### How Idempotency Works

1. **Trigger Existence Check**: Before processing, backend checks if the trigger file still exists
2. **Already Processed**: If trigger is missing, processing is skipped (duplicate event)
3. **Safe Retries**: All operations can be safely retried without side effects
4. **Meeting Creation**: Uses `changeId` as idempotency key in Microsoft Graph API

### Duplicate Event Scenarios

| Scenario | Trigger Exists? | Action |
|----------|----------------|--------|
| First processing attempt | Yes | Process normally |
| Duplicate SQS event (same change) | No | Skip (already processed) |
| Retry after archive update failure | Yes | Retry processing |
| Retry after trigger delete failure | No | Skip (processing complete) |

## Error Handling

### Archive Update Failure

```go
// If archive update fails:
1. Delete trigger file (prevents future duplicate processing)
2. Do NOT acknowledge SQS message
3. Message returns to queue for retry
4. Next attempt will skip (trigger already deleted)
```

**Rationale**: Deleting the trigger prevents duplicate processing attempts while allowing the SQS message to retry.

### Trigger Delete Failure

```go
// If trigger delete fails:
1. Log warning (non-fatal error)
2. Continue processing (archive already updated)
3. Acknowledge SQS message
4. Trigger may remain but won't cause issues (idempotency check)
```

**Rationale**: Processing is complete (archive updated), so trigger deletion failure is non-critical.

### SQS Message Acknowledgment

```go
// SQS message is acknowledged ONLY when:
1. Archive update succeeds
2. Processing results are persisted
3. Trigger deletion attempted (success or failure)
```

**Critical**: Never acknowledge SQS message before archive update succeeds.

## Benefits

### Operational Benefits

1. **No Data Synchronization**: Single source of truth eliminates sync issues
2. **Clean S3 Structure**: No long-term clutter in `customers/` prefix
3. **Immediate Cleanup**: Backend handles cleanup, no lifecycle policies needed
4. **Cost Optimization**: Minimal storage costs (one permanent copy per change)
5. **Clear Separation**: Operational triggers vs. permanent storage

### Technical Benefits

1. **Built-in Idempotency**: Trigger existence provides natural idempotency
2. **Safe Retries**: All operations can be safely retried
3. **Atomic Processing**: Update archive → delete trigger ensures consistency
4. **No Race Conditions**: Single authoritative source prevents conflicts
5. **Simplified Logic**: Backend always knows where to load data

### Debugging Benefits

1. **Clear Audit Trail**: All modifications tracked in `archive/` object
2. **Easy Troubleshooting**: Check if trigger exists to determine processing state
3. **No Confusion**: Only one location to check for current state
4. **Processing History**: Modification array shows complete processing history

## Migration from Previous Pattern

### Old Pattern (Version-Based)

```
customers/{customer-code}/{changeId}-v1.json
customers/{customer-code}/{changeId}-v2.json
archive/{changeId}-v1.json
archive/{changeId}-v2.json
```

**Problems**:
- Multiple copies of same data
- Synchronization issues between customers/ and archive/
- Lifecycle policies needed for cleanup
- Confusion about which version is current

### New Pattern (Transient Trigger)

```
customers/{customer-code}/{changeId}.json  # Temporary trigger
archive/{changeId}.json                     # Single source of truth
```

**Improvements**:
- Single authoritative copy
- No synchronization needed
- Immediate backend-driven cleanup
- Clear processing state

## Best Practices

### Frontend Development

1. **Always upload to archive/ first** before creating customer triggers
2. **Validate archive upload success** before proceeding
3. **Handle partial failures gracefully** (some customer triggers may fail)
4. **Never read from customers/ prefix** (always use archive/)

### Backend Development

1. **Always load from archive/** (never from customers/)
2. **Check trigger existence** before processing (idempotency)
3. **Update archive before deleting trigger** (atomic pattern)
4. **Log all operations** for debugging and audit
5. **Acknowledge SQS only after archive update** succeeds

### Operations

1. **Monitor trigger age** (triggers should be deleted quickly)
2. **Alert on old triggers** (indicates processing issues)
3. **Check archive/ for current state** (not customers/)
4. **Use modification array** for processing history

## Monitoring and Alerting

### Key Metrics

1. **Trigger Creation Rate**: Number of triggers created per minute
2. **Trigger Deletion Rate**: Number of triggers deleted per minute
3. **Trigger Age**: Time between creation and deletion
4. **Archive Update Success Rate**: Percentage of successful archive updates
5. **Processing Duration**: Time from SQS event to completion

### Recommended Alarms

1. **Old Triggers**: Triggers older than 15 minutes (indicates stuck processing)
2. **Archive Update Failures**: More than 5% failure rate
3. **Trigger Deletion Failures**: More than 10% failure rate (non-critical but worth investigating)
4. **SQS Message Age**: Messages older than 30 minutes in queue

## Troubleshooting

See [TRANSIENT_TRIGGER_TROUBLESHOOTING.md](./TRANSIENT_TRIGGER_TROUBLESHOOTING.md) for detailed troubleshooting procedures.

## FAQ

See [TRANSIENT_TRIGGER_FAQ.md](./TRANSIENT_TRIGGER_FAQ.md) for frequently asked questions.

## Related Documentation

- [Operational Runbook](./TRANSIENT_TRIGGER_RUNBOOK.md)
- [Troubleshooting Guide](./TRANSIENT_TRIGGER_TROUBLESHOOTING.md)
- [Architecture Diagrams](./TRANSIENT_TRIGGER_DIAGRAMS.md)
- [FAQ](./TRANSIENT_TRIGGER_FAQ.md)
- [Customer Onboarding](./CUSTOMER_ONBOARDING.md)
