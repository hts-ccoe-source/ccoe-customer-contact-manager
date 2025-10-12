# API Endpoints Reference

## Overview

This document lists all API endpoints exposed by the Node.js Upload Lambda (`lambda/upload_lambda/upload-metadata-lambda.js`).

## Authentication

All endpoints require SAML authentication via Lambda@Edge. The `userEmail` is extracted from the SAML assertion.

## Endpoint Categories

### 1. Upload Operations

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/upload` | `handleUpload` | Upload new change metadata |

### 2. Authentication

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/auth-check` | `handleAuthCheck` | Verify user authentication status |

### 3. Change Operations

#### Get Changes

| Method | Path | Handler | Description | Frontend Usage |
|--------|------|---------|-------------|----------------|
| GET | `/changes` | `handleGetChanges` | Get all changes | `search-changes.html`, `view-changes.html` |
| GET | `/changes/{id}` | `handleGetChange` | Get single change by ID | `my-changes.html`, `edit-change.html`, `create-change.html`, `view-changes.html` |
| GET | `/api/changes/{id}` | `handleGetChange` | Get single change by ID (API prefix) | `index.html` |
| GET | `/my-changes` | `handleGetMyChanges` | Get changes created by current user | Not used in frontend |
| GET | `/changes/recent` | `handleGetRecentChanges` | Get recent changes | Not used directly |
| GET | `/api/changes/recent` | `handleGetRecentChanges` | Get recent changes (API prefix) | `index.html` |
| GET | `/changes/statistics` | `handleGetStatistics` | Get change statistics | Not used directly |
| GET | `/api/changes/statistics` | `handleGetStatistics` | Get change statistics (API prefix) | `index.html` |

#### Modify Changes

| Method | Path | Handler | Description | Frontend Usage |
|--------|------|---------|-------------|----------------|
| PUT | `/changes/{id}` | `handleUpdateChange` | Update existing change | `edit-change.html` |
| PUT | `/api/changes/{id}` | `handleUpdateChange` | Update existing change (API prefix) | Not used |
| POST | `/changes/{id}/approve` | `handleApproveChange` | Approve a change | `my-changes.html` |
| POST | `/changes/{id}/complete` | `handleCompleteChange` | Complete a change | `my-changes.html` |
| POST | `/changes/{id}/cancel` | `handleCancelChange` | Cancel a change | `my-changes.html` |
| DELETE | `/changes/{id}` | `handleDeleteChange` | Delete a change | `my-changes.html` |

#### Search Changes

| Method | Path | Handler | Description | Frontend Usage |
|--------|------|---------|-------------|----------------|
| POST | `/changes/search` | `handleSearchChanges` | Search changes with criteria | `search-changes.html`, `create-change.html` |

#### Version History

| Method | Path | Handler | Description | Frontend Usage |
|--------|------|---------|-------------|----------------|
| GET | `/changes/{id}/versions` | `handleGetChangeVersions` | Get all versions of a change | `edit-change.html` |
| GET | `/api/changes/{id}/versions` | `handleGetChangeVersions` | Get all versions (API prefix) | Not used |
| GET | `/changes/{id}/versions/{version}` | Handler not found | Get specific version | `edit-change.html` (endpoint exists but handler missing?) |

### 4. Draft Operations

| Method | Path | Handler | Description | Frontend Usage |
|--------|------|---------|-------------|----------------|
| GET | `/drafts` | `handleGetDrafts` | Get all drafts for current user | Not used in frontend |
| GET | `/drafts/{id}` | `handleGetDraft` | Get single draft by ID | Not used directly |
| GET | `/api/drafts/{id}` | `handleGetDraft` | Get single draft by ID (API prefix) | Not used |
| POST | `/drafts` | `handleSaveDraft` | Save a draft | Not used in frontend |
| DELETE | `/drafts/{id}` | `handleDeleteDraft` | Delete a draft | Not used in frontend |

## Endpoint Inconsistencies Found

### 1. ✅ FIXED: Duplicate Routes with `/api` Prefix

**Issue:** Some endpoints had both `/changes/...` and `/api/changes/...` versions

**Resolution:**
- ✅ Removed all `/api/` prefix routes from Lambda handler
- ✅ Updated `index.html` to use `/changes/statistics` and `/changes/recent`
- ✅ Updated `edit-change.html` to use `/drafts/{id}` instead of `/api/drafts/{id}`

**All endpoints now use standard `/changes/...` pattern without `/api/` prefix**

### 2. Missing Handler for Version Retrieval

**Issue:** Frontend calls `GET /changes/{id}/versions/{version}` but no handler exists

**Frontend Usage:**
```javascript
// edit-change.html line 1148
const response = await fetch(`${portal.baseUrl}/changes/${this.changeId}/versions/${versionNumber}?t=${Date.now()}`
```

**Current Handlers:**
- ✅ `GET /changes/{id}/versions` - Lists all versions
- ❌ `GET /changes/{id}/versions/{version}` - **Missing handler**

**Recommendation:** Add handler for retrieving specific version:
```javascript
} else if (path.startsWith('/changes/') && path.match(/\/versions\/\d+$/) && method === 'GET') {
    return await handleGetChangeVersion(event, userEmail);
```

### 3. Unused Endpoints

**Issue:** Some endpoints have handlers but are not used by any frontend page

**Unused Endpoints:**
- `GET /my-changes` - Handler exists but frontend doesn't use it
- `GET /drafts` - Handler exists but frontend doesn't use it
- `POST /drafts` - Handler exists but frontend doesn't use it
- `DELETE /drafts/{id}` - Handler exists but frontend doesn't use it

**Recommendation:** Either:
- Remove unused handlers to reduce code complexity
- Document why they exist (future use, API completeness, etc.)

### 4. Inconsistent Base URL Usage

**Issue:** Frontend uses different base URL patterns

**Patterns Found:**
- `${portal.baseUrl}/changes/...` - Most pages
- `${window.location.origin}/changes/...` - `my-changes.html`
- `${portal.baseUrl}/api/changes/...` - `index.html`

**Recommendation:** Standardize on `${portal.baseUrl}/changes/...` for consistency

### 5. Status Change Endpoints Pattern

**Current Pattern:**
- `POST /changes/{id}/approve`
- `POST /changes/{id}/complete`
- `POST /changes/{id}/cancel`

**Observation:** These are consistent and follow RESTful conventions for actions

**No changes needed** - This pattern is good

## Recommended Endpoint Structure

### Standardized Endpoints

```
Authentication:
  GET  /auth-check

Upload:
  POST /upload

Changes:
  GET    /changes                    # List all changes
  GET    /changes/{id}               # Get single change
  PUT    /changes/{id}               # Update change
  DELETE /changes/{id}               # Delete change
  POST   /changes/search             # Search changes
  
Change Actions:
  POST   /changes/{id}/approve       # Approve change
  POST   /changes/{id}/complete      # Complete change
  POST   /changes/{id}/cancel        # Cancel change

Change Versions:
  GET    /changes/{id}/versions      # List all versions
  GET    /changes/{id}/versions/{n}  # Get specific version

Statistics:
  GET    /changes/statistics         # Get statistics
  GET    /changes/recent             # Get recent changes

Drafts:
  GET    /drafts                     # List user's drafts
  GET    /drafts/{id}                # Get single draft
  POST   /drafts                     # Save draft
  DELETE /drafts/{id}                # Delete draft

User-Specific:
  GET    /my-changes                 # Get user's changes
```

## Migration Plan

### Phase 1: Add Missing Handler
1. Add handler for `GET /changes/{id}/versions/{version}`
2. Test with `edit-change.html`

### Phase 2: Standardize Base URLs
1. Update `my-changes.html` to use `portal.baseUrl` instead of `window.location.origin`
2. Update `index.html` to use `/changes/...` instead of `/api/changes/...`

### Phase 3: Remove Duplicate Routes (Optional)
1. Remove `/api` prefix routes from Lambda handler
2. Update any remaining frontend code using `/api` prefix
3. Test all pages

### Phase 4: Clean Up Unused Endpoints (Optional)
1. Document why unused endpoints exist
2. If no future use planned, remove handlers
3. Update documentation

## Testing Checklist

After any endpoint changes:

- [ ] Test `index.html` - Dashboard statistics and recent changes
- [ ] Test `create-change.html` - Create new change, search duplicates
- [ ] Test `edit-change.html` - Edit change, view versions
- [ ] Test `my-changes.html` - List changes, approve, complete, cancel, delete
- [ ] Test `view-changes.html` - View all changes
- [ ] Test `search-changes.html` - Search functionality
- [ ] Test authentication flow
- [ ] Test error handling for 404, 403, 500 responses

## References

- Lambda Handler: `lambda/upload_lambda/upload-metadata-lambda.js`
- Frontend Pages: `html/*.html`
- State Machine: `docs/CHANGE_WORKFLOW_STATE_MACHINE.md`
