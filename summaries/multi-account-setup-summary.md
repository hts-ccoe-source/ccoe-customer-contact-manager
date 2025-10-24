# Multi-Account Setup Summary

## Overview

Your SES deliverability configuration uses a **three-account architecture** for security, isolation, and centralized management.

## Architecture Diagram

See: `generated-diagrams/ses-deliverability-architecture.png.png`

## The Three Account Types

### 1. 🎯 Orchestration Account (Central Management)
**What it does**: Central command center where you run the CLI tool

**Key characteristics**:
- Where DevOps engineers authenticate
- Stores `config.json` with all customer configurations
- Has permission to assume roles in other accounts
- No direct SES or Route53 resources

**IAM Role**: `OrchestrationRole`
**Key Permission**: `sts:AssumeRole` to DNS and customer accounts

### 2. 🌐 DNS Account (Shared Route53)
**What it does**: Centralized DNS management for all customer domains

**Key characteristics**:
- Hosts Route53 zones for all customer domains
- Manages all DNS records (A, CNAME, TXT, MX)
- Shared across all customers
- Orchestration account assumes role here to modify DNS

**IAM Role**: `Route53ManagementRole`
**Key Permissions**: `route53:ChangeResourceRecordSets`, `route53:GetHostedZone`

### 3. 📧 Customer SES Accounts (One Per Customer)
**What it does**: Individual SES configuration for each customer organization

**Key characteristics**:
- One account per customer (HTS, CDS, FDBUS, etc.)
- Contains SES domain identities and configuration
- Isolated from other customers
- Orchestration account assumes role here to configure SES

**IAM Role**: `SESManagementRole` (one per account)
**Key Permissions**: `ses:*` for identity and configuration management

## Data Flow

```
1. DevOps Engineer
   ↓ (authenticates)
2. Orchestration Account
   ↓ (assumes role)
3a. DNS Account → Creates DNS records (SPF, DMARC, MX)
   ↓ (assumes role)
3b. Customer SES Account → Configures SES (MAIL FROM, Config Sets)
```

## Why This Architecture?

### Security Benefits
- **Least Privilege**: Each account only has permissions it needs
- **Isolation**: Customer SES accounts are isolated from each other
- **Centralized Control**: All operations go through orchestration account
- **Audit Trail**: CloudTrail tracks all cross-account access

### Operational Benefits
- **Single Tool**: One CLI manages all customers
- **Consistent Configuration**: Same process for all customers
- **Shared DNS**: Centralized DNS management reduces complexity
- **Scalability**: Easy to add new customers

### Compliance Benefits
- **Separation of Duties**: DNS and SES managed separately
- **Audit Logging**: All operations logged in CloudTrail
- **Access Control**: Fine-grained IAM permissions
- **Traceability**: Know who did what, when

## Setup Checklist

### ✅ Orchestration Account
- [ ] Create/identify orchestration role
- [ ] Grant `sts:AssumeRole` to DNS and customer accounts
- [ ] Configure AWS credentials for CLI
- [ ] Test role assumption to DNS account
- [ ] Test role assumption to customer accounts

### ✅ DNS Account
- [ ] Create `Route53ManagementRole`
- [ ] Grant Route53 permissions
- [ ] Configure trust relationship with orchestration account
- [ ] Add external ID for security
- [ ] Test DNS record creation

### ✅ Each Customer SES Account
- [ ] Create `SESManagementRole`
- [ ] Grant SES permissions
- [ ] Configure trust relationship with orchestration account
- [ ] Add unique external ID per customer
- [ ] Create SNS topic for email events (optional)
- [ ] Test SES configuration

### ✅ Configuration File
- [ ] Add Route53 role ARN to `config.json`
- [ ] Add SES role ARN for each customer
- [ ] Add deliverability config for each customer
- [ ] Test configuration with `show-deliverability-dns`

## Quick Reference

### Account IDs
```
Orchestration: 123456789012
DNS:          999999999999
HTS:          111111111111
CDS:          222222222222
FDBUS:        333333333333
```

### Role ARNs
```
Route53: arn:aws:iam::999999999999:role/Route53ManagementRole
HTS SES: arn:aws:iam::111111111111:role/SESManagementRole
CDS SES: arn:aws:iam::222222222222:role/SESManagementRole
```

### Key Commands
```bash
# Show what DNS records are needed
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns \
  --customer-code hts

# Configure everything
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code hts \
  --configure-dns

# Verify setup
./ccoe-customer-contact-manager ses \
  --action verify-deliverability \
  --customer-code hts
```

## Files Created

### Documentation
- ✅ `summaries/iam-roles-and-permissions.md` - Detailed IAM documentation
- ✅ `summaries/multi-account-setup-summary.md` - This file
- ✅ `generated-diagrams/ses-deliverability-architecture.png.png` - Architecture diagram

### IAM Policy Templates
- ✅ `iam-policies/orchestration-role-policy.json`
- ✅ `iam-policies/route53-role-policy.json`
- ✅ `iam-policies/route53-role-trust.json`
- ✅ `iam-policies/ses-role-policy.json`
- ✅ `iam-policies/ses-role-trust.json`
- ✅ `iam-policies/README.md`

## Next Steps

1. **Review the architecture diagram** to understand the flow
2. **Read the detailed IAM documentation** in `summaries/iam-roles-and-permissions.md`
3. **Set up IAM roles** using templates in `iam-policies/`
4. **Test role assumptions** before running actual configuration
5. **Update config.json** with all role ARNs
6. **Run deliverability configuration** for one customer first
7. **Verify and monitor** before rolling out to all customers

## Common Questions

### Q: Why not put everything in one account?
**A**: Separation provides security, isolation, and follows AWS best practices for multi-tenant architectures.

### Q: Can I use the same role name in all customer accounts?
**A**: Yes! Each account can have a role named `SESManagementRole`. The ARN will be different due to different account IDs.

### Q: Do I need external IDs?
**A**: Highly recommended for security. They prevent the "confused deputy" problem in cross-account access.

### Q: Can I add more customers later?
**A**: Yes! Just create the SES role in the new customer account and add configuration to `config.json`.

### Q: What if a customer has their own DNS?
**A**: You can skip the `--configure-dns` flag and provide them with the DNS records to add manually.

### Q: How do I audit who made changes?
**A**: Enable CloudTrail in all accounts. Look for `AssumeRole` events and subsequent API calls.

## Support

For detailed information, see:
- **IAM Setup**: `summaries/iam-roles-and-permissions.md`
- **Quick Start**: `summaries/deliverability-quick-start.md`
- **Implementation Guide**: `summaries/ses-deliverability-implementation-guide.md`
- **Policy Templates**: `iam-policies/README.md`
