# Meeting Idempotency Implementation

## Overview

This document describes the idempotency implementation for Microsoft Graph API meeting creation, ensuring that duplicate meeting creation requests are handled gracefully and don't result in duplicate calendar events.

## Key Features

### 1. Native iCalUId-Based Idempotency

Instead of using custom markers or workarounds, we use the **standard iCalendar UID (`iCalUId`)** property that Microsoft Graph API natively supports:

```go
// Generate iCalUId from objectID
objectID := extractObjectID(metadata)  // e.g., "change-123-abc"
iCalUID := fmt.Sprintf("%s@ccoe-customer-contact-manager", objectID)
// Result: "change-123-abc@ccoe-customer-contact-manager"
```

**Benefits:**
- Standard iCalendar RFC compliance
- Native Graph API filtering support
- Server-side duplicate prevention
- Works across calendar systems

### 2. ObjectID Extraction

The `extractObjectID()` function works for both changes and announcements:

```go
// For changes: uses ChangeID
metadata := &types.ChangeMetadata{
    ChangeID: "change-123-abc",
}
objectID := extractObjectID(metadata)  // Returns: "change-123-abc"

// For announcements: uses announcement_id from Metadata map
metadata := &types.ChangeMetadata{
    ObjectType: "announcement_cic",
    Metadata: map[string]interface{}{
        "announcement_id": "announcement-456-def",
    },
}
objectID := extractObjectID(metadata)  // Returns: "announcement-456-def"
```

### 3. Duplicate Detection

Before creating a meeting, we check if one already exists with the same iCalUId:

```go
func checkMeetingExistsByObjectID(accessToken, organizerEmail, objectID string) (bool, *types.GraphMeetingResponse, error) {
    iCalUID := fmt.Sprintf("%s@ccoe-customer-contact-manager", objectID)
    
    // Use Graph API native filtering
    filterQuery := fmt.Sprintf("iCalUId eq '%s'", iCalUID)
    apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$filter=%s", 
        organizerEmail, url.QueryEscape(filterQuery))
    
    // If found, return existing meeting
    // If not found, proceed with creation
}
```

### 4. Retry Logic for Transient Failures

The `createGraphMeetingWithRetry()` function handles transient Graph API failures:

```go
func createGraphMeetingWithRetry(accessToken, organizerEmail, payload string, maxRetries int) (string, error) {
    for attempt := 1; attempt <= maxRetries; attempt++ {
        // Exponential backoff: 2^(attempt-1) seconds
        if attempt > 1 {
            backoffDuration := time.Duration(1<<uint(attempt-1)) * time.Second
            time.Sleep(backoffDuration)
        }
        
        // Attempt to create meeting
        // Retry on: 5xx errors, 429 rate limiting
        // Don't retry on: 4xx client errors (except 429)
    }
}
```

**Retry Strategy:**
- **Attempt 1**: Immediate
- **Attempt 2**: 2 second backoff
- **Attempt 3**: 4 second backoff
- **Transient errors**: 5xx, 429 (rate limiting)
- **Non-transient errors**: 4xx (except 429) - fail immediately

### 5. Immutable ID Header

All Graph API requests include the `Prefer: IdType="ImmutableId"` header:

```go
func setGraphAPIHeaders(req *http.Request, accessToken string, contentType string) {
    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("Prefer", "IdType=\"ImmutableId\"")  // Request stable IDs
    if contentType != "" {
        req.Header.Set("Content-Type", contentType)
    }
}
```

**Why this matters:**
- Ensures we get stable, immutable IDs from Microsoft Graph
- IDs won't change even if the meeting is moved or updated
- Critical for reliable idempotency checking

## Workflow

### Meeting Creation Flow

```
1. Extract objectID from metadata
   ├─ For changes: use ChangeID
   └─ For announcements: use announcement_id

2. Generate iCalUId
   └─ Format: "{objectID}@ccoe-customer-contact-manager"

3. Check if meeting exists (unless force-update)
   ├─ Query Graph API: $filter=iCalUId eq '{iCalUId}'
   ├─ If found: Return existing meeting ID
   └─ If not found: Continue to step 4

4. Create meeting with retry logic
   ├─ Include iCalUId in meeting payload
   ├─ Set Prefer: IdType="ImmutableId" header
   ├─ Retry on transient failures (5xx, 429)
   └─ Return meeting ID on success

5. Store meeting metadata in archive
   └─ (Handled by separate task 30)
```

### Duplicate Request Handling

```
Request 1 (Initial):
  objectID: "change-123-abc"
  iCalUId: "change-123-abc@ccoe-customer-contact-manager"
  → Check: No existing meeting found
  → Create: New meeting created with ID "AAMkAGI..."
  → Result: Meeting ID returned

Request 2 (Duplicate):
  objectID: "change-123-abc"  (same)
  iCalUId: "change-123-abc@ccoe-customer-contact-manager"  (same)
  → Check: Existing meeting found with ID "AAMkAGI..."
  → Skip: No new meeting created
  → Result: Existing meeting ID returned
```

## API Integration

### Meeting Payload Structure

