# CCOE Customer Contact Manager - Architecture Diagrams

**Generated:** October 22, 2025

This document provides an index of all architecture diagrams for the CCOE Customer Contact Manager system.

## Available Diagrams

### 1. High Level Architecture Overview
**File:** `../generated-diagrams/ccoe-architecture-overview.png`

**Description:** Shows the complete system architecture including:
- User interaction flow
- CloudFront distribution with Lambda@Edge authentication
- Web portal and API layer
- Storage and event processing
- Backend processing (Lambda and ECS)
- Customer services (SES, IAM, Identity Center)
- External integrations (Microsoft Graph API)

**Use Case:** Understanding the overall system design and component relationships.

---

### 2. Component Architecture - Detailed
**File:** `../generated-diagrams/component-architecture.png`

**Description:** Detailed view of all components showing:
- Frontend layer (HTML pages)
- API layer (Upload Lambda)
- Storage layer (S3 bucket structure)
- Event processing (S3 events, SQS)
- Backend layer (Go Lambda internal packages)
- ECS governance service
- Customer services per organization

**Use Case:** Understanding the internal structure and code organization.

---

### 3. Change Request Processing Flow
**File:** `../generated-diagrams/change-request-flow.png`

**Description:** End-to-end flow for processing a change request:
1. User submits change via web form
2. Upload Lambda validates and generates ID
3. Metadata uploaded to S3 (customer triggers + archive)
4. S3 event triggers SQS notification
5. Backend Lambda processes the change
6. Emails sent via customer SES
7. Trigger cleanup

**Use Case:** Understanding the change request lifecycle and transient trigger pattern.

---

### 4. Multi-Customer Email Distribution
**File:** `../generated-diagrams/multi-customer-distribution.png`

**Description:** Shows how a single change request is distributed to multiple customers:
- Single change affects 3 customers (HTS, CDS, FDBUS)
- Upload Lambda creates separate triggers for each customer
- Each customer path generates independent S3 events
- Backend Lambda processes each customer separately
- Each customer's SES sends emails to their users

**Use Case:** Understanding multi-customer notification isolation and parallel processing.

---

### 5. Authentication Flow - SAML SSO
**File:** `../generated-diagrams/authentication-flow.png`

**Description:** SAML authentication flow with AWS Identity Center:
1. User requests protected resource
2. Lambda@Edge checks for valid session
3. If no session, redirects to Identity Center
4. Identity Center returns SAML response
5. Lambda@Edge creates secure session cookie
6. Subsequent requests include session cookie
7. Lambda@Edge validates session and adds user headers

**Use Case:** Understanding authentication and session management.

---

### 6. Identity Center Integration
**File:** `../generated-diagrams/identity-center-integration.png`

**Description:** ECS governance service integration with Identity Center:
- EventBridge scheduled trigger
- ECS task runs Go CLI
- Assumes IAM role in customer account
- Queries Identity Center for users and groups
- Maps roles to SES topics based on SESConfig.json
- Subscribes users to appropriate topics

**Use Case:** Understanding automated contact import and role-based subscriptions.

---

## Diagram Generation

All diagrams were generated using the AWS Diagram MCP Server with the Python `diagrams` library.

### Regenerating Diagrams

To regenerate any diagram, use the MCP server with the appropriate code from the generation scripts.

### Diagram Format

- **Format:** PNG
- **Direction:** TB (Top to Bottom) or LR (Left to Right)
- **Style:** AWS official icons
- **Location:** `./generated-diagrams/`

## Related Documentation

- **Project Status Summary:** `./PROJECT_STATUS_SUMMARY.md`
- **Solution Overview:** `../docs/SOLUTION_OVERVIEW.md`
- **Lambda Backend Architecture:** `../docs/LAMBDA_BACKEND_ARCHITECTURE.md`
- **API Endpoints:** `../docs/API_ENDPOINTS.md`
- **Deployment Guide:** `../docs/DEPLOYMENT_GUIDE.md`

## Diagram Usage Guidelines

### For Presentations
- Use the high-level architecture overview for executive presentations
- Use component architecture for technical deep-dives
- Use flow diagrams for explaining specific processes

### For Documentation
- Embed diagrams in markdown using relative paths
- Reference diagrams in technical specifications
- Update diagrams when architecture changes

### For Onboarding
- Start with high-level architecture
- Progress to component architecture
- Use flow diagrams to explain specific features
- Reference Identity Center integration for understanding user management

## Maintenance

### When to Update Diagrams

Update diagrams when:
- New components are added to the system
- Component relationships change
- New customer services are integrated
- Authentication flow is modified
- Processing patterns change

### Diagram Versioning

- Diagrams are regenerated with each major architecture change
- Previous versions are not kept (regenerate from code)
- Diagram generation code is the source of truth

---

**Document Version:** 1.0  
**Last Updated:** October 22, 2025  
**Maintained By:** CCOE Platform Team
