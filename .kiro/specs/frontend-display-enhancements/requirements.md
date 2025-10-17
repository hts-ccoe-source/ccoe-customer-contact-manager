# Requirements Document

## Introduction

The CCOE Customer Contact Manager currently has basic change viewing capabilities through a "my-changes" pop-up and a "view changes" interface. This enhancement will improve the user experience by creating dedicated pages for approvals and announcements, enhancing the change details display, and ensuring proper object typing and schema documentation. The goal is to provide CCOE team members and customers with intuitive, purpose-built interfaces for viewing changes, managing approvals, and accessing various types of notifications including FinOps monthly reports, InnerSourcing Guild updates, and CIC/Cloud Enablement announcements.

## Requirements

### Requirement 1: Enhanced Change Details Pop-up

**User Story:** As a CCOE team member viewing my changes, I want an improved change details pop-up with better organization and more comprehensive information display, so that I can quickly understand the full context of each change without navigating to multiple screens.

#### Acceptance Criteria

1. WHEN a user clicks on a change in the "my-changes" list THEN the system SHALL display an enhanced pop-up with improved layout and information hierarchy
2. WHEN displaying change details THEN the pop-up SHALL show all relevant metadata including title, description, implementation plan, schedule, affected customers, and modification history
3. WHEN modification history exists THEN the pop-up SHALL display it in a timeline format with clear visual indicators for different event types
4. WHEN meeting metadata is associated with the change THEN the pop-up SHALL display meeting details with clickable join links
5. WHEN approval information exists THEN the pop-up SHALL prominently display approval status and approver details
6. WHEN the pop-up contains long content THEN it SHALL support scrolling while keeping key information visible
7. WHEN displaying customer information THEN it SHALL show both customer codes and friendly names

### Requirement 2: Remove Legacy "View Changes" Interface

**User Story:** As a system administrator maintaining the codebase, I want to remove the outdated "view changes" interface, so that users aren't confused by multiple ways to view the same information and the codebase is simplified.

#### Acceptance Criteria

1. WHEN the legacy "view changes" interface is identified THEN it SHALL be removed from the HTML files
2. WHEN removing the interface THEN all associated JavaScript functions SHALL be removed or refactored
3. WHEN removing the interface THEN any navigation links or buttons pointing to it SHALL be removed
4. WHEN the removal is complete THEN no broken links or references SHALL remain in the application
5. WHEN users access the application THEN they SHALL only see the new enhanced interfaces for viewing changes

### Requirement 3: Dedicated Approvals Page

**User Story:** As a CCOE team member responsible for approvals, I want a dedicated page that shows all changes organized by customer with their approval status, so that I can efficiently review and approve changes for each customer organization.

#### Acceptance Criteria

1. WHEN accessing the approvals page THEN the system SHALL display changes grouped by customer organization
2. WHEN displaying customer groups THEN each SHALL show the customer friendly name and code
3. WHEN displaying changes within a customer group THEN they SHALL be sorted by submission date with most recent first
4. WHEN a change requires approval THEN it SHALL be clearly indicated with visual styling
5. WHEN a change has been approved THEN it SHALL show approval status, approver name, and approval timestamp
6. WHEN clicking on a change THEN the system SHALL display the enhanced change details pop-up
7. WHEN filtering options are available THEN users SHALL be able to filter by approval status (pending, approved, all)
8. WHEN no changes exist for a customer THEN the system SHALL display an appropriate message
9. WHEN the page loads THEN it SHALL fetch change data from S3 using the appropriate customer prefixes

### Requirement 4: Dedicated Announcements Page

**User Story:** As a CCOE team member or customer, I want a dedicated announcements page that displays various notification types including FinOps monthly reports, InnerSourcing Guild updates, and CIC/Cloud Enablement announcements, so that I can access all important communications in one centralized location.

#### Acceptance Criteria

1. WHEN accessing the announcements page THEN the system SHALL display all announcement objects from S3
2. WHEN displaying announcements THEN they SHALL be organized by announcement type (FinOps, InnerSourcing Guild, CIC/Cloud Enablement, General)
3. WHEN displaying announcements THEN each SHALL show title, date, summary, and announcement type
4. WHEN clicking on an announcement THEN the system SHALL display full details in a modal or expanded view
5. WHEN announcements have associated links or attachments THEN they SHALL be displayed as clickable elements
6. WHEN filtering options are available THEN users SHALL be able to filter by announcement type
7. WHEN sorting options are available THEN users SHALL be able to sort by date (newest/oldest first)
8. WHEN no announcements exist for a type THEN the system SHALL display an appropriate message
9. WHEN the page loads THEN it SHALL fetch objects from S3 customer prefixes and filter by object_type starting with "announcement_"

### Requirement 5: Object Type Field Implementation

**User Story:** As a backend developer working with S3 objects, I want all objects to have an explicit "object_type" field, so that the system can properly identify and route different types of objects (changes, announcements, etc.) without relying on S3 key prefixes alone.

#### Acceptance Criteria

1. WHEN creating a change object THEN the frontend SHALL set object_type to "change"
2. WHEN creating an announcement object THEN the system SHALL set object_type to the appropriate announcement type (e.g., "announcement_finops", "announcement_innersourcing", "announcement_cic")
3. WHEN the backend processes objects THEN it SHALL use the object_type field to determine processing logic
4. WHEN displaying objects in the UI THEN the system SHALL use object_type to apply appropriate styling and behavior
5. WHEN migrating existing objects THEN the system SHALL add the object_type field based on the S3 key prefix or object structure
6. WHEN validating objects THEN the system SHALL require the object_type field to be present and valid
7. WHEN object_type is missing or invalid THEN the system SHALL log a warning and attempt to infer the type from context

### Requirement 6: JSON Schema Documentation

**User Story:** As a developer integrating with the CCOE Customer Contact Manager, I want comprehensive JSON schema documentation for all object types, so that I can understand the expected structure and validate my data before submitting it to the system.

#### Acceptance Criteria

1. WHEN documentation is created THEN it SHALL include JSON schema definitions for change objects
2. WHEN documentation is created THEN it SHALL include JSON schema definitions for announcement objects
3. WHEN documenting schemas THEN each field SHALL have a description, data type, and whether it's required or optional
4. WHEN documenting schemas THEN examples SHALL be provided for each object type
5. WHEN documenting schemas THEN validation rules SHALL be clearly specified (e.g., format constraints, allowed values)
6. WHEN documenting the modification history array THEN it SHALL reference the object-model-enhancement spec for detailed structure
7. WHEN documenting meeting metadata THEN it SHALL specify the Microsoft Graph fields that are captured
8. WHEN the schema is updated THEN the documentation SHALL be versioned and changes SHALL be tracked
9. WHEN developers access the documentation THEN it SHALL be available in a markdown file in the docs/ directory

### Requirement 7: Customer-Centric Data Organization

**User Story:** As a customer viewing the approvals or announcements pages, I want to see only the information relevant to my organization, so that I'm not overwhelmed with data from other customers and can focus on what matters to me.

#### Acceptance Criteria

1. WHEN a customer user accesses the approvals page THEN the system SHALL filter changes to show only their customer code
2. WHEN a customer user accesses the announcements page THEN the system SHALL show announcements relevant to their organization
3. WHEN determining customer context THEN the system SHALL use authentication information or customer selection
4. WHEN a CCOE admin accesses these pages THEN they SHALL see all customers with the ability to filter by customer
5. WHEN displaying customer-specific data THEN the system SHALL use the S3 prefix structure (customers/{customer-code}/)
6. WHEN no customer context is available THEN the system SHALL prompt the user to select their organization
7. WHEN customer context changes THEN the page SHALL refresh to show the appropriate data

### Requirement 8: Responsive Design and Accessibility

**User Story:** As a user accessing the CCOE Customer Contact Manager from various devices, I want all new pages and enhancements to work well on desktop, tablet, and mobile devices with proper accessibility support, so that I can effectively use the system regardless of my device or accessibility needs.

#### Acceptance Criteria

