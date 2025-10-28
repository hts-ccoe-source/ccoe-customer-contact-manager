# Implementation Plan

- [x] 1. Add centralized filtering method to CustomerAccountInfo
  - Add `FilterRecipients()` method to `internal/types/types.go`
  - Implement email normalization (lowercase, trim)
  - Return filtered list and skip count
  - Handle empty/nil `RestrictedRecipients` (no filtering)
  - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [ ]* 1.1 Write unit tests for FilterRecipients method
  - Test with no restrictions (returns all recipients)
  - Test with restrictions (filters correctly)
  - Test email normalization (case insensitive, whitespace)
  - Test all recipients filtered out scenario
  - Test empty input list
  - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [ ] 2. Update announcement processor to use filtering
  - Modify `sendAnnouncementEmails()` in `internal/processors/announcement_processor.go`
  - Get customer config from `p.Config.CustomerMappings[customerCode]`
  - Call `customerInfo.FilterRecipients()` before sending emails
  - Add logging for skipped recipients
  - Skip email sending if no recipients remain after filtering
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 5.1, 5.2, 5.3_

- [ ]* 2.1 Write unit tests for announcement email filtering
  - Test filtering applied before email send
  - Test skip count logged correctly
  - Test no recipients after filtering scenario
  - Test no restrictions configured scenario
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 3. Update meeting scheduler to use centralized filtering
  - Replace `filterRecipientsByRestrictions()` in `internal/ses/meetings.go`
  - Update `CreateMultiCustomerMeetingInvite()` to use `CustomerAccountInfo.FilterRecipients()`
  - Update `CreateMultiCustomerMeetingFromChangeMetadata()` to use `CustomerAccountInfo.FilterRecipients()`
  - Maintain existing logging behavior
  - Handle multi-customer scenarios (aggregate restrictions)
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 4.5_

- [ ]* 3.1 Write unit tests for meeting invitation filtering
  - Test filtering applied before meeting creation
  - Test multi-customer restriction aggregation
  - Test manual attendees filtered correctly
  - Test no recipients after filtering scenario
  - _Requirements: 2.1, 2.2, 2.3, 2.4_

- [x] 4. Verify change request handler still works
  - Review `shouldSendToRecipient()` in `internal/lambda/handlers.go`
  - Confirm it uses `IsRecipientAllowed()` correctly
  - No code changes needed (already working)
  - Document that this path is already compliant
  - _Requirements: 3.1, 3.2, 3.3, 3.4_

- [ ] 5. Add integration test for end-to-end announcement flow
  - Create test announcement for htsnonprod customer
  - Verify only restricted recipients receive emails
  - Check logs for skip messages
  - Verify email count matches expected filtered count
  - _Requirements: 1.1, 1.2, 1.3, 5.1, 5.2, 5.3_

- [ ] 6. Add integration test for end-to-end meeting flow
  - Create test meeting for htsnonprod customer
  - Verify only restricted recipients in attendee list
  - Check Graph API payload for correct attendees
  - Verify meeting creation skipped if no allowed recipients
  - _Requirements: 2.1, 2.2, 2.3, 5.1, 5.2, 5.3_

- [x] 7. Update documentation
  - Document `FilterRecipients()` method in code comments
  - Update `summaries/meeting-attendee-failsafe-fix.md` with announcement fix
  - Add testing guide for verifying failsafe in non-prod
  - Document expected log messages for troubleshooting
  - _Requirements: 5.1, 5.2, 5.3, 5.4_
