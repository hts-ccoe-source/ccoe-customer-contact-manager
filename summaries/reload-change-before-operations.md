# Critical Fix: Reload Change Before Operations

## Problem

Meeting metadata was being stripped when canceling, completing, or deleting changes. The meeting could not be cancelled because the `meeting_id` and `join_url` were missing from the change object sent to the backend.

## Root Cause

The meeting metadata is stored in the `modifications` array as a `meeting_scheduled` modification entry:

```json
{
  "modifications": [
    {
      "timestamp": "2025-10-11T20:48:06.305841204Z",
      "user_id": "arn:aws:iam::730335533660:role/...",
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGQyNGFlM2Y2...",
        "join_url": "https://teams.microsoft.com/l/meetup-join/...",
        "start_time": "2025-10-11T21:15:00Z",
        "end_time": "2025-10-11T21:30:00Z",
        "subject": "Change Implementation: ...",
        "organizer": "ccoe@hearst.com"
      }
    }
  ]
}
```

**The Issue:**
1. User loads the "My Changes" page
2. Frontend loads all changes and caches them in `this.allChanges`
3. User clicks "Approve" → Backend schedules meeting and adds `meeting_scheduled` modification
4. User clicks "Cancel" → Frontend uses CACHED change object (doesn't have meeting metadata)
5. Backend receives change without meeting metadata → Cannot cancel meeting

## The Fix

Before performing cancel/complete/delete operations, **reload the change from the server** to get the latest version with meeting metadata:

```javascript
async cancelChange(changeId) {
    // CRITICAL: Reload the change from server to get latest meeting metadata
    let change;
    try {
        const response = await fetch(`${window.location.origin}/changes/${changeId}`, {
            method: 'GET',
            credentials: 'same-origin'
        });
        
        if (response.ok) {
            change = await response.json();
            console.log('✅ Reloaded change from server with latest meeting metadata');
        } else {
            // Fallback to cached version if API fails
            change = this.allChanges.find(c => c.changeId === changeId);
            console.warn('⚠️  Failed to reload change from server, using cached version');
        }
    } catch (error) {
        // Fallback to cached version if API fails
        change = this.allChanges.find(c => c.changeId === changeId);
        console.warn('⚠️  Error reloading change from server, using cached version:', error);
    }
    
    // Now use the fresh change object with meeting metadata
    const cancelledChange = { ...change, status: 'cancelled', ... };
    
    // Send to backend
    await fetch(`${window.location.origin}/changes/${changeId}/cancel`, {
        method: 'POST',
        body: JSON.stringify(cancelledChange)
    });
}
```

## Why This Works

1. **Fresh Data**: Gets the latest change object from S3 via the backend API
2. **Meeting Metadata**: Includes the complete `modifications` array with `meeting_scheduled` entry
3. **Fallback**: If the API call fails, falls back to cached version (graceful degradation)
4. **Logging**: Logs whether meeting metadata was found for debugging

## Functions Updated

1. **cancelChange()** - Reloads change before canceling
2. **completeChange()** - Reloads change before completing
3. **deleteChange()** - Reloads change before deleting

## Backend Processing

The backend Lambda (`internal/lambda/handlers.go`) looks for meeting metadata in the `modifications` array:

```go
// Check for meeting metadata in modifications array
for _, mod := range metadata.Modifications {
    if mod.ModificationType == "meeting_scheduled" {
        if meetingMetadata, ok := mod.MeetingMetadata.(map[string]interface{}); ok {
            if meetingID, exists := meetingMetadata["meeting_id"]; exists {
                // Cancel the meeting using meeting_id
                err := CancelMicrosoftGraphMeeting(ctx, meetingID)
            }
        }
    }
}
```

## Testing

After this fix:

1. **Create and approve a change** with meeting required
2. **Wait for meeting to be scheduled** (check modifications array has `meeting_scheduled`)
3. **Click Cancel** button
4. **Check browser console** - should see:
   - "✅ Reloaded change from server with latest meeting metadata"
   - "✅ Found meeting_scheduled modification"
   - "Meeting ID: AAMkAGQyNGFlM2Y2..."
5. **Check backend logs** - should see meeting cancellation success

## Files Modified

- `html/my-changes.html` - Added reload logic to `cancelChange()`, `completeChange()`, and `deleteChange()`

## Related Issues

This fix addresses the core issue identified in the spec `.kiro/specs/frontend-meeting-metadata-preservation/` where meeting metadata was not being preserved during frontend operations.

The previous fix (sending full change object to backend) was correct, but incomplete - we also needed to ensure the frontend had the LATEST change object with meeting metadata before sending it.
