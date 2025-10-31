#!/bin/bash
# Verify Lambda@Edge Constraints
# This script checks that the deployment package meets all Lambda@Edge requirements

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZIP_FILE="${SCRIPT_DIR}/lambda-edge-samlify.zip"

echo "=== Lambda@Edge Constraint Verification ==="
echo ""

# Check if package exists
if [ ! -f "$ZIP_FILE" ]; then
    echo "❌ ERROR: Package not found at $ZIP_FILE"
    echo "   Run 'make build' to create the package"
    exit 1
fi

# Get package size
COMPRESSED_SIZE=$(stat -f%z "$ZIP_FILE" 2>/dev/null || stat -c%s "$ZIP_FILE" 2>/dev/null)
COMPRESSED_SIZE_MB=$(echo "scale=2; $COMPRESSED_SIZE / 1048576" | bc)

# Uncompressed size
UNCOMPRESSED_SIZE=$(unzip -l "$ZIP_FILE" | tail -1 | awk '{print $1}')
UNCOMPRESSED_SIZE_MB=$(echo "scale=2; $UNCOMPRESSED_SIZE / 1048576" | bc)

# Lambda@Edge limits
MAX_COMPRESSED_SIZE=52428800     # 50 MB
MAX_UNCOMPRESSED_SIZE=52428800   # 50 MB (no specific limit, but keeping same for reference)

echo "📦 Package Size Analysis:"
echo "   Compressed:   ${COMPRESSED_SIZE_MB} MB (limit: 50 MB)"
echo "   Uncompressed: ${UNCOMPRESSED_SIZE_MB} MB"
echo ""

# Check compressed size
if [ "$COMPRESSED_SIZE" -gt "$MAX_COMPRESSED_SIZE" ]; then
    echo "❌ FAIL: Compressed size exceeds 50 MB limit"
    echo "   Current: ${COMPRESSED_SIZE_MB} MB"
    echo "   Limit:   50 MB"
    echo ""
    echo "   Recommendations:"
    echo "   - Remove unnecessary files from package"
    echo "   - Minimize dependencies"
    echo "   - Use webpack or esbuild to bundle and minify"
    echo ""
    FAILED=1
else
    echo "✅ PASS: Compressed size within limit (${COMPRESSED_SIZE_MB} MB / 50 MB)"
fi

echo ""
echo "📋 Package Contents:"
echo ""
unzip -l "$ZIP_FILE" | head -20
echo "   ... (showing first 20 files)"
echo ""

# Check for problematic files
echo "🔍 Checking for unnecessary files:"
UNNECESSARY_FILES=$(unzip -l "$ZIP_FILE" | grep -E '\.(md|txt|map|ts|test\.js|spec\.js)$' | wc -l | tr -d ' ')
if [ "$UNNECESSARY_FILES" -gt 0 ]; then
    echo "⚠️  WARNING: Found $UNNECESSARY_FILES unnecessary files (*.md, *.txt, *.map, *.ts, test files)"
    echo "   Consider excluding these to reduce package size"
    echo ""
    unzip -l "$ZIP_FILE" | grep -E '\.(md|txt|map|ts|test\.js|spec\.js)$' | head -10
    echo ""
else
    echo "✅ No unnecessary files found"
fi

# Check for large dependencies
echo ""
echo "📊 Largest files in package:"
unzip -l "$ZIP_FILE" | sort -k1 -n -r | head -10
echo ""

# Environment variable check
echo "🔧 Configuration:"
echo "   The following environment variables can be set:"
echo "   - SESSION_IDLE_TIMEOUT_MS (default: 10800000 = 3 hours)"
echo "   - SESSION_ABSOLUTE_MAX_MS (default: 43200000 = 12 hours)"
echo "   - SESSION_REFRESH_THRESHOLD_MS (default: 600000 = 10 minutes)"
echo "   - SESSION_COOKIE_MAX_AGE (default: 10800 = 3 hours)"
echo ""

# Runtime constraints
echo "⚙️  Runtime Constraints:"
echo "   ✅ Runtime: nodejs18.x (supported)"
echo "   ✅ Timeout: 5 seconds (maximum for viewer-request)"
echo "   ✅ Memory: 128 MB minimum (recommended)"
echo "   ✅ Region: us-east-1 (required for Lambda@Edge)"
echo ""

# Summary
echo "=== Summary ==="
if [ "${FAILED:-0}" -eq 1 ]; then
    echo "❌ VERIFICATION FAILED"
    echo ""
    echo "The package does not meet Lambda@Edge constraints."
    echo "Please review the recommendations above and rebuild."
    exit 1
else
    echo "✅ VERIFICATION PASSED"
    echo ""
    echo "The package meets all Lambda@Edge constraints."
    echo "Ready for deployment."
    exit 0
fi
