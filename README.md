# AWS Alternate Contact Manager

A Go application to manage AWS alternate contacts across multiple AWS Organizations and SES mailing lists. This tool allows you to set, update, and delete alternate contacts (Security, Billing, and Operations) for all accounts within an AWS Organization, as well as manage SES contact lists and email suppression.

## Features

### Alternate Contact Management

- **Multi-Organization Support**: Manage contacts across multiple AWS Organizations
- **Contact Type Management**: Handle Security, Billing, and Operations contacts
- **Role Assumption**: Automatically assumes roles for cross-account operations
- **Overwrite Protection**: Option to protect existing contacts from being overwritten
- **Bulk Operations**: Set or delete contacts across all accounts in an organization

### SES Mailing List Management

- **Contact List Management**: Create and manage SES contact lists with topics
- **Subscription Management**: Add/remove email addresses with topic preferences
- **Suppression List Management**: Manage account-level email suppression for bounces and complaints
- **Bulk Operations**: List all contact lists and their subscribers
- **Topic-based Subscriptions**: Support for multiple subscription topics per contact

### General

- **AWS SDK v2**: Uses the latest AWS SDK for Go v2
- **Unified Tool**: Single binary for both alternate contacts and SES management

## Prerequisites

- Go 1.21 or later
- AWS credentials configured (IAM role, access keys, or instance profile)
- Appropriate IAM permissions for:
  - Organizations operations (ListAccounts, DescribeOrganization)
  - Account operations (GetAlternateContact, PutAlternateContact, DeleteAlternateContact)
  - STS operations (AssumeRole, GetCallerIdentity)
  - SES operations (CreateContactList, CreateContact, PutSuppressedDestination, etc.)

## Installation

1. Clone the repository:

```bash
git clone https://github.com/steven-craig/aws-alternate-contact-manager.git
cd aws-alternate-contact-manager
```

2. Initialize Go modules and download dependencies:

```bash
go mod tidy
```

3. Build the application:

```bash
go build -o aws-alternate-contact-manager aws-alternate-contact-manager.go
```

## Configuration

### Organization Configuration (OrgConfig.json)

Create an `OrgConfig.json` file to define your AWS Organizations:

```json
[
  {
    "mocb_org_friendly_name": "Hearst Production",
    "mocb_org_prefix": "hts-prod",
    "environment": "production",
    "management_account_id": "123456789012"
  },
  {
    "mocb_org_friendly_name": "Hearst Staging",
    "mocb_org_prefix": "hts-staging",
    "environment": "staging",
    "management_account_id": "234567890123"
  }
]
```

### Contact Configuration (ContactConfig.json)

Create a `ContactConfig.json` file to define the alternate contact information:

```json
{
  "security_email": "antoine@example.com",
  "security_name": "Antoine Security Team",
  "security_title": "Security Operations Manager",
  "security_phone": "+1-555-0123",
  "billing_email": "billing@example.com",
  "billing_name": "Finance Team",
  "billing_title": "Billing Manager",
  "billing_phone": "+1-555-0124",
  "operations_email": "ops@example.com",
  "operations_name": "Operations Team",
  "operations_title": "Operations Manager",
  "operations_phone": "+1-555-0125"
}
```

### SES Configuration (SESConfig.json)

Create a `SESConfig.json` file to define SES settings for mailing list management:

```json
{
  "topic_groups": [
    "aws",
    "wiz"
  ],
  "topics": [
    {
      "TopicName": "calendar",
      "DisplayName": "Change Calendar Invites",
      "Description": "Recieves calendar invites for scheduled CCOE Changes",
      "DefaultSubscriptionStatus": "OPT_OUT"
    },
    {
      "TopicName": "announce",
      "DisplayName": "Change Announcements",
      "Description": "Announce what/why/when for CCOE Changes",
      "DefaultSubscriptionStatus": "OPT_OUT"
    },
    {
      "TopicName": "approval",
      "DisplayName": "Change Approval Requests",
      "Description": "Requests for approval of CCOE Changes",
      "DefaultSubscriptionStatus": "OPT_IN"
    }
  ]
}
```

