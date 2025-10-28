# Requirements Document

## Introduction

The system must enforce `restricted_recipients` email filtering across ALL email sending paths to prevent unintended email delivery in non-production environments. Currently, the failsafe works for some email types but is bypassed for announcement emails and meeting invitations.

## Glossary

- **System**: The CCOE Customer Contact Manager application
- **restricted_recipients**: A configuration list per customer that whitelists email addresses allowed to receive emails (used for non-prod safety)
- **Announcement Email**: Emails sent when an announcement is approved (CIC, FinOps, InnerSource, General)
- **Meeting Invitation**: Microsoft Graph calendar invites sent via Teams
- **SES Topic**: Amazon SES contact list topic that users subscribe to
- **Customer Code**: Unique identifier for a customer environment (e.g., "htsnonprod", "hts")

## Requirements

### Requirement 1: Enforce Email Restrictions for Announcements

**User Story:** As a platform operator, I want announcement emails to respect the `restricted_recipients` configuration, so that non-production environments don't send emails to unintended recipients.

#### Acceptance Criteria

1. WHEN the System sends announcement emails AND the customer has `restricted_recipients` configured, THE System SHALL filter the recipient list to only include addresses in the `restricted_recipients` list
2. WHEN a recipient is filtered out due to `restricted_recipients`, THE System SHALL log the skipped recipient with the message "⏭️ Skipping {email} (not on restricted recipient list)"
3. WHEN all recipients are filtered out, THE System SHALL skip email sending AND log "⚠️ No allowed recipients after applying restricted_recipients filter"
4. WHEN the customer has no `restricted_recipients` configured, THE System SHALL send emails to all topic subscribers without filtering

### Requirement 2: Enforce Email Restrictions for Meeting Invitations

**User Story:** As a platform operator, I want meeting invitations to respect the `restricted_recipients` configuration, so that non-production environments don't send calendar invites to unintended recipients.

#### Acceptance Criteria

1. WHEN the System creates Microsoft Graph meetings AND the customer has `restricted_recipients` configured, THE System SHALL filter the attendee list to only include addresses in the `restricted_recipients` list
2. WHEN an attendee is filtered out due to `restricted_recipients`, THE System SHALL log the skipped attendee with the message "⏭️ Skipping meeting attendee {email} (not on restricted recipient list)"
3. WHEN all attendees are filtered out, THE System SHALL skip meeting creation AND log "⚠️ No allowed recipients after applying restricted_recipients filter - skipping meeting creation"
4. WHEN the customer has no `restricted_recipients` configured, THE System SHALL create meetings with all topic subscribers without filtering

### Requirement 3: Enforce Email Restrictions for Change Requests

**User Story:** As a platform operator, I want change request emails (approval requests, approved notifications, cancelled notifications) to respect the `restricted_recipients` configuration, so that non-production environments don't send emails to unintended recipients.

#### Acceptance Criteria

1. WHEN the System sends change request emails AND the customer has `restricted_recipients` configured, THE System SHALL filter the recipient list to only include addresses in the `restricted_recipients` list
2. WHEN a recipient is filtered out due to `restricted_recipients`, THE System SHALL log the skipped recipient with the message "⏭️ Skipping {email} (not on restricted recipient list)"
3. WHEN all recipients are filtered out, THE System SHALL skip email sending AND log "⚠️ No allowed recipients after applying restricted_recipients filter"
4. WHEN the customer has no `restricted_recipients` configured, THE System SHALL send emails to all topic subscribers without filtering
5. THE System SHALL apply filtering to approval request emails, approved change emails, and cancelled change emails

### Requirement 4: Consistent Filtering Logic

**User Story:** As a developer, I want a single reusable function for recipient filtering, so that the logic is consistent across all email sending paths.

#### Acceptance Criteria

1. THE System SHALL provide a centralized function that accepts customer configuration(s) and recipient list
2. THE System SHALL return filtered recipients and a count of skipped recipients
3. THE System SHALL normalize email addresses (lowercase, trimmed) for comparison
4. THE System SHALL treat empty or missing `restricted_recipients` as "no restrictions"
5. THE System SHALL be usable by announcement processor, change processor, and meeting scheduler

### Requirement 5: Logging and Visibility

**User Story:** As a platform operator, I want clear logging when recipients are filtered, so that I can verify the failsafe is working correctly.

#### Acceptance Criteria

1. WHEN recipients are filtered, THE System SHALL log the total count of skipped recipients
2. WHEN recipients are filtered, THE System SHALL log the count of allowed recipients
3. WHEN no recipients remain after filtering, THE System SHALL log a warning message
4. THE System SHALL include customer code in all filtering log messages


