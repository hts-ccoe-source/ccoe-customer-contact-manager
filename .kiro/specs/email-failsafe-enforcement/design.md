# Design Document

## Overview

This design implements comprehensive `restricted_recipients` filtering across all email sending paths in the CCOE Customer Contact Manager. The solution provides a centralized filtering mechanism that can be used by announcement processors, change processors, and meeting schedulers to enforce email restrictions in non-production environments.

## Architecture

### Current State

**Email Sending Paths:**
1. **Change Request Emails** (`internal/lambda/handlers.go`)
   - Uses `shouldSendToRecipient()` function ✅ WORKING
   - Checks global `currentCustomerInfo` variable
   - Filters before sending each email

2. **Announcement Emails** (`internal/processors/announcement_processor.go`)
   - No filtering currently ❌ BROKEN
   - Sends to all topic subscribers
   - Bypasses `restricted_recipients`

3. **Meeting Invitations** (`internal/ses/meetings.go`)
   - Recently fixed with `filterRecipientsByRestrictions()` ✅ FIXED
   - Filters before creating Graph API payload
   - Works for both single and multi-customer meetings

### Target State

All three email paths will use a consistent filtering approach:
- Centralized filtering function in a shared location
- Accepts customer configuration and recipient list
- Returns filtered recipients with skip count
- Consistent logging across all paths

## Components and Interfaces

### 1. Centralized Filtering Function

**Location:** `internal/types/types.go` (add to CustomerAccountInfo)

**Function Signature:**
```go
// FilterRecipients filters a list of email addresses based on restricted_recipients configuration
// Returns filtered list and count of skipped recipients
func (c *CustomerAccountInfo) FilterRecipients(recipients []string) (filtered []string, skipped int)
```

**Behavior:**
- If `RestrictedRecipients` is empty/nil, return all recipients (no filtering)
- Normalize emails (lowercase, trim) for comparison
- Build map of allowed recipients for O(1) lookup
- Filter input list, counting skipped recipients
- Return filtered list and skip count

### 2. Announcement Processor Updates

**File:** `internal/processors/announcement_processor.go`

**Changes:**
- Add customer info parameter to `sendAnnouncementEmails()`
- Get customer config from `p.Config.CustomerMappings[customerCode]`
- Call `customerInfo.FilterRecipients()` before sending emails
- Log filtered results
- Skip email sending if no recipients remain

**Modified Function:**
```go
func (p *AnnouncementProcessor) sendAnnouncementEmails(
    ctx context.Context,
    customerCode string,
    announcement *types.AnnouncementMetadata,
) error
```

### 3. Meeting Scheduler Updates

**File:** `internal/ses/meetings.go`

**Changes:**
- Replace existing `filterRecipientsByRestrictions()` with calls to `CustomerAccountInfo.FilterRecipients()`
- Simplify logic by using the centralized function
- Maintain existing logging behavior

### 4. Change Request Handler (No Changes)

**File:** `internal/lambda/handlers.go`

**Status:** Already working correctly
- Uses `shouldSendToRecipient()` which calls `currentCustomerInfo.IsRecipientAllowed()`
- No changes needed

## Data Models

### CustomerAccountInfo Enhancement

```go
type CustomerAccountInfo struct {
    // ... existing fields ...
    RestrictedRecipients []string `json:"restricted_recipients,omitempty"`
}

// FilterRecipients filters recipients based on restricted_recipients configuration
func (c *CustomerAccountInfo) FilterRecipients(recipients []string) ([]string, int) {
    // If no restrictions, allow all
    if len(c.RestrictedRecipients) == 0 {
        return recipients, 0
    }
    
    // Build allowed recipients map
    allowedMap := make(map[string]bool)
    for _, email := range c.RestrictedRecipients {
        normalized := strings.ToLower(strings.TrimSpace(email))
        allowedMap[normalized] = true
    }
    
    // Filter recipients
    var filtered []string
    skipped := 0
    
    for _, email := range recipients {
        normalized := strings.ToLower(strings.TrimSpace(email))
        if allowedMap[normalized] {
            filtered = append(filtered, email)
        } else {
            skipped++
        }
    }
    
    return filtered, skipped
}
```

## Error Handling

### No Recipients After Filtering

**Behavior:**
- Log warning: "⚠️ No allowed recipients after applying restricted_recipients filter"
- Skip email/meeting creation
- Return nil error (not a failure condition)

### Customer Config Not Found

**Behavior:**
- Log warning with customer code
- Proceed without filtering (fail-open for safety)
- Log: "⚠️ Customer config not found for {code}, proceeding without filtering"

### Empty Recipient List

**Behavior:**
- Log info: "ℹ️ No subscribers found for topic {topic}"
- Skip email/meeting creation
- Return nil error (not a failure condition)

## Testing Strategy

### Unit Tests

1. **CustomerAccountInfo.FilterRecipients()**
   - Test with no restrictions (returns all)
   - Test with restrictions (filters correctly)
   - Test with empty input list
   - Test email normalization (case, whitespace)
   - Test all filtered out scenario

2. **Announcement Processor**
   - Test filtering applied before email send
   - Test skip count logged correctly
   - Test no recipients scenario
   - Test no restrictions scenario

3. **Meeting Scheduler**
   - Test filtering applied before meeting creation
   - Test multi-customer aggregation
   - Test manual attendees filtered

### Integration Tests

1. **End-to-End Announcement Flow**
   - Create announcement for htsnonprod
   - Verify only restricted recipients receive email
   - Check logs for skip messages

2. **End-to-End Meeting Flow**
   - Create meeting for htsnonprod
   - Verify only restricted recipients in attendee list
   - Check Graph API payload

3. **Multi-Customer Scenarios**
   - Mix of customers with/without restrictions
   - Verify correct filtering per customer

## Implementation Notes

### Logging Standards

All filtering operations must log:
```
⏭️  Skipping {email} (not on restricted recipient list)
⏭️  Skipped {count} recipients due to restricted_recipients configuration
✅ {count} recipients allowed after filtering
⚠️  No allowed recipients after applying restricted_recipients filter
```

### Performance Considerations

- Use map for O(1) recipient lookup
- Normalize emails once during map building
- Avoid repeated config lookups

### Backward Compatibility

- Existing `IsRecipientAllowed()` method remains unchanged
- New `FilterRecipients()` method is additive
- No breaking changes to existing APIs

## Migration Path

1. Add `FilterRecipients()` method to `CustomerAccountInfo`
2. Update announcement processor to use new method
3. Update meeting scheduler to use new method
4. Deploy and test in non-prod
5. Verify logs show correct filtering
6. Deploy to production
