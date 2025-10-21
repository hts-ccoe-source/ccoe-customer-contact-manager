# ETag Polling Flow Diagram

## User Flow

```
User clicks "Approve" button
         ↓
Status changes to "approved"
         ↓
Success message: "Watching for meeting details..."
         ↓
Start ETag polling (every 2s)
         ↓
┌─────────────────────────────────────┐
│  Poll Loop (with ETag headers)     │
│                                     │
│  Request with If-None-Match: ETag  │
│         ↓                           │
│  Response 304? → Continue polling   │
│         ↓                           │
│  Response 200? → Check for meeting  │
│         ↓                           │
│  Meeting details found?             │
│    YES → Update card + modal        │
│    NO  → Continue polling           │
│         ↓                           │
│  After 20s: Slow down to 5s         │
│         ↓                           │
│  After 60s: Stop polling            │
└─────────────────────────────────────┘
         ↓
Meeting details detected!
         ↓
Update ONLY affected change card
         ↓
🎥 Join button appears (Teams purple)
         ↓
If modal open → Update modal content
         ↓
Success message: "Join button is now available."
```

## What Gets Updated

### Before Meeting Details
```
┌─────────────────────────────────────┐
│ Change Card                         │
│                                     │
│ [View Details] [💣 Cancel] [✅ Approve] │
└─────────────────────────────────────┘
```

### After Meeting Details (Optimized Update)
```
┌─────────────────────────────────────┐
│ Change Card                         │
│                                     │
│ [🎥 Join] [View Details]            │
│ ✓ Approved by User on Date         │
└─────────────────────────────────────┘
```

## Efficiency Comparison

### Old Approach (Full Refresh)
- Fetch ALL changes from S3
- Re-render entire page
- Lose scroll position
- Modal closes if open
- ~500KB+ data transfer

### New Approach (Targeted Update)
- Fetch ONLY updated change (with ETag)
- Update only affected card
- Preserve scroll position
- Modal updates in-place
- ~5KB data transfer (or 0KB with 304)

## ETag Efficiency

### First Poll (No ETag)
```
Request:  GET /changes/12345
Response: 200 OK
          ETag: "abc123"
          Body: {...change data...}
```

### Subsequent Polls (With ETag)
```
Request:  GET /changes/12345
          If-None-Match: "abc123"
Response: 304 Not Modified
          (No body - saves bandwidth!)
```

### When Meeting Details Added
```
Request:  GET /changes/12345
          If-None-Match: "abc123"
Response: 200 OK
          ETag: "xyz789"  (changed!)
          Body: {...change with meeting details...}
```

## Teams Purple Join Button

Color: `#6264A7` (Microsoft Teams brand color)
Hover: `#464775` (darker shade)

Visual appearance:
```
┌──────────────┐
│  🎥 Join     │  ← Teams purple background
└──────────────┘
     ↓ hover
┌──────────────┐
│  🎥 Join     │  ← Darker purple + lift effect
└──────────────┘
```
