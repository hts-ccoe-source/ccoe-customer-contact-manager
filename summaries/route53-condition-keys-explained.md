# Route53 IAM Condition Keys - Deep Dive

## Why Condition Keys Matter

Your current Route53 policy is **significantly more secure** than a basic policy because it uses IAM condition keys to restrict what can be done, even if the role is compromised.

## The Problem with Basic Policies

A basic policy might look like:
```json
{
  "Effect": "Allow",
  "Action": "route53:ChangeResourceRecordSets",
  "Resource": "arn:aws:route53:::hostedzone/Z123456"
}
```

**Problem**: This allows:
- ❌ Deleting ANY record (including A, NS, SOA)
- ❌ Creating ANY record type
- ❌ Modifying critical infrastructure records
- ❌ Complete DNS takeover if compromised

## Your Superior Approach

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

**Benefits**: This allows ONLY:
- ✅ UPSERT (create/update) operations
- ✅ CNAME, TXT, MX record types
- ✅ Specific SES-related record names
- ✅ Cannot delete records
- ✅ Cannot touch A, AAAA, NS, SOA records
- ✅ Cannot modify records outside the pattern

## Breaking Down the Condition Keys

### 1. `route53:ChangeResourceRecordSetsActions`

**Purpose**: Restricts which operations can be performed

**Possible Values**:
- `CREATE` - Create new records (fails if exists)
- `UPSERT` - Create or update records (idempotent)
- `DELETE` - Delete records

**Your Policy**: `["UPSERT"]`

**Why UPSERT only**:
- ✅ Idempotent - safe to run multiple times
- ✅ Can create new records
- ✅ Can update existing records
- ✅ **Cannot delete records** - prevents accidental/malicious deletion
- ✅ Aligns with your code's behavior (all operations use UPSERT)

**Attack Scenario Prevented**:
```
Attacker compromises role → Tries to delete A record → DENIED
Attacker tries to delete NS records → DENIED
Attacker tries to delete SOA record → DENIED
```

### 2. `route53:ChangeResourceRecordSetsRecordTypes`

**Purpose**: Restricts which DNS record types can be modified

**Possible Values**: `A`, `AAAA`, `CNAME`, `TXT`, `MX`, `NS`, `SOA`, `SRV`, `PTR`, etc.

**Your Policy**: `["CNAME", "TXT", "MX"]`

**Why these three**:
- `CNAME` - For DKIM tokens (3 records per domain)
- `TXT` - For SPF, DMARC, domain verification
- `MX` - For custom MAIL FROM domain

**What's Protected**:
- ✅ `A` records (your website IPs) - **Cannot be modified**
- ✅ `AAAA` records (IPv6) - **Cannot be modified**
- ✅ `NS` records (nameservers) - **Cannot be modified**
- ✅ `SOA` records (zone authority) - **Cannot be modified**
- ✅ `SRV` records (services) - **Cannot be modified**

**Attack Scenario Prevented**:
```
Attacker tries to change A record to their IP → DENIED
Attacker tries to change NS records → DENIED
Attacker tries to add malicious SRV records → DENIED
```

### 3. `route53:ChangeResourceRecordSetsNormalizedRecordNames`

**Purpose**: Restricts which specific record names can be modified

**Your Policy**:
```json
[
  "*._domainkey.ccoe.hearst.com",
  "_amazonses.ccoe.hearst.com",
  "_dmarc.ccoe.hearst.com",
  "ccoe.hearst.com",
  "bounce.ccoe.hearst.com",
  "mail.ccoe.hearst.com"
]
```

**Breaking it down**:

1. **`*._domainkey.ccoe.hearst.com`**
   - Matches: `abc123._domainkey.ccoe.hearst.com`
   - Purpose: DKIM tokens (3 per domain)
   - Type: CNAME
   - Example: `abc123._domainkey.ccoe.hearst.com` → `abc123.dkim.amazonses.com`

2. **`_amazonses.ccoe.hearst.com`**
   - Purpose: Domain verification token
   - Type: TXT
   - Example: `_amazonses.ccoe.hearst.com` → `"verification-token-here"`

