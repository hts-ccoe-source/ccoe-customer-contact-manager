# main.go Logging Audit

## Summary

**Total log statements found in main.go: ~60**
- `log.Printf`: 5 statements (all warnings with emoji)
- `log.Fatal` / `log.Fatalf`: ~55 statements (error handling)
- `fmt.Printf`: ~200+ statements (CLI output, not CloudWatch logs)

**Important Note:** main.go is primarily a CLI application. The Lambda handler immediately delegates to `lambda.StartLambdaMode()` which is in `internal/lambda/handlers.go`. The actual Lambda logging happens in the internal packages, NOT in main.go.

## Analysis by Category

### 1. Lambda Handler (Lines 107-112)
```go
if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
    lambda.SetVersionInfo(Version, GitCommit, BuildTime)
    lambda.StartLambdaMode()
    return
}
```
**Category:** KEEP AS-IS
**Reason:** This immediately delegates to the Lambda handler in internal/lambda. No logging here.

### 2. log.Printf Statements (5 total)

#### 2.1 Customer SES Role ARN Warnings (4 occurrences)
**Lines:** 1257, 1384, 1482, 2097
```go
log.Printf("⚠️  Warning: Customer %s (%s) has no SES role ARN configured, will be skipped\n",
    code, customer.CustomerName)
```
**Category:** CONVERT TO WARN
**Action:** Convert to `logger.Warn("customer has no SES role ARN", "customer_code", code, "customer_name", customer.CustomerName)`
**Reason:** This is a legitimate warning for CLI operations. Remove emoji.

#### 2.2 Reputation Metrics Warning (1 occurrence)
**Line:** 3307
```go
log.Printf("Warning: Could not retrieve reputation metrics: %v", err)
```
**Category:** CONVERT TO WARN
**Action:** Convert to `logger.Warn("could not retrieve reputation metrics", "error", err)`
**Reason:** Non-fatal warning for CLI operations.

### 3. log.Fatal / log.Fatalf Statements (~55 total)

All `log.Fatal` and `log.Fatalf` calls are in CLI command handlers for:
- Missing required flags
- Configuration validation failures
- Customer code not found
- Failed AWS operations

**Category:** KEEP AS-IS
**Reason:** These are CLI-only error handlers. They never execute in Lambda mode because Lambda delegates immediately to `lambda.StartLambdaMode()`. Fatal errors in CLI are appropriate.

**Examples:**
```go
log.Fatal("Customer code is required for create-contact-list action")
log.Fatalf("Failed to load config: %v", err)
log.Fatalf("Customer code %s not found in configuration", *customerCode)
```

### 4. fmt.Printf Statements (~200+ total)

All `fmt.Printf` calls are for CLI output:
- Usage messages
- Progress indicators
- Results display
- Dry-run output
- Summary statistics

**Category:** KEEP AS-IS
**Reason:** These are CLI user interface outputs, not logs. They never appear in CloudWatch because Lambda mode bypasses all CLI code.

**Examples:**
```go
fmt.Printf("🔧 SES Domain Configuration\n")
fmt.Printf("Customers to process: %d\n", len(customersToProcess))
fmt.Printf("✅ Successfully created contact list: %s\n", listName)
```

## Recommendations

### For Task 2 (Current Task - Audit main.go)

**Target: Reduce from ~200+ logs to ~20-30 logs**

**Reality Check:** main.go has only **5 log.Printf statements** that need attention. The "~200+ logs" mentioned in the task description are actually:
- 5 log.Printf (warnings)
- ~55 log.Fatal (CLI error handling - keep as-is)
- ~200+ fmt.Printf (CLI output - not logs, keep as-is)

**Actual Work Required:**
1. Convert 5 log.Printf statements to slog
2. Remove emoji from those 5 statements
3. Document that main.go is CLI-only and Lambda logging happens in internal/lambda

### Categorization of 5 log.Printf Statements

| Line | Statement | Category | Action | Reason |
|------|-----------|----------|--------|--------|
| 1257 | Customer SES role warning | WARN | Convert to slog.Warn | Legitimate warning, remove emoji |
| 1384 | Customer SES role warning | WARN | Convert to slog.Warn | Legitimate warning, remove emoji |
| 1482 | Customer SES role warning | WARN | Convert to slog.Warn | Legitimate warning, remove emoji |
| 2097 | Customer SES role warning | WARN | Convert to slog.Warn | Legitimate warning, remove emoji |
| 3307 | Reputation metrics warning | WARN | Convert to slog.Warn | Non-fatal warning, remove emoji |

### For Task 3 (Migrate main.go to slog)

