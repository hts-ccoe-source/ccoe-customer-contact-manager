# Implementation Plan

- [x] 1. Update frontend cancelChange to reload from S3 before cancellation
  - Modify `cancelChange()` in `html/my-changes.html` to reload the change from S3 via GET request BEFORE submitting cancellation
  - Add error handling if S3 reload fails (display error and abort operation)
  - Validate the reloaded change status (prevent cancelling completed changes)
  - Create cancelled change object using fresh S3 data as base (spread operator), then add `status: 'cancelled'` and cancellation metadata
  - POST the cancelled change object to `/changes/{id}/cancel` endpoint
  - Add comprehensive console logging to track the reload and verify meeting metadata presence
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 2. Update frontend deleteChange to reload from S3 before deletion
  - Modify `deleteChange()` in `html/my-changes.html` to reload the change from S3 via GET request BEFORE submitting deletion
  - Add error handling if S3 reload fails (display error and abort operation)
  - DELETE the change by sending fresh S3 data to `/changes/{id}` endpoint (no modification needed, Lambda will handle the move to deleted folder)
  - Add comprehensive console logging to track the reload and verify meeting metadata presence
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 3. Add logging to upload Lambda DELETE handler
  - Add console logging in `handleDeleteChange()` in `lambda/upload_lambda/upload-metadata-lambda.js` to verify meeting metadata is present when loaded from S3
  - Log whether `meeting_id` and `join_url` fields are present in the S3-loaded change object
  - _Requirements: 4.1, 4.3, 4.4_

- [x] 4. Add logging to upload Lambda CANCEL handler
  - Add console logging in `handleCancelChange()` in `lambda/upload_lambda/upload-metadata-lambda.js` to verify meeting metadata is present when loaded from S3
  - Log whether `meeting_id` and `join_url` fields are present in the S3-loaded change object
  - _Requirements: 4.2, 4.3, 4.4_

- [x] 5. Update completeChange to preserve meeting metadata
  - Verify `completeChange()` in `html/my-changes.html` uses spread operator to preserve all fields including meeting metadata
  - Add console logging to verify meeting metadata is present in the change object before sending
  - _Requirements: 2.1, 2.2, 5.1, 5.3_

- [x] 6. Document intentional exclusion in duplicateChange function
  - Add code comment in `duplicateChange()` in `html/my-changes.html` explaining why `meeting_id` and `join_url` are intentionally excluded
  - Verify that the function does not copy these fields
  - _Requirements: 3.1, 3.2, 3.3_

- [ ] 6. Test complete operation with meeting metadata
  - Create a change with meeting required
  - Submit and approve the change (meeting gets scheduled)
  - Complete the change via the UI
  - Verify in browser console that meeting metadata was sent
  - Verify in backend logs that meeting metadata was received
  - Verify in DynamoDB that meeting metadata is still present
  - _Requirements: 1.1, 1.4_

- [ ] 7. Test cancel operation with meeting metadata
  - Create a change with meeting required
  - Submit and approve the change (meeting gets scheduled)
  - Cancel the change via the UI
  - Verify in browser console that meeting metadata was sent
  - Verify in backend logs that meeting metadata was received and meeting was cancelled
  - Verify in DynamoDB that change status is cancelled
  - _Requirements: 1.2, 1.4_

- [ ] 8. Test delete operation with meeting metadata
  - Create a change with meeting required
  - Submit and approve the change (meeting gets scheduled)
  - Delete the change via the UI
  - Verify in browser console that meeting metadata was sent
  - Verify in backend logs that meeting metadata was received and meeting was cancelled
  - Verify change is moved to deleted folder in S3
  - _Requirements: 1.3, 1.4_

- [ ] 9. Test duplicate operation excludes meeting metadata
  - Create a change with meeting required
  - Submit and approve the change (meeting gets scheduled)
  - Duplicate the change via the UI
  - Verify in browser console that duplicated change does NOT have meeting_id or join_url
  - Verify duplicated change has all other meeting fields (meetingRequired, meetingTitle, etc.)
  - _Requirements: 2.1, 2.2, 2.3_
