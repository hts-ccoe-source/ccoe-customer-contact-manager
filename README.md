# CCOE Customer Contact Manager

A comprehensive multi-customer change management and notification system built on AWS. The solution enables centralized management of change requests, announcements, and automated email notifications across 25+ customer organizations using AWS SES, Lambda, S3, and Identity Center integration.

## Overview

The CCOE Customer Contact Manager is a full-featured platform that manages:

- **Change Requests**: Complete lifecycle management from draft to completion
- **Announcements**: Community announcements with approval workflows and meeting scheduling
- **Contact Management**: Multi-account AWS contact synchronization with Identity Center
- **Meeting Integration**: Automated Microsoft Graph meeting creation and management
- **Email Notifications**: Topic-based email campaigns with SES integration

## Architecture

### Four Main Components

1. **SAML Authentication Lambda@Edge** (`./lambda/saml_auth`)
   - CloudFront Lambda@Edge function for SAML SSO authentication
   - Integrates with AWS Identity Center
   - Manages user sessions via secure cookies
   - CloudWatch logs: Edge locations (multiple regions)

2. **Frontend API Lambda** (`./lambda/upload_lambda`)
   - Node.js Lambda with Function URL
   - Handles metadata uploads, change management, and API endpoints
   - Processes S3 events via SQS
   - CloudWatch logs: `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-api`

3. **Web Portal UI** (`./html`)
   - Multi-page SPA with vanilla JavaScript
   - Deployed to S3, served via CloudFront
   - Pages: Dashboard, Create Change, My Changes, Approvals, Announcements, Search
   - Creates S3 object events for backend processing

4. **Backend Golang Lambda** (`./main.go` and `./internal/`)
   - Processes S3 events via SQS
   - Sends emails via customer-specific SES
   - Schedules Microsoft Teams meetings
   - Imports contacts from Identity Center
   - CloudWatch logs: `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-backend`

5. **ECS Governance Cluster** (CLI mode)
   - Runs the same Go binary in ECS for scheduled tasks
   - Imports contacts into SES topic lists based on Identity Center roles
   - Configured via `./SESConfig.json`

### Technology Stack

**Frontend:**
- HTML5, CSS3, JavaScript (ES6+)
- Vanilla JS (no framework dependencies)
- S3 static website hosting
- CloudFront CDN

**Backend:**
- Go 1.23 with AWS SDK v2
- Node.js 18+ for Lambda functions
- AWS Lambda (Graviton ARM64)
- AWS SES v2 API
- Microsoft Graph API

**Infrastructure:**
- AWS CloudFront + Lambda@Edge
- AWS S3 (metadata storage)
- AWS SQS (event notifications)
- AWS Lambda (serverless compute)
- AWS ECS Fargate (scheduled tasks)
- AWS Identity Center (SSO)
- Route53 (DNS management)

## Current Status

### ✅ Completed Features

**Authentication & Authorization**
- SAML SSO integration with AWS Identity Center
- Lambda@Edge authentication at CloudFront edge
- Session management with secure cookies
- Domain-based authorization (@hearst.com)
- User context extraction from SAML assertions

**Change Management**
- Multi-customer change request creation
- Draft save/load functionality
- Change approval workflow (draft → submitted → approved → completed/cancelled)
- Version history tracking with modifications array
- Change cloning from existing changes
- Search functionality with filters
- Status transitions with validation

**Announcements**
- Multi-customer announcement creation
- Announcement types: Communication, Financial, Innovation, General
- Announcement approval workflow
- Meeting scheduling for announcements
- Attachment support

**Email Notifications**
- Customer-specific SES email delivery
- Topic-based subscriptions (calendar, announce, approval)
- Group prefix expansion (aws-, wiz-)
- HTML email templates with formatting
- Approval request emails
- Change notification emails
- Completion/cancellation emails

**Meeting Scheduling**
- Microsoft Teams meeting creation via Graph API
- Multi-customer meeting invites
- ICS calendar file generation
- Meeting metadata tracking in modifications array
- Meeting cancellation support
- Idempotent meeting creation using iCalUId

**Identity Center Integration**
- User lookup by email/username
- Group membership queries
- Role-based topic subscription
- Automatic contact import to SES
- Concurrent processing with rate limiting

**S3 Event Processing**
- Transient trigger pattern implementation
- Archive-first data loading
- Idempotency checks
- Event loop prevention
- Customer-specific trigger paths
- Automatic trigger cleanup

### 📊 System Metrics

- **Active Contacts**: 177+ managed contacts across accounts
- **Email Topics**: 9 configured topics (aws-announce, cic-announce, etc.)
- **Accounts**: 2 customer accounts (HTS prod/nonprod)
- **Meeting Integration**: Automated Teams meeting creation
- **State Management**: Robust workflow state machines

## Quick Start

### Prerequisites

- Go 1.23 or later
- AWS CLI configured with appropriate permissions
- Access to target AWS accounts via IAM roles
- Microsoft Graph API credentials (for meeting integration)

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd ccoe-customer-contact-manager

# Build the application
go build -o ccoe-customer-contact-manager

# Or run directly with config
go run main.go ses --config-file config.json --help
```

### Configuration

The system uses two main configuration files:

#### config.json - Customer Account Mappings

```json
{
  "customerMappings": {
    "hts": {
      "accountId": "654654178002",
      "roleArn": "arn:aws:iam::654654178002:role/hts-ccoe-customer-contact-manager",
      "sesContactListName": "ccoe-customer-contacts",
      "identityCenterRoleArn": "arn:aws:iam::748906912469:role/hts-ccoe-customer-contact-importer",
      "restrictedRecipients": ["steven.craig@hearst.com"]
    }
  }
}
```

#### SESConfig.json - Email Topic Configuration

```json
{
  "topics": {
    "cic-announce": {
      "displayName": "Hearst Cloud Innovator Community Announcements",
      "description": "Announce what/why/when for Hearst Cloud Innovator Community",
      "defaultSubscriptionStatus": "OPT_IN",
      "optInRoles": ["admin"]
    },
    "announce-approval": {
      "displayName": "Announcement Approval Requests",
      "description": "Approval Requests for CCOE Announcements",
      "defaultSubscriptionStatus": "OPT_OUT",
      "optInRoles": ["security"]
    }
  }
}
```

## Usage Examples

### Contact Management

```bash
# List all contacts for a customer
./ccoe-customer-contact-manager ses --config-file config.json --customer-code hts --action list-contacts

# Import users from Identity Center
./ccoe-customer-contact-manager ses --config-file config.json --action import-aws-contact-all

# Check specific contact details
./ccoe-customer-contact-manager ses --config-file config.json --customer-code hts --action describe-contact --email user@hearst.com
```

#### Testing with ECS Task Role

To test the CLI with the same permissions as the ECS task, assume the task role:

```bash
# Assume the ECS task role for HTS prod
aws sts assume-role \
  --role-arn arn:aws:iam::730335533660:role/4cm-prod-ccoe-customer-contact-manager-task \
  --role-session-name test-cli-session \
  --duration-seconds 3600

# Export the credentials (replace with actual values from the output)
export AWS_ACCESS_KEY_ID=<AccessKeyId>
export AWS_SECRET_ACCESS_KEY=<SecretAccessKey>
export AWS_SESSION_TOKEN=<SessionToken>

# Or use this one-liner to set them automatically
eval $(aws sts assume-role \
  --role-arn arn:aws:iam::730335533660:role/4cm-prod-ccoe-customer-contact-manager-task \
  --role-session-name test-cli-session \
  --duration-seconds 3600 \
  --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' \
  --output text | awk '{print "export AWS_ACCESS_KEY_ID="$1"\nexport AWS_SECRET_ACCESS_KEY="$2"\nexport AWS_SESSION_TOKEN="$3}')

# Now run CLI commands with task role permissions
./ccoe-customer-contact-manager ses --action import-aws-contact \
  --customer-code htsnonprod \
  --username Steven.Craig@hearst.com
```

### Web Interface

Access the web portal at: `https://change-management.ccoe.hearst.com/`

Main sections:
- `/` - Dashboard
- `/create-change.html` - Create new change requests
- `/my-changes.html` - View your changes
- `/approvals.html` - Approval queue
- `/announcements.html` - Announcement management

### Change Request Workflow

1. **Create Draft**: Use web interface or API to create change request
2. **Submit for Approval**: Triggers email to approvers
3. **Approve**: Approvers review and approve via web interface
4. **Schedule Meeting**: Automatic Teams meeting creation
5. **Complete**: Mark as completed after implementation

### Announcement Workflow

1. **Create Draft**: Draft announcement with meeting details
2. **Submit for Approval**: Send to announcement approvers
3. **Approve**: Approve and schedule community meeting
4. **Publish**: Automatic email to community subscribers

## Development

### Project Structure

```
.
├── main.go                           # Go backend entry point
├── internal/                         # Go internal packages
│   ├── aws/                         # AWS SDK utilities
│   ├── config/                      # Configuration management
│   ├── contacts/                    # Contact management
│   ├── lambda/                      # Lambda handlers
│   ├── ses/                         # SES operations
│   ├── route53/                     # DNS management
│   └── types/                       # Type definitions
├── lambda/                          # Lambda functions
│   ├── saml_auth/                   # SAML authentication
│   └── upload_lambda/               # Upload API handler
├── html/                            # Web portal UI
│   ├── index.html                   # Dashboard
│   ├── create-change.html           # Change creation
│   ├── my-changes.html              # User's changes
│   ├── approvals.html               # Approval queue
│   ├── announcements.html           # Announcements
│   └── assets/                      # CSS/JS assets
├── summaries/                       # Documentation
├── config.json                      # Main configuration
├── SESConfig.json                   # SES topic configuration
└── Makefile                         # Build automation
```

### Build Commands

```bash
# Build Go Lambda (Graviton ARM64)
make package-golang-lambda

# Build Node.js Upload Lambda
make package-upload-lambda

# Build SAML Auth Lambda
make package-saml-lambda

# Build all Lambda packages
make package-all-lambdas

# Deploy website
./deploy-website.sh

# Deploy Lambda backend
./deploy-lambda-backend.sh
```

### Testing

```bash
# Run Go tests
make test

# Run tests with coverage
make test-coverage

# Test internal packages only
make test-internal
```

## Deployment

### Deployment Targets

- **Terraform Directory**: `../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/`
- **S3 Bucket**: `4cm-prod-ccoe-change-management-metadata`
- **CloudFront Distribution**: `E3DIDLE5N99NVJ`
- **Domain**: `change-management.ccoe.hearst.com`

## Security

### Authentication

- SAML SSO via AWS Identity Center
- Lambda@Edge authentication at edge
- Session cookies (HttpOnly, Secure, SameSite)
- 1-hour session timeout

### Authorization

- Domain-based access control (@hearst.com)
- Role-based topic subscriptions
- Customer-specific SES roles
- Least privilege IAM policies

### Data Protection

- S3 versioning enabled
- Encryption at rest (S3 default)
- Encryption in transit (TLS 1.2+)
- CloudWatch logging for audit trail

## Monitoring & Logging

### CloudWatch Logs

- Lambda@Edge: Edge location logs (multiple regions)
- Upload Lambda: `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-api`
- Backend Lambda: `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-backend`
- ECS Tasks: `/ecs/governance-cluster`

### Metrics

- Lambda invocations and errors
- S3 event processing
- SES email delivery rates
- API Gateway request counts
- CloudFront cache hit rates

## Troubleshooting

### Common Issues

**Contact Import Issues**
- **AccessDenied on Identity Center role**: Ensure proper permissions to assume Identity Center importer role
- **Missing contacts**: Run import-aws-contact-all to sync from Identity Center
- **Pagination issues**: Recent fix ensures all 177+ contacts are retrieved

**Email Delivery Problems**
- **Not receiving emails**: Check topic subscription status with describe-contact
- **Partial recipient lists**: Pagination fix resolves missing subscribers
- **Restricted recipients**: Check config.json for restrictedRecipients settings

**Meeting Scheduling Issues**
- **Wrong meeting time**: Recent fix properly handles timezone and duration
- **Missing attendees**: Manual attendees now included beyond topic subscribers
- **Graph API errors**: Verify Microsoft Graph credentials and permissions

**Web Interface Issues**
- **Draft editing fails**: Check Lambda permissions for S3 operations
- **Delete not working**: Recent fix handles both changes and announcements
- **Date filtering**: Now prioritizes created_at over meeting_date for proper filtering

## Documentation

### Key Documents

- `summaries/PROJECT_STATUS_SUMMARY.md` - Current project status and architecture
- `SOLUTION_OVERVIEW.md` - Architecture and deployment
- `LAMBDA_BACKEND_ARCHITECTURE.md` - Backend Lambda design
- `API_ENDPOINTS.md` - API reference
- `DEPLOYMENT_GUIDE.md` - Deployment procedures

## Support

For issues or questions:

1. Check CloudWatch logs for detailed error information
2. Review the PROJECT_STATUS_SUMMARY.md for current known issues
3. Use the CLI tools for diagnostic information
4. Contact the development team for assistance

## License

Internal Hearst project - proprietary software.

---

**Document Version**: 2.0  
**Last Updated**: October 28, 2025  
**Maintained By**: CCOE Platform Team
