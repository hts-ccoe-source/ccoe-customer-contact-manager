# Implementation Plan

- [x] 1. Add object_type field to data structures
  - Add object_type field to change creation forms (create-change.html, edit-change.html)
  - Set object_type to "change" when creating/updating change objects
  - _Requirements: 5.1, 5.2, 5.6_

- [x] 2. Create shared JavaScript modules
- [x] 2.1 Create modal component module
  - Write html/assets/js/modal.js with reusable modal functionality
  - Implement show(), hide(), and render() methods
  - Add keyboard navigation support (ESC to close, tab trapping)
  - _Requirements: 1.6, 8.3_

- [x] 2.2 Create timeline component module
  - Write html/assets/js/timeline.js for modification history display
  - Implement renderTimeline() to display modifications array
  - Add icons and formatting for different modification types
  - _Requirements: 1.3, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8_

- [x] 2.3 Create S3 client module
  - Write html/assets/js/s3-client.js for S3 data operations
  - Implement fetchObjects() with retry logic and exponential backoff
  - Add caching mechanism (5-minute cache)
  - _Requirements: 9.1, 9.2, 9.3, 9.4_

- [x] 2.4 Create filter utilities module
  - Write html/assets/js/filters.js for filtering and sorting
  - Implement filterByStatus(), filterByCustomer(), filterByType()
  - Add debouncing for filter inputs (300ms)
  - _Requirements: 3.7, 4.6, 4.7_

- [x] 2.5 Create loading states module
  - Write html/assets/js/loading.js for loading indicators
  - Implement showLoading(), hideLoading(), showProgress()
  - Add error display functionality
  - _Requirements: 9.1, 9.2, 9.5_

- [x] 3. Enhance my-changes.html with new modal
- [x] 3.1 Create enhanced change details modal component
  - Write html/assets/js/change-details-modal.js
  - Implement modal structure with sections: Header, Details, Timeline, Meetings, Approvals
  - Add renderMeetingInfo() to display meeting metadata when available
  - Add renderApprovalStatus() to display approval information
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 3.2 Integrate modal into my-changes.html
  - Update my-changes.html to use new ChangeDetailsModal
  - Replace existing click handlers to open enhanced modal
  - Test modal with various change states (draft, submitted, approved)
  - _Requirements: 1.1, 1.7_

- [x] 4. Create approvals page
- [x] 4.1 Create approvals.html page structure
  - Create html/approvals.html with navigation and container
  - Add filter controls (status, customer, date range)
  - Add empty state messaging
  - _Requirements: 3.1, 3.7, 3.8, 10.1_

- [x] 4.2 Create approvals page JavaScript
  - Write html/assets/js/approvals-page.js
  - Implement loadChanges() to fetch from S3 and filter by object_type "change"
  - Implement groupByCustomer() to organize changes by customer code
  - Implement renderCustomerSection() for collapsible customer groups
  - _Requirements: 3.2, 3.9, 7.1, 7.5_

- [x] 4.3 Implement approval and cancel actions
  - Add handleApprovalAction() to update change status to "approved"
  - Add handleCancelAction() to update change status to "cancelled"
  - Update S3 object with new modification entry
  - Refresh view after action completes
  - _Requirements: 3.3, 3.4, 3.5, 3.6_

- [x] 4.4 Add customer filtering for non-admin users
  - Implement customer context detection from authentication
  - Filter changes to show only user's customer when not admin
  - Show all customers with filter dropdown for admin users
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.6_

- [x] 5. Create announcements page
- [x] 5.1 Create announcements.html page structure
  - Create html/announcements.html with navigation and container
  - Add type filter controls (FinOps, InnerSourcing, CIC, General, All)
  - Add sort controls (newest/oldest first)
  - Add empty state messaging
  - _Requirements: 4.1, 4.6, 4.7, 4.8, 10.1_

