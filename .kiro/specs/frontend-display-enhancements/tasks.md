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

- [ ] 10. Create JSON schema documentation
- [ ] 10.1 Document change object schema
  - Create docs/json-schemas.md
  - Document all fields in change object
  - Include object_type field specification
  - Document modifications array structure (reference object-model-enhancement spec)
  - Provide example change object
  - _Requirements: 6.1, 6.3, 6.4, 6.5, 6.6_

- [ ] 10.2 Document announcement object schema
  - Document all fields in announcement object
  - Document different announcement types (finops, innersourcing, cic, general)
  - Provide example announcement objects for each type
  - _Requirements: 6.2, 6.3, 6.4, 6.5_

- [ ] 10.3 Document meeting metadata structure
  - Document Microsoft Graph meeting fields
  - Reference object-model-enhancement spec for detailed structure
  - _Requirements: 6.7_

- [ ] 10.4 Add schema versioning
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

- [ ] 13. Final integration and deployment preparation
- [ ] 13.1 Update README documentation
  - Document new pages (approvals, announcements)
  - Document enhanced modal features
  - Document object_type field usage
  - Update navigation instructions
  - _Requirements: 6.9_

- [ ] 13.2 Create deployment checklist
  - List all files to upload to S3
  - Document CloudFront invalidation steps
  - Create rollback procedure
  - _Requirements: 2.5_

- [ ] 13.3 Final end-to-end testing
  - Test complete user workflow: create change → view in my-changes → approve → view announcement
  - Verify all features work together
  - Check for any console errors
  - Verify performance targets are met
  - _Requirements: 9.1, 9.2, 9.3, 9.4_