1. WHEN accessing pages on mobile devices THEN the layout SHALL adapt to smaller screen sizes
2. WHEN accessing pages on tablets THEN the layout SHALL provide an optimal viewing experience
3. WHEN using keyboard navigation THEN all interactive elements SHALL be accessible via keyboard
4. WHEN using screen readers THEN all content SHALL have appropriate ARIA labels and semantic HTML
5. WHEN displaying tables or lists THEN they SHALL be responsive and usable on small screens
6. WHEN pop-ups or modals are displayed THEN they SHALL be properly sized for the viewport
7. WHEN color is used to convey information THEN it SHALL not be the only indicator (support for color blindness)

### Requirement 9: Performance and Loading States

**User Story:** As a user accessing pages with potentially large amounts of data, I want clear loading indicators and good performance, so that I understand when data is being fetched and the interface remains responsive even with many changes or announcements.

#### Acceptance Criteria

1. WHEN pages are loading data from S3 THEN they SHALL display loading indicators
2. WHEN data fetching takes longer than 2 seconds THEN a progress indicator SHALL be shown
3. WHEN large lists are displayed THEN the system SHALL implement pagination or virtual scrolling
4. WHEN filtering or sorting data THEN the operations SHALL complete within 500ms for typical datasets
5. WHEN errors occur during data fetching THEN user-friendly error messages SHALL be displayed
6. WHEN retrying failed requests THEN the system SHALL use exponential backoff
7. WHEN data is successfully loaded THEN loading indicators SHALL be removed and content SHALL be displayed smoothly

### Requirement 10: Navigation and User Flow

**User Story:** As a user navigating the CCOE Customer Contact Manager, I want clear navigation between the my-changes view, approvals page, and announcements page, so that I can easily access the information I need without getting lost.

#### Acceptance Criteria

1. WHEN the application loads THEN it SHALL provide a navigation menu with links to My Changes, Approvals, and Announcements
2. WHEN on any page THEN the current page SHALL be visually indicated in the navigation
3. WHEN clicking navigation links THEN the system SHALL load the appropriate page without full page reloads (SPA behavior)
4. WHEN using browser back/forward buttons THEN the navigation SHALL work correctly
5. WHEN accessing a deep link THEN the system SHALL load the appropriate page directly
6. WHEN navigation occurs THEN the page title SHALL update to reflect the current view
7. WHEN on mobile devices THEN the navigation SHALL collapse into a hamburger menu or similar mobile-friendly pattern

### Requirement 11: Create Announcements Page

**User Story:** As a CCOE team member, I want to create announcements of different types (CIC, FinOps, InnerSource) with optional meeting scheduling and file attachments, so that I can communicate important information to customers through the appropriate channels.

#### Acceptance Criteria

1. WHEN accessing the create announcements page THEN the system SHALL display a form with fields for announcement type, title, summary, content, customer selection, and optional meeting
2. WHEN selecting announcement type THEN the system SHALL provide options for CIC, FinOps, and InnerSource
3. WHEN the announcement type is CIC THEN the system SHALL generate an ID with prefix "CIC-"
4. WHEN the announcement type is FinOps THEN the system SHALL generate an ID with prefix "FIN-"
5. WHEN the announcement type is InnerSource THEN the system SHALL generate an ID with prefix "INN-"
6. WHEN creating an announcement THEN the user SHALL be able to select one or more customers similar to change creation
7. WHEN customers are selected THEN the system SHALL display customer codes and friendly names
8. WHEN creating an announcement THEN the user SHALL be able to select yes/no for including a meeting
9. WHEN meeting is selected as yes THEN the system SHALL display meeting scheduling fields similar to change creation
10. WHEN meeting is scheduled THEN the system SHALL create a Microsoft Teams meeting and store meeting metadata
11. WHEN creating an announcement THEN the user SHALL be able to upload file attachments
12. WHEN file attachments are uploaded THEN they SHALL be stored in S3 under a new key prefix "announcements/{announcement-id}/attachments/"
13. WHEN an announcement is created THEN it SHALL have a status field with initial value "draft"
14. WHEN an announcement is submitted for approval THEN the status SHALL change to "submitted"
15. WHEN an announcement is approved THEN the frontend upload_lambda API SHALL change the status to "approved" and the backend Go Lambda SHALL schedule meetings if requested and send email notifications with calendar invites
16. WHEN an announcement is cancelled THEN the status SHALL change to "cancelled"
17. WHEN the announcement is submitted THEN the system SHALL set object_type to "announcement_{type}" (e.g., "announcement_cic", "announcement_finops", "announcement_innersource")
18. WHEN the announcement is saved THEN it SHALL be stored in S3 under the appropriate customer prefix for each selected customer
19. WHEN an announcement goes through status changes THEN the system SHALL add modification entries to the modifications array similar to changes

### Requirement 12: Backend Email Templates for Announcements

**User Story:** As a system administrator, I want the backend Go API to support type-specific email templates for announcements, so that each announcement type (CIC, FinOps, InnerSource) can have appropriately formatted email notifications.

#### Acceptance Criteria

1. WHEN the backend processes a CIC announcement THEN it SHALL use the CIC email template
2. WHEN the backend processes a FinOps announcement THEN it SHALL use the FinOps email template
3. WHEN the backend processes an InnerSource announcement THEN it SHALL use the InnerSource email template
4. WHEN an email template is used THEN it SHALL include announcement title, summary, content, and meeting details if applicable
5. WHEN an email template is used THEN it SHALL include links to file attachments if present
6. WHEN an email template is used THEN it SHALL include appropriate branding and styling for the announcement type
7. WHEN sending announcement emails THEN the backend Go Lambda SHALL use AWS SES topic management with the appropriate sender address, similar to change management announcements
8. WHEN an announcement has a meeting THEN the email SHALL include the meeting join link and schedule details, similar to the change management feature

### Requirement 13: Announcement Action Buttons and Status Management

**User Story:** As a CCOE team member managing announcements, I want the same action buttons (Approve, Cancel, Complete) available for announcements as we have for changes, so that I can manage the announcement lifecycle consistently with the change management workflow.

#### Acceptance Criteria

1. WHEN viewing an announcement in the approvals page THEN the system SHALL display action buttons based on the announcement status
2. WHEN an announcement has status "submitted" THEN the system SHALL display "Approve" and "Cancel" buttons
3. WHEN an announcement has status "approved" THEN the system SHALL display "Complete" and "Cancel" buttons
4. WHEN an announcement has status "completed" or "cancelled" THEN no action buttons SHALL be displayed
5. WHEN clicking the "Approve" button THEN the system SHALL update the announcement status to "approved" and add a modification entry
6. WHEN clicking the "Cancel" button THEN the system SHALL update the announcement status to "cancelled" and add a modification entry
7. WHEN clicking the "Complete" button THEN the system SHALL update the announcement status to "completed" and add a modification entry
8. WHEN an announcement is approved THEN the backend SHALL trigger email notifications and meeting scheduling if configured
9. WHEN an announcement is cancelled THEN the backend SHALL cancel any scheduled meetings
10. WHEN updating announcement status THEN the system SHALL use the frontend upload_lambda API endpoint
11. WHEN the upload_lambda API receives an announcement update THEN it SHALL update the S3 object for all affected customers
12. WHEN status changes occur THEN the system SHALL add appropriate modification entries with timestamp, user_id, and modification_type
13. WHEN action buttons are clicked THEN the system SHALL show loading indicators and disable buttons during processing
14. WHEN status update succeeds THEN the system SHALL show success message and refresh the view
15. WHEN status update fails THEN the system SHALL show error message and allow retry
16. WHEN viewing announcement details modal THEN action buttons SHALL also be available in the modal footer
17. WHEN a user lacks permission to perform an action THEN the action buttons SHALL be disabled or hidden

### Requirement 14: Edit Announcement Page

**User Story:** As a CCOE team member, I want to edit existing announcements and duplicate announcements to create new ones, so that I can update draft announcements or quickly create similar announcements without starting from scratch.

#### Acceptance Criteria