- [x] 5.2 Create announcements page JavaScript
  - Write html/assets/js/announcements-page.js
  - Implement loadAnnouncements() to fetch objects and filter by object_type starting with "announcement_"
  - Implement renderAnnouncementCard() with type icons
  - Implement getTypeIcon() to return appropriate icon for each announcement type
  - _Requirements: 4.2, 4.3, 4.9_

- [x] 5.3 Create announcement details modal
  - Implement renderAnnouncementDetails() to show full announcement
  - Display attachments as clickable links
  - Display related links
  - Show full content with proper formatting
  - _Requirements: 4.4, 4.5_

- [x] 5.4 Add customer filtering for announcements
  - Filter announcements to show only relevant to user's customer
  - Show all announcements for admin users
  - _Requirements: 7.2, 7.3, 7.4_

- [x] 6. Update navigation across all pages
- [x] 6.1 Update navigation in all HTML files
  - Update index.html navigation to include Approvals and Announcements
  - Update create-change.html navigation
  - Update edit-change.html navigation
  - Update my-changes.html navigation
  - Update search-changes.html navigation
  - _Requirements: 10.1, 10.2_

- [x] 6.2 Add active page highlighting
  - Implement JavaScript to detect current page
  - Add "active" class to current nav link
  - Update page title dynamically
  - _Requirements: 10.2, 10.6_

- [x] 6.3 Implement mobile navigation
  - Add hamburger menu for mobile devices
  - Make navigation responsive with CSS media queries
  - Test navigation on mobile viewport
  - _Requirements: 8.1, 8.2, 10.7_

- [x] 7. Remove legacy view-changes interface
- [x] 7.1 Remove view-changes.html file
  - Delete html/view-changes.html
  - _Requirements: 2.1_

- [x] 7.2 Remove references to view-changes
  - Remove navigation links to view-changes.html from all pages
  - Remove any JavaScript functions specific to view-changes
  - Search codebase for any remaining references
  - _Requirements: 2.2, 2.3, 2.4_

- [x] 7.3 Verify no broken links
  - Test all navigation links
  - Verify no 404 errors
  - _Requirements: 2.4, 2.5_

- [ ] 8. Implement responsive design
- [x] 8.1 Add responsive CSS for new pages
  - Add media queries for approvals.html (mobile, tablet, desktop)
  - Add media queries for announcements.html (mobile, tablet, desktop)
  - Ensure modals are responsive and properly sized
  - _Requirements: 8.1, 8.2, 8.6_

- [x] 8.2 Add accessibility features
  - Add ARIA labels to all interactive elements
  - Ensure keyboard navigation works throughout
  - Add focus indicators for keyboard users
  - Test with screen reader
  - _Requirements: 8.3, 8.4, 8.7_

- [x] 8.3 Test responsive layouts
  - Test on desktop (1200px+)
  - Test on tablet (768px-1199px)
  - Test on mobile (<768px)
  - Verify tables/lists are usable on small screens
  - _Requirements: 8.1, 8.2, 8.5_

- [ ] 9. Implement error handling and loading states
- [ ] 9.1 Add error handling to all S3 operations
  - Implement try-catch blocks in all fetch operations
  - Display user-friendly error messages
  - Log detailed errors for debugging
  - _Requirements: 9.2, 9.5_

- [ ] 9.2 Add loading indicators
  - Show loading spinner when fetching data
  - Show progress indicator for operations > 2 seconds
  - Remove loading indicators when data loads
  - _Requirements: 9.1, 9.2, 9.7_

- [ ] 9.3 Implement retry logic
  - Add exponential backoff for failed S3 requests
  - Retry up to 3 times before showing error
  - _Requirements: 9.6_

- [x] 10. Create JSON schema documentation
- [x] 10.1 Document change object schema
  - Create docs/json-schemas.md
  - Document all fields in change object
  - Include object_type field specification
  - Document modifications array structure (reference object-model-enhancement spec)
  - Provide example change object
  - _Requirements: 6.1, 6.3, 6.4, 6.5, 6.6_

- [x] 10.2 Document announcement object schema
  - Document all fields in announcement object
  - Document different announcement types (finops, innersourcing, cic, general)
  - Provide example announcement objects for each type
  - _Requirements: 6.2, 6.3, 6.4, 6.5_

- [x] 10.3 Document meeting metadata structure
  - Document Microsoft Graph meeting fields
  - Reference object-model-enhancement spec for detailed structure
  - _Requirements: 6.7_

- [x] 10.4 Add schema versioning
  - Add version number to schema documentation
  - Document change tracking process
  - _Requirements: 6.8, 6.9_

- [ ] 11. Integration and testing
- [ ] 11.1 Test enhanced modal integration
  - Test modal with changes that have modifications array
  - Test modal with changes that have meeting metadata
  - Test modal with changes that have approval information
  - Verify timeline displays correctly
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [ ] 11.2 Test approvals page functionality
  - Test loading changes from S3
  - Test customer grouping
  - Test filtering by status
  - Test approval action
  - Test cancel action
  - _Requirements: 3.1, 3.2, 3.3, 3.7_

- [ ] 11.3 Test announcements page functionality
  - Test loading announcements from S3
  - Test filtering by type
  - Test sorting by date
  - Test announcement details modal
  - _Requirements: 4.1, 4.2, 4.6, 4.7_

- [ ] 11.4 Test navigation flow
  - Test navigation between all pages
  - Test browser back/forward buttons
  - Test deep linking
  - Verify active page highlighting
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

- [ ] 11.5 Test customer filtering
  - Test as customer user (should see only their data)
  - Test as admin user (should see all data with filters)
  - Test customer context switching
  - _Requirements: 7.1, 7.2, 7.3, 7.4_

- [ ] 11.6 Cross-browser testing
  - Test on Chrome
  - Test on Firefox
  - Test on Safari
  - Test on Edge
  - _Requirements: 8.1, 8.2_

- [ ] 12. Performance optimization
- [ ] 12.1 Implement pagination for large datasets
  - Add pagination to approvals page (20 items per page)
  - Add pagination to announcements page (20 items per page)
  - Test with large datasets (>100 items)
  - _Requirements: 9.3_

- [ ] 12.2 Implement caching
  - Cache S3 responses for 5 minutes
  - Implement cache invalidation on updates
  - _Requirements: 9.4_

- [ ] 12.3 Optimize filter operations
  - Ensure filters complete within 500ms
  - Add debouncing to text inputs
  - _Requirements: 9.4_

- [x] 13. Create announcement creation page
- [x] 13.1 Create create-announcement.html page structure
  - Create html/create-announcement.html with navigation and form container
  - Add announcement type selection (CIC, FinOps, InnerSource)
  - Add title, summary, and content fields
  - Add customer selection checkboxes similar to create-change.html
  - Add meeting toggle (Yes/No)
  - Add file attachment upload area
  - _Requirements: 11.1, 11.2, 11.6, 11.7, 11.8, 11.11_

- [x] 13.2 Create announcement page JavaScript
  - Write html/assets/js/create-announcement-page.js
  - Implement generateAnnouncementId() with type-specific prefixes (CIC-, FIN-, INN-)
  - Implement handleTypeChange() to set object_type to "announcement_{type}"
  - Implement handleMeetingToggle() to show/hide meeting fields
  - _Requirements: 11.3, 11.4, 11.5, 11.8, 11.9, 11.17_

- [x] 13.3 Implement file attachment handling
  - Implement handleFileUpload() to upload files to S3
  - Store files under "announcements/{announcement-id}/attachments/" prefix
  - Track attachment metadata (name, s3_key, size, uploaded_at)
  - Display uploaded files with remove option
  - _Requirements: 11.11, 11.12_

- [x] 13.4 Implement save and submit functionality
  - Implement saveDraft() to save with status "draft"
  - Implement submitForApproval() to save with status "submitted"
  - Add modification entries for created and submitted events
  - Save to S3 under each selected customer prefix
  - _Requirements: 11.13, 11.14, 11.18, 11.19_

- [x] 13.5 Integrate meeting scheduling fields
  - Reuse meeting field components from create-change.html
  - Show/hide based on meeting toggle
  - Store meeting preference in announcement object
  - _Requirements: 11.9, 11.10_

- [x] 14. Update approvals page for announcements
- [x] 14.1 Extend approvals page to show announcements
  - Update html/assets/js/approvals-page.js to load both changes and announcements
  - Filter objects by object_type (change vs announcement_*)
  - Display announcements in separate section or mixed with changes
  - _Requirements: 11.14_

- [x] 14.2 Add announcement approval actions
  - Implement approval action for announcements
  - Update status from "submitted" to "approved"
  - Add modification entry for approval
  - _Requirements: 11.15_

- [x] 15. Implement backend email templates
- [x] 15.1 Create announcement email template module
  - Create internal/ses/announcement_templates.go
  - Implement GetAnnouncementTemplate() function
  - Create getCICTemplate(), getFinOpsTemplate(), getInnerSourceTemplate()
  - _Requirements: 12.1, 12.2, 12.3_

- [x] 15.2 Design CIC email template
  - Create HTML template with CIC branding (blue theme)
  - Include title, summary, content sections
  - Add meeting details section (conditional)
  - Add attachments section (conditional)
  - _Requirements: 12.4, 12.6, 12.8, 12.9_

- [x] 15.3 Design FinOps email template
  - Create HTML template with FinOps branding (green theme)
  - Include title, summary, content sections
  - Add meeting details section (conditional)
  - Add attachments section (conditional)
  - _Requirements: 12.4, 12.6, 12.8, 12.9_

- [x] 15.4 Design InnerSource email template
  - Create HTML template with InnerSource branding (purple theme)
  - Include title, summary, content sections
  - Add meeting details section (conditional)
  - Add attachments section (conditional)
  - _Requirements: 12.4, 12.6, 12.8, 12.9_

- [x] 15.5 Add attachment links to templates
  - Include clickable links to S3 attachments in all templates
  - Format file size and name appropriately
  - _Requirements: 12.5_

- [x] 16. Update backend Lambda for announcement processing
- [x] 16.1 Update S3 event handler
  - Modify internal/lambda/handlers.go HandleS3Event()
  - Detect objects with object_type starting with "announcement_"
  - Route to new handleAnnouncementEvent() function
  - _Requirements: 11.17_

- [x] 16.2 Implement announcement event handler
  - Create handleAnnouncementEvent() in internal/lambda/handlers.go
  - Check if status == "approved"
  - Call meeting scheduling if include_meeting == true
  - Call email sending function
  - _Requirements: 11.15, 11.10_

- [x] 16.3 Implement meeting scheduling for announcements
  - Reuse existing Microsoft Graph API integration
  - Create Teams meeting when announcement is approved
  - Update S3 object with meeting_metadata
  - Add modification entry for "meeting_scheduled"
  - _Requirements: 11.10_

- [x] 16.4 Implement announcement email sending
  - Create sendAnnouncementEmails() in internal/ses/announcement_emails.go
  - Extract announcement type from object_type field
  - Load appropriate template based on type
  - Send emails via SES topic management (same as changes)
  - Include meeting join link if applicable
  - _Requirements: 12.7, 12.8, 12.9_

- [x] 17. Update navigation for create announcement
- [x] 17.1 Add create announcement link to all pages
  - Update navigation in index.html
  - Update navigation in create-change.html
  - Update navigation in edit-change.html
  - Update navigation in my-changes.html
  - Update navigation in approvals.html
  - Update navigation in announcements.html
  - Update navigation in search-changes.html
  - _Requirements: 11.1_

- [ ] 18. Testing and integration
- [ ] 18.1 Test announcement creation workflow
  - Test creating CIC announcement with CIC- prefix
  - Test creating FinOps announcement with FIN- prefix
  - Test creating InnerSource announcement with INN- prefix
  - Test customer selection
  - Test file attachment upload
  - Test meeting toggle and fields
  - _Requirements: 11.3, 11.4, 11.5, 11.6, 11.11_

- [ ] 18.2 Test announcement approval workflow
  - Test submitting announcement for approval
  - Test approving announcement from approvals page
  - Verify status changes correctly
  - Verify modification history is tracked
  - _Requirements: 11.13, 11.14, 11.15, 11.19_

- [ ] 18.3 Test backend email sending
  - Test CIC email template rendering
  - Test FinOps email template rendering
  - Test InnerSource email template rendering
  - Verify emails sent via SES topic management
  - Verify meeting links included when applicable
  - Verify attachment links included when applicable
  - _Requirements: 12.1, 12.2, 12.3, 12.7, 12.8, 12.9_

- [ ] 18.4 Test backend meeting scheduling
  - Test meeting creation when announcement approved with meeting=yes
  - Verify meeting metadata stored in S3
  - Verify modification entry added
  - Verify meeting join link in email
  - _Requirements: 11.10, 11.15_

- [ ] 18.5 End-to-end announcement testing
  - Create announcement → submit → approve → verify email sent
  - Create announcement with meeting → verify meeting scheduled
  - Create announcement with attachments → verify links in email
  - Test for multiple customers
  - _Requirements: 11.14, 11.15, 11.19_

- [-] 19. Implement announcement action buttons
- [x] 19.1 Create announcement-actions.js module
  - Write html/assets/js/announcement-actions.js
  - Implement AnnouncementActions class with status-based button rendering
  - Implement approveAnnouncement(), cancelAnnouncement(), completeAnnouncement()
  - Add status transition validation
  - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6, 13.7_

- [x] 19.2 Integrate action buttons into approvals page
  - Update html/assets/js/approvals-page.js to render announcement action buttons
  - Add action buttons to announcement cards in customer sections
  - Implement button state management (loading, disabled)
  - _Requirements: 13.1, 13.13, 13.14, 13.15_

- [x] 19.3 Add action buttons to announcement details modal
  - Create announcement-details-modal.js similar to change-details-modal.js
  - Add action buttons to modal footer
  - Implement modal refresh after status changes
  - _Requirements: 13.16_

- [x] 19.4 Update upload_lambda API for announcement updates
  - Add "update_announcement" action handler to frontend Lambda
  - Implement multi-customer S3 update for announcements
  - Add validation for status transitions
  - Return success response with updated customer list
  - _Requirements: 13.10, 13.11_

- [x] 19.5 Test announcement action workflows
  - Test approve action: submitted → approved → emails sent
  - Test cancel action: submitted → cancelled → meetings cancelled
  - Test complete action: approved → completed
  - Test invalid transitions are blocked
  - Verify modification entries are added correctly
  - _Requirements: 13.5, 13.6, 13.7, 13.8, 13.9, 13.12_

- [ ] 19.6 Add permission checks for action buttons
  - Implement user permission validation
  - Hide/disable buttons for unauthorized users
  - Show appropriate error messages for permission denials
  - _Requirements: 13.17_

- [-] 20. Create edit announcement page
- [x] 20.1 Create edit-announcement.html page structure
  - Create html/edit-announcement.html matching edit-change.html pattern
  - Add announcement information header showing ID, type, status, creation date
  - Add form fields for all announcement properties (type, title, summary, content, customers, meeting)
  - Add file attachment management section
  - Include save, cancel, and preview buttons
  - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.12_

- [x] 20.2 Create edit announcement page JavaScript
  - Write inline JavaScript in edit-announcement.html following edit-change.html pattern
  - Implement loadAnnouncementFromUrl() to extract announcement ID from URL parameters
  - Implement loadAnnouncement() to fetch announcement data from S3
  - Implement populateForm() to fill all form fields with loaded data
  - _Requirements: 14.1, 14.2_

- [x] 20.3 Implement duplicate announcement mode
  - Detect duplicate mode from URL parameter (?duplicate=true)
  - Implement setupDuplicateMode() to update UI for duplication
  - Generate new announcement ID with appropriate prefix when duplicating
  - Clear status and version-related fields for duplicate
  - Clear meeting URL and meeting ID but preserve meeting time and duration
  - Update page title and button text for duplicate mode
  - _Requirements: 14.6, 14.7, 14.8, 14.9_

- [x] 20.4 Implement announcement type handling
  - Implement handleTypeChange() to update announcement ID prefix when type changes
  - Update object_type field based on selected announcement type
  - Validate type-specific requirements
  - _Requirements: 14.14_

- [x] 20.5 Implement file attachment management
  - Display existing attachments with remove buttons
  - Implement handleFileUpload() for adding new attachments
  - Implement removeAttachment() for removing attachments
  - Track attachment changes for S3 updates
  - _Requirements: 14.12_

- [x] 20.6 Implement meeting field handling
  - Display existing meeting metadata if present
  - Allow updates to meeting details
  - Handle meeting toggle for adding/removing meeting
  - Preserve meeting join URLs from existing meetings
  - _Requirements: 14.13_

- [x] 20.7 Implement customer selection changes
  - Pre-select customers from loaded announcement
  - Track customer selection changes
  - Implement logic to add/remove S3 objects for changed customers
  - _Requirements: 14.15_

- [x] 20.8 Implement save functionality
  - Implement handleUpdate() to validate and save changes
  - Add modification entry with type "updated" and timestamp
  - Update S3 objects for all selected customers
  - Handle customer additions (create new S3 objects)
  - Handle customer removals (delete S3 objects)
  - _Requirements: 14.9, 14.10, 14.11, 14.15_

- [x] 20.9 Implement version management for submitted announcements
  - Detect if announcement is submitted or approved
  - Create new version when editing non-draft announcements
  - Update modification history with version information
  - _Requirements: 14.5_

- [x] 20.10 Implement cancel and preview actions
  - Implement cancelEdit() to return to previous page
  - Implement previewAnnouncement() to show announcement preview modal
  - Use announcement-details-modal.js for preview display
  - _Requirements: 14.16, 14.17_

- [x] 20.11 Add edit links to announcements page
  - Update html/assets/js/announcements-page.js to add edit buttons
  - Add edit button to announcement cards for draft announcements
  - Add duplicate button to all announcements
  - Link buttons to edit-announcement.html with appropriate parameters
  - _Requirements: 14.1, 14.6_

- [x] 20.12 Add edit links to approvals page
  - Update html/assets/js/approvals-page.js to add edit buttons for announcements
  - Show edit button only for draft announcements
  - Show duplicate button for all announcements
  - _Requirements: 14.1, 14.6_

- [x] 20.13 Update navigation for edit announcement
  - Ensure edit-announcement.html has consistent navigation with other pages
  - Add active page highlighting
  - Test navigation flow from announcements → edit → back
  - _Requirements: 14.1, 14.16_

- [ ] 21. Final integration and deployment preparation
- [ ] 21.1 Update README documentation
  - Document new pages (approvals, announcements, create-announcement, edit-announcement)
  - Document enhanced modal features
  - Document object_type field usage
  - Document announcement types and ID prefixes
  - Document announcement action buttons and status workflow
  - Document edit and duplicate announcement workflows
  - Update navigation instructions
  - _Requirements: 6.9, 14.1_

- [ ] 21.2 Create deployment checklist
  - List all files to upload to S3
  - Document CloudFront invalidation steps
  - Create rollback procedure
  - _Requirements: 2.5_

