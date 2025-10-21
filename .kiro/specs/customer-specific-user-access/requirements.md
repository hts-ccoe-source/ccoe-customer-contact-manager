# Requirements Document

## Introduction

This feature enables customer-specific user access to the Change Management Portal by leveraging SAML attributes from AWS IAM Identity Center. Currently, all authenticated users are Hearst employees with full administrative access to all customers' changes and announcements. This enhancement will support external customer users who should only see content relevant to their specific organization.

The feature will extract customer affiliation information from SAML authentication responses and use it to automatically filter changes, announcements, and other content based on the user's associated customer code.

## Requirements

### Requirement 1: SAML Attribute Extraction

**User Story:** As a system administrator, I want to configure Identity Center to pass customer affiliation attributes in SAML responses, so that the application can identify which customer organization a user belongs to.

#### Acceptance Criteria

1. WHEN Identity Center is configured with custom user attributes THEN the SAML response SHALL include a `customerCode` attribute for customer-specific users
2. WHEN a Hearst employee authenticates THEN the SAML response SHALL NOT include a `customerCode` attribute (or it SHALL be null/empty)
3. IF a user has multiple customer affiliations THEN the SAML response SHALL include a comma-separated list of customer codes
4. WHEN the SAML Lambda processes an authentication response THEN it SHALL extract the `customerCode` attribute from the SAML assertion
5. WHEN the `customerCode` attribute is missing or empty THEN the system SHALL treat the user as a Hearst administrator with full access

### Requirement 2: Session Management with Customer Context

**User Story:** As a customer-specific user, I want my customer affiliation to be maintained throughout my session, so that I don't need to re-authenticate or specify my organization repeatedly.

#### Acceptance Criteria

1. WHEN a user successfully authenticates THEN the session cookie SHALL include the extracted `customerCode` value
2. WHEN the session cookie is created THEN it SHALL store the customer code in a secure, tamper-resistant format
3. WHEN a request is processed THEN the Lambda@Edge function SHALL extract the customer code from the session cookie
4. WHEN the session expires THEN the customer code SHALL be cleared and require re-authentication
5. IF the session cookie is modified or tampered with THEN the system SHALL reject the session and require re-authentication

### Requirement 3: Request Header Propagation

**User Story:** As a backend service, I want to receive the user's customer code in request headers, so that I can filter data appropriately without parsing session cookies.

#### Acceptance Criteria

1. WHEN the Lambda@Edge function validates a session THEN it SHALL add an `X-User-Customer` header to the request
2. WHEN a user is a Hearst administrator (no customer code) THEN the `X-User-Customer` header SHALL be empty or omitted
3. WHEN a user has multiple customer affiliations THEN the `X-User-Customer` header SHALL contain a comma-separated list of customer codes
4. WHEN the upload Lambda receives a request THEN it SHALL read the customer code from the `X-User-Customer` header
5. WHEN the customer code header is missing THEN the system SHALL treat the user as having full administrative access

### Requirement 4: Automatic Content Filtering for Changes

**User Story:** As a customer-specific user, I want to see only changes that are relevant to my organization, so that I'm not overwhelmed with irrelevant information.

#### Acceptance Criteria

1. WHEN a customer-specific user requests the changes list THEN the system SHALL return only changes where the user's customer code is in the `customers` array
2. WHEN a Hearst administrator requests the changes list THEN the system SHALL return all changes regardless of customer affiliation
3. WHEN a user has multiple customer affiliations THEN the system SHALL return changes for any of their affiliated customers
4. WHEN a customer-specific user attempts to view a change detail THEN the system SHALL verify the user's customer code is in the change's `customers` array
5. IF a customer-specific user attempts to access a change for a different customer THEN the system SHALL return a 403 Forbidden error

### Requirement 5: Automatic Content Filtering for Announcements

**User Story:** As a customer-specific user, I want to see only announcements that are relevant to my organization, so that I receive targeted communications.

#### Acceptance Criteria

1. WHEN a customer-specific user requests the announcements list THEN the system SHALL return only announcements where the user's customer code is in the `customers` array
2. WHEN a Hearst administrator requests the announcements list THEN the system SHALL return all announcements regardless of customer affiliation
3. WHEN a user has multiple customer affiliations THEN the system SHALL return announcements for any of their affiliated customers
4. WHEN a customer-specific user attempts to view an announcement detail THEN the system SHALL verify the user's customer code is in the announcement's `customers` array
5. IF a customer-specific user attempts to access an announcement for a different customer THEN the system SHALL return a 403 Forbidden error

### Requirement 6: Dashboard Statistics Filtering

**User Story:** As a customer-specific user, I want my dashboard statistics to reflect only my organization's changes, so that I have an accurate view of my relevant activities.

#### Acceptance Criteria

1. WHEN a customer-specific user views the dashboard THEN the statistics SHALL count only changes for their affiliated customer(s)
2. WHEN a Hearst administrator views the dashboard THEN the statistics SHALL count all changes they created or are assigned to
3. WHEN calculating "Total Changes" THEN the system SHALL include only changes where the user's customer code is in the `customers` array
4. WHEN calculating status-specific counts (draft, submitted, completed) THEN the system SHALL apply customer filtering before counting
5. WHEN a user has multiple customer affiliations THEN the statistics SHALL include changes for any of their affiliated customers

### Requirement 7: User Context API Endpoint

**User Story:** As a frontend application, I want to retrieve the current user's context including their customer affiliation, so that I can customize the UI appropriately.

#### Acceptance Criteria

1. WHEN the frontend calls `/api/user/context` THEN the system SHALL return the user's email, role, and customer code
2. WHEN a Hearst administrator requests user context THEN the response SHALL indicate `isAdmin: true` and `customerCode: null`
3. WHEN a customer-specific user requests user context THEN the response SHALL indicate `isAdmin: false` and include their `customerCode`
4. WHEN a user has multiple customer affiliations THEN the response SHALL include an array of customer codes
5. WHEN the user context is retrieved THEN the response SHALL include a timestamp for cache validation

### Requirement 8: Create/Edit Permission Enforcement

**User Story:** As a customer-specific user, I want to be restricted to creating and editing changes only for my organization, so that I cannot accidentally affect other customers.

#### Acceptance Criteria

1. WHEN a customer-specific user creates a change THEN the system SHALL automatically set the `customers` array to include only their affiliated customer code(s)
2. WHEN a customer-specific user attempts to manually add a different customer code THEN the system SHALL reject the request with a 403 Forbidden error
3. WHEN a Hearst administrator creates a change THEN the system SHALL allow them to select any customer codes
4. WHEN a customer-specific user edits a change THEN the system SHALL verify they have permission for all customers in the change's `customers` array
5. IF a customer-specific user attempts to edit a change for a different customer THEN the system SHALL return a 403 Forbidden error

### Requirement 9: Backward Compatibility

**User Story:** As a system administrator, I want the new customer-specific access feature to work alongside existing Hearst administrator access, so that current users are not disrupted.

#### Acceptance Criteria

1. WHEN a user authenticates without a `customerCode` attribute THEN the system SHALL grant full administrative access as it does today
2. WHEN existing Hearst employees authenticate THEN their access SHALL remain unchanged
3. WHEN the feature is deployed THEN existing changes and announcements SHALL remain accessible to Hearst administrators
4. WHEN a Hearst administrator uses the system THEN all existing functionality SHALL work without modification
5. WHEN customer-specific users are added THEN Hearst administrators SHALL continue to have full access to all content

### Requirement 10: Security and Authorization

**User Story:** As a security administrator, I want customer affiliation to be securely validated and enforced, so that users cannot bypass access controls.

#### Acceptance Criteria

1. WHEN customer code is extracted from SAML THEN it SHALL be validated against a list of known customer codes
2. WHEN an invalid customer code is provided THEN the system SHALL reject the authentication
3. WHEN customer filtering is applied THEN it SHALL be enforced at the backend API level, not just the frontend
4. WHEN a user attempts to manipulate request headers THEN the system SHALL use only the customer code from the validated session
5. WHEN audit logs are generated THEN they SHALL include the user's customer code for traceability
