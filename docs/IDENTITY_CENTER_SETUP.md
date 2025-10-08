# Identity Center Setup and Configuration

This document provides comprehensive instructions for setting up AWS IAM Identity Center (formerly AWS SSO) for the multi-customer email distribution system.

## Overview

The Identity Center integration provides:

- **Role-based access control** with four distinct roles
- **Attribute-based access control (ABAC)** for customer-specific permissions
- **Automated user provisioning** and deprovisioning
- **Comprehensive audit logging** and permission tracking
- **CLI tools** for user management

## Architecture

### Permission Sets

| Permission Set | Description | Access Level |
|---------------|-------------|--------------|
| **ChangeManager** | Full access to create and manage changes for all customers | Global |
| **CustomerManager** | Access to create and manage changes for assigned customers only | Customer-specific |
| **ChangeViewer** | Read-only access to view changes and metadata | Global (read-only) |
| **ChangeAuditor** | Audit access to all changes and execution logs | Global (audit) |

### Groups

| Group | Permission Set | Typical Users |
|-------|---------------|---------------|
| **ChangeManagers** | ChangeManager | Senior engineers, team leads |
| **CustomerManagers** | CustomerManager | Customer-specific engineers |
| **ChangeViewers** | ChangeViewer | Junior engineers, observers |
| **ChangeAuditors** | ChangeAuditor | Security team, compliance officers |

### Attribute-Based Access Control (ABAC)

ABAC uses user attributes to enforce customer-specific access:

- **CustomerCode**: Comma-separated list of accessible customer codes (e.g., "hts,cds,motor")
- **CustomerRegion**: Primary region for the user's customers
- **RoleType**: User's primary role type for additional context

## Setup Instructions

### 1. Deploy Terraform Infrastructure

```bash
# Navigate to the Identity Center example
cd terraform/examples/identity-center-setup

# Copy and customize the configuration
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your specific values

# Initialize and deploy
terraform init
terraform plan
terraform apply
```

### 2. Configure Identity Center Settings

Update the `examples/identity-center-config.json` file with your Identity Center details:

```json
{
  "identityStoreId": "d-your-identity-store-id",
  "instanceArn": "arn:aws:sso:::instance/ssoins-your-instance-id",
  "enableAbac": true,
  "customerAttributeName": "CustomerCode",
  "regionAttributeName": "CustomerRegion",
  "permissionSets": {
    "ChangeManager": "arn:aws:sso:::permissionSet/ssoins-your-instance/ps-change-manager",
    "CustomerManager": "arn:aws:sso:::permissionSet/ssoins-your-instance/ps-customer-manager",
    "ReadOnly": "arn:aws:sso:::permissionSet/ssoins-your-instance/ps-read-only",
    "Auditor": "arn:aws:sso:::permissionSet/ssoins-your-instance/ps-auditor"
  },
  "groups": {
    "ChangeManagers": "group-id-for-change-managers",
    "CustomerManagers": "group-id-for-customer-managers",
    "ChangeViewers": "group-id-for-change-viewers",
    "ChangeAuditors": "group-id-for-change-auditors"
  },
  "userProvisioningEnabled": true,
  "auditLoggingEnabled": true,
  "sessionTimeoutMinutes": 480
}
```

### 3. Create Initial Users

Use the CLI to create users with appropriate roles:

```bash
# Create a Change Manager (full access)
./ccoe-customer-contact-manager --mode=identity-center create-user examples/identity-center-user-configs/change-manager-user.json

# Create Customer Managers (customer-specific access)
./ccoe-customer-contact-manager --mode=identity-center create-user examples/identity-center-user-configs/customer-manager-user.json

# Create Auditors (audit access)
./ccoe-customer-contact-manager --mode=identity-center create-user examples/identity-center-user-configs/auditor-user.json

# Create Read-only users
./ccoe-customer-contact-manager --mode=identity-center create-user examples/identity-center-user-configs/readonly-user.json
```

## User Management

### CLI Commands

The system provides comprehensive CLI commands for user management:

```bash
# List all users
./ccoe-customer-contact-manager --mode=identity-center list-users

# Get detailed user information
./ccoe-customer-contact-manager --mode=identity-center get-user john.doe

# Create a new user
./ccoe-customer-contact-manager --mode=identity-center create-user user-config.json

# Update an existing user
./ccoe-customer-contact-manager --mode=identity-center update-user jane.smith update-config.json

# Delete a user (with confirmation)
./ccoe-customer-contact-manager --mode=identity-center delete-user charlie.brown

# Validate user access permissions
./ccoe-customer-contact-manager --mode=identity-center validate-access john.doe create_change hts,cds

# Provision user with automated workflow
./ccoe-customer-contact-manager --mode=identity-center provision-user user-config.json

# Audit permissions for all users
./ccoe-customer-contact-manager --mode=identity-center audit-permissions

# Show help
./ccoe-customer-contact-manager --mode=identity-center help
```

### User Configuration Files

#### Change Manager Example
```json
{
  "userName": "john.doe",
  "givenName": "John",
  "familyName": "Doe",
  "displayName": "John Doe",
  "email": "john.doe@company.com",
  "groups": ["ChangeManagers"],
  "customerCodes": []
}
```

#### Customer Manager Example
```json
{
  "userName": "jane.smith",
  "givenName": "Jane",
  "familyName": "Smith",
  "displayName": "Jane Smith",
  "email": "jane.smith@company.com",
  "groups": ["CustomerManagers"],
  "customerCodes": ["hts", "cds"]
}
```

#### Update Configuration Example
```json
{
  "displayName": "Jane Smith (Updated)",
  "email": "jane.smith.updated@company.com",
  "groups": ["CustomerManagers", "ChangeViewers"],
  "customerCodes": ["hts", "cds", "motor"]
}
```

## Permission Enforcement

### Access Validation

The system automatically validates user permissions for all operations:

```go
// Example: Validate user can create changes for specific customers
hasAccess, err := identityCenterManager.ValidateUserAccess(
    ctx, 
    "jane.smith", 
    "create_change", 
    []string{"hts", "cds"}
)
```

### Role-Based Permissions

| Role | Create Changes | Edit Changes | View All Changes | Audit Changes | Customer Access |
|------|---------------|--------------|------------------|---------------|-----------------|
| **ChangeManager** | ✅ | ✅ | ✅ | ✅ | All customers |
| **CustomerManager** | ✅ | ✅ | ❌ | ❌ | Assigned customers only |
| **ReadOnly** | ❌ | ❌ | ✅ | ❌ | All customers (read-only) |
| **Auditor** | ❌ | ❌ | ✅ | ✅ | All customers (audit) |

### Customer-Specific Access

Customer Managers are restricted to their assigned customers through ABAC:

```json
{
  "CustomerCode": "hts,cds",
  "CustomerRegion": "us-east-1",
  "RoleType": "CustomerManager"
}
```

## Automated Provisioning

### API Endpoint

If enabled, the system provides an API endpoint for automated user provisioning:

```bash
# Create user via API
curl -X POST https://api-gateway-url/users \
  -H "Content-Type: application/json" \
  -d '{
    "action": "create",
    "user": {
      "userName": "new.user",
      "givenName": "New",
      "familyName": "User",
      "displayName": "New User",
      "email": "new.user@company.com",
      "groups": ["CustomerManagers"],
      "customerCodes": ["hts"]
    }
  }'
```

### Webhook Integration

The provisioning API can be integrated with HR systems or other identity providers for automated user lifecycle management.

## Monitoring and Auditing

### CloudWatch Logs

All Identity Center operations are logged to CloudWatch:

- `/aws/identitycenter/audit` - Audit logs for all operations
- `/aws/lambda/identity-center-user-provisioning` - Provisioning API logs

### Audit Reports

Generate comprehensive audit reports:

```bash
# Audit all users
./ccoe-customer-contact-manager --mode=identity-center audit-permissions

# Audit specific user
./ccoe-customer-contact-manager --mode=identity-center audit-permissions john.doe
```

### CloudWatch Alarms

