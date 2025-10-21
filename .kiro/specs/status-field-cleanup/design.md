# Design Document

## Overview

This design eliminates the confusing `metadata` map from S3 objects and establishes a single source of truth for status using only top-level fields. The design ensures **identical** behavior between changes and announcements - both use separate types (`ChangeMetadata` and `AnnouncementMetadata`), the same status workflow, and the same status determination logic. The only differences are their specific content fields (e.g., `change_title` vs `title`) and email topics (e.g., `aws-approval` vs `cic-announce`).

## Architecture

### Current State (Problem)

```json
{
  "status": "approved",
  "source": "",
  "metadata": {
    "request_type": "approval_request",
    "status": "submitted"
  }
}
```

**Issues:**
- Three different status-related fields can conflict
- `DetermineRequestType()` checks `metadata.request_type` first, returning stale values
- Backend sends wrong emails (approval request instead of approved announcement)
- Confusion about which field is authoritative

### Target State (Solution)

```json
{
  "status": "approved",
  "prior_status": "submitted"
}
```

**Benefits:**
- Single authoritative status field
- Optional prior_status for tracking transitions
- No nested metadata map to cause confusion
- Simple, predictable behavior

## Components and Interfaces

### 1. Status Determination Logic

**Location:** `internal/lambda/handlers.go`

**New Implementation - Single Function for Both Types:**
```go
// DetermineRequestTypeFromStatus determines request type from status field only
// Works for both ChangeMetadata and AnnouncementMetadata
func DetermineRequestTypeFromStatus(status string) string {
    // ONLY check status parameter (no nested fields, no metadata map)
    switch status {
    case "submitted":
        return "approval_request"
    case "approved":
        return "approved_announcement"
    case "completed":
        return "change_complete"
    case "cancelled":
        return "change_cancelled"
    default:
        log.Printf("⚠️  Unknown status: %s", status)
        return "unknown"
    }
}

// For changes
func DetermineRequestType(metadata *types.ChangeMetadata) string {
    return DetermineRequestTypeFromStatus(metadata.Status)
}

// For announcements
func DetermineAnnouncementRequestType(metadata *types.AnnouncementMetadata) string {
    return DetermineRequestTypeFromStatus(metadata.Status)
}
```

### 2. Type Definitions

**Location:** `internal/types/types.go`

**Changes use ChangeMetadata:**
```go
type ChangeMetadata struct {
    ChangeID    string `json:"change_id"`
    ObjectType  string `json:"object_type"` // "change"
    Status      string `json:"status"`
    PriorStatus string `json:"prior_status"` // Required field
    
    // Change-specific fields
    ChangeTitle         string `json:"change_title"`
    ChangeReason        string `json:"change_reason"`
    ImplementationPlan  string `json:"implementation_plan"`
    // ... other change fields
    
    // Meeting field (unified name)
    IncludeMeeting bool `json:"include_meeting"` // Replaces meeting_required
    
    // Remove: Source field (unused)
    // Remove: Metadata map (causes confusion)
    // Remove: meeting_required (replaced by include_meeting)
}
```

**Announcements use AnnouncementMetadata:**
```go
type AnnouncementMetadata struct {
    AnnouncementID   string `json:"announcement_id"`
    ObjectType       string `json:"object_type"` // "announcement_cic", etc.
    AnnouncementType string `json:"announcement_type"` // "cic", "finops", etc.
    Status           string `json:"status"`
    PriorStatus      string `json:"prior_status"` // Required field
    
    // Announcement-specific fields
    Title   string `json:"title"`
    Summary string `json:"summary"`
    Content string `json:"content"`
    // ... other announcement fields
    
    // Meeting field (unified name)
    IncludeMeeting bool `json:"include_meeting"` // Already uses this name
    
    // Remove: Source field (unused)
    // Remove: Metadata map (causes confusion)
}
```

**Shared Status Workflow:**
Both types use the same status values and transitions:
- `draft` → `submitted` → `approved` → `completed`/`cancelled`

### 3. Frontend API Lambda

**Location:** `lambda/upload_lambda/upload-metadata-lambda.js`

**Changes Needed:**
- Remove any code that writes to `metadata.status`
- Remove any code that writes to `metadata.request_type`
- Remove any code that reads from `metadata` map for status determination
- Add `prior_status` field when status changes
- Remove the entire `metadata` map from objects before writing to S3

### 4. Backend Lambda

**Location:** `internal/lambda/handlers.go`

**Changes Needed:**
- Simplify `DetermineRequestType()` to only check `metadata.Status`
- Remove all checks for `metadata.Metadata["request_type"]`
- Remove all checks for `metadata.Metadata["status"]`
- Remove all checks for `metadata.Source`
- Add validation to log error if `metadata.Metadata` exists

## Data Models

### Change Object Structure

```json
{
  "change_id": "CHG-xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx",
  "object_type": "change",
  "status": "approved",
  "prior_status": "submitted",
  
  "change_title": "Example Change",
  "change_reason": "Reason for change",
  "implementation_plan": "Plan details",
  "test_plan": "Test details",
  "customer_impact": "Impact description",
  "rollback_plan": "Rollback details",
  
  "implementation_start": "2025-10-18T10:00:00Z",
  "implementation_end": "2025-10-18T12:00:00Z",
  "timezone": "America/New_York",
  
  "customers": ["customer1", "customer2"],
  
  "include_meeting": true,
  "meeting_title": "Change Implementation Meeting",
  "meeting_date": "2025-10-18T10:00:00Z",
  "meeting_duration": "60",
  "meeting_location": "Microsoft Teams",
  "meeting_id": "AAMkAD...",
  "join_url": "https://teams.microsoft.com/...",
  
  "created_at": "2025-10-18T09:00:00Z",
  "created_by": "user@example.com",
  "modified_at": "2025-10-18T09:30:00Z",
  "modified_by": "user@example.com",
  "submitted_at": "2025-10-18T09:30:00Z",
  "submitted_by": "user@example.com",
  "approved_at": "2025-10-18T09:45:00Z",
  "approved_by": "approver@example.com",
  
  "modifications": [
    {
      "timestamp": "2025-10-18T09:00:00Z",
      "user_id": "user@example.com",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-10-18T09:30:00Z",
      "user_id": "user@example.com",
      "modification_type": "submitted"
    },
    {
      "timestamp": "2025-10-18T09:45:00Z",
      "user_id": "approver@example.com",
      "modification_type": "approved"
    }
  ]
}
```

### Announcement Object Structure

```json
{
  "announcement_id": "CIC-xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx",
  "object_type": "announcement_cic",
  "announcement_type": "cic",
  "status": "approved",
  "prior_status": "submitted",
  
  "title": "Example Announcement",
  "summary": "Brief summary",
  "content": "Full content",
  
  "customers": ["customer1", "customer2"],
  
  "include_meeting": true,
  "meeting_title": "CIC Announcement Meeting",
  "meeting_date": "2025-10-18T10:00:00Z",
  "meeting_duration": "30",
  "meeting_location": "Microsoft Teams",
  "attendees": "user1@example.com,user2@example.com",
  "meeting_id": "AAMkAD...",
  "join_url": "https://teams.microsoft.com/...",
  
  "created_at": "2025-10-18T09:00:00Z",
  "created_by": "user@example.com",
  "modified_at": "2025-10-18T09:30:00Z",
  "modified_by": "user@example.com",
  "submitted_at": "2025-10-18T09:30:00Z",
  "submitted_by": "user@example.com",
  "approved_at": "2025-10-18T09:45:00Z",
  "approved_by": "approver@example.com",
  
  "modifications": [
    {
      "timestamp": "2025-10-18T09:00:00Z",
      "user_id": "user@example.com",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-10-18T09:30:00Z",
      "user_id": "user@example.com",
      "modification_type": "submitted"
    },
    {
      "timestamp": "2025-10-18T09:45:00Z",
      "user_id": "approver@example.com",
      "modification_type": "approved"
    }
  ]
}
```

### Key Differences Between Changes and Announcements

**Changes:**
- Use `change_id` and `change_*` fields
- Have detailed implementation fields (plan, test, rollback)
- Use `meeting_required` field

**Announcements:**
- Use `announcement_id` and `announcement_type` fields
- Have content fields (title, summary, content)
- Use `include_meeting` field
- Have `attendees` field

**Shared:**
- Same status workflow (draft → submitted → approved → completed/cancelled)
- Same meeting fields (meeting_id, join_url, meeting_title, etc.)
- Same timestamp fields (created_at, modified_at, submitted_at, approved_at)
- Same modifications array structure

## Error Handling

### Validation Errors

**Scenario:** Object contains legacy `metadata` map

**Handling:**
```go
if metadata.Metadata != nil && len(metadata.Metadata) > 0 {
    log.Printf("❌ ERROR: Object %s contains legacy metadata map - migration required", metadata.ChangeID)
    return fmt.Errorf("object contains legacy metadata map")
}
```

**Action:** Log error and fail processing. Operator must delete old objects.

### Unknown Status

**Scenario:** Status field contains unrecognized value

**Handling:**
```go
default:
    log.Printf("⚠️  Unknown status: %s for object %s", metadata.Status, metadata.ChangeID)
    return "unknown"
```

**Action:** Log warning and return "unknown" request type. No email sent.

## Testing Strategy

### Unit Tests

**Test: DetermineRequestType with valid statuses**
```go
func TestDetermineRequestType_ValidStatuses(t *testing.T) {
    tests := []struct {
        status   string
        expected string
    }{
        {"submitted", "approval_request"},
        {"approved", "approved_announcement"},
        {"completed", "change_complete"},
        {"cancelled", "change_cancelled"},
    }
    
    for _, tt := range tests {
        metadata := &types.ChangeMetadata{Status: tt.status}
        result := DetermineRequestType(metadata)
        if result != tt.expected {
            t.Errorf("Status %s: expected %s, got %s", tt.status, tt.expected, result)
        }
    }
}
```

**Test: DetermineRequestType ignores legacy metadata**
```go
func TestDetermineRequestType_IgnoresLegacyMetadata(t *testing.T) {
    metadata := &types.ChangeMetadata{
        Status: "approved",
        Metadata: map[string]interface{}{
            "request_type": "approval_request", // Stale value
            "status": "submitted",              // Stale value
        },
    }
    
    result := DetermineRequestType(metadata)
    if result != "approved_announcement" {
        t.Errorf("Expected approved_announcement, got %s", result)
    }
}
```

**Test: Validation detects legacy metadata**
```go
func TestValidation_DetectsLegacyMetadata(t *testing.T) {
    metadata := &types.ChangeMetadata{
        ChangeID: "CHG-123",
        Status: "approved",
        Metadata: map[string]interface{}{
            "request_type": "approval_request",
        },
    }
    
    err := ValidateMetadata(metadata)
    if err == nil {
        t.Error("Expected error for legacy metadata, got nil")
    }
}
```

### Integration Tests

**Test: Submit → Approve workflow**
1. Create change with status "draft"
2. Submit change (status → "submitted", prior_status → "draft")
3. Verify approval request email sent
4. Approve change (status → "approved", prior_status → "submitted")
5. Verify approved announcement email sent
6. Verify meeting scheduled (if required)

**Test: Approve → Cancel workflow**
1. Create approved change with meeting
2. Cancel change (status → "cancelled", prior_status → "approved")
3. Verify meeting cancelled
4. Verify cancellation email sent

## Migration Plan

### Step 1: Delete All Existing Objects

**Action:** Delete all objects from S3 buckets
- `archive/` - All changes and announcements
- `customers/` - All trigger files
- `drafts/` - All draft objects

**Command:**
```bash
aws s3 rm s3://4cm-prod-ccoe-change-management-metadata/archive/ --recursive
aws s3 rm s3://4cm-prod-ccoe-change-management-metadata/customers/ --recursive
aws s3 rm s3://4cm-prod-ccoe-change-management-metadata/drafts/ --recursive
```

### Step 2: Deploy Backend Changes

**Files to Update:**
- `internal/types/types.go` - Remove Metadata field, add PriorStatus
- `internal/lambda/handlers.go` - Simplify DetermineRequestType()
- Add validation to detect legacy metadata

**Deploy:**
```bash
make build
make deploy
```

### Step 3: Deploy Frontend Changes

**Files to Update:**
- `lambda/upload_lambda/upload-metadata-lambda.js` - Remove metadata map writes
- Add prior_status field when status changes

**Deploy:**
```bash
cd lambda/upload_lambda
npm install
./deploy.sh
```

### Step 4: Verify

**Test:**
1. Create new change
2. Submit for approval
3. Approve change
4. Verify correct emails sent
5. Verify meeting scheduled (if required)
6. Cancel change
7. Verify meeting cancelled
8. Verify cancellation email sent

## Design Decisions

### Why Remove metadata Map Entirely?

**Problem:** Nested maps cause confusion and bugs

**Solution:** Use only top-level fields

**Benefits:**
- Single source of truth for status
- No conflicting values
- Simpler code
- Easier to understand
- Consistent behavior

### Why Add prior_status Field?

**Use Case:** Track status transitions for audit/debugging

**Implementation:** Required top-level field

**Example:**
```json
{
  "status": "approved",
  "prior_status": "submitted"
}
```

**Special Case:** For newly created objects, set `prior_status` to empty string `""`

**Benefits:**
- Simple status transition tracking
- No nested structure
- Easy to query
- Always present (required field)

### Why No Backward Compatibility?

**Reason:** Clean slate approach

**Benefits:**
- Simpler implementation
- No legacy code paths
- Faster development
- Cleaner codebase

**Trade-off:** Must delete all existing objects

**Mitigation:** System is in development, no production data to preserve

## References

- State Machine: `docs/CHANGE_WORKFLOW_STATE_MACHINE.md`
- Backend Types: `internal/types/types.go`
- Backend Handlers: `internal/lambda/handlers.go`
- Frontend API: `lambda/upload_lambda/upload-metadata-lambda.js`
