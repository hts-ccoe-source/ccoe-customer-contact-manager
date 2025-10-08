#!/bin/bash

# CCOE Customer Contact Manager - Specific Customer Testing
# Test a specific customer configuration in detail

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Check if customer code is provided
if [[ $# -eq 0 ]]; then
    echo "Usage: $0 <customer-code>"
    echo ""
    echo "Available customers:"
    if [[ -f "$PROJECT_ROOT/CustomerCodes.json" ]]; then
        jq -r '.validCustomers[]' "$PROJECT_ROOT/CustomerCodes.json" 2>/dev/null | sed 's/^/  /'
    else
        echo "  (CustomerCodes.json not found)"
    fi
    exit 1
fi

CUSTOMER_CODE="$1"
LOG_FILE="$RESULTS_DIR/customer-test-$CUSTOMER_CODE-$TIMESTAMP.log"

echo "=== CCOE Customer Contact Manager - Customer-Specific Tests ===" | tee "$LOG_FILE"
echo "Customer: $CUSTOMER_CODE" | tee -a "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Build the application if not already built
cd "$PROJECT_ROOT"
APP_BINARY="$RESULTS_DIR/ccoe-customer-contact-manager-test"
if [[ ! -f "$APP_BINARY" ]]; then
    echo "Building application..." | tee -a "$LOG_FILE"
    if go build -o "$APP_BINARY" .; then
        echo "✅ Application built successfully" | tee -a "$LOG_FILE"
    else
        echo "❌ Failed to build application" | tee -a "$LOG_FILE"
        exit 1
    fi
fi

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

# Function to test customer operation
test_customer_operation() {
    local description="$1"
    local command="$2"
    local expect_success="$3"  # true/false
    
    echo "Testing $description..." | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo "Command: $command" >> "$LOG_FILE"
    
    if eval "$command" >> "$LOG_FILE" 2>&1; then
        if [[ "$expect_success" == "true" ]]; then
            echo "✅ PASS: $description" | tee -a "$LOG_FILE"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "❌ FAIL: $description (expected failure but succeeded)" | tee -a "$LOG_FILE"
        fi
    else
        if [[ "$expect_success" == "false" ]]; then
            echo "✅ PASS: $description (expected failure)" | tee -a "$LOG_FILE"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "❌ FAIL: $description" | tee -a "$LOG_FILE"
        fi
    fi
    echo "" >> "$LOG_FILE"
}

# Test 1: Customer code validation
echo "=== Test 1: Customer Code Validation ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/CustomerCodes.json" ]]; then
    VALID_CUSTOMERS=$(jq -r '.validCustomers[]' "$PROJECT_ROOT/CustomerCodes.json" 2>/dev/null)
    if echo "$VALID_CUSTOMERS" | grep -q "^$CUSTOMER_CODE$"; then
        echo "✅ PASS: Customer code '$CUSTOMER_CODE' is valid" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
        
        # Get customer name
        CUSTOMER_NAME=$(jq -r ".customerMapping[\"$CUSTOMER_CODE\"]" "$PROJECT_ROOT/CustomerCodes.json" 2>/dev/null)
        if [[ "$CUSTOMER_NAME" != "null" && -n "$CUSTOMER_NAME" ]]; then
            echo "   Customer name: $CUSTOMER_NAME" | tee -a "$LOG_FILE"
        fi
    else
        echo "❌ FAIL: Customer code '$CUSTOMER_CODE' is not valid" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        echo "Valid customers: $VALID_CUSTOMERS" | tee -a "$LOG_FILE"
        exit 1
    fi
else
    echo "❌ FAIL: CustomerCodes.json not found" | tee -a "$LOG_FILE"
    exit 1
fi
echo "" | tee -a "$LOG_FILE"

# Test 2: Dry-run validation
echo "=== Test 2: Dry-Run Validation ===" | tee -a "$LOG_FILE"
test_customer_operation "Dry-run update for $CUSTOMER_CODE" "$APP_BINARY -mode=update -customer=$CUSTOMER_CODE -dry-run" "true"

# Test 3: Validation mode for specific customer
echo "=== Test 3: Customer-Specific Validation ===" | tee -a "$LOG_FILE"
test_customer_operation "Validation mode for $CUSTOMER_CODE" "$APP_BINARY -mode=validate -customer=$CUSTOMER_CODE" "true"

# Test 4: Different log levels
echo "=== Test 4: Log Level Testing ===" | tee -a "$LOG_FILE"
for log_level in "debug" "info" "warn" "error"; do
    test_customer_operation "Dry-run with $log_level logging" "$APP_BINARY -mode=update -customer=$CUSTOMER_CODE -dry-run -log-level=$log_level" "true"
