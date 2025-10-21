# ETag-Based Polling for Meeting Details

## Overview
Implemented smart polling to automatically refresh the UI when the backend adds meeting invite details to approved changes and announcements. Uses targeted updates to avoid full page reloads.

## Implementation Details

### Files Modified
- `html/assets/js/approvals-page.js` - Polling logic and UI updates for changes
- `html/assets/js/announcements-page.js` - Polling logic and UI updates for announcements
- `html/my-changes.html` - Polling logic and UI updates for user's changes
- `html/assets/css/shared.css` - Join button styling

### New Methods

#### `startMeetingDetailsWatch(changeId, options)`
- **Trigger**: Called automatically after a change is approved
- **Purpose**: Poll for meeting details to appear in the change object
- **Efficiency**: Uses ETag headers to avoid unnecessary data transfer

#### `updateSingleChange(updatedChange)`
- Updates only the specific change in local data arrays
- Triggers targeted card and modal updates
- No full page refresh needed

#### `updateChangeCard(change)`
- Re-renders only the affected change card(s) in the DOM
- Preserves scroll position and other UI state
- Smooth update without flicker

#### `updateModalIfOpen(change)`
- Detects if the change details modal is open for this change
- Updates modal content in real-time if open
- User sees meeting details appear without closing/reopening

### Polling Strategy
```javascript
{
  initialIntervalMs: 2000,    // Check every 2s for first 20s
  laterIntervalMs: 5000,       // Then every 5s
  maxDurationMs: 60000,        // Give up after 1 minute
  transitionTimeMs: 20000      // Switch to slower polling after 20s
}
```

### How It Works

1. **User approves change** → Status changes to "approved"
2. **Backend processes approval** → SQS → Lambda → Schedules meeting → Updates S3 object
3. **Frontend polls with ETag** → Efficient conditional requests (304 vs 200)
4. **Meeting details detected** → Targeted update of card and modal
5. **Join button appears** → Teams purple button with meeting URL

### ETag Benefits
- **304 Not Modified**: Server returns no body when object unchanged (saves bandwidth)
- **200 OK**: Full object returned only when changed
- **S3 Native**: S3 automatically generates ETags (MD5 hash)
- **CloudFront Compatible**: Works seamlessly with existing CDN setup

### User Experience
- Immediate feedback: "Change approved successfully! Watching for meeting details..."
- Automatic update: "Meeting scheduled! Join button is now available."
- **Join button appears** on the left side of action buttons
- Teams purple color (#6264A7) with hover effect
- No manual refresh needed
- No scroll position loss
- Modal updates in real-time if open
- Graceful timeout after 1 minute if backend takes longer than expected

## Join Button Styling
- Background: Teams purple (#6264A7)
- Hover: Darker purple (#464775) with subtle lift effect
- Icon: 🎥 emoji
- Opens meeting in new tab
- Positioned first in the action button row

## Technical Notes
- Only polls after approval (not on cancel or other status changes)
- Adaptive polling: Fast initially, then slower to reduce load
- Error-tolerant: Continues polling even if individual requests fail
- Clean cleanup: Stops polling when meeting details appear or timeout reached
- Efficient: Updates only affected DOM elements, not entire page
- Modal-aware: Updates open modals automatically (approvals page only)
- Works consistently across all pages:
  - **Approvals page**: Targeted card updates + modal updates
  - **Announcements page**: Targeted card updates
  - **My Changes page**: Re-applies filters to update view
- DRY implementation: Same polling logic reused across all pages
