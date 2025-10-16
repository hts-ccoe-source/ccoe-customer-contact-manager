# Task 19.5: Announcement Action Workflows - Testing Plan

## Overview
Testing plan for announcement action buttons (approve, cancel, complete) to verify the complete workflow from frontend to backend.

## Test Environment Setup

### Prerequisites
1. ✅ Frontend files deployed to S3/CloudFront
   - `html/assets/js/announcement-actions.js`
   - `html/assets/js/announcement-details-modal.js`
   - `html/assets/js/approvals-page.js`
   - `html/approvals.html`

2. ✅ Backend Lambda deployed
   - `lambda/upload_lambda/upload-metadata-lambda.js` with `handleUpdateAnnouncement`

3. Test data needed:
   - At least one announcement in `submitted` or `pending_approval` status
   - Announcement should have multiple customers
   - User should be authenticated via SAML

## Test Scenarios

### Test 1: Approve Action Workflow
**Objective**: Verify announcement can be approved and triggers backend processing

**Steps**:
1. Navigate to approvals page
2. Find an announcement with status `submitted` or `pending_approval`
3. Click "Approve" button
4. Confirm the approval dialog
5. Wait for success message

**Expected Results**:
- ✅ Confirmation dialog appears
- ✅ Button shows loading state during processing
- ✅ Success message displays
- ✅ Announcement status changes to `approved`
- ✅ Modification entry added with timestamp and user
- ✅ Page refreshes showing updated status
- ✅ SQS notification sent (check backend logs)
- ✅ Backend processes email/meeting scheduling

**API Call Verification**:
```json
POST /api/upload
{
  "action": "update_announcement",
  "announcement_id": "CIC-2025-001",
  "status": "approved",
  "modification": {
    "timestamp": "2025-10-16T...",
    "user_id": "user@example.com",
    "modification_type": "approved"
  },
  "customers": ["hts", "cds"]
}
```

**Backend Verification**:
- Check S3 `archive/CIC-2025-001.json` - status should be `approved`
- Check S3 `customers/hts/CIC-2025-001.json` - status should be `approved`
- Check S3 `customers/cds/CIC-2025-001.json` - status should be `approved`
- Check SQS queue for notification message
- Check backend Lambda logs for email sending

---

### Test 2: Cancel Action Workflow
**Objective**: Verify announcement can be cancelled with reason

**Steps**:
1. Navigate to approvals page
2. Find an announcement with status `submitted` or `pending_approval`
3. Click "Cancel" button
4. Enter cancellation reason in prompt
5. Confirm cancellation

**Expected Results**:
- ✅ Reason prompt appears
- ✅ Button shows loading state
- ✅ Success message displays
- ✅ Announcement status changes to `cancelled`
- ✅ Modification entry includes cancellation reason
- ✅ Page refreshes showing updated status
- ✅ If meeting was scheduled, it should be cancelled

**API Call Verification**:
```json
POST /api/upload
{
  "action": "update_announcement",
  "announcement_id": "FIN-2025-001",
  "status": "cancelled",
  "modification": {
    "timestamp": "2025-10-16T...",
    "user_id": "user@example.com",
    "modification_type": "cancelled",
    "reason": "User provided reason"
  },
  "customers": ["hts"]
}
```

---

### Test 3: Complete Action Workflow
**Objective**: Verify approved announcement can be marked complete

**Steps**:
1. Navigate to approvals page
2. Find an announcement with status `approved`
3. Click "Complete" button
4. Confirm completion

**Expected Results**:
- ✅ Confirmation dialog appears
- ✅ Button shows loading state
- ✅ Success message displays
- ✅ Announcement status changes to `completed`
- ✅ Modification entry added
- ✅ Page refreshes showing updated status
- ✅ No action buttons shown for completed announcement

---

### Test 4: Invalid Status Transitions
**Objective**: Verify invalid transitions are blocked

**Test Cases**:
1. Try to approve a `completed` announcement → Should fail
2. Try to complete a `submitted` announcement → Should fail
3. Try to approve a `cancelled` announcement → Should fail

**Expected Results**:
- ✅ Frontend validation prevents invalid actions
- ✅ Backend returns 400 error with clear message
- ✅ Error message displayed to user
- ✅ Announcement status unchanged

---

### Test 5: Multi-Customer Updates
**Objective**: Verify announcement updates apply to all customers

**Steps**:
1. Create/find announcement with multiple customers (e.g., hts, cds, fdbus)
2. Approve the announcement
3. Verify S3 updates

