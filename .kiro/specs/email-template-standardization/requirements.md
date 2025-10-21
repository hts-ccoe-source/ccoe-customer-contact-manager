# Requirements Document

## Introduction

This specification defines the standardization of all email communication templates in the CCOE Customer Contact Manager system. The system currently sends various types of notifications for announcements (CIC, FinOps, InnerSource, General) and changes, including approval requests, approval notifications, meeting invitations, completion notifications, and cancellation notifications. This effort will establish consistent design, structure, branding, and configuration management across all email templates.

The details of the notifications are particulary important in regards to mobile devices.  We should ensure that for our Title/Subject and first line of the communications, we 'get right to the point' so that previews of the messages show up well in modern mobile devices.

Images should be kept to a minimum or avoided altogether to ensure that their download is not blocked by mail clients and cause the overall design to appear poor and incomplete.

## Glossary

- **System**: The CCOE Customer Contact Manager application
- **SES**: Amazon Simple Email Service, the AWS service used for sending emails
- **Email Template**: A structured message format used for system notifications
- **Announcement**: A notification event categorized as CIC, FinOps, InnerSource, or General
- **Change**: A change request event tracked in the system
- **Event ID**: A unique identifier for announcements (CIC/INN/FIN-xxxxxx) or changes (CHG-xxxxxx)
- **Config File**: The main config.json file containing system-wide configuration
- **SES Macro**: A special {{macro}} token that SES replaces with unsubscribe/preference links
- **Meeting Organizer**: The email address shown as the organizer for calendar meeting invitations

## Requirements

### Requirement 1

**User Story:** As a system administrator, I want all email sender addresses to be sourced from the main config.json file, so that I can manage email configuration centrally without code changes

#### Acceptance Criteria

1. WHEN THE System sends any email notification, THE System SHALL use the sender address defined in the main config.json file
2. THE System SHALL use `ccoe@nonprod.ccoe.hearst.com` as the default sender address value in config.json
3. THE System SHALL validate that the sender address exists in config.json during application initialization
4. IF the sender address is missing from config.json, THEN THE System SHALL log an error and fail to start

### Requirement 2

**User Story:** As a system administrator, I want all meeting organizer addresses to be sourced from the main config.json file, so that calendar invitations show consistent organizer information

#### Acceptance Criteria

1. WHEN THE System creates a calendar meeting invitation, THE System SHALL use the meeting organizer address defined in the main config.json file
2. THE System SHALL use `ccoe@hearst.com` as the default meeting organizer address value in config.json
3. THE System SHALL validate that the meeting organizer address exists in config.json during application initialization
4. IF the meeting organizer address is missing from config.json, THEN THE System SHALL log an error and fail to start

### Requirement 3

**User Story:** As an email recipient, I want each email to have exactly one emoji as the first character of the subject line, so that I can quickly identify the message type visually

#### Acceptance Criteria

1. THE System SHALL include exactly one emoji as the first character of every email subject line
2. WHEN THE System sends an approval request email, THE System SHALL use the yellow yield sign emoji (⚠️) as the first character
3. WHEN THE System sends an approved notification email for changes, THE System SHALL use an emoji other than the green checkmark
4. WHEN THE System sends a completed notification email, THE System SHALL use the green checkmark emoji (✅) as the first character
5. WHEN THE System sends a cancelled notification email, THE System SHALL use the red X emoji (❌) as the first character
6. WHEN THE System sends an approved announcement notification for FinOps, THE System SHALL use the money bag emoji (💰) as the first character
7. WHEN THE System sends an approved announcement notification for CIC, THE System SHALL use the cloud emoji (☁️) as the first character
8. WHEN THE System sends an approved announcement notification for InnerSource, THE System SHALL use the wrench emoji (🔧) as the first character

### Requirement 4

**User Story:** As an email recipient, I want all emails to include SES unsubscribe macros and consistent taglines, so that I can manage my subscription preferences and identify the message source

#### Acceptance Criteria

1. THE System SHALL include the SES {{macro}} token at the bottom of every email message body
2. THE System SHALL include a consistent tagline in every email message body
3. THE System SHALL format the tagline as "event ID [EVENT_ID] sent by the CCOE customer contact manager"
4. WHEN THE System sends an announcement email, THE System SHALL include the announcement event ID in the format CIC-xxxxxx, INN-xxxxxx, or FIN-xxxxxx in the tagline
5. WHEN THE System sends a change email, THE System SHALL include the change event ID in the format CHG-xxxxxx in the tagline

### Requirement 5

**User Story:** As a system maintainer, I want all email templates to follow a consistent structure and design, so that recipients have a uniform experience and the codebase is easier to maintain

#### Acceptance Criteria

1. THE System SHALL apply consistent HTML structure to all email templates
2. THE System SHALL apply consistent styling to all email templates
3. THE System SHALL include consistent header elements in all email templates
4. THE System SHALL include consistent footer elements in all email templates
5. THE System SHALL organize all email template code in a single module or package

### Requirement 6

**User Story:** As a mobile device user, I want email subjects and first lines to be concise and direct, so that message previews display effectively on my device

#### Acceptance Criteria

1. THE System SHALL limit email subject lines to convey the essential message within the first 50 characters
2. THE System SHALL structure the first line of email body content to communicate the key information directly
3. THE System SHALL avoid verbose introductions or preambles in email subject lines
4. THE System SHALL place critical information before supplementary details in email body content
5. THE System SHALL ensure that emoji and essential message text appear within mobile preview limits

### Requirement 7

**User Story:** As an email recipient, I want emails to minimize or avoid images, so that my email client does not block content and cause the design to appear incomplete

#### Acceptance Criteria

1. THE System SHALL minimize the use of images in email templates
2. THE System SHALL avoid embedding images that are critical to understanding the email content
3. THE System SHALL use text-based formatting and emojis instead of image-based icons where possible
4. WHERE images are necessary, THE System SHALL provide meaningful alt text for accessibility
5. THE System SHALL ensure that email templates remain readable and complete when images are blocked by email clients

### Requirement 8

**User Story:** As a developer, I want all email template types to be clearly defined and documented, so that I can understand which templates exist and when they are used

#### Acceptance Criteria

1. THE System SHALL support approval request email templates for both announcements and changes
2. THE System SHALL support approved notification email templates for both announcements and changes
3. THE System SHALL support meeting invitation email templates
4. THE System SHALL support completed notification email templates for both announcements and changes
5. THE System SHALL support cancelled notification email templates for both announcements and changes
6. THE System SHALL document the purpose and usage of each email template type
