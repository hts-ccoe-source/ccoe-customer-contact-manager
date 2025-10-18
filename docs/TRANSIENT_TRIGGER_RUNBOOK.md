# Transient Trigger Pattern - Operational Runbook

## Overview

This runbook provides operational procedures for managing the Transient Trigger Pattern architecture in the multi-customer email distribution system.

## Daily Operations

### Health Check Procedures

#### 1. Check System Health

```bash
# Check SQS queue depths (should be near zero during normal operation)
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-hts-changes \
  --attribute-names ApproximateNumberOfMessages ApproximateNumberOfMessagesNotVisible

# Check for old triggers (should return empty or very few results)
aws s3 ls s3://metadata-bucket/customers/ --recursive | \
  awk '{if ($1 < "'$(date -d '15 minutes ago' '+%Y-%m-%d')'" || ($1 == "'$(date -d '15 minutes ago' '+%Y-%m-%d')'" && $2 < "'$(date -d '15 minutes ago' '+%H:%M:%S')'")) print}'

# Check archive/ for recent changes
aws s3 ls s3://metadata-bucket/archive/ --recursive | tail -20
```

#### 2. Verify Processing Pipeline

```bash
# Check ECS task status for each customer
aws ecs describe-services \
  --cluster change-processor-cluster \
  --services customer-hts-processor customer-htsnonprod-processor

# Check CloudWatch logs for errors
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "ERROR" \
  --start-time $(date -d '1 hour ago' +%s)000
```

#### 3. Monitor Key Metrics

```bash
# Check trigger creation vs deletion rates (should be balanced)
aws cloudwatch get-metric-statistics \
  --namespace CustomMetrics \
  --metric-name TriggerCreationRate \
  --start-time $(date -d '1 hour ago' --iso-8601) \
  --end-time $(date --iso-8601) \
  --period 300 \
  --statistics Sum

aws cloudwatch get-metric-statistics \
  --namespace CustomMetrics \
  --metric-name TriggerDeletionRate \
  --start-time $(date -d '1 hour ago' --iso-8601) \
  --end-time $(date --iso-8601) \
  --period 300 \
  --statistics Sum
```

## Common Operational Tasks

### Task 1: Submit a New Change

**Procedure:**

1. Access https://change-management.ccoe.hearst.com
2. Navigate to "Create Change"
3. Fill out change details and select affected customers
4. Click "Submit Change" (not "Save Draft")
5. Verify upload progress shows success for all customers
6. Confirm SQS messages are generated (check queue metrics)

**Expected Behavior:**

- Archive upload completes first
- Customer triggers created for each selected customer
- S3 events trigger SQS messages
- Backend processes each customer independently
- Triggers deleted after successful processing

**Verification:**

```bash
# Check if change exists in archive
CHANGE_ID="your-change-id"
aws s3 ls s3://metadata-bucket/archive/${CHANGE_ID}.json

# Check if triggers were created and deleted
aws s3 ls s3://metadata-bucket/customers/hts/${CHANGE_ID}.json
# Should return "Not Found" if already processed

# Check SQS queue for pending messages
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-hts-changes \
  --attribute-names ApproximateNumberOfMessages
```

### Task 2: Verify Change Processing

**Procedure:**

1. Identify the changeId to verify
2. Check archive/ for current state
3. Review modification array for processing history
4. Verify triggers were deleted
5. Check customer email delivery logs

**Commands:**

```bash
CHANGE_ID="your-change-id"

# Download and inspect archive object
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | jq .

# Check modification array for processing entries
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | \
  jq '.modifications[] | select(.modificationType == "processed")'

# Verify triggers are deleted (should return empty)
aws s3 ls s3://metadata-bucket/customers/ --recursive | grep ${CHANGE_ID}

# Check CloudWatch logs for processing
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID}" \
  --start-time $(date -d '24 hours ago' +%s)000
```

### Task 3: Handle Stuck Triggers

**Symptoms:**

- Triggers older than 15 minutes in customers/ prefix
- SQS messages not being processed
- CloudWatch alarms for old triggers

**Diagnosis:**

```bash
# Find old triggers
aws s3 ls s3://metadata-bucket/customers/ --recursive | \
  awk '{if ($1 < "'$(date -d '15 minutes ago' '+%Y-%m-%d')'" || ($1 == "'$(date -d '15 minutes ago' '+%Y-%m-%d')'" && $2 < "'$(date -d '15 minutes ago' '+%H:%M:%S')'")) print}'

# Check SQS queue for messages
CUSTOMER_CODE="hts"
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
  --attribute-names All

# Check ECS task status
aws ecs list-tasks \
  --cluster change-processor-cluster \
  --service-name customer-${CUSTOMER_CODE}-processor

# Check for errors in logs
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "ERROR" \
  --start-time $(date -d '1 hour ago' +%s)000
```

**Resolution:**

```bash
# Option 1: Restart ECS service (forces new task)
aws ecs update-service \
  --cluster change-processor-cluster \
  --service customer-${CUSTOMER_CODE}-processor \
  --force-new-deployment

# Option 2: Manually process stuck change
./ccoe-customer-contact-manager \
  --customer ${CUSTOMER_CODE} \
  --action process-sqs-message \
  --change-id ${CHANGE_ID}

# Option 3: Manual cleanup (last resort)
# Only if processing is confirmed complete in archive/
aws s3 rm s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json
```

### Task 4: Investigate Processing Failures

**Procedure:**

1. Identify failed change (from alerts or monitoring)
2. Check archive/ for current state
3. Review CloudWatch logs for errors
4. Check SQS dead letter queue
5. Determine root cause
6. Apply appropriate fix

**Commands:**

```bash
CHANGE_ID="failed-change-id"
CUSTOMER_CODE="hts"

# Check archive state
aws s3 cp s3://metadata-bucket/archive/${CHANGE_ID}.json - | jq .

# Check if trigger still exists
aws s3 ls s3://metadata-bucket/customers/${CUSTOMER_CODE}/${CHANGE_ID}.json

# Check dead letter queue
aws sqs receive-message \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes-dlq \
  --max-number-of-messages 10

# Review error logs
aws logs filter-log-events \
  --log-group-name /ecs/change-processor \
  --filter-pattern "${CHANGE_ID}" \
  --start-time $(date -d '24 hours ago' +%s)000 | \
  jq '.events[] | select(.message | contains("ERROR"))'
```

**Common Failure Scenarios:**

| Failure Type | Symptoms | Resolution |
|--------------|----------|------------|
| Archive update failed | Trigger exists, no processing entry in archive | Retry processing, check IAM permissions |
| Trigger delete failed | Processing entry exists, trigger still present | Manual cleanup (non-critical) |
| SES role assumption failed | Error logs show AssumeRole failure | Check IAM role trust policy |
| Meeting creation failed | No meeting metadata in archive | Check Microsoft Graph API credentials |
| Email delivery failed | Processing entry exists, no delivery confirmation | Check SES configuration and quotas |

### Task 5: Monitor Trigger Lifecycle

**Procedure:**

1. Track trigger creation and deletion rates
2. Monitor trigger age distribution
3. Alert on anomalies
4. Investigate long-lived triggers

**Monitoring Dashboard:**

```bash
# Create CloudWatch dashboard
aws cloudwatch put-dashboard \
  --dashboard-name TransientTriggerMonitoring \
  --dashboard-body file://trigger-monitoring-dashboard.json
```

**Dashboard Configuration (trigger-monitoring-dashboard.json):**

```json
{
  "widgets": [
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["CustomMetrics", "TriggerCreationRate"],
          [".", "TriggerDeletionRate"]
        ],
        "period": 300,
        "stat": "Sum",
        "region": "us-east-1",
        "title": "Trigger Creation vs Deletion"
      }
    },
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["CustomMetrics", "TriggerAge", {"stat": "Average"}],
          ["...", {"stat": "Maximum"}]
        ],
        "period": 300,
        "region": "us-east-1",
        "title": "Trigger Age (seconds)"
      }
    },
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["CustomMetrics", "ArchiveUpdateSuccess"],
          [".", "ArchiveUpdateFailure"]
        ],
        "period": 300,
        "stat": "Sum",
        "region": "us-east-1",
        "title": "Archive Update Success/Failure"
      }
    }
  ]
}
```

## Maintenance Procedures

### Weekly Maintenance

1. **Review Processing Metrics**
   - Check average trigger age (should be < 5 minutes)
   - Verify archive update success rate (should be > 99%)
   - Review dead letter queue depth (should be near zero)

2. **Audit Archive Integrity**
   - Verify all recent changes have modification entries
   - Check for orphaned triggers (older than 1 hour)
   - Validate meeting metadata for calendar invites

3. **Capacity Planning**
   - Review ECS task utilization
   - Check SQS queue throughput
   - Monitor S3 storage growth

### Monthly Maintenance

1. **Performance Review**
   - Analyze processing duration trends
   - Identify bottlenecks or slow customers
   - Review and optimize ECS task configurations

2. **Cost Optimization**
   - Review S3 storage costs (archive/ should be primary cost)
   - Analyze SQS message costs
   - Optimize ECS task sizing

3. **Security Audit**
   - Review IAM role permissions
   - Audit cross-account access logs
   - Verify encryption settings

## Incident Response

### Severity Levels

| Severity | Description | Response Time | Escalation |
|----------|-------------|---------------|------------|
| P1 - Critical | All customers unable to process changes | 15 minutes | Immediate |
| P2 - High | Single customer unable to process | 1 hour | After 2 hours |
| P3 - Medium | Degraded performance, no data loss | 4 hours | After 8 hours |
| P4 - Low | Minor issues, workarounds available | 24 hours | After 48 hours |

### P1 Incident: Complete System Outage

**Symptoms:**
- No changes being processed for any customer
- All SQS queues backing up
- Multiple CloudWatch alarms firing

**Response:**

1. **Immediate Actions (0-15 minutes)**
   ```bash
   # Check S3 bucket accessibility
   aws s3 ls s3://metadata-bucket/archive/
   
   # Check SQS queue accessibility
   aws sqs list-queues
   
   # Check ECS cluster status
   aws ecs describe-clusters --clusters change-processor-cluster
   
   # Check IAM role validity
   aws sts get-caller-identity
   ```

2. **Diagnosis (15-30 minutes)**
   - Review CloudWatch logs for common error patterns
   - Check AWS Health Dashboard for service issues
   - Verify network connectivity
   - Test IAM role assumptions

3. **Resolution (30-60 minutes)**
   - Apply appropriate fix based on root cause
   - Restart affected services
   - Verify processing resumes
   - Monitor for stability

4. **Post-Incident (1-24 hours)**
   - Document root cause
   - Implement preventive measures
   - Update runbooks
   - Conduct post-mortem

### P2 Incident: Single Customer Failure

**Symptoms:**
- One customer's changes not processing
- Customer-specific SQS queue backing up
- Other customers unaffected

**Response:**

1. **Immediate Actions (0-30 minutes)**
   ```bash
   CUSTOMER_CODE="affected-customer"
   
   # Check customer-specific queue
   aws sqs get-queue-attributes \
     --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-${CUSTOMER_CODE}-changes \
     --attribute-names All
   
   # Check customer ECS service
   aws ecs describe-services \
     --cluster change-processor-cluster \
     --services customer-${CUSTOMER_CODE}-processor
   
   # Check customer IAM role
   aws sts assume-role \
     --role-arn arn:aws:iam::CUSTOMER_ACCOUNT:role/ses-access \
     --role-session-name test-session
   ```

2. **Resolution**
   - Fix customer-specific configuration
   - Restart customer ECS service
   - Reprocess failed messages from DLQ if needed

## Backup and Recovery

### Archive Backup

```bash
# Daily backup of archive/ prefix
aws s3 sync s3://metadata-bucket/archive/ s3://backup-bucket/archive-$(date +%Y%m%d)/

# Verify backup
aws s3 ls s3://backup-bucket/archive-$(date +%Y%m%d)/ --recursive | wc -l
```

### Recovery Procedures

**Scenario: Accidental Archive Deletion**

```bash
# Restore from most recent backup
BACKUP_DATE="20250101"
aws s3 sync s3://backup-bucket/archive-${BACKUP_DATE}/ s3://metadata-bucket/archive/

# Verify restoration
aws s3 ls s3://metadata-bucket/archive/ --recursive | wc -l
```

**Scenario: Corrupted Archive Object**

```bash
# Restore specific object from backup
CHANGE_ID="corrupted-change-id"
BACKUP_DATE="20250101"
aws s3 cp s3://backup-bucket/archive-${BACKUP_DATE}/${CHANGE_ID}.json \
  s3://metadata-bucket/archive/${CHANGE_ID}.json
```

## Performance Tuning

### ECS Task Optimization

```bash
# Monitor task CPU and memory usage
aws cloudwatch get-metric-statistics \
  --namespace AWS/ECS \
  --metric-name CPUUtilization \
  --dimensions Name=ServiceName,Value=customer-hts-processor \
  --start-time $(date -d '24 hours ago' --iso-8601) \
  --end-time $(date --iso-8601) \
  --period 3600 \
  --statistics Average,Maximum

# Adjust task size if needed
aws ecs update-service \
  --cluster change-processor-cluster \
  --service customer-hts-processor \
  --task-definition customer-hts-processor:2  # New task definition with adjusted resources
```

### SQS Queue Tuning

```bash
# Adjust visibility timeout based on processing duration
aws sqs set-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-hts-changes \
  --attributes VisibilityTimeout=900  # 15 minutes

# Adjust message retention if needed
aws sqs set-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/ACCOUNT/customer-hts-changes \
  --attributes MessageRetentionPeriod=1209600  # 14 days
```

## Contact Information

### Escalation Path

1. **On-Call Engineer**: pagerduty-oncall@company.com
2. **Team Lead**: team-lead@company.com
3. **Engineering Manager**: eng-manager@company.com
4. **Director of Engineering**: director@company.com

### External Dependencies

- **AWS Support**: Premium Support Case
- **Microsoft Graph API**: support@microsoft.com
- **Identity Center**: AWS Support Case

## Related Documentation

- [Architecture Overview](./TRANSIENT_TRIGGER_PATTERN.md)
- [Troubleshooting Guide](./TRANSIENT_TRIGGER_TROUBLESHOOTING.md)
- [FAQ](./TRANSIENT_TRIGGER_FAQ.md)
- [Customer Onboarding](./CUSTOMER_ONBOARDING.md)
