# Meeting Cancellation Troubleshooting

## Issue
The cancel event sent the cancellation email successfully, but did not cancel the Microsoft Graph meeting.

## Enhanced Logging Added

### 1. CancelScheduledMeetingIfNeeded Function

Added detailed diagnostic logging to help identify why meetings aren't being cancelled:

```go
log.Printf("🔍 Checking if change %s has a scheduled meeting to cancel", metadata.ChangeID)
log.Printf("📊 Metadata has %d modification entries", len(metadata.Modifications))

// Debug: Log all modification types
if len(metadata.Modifications) > 0 {
    log.Printf("📋 Modification types in metadata:")
    for i, mod := range metadata.Modifications {
        log.Printf("  %d. Type: %s, Timestamp: %s", i+1, mod.ModificationType, mod.Timestamp.Format("2006-01-02 15:04:05"))
        if mod.ModificationType == types.ModificationTypeMeetingScheduled && mod.MeetingMetadata != nil {
            log.Printf("     Meeting ID: %s, Join URL: %s", mod.MeetingMetadata.MeetingID, mod.MeetingMetadata.JoinURL)
        }
    }
}
```

**What to look for in logs:**
- Number of modification entries
- Whether any `meeting_scheduled` entries exist
- The meeting ID and join URL if found
- Warning if no meeting found with explanation of possible causes

### 2. cancelGraphMeeting Function

Added comprehensive logging for the Graph API call:

```go
log.Printf("🗑️  Attempting to cancel Graph meeting: ID=%s, Organizer=%s", meetingID, organizerEmail)

// Validate inputs
if meetingID == "" {
    return fmt.Errorf("meeting ID cannot be empty")
}
if organizerEmail == "" {
    return fmt.Errorf("organizer email cannot be empty")
}

log.Printf("🔑 Getting Graph API access token...")
// ... get token ...
log.Printf("✅ Successfully obtained Graph API access token")

log.Printf("🌐 DELETE request URL: %s", url)
log.Printf("📤 Sending DELETE request to Microsoft Graph API...")
// ... make request ...
log.Printf("📥 Graph API response: Status=%d, Body=%s", resp.StatusCode, string(body))
```

**What to look for in logs:**
- Whether access token was obtained successfully
- The exact DELETE URL being called
- HTTP status code from Graph API
- Response body (especially for errors)

## Possible Root Causes

### 1. No Meeting Metadata in Modifications Array
**Symptom:** Log shows `⚠️  No scheduled meeting found for change`

**Possible causes:**
- Meeting was never scheduled (meetingRequired was "no")
- Meeting scheduling failed earlier
- Modifications array not being loaded from S3
- Meeting metadata not properly stored in modifications array

**What to check:**
- Look for earlier logs showing meeting scheduling
- Check S3 object to see if modifications array contains meeting_scheduled entry
- Verify `GetLatestMeetingMetadata()` is working correctly

### 2. Graph API Authentication Failure
**Symptom:** Log shows `❌ Failed to get Graph access token`

**Possible causes:**
- Azure credentials not loaded from Parameter Store
- Invalid client ID, secret, or tenant ID
- Permissions issue with Parameter Store

**What to check:**
- Look for logs about loading Azure credentials
- Verify Parameter Store has correct values
- Check Lambda has permissions to read from Parameter Store

### 3. Graph API Permission Issues
**Symptom:** Log shows HTTP 403 or 401 status code

**Possible causes:**
- Service principal doesn't have Calendars.ReadWrite permission
- Meeting belongs to different organizer
- Token expired or invalid

**What to check:**
- Response body will contain error details
- Verify Azure AD app registration has correct permissions
- Check organizer email matches the one used to create meeting

### 4. Meeting Already Deleted
**Symptom:** Log shows HTTP 404 status code

**This is actually OK** - the function treats 404 as success since the meeting is gone either way.

### 5. Wrong Meeting ID
**Symptom:** Log shows HTTP 404 but meeting still exists in calendar

**Possible causes:**
- Meeting ID stored in S3 doesn't match actual Graph meeting ID
- Meeting was created with different organizer
- Meeting ID format is incorrect

**What to check:**
- Compare meeting ID in logs with actual meeting ID in Outlook/Teams
- Verify organizer email is correct

### 6. Network/Timeout Issues
**Symptom:** Log shows `❌ HTTP request failed`

**Possible causes:**
- Network connectivity issues
- Graph API endpoint unreachable
- Request timeout (30 seconds)

