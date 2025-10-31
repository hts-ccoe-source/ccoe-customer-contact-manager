#!/bin/bash

# Script to verify CloudWatch logs for Lambda@Edge SAML function
# This script checks that session lifecycle logs are appearing correctly

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
FUNCTION_NAME="${1:-ccoe-customer-contact-manager-saml-auth}"
REGIONS=("us-east-1" "us-west-2" "eu-west-1" "ap-southeast-1")
TIME_RANGE_MINUTES=60

echo -e "${BLUE}=== CloudWatch Logs Verification for SAML Session Monitoring ===${NC}"
echo ""
echo "Function: ${FUNCTION_NAME}"
echo "Time Range: Last ${TIME_RANGE_MINUTES} minutes"
echo ""

# Function to check logs in a specific region
check_region_logs() {
    local region=$1
    local log_group="/aws/lambda/${region}.${FUNCTION_NAME}"
    
    echo -e "${YELLOW}Checking region: ${region}${NC}"
    echo "Log Group: ${log_group}"
    
    # Check if log group exists
    if ! aws logs describe-log-groups \
        --log-group-name-prefix "${log_group}" \
        --region "${region}" \
        --query 'logGroups[0].logGroupName' \
        --output text 2>/dev/null | grep -q "${log_group}"; then
        echo -e "${RED}✗ Log group not found in ${region}${NC}"
        echo ""
        return 1
    fi
    
    echo -e "${GREEN}✓ Log group exists${NC}"
    
    # Calculate time range
    local end_time=$(date +%s)000
    local start_time=$((end_time - (TIME_RANGE_MINUTES * 60 * 1000)))
    
    # Check for session lifecycle events
    echo ""
    echo "Checking for session lifecycle events..."
    
    # Session creation events
    local creation_count=$(aws logs filter-log-events \
        --log-group-name "${log_group}" \
        --region "${region}" \
        --start-time "${start_time}" \
        --end-time "${end_time}" \
        --filter-pattern "New session created" \
        --query 'length(events)' \
        --output text 2>/dev/null || echo "0")
    
    echo -e "  Session Creation: ${creation_count} events"
    
    # Session validation events
    local validation_count=$(aws logs filter-log-events \
        --log-group-name "${log_group}" \
        --region "${region}" \
        --start-time "${start_time}" \
        --end-time "${end_time}" \
        --filter-pattern "Valid session found" \
        --query 'length(events)' \
        --output text 2>/dev/null || echo "0")
    
    echo -e "  Session Validation: ${validation_count} events"
    
    # Session refresh events
    local refresh_count=$(aws logs filter-log-events \
        --log-group-name "${log_group}" \
        --region "${region}" \
        --start-time "${start_time}" \
        --end-time "${end_time}" \
        --filter-pattern "Session refresh triggered" \
        --query 'length(events)' \
        --output text 2>/dev/null || echo "0")
    
    echo -e "  Session Refresh: ${refresh_count} events"
    
    # Session expiration events
    local expiration_count=$(aws logs filter-log-events \
        --log-group-name "${log_group}" \
        --region "${region}" \
        --start-time "${start_time}" \
        --end-time "${end_time}" \
        --filter-pattern "Session validation failed" \
        --query 'length(events)' \
        --output text 2>/dev/null || echo "0")
    
    echo -e "  Session Expiration: ${expiration_count} events"
    
    # Session error events
    local error_count=$(aws logs filter-log-events \
        --log-group-name "${log_group}" \
        --region "${region}" \
        --start-time "${start_time}" \
        --end-time "${end_time}" \
        --filter-pattern "Session validation error" \
        --query 'length(events)' \
        --output text 2>/dev/null || echo "0")
    
    echo -e "  Session Errors: ${error_count} events"
    
    # Calculate total events
    local total_events=$((creation_count + validation_count + refresh_count + expiration_count + error_count))
    
    echo ""
    if [ "${total_events}" -gt 0 ]; then
        echo -e "${GREEN}✓ Found ${total_events} session lifecycle events in ${region}${NC}"
        
        # Show sample logs
        echo ""
        echo "Sample recent logs:"
        aws logs filter-log-events \
            --log-group-name "${log_group}" \
            --region "${region}" \
            --start-time "${start_time}" \
            --end-time "${end_time}" \
            --filter-pattern "session" \
            --query 'events[0:3].[timestamp,message]' \
            --output text 2>/dev/null | while read -r timestamp message; do
                local readable_time=$(date -r $((timestamp / 1000)) '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "N/A")
                echo "  [${readable_time}] ${message:0:100}..."
            done
        
        return 0
    else
        echo -e "${YELLOW}⚠ No session lifecycle events found in ${region} (last ${TIME_RANGE_MINUTES} minutes)${NC}"
        echo "  This may be normal if there's no traffic to this edge location"
        return 1
    fi
    
    echo ""
}

# Function to run CloudWatch Insights query
run_insights_query() {
    local region=$1
    local log_group="/aws/lambda/${region}.${FUNCTION_NAME}"
    
    echo -e "${BLUE}Running CloudWatch Insights query in ${region}...${NC}"
    
    # Query for session statistics
    local query="fields @timestamp, @message
| filter @message like /session/
| stats 
    count(@message like /New session created/) as newSessions,
    count(@message like /Valid session found/) as validations,
    count(@message like /Session refresh triggered/) as refreshes,
    count(@message like /Session validation failed/) as expirations,
    count(@message like /Session validation error/) as errors"
    
    # Start query
    local query_id=$(aws logs start-query \
        --log-group-name "${log_group}" \
        --region "${region}" \
        --start-time $(($(date +%s) - (TIME_RANGE_MINUTES * 60))) \
        --end-time $(date +%s) \
        --query-string "${query}" \
        --query 'queryId' \
        --output text 2>/dev/null)
    
    if [ -z "${query_id}" ]; then
        echo -e "${RED}✗ Failed to start query${NC}"
        return 1
    fi
    
    echo "Query ID: ${query_id}"
    echo "Waiting for results..."
    
    # Wait for query to complete (max 30 seconds)
    local attempts=0
    local max_attempts=30
    while [ ${attempts} -lt ${max_attempts} ]; do
        local status=$(aws logs get-query-results \
            --query-id "${query_id}" \
            --region "${region}" \
            --query 'status' \
            --output text 2>/dev/null)
        
        if [ "${status}" = "Complete" ]; then
            echo -e "${GREEN}✓ Query completed${NC}"
            echo ""
            echo "Results:"
            aws logs get-query-results \
                --query-id "${query_id}" \
                --region "${region}" \
                --query 'results[0]' \
                --output table 2>/dev/null
            return 0
        elif [ "${status}" = "Failed" ]; then
            echo -e "${RED}✗ Query failed${NC}"
            return 1
        fi
        
        sleep 1
        attempts=$((attempts + 1))
    done
    
    echo -e "${YELLOW}⚠ Query timed out${NC}"
    return 1
}

# Main execution
echo "Checking CloudWatch logs across regions..."
echo ""

found_logs=false
for region in "${REGIONS[@]}"; do
    if check_region_logs "${region}"; then
        found_logs=true
        
        # Run insights query for the first region with logs
        echo ""
        run_insights_query "${region}"
        break
    fi
done

echo ""
echo -e "${BLUE}=== Verification Summary ===${NC}"
echo ""

if [ "${found_logs}" = true ]; then
    echo -e "${GREEN}✓ Session lifecycle logs are being generated correctly${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Review the MONITORING.md file for CloudWatch Insights queries"
    echo "  2. Create CloudWatch dashboards using the provided queries"
    echo "  3. Set up alerts for critical session events"
    echo ""
    echo "To view logs in real-time:"
    echo "  aws logs tail /aws/lambda/us-east-1.${FUNCTION_NAME} --follow --region us-east-1"
else
    echo -e "${YELLOW}⚠ No session lifecycle logs found in any region${NC}"
    echo ""
    echo "Possible reasons:"
    echo "  1. No traffic to the application in the last ${TIME_RANGE_MINUTES} minutes"
    echo "  2. Lambda@Edge function not deployed or not attached to CloudFront"
    echo "  3. Function name may be different (current: ${FUNCTION_NAME})"
    echo ""
    echo "To troubleshoot:"
    echo "  1. Verify Lambda@Edge function is deployed"
    echo "  2. Check CloudFront distribution configuration"
    echo "  3. Generate test traffic to the application"
    echo "  4. Run this script with the correct function name:"
    echo "     ./verify-cloudwatch-logs.sh <function-name>"
fi

echo ""
echo "For more information, see:"
echo "  - lambda/saml_auth/MONITORING.md"
echo "  - lambda/saml_auth/README.md"
echo ""
