# Requirements Document

## Introduction

This spec addresses the confusion caused by multiple status fields in the S3 object structure. Currently, there are three different status-related fields that can conflict:
1. `status` (top-level field) - The authoritative current state
2. `metadata.status` (nested field) - Stale status from previous state
3. `metadata.request_type` (nested field) - Stale request type from previous state

This creates bugs where the backend uses stale values instead of the current status, causing incorrect email notifications and workflow behavior.

## Glossary

- **System**: The AWS Customer Contact Manager backend Lambda
- **Status**: The current state of a change or announcement in the workflow
- **Request Type**: The type of email notification to send based on the current status
- **S3 Object**: The JSON document stored in S3 representing a change or announcement

## Requirements

### Requirement 1

**User Story:** As a system administrator, I want a single authoritative status field, so that there is no confusion about the current state of a change or announcement.

#### Acceptance Criteria

1. THE System SHALL use only the top-level `status` field to determine the current state
2. THE System SHALL NOT use any nested `metadata` object for status determination
3. WHEN determining the request type, THE System SHALL derive it from the top-level `status` field only

### Requirement 2

**User Story:** As a developer, I want to eliminate the metadata map entirely, so that there is no confusion or conflict with top-level fields.

#### Acceptance Criteria

1. THE System SHALL NOT write a `metadata` map to S3 objects
2. THE System SHALL NOT read from a `metadata` map when processing objects
3. WHEN writing to S3, THE System SHALL use only top-level fields
4. WHERE status tracking is needed, THE System SHALL use a top-level `prior_status` field

### Requirement 3

**User Story:** As a system operator, I want consistent behavior between changes and announcements, so that the workflow is predictable.

#### Acceptance Criteria

1. THE System SHALL apply the same status determination logic to both changes and announcements
2. THE System SHALL use the same status values for both changes and announcements
3. THE System SHALL derive request types from status using the same logic for both object types
4. THE System SHALL only differ in email topics and metadata fields between changes and announcements

### Requirement 4

**User Story:** As a backend developer, I want the request type determination to be simple and reliable, so that the correct emails are sent.

#### Acceptance Criteria

1. WHEN status is "submitted", THE System SHALL send approval request emails
2. WHEN status is "approved", THE System SHALL send approved announcement emails and schedule meetings
3. WHEN status is "completed", THE System SHALL send completion emails
4. WHEN status is "cancelled", THE System SHALL send cancellation emails and cancel meetings
5. THE System SHALL NOT use any other fields to determine request type

### Requirement 5

**User Story:** As a system maintainer, I want a clean migration without legacy data, so that the system starts fresh with the new structure.

#### Acceptance Criteria

1. THE System SHALL NOT support reading objects with legacy `metadata` maps
2. THE System SHALL expect all objects to use only top-level fields
3. WHEN encountering an object with a `metadata` map, THE System SHALL log an error
4. THE System SHALL NOT write a `metadata` map under any circumstances
