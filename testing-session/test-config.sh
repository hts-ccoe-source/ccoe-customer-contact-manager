#!/bin/bash

# AWS Alternate Contact Manager - Configuration Tests
# Tests all JSON configuration files and validates their structure

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/config-test-$TIMESTAMP.log"

echo "=== AWS Alternate Contact Manager - Configuration Tests ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "Project root: $PROJECT_ROOT" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Function to test JSON file validity
test_json_file() {
    local file="$1"
    local description="$2"
    
    echo "Testing $description..." | tee -a "$LOG_FILE"
    
    if [[ ! -f "$file" ]]; then
        echo "❌ FAIL: File not found: $file" | tee -a "$LOG_FILE"
        return 1
    fi
    
    if jq empty "$file" 2>/dev/null; then
        echo "✅ PASS: Valid JSON structure" | tee -a "$LOG_FILE"
        
        # Show file size and key count
        local size=$(wc -c < "$file")
        local keys=$(jq 'keys | length' "$file" 2>/dev/null || echo "N/A")
        echo "   Size: $size bytes, Keys: $keys" | tee -a "$LOG_FILE"
        return 0
    else
        echo "❌ FAIL: Invalid JSON structure" | tee -a "$LOG_FILE"
        jq empty "$file" 2>&1 | head -3 | sed 's/^/   /' | tee -a "$LOG_FILE"
        return 1
    fi
}

# Function to test configuration content
test_config_content() {
    local file="$1"
    local description="$2"
    local required_keys="$3"
    
    echo "Testing $description content..." | tee -a "$LOG_FILE"
    
    if [[ ! -f "$file" ]]; then
        echo "❌ FAIL: File not found: $file" | tee -a "$LOG_FILE"
        return 1
    fi
    
    local missing_keys=()
    IFS=',' read -ra KEYS <<< "$required_keys"
    
    for key in "${KEYS[@]}"; do
        if ! jq -e "has(\"$key\")" "$file" >/dev/null 2>&1; then
            missing_keys+=("$key")
        fi
    done
    
    if [[ ${#missing_keys[@]} -eq 0 ]]; then
        echo "✅ PASS: All required keys present" | tee -a "$LOG_FILE"
        return 0
    else
        echo "❌ FAIL: Missing required keys: ${missing_keys[*]}" | tee -a "$LOG_FILE"
        return 1
    fi
}

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

# Test 1: Main Configuration (config.json)
echo "=== Test 1: Main Configuration ===" | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if test_json_file "$PROJECT_ROOT/config.json" "config.json"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    test_config_content "$PROJECT_ROOT/config.json" "config.json" "aws_region,customer_mappings,contact_config,s3_config"
    
    # Test customer mappings structure
    echo "Testing customer mappings structure..." | tee -a "$LOG_FILE"
    if jq -e '.customer_mappings | type == "object" and length > 0' "$PROJECT_ROOT/config.json" >/dev/null 2>&1; then
        echo "✅ PASS: Customer mappings structure valid" | tee -a "$LOG_FILE"
        
        # Check if customers have required fields
        customer_count=$(jq '.customer_mappings | length' "$PROJECT_ROOT/config.json")
        echo "   Customers configured: $customer_count" | tee -a "$LOG_FILE"
        
        # Check for SQS queue ARN in customer mappings
        if jq -e '.customer_mappings | to_entries[] | .value | has("sqs_queue_arn")' "$PROJECT_ROOT/config.json" >/dev/null 2>&1; then
            echo "✅ PASS: Customer SQS queues configured" | tee -a "$LOG_FILE"
        else
            echo "❌ FAIL: Missing SQS queue ARN in customer mappings" | tee -a "$LOG_FILE"
        fi
    else
        echo "❌ FAIL: Invalid customer mappings structure" | tee -a "$LOG_FILE"
    fi
fi
echo "" | tee -a "$LOG_FILE"

# Test 2: OrgConfig.json (Optional)
echo "=== Test 2: Organization Configuration ===" | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [[ -f "$PROJECT_ROOT/OrgConfig.json" ]]; then
    if test_json_file "$PROJECT_ROOT/OrgConfig.json" "OrgConfig.json"; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        # Check if it's an array and has required fields
        if jq -e 'type == "array" and length > 0' "$PROJECT_ROOT/OrgConfig.json" >/dev/null 2>&1; then
            echo "✅ PASS: Valid array structure with entries" | tee -a "$LOG_FILE"
        else
            echo "❌ FAIL: Should be an array with entries" | tee -a "$LOG_FILE"
        fi
    fi
else
    echo "⚠️  SKIP: OrgConfig.json not found (optional)" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))  # Count as passed since it's optional
fi
echo "" | tee -a "$LOG_FILE"

# Test 3: Main Configuration File
echo "=== Test 3: Main Configuration File ===" | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if test_json_file "$PROJECT_ROOT/config.json" "config.json"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    test_config_content "$PROJECT_ROOT/config.json" "config.json" "aws_region,customer_mappings,contact_config"
fi
echo "" | tee -a "$LOG_FILE"

# Test 4: SubscriptionConfig.json (Optional)
echo "=== Test 4: Subscription Configuration ===" | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [[ -f "$PROJECT_ROOT/SubscriptionConfig.json" ]]; then
    if test_json_file "$PROJECT_ROOT/SubscriptionConfig.json" "SubscriptionConfig.json"; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        # Check if it has topic keys
        topic_count=$(jq 'keys | length' "$PROJECT_ROOT/SubscriptionConfig.json" 2>/dev/null || echo "0")
        echo "   Topics configured: $topic_count" | tee -a "$LOG_FILE"
    fi
else
    echo "⚠️  SKIP: SubscriptionConfig.json not found (optional)" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))  # Count as passed since it's optional
fi
echo "" | tee -a "$LOG_FILE"

# Test 5: Go module validation
echo "=== Test 5: Go Module Validation ===" | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
cd "$PROJECT_ROOT"
if go mod verify; then
    echo "✅ PASS: Go modules verified" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ FAIL: Go module verification failed" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 6: Build test
echo "=== Test 6: Application Build Test ===" | tee -a "$LOG_FILE"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
cd "$PROJECT_ROOT"
if go build -o "$RESULTS_DIR/aws-alternate-contact-manager-test" .; then
    echo "✅ PASS: Application builds successfully" | tee -a "$LOG_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))
    
    # Test version command
    if "$RESULTS_DIR/aws-alternate-contact-manager-test" -mode=version; then
        echo "✅ PASS: Version command works" | tee -a "$LOG_FILE"
    else
        echo "❌ FAIL: Version command failed" | tee -a "$LOG_FILE"
    fi
else
    echo "❌ FAIL: Application build failed" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Summary
echo "=== Configuration Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
echo "Completed at: $(date)" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 All configuration tests passed!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some configuration tests failed. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi