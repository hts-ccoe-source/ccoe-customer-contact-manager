# AWS Alternate Contact Manager

A Go application to manage AWS alternate contacts across multiple AWS Organizations. This tool allows you to set, update, and delete alternate contacts (Security, Billing, and Operations) for all accounts within an AWS Organization.

## Features

- **Multi-Organization Support**: Manage contacts across multiple AWS Organizations
- **Contact Type Management**: Handle Security, Billing, and Operations contacts
- **Role Assumption**: Automatically assumes roles for cross-account operations
- **Overwrite Protection**: Option to protect existing contacts from being overwritten
- **Bulk Operations**: Set or delete contacts across all accounts in an organization
- **AWS SDK v2**: Uses the latest AWS SDK for Go v2

## Prerequisites

- Go 1.21 or later
- AWS credentials configured (IAM role, access keys, or instance profile)
- Appropriate IAM permissions for:
  - Organizations operations (ListAccounts, DescribeOrganization)
  - Account operations (GetAlternateContact, PutAlternateContact, DeleteAlternateContact)
  - STS operations (AssumeRole, GetCallerIdentity)

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

## Environment Variables

Set the `CONFIG_PATH` environment variable to specify the directory containing your configuration files:

```bash
export CONFIG_PATH="/path/to/config/files/"
```

## Usage

### Set Alternate Contacts

Set alternate contacts for all accounts in an organization:

```bash
./aws-alternate-contact-manager set-single \
  -contact-config-file ContactConfig.json \
  -org-prefix hts-prod \
  -overwrite=true
```

### Delete Alternate Contacts

Delete specific contact types from all accounts in an organization:

```bash
./aws-alternate-contact-manager delete \
  -org-prefix hts-prod \
  -contact-types security,billing,operations
```

### Command Line Options

#### set-single command:
- `-contact-config-file`: Path to the contact configuration file (required)
- `-org-prefix`: Organization prefix from OrgConfig.json (required)
- `-overwrite`: Whether to overwrite existing contacts (default: false)

#### delete command:
- `-org-prefix`: Organization prefix from OrgConfig.json (required)
- `-contact-types`: Comma-separated list of contact types to delete (security, billing, operations) (required)

## IAM Permissions

The application requires the following IAM permissions:

### For the execution role:
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

### For the cross-account role (in management accounts):
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
# Test with overwrite protection disabled
./aws-alternate-contact-manager set-single \
  -contact-config-file ContactConfig.json \
  -org-prefix hts-dev \
  -overwrite=false
```

## Troubleshooting

### Common Issues

1. **Role assumption failures**: Verify the cross-account role exists and has the correct trust policy
2. **Configuration file errors**: Ensure JSON files are valid and CONFIG_PATH is set correctly
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