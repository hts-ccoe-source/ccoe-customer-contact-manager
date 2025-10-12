# Requirements Document

## Introduction

The frontend JavaScript code currently does not preserve meeting metadata (`meeting_id` and `join_url`) when performing status change operations (approve, complete, cancel, delete). The root cause is a **race condition and stale cache issue**:

1. **Cancel/Delete buttons are on list pages**, not edit forms - any edits have already been persisted to S3
2. **Backend updates S3 asynchronously** - when a meeting is scheduled, the backend writes `meeting_id` and `join_url` to S3
3. **Frontend uses cached data** - the frontend's in-memory cache may not have the latest S3 updates
4. **Lambda overwrites S3** - when the frontend sends a cancel/delete request with stale data, the Lambda overwrites the S3 object, losing meeting metadata

This causes meeting cancellation to fail because the backend cannot find the meeting ID to cancel.

This feature will ensure that cancel and delete operations **always reload the latest change object from S3** before submitting the operation, guaranteeing that meeting metadata is preserved.

## Requirements

### Requirement 1: Reload Fresh Data Before Cancel/Delete Operations

**User Story:** As a change manager, I want cancel and delete operations to use the latest S3 data, so that meeting metadata added by the backend is not lost.

#### Acceptance Criteria

1. WHEN a user clicks the "Cancel" button on a change THEN the frontend SHALL reload the change object from S3 via GET API call BEFORE submitting the cancellation
2. WHEN a user clicks the "Delete" button on a change THEN the frontend SHALL reload the change object from S3 via GET API call BEFORE submitting the deletion
3. WHEN the S3 reload fails THEN the frontend SHALL display an error message and NOT proceed with the cancel/delete operation
4. WHEN the reloaded change object is obtained THEN the frontend SHALL use this fresh data (not cached data) for the cancel/delete API request
5. WHEN the fresh change object is sent to the API THEN it SHALL include all backend-added fields including `meeting_id`, `join_url`, and the complete `modifications` array

### Requirement 2: Preserve Meeting Metadata in Complete Operations

**User Story:** As a change manager, I want meeting metadata to be preserved when I complete a change, so that the audit trail remains intact.

#### Acceptance Criteria

1. WHEN a user clicks the "Complete" button on a change THEN the frontend SHALL include `meeting_id` and `join_url` fields in the API request body if they exist in the cached change object
2. WHEN a complete operation is performed THEN the frontend SHALL preserve ALL fields from the cached change object, not just a subset

### Requirement 3: Preserve Meeting Metadata in Duplicate Operations

**User Story:** As a change manager, I want to ensure that when I duplicate a change, meeting metadata is explicitly excluded, so that duplicated changes don't reference the original change's meeting.

#### Acceptance Criteria

1. WHEN a user duplicates a change THEN the frontend SHALL explicitly exclude `meeting_id` and `join_url` fields from the duplicated change object
2. WHEN a change is duplicated THEN the frontend SHALL preserve all other fields including `meetingRequired`, `meetingTitle`, `meetingDate`, `meetingDuration`, and `meetingLocation`
3. WHEN a duplicated change is saved THEN it SHALL NOT contain any reference to the original change's Microsoft Graph meeting

### Requirement 4: Upload Lambda Handlers Load from S3

**User Story:** As a backend developer, I want the Node.js upload Lambda handlers (`lambda/upload_lambda/upload-metadata-lambda.js`) to always load from S3 as the single source of truth, so that meeting metadata is reliably available for cancellation.

#### Acceptance Criteria

1. WHEN the `handleDeleteChange()` function in upload-metadata-lambda.js receives a request THEN it SHALL load the change object from S3 (not request body)
2. WHEN the `handleCancelChange()` function in upload-metadata-lambda.js receives a request THEN it SHALL load the change object from S3 (not request body)
3. WHEN the Lambda loads from S3 THEN it SHALL log whether meeting metadata (`meeting_id`, `join_url`) is present
4. WHEN the Lambda processes a cancel/delete operation THEN it SHALL use the S3-loaded data for meeting cancellation operations
5. WHEN the change is not found in S3 THEN the Lambda SHALL return a 404 error

### Requirement 5: Consistent Schema Handling Across All Operations

**User Story:** As a developer, I want all frontend operations to use a consistent approach for handling the change object schema, so that future schema additions don't cause similar issues.

#### Acceptance Criteria

1. WHEN the change object schema is extended with new fields THEN existing frontend operations SHALL automatically include those fields without code changes
2. WHEN an operation needs to exclude specific fields THEN it SHALL use an explicit exclusion list rather than an inclusion list
3. WHEN debugging schema issues THEN developers SHALL be able to see which fields are being sent in API requests through console logging


