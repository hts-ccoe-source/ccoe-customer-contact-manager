# JSON Schema Documentation

**Version:** 1.0.0  
**Last Updated:** 2025-10-14

This document provides comprehensive JSON schema definitions for all object types used in the CCOE Customer Contact Manager system. These schemas define the structure, data types, validation rules, and examples for change objects and announcement objects stored in AWS S3.

## Table of Contents

1. [Change Object Schema](#change-object-schema)
2. [Announcement Object Schema](#announcement-object-schema)
3. [Shared Data Structures](#shared-data-structures)
4. [Validation Rules](#validation-rules)
5. [Examples](#examples)

---

## Change Object Schema

### Overview

Change objects represent planned infrastructure or configuration changes that require approval and scheduling. They are stored in S3 under the `customers/{customer-code}/` prefix with the filename pattern `{change-id}.json`.

### Schema Definition

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": [
    "object_type",
    "change_id",
    "title",
    "description",
    "status",
    "created_by",
    "created_at",
    "modifications"
  ],
  "properties": {
    "object_type": {
      "type": "string",
      "const": "change",
      "description": "Identifies this object as a change. Must always be 'change'."
    },
    "change_id": {
      "type": "string",
      "pattern": "^CHANGE-[0-9]{4}-[0-9]{3,}$",
      "description": "Unique identifier for the change in format CHANGE-YYYY-NNN",
      "example": "CHANGE-2025-001"
    },
    "title": {
      "type": "string",
      "minLength": 1,
      "maxLength": 200,
      "description": "Brief title describing the change"
    },
    "description": {
      "type": "string",
      "minLength": 1,
      "description": "Detailed description of what the change entails"
    },
    "implementation_plan": {
      "type": "string",
      "description": "Step-by-step plan for implementing the change"
    },
    "schedule": {
      "type": "object",
      "required": ["start_time", "end_time", "timezone"],
      "properties": {
        "start_time": {
          "type": "string",
          "format": "date-time",
          "description": "ISO 8601 timestamp for when the change begins"
        },
        "end_time": {
          "type": "string",
          "format": "date-time",
          "description": "ISO 8601 timestamp for when the change completes"
        },
        "timezone": {
          "type": "string",
          "description": "Timezone identifier (e.g., 'UTC', 'America/New_York')"
        }
      }
    },
    "affected_customers": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Array of customer codes affected by this change"
    },
    "status": {
      "type": "string",
      "enum": ["draft", "pending_approval", "approved", "cancelled", "completed"],
      "description": "Current status of the change"
    },
    "created_by": {
      "type": "string",
      "description": "User ID of the person who created the change"
    },
    "created_at": {
      "type": "string",
      "format": "date-time",
      "description": "ISO 8601 timestamp when the change was created"
    },
    "modifications": {
      "type": "array",
      "items": {
        "$ref": "#/definitions/Modification"
      },
      "description": "Array of modification history entries. See Modification schema below."
    }
  }
}
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `object_type` | string | Yes | Must be "change". Used to identify object type in S3. |
| `change_id` | string | Yes | Unique identifier in format CHANGE-YYYY-NNN (e.g., CHANGE-2025-001) |
| `title` | string | Yes | Brief title (1-200 characters) |
| `description` | string | Yes | Detailed description of the change |
| `implementation_plan` | string | No | Step-by-step implementation instructions |
| `schedule` | object | No | Scheduling information with start_time, end_time, timezone |
| `affected_customers` | array | No | List of customer codes (e.g., ["hts", "cds"]) |
| `status` | string | Yes | One of: draft, pending_approval, approved, cancelled, completed |
| `created_by` | string | Yes | User ID of creator |
| `created_at` | string | Yes | ISO 8601 timestamp |
| `modifications` | array | Yes | History of all modifications (see Modification schema) |

---

## Announcement Object Schema

### Overview

Announcement objects represent communications to customers including FinOps reports, InnerSourcing Guild updates, and CIC/Cloud Enablement announcements. They are stored in S3 under the `customers/{customer-code}/` prefix.

### Schema Definition

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": [
    "object_type",
    "announcement_id",
    "announcement_type",
    "title",
    "summary",
    "content",
    "customers",
    "status",
    "created_by",
    "created_at",
    "modifications"
  ],
  "properties": {
    "object_type": {
      "type": "string",
      "enum": [
        "announcement_cic",
        "announcement_finops",
        "announcement_innersource",
        "announcement_general"
      ],
      "description": "Identifies the announcement type. Format: announcement_{type}"
    },
    "announcement_id": {
      "type": "string",
      "pattern": "^(CIC|FIN|INN)-[0-9]{4}-[0-9]{3,}$",
      "description": "Unique identifier with type-specific prefix (CIC-YYYY-NNN, FIN-YYYY-NNN, INN-YYYY-NNN)",
      "example": "CIC-2025-001"
    },
    "announcement_type": {
      "type": "string",
      "enum": ["cic", "finops", "innersource", "general"],
      "description": "Type of announcement"
    },
    "title": {
      "type": "string",
      "minLength": 1,
      "maxLength": 200,
      "description": "Announcement title"
    },
    "summary": {
      "type": "string",
      "minLength": 1,
      "maxLength": 500,
      "description": "Brief summary for list views"
    },
    "content": {
      "type": "string",
      "minLength": 1,
      "description": "Full announcement content (supports markdown or HTML)"
    },
    "customers": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "minItems": 1,
      "description": "Array of customer codes this announcement applies to"
    },
    "status": {
      "type": "string",
      "enum": ["draft", "pending_approval", "approved", "cancelled"],
      "description": "Current status of the announcement"
    },
    "include_meeting": {
      "type": "boolean",
      "description": "Whether a meeting should be scheduled for this announcement"
    },
    "created_by": {
      "type": "string",
      "description": "User ID of the person who created the announcement"
    },
    "created_at": {
      "type": "string",
      "format": "date-time",
      "description": "ISO 8601 timestamp when the announcement was created"
    },
    "attachments": {
      "type": "array",
      "items": {
        "$ref": "#/definitions/Attachment"
      },
      "description": "Array of file attachments"
    },
    "modifications": {
      "type": "array",
      "items": {
        "$ref": "#/definitions/Modification"
      },
      "description": "Array of modification history entries"
    }
  }
}
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `object_type` | string | Yes | One of: announcement_cic, announcement_finops, announcement_innersource, announcement_general |
| `announcement_id` | string | Yes | Unique ID with prefix: CIC-YYYY-NNN, FIN-YYYY-NNN, or INN-YYYY-NNN |
| `announcement_type` | string | Yes | One of: cic, finops, innersource, general |
| `title` | string | Yes | Announcement title (1-200 characters) |
| `summary` | string | Yes | Brief summary (1-500 characters) for list views |
| `content` | string | Yes | Full content (markdown or HTML supported) |
| `customers` | array | Yes | Customer codes (minimum 1) |
| `status` | string | Yes | One of: draft, pending_approval, approved, cancelled |
| `include_meeting` | boolean | No | Whether to schedule a meeting (default: false) |
| `created_by` | string | Yes | User ID of creator |
| `created_at` | string | Yes | ISO 8601 timestamp |
| `attachments` | array | No | File attachments (see Attachment schema) |
| `modifications` | array | Yes | History of modifications (see Modification schema) |

---

## Shared Data Structures

### Modification Schema

The `modifications` array tracks all changes made to an object throughout its lifecycle. This structure is shared between change and announcement objects.

**Reference:** For detailed modification history structure and implementation, see the `object-model-enhancement` spec.

```json
{
  "type": "object",
  "required": ["timestamp", "user_id", "modification_type"],
  "properties": {
    "timestamp": {
      "type": "string",
      "format": "date-time",
      "description": "ISO 8601 timestamp when the modification occurred"
    },
    "user_id": {
      "type": "string",
      "description": "User ID who made the modification (or 'system' for automated changes)"
    },
    "modification_type": {
      "type": "string",
      "enum": [
        "created",
        "updated",
        "submitted",
        "approved",
        "cancelled",
        "deleted",
        "meeting_scheduled",
        "meeting_cancelled"
      ],
      "description": "Type of modification"
    },
    "meeting_metadata": {
      "type": "object",
      "description": "Present only when modification_type is 'meeting_scheduled'. See Meeting Metadata schema."
    },
    "comment": {
      "type": "string",
      "description": "Optional comment explaining the modification"
    }
  }
}
```

#### Modification Types

| Type | Description | User ID |
|------|-------------|---------|
| `created` | Object was initially created | User who created |
| `updated` | Object fields were modified | User who updated |
| `submitted` | Object submitted for approval | User who submitted |
| `approved` | Object was approved | User who approved |
| `cancelled` | Object was cancelled | User who cancelled |
| `deleted` | Object was deleted | User who deleted |
| `meeting_scheduled` | Meeting was scheduled (includes meeting_metadata) | system |
| `meeting_cancelled` | Meeting was cancelled | system or user |

### Meeting Metadata Schema

Meeting metadata is stored in modification entries when a meeting is scheduled. This structure captures Microsoft Graph API meeting details.

**Reference:** For detailed Microsoft Graph field mappings, see the `object-model-enhancement` spec.

```json
{
  "type": "object",
  "required": ["meeting_id", "join_url", "start_time", "end_time"],
  "properties": {
    "meeting_id": {
      "type": "string",
      "description": "Microsoft Graph meeting ID"
    },
    "join_url": {
      "type": "string",
      "format": "uri",
      "description": "Microsoft Teams meeting join URL"
    },
    "start_time": {
      "type": "string",
      "format": "date-time",
      "description": "ISO 8601 timestamp for meeting start"
    },
    "end_time": {
      "type": "string",
      "format": "date-time",
      "description": "ISO 8601 timestamp for meeting end"
    },
    "organizer": {
      "type": "string",
      "description": "Email address of meeting organizer"
    },
    "attendees": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Array of attendee email addresses"
    }
  }
}
```

#### Microsoft Graph Fields Captured

The following fields from Microsoft Graph API are captured in meeting metadata:

- `id` → `meeting_id`
- `joinWebUrl` → `join_url`
- `start.dateTime` → `start_time`
- `end.dateTime` → `end_time`
- `organizer.emailAddress.address` → `organizer`
- `attendees[].emailAddress.address` → `attendees[]`

### Attachment Schema

Attachments are files uploaded with announcements and stored in S3.

```json
{
  "type": "object",
  "required": ["name", "s3_key", "size", "uploaded_at"],
  "properties": {
    "name": {
      "type": "string",
      "description": "Original filename"
    },
    "s3_key": {
      "type": "string",
      "description": "S3 key where file is stored (announcements/{announcement-id}/attachments/{filename})"
    },
    "size": {
      "type": "integer",
      "description": "File size in bytes"
    },
    "uploaded_at": {
      "type": "string",
      "format": "date-time",
      "description": "ISO 8601 timestamp when file was uploaded"
    },
    "content_type": {
      "type": "string",
      "description": "MIME type of the file (e.g., 'application/pdf')"
    }
  }
}
```

---

## Validation Rules

### Change Object Validation

1. **change_id Format**: Must match pattern `CHANGE-YYYY-NNN` where YYYY is a 4-digit year and NNN is a 3+ digit sequence number
2. **title Length**: Between 1 and 200 characters
3. **status Values**: Must be one of: draft, pending_approval, approved, cancelled, completed
4. **object_type**: Must be exactly "change"
5. **Timestamps**: All timestamp fields must be valid ISO 8601 format
6. **modifications Array**: Must contain at least one entry (the "created" modification)

### Announcement Object Validation

1. **announcement_id Format**: Must match pattern `{PREFIX}-YYYY-NNN` where:
   - PREFIX is CIC, FIN, or INN
   - YYYY is a 4-digit year
   - NNN is a 3+ digit sequence number
