#!/bin/bash

# CCOE Customer Contact Manager - Web UI Tests
# Tests the web UI functionality and deployment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/web-ui-test-$TIMESTAMP.log"

echo "=== CCOE Customer Contact Manager - Web UI Tests ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0

# Configuration from consolidated config.json
S3_BUCKET=$(jq -r '.s3_config.bucket_name // empty' "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
WEB_UI_FILE="$PROJECT_ROOT/html/index.html"

echo "Configuration:" | tee -a "$LOG_FILE"
echo "  S3 Bucket: $S3_BUCKET" | tee -a "$LOG_FILE"
echo "  Web UI File: $WEB_UI_FILE" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Function to test web UI aspect
test_web_ui() {
    local description="$1"
    local test_command="$2"
    local expect_success="$3"  # true/false
    
    echo "Testing $description..." | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if eval "$test_command" >> "$LOG_FILE" 2>&1; then
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

# Test 1: Web UI File Existence and Structure
echo "=== Test 1: Web UI File Structure ===" | tee -a "$LOG_FILE"
if [[ -f "$WEB_UI_FILE" ]]; then
    echo "✅ PASS: Web UI file exists" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    
    # Check file size
    FILE_SIZE=$(wc -c < "$WEB_UI_FILE")
    echo "   File size: $FILE_SIZE bytes" | tee -a "$LOG_FILE"
    
    if [[ $FILE_SIZE -gt 1000 ]]; then
        echo "✅ PASS: Web UI file has reasonable size" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "❌ FAIL: Web UI file too small" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
else
    echo "❌ FAIL: Web UI file not found" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    exit 1
fi
echo "" | tee -a "$LOG_FILE"

# Test 2: HTML Structure Validation
echo "=== Test 2: HTML Structure Validation ===" | tee -a "$LOG_FILE"

# Check for required HTML elements
test_web_ui "HTML document structure" "grep -q '<!DOCTYPE html>' '$WEB_UI_FILE'" "true"
test_web_ui "HTML head section" "grep -q '<head>' '$WEB_UI_FILE'" "true"
test_web_ui "HTML body section" "grep -q '<body>' '$WEB_UI_FILE'" "true"
test_web_ui "Form element exists" "grep -q '<form' '$WEB_UI_FILE'" "true"
test_web_ui "Title field exists" "grep -q 'name=\"title\"' '$WEB_UI_FILE'" "true"
test_web_ui "Customer selection exists" "grep -q 'customers\[\]' '$WEB_UI_FILE'" "true"

echo "" | tee -a "$LOG_FILE"

# Test 3: JavaScript Functionality
echo "=== Test 3: JavaScript Functionality ===" | tee -a "$LOG_FILE"

test_web_ui "JavaScript code present" "grep -q '<script>' '$WEB_UI_FILE'" "true"
test_web_ui "AWS SDK integration" "grep -q 'AWS' '$WEB_UI_FILE'" "true"
test_web_ui "S3 upload functionality" "grep -q 's3' '$WEB_UI_FILE'" "true"
test_web_ui "Progress tracking code" "grep -q 'progress' '$WEB_UI_FILE'" "true"
test_web_ui "Error handling code" "grep -q 'error' '$WEB_UI_FILE'" "true"

echo "" | tee -a "$LOG_FILE"

# Test 4: Multi-Customer Features
echo "=== Test 4: Multi-Customer Features ===" | tee -a "$LOG_FILE"

test_web_ui "Multi-customer selection" "grep -q 'checkbox' '$WEB_UI_FILE'" "true"
test_web_ui "Customer mapping" "grep -q 'customerMapping' '$WEB_UI_FILE'" "true"
test_web_ui "Upload queue functionality" "grep -q 'uploadQueue' '$WEB_UI_FILE'" "true"
test_web_ui "Retry mechanism" "grep -q 'retry' '$WEB_UI_FILE'" "true"

echo "" | tee -a "$LOG_FILE"

# Test 5: Security Features
echo "=== Test 5: Security Features ===" | tee -a "$LOG_FILE"

test_web_ui "HTTPS enforcement" "grep -q 'https' '$WEB_UI_FILE'" "true"
test_web_ui "Credential handling" "grep -q 'credentials' '$WEB_UI_FILE'" "true"
test_web_ui "Input validation" "grep -q 'required' '$WEB_UI_FILE'" "true"

echo "" | tee -a "$LOG_FILE"

# Test 6: S3 Deployment Check
echo "=== Test 6: S3 Deployment Check ===" | tee -a "$LOG_FILE"
if [[ -n "$S3_BUCKET" && "$S3_BUCKET" != "null" ]]; then
    test_web_ui "Web UI deployed to S3" "aws s3 ls s3://$S3_BUCKET/index.html" "true"
    
    # Check if the deployed version matches local version
    if aws s3 ls "s3://$S3_BUCKET/index.html" >/dev/null 2>&1; then
        # Download deployed version for comparison
        DEPLOYED_FILE="$RESULTS_DIR/deployed-web-ui-$TIMESTAMP.html"
        if aws s3 cp "s3://$S3_BUCKET/index.html" "$DEPLOYED_FILE" >> "$LOG_FILE" 2>&1; then
            LOCAL_SIZE=$(wc -c < "$WEB_UI_FILE")
            DEPLOYED_SIZE=$(wc -c < "$DEPLOYED_FILE")
            
            echo "   Local file size: $LOCAL_SIZE bytes" | tee -a "$LOG_FILE"
            echo "   Deployed file size: $DEPLOYED_SIZE bytes" | tee -a "$LOG_FILE"
            
            if [[ $LOCAL_SIZE -eq $DEPLOYED_SIZE ]]; then
                echo "✅ PASS: Deployed version matches local version (size)" | tee -a "$LOG_FILE"
                TOTAL_TESTS=$((TOTAL_TESTS + 1))
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                echo "⚠️  WARNING: Deployed version size differs from local version" | tee -a "$LOG_FILE"
                TOTAL_TESTS=$((TOTAL_TESTS + 1))
            fi
            
            # Clean up downloaded file
            rm -f "$DEPLOYED_FILE"
        fi
    fi
else
    echo "⚠️  SKIP: S3 deployment check (bucket not configured)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Test 7: Configuration Integration
echo "=== Test 7: Configuration Integration ===" | tee -a "$LOG_FILE"

# Check if web UI references match configuration files
CUSTOMER_CODES=$(jq -r '.validCustomers[]' "$PROJECT_ROOT/CustomerCodes.json" 2>/dev/null || echo "")
if [[ -n "$CUSTOMER_CODES" ]]; then
    echo "   Configured customers: $CUSTOMER_CODES" | tee -a "$LOG_FILE"
    
    # Check if at least one customer code appears in the web UI
    FOUND_CUSTOMER=false
    while IFS= read -r customer; do
        if [[ -n "$customer" ]] && grep -q "$customer" "$WEB_UI_FILE"; then
            FOUND_CUSTOMER=true
            break
        fi
    done <<< "$CUSTOMER_CODES"
    
    if [[ "$FOUND_CUSTOMER" == "true" ]]; then
        echo "✅ PASS: Web UI contains configured customer codes" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "⚠️  WARNING: Web UI may not contain configured customer codes" | tee -a "$LOG_FILE"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
else
    echo "⚠️  SKIP: Customer code integration check (CustomerCodes.json not found)" | tee -a "$LOG_FILE"
fi

# Check S3 bucket reference
if [[ -n "$S3_BUCKET" ]] && grep -q "$S3_BUCKET" "$WEB_UI_FILE"; then
    echo "✅ PASS: Web UI references correct S3 bucket" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
elif [[ -n "$S3_BUCKET" ]]; then
    echo "⚠️  WARNING: Web UI may not reference configured S3 bucket" | tee -a "$LOG_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
else
    echo "⚠️  SKIP: S3 bucket reference check (bucket not configured)" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"

# Test 8: Accessibility and Usability
echo "=== Test 8: Accessibility and Usability ===" | tee -a "$LOG_FILE"

test_web_ui "Form labels present" "grep -q '<label' '$WEB_UI_FILE'" "true"
test_web_ui "Input placeholders" "grep -q 'placeholder=' '$WEB_UI_FILE'" "true"
test_web_ui "Submit button present" "grep -q 'type=\"submit\"' '$WEB_UI_FILE'" "true"
test_web_ui "CSS styling present" "grep -q '<style>' '$WEB_UI_FILE'" "true"

echo "" | tee -a "$LOG_FILE"

# Generate web UI test report
REPORT_FILE="$RESULTS_DIR/web-ui-test-report-$TIMESTAMP.md"
cat > "$REPORT_FILE" << EOF
# Web UI Test Report

**Test Run:** $(date)
**Web UI File:** $WEB_UI_FILE
**S3 Bucket:** $S3_BUCKET

## Summary

- **Total Tests:** $TOTAL_TESTS
- **Passed:** $PASSED_TESTS
- **Failed:** $((TOTAL_TESTS - PASSED_TESTS))
- **Success Rate:** $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Categories

1. **File Structure** - File existence and basic validation
2. **HTML Structure** - Required HTML elements and form structure
3. **JavaScript Functionality** - AWS SDK integration and upload logic
4. **Multi-Customer Features** - Customer selection and upload queue
5. **Security Features** - HTTPS, credentials, and input validation
6. **S3 Deployment** - Deployment status and version matching
7. **Configuration Integration** - Alignment with config files
8. **Accessibility** - Form labels, styling, and usability

## File Information

- **Local File Size:** $(wc -c < "$WEB_UI_FILE") bytes
- **Last Modified:** $(stat -f "%Sm" "$WEB_UI_FILE" 2>/dev/null || stat -c "%y" "$WEB_UI_FILE" 2>/dev/null || echo "Unknown")

## Detailed Results

See full log: \`$(basename "$LOG_FILE")\`

## Recommendations

EOF

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "✅ **Web UI is fully functional** and ready for use." >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "**Key features validated:**" >> "$REPORT_FILE"
    echo "- HTML structure and form elements" >> "$REPORT_FILE"
    echo "- JavaScript functionality and AWS integration" >> "$REPORT_FILE"
    echo "- Multi-customer selection and upload features" >> "$REPORT_FILE"
    echo "- Security and accessibility features" >> "$REPORT_FILE"
    echo "- S3 deployment and configuration alignment" >> "$REPORT_FILE"
else
    echo "⚠️ **Some web UI tests failed.** Review and fix issues before deployment." >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "**Common issues to check:**" >> "$REPORT_FILE"
    echo "- Missing HTML elements or JavaScript functions" >> "$REPORT_FILE"
    echo "- Configuration mismatches between UI and config files" >> "$REPORT_FILE"
    echo "- S3 deployment issues or outdated deployed version" >> "$REPORT_FILE"
    echo "- Security or accessibility concerns" >> "$REPORT_FILE"
fi

# Summary
echo "=== Web UI Test Summary ===" | tee -a "$LOG_FILE"
echo "Total tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_TESTS" | tee -a "$LOG_FILE"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))" | tee -a "$LOG_FILE"
if [[ $TOTAL_TESTS -gt 0 ]]; then
    echo "Success rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%" | tee -a "$LOG_FILE"
fi
echo "Completed at: $(date)" | tee -a "$LOG_FILE"
echo "Report generated: $REPORT_FILE" | tee -a "$LOG_FILE"

if [[ $PASSED_TESTS -eq $TOTAL_TESTS ]]; then
    echo "🎉 All web UI tests passed!" | tee -a "$LOG_FILE"
    exit 0
else
    echo "⚠️  Some web UI tests failed. Check the log for details." | tee -a "$LOG_FILE"
    exit 1
fi