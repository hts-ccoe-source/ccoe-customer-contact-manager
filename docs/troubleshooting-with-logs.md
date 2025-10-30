# Troubleshooting Guide Using Structured Logs

## Overview

This guide shows how to troubleshoot common issues using the new structured logging format and summary statistics.

## Quick Start: Finding Issues

### 1. Check Lambda Execution Summary

Every Lambda execution logs a comprehensive summary at the end. Start here:

**CloudWatch Insights Query:**
```
fields @timestamp, duration_ms, total_messages, successful, retryable_errors, non_retryable_errors, customers
| filter @message like /lambda execution complete/
| sort @timestamp desc
| limit 20
```

**What to look for:**
- `retryable_errors > 0` - Temporary failures (will retry)
- `non_retryable_errors > 0` - Permanent failures (won't retry)
- `discarded_events > 0` - Backend-generated events (expected)
- High `duration_ms` - Performance issues

### 2. Find Errors

**All errors in last hour:**
```
fields @timestamp, @message, error, customer, change_id, meeting_id
| filter level = "ERROR"
| filter @timestamp > ago(1h)
| sort @timestamp desc
```

### 3. Find Warnings

**All warnings in last hour:**
```
fields @timestamp, @message, customer, topic
| filter level = "WARN"
| filter @timestamp > ago(1h)
| sort @timestamp desc
```

## Common Scenarios

### Scenario 1: Change Request Not Processing

**Symptoms:**
- Customer reports change request not received
- No email sent
- No meeting scheduled

**Investigation Steps:**

1. **Find the Lambda execution:**
   ```
   fields @timestamp, customers, total_messages, successful
   | filter @message like /lambda execution complete/
   | filter customers like /CUSTOMER_CODE/
   | sort @timestamp desc
   | limit 10
   ```

2. **Check for errors:**
   ```
   fields @timestamp, @message, error, customer, change_id
   | filter level = "ERROR"
   | filter customer = "CUSTOMER_CODE"
   | sort @timestamp desc
   ```

3. **Check if event was discarded:**
   ```
   fields @timestamp, discarded_events, customers
   | filter @message like /lambda execution complete/
   | filter discarded_events > 0
   | filter customers like /CUSTOMER_CODE/
   ```

4. **Check summary statistics:**
   ```
   fields @timestamp, emails_sent, meetings_scheduled, approval_requests
   | filter @message like /lambda execution complete/
   | filter customers like /CUSTOMER_CODE/
   ```

**Common Causes:**
- Event discarded (backend-generated) - `discarded_events > 0`
- Email sending failed - Check `email_errors`
- Meeting creation failed - Check `meeting_errors`
- S3 download failed - Check `s3_errors`

### Scenario 2: Email Not Sent

**Symptoms:**
- Meeting scheduled but no email
- Or email expected but not received

**Investigation Steps:**

1. **Check email statistics:**
   ```
   fields @timestamp, emails_sent, emails_before_filter, emails_filtered, email_errors
   | filter @message like /lambda execution complete/
   | filter customers like /CUSTOMER_CODE/
   | sort @timestamp desc
   ```

2. **Check for email errors:**
   ```
   fields @timestamp, @message, error, customer, topic
   | filter level = "ERROR"
   | filter @message like /email/
   | filter customer = "CUSTOMER_CODE"
   ```

3. **Check for no subscribers warning:**
   ```
   fields @timestamp, @message, topic, customer
   | filter level = "WARN"
   | filter @message like /no contacts subscribed/
   | filter customer = "CUSTOMER_CODE"
   ```

**Common Causes:**
- No subscribers to topic - Check warnings for "no contacts subscribed"
- All recipients filtered - `emails_filtered == emails_before_filter`
- Email sending failed - `email_errors > 0`
- SES API error - Check ERROR logs

### Scenario 3: Meeting Not Created

**Symptoms:**
- Change approved but no meeting
- Meeting ID missing

**Investigation Steps:**

1. **Check meeting statistics:**
   ```
   fields @timestamp, meetings_scheduled, meeting_errors, total_attendees, filtered_attendees
   | filter @message like /lambda execution complete/
   | filter customers like /CUSTOMER_CODE/
   | sort @timestamp desc
   ```

2. **Check for meeting errors:**
   ```
   fields @timestamp, @message, error, change_id, customers
   | filter level = "ERROR"
   | filter @message like /meeting/
   | sort @timestamp desc
   ```

3. **Check meeting scheduled logs:**
   ```
   fields @timestamp, meeting_id, change_id, attendee_count
   | filter @message like /meeting scheduled/
   | sort @timestamp desc
   ```

**Common Causes:**
- Graph API error - Check ERROR logs for "failed to create meeting"
- No attendees after filtering - `filtered_attendees == total_attendees`
- Missing Azure credentials - Check ERROR logs for "failed to get Graph access token"
- Meeting metadata not saved to S3 - Check ERROR logs for "CRITICAL: Failed to update S3"

### Scenario 4: High Error Rate

**Symptoms:**
- Many Lambda executions failing
- CloudWatch alarms triggered

**Investigation Steps:**

1. **Find executions with errors:**
   ```
   fields @timestamp, total_messages, successful, retryable_errors, non_retryable_errors
   | filter @message like /lambda execution complete/
   | filter retryable_errors > 0 or non_retryable_errors > 0
   | sort @timestamp desc
   | limit 50
   ```

2. **Group errors by type:**
   ```
   fields @message as error_message
   | filter level = "ERROR"
   | stats count() by error_message
   | sort count desc
   ```

3. **Find most common error:**
   ```
   fields @timestamp, @message, error, customer
   | filter level = "ERROR"
   | filter @timestamp > ago(1h)
   | sort @timestamp desc
   | limit 100
   ```

**Common Causes:**
- S3 access issues - Check for "failed to download from S3"
- SES API throttling - Check for "failed to send email"
- Graph API issues - Check for "failed to create meeting"
- Invalid customer configuration - Check for "failed to get customer config"

### Scenario 5: Slow Performance

**Symptoms:**
- Lambda timeouts
- High duration times

**Investigation Steps:**

1. **Find slow executions:**
   ```
   fields @timestamp, duration_ms, total_messages, customers
   | filter @message like /lambda execution complete/
   | filter duration_ms > 30000
   | sort duration_ms desc
   | limit 20
   ```

2. **Check for concurrent processing:**
   ```
   fields @timestamp, duration_ms, total_messages, customers
   | filter @message like /lambda execution complete/
   | stats avg(duration_ms), max(duration_ms), count() by total_messages
   ```

3. **Check S3 operations:**
   ```
   fields @timestamp, s3_downloads, s3_uploads, s3_errors, duration_ms
   | filter @message like /lambda execution complete/
   | filter duration_ms > 20000
   ```

**Common Causes:**
- Many S3 operations - Check `s3_downloads` and `s3_uploads`
- Multiple customers in single execution - Check `customers` array length
- External API latency - Graph API or SES API slow
- Large metadata files - Check S3 object sizes

### Scenario 6: Unexpected Behavior

**Symptoms:**
- Wrong email sent
- Wrong meeting attendees
- Incorrect filtering

**Investigation Steps:**

1. **Check filtering statistics:**
   ```
   fields @timestamp, emails_before_filter, emails_filtered, total_attendees, filtered_attendees
   | filter @message like /lambda execution complete/
   | filter customers like /CUSTOMER_CODE/
   ```

2. **Check for warnings:**
   ```
   fields @timestamp, @message, customer, topic
   | filter level = "WARN"
   | filter customer = "CUSTOMER_CODE"
   | sort @timestamp desc
   ```

3. **Check change request state:**
   ```
   fields @timestamp, approval_requests, approved_changes, completed_changes, cancelled_changes
   | filter @message like /lambda execution complete/
   | filter customers like /CUSTOMER_CODE/
   ```

**Common Causes:**
- Restricted recipients filtering - Check `emails_filtered` and `filtered_attendees`
- Wrong topic used - Check INFO logs for topic name
- Change request in wrong state - Check approval/approved/completed counts

## Understanding Summary Statistics

### Message Processing

```
total_messages: 3          # SQS messages received
successful: 2              # Successfully processed
retryable_errors: 1        # Temporary failures (will retry)
non_retryable_errors: 0    # Permanent failures (won't retry)
discarded_events: 0        # Backend-generated events (ignored)
```

**Interpretation:**
- `successful + retryable_errors + non_retryable_errors + discarded_events` should equal `total_messages`
- `retryable_errors > 0` - Lambda will be invoked again with failed messages
- `non_retryable_errors > 0` - Messages moved to DLQ (investigate immediately)
- `discarded_events > 0` - Expected for backend-generated events

### Email Statistics

```
emails_sent: 3             # Emails successfully sent
emails_before_filter: 25   # Total recipients before filtering
emails_filtered: 5         # Recipients filtered by restricted_recipients
email_errors: 0            # Email sending failures
```

**Interpretation:**
- `emails_before_filter - emails_filtered` should roughly equal recipients who received email
- High `emails_filtered` - Many recipients restricted (check restricted_recipients config)
- `email_errors > 0` - SES API failures (investigate)
- `emails_sent == 0` but expected - Check for "no contacts subscribed" warning

### Meeting Statistics

```
meetings_scheduled: 1      # Meetings created
meetings_cancelled: 0      # Meetings cancelled
meetings_updated: 0        # Meetings updated
meeting_errors: 0          # Meeting operation failures
total_attendees: 20        # Total attendees before filtering
filtered_attendees: 5      # Attendees filtered by restricted_recipients
manual_attendees: 2        # Manually added attendees
final_attendee_count: 17   # Final attendees in meeting
```

**Interpretation:**
- `total_attendees - filtered_attendees + manual_attendees` should equal `final_attendee_count`
- High `filtered_attendees` - Many attendees restricted (check restricted_recipients config)
- `meeting_errors > 0` - Graph API failures (investigate)
- `meetings_scheduled == 0` but expected - Check ERROR logs

### S3 Operations

```
s3_downloads: 3            # S3 objects downloaded
s3_uploads: 2              # S3 objects uploaded
s3_deletes: 0              # S3 objects deleted
s3_errors: 0               # S3 operation failures
```

**Interpretation:**
- `s3_downloads` should match number of change requests processed
- `s3_uploads` should match number of metadata updates
- `s3_errors > 0` - S3 API failures or permission issues (investigate)

### Change Request Processing

```
approval_requests: 1       # Approval request emails sent
approved_changes: 0        # Approved announcement emails sent
completed_changes: 0       # Completion emails sent
cancelled_changes: 0       # Changes cancelled
```

**Interpretation:**
- Tracks change request lifecycle
- Should see progression: approval_request → approved → completed
- Or: approval_request → cancelled

## Advanced Queries

### Find All Activity for a Change ID

```
fields @timestamp, @message, level, customer, change_id
| filter change_id = "CHANGE_ID"
| sort @timestamp asc
```

### Find All Activity for a Customer

```
fields @timestamp, @message, level, change_id
| filter customer = "CUSTOMER_CODE"
| sort @timestamp desc
| limit 100
```

### Find Executions with Specific Error

```
fields @timestamp, customers, error
| filter level = "ERROR"
| filter @message like /failed to send email/
| sort @timestamp desc
```

### Calculate Success Rate

```
fields @timestamp, successful, retryable_errors, non_retryable_errors
| filter @message like /lambda execution complete/
| stats sum(successful) as total_success, 
        sum(retryable_errors) as total_retryable,
        sum(non_retryable_errors) as total_non_retryable
```

### Find Peak Usage Times

```
fields @timestamp, total_messages
| filter @message like /lambda execution complete/
| stats sum(total_messages) as messages by bin(1h)
| sort messages desc
```

### Monitor Email Filtering Rate

```
fields @timestamp, emails_before_filter, emails_filtered
| filter @message like /lambda execution complete/
| filter emails_before_filter > 0
| stats avg(emails_filtered * 100.0 / emails_before_filter) as avg_filter_rate
```

## Error Patterns and Solutions

### "failed to download from S3"

**Cause:** S3 access denied or object doesn't exist

**Solution:**
1. Check Lambda execution role has S3 read permissions
2. Verify S3 bucket and key are correct
3. Check if object was deleted

### "failed to send email"

**Cause:** SES API error or throttling

**Solution:**
1. Check SES sending limits
2. Verify SES topic exists and has subscribers
3. Check for SES API throttling (429 errors)
4. Verify customer role has SES permissions

### "failed to create meeting"

**Cause:** Graph API error or missing credentials

**Solution:**
1. Check Azure credentials in Parameter Store
2. Verify Graph API permissions
3. Check for Graph API throttling
4. Verify meeting metadata is valid

### "no contacts subscribed to topic"

**Cause:** SES topic has no subscribers

**Solution:**
1. Verify topic name is correct
2. Check if contacts are subscribed to topic in SES
3. Verify customer has contacts in SES

### "discarding backend event"

**Cause:** Event generated by backend Lambda (event loop prevention)

**Solution:** This is expected behavior, no action needed

## Monitoring Recommendations

### CloudWatch Alarms

1. **High Error Rate**
   ```
   Metric: non_retryable_errors
   Threshold: > 5 in 5 minutes
   ```

2. **High Retry Rate**
   ```
   Metric: retryable_errors
   Threshold: > 10 in 5 minutes
   ```

3. **Slow Execution**
   ```
   Metric: duration_ms
   Threshold: > 45000 (45 seconds)
   ```

4. **No Emails Sent**
   ```
   Metric: emails_sent
   Threshold: == 0 for 1 hour (when expected)
   ```

### Dashboard Widgets

1. **Success Rate**
   - Show: `successful / total_messages * 100`
   - Time range: Last 24 hours

2. **Email Statistics**
   - Show: `emails_sent`, `emails_filtered`, `email_errors`
   - Time range: Last 24 hours

3. **Meeting Statistics**
   - Show: `meetings_scheduled`, `meeting_errors`
   - Time range: Last 24 hours

4. **Error Trends**
   - Show: `retryable_errors`, `non_retryable_errors` over time
   - Time range: Last 7 days

## Getting Help

If you can't resolve an issue:

1. **Gather information:**
   - Lambda execution summary log
   - All ERROR logs for the execution
   - Change ID or customer code
   - Expected vs actual behavior

2. **Check documentation:**
   - `docs/logging-standards.md` - Logging guidelines
   - `.kiro/specs/reduce-backend-logging/summary-metrics-mapping.md` - Metric definitions

3. **Contact team:**
   - Include CloudWatch Insights query results
   - Include relevant log excerpts
   - Include summary statistics
