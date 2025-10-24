# CCOE Customer Contact Manager - Quick Reference

**Last Updated:** October 22, 2025

## System Overview

**Purpose:** Multi-customer change management and notification system  
**Customers:** 25+ organizations  
**Domain:** https://change-management.ccoe.hearst.com  
**AWS Account:** 730335533660 (hts-aws-com-std-app-prod)

## Key Components

| Component | Technology | Location | Logs |
|-----------|-----------|----------|------|
| SAML Auth | Lambda@Edge | `lambda/saml_auth/` | Edge locations |
| Upload API | Node.js Lambda | `lambda/upload_lambda/` | `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-api` |
| Backend | Go Lambda | `main.go`, `internal/` | `/aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-backend` |
| Web Portal | HTML/CSS/JS | `html/` | CloudFront logs |
| Governance | ECS Fargate | Same Go binary | `/ecs/governance-cluster` |

## Quick Commands

### Build

```bash
# Build all Lambda packages
make package-all-lambdas

# Build Go Lambda only
make package-golang-lambda

# Build Node.js Upload Lambda
make package-upload-lambda

# Build SAML Auth Lambda
make package-saml-lambda

# Build for local testing
make build-local
```

### Deploy

```bash
# Deploy website to S3
./deploy-website.sh

# Deploy Lambda backend
./deploy-lambda-backend.sh

# Invalidate CloudFront cache
export CLOUDFRONT_DISTRIBUTION_ID=E3DIDLE5N99NVJ
aws cloudfront create-invalidation \
  --distribution-id $CLOUDFRONT_DISTRIBUTION_ID \
  --paths "/*.html" "/assets/*"
```

### Test

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Test internal packages only
make test-internal
```

## Configuration Files

| File | Purpose | Location |
|------|---------|----------|
| `config.json` | Main configuration, customer mappings | Project root |
| `SESConfig.json` | SES topic definitions, role mappings | Project root |
| `SubscriptionConfig.json` | Bulk subscription mappings | Project root |

## S3 Bucket Structure

```
s3://4cm-prod-ccoe-change-management-metadata/
├── customers/{code}/     # Transient triggers (auto-deleted)
├── archive/              # Permanent storage
├── drafts/               # Draft changes
├── deleted/              # Deleted items
└── *.html, assets/       # Website files
```

## API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/upload` | Submit change/announcement |
| GET | `/changes` | List all changes |
| GET | `/changes/{id}` | Get specific change |
| PUT | `/changes/{id}` | Update change |
| DELETE | `/changes/{id}` | Delete change |
| POST | `/changes/{id}/approve` | Approve change |
| POST | `/changes/{id}/complete` | Complete change |
| POST | `/changes/{id}/cancel` | Cancel change |
| POST | `/changes/search` | Search changes |
| GET | `/announcements` | List announcements |
| GET | `/announcements/{id}` | Get announcement |

## Common Tasks

### Add New Customer

1. Update `config.json` with customer mapping
2. Add SES role ARN for customer
3. Deploy customer SES resources
4. Test email delivery
5. Update documentation

### Update SES Topics

1. Edit `SESConfig.json`
2. Run `manage-topic-all` action
3. Verify topics in SES console
4. Test subscriptions

### View Logs

```bash
# Backend Lambda logs
aws logs tail /aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-backend --follow

# Upload Lambda logs
aws logs tail /aws/lambda/hts-ccoe-prod-ccoe-customer-contact-manager-api --follow

# CloudFront logs (check S3 bucket)
aws s3 ls s3://cloudfront-logs-bucket/
```

### Troubleshoot Email Delivery

1. Check SES domain verification status
2. Verify IAM role assumption works
3. Check CloudWatch logs for errors
4. Verify contact exists in SES list
5. Check topic subscriptions

## Status Transitions

### Changes

```
draft → submitted → approved → completed
                            → cancelled
```

### Announcements

```
draft → submitted → approved → completed
                            → cancelled
```

## Customer Codes

| Code | Organization | Account ID |
|------|-------------|------------|
| hts | HTS Prod | 748906912469 |
| htsnonprod | HTS NonProd | 869445953789 |
| cds | CDS Global | 292011262127 |
| fdbus | FDBUS | 268851382408 |
| ... | (25+ total) | ... |

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `S3_BUCKET_NAME` | Metadata bucket | `4cm-prod-ccoe-change-management-metadata` |
| `CONFIG_PATH` | Config directory | `./` |
| `AWS_REGION` | AWS region | `us-east-1` |
| `BACKEND_ROLE_ARN` | Backend Lambda role | (from environment) |

## Key Patterns

### Transient Trigger Pattern

1. Upload creates trigger in `customers/{code}/`
2. S3 event → SQS → Backend Lambda
3. Backend checks trigger exists (idempotency)
4. Backend loads from `archive/` (authoritative)
5. Backend processes and sends emails
6. Backend deletes trigger (cleanup)

### Multi-Customer Distribution

1. Single change affects multiple customers
2. Upload creates trigger for each customer
3. Each trigger generates independent S3 event
4. Backend processes each customer separately
5. Each customer's SES sends emails

### Event Loop Prevention

1. Backend checks `userIdentity` in S3 event
2. If event from backend role, discard
3. Only process events from frontend/users

## Security

### Authentication
- SAML SSO via AWS Identity Center
- Lambda@Edge validates sessions
- Session timeout: 1 hour

### Authorization
- Domain-based: Must be @hearst.com
- Role-based topic subscriptions
- Customer-specific SES roles

### Data Protection
- S3 versioning enabled
- Encryption at rest (S3 default)
- Encryption in transit (TLS 1.2+)

## Monitoring

### Key Metrics
- Lambda invocations and errors
- S3 event processing rate
- SES email delivery rate
- CloudFront cache hit rate

### Alarms
- Lambda error rate > 5%
- SQS queue depth > 100
- SES bounce rate > 5%

## Support Contacts

- **Platform Team:** CCOE Platform Team
- **CloudWatch Logs:** See component table above
- **Documentation:** `docs/` and `summaries/`

## Useful Links

- **CloudFront Distribution:** https://console.aws.amazon.com/cloudfront/v3/home#/distributions/E3DIDLE5N99NVJ
- **S3 Bucket:** https://s3.console.aws.amazon.com/s3/buckets/4cm-prod-ccoe-change-management-metadata
- **Lambda Functions:** https://console.aws.amazon.com/lambda/home?region=us-east-1
- **SES Console:** https://console.aws.amazon.com/ses/home?region=us-east-1

---

**For detailed information, see:**
- [Project Status Summary](./PROJECT_STATUS_SUMMARY.md)
- [Architecture Diagrams](./DIAGRAMS_INDEX.md)
- [Full Documentation](../docs/)
