# Requirements Document

## Introduction

This feature enhances the SAML authentication Lambda@Edge function to implement session refresh capabilities. Currently, users must re-authenticate every hour regardless of activity, which interrupts active users and can cause issues during long-running operations like file uploads. This enhancement implements a sliding window session refresh mechanism with an absolute maximum timeout to balance user experience with security requirements.

## Glossary

- **SAML Lambda**: The Lambda@Edge function at `./lambda/saml_auth/lambda-edge-samlify.js` that handles SAML authentication
- **Session Cookie**: The `SAML_SESSION` HTTP-only cookie containing base64-encoded session data
- **Sliding Window**: A session timeout mechanism that extends the session expiration on each user activity
- **Absolute Maximum**: A hard limit on total session duration regardless of activity
- **Session Refresh**: The process of issuing a new session cookie with an updated timestamp
- **Active User**: A user making requests to the application within the session timeout period
- **Idle User**: A user who has not made requests within the idle timeout period

## Requirements

### Requirement 1

**User Story:** As an active user uploading files or managing contacts, I want my session to remain valid while I'm actively using the application, so that I don't get interrupted mid-task.

#### Acceptance Criteria

1. WHEN a user makes a request with a valid session, THE SAML Lambda SHALL extend the session expiration by issuing a new session cookie
2. WHEN a user's session is within the sliding window period, THE SAML Lambda SHALL update the session cookie with a fresh timestamp
3. WHEN a user is actively making requests, THE SAML Lambda SHALL maintain their authenticated state without requiring re-authentication
4. WHERE the session has not reached the absolute maximum duration, THE SAML Lambda SHALL allow session refresh to occur
5. WHEN a session refresh occurs, THE SAML Lambda SHALL preserve the original session creation timestamp for absolute maximum enforcement

### Requirement 2

**User Story:** As a security administrator, I want to enforce both idle timeouts and absolute maximum session durations, so that inactive sessions expire while preventing indefinite session lifetimes.

#### Acceptance Criteria

1. THE SAML Lambda SHALL enforce a configurable idle timeout period of 3 hours for session inactivity
2. THE SAML Lambda SHALL enforce a configurable absolute maximum session duration of 12 hours from initial authentication
3. WHEN a session exceeds the absolute maximum duration, THE SAML Lambda SHALL require re-authentication regardless of activity
4. WHEN a session exceeds the idle timeout, THE SAML Lambda SHALL require re-authentication
5. THE SAML Lambda SHALL store both the original creation timestamp and last activity timestamp in the session cookie

### Requirement 3

**User Story:** As a developer, I want the session refresh mechanism to work efficiently at CloudFront edge locations, so that it doesn't add latency or require additional infrastructure.

#### Acceptance Criteria

1. THE SAML Lambda SHALL implement session refresh using only cookie-based storage without requiring database lookups
2. THE SAML Lambda SHALL maintain compatibility with Lambda@Edge execution constraints
3. THE SAML Lambda SHALL issue new session cookies only when necessary to minimize cookie traffic
4. THE SAML Lambda SHALL validate session data integrity before performing refresh operations
5. THE SAML Lambda SHALL handle session refresh failures gracefully by initiating re-authentication

### Requirement 4

**User Story:** As a system operator, I want configurable timeout values for different security policies, so that I can adjust session behavior without code changes.

#### Acceptance Criteria

1. THE SAML Lambda SHALL support configuration of idle timeout duration via environment variables or constants
2. THE SAML Lambda SHALL support configuration of absolute maximum duration via environment variables or constants
3. THE SAML Lambda SHALL support configuration of refresh threshold (when to issue new cookies) via environment variables or constants
4. THE SAML Lambda SHALL use sensible default values when configuration is not provided
5. THE SAML Lambda SHALL log the active timeout configuration values at startup for audit purposes

### Requirement 5

**User Story:** As a user, I want clear feedback when my session expires, so that I understand why I need to re-authenticate.

#### Acceptance Criteria

1. WHEN a session exceeds the absolute maximum duration, THE SAML Lambda SHALL log the expiration reason
2. WHEN a session exceeds the idle timeout, THE SAML Lambda SHALL log the expiration reason
3. WHEN session refresh fails due to invalid data, THE SAML Lambda SHALL log the failure reason
4. THE SAML Lambda SHALL maintain existing redirect behavior to Identity Center for expired sessions
5. THE SAML Lambda SHALL include session age information in CloudWatch logs for troubleshooting
