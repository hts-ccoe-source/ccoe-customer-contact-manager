#!/bin/bash

# Deploy Website to S3
# This script deploys the html/ directory contents to the root of the S3 bucket

set -e

# Configuration
S3_BUCKET_NAME="${S3_BUCKET_NAME:-4cm-prod-ccoe-change-management-metadata}"
CLOUDFRONT_DISTRIBUTION_ID="${CLOUDFRONT_DISTRIBUTION_ID:-E3EC4FE9RZANXB}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

echo_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if html directory exists
if [ ! -d "html" ]; then
    echo_error "HTML directory not found: html/"
    echo_error "Please ensure you're running this script from the project root"
    exit 1
fi

# Check if AWS CLI is configured
if ! aws sts get-caller-identity >/dev/null 2>&1; then
    echo_error "AWS CLI not configured or no valid credentials found"
    exit 1
fi

echo_info "Starting website deployment..."
echo_info "Source: html/ directory"
echo_info "Target: s3://$S3_BUCKET_NAME/ (root)"

# Deploy HTML files to S3 root
# Note: We don't use --delete because this bucket also contains
# customer change archives, drafts, and other important data
echo_info "Step 1: Uploading website files to S3..."
echo_info "Running: aws s3 sync html/ s3://$S3_BUCKET_NAME/ --cache-control 'no-cache, no-store, must-revalidate'"
echo ""

aws s3 sync html/ s3://$S3_BUCKET_NAME/ \
    --cache-control "no-cache, no-store, must-revalidate" \
    --exclude "*.DS_Store" \
    --exclude "*.git*"

SYNC_EXIT_CODE=$?
echo ""

if [ $SYNC_EXIT_CODE -eq 0 ]; then
    echo_success "S3 sync command completed (exit code: 0)"
    echo_info "Note: If no files are listed above, it means all files are already up-to-date"
else
    echo_error "Failed to upload website files (exit code: $SYNC_EXIT_CODE)"
    exit 1
fi

# Invalidate CloudFront cache
if [ -n "$CLOUDFRONT_DISTRIBUTION_ID" ]; then
    echo_info "Step 2: Invalidating CloudFront cache..."
    echo_info "Distribution ID: $CLOUDFRONT_DISTRIBUTION_ID"
    echo_info "Paths: /*.html /assets/*"
    
    # Use timeout to prevent hanging (30 second timeout)
    if command -v timeout >/dev/null 2>&1; then
        INVALIDATION_OUTPUT=$(timeout 30 aws cloudfront create-invalidation \
            --distribution-id "$CLOUDFRONT_DISTRIBUTION_ID" \
            --paths "/*.html" "/assets/*" \
            --query 'Invalidation.Id' \
            --output text \
            2>&1)
        INVALIDATION_EXIT_CODE=$?
    else
        # macOS doesn't have timeout by default, use gtimeout if available
        if command -v gtimeout >/dev/null 2>&1; then
            INVALIDATION_OUTPUT=$(gtimeout 30 aws cloudfront create-invalidation \
                --distribution-id "$CLOUDFRONT_DISTRIBUTION_ID" \
                --paths "/*.html" "/assets/*" \
                --query 'Invalidation.Id' \
                --output text \
                2>&1)
            INVALIDATION_EXIT_CODE=$?
        else
            # No timeout available, run without it
            INVALIDATION_OUTPUT=$(aws cloudfront create-invalidation \
                --distribution-id "$CLOUDFRONT_DISTRIBUTION_ID" \
                --paths "/*.html" "/assets/*" \
                --query 'Invalidation.Id' \
                --output text \
                2>&1)
            INVALIDATION_EXIT_CODE=$?
        fi
    fi
    
    if [ $INVALIDATION_EXIT_CODE -eq 0 ] && [ -n "$INVALIDATION_OUTPUT" ] && [ "$INVALIDATION_OUTPUT" != "None" ]; then
        echo_success "CloudFront invalidation created: $INVALIDATION_OUTPUT"
        echo_info "Cache invalidation may take 5-15 minutes to complete"
        echo_info "Check status: aws cloudfront get-invalidation --distribution-id $CLOUDFRONT_DISTRIBUTION_ID --id $INVALIDATION_OUTPUT"
    elif [ $INVALIDATION_EXIT_CODE -eq 124 ]; then
        echo_error "CloudFront invalidation timed out after 30 seconds"
        echo_warn "The invalidation may still be processing. Check AWS Console to verify."
    else
        echo_error "Failed to create CloudFront invalidation (exit code: $INVALIDATION_EXIT_CODE)"
        echo_error "Output: $INVALIDATION_OUTPUT"
        echo_warn "You may need to manually invalidate the cache in the AWS Console"
        echo_info "Manual command: aws cloudfront create-invalidation --distribution-id $CLOUDFRONT_DISTRIBUTION_ID --paths '/*.html' '/assets/*'"
    fi
else
    echo_warn "CLOUDFRONT_DISTRIBUTION_ID not set, skipping cache invalidation"
fi

# Verify deployment
echo_info "Step 3: Verifying deployment..."

S3_URL="https://$S3_BUCKET_NAME.s3.amazonaws.com/index.html"
echo_info "S3 Direct URL: $S3_URL"

if [ -n "$CLOUDFRONT_DISTRIBUTION_ID" ]; then
    # Try to get the CloudFront domain
    CF_DOMAIN=$(aws cloudfront get-distribution \
        --id $CLOUDFRONT_DISTRIBUTION_ID \
        --query 'Distribution.DomainName' \
        --output text 2>/dev/null)
    
    if [ $? -eq 0 ] && [ "$CF_DOMAIN" != "None" ]; then
        echo_info "CloudFront URL: https://$CF_DOMAIN/"
    fi
fi

echo_success "Website deployment completed!"
echo_info "The website is now available at the URLs above."

# Optional: Clean up old website files (use with caution)
# Uncomment the following function if you need to remove old HTML files
# clean_old_website_files() {
#     echo_warn "Cleaning up old website files..."
#     echo_warn "This will remove specific HTML files from S3 root"
#     
#     # List of known website files to remove
#     OLD_FILES=(
#         "metadata-collector-multi-customer.html"
#         "metadata-collector.html" 
#         "metadata-collector-enhanced.html"
#     )
#     
#     for file in "${OLD_FILES[@]}"; do
#         if aws s3 ls "s3://$S3_BUCKET_NAME/$file" >/dev/null 2>&1; then
#             echo_info "Removing old file: $file"
#             aws s3 rm "s3://$S3_BUCKET_NAME/$file"
#         fi
#     done
# }
# 
# # Uncomment to run cleanup
# # clean_old_website_files