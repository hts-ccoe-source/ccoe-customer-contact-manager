# SAML Session Log Format Reference

This document provides a comprehensive reference for all log messages emitted by the SAML Lambda@Edge function, including their structure, fields, and usage.

## Log Message Categories

### 1. Configuration Logs

#### Session Configuration Validated
```
✅ Session configuration validated
```
**When**: Function startup, after configuration validation passes
**Level**: INFO
**Use**: Confirm function initialized successfully

#### Session Configuration
```json
{
  "message": "Session configuration:",
  "idleTimeout": "180 minutes (3 hours)",
  "absoluteMax": "720 minutes (12 hours)",
  "refreshThreshold": "10 minutes",
  "cookieMaxAge": "10800 seconds (3 hours)"
}
```
**When**: Function startup
**Level**: INFO
**Use**: Verify active timeout configuration

#### Environment Overrides
```json
{
  "message": "Environment overrides:",
  "SESSION_IDLE_TIMEOUT_MS": "not set (using default)",
  "SESSION_ABSOLUTE_MAX_MS": "not set (using default)",
  "SESSION_REFRESH_THRESHOLD_MS": "not set (using default)",
  "SESSION_COOKIE_MAX_AGE": "not set (using default)"
}
```
**When**: Function startup
**Level**: INFO
**Use**: Verify which configuration values are overridden

#### Configuration Validation Failed
```
❌ Session configuration validation failed: <error message>
```
**When**: Function startup, if configuration is invalid
**Level**: ERROR
**Use**: Identify configuration errors preventing function startup

### 2. Request Processing Logs

#### Request Start
```
=== LAMBDA@EDGE SAMLIFY REQUEST START ===
URI: /path/to/resource
Method: GET
Headers: ["cookie", "host", "user-agent", ...]
```
**When**: Every request
**Level**: INFO
**Use**: Track request flow and identify request details

#### Request End
```
=== LAMBDA@EDGE REQUEST END - ALLOWING ACCESS ===
```
**When**: Successful authentication
**Level**: INFO
**Use**: Confirm request was allowed through

### 3. SAML Processing Logs

#### SAML Response Processing
```
=== PROCESSING SAML RESPONSE ===
POST Body: <form data>
SAMLResponse received, length: 12345
SAML response decoded, length: 67890
```
**When**: SAML ACS endpoint receives response from IdP
**Level**: INFO
**Use**: Debug SAML response processing

#### Email Extraction Success
```
✅ Extracted email: user@hearst.com
```
**When**: Email successfully extracted from SAML response
**Level**: INFO
**Use**: Verify SAML assertion parsing

#### Email Extraction Failure
```
❌ Could not extract valid email from SAML response
First 1000 chars of SAML: <saml excerpt>
```
**When**: Email extraction fails
**Level**: ERROR
**Use**: Debug SAML assertion format issues

#### User Authorization Success
```
✅ User authorized: user@hearst.com
```
**When**: User passes authorization checks
**Level**: INFO
**Use**: Confirm user is authorized

#### User Authorization Failure
```
❌ User not authorized: user@example.com
User not from hearst.com domain: user@example.com
```
**When**: User fails authorization checks
**Level**: INFO
**Use**: Track unauthorized access attempts

### 4. Session Lifecycle Logs

#### Session Creation
```
🆕 New session created: {
  "email": "user@hearst.com",
  "createdAt": "2025-10-29T10:00:00Z"
}
```
**When**: New user authentication via SAML
**Level**: INFO
**Fields**:
- `email`: User's email address
- `createdAt`: RFC3339 timestamp of session creation
**Use**: Track new authentications

#### Session Migration
```
📦 Migrated legacy session for: user@hearst.com
```
**When**: Legacy session (without lastActivityAt) is migrated
**Level**: INFO
**Use**: Track backward compatibility migrations

#### Session Validation Success
```
✅ Valid session found for user: user@hearst.com
📊 Session metrics - Age: 45min, Idle: 15min
```
**When**: Existing session passes validation
**Level**: INFO
**Fields**:
- `email`: User's email address
- `Age`: Total session age in minutes
- `Idle`: Time since last activity in minutes
**Use**: Monitor active sessions

#### Session Validation Failure
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
**When**: Session fails validation (expired or invalid)
**Level**: ERROR
**Fields**:
- `email`: User's email address
- `reason`: Why validation failed
- `sessionAge`: Total session age in minutes
- `idleTime`: Time since last activity in minutes
- `createdAt`: Original session creation timestamp
- `lastActivityAt`: Last activity timestamp
- `timestamp`: Current timestamp
**Use**: Analyze session expiration patterns

**Common Reasons**:
- `Session exceeded idle timeout` - User idle > 3 hours
- `Session exceeded absolute maximum duration` - Session age > 12 hours
- `Missing required session fields` - Malformed session data
- `Failed to parse session timestamps` - Invalid timestamp format

#### Session Refresh Triggered
```
🔄 Session refresh triggered for user: user@hearst.com
📊 Refresh metrics - Previous activity: 2025-10-29T10:00:00Z, Time until idle expiry: 10min
```
**When**: Session is valid but approaching idle timeout
**Level**: INFO
**Fields**:
- `email`: User's email address
- `Previous activity`: Last activity timestamp
- `Time until idle expiry`: Minutes until idle timeout
**Use**: Monitor session refresh patterns

#### Session Refresh Success
```
✅ Session refresh successful, returning 204 with new cookie
```
**When**: Session cookie successfully refreshed
**Level**: INFO
**Use**: Confirm refresh operations

#### Session Refresh Failure
```json
{
  "message": "⚠️ Session refresh failed, continuing without refresh:",
  "error": "Error message",
  "errorType": "TypeError",
  "email": "user@hearst.com",
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```
**When**: Session refresh fails but session is still valid
**Level**: ERROR
**Fields**:
- `error`: Error message
- `errorType`: JavaScript error type
- `email`: User's email address
- `timestamp`: Current timestamp
**Use**: Debug refresh failures

### 5. Session Error Logs

#### Session Cookie Parse Error
```json
{
  "message": "❌ Session cookie parse error:",
  "error": "Unexpected token",
  "errorType": "SyntaxError",
  "cookieLength": 256,
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```
**When**: Session cookie cannot be decoded or parsed
**Level**: ERROR
**Fields**:
- `error`: Error message
- `errorType`: JavaScript error type
- `cookieLength`: Length of cookie value
- `timestamp`: Current timestamp
**Use**: Debug cookie corruption or tampering

#### Session Validation Error
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
**When**: Session validation encounters an error
**Level**: ERROR
**Fields**:
- `error`: Error message
- `errorType`: JavaScript error type
- `email`: User's email address
- `createdAt`: Session creation timestamp (may be invalid)
- `lastActivityAt`: Last activity timestamp (may be invalid)
- `timestamp`: Current timestamp
**Use**: Debug timestamp parsing issues

#### Session Migration Error
```json
{
  "message": "❌ Session migration error:",
  "error": "Error message",
  "errorType": "TypeError",
  "email": "user@hearst.com",
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```
**When**: Legacy session migration fails
**Level**: ERROR
**Fields**:
- `error`: Error message
- `errorType`: JavaScript error type
- `email`: User's email address
- `timestamp`: Current timestamp
**Use**: Debug migration issues

#### Unexpected Session Processing Error
```json
{
  "message": "❌ Unexpected error during session validation:",
  "error": "Error message",
  "errorType": "Error",
  "stack": "Error stack trace",
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```
**When**: Unexpected error during session processing
**Level**: ERROR
**Fields**:
- `error`: Error message
- `errorType`: JavaScript error type
- `stack`: Full error stack trace
- `timestamp`: Current timestamp
**Use**: Debug unexpected errors

### 6. Auth-Check Endpoint Logs

#### Auth Check Processing
```
=== PROCESSING AUTH CHECK ===
```
**When**: /auth-check endpoint is called
**Level**: INFO
**Use**: Track auth-check requests

#### Auth Check Passed
```
✅ Auth check passed for user: user@hearst.com
📊 Session metrics - Age: 45min, Idle: 15min
```
**When**: Auth-check validates session successfully
**Level**: INFO
**Use**: Monitor auth-check success

#### Auth Check Failed
```json
{
  "message": "❌ Auth check failed - session validation failed:",
  "email": "user@hearst.com",
  "reason": "Session exceeded idle timeout",
  "sessionAge": "120min",
  "idleTime": "180min",
  "createdAt": "2025-10-29T10:00:00Z",
  "lastActivityAt": "2025-10-29T10:30:00Z",
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```
**When**: Auth-check session validation fails
**Level**: ERROR
**Use**: Debug auth-check failures

#### Auth Check Redirect
```
❌ Auth check failed - redirecting to IdP
```
**When**: Auth-check redirects to IdP
**Level**: INFO
**Use**: Track auth-check redirects

### 7. Authentication Flow Logs

#### No Valid Session
```
❌ No valid session found, initiating SAML authentication
Generated SAML AuthnRequest URL: <url>
=== REDIRECTING TO IDENTITY CENTER ===
```
**When**: No valid session, redirecting to IdP
**Level**: INFO
**Use**: Track authentication initiations

#### SAML AuthnRequest Error
```json
{
  "message": "❌ Error creating SAML AuthnRequest:",
  "error": "Error message",
  "errorType": "Error",
  "timestamp": "2025-10-29T13:00:00.000Z"
}
```
**When**: SAML AuthnRequest creation fails
**Level**: ERROR
**Use**: Debug SAML configuration issues

#### User Authenticated Successfully
```
✅ User authenticated successfully: user@hearst.com
```
**When**: User passes all authentication checks
**Level**: INFO
**Use**: Confirm successful authentication

### 8. General Error Logs

#### Lambda@Edge Error
```
Lambda@Edge error: <error>
```
**When**: Unhandled error in Lambda function
**Level**: ERROR
**Use**: Debug critical function errors

#### SAML Processing Failed
```
SAML processing failed: <error message>
```
**When**: SAML response processing fails
**Level**: ERROR
**Use**: Debug SAML processing issues

## Log Patterns for Filtering

### CloudWatch Logs Filter Patterns

#### All Session Events
```
[message = *session*]
```

#### Session Creation
```
[message = "*New session created*"]
```

#### Session Validation Success
```
[message = "*Valid session found*"]
```

#### Session Refresh
```
[message = "*Session refresh triggered*"]
```

#### Session Expiration
```
[message = "*Session validation failed*"]
```

#### Session Errors
```
[message = "*Session*error*" || message = "*Session*failed*"]
```

#### Specific User
```
[message = "*user@hearst.com*"]
```

#### Idle Timeout Expirations
```
[message = "*exceeded idle timeout*"]
```

#### Absolute Maximum Expirations
```
[message = "*exceeded absolute maximum*"]
```

## Log Analysis Tips

### 1. Identifying Session Issues

**High expiration rate**: Filter for "Session validation failed" and group by reason
**Refresh failures**: Filter for "Session refresh failed" and check error types
**Parse errors**: Filter for "Session cookie parse error" and check cookie lengths

### 2. User Behavior Analysis

**Session duration**: Calculate time between "New session created" and "Session validation failed"
**Activity patterns**: Track "Session refresh triggered" frequency per user
**Peak times**: Group "New session created" by hour

### 3. Performance Monitoring

**Lambda execution time**: Check REPORT logs for duration
**Memory usage**: Check REPORT logs for maxMemoryUsed
**Error rate**: Count error logs vs total logs

### 4. Security Monitoring

**Unauthorized access**: Filter for "User not authorized"
**Suspicious patterns**: Multiple parse errors from same user
**Session tampering**: Parse errors with unusual cookie lengths

## Structured Logging Best Practices

### 1. Always Include Context

Every error log includes:
- User email (when available)
- Timestamp
- Error type and message
- Relevant session data

### 2. Use Consistent Emoji Markers

- ✅ Success operations
- ❌ Errors and failures
- 🔄 Refresh operations
- 📊 Metrics and statistics
- 🆕 New resource creation
- 📦 Migration operations
- ⚠️ Warnings (non-critical issues)

### 3. Include Actionable Information

Every error log includes enough information to:
- Identify the affected user
- Understand what went wrong
- Reproduce the issue
- Determine the impact

### 4. Use Appropriate Log Levels

- **INFO**: Normal operations, successful flows
- **ERROR**: Failures, exceptions, validation errors
- **DEBUG**: Detailed information (not currently used)

## Related Documentation

- [MONITORING.md](./MONITORING.md) - CloudWatch Insights queries and dashboards
- [README.md](./README.md) - Function overview and configuration
- [Design Document](./.kiro/specs/saml-session-refresh/design.md) - Session refresh design
