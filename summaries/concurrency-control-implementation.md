# Concurrency Control Implementation for Multi-Customer SES Operations

## Overview

Implemented the `--max-customer-concurrency` flag to control how many customers are processed in parallel during multi-customer SES operations. This provides administrators with fine-grained control over resource usage and API rate limiting.

## Changes Made

### 1. CLI Flag Addition

Added new flag to `handleSESCommand()` in `main.go`:
```go
maxCustomerConcurrency := fs.Int("max-customer-concurrency", 0, "Maximum concurrent customers for -all actions (0 = unlimited, default)")
```

**Default Behavior:**
- Value of `0` (default) = unlimited concurrency (processes all customers in parallel)
- Positive value = limits concurrent processing to that number of customers
- Values higher than the number of customers are automatically capped

### 2. Function Signature Updates

Updated all `-all` action handlers to accept the concurrency parameter:

**handleDescribeListAll:**
```go
func handleDescribeListAll(cfg *types.Config, maxCustomerConcurrency int)
```

**handleListContactsAll:**
```go
func handleListContactsAll(cfg *types.Config, maxCustomerConcurrency int)
```

**handleManageTopicAll:**
```go
func handleManageTopicAll(cfg *types.Config, configFile *string, dryRun bool, maxCustomerConcurrency int)
```

**handleDescribeTopicsAll:**
```go
func handleDescribeTopicsAll(cfg *types.Config, maxCustomerConcurrency int)
```

### 3. Concurrency Control Implementation

Each handler now:
1. Accepts the `maxCustomerConcurrency` parameter
2. Displays concurrency limit in operation summary (when limit is active)
3. Passes the value to `concurrent.ProcessCustomersConcurrently()`

**Example output when limit is active:**
```
🔄 Managing topics across 5 customer(s)
📋 SES Config: SESConfig.json
📊 Topics to manage: 3
⚙️  Concurrency limit: 2 customers at a time
```

### 4. Updated Function Calls

Updated all switch case statements to pass the new parameter:
```go
case "describe-list-all":
    handleDescribeListAll(cfg, *maxCustomerConcurrency)
case "list-contacts-all":
    handleListContactsAll(cfg, *maxCustomerConcurrency)
case "manage-topic-all":
    handleManageTopicAll(cfg, configFile, *dryRun, *maxCustomerConcurrency)
case "describe-topics-all":
    handleDescribeTopicsAll(cfg, *maxCustomerConcurrency)
```

### 5. Documentation Updates

**Help Text:**
- Added flag documentation in COMMON FLAGS section
- Added example showing concurrency control usage

**New Example:**
```bash
# Manage topics with limited concurrency (3 customers at a time)
ccoe-customer-contact-manager ses --action manage-topic-all \
  --config-file config.json --max-customer-concurrency 3
```

## Implementation Details

### Concurrency Control Logic

The actual concurrency control is implemented in `internal/concurrent/processor.go`:

```go
func ProcessCustomersConcurrently(
    customerCodes []string,
    customerNames map[string]string,
    operation CustomerOperation,
    maxConcurrency int,
) []CustomerResult {
    // Default to processing all customers concurrently if maxConcurrency is not set
    if maxConcurrency <= 0 || maxConcurrency > len(customerCodes) {
        maxConcurrency = len(customerCodes)
    }
    
    // Create worker pool with semaphore for concurrency control
    semaphore := make(chan struct{}, maxConcurrency)
    // ... rest of implementation
}
```

**Key Features:**
- Uses a semaphore pattern (buffered channel) to limit concurrent goroutines
- Automatically caps to number of customers if limit is higher
- Zero or negative values mean unlimited concurrency
- Thread-safe result collection with mutex

## Use Cases

### 1. Unlimited Concurrency (Default)
```bash
ccoe-customer-contact-manager ses --action manage-topic-all \
  --config-file config.json
```
Processes all customers in parallel - fastest execution.

### 2. Limited Concurrency
```bash
ccoe-customer-contact-manager ses --action manage-topic-all \
  --config-file config.json --max-customer-concurrency 3
```
Processes 3 customers at a time - useful for:
- Avoiding API rate limits
- Reducing memory usage
- Controlling resource consumption

### 3. Sequential Processing
```bash
ccoe-customer-contact-manager ses --action manage-topic-all \
  --config-file config.json --max-customer-concurrency 1
```
Processes one customer at a time - useful for:
- Debugging
- Ensuring predictable execution order
- Minimizing resource usage

## Requirements Satisfied

✅ **Requirement 4.1:** Support `--max-customer-concurrency` flag to control parallel processing
✅ **Requirement 4.2:** Default to processing all customers concurrently (unlimited)
✅ **Requirement 4.3:** Allow setting lower values to limit concurrency
✅ **Requirement 4.4:** Ignore values higher than number of customers

## Testing

The implementation:
- Compiles successfully without errors
- Flag appears in help output
- All `-all` actions accept the parameter
- Backward compatible (default behavior unchanged)

## Benefits

1. **Performance Control:** Administrators can balance speed vs. resource usage
2. **API Rate Limiting:** Prevents overwhelming AWS APIs with too many concurrent requests
3. **Resource Management:** Reduces memory and CPU usage when needed
4. **Debugging:** Sequential processing (concurrency=1) makes debugging easier
5. **Flexibility:** Default behavior (unlimited) maintains existing performance
