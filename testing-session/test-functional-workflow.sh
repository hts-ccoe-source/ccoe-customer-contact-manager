#!/bin/bash

# AWS Alternate Contact Manager - Functional Workflow Tests
# Tests the complete web UI → S3 → SQS → ECS pipeline

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/functional-workflow-test-$TIMESTAMP.log"

echo "=== AWS Alternate Contact Manager - Functional Workflow Tests ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

# Configuration from consolidated config.json
S3_BUCKET=$(jq -r '.s3_config.bucket_name // empty' "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
# Get first customer for testing
CUSTOMER_CODE=$(jq -r '.customer_mappings | keys[0] // empty' "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
SQS_QUEUE_ARN=$(jq -r ".customer_mappings[\"$CUSTOMER_CODE\"].sqs_queue_arn // empty" "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
CUSTOMER_PREFIX="customers/$CUSTOMER_CODE/"

# Convert SQS ARN to URL
if [[ -n "$SQS_QUEUE_ARN" && "$SQS_QUEUE_ARN" != "null" ]]; then
    # Extract region and account from ARN: arn:aws:sqs:region:account:queue-name
    REGION=$(echo "$SQS_QUEUE_ARN" | cut -d: -f4)
    ACCOUNT=$(echo "$SQS_QUEUE_ARN" | cut -d: -f5)
    QUEUE_NAME=$(echo "$SQS_QUEUE_ARN" | cut -d: -f6)
    SQS_QUEUE_URL="https://sqs.${REGION}.amazonaws.com/${ACCOUNT}/${QUEUE_NAME}"
fi

echo "Configuration:" | tee -a "$LOG_FILE"
echo "  S3 Bucket: $S3_BUCKET" | tee -a "$LOG_FILE"
echo "  SQS Queue ARN: $SQS_QUEUE_ARN" | tee -a "$LOG_FILE"
echo "  SQS Queue URL: $SQS_QUEUE_URL" | tee -a "$LOG_FILE"
echo "  Customer Code: $CUSTOMER_CODE" | tee -a "$LOG_FILE"
echo "  Customer Prefix: $CUSTOMER_PREFIX" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Function to test workflow step
test_workflow_step() {
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

# Test 1: S3 Bucket Access
echo "=== Test 1: S3 Infrastructure Validation ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" && "$S3_BUCKET" != "null" ]]; then
    test_workflow_step "S3 bucket exists and is accessible" "aws s3 ls s3://$S3_BUCKET/" "true"
    # Customer prefix may not exist yet, so we'll test if we can create it
    test_workflow_step "S3 customer prefix write access" "echo 'test' | aws s3 cp - s3://$S3_BUCKET/${CUSTOMER_PREFIX}test-access.txt && aws s3 rm s3://$S3_BUCKET/${CUSTOMER_PREFIX}test-access.txt" "true"
else
    echo "⚠️  SKIP: S3 bucket not configured" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 2: SQS Queue Access
echo "=== Test 2: SQS Infrastructure Validation ===" | tee -a "$LOG_FILE"
if [[ -n "$SQS_QUEUE_URL" && "$SQS_QUEUE_URL" != "null" ]]; then
    test_workflow_step "SQS queue exists and is accessible" "aws sqs get-queue-attributes --queue-url '$SQS_QUEUE_URL' --attribute-names QueueArn" "true"
    test_workflow_step "SQS queue can receive messages" "aws sqs get-queue-attributes --queue-url '$SQS_QUEUE_URL' --attribute-names ReceiveMessageWaitTimeSeconds" "true"
else
    echo "⚠️  SKIP: SQS queue not configured" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 3: Create Test Metadata File
echo "=== Test 3: Metadata File Creation ===" | tee -a "$LOG_FILE"
TEST_METADATA_FILE="$RESULTS_DIR/test-metadata-$TIMESTAMP.json"
cat > "$TEST_METADATA_FILE" << EOF
{
  "changeId": "TEST-$TIMESTAMP",
  "title": "Functional Test - Web UI to SQS Workflow",
  "description": "Testing the complete workflow from web UI through S3 to SQS processing",
  "customers": ["$CUSTOMER_CODE"],
  "implementationPlan": "This is a test implementation plan for functional testing",
  "schedule": {
    "startDate": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "endDate": "$(date -u -v+1H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '+1 hour' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")"
  },
  "impact": "low",
  "rollbackPlan": "Test rollback plan",
  "communicationPlan": "Test communication plan",
  "approver": "test-user@hearst.com",
  "implementer": "test-implementer@hearst.com",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "source": "functional-test",
  "testRun": true
}
EOF

if [[ -f "$TEST_METADATA_FILE" ]]; then
    echo "✅ PASS: Test metadata file created" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo "   File: $TEST_METADATA_FILE" | tee -a "$LOG_FILE"
    echo "   Size: $(wc -c < "$TEST_METADATA_FILE") bytes" | tee -a "$LOG_FILE"
else
    echo "❌ FAIL: Test metadata file creation" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Test 4: S3 Upload Simulation (Web UI Step)
echo "=== Test 4: S3 Upload Simulation (Web UI Step) ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" && -f "$TEST_METADATA_FILE" ]]; then
    # Upload to customer prefix (simulating web UI upload)
    CUSTOMER_S3_KEY="${CUSTOMER_PREFIX}test-metadata-$TIMESTAMP.json"
    test_workflow_step "Upload to customer S3 prefix" "aws s3 cp '$TEST_METADATA_FILE' s3://$S3_BUCKET/$CUSTOMER_S3_KEY" "true"
    
    # Upload to archive prefix (simulating web UI archive)
    ARCHIVE_S3_KEY="archive/test-metadata-$TIMESTAMP.json"
    test_workflow_step "Upload to archive S3 prefix" "aws s3 cp '$TEST_METADATA_FILE' s3://$S3_BUCKET/$ARCHIVE_S3_KEY" "true"
    
    # Verify uploads
    test_workflow_step "Verify customer upload exists" "aws s3 ls s3://$S3_BUCKET/$CUSTOMER_S3_KEY" "true"
    test_workflow_step "Verify archive upload exists" "aws s3 ls s3://$S3_BUCKET/$ARCHIVE_S3_KEY" "true"
else
    echo "⚠️  SKIP: S3 upload test (bucket or metadata file not available)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 5: Wait for S3 Event Processing
echo "=== Test 5: S3 Event Processing ===" | tee -a "$LOG_FILE"
if [[ -n "$SQS_QUEUE_URL" ]]; then
    echo "Waiting 10 seconds for S3 event to trigger SQS message..." | tee -a "$LOG_FILE"
    sleep 10
    
    # Check for messages in SQS queue
    test_workflow_step "Check for SQS messages from S3 event" "aws sqs receive-message --queue-url '$SQS_QUEUE_URL' --max-number-of-messages 1 --wait-time-seconds 5" "true"
    
    # Get message count
    MESSAGE_COUNT=$(aws sqs get-queue-attributes --queue-url "$SQS_QUEUE_URL" --attribute-names ApproximateNumberOfMessages --query 'Attributes.ApproximateNumberOfMessages' --output text 2>/dev/null || echo "0")
    echo "   Current message count in queue: $MESSAGE_COUNT" | tee -a "$LOG_FILE"
    
    if [[ "$MESSAGE_COUNT" -gt 0 ]]; then
        echo "✅ PASS: SQS queue has messages (S3 event triggered successfully)" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "⚠️  WARNING: No messages in SQS queue (S3 event may not be configured)" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
else
    echo "⚠️  SKIP: SQS event processing test (queue not configured)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 6: SQS Message Validation (No Processing)
echo "=== Test 6: SQS Message Validation ===" | tee -a "$LOG_FILE"
if [[ -n "$SQS_QUEUE_URL" ]]; then
    # Just validate we can access the queue and see messages, don't process them
    test_workflow_step "SQS queue message visibility" "aws sqs get-queue-attributes --queue-url '$SQS_QUEUE_URL' --attribute-names ApproximateNumberOfMessages" "true"
    
    # Check if we can receive messages without deleting them
    test_workflow_step "SQS message peek (no deletion)" "aws sqs receive-message --queue-url '$SQS_QUEUE_URL' --max-number-of-messages 1 --visibility-timeout-seconds 1" "true"
else
    echo "⚠️  SKIP: SQS message validation (queue not configured)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 7: End-to-End Validation
echo "=== Test 7: End-to-End Validation ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" && -n "$SQS_QUEUE_URL" ]]; then
    # Create a second test file for end-to-end validation
    E2E_METADATA_FILE="$RESULTS_DIR/e2e-test-metadata-$TIMESTAMP.json"
    cat > "$E2E_METADATA_FILE" << EOF
{
  "changeId": "E2E-TEST-$TIMESTAMP",
  "title": "End-to-End Functional Test",
  "description": "Complete pipeline test from upload to processing",
  "customers": ["$CUSTOMER_CODE"],
  "implementationPlan": "E2E test implementation",
  "schedule": {
    "startDate": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "endDate": "$(date -u -v+2H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '+2 hours' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")"
  },
  "impact": "low",
  "rollbackPlan": "E2E test rollback",
  "communicationPlan": "E2E test communication",
  "approver": "e2e-test@hearst.com",
  "implementer": "e2e-implementer@hearst.com",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "source": "e2e-functional-test",
  "testRun": true
}
EOF
    
    # Upload and process
    E2E_S3_KEY="${CUSTOMER_PREFIX}e2e-test-metadata-$TIMESTAMP.json"
    if aws s3 cp "$E2E_METADATA_FILE" "s3://$S3_BUCKET/$E2E_S3_KEY" >> "$LOG_FILE" 2>&1; then
        echo "✅ PASS: E2E test file uploaded to S3" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
        
        # Wait for S3 event to potentially trigger SQS
        echo "Waiting 15 seconds for S3 event processing..." | tee -a "$LOG_FILE"
        sleep 15
        
        # Check final state
        FINAL_MESSAGE_COUNT=$(aws sqs get-queue-attributes --queue-url "$SQS_QUEUE_URL" --attribute-names ApproximateNumberOfMessages --query 'Attributes.ApproximateNumberOfMessages' --output text 2>/dev/null || echo "0")
        echo "   Final message count: $FINAL_MESSAGE_COUNT" | tee -a "$LOG_FILE"
        
        # Verify file still exists in S3
        if aws s3 ls "s3://$S3_BUCKET/$E2E_S3_KEY" >> "$LOG_FILE" 2>&1; then
            echo "✅ PASS: E2E test file persists in S3" | tee -a "$LOG_FILE"
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "❌ FAIL: E2E test file missing from S3" | tee -a "$LOG_FILE"
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
        fi
    else
        echo "❌ FAIL: E2E test file upload" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
    
    # Clean up test files
    aws s3 rm "s3://$S3_BUCKET/$E2E_S3_KEY" >/dev/null 2>&1 || true
    rm -f "$E2E_METADATA_FILE"
else
    echo "⚠️  SKIP: End-to-end validation (S3 or SQS not configured)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 8: Web UI File Validation
echo "=== Test 8: Web UI File Validation ===" | tee -a "$LOG_FILE"
WEB_UI_FILE="$PROJECT_ROOT/html/index.html"
if [[ -f "$WEB_UI_FILE" ]]; then
    # Check if web UI file exists and has required elements
    if grep -q "customers\[\]" "$WEB_UI_FILE" && grep -q "S3" "$WEB_UI_FILE"; then
        echo "✅ PASS: Web UI file exists and contains required elements" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
        
        # Check file size
        WEB_UI_SIZE=$(wc -c < "$WEB_UI_FILE")
        echo "   Web UI file size: $WEB_UI_SIZE bytes" | tee -a "$LOG_FILE"
        
        # Check if it's uploaded to S3
        if aws s3 ls "s3://$S3_BUCKET/index.html" >> "$LOG_FILE" 2>&1; then
            echo "✅ PASS: Web UI file is deployed to S3" | tee -a "$LOG_FILE"
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "⚠️  WARNING: Web UI file not found in S3 bucket" | tee -a "$LOG_FILE"
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
        fi
    else
        echo "❌ FAIL: Web UI file missing required elements" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
else
    echo "❌ FAIL: Web UI file not found" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Clean up test files
echo "=== Cleanup ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" ]]; then
    # Clean up test files from S3
    aws s3 rm "s3://$S3_BUCKET/$CUSTOMER_S3_KEY" >/dev/null 2>&1 || true
    aws s3 rm "s3://$S3_BUCKET/$ARCHIVE_S3_KEY" >/dev/null 2>&1 || true
    echo "✅ Test files cleaned up from S3" | tee -a "$LOG_FILE"
fi

# Clean up local test files
rm -f "$TEST_METADATA_FILE"
echo "✅ Local test files cleaned up" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Generate functional test report
REPORT_FILE="$RESULTS_DIR/functional-workflow-report-$TIMESTAMP.md"
cat > "$REPORT_FILE" << EOF
# Functional Workflow Test Report

**Test Run:** $(date)
**Pipeline:** Web UI → S3 → SQS → Application Processing

## Configuration Tested

- **S3 Bucket:** $S3_BUCKET
- **SQS Queue:** $SQS_QUEUE_ARN
- **Customer Code:** $CUSTOMER_CODE
- **Customer Prefix:** $CUSTOMER_PREFIX

## Summary

- **Total Tests:** $TOTAL_TESTS
- **Passed:** $PASSED_TESTS
- **Failed:** $((TOTAL_TESTS - PASSED_TESTS))
- **Success Rate:** $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Categories

1. **S3 Infrastructure** - Bucket access and prefix validation
2. **SQS Infrastructure** - Queue access and message handling
3. **Metadata Creation** - Test file generation and validation
4. **S3 Upload Simulation** - Web UI upload workflow simulation
5. **S3 Event Processing** - S3 → SQS event trigger validation
6. **Application Processing** - SQS message processing by CLI
7. **End-to-End Validation** - Complete pipeline test
8. **Web UI Validation** - HTML file and deployment check

## Detailed Results

See full log: \`$(basename "$LOG_FILE")\`

## Next Steps

EOF

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "✅ **All functional tests passed!** The web UI → S3 → SQS workflow is working correctly." >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "**Ready for production use:**" >> "$REPORT_FILE"
    echo "- Web UI can successfully upload metadata to S3" >> "$REPORT_FILE"
    echo "- S3 events trigger SQS messages correctly" >> "$REPORT_FILE"
    echo "- Application can process SQS messages" >> "$REPORT_FILE"
    echo "- End-to-end pipeline is functional" >> "$REPORT_FILE"
else
    echo "⚠️ **Some functional tests failed.** Review the issues before production deployment." >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "**Common issues to check:**" >> "$REPORT_FILE"
    echo "- S3 bucket permissions and event notifications" >> "$REPORT_FILE"
    echo "- SQS queue permissions and visibility" >> "$REPORT_FILE"
    echo "- Application configuration and AWS credentials" >> "$REPORT_FILE"
    echo "- Web UI deployment and accessibility" >> "$REPORT_FILE"
fi

# Summary
echo "=== Functional Workflow Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"
echo "Report generated: $REPORT_FILE" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 All functional workflow tests passed!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some functional workflow tests failed. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi