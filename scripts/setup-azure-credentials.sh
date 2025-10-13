#!/bin/bash

# Script to set up Azure credentials in AWS Parameter Store for Microsoft Graph API integration
# This script helps store the required Azure app registration credentials securely

set -e

echo "🔐 Setting up Azure credentials in AWS Parameter Store"
echo "This script will store Azure app registration credentials for Microsoft Graph API integration"
echo ""

# Check if AWS CLI is available
if ! command -v aws &> /dev/null; then
    echo "❌ AWS CLI is not installed or not in PATH"
    exit 1
fi

# Check AWS credentials
if ! aws sts get-caller-identity &> /dev/null; then
    echo "❌ AWS credentials not configured or invalid"
    echo "Please run 'aws configure' or set AWS environment variables"
    exit 1
fi

echo "✅ AWS CLI is configured and working"
echo ""

# Function to securely prompt for input
read_secret() {
    local prompt="$1"
    local var_name="$2"
    echo -n "$prompt: "
    read -s value
    echo ""
    eval "$var_name='$value'"
}

# Prompt for Azure credentials
echo "Please provide your Azure app registration credentials:"
echo "(These will be stored securely in AWS Parameter Store as SecureString parameters)"
echo ""

read_secret "Azure Client ID" AZURE_CLIENT_ID
read_secret "Azure Client Secret" AZURE_CLIENT_SECRET
read_secret "Azure Tenant ID" AZURE_TENANT_ID

echo ""
echo "🔍 Validating input..."

# Basic validation
if [[ -z "$AZURE_CLIENT_ID" || -z "$AZURE_CLIENT_SECRET" || -z "$AZURE_TENANT_ID" ]]; then
    echo "❌ All three credentials are required"
    exit 1
fi

# Validate GUID format for Client ID and Tenant ID
if [[ ! "$AZURE_CLIENT_ID" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; then
    echo "❌ Azure Client ID should be in GUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)"
    exit 1
fi

if [[ ! "$AZURE_TENANT_ID" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; then
    echo "❌ Azure Tenant ID should be in GUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)"
    exit 1
fi

echo "✅ Input validation passed"
echo ""

# Store parameters in AWS Parameter Store
echo "📝 Storing parameters in AWS Parameter Store..."

# Store Client ID (not sensitive)
echo "Storing Azure Client ID..."
aws ssm put-parameter \
    --name "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_ID" \
    --value "$AZURE_CLIENT_ID" \
    --type "String" \
    --overwrite \
    --description "Azure App Registration Client ID for Microsoft Graph API"

# Store Client Secret (sensitive - use SecureString)
echo "Storing Azure Client Secret..."
aws ssm put-parameter \
    --name "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_SECRET" \
    --value "$AZURE_CLIENT_SECRET" \
    --type "SecureString" \
    --overwrite \
    --description "Azure App Registration Client Secret for Microsoft Graph API"

# Store Tenant ID (not sensitive)
echo "Storing Azure Tenant ID..."
aws ssm put-parameter \
    --name "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_TENANT_ID" \
    --value "$AZURE_TENANT_ID" \
    --type "String" \
    --overwrite \
    --description "Azure Tenant ID for Microsoft Graph API"

echo ""
echo "✅ Successfully stored all Azure credentials in Parameter Store"
echo ""
echo "📋 Stored parameters:"
echo "  - /hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_ID"
echo "  - /hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_SECRET"
echo "  - /hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_TENANT_ID"
echo ""
echo "🔒 Client Secret is encrypted using AWS KMS (SecureString)"
echo "📋 Client ID and Tenant ID are stored as regular String parameters"
echo ""
echo "🧪 You can now test the meeting functionality using:"
echo "  ./ccoe-customer-contact-manager create-multi-customer-meeting-invite \\"
echo "    --customer-codes hts,htsnonprod \\"
echo "    --topic-name aws-calendar \\"
echo "    --json-metadata test-multi-customer-meeting-metadata.json \\"
echo "    --sender-email your-email@hearst.com \\"
echo "    --dry-run"
echo ""
echo "🚀 For Lambda mode, the credentials will be automatically loaded from Parameter Store"

# Clear variables for security
unset AZURE_CLIENT_ID
unset AZURE_CLIENT_SECRET
unset AZURE_TENANT_ID