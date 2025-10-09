# Dashboard Counting Fix Summary

## Problem Identified
The HTML dashboard was showing incorrect counts:
- **0 approval requests** when there should be 1
- **1 completed request** when there should be 0

## Root Cause Analysis

### 1. **Flawed Statistics Calculation**
The Lambda function `handleGetStatistics()` was using **estimates** instead of actual data:
```javascript
// OLD (BROKEN) CODE:
activeChanges = Math.floor(totalChanges * 0.3); // 30% estimate
completedChanges = totalChanges - activeChanges;  // 70% estimate
```

### 2. **Missing User Filtering**
Both statistics and recent changes were showing **all users' data** instead of filtering by the current user.

### 3. **Status Mapping Issues**
The dashboard expected:
- **Approval Requests** = status "submitted"
- **Completed Changes** = status "completed"

But the Lambda was providing estimated "active" and "completed" counts.

## Solution Implemented

### 1. **Accurate Status Counting** (`lambda/upload_lambda/upload-metadata-lambda.js`)

#### Before (Estimates):
```javascript
activeChanges = Math.floor(totalChanges * 0.3); // Estimate!
completedChanges = totalChanges - activeChanges; // Estimate!
```

#### After (Actual Data):
```javascript
// Read each file and count by actual status
const status = change.status || 'submitted';
switch (status.toLowerCase()) {
    case 'submitted':
        submittedChanges++;  // Real count
        break;
    case 'completed':
        completedChanges++;  // Real count
        break;
    // ... other statuses
}
```

### 2. **User-Specific Filtering**

#### Statistics Function:
```javascript
// Check if this change belongs to the current user
const isUserChange = change.createdBy === userEmail || 
                   change.submittedBy === userEmail || 
                   change.modifiedBy === userEmail;

if (isUserChange) {
    // Only count changes for this user
    totalChanges++;
    // Count by status...
}
```

#### Recent Changes Function:
```javascript
// Filter recent changes by user
const isUserChange = change.createdBy === userEmail || 
                   change.submittedBy === userEmail || 
                   change.modifiedBy === userEmail;

if (isUserChange) {
    changes.push(change);
}
```

### 3. **Correct Status Mapping**

#### API Response Format:
```javascript
return {
    total: totalChanges,
    draft: draftChanges,
    submitted: submittedChanges,    // Maps to "Approval Requests"
    approved: approvedChanges,
    completed: completedChanges,    // Maps to "Completed Changes"
    cancelled: cancelledChanges,
    active: submittedChanges        // Backward compatibility
};
```

#### Dashboard Mapping:
```javascript
// HTML dashboard correctly maps:
document.getElementById('activeChanges').textContent = stats.submitted || stats.active || 0;
document.getElementById('completedChanges').textContent = stats.completed || 0;
```

## Status Types Handled

Based on `status.md`, the system now correctly handles:

| Status | Dashboard Label | Description |
|--------|----------------|-------------|
| `draft` | "Draft Changes" | Saved but not submitted |
| `submitted` | "Approval Requests" | Awaiting approval |
| `approved` | "Approved" | Approved but not implemented |
| `completed` | "Completed Changes" | Fully implemented |
| `cancelled` | "Cancelled" | Cancelled changes |

## Performance Optimizations

### 1. **Limited File Reading**
```javascript
// Limit to recent files to avoid timeout
const recentObjects = archiveObjects.Contents
    .sort((a, b) => new Date(b.LastModified) - new Date(a.LastModified))
    .slice(0, 100);  // Only read last 100 files
```

### 2. **Efficient User Filtering**
```javascript
// Stop when we have enough user changes
if (changes.length >= limit) {
    break;
}
```

### 3. **Error Handling**
```javascript
try {
    const change = JSON.parse(objData.Body.toString());
    // Process change...
} catch (error) {
    console.warn(`Error reading ${obj.Key}:`, error.message);
    // Continue processing other files
}
```

## Expected Results

After this fix, the dashboard should show:

### ✅ **Accurate Counts**
- **Approval Requests**: Actual count of changes with status "submitted" for current user
- **Completed Changes**: Actual count of changes with status "completed" for current user
- **Draft Changes**: Actual count of draft files for current user
- **Total Changes**: Sum of all changes for current user

### ✅ **User-Specific Data**
- Only shows counts and recent changes for the logged-in user
- No longer shows other users' changes in personal dashboard

### ✅ **Real-Time Accuracy**
- Counts reflect actual file contents, not estimates
- Status changes are immediately reflected in dashboard
- No more "phantom" completed changes

## Testing Scenarios

### Test Case 1: User with 1 Submitted Change
- **Expected**: Approval Requests = 1, Completed = 0
- **Previous**: Approval Requests = 0, Completed = 1 (wrong)
- **After Fix**: Approval Requests = 1, Completed = 0 ✅

### Test Case 2: User with Mixed Statuses
- **Data**: 2 submitted, 1 completed, 3 drafts
- **Expected**: Approval Requests = 2, Completed = 1, Drafts = 3, Total = 6
- **After Fix**: Accurate counts based on actual file status ✅

### Test Case 3: Multiple Users
- **Expected**: Each user sees only their own changes
- **Previous**: All users saw combined counts (wrong)
- **After Fix**: User-specific filtering ✅

## Monitoring

To verify the fix is working:

1. **Check CloudWatch Logs** for the Lambda function:
   ```
   Getting statistics for user: user@example.com
   Statistics for user@example.com: total=1, draft=0, submitted=1, completed=0
   ```

2. **Verify Dashboard API Calls**:
   - `/api/changes/statistics` returns user-specific counts
   - `/api/changes/recent` returns user-specific recent changes

3. **Test Different Users**:
   - Each user should see different counts
   - Counts should match actual file contents

The dashboard counting issue should now be resolved with accurate, user-specific statistics based on actual file data rather than estimates.