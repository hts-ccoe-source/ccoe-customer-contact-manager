# Requirements Document: Reduce Backend Logging

## Introduction

The backend Go Lambda has excessive logging that makes it difficult to find important information in CloudWatch Logs. We need to reduce logging to only essential messages while maintaining the ability to debug issues when they occur.

## Requirements

### Requirement 1: Remove Verbose Debug Logging

**User Story:** As a developer, I want to see only essential log messages in CloudWatch, so that I can quickly identify issues without wading through verbose debug output.

#### Acceptance Criteria

1. WHEN processing a successful request THEN the backend SHALL log only one summary line per request
2. WHEN processing multiple SQS messages THEN the backend SHALL log only the summary count, not individual message processing
3. WHEN parsing S3 events THEN the backend SHALL NOT log successful parsing details
4. WHEN extracting customer codes THEN the backend SHALL NOT log successful extraction

### Requirement 2: Keep Error and Warning Logs

**User Story:** As a developer, I want to see all errors and warnings, so that I can diagnose issues when they occur.

#### Acceptance Criteria

1. WHEN an error occurs THEN the backend SHALL log the error with context
2. WHEN a warning condition is detected THEN the backend SHALL log the warning
3. WHEN event loop prevention discards an event THEN the backend SHALL log the discard reason
4. WHEN configuration is missing THEN the backend SHALL log the warning

### Requirement 3: Keep Critical Operation Logs

**User Story:** As a developer, I want to see logs for critical operations like meeting scheduling and cancellation, so that I can verify these operations completed successfully.

#### Acceptance Criteria

1. WHEN a meeting is scheduled THEN the backend SHALL log the meeting ID
2. WHEN a meeting is cancelled THEN the backend SHALL log the cancellation result
3. WHEN an email is sent THEN the backend SHALL log the email type and recipient count
4. WHEN request type determination fails THEN the backend SHALL log the unknown type

### Requirement 4: Remove Emoji and Decorative Logging

**User Story:** As a developer, I want clean, parseable log messages, so that I can use log analysis tools effectively.

#### Acceptance Criteria

1. WHEN logging messages THEN the backend SHALL NOT use emoji characters
2. WHEN logging messages THEN the backend SHALL use consistent formatting
3. WHEN logging errors THEN the backend SHALL use standard error prefixes
