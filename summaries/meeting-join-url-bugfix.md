# Meeting Join URL Bugfix

## Problem
The system was failing to retrieve and store the `join_url` from Microsoft Graph API responses when scheduling meetings. The error message was:
```
failed to create meeting scheduled entry: invalid meeting metadata: join_url is required
```

## Root Cause
1. The `GraphMeetingResponse` struct in `internal/types/types.go` was missing the `OnlineMeeting` field that contains the `joinUrl` from the Microsoft Graph API
2. Graph API calls were not requesting the `onlineMeeting` field in the `$select` parameter
3. The conversion function was not extracting the join URL from the Graph response
4. The Lambda handler was trying to extract the join URL from the meeting body HTML content instead of using the structured `OnlineMeeting.JoinURL` field

## Changes Made

### 1. Updated GraphMeetingResponse Structure
**File:** `internal/types/types.go`

Added the `OnlineMeeting` field to capture the join URL from Microsoft Graph API:

```go
type GraphMeetingResponse struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Body    *struct {
		ContentType string `json:"contentType"`
		Content     string `json:"content"`
	} `json:"body,omitempty"`
	Start *struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"start,omitempty"`
	End *struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"end,omitempty"`
	OnlineMeeting *struct {
		JoinURL string `json:"joinUrl"`
	} `json:"onlineMeeting,omitempty"`  // NEW FIELD
}
```

### 2. Updated Graph API Calls to Request onlineMeeting Field
**File:** `internal/ses/meetings.go`

Updated three Graph API calls to include `onlineMeeting` in the `$select` parameter:

1. **checkMeetingExists** (line ~725):
   ```go
   url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$top=50&$select=id,subject,start,end,attendees,onlineMeeting&$orderby=start/dateTime desc",
       organizerEmail)
   ```

2. **getMeetingDetails** (line ~786):
   ```go
   url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events/%s?$select=id,subject,body,start,end,onlineMeeting", organizerEmail, meetingID)
   ```

3. **checkMeetingExistsFlat** (line ~2034):
   ```go
   url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$top=50&$select=id,subject,start,end,attendees,onlineMeeting&$orderby=start/dateTime desc",
       organizerEmail)
   ```

### 3. Updated Conversion Function to Extract Join URL
**File:** `internal/lambda/modifications.go`

Enhanced `CreateMeetingMetadataFromGraphResponse` to extract the join URL:

```go
// Extract join URL from online meeting info
if graphResponse.OnlineMeeting != nil && graphResponse.OnlineMeeting.JoinURL != "" {
    metadata.JoinURL = graphResponse.OnlineMeeting.JoinURL
    log.Printf("📎 Extracted join URL from Graph response")
} else {
    log.Printf("⚠️  No join URL found in Graph response")
}

log.Printf("✅ Created MeetingMetadata: ID=%s, Subject=%s, JoinURL=%s", metadata.MeetingID, metadata.Subject, metadata.JoinURL)
```

### 4. Updated Lambda Handler Join URL Extraction
**File:** `internal/lambda/handlers.go`

Fixed the `createGraphMeeting` function to extract join URL from the structured field instead of parsing HTML:

```go
// Extract join URL from online meeting info
joinURL := ""
if graphResponse.OnlineMeeting != nil && graphResponse.OnlineMeeting.JoinURL != "" {
    joinURL = graphResponse.OnlineMeeting.JoinURL
    log.Printf("✅ Extracted join URL from Graph response: %s", joinURL)
} else {
    // Fallback: try to extract from meeting body content
    if graphResponse.Body != nil && graphResponse.Body.Content != "" {
        joinURL = ses.ExtractTeamsJoinURL(graphResponse.Body.Content)
        if joinURL != "" {
            log.Printf("✅ Extracted join URL from meeting body content")
        }
    }
}
if joinURL == "" {
    joinURL = "https://teams.microsoft.com" // Fallback URL
    log.Printf("⚠️  Could not extract Teams join URL from Graph response")
}
```

## Data Flow

The complete flow for join URL retrieval and storage:

1. **Meeting Creation**: `ses.CreateMultiCustomerMeetingFromChangeMetadata()` creates meeting via Graph API
2. **Graph API Response**: Microsoft Graph returns meeting details including `onlineMeeting.joinUrl`
3. **Response Parsing**: `GraphMeetingResponse` struct captures the `OnlineMeeting.JoinURL` field
4. **Meeting Details Retrieval**: `ses.GetGraphMeetingDetails()` fetches full meeting details with join URL
5. **Metadata Conversion**: `CreateMeetingMetadataFromGraphResponse()` extracts join URL into `MeetingMetadata`
6. **Validation**: `MeetingMetadata.ValidateMeetingMetadata()` ensures join URL is present
7. **Modification Entry**: `NewMeetingScheduledEntry()` creates modification entry with meeting metadata
8. **S3 Update**: `UpdateChangeObjectWithMeetingMetadata()` stores the complete metadata back to S3

## Testing

To verify the fix:

1. Upload a change request with `meetingRequired: "yes"`
2. Check CloudWatch logs for:
   - `✅ Extracted join URL from Graph response: https://teams.microsoft.com/...`
   - `✅ Created MeetingMetadata: ID=..., Subject=..., JoinURL=https://teams.microsoft.com/...`
3. Verify S3 object contains meeting metadata with join_url:
   ```json
   {
     "modifications": [
       {
         "modification_type": "meeting_scheduled",
         "meeting_metadata": {
           "meeting_id": "AAMkA...",
           "join_url": "https://teams.microsoft.com/l/meetup-join/...",
           "start_time": "2025-01-20T14:00:00Z",
           "end_time": "2025-01-20T15:00:00Z",
           "subject": "Change Implementation: ..."
         }
       }
     ]
   }
   ```

## Related Files

- `internal/types/types.go` - Type definitions
- `internal/lambda/modifications.go` - Modification entry creation
- `internal/lambda/handlers.go` - Lambda handler and meeting scheduler
- `internal/ses/meetings.go` - Microsoft Graph API integration

## Validation

All validation rules remain intact:
- `join_url` is required in `MeetingMetadata`
- `meeting_id` is required
- `start_time` and `end_time` must be valid ISO 8601 timestamps
- `subject` is required

The fix ensures that the `join_url` is properly extracted from the Microsoft Graph API response and flows through the entire system to S3 storage.