2. **object_type and announcement_type Consistency**:
   - If `announcement_type` is "cic", then `object_type` must be "announcement_cic"
   - If `announcement_type` is "finops", then `object_type` must be "announcement_finops"
   - If `announcement_type` is "innersource", then `object_type` must be "announcement_innersource"
3. **title Length**: Between 1 and 200 characters
4. **summary Length**: Between 1 and 500 characters
5. **customers Array**: Must contain at least one customer code
6. **status Values**: Must be one of: draft, pending_approval, approved, cancelled
7. **Timestamps**: All timestamp fields must be valid ISO 8601 format
8. **modifications Array**: Must contain at least one entry (the "created" modification)

### Modification Entry Validation

1. **modification_type**: Must be one of the defined types
2. **meeting_metadata**: Only present when `modification_type` is "meeting_scheduled"
3. **Chronological Order**: Modifications should be ordered by timestamp (oldest first)
4. **First Entry**: First modification must have type "created"

### Attachment Validation

1. **s3_key Format**: Must follow pattern `announcements/{announcement-id}/attachments/{filename}`
2. **size**: Must be a positive integer
3. **File Size Limits**: Individual files should not exceed 10MB (enforced at upload time)

---

## Examples

### Example 1: Complete Change Object

```json
{
  "object_type": "change",
  "change_id": "CHANGE-2025-001",
  "title": "Security Baseline Updates",
  "description": "Implement new security baseline configurations across all production environments",
  "implementation_plan": "1. Backup current configurations\n2. Apply new security policies\n3. Validate changes\n4. Monitor for 24 hours",
  "schedule": {
    "start_time": "2025-10-20T02:00:00Z",
    "end_time": "2025-10-20T04:00:00Z",
    "timezone": "UTC"
  },
  "affected_customers": ["hts", "cds"],
  "status": "approved",
  "created_by": "user-id-123",
  "created_at": "2025-10-14T09:00:00Z",
  "modifications": [
    {
      "timestamp": "2025-10-14T09:00:00Z",
      "user_id": "user-id-123",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-10-14T14:30:00Z",
      "user_id": "user-id-123",
      "modification_type": "updated",
      "comment": "Updated implementation plan with additional validation steps"
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
        "meeting_id": "AAMkAGI2TG93AAA=",
        "join_url": "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc123",
        "start_time": "2025-10-19T19:00:00Z",
        "end_time": "2025-10-19T20:00:00Z",
        "organizer": "ccoe-admin@example.com",
        "attendees": ["customer1@example.com", "customer2@example.com"]
      }
    },
    {
      "timestamp": "2025-10-15T10:30:00Z",
      "user_id": "user-id-456",
      "modification_type": "approved",
      "comment": "Looks good to proceed"
    }
  ]
}
```

### Example 2: FinOps Announcement with Meeting

