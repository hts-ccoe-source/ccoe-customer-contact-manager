# Design Document

## Overview

This design extends the SES CLI with new `-all` actions that operate on multiple customers concurrently. The design follows the existing pattern established by `import-aws-contact-all`, reading customer configurations from `config.json` and using goroutines to process customers in parallel while maintaining thread-safe logging and error handling.

## Architecture

### High-Level Flow

```
User executes `-all` action
    ↓
Load config.json
    ↓
Extract customer mappings
    ↓
Validate SES role ARNs exist
    ↓
Create worker pool (goroutines)
    ↓
Process customers concurrently
    ↓
Aggregate results
    ↓
Display summary
```

### Component Interaction

```
main.go (CLI entry point)
    ↓
handleSESCommand() - Parse flags and route to action handler
    ↓
handleManageTopicAll() - New handler for manage-topic-all
    ↓
processCustomersConcurrently() - Generic concurrent processor
    ↓
For each customer:
    - Assume SES role ARN
    - Execute single-customer operation
    - Collect results
    ↓
aggregateAndDisplayResults() - Show summary
```

## Components and Interfaces

### 1. New Action Handlers

Create new handler functions for each `-all` action:

```go
// Handler for manage-topic-all action
func handleManageTopicAll(cfg *config.Config, configFile string, dryRun bool) {
    // Load SESConfig.json
    // Process all customers concurrently
    // Display aggregated results
}

// Handler for describe-list-all action
func handleDescribeListAll(cfg *config.Config) {
    // Process all customers concurrently
    // Display aggregated results
}

// Handler for list-contacts-all action
func handleListContactsAll(cfg *config.Config) {
    // Process all customers concurrently
    // Display aggregated results
}

// Handler for describe-topics-all action
func handleDescribeTopicsAll(cfg *config.Config) {
    // Process all customers concurrently
    // Display aggregated results
}
```

### 2. Generic Concurrent Processor

Create a reusable function for processing customers concurrently:

```go
type CustomerOperation func(customerCode string, credentialManager *credentials.CredentialManager) error

type CustomerResult struct {
    CustomerCode string
    Success      bool
    Error        error
    Data         interface{} // Optional result data
}

func processCustomersConcurrently(
    cfg *config.Config,
    operation CustomerOperation,
    maxConcurrency int,
) []CustomerResult {
    // Create worker pool
    // Process customers with concurrency control
    // Collect and return results
}
```

### 3. Result Aggregation

Create functions to aggregate and display results:

```go
func aggregateResults(results []CustomerResult) {
    successCount := 0
    failureCount := 0
    skippedCount := 0
    
    for _, result := range results {
        if result.Success {
            successCount++
        } else if result.Error != nil {
            failureCount++
        } else {
            skippedCount++
        }
    }
    
    // Display summary
}

func displayCustomerResult(result CustomerResult) {
    // Display individual customer result with proper formatting
}
```

### 4. Configuration Validation

Add validation to ensure config.json has required fields:

```go
func validateCustomerConfigs(cfg *config.Config) error {
    if len(cfg.CustomerMappings) == 0 {
        return fmt.Errorf("no customers configured in config.json")
    }
    
    for code, customer := range cfg.CustomerMappings {
        if customer.SESRoleArn == "" {
            log.Printf("⚠️  Warning: Customer %s has no SES role ARN configured, will be skipped\n", code)
        }
    }
    
    return nil
}
```

## Data Models

### CustomerResult Structure

```go
type CustomerResult struct {
    CustomerCode   string
    CustomerName   string
    Success        bool
    Error          error
    Data           interface{}
    ProcessingTime time.Duration
}
```

### Multi-Customer Summary

```go
type MultiCustomerSummary struct {
    TotalCustomers    int
    SuccessfulCount   int
    FailedCount       int
    SkippedCount      int
    Results           []CustomerResult
    TotalDuration     time.Duration
}
```

## Error Handling

### Error Categories

1. **Configuration Errors** - Missing config.json, invalid format
2. **Authentication Errors** - Cannot assume SES role ARN
3. **Operation Errors** - SES API failures
4. **Timeout Errors** - Customer operation takes too long

### Error Handling Strategy

```go
func handleCustomerOperation(customerCode string, operation CustomerOperation) CustomerResult {
    result := CustomerResult{
        CustomerCode: customerCode,
    }
    
    startTime := time.Now()
    
    defer func() {
        result.ProcessingTime = time.Since(startTime)
        if r := recover(); r != nil {
            result.Success = false
            result.Error = fmt.Errorf("panic: %v", r)
        }
    }()
    
    err := operation(customerCode)
    if err != nil {
        result.Success = false
        result.Error = err
        return result
    }
    
    result.Success = true
    return result
}
```

### Graceful Degradation

- If one customer fails, continue processing others
- Log errors with customer context
- Display summary at the end showing all failures
- Exit with non-zero code if any customer failed

## Testing Strategy

### Unit Tests

1. **Test concurrent processing logic**
   - Verify correct number of goroutines spawned
   - Verify results are collected correctly
   - Verify thread-safe logging

2. **Test result aggregation**
   - Verify counts are accurate
   - Verify error messages are preserved

3. **Test configuration validation**
   - Verify missing config.json is detected
   - Verify missing SES role ARNs are detected

### Integration Tests

1. **Test with mock customers**
   - Create test config.json with multiple customers
   - Mock SES operations
   - Verify all customers are processed

2. **Test error scenarios**
   - One customer fails, others succeed
   - All customers fail
   - No customers configured

3. **Test dry-run mode**
   - Verify no actual changes are made
   - Verify preview output is correct

### Manual Testing

1. **Test manage-topic-all**
   - Run against real customers in non-prod
   - Verify topics are created/updated correctly
   - Verify dry-run shows correct preview

2. **Test describe-list-all**
   - Run against real customers
   - Verify output is formatted correctly
   - Verify all customers are included

3. **Test concurrency control**
   - Test with different `--max-customer-concurrency` values
   - Verify performance scales appropriately

## Implementation Details

### New CLI Flags

No new flags needed - reuse existing:
- `--dry-run` - Preview mode
- `--max-customer-concurrency` - Control parallelism (new, optional)

### New Actions

Add to the switch statement in `handleSESCommand()`:

```go
case "manage-topic-all":
    handleManageTopicAll(cfg, configFile, *dryRun)
case "describe-list-all":
    handleDescribeListAll(cfg)
case "list-contacts-all":
    handleListContactsAll(cfg)
case "describe-topics-all":
    handleDescribeTopicsAll(cfg)
```

### Backward Compatibility

- Existing single-customer actions remain unchanged
- No changes to existing function signatures
- New `-all` actions are additive only

## Concurrency Model

### Worker Pool Pattern

```go
func processCustomersConcurrently(
    customers map[string]*config.CustomerConfig,
    operation CustomerOperation,
    maxConcurrency int,
) []CustomerResult {
    // Default to processing all customers concurrently
    if maxConcurrency <= 0 || maxConcurrency > len(customers) {
        maxConcurrency = len(customers)
    }
    
    var wg sync.WaitGroup
    var mu sync.Mutex
    results := make([]CustomerResult, 0, len(customers))
    
    // Create customer channel
    customerChan := make(chan *config.CustomerConfig, len(customers))
    
    // Start workers
    for i := 0; i < maxConcurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for customer := range customerChan {
                result := processCustomer(customer, operation)
                
                mu.Lock()
                results = append(results, result)
                mu.Unlock()
            }
        }()
    }
    
    // Send customers to workers
    for _, customer := range customers {
        customerChan <- customer
    }
    close(customerChan)
    
    // Wait for completion
    wg.Wait()
    
    return results
}
```

### Thread Safety

- Use `sync.Mutex` for result aggregation
- Use `sync.WaitGroup` for goroutine coordination
- Use buffered channels for customer distribution
- Logging is thread-safe (both `log` and `slog` packages are safe for concurrent use)

### Log Buffering for Concurrent Operations

To prevent interleaved log output from concurrent customer processing, all multi-customer operations MUST buffer logs per customer and flush them as a block:

```go
// CustomerLogBuffer buffers log messages for a customer to flush them as a block
type CustomerLogBuffer struct {
    customerCode string
    logs         []string
    mu           sync.Mutex
}

func (b *CustomerLogBuffer) Printf(format string, args ...interface{}) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.logs = append(b.logs, fmt.Sprintf(format, args...))
}

func (b *CustomerLogBuffer) Flush() {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    if len(b.logs) == 0 {
        return
    }
    
    // Print all logs as a block with clear customer separation
    fmt.Printf("\n═══════════════════════════════════════════════════════════════\n")
    fmt.Printf("Customer: %s\n", b.customerCode)
    fmt.Printf("═══════════════════════════════════════════════════════════════\n")
    for _, logMsg := range b.logs {
        fmt.Println(logMsg)
    }
    fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")
}
```

**Benefits:**
- Clean, readable output with clear customer boundaries
- No interleaved log messages from concurrent operations
- Easy to identify which logs belong to which customer
- Maintains chronological order within each customer's processing

**Implementation Pattern:**
1. Create a `CustomerLogBuffer` at the start of each customer operation
2. Pass the buffer to all functions that need to log
3. Flush the buffer at the end of customer processing (success or failure)
4. Use `fmt.Printf` instead of `log.Printf` to avoid double timestamps

## Output Format

### Progress Indicators with Buffered Logs

```
🔄 Processing 3 customers concurrently...

═══════════════════════════════════════════════════════════════
Customer: htsnonprod
═══════════════════════════════════════════════════════════════
🔄 Customer htsnonprod: Starting processing
🔐 Customer htsnonprod: Using Identity Center role from config: arn:aws:iam::...
📊 Customer htsnonprod: Retrieving Identity Center data via role assumption
✅ Customer htsnonprod: Retrieved 150 users and 200 group memberships
🔐 Customer htsnonprod: Assuming SES role: arn:aws:iam::...
📥 Customer htsnonprod: Importing contacts to SES
✅ Customer htsnonprod: Successfully imported contacts
═══════════════════════════════════════════════════════════════

═══════════════════════════════════════════════════════════════
Customer: hts
═══════════════════════════════════════════════════════════════
🔄 Customer hts: Starting processing
🔐 Customer hts: Using Identity Center role from config: arn:aws:iam::...
📊 Customer hts: Retrieving Identity Center data via role assumption
✅ Customer hts: Retrieved 200 users and 300 group memberships
🔐 Customer hts: Assuming SES role: arn:aws:iam::...
📥 Customer hts: Importing contacts to SES
✅ Customer hts: Successfully imported contacts
═══════════════════════════════════════════════════════════════

═══════════════════════════════════════════════════════════════
Customer: customer3
═══════════════════════════════════════════════════════════════
🔄 Customer customer3: Starting processing
❌ Customer customer3: Failed to assume SES role
═══════════════════════════════════════════════════════════════

📊 Summary:
   Total customers: 3
   ✅ Successful: 2
   ❌ Failed: 1
   ⏭️  Skipped: 0
   ⏱️  Total time: 2.5s
```

### Dry-Run Output

```
🔍 DRY RUN MODE - No changes will be made

🔄 Customer htsnonprod:
   Would manage topics:
   - aws-calendar
   - aws-announce
   - aws-approval

🔄 Customer hts:
   Would manage topics:
   - aws-calendar
   - aws-announce
   - aws-approval

📊 Dry-run complete: 2 customers would be processed
```

## Performance Considerations

### Concurrency Defaults

- Default: Process all customers concurrently (no limit)
- Rationale: Each customer is a separate AWS account with independent rate limits
- Override: Allow `--max-customer-concurrency` to limit if needed

### Memory Usage

- Each goroutine uses minimal memory (~2KB stack)
- Processing 10-20 customers concurrently is negligible
- Results are collected in a slice (small memory footprint)

### API Rate Limits

- Each customer has independent SES rate limits
- No shared rate limiting needed between customers
- Individual customer operations already have rate limiting

## Security Considerations

### IAM Role Assumptions

- Each customer's SES role ARN is assumed independently
- Failed role assumptions don't affect other customers
- Credentials are scoped to individual goroutines

### Configuration Security

- config.json should be protected (contains role ARNs)
- No sensitive data in logs
- Errors don't expose credentials

## Migration Path

### Phase 1: Add New Actions

1. Implement `manage-topic-all`
2. Test thoroughly in non-prod
3. Document usage

### Phase 2: Add Read-Only Actions

1. Implement `describe-list-all`
2. Implement `list-contacts-all`
3. Implement `describe-topics-all`

### Phase 3: Documentation and Training

1. Update README with new actions
2. Add examples to documentation
3. Update help text

## Alternative Approaches Considered

### Sequential Processing

**Rejected**: Too slow for multiple customers. Concurrent processing is essential for good UX.

### External Orchestration (Step Functions)

**Rejected**: Adds complexity. CLI should be self-contained and easy to run locally or in ECS.

### Separate Binary for Multi-Customer Operations

**Rejected**: Increases maintenance burden. Better to have one binary with both single and multi-customer modes.

## Open Questions

None - design is complete and ready for implementation.

## Dependencies

- Existing `config` package for loading config.json
- Existing `credentials` package for role assumption
- Existing SES operation handlers (reuse for single-customer logic)
- Go standard library: `sync`, `time`, `fmt`

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| One customer failure affects others | Medium | Isolate errors per customer, continue processing |
| Concurrent logging is garbled | Low | Use thread-safe logging, add customer prefix |
| Config.json missing or invalid | High | Validate early, fail fast with clear error |
| Too many concurrent operations | Low | Default to all customers (separate accounts), allow override |
| Breaking existing workflows | High | Keep existing actions unchanged, add new `-all` actions |
