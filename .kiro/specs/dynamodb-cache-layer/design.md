# Design Document

## Overview

This design adds a DynamoDB cache layer to accelerate read operations while maintaining S3 as the source of truth. The implementation uses a write-through cache pattern where writes go to DynamoDB first, then S3. S3 events keep the cache synchronized for direct S3 writes. The cache stores recent/active documents (90-day TTL) to optimize common query patterns: time-range filtering and single-object lookups.

### Key Design Decisions

1. **Write-Through Pattern**: DynamoDB writes before S3 ensures cache consistency and allows rollback on S3 failures
2. **90-Day TTL**: Balances query performance for recent documents with storage costs
3. **S3 as Source of Truth**: Maintains existing event-driven workflows and durability guarantees
4. **Fallback Strategy**: Graceful degradation to S3 when DynamoDB is unavailable
5. **Eventual Consistency**: Acceptable for S3 event-driven cache updates (typically <1 second delay)

## Architecture

### High-Level Flow

```
Write Path (New):
  Application → DynamoDB Cache → S3 Storage → S3 Events → (already in cache)

Read Path (New):
  Application → DynamoDB Cache (hit) → Return
  Application → DynamoDB Cache (miss) → S3 Storage → Update Cache → Return

Direct S3 Write Path (Existing):
  External Tool → S3 Storage → S3 Events → Lambda → DynamoDB Cache
```

### DynamoDB Table Design

**Table Name**: `ccoe-customer-contact-manager-cache`

**Primary Key**:
- Partition Key: `object_id` (String) - The changeId or announcementId
- Sort Key: None (simple primary key for direct lookups)

**Global Secondary Indexes**:

1. **TimeRangeIndex** (for time-range queries)
   - Partition Key: `object_type` (String) - "change" or "announcement"
   - Sort Key: `created_at` (Number) - Unix timestamp
   - Projection: ALL

2. **CustomerTimeIndex** (for customer-specific time-range queries)
   - Partition Key: `customer_code` (String) - e.g., "hts", "cds"
   - Sort Key: `created_at` (Number) - Unix timestamp
   - Projection: ALL

3. **StatusTimeIndex** (for status-filtered time-range queries)
   - Partition Key: `status` (String) - e.g., "draft", "submitted", "approved"
   - Sort Key: `created_at` (Number) - Unix timestamp
   - Projection: ALL

**Attributes**:
- `object_id` (String, PK) - Unique identifier
- `object_type` (String) - "change" or "announcement"
- `customer_code` (String) - Primary customer code (first in customers array)
- `customers` (StringSet) - All affected customer codes
- `status` (String) - Current status
- `created_at` (Number) - Unix timestamp
- `modified_at` (Number) - Unix timestamp
- `s3_bucket` (String) - S3 bucket name
- `s3_key` (String) - S3 object key
- `ttl` (Number) - TTL expiration timestamp (created_at + 90 days)
- `document_json` (String) - Full JSON document (for single-object lookups)

**Capacity Mode**: On-Demand (handles variable workload, no capacity planning needed)

**TTL Configuration**: Enabled on `ttl` attribute

### Component Interactions

```
┌─────────────────┐
│   Application   │
│   (main.go)     │
└────────┬────────┘
         │
         ├─────────────────────────────────┐
         │                                 │
         v                                 v
┌─────────────────┐              ┌─────────────────┐
│  Cache Manager  │              │  S3 Operations  │
│ (internal/cache)│              │(internal/lambda)│
└────────┬────────┘              └────────┬────────┘
         │                                 │
         v                                 v
┌─────────────────┐              ┌─────────────────┐
│    DynamoDB     │              │       S3        │
│   (AWS SDK)     │              │   (AWS SDK)     │
└─────────────────┘              └────────┬────────┘
                                          │
                                          v
                                 ┌─────────────────┐
                                 │   S3 Events     │
                                 │   → Lambda      │
                                 └────────┬────────┘
                                          │
                                          v
                                 ┌─────────────────┐
                                 │ Event Handler   │
                                 │(internal/cache) │
                                 └─────────────────┘
```

## Components and Interfaces

### 1. Cache Manager (`internal/cache/manager.go`)

Primary interface for cache operations.

```go
type CacheManager struct {
    dynamoClient *dynamodb.Client
    tableName    string
    ttlDays      int
    dryRun       bool
    logger       *slog.Logger
}

// Core operations
func NewCacheManager(region string, tableName string, ttlDays int, dryRun bool) (*CacheManager, error)
func (cm *CacheManager) PutDocument(ctx context.Context, doc *types.ChangeMetadata) error
func (cm *CacheManager) GetDocument(ctx context.Context, objectID string) (*types.ChangeMetadata, error)
func (cm *CacheManager) DeleteDocument(ctx context.Context, objectID string) error
func (cm *CacheManager) QueryByTimeRange(ctx context.Context, objectType string, startTime, endTime time.Time) ([]*types.ChangeMetadata, error)
func (cm *CacheManager) QueryByCustomerAndTime(ctx context.Context, customerCode string, startTime, endTime time.Time) ([]*types.ChangeMetadata, error)
func (cm *CacheManager) QueryByStatusAndTime(ctx context.Context, status string, startTime, endTime time.Time) ([]*types.ChangeMetadata, error)
```

### 2. Write-Through Handler (`internal/cache/write_through.go`)

Coordinates writes to both DynamoDB and S3 with rollback on failure.

```go
type WriteThroughHandler struct {
    cacheManager *CacheManager
    s3Manager    *lambda.S3UpdateManager
    dryRun       bool
    logger       *slog.Logger
}

func NewWriteThroughHandler(cacheManager *CacheManager, s3Manager *lambda.S3UpdateManager, dryRun bool) *WriteThroughHandler
func (wth *WriteThroughHandler) WriteDocument(ctx context.Context, bucket, key string, doc *types.ChangeMetadata) error
func (wth *WriteThroughHandler) rollbackCacheWrite(ctx context.Context, objectID string) error
```

**Write Algorithm**:
1. Validate document
2. Write to DynamoDB cache (with retry)
3. If DynamoDB write fails, return error
4. Write to S3 (with retry)
5. If S3 write fails, rollback DynamoDB write
6. Return success

### 3. S3 Event Handler (`internal/cache/s3_event_handler.go`)

Lambda function handler for S3 events to keep cache synchronized.

```go
type S3EventHandler struct {
    cacheManager *CacheManager
    s3Client     *s3.Client
    logger       *slog.Logger
}

func NewS3EventHandler(cacheManager *CacheManager, s3Client *s3.Client) *S3EventHandler
func (seh *S3EventHandler) HandleS3Event(ctx context.Context, event events.S3Event) error
func (seh *S3EventHandler) handlePutEvent(ctx context.Context, record events.S3EventRecord) error
func (seh *S3EventHandler) handleDeleteEvent(ctx context.Context, record events.S3EventRecord) error
```

**Event Handling Algorithm**:
1. Parse S3 event record
2. For PUT events:
   - Download document from S3
   - Parse JSON
   - Write to DynamoDB cache
3. For DELETE events:
   - Remove from DynamoDB cache
4. Log all operations with structured logging

### 4. Fallback Reader (`internal/cache/fallback_reader.go`)

Handles cache misses and DynamoDB unavailability.

```go
type FallbackReader struct {
    cacheManager *CacheManager
    s3Manager    *lambda.S3UpdateManager
    logger       *slog.Logger
}

func NewFallbackReader(cacheManager *CacheManager, s3Manager *lambda.S3UpdateManager) *FallbackReader
func (fr *FallbackReader) GetDocumentWithFallback(ctx context.Context, objectID, bucket, key string) (*types.ChangeMetadata, error)
func (fr *FallbackReader) backfillCache(ctx context.Context, doc *types.ChangeMetadata) error
```

**Fallback Algorithm**:
1. Try DynamoDB cache
2. If cache hit, return document
3. If cache miss or error:
   - Try S3 read
   - If S3 succeeds, backfill cache
   - Return document
4. If both fail, return error

### 5. Cache Item Converter (`internal/cache/converter.go`)

Converts between ChangeMetadata/AnnouncementMetadata and DynamoDB items.

```go
type CacheItemConverter struct{}

func (cic *CacheItemConverter) ToItem(doc interface{}) (map[string]types.AttributeValue, error)
func (cic *CacheItemConverter) FromItem(item map[string]types.AttributeValue) (*types.ChangeMetadata, error)
func (cic *CacheItemConverter) extractCustomerCode(customers []string) string
func (cic *CacheItemConverter) calculateTTL(createdAt time.Time, ttlDays int) int64
```

