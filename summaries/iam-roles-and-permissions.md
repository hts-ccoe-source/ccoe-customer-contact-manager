# IAM Roles and Permissions for SES Deliverability

## Architecture Overview

The SES deliverability configuration involves **three types of AWS accounts**:

1. **Orchestration Account** - Where the CLI tool runs (central management)
2. **DNS Account** - Where Route53 hosted zones are managed (shared DNS)
3. **Customer SES Accounts** - Where SES services are configured (one per customer)

See the architecture diagram: `generated-diagrams/ses-deliverability-architecture.png.png`

## Account Roles and Responsibilities

### 1. Orchestration Account (Central Management)

**Purpose**: Central account where DevOps engineers run the CLI tool

**What runs here**:
- Go CLI binary (`ccoe-customer-contact-manager`)
- Configuration file (`config.json`)
- Credentials for assuming roles in other accounts

**IAM Role**: `OrchestrationRole` (or your existing execution role)

**Permissions Needed**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sts:AssumeRole"
      ],
      "Resource": [
        "arn:aws:iam::DNS-ACCOUNT-ID:role/Route53ManagementRole",
        "arn:aws:iam::CUSTOMER1-ACCOUNT-ID:role/SESManagementRole",
        "arn:aws:iam::CUSTOMER2-ACCOUNT-ID:role/SESManagementRole",
        "arn:aws:iam::CUSTOMERN-ACCOUNT-ID:role/SESManagementRole"
      ]
    }
  ]
}
```

**Trust Relationship**: None needed if running with IAM user credentials or instance profile

**Who uses this**: DevOps engineers, CI/CD pipelines, Lambda functions

---

### 2. DNS Account (Shared Route53)

**Purpose**: Centralized DNS management for all customer domains

**What's here**:
- Route53 hosted zones for all customer domains
- DNS records (A, CNAME, TXT, MX, etc.)

**IAM Role**: `Route53ManagementRole`

**Permissions Needed** (Least Privilege with Conditions):
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Route53ReadHostedZone",
      "Effect": "Allow",
      "Action": [
        "route53:GetHostedZone",
        "route53:ListResourceRecordSets"
      ],
      "Resource": "arn:aws:route53:::hostedzone/Z02954802IDGJ8J3833M2"
    },
    {
      "Sid": "Route53ManageSESDeliverabilityRecords",
      "Effect": "Allow",
      "Action": "route53:ChangeResourceRecordSets",
      "Resource": "arn:aws:route53:::hostedzone/Z02954802IDGJ8J3833M2",
      "Condition": {
        "ForAllValues:StringEquals": {
          "route53:ChangeResourceRecordSetsActions": ["UPSERT"],
          "route53:ChangeResourceRecordSetsRecordTypes": [
            "CNAME",
            "TXT",
            "MX"
          ]
        },
        "ForAllValues:StringLike": {
          "route53:ChangeResourceRecordSetsNormalizedRecordNames": [
            "*._domainkey.ccoe.hearst.com",
            "_amazonses.ccoe.hearst.com",
            "_dmarc.ccoe.hearst.com",
            "ccoe.hearst.com",
            "bounce.ccoe.hearst.com",
            "mail.ccoe.hearst.com"
          ]
        }
      }
    },
    {
      "Sid": "Route53GetChangeStatus",
      "Effect": "Allow",
      "Action": "route53:GetChange",
      "Resource": "arn:aws:route53:::change/*"
    }
  ]
}
```

**Key Security Features**:
- ✅ **Action Restriction**: Only allows `UPSERT` (create/update), not `DELETE`
- ✅ **Record Type Restriction**: Only `CNAME`, `TXT`, and `MX` records
- ✅ **Name Pattern Restriction**: Only specific SES-related record names
- ✅ **Resource Scoping**: Limited to specific hosted zone
- ✅ **No Wildcards**: Explicit list of allowed record names

**Trust Relationship**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::ORCHESTRATION-ACCOUNT-ID:role/OrchestrationRole"
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "optional-external-id-for-security"
        }
      }
    }
  ]
}
```

**DNS Records Managed**:
- SPF records (TXT on main domain: `ccoe.hearst.com`)
- DMARC records (TXT on `_dmarc` subdomain: `_dmarc.ccoe.hearst.com`)
- DKIM records (CNAME: `*._domainkey.ccoe.hearst.com`)
- MX records (for custom MAIL FROM domain: `bounce.ccoe.hearst.com` or `mail.ccoe.hearst.com`)
- Domain verification (TXT: `_amazonses.ccoe.hearst.com`)

**Understanding the Condition Keys**:

1. **`route53:ChangeResourceRecordSetsActions`**: Restricts to UPSERT only
   - `UPSERT` = Create or Update (safe)
   - `DELETE` = Not allowed (prevents accidental deletion)
   - `CREATE` = Not needed (UPSERT handles this)

2. **`route53:ChangeResourceRecordSetsRecordTypes`**: Limits record types
   - `CNAME` = For DKIM tokens
   - `TXT` = For SPF, DMARC, verification
   - `MX` = For custom MAIL FROM domain
   - Prevents modification of A, AAAA, NS, SOA records

3. **`route53:ChangeResourceRecordSetsNormalizedRecordNames`**: Explicit name patterns
   - `*._domainkey.ccoe.hearst.com` = DKIM tokens (wildcard for 3 tokens)
   - `_amazonses.ccoe.hearst.com` = Domain verification
   - `_dmarc.ccoe.hearst.com` = DMARC policy
   - `ccoe.hearst.com` = SPF record on apex
   - `bounce.ccoe.hearst.com` = Custom MAIL FROM subdomain
   - `mail.ccoe.hearst.com` = Alternative MAIL FROM subdomain

**Customizing for Your Domains**:

If you have multiple customer domains, add them to the condition:
```json
"route53:ChangeResourceRecordSetsNormalizedRecordNames": [
  "*._domainkey.ccoe.hearst.com",
  "_amazonses.ccoe.hearst.com",
  "_dmarc.ccoe.hearst.com",
  "ccoe.hearst.com",
  "bounce.ccoe.hearst.com",
  "*._domainkey.customer2.com",
  "_amazonses.customer2.com",
  "_dmarc.customer2.com",
  "customer2.com",
  "bounce.customer2.com"
]
```

---

### 3. Customer SES Accounts (Per Customer)

**Purpose**: Individual SES configuration for each customer organization

**What's here**:
- SES domain identities
- SES configuration sets
- SES event destinations
- SNS topics for email events

**IAM Role**: `SESManagementRole` (one per customer account)

**Permissions Needed**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SESIdentityManagement",
      "Effect": "Allow",
      "Action": [
        "ses:GetEmailIdentity",
        "ses:CreateEmailIdentity",
        "ses:DeleteEmailIdentity",
        "ses:PutEmailIdentityDkimAttributes",
        "ses:PutEmailIdentityMailFromAttributes",
        "ses:PutEmailIdentityConfigurationSetAttributes"
      ],
      "Resource": [
        "arn:aws:ses:*:CUSTOMER-ACCOUNT-ID:identity/*"
      ]
    },
    {
      "Sid": "SESConfigurationSetManagement",
      "Effect": "Allow",
      "Action": [
        "ses:CreateConfigurationSet",
        "ses:DeleteConfigurationSet",
        "ses:GetConfigurationSet",
        "ses:ListConfigurationSets",
        "ses:PutConfigurationSetDeliveryOptions",
        "ses:PutConfigurationSetReputationOptions",
        "ses:PutConfigurationSetSendingOptions",
        "ses:PutConfigurationSetSuppressionOptions",
        "ses:PutConfigurationSetTrackingOptions"
      ],
      "Resource": [
        "arn:aws:ses:*:CUSTOMER-ACCOUNT-ID:configuration-set/*"
      ]
    },
    {
      "Sid": "SESEventDestinationManagement",
      "Effect": "Allow",
      "Action": [
        "ses:CreateConfigurationSetEventDestination",
        "ses:DeleteConfigurationSetEventDestination",
        "ses:GetConfigurationSetEventDestinations",
        "ses:PutConfigurationSetEventDestination"
      ],
      "Resource": [
        "arn:aws:ses:*:CUSTOMER-ACCOUNT-ID:configuration-set/*"
      ]
    },
    {
      "Sid": "SESAccountLevelOperations",
      "Effect": "Allow",
      "Action": [
        "ses:GetAccount",
        "ses:PutAccountDetails",
        "ses:PutAccountSendingAttributes",
        "ses:PutAccountSuppressionAttributes"
      ],
      "Resource": "*"
    },
    {
      "Sid": "SNSTopicAccess",
      "Effect": "Allow",
      "Action": [
        "sns:GetTopicAttributes",
        "sns:SetTopicAttributes"
      ],
      "Resource": [
        "arn:aws:sns:*:CUSTOMER-ACCOUNT-ID:*ses*"
      ]
    }
  ]
}
```

**Trust Relationship**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::ORCHESTRATION-ACCOUNT-ID:role/OrchestrationRole"
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "optional-external-id-for-security"
        }
      }
    }
  ]
}
```

**SES Resources Managed**:
- Email domain identities
- DKIM signing configuration
- Custom MAIL FROM domain
- Configuration sets
- Event destinations (SNS)
- Suppression lists
- Reputation metrics

---

## Configuration Flow

### Step 1: CLI Authenticates to Orchestration Account
```
DevOps Engineer → AWS Credentials → Orchestration Account
```

### Step 2: CLI Assumes DNS Role
```
Orchestration Role → sts:AssumeRole → Route53ManagementRole (DNS Account)
```

### Step 3: CLI Configures DNS Records
```
Route53ManagementRole → route53:ChangeResourceRecordSets → Hosted Zone
```

Creates:
- SPF record: `TXT @ "v=spf1 include:amazonses.com ~all"`
- DMARC record: `TXT _dmarc "v=DMARC1; p=none; ..."`
- MX record: `MX bounce.domain.com "10 feedback-smtp.region.amazonses.com"`
- SPF for bounce: `TXT bounce.domain.com "v=spf1 include:amazonses.com ~all"`

### Step 4: CLI Assumes Customer SES Role
```
Orchestration Role → sts:AssumeRole → SESManagementRole (Customer Account)
```

### Step 5: CLI Configures SES Resources
```
SESManagementRole → ses:* → SES Service
```

Creates:
- Custom MAIL FROM domain configuration
- Configuration set with tracking
- Event destination (SNS topic)
- Assigns configuration set to domain

---

## Security Best Practices

### 1. Use External IDs
Add external IDs to trust relationships for additional security:
```json
"Condition": {
  "StringEquals": {
    "sts:ExternalId": "unique-random-string-per-customer"
  }
}
```

### 2. Least Privilege
- Only grant permissions needed for specific operations
- Use resource-level permissions where possible
- Avoid wildcards (`*`) in Resource ARNs

### 3. Session Duration
Configure appropriate session durations for assumed roles:
```json
"MaxSessionDuration": 3600  // 1 hour
```

### 4. MFA Requirement (Optional)
Require MFA for sensitive operations:
```json
"Condition": {
  "Bool": {
    "aws:MultiFactorAuthPresent": "true"
  }
}
```

### 5. IP Restrictions (Optional)
Restrict role assumption to specific IP ranges:
```json
"Condition": {
  "IpAddress": {
    "aws:SourceIp": ["10.0.0.0/8", "192.168.1.0/24"]
  }
}
```

### 6. CloudTrail Logging
Enable CloudTrail in all accounts to audit:
- Role assumptions
- SES configuration changes
- Route53 DNS modifications

---

## Setup Instructions

### For Orchestration Account

1. **Create or identify the orchestration role**:
   ```bash
   aws iam create-role \
     --role-name OrchestrationRole \
     --assume-role-policy-document file://orchestration-trust.json
   ```

2. **Attach policy for assuming other roles**:
   ```bash
   aws iam put-role-policy \
     --role-name OrchestrationRole \
     --policy-name AssumeRolePolicy \
     --policy-document file://orchestration-policy.json
   ```

### For DNS Account

1. **Create Route53 management role**:
   ```bash
   aws iam create-role \
     --role-name Route53ManagementRole \
     --assume-role-policy-document file://route53-trust.json
   ```

2. **Attach Route53 permissions**:
   ```bash
   aws iam put-role-policy \
     --role-name Route53ManagementRole \
     --policy-name Route53Policy \
     --policy-document file://route53-policy.json
   ```

### For Each Customer SES Account

1. **Create SES management role**:
   ```bash
   aws iam create-role \
     --role-name SESManagementRole \
     --assume-role-policy-document file://ses-trust.json
   ```

2. **Attach SES permissions**:
   ```bash
   aws iam put-role-policy \
     --role-name SESManagementRole \
     --policy-name SESPolicy \
     --policy-document file://ses-policy.json
   ```

---

## Configuration File Example

Update your `config.json` with role ARNs:

```json
{
  "aws_region": "us-east-1",
  "customerMappings": {
    "hts": {
      "customer_code": "hts",
      "customer_name": "HTS Prod",
      "ses_role_arn": "arn:aws:iam::111111111111:role/SESManagementRole",
      "region": "us-east-1",
      "deliverability": {
        "mail_from_domain": "bounce.htsprod.com",
        "configuration_set_name": "hts-production-emails",
        "dmarc_policy": "none",
        "dmarc_report_email": "dmarc-reports@htsprod.com",
        "sns_topic_arn": "arn:aws:sns:us-east-1:111111111111:hts-ses-events"
      }
    },
    "cds": {
      "customer_code": "cds",
      "customer_name": "CDS Global",
      "ses_role_arn": "arn:aws:iam::222222222222:role/SESManagementRole",
      "region": "us-east-1",
      "deliverability": {
        "mail_from_domain": "bounce.cdsglobal.com",
        "configuration_set_name": "cds-production-emails",
        "dmarc_policy": "none",
        "dmarc_report_email": "dmarc-reports@cdsglobal.com",
        "sns_topic_arn": "arn:aws:sns:us-east-1:222222222222:cds-ses-events"
      }
    }
  },
  "route53_config": {
    "zone_id": "Z1234567890ABC",
    "role_arn": "arn:aws:iam::999999999999:role/Route53ManagementRole"
  }
}
```

---

## Testing Role Assumptions

### Test DNS Role Assumption
```bash
aws sts assume-role \
  --role-arn arn:aws:iam::DNS-ACCOUNT-ID:role/Route53ManagementRole \
  --role-session-name test-session

