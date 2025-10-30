# Implementation Plan

- [ ] 1. Add session configuration constants and validation
  - Add SESSION_CONFIG object with idle timeout, absolute maximum, refresh threshold, and cookie max-age
  - Implement configuration validation function to ensure values are sensible
  - Add startup logging to display active configuration values
  - Support environment variable overrides for all timeout values
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [ ] 2. Implement session validation logic
  - [ ] 2.1 Create validateSession function with dual timeout checks
    - Implement required field validation (email, createdAt, lastActivityAt)
    - Add timestamp parsing using existing parseDateTime utility
    - Calculate sessionAge (time since createdAt) and idleTime (time since lastActivityAt)
    - Check if sessionAge exceeds absolute maximum duration
    - Check if idleTime exceeds idle timeout
    - Determine if refresh is needed based on refresh threshold
    - Return validation result object with valid, shouldRefresh, reason, sessionAge, and idleTime
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 3.4_

  - [ ] 2.2 Add session migration logic for backward compatibility
    - Implement migrateSessionData function to handle legacy sessions without lastActivityAt
    - Set lastActivityAt to createdAt for legacy sessions
    - Log migration events for monitoring
    - _Requirements: 1.5, 2.5_

- [ ] 3. Implement session refresh mechanism
  - [ ] 3.1 Create refreshSessionCookie function
    - Accept existing sessionData as parameter
    - Preserve original createdAt timestamp
    - Update lastActivityAt to current time using toRFC3339
    - Encode session data as base64 JSON
    - Return Set-Cookie header object with proper attributes
    - _Requirements: 1.1, 1.2, 1.5, 3.1_

  - [ ] 3.2 Modify createSessionCookie for initial authentication
    - Add parameters for isRefresh flag and existingCreatedAt
    - Set both createdAt and lastActivityAt to current time for new sessions
    - Use existingCreatedAt when refreshing to preserve original timestamp
    - Maintain existing cookie attributes (HttpOnly, Secure, SameSite, Max-Age)
    - _Requirements: 1.5, 2.5, 3.1_

- [ ] 4. Integrate session validation into main request handler
  - [ ] 4.1 Update session validation section in main handler
    - Replace simple age check with validateSession function call
    - Handle validation result and log session metrics (age, idle time)
    - Migrate legacy session data if needed
    - Store userInfo for authenticated requests
    - _Requirements: 1.3, 2.3, 2.4, 3.4_

  - [ ] 4.2 Implement session refresh response logic
    - Check if validation.shouldRefresh is true
    - Generate refreshed cookie using refreshSessionCookie
    - Return 204 No Content response with Set-Cookie header
    - Add Cache-Control header to prevent caching
    - Log refresh events with session metrics
    - _Requirements: 1.1, 1.2, 3.2, 3.3_

  - [ ] 4.3 Add enhanced logging for session lifecycle
    - Log session validation success with age and idle time
    - Log session expiration with reason (idle vs absolute)
    - Log session refresh events with previous and new activity timestamps
    - Include email in all session-related logs
    - _Requirements: 5.1, 5.2, 5.3, 5.5_

- [ ] 5. Update auth-check endpoint with session validation
  - Replace simple age check with validateSession function
  - Apply same validation logic as main handler for consistency
  - Return 200 with authenticated status for valid sessions
  - Return 302 redirect to IdP for invalid sessions
  - Log validation results for monitoring
  - _Requirements: 1.3, 2.3, 2.4, 3.4_

- [ ] 6. Add error handling for session operations
  - Wrap session validation in try-catch blocks
  - Handle JSON parse errors gracefully
  - Handle timestamp parsing errors with clear error messages
  - Log all session validation failures with details
  - Redirect to IdP on any validation error
  - Continue without refresh if refresh cookie creation fails
  - _Requirements: 3.5, 5.3, 5.4_

- [ ]* 7. Create unit tests for session validation
  - [ ]* 7.1 Write tests for validateSession function
    - Test valid session within both timeouts
    - Test session exceeding idle timeout
    - Test session exceeding absolute maximum
    - Test session with missing required fields
    - Test session with malformed timestamps
    - Test session near expiration (should refresh)
    - Test session far from expiration (no refresh)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.4_

  - [ ]* 7.2 Write tests for session cookie functions
    - Test initial session cookie creation
    - Test refresh cookie preserves createdAt
    - Test refresh cookie updates lastActivityAt
    - Test cookie format and attributes
    - Test session migration for legacy cookies
    - _Requirements: 1.5, 2.5, 3.1_

- [ ]* 8. Create integration tests for session flows
  - [ ]* 8.1 Test full authentication and refresh flow
    - Test new user authentication creates session with both timestamps
    - Test active user receives refreshed cookie at threshold
    - Test refreshed cookie preserves original createdAt
    - Test multiple refreshes maintain original createdAt
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

  - [ ]* 8.2 Test session expiration scenarios
    - Test idle user session expires after 1 hour
    - Test active user session expires after 8 hours absolute maximum
    - Test expired sessions redirect to IdP
    - Test expiration logging includes correct reason
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 5.1, 5.2_

- [ ] 9. Update Lambda deployment configuration
  - Add environment variables for session timeouts to CloudFormation/Terraform
  - Set default values matching SESSION_CONFIG constants
  - Document configuration options in deployment README
  - Verify Lambda@Edge constraints are met (size, memory, timeout)
  - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [ ] 10. Add monitoring and observability
  - Add structured logging for session lifecycle events (create, validate, refresh, expire)
  - Include session metrics in logs (age, idle time, reason)
  - Document CloudWatch Insights queries for session analysis
  - Verify logs appear in correct CloudWatch log group
  - _Requirements: 4.5, 5.1, 5.2, 5.3, 5.5_