#### Topic Configuration

The configuration uses **topic groups** to generate multiple related topics:

- **`topic_groups`**: Array of group names (e.g., `["aws", "wiz"]`)
- **`topics`**: Base topic definitions that get expanded for each group

Each topic in the `topics` array supports the following fields:

- **`TopicName`**: Base name for the topic (will be prefixed with group)
- **`DisplayName`**: Human-readable name shown to users
- **`Description`**: Base description (will be prefixed with group)
- **`DefaultSubscriptionStatus`**: Default subscription status for new contacts (`"OPT_IN"` or `"OPT_OUT"`)

#### Topic Expansion

For each topic group, all topics are created with sophisticated string manipulation:

- **TopicName**: `{lowercase_group}-{TopicName}` (e.g., `aws-calendar`, `wiz-calendar`)
- **DisplayName**: `{UPPERCASE_GROUP} {DisplayName}` (e.g., `AWS Change Calendar Invites`)
- **Description**: Insert `{UPPERCASE_GROUP}` at index 1 of space-separated words

**Description Examples**:

- `"Recieves calendar invites for scheduled CCOE Changes"` → `"Recieves AWS calendar invites for scheduled CCOE Changes"`
- `"Announce what/why/when for CCOE Changes"` → `"Announce WIZ what/why/when for CCOE Changes"`
- `"Requests for approval of CCOE Changes"` → `"Requests AWS for approval of CCOE Changes"`

**Complete Example**: With groups `["aws", "wiz"]` and base topic:

```json
{
  "TopicName": "calendar",
  "DisplayName": "Change Calendar Invites", 
  "Description": "Recieves calendar invites for scheduled CCOE Changes"
}
```

**Generated Topics**:

- `aws-calendar`:
  - DisplayName: `"AWS Change Calendar Invites"`
  - Description: `"Recieves AWS calendar invites for scheduled CCOE Changes"`
- `wiz-calendar`:
  - DisplayName: `"WIZ Change Calendar Invites"`  
  - Description: `"Recieves WIZ calendar invites for scheduled CCOE Changes"`

**Note**: Region is automatically detected from your AWS configuration (environment variables, ~/.aws/config, or instance metadata).

### Subscription Configuration (SubscriptionConfig.json)

Create a `SubscriptionConfig.json` file to define bulk subscription mappings for the `subscribe` and `unsubscribe` actions:

```json
{
  "aws-calendar": [
    "Scott.Johnson@hearst.com",
    "Einav.Sharon@hearst.com"
  ],
  "aws-announce": [
    "Scott.Johnson@hearst.com",
    "Einav.Sharon@hearst.com",
    "Yogesh.Prabhakar@hearst.com"
  ],
  "aws-approval": [
    "Yogesh.Prabhakar@hearst.com",
    "steven.craig@hearst.com"
  ],
  "wiz-calendar": [
    "steven.craig@hearst.com"
  ],
  "wiz-announce": [
    "steven.craig@hearst.com",
    "Yogesh.Prabhakar@hearst.com"
  ],
  "wiz-approval": [
    "steven.craig@hearst.com",
    "Yogesh.Prabhakar@hearst.com"
  ]
}
```

#### Configuration Structure

- **Keys**: Topic names that exist in your SES contact list
- **Values**: Arrays of email addresses to subscribe/unsubscribe to/from that topic

#### Usage with Actions

- **`subscribe`**: Subscribes all listed email addresses to their respective topics
- **`unsubscribe`**: Unsubscribes all listed email addresses from their respective topics
- **Dry-run support**: Both actions support `--dry-run` to preview changes
- **Smart validation**: Only processes contacts that exist in the contact list
- **Idempotent operations**: Skips contacts already in the desired subscription state

#### Example Operations

```bash
# Preview subscription changes
./aws-alternate-contact-manager ses -action subscribe -dry-run

# Apply subscription changes
./aws-alternate-contact-manager ses -action subscribe

# Preview unsubscription changes  
./aws-alternate-contact-manager ses -action unsubscribe -dry-run

# Apply unsubscription changes
./aws-alternate-contact-manager ses -action unsubscribe

# Use custom config file
./aws-alternate-contact-manager ses -action subscribe -subscription-config MySubscriptions.json
```