# Then test Route53 access
aws route53 list-hosted-zones --profile assumed-role
```

### Test SES Role Assumption
```bash
aws sts assume-role \
  --role-arn arn:aws:iam::CUSTOMER-ACCOUNT-ID:role/SESManagementRole \
  --role-session-name test-session

# Then test SES access
aws sesv2 get-account --profile assumed-role
```

---

## Troubleshooting

### "Access Denied" when assuming role

**Possible causes**:
1. Trust relationship not configured correctly
2. Orchestration role doesn't have `sts:AssumeRole` permission
3. External ID mismatch
4. IP restriction blocking access

**Solution**: Check trust relationship and orchestration role policy

### "Access Denied" when modifying Route53

**Possible causes**:
1. Route53 role doesn't have required permissions
2. Hosted zone ID not in allowed resources
3. Wrong region specified

**Solution**: Verify Route53 role permissions and resource ARNs

### "Access Denied" when configuring SES

**Possible causes**:
1. SES role doesn't have required permissions
2. Resource ARN doesn't match
3. Service not available in region

**Solution**: Verify SES role permissions and region availability

---

## Monitoring and Auditing

### CloudTrail Events to Monitor

**Role Assumptions**:
- `AssumeRole` events from orchestration account
- Check `userIdentity.sessionContext.sessionIssuer.arn`

**Route53 Changes**:
- `ChangeResourceRecordSets` events
- Review DNS record modifications

**SES Configuration Changes**:
- `CreateConfigurationSet`
- `PutEmailIdentityMailFromAttributes`
- `CreateConfigurationSetEventDestination`

### CloudWatch Alarms

Set up alarms for:
- Failed role assumptions
- Unauthorized API calls
- SES reputation metrics
- Bounce/complaint rates

---

## Summary

### Three Account Types
1. **Orchestration** - Central management, assumes roles
2. **DNS** - Route53 hosted zones, DNS records
3. **Customer SES** - SES configuration per customer

### Key Permissions
- **Orchestration**: `sts:AssumeRole`
- **DNS**: `route53:ChangeResourceRecordSets`
- **SES**: `ses:*` for identity and configuration management

### Security
- Use external IDs for trust relationships
- Enable CloudTrail in all accounts
- Follow least privilege principle
- Regular audit of role assumptions

### Configuration
- All role ARNs in `config.json`
- One SES role per customer account
- Shared Route53 role for all DNS operations