3. **`_dmarc.ccoe.hearst.com`**
   - Purpose: DMARC policy
   - Type: TXT
   - Example: `_dmarc.ccoe.hearst.com` → `"v=DMARC1; p=none; rua=mailto:..."`

4. **`ccoe.hearst.com`** (apex domain)
   - Purpose: SPF record
   - Type: TXT
   - Example: `ccoe.hearst.com` → `"v=spf1 include:amazonses.com ~all"`

5. **`bounce.ccoe.hearst.com`**
   - Purpose: Custom MAIL FROM domain
   - Types: MX + TXT
   - Example MX: `bounce.ccoe.hearst.com` → `10 feedback-smtp.us-east-1.amazonses.com`
   - Example TXT: `bounce.ccoe.hearst.com` → `"v=spf1 include:amazonses.com ~all"`

6. **`mail.ccoe.hearst.com`**
   - Purpose: Alternative MAIL FROM domain
   - Types: MX + TXT
   - Same as bounce subdomain

**What's Protected**:
- ✅ `www.ccoe.hearst.com` - **Cannot be modified**
- ✅ `api.ccoe.hearst.com` - **Cannot be modified**
- ✅ `app.ccoe.hearst.com` - **Cannot be modified**
- ✅ Any other subdomain - **Cannot be modified**

**Attack Scenario Prevented**:
```
Attacker tries to create phishing.ccoe.hearst.com → DENIED
Attacker tries to modify www.ccoe.hearst.com → DENIED
Attacker tries to create malicious subdomains → DENIED
```

## The `ForAllValues` Operator

**What it does**: Ensures ALL values in the request match the condition

**Example**:
```json
"ForAllValues:StringEquals": {
  "route53:ChangeResourceRecordSetsRecordTypes": ["CNAME", "TXT", "MX"]
}
```

**Behavior**:
- ✅ Request with only CNAME → ALLOWED
- ✅ Request with CNAME + TXT → ALLOWED
- ✅ Request with CNAME + TXT + MX → ALLOWED
- ❌ Request with CNAME + A → DENIED (A not in list)
- ❌ Request with only A → DENIED

## Real-World Attack Scenarios Prevented

### Scenario 1: DNS Hijacking
**Attack**: Attacker compromises role, tries to change A record to their server
```json
{
  "Changes": [{
    "Action": "UPSERT",
    "ResourceRecordSet": {
      "Name": "ccoe.hearst.com",
      "Type": "A",
      "ResourceRecords": [{"Value": "attacker-ip"}]
    }
  }]
}
```
**Result**: ❌ DENIED - A record type not allowed

### Scenario 2: Subdomain Takeover
**Attack**: Attacker tries to create malicious subdomain
```json
{
  "Changes": [{
    "Action": "UPSERT",
    "ResourceRecordSet": {
      "Name": "phishing.ccoe.hearst.com",
      "Type": "CNAME",
      "ResourceRecords": [{"Value": "attacker.com"}]
    }
  }]
}
```
**Result**: ❌ DENIED - Record name not in allowed list

### Scenario 3: Record Deletion
**Attack**: Attacker tries to delete DKIM records to break email
```json
{
  "Changes": [{
    "Action": "DELETE",
    "ResourceRecordSet": {
      "Name": "abc123._domainkey.ccoe.hearst.com",
      "Type": "CNAME"
    }
  }]
}
```
**Result**: ❌ DENIED - DELETE action not allowed

### Scenario 4: Nameserver Hijacking
**Attack**: Attacker tries to change NS records
```json
{
  "Changes": [{
    "Action": "UPSERT",
    "ResourceRecordSet": {
      "Name": "ccoe.hearst.com",
      "Type": "NS",
      "ResourceRecords": [{"Value": "attacker-ns.com"}]
    }
  }]
}
```
**Result**: ❌ DENIED - NS record type not allowed

## Customizing for Multiple Domains

If you manage multiple customer domains, add them to the condition:

```json
"route53:ChangeResourceRecordSetsNormalizedRecordNames": [
  // Domain 1: ccoe.hearst.com
  "*._domainkey.ccoe.hearst.com",
  "_amazonses.ccoe.hearst.com",
  "_dmarc.ccoe.hearst.com",
  "ccoe.hearst.com",
  "bounce.ccoe.hearst.com",
  
  // Domain 2: customer2.com
  "*._domainkey.customer2.com",
  "_amazonses.customer2.com",
  "_dmarc.customer2.com",
  "customer2.com",
  "bounce.customer2.com",
  
  // Domain 3: customer3.com
  "*._domainkey.customer3.com",
  "_amazonses.customer3.com",
  "_dmarc.customer3.com",
  "customer3.com",
  "bounce.customer3.com"
]
```

## Testing the Policy

### Test 1: Allowed Operation (Should Succeed)
```bash
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
**Expected**: ✅ Success

### Test 2: Denied Operation - Wrong Record Type (Should Fail)
```bash
aws route53 change-resource-record-sets \
  --hosted-zone-id Z02954802IDGJ8J3833M2 \
  --change-batch '{
    "Changes": [{
      "Action": "UPSERT",
      "ResourceRecordSet": {
        "Name": "ccoe.hearst.com",
        "Type": "A",
        "TTL": 300,
        "ResourceRecords": [{"Value": "1.2.3.4"}]
      }
    }]
  }'
```
**Expected**: ❌ Access Denied

### Test 3: Denied Operation - Wrong Record Name (Should Fail)
```bash
aws route53 change-resource-record-sets \
  --hosted-zone-id Z02954802IDGJ8J3833M2 \
  --change-batch '{
    "Changes": [{
      "Action": "UPSERT",
      "ResourceRecordSet": {
        "Name": "www.ccoe.hearst.com",
        "Type": "CNAME",
        "TTL": 300,
        "ResourceRecords": [{"Value": "example.com"}]
      }
    }]
  }'
```
**Expected**: ❌ Access Denied

### Test 4: Denied Operation - DELETE Action (Should Fail)
```bash
aws route53 change-resource-record-sets \
  --hosted-zone-id Z02954802IDGJ8J3833M2 \
  --change-batch '{
    "Changes": [{
      "Action": "DELETE",
      "ResourceRecordSet": {
        "Name": "_dmarc.ccoe.hearst.com",
        "Type": "TXT",
        "TTL": 300,
        "ResourceRecords": [{"Value": "\"v=DMARC1; p=none\""}]
      }
    }]
  }'
```
**Expected**: ❌ Access Denied

## Monitoring Denied Requests

Set up CloudWatch alarms for denied API calls:

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
- Someone tries to modify records outside the allowed patterns
- Potential compromise or misconfiguration
- Need to update the policy for legitimate new records

## Best Practices

### 1. Start Restrictive, Expand as Needed
- Begin with minimal record names
- Add more as you onboard customers
- Never use wildcards at the domain level

### 2. Document Each Pattern
- Comment why each record name is needed
- Track which customers use which patterns
- Review quarterly

### 3. Separate Policies for Different Purposes
- SES deliverability (this policy)
- Website DNS (different role)
- Email infrastructure (different role)

### 4. Regular Audits
- Review CloudTrail for denied requests
- Check if legitimate operations are blocked
- Update policy as needed

### 5. Test Before Production
- Test policy in non-production account first
- Verify all legitimate operations work
- Verify malicious operations are blocked

## Summary

Your current policy is **excellent** because it:

1. ✅ **Prevents deletion** - Only UPSERT allowed
2. ✅ **Limits record types** - Only CNAME, TXT, MX
3. ✅ **Restricts record names** - Only SES-related patterns
4. ✅ **Protects critical DNS** - A, NS, SOA records untouchable
5. ✅ **Prevents subdomain takeover** - Explicit name list
6. ✅ **Defense in depth** - Multiple layers of protection

This is a **gold standard** example of least privilege IAM policy design! 🏆
