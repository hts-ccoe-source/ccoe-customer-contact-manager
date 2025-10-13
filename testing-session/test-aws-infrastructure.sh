#!/bin/bash

# CCOE Customer Contact Manager - AWS Infrastructure Tests
# Tests AWS credentials, permissions, and infrastructure components

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/aws-infrastructure-test-$TIMESTAMP.log"

echo "=== CCOE Customer Contact Manager - Infrastructure Tests ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

# Function to run AWS CLI command with error handling
run_aws_command() {
    local description="$1"
    local command="$2"
    
    echo "Testing $description..." | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if eval "$command" >> "$LOG_FILE" 2>&1; then
        echo "✅ PASS: $description" | tee -a "$LOG_FILE"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo "❌ FAIL: $description" | tee -a "$LOG_FILE"
        return 1
    fi
}

# Test 1: AWS CLI availability
echo "=== Test 1: AWS CLI Availability ===" | tee -a "$LOG_FILE"
if command -v aws >/dev/null 2>&1; then
    echo "✅ PASS: AWS CLI is installed" | tee -a "$LOG_FILE"
    aws --version | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: AWS CLI not found" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Test 2: AWS Credentials
echo "=== Test 2: AWS Credentials ===" | tee -a "$LOG_FILE"
run_aws_command "AWS credentials and caller identity" "aws sts get-caller-identity"
echo "" | tee -a "$LOG_FILE"

# Test 3: AWS Region Configuration
echo "=== Test 3: AWS Region Configuration ===" | tee -a "$LOG_FILE"
if AWS_REGION=$(aws configure get region 2>/dev/null) && [[ -n "$AWS_REGION" ]]; then
    echo "✅ PASS: AWS region configured: $AWS_REGION" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: AWS region not configured" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi
echo "" | tee -a "$LOG_FILE"

# Test 4: Organizations Access
echo "=== Test 4: AWS Organizations Access ===" | tee -a "$LOG_FILE"
run_aws_command "Organizations describe-organization" "aws organizations describe-organization"
echo "" | tee -a "$LOG_FILE"

# Test 5: SES Access
echo "=== Test 5: SES Access ===" | tee -a "$LOG_FILE"
run_aws_command "SES get-send-quota" "aws ses get-send-quota"
echo "" | tee -a "$LOG_FILE"

# Test 6: SQS Access
echo "=== Test 6: SQS Access ===" | tee -a "$LOG_FILE"
run_aws_command "SQS list-queues" "aws sqs list-queues"
echo "" | tee -a "$LOG_FILE"

# Test 7: S3 Access
echo "=== Test 7: S3 Access ===" | tee -a "$LOG_FILE"
run_aws_command "S3 list-buckets" "aws s3 ls"
echo "" | tee -a "$LOG_FILE"

# Test 8: IAM Access
echo "=== Test 8: IAM Access ===" | tee -a "$LOG_FILE"
# Try get-user first, if it fails (assumed role), try list-roles instead
if aws iam get-user >/dev/null 2>&1; then
    run_aws_command "IAM get-user" "aws iam get-user"
else
    run_aws_command "IAM list-roles (assumed role detected)" "aws iam list-roles --max-items 1"
fi
echo "" | tee -a "$LOG_FILE"

# Test 9: Test specific S3 bucket (if configured)
echo "=== Test 9: S3 Bucket Configuration ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/config.json" ]]; then
    BUCKET_NAME=$(jq -r '.s3_config.bucket_name' "$PROJECT_ROOT/config.json" 2>/dev/null)
    if [[ "$BUCKET_NAME" != "null" && -n "$BUCKET_NAME" ]]; then
        run_aws_command "S3 bucket access: $BUCKET_NAME" "aws s3 ls s3://$BUCKET_NAME/"
    else
        echo "⚠️  SKIP: No bucket name configured in config.json" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: config.json not found" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 10: Test SQS queues (if configured)
echo "=== Test 10: SQS Queue Configuration ===" | tee -a "$LOG_FILE"
if [[ -f "$PROJECT_ROOT/config.json" ]]; then
    # Extract SQS queue ARNs from customer mappings
    QUEUE_ARNS=$(jq -r '.customer_mappings | to_entries[].value.sqs_queue_arn' "$PROJECT_ROOT/config.json" 2>/dev/null)
    if [[ -n "$QUEUE_ARNS" ]]; then
        while IFS= read -r queue_arn; do
            if [[ "$queue_arn" != "null" && -n "$queue_arn" ]]; then
                # Extract components from ARN: arn:aws:sqs:region:account-id:queue-name
                REGION=$(echo "$queue_arn" | cut -d: -f4)
                ACCOUNT_ID=$(echo "$queue_arn" | cut -d: -f5)
                QUEUE_NAME=$(echo "$queue_arn" | cut -d: -f6)
                QUEUE_URL="https://sqs.${REGION}.amazonaws.com/${ACCOUNT_ID}/${QUEUE_NAME}"
                run_aws_command "SQS queue attributes: $queue_arn" "aws sqs get-queue-attributes --queue-url '$QUEUE_URL' --attribute-names All"
            fi
        done <<< "$QUEUE_ARNS"
    else
        echo "⚠️  SKIP: No SQS queues configured in config.json" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: config.json not found" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 11: Test application with validate mode
echo "=== Test 11: Application Validate Mode ===" | tee -a "$LOG_FILE"
cd "$PROJECT_ROOT"
if [[ -f "$RESULTS_DIR/ccoe-customer-contact-manager-test" ]]; then
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if "$RESULTS_DIR/ccoe-customer-contact-manager-test" -mode=validate >> "$LOG_FILE" 2>&1; then
        echo "✅ PASS: Application validate mode works" | tee -a "$LOG_FILE"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "❌ FAIL: Application validate mode failed" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: Application binary not found (run config tests first)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 12: Test dry-run mode
echo "=== Test 12: Application Dry-Run Mode ===" | tee -a "$LOG_FILE"
cd "$PROJECT_ROOT"
if [[ -f "$RESULTS_DIR/ccoe-customer-contact-manager-test" ]]; then
    # Get first customer from config.json if available
    if [[ -f "config.json" ]]; then
        FIRST_CUSTOMER=$(jq -r '.customer_mappings | keys[0]' config.json 2>/dev/null)
        if [[ "$FIRST_CUSTOMER" != "null" && -n "$FIRST_CUSTOMER" ]]; then
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
            if "$RESULTS_DIR/ccoe-customer-contact-manager-test" -mode=update -customer="$FIRST_CUSTOMER" -dry-run >> "$LOG_FILE" 2>&1; then
                echo "✅ PASS: Application dry-run mode works for customer: $FIRST_CUSTOMER" | tee -a "$LOG_FILE"
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                echo "❌ FAIL: Application dry-run mode failed for customer: $FIRST_CUSTOMER" | tee -a "$LOG_FILE"
            fi
        else
            echo "⚠️  SKIP: No valid customers found in config.json" | tee -a "$LOG_FILE"
        fi
    else
        echo "⚠️  SKIP: config.json not found" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: Application binary not found (run config tests first)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Summary
echo "=== AWS Infrastructure Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 All AWS infrastructure tests passed!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some AWS infrastructure tests failed. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi