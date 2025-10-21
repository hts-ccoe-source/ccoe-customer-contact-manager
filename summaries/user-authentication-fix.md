# User Authentication Fix Summary

## Problem
Announcements and other actions were showing `current-user-id` instead of the actual user's email address.

## Root Cause
The frontend `window.portal.currentUser` was falling back to `demo.user@hearst.com` because:
1. The `/auth-check` endpoint didn't exist
2. No simple endpoint to retrieve the authenticated user's email from SAML headers

## Solution

### 1. Added `/api/user` Endpoint (lambda/upload_lambda/upload-metadata-lambda.js)
Created a new simple endpoint that returns the authenticated user's email from SAML headers:

```javascript
else if (path === '/api/user' && method === 'GET') {
    return {
        statusCode: 200,
        headers: {
            'Content-Type': 'application/json',
            'Access-Control-Allow-Origin': '*'
        },
        body: JSON.stringify({ 
            email: userEmail,
            user: userEmail,
            authenticated: true
        })
    };
}
```

### 2. Updated Frontend Authentication (html/assets/js/shared.js)
Improved the `checkAuthentication()` method to:
- Try `/api/user` endpoint first (includes SAML headers)
- Fall back to extracting from response headers
- Only use demo mode as last resort

```javascript
async checkAuthentication() {
    try {
        // Try to get user from API endpoint (includes SAML headers)
        const response = await fetch(`${this.baseUrl}/api/user`, {
            method: 'GET',
            credentials: 'same-origin'
        });

        if (response.ok) {
            const data = await response.json();
            this.currentUser = data.email || data.user || data.username;
            console.log('✅ User authenticated:', this.currentUser);
            this.updateUserInfo();
            return true;
        }
    } catch (error) {
        console.log('⚠️  /api/user endpoint not available, trying alternative methods');
    }
    // ... fallback methods ...
}
```

### 3. Fixed Announcement Topic Mapping (internal/lambda/handlers.go)
Updated `sendAnnouncementEmailViaSES` to properly map announcement types to SES topics:

```go
// Map announcement type to SES topic name
topicName := getAnnouncementTopicName(template.Type)

// Added new function:
func getAnnouncementTopicName(announcementType string) string {
    switch strings.ToLower(announcementType) {
    case "cic":
        return "cic-announce"
    case "finops":
        return "finops-announce"
    case "innersource", "inner":
        return "inner-announce"
    case "general":
        return "general-updates"
    default:
        return "general-updates"
    }
}
```

## SES Topic Mapping

Based on `SESConfig.json`, announcements now correctly route to:

| Announcement Type | SES Topic | Subscribers |
|------------------|-----------|-------------|
| CIC | `cic-announce` | All users (OPT_IN) |
| FinOps | `finops-announce` | Finance role (OPT_OUT) |
| InnerSource | `inner-announce` | cloudeng, developer, devops, networking (OPT_OUT) |
| General | `general-updates` | No default roles (OPT_OUT) |

## Testing

### Frontend Testing
1. Open browser DevTools console
2. Navigate to any page (approvals, announcements, etc.)
3. Check console for: `✅ User authenticated: your.email@hearst.com`
4. Verify `window.portal.currentUser` contains your email:
   ```javascript
   console.log(window.portal.currentUser);
   // Should output: "your.email@hearst.com"
   ```

### Backend Testing
1. Deploy updated Lambda function
2. Test the `/api/user` endpoint:
   ```bash
   curl https://your-api-url.com/api/user \
     -H "x-user-email: test@hearst.com" \
     -H "x-authenticated: true"
   ```
3. Should return:
   ```json
   {
     "email": "test@hearst.com",
     "user": "test@hearst.com",
     "authenticated": true
   }
   ```

### Announcement Action Testing
1. Navigate to approvals page
2. Approve an announcement
3. Check S3 file modifications array:
   ```json
   {
     "modifications": [
       {
         "timestamp": "2025-10-16T...",
         "user_id": "your.email@hearst.com",  // ← Should be your actual email
         "modification_type": "approved"
       }
     ]
   }
   ```

## Files Changed

1. **lambda/upload_lambda/upload-metadata-lambda.js**
   - Added `/api/user` endpoint

2. **html/assets/js/shared.js**
   - Updated `checkAuthentication()` method

3. **internal/lambda/handlers.go**
   - Updated `sendAnnouncementEmailViaSES()` function
   - Added `getAnnouncementTopicName()` function

## Deployment Steps

1. **Deploy Backend Lambda**:
   ```bash
   cd lambda/upload_lambda
   npm install
   zip -r function.zip .
   aws lambda update-function-code \
     --function-name upload-lambda \
     --zip-file fileb://function.zip
   ```

2. **Deploy Frontend**:
   ```bash
   aws s3 cp html/assets/js/shared.js s3://your-bucket/assets/js/
   aws cloudfront create-invalidation \
     --distribution-id YOUR_DIST_ID \
     --paths "/assets/js/shared.js"
   ```

3. **Deploy Backend Handler** (if using Go Lambda):
   ```bash
   make build
   # Deploy according to your process
   ```

## Verification Checklist

- [ ] `/api/user` endpoint returns authenticated user email
- [ ] Frontend console shows correct user email
- [ ] `window.portal.currentUser` contains actual email
- [ ] Announcement modifications show correct user_id
- [ ] Change modifications show correct user_id
- [ ] Approval actions show correct user_id
- [ ] No more "current-user-id" or "demo.user@hearst.com" in production

## Benefits

1. **Accurate Audit Trail**: All modifications now track the actual user who performed the action
2. **Better User Experience**: Users see their own email in the UI
3. **Proper Email Routing**: Announcements go to the correct SES topics based on type
4. **Security**: User identity comes from SAML authentication headers
5. **Debugging**: Easier to trace who performed actions in logs and S3 files

## Notes

- The `/api/user` endpoint requires SAML authentication (x-user-email and x-authenticated headers)
- Falls back to demo mode only in development when SAML is not available
- All existing functionality remains backward compatible
