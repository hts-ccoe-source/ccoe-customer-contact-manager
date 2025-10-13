#!/bin/bash

# Build script for Upload Metadata Lambda function
# This script creates the deployment package for AWS Lambda

set -e

echo "Building Upload Metadata Lambda package..."

# Clean up any existing zip file
rm -f upload-metadata-lambda.zip

# Install dependencies (if node_modules doesn't exist or package.json changed)
if [ ! -d "node_modules" ] || [ "package.json" -nt "node_modules" ]; then
    echo "Installing dependencies..."
    npm install --production
fi

# Create deployment package
echo "Creating deployment package..."
zip -q -r upload-metadata-lambda.zip . -x "*.sh" "README.md" "Makefile" ".git/*" "*.zip"

echo "✅ Upload Metadata Lambda package created: upload-metadata-lambda.zip"
echo "Package size: $(du -h upload-metadata-lambda.zip | cut -f1)"

# Verify the package contents (commented out for quiet operation)
# echo "Package contents:"
# unzip -l upload-metadata-lambda.zip