### 6. Rate Limiter (`internal/cache/rate_limiter.go`)

Implements token bucket rate limiting for DynamoDB operations.

```go
type DynamoRateLimiter struct {
    requestsPerSecond int
    ticker            *time.Ticker
    tokens            chan struct{}
}

func NewDynamoRateLimiter(requestsPerSecond int) *DynamoRateLimiter
func (drl *DynamoRateLimiter) Wait(ctx context.Context) error
func (drl *DynamoRateLimiter) Stop()
```

## Data Models

### Cache Item Structure

```json
{
  "object_id": "CHG-2025-001",
  "object_type": "change",
  "customer_code": "hts",
  "customers": ["hts", "cds"],
  "status": "approved",
  "created_at": 1729123456,
  "modified_at": 1729234567,
  "s3_bucket": "ccoe-change-metadata",
  "s3_key": "customers/hts/CHG-2025-001.json",
  "ttl": 1736899456,
  "document_json": "{...full JSON document...}"
}
```

### Configuration

Environment variables for cache configuration:

```bash
DYNAMODB_CACHE_TABLE_NAME="ccoe-customer-contact-manager-cache"
DYNAMODB_CACHE_TTL_DAYS="90"
DYNAMODB_CACHE_ENABLED="true"
DYNAMODB_RATE_LIMIT_RPS="100"
```

## Error Handling

### Error Types

```go
type CacheError struct {
    Type      CacheErrorType
    Message   string
    Cause     error
    Retryable bool
}

type CacheErrorType string

const (
    CacheErrorTypeNotFound      CacheErrorType = "not_found"
    CacheErrorTypeThrottling    CacheErrorType = "throttling"
    CacheErrorTypeValidation    CacheErrorType = "validation"
    CacheErrorTypeNetwork       CacheErrorType = "network"
    CacheErrorTypeUnavailable   CacheErrorType = "unavailable"
)
```

### Retry Strategy

- **DynamoDB Throttling**: Exponential backoff starting at 100ms, max 5 retries
- **Network Errors**: Exponential backoff starting at 1s, max 3 retries
- **S3 Errors**: Use existing S3UpdateManager retry logic
- **Non-Retryable Errors**: Validation errors, not found errors

### Fallback Behavior

1. **DynamoDB Unavailable**: Fall back to S3 for all reads
2. **S3 Unavailable**: Return error (S3 is source of truth)
3. **Both Unavailable**: Return error with clear message
4. **Partial Failure**: Log warning, continue with available data source

## Testing Strategy

### Unit Tests

1. **Cache Manager Tests** (`internal/cache/manager_test.go`)
   - Test PutDocument with valid/invalid documents
   - Test GetDocument with cache hit/miss
   - Test DeleteDocument
   - Test query operations with various filters
   - Test TTL calculation
   - Test dry-run mode

2. **Write-Through Handler Tests** (`internal/cache/write_through_test.go`)
   - Test successful write to both DynamoDB and S3
   - Test DynamoDB failure (no S3 write)
   - Test S3 failure (rollback DynamoDB)
   - Test retry logic
   - Test idempotency

3. **S3 Event Handler Tests** (`internal/cache/s3_event_handler_test.go`)
   - Test PUT event handling
   - Test DELETE event handling
   - Test malformed events
   - Test retry logic

4. **Fallback Reader Tests** (`internal/cache/fallback_reader_test.go`)
   - Test cache hit
   - Test cache miss with S3 fallback
   - Test cache unavailable with S3 fallback
   - Test backfill logic

5. **Converter Tests** (`internal/cache/converter_test.go`)
   - Test ChangeMetadata to DynamoDB item conversion
   - Test AnnouncementMetadata to DynamoDB item conversion
   - Test DynamoDB item to ChangeMetadata conversion
   - Test customer code extraction
   - Test TTL calculation

### Integration Tests

1. **End-to-End Write Test**
   - Write document via write-through handler
   - Verify DynamoDB cache entry
   - Verify S3 object
   - Query by time range
   - Query by customer code

2. **S3 Event Integration Test**
   - Upload document directly to S3
   - Trigger S3 event
   - Verify cache update
   - Query cache for document

3. **Fallback Integration Test**
   - Disable DynamoDB (simulate unavailability)
   - Perform read operations
   - Verify S3 fallback
   - Re-enable DynamoDB
   - Verify cache backfill

