# Design Document: SAML Session Refresh

## Overview

This design implements a sliding window session refresh mechanism for the SAML authentication Lambda@Edge function. The solution enhances user experience by maintaining sessions for active users while enforcing security through dual timeout controls (3-hour idle timeout and 12-hour absolute maximum). The implementation is optimized for Lambda@Edge constraints, using only cookie-based storage without external dependencies.

### Design Goals

1. **Zero Infrastructure Changes**: Implement entirely within the existing Lambda@Edge function using cookie-based state
2. **Minimal Performance Impact**: Add session refresh logic without introducing latency or additional API calls
3. **Security Compliance**: Enforce both idle timeout (3 hours) and absolute maximum duration (12 hours)
4. **Backward Compatibility**: Maintain existing authentication flow and cookie structure
5. **Operational Visibility**: Provide clear logging for session lifecycle events

## Architecture

### Current Session Flow

```
User Request → Lambda@Edge → Session Validation → Allow/Deny
                                    ↓
                            Check Cookie Age
                                    ↓
                            < 1 hour? → Allow
                            ≥ 1 hour? → Redirect to IdP
```

### Enhanced Session Flow with Refresh

```
User Request → Lambda@Edge → Session Validation
                                    ↓
                            Parse Session Cookie
                                    ↓
                    ┌───────────────┴───────────────┐
                    ↓                               ↓
            Check Absolute Max              Check Idle Timeout
            (createdAt + 12h)              (lastActivityAt + 3h)
                    ↓                               ↓
            Exceeded? → Redirect           Exceeded? → Redirect
                    ↓                               ↓
                Not Exceeded                   Not Exceeded
                    └───────────────┬───────────────┘
                                    ↓
                        Should Refresh Cookie?
                        (lastActivityAt + 2h50min)
                                    ↓
                            ┌───────┴───────┐
                            ↓               ↓
                        Yes: Issue      No: Continue
                        New Cookie      with Request
```

## Components and Interfaces

### 1. Session Data Structure

**Current Structure:**
```javascript
{
  email: "user@hearst.com",
  createdAt: "2025-10-29T10:00:00Z"  // RFC3339 format
}
```

**Enhanced Structure:**
```javascript
{
  email: "user@hearst.com",
  createdAt: "2025-10-29T10:00:00Z",      // Original authentication time (RFC3339)
  lastActivityAt: "2025-10-29T10:45:00Z"  // Last refresh time (RFC3339)
}
```

### 2. Configuration Constants

```javascript
// Session timeout configuration
const SESSION_CONFIG = {
  // Idle timeout: session expires after this period of inactivity
  IDLE_TIMEOUT_MS: 3 * 60 * 60 * 1000,        // 3 hours (10800000ms)
  
  // Absolute maximum: session expires after this period regardless of activity
  ABSOLUTE_MAX_MS: 12 * 60 * 60 * 1000,       // 12 hours (43200000ms)
  
  // Refresh threshold: issue new cookie when this much time remains
  REFRESH_THRESHOLD_MS: 10 * 60 * 1000,       // 10 minutes (600000ms)
  
  // Cookie Max-Age for browser (should match idle timeout)
  COOKIE_MAX_AGE_SECONDS: 10800                // 3 hours
};
```

### 3. Session Validation Function

**Function Signature:**
```javascript
/**
 * Validates a session and determines if refresh is needed
 * @param {Object} sessionData - Decoded session data from cookie
 * @param {number} currentTime - Current timestamp in milliseconds
 * @returns {Object} Validation result with status and refresh flag
 */
function validateSession(sessionData, currentTime)
```

**Return Structure:**
```javascript
{
  valid: boolean,           // Is session valid?
  shouldRefresh: boolean,   // Should we issue a new cookie?
  reason: string,          // Reason for validation result
  sessionAge: number,      // Total session age in milliseconds
  idleTime: number         // Time since last activity in milliseconds
}
```

### 4. Session Refresh Function

