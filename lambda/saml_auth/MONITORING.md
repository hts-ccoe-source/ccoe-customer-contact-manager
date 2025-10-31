# SAML Session Monitoring and Observability Guide

This document provides comprehensive guidance for monitoring and analyzing SAML session behavior using CloudWatch Logs and CloudWatch Insights.

## Table of Contents

- [Session Lifecycle Events](#session-lifecycle-events)
- [CloudWatch Log Groups](#cloudwatch-log-groups)
- [CloudWatch Insights Queries](#cloudwatch-insights-queries)
- [Monitoring Dashboards](#monitoring-dashboards)
- [Alerting Recommendations](#alerting-recommendations)
- [Troubleshooting Guide](#troubleshooting-guide)

## Session Lifecycle Events

The Lambda@Edge function emits structured logs for all session lifecycle events. Each event includes relevant metrics and context for analysis.

### Event Types

#### 1. Session Creation

**Log Pattern**: `🆕 New session created`

**When**: User successfully authenticates via SAML

**Log Fields**:
```json
{
  "message": "🆕 New session created:",
  "email": "user@hearst.com",
  "createdAt": "2025-10-29T10:00:00Z"
}
```

#### 2. Session Validation Success

**Log Pattern**: `✅ Valid session found for user:`

**When**: Existing session passes validation checks

**Log Fields**:
```json
{
  "message": "✅ Valid session found for user: user@hearst.com",
  "sessionMetrics": "📊 Session metrics - Age: 45min, Idle: 15min"
}
```

#### 3. Session Refresh

**Log Pattern**: `🔄 Session refresh triggered`

**When**: Session is valid but approaching idle timeout threshold

**Log Fields**:
```json
{
  "message": "🔄 Session refresh triggered for user: user@hearst.com",
  "refreshMetrics": "📊 Refresh metrics - Previous activity: 2025-10-29T10:00:00Z, Time until idle expiry: 10min",
  "result": "✅ Session refresh successful, returning 204 with new cookie"
}
```

#### 4. Session Expiration

**Log Pattern**: `❌ Session validation failed`

**When**: Session exceeds idle timeout or absolute maximum duration

**Log Fields**:
```json
{
  "message": "❌ Session validation failed:",
  "email": "user@hearst.com",
  "reason": "Session exceeded idle timeout",
  "sessionAge": "120min",
  "idleTime": "180min",
  "createdAt": "2025-10-29T10:00:00Z",
  "lastActivityAt": "2025-10-29T10:30:00Z",
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```

#### 5. Session Errors

**Log Patterns**: 
- `❌ Session cookie parse error`
- `❌ Session validation error`
- `❌ Unexpected error during session validation`

**When**: Session processing encounters errors

**Log Fields**:
```json
{
  "message": "❌ Session validation error:",
  "error": "Invalid date format",
  "errorType": "TypeError",
  "email": "user@hearst.com",
  "createdAt": "invalid-timestamp",
  "lastActivityAt": "2025-10-29T10:00:00Z",
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```

#### 6. Session Migration

**Log Pattern**: `📦 Migrated legacy session for:`

**When**: Legacy session (without lastActivityAt) is migrated

**Log Fields**:
```json
{
  "message": "📦 Migrated legacy session for: user@hearst.com"
}
```

## CloudWatch Log Groups

### Finding Lambda@Edge Logs

Lambda@Edge functions execute at CloudFront edge locations worldwide. Logs are created in the region closest to where the function executed.

**Log Group Naming Pattern**:
```
/aws/lambda/us-east-1.<function-name>
```

**Common Regions**:
- `us-east-1` (N. Virginia) - Primary region for most US traffic
- `us-west-2` (Oregon) - West coast US traffic
- `eu-west-1` (Ireland) - European traffic
- `ap-southeast-1` (Singapore) - Asia-Pacific traffic

**Finding Your Logs**:

1. Open CloudWatch Console
2. Navigate to "Logs" → "Log groups"
3. Search for your function name
4. Check multiple regions using the region selector

### Log Retention

Configure log retention to balance cost and compliance requirements:

```bash
# Set retention to 30 days (recommended for production)
aws logs put-retention-policy \
  --log-group-name /aws/lambda/us-east-1.<function-name> \
  --retention-in-days 30 \
  --region us-east-1
```

**Recommended Retention Periods**:
- **Development**: 7 days
- **Production**: 30-90 days
- **Compliance**: 365+ days (if required)

## CloudWatch Insights Queries

### Session Analysis Queries

#### 1. Sessions by Expiration Reason

Analyze why sessions are expiring to identify patterns.

```sql
fields @timestamp, @message
| filter @message like /Session validation failed/
| parse @message /"reason": "(?<reason>[^"]+)"/
| stats count() as sessionCount by reason
| sort sessionCount desc
```

**Use Case**: Identify if users are hitting idle timeout vs absolute maximum

**Expected Results**:
```
reason                                    | sessionCount
------------------------------------------|-------------
Session exceeded idle timeout            | 1234
Session exceeded absolute maximum        | 56
Missing required session fields          | 12
Failed to parse session timestamps       | 3
```

#### 2. Average Session Duration

Calculate average session lifetime before expiration.

```sql
fields @timestamp, @message
| filter @message like /Session validation failed/
| parse @message /"sessionAge": "(?<age>\d+)min"/
| stats avg(age) as avgSessionMinutes, 
        max(age) as maxSessionMinutes,
        min(age) as minSessionMinutes,
        count() as totalExpired
```

**Use Case**: Understand typical user session patterns

**Expected Results**:
```
avgSessionMinutes | maxSessionMinutes | minSessionMinutes | totalExpired
------------------|-------------------|-------------------|-------------
165.5             | 720               | 5                 | 1305
```

#### 3. Session Refresh Rate

Monitor how often sessions are being refreshed.

```sql
fields @timestamp, @message
| filter @message like /Session refresh triggered/
| stats count() as refreshCount by bin(5m) as timeWindow
| sort timeWindow desc
```

**Use Case**: Identify peak usage times and refresh patterns

**Expected Results**:
```
timeWindow           | refreshCount
---------------------|-------------
2025-10-29 14:00:00  | 45
2025-10-29 13:55:00  | 52
2025-10-29 13:50:00  | 38
```

#### 4. Session Idle Time Distribution

Analyze how long users are idle before their sessions expire.

```sql
fields @timestamp, @message
| filter @message like /Session validation failed/
| parse @message /"idleTime": "(?<idle>\d+)min"/
| stats count() as sessionCount by idle
| sort idle asc
```

**Use Case**: Determine if idle timeout is appropriate for user behavior

#### 5. Active Users by Hour

Count unique authenticated users per hour.

```sql
fields @timestamp, @message
| filter @message like /Valid session found for user:/
| parse @message /user: (?<email>[^\s]+)/
| stats count_distinct(email) as uniqueUsers by bin(1h) as hour
| sort hour desc
```

**Use Case**: Track daily active user patterns

#### 6. Session Errors by Type

Categorize and count session processing errors.

```sql
fields @timestamp, @message
| filter @message like /❌/ and @message like /session/
| parse @message /"errorType": "(?<errorType>[^"]+)"/
| parse @message /"error": "(?<errorMessage>[^"]+)"/
| stats count() as errorCount by errorType, errorMessage
| sort errorCount desc
```

**Use Case**: Identify common error patterns for troubleshooting

#### 7. Session Refresh Success Rate

Calculate the success rate of session refresh operations.

```sql
fields @timestamp, @message
| filter @message like /Session refresh/
| stats 
    count(@message like /refresh successful/) as successCount,
    count(@message like /refresh failed/) as failureCount
| extend totalRefreshes = successCount + failureCount
| extend successRate = (successCount * 100.0) / totalRefreshes
```

**Use Case**: Monitor session refresh reliability

#### 8. User Session Timeline

Track a specific user's session activity over time.

```sql
fields @timestamp, @message
| filter @message like /user@hearst.com/
| sort @timestamp asc
| display @timestamp, @message
```

**Use Case**: Debug specific user authentication issues

**Note**: Replace `user@hearst.com` with the actual email address

#### 9. Session Age at Expiration

Analyze how long sessions lived before expiring.

```sql
fields @timestamp, @message
| filter @message like /Session validation failed/
| parse @message /"sessionAge": "(?<ageMin>\d+)min"/
| parse @message /"reason": "(?<reason>[^"]+)"/
| stats avg(ageMin) as avgAge, 
        max(ageMin) as maxAge,
        min(ageMin) as minAge,
        count() as count
  by reason
```

**Use Case**: Understand session lifetime by expiration reason

#### 10. Peak Authentication Times

Identify when users are authenticating most frequently.

```sql
fields @timestamp, @message
| filter @message like /New session created/
| stats count() as authCount by bin(1h) as hour
| sort authCount desc
| limit 10
```

**Use Case**: Capacity planning and identifying peak usage periods

### Performance Queries

#### 11. Lambda Execution Duration

Monitor Lambda@Edge execution time to ensure performance.

```sql
fields @timestamp, @duration, @message
| filter @type = "REPORT"
| stats avg(@duration) as avgDuration,
        max(@duration) as maxDuration,
        min(@duration) as minDuration,
        pct(@duration, 95) as p95Duration,
        pct(@duration, 99) as p99Duration
```

**Use Case**: Ensure Lambda@Edge stays within performance constraints

**Target**: < 50ms average, < 100ms p99

#### 12. Memory Usage

Monitor memory consumption to optimize Lambda configuration.

```sql
fields @timestamp, @maxMemoryUsed, @memorySize
| filter @type = "REPORT"
| stats avg(@maxMemoryUsed) as avgMemory,
        max(@maxMemoryUsed) as maxMemory,
        avg(@memorySize) as configuredMemory
```

**Use Case**: Right-size Lambda memory allocation

### Security Queries

#### 13. Failed Authentication Attempts

Track authentication failures for security monitoring.

```sql
fields @timestamp, @message
| filter @message like /User not authorized/ or @message like /Access denied/
| parse @message /email: (?<email>[^\s]+)/
| stats count() as failedAttempts by email
| sort failedAttempts desc
```

**Use Case**: Detect potential unauthorized access attempts

#### 14. Session Hijacking Detection

Identify suspicious session patterns that might indicate hijacking.

```sql
fields @timestamp, @message
| filter @message like /Session validation failed/ and @message like /parse error/
| parse @message /"email": "(?<email>[^"]+)"/
| stats count() as parseErrors by email
| sort parseErrors desc
| filter parseErrors > 5
```

**Use Case**: Detect tampered or corrupted session cookies

## Monitoring Dashboards

### Recommended CloudWatch Dashboard

Create a CloudWatch dashboard with the following widgets:

#### Widget 1: Active Sessions (Line Chart)
- **Query**: Active Users by Hour (#5)
- **Visualization**: Line chart
- **Time Range**: Last 24 hours

#### Widget 2: Session Expiration Reasons (Pie Chart)
- **Query**: Sessions by Expiration Reason (#1)
- **Visualization**: Pie chart
- **Time Range**: Last 7 days

#### Widget 3: Session Refresh Rate (Line Chart)
- **Query**: Session Refresh Rate (#3)
- **Visualization**: Line chart
- **Time Range**: Last 24 hours

#### Widget 4: Average Session Duration (Number)
- **Query**: Average Session Duration (#2)
- **Visualization**: Number
- **Time Range**: Last 7 days

#### Widget 5: Session Errors (Bar Chart)
- **Query**: Session Errors by Type (#6)
- **Visualization**: Bar chart
- **Time Range**: Last 24 hours

#### Widget 6: Lambda Performance (Line Chart)
- **Query**: Lambda Execution Duration (#11)
- **Visualization**: Line chart with p50, p95, p99
- **Time Range**: Last 24 hours

### Creating the Dashboard

```bash
# Create dashboard using AWS CLI
aws cloudwatch put-dashboard \
  --dashboard-name "SAML-Session-Monitoring" \
  --dashboard-body file://dashboard-config.json \
  --region us-east-1
```

## Alerting Recommendations

### Critical Alerts

#### 1. High Session Error Rate

**Condition**: Session errors > 5% of total sessions

**Query**:
```sql
fields @timestamp
| filter @message like /session/
| stats 
    count(@message like /❌/) as errors,
    count() as total
| extend errorRate = (errors * 100.0) / total
| filter errorRate > 5
```

**Action**: Investigate session validation logic and cookie integrity

#### 2. Session Refresh Failures

**Condition**: Refresh failure rate > 1%

**Query**:
```sql
fields @timestamp
| filter @message like /Session refresh/
| stats 
    count(@message like /failed/) as failures,
    count() as total
| extend failureRate = (failures * 100.0) / total
| filter failureRate > 1
```

**Action**: Check Lambda@Edge function health and cookie generation

#### 3. Lambda Performance Degradation

**Condition**: p99 execution time > 100ms

**Query**:
```sql
fields @timestamp, @duration
| filter @type = "REPORT"
| stats pct(@duration, 99) as p99
| filter p99 > 100
```

**Action**: Review function code for performance bottlenecks

### Warning Alerts

#### 4. Unusual Session Expiration Pattern

**Condition**: Absolute maximum expirations > 10% of total expirations

**Query**:
```sql
fields @timestamp
| filter @message like /Session validation failed/
| parse @message /"reason": "(?<reason>[^"]+)"/
| stats 
    count(reason like /absolute maximum/) as absoluteExpired,
    count() as totalExpired
| extend absoluteRate = (absoluteExpired * 100.0) / totalExpired
| filter absoluteRate > 10
```

**Action**: Review if absolute maximum timeout is appropriate

#### 5. High Authentication Volume

**Condition**: New authentications > 1000/hour

**Query**:
```sql
fields @timestamp
| filter @message like /New session created/
| stats count() as authCount by bin(1h)
| filter authCount > 1000
```

**Action**: Verify if traffic spike is expected or investigate potential issues

## Troubleshooting Guide

### Common Issues and Solutions

#### Issue 1: Sessions Expiring Too Quickly

**Symptoms**: Users report frequent re-authentication

**Investigation**:
```sql
fields @timestamp, @message
| filter @message like /Session validation failed/
| parse @message /"idleTime": "(?<idle>\d+)min"/
| stats avg(idle) as avgIdleTime, count() as expiredCount
```

**Solutions**:
- Increase `SESSION_IDLE_TIMEOUT_MS` if users are legitimately idle
- Check if session refresh is working correctly
- Verify browser is accepting cookies

#### Issue 2: Session Refresh Not Working

**Symptoms**: Sessions expire despite user activity

**Investigation**:
```sql
fields @timestamp, @message
| filter @message like /Session refresh/
| display @timestamp, @message
```

**Solutions**:
- Verify `SESSION_REFRESH_THRESHOLD_MS` is set correctly
- Check CloudFront cache behavior configuration
- Ensure browser is processing 204 responses correctly

#### Issue 3: High Session Error Rate

**Symptoms**: Many session validation errors in logs

**Investigation**:
```sql
fields @timestamp, @message
| filter @message like /Session validation error/
| parse @message /"error": "(?<error>[^"]+)"/
| stats count() by error
```

**Solutions**:
- Check for timestamp format issues
- Verify session cookie structure
- Review recent code changes

#### Issue 4: Users Can't Authenticate

**Symptoms**: Authentication redirects fail

**Investigation**:
```sql
fields @timestamp, @message
| filter @message like /SAML/ or @message like /authentication/
| sort @timestamp desc
| limit 50
```

**Solutions**:
- Verify SAML metadata is correct
- Check Identity Center configuration
- Ensure CloudFront is routing to Lambda@Edge correctly

### Debug Mode

To enable verbose logging for troubleshooting:

1. Add console.log statements in the Lambda function
2. Deploy updated function
3. Monitor CloudWatch Logs in real-time:

```bash
# Tail logs in real-time
aws logs tail /aws/lambda/us-east-1.<function-name> \
  --follow \
  --region us-east-1
```

### Session Data Inspection

To inspect session cookie data for debugging:

```javascript
// In browser console
document.cookie.split(';').find(c => c.includes('SAML_SESSION'))

// Decode session data
const sessionCookie = document.cookie.split(';')
  .find(c => c.includes('SAML_SESSION'))
  .split('=')[1];
const sessionData = JSON.parse(atob(sessionCookie));
console.log(sessionData);
```

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SESSION_IDLE_TIMEOUT_MS` | 10800000 | Idle timeout in milliseconds (3 hours) |
| `SESSION_ABSOLUTE_MAX_MS` | 43200000 | Absolute maximum in milliseconds (12 hours) |
| `SESSION_REFRESH_THRESHOLD_MS` | 600000 | Refresh threshold in milliseconds (10 minutes) |
| `SESSION_COOKIE_MAX_AGE` | 10800 | Cookie max-age in seconds (3 hours) |

### Session Configuration Validation

The function validates configuration at startup and logs the active values:

```
=== SESSION CONFIGURATION ===
Session configuration: {
  idleTimeout: "180 minutes (3 hours)",
  absoluteMax: "720 minutes (12 hours)",
  refreshThreshold: "10 minutes",
  cookieMaxAge: "10800 seconds (3 hours)"
}
Environment overrides: {
  SESSION_IDLE_TIMEOUT_MS: "not set (using default)",
  SESSION_ABSOLUTE_MAX_MS: "not set (using default)",
  SESSION_REFRESH_THRESHOLD_MS: "not set (using default)",
  SESSION_COOKIE_MAX_AGE: "not set (using default)"
}
=============================
```

## Best Practices

### 1. Regular Monitoring

- Review session metrics weekly
- Set up automated alerts for critical issues
- Monitor session expiration patterns

### 2. Log Analysis

- Use CloudWatch Insights for trend analysis
- Export logs to S3 for long-term analysis
- Create custom dashboards for your use case

### 3. Performance Optimization

- Keep Lambda@Edge execution time < 50ms
- Monitor memory usage and right-size allocation
- Review session refresh patterns for optimization

### 4. Security

- Monitor failed authentication attempts
- Alert on unusual session patterns
- Regularly review session timeout values

### 5. Capacity Planning

- Track peak authentication times
- Monitor active user counts
- Plan for traffic growth

## Additional Resources

- [AWS Lambda@Edge Documentation](https://docs.aws.amazon.com/lambda/latest/dg/lambda-edge.html)
- [CloudWatch Logs Insights Query Syntax](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/CWL_QuerySyntax.html)
- [SAML 2.0 Specification](http://docs.oasis-open.org/security/saml/v2.0/)
- [Session Management Best Practices](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