**Expected Results**:
- ✅ Archive updated: `archive/ANN-ID.json`
- ✅ Customer 1 updated: `customers/hts/ANN-ID.json`
- ✅ Customer 2 updated: `customers/cds/ANN-ID.json`
- ✅ Customer 3 updated: `customers/fdbus/ANN-ID.json`
- ✅ All files have same status and modifications array
- ✅ API response shows success for all customers

---

### Test 6: Modification History Tracking
**Objective**: Verify modification entries are correctly added

**Steps**:
1. Create new announcement (status: draft)
2. Submit for approval (status: pending_approval)
3. Approve (status: approved)
4. Complete (status: completed)
5. View announcement details modal

**Expected Results**:
- ✅ Modifications array has 4 entries
- ✅ Each entry has: timestamp, user_id, modification_type
- ✅ Timeline displays in correct order
- ✅ Icons and labels are correct
- ✅ Timestamps are formatted properly

---

### Test 7: Announcement Details Modal Actions
**Objective**: Verify action buttons work in modal

**Steps**:
1. Open announcement details modal
2. Verify action buttons appear in footer
3. Click approve/cancel/complete from modal
4. Verify modal refreshes after action

**Expected Results**:
- ✅ Action buttons render in modal footer
- ✅ Buttons match announcement status
- ✅ Actions work same as in list view
- ✅ Modal shows updated status after action
- ✅ Can close modal after action completes

---

### Test 8: Error Handling
**Objective**: Verify graceful error handling

**Test Cases**:
1. Network failure during update
2. Invalid announcement ID
3. Missing required fields
4. Backend Lambda error

**Expected Results**:
- ✅ Error messages displayed to user
- ✅ Buttons re-enabled after error
- ✅ No partial updates (all-or-nothing)
- ✅ User can retry action
- ✅ Console logs show detailed error info

---

### Test 9: Permission Checks (Future - Task 19.6)
**Objective**: Verify unauthorized users cannot perform actions

**Note**: This will be implemented in Task 19.6

---

### Test 10: Browser Compatibility
**Objective**: Verify functionality across browsers

**Browsers to Test**:
- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)

**Expected Results**:
- ✅ All buttons render correctly
- ✅ Modals display properly
- ✅ Actions work in all browsers
- ✅ No console errors

---

## Manual Testing Checklist

### Frontend Testing
- [ ] Announcement action buttons render correctly in approvals page
- [ ] Buttons show correct states (pending → approve/cancel, approved → complete/cancel)
- [ ] Button loading states work
- [ ] Confirmation dialogs appear
- [ ] Success/error messages display
- [ ] Page refreshes after actions
- [ ] Modal action buttons work
- [ ] Timeline displays modifications correctly

### Backend Testing
- [ ] API accepts update_announcement action
- [ ] Status transition validation works
- [ ] Multi-customer S3 updates succeed
- [ ] Modification entries added correctly
- [ ] SQS notifications sent on approval
- [ ] Error responses are clear and helpful
- [ ] Logs show detailed processing info

### Integration Testing
- [ ] Frontend → Backend communication works
- [ ] S3 updates trigger backend processing
- [ ] Email notifications sent on approval
- [ ] Meeting scheduling works (if configured)
- [ ] Meeting cancellation works
- [ ] End-to-end workflow completes successfully

---

## Test Data Setup

### Create Test Announcement
```bash
# Use create-announcement.html to create test announcements with:
- Type: CIC, FinOps, or InnerSource
- Multiple customers
- Optional meeting
- Status: submitted or pending_approval
```

### Check S3 Structure
```bash
aws s3 ls s3://bucket-name/archive/ --recursive | grep -E "(CIC|FIN|INN)-"
aws s3 ls s3://bucket-name/customers/hts/ --recursive | grep -E "(CIC|FIN|INN)-"
```

### Check Backend Logs
```bash
# For upload Lambda
aws logs tail /aws/lambda/upload-lambda --follow

# For backend Lambda
aws logs tail /aws/lambda/backend-lambda --follow
```

---

## Known Issues / Notes

1. **Meeting Cancellation**: Requires backend Lambda to handle cancelled status
2. **Email Templates**: Backend should use type-specific templates (Task 15 completed)
3. **Permission Checks**: Not yet implemented (Task 19.6)

---

## Success Criteria

All tests pass with:
- ✅ No console errors
- ✅ Correct status transitions
- ✅ Proper modification history
- ✅ Multi-customer updates work
- ✅ Backend processing triggered
- ✅ User-friendly error messages
- ✅ Consistent behavior across browsers

---

## Next Steps After Testing

1. Fix any bugs found during testing
2. Document any edge cases
3. Implement Task 19.6 (permission checks)
4. Update user documentation
5. Deploy to production
