# Change Cancelled Event Type Implementation

## Summary
Added support for the `change_cancelled` event type to send cancellation notifications and automatically cancel scheduled Microsoft Graph meetings.

## Changes Made

### 1. Updated Request Type Detection
**File:** `internal/lambda/handlers.go`

Added `change_cancelled` detection in `DetermineRequestType()`:

```go
// Check status field
if statusLower == "cancelled" {
    return "change_cancelled"
}

// Check top-level status
if metadata.Status == "cancelled" {
    return "change_cancelled"
}
```

### 2. Added Change Cancelled Email Handler
**File:** `internal/lambda/handlers.go`

Created three new functions for change cancellation emails:

#### SendChangeCancelledEmail
Sends cancellation notification to customers using SES:
- Assumes customer role
- **Smart Topic Selection**:
  - If change was **approved**: Sends to `aws-announce` topic (broader audience)
  - If change was **not approved** (submitted/waiting): Sends to `aws-approval` topic (approval team only)
- Uses red/cancelled themed email template

#### sendChangeCancelledEmailDirect
Handles the actual SES email sending:
- Gets subscribed contacts for topic
- Sends individual emails to each subscriber
- Logs success/error counts

#### generateChangeCancelledHTML
Generates HTML email content:
- Red gradient banner with "❌ CHANGE CANCELLED"
- Change summary with title and customers
- Status marked as "❌ CANCELLED"
- Unsubscribe link

### 3. Added Meeting Cancellation Logic
**File:** `internal/lambda/handlers.go`

#### CancelScheduledMeetingIfNeeded
Cancels Microsoft Graph meetings when a change is cancelled:
- Checks for scheduled meeting in modifications array
- Calls Microsoft Graph API to delete the meeting
- Updates S3 object with meeting_cancelled modification entry
- Handles cases where meeting doesn't exist (already deleted)

#### cancelGraphMeeting
Calls Microsoft Graph API to delete a meeting:
- Uses DELETE endpoint: `/users/{user-id}/events/{event-id}`
- Returns success on 204 No Content
- Handles 404 Not Found gracefully (meeting already gone)
- Logs detailed error information

### 4. Updated Unknown Event Type Handling
**File:** `internal/lambda/handlers.go`

Changed default case to ignore unknown event types instead of treating them as approval requests:

```go
default:
    log.Printf("⚠️  Unknown event type '%s' - ignoring", requestType)
    // Do not process unknown event types - just log and return
    return nil
```

## Event Flow

### When a Change is Cancelled:

1. **S3 Event Triggered**: Change object updated with `status: "cancelled"`
2. **Request Type Detection**: `DetermineRequestType()` returns `"change_cancelled"`
3. **Approval Status Check**: System checks if change was ever approved:
   - Looks for `ModificationTypeApproved` in modifications array
   - Checks `status`, `approvedAt`, and `approvedBy` fields as fallback
4. **Email Notification**: `SendChangeCancelledEmail()` sends cancellation emails:
   - **If approved**: Sends to `aws-announce` topic (all stakeholders)
   - **If not approved**: Sends to `aws-approval` topic (approval team only)
5. **Meeting Cancellation**: `CancelScheduledMeetingIfNeeded()` checks for scheduled meeting
6. **Graph API Call**: `cancelGraphMeeting()` deletes the meeting via Microsoft Graph API
7. **S3 Update**: Adds `meeting_cancelled` modification entry to S3 object

## Email Template

