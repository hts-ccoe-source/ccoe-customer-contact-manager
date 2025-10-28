# Implementation Plan

- [ ] 1. Create HTML view contacts page
  - Create `html/view-contacts.html` with customer selector and contact list table
  - Implement filtering by customer and topic
  - Add sorting by email, customer, or date
  - Display empty state with "Add Contacts" CTA
  - Implement pagination for large lists
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [ ] 2. Create HTML add contacts page
  - Create `html/add-contacts.html` with customer and topic selectors
  - Implement three input methods: text area, CSV upload, Excel upload
  - Add template download links for CSV and Excel
  - Implement email validation preview
  - Add submit button that creates draft and transitions to submitted
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_

- [ ] 3. Create contact import template files
  - Create `templates/contact-import-template.csv` with columns: Customer, Email Address, Topics
  - Create `templates/contact-import-template.xlsx` with same structure
  - Upload templates to S3 for download links
  - _Requirements: 2.5_

- [ ] 4. Implement frontend Node API endpoints for contacts
- [ ] 4.1 Add `GET /contacts` endpoint
  - Parse query parameters for customers and topic filter
  - Read contact imports from S3 archive
  - Filter and aggregate contacts by customer
  - Return JSON response with contact list
  - _Requirements: 1.1, 1.2_

- [ ] 4.2 Add `GET /contacts/topics` endpoint
  - Read SESConfig.json to get available topics
  - Return JSON response with topic list including display names and descriptions
  - _Requirements: 1.2_

- [ ] 4.3 Add `POST /contacts/drafts` endpoint
  - Extract user identity from x-user-email header
  - Generate TACT-GUID using UUID v4 format with TACT- prefix
  - Parse request body with customer, contacts array, and source
  - Validate email formats
  - Write draft JSON to S3 at `drafts/{TACT-GUID}.json`
  - Set S3 metadata: status=draft, object-type=contact_import, object-id={TACT-GUID}
  - Return JSON response with draft ID
  - _Requirements: 7.1, 7.2, 8.1, 8.2, 8.3, 8.4_

- [ ] 4.4 Add `POST /contacts/{TACT-GUID}/submit` endpoint
  - Load draft from S3 `drafts/{TACT-GUID}.json`
  - Update status to "submitted"
  - Update submitted_at and submitted_by fields
  - Copy to `archive/{TACT-GUID}.json`
  - Copy to `customers/{customer-code}/{TACT-GUID}.json` (ephemeral trigger)
  - Set S3 metadata on all copies
  - Delete draft from `drafts/`
  - Return JSON response with import ID and status
  - _Requirements: 7.3, 8.1, 8.3, 8.4_

- [ ] 5. Add routing for contact endpoints in Lambda handler
  - Add route for `GET /contacts` to handleGetContacts
  - Add route for `GET /contacts/topics` to handleGetContactTopics
  - Add route for `POST /contacts/drafts` to handleCreateContactDraft
  - Add route for `POST /contacts/{TACT-GUID}/submit` to handleSubmitContactImport
  - _Requirements: 4.1, 4.2_

- [ ] 6. Implement Golang backend contact import processor
- [ ] 6.1 Create `internal/contacts/additional_contacts.go`
  - Define ContactImportMetadata, ContactEntry, ProcessingResults structs
  - Implement ProcessContactImport function following Transient Trigger Pattern
  - Check if trigger exists (idempotency)
  - Load authoritative data from archive
  - _Requirements: 4.4, 4.5, 7.5, 8.1, 8.2_

- [ ] 6.2 Implement email uniqueness validation
  - Create LoadAllExistingContacts function to retrieve all contacts from all customers once
  - Create ValidateEmailUniqueness function to check emails against existing map
  - Implement htsnonprod exception logic
  - Return lists of valid and duplicate emails with reasons
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [ ] 6.3 Implement SES contact operations
  - Create AddContactsToSES function to add validated contacts to SES contact lists
  - Implement retry logic with exponential backoff for SES API calls
  - Create SubscribeContactsToTopics function to subscribe contacts to specified topics
  - Load SESConfig.json and respect OPT_IN/OPT_OUT settings for topics
  - Handle partial success scenarios (some succeed, some fail)
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ] 6.4 Update archive with processing results
  - Create UpdateArchiveWithResults function
  - Add processing_results with successful and failed arrays
  - Update status to "completed" or "completed_with_errors"
  - Set completed_at and completed_by in S3 metadata
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 7.5, 8.3, 8.4_

- [ ] 6.5 Delete ephemeral trigger
  - Implement DeleteTrigger function to remove from `customers/{customer-code}/`
  - Log trigger deletion
  - _Requirements: 4.4_

- [ ] 7. Integrate contact import detection in main handler
  - Update ProcessS3Event to detect `x-amz-meta-object-type: contact_import`
  - Route contact import objects to ProcessContactImport
  - Ensure Transient Trigger Pattern is followed
  - _Requirements: 4.4, 4.5_

- [ ] 8. Add contact import to frontend navigation
  - Add "Additional Contacts" menu item to navigation
  - Link to view-contacts.html
  - Add "Add Contacts" button that links to add-contacts.html
  - _Requirements: 1.1, 2.1_

- [ ] 9. Update shared JavaScript utilities
  - Add generateContactImportId function to shared.js using UUID v4 format
  - Add parseContactsFromText function to handle comma/semicolon/newline separation
  - Add parseContactsFromCSV function
  - Add parseContactsFromExcel function
  - _Requirements: 2.1, 2.6_

- [ ] 10. Add CloudWatch logging and metrics
  - Log contact import creation, submission, and completion events
  - Log validation failures with reasons
  - Log SES operations (success/failure)
  - Add metrics for imports by customer, success/failure rates, duplicate rejections
  - _Requirements: 6.5_

- [ ] 11. Update configuration documentation
  - Document new S3 key structure for contact imports
  - Document new API endpoints
  - Document contact import workflow states
  - Add examples of contact import JSON structure
  - _Requirements: 8.1, 8.2, 8.3, 8.4_
