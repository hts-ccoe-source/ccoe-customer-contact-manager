# SES Domain Validation CLI

This document describes the CLI commands for configuring SES domain validation resources and Route53 DNS records for email sending capabilities.

## Overview

The SES domain validation CLI provides two main workflows:

1. **Integrated Workflow (Recommended)**: Configure SES resources and DNS records in a single command
2. **Standalone Workflows**: Configure SES and DNS separately when needed

## Configuration

### Configuration File Structure

The CLI uses a JSON configuration file (default: `./config.json`) with the following structure:

```json
{
  "aws_region": "us-east-1",
  "log_level": "info",
  
  "customer_mappings": {
    "customer1": {
      "customer_code": "customer1",
      "environment": "prod",
      "customer_name": "Customer One",
      "region": "us-east-1",
      "ses_role_arn": "arn:aws:iam::123456789012:role/customer-contact-manager",
      "sqs_queue_arn": "arn:aws:sqs:us-east-1:123456789012:queue-name",
      
      // Optional: Only needed for standalone Route53 workflow
      "dkim_tokens": ["token1", "token2", "token3"],
      "verification_token": "verification-token-string"
    }
  },
  
  "email_config": {
    "sender_address": "ccoe@hearst.com",
    "meeting_organizer": "ccoe@hearst.com",
    "portal_base_url": "https://change-management.ccoe.hearst.com"
  },
  
  // Required for DNS configuration
  "route53_config": {
    "zone_id": "Z1234567890ABC",           // Route53 hosted zone ID
    "role_arn": "arn:aws:iam::123456789012:role/DNSManagementRole"
  }
}
```

### Configuration Fields

#### route53_config (Required for DNS operations)

- **zone_id**: Route53 hosted zone ID where DNS records will be created
- **role_arn**: IAM role ARN to assume in the DNS account (must have Route53 permissions)

#### customer_mappings (Extended fields)

- **dkim_tokens** (Optional): Array of exactly 3 DKIM tokens from SES
  - Only required for standalone Route53 workflow
  - Automatically captured in integrated workflow
- **verification_token** (Optional): Domain verification token from SES
  - Only required for standalone Route53 workflow
  - Automatically captured in integrated workflow

See `examples/ses-domain-validation-config.json` for a complete example with multiple customers.

## Commands

### ses configure-domain

Configure SES domain validation resources in customer accounts, optionally creating DNS records.

#### Syntax

```bash
./ccoe-customer-contact-manager ses configure-domain [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `./config.json` | Path to configuration file |
| `--customer` | string | (empty) | Customer code to process (processes all if empty) |
| `--dry-run` | bool | `false` | Show what would be done without making changes |
| `--profile` | string | (empty) | AWS profile to use |
| `--region` | string | `us-east-1` | AWS region |
| `--configure-dns` | bool | `true` | Automatically configure Route53 DNS records |
| `--dns-role-arn` | string | (empty) | IAM role ARN for DNS account (overrides config) |

#### What It Does

1. For each customer account:
   - Assumes the SES role specified in `ses_role_arn`
   - Creates SESV2 email identity for `ccoe@hearst.com`
   - Creates SESV2 domain identity for `ccoe.hearst.com`
   - Configures DKIM signing attributes
   - Retrieves verification token and 3 DKIM tokens

2. If `--configure-dns=true` (default):
   - Assumes the DNS role specified in `route53_config.role_arn`
   - Creates 3 CNAME records for DKIM tokens
   - Creates TXT record for domain verification
   - All records use TTL of 600 seconds

#### Examples

**Integrated Workflow - All Customers (Recommended)**

Configure SES and DNS for all customers in one command:

```bash
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --configure-dns=true
```

**Integrated Workflow - Single Customer**

Configure SES and DNS for a specific customer:

```bash
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --customer htsnonprod \
  --configure-dns=true
```

**Dry Run - Preview Changes**

See what would be done without making actual changes:

```bash
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --dry-run
```

**SES Only - No DNS Configuration**

Configure SES resources without creating DNS records (tokens output to console):

```bash
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --configure-dns=false
```

**With Custom AWS Profile**

Use a specific AWS profile for authentication:

```bash
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --profile production \
  --region us-east-1
```

**Override DNS Role ARN**

Specify a different DNS role ARN than what's in the config:

```bash
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --configure-dns=true \
  --dns-role-arn arn:aws:iam::999999999999:role/AlternateDNSRole
```

#### Output Example

```
INFO Starting SES domain configuration
INFO Processing customer customer_code=htsnonprod
INFO Creating email identity email=ccoe@hearst.com customer=htsnonprod
INFO Creating domain identity domain=ccoe.hearst.com customer=htsnonprod
INFO Configuring DKIM signing customer=htsnonprod
INFO Retrieved domain tokens customer=htsnonprod dkim_tokens=3 verification_token=present
INFO Processing customer customer_code=htsprod
INFO Creating email identity email=ccoe@hearst.com customer=htsprod
INFO Creating domain identity domain=ccoe.hearst.com customer=htsprod
INFO Configuring DKIM signing customer=htsprod
INFO Retrieved domain tokens customer=htsprod dkim_tokens=3 verification_token=present

INFO Starting Route53 DNS configuration
INFO Looking up hosted zone name zone_id=Z1234567890ABC
INFO Creating DKIM records organization=htsnonprod records=3
INFO Creating verification record organization=htsnonprod
INFO Creating DKIM records organization=htsprod records=3
INFO Creating verification record organization=htsprod
INFO Applying DNS changes batch=1 changes=8
INFO DNS changes applied successfully

SES Domain Configuration Summary:
- Total customers: 2
- Successful: 2
- Failed: 0
- Email identities created: 2
- Domain identities created: 2
- DKIM configurations: 2

Route53 Configuration Summary:
- Total organizations: 2
- Successful: 2
- Failed: 0
- DKIM records created: 6 (3 per org)
- Verification records created: 2
```

### route53 configure

Configure Route53 DNS records for SES validation using tokens from the configuration file.

**Note**: This is a standalone command for cases where SES is already configured. The integrated workflow (`ses configure-domain --configure-dns=true`) is recommended for most use cases.

#### Syntax

```bash
./ccoe-customer-contact-manager route53 configure [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `./config.json` | Path to configuration file with tokens |
| `--role-arn` | string | (empty) | IAM role ARN for DNS account (overrides config) |
| `--dry-run` | bool | `false` | Show what would be done without making changes |
| `--profile` | string | (empty) | AWS profile to use |
| `--region` | string | `us-east-1` | AWS region |

#### What It Does

1. Reads tokens from `customer_mappings` in the configuration file
2. Validates that all customers have required tokens (3 DKIM tokens, 1 verification token)
3. Assumes the DNS role specified in `route53_config.role_arn`
4. Creates DNS records for all customers:
   - 3 CNAME records for DKIM tokens per customer
   - 1 TXT record for domain verification per customer
   - All records use TTL of 600 seconds

#### Examples

**Standalone Route53 Configuration**

Configure DNS records using tokens already in the config file:

```bash
./ccoe-customer-contact-manager route53 configure \
  --config ./config.json
```

**Dry Run - Preview DNS Changes**

See what DNS records would be created:

```bash
./ccoe-customer-contact-manager route53 configure \
  --config ./config.json \
  --dry-run
```

**With Custom Role ARN**

Override the DNS role ARN from the config:

```bash
./ccoe-customer-contact-manager route53 configure \
  --config ./config.json \
  --role-arn arn:aws:iam::999999999999:role/CustomDNSRole
```

**With AWS Profile**

Use a specific AWS profile:

```bash
./ccoe-customer-contact-manager route53 configure \
  --config ./config.json \
  --profile dns-admin \
  --region us-east-1
```

#### Output Example

```
INFO Starting Route53 DNS configuration
INFO Looking up hosted zone name zone_id=Z1234567890ABC
INFO Validating organization organization=htsnonprod dkim_tokens=3 verification_token=present
INFO Validating organization organization=htsprod dkim_tokens=3 verification_token=present
INFO Creating DKIM records organization=htsnonprod records=3
INFO Creating verification record organization=htsnonprod
INFO Creating DKIM records organization=htsprod records=3
INFO Creating verification record organization=htsprod
INFO Applying DNS changes batch=1 changes=8
INFO DNS changes applied successfully

Route53 Configuration Summary:
- Total organizations: 2
- Successful: 2
- Failed: 0
- DKIM records created: 6 (3 per org)
- Verification records created: 2
```

## Workflows

### Integrated Workflow (Recommended)

This is the simplest and most common workflow. It configures both SES and DNS in a single command.

```bash
# 1. Ensure route53_config is present in config.json
# 2. Run the integrated command
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --configure-dns=true

# 3. Wait for DNS propagation (5-10 minutes)
# 4. Verify in AWS Console:
#    - SES: Email identities are verified
#    - Route53: DNS records are created
# 5. Test email sending
```

### Standalone SES Workflow

Use this when you want to configure SES first and handle DNS separately.

```bash
# 1. Configure SES resources only
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --configure-dns=false

# 2. Tokens are output to console
# 3. Manually add tokens to config.json under each customer_mapping:
#    "dkim_tokens": ["token1", "token2", "token3"],
#    "verification_token": "verification-token"

# 4. Configure DNS separately
./ccoe-customer-contact-manager route53 configure \
  --config ./config.json
```

### Single Customer Workflow

Process only one customer at a time:

```bash
# Configure SES and DNS for a specific customer
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --customer htsnonprod \
  --configure-dns=true
```

### Dry Run Workflow

Preview changes before applying them:

```bash
# 1. Preview SES and DNS changes
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json \
  --dry-run

# 2. Review the output
# 3. Run without --dry-run to apply changes
./ccoe-customer-contact-manager ses configure-domain \
  --config ./config.json
```

## DNS Record Formats

### DKIM CNAME Records

For each customer, 3 CNAME records are created:

```
{token1}._domainkey.ccoe.hearst.com -> {token1}.dkim.amazonses.com
{token2}._domainkey.ccoe.hearst.com -> {token2}.dkim.amazonses.com
{token3}._domainkey.ccoe.hearst.com -> {token3}.dkim.amazonses.com
```

### Verification TXT Records

For each customer, 1 TXT record is created:

```
_amazonses.ccoe.hearst.com -> "{verification-token}"
```

**Note**: Multiple TXT records with the same name are supported by Route53. Each customer account gets its own verification token, and SES will check if any of the tokens in DNS matches what it expects.

## Error Handling

### Idempotency

All operations are fully idempotent:

- **SES resources**: If an email identity or domain identity already exists, it will be updated rather than failing
- **Route53 records**: Uses UPSERT action, so existing records are updated
- **Safe to re-run**: You can safely run commands multiple times without creating duplicates

### Retry Logic

All AWS API calls implement exponential backoff with retries:

- **Max attempts**: 5
- **Initial delay**: 1 second
- **Max delay**: 30 seconds
- **Backoff factor**: 2.0
- **Jitter**: Applied to prevent thundering herd

### Error Recovery

If processing multiple customers:

- **Continue on failure**: If one customer fails, processing continues for remaining customers
- **Summary output**: Shows which customers succeeded and which failed
- **Exit code**: Non-zero if any customer failed, zero if all succeeded

### Common Errors

**Configuration file not found**
```
ERROR Failed to load configuration file error="open ./config.json: no such file or directory"
```
Solution: Ensure config.json exists or specify correct path with `--config`

**Missing route53_config**
```
ERROR route53_config is required when configure-dns=true
```
Solution: Add route53_config section to config.json or use `--configure-dns=false`

**Invalid customer code**
```
ERROR Customer not found customer=invalid-code
```
Solution: Check customer_code in config.json matches the `--customer` flag

**Role assumption failed**
```
ERROR Failed to assume role role_arn=arn:aws:iam::123456789012:role/DNSRole error="AccessDenied"
```
Solution: Verify IAM permissions and trust relationships

**Invalid token count**
```
WARN Skipping organization due to invalid configuration organization=customer1 reason="expected 3 DKIM tokens, got 2"
```
Solution: Ensure exactly 3 DKIM tokens are present in customer_mappings

## IAM Permissions

### SES Operations (Customer Accounts)

The role specified in `ses_role_arn` needs:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ses:CreateEmailIdentity",
        "ses:GetEmailIdentity",
        "ses:PutEmailIdentityDkimAttributes",
        "ses:GetEmailIdentityDkimAttributes"
      ],
      "Resource": "*"
    }
  ]
}
```

### Route53 Operations (DNS Account)

The role specified in `route53_config.role_arn` needs:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "route53:GetHostedZone",
        "route53:ChangeResourceRecordSets",
        "route53:GetChange",
        "route53:ListResourceRecordSets"
      ],
      "Resource": [
        "arn:aws:route53:::hostedzone/{zone-id}",
        "arn:aws:route53:::change/*"
      ]
    }
  ]
}
```