## Environment Variables

Set the `CONFIG_PATH` environment variable to specify the directory containing your configuration files. If not set, the application will look for configuration files in the current directory (`./`).

```bash
export CONFIG_PATH="/path/to/config/files/"
```

If `CONFIG_PATH` is not specified, the application will use the current working directory.

## Usage

The application supports two main command categories: alternate contact management and SES mailing list management.

### Alternate Contact Management

#### Set Alternate Contacts for All Organizations

Set alternate contacts for all accounts in ALL organizations defined in OrgConfig.json:

```bash
# Using default ContactConfig.json
./aws-alternate-contact-manager alt-contact -action set-all -overwrite=true

# Or specifying a custom config file
./aws-alternate-contact-manager alt-contact \
  -action set-all \
  -contact-config-file CustomContactConfig.json \
  -overwrite=true
```

#### Set Alternate Contacts for a Single Organization

Set alternate contacts for all accounts in a SINGLE organization:

```bash
# Using default ContactConfig.json
./aws-alternate-contact-manager alt-contact \
  -action set-one \
  -org-prefix htsnonprod \
  -overwrite=true

# Or specifying a custom config file
./aws-alternate-contact-manager alt-contact \
  -action set-one \
  -contact-config-file CustomContactConfig.json \
  -org-prefix htsnonprod \
  -overwrite=true
```

#### Delete Alternate Contacts

Delete specific contact types from all accounts in an organization:

```bash
./aws-alternate-contact-manager alt-contact \
  -action delete \
  -org-prefix hts-prod \
  -contact-types security,billing,operations
```

### SES Mailing List Management

#### Create Contact List

Create a new contact list with specified topics:

```bash
./aws-alternate-contact-manager ses -action create-list -topics "weekly,alerts,updates"
```

Or restore a complete contact list from a backup file:

```bash
./aws-alternate-contact-manager ses -action create-list -backup-file "ses-backup-MyList-20250915-171741.json"
```

#### Add Contact to List

Add an email address to a contact list with topic subscriptions:

```bash
./aws-alternate-contact-manager ses -action add-contact -email "ccoe@hearst.com" -topics "weekly,alerts"
```

#### Remove Contact from List

Remove an email address from a contact list:

```bash
./aws-alternate-contact-manager ses -action remove-contact -email "ccoe@hearst.com"
```

#### Remove All Contacts from List

Remove all email addresses from a contact list. **Automatically creates a backup** before removal:

```bash
./aws-alternate-contact-manager ses -action remove-contact-all
```

**Safety Features:**

- Creates automatic backup before removal (`ses-backup-{listname}-{timestamp}.json`)
- Shows progress for each contact removal
- Provides detailed success/error reporting
- Backup can be used to restore contacts if needed

#### Manage Suppression List

Add or remove emails from the account-level suppression list:

```bash
# Add to suppression list
./aws-alternate-contact-manager ses -action suppress -email "bounced@example.com" -suppression-reason "bounce"

# Remove from suppression list
./aws-alternate-contact-manager ses -action unsuppress -email "user@example.com"
```

#### List Operations

List contact lists and their contents:

```bash
# Describe the account's main contact list
./aws-alternate-contact-manager ses -action describe-account

# List contacts in the account's main list
./aws-alternate-contact-manager ses -action list-contacts
```

#### Topic Operations

Get detailed information about subscription topics:

```bash
# Describe all topics in the account
./aws-alternate-contact-manager ses -action describe-topic-all

# Describe a specific topic with subscription details
./aws-alternate-contact-manager ses -action describe-topic -topic-name "Approval"
```

#### Contact Operations

Get detailed information about specific contacts:

```bash
# Describe a specific contact's subscription status
./aws-alternate-contact-manager ses -action describe-contact -email "user@example.com"
```

#### Topic Management

Idempotently manage topics based on configuration file:

```bash
# Show what changes would be made to topics (dry run)
./aws-alternate-contact-manager ses -action manage-topic --dry-run

# Apply topic changes based on configuration
./aws-alternate-contact-manager ses -action manage-topic
```

