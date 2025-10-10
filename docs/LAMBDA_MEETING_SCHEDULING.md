# Lambda Meeting Scheduling Implementation

## Overview

The Lambda handler has been enhanced to automatically schedule multi-customer meetings when changes are approved and contain meeting settings. This functionality integrates with the existing SQS processing workflow to provide seamless meeting scheduling as part of the change management process.

## Implementation Details

### Trigger Conditions

The Lambda function will automatically schedule a multi-customer meeting when:

1. **Change Status**: The change request type is `approved_announcement`
2. **Meeting Settings Present**: The change metadata contains meeting-related fields
3. **Multi-Customer**: The change affects multiple customers (for auto-scheduling)

### Meeting Detection Logic

The system checks for meeting requirements in the following order:

#### 1. Explicit Meeting Required Field
```json
{
  "metadata": {
    "meetingRequired": "yes"  // or "true" or boolean true
  }
}
```

#### 2. Meeting Details Present
```json
{
  "metadata": {
    "meetingTitle": "Implementation Meeting",
    "meetingDate": "2025-01-10",
    "meetingDuration": "60",
    "meetingLocation": "Microsoft Teams"
  }
}
```

#### 3. Auto-Scheduling for Multi-Customer Changes
- **Condition**: Change affects 2+ customers AND has implementation schedule
- **Auto-generated meeting**: Uses implementation date/time for meeting scheduling
- **Default settings**: 60-minute meeting in Microsoft Teams

### Integration Points

#### Lambda Handler Flow
```
SQS Event → ProcessChangeRequest → Send Email → Check Meeting → Schedule Meeting
```

#### Key Functions Added

1. **`ScheduleMultiCustomerMeetingIfNeeded()`**
   - Main orchestration function
   - Checks meeting requirements
   - Calls multi-customer meeting creation

2. **`createTempMeetingMetadata()`**
   - Converts ChangeMetadata to ApprovalRequestMetadata format
   - Creates temporary JSON file for meeting functions
   - Handles format compatibility

### Meeting Scheduling Process

#### Step 1: Meeting Detection
```go
// Check explicit meeting requirement
if metadata.Metadata["meetingRequired"] == "yes" {
    meetingRequired = true
}

// Check for meeting details
if metadata.Metadata["meetingTitle"] != "" {
    meetingRequired = true
}

// Auto-schedule for multi-customer implementations
if len(metadata.Customers) > 1 && metadata.ImplementationBeginDate != "" {
    meetingRequired = true
}
```

#### Step 2: Metadata Conversion
```go
// Convert to ApprovalRequestMetadata format
meetingMetadata := types.ApprovalRequestMetadata{
    ChangeMetadata: {
        Title: metadata.ChangeTitle,
        CustomerCodes: metadata.Customers,
        // ... other fields
    },
    MeetingInvite: {
        Title: meetingTitle,
        StartTime: meetingStartTime,
        DurationMinutes: durationMinutes,
        Location: meetingLocation,
    },
}
```

#### Step 3: Multi-Customer Meeting Creation
```go
err = ses.CreateMultiCustomerMeetingInvite(
    credentialManager,
    metadata.Customers,
    "aws-calendar",
    tempMetadataFile,
    "ccoe@hearst.com",
    false, // not dry-run
    false, // not force-update
)
```

## Configuration

### Required Settings

#### Environment Variables
- **Azure AD credentials** (loaded from Parameter Store):
  - `/azure/client-id`
  - `/azure/client-secret`
  - `/azure/tenant-id`

#### Customer Configuration
- **SES roles**: Each customer must have SES role configured
- **aws-calendar topic**: Must exist in each customer's SES service
- **Topic subscribers**: Users subscribed to aws-calendar topic receive invites

### Default Values

| Setting | Default Value | Description |
|---------|---------------|-------------|
| Topic Name | `aws-calendar` | SES topic for meeting invites |
| Sender Email | `ccoe@hearst.com` | Meeting organizer |
| Duration | `60` minutes | Default meeting length |
| Location | `Microsoft Teams` | Default meeting location |
| Timezone | `America/New_York` | Default timezone |

## Metadata Format Examples

### Explicit Meeting Request
```json
{
  "changeId": "CHG-12345",
  "changeTitle": "Production Deployment",
  "customers": ["customer-a", "customer-b"],
  "status": "approved",
  "metadata": {
    "meetingRequired": "yes",
    "meetingTitle": "Production Deployment Meeting",
    "meetingDate": "2025-01-10",
    "meetingDuration": "90",
    "meetingLocation": "Microsoft Teams"
  },
  "implementationBeginDate": "2025-01-10",
  "implementationBeginTime": "10:00",
  "timezone": "America/New_York"
}
```

### Auto-Scheduled Meeting (Multi-Customer)
```json
{
  "changeId": "CHG-67890",
  "changeTitle": "Infrastructure Update",
  "customers": ["customer-a", "customer-b", "customer-c"],
  "status": "approved",
  "implementationBeginDate": "2025-01-15",
  "implementationBeginTime": "14:00",
  "implementationEndDate": "2025-01-15",
  "implementationEndTime": "16:00",
  "timezone": "America/New_York"
}
```

