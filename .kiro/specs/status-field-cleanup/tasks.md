# Implementation Plan

- [ ] 1. Update type definitions
  - Update `ChangeMetadata` struct to remove `Metadata` map and `Source` field, add `PriorStatus` field
  - Update `AnnouncementMetadata` struct to remove `Metadata` map and `Source` field, add `PriorStatus` field
  - Change `meeting_required` to `include_meeting` in `ChangeMetadata`
  - Ensure both types use `include_meeting` boolean field
  - _Requirements: 1.1, 1.2, 2.1, 2.2, 3.1_

- [ ] 2. Simplify status determination logic
  - [ ] 2.1 Create `DetermineRequestTypeFromStatus()` function that takes status string parameter
    - Implement switch statement for status values (submitted, approved, completed, cancelled)
    - Return appropriate request type for each status
    - Log warning for unknown status values
    - _Requirements: 1.3, 1.4, 4.1, 4.2, 4.3, 4.4, 4.5_
  
  - [ ] 2.2 Update `DetermineRequestType()` for changes
    - Call `DetermineRequestTypeFromStatus(metadata.Status)`
    - Remove all checks for `metadata.Metadata` map
    - Remove all checks for `metadata.Source` field
    - _Requirements: 1.1, 1.2, 1.3, 4.5_
  
  - [ ] 2.3 Create `DetermineAnnouncementRequestType()` for announcements
    - Call `DetermineRequestTypeFromStatus(metadata.Status)`
    - Ensure identical logic to change request type determination
    - _Requirements: 3.1, 3.2, 3.3_

- [ ] 3. Add validation for legacy metadata
  - [ ] 3.1 Add validation function to detect legacy `metadata` map
    - Check if `Metadata` field exists and is non-empty
    - Log error with object ID if legacy metadata found
    - Return error to fail processing
    - _Requirements: 5.1, 5.2, 5.3_
  
  - [ ] 3.2 Call validation in `ProcessTransientTrigger()`
    - Validate after loading from S3
    - Fail processing if validation fails
    - _Requirements: 5.1, 5.2, 5.3_

- [ ] 4. Update frontend API Lambda
  - [ ] 4.1 Remove metadata map writes
    - Remove code that writes `metadata.status`
    - Remove code that writes `metadata.request_type`
    - Remove any other writes to `metadata` map
    - _Requirements: 2.1, 2.2, 2.3_
  
  - [ ] 4.2 Add prior_status tracking
    - Set `prior_status` to current `status` before changing status
    - Set `prior_status` to empty string for newly created objects
    - _Requirements: 2.4_
  
  - [ ] 4.3 Update meeting field name
    - Change `meeting_required` to `include_meeting` for changes
    - Ensure announcements continue using `include_meeting`
    - Update all references in approval/submit/complete/cancel handlers
    - _Requirements: 3.1, 3.2, 3.3_

- [ ] 5. Update backend meeting logic
  - Update `isMeetingRequired()` function to check `include_meeting` field only
  - Remove any checks for `meeting_required` field
  - Ensure logic works for both changes and announcements
  - _Requirements: 3.1, 3.2, 3.3_

- [ ] 6. Delete all existing S3 objects
  - Delete all objects from `archive/` prefix
  - Delete all objects from `customers/` prefix
  - Delete all objects from `drafts/` prefix
  - _Requirements: 5.1, 5.2_

- [ ] 7. Deploy backend changes
  - Build Go Lambda binary
  - Deploy to AWS Lambda
  - Verify deployment successful
  - _Requirements: All_

- [ ] 8. Deploy frontend API changes
  - Install Node.js dependencies
  - Deploy upload Lambda
  - Verify deployment successful
  - _Requirements: All_

- [ ] 9. Integration testing
  - [ ] 9.1 Test change workflow
    - Create new change (status: draft, prior_status: "")
    - Submit change (status: submitted, prior_status: draft)
    - Verify approval request email sent
    - Approve change (status: approved, prior_status: submitted)
    - Verify approved announcement email sent
    - Verify meeting scheduled if `include_meeting: true`
    - _Requirements: 3.1, 3.2, 3.3, 4.1, 4.2_
  
  - [ ] 9.2 Test announcement workflow
    - Create new announcement (status: draft, prior_status: "")
    - Submit announcement (status: submitted, prior_status: draft)
    - Verify approval request email sent
    - Approve announcement (status: approved, prior_status: submitted)
    - Verify approved announcement email sent
    - Verify meeting scheduled if `include_meeting: true`
    - _Requirements: 3.1, 3.2, 3.3, 4.1, 4.2_
  
  - [ ] 9.3 Test cancellation workflow
    - Approve change/announcement with meeting
    - Cancel (status: cancelled, prior_status: approved)
    - Verify meeting cancelled
    - Verify cancellation email sent
    - _Requirements: 4.4_
  
  - [ ] 9.4 Test completion workflow
    - Approve change/announcement
    - Complete (status: completed, prior_status: approved)
    - Verify completion email sent
    - _Requirements: 4.3_
  
  - [ ] 9.5 Verify no metadata map in S3 objects
    - Check archived objects have no `metadata` field
    - Check all objects have `prior_status` field
    - Check all objects use `include_meeting` field
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 5.4_
