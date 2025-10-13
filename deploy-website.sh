#!/bin/bash

# Deploy Website to S3
# This script deploys the html/ directory contents to the root of the S3 bucket

set -e

# Configuration
S3_BUCKET_NAME="${S3_BUCKET_NAME:-4cm-prod-ccoe-change-management-metadata}"
CLOUDFRONT_DISTRIBUTION_ID="${CLOUDFRONT_DISTRIBUTION_ID:-E3DIDLE5N99NVJ}"

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
aws s3 sync html/ s3://$S3_BUCKET_NAME/ \
    --cache-control "no-cache, no-store, must-revalidate" \
    --exclude "*.DS_Store" \
    --exclude "*.git*"

if [ $? -eq 0 ]; then
    echo_success "Website files uploaded successfully"
else
    echo_error "Failed to upload website files"
    exit 1
fi

# Invalidate CloudFront cache
if [ -n "$CLOUDFRONT_DISTRIBUTION_ID" ]; then
    echo_info "Step 2: Invalidating CloudFront cache..."
    
    INVALIDATION_ID=$(aws cloudfront create-invalidation \
        --distribution-id $CLOUDFRONT_DISTRIBUTION_ID \
        --paths "/*.html" "/assets/*" \
        --query 'Invalidation.Id' \
        --output text \
        2>/dev/null)
    
    if [ $? -eq 0 ]; then
        echo_success "CloudFront invalidation created: $INVALIDATION_ID"
        echo_info "Cache invalidation may take 5-15 minutes to complete"
    else
        echo_warn "Failed to create CloudFront invalidation"
        echo_warn "You may need to manually invalidate the cache"
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
echo_info ""
echo_info "Files deployed:"
echo_info "  - index.html (Dashboard)"
echo_info "  - create-change.html (Create Change)"
echo_info "  - my-changes.html (My Changes)"
echo_info "  - view-changes.html (View Changes)"
echo_info "  - search-changes.html (Search)"
echo_info "  - edit-change.html (Edit Change)"
echo_info "  - assets/ (CSS and JavaScript)"
echo_info ""
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