The system includes CloudWatch alarms for:

- Failed login attempts
- Unusual permission changes
- Provisioning API errors

## Security Best Practices

### 1. Principle of Least Privilege

- Assign users to the most restrictive role that meets their needs
- Use Customer Managers instead of Change Managers when possible
- Regularly audit and review permissions

### 2. Attribute Management

- Keep customer code attributes up to date
- Remove access when users change roles or leave
- Use automation for attribute updates when possible

### 3. Session Management

- Configure appropriate session timeouts (default: 8 hours for Change Managers, 4 hours for others)
- Enable MFA for all users
- Monitor for unusual login patterns

### 4. Regular Audits

- Run monthly permission audits
- Review user access quarterly
- Validate customer assignments annually

## Troubleshooting

### Common Issues

#### 1. User Cannot Access Customers

**Symptoms**: User gets "Access Denied" errors for specific customers

**Solutions**:
- Check user's CustomerCode attribute
- Verify user is in correct group
- Validate customer codes exist in system

```bash
# Check user permissions
./ccoe-customer-contact-manager --mode=identity-center get-user username

# Validate access
./ccoe-customer-contact-manager --mode=identity-center validate-access username create_change customer-code
```

#### 2. Permission Set Not Working

**Symptoms**: User has correct groups but wrong permissions

**Solutions**:
- Verify permission set policies
- Check account assignments
- Validate IAM policies in permission sets

#### 3. ABAC Not Enforcing Customer Restrictions

**Symptoms**: Customer Manager can access all customers

**Solutions**:
- Verify ABAC is enabled in configuration
- Check CustomerCode attribute format
- Validate policy conditions use correct attribute names

### Debugging Commands

```bash
# List all groups and their members
./ccoe-customer-contact-manager --mode=identity-center list-groups

# List all permission sets
./ccoe-customer-contact-manager --mode=identity-center list-permission-sets

# Get detailed user information
./ccoe-customer-contact-manager --mode=identity-center get-user username

# Test specific access scenarios
./ccoe-customer-contact-manager --mode=identity-center validate-access username action customer-codes
```

## Integration with Web Portal

The Identity Center integration works seamlessly with the web portal:

1. **Authentication**: Users authenticate via Identity Center SSO
2. **Authorization**: Portal checks user permissions before displaying features
3. **Customer Filtering**: Portal only shows accessible customers
4. **Audit Trail**: All portal actions are logged with user context

### Portal Permission Mapping

| Portal Feature | Required Permission | Available To |
|---------------|-------------------|--------------|
| Create Change | `CanCreateChanges` | ChangeManagers, CustomerManagers |
| Edit Change | `CanEditChanges` | ChangeManagers, CustomerManagers |
| View All Changes | `CanViewAllChanges` | ChangeManagers, ReadOnly, Auditors |
| Customer Selection | Customer attributes | Based on CustomerCode attribute |
| Audit Logs | `CanAuditChanges` | ChangeManagers, Auditors |

## Migration Guide

### From Manual User Management

1. **Export existing users** to configuration files
2. **Create Identity Center groups** using Terraform
3. **Import users** using CLI provisioning commands
4. **Validate permissions** for each user
5. **Update portal configuration** to use Identity Center

### From Other SSO Solutions

1. **Map existing roles** to Identity Center permission sets
2. **Export user attributes** and customer assignments
3. **Create migration scripts** using the provisioning API
4. **Test access patterns** before switching over
5. **Update authentication flows** in applications

## Support and Maintenance

### Regular Tasks

- **Weekly**: Review failed login alarms
- **Monthly**: Run permission audits
- **Quarterly**: Review and update customer assignments
- **Annually**: Validate all user access and clean up unused accounts

### Backup and Recovery

- **Permission Sets**: Managed by Terraform (version controlled)
- **User Data**: Backed up via Identity Center native capabilities
- **Configuration**: Stored in version control
- **Audit Logs**: Retained in CloudWatch with appropriate retention policies

For additional support, refer to the AWS IAM Identity Center documentation or contact your system administrator.