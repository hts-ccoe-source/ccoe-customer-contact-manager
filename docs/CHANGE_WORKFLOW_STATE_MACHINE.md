# Change and Announcement Workflow State Machine

## Overview

This document defines the complete state machine for both change management and announcement workflows, including valid status transitions, business rules, and operation constraints. The same state machine applies to both object types.

## Change Status States

### Valid States

1. **draft** - Initial state, change is being created/edited
2. **submitted** - Change has been submitted for approval
3. **approved** - Change has been approved and scheduled
4. **completed** - Change implementation has been completed
5. **cancelled** - Change has been cancelled

### State Diagram

```
┌─────────┐
│  draft  │
└────┬────┘
     │ submit
     │
     │ delete (draft only)
     │         │
     ▼         ▼
┌───────────┐  [deleted]
│ submitted │
└─────┬─────┘
      │ approve
      │         │
      │ cancel  │
      ▼         ▼
┌──────────┐  ┌──────────┐
│ approved │  │cancelled │
└────┬─────┘  └────┬─────┘
     │ complete    │ delete (cancelled only)
     │ cancel      │
     ▼             ▼
┌───────────┐  [deleted]
│ completed │
└───────────┘
(permanent)
```

## Status Transitions

### Valid Transitions

| From State | To State | Operation | Trigger | Meeting Action |
|-----------|----------|-----------|---------|----------------|
| draft | submitted | Submit | User submits change | None |
| draft | deleted | Delete | User deletes draft | None |
| submitted | approved | Approve | User approves change | Schedule meeting (if required) |
| submitted | cancelled | Cancel | User cancels change | None (no meeting scheduled yet) |
| approved | submitted | Edit | User edits approved change | **Cancel meeting** (reverts to submitted) |
| approved | completed | Complete | User completes change | None (meeting already occurred) |
| approved | cancelled | Cancel | User cancels change | **Cancel meeting** (critical!) |
| cancelled | deleted | Delete | User deletes cancelled change | None (meeting already cancelled) |

### Invalid Transitions

| From State | To State | Reason |
|-----------|----------|--------|
| draft | cancelled | Drafts are just deleted, not cancelled |
| draft | approved | Must go through submitted state first |
| draft | completed | Must go through submitted → approved first |
| submitted | deleted | Must cancel or approve first, cannot delete directly |
| approved | deleted | Must cancel first, cannot delete directly |
| completed | cancelled | Cannot cancel a completed change |
| completed | deleted | Cannot delete completed changes (permanent record) |
| completed | any | Completed changes are final |
| cancelled | any (except deleted) | Cancelled changes can only be deleted |

## Business Rules

### Edit Operation

**Allowed States:**

- ✅ **draft** - Can edit draft changes (remains draft)
- ✅ **submitted** - Can edit submitted changes (remains submitted)
- ✅ **approved** - Can edit approved changes (**reverts to submitted**, requires re-approval)

**Disallowed States:**

- ❌ **cancelled** - Cannot edit cancelled changes (final state)
- ❌ **completed** - Cannot edit completed changes (permanent record)

**Status Change on Edit:**

- **draft** → edit → **draft** (no status change)
- **submitted** → edit → **submitted** (no status change)
- **approved** → edit → **submitted** (REVERTS to submitted, requires re-approval)

**Meeting Impact:**

- WHEN approved change is edited
- THEN status reverts to submitted
- AND scheduled meeting MUST be cancelled
- AND change requires re-approval before new meeting is scheduled

### Cancel Operation

**Allowed States:**

- ✅ **submitted** - Can cancel before approval
- ✅ **approved** - Can cancel after approval (MUST cancel meeting)

**Disallowed States:**

- ❌ **draft** - Drafts are deleted, not cancelled
- ❌ **completed** - Cannot cancel completed changes
- ❌ **cancelled** - Already cancelled

**Meeting Cancellation:**

- WHEN status is **approved** AND meeting was scheduled
- THEN meeting MUST be cancelled via Microsoft Graph API
- BEFORE sending cancellation email notification

