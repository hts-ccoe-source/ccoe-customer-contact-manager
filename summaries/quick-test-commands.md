# Quick Test Commands Reference

## 🚀 Copy-Paste Commands for Testing

### 1. Show What's Needed (Safe - No Changes)
```bash
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns \
  --customer-code htsnonprod
```

### 2. Dry Run (Safe - No Changes)
```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code htsnonprod \
  --configure-dns \
  --dry-run \
  --log-level debug
```

### 3. Configure SES Only (No DNS)
```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code htsnonprod \
  --log-level info
```

### 4. Configure Everything (SES + DNS)
```bash
./ccoe-customer-contact-manager ses \
  --action configure-deliverability \
  --customer-code htsnonprod \
  --configure-dns \
  --log-level info
```

### 5. Verify Setup
```bash
./ccoe-customer-contact-manager ses \
  --action verify-deliverability \
  --customer-code htsnonprod \
  --log-level info
```

## 🔍 Verify DNS Records

```bash
# SPF record
dig TXT ccoe.hearst.com +short

# DMARC record
dig TXT _dmarc.ccoe.hearst.com +short

# MX record for bounce domain
dig MX bounce.ccoe.hearst.com +short

# SPF for bounce domain
dig TXT bounce.ccoe.hearst.com +short
```

## 🧪 Test Role Assumptions

```bash
# Test DNS role
aws sts assume-role \
  --role-arn arn:aws:iam::533267020082:role/ccoe-ses-dns-management \
  --role-session-name test-dns

# Test SES role
aws sts assume-role \
  --role-arn arn:aws:iam::869445953789:role/hts-ccoe-customer-contact-manager \
  --role-session-name test-ses
```

## 📝 Minimal config.json Addition

Add this to your `htsnonprod` customer in config.json:

```json
"deliverability": {
  "mail_from_domain": "bounce.ccoe.hearst.com",
  "configuration_set_name": "htsnonprod-ccoe-emails",
  "dmarc_policy": "none",
  "dmarc_report_email": "dmarc-reports@ccoe.hearst.com"
}
```

## 🎯 Testing Order

1. `show-deliverability-dns` - See what's needed
2. `configure-deliverability --dry-run` - Test without changes
3. `configure-deliverability` (no --configure-dns) - SES only
4. `configure-deliverability --configure-dns` - Add DNS
5. `verify-deliverability` - Check everything
6. `dig` commands - Verify DNS propagation