The cancellation email features:
- **Subject**: `❌ CANCELLED: {Change Title}`
- **Banner**: Red gradient with "❌ CHANGE CANCELLED"
- **Content**: Change title, customers, and cancelled status
- **Styling**: Red theme (#dc3545) to indicate cancellation

### Topic Selection Logic

The system intelligently routes cancellation emails based on approval status:

**Approved Changes → `aws-announce` topic**
- Change was approved and announced to stakeholders
- Cancellation should go to same broad audience
- Checked by:
  1. Looking for `ModificationTypeApproved` in modifications array
  2. Checking `status == "approved"`
  3. Checking if `approvedAt` timestamp exists
  4. Checking if `approvedBy` field is populated

**Not Approved Changes → `aws-approval` topic**
- Change was still in approval/submitted state
- Cancellation only needs to go to approval team
- No need to notify broader stakeholder audience

This ensures:
- Stakeholders only get cancellation notices for changes they were notified about
- Approval team sees all cancellations (both approved and unapproved)
- Reduces noise for stakeholders

## Meeting Cancellation

### Microsoft Graph API Integration:
- **Endpoint**: `DELETE /v1.0/users/{organizer}/events/{meeting-id}`
- **Success Response**: 204 No Content
- **Already Deleted**: 404 Not Found (treated as success)
- **Error Handling**: Logs errors but continues with S3 update

### S3 Modification Entry:
```json
{
  "timestamp": "2025-01-15T16:00:00Z",
  "user_id": "arn:aws:iam::123456789012:role/backend-lambda-role",
  "modification_type": "meeting_cancelled"
}
```

## Unknown Event Types

### The Problem
Previously, unknown event types were **incorrectly defaulted to approval requests**, which caused:
- Duplicate approval request emails being sent
- Confusion for recipients
- Incorrect workflow processing

**Root Cause**: The `DetermineRequestType()` function had this problematic default:
```go
// Default to approval_request for unknown cases (most common workflow)
return "approval_request"  // ❌ BAD - causes duplicate emails
```

### The Fix
Changed the default behavior to return `"unknown"` instead:

```go
// Return unknown for unrecognized cases - do not default to approval_request
// This prevents incorrect email notifications for unknown event types
log.Printf("⚠️  Could not determine request type from metadata - Status: %s, Source: %s", metadata.Status, metadata.Source)
return "unknown"  // ✅ GOOD - will be caught by default case
```

Now, unknown event types are:
- Returned as `"unknown"` from `DetermineRequestType()`
- Logged with warning: `⚠️  Unknown event type 'unknown' - ignoring`
- Silently ignored (no email sent)
- Removed from queue (not retried)

This prevents the scenario where:
1. Change is approved → sends approved_announcement email ✅
2. Unknown event triggers → was sending approval_request email ❌
3. Now → unknown event is ignored ✅

## Testing

### To Test Change Cancellation:

1. Create and approve a change with a meeting
2. Update the change status to "cancelled":
   ```json
   {
     "status": "cancelled",
     "request_type": "change_cancelled"
   }
   ```
3. Verify:
   - Cancellation email sent to customers
   - Meeting deleted from Microsoft Graph
   - S3 object updated with `meeting_cancelled` entry

### Expected Log Output:

**For Approved Change:**
```
📧 Sending change cancelled notification email for change CHG-123
📧 Change CHG-123 was approved - sending cancellation to aws-announce topic
📧 Sending change cancelled notification email for change CHG-123 to topic aws-announce
✅ Change cancelled notification email sent to 15 members of topic aws-announce
🔍 Checking if change CHG-123 has a scheduled meeting to cancel
📅 Found scheduled meeting for change CHG-123: ID=AAMkAGVm...
🗑️  Cancelling Graph meeting: ID=AAMkAGVm..., Organizer=ccoe@hearst.com
✅ Successfully deleted Graph meeting AAMkAGVm...
✅ Updated S3 object with meeting cancelled entry
```

**For Unapproved Change:**
```
📧 Sending change cancelled notification email for change CHG-456
📧 Change CHG-456 was not approved - sending cancellation to aws-approval topic
📧 Sending change cancelled notification email for change CHG-456 to topic aws-approval
✅ Change cancelled notification email sent to 5 members of topic aws-approval
🔍 Checking if change CHG-456 has a scheduled meeting to cancel
✅ No scheduled meeting found for change CHG-456, nothing to cancel
```

## Error Handling

### Meeting Cancellation Errors:
- If Graph API call fails, logs error but continues
- S3 update still happens even if meeting deletion fails
- 404 Not Found treated as success (meeting already gone)

### Email Sending Errors:
- Logs individual recipient failures
- Returns error if any emails fail
- Provides summary of success/error counts

## Related Files

- `internal/lambda/handlers.go` - Main implementation
- `internal/lambda/modifications.go` - Modification entry creation
- `internal/lambda/s3_operations.go` - S3 update operations
- `internal/ses/meetings.go` - Graph API access token
- `internal/types/types.go` - Type definitions

## Environment Variables

- `MEETING_ORGANIZER_EMAIL` - Email of meeting organizer (default: ccoe@hearst.com)
- `BACKEND_ROLE_ARN` - IAM role ARN for modification entries

## Supported Event Types

After this change, the system supports:
1. `approval_request` - Request approval for a change
2. `approved_announcement` - Announce approved change and schedule meeting
3. `change_complete` - Notify that change is completed
4. `change_cancelled` - Notify that change is cancelled and cancel meeting
5. Unknown types - Logged and ignored

## Frontend Protection

### Action Button Visibility
**File:** `html/my-changes.html`

Action buttons (Edit, Approve, Complete, Cancel) are only shown for changes that are NOT completed or cancelled:

```javascript
${(change.status !== 'completed' && change.status !== 'cancelled') ? `
    // Edit, Approve, Complete, and Cancel buttons only appear for active changes
    ${change.status === 'draft' ? `
        <a href="create-change.html?changeId=${change.changeId}" class="action-btn primary">
            ✏️ Edit
        </a>
        <button class="action-btn success" onclick="submitDraft('${change.changeId}')">
            🚀 Submit
        </button>
    ` : `
        <a href="edit-change.html?changeId=${change.changeId}" class="action-btn primary">
            ✏️ Edit
        </a>
    `}
    
    ${(change.status === 'submitted' || change.status === 'waiting for approval') ? `
        <button class="action-btn approve" onclick="approveChange('${change.changeId}')">
            ✅ Approve
        </button>
        <button class="action-btn cancel" onclick="cancelChange('${change.changeId}')">
            💣 Cancel
        </button>
    ` : ''}
    
    ${change.status === 'approved' ? `
        <button class="action-btn complete" onclick="completeChange('${change.changeId}')">
            🎯 Complete
        </button>
        <button class="action-btn cancel" onclick="cancelChange('${change.changeId}')">
            💣 Cancel
        </button>
    ` : ''}
` : ''}
```

**Completed and Cancelled changes only show:**
- 🗑️ Delete button
- 📋 Duplicate button

### Cancel Function Validation

Added explicit status check in `cancelChange()` function:

```javascript
// Prevent cancelling completed changes
if (change.status === 'completed') {
    if (window.portal) {
        window.portal.showStatus('Cannot cancel a completed change', 'error');
    }
    return;
}
```

This provides defense-in-depth:
1. **UI Layer**: Action buttons (Edit, Cancel, etc.) not shown for completed or cancelled changes
2. **Function Layer**: Explicit check prevents cancellation even if button somehow appears
3. **User Feedback**: Clear error message if attempted

### Button Visibility by Status

| Status | Delete | Duplicate | Edit | Submit | Approve | Complete | Cancel |
|--------|--------|-----------|------|--------|---------|----------|--------|
| Draft | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Submitted | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ |
| Approved | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ |
| Completed | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Cancelled | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |

## Benefits

1. **Automatic Meeting Cleanup**: Cancelled changes automatically cancel their meetings
2. **Clear Communication**: Recipients know immediately when a change is cancelled
3. **Audit Trail**: S3 object tracks meeting cancellation
4. **Error Resilience**: Handles edge cases like already-deleted meetings
5. **Cleaner Processing**: Unknown event types no longer trigger incorrect emails
6. **Protected Workflow**: Completed changes cannot be cancelled (UI + validation)