```json
{
  "object_type": "announcement_finops",
  "announcement_id": "FIN-2025-001",
  "announcement_type": "finops",
  "title": "FinOps Monthly Report - October 2025",
  "summary": "Monthly cost optimization report showing significant savings opportunities in compute and storage resources.",
  "content": "# October 2025 FinOps Report\n\n## Executive Summary\n\nThis month we identified $50,000 in potential savings...\n\n## Key Findings\n\n1. Underutilized EC2 instances\n2. Unattached EBS volumes\n3. Old snapshots\n\n## Recommendations\n\n...",
  "customers": ["hts", "cds", "fdbus"],
  "status": "approved",
  "include_meeting": true,
  "created_by": "user-id-789",
  "created_at": "2025-10-01T09:00:00Z",
  "attachments": [
    {
      "name": "October_2025_Report.pdf",
      "s3_key": "announcements/FIN-2025-001/attachments/October_2025_Report.pdf",
      "size": 2457600,
      "uploaded_at": "2025-10-01T09:15:00Z",
      "content_type": "application/pdf"
    },
    {
      "name": "Cost_Analysis_Spreadsheet.xlsx",
      "s3_key": "announcements/FIN-2025-001/attachments/Cost_Analysis_Spreadsheet.xlsx",
      "size": 1048576,
      "uploaded_at": "2025-10-01T09:20:00Z",
      "content_type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
    }
  ],
  "modifications": [
    {
      "timestamp": "2025-10-01T09:00:00Z",
      "user_id": "user-id-789",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-10-01T10:00:00Z",
      "user_id": "user-id-789",
      "modification_type": "submitted"
    },
    {
      "timestamp": "2025-10-01T11:00:00Z",
      "user_id": "system",
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGI2TG94BBB=",
        "join_url": "https://teams.microsoft.com/l/meetup-join/19%3ameeting_xyz789",
        "start_time": "2025-10-05T14:00:00Z",
        "end_time": "2025-10-05T15:00:00Z",
        "organizer": "finops-team@example.com",
        "attendees": ["hts-contact@example.com", "cds-contact@example.com", "fdbus-contact@example.com"]
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

### Example 3: CIC Announcement (Draft, No Meeting)

```json
{
  "object_type": "announcement_cic",
  "announcement_id": "CIC-2025-015",
  "announcement_type": "cic",
  "title": "New AWS Best Practices for Serverless Applications",
  "summary": "Updated guidelines for building serverless applications on AWS, including Lambda, API Gateway, and DynamoDB best practices.",
  "content": "# Serverless Best Practices\n\n## Introduction\n\nThe Cloud Innovation Center has updated our serverless application guidelines...\n\n## Lambda Best Practices\n\n1. Use environment variables for configuration\n2. Implement proper error handling\n3. Optimize cold start times\n\n...",
  "customers": ["hts"],
  "status": "draft",
  "include_meeting": false,
  "created_by": "user-id-321",
  "created_at": "2025-10-10T15:30:00Z",
  "attachments": [],
  "modifications": [
    {
      "timestamp": "2025-10-10T15:30:00Z",
      "user_id": "user-id-321",
      "modification_type": "created"
    }
  ]
}
```

### Example 4: InnerSource Announcement

```json
{
  "object_type": "announcement_innersource",
  "announcement_id": "INN-2025-008",
  "announcement_type": "innersource",
  "title": "InnerSource Guild - New Shared Component Library",
  "summary": "Announcing the release of our new shared React component library available for all internal projects.",
  "content": "# New Component Library Release\n\n## Overview\n\nThe InnerSource Guild is excited to announce...\n\n## Available Components\n\n- Button\n- Input\n- Modal\n- DataTable\n- Charts\n\n## Getting Started\n\n```bash\nnpm install @company/component-library\n```\n\n...",
  "customers": ["hts", "cds", "fdbus"],
  "status": "approved",
  "include_meeting": false,
  "created_by": "user-id-555",
  "created_at": "2025-09-28T10:00:00Z",
  "attachments": [
    {
      "name": "Component_Library_Documentation.pdf",
      "s3_key": "announcements/INN-2025-008/attachments/Component_Library_Documentation.pdf",
      "size": 3145728,
      "uploaded_at": "2025-09-28T10:15:00Z",
      "content_type": "application/pdf"
    }
  ],
  "modifications": [
    {
      "timestamp": "2025-09-28T10:00:00Z",
      "user_id": "user-id-555",
      "modification_type": "created"
    },
    {
      "timestamp": "2025-09-28T11:00:00Z",
      "user_id": "user-id-555",
      "modification_type": "submitted"
    },
    {
      "timestamp": "2025-09-28T14:00:00Z",
      "user_id": "user-id-456",
      "modification_type": "approved"
    }
  ]
}
```

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2025-10-14 | Initial schema documentation |

---

## Notes

1. **Backward Compatibility**: When updating schemas, maintain backward compatibility by making new fields optional rather than required.

2. **Migration**: Existing objects without the `object_type` field should be migrated by inferring the type from S3 key prefix or object structure.

3. **Validation**: Frontend and backend should validate objects against these schemas before saving to S3.

4. **Extensions**: Custom fields can be added to objects but should not conflict with defined schema fields.

5. **References**: For implementation details on modification history and meeting metadata, refer to the `object-model-enhancement` specification document.
