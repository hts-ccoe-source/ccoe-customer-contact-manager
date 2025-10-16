# Duplicate and Clone Meeting Verification

## Summary
✅ **VERIFIED**: Both the "Duplicate" button and "Clone existing change" functionality correctly exclude `meeting_id` and `join_url` from copied changes.

## Analysis

### Data Structure
Meeting IDs and join URLs are stored in the `modifications` array within the `MeetingMetadata` structure:

```json
{
  "changeId": "CHG-123",
  "changeTitle": "Example Change",
  "modifications": [
    {
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGVmMDEzMTM4...",
        "join_url": "https://teams.microsoft.com/l/meetup-join/...",
        "start_time": "2025-01-20T14:00:00Z",
        "end_time": "2025-01-20T15:00:00Z",
        "subject": "Change Implementation: ..."
      }
    }
  ]
}
```

### Duplicate Function (my-changes.html)

**Location:** `html/my-changes.html` lines 1205-1275

**What it copies:**
- ✅ Basic change information (title, reason, plans)
- ✅ Schedule information (dates, times, timezone)
- ✅ Tickets (SNOW, JIRA)
- ✅ Customer selection
- ✅ Meeting settings (meetingRequired, meetingTitle, meetingDate, meetingDuration, meetingLocation, attendees)

**What it does NOT copy:**
- ❌ `modifications` array (which contains meeting_id and join_url)
- ❌ Status (resets to 'draft')
- ❌ Change ID (generates new ID)
- ❌ Audit timestamps (creates new timestamps)

```javascript
const duplicated = {
    changeId: newChangeId,  // NEW ID
    version: 1,
    status: 'draft',  // RESET STATUS
    createdAt: new Date().toISOString(),  // NEW TIMESTAMP
    modifiedAt: new Date().toISOString(),
    createdBy: window.portal.currentUser,
    modifiedBy: window.portal.currentUser,

    // Copy fields
    changeTitle: change.changeTitle || '',
    snowTicket: change.snowTicket || '',
    // ... other fields ...
    
    // Meeting details (form fields only, NOT meeting_id/join_url)
    meetingRequired: change.meetingRequired || 'no',
    meetingTitle: change.meetingTitle || '',
    meetingDate: change.meetingDate || '',
    meetingDuration: change.meetingDuration || '',
    meetingLocation: change.meetingLocation || '',
    attendees: change.attendees || ''
    
    // NOTE: modifications array is NOT copied
};
```

### Clone Function (create-change.html)

**Location:** `html/create-change.html` lines 1440-1510

**What it copies:**
- ✅ Basic change information (title with "Copy of" prefix, reason, plans)
- ✅ Schedule information (dates, times, timezone)
- ✅ Tickets (SNOW, JIRA)
- ✅ Customer selection

**What it does NOT copy:**
- ❌ `modifications` array (which contains meeting_id and join_url)
- ❌ Meeting settings (meetingRequired, meetingTitle, etc.)
- ❌ Change ID (generates new ID via `generateNewId()`)
- ❌ Status (form starts fresh)

```javascript
function populateFormFromChange(changeData) {
    // Populate basic fields
    document.getElementById('changeTitle').value = `Copy of ${changeData.changeTitle || ''}`;
    document.getElementById('changeReason').value = changeData.changeReason || '';
    document.getElementById('implementationPlan').value = changeData.implementationPlan || '';
    document.getElementById('testPlan').value = changeData.testPlan || '';
    document.getElementById('customerImpact').value = changeData.customerImpact || '';
    document.getElementById('rollbackPlan').value = changeData.rollbackPlan || '';

    // Populate customer checkboxes
    const customers = changeData.customers || [];
    customers.forEach(customerCode => {
        const checkbox = document.getElementById(`customer-${customerCode}`);
        if (checkbox) {
            checkbox.checked = true;
        }
    });

    // Populate schedule fields
    document.getElementById('implementationBeginDate').value = changeData.implementationBeginDate || '';
    document.getElementById('implementationBeginTime').value = changeData.implementationBeginTime || '';
    document.getElementById('implementationEndDate').value = changeData.implementationEndDate || '';
    document.getElementById('implementationEndTime').value = changeData.implementationEndTime || '';
    document.getElementById('timezone').value = changeData.timezone || '';

    // Populate tickets
    document.getElementById('jiraTicket').value = changeData.jiraTicket || '';
    document.getElementById('snowTicket').value = changeData.snowTicket || '';
    
    // NOTE: Meeting fields are NOT populated
    // NOTE: modifications array is NOT copied
}
```

## Why This Works Correctly

1. **Meeting ID and Join URL Location**: These are stored exclusively in the `modifications` array within `MeetingMetadata` objects
2. **No Top-Level Fields**: There are no top-level `meetingId` or `joinUrl` fields in the `ChangeMetadata` structure
3. **Modifications Array Not Copied**: Neither function copies the `modifications` array
4. **Meeting Settings vs Meeting Metadata**: 
   - Meeting settings (meetingRequired, meetingTitle, etc.) are user-configurable form fields
   - Meeting metadata (meeting_id, join_url) are system-generated values from Microsoft Graph API
   - Only the settings are copied, not the metadata

## Expected Behavior

### When Duplicating a Change with a Scheduled Meeting:

**Original Change:**
```json
{
  "changeId": "CHG-001",
  "changeTitle": "Database Migration",
  "meetingRequired": "yes",
  "meetingTitle": "Migration Planning",
  "modifications": [
    {
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGVm...",
        "join_url": "https://teams.microsoft.com/l/meetup-join/abc123"
      }
    }
  ]
}
```

**Duplicated Change:**
```json
{
  "changeId": "CHG-002",  // NEW ID
  "changeTitle": "Database Migration",  // Same title
  "meetingRequired": "yes",  // Copied
  "meetingTitle": "Migration Planning",  // Copied
  "modifications": []  // EMPTY - no meeting scheduled yet
}
```

### When Cloning a Change:

**Original Change:**
```json
{
  "changeId": "CHG-001",
  "changeTitle": "Database Migration",
  "meetingRequired": "yes",
  "modifications": [
    {
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGVm...",
        "join_url": "https://teams.microsoft.com/l/meetup-join/abc123"
      }
    }
  ]
}
```

**Cloned Change (Form State):**
```json
{
  "changeId": "CHG-003",  // NEW ID
  "changeTitle": "Copy of Database Migration",  // Prefixed
  "meetingRequired": "",  // NOT copied - form field empty
  "modifications": []  // EMPTY - no meeting scheduled yet
}
```

## Conclusion

✅ **Both duplicate and clone functions work correctly:**
- They copy user-configurable meeting settings (title, duration, location)
- They do NOT copy system-generated meeting metadata (meeting_id, join_url)
- When the duplicated/cloned change is approved and requires a meeting, a NEW meeting will be created with a NEW meeting_id and join_url

This is the correct behavior because:
1. Each change should have its own unique meeting
2. Meeting IDs and join URLs are tied to specific calendar events
3. Reusing meeting IDs would cause conflicts and confusion
4. Users can still benefit from having the meeting settings pre-filled (title, duration, etc.)
