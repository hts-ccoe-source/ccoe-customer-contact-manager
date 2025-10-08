#!/bin/bash

# CCOE Customer Contact Manager - Application Mode Tests
# Tests different application modes and functionality

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/app-modes-test-$TIMESTAMP.log"

echo "=== CCOE Customer Contact Manager - Application Mode Tests ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

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

# Function to test application mode
test_app_mode() {
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

# Test 1: Version mode
echo "=== Test 1: Version Mode ===" | tee -a "$LOG_FILE"
test_app_mode "Version command" "$APP_BINARY -mode=version" "true"

# Test 2: Help mode
echo "=== Test 2: Help Mode ===" | tee -a "$LOG_FILE"
test_app_mode "Help command" "$APP_BINARY -mode=help" "true"

# Test 3: Validate mode
echo "=== Test 3: Validate Mode ===" | tee -a "$LOG_FILE"
test_app_mode "Validate mode" "$APP_BINARY -mode=validate" "true"

# Test 4: Update mode without customer (should fail)
echo "=== Test 4: Update Mode - No Customer ===" | tee -a "$LOG_FILE"
test_app_mode "Update mode without customer" "$APP_BINARY -mode=update" "false"

# Test 5: Update mode with invalid customer (should fail)
echo "=== Test 5: Update Mode - Invalid Customer ===" | tee -a "$LOG_FILE"
test_app_mode "Update mode with invalid customer" "$APP_BINARY -mode=update -customer=invalid-customer-code" "false"

# Test 6: Update mode with valid customer (dry-run)
echo "=== Test 6: Update Mode - Valid Customer Dry Run ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/CustomerCodes.json" ]]; then
    FIRST_CUSTOMER=$(jq -r '.validCustomers[0]' "$PROJECT_ROOT/CustomerCodes.json" 2>/dev/null)
    if [[ "$FIRST_CUSTOMER" != "null" && -n "$FIRST_CUSTOMER" ]]; then
        test_app_mode "Update mode with valid customer (dry-run)" "$APP_BINARY -mode=update -customer=$FIRST_CUSTOMER -dry-run" "true"
    else
        echo "⚠️  SKIP: No valid customers found in CustomerCodes.json" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: CustomerCodes.json not found" | tee -a "$LOG_FILE"
fi

# Test 7: SQS mode without queue URL (should fail)
echo "=== Test 7: SQS Mode - No Queue URL ===" | tee -a "$LOG_FILE"
test_app_mode "SQS mode without queue URL" "$APP_BINARY -mode=sqs" "false"

# Test 8: SQS mode with invalid queue URL (should fail)
echo "=== Test 8: SQS Mode - Invalid Queue URL ===" | tee -a "$LOG_FILE"
test_app_mode "SQS mode with invalid queue URL" "$APP_BINARY -mode=sqs -sqs-queue=invalid-queue-url" "false"

# Test 9: Unknown mode (should fail)
echo "=== Test 9: Unknown Mode ===" | tee -a "$LOG_FILE"
test_app_mode "Unknown mode" "$APP_BINARY -mode=unknown-mode" "false"

# Test 10: Configuration file validation
echo "=== Test 10: Configuration File Validation ===" | tee -a "$LOG_FILE"
# Create a temporary invalid config file
TEMP_CONFIG="$RESULTS_DIR/invalid-config.json"
echo '{"invalid": "json"' > "$TEMP_CONFIG"
test_app_mode "Invalid configuration file" "$APP_BINARY -mode=validate -config=$TEMP_CONFIG" "false"

# Test 11: Log level validation
echo "=== Test 11: Log Level Validation ===" | tee -a "$LOG_FILE"
test_app_mode "Debug log level" "$APP_BINARY -mode=version -log-level=debug" "true"
test_app_mode "Info log level" "$APP_BINARY -mode=version -log-level=info" "true"
test_app_mode "Warn log level" "$APP_BINARY -mode=version -log-level=warn" "true"
test_app_mode "Error log level" "$APP_BINARY -mode=version -log-level=error" "true"
test_app_mode "Invalid log level" "$APP_BINARY -mode=version -log-level=invalid" "true"  # Should default to info

# Test 12: Multiple customers dry-run test
echo "=== Test 12: Multiple Customers Dry Run ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/CustomerCodes.json" ]]; then
    # Get first 3 customers for testing
    CUSTOMERS=$(jq -r '.validCustomers[0:3][]' "$PROJECT_ROOT/CustomerCodes.json" 2>/dev/null)
    if [[ -n "$CUSTOMERS" ]]; then
        while IFS= read -r customer; do
            if [[ -n "$customer" ]]; then
                test_app_mode "Dry-run for customer: $customer" "$APP_BINARY -mode=update -customer=$customer -dry-run" "true"
            fi
        done <<< "$CUSTOMERS"
    else
        echo "⚠️  SKIP: No customers found in CustomerCodes.json" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: CustomerCodes.json not found" | tee -a "$LOG_FILE"
fi

# Test 13: SES command help
echo "=== Test 13: SES Command Help ===" | tee -a "$LOG_FILE"
test_app_mode "SES help command" "$APP_BINARY ses -action help" "true"

# Test 14: Customer validation
echo "=== Test 14: Customer Validation ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/sample-metadata.json" ]]; then
    test_app_mode "Customer validation with sample metadata" "$APP_BINARY validate-customers -json-metadata=$PROJECT_ROOT/sample-metadata.json" "true"
else
    echo "⚠️  SKIP: sample-metadata.json not found" | tee -a "$LOG_FILE"
fi

# Test 15: Performance test with multiple rapid calls
echo "=== Test 15: Performance Test ===" | tee -a "$LOG_FILE"
echo "Running performance test with 5 rapid version calls..." | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
START_TIME=$(date +%s)
for i in {1..5}; do
    "$APP_BINARY" -mode=version >/dev/null 2>&1
done
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
if [[ $DURATION -lt 10 ]]; then
    echo "✅ PASS: Performance test (5 calls in ${DURATION}s)" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: Performance test too slow (5 calls in ${DURATION}s)" | tee -a "$LOG_FILE"
fi

# Clean up temporary files
rm -f "$TEMP_CONFIG"

# Summary
echo "" | tee -a "$LOG_FILE"
echo "=== Application Mode Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 All application mode tests passed!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some application mode tests failed. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi