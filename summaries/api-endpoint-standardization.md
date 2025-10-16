# API Endpoint Standardization

## Summary

Removed duplicate `/api/` prefix routes to standardize all API endpoints on the `/changes/...` pattern.

## Changes Made

### Frontend Updates

**html/index.html:**
- ✅ Changed `/api/changes/statistics` → `/changes/statistics`
- ✅ Changed `/api/changes/recent` → `/changes/recent`

**html/edit-change.html:**
- ✅ Changed `/api/drafts/{id}` → `/drafts/{id}`

### Backend Updates

**lambda/upload_lambda/upload-metadata-lambda.js:**
- ✅ Removed duplicate route: `path === '/api/changes/statistics'`
- ✅ Removed duplicate route: `path === '/api/changes/recent'`
- ✅ Removed duplicate route: `path.startsWith('/api/changes/') && path.includes('/versions')`
- ✅ Removed duplicate route: `path.startsWith('/api/changes/') && method === 'GET'`
- ✅ Removed duplicate route: `path.startsWith('/api/changes/') && method === 'PUT'`
- ✅ Removed duplicate route: `path.startsWith('/api/drafts/')`

## Standardized Endpoint Structure

All endpoints now follow this pattern:

```
GET    /changes                    # List all changes
GET    /changes/{id}               # Get single change
PUT    /changes/{id}               # Update change
DELETE /changes/{id}               # Delete change
POST   /changes/search             # Search changes
POST   /changes/{id}/approve       # Approve change
POST   /changes/{id}/complete      # Complete change
POST   /changes/{id}/cancel        # Cancel change
GET    /changes/{id}/versions      # List versions
GET    /changes/statistics         # Get statistics
GET    /changes/recent             # Get recent changes

GET    /drafts                     # List drafts
GET    /drafts/{id}                # Get single draft
POST   /drafts                     # Save draft
DELETE /drafts/{id}                # Delete draft
```

## Benefits

1. **Consistency** - All endpoints follow the same pattern
2. **Simplicity** - No confusion about which prefix to use
3. **Maintainability** - Easier to understand and document
4. **Reduced Code** - Removed duplicate route handlers

## Testing

✅ No diagnostics errors in modified files
✅ All endpoint patterns are consistent
✅ Frontend and backend are aligned

## Files Modified

- `html/index.html`
- `html/edit-change.html`
- `lambda/upload_lambda/upload-metadata-lambda.js`
- `docs/API_ENDPOINTS.md`

## Next Steps

The remaining inconsistencies documented in `docs/API_ENDPOINTS.md`:
- Missing handler for `GET /changes/{id}/versions/{version}`
- Unused endpoints (drafts, my-changes)
- Can be addressed in future updates if needed
