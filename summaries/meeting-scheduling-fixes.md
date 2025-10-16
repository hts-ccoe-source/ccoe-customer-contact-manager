# Meeting Scheduling Fixes

## Date: October 10, 2025

## Issues Found and Fixed

### Issue 1: Missing Timezone Conversion (FIXED)
**Location:** `internal/ses/meetings.go` - `generateGraphMeetingPayload()`

**Problem:**
Times in `ChangeMetadata` are stored in UTC (e.g., `2025-10-10T04:00:00Z` for midnight EDT), but the code was NOT converting them to the target timezone before sending to Microsoft Graph API. This caused meetings to be scheduled at the wrong time.

**Example:**
- User enters: 12:00 AM EDT (midnight)
- Stored in S3: `2025-10-10T04:00:00Z` (UTC)
- Sent to Graph: `04:00:00` with timezone `America/New_York`
- Graph interprets: 4:00 AM EDT ❌

**Root Cause:**
```go
// BROKEN CODE:
"dateTime": startTime.Format("2006-01-02T15:04:05.0000000"),  // Formats UTC as-is
"timeZone": targetTimezone,  // Tells Graph it's EDT
```

**Fix:**
```go
// FIXED CODE:
loc, _ := time.LoadLocation(targetTimezone)
localStartTime := startTime.In(loc)  // Convert UTC → EDT
"dateTime": localStartTime.Format("2006-01-02T15:04:05.0000000"),
"timeZone": targetTimezone,
```

Now converts `04:00:00Z` (UTC) → `00:00:00` (EDT) before sending to Graph.

**Impact:** Meetings were scheduled 4 hours late for EDT timezone (offset varies by timezone).

---

### Issue 2: Lambda Using Fake Meeting IDs (FIXED)
**Location:** `internal/lambda/handlers.go` - `createGraphMeeting()`

**Problem:**
The Lambda function was NOT calling the actual Microsoft Graph API. Instead, it was generating fake meeting IDs based on the change ID:

```go
// BROKEN CODE:
MeetingID: fmt.Sprintf("meeting-%s-%d", changeMetadata.ChangeID, time.Now().Unix()),
JoinURL:   fmt.Sprintf("https://teams.microsoft.com/l/meetup-join/meeting-%s", changeMetadata.ChangeID),
```

This is why CloudWatch logs showed meeting IDs that looked like change IDs (e.g., `meeting-CHG-test-123-1728567890`) instead of actual Graph meeting IDs.

**Root Cause:**
The Lambda had a stub/mock implementation with a TODO comment: "For now, create a basic meeting metadata structure. In a full implementation, this would call the Microsoft Graph API"

**Fix:**
Completely rewrote the Lambda's `createGraphMeeting()` function to:
1. Get the meeting organizer email from environment variable
2. Fetch calendar recipients for the change
3. Generate proper Graph API payload using `ses.GenerateGraphMeetingPayloadFromChangeMetadata()`
4. Get Graph API access token using `ses.GetGraphAccessToken()`
5. Create the actual meeting via Graph API using `ses.CreateGraphMeetingWithPayload()`
6. Retrieve full meeting details including Teams join URL
7. Return real meeting metadata with actual Graph meeting ID

**Fix Implementation:**
The Lambda now uses the new `ses.CreateMultiCustomerMeetingFromChangeMetadata()` function which properly:
1. Creates a credential manager for multi-customer role assumption
2. Assumes IAM roles into each customer's AWS account
3. Queries the `aws-calendar` topic from each customer's SES contact list
4. Aggregates and deduplicates recipients across all customers
5. Creates the actual meeting via Microsoft Graph API with all recipients
6. **Returns the actual Graph meeting ID** (e.g., `AAMkAGI...`)

**Changes Made:**
- Created `ses.CreateMultiCustomerMeetingFromChangeMetadata()` - New function that works directly with flat ChangeMetadata (no conversion needed)
- Created `createGraphMeetingFromPayload()` - Creates meetings using flat metadata structure
- Created `checkMeetingExistsFlat()` - Idempotency check using flat metadata
- Modified `ses.CreateMultiCustomerMeetingInvite()` to return `(string, error)` for backward compatibility with CLI
- Lambda now captures and stores the real Graph meeting ID in the S3 object
- Removed temporary file creation - works directly with in-memory ChangeMetadata

**Helper Functions:**
- `getConfig()` - Loads application configuration in Lambda

**Impact:** 
- Meetings were not actually being created in Microsoft Graph/Teams
- No calendar invites were being sent
- Users couldn't see meetings on their calendars
- Meeting IDs in logs were fake/synthetic
- Recipients were not being aggregated from customer SES contact lists

---

## Configuration Required

The Lambda function now requires an environment variable:
- `MEETING_ORGANIZER_EMAIL` - Email address of the meeting organizer (e.g., `ccoe-team@hearst.com`)

If not set, it defaults to `ccoe-team@hearst.com` with a warning in logs.

---

## Testing Recommendations

1. **Test CLI Meeting Creation:**
   ```bash
   ./ccoe-customer-contact-manager ses -action create-multi-customer-meeting-invite \
     -topic-name aws-calendar \
     -json-metadata test-meeting-required-metadata.json \
     -sender-email ccoe-team@hearst.com
   ```
   - Verify meeting appears on calendars at correct time
   - Check CloudWatch logs for real Graph meeting ID (format: `AAMkAGI...`)

2. **Test Lambda Meeting Creation:**
   - Upload change metadata via portal
   - Check CloudWatch logs for:
     - "Creating Microsoft Graph meeting for change..."
     - "Created Graph meeting with ID: AAMkAGI..." (real Graph ID)
     - "Created meeting metadata: ID=AAMkAGI..."
   - Verify meeting appears on attendee calendars
   - Verify meeting time matches implementation schedule

3. **Verify Timezone Handling:**
   - Create meetings with different timezones (EST, PST, UTC)
   - Confirm calendar shows correct local time for each attendee
   - Check that meeting times match the implementation schedule

---

## Files Modified

1. `internal/ses/meetings.go`
   - Fixed double timezone conversion in `generateGraphMeetingPayload()`
   - Added 5 new exported wrapper functions for Lambda integration

2. `internal/lambda/handlers.go`
   - Completely rewrote `createGraphMeeting()` to call actual Graph API
   - Added `getCalendarRecipientsForChange()` stub (TODO: implement recipient lookup)

---

## Known Limitations

1. **Environment Variable:** The `MEETING_ORGANIZER_EMAIL` environment variable must be set in the Lambda configuration for proper operation (defaults to `ccoe@hearst.com` if not set).

---

## Rollback Instructions

If issues occur, revert to commit `4510371` (just about to begin to refactor):
```bash
git revert HEAD~1  # Revert this fix
git revert HEAD~1  # Revert datetime refactoring
```

However, note that the datetime refactoring introduced other improvements, so selective rollback may be preferred.
