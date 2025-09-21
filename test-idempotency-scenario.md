# Calendar Invite Idempotency Test Scenarios

## 🎯 Improved Idempotency Behavior

### Before the Fix:
- ❌ Only checked subject + start time (within 1 minute)
- ❌ If meeting existed, did nothing (no updates)
- ❌ Time changes created duplicate meetings
- ❌ Attendee changes were ignored

### After the Fix:
- ✅ Checks subject with unique identifier (ticket number)
- ✅ Updates existing meetings with new details
- ✅ True idempotency - same result regardless of how many times you run it
- ✅ Handles all types of changes (time, attendees, location, etc.)

## 📋 Test Scenarios

### Scenario 1: Initial Meeting Creation
```bash
# First run - creates new meeting
./aws-alternate-contact-manager ses -action create-meeting-invite \
  -json-metadata sample-metadata-from-form.json \
  -sender-email change-manager@hearst.com

Expected Output:
✅ Meeting created successfully:
   Meeting ID: AAMkAD...
   Subject: PostgreSQL RDS Upgrade - Implementation Bridge [CHG0123456]
```

### Scenario 2: Idempotent Re-run (No Changes)
```bash
# Second run with same metadata - should detect no changes
./aws-alternate-contact-manager ses -action create-meeting-invite \
  -json-metadata sample-metadata-from-form.json \
  -sender-email change-manager@hearst.com

Expected Output:
✅ Meeting already exists (idempotent):
   Meeting ID: AAMkAD...
   Subject: PostgreSQL RDS Upgrade - Implementation Bridge [CHG0123456]
📋 No changes detected - meeting is already up to date
   Web Link: https://teams.microsoft.com/...
   Teams Join URL: https://teams.microsoft.com/l/meetup-join/...
```

### Scenario 3: Time Change Update
```bash
# Modify the metadata file to change start time from 02:00 to 03:00
# Run again - should detect changes and update existing meeting

Expected Output:
✅ Meeting already exists (idempotent):
   Meeting ID: AAMkAD... (same ID)
   Subject: PostgreSQL RDS Upgrade - Implementation Bridge [CHG0123456]
🔄 Detected changes - updating meeting details...
✅ Meeting updated successfully
```

### Scenario 4: Attendee List Changes
```bash
# Add new attendees to the metadata file
# Run again - should detect changes and update existing meeting with new attendee list

Expected Output:
✅ Meeting already exists (idempotent):
   Meeting ID: AAMkAD... (same ID)
   Subject: PostgreSQL RDS Upgrade - Implementation Bridge [CHG0123456]
🔄 Detected changes - updating meeting details...
✅ Meeting updated successfully
```

### Scenario 5: Force Update (Body Content Changes)
```bash
# Modify body content in the metadata file (change reason, implementation plan, etc.)
# Run with --force-update flag to update regardless of detection

./aws-alternate-contact-manager ses -action create-meeting-invite \
  -json-metadata sample-metadata-from-form.json \
  -sender-email change-manager@hearst.com \
  --force-update

Expected Output:
✅ Meeting already exists (idempotent):
   Meeting ID: AAMkAD... (same ID)
   Subject: PostgreSQL RDS Upgrade - Implementation Bridge [CHG0123456]
🔄 Force update requested - updating meeting details...
✅ Meeting updated successfully (forced)
```

## 🔧 Technical Implementation Details

### Unique Subject Generation:
- **With ServiceNow ticket**: `"Meeting Title [CHG0123456]"`
- **With JIRA ticket only**: `"Meeting Title [INFRA-2847]"`
- **No tickets**: `"Meeting Title"` (falls back to original title)

### Meeting Identification:
- Uses **exact subject match** for finding existing meetings
- No longer depends on time matching (more flexible)
- Ticket numbers in subject ensure uniqueness across different changes

### Update Process:
1. **Search** for existing meeting by enhanced subject
2. **If found**: Use Microsoft Graph PATCH API to update all details
3. **If not found**: Create new meeting with POST API
4. **Result**: Same meeting ID maintained across updates

## ✅ Benefits

1. **True Idempotency**: Run the command multiple times safely
2. **Smart Change Detection**: Automatically detects changes in subject, times, location, and attendees
3. **Body Content Flexibility**: Use `--force-update` when body content changes (avoids complex HTML comparison)
4. **No Duplicates**: Prevents creation of multiple meetings for the same change
5. **Unique Identification**: Ticket numbers ensure meetings for different changes don't conflict
6. **User Control**: Choose when to force updates vs. rely on automatic detection
7. **Backward Compatible**: Still works with existing metadata files

## 🧪 Testing Checklist

- [ ] Initial meeting creation works
- [ ] Re-running with same data updates (doesn't duplicate)
- [ ] Time changes update existing meeting
- [ ] Attendee changes update existing meeting
- [ ] Location changes update existing meeting
- [ ] Different ticket numbers create separate meetings
- [ ] Meetings without tickets still work (fallback behavior)