### Delete Operation

**Allowed States:**

- ✅ **draft** - Can delete draft changes
- ✅ **cancelled** - Can delete cancelled changes

**Disallowed States:**

- ❌ **submitted** - Cannot delete submitted changes (must cancel or approve first)
- ❌ **approved** - Cannot delete approved changes (must cancel first)
- ❌ **completed** - Cannot delete completed changes (permanent record)

**Meeting Cancellation:**

- Delete operation does NOT cancel meetings
- Changes must be cancelled FIRST (which cancels the meeting)
- THEN the cancelled change can be deleted

### Duplicate Operation

**Allowed States:**

- ✅ **draft** - Can duplicate draft changes/announcements
- ✅ **submitted** - Can duplicate submitted changes/announcements
- ✅ **approved** - Can duplicate approved changes/announcements
- ✅ **cancelled** - Can duplicate cancelled changes/announcements
- ✅ **completed** - Can duplicate completed changes/announcements

**Behavior:**

- Creates a new change/announcement with a new ID
- Copies all content, metadata, and settings from the original
- New duplicate starts in **draft** status
- User is redirected to edit page for the new duplicate
- Original change/announcement remains unchanged
- Available on all statuses (no restrictions)

### Complete Operation

**Allowed States:**

- ✅ **approved** - Can complete approved changes

**Disallowed States:**

- ❌ **draft** - Must be submitted and approved first
- ❌ **submitted** - Must be approved first
- ❌ **cancelled** - Cannot complete cancelled changes
- ❌ **completed** - Already completed

## Meeting Lifecycle

### Meeting Scheduling

**Trigger:** Change status transitions from **submitted** → **approved**

**Conditions:**

- Change has `meetingRequired: 'yes'`
- Meeting details are provided (title, date, duration, location)

**Actions:**

1. Backend schedules meeting via Microsoft Graph API
2. Backend updates S3 with meeting metadata in top-level fields: `meeting_id`, `join_url`
3. Backend adds `meeting_scheduled` event to modifications array (timestamp and modifier only, no meeting data)
4. Meeting invites sent to participants

### Meeting Cancellation

**Trigger:** Change status transitions to **cancelled** OR change is **deleted**

**Conditions:**

- Change status is **approved** (meeting was scheduled)
- Change has `meeting_id` field populated

**Actions:**

1. **FIRST:** Backend cancels meeting via Microsoft Graph API
2. **THEN:** Backend sends cancellation email notification
3. Meeting removed from participants' calendars

**Critical Order:**

- Meeting cancellation MUST happen BEFORE email notification
- Ensures meeting is removed from calendars before users receive email

## Frontend Operations

### Status Change Operations

All status change operations follow this pattern:

1. **Reload from S3** - Get latest data (for cancel/delete/edit-approved)
2. **Validate state** - Check if operation is allowed
3. **Confirm with user** - Show confirmation dialog (for cancel/delete only)
4. **Submit operation** - Send request to Lambda
5. **Update UI** - Refresh change list

**Operations requiring S3 reload:**

- ✅ **Cancel** - Need meeting metadata to cancel meeting
- ✅ **Delete** - Need to validate status (can only delete draft or cancelled)
- ✅ **Edit (approved only)** - Need meeting metadata to cancel meeting when reverting to submitted

**Operations requiring confirmation:**

- ✅ **Cancel** - Destructive operation, cancels meeting
- ✅ **Delete** - Destructive operation, moves to deleted folder
- ✅ **Edit (approved only)** - Warning that scheduled meeting will be cancelled

**Confirmation messages:**

- **Cancel**: "Are you sure you want to cancel this change? This will cancel any scheduled meetings."
- **Delete**: "Are you sure you want to delete this change? This will move it to the deleted folder."
- **Edit (approved)**: "This change has been approved and a meeting is scheduled. Editing will revert the status to submitted and cancel the meeting. Continue?"

**Operations NOT requiring confirmation:**

- ❌ **Submit** - Normal workflow progression
- ❌ **Approve** - Normal workflow progression
- ❌ **Complete** - Normal workflow progression
- ❌ **Edit (draft/submitted)** - Normal editing operation

### Operation-Specific Patterns

#### Submit Operation

```javascript
async submitChange(changeId) {
    const change = this.allChanges.find(c => c.changeId === changeId);
    // Modify: status = 'submitted', add submittedAt/submittedBy
    POST /changes/{id}/submit with modified change
}
```

#### Approve Operation

```javascript
async approveChange(changeId) {
    const change = this.allChanges.find(c => c.changeId === changeId);
    // Modify: status = 'approved', add approvedAt/approvedBy
    POST /changes/{id}/approve with modified change
    // Backend will schedule meeting if required
}
```

#### Complete Operation

```javascript
async completeChange(changeId) {
    const change = this.allChanges.find(c => c.changeId === changeId);
    // Modify: status = 'completed', add completedAt/completedBy
    POST /changes/{id}/complete with modified change
}
```

#### Cancel Operation

```javascript
async cancelChange(changeId) {
    // CRITICAL: Reload from S3 first
    const freshChange = await GET /changes/{id};
    
    // Validate: cannot cancel completed changes
    if (freshChange.status === 'completed') {
        return error;
    }
    
    // Modify: status = 'cancelled', add cancelledAt/cancelledBy
    POST /changes/{id}/cancel with modified fresh change
    // Backend will cancel meeting if status was 'approved'
}
```

#### Delete Operation

```javascript
async deleteChange(changeId) {
    // CRITICAL: Reload from S3 first
    const freshChange = await GET /changes/{id};
    
    // Validate: can only delete draft or cancelled changes
    if (freshChange.status !== 'draft' && freshChange.status !== 'cancelled') {
        return error('Can only delete draft or cancelled changes');
    }
    
    DELETE /changes/{id}
    // No meeting cancellation needed (drafts have no meeting, cancelled already cancelled)
}
```

## Frontend and Backend Processing

### Frontend node js 'upload' Lambda Handler Flow

```
1. Lambda receives request (cancel/delete/complete/etc.)
2. Lambda loads change from S3 (single source of truth)
3. Lambda validates operation is allowed
4. Lambda performs operation-specific actions:
   - Cancel: Update status to cancelled
   - Delete: Move to deleted folder
   - Complete: Update status to completed
5. Lambda writes updated change to S3
6. Lambda writes to customer buckets (S3 event → SQS → triggers backend)
```

### Backend Go Lang Lambda Event Processing

```
1. Backend receives S3 event notification via SQS
2. Backend loads change from S3
3. Backend processes event, potentially sends templated email
4. Backend performs additional actions based on event type:
   - approved_announcement: Schedule meeting (if required) → Send approval email
   - change_cancelled: Cancel meeting (if meeting_id exists) → Send cancellation email
   - change_deleted: Cancel meeting (if meeting_id exists)
   - change_completed: Send completion email
5. Backend updates S3 with meeting metadata (if applicable, e.g., after s
cheduling)
```

## Data Flow

### Meeting Metadata Storage

**Single Storage Strategy:**

1. **Top-level fields** (primary storage, survives Lambda overwrites):

   ```json
   {
     "meeting_id": "AAMkAD...",
     "join_url": "https://teams.microsoft.com/..."
   }
   ```

2. **Modifications array** (audit trail):

   ```json
   {
     "modifications": [
       {
         "modificationType": "meeting_scheduled",
         "timestamp": "2025-01-10T15:30:00Z",
         "modifiedBy": "arn:aws:iam::123456789012:role/backend-lambda-role"
       }
     ]
   }
   ```

   **Note:** Meeting metadata (meetingId, joinUrl) is NOT stored in modifications array - only in top-level fields. The modifications array tracks all change events (meeting_scheduled, status_changed, field_updated, etc.) with timestamps and the modifier (user email or IAM role ARN).

### Race Condition Prevention

**Problem:** Frontend cache becomes stale after backend updates S3