done

# Test 5: Configuration file validation for customer
echo "=== Test 5: Customer Configuration Validation ===" | tee -a "$LOG_FILE"

# Check if customer has specific SQS queue configured
if [[ -f "$PROJECT_ROOT/config.json" ]]; then
    CUSTOMER_QUEUE=$(jq -r ".customer_mappings[\"$CUSTOMER_CODE\"].sqs_queue_arn" "$PROJECT_ROOT/config.json" 2>/dev/null)
    if [[ "$CUSTOMER_QUEUE" != "null" && -n "$CUSTOMER_QUEUE" ]]; then
        echo "✅ PASS: Customer has SQS queue configured: $CUSTOMER_QUEUE" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
        
        # Test SQS queue access
        QUEUE_URL=$(echo "$CUSTOMER_QUEUE" | sed 's/arn:aws:sqs:/https:\/\/sqs./' | sed 's/:/\//' | sed 's/:/\//')
        test_customer_operation "SQS queue access for $CUSTOMER_CODE" "aws sqs get-queue-attributes --queue-url '$QUEUE_URL' --attribute-names QueueArn" "true"
    else
        echo "⚠️  WARNING: No SQS queue configured for customer $CUSTOMER_CODE" | tee -a "$LOG_FILE"
    fi
fi

# Test 6: S3 prefix validation
echo "=== Test 6: S3 Prefix Validation ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/config.json" ]]; then
    BUCKET_NAME=$(jq -r '.s3_config.bucket_name' "$PROJECT_ROOT/config.json" 2>/dev/null)
    CUSTOMER_PREFIX="customers/$CUSTOMER_CODE/"
    
    if [[ "$BUCKET_NAME" != "null" && "$CUSTOMER_PREFIX" != "null" && -n "$BUCKET_NAME" && -n "$CUSTOMER_PREFIX" ]]; then
        test_customer_operation "S3 customer prefix access" "aws s3 ls s3://$BUCKET_NAME/$CUSTOMER_PREFIX" "true"
        
        # Test uploading to customer prefix
        TEST_FILE="$RESULTS_DIR/test-$CUSTOMER_CODE-$TIMESTAMP.json"
        echo "{\"customer\": \"$CUSTOMER_CODE\", \"test\": true, \"timestamp\": \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\"}" > "$TEST_FILE"
        
        test_customer_operation "S3 upload to customer prefix" "aws s3 cp '$TEST_FILE' s3://$BUCKET_NAME/$CUSTOMER_PREFIX" "true"
        
        # Clean up
        aws s3 rm "s3://$BUCKET_NAME/${CUSTOMER_PREFIX}$(basename "$TEST_FILE")" >/dev/null 2>&1 || true
        rm -f "$TEST_FILE"
    else
        echo "⚠️  WARNING: S3 configuration incomplete for customer $CUSTOMER_CODE" | tee -a "$LOG_FILE"
    fi
fi

# Test 7: SES integration for customer
echo "=== Test 7: SES Integration ===" | tee -a "$LOG_FILE"
test_customer_operation "SES describe-list" "$APP_BINARY ses -action describe-list" "true"

# Test 8: Performance test for customer
echo "=== Test 8: Customer Performance Test ===" | tee -a "$LOG_FILE"
echo "Running performance test with 3 rapid dry-runs for $CUSTOMER_CODE..." | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
START_TIME=$(date +%s)
for i in {1..3}; do
    "$APP_BINARY" -mode=update -customer="$CUSTOMER_CODE" -dry-run >/dev/null 2>&1
done
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
if [[ $DURATION -lt 15 ]]; then
    echo "✅ PASS: Performance test for $CUSTOMER_CODE (3 dry-runs in ${DURATION}s)" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: Performance test too slow for $CUSTOMER_CODE (3 dry-runs in ${DURATION}s)" | tee -a "$LOG_FILE"
fi

# Test 9: Error handling for customer
echo "=== Test 9: Error Handling ===" | tee -a "$LOG_FILE"
# Test with invalid config
TEMP_CONFIG="$RESULTS_DIR/invalid-config-$CUSTOMER_CODE.json"
echo '{"invalid": "json"' > "$TEMP_CONFIG"
test_customer_operation "Invalid config handling" "$APP_BINARY -mode=update -customer=$CUSTOMER_CODE -config=$TEMP_CONFIG -dry-run" "false"
rm -f "$TEMP_CONFIG"

