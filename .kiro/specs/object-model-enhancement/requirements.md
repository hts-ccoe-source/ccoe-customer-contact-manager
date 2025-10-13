# Requirements Document

## Introduction

The current JSON object model used by the frontend and consumed by the backends needs enhancement to support better modification tracking and Microsoft Graph meeting metadata integration. Currently, the system only stores simple strings for who and when a document was modified, which overwrites previous modification history. Additionally, there's no mechanism to capture Microsoft Graph meeting metadata from the backend Go Lambda and associate it with the change request object in S3. This enhancement will implement an array-based modification tracking system and establish a pathway for the backend to update change objects with meeting metadata.

## Requirements

### Requirement 1: Array-Based Storage Behavior Change

**User Story:** As a system administrator reviewing change history, I want the system to preserve all historical modifications and approvals instead of overwriting them, so that I can see the complete audit trail of what happened to each change request over time.

#### Acceptance Criteria

1. WHEN a change request is modified THEN the system SHALL append a new entry to a modifications array instead of overwriting the single "lastModified" field
2. WHEN a change request is approved THEN the system SHALL append a new entry with modification_type "approved" to the modifications array instead of overwriting the single "approvedBy" field
3. WHEN displaying modification history THEN the system SHALL show all entries in chronological order with the most recent first
4. WHEN filtering for approvals THEN the system SHALL show entries where modification_type equals "approved"
5. WHEN the frontend creates a new change THEN it SHALL initialize an empty modifications array
6. WHEN saving objects THEN they SHALL always use the new array-based format

### Requirement 2: Data Structure and Field Definitions

**User Story:** As a developer working with change request data, I want clearly defined data structures for modification entries (including approvals), so that I can implement consistent data handling and ensure all necessary information is captured.

#### Acceptance Criteria

1. WHEN creating a modification entry THEN it SHALL include these required fields: timestamp, user_id, modification_type
2. WHEN the modification_type is "created" THEN it SHALL capture initial creation metadata
3. WHEN the modification_type is "updated" THEN it SHALL capture what specific fields were changed
4. WHEN the modification_type is "submitted" THEN it SHALL capture the draft-to-submitted transition
5. WHEN the modification_type is "approved" THEN it SHALL capture approval events (user_id becomes approver_id)
6. WHEN the modification_type is "deleted" THEN it SHALL capture deletion events when the change is moved to the deleted key prefix
7. WHEN the modification_type is "meeting_scheduled" THEN it SHALL include meeting metadata
8. WHEN the modification_type is "meeting_cancelled" THEN it SHALL capture meeting cancellation events
9. WHEN storing user identity fields THEN they SHALL use Identity Center user ID format

### Requirement 3: Microsoft Graph Meeting Metadata Integration

**User Story:** As a CCOE team member scheduling change-related meetings, I want the meeting metadata from Microsoft Graph to be automatically associated with the change request, so that I can track which meetings are related to which changes and access meeting details directly from the change management interface.

#### Acceptance Criteria

1. WHEN the Go Lambda backend creates a Microsoft Graph meeting THEN it SHALL capture the meeting metadata including meeting ID, join URL, and scheduling details
2. WHEN meeting metadata is available THEN the backend SHALL update the change request object in S3 with the meeting information
3. WHEN updating with meeting metadata THEN it SHALL add a new modification entry of type "meeting_scheduled" to the modifications array
4. WHEN displaying change details THEN the frontend SHALL show associated meeting information if available
5. IF multiple meetings are associated with a change THEN each SHALL be tracked as separate modification entries

### Requirement 4: Direct S3 Update Mechanism

**User Story:** As a backend developer, I want the Go Lambda to directly update change request objects in S3 after processing events, so that meeting metadata and other backend-generated information can be efficiently stored with the change request.

#### Acceptance Criteria

1. WHEN the Go Lambda backend processes a change request THEN it SHALL already have the change object loaded in memory from the initial S3 read
2. WHEN the backend schedules a meeting THEN it SHALL append the meeting metadata as a new modification entry to the in-memory object
3. WHEN the backend completes processing THEN it SHALL write the updated object back to S3 using a direct PUT operation
4. WHEN updating the S3 object THEN the backend SHALL use its existing IAM execution role with S3 read/write permissions
5. WHEN concurrent updates might occur THEN the system SHALL use S3 versioning or conditional updates to prevent data loss
6. IF the S3 update fails THEN the backend SHALL retry with exponential backoff and log detailed error information
7. WHEN the update is successful THEN the backend SHALL log the operation for audit purposes

### Requirement 5: Frontend Display and User Experience

**User Story:** As a CCOE team member viewing change requests, I want to see the complete modification history in an intuitive interface, so that I can quickly understand the change lifecycle, approval status, and any associated meetings or updates.

#### Acceptance Criteria

1. WHEN viewing a change request THEN the frontend SHALL display the modification history in a unified timeline
2. WHEN modification entries include meeting metadata THEN they SHALL show meeting details with clickable join links
3. WHEN modification entries have type "approved" THEN they SHALL be displayed as approval events
4. WHEN there are many modification entries THEN the interface SHALL support pagination or collapsible sections
5. WHEN displaying modification types THEN each SHALL have appropriate icons and formatting for easy recognition
6. WHEN users hover over modification entries THEN they SHALL see additional details in tooltips or expanded views
7. WHEN filtering by type THEN users SHALL be able to view only approvals, only meetings, etc.

### Requirement 6: Meeting Scheduling Idempotency

**User Story:** As a system operator managing change-related meetings, I want the meeting scheduling functionality to be idempotent using the embedded meeting metadata, so that when meeting times change the system updates the existing meeting instead of creating duplicate meetings.

#### Acceptance Criteria

1. WHEN the Go Lambda backend needs to schedule a meeting THEN it SHALL first check the change object for existing meeting metadata
2. WHEN existing meeting metadata is found THEN the system SHALL update the existing Microsoft Graph meeting instead of creating a new one
3. WHEN meeting times or details change THEN the system SHALL use the stored meeting ID to update the existing meeting
4. WHEN no existing meeting metadata is found THEN the system SHALL create a new meeting and store the metadata
5. WHEN meeting updates are successful THEN the system SHALL update the stored meeting metadata with any changes
6. IF the existing meeting cannot be found or updated THEN the system SHALL create a new meeting and update the stored metadata
7. WHEN meeting operations complete THEN the system SHALL add appropriate modification entries to track the meeting lifecycle

### Requirement 7: Meeting Cancellation Integration

**User Story:** As a CCOE team member cancelling a change request, I want any associated meetings to be automatically cancelled when I cancel the change, so that attendees are properly notified and calendar entries are cleaned up.

#### Acceptance Criteria

1. WHEN the frontend receives a "cancel" change event THEN it SHALL check the change object for existing meeting metadata
2. WHEN meeting metadata exists for a cancelled change THEN the system SHALL generate an event instructing the backend to cancel the associated meeting
3. WHEN the backend receives a meeting cancellation instruction THEN it SHALL use the stored meeting ID to cancel the Microsoft Graph meeting
4. WHEN a meeting is successfully cancelled THEN the backend SHALL add a modification entry of type "meeting_cancelled" to the change object
5. WHEN meeting cancellation fails THEN the backend SHALL log the error and add a modification entry indicating the cancellation attempt failed
6. WHEN displaying cancelled changes with meetings THEN the frontend SHALL show the meeting cancellation status
7. IF multiple meetings were associated with the change THEN all meetings SHALL be cancelled when the change is cancelled

### Requirement 8: S3 Event Loop Prevention via UserIdentity

**User Story:** As a system architect, I want the backend to immediately identify and ignore S3 events that it generates itself, so that the system doesn't create infinite processing loops when the backend updates S3 objects.

#### Acceptance Criteria

1. WHEN the backend receives an SQS message containing an S3 event THEN it SHALL check the userIdentity field in the S3 event payload
2. WHEN the userIdentity matches the backend Lambda's execution role ARN THEN the backend SHALL immediately discard the message without processing
3. WHEN the userIdentity matches the frontend Lambda's execution role ARN THEN the backend SHALL process the event normally
4. WHEN discarding its own events THEN the backend SHALL log the decision with the event details for debugging
5. WHEN the userIdentity check is inconclusive THEN the backend SHALL err on the side of processing to avoid missing legitimate events
6. WHEN processing SQS messages THEN the backend SHALL perform the userIdentity check before any S3 API calls to maximize efficiency
7. IF the S3 event payload is malformed or missing userIdentity THEN the backend SHALL process the event normally and log a warning

### Requirement 9: Data Validation and Error Handling

**User Story:** As a developer working with the enhanced object model, I want robust validation and error handling for modification entries, approval entries, and meeting metadata, so that data integrity is maintained even when backend systems encounter errors.

#### Acceptance Criteria

1. WHEN creating modification entries THEN the system SHALL validate required fields and data types
2. WHEN creating approval entries THEN the system SHALL validate the same fields as the current approval system
3. WHEN processing meeting metadata THEN the system SHALL validate Microsoft Graph response data before storing
4. WHEN backend updates fail THEN the system SHALL log detailed error information and continue processing
5. WHEN invalid modification or approval data is encountered THEN the system SHALL handle it gracefully without breaking the change display
6. WHEN duplicate approvals are submitted THEN the system SHALL handle them according to current business rules (likely allowing multiple approvals from the same person over time)
7. IF data corruption is detected THEN the system SHALL provide recovery mechanisms and alert administrators