**Scope:** Only 5 log.Printf statements need conversion
**Approach:**
1. Initialize slog at start of CLI commands (not Lambda - Lambda uses internal/lambda logger)
2. Convert 5 log.Printf to logger.Warn
3. Remove emoji characters
4. Keep all log.Fatal as-is (CLI error handling)
5. Keep all fmt.Printf as-is (CLI output)

### Critical Clarification

**The actual Lambda logging reduction work is in these files:**
- `internal/lambda/handlers.go` - Lambda handler entry point
- `internal/processors/announcement_processor.go` - Announcement processing
- `internal/ses/meetings.go` - Meeting operations
- `internal/ses/operations.go` - SES operations

**main.go is NOT where the Lambda logging problem exists.** The task description's "~200+ logs" refers to the entire codebase, not just main.go.

## Scheduled CLI Commands Requiring Structured Logging

**Important:** Some CLI commands are run on a scheduled basis (e.g., via cron/ECS scheduled tasks) and need structured JSON logging in addition to human-friendly CLI output.

### Commands Requiring Dual Logging Support

1. **`import-aws-contact-all`** (handleImportAWSContactAll)
   - Currently: Uses fmt.Printf with emoji for CLI output
   - Needs: Structured JSON logging option via --log-format flag
   - Status: NO structured logging support currently

2. **`manage-topic-all`** (handleManageTopicAll)
   - Currently: Uses fmt.Printf with emoji for CLI output
   - Needs: Structured JSON logging option via --log-format flag
   - Status: NO structured logging support currently

3. **`describe-list-all`** (handleDescribeListAll)
   - Currently: Uses fmt.Printf with emoji for CLI output
   - Needs: Structured JSON logging option via --log-format flag
   - Status: NO structured logging support currently

4. **`configure-ses-complete`** (handleConfigureSESComplete)
   - Currently: Accepts --log-format and --log-level flags
   - Delegates to handleSESConfigureDomainAction and handleConfigureDeliverability
   - Status: ✅ ALREADY HAS structured logging support (JSON/text)

### Implementation Requirements

For the 3 commands without structured logging:

1. **Add CLI flags:**
   ```go
   logLevel := fs.String("log-level", "info", "Log level (debug, info, warn, error)")
   logFormat := fs.String("log-format", "text", "Log format (text, json)")
   ```

2. **Initialize slog logger:**
   ```go
   var handler slog.Handler
   if strings.ToLower(*logFormat) == "json" {
       handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
   } else {
       handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
   }
   logger := slog.New(handler)
   ```

3. **Dual output pattern:**
   - When `--log-format=json`: Use logger.Info/Warn/Error for all output (no emoji, no fmt.Printf)
   - When `--log-format=text` (default): Use fmt.Printf ONLY for human-friendly output (keep emoji, NO slog)
   - Completely separate: JSON mode = structured logs only, Text mode = CLI output only

4. **Pass logger to internal functions:**
   - Update function signatures to accept `*slog.Logger`
   - Use logger for operational events
   - Keep fmt.Printf for progress indicators in text mode

### Example Pattern

```go
func handleImportAWSContactAll(cfg *types.Config, logLevel, logFormat *string, ...) {
    if *logFormat == "json" {
        // JSON mode: Structured logging only (no emoji, no fmt.Printf)
        logger := setupLogger(*logLevel, *logFormat)
        logger.Info("starting import", 
            "total_customers", len(customers),
            "max_concurrency", maxConcurrency)
    } else {
        // Text mode: Human-friendly CLI output only (with emoji, no slog)
        fmt.Printf("🚀 Starting concurrent customer processing\n")
        fmt.Printf("📊 Total customers: %d\n", len(customers))
        fmt.Printf("⚙️  Max concurrency: %d\n", maxConcurrency)
    }
}
```

## Conclusion

**For main.go specifically:**
- **Total log.Printf to convert:** 5 statements
- **Total log.Fatal to keep:** ~55 statements (CLI error handling)
- **Total fmt.Printf to keep:** ~200+ statements (CLI output)
- **Reduction target:** Convert 5 log.Printf to slog.Warn, remove emoji
- **Lambda impact:** NONE - Lambda bypasses all main.go CLI code
- **Scheduled CLI commands:** 3 commands need structured logging support added

**Next Steps:**
1. Update task description to clarify main.go scope
2. Focus logging reduction efforts on internal/lambda and internal/processors
3. Convert 5 log.Printf in main.go to slog.Warn as part of Task 3
4. Add structured logging support to 3 scheduled CLI commands:
   - import-aws-contact-all
   - manage-topic-all
   - describe-list-all
