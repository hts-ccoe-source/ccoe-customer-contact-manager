# Implementation Plan

- [x] 1. Update configuration structure
  - Add `email_config` section to config.json with sender_address, meeting_organizer, and portal_base_url fields
  - Update `internal/config/config.go` to add EmailConfig type with validation
  - Add email format validation function
  - Add URL format validation function
  - Update application initialization to validate email configuration at startup
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3, 2.4_

- [x] 2. Create template infrastructure package
  - [x] 2.1 Create `internal/ses/templates/` directory structure
    - Create base.go for core template structures
    - Create emojis.go for emoji constants and mapping
    - Create shared.go for common template components
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

  - [x] 2.2 Implement emoji manager in emojis.go
    - Define NotificationType and CategoryType enums
    - Define emoji constants for all notification types and categories
    - Implement GetEmojiForNotification function
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8_

  - [x] 2.3 Implement base template structures in base.go
    - Define BaseTemplateData struct with all required fields including Status and Attachments
    - Define ApprovalRecord struct for tracking multiple approvers
    - Define notification-specific data structures (ApprovalRequestData, ApprovedNotificationData, MeetingData, CompletionData, CancellationData)
    - Define EmailTemplate struct
    - Define TemplateBuilder interface
    - _Requirements: 5.1, 5.2, 8.1, 8.2, 8.3, 8.4, 8.5_

  - [x] 2.4 Implement shared template components in shared.go
    - Implement renderHiddenMetadata for hidden HTML tracking fields
    - Implement renderHTMLHeader function
    - Implement renderStatusSubtitle function
    - Implement renderAttachments function for HTML
    - Implement buildTagline function with hyperlinked event ID (HTML version)
    - Implement buildTaglineText function for plain text emails
    - Implement renderHTMLFooter function
    - Implement renderSESMacro function
    - Implement renderTextHeader function
    - Implement renderTextStatusLine function
    - Implement renderTextAttachments function
    - Implement renderTextFooter function
    - Implement buildSubject function for mobile-optimized subject lines
    - Implement getStatusDisplay function for status mapping
    - Implement formatContentForHTML helper function
    - Implement HTML sanitization functions
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3, 5.4, 6.1, 6.2, 6.3, 6.4, 6.5, 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 3. Implement announcement templates
  - [x] 3.1 Create announcements.go with AnnouncementTemplateBuilder
    - Implement AnnouncementTemplateBuilder struct
    - Implement BuildApprovalRequest method for announcements
    - Implement BuildApprovedNotification method for announcements with multiple approvers support
    - Implement BuildMeetingInvitation method for announcements
    - Implement BuildCompletion method for announcements
    - Implement BuildCancellation method for announcements
    - _Requirements: 3.6, 3.7, 3.8, 8.1, 8.2, 8.3, 8.4, 8.5_

  - [x] 3.2 Implement category-specific styling
    - Define color scheme map for each category (CIC, FinOps, InnerSource, General)
    - Apply category colors to HTML headers
    - Ensure consistent styling across all announcement types
    - _Requirements: 5.1, 5.2_

  - [x] 3.3 Integrate shared components into announcement templates
    - Use renderHiddenMetadata in all announcement templates
    - Use buildSubject with category-specific emojis
    - Use renderHTMLHeader with category colors
    - Use renderStatusSubtitle in template body
    - Use renderAttachments when attachments present
    - Use buildTagline with hyperlinked event ID
    - Use renderSESMacro in all templates
    - _Requirements: 3.1, 3.6, 3.7, 3.8, 4.1, 4.2, 4.3, 5.3, 5.4, 6.1, 6.2, 6.3, 6.4, 6.5, 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 4. Implement change templates
  - [x] 4.1 Create changes.go with ChangeTemplateBuilder
    - Implement ChangeTemplateBuilder struct
    - Implement BuildApprovalRequest method for changes
    - Implement BuildApprovedNotification method for changes with multiple approvers support
    - Implement BuildMeetingInvitation method for changes
    - Implement BuildCompletion method for changes
    - Implement BuildCancellation method for changes
    - _Requirements: 3.2, 3.3, 3.4, 3.5, 8.1, 8.2, 8.3, 8.4, 8.5_

  - [x] 4.2 Implement change-specific styling
    - Define color scheme for change category
    - Apply change colors to HTML headers
    - Ensure consistent styling across all change notification types
    - _Requirements: 5.1, 5.2_

  - [x] 4.3 Integrate shared components into change templates
    - Use renderHiddenMetadata in all change templates
    - Use buildSubject with notification-specific emojis
    - Use renderHTMLHeader with change colors
    - Use renderStatusSubtitle in template body
    - Use renderAttachments when attachments present
    - Use buildTagline with hyperlinked event ID (edit-change.html?changeId={ID})
    - Use renderSESMacro in all templates
    - _Requirements: 3.1, 3.2, 3.4, 3.5, 4.1, 4.2, 4.3, 5.3, 5.4, 6.1, 6.2, 6.3, 6.4, 6.5, 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 5. Create template registry
  - Implement TemplateRegistry struct in base.go
  - Implement NewTemplateRegistry constructor
  - Implement GetTemplate method that routes to appropriate builder
  - Wire up announcement and change builders
  - _Requirements: 5.5, 8.6_

- [x] 6. Update email sending code
  - [x] 6.1 Update internal/ses/operations.go
    - Import new templates package
    - Initialize TemplateRegistry with email config
    - Update email sending functions to use new template registry
    - Pass config values (sender_address, meeting_organizer, portal_base_url) to templates
    - _Requirements: 1.1, 2.1, 5.5_

  - [x] 6.2 Update internal/lambda/handlers.go
    - Update announcement handlers to use new template system
    - Update change handlers to use new template system
    - Pass complete data including status, attachments, and approvals
    - Ensure all notification types are covered
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [x] 7. Remove old template code
  - Delete internal/ses/announcement_templates.go
  - Remove any references to old template functions
  - Update imports throughout codebase
  - _Requirements: 5.5_

- [ ] 8. Update documentation
  - Update deployment guide with new email_config requirements
  - Document email template structure and customization
  - Provide examples of each notification type
  - Document emoji usage patterns
  - Update API documentation if applicable
  - _Requirements: 8.6_
