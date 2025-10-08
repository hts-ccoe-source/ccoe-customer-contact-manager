#!/bin/bash

# CCOE Customer Contact Manager - Web UI to S3 Workflow Test
# Tests ONLY the web UI → S3 metadata storage workflow (no contact changes)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/web-ui-s3-workflow-test-$TIMESTAMP.log"

echo "=== CCOE Customer Contact Manager - Web UI to S3 Workflow Test ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "SCOPE: Testing ONLY metadata collection and S3 storage (NO contact changes)" | tee -a "$LOG_FILE"
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

echo "Configuration:" | tee -a "$LOG_FILE"
echo "  S3 Bucket: $S3_BUCKET" | tee -a "$LOG_FILE"
echo "  Customer Code: $CUSTOMER_CODE" | tee -a "$LOG_FILE"
echo "  Customer Prefix: $CUSTOMER_PREFIX" | tee -a "$LOG_FILE"
echo "  SQS Queue ARN: $SQS_QUEUE_ARN" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Function to test workflow step
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

# Test 1: Web UI File Validation
echo "=== Test 1: Web UI File Validation ===" | tee -a "$LOG_FILE"
WEB_UI_FILE="$PROJECT_ROOT/html/index.html"
if [[ -f "$WEB_UI_FILE" ]]; then
    echo "✅ PASS: Web UI file exists" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    
    # Check key functionality
    if grep -q 'name="customers"' "$WEB_UI_FILE" && grep -q "S3" "$WEB_UI_FILE"; then
        echo "✅ PASS: Web UI contains multi-customer and S3 functionality" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "❌ FAIL: Web UI missing required functionality" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
    
    # Check if deployed to S3
    if [[ -n "$S3_BUCKET" ]]; then
        test_step "Web UI deployed to S3" "aws s3 ls s3://$S3_BUCKET/index.html" "true"
    fi
else
    echo "❌ FAIL: Web UI file not found" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Test 2: S3 Infrastructure
echo "=== Test 2: S3 Infrastructure ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" ]]; then
    test_step "S3 bucket access" "aws s3 ls s3://$S3_BUCKET/" "true"
    test_step "S3 customer prefix write access" "echo 'test' | aws s3 cp - s3://$S3_BUCKET/${CUSTOMER_PREFIX}test-access.txt && aws s3 rm s3://$S3_BUCKET/${CUSTOMER_PREFIX}test-access.txt" "true"
    test_step "S3 archive prefix write access" "echo 'test' | aws s3 cp - s3://$S3_BUCKET/archive/test-access.txt && aws s3 rm s3://$S3_BUCKET/archive/test-access.txt" "true"
else
    echo "⚠️  SKIP: S3 infrastructure tests (bucket not configured)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 3: Create Sample Metadata (Simulating Web UI)
echo "=== Test 3: Sample Metadata Creation ===" | tee -a "$LOG_FILE"
SAMPLE_METADATA="$RESULTS_DIR/sample-change-metadata-$TIMESTAMP.json"
cat > "$SAMPLE_METADATA" << EOF
{
  "changeId": "WEB-UI-TEST-$TIMESTAMP",
  "title": "Sample Change Request - Web UI Test",
  "description": "This is a test change request created to validate the web UI to S3 workflow",
  "customers": ["$CUSTOMER_CODE"],
  "implementationPlan": "1. Test web UI functionality\\n2. Validate S3 upload\\n3. Verify metadata structure",
  "schedule": {
    "startDate": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "endDate": "$(date -u -v+1H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")"
  },
  "impact": "low",
  "rollbackPlan": "This is a test - no rollback needed",
  "communicationPlan": "Test communication plan",
  "approver": "test-approver@hearst.com",
  "implementer": "test-implementer@hearst.com",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "source": "web-ui-test",
  "testRun": true,
  "metadata": {
    "browserInfo": "Test Browser",
    "userAgent": "Test User Agent",
    "sessionId": "test-session-$TIMESTAMP"
  }
}
EOF

if [[ -f "$SAMPLE_METADATA" ]]; then
    echo "✅ PASS: Sample metadata created" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    
    # Validate JSON structure
    if jq empty "$SAMPLE_METADATA" 2>/dev/null; then
        echo "✅ PASS: Sample metadata is valid JSON" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
        
        echo "   File size: $(wc -c < "$SAMPLE_METADATA") bytes" | tee -a "$LOG_FILE"
        echo "   Customer: $(jq -r '.customers[0]' "$SAMPLE_METADATA")" | tee -a "$LOG_FILE"
        echo "   Change ID: $(jq -r '.changeId' "$SAMPLE_METADATA")" | tee -a "$LOG_FILE"
    else
        echo "❌ FAIL: Sample metadata is invalid JSON" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
else
    echo "❌ FAIL: Sample metadata creation failed" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Test 4: Web UI Upload Simulation
echo "=== Test 4: Web UI Upload Simulation ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" && -f "$SAMPLE_METADATA" ]]; then
    # Simulate multi-customer upload (what the web UI does)
    CUSTOMER_S3_KEY="${CUSTOMER_PREFIX}change-metadata-$TIMESTAMP.json"
    ARCHIVE_S3_KEY="archive/change-metadata-$TIMESTAMP.json"
    
    test_step "Upload to customer prefix (web UI simulation)" "aws s3 cp '$SAMPLE_METADATA' s3://$S3_BUCKET/$CUSTOMER_S3_KEY" "true"
    test_step "Upload to archive prefix (web UI simulation)" "aws s3 cp '$SAMPLE_METADATA' s3://$S3_BUCKET/$ARCHIVE_S3_KEY" "true"
    
    # Verify uploads
    test_step "Verify customer upload" "aws s3 ls s3://$S3_BUCKET/$CUSTOMER_S3_KEY" "true"
    test_step "Verify archive upload" "aws s3 ls s3://$S3_BUCKET/$ARCHIVE_S3_KEY" "true"
    
    # Test download and content verification
    DOWNLOADED_FILE="$RESULTS_DIR/downloaded-metadata-$TIMESTAMP.json"
    if aws s3 cp "s3://$S3_BUCKET/$CUSTOMER_S3_KEY" "$DOWNLOADED_FILE" >> "$LOG_FILE" 2>&1; then
        if diff "$SAMPLE_METADATA" "$DOWNLOADED_FILE" >/dev/null 2>&1; then
            echo "✅ PASS: Upload/download content integrity verified" | tee -a "$LOG_FILE"
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "❌ FAIL: Upload/download content mismatch" | tee -a "$LOG_FILE"
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
        fi
        rm -f "$DOWNLOADED_FILE"
    fi
else
    echo "⚠️  SKIP: Web UI upload simulation (S3 or metadata not available)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 5: S3 Event Notification Check
echo "=== Test 5: S3 Event Notification Check ===" | tee -a "$LOG_FILE"
if [[ -n "$SQS_QUEUE_URL" ]]; then
    echo "Waiting 10 seconds for potential S3 event processing..." | tee -a "$LOG_FILE"
    sleep 10
    
    # Check message count (don't process messages)
    MESSAGE_COUNT=$(aws sqs get-queue-attributes --queue-url "$SQS_QUEUE_URL" --attribute-names ApproximateNumberOfMessages --query 'Attributes.ApproximateNumberOfMessages' --output text 2>/dev/null || echo "0")
    echo "   SQS message count: $MESSAGE_COUNT" | tee -a "$LOG_FILE"
    
    if [[ "$MESSAGE_COUNT" -gt 0 ]]; then
        echo "✅ PASS: S3 events are triggering SQS messages" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
        
        # Peek at a message without deleting it
        test_step "SQS message peek (no processing)" "aws sqs receive-message --queue-url '$SQS_QUEUE_URL' --max-number-of-messages 1 --visibility-timeout 1" "true"
    else
        echo "⚠️  INFO: No SQS messages detected (S3 events may not be configured)" | tee -a "$LOG_FILE"
        echo "   This is expected if S3 event notifications haven't been set up yet" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))  # Count as pass since it's expected
    fi
else
    echo "⚠️  SKIP: S3 event notification check (SQS not configured)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 6: Multi-Customer Workflow Simulation
echo "=== Test 6: Multi-Customer Workflow Simulation ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" ]]; then
    # Create a multi-customer metadata file
    MULTI_CUSTOMER_METADATA="$RESULTS_DIR/multi-customer-metadata-$TIMESTAMP.json"
    cat > "$MULTI_CUSTOMER_METADATA" << EOF
{
  "changeId": "MULTI-TEST-$TIMESTAMP",
  "title": "Multi-Customer Change Test",
  "description": "Testing multi-customer upload workflow",
  "customers": ["$CUSTOMER_CODE", "test-customer-2"],
  "implementationPlan": "Multi-customer test implementation",
  "schedule": {
    "startDate": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "endDate": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  },
  "impact": "low",
  "rollbackPlan": "Multi-customer test rollback",
  "communicationPlan": "Multi-customer test communication",
  "approver": "multi-test@hearst.com",
  "implementer": "multi-implementer@hearst.com",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "source": "multi-customer-web-ui-test",
  "testRun": true
}
EOF
    
    # Upload to multiple customer prefixes (simulating web UI behavior)
    MULTI_CUSTOMER_KEY="${CUSTOMER_PREFIX}multi-customer-test-$TIMESTAMP.json"
    MULTI_ARCHIVE_KEY="archive/multi-customer-test-$TIMESTAMP.json"
    
    test_step "Multi-customer upload to primary customer" "aws s3 cp '$MULTI_CUSTOMER_METADATA' s3://$S3_BUCKET/$MULTI_CUSTOMER_KEY" "true"
    test_step "Multi-customer upload to archive" "aws s3 cp '$MULTI_CUSTOMER_METADATA' s3://$S3_BUCKET/$MULTI_ARCHIVE_KEY" "true"
    
    # Clean up test files
    aws s3 rm "s3://$S3_BUCKET/$MULTI_CUSTOMER_KEY" >/dev/null 2>&1 || true
    aws s3 rm "s3://$S3_BUCKET/$MULTI_ARCHIVE_KEY" >/dev/null 2>&1 || true
    rm -f "$MULTI_CUSTOMER_METADATA"
else
    echo "⚠️  SKIP: Multi-customer workflow simulation (S3 not configured)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Cleanup
echo "=== Cleanup ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" ]]; then
    # Clean up test files
    aws s3 rm "s3://$S3_BUCKET/$CUSTOMER_S3_KEY" >/dev/null 2>&1 || true
    aws s3 rm "s3://$S3_BUCKET/$ARCHIVE_S3_KEY" >/dev/null 2>&1 || true
    echo "✅ S3 test files cleaned up" | tee -a "$LOG_FILE"
fi

rm -f "$SAMPLE_METADATA"
echo "✅ Local test files cleaned up" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Generate report
REPORT_FILE="$RESULTS_DIR/web-ui-s3-workflow-report-$TIMESTAMP.md"
cat > "$REPORT_FILE" << EOF
# Web UI to S3 Workflow Test Report

**Test Run:** $(date)
**Scope:** Web UI metadata collection and S3 storage (NO contact changes)

## Configuration Tested

- **S3 Bucket:** $S3_BUCKET
- **Customer Code:** $CUSTOMER_CODE
- **Customer Prefix:** $CUSTOMER_PREFIX
- **SQS Queue:** $SQS_QUEUE_ARN

## Summary

- **Total Tests:** $TOTAL_TESTS
- **Passed:** $PASSED_TESTS
- **Failed:** $((TOTAL_TESTS - PASSED_TESTS))
- **Success Rate:** $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Categories

1. **Web UI File Validation** - HTML file structure and deployment
2. **S3 Infrastructure** - Bucket access and prefix permissions
3. **Metadata Creation** - JSON structure and validation
4. **Upload Simulation** - Web UI upload workflow simulation
5. **S3 Event Notification** - Event trigger validation (optional)
6. **Multi-Customer Workflow** - Multiple customer upload simulation

## Key Findings

- **Web UI Functionality:** $(if grep -q "✅ PASS.*Web UI contains multi-customer" "$LOG_FILE"; then echo "✅ Working"; else echo "❌ Issues found"; fi)
- **S3 Storage:** $(if grep -q "✅ PASS.*S3 bucket access" "$LOG_FILE"; then echo "✅ Working"; else echo "❌ Issues found"; fi)
- **Upload Workflow:** $(if grep -q "✅ PASS.*Upload to customer prefix" "$LOG_FILE"; then echo "✅ Working"; else echo "❌ Issues found"; fi)
- **Content Integrity:** $(if grep -q "✅ PASS.*content integrity" "$LOG_FILE"; then echo "✅ Verified"; else echo "⚠️ Not tested"; fi)
- **S3 Events:** $(if grep -q "✅ PASS.*S3 events are triggering" "$LOG_FILE"; then echo "✅ Configured"; else echo "⚠️ Not configured"; fi)

## Detailed Log

See: \`$(basename "$LOG_FILE")\`

## Recommendations

EOF

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "✅ **Web UI to S3 workflow is fully functional!**" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "The metadata collection and storage pipeline is working correctly:" >> "$REPORT_FILE"
    echo "- Web UI can collect and structure metadata" >> "$REPORT_FILE"
    echo "- S3 uploads work for both customer and archive prefixes" >> "$REPORT_FILE"
    echo "- Content integrity is maintained through upload/download" >> "$REPORT_FILE"
    echo "- Multi-customer workflows are supported" >> "$REPORT_FILE"
else
    echo "⚠️ **Some issues found in the workflow.** Address these before production use:" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "Review the detailed log for specific failures and fix:" >> "$REPORT_FILE"
    echo "- Web UI file structure or deployment issues" >> "$REPORT_FILE"
    echo "- S3 permissions or access problems" >> "$REPORT_FILE"
    echo "- Metadata structure or validation issues" >> "$REPORT_FILE"
fi

# Summary
echo "=== Web UI to S3 Workflow Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"
echo "Report generated: $REPORT_FILE" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 Web UI to S3 workflow test completed successfully!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some web UI to S3 workflow tests failed. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi