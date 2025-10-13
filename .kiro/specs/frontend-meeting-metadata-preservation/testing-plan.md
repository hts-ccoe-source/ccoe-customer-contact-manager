# Testing Plan: Frontend Meeting Metadata Preservation

## Overview

This testing plan validates that the race condition fix properly preserves meeting metadata (`meeting_id` and `join_url`) during cancel and delete operations. The fix ensures the frontend always reloads fresh data from S3 before submitting operations, preventing stale cached data from overwriting backend-added meeting metadata.

## Prerequisites

- Access to the Change Management Portal UI
- Access to browser developer console (F12)
- Access to AWS CloudWatch logs for Lambda functions
- Access to DynamoDB or S3 to verify data persistence
- Ability to create, submit, and approve changes
- Backend meeting scheduling functionality must be working

## Test Environment Setup

1. Open the Change Management Portal in your browser
2. Open Developer Console (F12) and navigate to the Console tab
3. Keep CloudWatch logs open in another tab for Lambda monitoring
4. Have S3 console or AWS CLI ready to inspect change objects

## Test Suite

### Test 1: Cancel Operation Preserves Meeting Metadata

**Objective:** Verify that cancelling a change with a scheduled meeting preserves meeting metadata and successfully cancels the meeting.

**Steps:**

1. **Create a change with meeting required:**
   - Navigate to "Create Change" page
   - Fill in all required fields
   - Set "Meeting Required" to "Yes"
   - Fill in meeting details (title, date, duration, location)
   - Save as draft

2. **Submit the change:**
   - Go to "My Changes" page
   - Find your draft and click "Submit"
   - Verify change status changes to "submitted"

3. **Approve the change:**
   - Navigate to "View Changes" or have an approver approve it
   - Verify change status changes to "approved"
   - **Wait 30-60 seconds** for backend to schedule the meeting and update S3

4. **Verify meeting was scheduled:**
   - Check CloudWatch logs for meeting scheduling confirmation
   - Look for log entries showing `meeting_id` and `join_url` were written to S3
   - Optional: Check S3 directly to confirm the change object has meeting metadata

5. **Cancel the change:**
   - Go to "My Changes" page
   - Find the approved change
   - Click "Cancel" button
   - Confirm the cancellation dialog

6. **Verify in Browser Console:**
   - Look for these log messages:
     ```
     🔄 Reloading change from S3 before cancellation: [changeId]
     ✅ Reloaded fresh change from S3
     📋 Fresh change has meeting_id: true
     📋 Fresh change has join_url: true
     📤 Sending cancellation with fresh data
     📋 Sending meeting_id: true
     📋 Sending join_url: true
     ```
   - If any of these show `false`, the test has FAILED

7. **Verify in Lambda Logs (CloudWatch):**
   - Find the `handleCancelChange` execution logs
   - Look for:
     ```
     📋 Loaded change from S3 for cancellation
     📋 Change has meeting_id: true
     📋 Change has join_url: true
     ```
   - Look for meeting cancellation confirmation logs

8. **Verify in Data Store:**
   - Check S3 or DynamoDB for the cancelled change
   - Verify the change object still contains:
     - `meeting_id` field
     - `join_url` field
     - `status: "cancelled"`
     - `cancelledAt` timestamp
     - `cancelledBy` user email

9. **Verify Meeting Cancellation:**
   - Check Microsoft Graph/Outlook to confirm the meeting was cancelled
   - Or check backend logs for meeting cancellation API calls

**Expected Results:**
- ✅ Browser console shows meeting metadata was loaded and sent
- ✅ Lambda logs show meeting metadata was present in S3
- ✅ Change object in S3/DynamoDB retains meeting metadata
- ✅ Meeting was successfully cancelled in Microsoft Graph
- ✅ No errors in browser console or Lambda logs

**Failure Indicators:**
- ❌ Console shows `meeting_id: false` or `join_url: false`
- ❌ Lambda logs show missing meeting metadata
- ❌ Meeting was not cancelled in Microsoft Graph
- ❌ Change object lost meeting metadata after cancellation

---

### Test 2: Delete Operation Preserves Meeting Metadata

**Objective:** Verify that deleting a change with a scheduled meeting preserves meeting metadata and successfully cancels the meeting.

**Steps:**

1. **Create and approve a change with meeting:**
   - Follow steps 1-4 from Test 1
   - Ensure the change is approved and meeting is scheduled

2. **Delete the change:**
   - Go to "My Changes" page
   - Find the approved change
   - Click "Delete" button
   - Confirm the deletion dialog

3. **Verify in Browser Console:**
   - Look for these log messages:
     ```
     🔄 Reloading change from S3 before deletion: [changeId]
     ✅ Reloaded fresh change from S3
     📋 Fresh change has meeting_id: true
     📋 Fresh change has join_url: true
     📤 Sending deletion with fresh data
     ```
   - If any of these show `false`, the test has FAILED

4. **Verify in Lambda Logs (CloudWatch):**
   - Find the `handleDeleteChange` execution logs
   - Look for:
     ```
     📋 Loaded change from S3 for deletion
     📋 Change has meeting_id: true
     📋 Change has join_url: true
     ```
   - Look for meeting cancellation confirmation logs

5. **Verify in S3:**
   - Check that the change was moved to `deleted/archive/` folder
   - Verify the deleted change object still contains:
     - `meeting_id` field
     - `join_url` field
     - `deletedAt` timestamp
     - `deletedBy` user email

6. **Verify Meeting Cancellation:**
   - Check Microsoft Graph/Outlook to confirm the meeting was cancelled
   - Or check backend logs for meeting cancellation API calls

**Expected Results:**
- ✅ Browser console shows meeting metadata was loaded and sent
- ✅ Lambda logs show meeting metadata was present in S3
- ✅ Change moved to deleted folder with meeting metadata intact
- ✅ Meeting was successfully cancelled in Microsoft Graph
- ✅ No errors in browser console or Lambda logs

**Failure Indicators:**
- ❌ Console shows `meeting_id: false` or `join_url: false`
- ❌ Lambda logs show missing meeting metadata
- ❌ Meeting was not cancelled in Microsoft Graph
- ❌ Deleted change object lost meeting metadata

---

### Test 3: Complete Operation Preserves Meeting Metadata

**Objective:** Verify that completing a change preserves meeting metadata for audit trail purposes.

**Steps:**

1. **Create and approve a change with meeting:**
   - Follow steps 1-4 from Test 1
   - Ensure the change is approved and meeting is scheduled

2. **Complete the change:**
   - Go to "My Changes" page
   - Find the approved change
   - Click "Complete" button
   - Confirm the completion dialog

3. **Verify in Browser Console:**
   - Look for log messages showing the change object being sent
   - Verify meeting metadata is included in the request
   - Look for: `Has meeting_id: true` and `Has join_url: true`

4. **Verify in Lambda Logs (CloudWatch):**
   - Find the `handleCompleteChange` execution logs
   - Verify meeting metadata is present in the received change object

5. **Verify in Data Store:**
   - Check S3 or DynamoDB for the completed change
   - Verify the change object still contains:
     - `meeting_id` field
     - `join_url` field
     - `status: "completed"`
     - `completedAt` timestamp
     - `completedBy` user email

**Expected Results:**
- ✅ Browser console shows meeting metadata in the change object
- ✅ Lambda logs show meeting metadata was received
- ✅ Completed change retains meeting metadata for audit trail
- ✅ No errors in browser console or Lambda logs

**Failure Indicators:**
- ❌ Meeting metadata missing from browser console logs
- ❌ Lambda logs show missing meeting metadata
- ❌ Completed change object lost meeting metadata

---

### Test 4: Duplicate Operation Excludes Meeting Metadata

**Objective:** Verify that duplicating a change explicitly excludes meeting metadata so the duplicate doesn't reference the original meeting.

**Steps:**

1. **Create and approve a change with meeting:**
   - Follow steps 1-4 from Test 1
   - Ensure the change is approved and meeting is scheduled

2. **Duplicate the change:**
   - Go to "My Changes" page
   - Find the approved change
   - Click "Duplicate" button

3. **Verify in Browser Console:**
   - Look for the duplicated change object being created
   - Verify these fields are EXCLUDED:
     - `meeting_id` should be undefined/missing
     - `join_url` should be undefined/missing
   - Verify these fields are INCLUDED:
     - `meetingRequired`
     - `meetingTitle`
     - `meetingDate`
     - `meetingDuration`
     - `meetingLocation`

4. **Verify the duplicated change:**
   - The duplicate should open in edit mode or be saved as a draft
   - Check the change object in browser console
   - Confirm: `meeting_id: undefined` and `join_url: undefined`

5. **Submit and approve the duplicate:**
   - Submit the duplicated change
   - Have it approved
   - Verify a NEW meeting is scheduled (not reusing the original meeting)

**Expected Results:**
- ✅ Duplicated change does NOT have `meeting_id` or `join_url`
- ✅ Duplicated change DOES have other meeting fields (title, date, etc.)
- ✅ When approved, a NEW meeting is scheduled with a different `meeting_id`
- ✅ Original change's meeting is not affected

**Failure Indicators:**
- ❌ Duplicated change has `meeting_id` or `join_url` from original
- ❌ Duplicated change missing meeting configuration fields
- ❌ Duplicate references the original meeting instead of creating new one

---

## Race Condition Validation Test

**Objective:** Verify the fix handles the race condition where backend updates S3 after frontend loads the page.

**Steps:**

1. **Create and submit a change with meeting:**
   - Create a change with meeting required
   - Submit it for approval

2. **Approve the change:**
   - Approve the change (triggers backend meeting scheduling)

3. **IMMEDIATELY (within 5 seconds) try to cancel:**
   - Before backend finishes writing to S3
   - Click "Cancel" button
   - This tests if the frontend reload catches the race condition

4. **Verify behavior:**
   - If backend hasn't written yet: Frontend should reload and get the change without meeting metadata (acceptable)
   - If backend has written: Frontend should reload and get meeting metadata (desired)
   - Either way: No errors should occur, operation should complete

5. **Wait 60 seconds and try again:**
   - Refresh the page
   - Try to cancel again
   - This time, meeting metadata MUST be present

**Expected Results:**
- ✅ No errors occur even if cancelling immediately after approval
- ✅ After waiting, meeting metadata is definitely present and preserved
- ✅ System handles both scenarios gracefully

---

## Troubleshooting Guide

### Issue: Console shows `meeting_id: false`

**Possible Causes:**
1. Backend hasn't scheduled the meeting yet (wait longer)
2. Backend meeting scheduling failed (check backend logs)
3. S3 write failed (check S3 bucket permissions)
4. Frontend reload failed (check network tab for 404/500 errors)

**Resolution:**
- Check CloudWatch logs for meeting scheduling errors
- Verify S3 bucket has the change object with meeting metadata
- Check browser Network tab for failed API calls

### Issue: Lambda logs show missing meeting metadata

**Possible Causes:**
1. Frontend didn't reload from S3 (check browser console)
2. S3 object doesn't have meeting metadata (backend issue)
3. Lambda is reading from wrong S3 key

**Resolution:**
- Verify browser console shows successful reload
- Check S3 directly to see if meeting metadata exists
- Verify Lambda is using correct bucket and key

### Issue: Meeting not cancelled

**Possible Causes:**
1. Meeting metadata missing (see above)
2. Backend meeting cancellation API failed
3. Microsoft Graph API permissions issue

**Resolution:**
- Check backend logs for meeting cancellation attempts
- Verify Microsoft Graph API credentials and permissions
- Check if `meeting_id` is valid and exists in Microsoft Graph

---

## Test Results Template

Use this template to document your test results:

```markdown
## Test Execution Results

**Date:** [Date]
**Tester:** [Your Name]
**Environment:** [Production/Staging/Dev]

### Test 1: Cancel Operation
- Status: ✅ PASS / ❌ FAIL
- Browser Console: [Screenshot or logs]
- Lambda Logs: [CloudWatch log excerpt]
- Meeting Cancelled: YES / NO
- Notes: [Any observations]

### Test 2: Delete Operation
- Status: ✅ PASS / ❌ FAIL
- Browser Console: [Screenshot or logs]
- Lambda Logs: [CloudWatch log excerpt]
- Meeting Cancelled: YES / NO
- Notes: [Any observations]

### Test 3: Complete Operation
- Status: ✅ PASS / ❌ FAIL
- Browser Console: [Screenshot or logs]
- Lambda Logs: [CloudWatch log excerpt]
- Metadata Preserved: YES / NO
- Notes: [Any observations]

### Test 4: Duplicate Operation
- Status: ✅ PASS / ❌ FAIL
- Browser Console: [Screenshot or logs]
- Metadata Excluded: YES / NO
- New Meeting Created: YES / NO
- Notes: [Any observations]

### Race Condition Test
- Status: ✅ PASS / ❌ FAIL
- Immediate Cancel: [Result]
- Delayed Cancel: [Result]
- Notes: [Any observations]

## Overall Assessment
- All Tests Passed: YES / NO
- Ready for Production: YES / NO
- Issues Found: [List any issues]
- Recommendations: [Any recommendations]
```

---

## Success Criteria

The implementation is considered successful when:

1. ✅ All 4 main tests pass without errors
2. ✅ Browser console consistently shows meeting metadata being loaded and sent
3. ✅ Lambda logs consistently show meeting metadata present in S3
4. ✅ Meetings are successfully cancelled when changes are cancelled/deleted
5. ✅ Meeting metadata is preserved in all operations (except duplicate)
6. ✅ Duplicate operation correctly excludes meeting metadata
7. ✅ No race condition errors occur
8. ✅ System handles edge cases gracefully (immediate cancel, network errors, etc.)

---

## Next Steps After Testing

If all tests pass:
1. Document the test results using the template above
2. Mark tasks 6-9 as complete in `tasks.md`
3. Consider the race condition fix validated and ready for production

If any tests fail:
1. Document the failure details
2. Review the implementation code
3. Check for missing logging or error handling
4. Re-test after fixes are applied
