# Requirements Document

## Introduction

This feature consolidates the `announce-approval` SES topic to exist only in the HTS production organization, removing it from per-customer topic creation. Since announcement-type objects are managed by the CCOE team (which operates within the HTS Production Organization), approval requests for announcements should only be sent to the HTS production topic, not to individual customer organizations.

## Glossary

- **SES**: Amazon Simple Email Service - AWS email sending service
- **Topic**: A subscription category within an SES contact list (e.g., "aws-calendar", "aws-announce", "announce-approval")
- **Topic Group**: A set of topics that are created per-customer with a prefix (e.g., "aws-" prefix creates "aws-calendar", "aws-announce", "aws-approval")
- **Standalone Topic**: A topic defined in the `topics` array that is created without customer-specific prefixes
- **CCOE**: Cloud Center of Excellence - the team that manages the system and operates within HTS Production Organization
- **HTS Production Organization**: The AWS Organization where the CCOE team operates and where announcement approvals are managed
- **Customer Organization**: Individual AWS Organizations managed by the CCOE (e.g., htsnonprod, other customers)
- **SESConfig.json**: Configuration file defining SES topics, topic groups, and subscription settings
- **Announcement**: A CCOE-managed communication that is distributed to selected customer organizations
- **Change Request**: A customer-specific change notification that requires approval from that customer's security team

## Requirements

### Requirement 1

**User Story:** As a CCOE administrator, I want announcement approval requests to only be sent to the HTS production organization's `announce-approval` topic, so that CCOE team members receive approval requests for announcements without creating unnecessary topics in customer organizations.

#### Acceptance Criteria

1. WHEN the system processes an announcement approval request, THE System SHALL send the approval email only to the `announce-approval` topic in the HTS production organization
2. WHEN the CLI creates SES topics for customer organizations, THE System SHALL NOT create an `announce-approval` topic in customer organizations
3. WHEN the CLI creates SES topics for the HTS production organization, THE System SHALL create the `announce-approval` topic as a standalone topic
4. WHEN viewing the SESConfig.json configuration, THE `announce-approval` topic SHALL remain in the standalone `topics` array and SHALL NOT be added to `topic_group_members`
5. WHEN the system determines email recipients for announcement approvals, THE System SHALL query only the HTS production organization's `announce-approval` topic

### Requirement 2

**User Story:** As a CCOE administrator, I want change request approval requests to continue being sent to each customer's own `aws-approval` topic, so that customer security teams receive approval requests for changes affecting their organization.

#### Acceptance Criteria

1. WHEN the system processes a change request approval, THE System SHALL send the approval email to the `aws-approval` topic in each affected customer organization
2. WHEN the CLI creates SES topics for customer organizations, THE System SHALL create the `aws-approval` topic with the customer-specific prefix (e.g., "aws-approval")
3. WHEN viewing the SESConfig.json configuration, THE `approval` topic SHALL remain in the `topic_group_members` array with the "aws" prefix
4. WHEN the system determines email recipients for change request approvals, THE System SHALL query the `aws-approval` topic from each affected customer organization
5. WHEN a change request affects multiple customers, THE System SHALL send approval requests to each customer's own `aws-approval` topic independently

### Requirement 3

**User Story:** As a system architect, I want clear separation between CCOE-managed announcement workflows and customer-specific change request workflows, so that the system correctly routes approval requests based on the type of notification.

#### Acceptance Criteria

1. WHEN the backend processes an object with type "announcement", THE System SHALL route approval requests to the HTS production `announce-approval` topic
2. WHEN the backend processes an object with type "change", THE System SHALL route approval requests to customer-specific `aws-approval` topics
3. WHEN determining the SES role ARN for announcement approvals, THE System SHALL use the HTS production organization's SES role ARN
4. WHEN determining the SES role ARN for change request approvals, THE System SHALL use each customer's own SES role ARN
5. WHEN logging approval request processing, THE System SHALL clearly indicate whether the approval is for an announcement (CCOE-only) or a change request (customer-specific)

### Requirement 4

**User Story:** As a CCOE administrator, I want existing customer organizations to have their unused `announce-approval` topics cleaned up, so that the system configuration matches the new design without orphaned topics.

#### Acceptance Criteria

1. WHEN migrating to the new design, THE System SHALL provide a CLI command to identify customer organizations with existing `announce-approval` topics
2. WHEN cleaning up orphaned topics, THE CLI SHALL support a dry-run mode to preview which topics would be deleted
3. WHEN deleting orphaned `announce-approval` topics, THE CLI SHALL only delete topics from customer organizations and SHALL NOT delete the topic from HTS production organization
4. WHEN a customer organization has contacts subscribed to the orphaned `announce-approval` topic, THE CLI SHALL log a warning and optionally skip deletion
5. WHEN cleanup is complete, THE CLI SHALL provide a summary report showing which topics were deleted and which were skipped

### Requirement 5

**User Story:** As a developer, I want the backend email routing logic to correctly distinguish between announcement and change request approval workflows, so that approval requests are sent to the correct topics without manual intervention.

#### Acceptance Criteria

1. WHEN the backend determines the notification type, THE System SHALL check the object type field to distinguish between "announcement" and "change"
2. WHEN the object type is "announcement" and status requires approval, THE System SHALL use the HTS production organization's credentials and send to `announce-approval` topic
3. WHEN the object type is "change" and status requires approval, THE System SHALL use each customer's credentials and send to their `aws-approval` topic
4. WHEN the backend encounters an unknown object type, THE System SHALL log an error and SHALL NOT send any approval requests
5. WHEN processing approval requests, THE System SHALL validate that the target topic exists before attempting to send emails

### Requirement 6

**User Story:** As a CCOE administrator, I want documentation that clearly explains the difference between announcement and change request approval workflows, so that team members understand when each topic is used.

#### Acceptance Criteria

1. WHEN viewing the SESConfig.json file, THE documentation SHALL include comments explaining that `announce-approval` is HTS production only
2. WHEN viewing the system architecture documentation, THE documentation SHALL clearly describe the two approval workflows: announcement (CCOE-only) and change request (customer-specific)
3. WHEN viewing the CLI help for topic management commands, THE help text SHALL explain that `announce-approval` is not created per-customer
4. WHEN viewing the backend code, THE code comments SHALL explain the routing logic for announcement vs. change request approvals
5. WHEN onboarding new CCOE team members, THE documentation SHALL provide examples of when to use announcements vs. change requests