1. WHEN accessing the edit announcement page with an announcement ID THEN the system SHALL load the announcement data from S3
2. WHEN the announcement is loaded THEN the system SHALL populate all form fields with the existing announcement data
3. WHEN editing an announcement THEN the system SHALL display announcement information header showing announcement ID, type, status, and creation date
4. WHEN editing a draft announcement THEN the user SHALL be able to modify all fields including type, title, summary, content, customers, and meeting details
5. WHEN editing a submitted or approved announcement THEN the system SHALL create a new version with updated modification history
6. WHEN duplicating an announcement THEN the system SHALL load the original announcement data but generate a new announcement ID
7. WHEN duplicating an announcement THEN the page title SHALL indicate "Duplicate Announcement" mode
8. WHEN duplicating an announcement THEN the system SHALL clear status-related fields and set status to "draft"
9. WHEN duplicating an announcement with meeting metadata THEN the system SHALL preserve meeting time and duration but clear meeting URL and meeting ID
10. WHEN saving an edited announcement THEN the system SHALL validate all required fields
11. WHEN saving an edited announcement THEN the system SHALL update the S3 object for all selected customers
12. WHEN saving an edited announcement THEN the system SHALL add a modification entry with type "updated"
13. WHEN file attachments exist THEN the system SHALL display them with options to remove or add new attachments
14. WHEN meeting metadata exists THEN the system SHALL display meeting information and allow updates
15. WHEN the announcement type is changed THEN the system SHALL update the announcement ID prefix accordingly
16. WHEN customer selection is changed THEN the system SHALL handle adding/removing S3 objects for affected customers
17. WHEN canceling the edit THEN the system SHALL return to the previous page without saving changes
18. WHEN preview is clicked THEN the system SHALL display a preview of the announcement as it would appear to recipients


### Requirement 15: Separate Announcement and Change Processing in Backend

**User Story:** As a backend developer maintaining data integrity, I want announcements to be processed as AnnouncementMetadata throughout their entire lifecycle without conversion to ChangeMetadata, so that announcement-specific fields are preserved and announcements don't lose data when their status changes.

#### Acceptance Criteria

1. WHEN the backend reads an announcement from S3 THEN it SHALL parse it as AnnouncementMetadata and keep it in that format throughout processing
2. WHEN the backend processes announcement events THEN it SHALL use announcement-specific handler functions instead of converting to ChangeMetadata
3. WHEN the backend saves an announcement back to S3 THEN it SHALL serialize the AnnouncementMetadata structure preserving all announcement-specific fields
4. WHEN an announcement status changes (submitted, approved, cancelled, completed) THEN the system SHALL update the AnnouncementMetadata object and save it without data loss
5. WHEN an announcement is cancelled THEN the system SHALL preserve the announcement_id, title, summary, and content fields
6. WHEN the backend schedules meetings for announcements THEN it SHALL work with AnnouncementMetadata without requiring conversion to ChangeMetadata
7. WHEN the backend sends emails for announcements THEN it SHALL extract data from AnnouncementMetadata fields (announcement_id, title, summary, content) not ChangeMetadata fields (changeId, changeTitle, changeReason)
8. WHEN the backend processes modifications for announcements THEN it SHALL update the AnnouncementMetadata.Modifications array
9. WHEN displaying announcements in the frontend THEN the system SHALL receive properly structured AnnouncementMetadata with all fields intact
10. WHEN creating handler functions for announcements THEN they SHALL accept AnnouncementMetadata type parameters
11. WHEN email template functions process announcements THEN they SHALL accept AnnouncementMetadata and extract fields directly without intermediate conversion
12. WHEN meeting creation functions process announcements THEN they SHALL accept AnnouncementMetadata and map fields appropriately for Microsoft Graph API
13. WHEN the backend updates announcement meeting metadata THEN it SHALL update the AnnouncementMetadata.MeetingMetadata field not ChangeMetadata fields

### Requirement 16: Data Cleanup for Announcement Migration

**User Story:** As a system administrator deploying the new announcement architecture, I want to delete all existing broken announcements and start fresh, so that we have a clean slate with proper data integrity from the beginning.

#### Acceptance Criteria

1. WHEN deploying the new announcement architecture THEN all existing announcement objects SHALL be deleted from S3
2. WHEN deleting announcements THEN the system SHALL identify objects by object_type starting with "announcement_"
3. WHEN deleting announcements THEN the system SHALL remove them from all customer prefixes
4. WHEN announcements are deleted THEN no migration or data recovery SHALL be attempted
5. WHEN the new architecture is deployed THEN all new announcements SHALL be created with proper AnnouncementMetadata structure
6. WHEN the cleanup is complete THEN the system SHALL verify no announcement objects remain in S3

