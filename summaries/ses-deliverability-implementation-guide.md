# SES Deliverability Implementation Guide

## Overview

This guide provides step-by-step instructions to implement email deliverability improvements for your SES setup. These improvements will significantly increase inbox placement rates and protect your sender reputation.

## Quick Start - Priority Actions

### 1. Add SPF Record (5 minutes)

**Impact**: Immediate improvement in deliverability

Add this TXT record to your DNS:

```
Type: TXT
Name: @ (or your domain name)
Value: v=spf1 include:amazonses.com ~all
```

If you already have an SPF record (e.g., for Google Workspace):

```
Value: v=spf1 include:amazonses.com include:_spf.google.com ~all
```

**Verification**:

```bash
dig TXT yourdomain.com +short | grep spf
```

### 2. Add DMARC Record (5 minutes)

**Impact**: Required by Gmail/Yahoo, major deliverability boost

Add this TXT record to your DNS:

```
Type: TXT
Name: _dmarc
Value: v=DMARC1; p=none; rua=mailto:dmarc-reports@yourdomain.com; pct=100; adkim=s; aspf=s
```

**Start with `p=none`** to monitor, then progress to:

- Week 2-4: `p=quarantine` (send failures to spam)
- Week 4+: `p=reject` (reject failures completely)

**Verification**:

```bash
dig TXT _dmarc.yourdomain.com +short
```

### 3. Configure Custom MAIL FROM Domain (15 minutes)

**Impact**: Better deliverability, SPF alignment, professional appearance

#### Step 1: Choose subdomain

- `bounce.yourdomain.com` (recommended)
- `mail.yourdomain.com`

#### Step 2: Add DNS records

```
Type: MX
Name: bounce.yourdomain.com
Value: 10 feedback-smtp.us-east-1.amazonses.com
TTL: 300

Type: TXT
Name: bounce.yourdomain.com
Value: v=spf1 include:amazonses.com ~all
TTL: 300
```

#### Step 3: Configure in SES

```bash
./ccoe-customer-contact-manager ses \
  --action configure-mail-from \
  --customer-code hts \
  --domain yourdomain.com \
  --mail-from-domain bounce.yourdomain.com
```

**Verification**:

```bash
dig MX bounce.yourdomain.com +short
dig TXT bounce.yourdomain.com +short
```

### 4. Create Configuration Set (10 minutes)

**Impact**: Track metrics, monitor reputation, prevent blacklisting

```bash
# Create configuration set
./ccoe-customer-contact-manager ses \
  --action create-config-set \
  --customer-code hts \
  --config-set-name production-emails

# Assign to domain
./ccoe-customer-contact-manager ses \
  --action assign-config-set \
  --customer-code hts \
  --domain yourdomain.com \
  --config-set-name production-emails
```

### 5. Set Up Event Tracking (15 minutes)

**Impact**: Monitor bounces/complaints, maintain good reputation

#### Step 1: Create SNS topic

```bash
aws sns create-topic --name ses-email-events
```

#### Step 2: Configure event destination

```bash
./ccoe-customer-contact-manager ses \
  --action configure-events \
  --customer-code hts \
  --config-set-name production-emails \
  --sns-topic-arn arn:aws:sns:us-east-1:123456789012:ses-email-events
```

## CLI Commands Reference

### Deliverability Configuration

#### Show DNS Records Needed

```bash
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns \
  --customer-code hts \
  --domain yourdomain.com \
  --mail-from-domain bounce.yourdomain.com \
  --dmarc-policy none \
  --dmarc-email dmarc-reports@yourdomain.com
```

#### Configure Custom MAIL FROM

```bash
# Dry run first
./ccoe-customer-contact-manager ses \
  --action configure-mail-from \
  --customer-code hts \
  --domain yourdomain.com \
  --mail-from-domain bounce.yourdomain.com \
  --dry-run

# Apply changes
./ccoe-customer-contact-manager ses \
  --action configure-mail-from \
  --customer-code hts \
  --domain yourdomain.com \
  --mail-from-domain bounce.yourdomain.com
```

#### Create Configuration Set

```bash
./ccoe-customer-contact-manager ses \
  --action create-config-set \
  --customer-code hts \
  --config-set-name production-emails
```

#### Assign Configuration Set to Domain

```bash
./ccoe-customer-contact-manager ses \
  --action assign-config-set \
  --customer-code hts \
  --domain yourdomain.com \
  --config-set-name production-emails
```

#### Configure Event Tracking

```bash
./ccoe-customer-contact-manager ses \
  --action configure-events \
  --customer-code hts \
  --config-set-name production-emails \
  --sns-topic-arn arn:aws:sns:us-east-1:123456789012:ses-email-events
```

#### Verify Deliverability Setup

```bash
./ccoe-customer-contact-manager ses \
  --action verify-deliverability \
  --customer-code hts \
  --domain yourdomain.com
```

#### Get Reputation Metrics

```bash
./ccoe-customer-contact-manager ses \
  --action get-reputation \
  --customer-code hts
```

### Multi-Customer Operations

#### Configure All Customers

```bash
# Show DNS records for all customers
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns-all \
  --config-file config.json

# Configure MAIL FROM for all customers
./ccoe-customer-contact-manager ses \
  --action configure-mail-from-all \
  --config-file config.json \
  --dry-run

# Create configuration sets for all customers
./ccoe-customer-contact-manager ses \
  --action create-config-set-all \
  --config-file config.json

# Verify all customers
./ccoe-customer-contact-manager ses \
  --action verify-deliverability-all \
  --config-file config.json
```

## Configuration File Updates

Add deliverability settings to your `config.json`:

```json
{
  "customerMappings": {
    "hts": {
      "name": "HTS Prod",
      "ses_role_arn": "arn:aws:iam::123456789012:role/SESRole",
      "deliverability": {
        "mail_from_domain": "bounce.htsprod.com",
        "configuration_set": "hts-production-emails",
        "dmarc_policy": "quarantine",
        "dmarc_report_email": "dmarc-reports@htsprod.com",
        "sns_topic_arn": "arn:aws:sns:us-east-1:123456789012:hts-ses-events"
      }
    }
  }
}
```

## DNS Records Checklist

For each domain, ensure you have:

- [ ] **DKIM Records** (3 CNAME records) - Already configured ✅
- [ ] **Domain Verification** (1 TXT record) - Already configured ✅
- [ ] **SPF Record** (1 TXT record on main domain)
- [ ] **DMARC Record** (1 TXT record on _dmarc subdomain)
- [ ] **MAIL FROM MX Record** (1 MX record on bounce subdomain)
- [ ] **MAIL FROM SPF Record** (1 TXT record on bounce subdomain)

## Monitoring and Maintenance

### Daily Checks

```bash
# Check reputation metrics
./ccoe-customer-contact-manager ses --action get-reputation --customer-code hts

# Review bounce/complaint rates (should be < 5% and < 0.1% respectively)
```

### Weekly Tasks

- Review DMARC reports
- Check for any blacklist listings
- Monitor delivery rates
- Review bounce/complaint trends

### Monthly Tasks

- Audit email content for spam triggers
- Review and clean contact lists
- Update suppression list
- Test email deliverability to major providers

## Troubleshooting

### High Bounce Rate (> 5%)

1. Clean your contact list
2. Implement double opt-in
3. Remove invalid addresses
4. Check for typos in email addresses

### High Complaint Rate (> 0.1%)

1. Review email content
2. Ensure clear unsubscribe link
3. Honor unsubscribe requests immediately
4. Verify you have permission to email recipients

### Emails Going to Spam

1. Verify SPF, DKIM, DMARC are all passing
2. Check sender reputation
3. Review email content for spam triggers
4. Ensure proper text-to-image ratio
5. Include physical mailing address
6. Test with mail-tester.com

### DMARC Failures

1. Verify SPF record includes amazonses.com
2. Verify DKIM is properly configured
3. Check custom MAIL FROM domain alignment
4. Review DMARC reports for specific failures

## Expected Results

After implementing all improvements:

| Metric | Before | After |
|--------|--------|-------|
| Inbox Placement | 60-70% | 85-95% |
| Spam Folder Rate | 20-30% | <5% |
| Bounce Rate | Variable | <5% |
| Complaint Rate | Variable | <0.1% |
| Sender Reputation | Unknown | Monitored & Protected |

## Additional Resources

- [AWS SES Best Practices](https://docs.aws.amazon.com/ses/latest/dg/best-practices.html)
- [SPF Record Syntax](https://www.rfc-editor.org/rfc/rfc7208.html)
- [DMARC Guide](https://dmarc.org/overview/)
- [Email Deliverability Testing](https://www.mail-tester.com/)
- [Sender Score Check](https://senderscore.org/)

## Support

For issues or questions:

1. Check CloudWatch logs for SES events
2. Review SNS notifications for bounces/complaints
3. Use `verify-deliverability` command to check configuration
4. Contact AWS Support for SES-specific issues