## Verification

After running the commands, verify the setup:

### 1. Check SES Console

In each customer account:
- Navigate to SES console
- Check "Verified identities"
- Verify `ccoe@hearst.com` and `ccoe.hearst.com` are listed
- Check DKIM status is "Successful"

### 2. Check Route53 Console

In the DNS account:
- Navigate to Route53 console
- Open the hosted zone
- Verify CNAME records for DKIM (3 per customer)
- Verify TXT records for verification (1 per customer)

### 3. Wait for DNS Propagation

DNS changes typically propagate within 5-10 minutes. You can check propagation:

```bash
# Check DKIM CNAME records
dig {token}._domainkey.ccoe.hearst.com CNAME

# Check verification TXT record
dig _amazonses.ccoe.hearst.com TXT
```

### 4. Test Email Sending

Once verification is complete, test email sending through SES in the customer accounts.

## Troubleshooting

### DNS Records Not Appearing

- Check that `route53_config.zone_id` is correct
- Verify DNS role has proper permissions
- Check CloudWatch logs for errors
- Try with `--dry-run` to see what would be created

### SES Verification Pending

- Wait 5-10 minutes for DNS propagation
- Verify DNS records are correct in Route53
- Check that verification token matches between SES and DNS
- Use `dig` to verify DNS records are resolvable

### Rate Limiting Errors

- The CLI automatically retries with exponential backoff
- If processing many customers, consider processing in smaller batches
- Check CloudWatch logs for retry attempts

### Batch Processing Failures

- Review summary output to identify which customers failed
- Use `--customer` flag to reprocess specific customers
- Check individual customer configurations for issues

## Best Practices

1. **Always use dry-run first**: Preview changes before applying them
2. **Use integrated workflow**: Simplifies operations and reduces errors
3. **Process all customers at once**: More efficient than individual processing
4. **Monitor CloudWatch logs**: Structured logging provides detailed operation tracking
5. **Verify after changes**: Always check SES and Route53 consoles after running commands
6. **Keep config in version control**: Track changes to customer configurations
7. **Use least-privilege IAM roles**: Only grant necessary permissions
8. **Test in non-prod first**: Validate commands in non-production environments

## See Also

- [AWS SES Documentation](https://docs.aws.amazon.com/ses/)
- [AWS Route53 Documentation](https://docs.aws.amazon.com/route53/)
- [DKIM Email Authentication](https://docs.aws.amazon.com/ses/latest/dg/send-email-authentication-dkim.html)
- Example configuration: `examples/ses-domain-validation-config.json`