# Test 10: Memory usage for customer operations
echo "=== Test 10: Memory Usage Test ===" | tee -a "$LOG_FILE"
if command -v ps >/dev/null 2>&1; then
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    # Start dry-run in background
    "$APP_BINARY" -mode=update -customer="$CUSTOMER_CODE" -dry-run &
    APP_PID=$!
    
    # Give it a moment to start
    sleep 1
    
    # Check memory usage
    if kill -0 $APP_PID 2>/dev/null; then
        MEMORY_KB=$(ps -o rss= -p $APP_PID 2>/dev/null || echo "0")
        MEMORY_MB=$((MEMORY_KB / 1024))
        
        # Wait for process to complete
        wait $APP_PID
        APP_RESULT=$?
        
        if [[ $APP_RESULT -eq 0 && $MEMORY_MB -lt 50 ]]; then
            echo "✅ PASS: Memory usage for $CUSTOMER_CODE (${MEMORY_MB}MB)" | tee -a "$LOG_FILE"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "❌ FAIL: Memory usage for $CUSTOMER_CODE (${MEMORY_MB}MB, exit: $APP_RESULT)" | tee -a "$LOG_FILE"
        fi
    else
        echo "❌ FAIL: Process died unexpectedly for $CUSTOMER_CODE" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: ps command not available" | tee -a "$LOG_FILE"
fi

# Generate customer-specific report
REPORT_FILE="$RESULTS_DIR/customer-report-$CUSTOMER_CODE-$TIMESTAMP.md"
cat > "$REPORT_FILE" << EOF
# Customer-Specific Test Report: $CUSTOMER_CODE

**Test Run:** $(date)
**Customer Code:** $CUSTOMER_CODE
**Customer Name:** $(jq -r ".customerMapping[\"$CUSTOMER_CODE\"]" "$PROJECT_ROOT/CustomerCodes.json" 2>/dev/null || echo "Unknown")

## Summary

- **Total Tests:** $TOTAL_TESTS
- **Passed:** $PASSED_TESTS
- **Failed:** $((TOTAL_TESTS - PASSED_TESTS))
- **Success Rate:** $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Results

### Configuration
- Customer code validation: $(if echo "$VALID_CUSTOMERS" | grep -q "^$CUSTOMER_CODE$"; then echo "✅ PASS"; else echo "❌ FAIL"; fi)
- SQS queue configuration: $(if [[ -n "$CUSTOMER_QUEUE" ]]; then echo "✅ Configured"; else echo "⚠️ Not configured"; fi)
- S3 prefix configuration: $(if [[ -n "$CUSTOMER_PREFIX" ]]; then echo "✅ Configured"; else echo "⚠️ Not configured"; fi)

### Functionality
- Dry-run operations: $(if grep -q "✅ PASS.*Dry-run update for $CUSTOMER_CODE" "$LOG_FILE"; then echo "✅ PASS"; else echo "❌ FAIL"; fi)
- Validation mode: $(if grep -q "✅ PASS.*Validation mode for $CUSTOMER_CODE" "$LOG_FILE"; then echo "✅ PASS"; else echo "❌ FAIL"; fi)
- Error handling: $(if grep -q "✅ PASS.*Invalid config handling" "$LOG_FILE"; then echo "✅ PASS"; else echo "❌ FAIL"; fi)

### Performance
- Response time: $(if grep -q "✅ PASS.*Performance test for $CUSTOMER_CODE" "$LOG_FILE"; then echo "✅ Good"; else echo "⚠️ Slow"; fi)
- Memory usage: $(if grep -q "✅ PASS.*Memory usage for $CUSTOMER_CODE" "$LOG_FILE"; then echo "✅ Efficient"; else echo "⚠️ High"; fi)

## Detailed Log

See: \`$(basename "$LOG_FILE")\`

## Recommendations

EOF

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "✅ **Customer $CUSTOMER_CODE is fully operational** and ready for production use." >> "$REPORT_FILE"
else
    echo "⚠️ **Customer $CUSTOMER_CODE has issues** that need to be addressed before production use." >> "$REPORT_FILE"
fi

# Summary
echo "" | tee -a "$LOG_FILE"
echo "=== Customer Test Summary ===" | tee -a "$LOG_FILE"
echo "Customer: $CUSTOMER_CODE" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"
echo "Report generated: $REPORT_FILE" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 All tests passed for customer $CUSTOMER_CODE!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some tests failed for customer $CUSTOMER_CODE. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi