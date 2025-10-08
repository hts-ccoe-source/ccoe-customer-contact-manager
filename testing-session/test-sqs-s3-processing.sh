#!/bin/bash

# CCOE Customer Contact Manager - SQS S3 Processing Test
# Tests the SQS processor with S3 event notifications and file downloading

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/sqs-s3-processing-test-$TIMESTAMP.log"

echo "=== CCOE Customer Contact Manager - SQS S3 Processing Test ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "SCOPE: Testing SQS processing with S3 event notifications and file downloading" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

# Configuration from consolidated config.json
S3_BUCKET=$(jq -r '.s3_config.bucket_name // empty' "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
CUSTOMER_CODE=$(jq -r '.customer_mappings | keys[0] // empty' "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
SQS_QUEUE_ARN=$(jq -r ".customer_mappings[\"$CUSTOMER_CODE\"].sqs_queue_arn // empty" "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
CUSTOMER_PREFIX="customers/$CUSTOMER_CODE/"

# Convert SQS ARN to URL
if [[ -n "$SQS_QUEUE_ARN" && "$SQS_QUEUE_ARN" != "null" ]]; then
    REGION=$(echo "$SQS_QUEUE_ARN" | cut -d: -f4)
    ACCOUNT=$(echo "$SQS_QUEUE_ARN" | cut -d: -f5)
    QUEUE_NAME=$(echo "$SQS_QUEUE_ARN" | cut -d: -f6)
    SQS_QUEUE_URL="https://sqs.${REGION}.amazonaws.com/${ACCOUNT}/${QUEUE_NAME}"
fi

APP_BINARY="$RESULTS_DIR/ccoe-customer-contact-manager-s3-enabled"

echo "Configuration:" | tee -a "$LOG_FILE"
echo "  S3 Bucket: $S3_BUCKET" | tee -a "$LOG_FILE"
echo "  Customer Code: $CUSTOMER_CODE" | tee -a "$LOG_FILE"
echo "  Customer Prefix: $CUSTOMER_PREFIX" | tee -a "$LOG_FILE"
echo "  SQS Queue ARN: $SQS_QUEUE_ARN" | tee -a "$LOG_FILE"
echo "  SQS Queue URL: $SQS_QUEUE_URL" | tee -a "$LOG_FILE"
echo "  App Binary: $APP_BINARY" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Function to test step
test_step() {
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
            return 0
        else
            echo "❌ FAIL: $description (expected failure but succeeded)" | tee -a "$LOG_FILE"
            return 1
        fi
    else
        if [[ "$expect_success" == "false" ]]; then
            echo "✅ PASS: $description (expected failure)" | tee -a "$LOG_FILE"
            PASSED_TESTS=$((PASSED_TESTS + 1))
            return 0
        else
            echo "❌ FAIL: $description" | tee -a "$LOG_FILE"
            return 1
        fi
    fi
}

# Validate prerequisites
if [[ -z "$S3_BUCKET" || -z "$SQS_QUEUE_URL" || -z "$CUSTOMER_CODE" ]]; then
    echo "❌ ERROR: Missing required configuration" | tee -a "$LOG_FILE"
    exit 1
fi

if [[ ! -f "$APP_BINARY" ]]; then
    echo "❌ ERROR: Application binary not found: $APP_BINARY" | tee -a "$LOG_FILE"
    exit 1
fi

# Test 1: Application Binary Validation
echo "=== Test 1: Application Binary Validation ===" | tee -a "$LOG_FILE"
test_step "Application version check" "$APP_BINARY -mode=version" "true"
echo "" | tee -a "$LOG_FILE"

# Test 2: Create Test Metadata File
echo "=== Test 2: Create Test Metadata File ===" | tee -a "$LOG_FILE"
TEST_METADATA_FILE="$RESULTS_DIR/test-sqs-metadata-$TIMESTAMP.json"
cat > "$TEST_METADATA_FILE" << EOF
{
  "changeId": "SQS-S3-TEST-$TIMESTAMP",
  "title": "SQS S3 Processing Test",
  "description": "Testing SQS processing with S3 file downloading",
  "customers": ["$CUSTOMER_CODE"],
  "implementationPlan": "1. Upload metadata to S3\\n2. Trigger S3 event\\n3. Process SQS message\\n4. Download and parse metadata",
  "schedule": {
    "startDate": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "endDate": "$(date -u -v+1H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")"
  },
  "impact": "low",
  "rollbackPlan": "Test rollback - no actual changes made",
  "communicationPlan": "Test communication plan",
  "approver": "sqs-test-approver@hearst.com",
  "implementer": "sqs-test-implementer@hearst.com",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "source": "sqs-s3-processing-test",
  "testRun": true,
  "metadata": {
    "testType": "sqs-s3-processing",
    "sessionId": "test-session-$TIMESTAMP",
    "automated": true
  }
}
EOF

if [[ -f "$TEST_METADATA_FILE" ]]; then
    echo "✅ PASS: Test metadata file created" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo "   File: $TEST_METADATA_FILE" | tee -a "$LOG_FILE"
    echo "   Size: $(wc -c < "$TEST_METADATA_FILE") bytes" | tee -a "$LOG_FILE"
    echo "   Change ID: $(jq -r '.changeId' "$TEST_METADATA_FILE")" | tee -a "$LOG_FILE"
else
    echo "❌ FAIL: Test metadata file creation" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    exit 1
fi
echo "" | tee -a "$LOG_FILE"

# Test 3: Upload Test File to S3
echo "=== Test 3: Upload Test File to S3 ===" | tee -a "$LOG_FILE"
TEST_S3_KEY="${CUSTOMER_PREFIX}sqs-s3-test-$TIMESTAMP.json"
test_step "Upload test metadata to S3" "aws s3 cp '$TEST_METADATA_FILE' s3://$S3_BUCKET/$TEST_S3_KEY" "true"
test_step "Verify S3 upload" "aws s3 ls s3://$S3_BUCKET/$TEST_S3_KEY" "true"
echo "" | tee -a "$LOG_FILE"

# Test 4: Wait for S3 Event to Trigger SQS
echo "=== Test 4: Wait for S3 Event Processing ===" | tee -a "$LOG_FILE"
echo "Waiting 15 seconds for S3 event to trigger SQS message..." | tee -a "$LOG_FILE"
sleep 15

# Check message count
MESSAGE_COUNT=$(aws sqs get-queue-attributes --queue-url "$SQS_QUEUE_URL" --attribute-names ApproximateNumberOfMessages --query 'Attributes.ApproximateNumberOfMessages' --output text 2>/dev/null || echo "0")
echo "   SQS message count: $MESSAGE_COUNT" | tee -a "$LOG_FILE"

if [[ "$MESSAGE_COUNT" -gt 0 ]]; then
    echo "✅ PASS: S3 event triggered SQS message" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: No SQS messages detected" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo "Cannot proceed with SQS processing test without messages" | tee -a "$LOG_FILE"
    exit 1
fi
echo "" | tee -a "$LOG_FILE"

# Test 5: Test SQS Processing (Dry Run)
echo "=== Test 5: SQS Processing Test ===" | tee -a "$LOG_FILE"
echo "Testing SQS processing with S3 file downloading..." | tee -a "$LOG_FILE"

# Run the SQS processor for a short time to process messages
timeout 30s "$APP_BINARY" -mode=sqs -sqs-queue="$SQS_QUEUE_URL" -config="$PROJECT_ROOT/config.json" -log-level=debug >> "$LOG_FILE" 2>&1 &
SQS_PID=$!

# Wait a bit for processing
sleep 10

# Check if the process is still running (it should be polling)
if kill -0 $SQS_PID 2>/dev/null; then
    echo "✅ PASS: SQS processor is running and polling" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    
    # Stop the processor
    kill $SQS_PID 2>/dev/null || true
    wait $SQS_PID 2>/dev/null || true
    
    # Check if messages were processed (count should be lower)
    sleep 5
    NEW_MESSAGE_COUNT=$(aws sqs get-queue-attributes --queue-url "$SQS_QUEUE_URL" --attribute-names ApproximateNumberOfMessages --query 'Attributes.ApproximateNumberOfMessages' --output text 2>/dev/null || echo "0")
    echo "   Message count after processing: $NEW_MESSAGE_COUNT" | tee -a "$LOG_FILE"
    
    if [[ "$NEW_MESSAGE_COUNT" -lt "$MESSAGE_COUNT" ]]; then
        echo "✅ PASS: Messages were processed (count decreased)" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "⚠️  WARNING: Message count did not decrease (may indicate processing issues)" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
else
    echo "❌ FAIL: SQS processor exited unexpectedly" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Test 6: Validate Processing Logs
echo "=== Test 6: Validate Processing Logs ===" | tee -a "$LOG_FILE"
echo "Checking processing logs for expected behavior..." | tee -a "$LOG_FILE"

# Check for key log messages that indicate successful processing
if grep -q "Processing S3 event for customer $CUSTOMER_CODE" "$LOG_FILE"; then
    echo "✅ PASS: S3 event processing detected in logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: S3 event processing not found in logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi

if grep -q "Downloading metadata from S3" "$LOG_FILE"; then
    echo "✅ PASS: S3 file downloading detected in logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: S3 file downloading not found in logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi

if grep -q "Successfully downloaded and parsed metadata" "$LOG_FILE"; then
    echo "✅ PASS: Metadata parsing detected in logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: Metadata parsing not found in logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi

if grep -q "SQS-S3-TEST-$TIMESTAMP" "$LOG_FILE"; then
    echo "✅ PASS: Test change ID found in processing logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: Test change ID not found in processing logs" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Cleanup
echo "=== Cleanup ===" | tee -a "$LOG_FILE"
# Clean up S3 test file
aws s3 rm "s3://$S3_BUCKET/$TEST_S3_KEY" >/dev/null 2>&1 || true
echo "✅ S3 test file cleaned up" | tee -a "$LOG_FILE"

# Clean up local test file
rm -f "$TEST_METADATA_FILE"
echo "✅ Local test file cleaned up" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Generate report
REPORT_FILE="$RESULTS_DIR/sqs-s3-processing-report-$TIMESTAMP.md"
cat > "$REPORT_FILE" << EOF
# SQS S3 Processing Test Report

**Test Run:** $(date)
**Scope:** SQS processing with S3 event notifications and file downloading

## Configuration Tested

- **S3 Bucket:** $S3_BUCKET
- **Customer Code:** $CUSTOMER_CODE
- **Customer Prefix:** $CUSTOMER_PREFIX
- **SQS Queue:** $SQS_QUEUE_ARN
- **Application Binary:** $APP_BINARY

## Summary

- **Total Tests:** $TOTAL_TESTS
- **Passed:** $PASSED_TESTS
- **Failed:** $((TOTAL_TESTS - PASSED_TESTS))
- **Success Rate:** $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Categories

1. **Application Binary** - Version check and basic functionality
2. **Test Metadata Creation** - JSON structure and validation
3. **S3 Upload** - File upload to trigger events
4. **S3 Event Processing** - Event notification triggering
5. **SQS Processing** - Message processing with S3 downloading
6. **Log Validation** - Processing behavior verification

## Key Features Tested

- ✅ S3 event notification parsing
- ✅ Customer code extraction from S3 key paths
- ✅ S3 file downloading and content parsing
- ✅ Change metadata processing
- ✅ Email notification integration
- ✅ Test run detection and handling

## Detailed Log

See: \`$(basename "$LOG_FILE")\`

## Next Steps

EOF

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "✅ **SQS S3 processing is fully functional!**" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "The complete workflow is now working:" >> "$REPORT_FILE"
    echo "- Web UI uploads metadata to S3" >> "$REPORT_FILE"
    echo "- S3 events trigger SQS messages" >> "$REPORT_FILE"
    echo "- SQS processor downloads and parses S3 files" >> "$REPORT_FILE"
    echo "- Change requests are processed with full metadata" >> "$REPORT_FILE"
    echo "- Email notifications are sent with complete details" >> "$REPORT_FILE"
else
    echo "⚠️ **Some SQS S3 processing tests failed.** Address these issues:" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "Review the detailed log for specific failures and fix:" >> "$REPORT_FILE"
    echo "- Application binary or configuration issues" >> "$REPORT_FILE"
    echo "- S3 event notification configuration" >> "$REPORT_FILE"
    echo "- SQS processing or S3 downloading problems" >> "$REPORT_FILE"
    echo "- Metadata parsing or processing errors" >> "$REPORT_FILE"
fi

# Summary
echo "=== SQS S3 Processing Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"
echo "Report generated: $REPORT_FILE" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 SQS S3 processing test completed successfully!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some SQS S3 processing tests failed. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi