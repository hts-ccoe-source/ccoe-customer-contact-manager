#!/bin/bash

# AWS Alternate Contact Manager - Integration Tests
# End-to-end integration testing with real AWS services

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/integration-test-$TIMESTAMP.log"

echo "=== AWS Alternate Contact Manager - Integration Tests ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

# Build the application if not already built
cd "$PROJECT_ROOT"
APP_BINARY="$RESULTS_DIR/aws-alternate-contact-manager-test"
if [[ ! -f "$APP_BINARY" ]]; then
    echo "Building application..." | tee -a "$LOG_FILE"
    if go build -o "$APP_BINARY" .; then
        echo "✅ Application built successfully" | tee -a "$LOG_FILE"
    else
        echo "❌ Failed to build application" | tee -a "$LOG_FILE"
        exit 1
    fi
fi

# Function to test integration scenario
test_integration() {
    local description="$1"
    local command="$2"
    local expect_success="$3"  # true/false
    
    echo "Testing $description..." | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo "Command: $command" >> "$LOG_FILE"
    echo "Expected success: $expect_success" >> "$LOG_FILE"
    
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

# Test 1: End-to-end validation workflow
echo "=== Test 1: End-to-End Validation Workflow ===" | tee -a "$LOG_FILE"
test_integration "Complete validation workflow" "$APP_BINARY -mode=validate -log-level=debug" "true"

# Test 2: Multi-customer dry-run workflow
echo "=== Test 2: Multi-Customer Dry-Run Workflow ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/config.json" ]]; then
    # Get all customers from the consolidated config
    CUSTOMERS=$(jq -r '.customer_mappings | keys[]' "$PROJECT_ROOT/config.json" 2>/dev/null)
    if [[ -n "$CUSTOMERS" ]]; then
        CUSTOMER_COUNT=0
        SUCCESS_COUNT=0
        
        while IFS= read -r customer; do
            if [[ -n "$customer" ]]; then
                CUSTOMER_COUNT=$((CUSTOMER_COUNT + 1))
                echo "Testing customer: $customer" | tee -a "$LOG_FILE"
                
                if "$APP_BINARY" -mode=update -customer="$customer" -dry-run >> "$LOG_FILE" 2>&1; then
                    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
                    echo "  ✅ Success for $customer" | tee -a "$LOG_FILE"
                else
                    echo "  ❌ Failed for $customer" | tee -a "$LOG_FILE"
                fi
            fi
        done <<< "$CUSTOMERS"
        
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        if [[ $SUCCESS_COUNT -eq $CUSTOMER_COUNT ]]; then
            echo "✅ PASS: All $CUSTOMER_COUNT customers validated successfully" | tee -a "$LOG_FILE"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "❌ FAIL: Only $SUCCESS_COUNT/$CUSTOMER_COUNT customers validated successfully" | tee -a "$LOG_FILE"
        fi
    else
        echo "⚠️  SKIP: No customers found in config.json" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: config.json not found" | tee -a "$LOG_FILE"
fi

# Test 3: SES Integration Test
echo "=== Test 3: SES Integration Test ===" | tee -a "$LOG_FILE"
test_integration "SES describe-list operation" "$APP_BINARY ses -action describe-list" "true"

# Test 4: Organizations Integration Test
echo "=== Test 4: Organizations Integration Test ===" | tee -a "$LOG_FILE"
# This will test if we can access AWS Organizations
test_integration "Organizations access validation" "aws organizations describe-organization" "true"

# Test 5: S3 Integration Test (if bucket configured)
echo "=== Test 5: S3 Integration Test ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/config.json" ]]; then
    BUCKET_NAME=$(jq -r '.s3_config.bucket_name' "$PROJECT_ROOT/config.json" 2>/dev/null)
    if [[ "$BUCKET_NAME" != "null" && -n "$BUCKET_NAME" ]]; then
        # Test S3 bucket access
        test_integration "S3 bucket access test" "aws s3 ls s3://$BUCKET_NAME/" "true"
        
        # Test S3 upload (create a test file)
        TEST_FILE="$RESULTS_DIR/test-upload-$TIMESTAMP.json"
        echo '{"test": "integration", "timestamp": "'$(date -Iseconds)'"}' > "$TEST_FILE"
        
        test_integration "S3 upload test" "aws s3 cp '$TEST_FILE' s3://$BUCKET_NAME/test/" "true"
        
        # Test S3 download
        DOWNLOAD_FILE="$RESULTS_DIR/test-download-$TIMESTAMP.json"
        test_integration "S3 download test" "aws s3 cp s3://$BUCKET_NAME/test/test-upload-$TIMESTAMP.json '$DOWNLOAD_FILE'" "true"
        
        # Verify file content
        if [[ -f "$DOWNLOAD_FILE" ]]; then
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
            if diff "$TEST_FILE" "$DOWNLOAD_FILE" >/dev/null 2>&1; then
                echo "✅ PASS: S3 upload/download content verification" | tee -a "$LOG_FILE"
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                echo "❌ FAIL: S3 upload/download content mismatch" | tee -a "$LOG_FILE"
            fi
        fi
        
        # Clean up test files
        aws s3 rm "s3://$BUCKET_NAME/test/test-upload-$TIMESTAMP.json" >/dev/null 2>&1 || true
        rm -f "$TEST_FILE" "$DOWNLOAD_FILE"
    else
        echo "⚠️  SKIP: No S3 bucket configured" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: config.json not found" | tee -a "$LOG_FILE"
fi

# Test 6: SQS Integration Test (if queues configured)
echo "=== Test 6: SQS Integration Test ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/config.json" ]]; then
    # Get first SQS queue from customer mappings
    FIRST_QUEUE_ARN=$(jq -r '.customer_mappings | to_entries[0].value.sqs_queue_arn' "$PROJECT_ROOT/config.json" 2>/dev/null)
    if [[ "$FIRST_QUEUE_ARN" != "null" && -n "$FIRST_QUEUE_ARN" ]]; then
        # Convert ARN to URL
        QUEUE_URL=$(echo "$FIRST_QUEUE_ARN" | sed 's/arn:aws:sqs:/https:\/\/sqs./' | sed 's/:/\//' | sed 's/:/\//')
        
        test_integration "SQS queue attributes test" "aws sqs get-queue-attributes --queue-url '$QUEUE_URL' --attribute-names QueueArn" "true"
        
        # Test sending a message
        TEST_MESSAGE='{"test": "integration", "timestamp": "'$(date -Iseconds)'"}'
        test_integration "SQS send message test" "aws sqs send-message --queue-url '$QUEUE_URL' --message-body '$TEST_MESSAGE'" "true"
        
        # Test receiving messages (should get our test message)
        test_integration "SQS receive message test" "aws sqs receive-message --queue-url '$QUEUE_URL' --max-number-of-messages 1" "true"
    else
        echo "⚠️  SKIP: No SQS queues configured" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: config.json not found" | tee -a "$LOG_FILE"
fi

# Test 7: Configuration Integration Test
echo "=== Test 7: Configuration Integration Test ===" | tee -a "$LOG_FILE"
# Test loading all configurations together
TOTAL_TESTS=$((TOTAL_TESTS + 1))
CONFIG_ERRORS=0

# Check each config file
for config_file in "config.json" "OrgConfig.json" "SESConfig.json" "SubscriptionConfig.json"; do
    if [[ -f "$PROJECT_ROOT/$config_file" ]]; then
        if ! jq empty "$PROJECT_ROOT/$config_file" 2>/dev/null; then
            echo "❌ Configuration error in $config_file" | tee -a "$LOG_FILE"
            CONFIG_ERRORS=$((CONFIG_ERRORS + 1))
        fi
    fi
done

if [[ $CONFIG_ERRORS -eq 0 ]]; then
    echo "✅ PASS: All configuration files are valid JSON" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: $CONFIG_ERRORS configuration files have errors" | tee -a "$LOG_FILE"
fi

# Test 8: Performance Integration Test
echo "=== Test 8: Performance Integration Test ===" | tee -a "$LOG_FILE"
echo "Running performance test with concurrent operations..." | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))

# Create a function to run validation in background
run_validation() {
    "$APP_BINARY" -mode=validate >/dev/null 2>&1
    return $?
}

# Run 3 concurrent validations
START_TIME=$(date +%s)
run_validation &
PID1=$!
run_validation &
PID2=$!
run_validation &
PID3=$!

# Wait for all to complete
wait $PID1
RESULT1=$?
wait $PID2
RESULT2=$?
wait $PID3
RESULT3=$?

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

if [[ $RESULT1 -eq 0 && $RESULT2 -eq 0 && $RESULT3 -eq 0 && $DURATION -lt 30 ]]; then
    echo "✅ PASS: Concurrent performance test (3 validations in ${DURATION}s)" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: Concurrent performance test failed (results: $RESULT1,$RESULT2,$RESULT3, time: ${DURATION}s)" | tee -a "$LOG_FILE"
fi

# Test 9: Error Recovery Test
echo "=== Test 9: Error Recovery Test ===" | tee -a "$LOG_FILE"
# Test how the application handles various error conditions
test_integration "Invalid customer code handling" "$APP_BINARY -mode=update -customer=definitely-invalid-customer-code-12345 -dry-run" "false"
test_integration "Missing config file handling" "$APP_BINARY -mode=validate -config=/nonexistent/path/config.json" "false"

# Test 10: Memory and Resource Test
echo "=== Test 10: Memory and Resource Test ===" | tee -a "$LOG_FILE"
echo "Running memory usage test..." | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))

# Run the application and monitor memory usage
if command -v ps >/dev/null 2>&1; then
    # Start the application in background for a validation
    "$APP_BINARY" -mode=validate &
    APP_PID=$!
    
    # Give it a moment to start
    sleep 2
    
    # Check if process is still running and get memory usage
    if kill -0 $APP_PID 2>/dev/null; then
        MEMORY_KB=$(ps -o rss= -p $APP_PID 2>/dev/null || echo "0")
        MEMORY_MB=$((MEMORY_KB / 1024))
        
        # Wait for process to complete
        wait $APP_PID
        APP_RESULT=$?
        
        if [[ $APP_RESULT -eq 0 && $MEMORY_MB -lt 100 ]]; then
            echo "✅ PASS: Memory usage test (${MEMORY_MB}MB, exit code: $APP_RESULT)" | tee -a "$LOG_FILE"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "❌ FAIL: Memory usage test (${MEMORY_MB}MB, exit code: $APP_RESULT)" | tee -a "$LOG_FILE"
        fi
    else
        echo "❌ FAIL: Application process died unexpectedly" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: ps command not available for memory testing" | tee -a "$LOG_FILE"
fi

# Summary
echo "" | tee -a "$LOG_FILE"
echo "=== Integration Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"

# Generate integration test report
REPORT_FILE="$RESULTS_DIR/integration-test-report-$TIMESTAMP.md"
cat > "$REPORT_FILE" << EOF
# AWS Alternate Contact Manager - Integration Test Report

**Test Run:** $(date)
**Duration:** Started at $(head -2 "$LOG_FILE" | tail -1 | cut -d: -f2-)

## Summary

- **Total Tests:** $TOTAL_TESTS
- **Passed:** $PASSED_TESTS
- **Failed:** $((TOTAL_TESTS - PASSED_TESTS))
- **Success Rate:** $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Categories

1. **End-to-End Validation** - Complete workflow validation
2. **Multi-Customer Operations** - Testing all configured customers
3. **AWS Service Integration** - SES, S3, SQS, Organizations
4. **Configuration Management** - All config files validation
5. **Performance Testing** - Concurrent operations and memory usage
6. **Error Handling** - Recovery from various error conditions

## Detailed Results

See full log: \`$(basename "$LOG_FILE")\`

## Recommendations

EOF

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "✅ **All tests passed!** The application is ready for production use." >> "$REPORT_FILE"
    echo "🎉 All integration tests passed!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️ **Some tests failed.** Review the detailed log and address issues before production deployment." >> "$REPORT_FILE"
    echo "⚠️  Some integration tests failed. Check the log and report for details." | tee -a "$LOG_FILE"
    exit 1
fi