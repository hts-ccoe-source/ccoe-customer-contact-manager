# CLI Testing Pre-Flight Checklist

## ✅ What You've Already Done

- [x] Updated Route53 IAM policy with condition keys
- [x] Deployed policy to DNS account (533267020082)
- [x] Have `route53_config` in config.json with correct zone ID and role ARN

## 📋 What You Need to Do Before Testing

### 1. Add Deliverability Configuration to config.json

You need to add a `deliverability` section for each customer you want to test.

**For htsnonprod (recommended for testing first)**:

```json
{
  "customer_mappings": {
    "htsnonprod": {
      "customer_code": "htsnonprod",
      "environment": "nonprod",
      "customer_name": "HTS NonProd",
      "region": "us-east-1",
      "identity_center_role_arn": "arn:aws:iam::978660766591:role/hts-ccoe-customer-contact-importer",
      "ses_role_arn": "arn:aws:iam::869445953789:role/hts-ccoe-customer-contact-manager",
      "sqs_queue_arn": "arn:aws:sqs:us-east-1:730335533660:hts-prod-ccoe-customer-contact-manager-htsnonprod",
      "deliverability": {
        "mail_from_domain": "bounce.ccoe.hearst.com",
        "configuration_set_name": "htsnonprod-ccoe-emails",
        "dmarc_policy": "none",
        "dmarc_report_email": "dmarc-reports@ccoe.hearst.com",
        "sns_topic_arn": "arn:aws:sns:us-east-1:869445953789:htsnonprod-ses-events"
      }
    }
  }
}
```

**Configuration Options Explained**:

- **`mail_from_domain`**: Subdomain for bounce handling
  - Recommended: `bounce.ccoe.hearst.com`
  - Alternative: `mail.ccoe.hearst.com`
  - Must match pattern in Route53 policy

- **`configuration_set_name`**: Name for SES tracking
  - Format: `{customer}-ccoe-emails`
  - Example: `htsnonprod-ccoe-emails`

- **`dmarc_policy`**: Email authentication policy
  - Start with: `"none"` (monitoring only)
  - Later upgrade to: `"quarantine"` or `"reject"`

- **`dmarc_report_email`**: Where DMARC reports go
  - Use: `dmarc-reports@ccoe.hearst.com`
  - Must be a real, monitored email

- **`sns_topic_arn`**: SNS topic for email events (OPTIONAL)
  - Format: `arn:aws:sns:us-east-1:{account-id}:{topic-name}`
  - Leave out if you don't have SNS topic yet

### 2. Verify SES Role Permissions

The SES role in the customer account needs these permissions:

**Account**: 869445953789 (htsnonprod)
**Role**: `hts-ccoe-customer-contact-manager`

**Required Permissions**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SESIdentityManagement",
      "Effect": "Allow",
      "Action": [
        "ses:GetEmailIdentity",
        "ses:PutEmailIdentityDkimAttributes",
        "ses:PutEmailIdentityMailFromAttributes",
        "ses:PutEmailIdentityConfigurationSetAttributes"
      ],
      "Resource": "arn:aws:ses:*:869445953789:identity/*"
    },
    {
      "Sid": "SESConfigurationSetManagement",
      "Effect": "Allow",
      "Action": [
        "ses:CreateConfigurationSet",
        "ses:GetConfigurationSet",
        "ses:PutConfigurationSetDeliveryOptions",
        "ses:PutConfigurationSetReputationOptions",
        "ses:PutConfigurationSetSendingOptions",
        "ses:PutConfigurationSetSuppressionOptions",
        "ses:PutConfigurationSetTrackingOptions"
      ],
      "Resource": "arn:aws:ses:*:869445953789:configuration-set/*"
    },
    {
      "Sid": "SESEventDestinationManagement",
      "Effect": "Allow",
      "Action": [
        "ses:CreateConfigurationSetEventDestination"
      ],
      "Resource": "arn:aws:ses:*:869445953789:configuration-set/*"
    },
    {
      "Sid": "SESAccountLevelOperations",
      "Effect": "Allow",
      "Action": [
        "ses:GetAccount"
      ],
      "Resource": "*"
    }
  ]
}
```

**How to check**:
```bash
aws iam get-role-policy \
  --role-name hts-ccoe-customer-contact-manager \
  --policy-name SESManagementPolicy \
  --profile htsnonprod
```

### 3. Verify Orchestration Role Can Assume Roles

Your orchestration account needs permission to assume both roles.

**Check orchestration role has**:
```json
{
  "Effect": "Allow",
  "Action": "sts:AssumeRole",
  "Resource": [
    "arn:aws:iam::533267020082:role/ccoe-ses-dns-management",
    "arn:aws:iam::869445953789:role/hts-ccoe-customer-contact-manager"
  ]
}
```

### 4. Verify Trust Relationships

**DNS Account (533267020082)**:
Role `ccoe-ses-dns-management` must trust your orchestration account.

**Customer Account (869445953789)**:
Role `hts-ccoe-customer-contact-manager` must trust your orchestration account.

**How to check**:
```bash
# Check DNS role trust
aws iam get-role \
  --role-name ccoe-ses-dns-management \
  --profile dns-account

# Check SES role trust
aws iam get-role \
  --role-name hts-ccoe-customer-contact-manager \
  --profile htsnonprod
```

### 5. Build the CLI Binary

```bash
# Build the binary
go build -o ccoe-customer-contact-manager main.go

# Verify it built
./ccoe-customer-contact-manager --help
```

### 6. Set Up AWS Credentials

Make sure you have AWS credentials configured for the orchestration account:

```bash
# Check current credentials
aws sts get-caller-identity

# Should show your orchestration account
```

### 7. Create SNS Topic (Optional but Recommended)

If you want email event tracking, create an SNS topic first:

```bash
# In the customer account (869445953789)
aws sns create-topic \
  --name htsnonprod-ses-events \
  --profile htsnonprod

# Subscribe your email for testing
aws sns subscribe \
  --topic-arn arn:aws:sns:us-east-1:869445953789:htsnonprod-ses-events \
  --protocol email \
  --notification-endpoint your-email@hearst.com \
  --profile htsnonprod

# Confirm the subscription via email
```

## 🧪 Testing Steps

### Step 1: Show DNS Records (No Changes)

This is safe - it just displays what's needed:

```bash
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns \
  --customer-code htsnonprod
```

**Expected Output**:
```
📋 DNS Records Needed for Email Deliverability
======================================================================

Customer: HTS NonProd (htsnonprod)
Domain: ccoe.hearst.com

Required DNS Records:

1. Type: TXT
   Name: ccoe.hearst.com
   Value: v=spf1 include:amazonses.com ~all
   TTL: 300

2. Type: TXT
   Name: _dmarc.ccoe.hearst.com
   Value: v=DMARC1; p=none; rua=mailto:dmarc-reports@ccoe.hearst.com; pct=100; adkim=s; aspf=s
   TTL: 300

3. Type: MX
   Name: bounce.ccoe.hearst.com
   Value: 10 feedback-smtp.us-east-1.amazonses.com
   TTL: 300

4. Type: TXT
   Name: bounce.ccoe.hearst.com
   Value: v=spf1 include:amazonses.com ~all
   TTL: 300
```

**If this fails**:
- Check `deliverability` config is in config.json
- Check customer code matches
- Check config.json syntax is valid

### Step 2: Dry Run (No Changes)

This tests role assumptions and shows what would be done:

```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code htsnonprod \
  --configure-dns \
  --dry-run \
  --log-level debug
```

**What this tests**:
- ✅ Can read config.json
- ✅ Can assume DNS role
- ✅ Can assume SES role
- ✅ DNS policy allows the operations
- ✅ SES policy allows the operations

**Expected Output**:
```
INFO starting deliverability configuration customer_code=htsnonprod configure_dns=true dry_run=true
INFO configuring deliverability for domain domain=ccoe.hearst.com mail_from_domain=bounce.ccoe.hearst.com
INFO dry-run: would configure custom MAIL FROM domain
INFO dry-run: would create configuration set
INFO dry-run: would assign configuration set to domain
INFO assuming DNS role role_arn=arn:aws:iam::533267020082:role/ccoe-ses-dns-management
INFO dry-run: would complete deliverability DNS configuration zone_id=Z02954802IDGJ8J3833M2
INFO deliverability configuration completed customer_code=htsnonprod dry_run=true
```

**If this fails**:
- Check error message for which role assumption failed
- Verify trust relationships
- Check IAM permissions
- Look for "Access Denied" errors

### Step 3: Configure SES Only (No DNS)

Test SES configuration without touching DNS:

```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code htsnonprod \
  --log-level info
```

**What this does**:
- ✅ Configures custom MAIL FROM domain in SES
- ✅ Creates configuration set
- ✅ Assigns configuration set to domain
- ✅ Creates event destination (if SNS topic configured)
- ❌ Does NOT modify DNS

**Expected Output**:
```
INFO starting deliverability configuration customer_code=htsnonprod configure_dns=false
INFO configured custom MAIL FROM domain
INFO created configuration set
INFO assigned configuration set to domain
INFO configured event destination
INFO deliverability configuration completed
```

### Step 4: Configure DNS Records

Now add the DNS records:

```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code htsnonprod \
  --configure-dns \
  --log-level info
```

**What this does**:
- ✅ Everything from Step 3
- ✅ Creates SPF record on ccoe.hearst.com
- ✅ Creates DMARC record on _dmarc.ccoe.hearst.com
- ✅ Creates MX record on bounce.ccoe.hearst.com
- ✅ Creates SPF record on bounce.ccoe.hearst.com

**Expected Output**:
```
INFO starting deliverability configuration customer_code=htsnonprod configure_dns=true
INFO configured custom MAIL FROM domain
INFO created configuration set
INFO assigned configuration set to domain
INFO assuming DNS role
INFO prepared SPF record domain=ccoe.hearst.com
INFO prepared DMARC record name=_dmarc.ccoe.hearst.com policy=none
INFO prepared MX record domain=bounce.ccoe.hearst.com
INFO prepared MAIL FROM SPF record domain=bounce.ccoe.hearst.com
INFO deliverability DNS records configured successfully
INFO deliverability configuration completed
```

### Step 5: Verify Setup

Check that everything is configured correctly:

```bash
./ccoe-customer-contact-manager ses \
  --action verify-deliverability \
  --customer-code htsnonprod \
  --log-level info
```

**Expected Output**:
```
🔍 Verifying Deliverability Setup
======================================================================

Customer: HTS NonProd (htsnonprod)
Domain: ccoe.hearst.com

INFO deliverability setup verification domain=ccoe.hearst.com verified=true dkim_enabled=true mail_from_configured=true

📊 Reputation Metrics:
INFO SES account reputation metrics sending_enabled=true production_access_enabled=true
INFO send quota max_24_hour_send=50000 max_send_rate=14 sent_last_24_hours=0

✅ Verification completed
```

### Step 6: Verify DNS Records

Check that DNS records were created:

```bash
# Check SPF record
dig TXT ccoe.hearst.com +short

# Check DMARC record
dig TXT _dmarc.ccoe.hearst.com +short

# Check MX record
dig MX bounce.ccoe.hearst.com +short

# Check bounce SPF
dig TXT bounce.ccoe.hearst.com +short
```

**Expected Results**:
```
# SPF
"v=spf1 include:amazonses.com ~all"

# DMARC
"v=DMARC1; p=none; rua=mailto:dmarc-reports@ccoe.hearst.com; pct=100; adkim=s; aspf=s"

# MX
10 feedback-smtp.us-east-1.amazonses.com.

# Bounce SPF
"v=spf1 include:amazonses.com ~all"
```

## 🚨 Troubleshooting

### Error: "No deliverability configuration found"

**Problem**: Missing `deliverability` section in config.json

**Solution**: Add the deliverability configuration (see Step 1 above)

### Error: "Failed to assume DNS role"

**Problem**: Trust relationship or orchestration role permissions

**Solutions**:
1. Check DNS role trust relationship allows your orchestration account
2. Check orchestration role has `sts:AssumeRole` permission
3. Verify role ARN in config.json is correct

### Error: "Failed to assume SES role"

**Problem**: Trust relationship or orchestration role permissions

**Solutions**:
1. Check SES role trust relationship allows your orchestration account
2. Check orchestration role has `sts:AssumeRole` permission
3. Verify role ARN in config.json is correct

### Error: "Access Denied" when creating DNS records

**Problem**: Route53 policy doesn't allow the operation

**Solutions**:
1. Verify the record name matches the policy pattern
2. Check the record type is allowed (CNAME, TXT, MX)
3. Verify the action is UPSERT
4. Check CloudTrail for detailed error

### Error: "Access Denied" when configuring SES

**Problem**: SES role doesn't have required permissions

**Solutions**:
1. Check SES role has required permissions (see Step 2)
2. Verify resource ARNs match the account
3. Check CloudTrail for detailed error

### Error: "Configuration set already exists"

**Problem**: Configuration set was created in a previous run

**Solution**: This is normal - the code handles this gracefully. The existing config set will be used.

## 📝 Complete Example config.json

Here's what your config.json should look like with deliverability added:

```json
{
  "aws_region": "us-east-1",
  "log_level": "info",
  "s3_config": {
    "bucket_name": "4cm-prod-ccoe-change-management-metadata"
  },
  "customer_mappings": {
    "htsnonprod": {
      "customer_code": "htsnonprod",
      "environment": "nonprod",
      "customer_name": "HTS NonProd",
      "region": "us-east-1",
      "identity_center_role_arn": "arn:aws:iam::978660766591:role/hts-ccoe-customer-contact-importer",
      "ses_role_arn": "arn:aws:iam::869445953789:role/hts-ccoe-customer-contact-manager",
      "sqs_queue_arn": "arn:aws:sqs:us-east-1:730335533660:hts-prod-ccoe-customer-contact-manager-htsnonprod",
      "deliverability": {
        "mail_from_domain": "bounce.ccoe.hearst.com",
        "configuration_set_name": "htsnonprod-ccoe-emails",
        "dmarc_policy": "none",
        "dmarc_report_email": "dmarc-reports@ccoe.hearst.com"
      }
    },
    "hts": {
      "customer_code": "hts",
      "environment": "prod",
      "customer_name": "HTS Prod",
      "region": "us-east-1",
      "identity_center_role_arn": "arn:aws:iam::748906912469:role/hts-ccoe-customer-contact-importer",
      "ses_role_arn": "arn:aws:iam::654654178002:role/hts-ccoe-customer-contact-manager",
      "sqs_queue_arn": "arn:aws:sqs:us-east-1:730335533660:hts-prod-ccoe-customer-contact-manager-hts",
      "deliverability": {
        "mail_from_domain": "bounce.ccoe.hearst.com",
        "configuration_set_name": "hts-ccoe-emails",
        "dmarc_policy": "none",
        "dmarc_report_email": "dmarc-reports@ccoe.hearst.com"
      }
    }
  },
  "email_config": {
    "sender_address": "ccoe@ccoe.hearst.com",
    "meeting_organizer": "ccoe@hearst.com",
    "portal_base_url": "https://change-management.ccoe.hearst.com",
    "domain": "ccoe.hearst.com"
  },
  "route53_config": {
    "zone_id": "Z02954802IDGJ8J3833M2",
    "role_arn": "arn:aws:iam::533267020082:role/ccoe-ses-dns-management"
  }
}
```

## ✅ Final Checklist

Before running the CLI:

- [ ] Added `deliverability` config to config.json for test customer
- [ ] Verified SES role has required permissions
- [ ] Verified orchestration role can assume both DNS and SES roles
- [ ] Verified trust relationships are configured
- [ ] Built the CLI binary
- [ ] AWS credentials configured for orchestration account
- [ ] Ran `show-deliverability-dns` successfully
- [ ] Ran dry-run successfully
- [ ] Ready to configure SES (without DNS first)
- [ ] Ready to configure DNS records

## 🎯 Recommended Testing Order

1. ✅ Test with **htsnonprod** first (non-production)
2. ✅ Run **show-deliverability-dns** (safe, no changes)
3. ✅ Run **dry-run** (safe, no changes)
4. ✅ Configure **SES only** (no DNS)
5. ✅ Verify SES configuration in AWS console
6. ✅ Configure **DNS records**
7. ✅ Verify DNS records with `dig`
8. ✅ Run **verify-deliverability**
9. ✅ Send test email
10. ✅ Roll out to **hts** (production)

Good luck with testing! 🚀
