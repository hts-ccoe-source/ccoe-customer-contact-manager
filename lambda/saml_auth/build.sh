#!/bin/bash

# Build script for Lambda@Edge SAML authentication function
# This script creates the deployment package for AWS Lambda@Edge

set -e

echo "Building Lambda@Edge SAML authentication package..."

# Clean up any existing zip file
rm -f lambda-edge-samlify.zip

# Install dependencies (if node_modules doesn't exist or package.json changed)
if [ ! -d "node_modules" ] || [ "package.json" -nt "node_modules" ]; then
    echo "Installing dependencies..."
    npm install --production
fi

# Create deployment package
echo "Creating deployment package..."
zip -q -r lambda-edge-samlify.zip . -x "*.sh" "README.md" "Makefile" ".git/*" "*.zip" "*.pem" "*.xml"

echo "✅ Lambda@Edge package created: lambda-edge-samlify.zip"
echo "Package size: $(du -h lambda-edge-samlify.zip | cut -f1)"

# Verify the package contents (commented out for quiet operation)
# echo "Package contents:"
# unzip -l lambda-edge-samlify.zip