### Performance Tests

1. **Query Performance**
   - Measure time-range query latency (target: <200ms for 90-day range)
   - Measure single-object lookup latency (target: <50ms)
   - Compare DynamoDB vs S3 query performance

2. **Concurrency Tests**
   - Test concurrent writes (10-100 concurrent operations)
   - Test concurrent reads (100-1000 concurrent operations)
   - Verify rate limiting behavior

3. **TTL Tests**
   - Create cache entries with short TTL (1 minute for testing)
   - Verify automatic expiration
   - Verify S3 objects remain after TTL expiration

## Deployment Considerations

### Infrastructure Changes

1. **DynamoDB Table Creation**
   - Create table with on-demand capacity
   - Create GSIs (TimeRangeIndex, CustomerTimeIndex, StatusTimeIndex)
   - Enable TTL on `ttl` attribute
   - Tag table for cost tracking

2. **Lambda Function for S3 Events**
   - Create new Lambda function: `ccoe-cache-sync-handler`
   - Configure S3 event trigger for PUT and DELETE events
   - Set timeout: 30 seconds
   - Set memory: 256 MB
   - Add IAM permissions for DynamoDB and S3

3. **IAM Permissions**
   - Add DynamoDB permissions to existing Lambda execution role
   - Add DynamoDB permissions to CLI execution role/user
   - Permissions needed:
     - `dynamodb:PutItem`
     - `dynamodb:GetItem`
     - `dynamodb:DeleteItem`
     - `dynamodb:Query`
     - `dynamodb:DescribeTable`

### Migration Strategy

1. **Phase 1: Deploy Infrastructure**
   - Create DynamoDB table
   - Deploy Lambda function
   - Configure S3 event triggers

2. **Phase 2: Enable Write-Through (Opt-In)**
   - Set `DYNAMODB_CACHE_ENABLED=false` initially
   - Deploy code changes
   - Enable cache for test environment
   - Monitor for issues

3. **Phase 3: Backfill Cache**
   - Run CLI command to backfill recent documents (last 90 days)
   - Monitor DynamoDB write capacity
   - Verify cache entries

4. **Phase 4: Enable for Production**
   - Set `DYNAMODB_CACHE_ENABLED=true` in production
   - Monitor cache hit rates
   - Monitor query performance improvements

### Monitoring and Metrics

**CloudWatch Metrics**:
- Cache hit rate (custom metric)
- Cache miss rate (custom metric)
- Query latency (custom metric)
- DynamoDB read/write capacity
- Lambda invocation count and errors
- S3 event processing latency

**Alarms**:
- Cache hit rate < 80%
- Query latency > 500ms (p99)
- Lambda error rate > 1%
- DynamoDB throttling events

**Logs**:
- All cache operations logged with structured JSON
- Include operation type, duration, success/failure
- Include cache hit/miss status for reads
- Include retry attempts and final outcome

## Cost Estimation

### DynamoDB Costs (On-Demand)

Assumptions:
- 10,000 documents per month
- 90-day retention (27,000 active documents)
- Average document size: 10 KB
- Read:Write ratio: 10:1

**Storage**: 27,000 documents × 10 KB = 270 MB = $0.07/month

**Writes**: 10,000 writes/month = $1.25/month

**Reads**: 100,000 reads/month = $0.25/month

**Total DynamoDB**: ~$1.57/month

### Lambda Costs

Assumptions:
- 10,000 S3 events per month
- 256 MB memory
- 100ms average execution time

**Invocations**: 10,000 × $0.20/1M = $0.002

**Compute**: 10,000 × 0.1s × 256MB = 256,000 MB-seconds = $0.004

**Total Lambda**: ~$0.006/month

### Total Additional Cost

**~$1.58/month** for the cache layer

This is negligible compared to the performance improvement for time-range queries.

## Security Considerations

1. **Encryption at Rest**: Enable DynamoDB encryption using AWS managed keys
2. **Encryption in Transit**: All AWS SDK calls use TLS
3. **IAM Least Privilege**: Grant only necessary DynamoDB permissions
4. **VPC Endpoints**: Use VPC endpoints for DynamoDB if Lambda is in VPC
5. **Audit Logging**: Enable CloudTrail for DynamoDB API calls
6. **Data Retention**: TTL ensures automatic cleanup of old data
