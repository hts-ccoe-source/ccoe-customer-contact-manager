#!/bin/bash

# AWS Alternate Contact Manager - Run All Tests
# Master test runner that executes all test suites

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Master log file
MASTER_LOG="$RESULTS_DIR/master-test-run-$TIMESTAMP.log"

echo "=== AWS Alternate Contact Manager - Master Test Suite ===" | tee "$MASTER_LOG"
echo "Started at: $(date)" | tee -a "$MASTER_LOG"
echo "Test session: $TIMESTAMP" | tee -a "$MASTER_LOG"
echo "" | tee -a "$MASTER_LOG"

# Test suite results
TOTAL_SUITES=0
PASSED_SUITES=0

# Function to run test suite
run_test_suite() {
    local suite_name="$1"
    local script_path="$2"
    
    echo "=== Running $suite_name ===" | tee -a "$MASTER_LOG"
    TOTAL_SUITES=$((TOTAL_SUITES + 1))
    
    if [[ ! -f "$script_path" ]]; then
        echo "❌ FAIL: Test script not found: $script_path" | tee -a "$MASTER_LOG"
        return 1
    fi
    
    # Make script executable
    chmod +x "$script_path"
    
    # Run the test suite
    if "$script_path"; then
        echo "✅ PASS: $suite_name completed successfully" | tee -a "$MASTER_LOG"
        PASSED_SUITES=$((PASSED_SUITES + 1))
        return 0
    else
        echo "❌ FAIL: $suite_name failed" | tee -a "$MASTER_LOG"
        return 1
    fi
}

# Pre-flight checks
echo "=== Pre-flight Checks ===" | tee -a "$MASTER_LOG"

# Check if we're in the right directory (either project root or testing-session)
if [[ ! -f "main.go" && ! -f "../main.go" ]]; then
    echo "❌ FAIL: Not in the correct directory. Run from project root or testing-session/" | tee -a "$MASTER_LOG"
    exit 1
fi

# If running from project root, adjust paths
if [[ -f "main.go" ]]; then
    PROJECT_ROOT="."
else
    PROJECT_ROOT=".."
fi

# Check required tools
MISSING_TOOLS=()
for tool in "go" "jq" "aws"; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        MISSING_TOOLS+=("$tool")
    fi
done

if [[ ${#MISSING_TOOLS[@]} -gt 0 ]]; then
    echo "❌ FAIL: Missing required tools: ${MISSING_TOOLS[*]}" | tee -a "$MASTER_LOG"
    echo "Please install the missing tools and try again." | tee -a "$MASTER_LOG"
    exit 1
fi

echo "✅ PASS: All required tools available" | tee -a "$MASTER_LOG"
echo "" | tee -a "$MASTER_LOG"

# Run test suites in order
echo "=== Test Suite Execution ===" | tee -a "$MASTER_LOG"

# Suite 1: Configuration Tests
run_test_suite "Configuration Tests" "$SCRIPT_DIR/test-config.sh"
echo "" | tee -a "$MASTER_LOG"

# Suite 2: AWS Infrastructure Tests
run_test_suite "AWS Infrastructure Tests" "$SCRIPT_DIR/test-aws-infrastructure.sh"
echo "" | tee -a "$MASTER_LOG"

# Suite 3: Application Mode Tests
run_test_suite "Application Mode Tests" "$SCRIPT_DIR/test-app-modes.sh"
echo "" | tee -a "$MASTER_LOG"

# Suite 4: Integration Tests
run_test_suite "Integration Tests" "$SCRIPT_DIR/test-integration.sh"
echo "" | tee -a "$MASTER_LOG"

# Generate master test report
REPORT_FILE="$RESULTS_DIR/master-test-report-$TIMESTAMP.md"
cat > "$REPORT_FILE" << EOF
# AWS Alternate Contact Manager - Master Test Report

**Test Session:** $TIMESTAMP  
**Started:** $(date)  
**Duration:** $(( $(date +%s) - $(date -d "$(head -2 "$MASTER_LOG" | tail -1 | cut -d: -f2-)" +%s) )) seconds

## Executive Summary

- **Total Test Suites:** $TOTAL_SUITES
- **Passed Suites:** $PASSED_SUITES
- **Failed Suites:** $((TOTAL_SUITES - PASSED_SUITES))
- **Success Rate:** $(( (PASSED_SUITES * 100) / TOTAL_SUITES ))%

## Test Suite Results

| Suite | Status | Details |
|-------|--------|---------|
| Configuration Tests | $(if grep -q "✅ PASS.*Configuration Tests" "$MASTER_LOG"; then echo "✅ PASS"; else echo "❌ FAIL"; fi) | JSON validation, Go modules, build test |
| AWS Infrastructure Tests | $(if grep -q "✅ PASS.*AWS Infrastructure Tests" "$MASTER_LOG"; then echo "✅ PASS"; else echo "❌ FAIL"; fi) | AWS CLI, credentials, service access |
| Application Mode Tests | $(if grep -q "✅ PASS.*Application Mode Tests" "$MASTER_LOG"; then echo "✅ PASS"; else echo "❌ FAIL"; fi) | CLI modes, validation, error handling |
| Integration Tests | $(if grep -q "✅ PASS.*Integration Tests" "$MASTER_LOG"; then echo "✅ PASS"; else echo "❌ FAIL"; fi) | End-to-end workflows, performance |

## Detailed Logs

- **Master Log:** \`$(basename "$MASTER_LOG")\`
- **Individual Logs:** Check \`test-results/\` directory for detailed logs from each suite

## Environment Information

- **Operating System:** $(uname -s)
- **Go Version:** $(go version)
- **AWS CLI Version:** $(aws --version)
- **jq Version:** $(jq --version)

## Next Steps

EOF

if [[ $PASSED_SUITES -eq $TOTAL_SUITES ]]; then
    cat >> "$REPORT_FILE" << EOF
### ✅ All Tests Passed!

The AWS Alternate Contact Manager is ready for production use. All test suites passed successfully.

**Recommended Actions:**
1. Deploy to production environment
2. Set up monitoring and alerting
3. Schedule regular health checks
4. Document operational procedures

EOF
    echo "🎉 ALL TEST SUITES PASSED!" | tee -a "$MASTER_LOG"
    echo "📊 Master test report generated: $REPORT_FILE" | tee -a "$MASTER_LOG"
else
    cat >> "$REPORT_FILE" << EOF
### ⚠️ Some Tests Failed

$((TOTAL_SUITES - PASSED_SUITES)) out of $TOTAL_SUITES test suites failed. Review the detailed logs and address issues before production deployment.

**Required Actions:**
1. Review failed test logs in detail
2. Fix configuration or infrastructure issues
3. Re-run failed test suites
4. Ensure all tests pass before deployment

**Common Issues:**
- AWS credentials or permissions
- Missing or invalid configuration files
- Network connectivity to AWS services
- Resource limits or quotas

EOF
    echo "⚠️  SOME TEST SUITES FAILED!" | tee -a "$MASTER_LOG"
    echo "📊 Master test report generated: $REPORT_FILE" | tee -a "$MASTER_LOG"
fi

# Final summary
echo "" | tee -a "$MASTER_LOG"
echo "=== Final Summary ===" | tee -a "$MASTER_LOG"
echo "Test suites completed: $TOTAL_SUITES" | tee -a "$MASTER_LOG"
echo "Test suites passed: $PASSED_SUITES" | tee -a "$MASTER_LOG"
echo "Test suites failed: $((TOTAL_SUITES - PASSED_SUITES))" | tee -a "$MASTER_LOG"
echo "Overall success rate: $(( (PASSED_SUITES * 100) / TOTAL_SUITES ))%" | tee -a "$MASTER_LOG"
echo "Completed at: $(date)" | tee -a "$MASTER_LOG"

# List all generated files
echo "" | tee -a "$MASTER_LOG"
echo "Generated files:" | tee -a "$MASTER_LOG"
ls -la "$RESULTS_DIR"/*-$TIMESTAMP.* | tee -a "$MASTER_LOG"

if [[ $PASSED_SUITES -eq $TOTAL_SUITES ]]; then
    exit 0
else
    exit 1
fi