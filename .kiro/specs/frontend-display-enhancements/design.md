# Design Document

## Overview

This design document outlines the frontend enhancements for the CCOE Customer Contact Manager, focusing on improving the user experience for viewing changes, managing approvals, and accessing announcements. The design builds upon the existing HTML/CSS/JavaScript architecture and integrates with the enhanced object model defined in the `object-model-enhancement` spec.

### Key Design Goals

1. **Unified User Experience**: Create consistent, intuitive interfaces across all pages
2. **Customer-Centric Views**: Organize data by customer for efficient review and management
3. **Performance**: Ensure responsive loading and interaction even with large datasets
4. **Maintainability**: Use modular JavaScript and shared CSS for easy updates
5. **Accessibility**: Support keyboard navigation, screen readers, and responsive design

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Browser (Frontend)                        │
├─────────────────────────────────────────────────────────────┤
│  Navigation Layer                                            │
│  ├─ index.html (Dashboard)                                   │
│  ├─ my-changes.html (Enhanced)                               │
│  ├─ approvals.html (NEW)                                     │
│  └─ announcements.html (NEW)                                 │
├─────────────────────────────────────────────────────────────┤
│  Presentation Layer                                          │
│  ├─ Enhanced Change Details Modal                            │
│  ├─ Customer-Grouped Approvals View                          │
│  └─ Type-Filtered Announcements View                         │
├─────────────────────────────────────────────────────────────┤
│  Data Access Layer (JavaScript Modules)                      │
│  ├─ s3-client.js (S3 data fetching)                          │
│  ├─ change-service.js (Change operations)                    │
│  ├─ approval-service.js (Approval operations)                │
│  └─ announcement-service.js (Announcement operations)        │
├─────────────────────────────────────────────────────────────┤
│  Shared Components                                           │
│  ├─ modal.js (Reusable modal component)                      │
│  ├─ timeline.js (Modification history timeline)              │
│  ├─ filters.js (Filtering and sorting utilities)            │
│  └─ loading.js (Loading states and indicators)              │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    AWS S3 Storage                            │
├─────────────────────────────────────────────────────────────┤
│  customers/{customer-code}/                                  │
│  ├─ {change-id}.json (object_type: "change")                │
│  └─ {announcement-id}.json (object_type: "announcement_*")   │
│                                                              │
│  archive/                                                    │
│  └─ {timestamp}-{object-id}.json                             │
│                                                              │
│  Note: Objects are differentiated by object_type field,     │
│        not by S3 key prefix structure                        │
└─────────────────────────────────────────────────────────────┘
```

### Technology Stack

- **Frontend Framework**: Vanilla JavaScript (ES6+) with modular architecture
- **Styling**: CSS3 with existing shared.css design system
- **Data Storage**: AWS S3 (existing infrastructure)
- **Authentication**: SAML integrated with AWS Identity Center (existing)
- **Build Tools**: None required (static HTML/CSS/JS)

## Components and Interfaces

### 1. Enhanced Change Details Modal

#### Purpose
Replace the current simple pop-up with a comprehensive modal that displays all change information including modification history, meeting metadata, and approval status.

#### Component Structure

```javascript
// change-details-modal.js
class ChangeDetailsModal {
    constructor(changeData) {
        this.changeData = changeData;
        this.modalElement = null;
    }

    render() {
        // Create modal structure
        // Display sections: Header, Details, Timeline, Meetings, Approvals
    }

    renderTimeline() {
        // Render modification history as visual timeline
        // Group by type, show icons, format timestamps
    }

    renderMeetingInfo() {
        // Display meeting metadata if available
        // Show join links, schedule, attendees
    }

    renderApprovalStatus() {
        // Display approval information
        // Show approvers, timestamps, status
    }

    show() {
        // Display modal with animation
    }

    hide() {
        // Hide modal and cleanup
    }
}
```

#### Visual Design

```
┌────────────────────────────────────────────────────────────┐
│  Change Details                                        [X]  │
├────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────┐  │
│  │  CHANGE-2025-001                                     │  │
│  │  Implement Security Baseline Updates                 │  │
│  │  Status: Approved  │  Customer: HTS Prod            │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  📋 Details                                                │
│  ├─ Implementation Plan: [scrollable text]                │
│  ├─ Schedule: 2025-10-20 02:00 - 04:00 UTC               │
│  └─ Affected Systems: [list]                              │
│                                                            │
│  📅 Meeting Information                                    │
│  ├─ Join URL: [clickable link]                            │
│  ├─ Start: 2025-10-19 15:00 EDT                           │
│  └─ Duration: 1 hour                                       │
│                                                            │
│  ✅ Approval Status                                        │
│  ├─ Approved by: John Doe                                 │
│  ├─ Approved at: 2025-10-15 10:30 EDT                     │
│  └─ Comments: Looks good to proceed                       │
│                                                            │
│  📊 Modification History                                   │
│  ├─ ● Created by Jane Smith (2025-10-14 09:00)           │
│  ├─ ● Updated by Jane Smith (2025-10-14 14:30)           │
│  ├─ ● Submitted by Jane Smith (2025-10-14 16:00)         │
│  ├─ ● Meeting Scheduled (2025-10-15 09:00)               │
│  └─ ● Approved by John Doe (2025-10-15 10:30)            │
└────────────────────────────────────────────────────────────┘
```

### 2. Approvals Page (approvals.html)

#### Purpose
Provide a dedicated page for reviewing and approving changes, organized by customer for efficient workflow.

#### Component Structure

```javascript
// approvals-page.js
class ApprovalsPage {
    constructor() {
        this.changes = [];
        this.customers = new Map();
        this.filters = {
            status: 'pending', // 'pending', 'approved', 'all'
            customer: 'all',
            dateRange: null
        };
    }

    async loadChanges() {
        // Fetch changes from S3
        // Group by customer
        // Apply filters
    }

    groupByCustomer() {
        // Organize changes by customer code
        // Sort customers alphabetically
    }

    renderCustomerSection(customerCode, changes) {
        // Render a collapsible section for each customer
        // Show customer name, change count, status summary
    }

    renderChangeCard(change) {
        // Render individual change card
        // Show title, status, date, quick actions
    }

    applyFilters() {
        // Filter changes based on current filter state
        // Re-render view
    }

    handleApprovalAction(changeId) {
        // Handle approval button click
        // Update change object
        // Refresh view
    }
}
```

#### Visual Design

```
┌────────────────────────────────────────────────────────────┐
│  Approvals                                                  │
├────────────────────────────────────────────────────────────┤
│  Filters: [Pending ▼] [All Customers ▼] [Date Range]      │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ▼ HTS Prod (3 pending)                                    │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ CHANGE-2025-001 - Security Baseline Updates          │  │
│  │ Submitted: 2025-10-14 16:00  │  Status: Pending      │  │
│  │ [View Details] [Approve] [Cancel]                     │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ CHANGE-2025-002 - Database Migration                 │  │
│  │ Submitted: 2025-10-13 10:00  │  Status: Pending      │  │
│  │ [View Details] [Approve] [Cancel]                     │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  ▼ CDS Global (1 pending)                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ CHANGE-2025-003 - Network Configuration              │  │
│  │ Submitted: 2025-10-15 08:00  │  Status: Pending      │  │
│  │ [View Details] [Approve] [Cancel]                     │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  ▶ FDBUS (0 pending)                                       │
└────────────────────────────────────────────────────────────┘
```

### 3. Announcements Page (announcements.html)

#### Purpose
Centralized location for viewing various types of announcements including FinOps reports, InnerSourcing Guild updates, and CIC/Cloud Enablement communications.

#### Component Structure

```javascript
// announcements-page.js
class AnnouncementsPage {
    constructor() {
        this.announcements = [];
        this.filters = {
            type: 'all', // 'finops', 'innersourcing', 'cic', 'general', 'all'
            dateRange: null
        };
    }

    async loadAnnouncements() {
        // Fetch all objects from S3 customer prefixes
        // Filter by object_type (announcement_*)
        // Sort by date
    }

    renderAnnouncementCard(announcement) {
        // Render announcement card
        // Show type icon, title, date, summary
    }

    renderAnnouncementDetails(announcement) {
        // Show full announcement in modal
        // Include attachments, links, full content
    }

    applyFilters() {
        // Filter announcements by type and date
        // Re-render view
    }

    getTypeIcon(type) {
        // Return appropriate icon for announcement type
    }
}
```

#### Visual Design

```
┌────────────────────────────────────────────────────────────┐
│  Announcements                                              │
├────────────────────────────────────────────────────────────┤
│  Filters: [All Types ▼] [Sort: Newest ▼]                  │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  💰 FinOps Monthly Report - October 2025                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Posted: 2025-10-01                                    │  │
│  │ Summary: Monthly cost optimization report showing...  │  │
│  │ [Read More]                                           │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  🔧 InnerSourcing Guild - New Project Showcase            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Posted: 2025-09-28                                    │  │
│  │ Summary: Check out the latest internal projects...   │  │
│  │ [Read More]                                           │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  ☁️ CIC Cloud Enablement - AWS Best Practices             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Posted: 2025-09-25                                    │  │
│  │ Summary: Updated guidelines for AWS resource...      │  │
│  │ [Read More]                                           │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

### 4. Navigation Enhancement

#### Updated Navigation Structure

```html
<nav class="nav-menu">
    <div class="nav-item">
        <a href="index.html" class="nav-link">Dashboard</a>
    </div>
    <div class="nav-item">
        <a href="create-change.html" class="nav-link">Create Change</a>
    </div>
    <div class="nav-item">
        <a href="my-changes.html" class="nav-link">My Changes</a>
    </div>
    <div class="nav-item">
        <a href="approvals.html" class="nav-link">Approvals</a>
    </div>
    <div class="nav-item">
        <a href="announcements.html" class="nav-link">Announcements</a>
    </div>
    <div class="nav-item">
        <a href="search-changes.html" class="nav-link">Search</a>
    </div>
</nav>
```

## Data Models

### Enhanced Change Object

```json
{
  "object_type": "change",
  "change_id": "CHANGE-2025-001",
  "title": "Security Baseline Updates",
  "description": "Implement new security baseline configurations",
  "implementation_plan": "Detailed implementation steps...",
  "schedule": {
    "start_time": "2025-10-20T02:00:00Z",
    "end_time": "2025-10-20T04:00:00Z",
    "timezone": "UTC"
  },
  "affected_customers": ["hts", "cds"],
  "status": "approved",
  "created_by": "user-id-123",
  "modifications": [
    {
      "timestamp": "2025-10-14T09:00:00Z",
      "user_id": "user-id-123",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-10-14T14:30:00Z",
      "user_id": "user-id-123",
      "modification_type": "updated"
    },
    {
      "timestamp": "2025-10-14T16:00:00Z",
      "user_id": "user-id-123",
      "modification_type": "submitted"
    },
    {
      "timestamp": "2025-10-15T09:00:00Z",
      "user_id": "system",
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGI2...",
        "join_url": "https://teams.microsoft.com/l/meetup/...",
        "start_time": "2025-10-19T19:00:00Z",
        "end_time": "2025-10-19T20:00:00Z"
      }
    },
    {
      "timestamp": "2025-10-15T10:30:00Z",
      "user_id": "user-id-456",
      "modification_type": "approved"
    }
  ]
}
```

### Announcement Object

```json
{
  "object_type": "announcement_finops",
  "announcement_id": "ANNOUNCE-2025-10-001",
  "title": "FinOps Monthly Report - October 2025",
  "summary": "Monthly cost optimization report showing significant savings...",
  "content": "Full announcement content in markdown or HTML...",
  "posted_date": "2025-10-01T00:00:00Z",
  "author": "FinOps Team",
  "tags": ["finops", "cost-optimization", "monthly-report"],
  "attachments": [
    {
      "name": "October_2025_Report.pdf",
      "url": "https://s3.amazonaws.com/..."
    }
  ],
  "links": [
    {
      "text": "View Dashboard",
      "url": "https://finops.example.com/dashboard"
    }
  ]
}
```

### Object Type Enumeration

```javascript
const OBJECT_TYPES = {
  CHANGE: 'change',
  ANNOUNCEMENT_FINOPS: 'announcement_finops',
  ANNOUNCEMENT_INNERSOURCING: 'announcement_innersourcing',
  ANNOUNCEMENT_CIC: 'announcement_cic',
  ANNOUNCEMENT_GENERAL: 'announcement_general'
};

const MODIFICATION_TYPES = {
  CREATED: 'created',
  UPDATED: 'updated',
  SUBMITTED: 'submitted',
  APPROVED: 'approved',
  DELETED: 'deleted',
  MEETING_SCHEDULED: 'meeting_scheduled',
  MEETING_CANCELLED: 'meeting_cancelled'
};
```

## Error Handling

### Error Handling Strategy

1. **Network Errors**: Retry with exponential backoff (3 attempts)
2. **S3 Access Errors**: Display user-friendly message, log details
3. **Data Validation Errors**: Show specific field errors, prevent submission
4. **Missing Data**: Gracefully handle missing fields, show placeholders
5. **Authentication Errors**: Redirect to login, preserve state

### Error Display Component

```javascript
class ErrorHandler {
    static showError(message, type = 'error') {
        // Display toast notification
        // Types: 'error', 'warning', 'info', 'success'
    }

    static handleS3Error(error) {
        // Parse S3 error
        // Show appropriate message
        // Log for debugging
    }

    static handleValidationError(errors) {
        // Display field-specific errors
        // Highlight invalid fields
    }
}
```

## Testing Strategy

### Unit Testing

- **JavaScript Modules**: Test individual functions and classes
- **Data Transformations**: Test object parsing and formatting
- **Filtering Logic**: Test filter and sort operations
- **Validation**: Test input validation rules

### Integration Testing

- **S3 Integration**: Test data fetching and writing
- **Modal Interactions**: Test modal open/close, data display
- **Navigation**: Test page transitions and state preservation
- **Filter Interactions**: Test filter combinations

### End-to-End Testing

- **User Workflows**: Test complete user journeys
  - View change → Open details → Approve
  - Filter announcements → Read announcement
  - Navigate between pages → Verify state
- **Cross-Browser**: Test on Chrome, Firefox, Safari, Edge
- **Responsive Design**: Test on mobile, tablet, desktop

### Manual Testing Checklist

```markdown
## Enhanced Change Details Modal
- [ ] Modal opens with correct data
- [ ] Timeline displays all modifications
- [ ] Meeting info shows when available
- [ ] Approval status displays correctly
- [ ] Modal closes properly
- [ ] Keyboard navigation works (ESC to close)

## Approvals Page
- [ ] Changes load and group by customer
- [ ] Filters work correctly
- [ ] Approval action updates status
- [ ] Empty states display properly
- [ ] Pagination works for large datasets

## Announcements Page
- [ ] Announcements load and display
- [ ] Type filtering works
- [ ] Date sorting works
- [ ] Full announcement modal opens
- [ ] Links and attachments are clickable

## Navigation
- [ ] All nav links work
- [ ] Active page is highlighted
- [ ] Mobile menu works
- [ ] Browser back/forward works

## Responsive Design
- [ ] Desktop layout (1200px+)
- [ ] Tablet layout (768px-1199px)
- [ ] Mobile layout (<768px)

## Accessibility
- [ ] Keyboard navigation
- [ ] Screen reader compatibility
- [ ] Color contrast meets WCAG AA
- [ ] Focus indicators visible
```

## Performance Considerations

### Optimization Strategies

1. **Lazy Loading**: Load data only when needed
2. **Pagination**: Limit items per page (20-50)
3. **Caching**: Cache S3 responses for 5 minutes
4. **Debouncing**: Debounce filter inputs (300ms)
5. **Virtual Scrolling**: For very long lists (>100 items)

### Performance Targets

- **Initial Page Load**: < 2 seconds
- **Data Fetch**: < 1 second
- **Filter/Sort**: < 500ms
- **Modal Open**: < 200ms
- **Navigation**: < 100ms

### Monitoring

```javascript
class PerformanceMonitor {
    static measurePageLoad() {
        // Use Performance API
        // Log timing metrics
    }

    static measureDataFetch(operation) {
        // Track S3 fetch times
        // Alert if > 2 seconds
    }

    static measureInteraction(action) {
        // Track user interactions
        // Identify slow operations
    }
}
```

## Security Considerations

### Frontend Security

1. **Input Sanitization**: Sanitize all user inputs before display
2. **XSS Prevention**: Use textContent instead of innerHTML where possible
3. **CORS**: Ensure proper CORS configuration for S3
4. **Authentication**: Verify user authentication on page load
5. **Authorization**: Check user permissions before showing actions

### Data Access Control

```javascript
class SecurityService {
    static async checkPermissions(action, resource) {
        // Verify user can perform action on resource
        // Return true/false
    }

    static sanitizeInput(input) {
        // Remove potentially dangerous characters
        // Escape HTML entities
    }

    static validateObjectType(obj) {
        // Ensure object_type is valid
        // Prevent injection attacks
    }
}
```

## Migration Strategy

### Phase 1: Preparation
1. Add object_type field to existing change objects (migration script)
2. Create new HTML files (approvals.html, announcements.html)
3. Develop shared JavaScript modules
4. Update navigation in all existing pages

### Phase 2: Enhancement
1. Enhance my-changes.html with new modal
2. Test enhanced modal thoroughly
3. Deploy to staging environment

### Phase 3: New Pages
1. Deploy approvals.html
2. Deploy announcements.html
3. Test customer filtering
4. Verify S3 data access

### Phase 4: Cleanup
1. Remove view-changes.html
2. Remove associated JavaScript
3. Update all navigation references
4. Clean up unused CSS

### Phase 5: Documentation
1. Create JSON schema documentation
2. Update README with new features
3. Create user guide
4. Document API changes

## Deployment Plan

### Deployment Steps

1. **Build Assets**: Minify CSS/JS (optional, for production)
2. **Upload to S3**: Upload HTML/CSS/JS to S3 bucket
3. **Update CloudFront**: Invalidate cache if using CDN
4. **Test Production**: Verify all pages load correctly
5. **Monitor**: Watch for errors in first 24 hours

### Rollback Plan

1. Keep previous version in S3 with version suffix
2. If issues detected, revert S3 objects to previous version
3. Invalidate CloudFront cache
4. Notify users of temporary rollback

## Future Enhancements

### Potential Improvements (Not in Current Scope)

1. **Real-time Updates**: 
   - Option A: S3 Event Notifications → SNS → WebSocket API Gateway → Browser
   - Option B: Client-side polling every 30-60 seconds to check for new objects
   - Would allow users to see new changes/approvals without manual refresh
   
2. **Advanced Filtering**: Saved filter presets, complex queries

3. **Bulk Actions**: Approve multiple changes at once

4. **Export Functionality**: Export changes/announcements to CSV/PDF

5. **Notification System**: Browser notifications for new approvals

6. **Dark Mode**: Theme toggle for user preference

7. **Internationalization**: Multi-language support

8. **Analytics Dashboard**: Usage metrics and insights

**Note**: These are potential future enhancements and are NOT part of the current implementation plan.
