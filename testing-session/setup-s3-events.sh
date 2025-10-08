#!/bin/bash

# CCOE Customer Contact Manager - S3 Event Notification Setup
# Configures S3 bucket to send notifications to SQS when files are uploaded

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create results directory
mkdir -p "$RESULTS_DIR"

# Log file
LOG_FILE="$RESULTS_DIR/s3-events-setup-$TIMESTAMP.log"

echo "=== CCOE Customer Contact Manager - S3 Event Notification Setup ===" | tee "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Configuration from consolidated config.json
S3_BUCKET=$(jq -r '.s3_config.bucket_name // empty' "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
CUSTOMER_CODE=$(jq -r '.customer_mappings | keys[0] // empty' "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
SQS_QUEUE_ARN=$(jq -r ".customer_mappings[\"$CUSTOMER_CODE\"].sqs_queue_arn // empty" "$PROJECT_ROOT/config.json" 2>/dev/null || echo "")
CUSTOMER_PREFIX="customers/$CUSTOMER_CODE/"

echo "Configuration:" | tee -a "$LOG_FILE"
echo "  S3 Bucket: $S3_BUCKET" | tee -a "$LOG_FILE"
echo "  Customer Code: $CUSTOMER_CODE" | tee -a "$LOG_FILE"
echo "  Customer Prefix: $CUSTOMER_PREFIX" | tee -a "$LOG_FILE"
echo "  SQS Queue ARN: $SQS_QUEUE_ARN" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

if [[ -z "$S3_BUCKET" || -z "$SQS_QUEUE_ARN" ]]; then
    echo "❌ ERROR: Missing required configuration (S3 bucket or SQS queue)" | tee -a "$LOG_FILE"
    exit 1
fi

# Step 1: Check current S3 event configuration
echo "=== Step 1: Check Current S3 Event Configuration ===" | tee -a "$LOG_FILE"
echo "Checking existing event notifications..." | tee -a "$LOG_FILE"
if aws s3api get-bucket-notification-configuration --bucket "$S3_BUCKET" >> "$LOG_FILE" 2>&1; then
    echo "✅ Successfully retrieved current notification configuration" | tee -a "$LOG_FILE"
else
    echo "⚠️  No existing notification configuration found (this is normal)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Step 2: Create SQS queue policy to allow S3 to send messages
echo "=== Step 2: Configure SQS Queue Policy ===" | tee -a "$LOG_FILE"

# Extract queue URL from ARN
REGION=$(echo "$SQS_QUEUE_ARN" | cut -d: -f4)
ACCOUNT=$(echo "$SQS_QUEUE_ARN" | cut -d: -f5)
QUEUE_NAME=$(echo "$SQS_QUEUE_ARN" | cut -d: -f6)
SQS_QUEUE_URL="https://sqs.${REGION}.amazonaws.com/${ACCOUNT}/${QUEUE_NAME}"

echo "  Queue URL: $SQS_QUEUE_URL" | tee -a "$LOG_FILE"

# Create SQS policy that allows S3 to send messages
SQS_POLICY_FILE="$RESULTS_DIR/sqs-policy-$TIMESTAMP.json"
cat > "$SQS_POLICY_FILE" << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowS3ToSendMessages",
      "Effect": "Allow",
      "Principal": {
        "Service": "s3.amazonaws.com"
      },
      "Action": "sqs:SendMessage",
      "Resource": "$SQS_QUEUE_ARN",
      "Condition": {
        "ArnEquals": {
          "aws:SourceArn": "arn:aws:s3:::$S3_BUCKET"
        }
      }
    }
  ]
}
EOF

echo "Setting SQS queue policy to allow S3 notifications..." | tee -a "$LOG_FILE"
if aws sqs set-queue-attributes \
    --queue-url "$SQS_QUEUE_URL" \
    --attributes "Policy=$(cat "$SQS_POLICY_FILE" | jq -c .)" >> "$LOG_FILE" 2>&1; then
    echo "✅ SQS queue policy configured successfully" | tee -a "$LOG_FILE"
else
    echo "❌ Failed to set SQS queue policy" | tee -a "$LOG_FILE"
    exit 1
fi
echo "" | tee -a "$LOG_FILE"

# Step 3: Configure S3 event notification
echo "=== Step 3: Configure S3 Event Notification ===" | tee -a "$LOG_FILE"

# Create S3 notification configuration
S3_NOTIFICATION_FILE="$RESULTS_DIR/s3-notification-$TIMESTAMP.json"
cat > "$S3_NOTIFICATION_FILE" << EOF
{
  "QueueConfigurations": [
    {
      "Id": "CustomerMetadataUpload-$CUSTOMER_CODE",
      "QueueArn": "$SQS_QUEUE_ARN",
      "Events": [
        "s3:ObjectCreated:*"
      ],
      "Filter": {
        "Key": {
          "FilterRules": [
            {
              "Name": "prefix",
              "Value": "$CUSTOMER_PREFIX"
            },
            {
              "Name": "suffix",
              "Value": ".json"
            }
          ]
        }
      }
    }
  ]
}
EOF

echo "Configuring S3 event notification..." | tee -a "$LOG_FILE"
echo "  Event: s3:ObjectCreated:*" | tee -a "$LOG_FILE"
echo "  Prefix: $CUSTOMER_PREFIX" | tee -a "$LOG_FILE"
echo "  Suffix: .json" | tee -a "$LOG_FILE"

if aws s3api put-bucket-notification-configuration \
    --bucket "$S3_BUCKET" \
    --notification-configuration "file://$S3_NOTIFICATION_FILE" >> "$LOG_FILE" 2>&1; then
    echo "✅ S3 event notification configured successfully" | tee -a "$LOG_FILE"