**Function Signature:**
```javascript
/**
 * Creates a refreshed session cookie with updated lastActivityAt
 * @param {Object} sessionData - Current session data
 * @returns {Object} Set-Cookie header object
 */
function refreshSessionCookie(sessionData)
```

### 5. Modified Cookie Creation

**Enhanced createSessionCookie:**
```javascript
function createSessionCookie(userInfo, isRefresh = false, existingCreatedAt = null) {
  const now = toRFC3339(new Date());
  
  const sessionData = {
    email: userInfo.email,
    createdAt: existingCreatedAt || now,  // Preserve original on refresh
    lastActivityAt: now                    // Always update to current time
  };

  const sessionValue = Buffer.from(JSON.stringify(sessionData)).toString('base64');

  return {
    key: 'Set-Cookie',
    value: `SAML_SESSION=${sessionValue}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=${SESSION_CONFIG.COOKIE_MAX_AGE_SECONDS}`
  };
}
```

## Data Models

### Session Cookie Structure

**Storage Format:** Base64-encoded JSON in HTTP-only cookie

**Cookie Attributes:**
- `Path=/` - Available for all application paths
- `HttpOnly` - Not accessible via JavaScript (XSS protection)
- `Secure` - Only transmitted over HTTPS
- `SameSite=Lax` - CSRF protection while allowing navigation
- `Max-Age=10800` - Browser-side expiration (3 hours)

**Data Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| email | string | Yes | User's email address from SAML assertion |
| createdAt | string | Yes | RFC3339 timestamp of initial authentication |
| lastActivityAt | string | Yes | RFC3339 timestamp of last session refresh |

### Validation States

| State | Condition | Action |
|-------|-----------|--------|
| Valid & Fresh | Within idle timeout, not near expiration | Continue without refresh |
| Valid & Stale | Within idle timeout, near expiration | Continue with refresh |
| Expired (Idle) | Exceeded idle timeout | Redirect to IdP |
| Expired (Absolute) | Exceeded absolute maximum | Redirect to IdP |
| Invalid | Malformed or missing data | Redirect to IdP |

## Implementation Details

### Session Validation Logic

```javascript
function validateSession(sessionData, currentTime) {
  // Validate required fields
  if (!sessionData || !sessionData.email || !sessionData.createdAt || !sessionData.lastActivityAt) {
    return {
      valid: false,
      shouldRefresh: false,
      reason: 'Missing required session fields',
      sessionAge: 0,
      idleTime: 0
    };
  }

  try {
    // Parse timestamps using existing datetime utilities
    const createdDate = parseDateTime(sessionData.createdAt);
    const lastActivityDate = parseDateTime(sessionData.lastActivityAt);
    
    // Calculate time deltas
    const sessionAge = currentTime - createdDate.getTime();
    const idleTime = currentTime - lastActivityDate.getTime();
    
    // Check absolute maximum duration
    if (sessionAge >= SESSION_CONFIG.ABSOLUTE_MAX_MS) {
      return {
        valid: false,
        shouldRefresh: false,
        reason: 'Session exceeded absolute maximum duration',
        sessionAge,
        idleTime
      };
    }
    
    // Check idle timeout
    if (idleTime >= SESSION_CONFIG.IDLE_TIMEOUT_MS) {
      return {
        valid: false,
        shouldRefresh: false,
        reason: 'Session exceeded idle timeout',
        sessionAge,
        idleTime
      };
    }
    
    // Session is valid - determine if refresh is needed
    const timeUntilIdleExpiry = SESSION_CONFIG.IDLE_TIMEOUT_MS - idleTime;
    const shouldRefresh = timeUntilIdleExpiry <= SESSION_CONFIG.REFRESH_THRESHOLD_MS;
    
    return {
      valid: true,
      shouldRefresh,
      reason: shouldRefresh ? 'Session valid, refresh recommended' : 'Session valid',
      sessionAge,
      idleTime
    };
    
  } catch (error) {
    return {
      valid: false,
      shouldRefresh: false,
      reason: `Failed to parse session timestamps: ${error.message}`,
      sessionAge: 0,
      idleTime: 0
    };
  }
}
```

### Integration Points

**1. Main Request Handler (Existing Session Check)**

Current location: Lines 290-330 in `lambda-edge-samlify.js`

```javascript
// Enhanced session validation section
if (headers.cookie) {
  const cookies = parseCookies(headers.cookie[0].value);
  const samlSession = cookies['SAML_SESSION'];

  if (samlSession) {
    try {
      const sessionData = JSON.parse(Buffer.from(samlSession, 'base64').toString());
      const validation = validateSession(sessionData, Date.now());
      
      if (validation.valid) {
        console.log('✅ Valid session found for user:', sessionData.email);
        console.log(`Session age: ${Math.floor(validation.sessionAge / 60000)}min, Idle: ${Math.floor(validation.idleTime / 60000)}min`);
        
        sessionValid = true;
        userInfo = sessionData;
        
        // Check if refresh is needed
        if (validation.shouldRefresh) {
          console.log('🔄 Session refresh triggered');
          const refreshedCookie = refreshSessionCookie(sessionData);
          // Store for later addition to response headers
          request.refreshCookie = refreshedCookie;
        }
      } else {
        console.log('❌ Session invalid:', validation.reason);
        console.log(`Session age: ${Math.floor(validation.sessionAge / 60000)}min, Idle: ${Math.floor(validation.idleTime / 60000)}min`);
      }
    } catch (error) {
      console.error('❌ Failed to validate session:', error);
    }
  }
}
```

**2. Auth-Check Endpoint**

Current location: Lines 200-260 in `lambda-edge-samlify.js`

Apply same validation logic to `/auth-check` endpoint for consistency.

**3. Response Modification for Cookie Refresh**

**The Challenge:**

Lambda@Edge functions on **viewer-request** events cannot modify response headers directly. They can only:
- Modify the incoming request
- Return a response directly (bypassing the origin)

**Recommended Solution: Direct Response with Cookie**

When a session refresh is needed, we'll return a lightweight response directly from the Lambda with the refreshed cookie. This works because:

1. The browser receives the Set-Cookie header and updates the cookie
2. For static content (HTML/JS/CSS), we can serve from CloudFront cache on the next request
3. For API calls, the refreshed cookie will be included in subsequent requests

**Implementation:**

```javascript
// After successful validation with refresh needed
if (sessionValid && validation.shouldRefresh) {
  console.log('🔄 Session refresh triggered');
  const refreshedCookie = refreshSessionCookie(userInfo);
  
  // Return a minimal 204 No Content response with the refreshed cookie
  // The browser will update the cookie and the user can continue
  return {
    status: '204',
    statusDescription: 'No Content',
    headers: {
      'set-cookie': [refreshedCookie],
      'cache-control': [{
        key: 'Cache-Control',
        value: 'no-cache, no-store, must-revalidate'
      }]
    }
  };
}

// If no refresh needed, continue with the request normally
console.log('✅ User authenticated successfully:', userInfo.email);
request.headers['x-user-email'] = [{ key: 'X-User-Email', value: userInfo.email }];
// ... continue with normal request processing
```

**Why 204 No Content?**

- Minimal response (no body required)
- Browser automatically retries the request with the new cookie
- Transparent to the user (no visible page reload)
- Works for all request types (HTML, API, static assets)

**User Experience:**

1. User makes request at 2h50min (refresh threshold reached)
2. Lambda returns 204 with refreshed cookie
3. Browser receives cookie, automatically retries the original request
4. Second request succeeds with fresh session
5. Total delay: ~50-100ms (one extra round trip)

**Alternative for API Calls:**

For API endpoints (like `/api/*`), we could pass the request through and let the API Lambda set the cookie:

```javascript
// For API calls, add headers for the API Lambda to handle
if (sessionValid && validation.shouldRefresh && uri.startsWith('/api/')) {
  request.headers['x-session-refresh'] = [{
    key: 'X-Session-Refresh',
    value: 'true'
  }];
  request.headers['x-refresh-cookie'] = [{
    key: 'X-Refresh-Cookie',
    value: refreshSessionCookie(userInfo).value
  }];
  // Let the API Lambda set the cookie in its response
}
```

However, the direct 204 response approach is simpler and works universally for all request types.

## Error Handling

### Error Scenarios

| Scenario | Detection | Handling | Logging |
|----------|-----------|----------|---------|
| Missing session fields | Field validation | Treat as invalid, redirect to IdP | Log missing fields |
| Malformed timestamps | parseDateTime exception | Treat as invalid, redirect to IdP | Log parse error |
| Session exceeded absolute max | Time comparison | Redirect to IdP | Log expiration with age |
| Session exceeded idle timeout | Time comparison | Redirect to IdP | Log expiration with idle time |
| Cookie decode failure | Base64/JSON parse error | Redirect to IdP | Log decode error |
| Refresh cookie creation failure | Exception in refreshSessionCookie | Continue without refresh | Log error, allow request |

### Error Logging Format

```javascript
// Structured logging for session errors
console.error('Session validation failed:', {
  reason: validation.reason,
  email: sessionData.email || 'unknown',
  sessionAge: `${Math.floor(validation.sessionAge / 60000)}min`,
  idleTime: `${Math.floor(validation.idleTime / 60000)}min`,
  timestamp: new Date().toISOString()
});
```

### Graceful Degradation

- If session refresh fails, allow the request to proceed with the existing session
- Log the failure for monitoring but don't interrupt the user
- The session will eventually expire naturally at the idle timeout
- Next request will trigger re-authentication

## Testing Strategy

### Unit Tests

**Test File:** `lambda/saml_auth/__tests__/session-refresh.test.js`

**Test Cases:**

1. **Session Validation Tests**
   - Valid session within both timeouts
   - Session exceeding idle timeout
   - Session exceeding absolute maximum
   - Session missing required fields
   - Session with malformed timestamps
   - Session near expiration (should refresh)
   - Session far from expiration (no refresh)

2. **Cookie Creation Tests**
   - Initial session cookie creation
   - Refresh cookie preserves createdAt
   - Refresh cookie updates lastActivityAt
   - Cookie format and attributes

3. **Timestamp Handling Tests**
   - RFC3339 parsing and formatting
   - Time delta calculations
   - Edge cases (exactly at timeout boundary)

### Integration Tests

**Test Scenarios:**

1. **Full Authentication Flow**
   - New user authentication creates session with both timestamps
   - Session cookie contains correct structure
   - Session validates successfully on subsequent requests

2. **Session Refresh Flow**
   - Active user receives refreshed cookie
   - Refreshed cookie preserves original createdAt
   - Multiple refreshes maintain original createdAt

3. **Expiration Scenarios**
   - Idle user session expires after 3 hours
   - Active user session expires after 12 hours absolute maximum
   - Expired session redirects to IdP

4. **Edge Cases**
   - Session created just before absolute maximum
   - Rapid successive requests (refresh throttling)
   - Malformed cookie handling

### Manual Testing Checklist

- [ ] New authentication creates session with correct timestamps
- [ ] Active user (requests every hour) stays authenticated for 12 hours
- [ ] Idle user (no requests) expires after 3 hours
- [ ] Session refresh occurs around 2h50min mark
- [ ] Absolute maximum enforced at 12 hours regardless of activity
- [ ] CloudWatch logs show session age and idle time
- [ ] Expired sessions redirect to Identity Center
- [ ] Refreshed cookies work correctly in browser
- [ ] Multiple browser tabs share session correctly
- [ ] Session survives browser refresh

### Performance Testing

**Metrics to Monitor:**

- Lambda execution time (should remain < 50ms)
- Cookie size (should remain < 4KB)
- Memory usage (should remain < 128MB)
- CloudWatch log volume

**Load Testing:**

- 1000 requests/second with mixed session states
- Verify no performance degradation
- Monitor Lambda@Edge metrics in CloudWatch

## Configuration Management

### Environment Variables

The Lambda@Edge function will support configuration through environment variables (set in CloudFormation/Terraform):

```javascript
const SESSION_CONFIG = {
  IDLE_TIMEOUT_MS: parseInt(process.env.SESSION_IDLE_TIMEOUT_MS || '10800000'),
  ABSOLUTE_MAX_MS: parseInt(process.env.SESSION_ABSOLUTE_MAX_MS || '43200000'),
  REFRESH_THRESHOLD_MS: parseInt(process.env.SESSION_REFRESH_THRESHOLD_MS || '600000'),
  COOKIE_MAX_AGE_SECONDS: parseInt(process.env.SESSION_COOKIE_MAX_AGE || '10800')
};

// Log configuration at startup
console.log('Session configuration:', {
  idleTimeout: `${SESSION_CONFIG.IDLE_TIMEOUT_MS / 60000}min`,
  absoluteMax: `${SESSION_CONFIG.ABSOLUTE_MAX_MS / 3600000}hr`,
  refreshThreshold: `${SESSION_CONFIG.REFRESH_THRESHOLD_MS / 60000}min`
});
```

### Default Values

| Configuration | Default | Description |
|--------------|---------|-------------|
| SESSION_IDLE_TIMEOUT_MS | 10800000 | 3 hour idle timeout |
| SESSION_ABSOLUTE_MAX_MS | 43200000 | 12 hour absolute maximum |
| SESSION_REFRESH_THRESHOLD_MS | 600000 | 10 minute refresh threshold |
| SESSION_COOKIE_MAX_AGE | 10800 | 3 hour cookie max-age |

### Configuration Validation

```javascript
// Validate configuration at startup
function validateConfig() {
  if (SESSION_CONFIG.IDLE_TIMEOUT_MS <= 0) {
    throw new Error('SESSION_IDLE_TIMEOUT_MS must be positive');
  }
  
  if (SESSION_CONFIG.ABSOLUTE_MAX_MS <= SESSION_CONFIG.IDLE_TIMEOUT_MS) {
    throw new Error('SESSION_ABSOLUTE_MAX_MS must be greater than SESSION_IDLE_TIMEOUT_MS');
  }
  
  if (SESSION_CONFIG.REFRESH_THRESHOLD_MS >= SESSION_CONFIG.IDLE_TIMEOUT_MS) {
    throw new Error('SESSION_REFRESH_THRESHOLD_MS must be less than SESSION_IDLE_TIMEOUT_MS');
  }
  
  console.log('✅ Session configuration validated');
}
```

## Monitoring and Observability

### CloudWatch Metrics

**Custom Metrics to Emit:**

1. `SessionRefreshCount` - Number of session refreshes performed
2. `SessionExpiredIdle` - Sessions expired due to idle timeout
3. `SessionExpiredAbsolute` - Sessions expired due to absolute maximum
4. `SessionValidationErrors` - Failed session validations

### Log Patterns

**Session Lifecycle Logs:**

```javascript
// New session created
console.log('🆕 New session created:', {
  email: userInfo.email,
  createdAt: sessionData.createdAt
});

// Session validated
console.log('✅ Session valid:', {
  email: sessionData.email,
  sessionAge: `${Math.floor(sessionAge / 60000)}min`,
  idleTime: `${Math.floor(idleTime / 60000)}min`,
  willRefresh: shouldRefresh
});

// Session refreshed
console.log('🔄 Session refreshed:', {
  email: sessionData.email,
  sessionAge: `${Math.floor(sessionAge / 60000)}min`,
  previousActivity: sessionData.lastActivityAt,
  newActivity: now
});

// Session expired
console.log('❌ Session expired:', {
  email: sessionData.email,
  reason: validation.reason,
  sessionAge: `${Math.floor(sessionAge / 60000)}min`,
  idleTime: `${Math.floor(idleTime / 60000)}min`
});
```

### CloudWatch Insights Queries

```sql
-- Sessions by expiration reason
fields @timestamp, @message
| filter @message like /Session expired/
| parse @message /reason: "(?<reason>[^"]+)"/
| stats count() by reason

-- Average session duration
fields @timestamp, @message
| filter @message like /Session expired/
| parse @message /sessionAge: "(?<age>\d+)min"/
| stats avg(age) as avgSessionMinutes

-- Refresh rate
fields @timestamp, @message
| filter @message like /Session refreshed/
| stats count() as refreshCount by bin(5m)
```

## Security Considerations

### Session Hijacking Prevention

1. **HttpOnly Cookie**: Prevents JavaScript access to session cookie
2. **Secure Flag**: Ensures cookie only transmitted over HTTPS
3. **SameSite=Lax**: Provides CSRF protection
4. **Absolute Maximum**: Limits session lifetime even if stolen
5. **No Session ID**: Session data is self-contained, no server-side lookup

### Data Integrity

1. **Base64 Encoding**: Prevents cookie parsing issues
2. **JSON Structure**: Validates data structure on parse
3. **Timestamp Validation**: Ensures timestamps are valid RFC3339 format
4. **Field Validation**: Checks for required fields before processing

### Audit Trail

1. **Session Creation**: Logged with email and timestamp
2. **Session Refresh**: Logged with session age and idle time
3. **Session Expiration**: Logged with reason and metrics
4. **Validation Failures**: Logged with error details

## Migration Strategy

### Backward Compatibility

The enhanced session structure is backward compatible with existing sessions:

1. **Existing Sessions**: Will be treated as having `lastActivityAt = createdAt`
2. **First Refresh**: Will add the `lastActivityAt` field
3. **Gradual Migration**: All sessions will be updated within 3 hours (idle timeout)

### Migration Code

```javascript
function migrateSessionData(sessionData) {
  // If lastActivityAt is missing, use createdAt
  if (!sessionData.lastActivityAt && sessionData.createdAt) {
    sessionData.lastActivityAt = sessionData.createdAt;
    console.log('📦 Migrated legacy session for:', sessionData.email);
  }
  return sessionData;
}
```

### Rollback Plan

If issues arise, rollback is simple:

1. Deploy previous Lambda version
2. Existing sessions continue to work (backward compatible)
3. New sessions created without `lastActivityAt` field
4. No data loss or user impact

## Performance Optimization

### Minimal Cookie Updates

- Only issue new cookie when refresh threshold is reached
- Avoid updating cookie on every request
- Reduces Set-Cookie header traffic by ~94% (10min vs 180min)

### Efficient Timestamp Parsing

- Use existing `parseDateTime` utility (already optimized)
- Cache parsed timestamps within request scope
- Avoid redundant parsing operations

### Lambda@Edge Constraints

- Keep function size under 1MB (currently ~50KB)
- Maintain execution time under 50ms (current: ~20ms)
- Stay within 128MB memory limit (current: ~40MB)
- No external API calls or database lookups

## Future Enhancements

### Potential Improvements

1. **Remember Me**: Optional extended session duration for trusted devices
2. **Session Revocation**: Server-side session invalidation (requires DynamoDB)
3. **Device Fingerprinting**: Additional security layer for session validation
4. **Adaptive Timeouts**: Adjust timeouts based on user behavior patterns
5. **Session Analytics**: Detailed metrics on session duration and patterns

### Not Included in This Design

- Server-side session storage (adds latency and complexity)
- Multi-device session management (requires database)
- Session transfer between devices (requires server-side state)
- Concurrent session limits (requires centralized tracking)

These features would require significant architectural changes and are out of scope for this enhancement.