**Smart List Creation**: If no contact list exists in the account, `manage-topic` will automatically create one with all configured topics.

**Note**: The `manage-topic` action performs different operations based on the current state:

**If no contact list exists:**

1. Creates a new contact list with all configured topics
2. No backup needed (nothing to back up)

**If contact list exists and topics need changes:**

1. Retrieving all existing contacts and their preferences
2. **Creating a backup file** with complete contact list and contact data
3. Deleting the old contact list  
4. Creating a new contact list with correct topics
5. Migrating all contacts with preserved preferences

**Backup Files**: Automatic backups are saved as `ses-backup-{listname}-{timestamp}.json` in the config directory. These contain complete contact list metadata, all topics, and all contacts with their preferences for disaster recovery.

This operation is safe and fully automated with automatic backup protection.

#### Subscription Management

Bulk subscribe or unsubscribe contacts to/from topics based on configuration file:

```bash
# Preview subscription changes
./aws-alternate-contact-manager ses -action subscribe -dry-run

# Apply subscription changes
./aws-alternate-contact-manager ses -action subscribe

# Preview unsubscription changes
./aws-alternate-contact-manager ses -action unsubscribe -dry-run

# Apply unsubscription changes
./aws-alternate-contact-manager ses -action unsubscribe

# Use custom subscription config file
./aws-alternate-contact-manager ses -action subscribe -subscription-config MySubscriptions.json -dry-run
```

**Smart Processing**: The subscription management actions provide intelligent handling:

- **Contact validation**: Only processes email addresses that exist in the contact list
- **Idempotent operations**: Skips contacts already in the desired subscription state
- **Detailed reporting**: Shows successful, error, and skipped counts with reasons
- **Dry-run support**: Preview all changes before applying them

**Example Output**:

```
📧 Subscribe operation using config: SubscriptionConfig.json
📋 Using SES contact list: AppCommonNonProd
🔍 DRY RUN MODE - No changes will be made

🏷️  Processing topic: aws-calendar (2 emails)
   🔍 Would subscribe Scott.Johnson@hearst.com to aws-calendar
   🔍 Would subscribe Einav.Sharon@hearst.com to aws-calendar

🏷️  Processing topic: aws-announce (3 emails)
   🔍 Would subscribe Scott.Johnson@hearst.com to aws-announce
   ⏭️  Einav.Sharon@hearst.com already subscribed to aws-announce, skipping
   ⚠️  Contact nonexistent@hearst.com does not exist in contact list, skipping

📊 Subscribe Summary:
   ✅ Successful: 3
   ❌ Errors: 0
   ⏭️  Skipped: 2
   📋 Total processed: 5
```

### Command Line Options

#### alt-contact command

- `-action`: Action to perform (required) - Options: set-all, set-one, delete
- `-contact-config-file`: Path to the contact configuration file (default: ContactConfig.json)
- `-org-prefix`: Organization prefix from OrgConfig.json (required for set-one and delete actions)
- `-overwrite`: Whether to overwrite existing contacts (default: false)
- `-contact-types`: Comma-separated list of contact types to delete (required for delete action)

#### ses command

- `-action`: SES action to perform (required) - Options: create-list, add-contact, remove-contact, remove-contact-all, suppress, unsuppress, list-contacts, describe-list, describe-account, describe-topic, describe-topic-all, describe-contact, manage-topic, subscribe, unsubscribe, send-approval-request, send-general-preferences, create-ics-invite, create-meeting-invite, list-identity-center-user, list-identity-center-user-all, list-group-membership, list-group-membership-all, import-aws-contact, import-aws-contact-all, help
- `-ses-config-file`: Path to SES configuration file (default: SESConfig.json)
- `-subscription-config`: Path to subscription configuration file (default: SubscriptionConfig.json)
- `-backup-file`: Path to backup file for restore operations (for create-list action)
- `-email`: Email address for contact operations
- `-topics`: Comma-separated list of topics for subscriptions
- `-suppression-reason`: Reason for suppression - "bounce" or "complaint" (default: bounce)
- `-topic-name`: Topic name for topic-specific operations (required for describe-topic)
- `--dry-run`: Show what would be done without making changes (for manage-topic)
- `-ses-role-arn`: Optional IAM role ARN to assume for SES operations
- `-mgmt-role-arn`: Management account IAM role ARN to assume for Identity Center operations
- `-identity-center-id`: Identity Center instance ID (format: d-xxxxxxxxxx) - Optional when files exist, auto-detected
- `-username`: Username to search for in Identity Center
- `-json-metadata`: Path to JSON metadata file for email/calendar actions
- `-html-template`: Path to HTML template file for approval requests
- `-sender-email`: Sender email address for email/calendar actions
- `-max-concurrency`: Maximum concurrent workers for Identity Center operations (default: 10)
- `-requests-per-second`: API requests per second rate limit (default: 10)

#### Getting Help

To see detailed help with examples for all SES actions:

```bash
./aws-alternate-contact-manager ses -action help
```

This displays:

- Complete list of all available actions
- Required and optional parameters for each action
- Usage examples with real commands
- Safety features and backup information
- Configuration options

#### Identity Center Integration

**Note:** `identity-center-id` is auto-detected from existing files when available, making it optional for most operations.

List users from AWS Identity Center with role assumption and rate limiting:

```bash
# List specific user (identity-center-id auto-detected if files exist)
./aws-alternate-contact-manager ses --action list-identity-center-user \
-username steven.craig@hearst.com \
-mgmt-role-arn arn:aws:iam::978660766591:role/hts-nonprod-org-identity-center-ro

# List specific user with explicit identity-center-id
./aws-alternate-contact-manager ses --action list-identity-center-user \
-identity-center-id d-906638888d \
-username steven.craig@hearst.com \
-mgmt-role-arn arn:aws:iam::978660766591:role/hts-nonprod-org-identity-center-ro

# List all users with custom concurrency and rate limiting
./aws-alternate-contact-manager ses -action list-identity-center-user-all \
-identity-center-id d-906638888d \
-mgmt-role-arn arn:aws:iam::978660766591:role/hts-nonprod-org-identity-center-ro \
-max-concurrency 10 \
-requests-per-second 15

# List group memberships for specific user
./aws-alternate-contact-manager ses -action list-group-membership \
-identity-center-id d-906638888d \
-mgmt-role-arn arn:aws:iam::978660766591:role/hts-nonprod-org-identity-center-ro \
-username steven.craig@hearst.com \

# List group memberships for all users
./aws-alternate-contact-manager ses -action list-group-membership-all \
-identity-center-id d-906638888d \
-mgmt-role-arn arn:aws:iam::978660766591:role/hts-nonprod-org-identity-center-ro \
-max-concurrency 10 \
-requests-per-second 80

# Use SES operations with role assumption
./aws-alternate-contact-manager ses -action list-contacts \
  -ses-role-arn arn:aws:iam::123456789012:role/SESRole
```

**Features:**

- **Role assumption** - Assumes specified IAM role for Identity Center access
- **Concurrency control** - Configurable worker threads (default: 10)
- **Rate limiting** - API request throttling (default: 10 req/sec)
- **Comprehensive user data** - Username, display name, email, names, status
- **Progress tracking** - Shows pagination and processing progress
- **Error handling** - Continues processing on individual failures
- **JSON output** - Automatically saves retrieved data to timestamped JSON files
- **CCOE cloud group parsing** - Automatically extracts AWS account information from ccoe-cloud-* groups

#### CCOE Cloud Group Parsing

The tool automatically identifies and parses `ccoe-cloud-*` groups to extract AWS account information:

**Group naming pattern:**

```
ccoe-cloud-{account-name}-{account-id}-idp-{application-prefix}-{role-name}
```

**Examples:**

- `ccoe-cloud-prod-app-123456789012-idp-myapp-ReadOnlyAccess`
  - Account Name: `prod-app`
  - Account ID: `123456789012`
  - Application Prefix: `myapp`
  - Role Name: `ReadOnlyAccess`

