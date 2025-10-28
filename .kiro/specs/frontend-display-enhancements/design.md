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
        <a href="create-announcement.html" class="nav-link">Create Announcement</a>
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

### 5. Create Announcement Page (create-announcement.html)

#### Purpose

Provide a form for CCOE team members to create announcements of different types (CIC, FinOps, InnerSource) with optional meeting scheduling and file attachments, following a similar workflow to change creation.

#### Component Structure

```javascript
// create-announcement-page.js
class CreateAnnouncementPage {
    constructor() {
        this.announcementData = {
            announcement_id: null,
            object_type: null,
            announcement_type: null,
            title: '',
            summary: '',
            content: '',
            customers: [],
            include_meeting: false,
            meeting_metadata: null,
            attachments: [],
            status: 'draft',
            modifications: []
        };
    }

    generateAnnouncementId(type) {
        // Generate ID with appropriate prefix
        // CIC: "CIC-YYYY-NNN"
        // FinOps: "FIN-YYYY-NNN"
        // InnerSource: "INN-YYYY-NNN"
        const prefix = this.getTypePrefix(type);
        const year = new Date().getFullYear();
        const sequence = this.getNextSequence(prefix);
        return `${prefix}-${year}-${sequence}`;
    }

    getTypePrefix(type) {
        const prefixes = {
            'cic': 'CIC',
            'finops': 'FIN',
            'innersource': 'INN'
        };
        return prefixes[type];
    }

    handleTypeChange(type) {
        // Update announcement_type and object_type
        this.announcementData.announcement_type = type;
        this.announcementData.object_type = `announcement_${type}`;
        this.announcementData.announcement_id = this.generateAnnouncementId(type);
    }

    handleMeetingToggle(includeMeeting) {
        // Show/hide meeting fields
        this.announcementData.include_meeting = includeMeeting;
        if (includeMeeting) {
            this.renderMeetingFields();
        } else {
            this.hideMeetingFields();
        }
    }

    async handleFileUpload(files) {
        // Upload files to S3 under announcements/{announcement-id}/attachments/
        // Store attachment metadata
        for (const file of files) {
            const s3Key = `announcements/${this.announcementData.announcement_id}/attachments/${file.name}`;
            await this.uploadToS3(file, s3Key);
            this.announcementData.attachments.push({
                name: file.name,
                s3_key: s3Key,
                size: file.size,
                uploaded_at: new Date().toISOString()
            });
        }
    }

    async saveDraft() {
        // Save announcement with status "draft"
        this.announcementData.status = 'draft';
        this.addModification('created');
        await this.saveToS3();
    }

    async submitForApproval() {
        // Validate required fields
        // Change status to "submitted"
        this.announcementData.status = 'submitted';
        this.addModification('submitted');
        await this.saveToS3();
    }

    async saveToS3() {
        // Save to S3 under each selected customer prefix
        for (const customer of this.announcementData.customers) {
            const s3Key = `customers/${customer}/announcements/${this.announcementData.announcement_id}.json`;
            await this.uploadObjectToS3(this.announcementData, s3Key);
        }
    }

    addModification(type) {
        this.announcementData.modifications.push({
            timestamp: new Date().toISOString(),
            user_id: this.getCurrentUserId(),
            modification_type: type
        });
    }
}
```

#### Visual Design

```
┌────────────────────────────────────────────────────────────┐
│  Create Announcement                                        │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Announcement Type *                                       │
│  ○ CIC (Cloud Innovator Community)                          │
│  ○ FinOps (Financial Operations)                          │
│  ○ InnerSource (Internal Open Source)                     │
│                                                            │
│  Announcement ID: [Auto-generated: CIC-2025-001]          │
│                                                            │
│  Title *                                                   │
│  [_____________________________________________]           │
│                                                            │
│  Summary *                                                 │
│  [_____________________________________________]           │
│  [_____________________________________________]           │
│                                                            │
│  Content *                                                 │
│  [_____________________________________________]           │
│  [_____________________________________________]           │
│  [_____________________________________________]           │
│  [_____________________________________________]           │
│                                                            │
│  Select Customers *                                        │
│  ☑ HTS Prod (hts)                                         │
│  ☐ CDS Global (cds)                                       │
│  ☑ FDBUS (fdbus)                                          │
│                                                            │
│  Include Meeting?                                          │
│  ○ Yes  ● No                                              │
│                                                            │
│  [If Yes selected, show meeting fields similar to         │
│   create-change.html: date, time, duration, attendees]    │
│                                                            │
│  File Attachments                                          │
│  [Drop files here or click to browse]                     │
│  📎 Q4_Report.pdf (2.3 MB) [Remove]                       │
│  📎 Presentation.pptx (5.1 MB) [Remove]                   │
│                                                            │
│  [Save Draft]  [Submit for Approval]  [Cancel]           │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### 6. Backend Email Template System

#### Purpose

Provide type-specific email templates for announcements that are triggered by the backend Go Lambda when announcements are approved.

#### Email Template Structure

```go
// internal/ses/announcement_templates.go
package ses

type AnnouncementEmailTemplate struct {
    Type        string
    Subject     string
    HTMLBody    string
    TextBody    string
}

func GetAnnouncementTemplate(announcementType string, data AnnouncementData) AnnouncementEmailTemplate {
    switch announcementType {
    case "cic":
        return getCICTemplate(data)
    case "finops":
        return getFinOpsTemplate(data)
    case "innersource":
        return getInnerSourceTemplate(data)
    default:
        return getGenericTemplate(data)
    }
}

func getCICTemplate(data AnnouncementData) AnnouncementEmailTemplate {
    return AnnouncementEmailTemplate{
        Type:    "cic",
        Subject: fmt.Sprintf("CIC Announcement: %s", data.Title),
        HTMLBody: renderCICHTMLTemplate(data),
        TextBody: renderCICTextTemplate(data),
    }
}

func getFinOpsTemplate(data AnnouncementData) AnnouncementEmailTemplate {
    return AnnouncementEmailTemplate{
        Type:    "finops",
        Subject: fmt.Sprintf("FinOps Update: %s", data.Title),
        HTMLBody: renderFinOpsHTMLTemplate(data),
        TextBody: renderFinOpsTextTemplate(data),
    }
}

func getInnerSourceTemplate(data AnnouncementData) AnnouncementEmailTemplate {
    return AnnouncementEmailTemplate{
        Type:    "innersource",
        Subject: fmt.Sprintf("Innersource Guild: %s", data.Title),
        HTMLBody: renderInnerSourceHTMLTemplate(data),
        TextBody: renderInnerSourceTextTemplate(data),
    }
}
```

#### Email Template Content Structure

Each template will include:

1. **Header**: Type-specific branding and logo
2. **Title**: Announcement title
3. **Summary**: Brief summary
4. **Content**: Full announcement content
5. **Meeting Details** (if applicable): Join link, date/time, duration
6. **Attachments** (if applicable): Links to download files
7. **Footer**: Standard CCOE contact information

#### CIC Template Example

```html
<!DOCTYPE html>
<html>
<head>
    <style>
        .cic-header { background-color: #0066cc; color: white; padding: 20px; }
        .cic-content { padding: 20px; }
        .meeting-info { background-color: #f0f8ff; padding: 15px; margin: 20px 0; }
        .attachments { margin: 20px 0; }
    </style>
</head>
<body>
    <div class="cic-header">
        <h1>☁️ Cloud Innovator Community</h1>
    </div>
    <div class="cic-content">
        <h2>{{.Title}}</h2>
        <p><strong>Summary:</strong> {{.Summary}}</p>
        <div>{{.Content}}</div>
        
        {{if .MeetingMetadata}}
        <div class="meeting-info">
            <h3>📅 Meeting Information</h3>
            <p><strong>Join URL:</strong> <a href="{{.MeetingMetadata.JoinURL}}">Click to Join</a></p>
            <p><strong>Date/Time:</strong> {{.MeetingMetadata.StartTime}}</p>
            <p><strong>Duration:</strong> {{.MeetingMetadata.Duration}}</p>
        </div>
        {{end}}
        
        {{if .Attachments}}
        <div class="attachments">
            <h3>📎 Attachments</h3>
            {{range .Attachments}}
            <p><a href="{{.URL}}">{{.Name}}</a> ({{.Size}})</p>
            {{end}}
        </div>
        {{end}}
    </div>
</body>
</html>
```

#### FinOps Template Example

```html
<!DOCTYPE html>
<html>
<head>
    <style>
        .finops-header { background-color: #28a745; color: white; padding: 20px; }
        .finops-content { padding: 20px; }
        .cost-highlight { background-color: #d4edda; padding: 10px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="finops-header">
        <h1>💰 Cloud FinOps</h1>
    </div>
    <div class="finops-content">
        <h2>{{.Title}}</h2>
        <p><strong>Summary:</strong> {{.Summary}}</p>
        <div>{{.Content}}</div>
        
        <!-- Meeting and attachments sections similar to CIC -->
    </div>
</body>
</html>
```

#### InnerSource Template Example

```html
<!DOCTYPE html>
<html>
<head>
    <style>
        .innersource-header { background-color: #6f42c1; color: white; padding: 20px; }
        .innersource-content { padding: 20px; }
        .project-highlight { background-color: #e7d9f7; padding: 10px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="innersource-header">
        <h1>🔧 Innersource Guild</h1>
    </div>
    <div class="innersource-content">
        <h2>{{.Title}}</h2>
        <p><strong>Summary:</strong> {{.Summary}}</p>
        <div>{{.Content}}</div>
        
        <!-- Meeting and attachments sections similar to CIC -->
    </div>
</body>
</html>
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
  "announcement_id": "FIN-2025-001",
  "announcement_type": "finops",
  "title": "FinOps Monthly Report - October 2025",
  "summary": "Monthly cost optimization report showing significant savings...",
  "content": "Full announcement content in markdown or HTML...",
  "customers": ["hts", "cds", "fdbus"],
  "status": "approved",
  "include_meeting": true,
  "created_by": "user-id-123",
  "created_at": "2025-10-01T00:00:00Z",
  "attachments": [
    {
      "name": "October_2025_Report.pdf",
      "s3_key": "announcements/FIN-2025-001/attachments/October_2025_Report.pdf",
      "size": 2457600,
      "uploaded_at": "2025-10-01T00:00:00Z"
    }
  ],
  "modifications": [
    {
      "timestamp": "2025-10-01T09:00:00Z",
      "user_id": "user-id-123",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-10-01T10:00:00Z",
      "user_id": "user-id-123",
      "modification_type": "submitted"
    },
    {
      "timestamp": "2025-10-01T11:00:00Z",
      "user_id": "system",
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGI2...",
        "join_url": "https://teams.microsoft.com/l/meetup/...",
        "start_time": "2025-10-05T14:00:00Z",
        "end_time": "2025-10-05T15:00:00Z"
      }
    },
    {
      "timestamp": "2025-10-01T11:30:00Z",
      "user_id": "user-id-456",
      "modification_type": "approved"
    }
  ]
}
```

### Object Type Enumeration

```javascript
const OBJECT_TYPES = {
  CHANGE: 'change',
  ANNOUNCEMENT_FINOPS: 'announcement_finops',
  ANNOUNCEMENT_INNERSOURCE: 'announcement_innersource',
  ANNOUNCEMENT_CIC: 'announcement_cic',
  ANNOUNCEMENT_GENERAL: 'announcement_general'
};

const ANNOUNCEMENT_TYPES = {
  CIC: 'cic',
  FINOPS: 'finops',
  INNERSOURCE: 'innersource'
};

const ANNOUNCEMENT_ID_PREFIXES = {
  CIC: 'CIC',
  FINOPS: 'FIN',
  INNERSOURCE: 'INN'
};

const STATUS_TYPES = {
  DRAFT: 'draft',
  SUBMITTED: 'submitted',
  APPROVED: 'approved',
  CANCELLED: 'cancelled'
};

const MODIFICATION_TYPES = {
  CREATED: 'created',
  UPDATED: 'updated',
  SUBMITTED: 'submitted',
  APPROVED: 'approved',
  CANCELLED: 'cancelled',
  DELETED: 'deleted',
  MEETING_SCHEDULED: 'meeting_scheduled',
  MEETING_CANCELLED: 'meeting_cancelled'
};
```

### 7. Announcement Action Buttons and Status Management

#### Purpose

Provide consistent action buttons for announcements that mirror the change management workflow, allowing CCOE team members to approve, cancel, and complete announcements through the UI.

#### Component Structure

```javascript
// announcement-actions.js
class AnnouncementActions {
    constructor(announcementId, currentStatus) {
        this.announcementId = announcementId;
        this.currentStatus = currentStatus;
        this.baseUrl = window.location.origin;
    }

    /**
     * Render action buttons based on announcement status
     */
    renderActionButtons() {
        const buttons = this.getAvailableActions();
        return buttons.map(action => this.renderButton(action)).join('');
    }

    /**
     * Get available actions based on current status
     */
    getAvailableActions() {
        const actions = {
            'draft': [],
            'submitted': ['approve', 'cancel'],
            'approved': ['complete', 'cancel'],
            'completed': [],
            'cancelled': []
        };
        return actions[this.currentStatus] || [];
    }

    /**
     * Render a single action button
     */
    renderButton(action) {
        const buttonConfig = {
            approve: {
                label: '✅ Approve',
                class: 'action-btn approve',
                handler: 'approveAnnouncement'
            },
            cancel: {
                label: '💣 Cancel',
                class: 'action-btn cancel',
                handler: 'cancelAnnouncement'
            },
            complete: {
                label: '🎉 Complete',
                class: 'action-btn complete',
                handler: 'completeAnnouncement'
            }
        };

        const config = buttonConfig[action];
        return `
            <button class="${config.class}" 
                    onclick="announcementActions.${config.handler}('${this.announcementId}')"
                    aria-label="${config.label} announcement ${this.announcementId}">
                ${config.label}
            </button>
        `;
    }

    /**
     * Approve an announcement
     */
    async approveAnnouncement(announcementId) {
        if (!confirm('Are you sure you want to approve this announcement?')) {
            return;
        }

        try {
            showInfo('statusContainer', 'Approving announcement...', { duration: 0 });

            // Fetch current announcement data
            const announcement = await this.fetchAnnouncement(announcementId);

            // Update status and add modification entry
            const updatedAnnouncement = {
                ...announcement,
                status: 'approved',
                approvedAt: new Date().toISOString(),
                approvedBy: window.portal?.currentUser || 'Unknown'
            };

            if (!updatedAnnouncement.modifications) {
                updatedAnnouncement.modifications = [];
            }
            updatedAnnouncement.modifications.push({
                timestamp: updatedAnnouncement.approvedAt,
                user_id: updatedAnnouncement.approvedBy,
                modification_type: 'approved'
            });

            // Update via upload_lambda API
            await this.updateAnnouncementViaAPI(announcementId, updatedAnnouncement);

            clearMessages('statusContainer');
            showSuccess('statusContainer', 'Announcement approved successfully! Emails will be sent and meetings scheduled if configured.');

            // Refresh view
            if (window.approvalsPage) {
                await window.approvalsPage.refresh();
            }

        } catch (error) {
            console.error('Error approving announcement:', error);
            clearMessages('statusContainer');
            showError('statusContainer', `Error approving announcement: ${error.message}`);
        }
    }

    /**
     * Cancel an announcement
     */
    async cancelAnnouncement(announcementId) {
        if (!confirm('Are you sure you want to cancel this announcement?')) {
            return;
        }

        try {
            showInfo('statusContainer', 'Cancelling announcement...', { duration: 0 });

            const announcement = await this.fetchAnnouncement(announcementId);

            const updatedAnnouncement = {
                ...announcement,
                status: 'cancelled',
                cancelledAt: new Date().toISOString(),
                cancelledBy: window.portal?.currentUser || 'Unknown'
            };

            if (!updatedAnnouncement.modifications) {
                updatedAnnouncement.modifications = [];
            }
            updatedAnnouncement.modifications.push({
                timestamp: updatedAnnouncement.cancelledAt,
                user_id: updatedAnnouncement.cancelledBy,
                modification_type: 'cancelled'
            });

            await this.updateAnnouncementViaAPI(announcementId, updatedAnnouncement);

            clearMessages('statusContainer');
            showSuccess('statusContainer', 'Announcement cancelled successfully! Scheduled meetings will be cancelled.');

            if (window.approvalsPage) {
                await window.approvalsPage.refresh();
            }

        } catch (error) {
            console.error('Error cancelling announcement:', error);
            clearMessages('statusContainer');
            showError('statusContainer', `Error cancelling announcement: ${error.message}`);
        }
    }

    /**
     * Complete an announcement
     */
    async completeAnnouncement(announcementId) {
        if (!confirm('Mark this announcement as completed?')) {
            return;
        }

        try {
            showInfo('statusContainer', 'Completing announcement...', { duration: 0 });

            const announcement = await this.fetchAnnouncement(announcementId);

            const updatedAnnouncement = {
                ...announcement,
                status: 'completed',
                completedAt: new Date().toISOString(),
                completedBy: window.portal?.currentUser || 'Unknown'
            };

            if (!updatedAnnouncement.modifications) {
                updatedAnnouncement.modifications = [];
            }
            updatedAnnouncement.modifications.push({
                timestamp: updatedAnnouncement.completedAt,
                user_id: updatedAnnouncement.completedBy,
                modification_type: 'completed'
            });

            await this.updateAnnouncementViaAPI(announcementId, updatedAnnouncement);

            clearMessages('statusContainer');
            showSuccess('statusContainer', 'Announcement marked as completed!');

            if (window.approvalsPage) {
                await window.approvalsPage.refresh();
            }

        } catch (error) {
            console.error('Error completing announcement:', error);
            clearMessages('statusContainer');
            showError('statusContainer', `Error completing announcement: ${error.message}`);
        }
    }

    /**
     * Fetch announcement data from S3
     */
    async fetchAnnouncement(announcementId) {
        // Use s3Client to fetch announcement
        const announcements = await s3Client.fetchAllObjects();
        const announcement = announcements.find(a => 
            a.announcement_id === announcementId || a.id === announcementId
        );
        
        if (!announcement) {
            throw new Error('Announcement not found');
        }
        
        return announcement;
    }

    /**
     * Update announcement via upload_lambda API
     */
    async updateAnnouncementViaAPI(announcementId, announcementData) {
        const response = await fetch(`${this.baseUrl}/upload`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            credentials: 'same-origin',
            body: JSON.stringify({
                action: 'update_announcement',
                announcement_id: announcementId,
                data: announcementData
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Failed to update announcement');
        }

        return await response.json();
    }
}
```

#### Visual Design - Action Buttons in Approvals Page

```
┌────────────────────────────────────────────────────────────┐
│  📢 FIN-2025-001 - FinOps Monthly Report                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Type: FinOps  │  Status: Pending Approval            │  │
│  │ Submitted: 2025-10-01 10:00  │  By: Jane Smith      │  │
│  │                                                       │  │
│  │ Summary: Monthly cost optimization report...         │  │
│  │                                                       │  │
│  │ [View Details] [💣 Cancel] [✅ Approve]              │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────┐
│  📢 CIC-2025-003 - AWS Best Practices Update               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Type: CIC  │  Status: Approved                        │  │
│  │ Approved: 2025-10-02 14:30  │  By: John Doe          │  │
│  │                                                       │  │
│  │ Summary: Updated guidelines for AWS resources...     │  │
│  │                                                       │  │
│  │ [View Details] [💣 Cancel] [🎉 Complete]             │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

#### API Integration - upload_lambda Endpoint

The frontend will call the upload_lambda API endpoint to update announcement status:

```javascript
// API Request Format
POST /upload
Content-Type: application/json
Credentials: same-origin

{
  "action": "update_announcement",
  "announcement_id": "FIN-2025-001",
  "data": {
    "object_type": "announcement_finops",
    "announcement_id": "FIN-2025-001",
    "status": "approved",
    "approvedAt": "2025-10-15T10:30:00Z",
    "approvedBy": "john.doe@hearst.com",
    "modifications": [
      {
        "timestamp": "2025-10-15T10:30:00Z",
        "user_id": "john.doe@hearst.com",
        "modification_type": "approved"
      }
    ],
    // ... rest of announcement data
  }
}

// API Response Format
{
  "success": true,
  "message": "Announcement updated successfully",
  "announcement_id": "FIN-2025-001",
  "customers_updated": ["hts", "cds", "fdbus"]
}
```

**Note:** The `/upload` endpoint is used consistently across the application for both changes and announcements. Other endpoints follow the pattern:

- `/changes` - List all changes
- `/announcements` - List all announcements  
- `/auth-check` - Authentication check
- `/api/user/context` - User context (uses `/api/` prefix)

#### Backend Processing Flow

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend Action (Approve Button Clicked)                   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  upload_lambda API Endpoint                                  │
│  - Validates request                                         │
│  - Updates S3 objects for all customers                      │
│  - Returns success response                                  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  S3 Event Triggers Backend Lambda                            │
│  - Detects object_type starts with "announcement_"          │
│  - Routes to handleAnnouncementEvent()                       │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Backend Lambda Processing                                   │
│  - Checks status == "approved"                               │
│  - Schedules meeting if include_meeting == true              │
│  - Sends type-specific emails (CIC/FinOps/InnerSource)      │
│  - Updates S3 with meeting metadata                          │
└─────────────────────────────────────────────────────────────┘
```

#### Status Transition Rules

```javascript
const STATUS_TRANSITIONS = {
    'draft': ['submitted'],
    'submitted': ['approved', 'cancelled'],
    'approved': ['completed', 'cancelled'],
    'completed': [], // Terminal state
    'cancelled': []  // Terminal state
};

// Validate transition
function canTransitionTo(currentStatus, newStatus) {
    const allowedTransitions = STATUS_TRANSITIONS[currentStatus] || [];
    return allowedTransitions.includes(newStatus);
}
```

#### Modification Types for Announcements

```javascript
const ANNOUNCEMENT_MODIFICATION_TYPES = {
    CREATED: 'created',
    UPDATED: 'updated',
    SUBMITTED: 'submitted',
    APPROVED: 'approved',
    CANCELLED: 'cancelled',
    COMPLETED: 'completed',
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

## Backend Lambda Integration

### Announcement Processing Flow

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend (create-announcement.html)                        │
│  ├─ User creates announcement                               │
│  ├─ Saves to S3 with status "submitted"                    │
│  └─ Adds modification entry: "submitted"                    │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Frontend (approvals.html)                                   │
│  ├─ Approver reviews announcement                           │
│  ├─ Updates S3 object status to "approved"                  │
│  └─ Adds modification entry: "approved"                     │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  S3 Event Notification                                       │
│  └─ Triggers on object update                               │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Backend Go Lambda (main.go)                                 │
│  ├─ Detects object_type starts with "announcement_"         │
│  ├─ Checks status == "approved"                             │
│  ├─ If include_meeting == true:                             │
│  │   ├─ Calls Microsoft Graph API                           │
│  │   ├─ Creates Teams meeting                               │
│  │   ├─ Updates S3 with meeting_metadata                    │
│  │   └─ Adds modification: "meeting_scheduled"             │
│  ├─ Determines announcement type from object_type           │
│  ├─ Loads appropriate email template                        │
│  ├─ Sends emails via SES topic management                   │
│  └─ Includes meeting join link if applicable                │
└─────────────────────────────────────────────────────────────┘
```

### Backend Lambda Handler Updates

```go
// internal/lambda/handlers.go
func HandleS3Event(ctx context.Context, event events.S3Event) error {
    for _, record := range event.Records {
        // Fetch object from S3
        obj, err := fetchS3Object(record.S3.Bucket.Name, record.S3.Object.Key)
        if err != nil {
            return err
        }

        // Check object type
        if strings.HasPrefix(obj.ObjectType, "announcement_") {
            return handleAnnouncementEvent(ctx, obj)
        } else if obj.ObjectType == "change" {
            return handleChangeEvent(ctx, obj)
        }
    }
    return nil
}

func handleAnnouncementEvent(ctx context.Context, announcement Announcement) error {
    // Only process approved announcements
    if announcement.Status != "approved" {
        return nil
    }

    // Schedule meeting if requested
    if announcement.IncludeMeeting && announcement.MeetingMetadata == nil {
        meetingData, err := scheduleMeeting(ctx, announcement)
        if err != nil {
            return err
        }
        
        // Update S3 with meeting metadata
        announcement.MeetingMetadata = meetingData
        announcement.Modifications = append(announcement.Modifications, Modification{
            Timestamp: time.Now(),
            UserID: "system",
            ModificationType: "meeting_scheduled",
            MeetingMetadata: meetingData,
        })
        
        if err := updateS3Object(announcement); err != nil {
            return err
        }
    }

    // Send emails using type-specific template
    return sendAnnouncementEmails(ctx, announcement)
}
```

### Email Sending Implementation

```go
// internal/ses/announcement_emails.go
func sendAnnouncementEmails(ctx context.Context, announcement Announcement) error {
    // Get announcement type from object_type
    announcementType := strings.TrimPrefix(announcement.ObjectType, "announcement_")
    
    // Load appropriate template
    template := GetAnnouncementTemplate(announcementType, announcement)
    
    // Get customer contact lists from SES
    for _, customerCode := range announcement.Customers {
        contactList := getCustomerContactList(customerCode)
        
        // Send email via SES topic management
        err := sendEmailToContactList(ctx, contactList, template)
        if err != nil {
            log.Printf("Failed to send announcement to %s: %v", customerCode, err)
            continue
        }
    }
    
    return nil
}

func sendEmailToContactList(ctx context.Context, contactList string, template AnnouncementEmailTemplate) error {
    // Use the same SES topic management as the change system
    // This leverages existing contact list infrastructure and subscription management
    return sendViaSESTopicManagement(ctx, contactList, template)
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

## Backend Architecture: Separate Announcement and Change Processing

### Overview

This section addresses Requirement 15: ensuring announcements are processed as `AnnouncementMetadata` throughout their entire lifecycle without conversion to `ChangeMetadata`. This prevents data loss and maintains proper type separation between announcements and changes.

### Current Problem

The current implementation converts `AnnouncementMetadata` to `ChangeMetadata` for processing convenience, which causes:

- Loss of announcement-specific fields (announcement_id, title, summary, content) when saved back to S3
- "Untitled announcement" bugs when announcements are cancelled or updated
- Confusion between announcement and change types
- Incorrect field mappings in emails and meeting invites

### Proposed Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    S3 Event Trigger                          │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│              Backend Lambda Handler                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  1. Read S3 Object                                   │   │
│  │  2. Check object_type field                          │   │
│  │  3. Route to appropriate handler                     │   │
│  └──────────────────┬───────────────────────────────────┘   │
│                     │                                        │
│         ┌───────────┴───────────┐                           │
│         ▼                       ▼                           │
│  ┌─────────────┐         ┌─────────────┐                   │
│  │   Change    │         │Announcement │                   │
│  │   Handler   │         │   Handler   │                   │
│  │             │         │             │                   │
│  │ Processes   │         │ Processes   │                   │
│  │ Change      │         │ Announcement│                   │
│  │ Metadata    │         │ Metadata    │                   │
│  └─────────────┘         └─────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

### Type Definitions

#### AnnouncementMetadata Structure (Go)

```go
// internal/types/types.go

type AnnouncementMetadata struct {
    ObjectType       string              `json:"object_type"`        // "announcement_cic", "announcement_finops", etc.
    AnnouncementID   string              `json:"announcement_id"`    // "CIC-xxx", "FIN-xxx", "INN-xxx"
    AnnouncementType string              `json:"announcement_type"`  // "cic", "finops", "innersource"
    Title            string              `json:"title"`
    Summary          string              `json:"summary"`
    Content          string              `json:"content"`
    Customers        []string            `json:"customers"`
    Status           string              `json:"status"`             // "draft", "submitted", "approved", "cancelled", "completed"
    IncludeMeeting   bool                `json:"include_meeting"`
    MeetingMetadata  *MeetingMetadata    `json:"meeting_metadata,omitempty"`
    Attachments      []AttachmentInfo    `json:"attachments,omitempty"`
    Version          int                 `json:"version"`
    Modifications    []ModificationEntry `json:"modifications"`
    CreatedBy        string              `json:"created_by"`
    CreatedAt        time.Time           `json:"created_at"`
    ModifiedBy       string              `json:"modified_by"`
    ModifiedAt       time.Time           `json:"modified_at"`
    SubmittedBy      string              `json:"submitted_by,omitempty"`
    SubmittedAt      *time.Time          `json:"submitted_at,omitempty"`
    ApprovedBy       string              `json:"approved_by,omitempty"`
    ApprovedAt       *time.Time          `json:"approved_at,omitempty"`
}

type AttachmentInfo struct {
    Name       string    `json:"name"`
    S3Key      string    `json:"s3_key"`
    Size       int64     `json:"size"`
    UploadedAt time.Time `json:"uploaded_at"`
}
```

### Handler Functions

#### Main Event Router

```go
// internal/lambda/handlers.go

func HandleS3Event(ctx context.Context, event events.S3Event) error {
    for _, record := range event.Records {
        // Read object from S3
        obj, err := readS3Object(record.S3.Bucket.Name, record.S3.Object.Key)
        if err != nil {
            return err
        }

        // Parse to determine type
        var baseObj struct {
            ObjectType string `json:"object_type"`
        }
        if err := json.Unmarshal(obj, &baseObj); err != nil {
            return err
        }

        // Route based on object_type
        if strings.HasPrefix(baseObj.ObjectType, "announcement_") {
            return handleAnnouncementEvent(ctx, obj, record)
        } else if baseObj.ObjectType == "change" {
            return handleChangeEvent(ctx, obj, record)
        }
    }
    return nil
}
```

#### Announcement Event Handler

```go
// internal/lambda/announcement_handler.go

func handleAnnouncementEvent(ctx context.Context, objBytes []byte, record events.S3EventRecord) error {
    // Parse as AnnouncementMetadata
    var announcement types.AnnouncementMetadata
    if err := json.Unmarshal(objBytes, &announcement); err != nil {
        return fmt.Errorf("failed to parse announcement: %w", err)
    }

    log.Printf("Processing announcement %s with status %s", announcement.AnnouncementID, announcement.Status)

    // Route based on status
    switch announcement.Status {
    case "submitted":
        return handleAnnouncementSubmitted(ctx, &announcement)
    case "approved":
        return handleAnnouncementApproved(ctx, &announcement)
    case "cancelled":
        return handleAnnouncementCancelled(ctx, &announcement)
    case "completed":
        return handleAnnouncementCompleted(ctx, &announcement)
    default:
        log.Printf("No action needed for announcement %s with status %s", announcement.AnnouncementID, announcement.Status)
        return nil
    }
}

func handleAnnouncementSubmitted(ctx context.Context, announcement *types.AnnouncementMetadata) error {
    // Send approval request email
    return sendAnnouncementApprovalRequest(ctx, announcement)
}

func handleAnnouncementApproved(ctx context.Context, announcement *types.AnnouncementMetadata) error {
    // Schedule meeting if requested
    if announcement.IncludeMeeting {
        if err := scheduleAnnouncementMeeting(ctx, announcement); err != nil {
            log.Printf("Failed to schedule meeting: %v", err)
            // Don't fail the entire process
        }
    }

    // Send announcement emails
    return sendAnnouncementEmails(ctx, announcement)
}

func handleAnnouncementCancelled(ctx context.Context, announcement *types.AnnouncementMetadata) error {
    // Cancel meeting if scheduled
    if announcement.MeetingMetadata != nil && announcement.MeetingMetadata.MeetingID != "" {
        if err := cancelAnnouncementMeeting(ctx, announcement); err != nil {
            log.Printf("Failed to cancel meeting: %v", err)
        }
    }

    // Send cancellation email
    return sendAnnouncementCancellationEmail(ctx, announcement)
}

func handleAnnouncementCompleted(ctx context.Context, announcement *types.AnnouncementMetadata) error {
    // Send completion email
    return sendAnnouncementCompletionEmail(ctx, announcement)
}
```

### Email Functions

#### Announcement Email Sender

```go
// internal/lambda/announcement_emails.go

func sendAnnouncementEmails(ctx context.Context, announcement *types.AnnouncementMetadata) error {
    // Get appropriate email template based on announcement type
    template := ses.GetAnnouncementTemplate(announcement.AnnouncementType, ses.AnnouncementData{
        AnnouncementID:   announcement.AnnouncementID,
        AnnouncementType: announcement.AnnouncementType,
        Title:            announcement.Title,
        Summary:          announcement.Summary,
        Content:          announcement.Content,
        Customers:        announcement.Customers,
        MeetingMetadata:  announcement.MeetingMetadata,
        Attachments:      convertAttachments(announcement.Attachments),
        CreatedBy:        announcement.CreatedBy,
        CreatedAt:        announcement.CreatedAt,
    })

    // Send to appropriate SES topic based on announcement type
    topicName := getAnnouncementTopicName(announcement.AnnouncementType)
    
    return sendEmailToTopic(ctx, topicName, template)
}

func getAnnouncementTopicName(announcementType string) string {
    topics := map[string]string{
        "cic":         "cic-announce",
        "finops":      "finops-announce",
        "innersource": "innersource-announce",
    }
    if topic, ok := topics[announcementType]; ok {
        return topic
    }
    return "general-announce"
}
```

### Meeting Functions

#### Announcement Meeting Scheduler

```go
// internal/lambda/announcement_meetings.go

func scheduleAnnouncementMeeting(ctx context.Context, announcement *types.AnnouncementMetadata) error {
    // Extract meeting details from announcement metadata
    meetingData := extractMeetingDataFromAnnouncement(announcement)
    
    // Create meeting via Microsoft Graph API
    meetingID, joinURL, err := createGraphMeeting(ctx, meetingData)
    if err != nil {
        return fmt.Errorf("failed to create meeting: %w", err)
    }

    // Update announcement with meeting metadata
    announcement.MeetingMetadata = &types.MeetingMetadata{
        MeetingID: meetingID,
        JoinURL:   joinURL,
        StartTime: meetingData.StartTime,
        EndTime:   meetingData.EndTime,
        Subject:   fmt.Sprintf("%s Event: %s", strings.ToUpper(announcement.AnnouncementType), announcement.Title),
        Organizer: "ccoe@hearst.com",
    }

    // Add modification entry
    announcement.Modifications = append(announcement.Modifications, types.ModificationEntry{
        Timestamp:        time.Now(),
        UserID:           "system",
        ModificationType: "meeting_scheduled",
        MeetingMetadata:  announcement.MeetingMetadata,
    })

    // Save updated announcement back to S3
    return saveAnnouncementToS3(ctx, announcement)
}

func extractMeetingDataFromAnnouncement(announcement *types.AnnouncementMetadata) MeetingData {
    // Extract meeting time, duration, attendees from announcement metadata
    // This data comes from the create-announcement form
    return MeetingData{
        Subject:   fmt.Sprintf("%s Event: %s", strings.ToUpper(announcement.AnnouncementType), announcement.Title),
        StartTime: announcement.MeetingMetadata.StartTime,
        EndTime:   announcement.MeetingMetadata.EndTime,
        Attendees: extractAttendees(announcement),
        Body:      generateAnnouncementMeetingBody(announcement),
    }
}

func generateAnnouncementMeetingBody(announcement *types.AnnouncementMetadata) string {
    // Generate meeting body HTML specific to announcements
    return fmt.Sprintf(`
<h2>📢 %s Announcement</h2>
<p><strong>Title:</strong> %s</p>
<p><strong>Summary:</strong> %s</p>
<div><strong>Content:</strong><br/>%s</div>
`, strings.ToUpper(announcement.AnnouncementType), announcement.Title, announcement.Summary, announcement.Content)
}
```

### S3 Operations

#### Save Announcement to S3

```go
// internal/lambda/announcement_storage.go

func saveAnnouncementToS3(ctx context.Context, announcement *types.AnnouncementMetadata) error {
    // Serialize announcement as JSON
    data, err := json.MarshalIndent(announcement, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal announcement: %w", err)
    }

    // Save to S3 for each customer
    for _, customer := range announcement.Customers {
        key := fmt.Sprintf("customers/%s/announcements/%s.json", customer, announcement.AnnouncementID)
        if err := putS3Object(ctx, bucketName, key, data); err != nil {
            log.Printf("Failed to save announcement to %s: %v", key, err)
            return err
        }
    }

    // Also save to archive
    archiveKey := fmt.Sprintf("archive/%s.json", announcement.AnnouncementID)
    if err := putS3Object(ctx, bucketName, archiveKey, data); err != nil {
        log.Printf("Failed to save announcement to archive: %v", err)
    }

    return nil
}

func readAnnouncementFromS3(ctx context.Context, bucket, key string) (*types.AnnouncementMetadata, error) {
    data, err := getS3Object(ctx, bucket, key)
    if err != nil {
        return nil, err
    }

    var announcement types.AnnouncementMetadata
    if err := json.Unmarshal(data, &announcement); err != nil {
        return nil, fmt.Errorf("failed to unmarshal announcement: %w", err)
    }

    return &announcement, nil
}
```

### Data Cleanup Strategy

Since backwards compatibility is not required, the strategy is simplified:

1. **Delete all existing announcements**: Remove all announcement objects from S3 (no migration needed)
2. **Deploy new code**: Deploy the updated backend Lambda with separate announcement handlers
3. **Fresh start**: All new announcements will be created with proper AnnouncementMetadata structure
4. **Test**: Create new announcements and verify they maintain data integrity through all status changes
5. **Monitor**: Watch CloudWatch logs for any parsing errors or data issues

### Testing Strategy

#### Unit Tests

```go
// internal/lambda/announcement_handler_test.go

func TestHandleAnnouncementEvent(t *testing.T) {
    tests := []struct {
        name         string
        announcement types.AnnouncementMetadata
        expectedErr  bool
    }{
        {
            name: "submitted announcement sends approval email",
            announcement: types.AnnouncementMetadata{
                AnnouncementID:   "CIC-001",
                AnnouncementType: "cic",
                Title:            "Test Announcement",
                Summary:          "Test Summary",
                Content:          "Test Content",
                Status:           "submitted",
            },
            expectedErr: false,
        },
        {
            name: "approved announcement schedules meeting",
            announcement: types.AnnouncementMetadata{
                AnnouncementID:   "FIN-001",
                AnnouncementType: "finops",
                Title:            "Test Announcement",
                Status:           "approved",
                IncludeMeeting:   true,
            },
            expectedErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := handleAnnouncementEvent(context.Background(), &tt.announcement)
            if (err != nil) != tt.expectedErr {
                t.Errorf("handleAnnouncementEvent() error = %v, expectedErr %v", err, tt.expectedErr)
            }
        })
    }
}
```

#### Integration Tests

1. Create announcement via frontend → verify S3 object has all fields
2. Submit announcement → verify approval email sent
3. Approve announcement → verify meeting scheduled and emails sent
4. Cancel announcement → verify meeting cancelled and fields preserved
5. Read announcement from S3 → verify all fields intact

### Error Handling

```go
func handleAnnouncementEvent(ctx context.Context, objBytes []byte, record events.S3EventRecord) error {
    var announcement types.AnnouncementMetadata
    if err := json.Unmarshal(objBytes, &announcement); err != nil {
        log.Printf("ERROR: Failed to parse announcement from %s: %v", record.S3.Object.Key, err)
        return fmt.Errorf("failed to parse announcement: %w", err)
    }

    // Validate required fields
    if announcement.AnnouncementID == "" {
        log.Printf("ERROR: Announcement missing announcement_id in %s", record.S3.Object.Key)
        return fmt.Errorf("announcement missing announcement_id")
    }
    if announcement.Title == "" {
        log.Printf("ERROR: Announcement %s missing title", announcement.AnnouncementID)
        return fmt.Errorf("announcement missing title")
    }

    // Continue with processing...
}
```

### Benefits of This Approach

1. **Data Integrity**: Announcement fields are never lost during status changes
2. **Type Safety**: Clear separation between announcements and changes
3. **Maintainability**: Easier to add announcement-specific features
4. **Debugging**: Clearer logs and error messages for announcement processing
5. **Scalability**: Easy to add new announcement types without affecting changes