else
    echo "❌ Failed to configure S3 event notification" | tee -a "$LOG_FILE"
    exit 1
fi
echo "" | tee -a "$LOG_FILE"

# Step 4: Verify configuration
echo "=== Step 4: Verify Configuration ===" | tee -a "$LOG_FILE"
echo "Verifying S3 event notification configuration..." | tee -a "$LOG_FILE"
if aws s3api get-bucket-notification-configuration --bucket "$S3_BUCKET" >> "$LOG_FILE" 2>&1; then
    echo "✅ S3 event notification configuration verified" | tee -a "$LOG_FILE"
else
    echo "❌ Failed to verify S3 event notification configuration" | tee -a "$LOG_FILE"
fi

echo "Verifying SQS queue attributes..." | tee -a "$LOG_FILE"
if aws sqs get-queue-attributes --queue-url "$SQS_QUEUE_URL" --attribute-names Policy >> "$LOG_FILE" 2>&1; then
    echo "✅ SQS queue policy verified" | tee -a "$LOG_FILE"
else
    echo "❌ Failed to verify SQS queue policy" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Step 5: Test the configuration
echo "=== Step 5: Test S3 Event Notification ===" | tee -a "$LOG_FILE"
TEST_FILE="$RESULTS_DIR/test-s3-event-$TIMESTAMP.json"
cat > "$TEST_FILE" << EOF
{
  "testId": "s3-event-test-$TIMESTAMP",
  "purpose": "Testing S3 event notification to SQS",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "customer": "$CUSTOMER_CODE",
  "testRun": true
}
EOF

echo "Uploading test file to trigger S3 event..." | tee -a "$LOG_FILE"
TEST_S3_KEY="${CUSTOMER_PREFIX}test-s3-event-$TIMESTAMP.json"
if aws s3 cp "$TEST_FILE" "s3://$S3_BUCKET/$TEST_S3_KEY" >> "$LOG_FILE" 2>&1; then
    echo "✅ Test file uploaded successfully" | tee -a "$LOG_FILE"
    echo "  S3 Key: $TEST_S3_KEY" | tee -a "$LOG_FILE"
    
    # Wait for event processing
    echo "Waiting 15 seconds for S3 event to trigger SQS message..." | tee -a "$LOG_FILE"
    sleep 15
    
    # Check for SQS messages
    MESSAGE_COUNT=$(aws sqs get-queue-attributes --queue-url "$SQS_QUEUE_URL" --attribute-names ApproximateNumberOfMessages --query 'Attributes.ApproximateNumberOfMessages' --output text 2>/dev/null || echo "0")
    echo "  SQS message count after test: $MESSAGE_COUNT" | tee -a "$LOG_FILE"
    
    if [[ "$MESSAGE_COUNT" -gt 0 ]]; then
        echo "🎉 SUCCESS: S3 event triggered SQS message!" | tee -a "$LOG_FILE"
        
        # Peek at the message
        echo "Sample SQS message:" | tee -a "$LOG_FILE"
        aws sqs receive-message --queue-url "$SQS_QUEUE_URL" --max-number-of-messages 1 --visibility-timeout-seconds 1 >> "$LOG_FILE" 2>&1 || true
    else
        echo "⚠️  WARNING: No SQS messages detected after S3 upload" | tee -a "$LOG_FILE"
        echo "   This may indicate a configuration issue or delay in processing" | tee -a "$LOG_FILE"
    fi
    
    # Clean up test file
    aws s3 rm "s3://$S3_BUCKET/$TEST_S3_KEY" >> "$LOG_FILE" 2>&1 || true
    echo "✅ Test file cleaned up" | tee -a "$LOG_FILE"
else
    echo "❌ Failed to upload test file" | tee -a "$LOG_FILE"
fi

rm -f "$TEST_FILE"
echo "" | tee -a "$LOG_FILE"

# Clean up temporary files
rm -f "$SQS_POLICY_FILE" "$S3_NOTIFICATION_FILE"

# Summary
echo "=== S3 Event Notification Setup Summary ===" | tee -a "$LOG_FILE"
echo "Configuration completed at: $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"
echo "✅ SQS queue policy configured to allow S3 notifications" | tee -a "$LOG_FILE"
echo "✅ S3 bucket configured to send events to SQS queue" | tee -a "$LOG_FILE"
echo "✅ Event filter: ${CUSTOMER_PREFIX}*.json" | tee -a "$LOG_FILE"
echo "✅ Target queue: $SQS_QUEUE_ARN" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

if [[ "$MESSAGE_COUNT" -gt 0 ]]; then
    echo "🎉 S3 → SQS event notification is working!" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"
    echo "Next steps:" | tee -a "$LOG_FILE"
    echo "1. Test the complete workflow with the web UI" | tee -a "$LOG_FILE"
    echo "2. Run the functional workflow test again" | tee -a "$LOG_FILE"
    echo "3. Set up additional customer prefixes if needed" | tee -a "$LOG_FILE"
else
    echo "⚠️  S3 → SQS event notification may need troubleshooting" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"
    echo "Troubleshooting steps:" | tee -a "$LOG_FILE"
    echo "1. Check S3 bucket and SQS queue are in the same region" | tee -a "$LOG_FILE"
    echo "2. Verify SQS queue policy allows S3 service" | tee -a "$LOG_FILE"
    echo "3. Check S3 event notification configuration" | tee -a "$LOG_FILE"
    echo "4. Wait longer for event processing (can take up to 5 minutes)" | tee -a "$LOG_FILE"
fi

echo "Completed at: $(date)" | tee -a "$LOG_FILE"