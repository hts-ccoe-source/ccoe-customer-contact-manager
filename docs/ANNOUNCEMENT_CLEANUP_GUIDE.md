# Announcement Cleanup Guide

## Overview

This guide documents the process of cleaning up broken announcement objects from S3 as part of the announcement architecture refactoring (Task 22.6).

## Background

The previous announcement implementation converted `AnnouncementMetadata` to `ChangeMetadata` for processing, which caused data loss and the "Untitled announcement" bug. The new architecture processes announcements directly as `AnnouncementMetadata` throughout their entire lifecycle.

## Why Cleanup is Necessary

Existing announcement objects in S3 may have:
- Missing or corrupted `title`, `summary`, and `content` fields
- Incorrect data structure from conversion issues
- Incomplete `AnnouncementMetadata` fields

Rather than attempting to migrate these broken objects, we're starting fresh with a clean slate.

## Cleanup Process

### Prerequisites

- AWS CLI configured with appropriate credentials
- Access to the S3 bucket containing customer data
- `jq` command-line JSON processor installed

### Running the Cleanup Script

The cleanup script is located at `scripts/delete-broken-announcements.sh`.

#### Dry Run (Recommended First Step)

```bash
# Set your bucket name
export S3_BUCKET_NAME="your-bucket-name"

# Run in dry-run mode to see what would be deleted
./scripts/delete-broken-announcements.sh
```

This will:
- Scan all objects in the `customers/` prefix
- Identify objects with `object_type` starting with "announcement_"
- Display what would be deleted without actually deleting anything

#### Actual Deletion

```bash
# Set your bucket name
export S3_BUCKET_NAME="your-bucket-name"

# Run with actual deletion
DRY_RUN=false ./scripts/delete-broken-announcements.sh
```

This will:
- Delete all identified announcement objects
- Provide a summary of deletions
- Verify that no announcement objects remain

### What Gets Deleted

The script deletes any S3 object where:
- The object key starts with `customers/`
- The `object_type` field starts with `announcement_`

This includes all announcement types:
- `announcement_cic`
- `announcement_finops`
- `announcement_innersource`
- `announcement_general`

### What Doesn't Get Deleted

- Change objects (`object_type: "change"`)
- Archive objects
- Any objects outside the `customers/` prefix
- Objects without an `object_type` field

## Post-Cleanup

After cleanup:

1. **Verify Cleanup**: The script automatically verifies that no announcement objects remain
2. **Deploy New Code**: Deploy the updated Lambda functions with the new `AnnouncementProcessor`
3. **Create New Announcements**: All new announcements will use the proper `AnnouncementMetadata` structure
4. **Monitor**: Watch for any "Untitled announcement" issues (should be resolved)

## New Announcement Architecture

### Key Changes

1. **Direct Processing**: Announcements are processed as `AnnouncementMetadata` without conversion
2. **Separate Handlers**: New `announcement_handlers.go` with announcement-specific logic
3. **Dedicated Processor**: `AnnouncementProcessor` in `internal/processors/`
4. **Preserved Fields**: All announcement fields (`title`, `summary`, `content`) are preserved throughout processing

### Data Flow

```
S3 Event → handleAnnouncementEventNew() → downloadAnnouncementFromS3() 
→ AnnouncementProcessor.ProcessAnnouncement() → Email Templates
```

No conversion to `ChangeMetadata` occurs at any point.

## Rollback Plan

If issues arise after cleanup:

1. **Restore from Backup**: If you created S3 versioning or backups before cleanup
2. **Revert Code**: Deploy the previous Lambda version
3. **Contact Team**: Escalate to the development team

## Monitoring

After deployment, monitor:

- CloudWatch Logs for announcement processing
- Email delivery success rates
- User reports of missing announcements
- "Untitled announcement" occurrences (should be zero)

## Support

For issues or questions:
- Check CloudWatch Logs for error messages
- Review the `AnnouncementProcessor` code in `internal/processors/announcement_processor.go`
- Check email template rendering in `internal/ses/announcement_templates.go`

## Related Documentation

- [Object Model Enhancement Spec](.kiro/specs/object-model-enhancement/)
- [Frontend Display Enhancements Spec](.kiro/specs/frontend-display-enhancements/)
- [Announcement Templates](../internal/ses/announcement_templates.go)
- [Announcement Processor](../internal/processors/announcement_processor.go)