- [ ] 21.3 Final end-to-end testing
  - Test complete user workflow: create change → view in my-changes → approve → view announcement
  - Test complete announcement workflow: create announcement → approve → verify email → complete
  - Test announcement edit workflow: create → save draft → edit → submit
  - Test announcement duplicate workflow: create → duplicate → modify → save
  - Test announcement cancellation workflow with meeting cancellation
  - Verify all features work together
  - Check for any console errors
  - Verify performance targets are met
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 13.8, 13.9, 14.1, 14.6_

## Backend Architecture: Announcement and Change Separation

- [x] 22. Separate announcement processing from change processing
- [x] 22.1 Create AnnouncementProcessor structure
  - Create `internal/processors/announcement_processor.go`
  - Define AnnouncementProcessor struct with required clients (S3, SES, Graph)
  - Implement core announcement processing methods
  - _Requirements: 15.1, 15.2, 15.10_

- [x] 22.2 Create separate announcement event handlers
  - Create `internal/lambda/announcement_handlers.go`
  - Implement `handleAnnouncementEvent` function that works with AnnouncementMetadata
  - Implement announcement-specific status handlers (submitted, approved, cancelled, completed)
  - Update main handler routing to use announcement handlers for announcement_* object types
  - _Requirements: 15.1, 15.2, 15.4_

- [x] 22.3 Refactor email functions for announcements
  - Update `convertToAnnouncementData` to work with AnnouncementMetadata directly
  - Modify `sendAnnouncementApprovalRequest` to accept AnnouncementMetadata
  - Update `sendAnnouncementEmails` to work with AnnouncementMetadata
  - Update `sendAnnouncementCancellationEmail` and `sendAnnouncementCompletionEmail`
  - Extract data from AnnouncementMetadata fields (announcement_id, title, summary, content)
  - _Requirements: 15.7, 15.11_

- [x] 22.4 Refactor meeting functions for announcements
  - Update `ScheduleMultiCustomerMeetingIfNeeded` to handle AnnouncementMetadata
  - Modify meeting subject generation for announcements using announcement.Title
  - Update meeting body generation for announcements using announcement.Content
  - Ensure meeting metadata is saved to AnnouncementMetadata.MeetingMetadata
  - _Requirements: 15.6, 15.12_

- [x] 22.5 Update S3 operations for announcements
  - Ensure announcement parsing preserves AnnouncementMetadata structure
  - Update announcement saving to serialize AnnouncementMetadata without conversion
  - Modify modification tracking for AnnouncementMetadata
  - Add validation for required announcement fields before saving
  - _Requirements: 15.1, 15.3, 15.8_

- [x] 22.6 Delete all existing broken announcements
  - Create script to list all announcement objects in S3 (object_type starts with "announcement_")
  - Delete all existing announcement objects from all customer prefixes
  - Verify all announcement objects are removed
  - Document that announcements will be recreated fresh with new structure
  - _Requirements: 15.3, 15.9_

- [x] 22.7 Testing and validation
  - Create integration tests for announcement lifecycle
  - Test announcement creation, approval, cancellation, and completion
  - Verify email templates work correctly with AnnouncementMetadata
  - Verify meeting creation works correctly with AnnouncementMetadata
  - Verify no data loss when announcements change status
  - Verify "Untitled announcement" bug is fixed
  - _Requirements: 15.1, 15.3, 15.4, 15.9_

- [x] 22.8 Cleanup and documentation
  - Remove old announcement-to-change conversion logic
  - Update API documentation for announcement endpoints
  - Document new AnnouncementProcessor architecture
  - Create troubleshooting guide for announcement issues
  - _Requirements: 15.1, 15.2_

- [x] 22.9 Monitoring and alerting
  - Add CloudWatch metrics for announcement processing
  - Create alerts for announcement processing failures
  - Add logging for announcement data validation failures
  - Monitor announcement field completeness after deployment
  - _Requirements: 15.3, 15.9_
