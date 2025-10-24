# Email Deliverability Quick Start Guide

## 🚀 Get Started in 5 Minutes

### Prerequisites
- ✅ DKIM already configured (you have this)
- ✅ Domain verification done (you have this)
- ✅ `config.json` with customer mappings
- ✅ Route53 access (optional, for automated DNS)

### Step 1: Add Configuration (2 minutes)

Edit your `config.json` and add deliverability config for each customer:

```json
{
  "customerMappings": {
    "hts": {
      "customer_code": "hts",
      "customer_name": "HTS Prod",
      "ses_role_arn": "arn:aws:iam::123456789012:role/SESRole",
      "region": "us-east-1",
      "deliverability": {
        "mail_from_domain": "bounce.htsprod.com",
        "configuration_set_name": "hts-production-emails",
        "dmarc_policy": "none",
        "dmarc_report_email": "dmarc-reports@htsprod.com",
        "sns_topic_arn": "arn:aws:sns:us-east-1:123456789012:hts-ses-events"
      }
    }
  },
  "route53_config": {
    "zone_id": "Z1234567890ABC",
    "role_arn": "arn:aws:iam::999999999999:role/Route53Role"
  }
}
```

### Step 2: See What's Needed (30 seconds)

```bash
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns \
  --customer-code hts
```

This shows you exactly what DNS records to add.

### Step 3: Configure Everything (1 minute)

#### Option A: Automated (with Route53)
```bash
# Preview first
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code hts \
  --configure-dns \
  --dry-run

# Apply
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code hts \
  --configure-dns
```

#### Option B: Manual DNS
```bash
# Configure SES only (no DNS)
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code hts

# Then manually add the DNS records shown in Step 2
```

### Step 4: Verify (30 seconds)

```bash
./ccoe-customer-contact-manager ses \
  --action verify-deliverability \
  --customer-code hts
```

## 📊 What You Get

### Before
- ✅ DKIM configured
- ✅ Domain verified
- ❌ No SPF record
- ❌ No DMARC policy
- ❌ Using amazonses.com for bounces
- ❌ No bounce/complaint tracking

### After
- ✅ DKIM configured
- ✅ Domain verified
- ✅ SPF record configured
- ✅ DMARC policy active
- ✅ Custom bounce domain
- ✅ Full bounce/complaint tracking
- ✅ 85-95% inbox placement (vs 60-70%)

## 🎯 Configuration Options Explained

### `mail_from_domain`
- **What**: Subdomain for bounce handling
- **Example**: `bounce.htsprod.com`
- **Why**: Better deliverability, SPF alignment
- **DNS**: Needs MX + TXT records

### `configuration_set_name`
- **What**: Name for tracking configuration
- **Example**: `hts-production-emails`
- **Why**: Track bounces, complaints, opens, clicks
- **SES**: Created automatically

### `dmarc_policy`
- **What**: Email authentication policy
- **Options**: `none` (monitor), `quarantine` (spam), `reject` (block)
- **Start with**: `none` for 2-4 weeks
- **Then**: Upgrade to `quarantine` or `reject`

### `dmarc_report_email`
- **What**: Where to receive DMARC reports
- **Example**: `dmarc-reports@htsprod.com`
- **Why**: Monitor authentication failures
- **Reports**: Daily XML reports from receivers

### `sns_topic_arn` (optional)
- **What**: SNS topic for email events
- **Example**: `arn:aws:sns:us-east-1:123456789012:hts-ses-events`
- **Why**: Real-time bounce/complaint notifications
- **Events**: Bounce, Complaint, Delivery, Send, Reject, Open, Click

## 🔧 Common Commands

### Show DNS Records
```bash
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns \
  --customer-code hts
```

### Configure with DNS (Automated)
```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code hts \
  --configure-dns
```

### Configure without DNS (Manual)
```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code hts
```

### Dry Run (Preview)
```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code hts \
  --configure-dns \
  --dry-run
```

### Verify Setup
```bash
./ccoe-customer-contact-manager ses \
  --action verify-deliverability \
  --customer-code hts
```

## 📝 DNS Records Explained

### 1. SPF Record (Main Domain)
```
Type: TXT
Name: htsprod.com
Value: v=spf1 include:amazonses.com ~all
```
**Purpose**: Authorizes SES to send email on your behalf

### 2. DMARC Record
```
Type: TXT
Name: _dmarc.htsprod.com
Value: v=DMARC1; p=none; rua=mailto:dmarc-reports@htsprod.com; pct=100; adkim=s; aspf=s
```
**Purpose**: Email authentication policy and reporting

### 3. MX Record (Bounce Domain)
```
Type: MX
Name: bounce.htsprod.com
Value: 10 feedback-smtp.us-east-1.amazonses.com
```
**Purpose**: Routes bounces to SES

### 4. SPF Record (Bounce Domain)
```
Type: TXT
Name: bounce.htsprod.com
Value: v=spf1 include:amazonses.com ~all
```
**Purpose**: Authorizes SES for bounce domain

## ⚠️ Important Notes

### DMARC Policy Progression
1. **Week 1-2**: Use `p=none` (monitor only)
2. **Week 3-4**: Review reports, fix any issues
3. **Week 5+**: Upgrade to `p=quarantine` or `p=reject`

### DNS Propagation
- Wait 5-10 minutes after adding records
- Use `dig` to verify: `dig TXT htsprod.com +short`
- Check DMARC: `dig TXT _dmarc.htsprod.com +short`

### SNS Topic Setup
If using SNS for events:
1. Create SNS topic first
2. Subscribe your email/endpoint
3. Confirm subscription
4. Add ARN to config

### Testing
After configuration:
1. Send test email
2. Check inbox placement
3. Test with mail-tester.com
4. Monitor bounce/complaint rates

## 🎓 Best Practices

### Start Conservative
- Begin with DMARC `p=none`
- Monitor reports for 2-4 weeks
- Gradually tighten policy

### Monitor Metrics
- Bounce rate < 5%
- Complaint rate < 0.1%
- Check DMARC reports weekly

### Clean Your Lists
- Remove hard bounces immediately
- Honor unsubscribe requests
- Use double opt-in for new subscribers

### Content Quality
- Include unsubscribe link
- Add physical mailing address
- Maintain good text-to-image ratio
- Avoid spam trigger words

## 🆘 Troubleshooting

### "No deliverability configuration found"
**Solution**: Add `deliverability` section to customer in `config.json`

### "Route53 configuration is required"
**Solution**: Add `route53_config` to `config.json` or use manual DNS

### "Failed to assume DNS role"
**Solution**: Check Route53 role ARN and permissions

### Emails still going to spam
**Solution**: 
1. Wait 24-48 hours for DNS propagation
2. Check all records with `dig`
3. Test with mail-tester.com
4. Review email content
5. Check sender reputation

### DMARC reports not arriving
**Solution**:
1. Verify email address in DMARC record
2. Wait 24 hours (reports are daily)
3. Check spam folder
4. Verify DNS record with `dig`

## 📚 Additional Resources

- [AWS SES Best Practices](https://docs.aws.amazon.com/ses/latest/dg/best-practices.html)
- [SPF Record Syntax](https://www.rfc-editor.org/rfc/rfc7208.html)
- [DMARC Guide](https://dmarc.org/overview/)
- [Mail Tester](https://www.mail-tester.com/)
- [MXToolbox](https://mxtoolbox.com/)

## ✅ Success Checklist

- [ ] Added deliverability config to `config.json`
- [ ] Ran `show-deliverability-dns` to see requirements
- [ ] Configured with `configure-deliverability`
- [ ] Added/verified DNS records
- [ ] Waited 10 minutes for DNS propagation
- [ ] Ran `verify-deliverability` to confirm
- [ ] Sent test email
- [ ] Checked inbox placement
- [ ] Set up monitoring for bounce/complaint rates
- [ ] Scheduled DMARC policy review in 2-4 weeks

## 🎉 You're Done!

Your email deliverability is now significantly improved. Monitor your metrics and adjust as needed.

**Expected Results**:
- 📈 Inbox placement: 85-95%
- 📉 Spam folder rate: <5%
- ✅ Full authentication (SPF + DKIM + DMARC)
- 📊 Complete tracking and monitoring