### No Meeting Required
```json
{
  "changeId": "CHG-11111",
  "changeTitle": "Documentation Update",
  "customers": ["customer-a"],
  "status": "approved",
  "metadata": {}
}
```

## Error Handling

### Meeting Scheduling Failures
- **Non-blocking**: Meeting failures don't prevent email notifications
- **Logging**: Detailed error logging for troubleshooting
- **Graceful degradation**: Change processing continues even if meeting fails

### Common Error Scenarios
1. **Missing Azure credentials**: Meeting creation fails, emails still sent
2. **No calendar subscribers**: Meeting created but no attendees
3. **Invalid meeting metadata**: Uses default values where possible
4. **Graph API failures**: Error logged, no fallback (as per requirements)

## Logging and Monitoring

### Log Messages

#### Meeting Detection
```
🔍 Checking if change CHG-12345 requires meeting scheduling
📅 Meeting required for change CHG-12345 with 3 customers: [customer-a, customer-b, customer-c]
📋 Meeting details - Title: Implementation Meeting, Date: 2025-01-10, Duration: 60, Location: Microsoft Teams
```

#### Meeting Scheduling
```
🚀 Scheduling multi-customer meeting for change CHG-12345
📄 Created temporary meeting metadata file: /tmp/meeting-metadata-CHG-12345-1704902400.json
✅ Successfully scheduled multi-customer meeting for change CHG-12345 with 3 customers
```

#### Error Cases
```
⚠️  No customers specified for change CHG-12345, cannot schedule meeting
❌ Failed to schedule multi-customer meeting: failed to create meeting via Microsoft Graph API: access denied
📋 No meeting required for change CHG-12345
```

### Monitoring Points
- **Meeting scheduling success rate**
- **Meeting creation latency**
- **Graph API error rates**
- **Customer participation rates**

## Testing

### Unit Tests Added

#### Test Coverage
- **Meeting detection logic**: Various metadata formats
- **Auto-scheduling rules**: Multi-customer vs single-customer
- **Metadata conversion**: ChangeMetadata → ApprovalRequestMetadata
- **Error handling**: Missing fields, invalid data

#### Test Cases
1. **Explicit meeting required**: `meetingRequired: "yes"`
2. **Meeting details present**: `meetingTitle` provided
3. **Auto-scheduling**: Multi-customer with implementation schedule
4. **No meeting**: Single customer or no schedule
5. **Metadata conversion**: Proper format transformation

### Integration Testing

#### Lambda Event Simulation
```json
{
  "Records": [
    {
      "body": "{\"Records\":[{\"s3\":{\"bucket\":{\"name\":\"test-bucket\"},\"object\":{\"key\":\"customers/customer-a/approved-change.json\"}}}]}"
    }
  ]
}
```

#### Expected Behavior
1. **Parse S3 event**: Extract customer and object key
2. **Download metadata**: Get change details from S3
3. **Send approval email**: Normal email workflow
4. **Check meeting**: Detect meeting requirements
5. **Schedule meeting**: Create multi-customer meeting if needed

## Performance Considerations

### Concurrent Processing
- **Meeting scheduling**: Runs after email sending (non-blocking)
- **Recipient aggregation**: Concurrent queries across customers
- **Temporary files**: Cleaned up automatically by Lambda runtime

### Resource Usage
- **Memory**: Minimal additional memory for meeting metadata
- **Execution time**: ~2-5 seconds additional for meeting scheduling
- **API calls**: Microsoft Graph API calls for meeting creation

## Security

### Access Control
- **Customer isolation**: Each customer's SES service accessed separately
- **Role assumption**: Uses existing customer-specific SES roles
- **Meeting organizer**: Fixed organizer email for security

### Data Handling
- **Temporary files**: Created in Lambda `/tmp` directory
- **Automatic cleanup**: Files removed when Lambda execution ends
- **No persistent storage**: Meeting metadata not stored permanently

## Future Enhancements

### Potential Improvements
1. **Meeting templates**: Configurable meeting templates by change type
2. **Attendee management**: Optional vs required attendees
3. **Meeting updates**: Update existing meetings when changes are modified
4. **Calendar integration**: Integration with other calendar systems
5. **Meeting analytics**: Track meeting attendance and effectiveness

### Configuration Enhancements
1. **Per-customer settings**: Customer-specific meeting preferences
2. **Topic customization**: Configurable SES topics for different meeting types
3. **Time zone handling**: Automatic time zone detection and conversion
4. **Meeting duration rules**: Dynamic duration based on change complexity

## Conclusion

The Lambda meeting scheduling functionality provides seamless integration between change management and meeting coordination. By automatically detecting meeting requirements and scheduling multi-customer meetings, the system reduces manual coordination overhead while ensuring all stakeholders are properly notified and invited to relevant implementation meetings.

**Key Benefits:**
- ✅ **Automatic scheduling**: No manual meeting creation required
- ✅ **Multi-customer support**: Single meeting for all affected customers
- ✅ **Concurrent processing**: Fast recipient aggregation across customers
- ✅ **Error resilience**: Meeting failures don't block change processing
- ✅ **Comprehensive logging**: Full visibility into meeting scheduling process
- ✅ **Flexible detection**: Multiple ways to specify meeting requirements