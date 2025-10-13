# Design Document: Frontend Meeting Metadata Preservation

## Overview

This design addresses the issue where frontend JavaScript operations inadvertently strip out meeting metadata (`meeting_id` and `join_url`) when performing cancel and delete operations. The root cause is a **race condition between frontend cache and backend S3 updates**:

**The Problem:**
1. Cancel/Delete buttons exist on **list pages**, not edit forms
2. Backend **asynchronously updates S3** with meeting metadata after scheduling
3. Frontend uses **cached data** that may not include the latest S3 updates
4. Lambda handlers **overwrite S3** with the stale data sent by frontend
5. Meeting metadata is **lost**, causing cancellation to fail

**The Solution:**
- **Cancel/Delete operations** must **reload from S3** before submitting
- **Lambda handlers** must **use request body data** (not just S3) for meeting cancellation
- **Comprehensive logging** to track data flow and diagnose issues

## Architecture

### Current Architecture Issues

The current implementation in `html/my-changes.html` has critical race condition issues:

**The Race Condition:**
1. **T0**: User approves change → Backend schedules meeting → Updates S3 with `meeting_id` and `join_url`
2. **T1**: User loads "My Changes" page → Frontend caches changes (may not have latest S3 version)
3. **T2**: User clicks "Cancel" → Frontend uses **cached data** (missing meeting metadata)
4. **T3**: Lambda processes cancel → Overwrites S3 with cached data → **Meeting metadata lost**
5. **T4**: Backend processes S3 event → Cannot find meeting metadata → **Meeting not cancelled**

**Key Architectural Constraint:**
- Cancel/Delete buttons are on **list pages**, not edit forms
- Any edits have already been persisted to S3 before button click
- Frontend cache becomes stale after backend S3 updates
- Lambda overwrites complete S3 objects, not just specific fields

### Proposed Architecture

We will implement a **fresh data validation** pattern for cancel/delete operations:

1. **Frontend reloads from S3 before cancel/delete** - Makes GET request to retrieve and validate latest S3 version
2. **Frontend validates operation** - Checks status, confirms with user using fresh data
3. **Frontend submits operation** - Sends cancel/delete request (Lambda will load from S3)
4. **Lambda loads from S3** - Always reads from S3 as single source of truth for meeting cancellation
5. **Comprehensive logging** - Track data flow at every step to diagnose issues

**Data Flow After Fix:**
1. User clicks Cancel → Frontend GET `/changes/{id}` → Receives fresh S3 data → Validates status
2. Frontend POST `/changes/{id}/cancel` → Lambda loads from S3 → Finds `meeting_id` → Cancels meeting
3. Lambda writes to customer buckets → Backend processes → Meeting cancelled successfully

**Why Lambda loads from S3 (not request body):**
- **Single source of truth** - S3 is authoritative, no ambiguity
- **Simpler code** - No request body parsing needed
- **Idempotent** - Multiple requests get same S3 data
- **No race condition** - Frontend reload ensures S3 has data before Lambda runs

## Components and Interfaces

### Component 1: Status Change Functions (my-changes.html)

**Location:** `html/my-changes.html` - MyChanges class

**Modified Functions:**
- `approveChange(changeId)` - Remove (meetings not scheduled before approval)
- `completeChange(changeId)` - Preserve meeting metadata
- `cancelChange(changeId)` - Preserve meeting metadata  
- `deleteChange(changeId)` - Preserve meeting metadata

**Implementation Pattern for Cancel:**

```javascript
async cancelChange(changeId) {
    if (!confirm('Are you sure you want to cancel this change?')) {
        return;
    }

    try {
        // CRITICAL: Reload the change from S3 to get latest backend updates
        console.log('🔄 Reloading change from S3 before cancellation:', changeId);
        
        let freshChange;
        try {
            const response = await fetch(`${window.location.origin}/changes/${changeId}`, {
                method: 'GET',
                credentials: 'same-origin'
            });
            
            if (!response.ok) {
                throw new Error(`Failed to reload change: ${response.statusText}`);
            }
            
            freshChange = await response.json();
            console.log('✅ Reloaded fresh change from S3');
            console.log('📋 Fresh change has meeting_id:', !!freshChange.meeting_id);
            console.log('📋 Fresh change has join_url:', !!freshChange.join_url);
        } catch (error) {
            console.error('❌ Failed to reload change from S3:', error);
            if (window.portal) {
                window.portal.showStatus('Failed to reload change data', 'error');
            }
            return;
        }

        // Prevent cancelling completed changes
        if (freshChange.status === 'completed') {
            if (window.portal) {
                window.portal.showStatus('Cannot cancel a completed change', 'error');
            }
            return;
        }

        // Update the change status to cancelled using the FRESH data
        const cancelledChange = {
            ...freshChange,  // Use fresh data from S3, not cached data
            status: 'cancelled',
            cancelledAt: new Date().toISOString(),
            cancelledBy: window.portal?.currentUser || 'Unknown',
            modifiedAt: new Date().toISOString(),
            modifiedBy: window.portal?.currentUser || 'Unknown'
        };
        
        console.log('📤 Sending cancellation with fresh data');
        console.log('📋 Sending meeting_id:', !!cancelledChange.meeting_id);
        console.log('📋 Sending join_url:', !!cancelledChange.join_url);

        // Send to API with fresh data
        const response = await fetch(`${window.location.origin}/changes/${changeId}/cancel`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify(cancelledChange)
        });
        
        // ... rest of implementation
    } catch (error) {
        console.error('Error cancelling change:', error);
        if (window.portal) {
            window.portal.showStatus('Failed to cancel change', 'error');
        }
    }
}
```

**Key Design Decision:** Always reload from S3 before cancel/delete operations to ensure we have the latest backend updates, including meeting metadata.

### Component 2: Delete Change Function

**Location:** `html/my-changes.html` - MyChanges class

**Current Implementation:**
```javascript
async deleteChange(changeId) {
    const result = await window.changeLifecycle.deleteChange(changeId);
    // ...
}
```

**Issue:** The `changeLifecycle.deleteChange()` method in `shared.js` only passes the `changeId`, not the full change object. The backend needs the full object to cancel meetings.

**Modified Implementation:**
```javascript
async deleteChange(changeId) {
    if (!confirm('Are you sure you want to delete this change? This will move it to the deleted folder.')) {
        return;
    }

    try {
        // CRITICAL: Reload the change from S3 to get latest backend updates
        console.log('🔄 Reloading change from S3 before deletion:', changeId);
        
        let freshChange;
        try {
            const response = await fetch(`${window.location.origin}/changes/${changeId}`, {
                method: 'GET',
                credentials: 'same-origin'
            });
            
            if (!response.ok) {
                throw new Error(`Failed to reload change: ${response.statusText}`);
            }
            
            freshChange = await response.json();
            console.log('✅ Reloaded fresh change from S3');
            console.log('📋 Fresh change has meeting_id:', !!freshChange.meeting_id);
            console.log('📋 Fresh change has join_url:', !!freshChange.join_url);
        } catch (error) {
            console.error('❌ Failed to reload change from S3:', error);
            if (window.portal) {
                window.portal.showStatus('Failed to reload change data', 'error');
            }
            return;
        }

        // Send the fresh change object to the DELETE endpoint
        console.log('📤 Sending deletion with fresh data');
        const response = await fetch(`${window.portal.baseUrl}/changes/${changeId}`, {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify(freshChange)  // Send fresh S3 data
        });

        if (!response.ok && response.status !== 404) {
            throw new Error(`Failed to delete change: ${response.statusText}`);
        }

        if (window.portal) {
            window.portal.showStatus('Change moved to deleted folder successfully', 'success');
        }
        await this.loadAllChanges();
    } catch (error) {
        console.error('Error deleting change:', error);
        if (window.portal) {
            window.portal.showStatus('Failed to delete change', 'error');
        }
    }
}
```

### Component 3: Duplicate Change Function

**Location:** `html/my-changes.html` - MyChanges class

**Current Implementation:** Already correctly excludes `meeting_id` and `join_url` by not including them in the duplicated object.

**Verification:** The current duplicate function creates a new object with explicit field assignments and does NOT include `meeting_id` or `join_url`. This is correct behavior.

**No changes needed** - but we'll add a comment to document this intentional exclusion.

### Component 4: Upload Lambda DELETE Handler

**Location:** `lambda/upload_lambda/upload-metadata-lambda.js` - `handleDeleteChange()`

**Current Implementation:** The Node.js upload Lambda already loads from S3 using `changeId` from URL path.

**Modified Implementation:** Add logging to verify meeting metadata is present when loaded from S3. No other changes needed - Lambda already loads from S3 as single source of truth.

**Key Point:** The frontend reload ensures:
1. User sees latest status before confirming delete
2. S3 has the latest data (including meeting metadata) before Lambda runs
3. Lambda can reliably load from S3 and find meeting metadata

```javascript
async function handleDeleteChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').pop();
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `archive/${changeId}.json`;

    try {
        // Load change from S3 (single source of truth)
        const data = await s3.getObject({
            Bucket: bucketName,
            Key: key
        }).promise();

        const change = JSON.parse(data.Body.toString());
        console.log('📋 Loaded change from S3 for deletion');
        console.log('📋 Change has meeting_id:', !!change.meeting_id);
        console.log('📋 Change has join_url:', !!change.join_url);

        // Verify user owns the change
        if (change.createdBy !== userEmail && change.submittedBy !== userEmail) {
            return {
                statusCode: 403,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Access denied to delete this change' })
            };
        }

        // Move the change to deleted folder (change object includes meeting metadata)
        const deletedKey = `deleted/archive/${changeId}.json`;
        
        // ... rest of implementation uses 'change' object which has meeting metadata
    } catch (error) {
        if (error.code === 'NoSuchKey') {
            return {
                statusCode: 404,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Change not found' })
            };
        }
        throw error;
    }
}
```

### Component 5: Upload Lambda CANCEL Handler

**Location:** `lambda/upload_lambda/upload-metadata-lambda.js` - `handleCancelChange()`

**Current Implementation:** The Node.js upload Lambda already loads from S3.

**Modified Implementation:** Add logging to verify meeting metadata is present when loaded from S3. No other changes needed - Lambda already loads from S3 as single source of truth.

```javascript
async function handleCancelChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').filter(p => p && p !== 'cancel').pop();
    
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const archiveKey = `archive/${changeId}.json`;

    try {
        // Load change from S3 (single source of truth)
        const getParams = {
            Bucket: bucketName,
            Key: archiveKey
        };
        const data = await s3.getObject(getParams).promise();
        const existingChange = JSON.parse(data.Body.toString());
        
        console.log('📋 Loaded change from S3 for cancellation');
        console.log('📋 Change has meeting_id:', !!existingChange.meeting_id);
        console.log('📋 Change has join_url:', !!existingChange.join_url);
        
        // ... rest of implementation uses 'existingChange' which has meeting metadata
    } catch (error) {
        if (error.code === 'NoSuchKey') {
            return {
                statusCode: 404,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Change not found' })
            };
        }
        throw error;
    }
}
```

## Data Models

### Change Object Schema

The complete change object schema includes:

**Core Fields:**
- `changeId` (string)
- `version` (int)
- `status` (string)
- `changeTitle` (string)
- `customers` (array)
- `snowTicket` (string)
- `jiraTicket` (string)
- `changeReason` (string)
- `implementationPlan` (string)
- `testPlan` (string)
- `customerImpact` (string)
- `rollbackPlan` (string)

**Schedule Fields:**
- `implementationStart` (RFC3339 string)
- `implementationEnd` (RFC3339 string)
- `implementationBeginDate` (string)
- `implementationBeginTime` (string)
- `implementationEndDate` (string)
- `implementationEndTime` (string)
- `timezone` (string)

**Meeting Fields:**
- `meetingRequired` (string: 'yes' or 'no')
- `meetingTitle` (string)
- `meetingDate` (string)
- `meetingDuration` (string)
- `meetingLocation` (string)
- `meeting_id` (string) - **CRITICAL: Must be preserved**
- `join_url` (string) - **CRITICAL: Must be preserved**

**Metadata Fields:**
- `createdAt` (ISO8601 string)
- `createdBy` (string)
- `modifiedAt` (ISO8601 string)
- `modifiedBy` (string)
- `submittedAt` (ISO8601 string)
- `submittedBy` (string)
- `approvedAt` (ISO8601 string)
- `approvedBy` (string)
- `completedAt` (ISO8601 string)
- `completedBy` (string)
- `cancelledAt` (ISO8601 string)
- `cancelledBy` (string)
- `modifications` (array)

## Error Handling

### Frontend Error Handling

1. **Missing Change Object:** If change is not found in `this.allChanges`, display error and return early
2. **API Failure:** If API request fails, log error and display user-friendly message
3. **Network Errors:** Catch network errors and display appropriate message

### Backend Error Handling

1. **Missing Request Body:** If DELETE request body is missing or invalid, create minimal change object with just `changeId`
2. **Meeting Cancellation Failure:** Log warning but don't fail the delete operation
3. **DynamoDB Errors:** Return appropriate HTTP status codes

## Testing Strategy

### Manual Testing

1. **Test Complete with Meeting:**
   - Create change with meeting required
   - Submit and approve change (meeting gets scheduled)
   - Complete the change
   - Verify meeting metadata is preserved in DynamoDB
   - Verify no errors in browser console

2. **Test Cancel with Meeting:**
   - Create change with meeting required
   - Submit and approve change (meeting gets scheduled)
   - Cancel the change
   - Verify meeting is cancelled in Microsoft Graph
   - Verify meeting metadata is preserved in DynamoDB

3. **Test Delete with Meeting:**
   - Create change with meeting required
   - Submit and approve change (meeting gets scheduled)
   - Delete the change
   - Verify meeting is cancelled in Microsoft Graph
   - Verify change is moved to deleted folder

4. **Test Duplicate:**
   - Create change with meeting required
   - Submit and approve change (meeting gets scheduled)
   - Duplicate the change
   - Verify duplicated change does NOT have `meeting_id` or `join_url`
   - Verify duplicated change has all other meeting fields

### Console Logging

Add debug logging to verify field preservation:

```javascript
console.log('Cancelling change:', changeId);
console.log('Change object has meeting_id:', !!change.meeting_id);
console.log('Change object has join_url:', !!change.join_url);
console.log('Sending to API:', JSON.stringify(cancelledChange, null, 2));
```

### Backend Logging

Add logging in Go backend to verify received data:

```go
slog.Info("DELETE change request received",
    "changeId", change.ChangeID,
    "hasMeetingId", change.MeetingID != "",
    "hasJoinUrl", change.JoinURL != "")
```

## Implementation Notes

### Why Spread Operator Should Work

The JavaScript spread operator (`...change`) creates a shallow copy of all enumerable properties. As long as:
1. The change object loaded from the API contains `meeting_id` and `join_url`
2. These fields are enumerable properties (not getters/setters)
3. The object is not being transformed by any middleware

Then the spread operator WILL preserve these fields.

### Root Cause Analysis

The likely root cause is NOT the spread operator itself, but rather:
1. The DELETE operation not sending the full change object to the backend
2. The backend DELETE handler not reading the request body
3. Possible race condition where meeting is scheduled but not yet written to DynamoDB when delete occurs

### Defense in Depth

This design implements multiple layers of protection:
1. Frontend explicitly sends full change object
2. Backend reads request body to get full change object
3. Backend falls back to minimal object if body parsing fails
4. Logging at both frontend and backend to diagnose issues