- `ccoe-cloud-dev-multi-word-account-987654321098-idp-complex-app-name-DatabaseAdmin`
  - Account Name: `dev-multi-word-account`
  - Account ID: `987654321098`
  - Application Prefix: `complex-app-name`
  - Role Name: `DatabaseAdmin`

**Features:**

- **Automatic detection** - Finds all ccoe-cloud groups in membership data
- **Robust parsing** - Handles multi-word account names and application prefixes
- **Validation** - Only includes groups that match the expected pattern
- **Sorted output** - Groups sorted by account name, then application prefix, then role name

#### JSON Output Files

All Identity Center commands automatically generate JSON files with the retrieved data:

**File naming patterns:**

- Single user: `identity-center-user-{instance-id}-{username}-{timestamp}.json`
- All users: `identity-center-users-{instance-id}-{timestamp}.json`
- Single user groups: `identity-center-group-membership-{instance-id}-{username}-{timestamp}.json`
- All user groups (user-centric): `identity-center-group-memberships-user-centric-{instance-id}-{timestamp}.json`
- All user groups (group-centric): `identity-center-group-memberships-group-centric-{instance-id}-{timestamp}.json`
- CCOE cloud groups: `identity-center-ccoe-cloud-groups-{instance-id}-{timestamp}.json`

**Example files:**

```
identity-center-users-d-1234567890-20250915-143022.json
identity-center-group-memberships-d-1234567890-20250915-143155.json
```

**JSON structure examples:**

Single user:

```json
{
  "user_id": "12345678-1234-1234-1234-123456789012",
  "user_name": "john.doe",
  "display_name": "John Doe",
  "email": "john.doe@example.com",
  "given_name": "John",
  "family_name": "Doe",
  "active": true
}
```

Group membership (user-centric):

```json
{
  "user_id": "12345678-1234-1234-1234-123456789012",
  "user_name": "john.doe",
  "display_name": "John Doe",
  "email": "john.doe@example.com",
  "groups": [
    "Administrators",
    "Developers",
    "AWS-PowerUsers"
  ]
}
```

Group membership (group-centric):

```json
[
  {
    "group_name": "Administrators",
    "members": [
      {
        "user_id": "12345678-1234-1234-1234-123456789012",
        "user_name": "john.doe",
        "display_name": "John Doe",
        "email": "john.doe@example.com"
      },
      {
        "user_id": "87654321-4321-4321-4321-210987654321",
        "user_name": "admin.user",
        "display_name": "Admin User",
        "email": "admin@example.com"
      }
    ]
  },
  {
    "group_name": "Developers",
    "members": [
      {
        "user_id": "12345678-1234-1234-1234-123456789012",
        "user_name": "john.doe",
        "display_name": "John Doe",
        "email": "john.doe@example.com"
      },
      {
        "user_id": "11111111-2222-3333-4444-555555555555",
        "user_name": "jane.smith",
        "display_name": "Jane Smith",
        "email": "jane.smith@example.com"
      }
    ]
  }
]
```

CCOE cloud groups (parsed AWS account information):

```json
[
  {
    "group_name": "ccoe-cloud-prod-app-123456789012-idp-myapp-ReadOnlyAccess",
    "account_name": "prod-app",
    "account_id": "123456789012",
    "application_prefix": "myapp",
    "role_name": "ReadOnlyAccess",
    "is_valid": true
  },
  {
    "group_name": "ccoe-cloud-dev-database-987654321098-idp-dbapp-DatabaseAdmin",
    "account_name": "dev-database",
    "account_id": "987654321098",
    "application_prefix": "dbapp",
    "role_name": "DatabaseAdmin",
    "is_valid": true
  }
]
```

#### Email and Calendar Integration

Send approval requests and calendar invites based on metadata:

```bash
# Send approval request email
./aws-alternate-contact-manager ses -action send-approval-request \
  -topic-name aws-approval \
  -json-metadata metadata.json \
  -html-template approval-template.html \
  -sender-email notifications@example.com \
  -dry-run

# Send subscription preferences reminder
./aws-alternate-contact-manager ses -action send-general-preferences \
  -topic-name aws-announce \
  -sender-email notifications@example.com \
  -dry-run

# Create ICS calendar invite (email with attachment)
./aws-alternate-contact-manager ses -action create-ics-invite \
  -topic-name aws-calendar \
  -json-metadata metadata.json \
  -sender-email notifications@example.com \
  -dry-run

# Create Microsoft Graph meeting (requires Azure AD setup)
./aws-alternate-contact-manager ses -action create-meeting-invite \
  -topic-name aws-calendar \
  -json-metadata metadata.json \
  -sender-email notifications@example.com \
  -dry-run
```

**Features:**

- **Rich metadata support** - Includes change details, tracking numbers, implementation plans
- **Multiple formats** - HTML and plain text email versions
- **Calendar integration** - Both ICS attachments and Microsoft Graph meetings
- **Topic-based distribution** - Sends to all subscribers of specified topic
- **Dry-run support** - Preview emails and meetings before sending

**Microsoft Graph Integration:**

For `create-meeting-invite`, you need to set up Azure AD app registration:

1. **Register Azure AD app** with `Calendars.ReadWrite` and `User.ReadBasic.All` permissions
2. **Set environment variables:**
   ```bash
   export AZURE_CLIENT_ID="your-app-id"
   export AZURE_CLIENT_SECRET="your-secret"
   export AZURE_TENANT_ID="your-tenant-id"
   ```
3. **Grant admin consent** for the application permissions

#### SES Role Assumption

All SES operations (except Identity Center actions) support optional role assumption:

```bash
# Use SES operations with role assumption
./aws-alternate-contact-manager ses -action list-contacts \
  -ses-role-arn arn:aws:iam::123456789012:role/SESRole

# Create contact list with assumed role
./aws-alternate-contact-manager ses -action create-list \
  -ses-role-arn arn:aws:iam::123456789012:role/SESRole

# Add contact with role assumption
./aws-alternate-contact-manager ses -action add-contact \
  -ses-role-arn arn:aws:iam::123456789012:role/SESRole \
  -email user@example.com
```

**When to use:**

- **Cross-account SES access** - Access SES resources in different AWS accounts
- **Least privilege** - Use specific roles with minimal SES permissions
- **Centralized management** - Manage SES from a central account with assumed roles

#### AWS Contact Import

Import Identity Center users to SES contact lists based on their group memberships and roles:

```bash
# Import specific user (uses existing JSON files)
./aws-alternate-contact-manager ses -action import-aws-contact \
-identity-center-id d-906638888d \
-mgmt-role-arn arn:aws:iam::978660766591:role/hts-nonprod-org-identity-center-ro \
-username john.doe

# Import all users with auto-detected Identity Center ID
./aws-alternate-contact-manager ses -action import-aws-contact-all \
-dry-run

# Import all users (uses existing JSON files)
./aws-alternate-contact-manager ses -action import-aws-contact-all \
-identity-center-id d-906638888d \
-dry-run

# Import all users (generates data files if missing)
./aws-alternate-contact-manager ses -action import-aws-contact-all \
-identity-center-id d-906638888d \
-mgmt-role-arn arn:aws:iam::978660766591:role/hts-nonprod-org-identity-center-ro

```

**Role-to-Topic Mapping:**

- **Security roles** (`security`, `devops`, `cloudeng`, `networking`) → `aws-calendar`, `aws-announce`
- **All active users** → `general-updates`

**Features:**

- **Automatic data loading** - Uses existing Identity Center JSON files (no mgmt-role-arn needed)
- **Auto-generation** - Creates missing data files if mgmt-role-arn provided
- **Role-based mapping** - Maps CCOE cloud group roles to SES topics
- **Dry-run support** - Preview imports before execution
- **Active user filtering** - Only imports active Identity Center users

**Data Requirements:**

- `identity-center-users-{instance-id}-{timestamp}.json`
- `identity-center-group-memberships-user-centric-{instance-id}-{timestamp}.json`

## IAM Permissions

The application requires the following IAM permissions:

### For the execution role

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sts:GetCallerIdentity",
        "sts:AssumeRole"
      ],
      "Resource": "*"
    }
  ]
}
```

### For the cross-account role (in management accounts)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "organizations:ListAccounts",
        "organizations:DescribeOrganization",
        "account:GetAlternateContact",
        "account:PutAlternateContact",
        "account:DeleteAlternateContact"
      ],
      "Resource": "*"
    }
  ]
}
```

### For SES operations

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sesv2:CreateContactList",
        "sesv2:DeleteContactList",
        "sesv2:GetContactList",
        "sesv2:ListContactLists",
        "sesv2:CreateContact",
        "sesv2:DeleteContact",
        "sesv2:GetContact",
        "sesv2:ListContacts",
        "sesv2:PutSuppressedDestination",
        "sesv2:DeleteSuppressedDestination",
        "sesv2:GetSuppressedDestination",
        "sesv2:ListSuppressedDestinations"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "sts:AssumeRole"
      ],
      "Resource": [
        "arn:aws:iam::*:role/*IdentityCenter*",
        "arn:aws:iam::*:role/*SES*"
      ]
    }
  ]
}
```

### For SES operations (assumed role, if using -ses-role-arn)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sesv2:CreateContactList",
        "sesv2:DeleteContactList",
        "sesv2:GetContactList",
        "sesv2:ListContactLists",
        "sesv2:CreateContact",
        "sesv2:DeleteContact",
        "sesv2:GetContact",
        "sesv2:ListContacts",
        "sesv2:PutSuppressedDestination",
        "sesv2:DeleteSuppressedDestination",
        "sesv2:GetSuppressedDestination",
        "sesv2:ListSuppressedDestinations"
      ],
      "Resource": "*"
    }
  ]
}
```

### For Identity Center operations (assumed role, if using -mgmt-role-arn)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "identitystore:ListUsers",
        "identitystore:DescribeUser",
        "identitystore:GetUserId",
        "identitystore:ListGroupMembershipsForMember",
        "identitystore:DescribeGroup"
      ],
      "Resource": "*"
    }
  ]
}
```

## Role Assumption

The application follows this role assumption pattern:

1. If running from the management account: Uses the current credentials
2. If running from a non-management account: Assumes the role `arn:aws:iam::{MANAGEMENT_ACCOUNT_ID}:role/otc/hts-ccoe-mocb-alt-contact-manager`

## Security Considerations

- **Least Privilege**: Only grant the minimum required permissions
- **Role Isolation**: Use dedicated roles for alternate contact management
- **Audit Logging**: Enable CloudTrail to log all API calls
- **Contact Data**: Ensure contact information is accurate and up-to-date

## Error Handling

The application includes comprehensive error handling for:

- Missing configuration files
- Invalid organization prefixes
- Role assumption failures
- Contact operation failures
- Network connectivity issues

## Development

### Building from Source

```bash
go mod tidy
go build -o aws-alternate-contact-manager aws-alternate-contact-manager.go
```

### Testing

Before running in production, test with a single account or non-production organization:

```bash
# Test alternate contacts with overwrite protection disabled
./aws-alternate-contact-manager alt-contact \
  -action set-one \
  -contact-config-file ContactConfig.json \
  -org-prefix hts-dev \
  -overwrite=false

# Test SES operations
./aws-alternate-contact-manager ses -action describe-account
```

## Troubleshooting

### Common Issues

1. **Role assumption failures**: Verify the cross-account role exists and has the correct trust policy
2. **Configuration file errors**: Ensure JSON files are valid and are located in CONFIG_PATH directory (or current directory if CONFIG_PATH is not set)
3. **Permission denied**: Verify IAM permissions for the execution role and cross-account roles
4. **Contact conflicts**: Use the `-overwrite=true` flag to update existing contacts

### Debug Information

The application provides detailed logging including:

- Current account ID and role information
- Organization and account discovery
- Contact operation results
- Error messages and stack traces

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Support

For issues and questions:

1. Check the troubleshooting section
2. Review AWS documentation for alternate contacts
3. Open an issue in this repository

## Related Projects

- [organization-tag-creator](https://github.com/hts-ccoe-source/organization-tag-creator) - Similar Go application for managing AWS organization tags