**Solution:** Frontend reloads from S3 before cancel/delete/edit-approved operations

**Flow:**

1. User clicks Cancel/Delete/Edit-approved
2. Frontend GET /changes/{id} (reload from S3)
3. Frontend validates operation with fresh data
4. Frontend submits operation with fresh data
5. Backend Lambda loads from S3 (single source of truth)
6. Backend processes with complete meeting metadata

## Validation Rules

### Frontend Validation

- ✅ Cannot cancel completed changes
- ✅ Cannot delete completed changes
- ✅ Must reload from S3 before cancel/delete
- ✅ Must confirm destructive operations with user

### Lambda Validation

- ✅ Verify change exists in S3
- ✅ Validate operation is allowed for current status

### Backend Validation

- ✅ Check meeting_id exists before attempting cancellation
- ✅ Log all operations for audit trail

**Note:** User ownership verification is only needed for displaying "My Changes" in the UI, not for Lambda operations.

## Error Handling

### Frontend Errors

- **S3 reload fails** → Display error, abort operation
- **API request fails** → Display error, allow retry
- **Network error** → Display error, allow retry

### Backend Errors

- **Meeting cancellation fails** → Log error, continue with status change
- **Email sending fails** → Log error, status change still succeeds
- **S3 write fails** → Return error, operation fails

## Logging Requirements

### Frontend Logging

```javascript
console.log('🔄 Reloading change from S3:', changeId);
console.log('✅ Reloaded fresh change from S3');
console.log('📋 Fresh change has meeting_id:', !!freshChange.meeting_id);
console.log('📤 Sending cancellation with fresh data');
```

### Lambda Logging

```javascript
console.log('📋 Loaded change from S3 for cancellation');
console.log('📋 Change has meeting_id:', !!change.meeting_id);
console.log('📋 Change has join_url:', !!change.join_url);
```

### Backend Logging

```go
log.Printf("✅ Found meeting_id in top-level fields: %s", metadata.MeetingID)
log.Printf("📅 Cancelling meeting: %s", meetingID)
log.Printf("✅ Meeting cancelled successfully")
```

## Architecture Decisions

### Why Frontend Reloads from S3

**Problem:** Frontend cache becomes stale after backend updates

**Solution:** Reload before destructive operations

**Benefits:**

- Ensures latest meeting metadata is available
- Prevents race conditions
- Validates current state before operation
- Better user experience (shows fresh data)

### Why Upload Lambda Loads from S3

**Problem:** Request body might be stale or missing data

**Solution:** Node.js Upload Lambda always loads from S3 as single source of truth

**Benefits:**

- Single source of truth (S3)
- Simpler Lambda code (no request body parsing needed)
- Idempotent operations
- No race conditions

### Why Top-Level Fields for Meeting Metadata

**Problem:** Lambda overwrites complete S3 objects, losing meeting metadata

**Solution:** Store meeting metadata ONLY in top-level fields (`meeting_id`, `join_url`)

**Benefits:**

- Top-level fields survive Lambda overwrites
- Single source of truth (no duplication)
- Simpler data model
- Modifications array tracks events only (not data)

## Future Improvements

### Unified Status Change Pattern

**Current:** Frontend modifies object, Lambda writes to S3

**Proposed:** Lambda handles all status modifications

**Benefits:**

- DRY (Don't Repeat Yourself)
- Consistent pattern across all operations
- Simpler frontend code
- Atomic operations (no race conditions)

**Implementation:**

```javascript
// Frontend just triggers operation
POST /changes/{id}/approve  // No body needed

// Lambda handles everything
1. Load from S3
2. Modify status
3. Write to S3
4. Trigger backend processing
```

This would make all operations (submit, approve, complete, cancel, delete) work the same way.

## References

- Frontend: `html/my-changes.html`
- Lambda: `lambda/upload_lambda/upload-metadata-lambda.js`
- Backend: `internal/lambda/handlers.go`
- Types: `internal/types/types.go`
- S3 Operations: `internal/lambda/s3_operations.go`
