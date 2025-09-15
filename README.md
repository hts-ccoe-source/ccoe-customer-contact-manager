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
  "topics": [
    {
      "TopicName": "newsletters",
      "DisplayName": "Newsletter Updates",
      "Description": "Weekly newsletter with company updates and news",
      "DefaultSubscriptionStatus": "OPT_OUT"
    },
    {
      "TopicName": "announcements",
      "DisplayName": "Important Announcements", 
      "Description": "Critical announcements and system notifications",
      "DefaultSubscriptionStatus": "OPT_IN"
    },
    {
      "TopicName": "security-alerts",
      "DisplayName": "Security Alerts",
      "Description": "Security-related notifications and alerts", 
      "DefaultSubscriptionStatus": "OPT_IN"
    }
  ]
}
```

#### Topic Configuration

Each topic in the `topics` array supports the following fields:

- **`TopicName`**: Internal name for the topic (used in API calls)
- **`DisplayName`**: Human-readable name shown to users
- **`Description`**: Detailed description of what the topic covers
- **`DefaultSubscriptionStatus`**: Default subscription status for new contacts (`"OPT_IN"` or `"OPT_OUT"`)

**Note**: Region is automatically detected from your AWS configuration (environment variables, ~/.aws/config, or instance metadata).

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

# Apply topic changes based on configuration (with confirmation)
./aws-alternate-contact-manager ses -action manage-topic
```

**Note**: The `manage-topic` action performs a complete contact list recreation when topics need to be updated or removed. This includes:
1. Backing up all existing contacts and their preferences
2. Deleting the old contact list  
3. Creating a new contact list with correct topics
4. Migrating all contacts with preserved preferences

This operation is safe but requires confirmation due to its comprehensive nature.

### Command Line Options

#### alt-contact command

- `-action`: Action to perform (required) - Options: set-all, set-one, delete
- `-contact-config-file`: Path to the contact configuration file (default: ContactConfig.json)
- `-org-prefix`: Organization prefix from OrgConfig.json (required for set-one and delete actions)
- `-overwrite`: Whether to overwrite existing contacts (default: false)
- `-contact-types`: Comma-separated list of contact types to delete (required for delete action)

#### ses command

- `-action`: SES action to perform (required) - Options: create-list, add-contact, remove-contact, suppress, unsuppress, list-contacts, describe-list, describe-account, describe-topic, describe-topic-all, describe-contact, manage-topic
- `-ses-config-file`: Path to SES configuration file (default: SESConfig.json)
- `-email`: Email address for contact operations
- `-topics`: Comma-separated list of topics for subscriptions
- `-suppression-reason`: Reason for suppression - "bounce" or "complaint" (default: bounce)
- `-topic-name`: Topic name for topic-specific operations (required for describe-topic)
- `--dry-run`: Show what would be done without making changes (for manage-topic)

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
