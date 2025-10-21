# Approvals Page - How It Works

## Overview

The Approvals page is designed for CCOE team members to review and approve changes submitted by users. It organizes changes by customer organization for efficient workflow management.

## Key Concepts

### 1. User Context Detection

When the page loads, it determines if you're an **Admin** or a **Customer User**:

- **Admin Users**: Can see ALL changes from ALL customers
- **Customer Users**: Can only see changes for their own organization

The system detects this by:

1. Checking the `/api/user/context` endpoint
2. Falling back to email domain analysis (e.g., `user@hts.hearst.com` → customer code 'hts')
3. Defaulting to admin for demo/development

### 2. Data Loading Process

```
1. Page loads → Detect user context
2. Fetch changes from S3:
   - Admin: Fetch ALL changes from all customer prefixes
   - Customer: Fetch only from their customer prefix
3. Filter by object_type = "change" (excludes announcements)
4. Group changes by customer organization
5. Apply status filters (pending, approved, etc.)
6. Render customer sections
```

### 3. Customer Grouping

Changes are organized into collapsible sections by customer:

```
▼ HTS Prod (3 pending)
  ├─ CHANGE-2025-001 - Security Updates [Pending]
  ├─ CHANGE-2025-002 - Database Migration [Pending]
  └─ CHANGE-2025-003 - Network Config [Approved]

▼ CDS Global (1 pending)
  └─ CHANGE-2025-004 - API Updates [Pending]

▶ FDBUS (0 pending)
```

Each section shows:

- Customer friendly name (e.g., "HTS Prod")
- Customer code (e.g., "hts")
- Count of pending changes
- Expand/collapse toggle

### 4. Filtering Options

**Status Filter:**

- **Pending** (default): Shows submitted changes awaiting approval
- **Approved**: Shows approved changes
- **All**: Shows all changes regardless of status
- **Completed**: Shows completed changes
- **Cancelled**: Shows cancelled changes

**Customer Filter:**

- **Admin users**: Dropdown with all customers + "All Customers" option
- **Customer users**: Locked to their organization only

**Date Range Filter:**

- Filter by submission date
- Options: Today, This Week, This Month, etc.

### 5. Change Cards

Each change displays:

```
┌─────────────────────────────────────────────┐
│ CHANGE-2025-001                             │
│ Security Baseline Updates                   │
│                                             │
│ Submitted: 2025-10-14 16:00                │
│ Status: Pending                             │
│ Created by: jane.smith@hearst.com          │
│                                             │
│ [View Details] [Approve] [Cancel]          │
└─────────────────────────────────────────────┘
```

### 6. Actions

**View Details:**

- Opens the enhanced change details modal
- Shows full information including:
  - Implementation plan
  - Schedule
  - Modification history timeline
  - Meeting information (if scheduled)
  - Approval status

**Approve:**

- Updates change status to "approved"
- Adds modification entry with approver info and timestamp
- Updates the S3 object
- Refreshes the view

**Cancel:**

- Updates change status to "cancelled"
- Adds modification entry
- Updates the S3 object
- Refreshes the view

### 7. Data Flow

```
User Action (Approve/Cancel)
    ↓
JavaScript function (handleApprovalAction/handleCancelAction)
    ↓
Fetch change object from S3
    ↓
Add modification entry:
    {
        timestamp: "2025-10-15T10:30:00Z",
        user_id: "approver-email",
        modification_type: "approved" or "cancelled"
    }
    ↓
Update change.status field
    ↓
Write updated object back to S3
    ↓
Refresh page view
```

## Technical Details

### S3 Data Structure

Changes are stored in S3 with this structure:

```
customers/
  ├─ hts/
  │   ├─ CHANGE-2025-001.json
  │   └─ CHANGE-2025-002.json
  ├─ cds/
  │   └─ CHANGE-2025-003.json
  └─ fdbus/
      └─ CHANGE-2025-004.json
```

### Change Object Structure

```json
{
  "object_type": "change",
  "changeId": "CHANGE-2025-001",
  "title": "Security Baseline Updates",
  "status": "submitted",
  "customers": ["hts"],
  "createdBy": "jane.smith@hearst.com",
  "modifications": [
    {
      "timestamp": "2025-10-14T09:00:00Z",
      "user_id": "jane.smith@hearst.com",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-10-14T16:00:00Z",
      "user_id": "jane.smith@hearst.com",
      "modification_type": "submitted"
    }
  ]
}
```

### Key JavaScript Classes/Functions

**ApprovalsPage class:**

- `init()`: Initialize page and load data
- `detectUserContext()`: Determine admin vs customer user
- `loadChanges()`: Fetch changes from S3
- `applyFilters()`: Filter changes by status/customer/date
- `groupByCustomer()`: Organize changes by customer
- `renderCustomerSection()`: Render each customer group
- `handleApprovalAction()`: Process approval
- `handleCancelAction()`: Process cancellation

**S3Client module (s3-client.js):**

- `fetchAllChanges()`: Get all changes (admin)
- `fetchCustomerChanges(customerCode)`: Get customer-specific changes
- `filterByObjectType(objects, type)`: Filter by object_type field
- `updateChange(changeId, updates)`: Update change in S3

## User Experience Flow

### For Admin Users

1. Open Approvals page
2. See banner: "Admin View: You can see changes for all customers"
3. See all customers with pending changes
4. Use filters to narrow down (status, customer, date)
5. Click on a customer section to expand
6. Review change details
7. Click "Approve" or "Cancel"
8. Change updates immediately

### For Customer Users

1. Open Approvals page
2. See banner: "Viewing changes for: HTS Prod"
3. See only their organization's changes
4. Customer filter is disabled (locked to their org)
5. Use status and date filters
6. Review and approve/cancel their changes

## Empty States

**No pending changes:**

```
┌─────────────────────────────────────────────┐
│              📋                             │
│                                             │
│     No Pending Approvals                    │
│                                             │
│  All changes have been reviewed!            │
└─────────────────────────────────────────────┘
```

**No changes for filter:**

```
┌─────────────────────────────────────────────┐
│              🔍                             │
│                                             │
│     No Changes Found                        │
│                                             │
│  Try adjusting your filters                 │
└─────────────────────────────────────────────┘
```

## Security Considerations

1. **Authentication**: Page requires user to be logged in via SAML
2. **Authorization**: Customer users can only see their own changes
3. **Audit Trail**: All approvals/cancellations are logged in modifications array
4. **Idempotency**: Approval actions are idempotent (safe to retry)

## Performance

- **Caching**: S3 responses cached for 5 minutes
- **Lazy Loading**: Customer sections load content on expand
- **Pagination**: Planned for large datasets (>100 changes)
- **Debouncing**: Filter inputs debounced to reduce re-renders

## Future Enhancements

- Bulk approve multiple changes at once
- Email notifications when changes are approved
- Approval comments/notes
- Approval delegation
- Advanced filtering (by priority, risk level, etc.)