**Change Meeting (attendees visible):**
```json
{
  "subject": "Change Implementation: Configure Proof-of-Value exercise",
  "iCalUId": "change-123-abc@ccoe-customer-contact-manager",
  "body": {
    "contentType": "HTML",
    "content": "<h2>🔄 Change Implementation Meeting</h2>..."
  },
  "start": {
    "dateTime": "2025-09-20T10:00:00.0000000",
    "timeZone": "America/New_York"
  },
  "end": {
    "dateTime": "2025-09-20T17:00:00.0000000",
    "timeZone": "America/New_York"
  },
  "isOnlineMeeting": true,
  "onlineMeetingProvider": "teamsForBusiness",
  "hideAttendees": false,
  "attendees": [...]
}
```

**Announcement Meeting (attendees hidden):**
```json
{
  "subject": "CIC Event: Cloud Innovator Community Monthly Meeting",
  "iCalUId": "announcement-cic-456@ccoe-customer-contact-manager",
  "body": {
    "contentType": "HTML",
    "content": "<h2>📢 CIC Announcement Meeting</h2>..."
  },
  "start": {
    "dateTime": "2025-09-20T14:00:00.0000000",
    "timeZone": "America/New_York"
  },
  "end": {
    "dateTime": "2025-09-20T15:00:00.0000000",
    "timeZone": "America/New_York"
  },
  "isOnlineMeeting": true,
  "onlineMeetingProvider": "teamsForBusiness",
  "hideAttendees": true,
  "attendees": [...]
}
```

### Graph API Endpoints Used

1. **Create Meeting**: `POST /users/{organizer}/events`
2. **Query by iCalUId**: `GET /users/{organizer}/events?$filter=iCalUId eq '{uid}'`
3. **Get Meeting Details**: `GET /users/{organizer}/events/{id}`
4. **Update Meeting**: `PATCH /users/{organizer}/events/{id}`

## Testing

### Unit Tests

```bash
# Run all idempotency tests
go test -v ./internal/ses -run "Idempotency|ObjectID|ManualAttendees|ICalUID"
```

**Test Coverage:**
- `TestExtractObjectID`: Validates objectID extraction for changes and announcements
- `TestExtractManualAttendees`: Tests manual attendee parsing from metadata
- `TestICalUIdGeneration`: Verifies correct iCalUId format generation
- `TestIdempotencyWithDuplicateRequests`: Ensures consistent objectID extraction

### Integration Testing

For full integration testing with Microsoft Graph API:
1. Mock HTTP client responses
2. Test retry logic with simulated failures
3. Verify idempotency across multiple requests
4. Test with both changes and announcements

## Error Handling

### Transient Errors (Retry)
- **500-504**: Server errors
- **429**: Rate limiting
- **Network errors**: Connection failures, timeouts

### Non-Transient Errors (Fail Fast)
- **400**: Bad request (invalid payload)
- **401**: Unauthorized (invalid token)
- **403**: Forbidden (insufficient permissions)
- **404**: Not found (invalid organizer)

### Idempotency Errors
- **Missing objectID**: Fail with clear error message
- **Duplicate detection failure**: Log warning, proceed with creation
- **iCalUId conflict**: Return existing meeting ID

## Meeting Privacy Settings

### Hide Attendees for Announcements

Announcement meetings automatically set `hideAttendees = true` to provide privacy for broadcast-style events:

```go
// Determine if this is an announcement (hide attendees for broadcast-style events)
isAnnouncement := strings.HasPrefix(metadata.ObjectType, "announcement")

meeting := map[string]interface{}{
    // ... other fields ...
    "hideAttendees": isAnnouncement,  // true for announcements, false for changes
}
```

**Why hide attendees for announcements?**
- Announcements are broadcast-style events (CIC, FinOps, InnerSource)
- Attendees don't need to see each other's names
- Reduces visual clutter in large meetings
- Provides privacy for attendees
- Change meetings keep attendees visible for collaboration

**Supported announcement types:**
- `announcement_cic` - Cloud Innovator Community
- `announcement_finops` - FinOps events
- `announcement_innersource` - InnerSource Guild
- Any object type starting with `announcement_`

## Best Practices

1. **Always use objectID**: Never rely on subject or other mutable fields
2. **Include Prefer header**: Always request immutable IDs
3. **Handle duplicates gracefully**: Return existing meeting ID, don't error
4. **Retry transient failures**: Use exponential backoff
5. **Log all operations**: Include objectID and iCalUId in logs
6. **Test with real data**: Verify with actual change and announcement metadata
7. **Use hideAttendees appropriately**: Set to true for broadcast events, false for collaborative meetings

## Future Enhancements

1. **Archive Integration**: Store meeting metadata in S3 archive after creation (Task 30)
2. **Meeting Updates**: Handle updates to existing meetings based on metadata changes
3. **Cancellation Support**: Implement meeting cancellation with idempotency
4. **Metrics**: Track duplicate detection rate and retry success rate
5. **Extended Properties**: Consider using Graph API extended properties for additional metadata

## References

- [Microsoft Graph Calendar API](https://learn.microsoft.com/en-us/graph/api/resources/calendar)
- [iCalendar RFC 5545](https://datatracker.ietf.org/doc/html/rfc5545)
- [Graph API Best Practices](https://learn.microsoft.com/en-us/graph/best-practices-concept)
- [Prefer Header Documentation](https://learn.microsoft.com/en-us/graph/api/overview?view=graph-rest-1.0#prefer-header)
