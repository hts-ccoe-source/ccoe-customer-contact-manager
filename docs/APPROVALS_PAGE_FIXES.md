# Approvals Page - API Endpoint Fixes

## Problem

The approvals page was calling non-existent API endpoints, causing 404 errors:

```
❌ GET /api/user/context - 404 (Not Found)
❌ GET /api/changes/all - 404 (Not Found)
❌ GET /api/changes/customer/{code} - 404 (Not Found)
❌ PUT /api/changes/{id} - 404 (Not Found)
```

## Root Cause

The s3-client.js and approvals-page.js were using endpoints with `/api/` prefix that don't exist in the Lambda handler. According to `docs/API_ENDPOINTS.md`, the actual endpoints don't use the `/api/` prefix.

## Fixes Applied

### 1. Fixed s3-client.js Endpoints

**Before:**
```javascript
async fetchAllChanges() {
    const path = '/api/changes/all';  // ❌ Doesn't exist
    return await this.fetchObjects(path);
}

async fetchCustomerChanges(customerCode) {
    const path = `/api/changes/customer/${customerCode}`;  // ❌ Doesn't exist
    return await this.fetchObjects(path);
}

async updateChange(changeId, changeData) {
    const path = `/api/changes/${changeId}`;  // ❌ Wrong prefix
    // ...
}
```

**After:**
```javascript
async fetchAllChanges() {
    const path = '/changes';  // ✅ Correct endpoint
    return await this.fetchObjects(path);
}

async fetchCustomerChanges(customerCode) {
    // ✅ Fetch all and filter client-side (no customer-specific endpoint exists)
    const allChanges = await this.fetchAllChanges();
    return allChanges.filter(change => {
        if (Array.isArray(change.customers)) {
            return change.customers.includes(customerCode);
        } else if (change.customer) {
            return change.customer === customerCode;
        }
        return false;
    });
}

async updateChange(changeId, changeData) {
    const path = `/changes/${changeId}`;  // ✅ Correct endpoint
    // ...
}
```

### 2. Fixed approvals-page.js User Context Detection

**Before:**
```javascript
async detectUserContext() {
    const response = await fetch(`${window.location.origin}/api/user/context`, {
        method: 'GET',
        credentials: 'same-origin'
    });
    // ❌ This endpoint doesn't exist, causing 404
}
```

**After:**
```javascript
async detectUserContext() {
    // ✅ Skip the non-existent API call
    // Use window.portal.currentUser directly and infer context
    
    if (window.portal && window.portal.currentUser) {
        this.userContext = this.inferUserContext(window.portal.currentUser);
        console.log('User context inferred from portal:', this.userContext);
    } else {
        // Default to admin for demo/development
        this.userContext = {
            isAdmin: true,
            customerCode: null,
            email: 'demo.user@hearst.com'
        };
    }
}
```

### 3. Added Graceful Fallbacks for Announcements

Since announcement endpoints don't exist yet, added try-catch blocks:

```javascript
async fetchAnnouncements() {
    const path = '/announcements';
    try {
        return await this.fetchObjects(path);
    } catch (error) {
        console.warn('Announcements endpoint not available:', error);
        return [];  // ✅ Return empty array instead of throwing
    }
}
```

## Correct API Endpoints (from API_ENDPOINTS.md)

| Purpose | Correct Endpoint | Method |
|---------|-----------------|--------|
| Get all changes | `/changes` | GET |
| Get specific change | `/changes/{id}` | GET |
| Update change | `/changes/{id}` | PUT |
| Approve change | `/changes/{id}/approve` | POST |
| Cancel change | `/changes/{id}/cancel` | POST |
| Complete change | `/changes/{id}/complete` | POST |
| Delete change | `/changes/{id}` | DELETE |
| Get my changes | `/my-changes` | GET |
| Search changes | `/changes/search` | POST |

## Testing

After these fixes, the approvals page should:

1. ✅ Load without 404 errors
2. ✅ Fetch all changes using `/changes` endpoint
3. ✅ Filter changes by customer client-side
4. ✅ Detect user context from window.portal
5. ✅ Update changes using correct `/changes/{id}` endpoint
6. ✅ Handle missing announcement endpoints gracefully

## Notes

- **No `/api/` prefix**: The Lambda handler doesn't use `/api/` prefix for most endpoints
- **Client-side filtering**: Since there's no `/changes/customer/{code}` endpoint, we fetch all changes and filter client-side
- **User context**: The `/api/user/context` endpoint doesn't exist, so we infer from `window.portal.currentUser`
- **Announcements**: Announcement endpoints don't exist yet, so we return empty arrays gracefully

## Future Improvements

If backend endpoints are added in the future:

1. **Add `/api/user/context` endpoint** to return:
   ```json
   {
     "isAdmin": true,
     "customerCode": "hts",
     "email": "user@hearst.com"
   }
   ```

2. **Add customer-specific endpoint** for better performance:
   ```
   GET /changes/customer/{customerCode}
   ```

3. **Add announcement endpoints**:
   ```
   GET /announcements
   GET /announcements/customer/{customerCode}
   ```
