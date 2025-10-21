# Transient Trigger Pattern - Troubleshooting Guide

## Overview

This guide provides detailed troubleshooting procedures for common issues with the Transient Trigger Pattern architecture.

## Quick Diagnostic Commands

```bash
# Set your change ID and customer code
CHANGE_ID="your-change-id"
CUSTOMER_CODE="hts"

# Check if change exists in archive (should always exist)
aws s3 ls s3://metadata-bucket/archive/${CHANGE_ID}.json

# Check if trigger still exists (should be deleted after processing)
aws s3 ls s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# Check SQS queue depth
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --attribute-names ApproximateNumberOfMessages,ApproximateNumberOfMessagesNotVisible

# Check recent processing logs
aws logs tail /ecs/change-processor --follow --filter-pattern "${CHANGE_ID}"
```

## Common Issues and Solutions

### Issue 1: Trigger Not Deleted After Processing

**Symptoms:**
- Trigger file exists in `customers/{customer-code}/{changeId}.json`
- Processing entry exists in archive modification array
- No errors in CloudWatch logs

**Diagnosis:**

```bash
# Check archive for processing entry
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.modifications[] | select(.modificationType == "processed")'

# Check if trigger still exists
aws s3 ls s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# Check logs for delete operation
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID} delete trigger" \
  --start-time $(date -d '24 hours ago' +%s)000
```

**Root Causes:**

1. **Non-fatal delete failure**: Backend logged warning but continued
2. **IAM permission issue**: Backend lacks s3:DeleteObject permission
3. **S3 eventual consistency**: Delete operation succeeded but not yet visible

**Resolution:**

```bash
# Option 1: Manual cleanup (safe - processing is complete)
aws s3 rm s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# Option 2: Verify IAM permissions
aws iam get-role-policy \
  --role-name change-processor-role \
  --policy-name s3-access | \
  jq '.PolicyDocument.Statement[] | select(.Action[] | contains("DeleteObject"))'

# Option 3: Wait for eventual consistency (if recent)
# Check again in 1-2 minutes
```

**Prevention:**
- Monitor trigger deletion success rate
- Alert on triggers older than 15 minutes
- Ensure IAM policies include s3:DeleteObject

### Issue 2: Archive Update Failed

**Symptoms:**
- Trigger deleted but no processing entry in archive
- SQS message returned to queue
- Error logs show "failed to update archive"

**Diagnosis:**

```bash
# Check archive state
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | jq .

# Check for processing entries
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.modifications[] | select(.modificationType == "processed")'

# Check error logs
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID} failed to update archive" \
  --start-time $(date -d '24 hours ago' +%s)000
```

**Root Causes:**

1. **IAM permission issue**: Backend lacks s3:PutObject permission
2. **S3 bucket policy**: Bucket policy denies PutObject
3. **Network issue**: Temporary connectivity problem
4. **Malformed data**: Invalid JSON structure

**Resolution:**

```bash
# Check IAM permissions
aws iam get-role-policy \
  --role-name change-processor-role \
  --policy-name s3-access | \
  jq '.PolicyDocument.Statement[] | select(.Action[] | contains("PutObject"))'

# Check S3 bucket policy
aws s3api get-bucket-policy --bucket metadata-bucket | \
  jq '.Policy | fromjson'

# Manually retry processing
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-message \
  --change-id ${CHANGE_ID}

# Verify archive updated
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.modifications[] | select(.modificationType == "processed")'
```

**Prevention:**
- Monitor archive update success rate (should be > 99%)
- Alert on repeated archive update failures
- Implement retry logic with exponential backoff

### Issue 3: Duplicate Processing Attempts

**Symptoms:**
- Multiple processing entries in archive modification array
- Duplicate emails sent to recipients
- Multiple SQS events for same change

**Diagnosis:**

```bash
# Check modification array for duplicates
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.modifications[] | select(.modificationType == "processed") | .timestamp'

# Check SQS queue for duplicate messages
aws sqs receive-message \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --max-number-of-messages 10 | \
  jq '.Messages[] | select(.Body | contains("'${CHANGE_ID}'"))'

# Check logs for idempotency checks
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID} idempotency" \
  --start-time $(date -d '24 hours ago' +%s)000
```

**Root Causes:**

1. **Idempotency check not implemented**: Backend doesn't check trigger existence
2. **Race condition**: Multiple workers processing same message
3. **SQS visibility timeout too short**: Message returned to queue before processing complete
4. **Trigger not deleted**: Backend failed to delete trigger after processing

**Resolution:**

```bash
# Verify idempotency check is implemented
# Check backend code for trigger existence check before processing

# Adjust SQS visibility timeout
aws sqs set-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --attributes VisibilityTimeout=900  # 15 minutes

# Clean up duplicate processing entries (if needed)
# Download archive, manually edit to remove duplicates, re-upload
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json /tmp/${CHANGE_ID}.json
# Edit /tmp/${CHANGE_ID}.json to remove duplicate entries
aws s3 cp /tmp/${CHANGE_ID}.json s3://metadata-bucket/archive/${CHANGE_ID}.json
```

**Prevention:**
- Ensure idempotency check is always performed
- Set appropriate SQS visibility timeout (15 minutes recommended)
- Monitor for duplicate processing entries
- Use meeting idempotency keys for Graph API calls

### Issue 4: Trigger Created But No SQS Message

**Symptoms:**
- Trigger file exists in `customers/{customer-code}/`
- No corresponding SQS message in queue
- S3 event notification not triggered

**Diagnosis:**

```bash
# Check if trigger exists
aws s3 ls s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# Check SQS queue
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --attribute-names All

# Check S3 event notification configuration
aws s3api get-bucket-notification-configuration \
  --bucket metadata-bucket | \
  jq '.QueueConfigurations[] | select(.QueueArn | contains("'${CUSTOMER_CODE}'"))'

# Check S3 event logs (if enabled)
aws s3api get-bucket-logging --bucket metadata-bucket
```

**Root Causes:**

1. **S3 event notification not configured**: Missing or incorrect configuration
2. **Filter mismatch**: Trigger doesn't match prefix/suffix filters
3. **SQS queue policy**: Queue doesn't allow S3 to send messages
4. **S3 event notification disabled**: Configuration exists but disabled

**Resolution:**

```bash
# Verify S3 event notification configuration
aws s3api get-bucket-notification-configuration --bucket metadata-bucket

# Check SQS queue policy
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --attribute-names Policy | \
  jq '.Attributes.Policy | fromjson'

# Manually send test message to verify queue works
aws sqs send-message \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --message-body '{"test": "message"}'

# Re-upload trigger to force new S3 event
aws s3 cp s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json /tmp/
aws s3 cp /tmp/${CHANGE_ID}.json s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json
```

**Prevention:**
- Validate S3 event notification configuration during deployment
- Monitor S3 event delivery metrics
- Test event notifications after configuration changes

### Issue 5: Archive Object Not Found

**Symptoms:**
- Backend receives SQS event
- Trigger exists in `customers/{customer-code}/`
- Archive object missing from `archive/`
- Error logs show "archive not found"

**Diagnosis:**

```bash
# Check if archive exists
aws s3 ls s3://metadata-bucket/archive/${CHANGE_ID}.json

# Check if trigger exists
aws s3 ls s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# Check frontend upload logs
aws logs filter-log-events \
  --log-group-name /aws/lambda/upload-metadata \
  --filter-pattern "${CHANGE_ID}" \
  --start-time $(date -d '24 hours ago' +%s)000

# Check S3 access logs (if enabled)
aws s3 ls s3://logging-bucket/metadata-bucket-logs/ | grep ${CHANGE_ID}
```

**Root Causes:**

1. **Frontend upload failure**: Archive upload failed but trigger upload succeeded
2. **Accidental deletion**: Archive object deleted manually or by automation
3. **S3 replication lag**: Archive in different region not yet replicated
4. **Wrong bucket**: Archive uploaded to wrong S3 bucket

**Resolution:**

```bash
# Check if archive exists in backup
BACKUP_DATE=$(date -d '1 day ago' +%Y%m%d)
aws s3 ls s3://backup-bucket/archive-${BACKUP_DATE}/${CHANGE_ID}.json

# Restore from backup if found
aws s3 cp s3://backup-bucket/archive-${BACKUP_DATE}/${CHANGE_ID}.json \
  s3://metadata-bucket/archive/${CHANGE_ID}.json

# If no backup, reconstruct from trigger (last resort)
aws s3 cp s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json \
  s3://metadata-bucket/archive/${CHANGE_ID}.json

# Delete trigger to prevent further processing attempts
aws s3 rm s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json
```

**Prevention:**
- Implement atomic upload (archive first, then triggers)
- Enable S3 versioning on archive/ prefix
- Regular backups of archive/ prefix
- Monitor archive upload success rate

### Issue 6: Meeting Not Created

**Symptoms:**
- Change processed successfully
- No meeting metadata in archive
- No meeting invite received by recipients
- Logs show "meeting creation failed"

**Diagnosis:**

```bash
# Check archive for meeting metadata
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.meetingMetadata'

# Check for meeting_scheduled modification entry
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.modifications[] | select(.modificationType == "meeting_scheduled")'

# Check Graph API logs
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID} Graph API" \
  --start-time $(date -d '24 hours ago' +%s)000

# Test Graph API credentials
./ccoe-customer-contact-manager \
  --action validate-graph-credentials
```

**Root Causes:**

1. **Graph API authentication failure**: Invalid or expired credentials
2. **Insufficient permissions**: App registration lacks Calendar.ReadWrite permission
3. **Recipient list empty**: No subscribers to aws-calendar topic
4. **Meeting time invalid**: Start time in the past or invalid format
5. **Idempotency key conflict**: Meeting already exists with same changeId

**Resolution:**

```bash
# Verify Graph API credentials
./ccoe-customer-contact-manager \
  --action validate-graph-credentials

# Check aws-calendar topic subscribers
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action describe-topic \
  --topic aws-calendar

# Manually create meeting
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action create-meeting-invite \
  --change-id ${CHANGE_ID} \
  --dry-run

# Check for existing meeting with same idempotency key
# Query Microsoft Graph API for meetings with changeId in subject or body
```

**Prevention:**
- Monitor Graph API authentication success rate
- Alert on meeting creation failures
- Validate meeting time before creation
- Test Graph API credentials regularly

### Issue 7: SQS Messages Backing Up

**Symptoms:**
- SQS queue depth increasing
- Messages not being processed
- ECS tasks not running or failing

**Diagnosis:**

```bash
# Check queue depth
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --attribute-names ApproximateNumberOfMessages,ApproximateNumberOfMessagesNotVisible,ApproximateNumberOfMessagesDelayed

# Check ECS service status
aws ecs describe-services \
  --cluster change-processor-cluster \
  --services customer-${CUSTOMER_CODE}-processor

# Check running tasks
aws ecs list-tasks \
  --cluster change-processor-cluster \
  --service-name customer-${CUSTOMER_CODE}-processor

# Check task failures
aws ecs describe-tasks \
  --cluster change-processor-cluster \
  --tasks $(aws ecs list-tasks --cluster change-processor-cluster --service-name customer-${CUSTOMER_CODE}-processor --query 'taskArns[0]' --output text)
```

**Root Causes:**

1. **ECS service scaled to zero**: No tasks running to process messages
2. **Task failures**: Tasks crashing or failing health checks
3. **IAM permission issues**: Tasks can't assume customer roles
4. **Network issues**: Tasks can't reach S3 or SQS
5. **Resource constraints**: Tasks out of CPU or memory

**Resolution:**

```bash
# Scale up ECS service
aws ecs update-service \
  --cluster change-processor-cluster \
  --service customer-${CUSTOMER_CODE}-processor \
  --desired-count 2

# Force new deployment
aws ecs update-service \
  --cluster change-processor-cluster \
  --service customer-${CUSTOMER_CODE}-processor \
  --force-new-deployment

# Check task logs for errors
aws logs tail /ecs/change-processor --follow

# Manually process messages (emergency)
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-queue \
  --max-messages 10
```

**Prevention:**
- Monitor ECS service desired vs running task count
- Alert on queue depth > 10 messages
- Implement auto-scaling based on queue depth
- Regular health checks on ECS tasks

## Debugging Techniques

### Trace a Change Through the System

```bash
CHANGE_ID="your-change-id"
CUSTOMER_CODE="hts"

echo "=== Step 1: Check Archive (Source of Truth) ==="
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | jq .

echo "=== Step 2: Check Trigger Status ==="
aws s3 ls s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

echo "=== Step 3: Check SQS Queue ==="
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --attribute-names All

echo "=== Step 4: Check Processing Logs ==="
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID}" \
  --start-time $(date -d '24 hours ago' +%s)000 | \
  jq '.events[] | .message' -r

echo "=== Step 5: Check Modification History ==="
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.modifications[] | {timestamp, type: .modificationType, customer: .customerCode}'

echo "=== Step 6: Check Meeting Metadata (if applicable) ==="
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.meetingMetadata'
```

### Verify Idempotency

```bash
# Simulate duplicate SQS event
CHANGE_ID="test-change-id"
CUSTOMER_CODE="hts"

# First processing attempt
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-message \
  --change-id ${CHANGE_ID}

# Check trigger deleted
aws s3 ls s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json
# Should return "Not Found"

# Second processing attempt (should skip)
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-message \
  --change-id ${CHANGE_ID}

# Check logs for "already processed" message
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID} already processed" \
  --start-time $(date -d '5 minutes ago' +%s)000
```

### Test Archive-First Loading

```bash
# Verify backend always loads from archive
CHANGE_ID="test-change-id"
CUSTOMER_CODE="hts"

# Modify trigger file (should be ignored)
aws s3 cp s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json /tmp/trigger.json
jq '.changeMetadata.title = "MODIFIED TRIGGER"' /tmp/trigger.json > /tmp/trigger-modified.json
aws s3 cp /tmp/trigger-modified.json s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# Process change
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-message \
  --change-id ${CHANGE_ID}

# Check logs - should show original title from archive, not "MODIFIED TRIGGER"
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID}" \
  --start-time $(date -d '5 minutes ago' +%s)000 | \
  grep -i "title"
```

## Performance Troubleshooting

### Slow Processing

```bash
# Check processing duration
aws cloudwatch get-metric-statistics \
  --namespace CustomMetrics \
  --metric-name ProcessingDuration \
  --dimensions Name=CustomerCode,Value=${CUSTOMER_CODE} \
  --start-time $(date -d '24 hours ago' --iso-8601) \
  --end-time $(date --iso-8601) \
  --period 3600 \
  --statistics Average,Maximum

# Check ECS task resource utilization
aws cloudwatch get-metric-statistics \
  --namespace AWS/ECS \
  --metric-name CPUUtilization \
  --dimensions Name=ServiceName,Value=customer-${CUSTOMER_CODE}-processor \
  --start-time $(date -d '24 hours ago' --iso-8601) \
  --end-time $(date --iso-8601) \
  --period 3600 \
  --statistics Average,Maximum

# Profile specific change processing
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-message \
  --change-id ${CHANGE_ID} \
  --profile
```

### High Error Rate

```bash
# Check error rate by type
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "ERROR" \
  --start-time $(date -d '24 hours ago' +%s)000 | \
  jq '.events[] | .message' -r | \
  grep -oP 'ERROR: \K[^:]+' | \
  sort | uniq -c | sort -rn

# Check dead letter queue
aws sqs receive-message \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes-dlq \
  --max-number-of-messages 10 | \
  jq '.Messages[] | {messageId, body: .Body | fromjson}'
```

## Emergency Procedures

### Complete System Reset

**WARNING: Only use in emergency situations with approval**

```bash
# 1. Stop all processing
for customer in hts htsnonprod customer-a customer-b; do
  aws ecs update-service \
    --cluster change-processor-cluster \
    --service customer-${customer}-processor \
    --desired-count 0
done

# 2. Purge all SQS queues
for customer in hts htsnonprod customer-a customer-b; do
  aws sqs purge-queue \
    --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${customer}-changes
done

# 3. Clean up all triggers
aws s3 rm s3://metadata-bucket/customers/ --recursive

# 4. Restart processing
for customer in hts htsnonprod customer-a customer-b; do
  aws ecs update-service \
    --cluster change-processor-cluster \
    --service customer-${customer}-processor \
    --desired-count 1
done
```

### Manual Reprocessing

```bash
# Reprocess specific change for specific customer
CHANGE_ID="change-to-reprocess"
CUSTOMER_CODE="hts"

# 1. Verify archive exists
aws s3 ls s3://metadata-bucket/archive/${CHANGE_ID}.json

# 2. Recreate trigger
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json \
  s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# 3. Wait for automatic processing or manually trigger
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-message \
  --change-id ${CHANGE_ID}
```

## Related Documentation

- [Architecture Overview](./TRANSIENT_TRIGGER_PATTERN.md)
- [Operational Runbook](./TRANSIENT_TRIGGER_RUNBOOK.md)
- [FAQ](./TRANSIENT_TRIGGER_FAQ.md)
- [Customer Onboarding](./CUSTOMER_ONBOARDING.md)
