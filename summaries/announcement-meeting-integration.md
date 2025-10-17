# Announcement Microsoft Graph Meeting Integration

## Overview
Implemented full Microsoft Graph API integration for announcement meeting scheduling and cancellation, replacing the placeholder implementation.

## Changes Made

### File: `internal/processors/announcement_processor.go`

#### 1. Meeting Scheduling Implementation (`scheduleMeeting`)
**Previous**: Placeholder that logged intent but didn't create meetings
**Now**: Full implementation that:
- Converts announcement to ChangeMetadata format for compatibility with existing Graph API
- Retrieves attendees from SES topic subscriptions based on announcement type
- Generates Microsoft Graph meeting payload using existing infrastructure
- Creates Teams meeting via Graph API
- Updates announcement with meeting metadata (ID and join URL)
- Adds modification entry for meeting_scheduled
- Saves updated announcement back to S3

#### 2. Meeting Cancellation Implementation (`cancelMeeting`)
**Previous**: Placeholder that logged intent but didn't cancel meetings
**Now**: Full implementation that:
- Validates meeting metadata exists
- Calls Microsoft Graph API to delete the meeting
- Adds modification entry for meeting_cancelled
- Clears meeting metadata from announcement
- Saves updated announcement back to S3

#### 3. New Helper Functions

**`getAnnouncementAttendees`**
- Gets all attendees for an announcement from SES topic subscriptions
- Uses announcement type to determine the correct topic
- Returns list of email addresses

**`createGraphMeetingForAnnouncement`**
- Creates a Teams meeting using Microsoft Graph API
- Implements idempotency check to avoid duplicate meetings
- Returns meeting ID and join URL
- Handles Graph API authentication and error responses

**`checkAnnouncementMeetingExists`**
- Checks if a meeting for the announcement already exists
- Searches by announcement title to prevent duplicates
- Returns existing meeting if found

**`cancelGraphMeeting`**
- Cancels a meeting using Microsoft Graph API DELETE endpoint
- Handles authentication and error responses

## Integration Points

### Reused Existing Infrastructure
- `ses.GenerateGraphMeetingPayloadFromChangeMetadata()` - Generates Graph API payload
- `ses.GetGraphAccessToken()` - Obtains OAuth token for Graph API
- `ses.GetAccountContactList()` - Gets SES contact list
- `types.NewMeetingScheduledEntry()` - Creates modification entry
- `types.NewMeetingCancelledEntry()` - Creates cancellation entry

### Microsoft Graph API Endpoints Used
- `POST /users/{organizer}/events` - Create meeting
- `GET /users/{organizer}/events` - Search for existing meetings
- `DELETE /users/{organizer}/events/{meetingId}` - Cancel meeting

## Data Flow

### Meeting Creation Flow
1. Announcement approved with `include_meeting=true`
2. Convert announcement to ChangeMetadata format
3. Get attendees from SES topic subscriptions
4. Generate Graph API payload
5. Check for existing meeting (idempotency)
6. Create meeting via Graph API
7. Update announcement with meeting metadata
8. Add modification entry
9. Save to S3

### Meeting Cancellation Flow
1. Announcement cancelled
2. Check if meeting exists
3. Call Graph API to delete meeting
4. Add cancellation modification entry
5. Clear meeting metadata
6. Save to S3

## Meeting Metadata Structure
```go
MeetingMetadata {
    MeetingID string  // Graph API meeting ID
    JoinURL   string  // Teams meeting join URL
}
```

## Modification Entries
- `meeting_scheduled` - Added when meeting is created
- `meeting_cancelled` - Added when meeting is deleted

## Error Handling
- Meeting scheduling failures don't block announcement processing
- Warnings logged for non-critical errors
- Idempotency checks prevent duplicate meetings
- Graceful handling of missing meeting metadata

## Testing Recommendations
1. Test announcement approval with meeting creation
2. Test announcement cancellation with meeting deletion
3. Test idempotency (approving same announcement twice)
4. Test with different announcement types (CIC, FinOps, InnerSource)
5. Test with multiple customers
6. Test error scenarios (Graph API failures, missing credentials)

## Dependencies
- Microsoft Graph API credentials (from AWS Parameter Store)
- SES topic subscriptions for attendee lists
- Existing Graph API integration in `internal/ses/meetings.go`

## Notes
- Organizer email defaults to `ccoe@nonprod.ccoe.hearst.com`
- Meeting subject format: `{TYPE} Event: {Title}` (e.g., "CIC Event: New Feature Announcement")
- Attendees determined by SES topic subscriptions for announcement type
- Meetings are Teams online meetings with join URLs
