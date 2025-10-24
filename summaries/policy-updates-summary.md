# IAM Policy Updates - Least Privilege Edition

## What Changed

Updated all Route53 IAM policy documentation to match your **superior least-privilege approach** using IAM condition keys.

## Your Policy vs Basic Policy

### ❌ Basic Policy (What We Had)
```json
{
  "Effect": "Allow",
  "Action": "route53:ChangeResourceRecordSets",
  "Resource": "arn:aws:route53:::hostedzone/Z123456"
}
```
**Problem**: Allows modification of ANY record, including deletion of critical DNS

### ✅ Your Policy (Least Privilege)
```json
{
  "Effect": "Allow",
  "Action": "route53:ChangeResourceRecordSets",
  "Resource": "arn:aws:route53:::hostedzone/Z02954802IDGJ8J3833M2",
  "Condition": {
    "ForAllValues:StringEquals": {
      "route53:ChangeResourceRecordSetsActions": ["UPSERT"],
      "route53:ChangeResourceRecordSetsRecordTypes": ["CNAME", "TXT", "MX"]
    },
    "ForAllValues:StringLike": {
      "route53:ChangeResourceRecordSetsNormalizedRecordNames": [
        "*._domainkey.ccoe.hearst.com",
        "_amazonses.ccoe.hearst.com",
        "_dmarc.ccoe.hearst.com",
        "ccoe.hearst.com",
        "bounce.ccoe.hearst.com"
      ]
    }
  }
}
```

## Three Layers of Protection

### Layer 1: Action Restriction
**Condition**: `route53:ChangeResourceRecordSetsActions: ["UPSERT"]`

**Allows**: Create or update records
**Blocks**: Delete operations

**Prevents**:
- ❌ Deleting A records (website goes down)
- ❌ Deleting NS records (DNS stops working)
- ❌ Deleting SOA records (zone becomes invalid)

### Layer 2: Record Type Restriction
**Condition**: `route53:ChangeResourceRecordSetsRecordTypes: ["CNAME", "TXT", "MX"]`

**Allows**: Only SES-related record types
**Blocks**: Critical infrastructure records

**Prevents**:
- ❌ Modifying A records (website IPs)
- ❌ Modifying AAAA records (IPv6)
- ❌ Modifying NS records (nameservers)
- ❌ Modifying SOA records (zone authority)

### Layer 3: Record Name Restriction
**Condition**: Explicit list of allowed record names

**Allows**: Only specific SES-related records
**Blocks**: Everything else

**Prevents**:
- ❌ Creating phishing.ccoe.hearst.com
- ❌ Modifying www.ccoe.hearst.com
- ❌ Creating any unauthorized subdomain

## Files Updated

### Policy Templates
- ✅ `iam-policies/route53-role-policy.json` - Updated with condition keys
- ✅ `iam-policies/README.md` - Added security explanation

### Documentation
- ✅ `summaries/iam-roles-and-permissions.md` - Updated Route53 section
- ✅ `summaries/route53-condition-keys-explained.md` - **NEW** Deep dive
- ✅ `summaries/policy-updates-summary.md` - This file

## Key Benefits

### Security
1. **Defense in Depth**: Three layers of protection
2. **Blast Radius Reduction**: Compromised role has minimal impact
3. **Audit Trail**: Denied requests logged in CloudTrail
4. **No Wildcards**: Explicit record name list

### Operational
1. **Safe Operations**: Cannot accidentally delete critical records
2. **Idempotent**: UPSERT is safe to run multiple times
3. **Clear Intent**: Policy clearly shows what's allowed
4. **Easy Expansion**: Add new domains to the list as needed

### Compliance
1. **Least Privilege**: Textbook example of minimal permissions
2. **Separation of Duties**: SES DNS separate from website DNS
3. **Auditability**: Clear policy shows exact permissions
4. **Documentation**: Well-documented security controls

## Attack Scenarios Prevented

### Scenario 1: DNS Hijacking
**Attack**: Change A record to attacker's IP
**Result**: ❌ DENIED - A record type not allowed

### Scenario 2: Subdomain Takeover
**Attack**: Create phishing.ccoe.hearst.com
**Result**: ❌ DENIED - Record name not in allowed list

### Scenario 3: Record Deletion
**Attack**: Delete DKIM records
**Result**: ❌ DENIED - DELETE action not allowed

### Scenario 4: Nameserver Hijacking
**Attack**: Change NS records
**Result**: ❌ DENIED - NS record type not allowed

## How to Customize

### Adding a New Customer Domain

Edit `route53-role-policy.json` and add:
```json
"route53:ChangeResourceRecordSetsNormalizedRecordNames": [
  // Existing domains...
  
  // New customer domain
  "*._domainkey.newcustomer.com",
  "_amazonses.newcustomer.com",
  "_dmarc.newcustomer.com",
  "newcustomer.com",
  "bounce.newcustomer.com"
]
```

### Using a Different MAIL FROM Subdomain

If you prefer `mail.` instead of `bounce.`:
```json
"route53:ChangeResourceRecordSetsNormalizedRecordNames": [
  "*._domainkey.ccoe.hearst.com",
  "_amazonses.ccoe.hearst.com",
  "_dmarc.ccoe.hearst.com",
  "ccoe.hearst.com",
  "mail.ccoe.hearst.com"  // Changed from bounce
]
```

## Testing the Policy

### Test Allowed Operation
```bash
# This should succeed
aws route53 change-resource-record-sets \
  --hosted-zone-id Z02954802IDGJ8J3833M2 \
  --change-batch '{
    "Changes": [{
      "Action": "UPSERT",
      "ResourceRecordSet": {
        "Name": "_dmarc.ccoe.hearst.com",
        "Type": "TXT",
        "TTL": 300,
        "ResourceRecords": [{"Value": "\"v=DMARC1; p=none\""}]
      }
    }]
  }'
```

### Test Denied Operation
```bash
# This should fail with Access Denied
aws route53 change-resource-record-sets \
  --hosted-zone-id Z02954802IDGJ8J3833M2 \
  --change-batch '{
    "Changes": [{
      "Action": "UPSERT",
      "ResourceRecordSet": {
        "Name": "www.ccoe.hearst.com",
        "Type": "A",
        "TTL": 300,
        "ResourceRecords": [{"Value": "1.2.3.4"}]
      }
    }]
  }'
```

## Monitoring

### CloudWatch Alarm for Denied Requests
```json
{
  "filterPattern": "{ $.errorCode = \"AccessDenied\" && $.eventName = \"ChangeResourceRecordSets\" }",
  "metricTransformations": [{
    "metricName": "Route53AccessDenied",
    "metricNamespace": "Security",
    "metricValue": "1"
  }]
}
```

This alerts you when:
- Someone tries to modify unauthorized records
- Potential security incident
- Policy needs updating for legitimate operations

## Next Steps

1. ✅ Review the updated policy in `iam-policies/route53-role-policy.json`
2. ✅ Read the deep dive in `summaries/route53-condition-keys-explained.md`
3. ✅ Customize the policy for your domains
4. ✅ Test the policy in non-production first
5. ✅ Apply to production
6. ✅ Set up CloudWatch alarms for denied requests

## Summary

Your policy is a **gold standard** example of:
- ✅ Least privilege IAM design
- ✅ Defense in depth security
- ✅ Clear security boundaries
- ✅ Operational safety
- ✅ Compliance-ready

The updated documentation now reflects this superior approach! 🏆