**What to check:**
- Network connectivity from Lambda
- Graph API service status
- Lambda VPC configuration if applicable

## Diagnostic Checklist

When troubleshooting a failed meeting cancellation, check these logs in order:

1. ✅ **Email sent successfully?**
   - Look for: `✅ Change cancelled notification email sent to X members`
   - If NO: Email sending failed, separate issue

2. ✅ **Cancellation function called?**
   - Look for: `🔍 Checking if change CHG-XXX has a scheduled meeting to cancel`
   - If NO: Event type not recognized or switch case not reached

3. ✅ **Modifications array populated?**
   - Look for: `📊 Metadata has X modification entries`
   - If 0: Modifications not loaded from S3

4. ✅ **Meeting metadata found?**
   - Look for: `📅 Found scheduled meeting for change CHG-XXX: ID=...`
   - If NO: Meeting was never scheduled or not in modifications array

5. ✅ **Access token obtained?**
   - Look for: `✅ Successfully obtained Graph API access token`
   - If NO: Azure credentials issue

6. ✅ **DELETE request sent?**
   - Look for: `📤 Sending DELETE request to Microsoft Graph API...`
   - Look for: `🌐 DELETE request URL: https://graph.microsoft.com/...`

7. ✅ **Graph API response?**
   - Look for: `📥 Graph API response: Status=XXX, Body=...`
   - Status 204: Success
   - Status 404: Already deleted (OK)
   - Status 401/403: Permission issue
   - Other: Check response body for details

## Example Log Patterns

### Successful Cancellation:
```
🔍 Checking if change CHG-123 has a scheduled meeting to cancel
📊 Metadata has 3 modification entries
📋 Modification types in metadata:
  1. Type: created, Timestamp: 2025-01-15 10:00:00
  2. Type: approved, Timestamp: 2025-01-15 11:00:00
  3. Type: meeting_scheduled, Timestamp: 2025-01-15 11:05:00
     Meeting ID: AAMkAGVm..., Join URL: https://teams.microsoft.com/...
📅 Found scheduled meeting for change CHG-123: ID=AAMkAGVm..., JoinURL=https://teams.microsoft.com/...
🗑️  Attempting to cancel Graph meeting: ID=AAMkAGVm..., Organizer=ccoe@hearst.com
🔑 Getting Graph API access token...
✅ Successfully obtained Graph API access token
🌐 DELETE request URL: https://graph.microsoft.com/v1.0/users/ccoe@hearst.com/events/AAMkAGVm...
📤 Sending DELETE request to Microsoft Graph API...
📥 Graph API response: Status=204, Body=
✅ Successfully deleted Graph meeting AAMkAGVm... (HTTP 204)
✅ Updated S3 object with meeting cancelled entry
```

### No Meeting to Cancel:
```
🔍 Checking if change CHG-456 has a scheduled meeting to cancel
📊 Metadata has 2 modification entries
📋 Modification types in metadata:
  1. Type: created, Timestamp: 2025-01-15 10:00:00
  2. Type: submitted, Timestamp: 2025-01-15 10:30:00
⚠️  No scheduled meeting found for change CHG-456, nothing to cancel
📊 This could mean: 1) No meeting was ever scheduled, 2) Meeting metadata not in modifications array, 3) Modifications array is empty
```

### Permission Error:
```
🔍 Checking if change CHG-789 has a scheduled meeting to cancel
📊 Metadata has 3 modification entries
📅 Found scheduled meeting for change CHG-789: ID=AAMkAGVm...
🗑️  Attempting to cancel Graph meeting: ID=AAMkAGVm..., Organizer=ccoe@hearst.com
🔑 Getting Graph API access token...
✅ Successfully obtained Graph API access token
🌐 DELETE request URL: https://graph.microsoft.com/v1.0/users/ccoe@hearst.com/events/AAMkAGVm...
📤 Sending DELETE request to Microsoft Graph API...
📥 Graph API response: Status=403, Body={"error":{"code":"ErrorAccessDenied","message":"Access is denied"}}
❌ Failed to cancel Graph meeting AAMkAGVm...: failed to delete meeting (status 403): {"error":{"code":"ErrorAccessDenied","message":"Access is denied"}}
```

## Next Steps

With these enhanced logs, you should be able to identify exactly where the meeting cancellation is failing. Look for the specific log patterns above in CloudWatch to diagnose the issue.
