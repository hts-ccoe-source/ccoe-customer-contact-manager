# Design Document

## Overview

This design extends the existing golang CLI application to support SES domain validation and Route53 DNS record management. The solution introduces two new command categories: `ses configure-domain` for setting up SES resources in customer accounts, and `route53` for managing DNS records in the centralized DNS account.

The design follows the existing CLI patterns in the application, using the AWS SDK v2 for Go, supporting both dry-run and actual execution modes, implementing full idempotency, and providing structured logging with slog.

## Architecture

### High-Level Flow

**Integrated Workflow (Default - Recommended):**

```
┌─────────────────────────────────────────────────────────────────┐
│          ses configure-domain --configure-dns=true              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  Load Config    │
                    │  (config.json)  │
                    └─────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  For Each       │
                    │  Customer/Org   │
                    └─────────────────┘
                              │
                ┌─────────────┴─────────────┐
                │                           │
                ▼                           ▼
    ┌───────────────────────┐   ┌──────────────────────┐
    │  Assume Customer      │   │  Create SESV2        │
    │  Account Role         │   │  Resources           │
    └───────────────────────┘   │  - Email Identity    │
                │               │  - Domain Identity   │
                │               │  - DKIM Config       │
                │               └──────────────────────┘
                │                           │
                │                           ▼
                │               ┌──────────────────────┐
                │               │  Capture Tokens      │
                │               │  (In Memory)         │
                │               │  - Verification      │
                │               │  - DKIM (3)          │
                │               └──────────────────────┘
                │                           │
                └───────────────┬───────────┘
                                │
                                ▼
                    ┌──────────────────────┐
                    │  Assume DNS Account  │
                    │  Role                │
                    └──────────────────────┘
                                │
                                ▼
                    ┌──────────────────────┐
                    │  Create Route53      │
                    │  DNS Records         │
                    │  - DKIM CNAMEs (3)   │
                    │  - Verification TXT  │
                    └──────────────────────┘
                                │
                                ▼
                    ┌──────────────────────┐
                    │  Output Summary      │
                    │  - SES Resources     │
                    │  - DNS Records       │
                    │  - Success/Failures  │
                    └──────────────────────┘
```

**Standalone Workflows (Optional):**

```
SES Only:                          Route53 Only:
ses configure-domain               route53 configure
--configure-dns=false              (reads tokens from config)
```

### Component Interaction

**Integrated Workflow:**

```
┌──────────────────────────────────────────────────────────────┐
│                        main.go                               │
│                 (CLI Entry Point)                            │
│                                                              │
│ handleSESConfigureDomainCommand()                            │
│    ├─ Parse flags (--configure-dns, --dns-role-arn)          │
│    ├─ Load config.json                                       │
│    └─ Orchestrate workflow                                   │
└───────────────────┬──────────────────────────────────────────┘
                    │
                    │ For each customer/org
                    │
        ┌───────────┴───────────┐
        │                       │
        ▼                       ▼
┌───────────────────┐   ┌──────────────────────┐
│  internal/aws/    │   │  internal/ses/       │
│  utils.go         │   │  domain.go           │
│                   │   │                      │
│  AssumeRole()     │──>│  DomainManager       │
│  (Customer Acct)  │   │  - ConfigureDomain() │
└───────────────────┘   │  - CreateEmail()     │
                        │  - CreateDomain()    │
                        │  - GetTokens()       │
                        └──────────┬───────────┘
                                   │
                                   │ Returns DomainTokens
                                   │ (in memory)
                                   │
                                   ▼
                        ┌──────────────────────┐
                        │  AWS SESV2 API       │
                        │  (Customer Account)  │
                        └──────────────────────┘
                                   │
                                   │ Tokens captured
                                   │
                                   ▼
                        ┌──────────────────────┐
                        │  main.go             │
                        │  (Orchestrator)      │
                        │  - Collect tokens    │
                        │  - Build DNS config  │
                        └──────────┬───────────┘
                                   │
                                   │ Pass tokens
                                   │
        ┌──────────────────────────┴──────────┐
        │                                     │
        ▼                                     ▼
┌───────────────────┐           ┌──────────────────────┐
│  internal/aws/    │           │  internal/route53/   │
│  utils.go         │           │  manager.go          │
│                   │           │                      │
│  AssumeRole()     │──────────>│  DNSManager          │
│  (DNS Account)    │           │  - ConfigureRecords()│
└───────────────────┘           │  - CreateDKIM()      │
                                │  - CreateVerify()    │
                                └──────────┬───────────┘
                                           │
                                           ▼
                                ┌──────────────────────┐
                                │  AWS Route53 API     │
                                │  (DNS Account)       │
                                └──────────────────────┘
```

## Components and Interfaces

### 1. Command Handlers (main.go)

Add command handlers with integrated workflow:

```go
// handleSESConfigureDomainCommand handles the ses configure-domain subcommand
// This command can optionally configure Route53 DNS records immediately after SES setup
func handleSESConfigureDomainCommand() {
    fs := flag.NewFlagSet("ses configure-domain", flag.ExitOnError)
    
    configFile := fs.String("config", "./config.json", "Path to configuration file")
    customerCode := fs.String("customer", "", "Customer code (process single customer)")
    dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
    profile := fs.String("profile", "", "AWS profile to use")
    region := fs.String("region", "us-east-1", "AWS region")
    configureDNS := fs.Bool("configure-dns", true, "Automatically configure Route53 DNS records")
    dnsRoleArn := fs.String("dns-role-arn", "", "IAM role ARN to assume in DNS account (required if configure-dns=true)")
    
    fs.Parse(os.Args[2:])
    
    // Implementation:
    // 1. Call internal/ses/domain.go to create SES resources
    // 2. Capture tokens in memory
    // 3. If configure-dns=true, call internal/route53/manager.go with tokens
}

// handleRoute53Command handles the route53 subcommand (standalone mode)
// This is for cases where SES is already configured and only DNS needs updating
func handleRoute53Command() {
    fs := flag.NewFlagSet("route53", flag.ExitOnError)
    
    action := fs.String("action", "", "Action to perform (configure)")
    configFile := fs.String("config", "./config.json", "Path to configuration file with tokens")
    roleArn := fs.String("role-arn", "", "IAM role ARN to assume in DNS account")
    dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
    profile := fs.String("profile", "", "AWS profile to use")
    region := fs.String("region", "us-east-1", "AWS region")
    
    fs.Parse(os.Args[2:])
    
    // Implementation calls internal/route53/manager.go
    // Reads tokens from config file (for standalone use)
}
```

### 2. SES Domain Manager (internal/ses/domain.go)

New package for managing SES domain validation resources:

```go
package ses

import (
    "context"
    "fmt"
    "log/slog"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/sesv2"
    "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// DomainManager manages SES domain validation resources
type DomainManager struct {
    client *sesv2.Client
    logger *slog.Logger
    dryRun bool
}

// DomainConfig holds configuration for domain setup
type DomainConfig struct {
    EmailAddress string // e.g., "ccoe@hearst.com"
    DomainName   string // e.g., "ccoe.hearst.com"
}

// DomainTokens holds the tokens needed for DNS configuration
type DomainTokens struct {
    VerificationToken string
    DKIMTokens        []string // Always 3 tokens
}

// NewDomainManager creates a new domain manager
func NewDomainManager(cfg aws.Config, dryRun bool, logger *slog.Logger) *DomainManager

// ConfigureDomain sets up SES email and domain identities with DKIM
func (dm *DomainManager) ConfigureDomain(ctx context.Context, config DomainConfig) (*DomainTokens, error)

// createEmailIdentity creates an SESV2 email identity
func (dm *DomainManager) createEmailIdentity(ctx context.Context, emailAddress string) error

// createDomainIdentity creates an SESV2 domain identity with DKIM enabled
func (dm *DomainManager) createDomainIdentity(ctx context.Context, domainName string) (*DomainTokens, error)

// getDomainTokens retrieves verification and DKIM tokens for a domain
func (dm *DomainManager) getDomainTokens(ctx context.Context, domainName string) (*DomainTokens, error)
```

### 3. Route53 DNS Manager (internal/route53/manager.go)

New package for managing Route53 DNS records:

```go
package route53

import (
    "context"
    "fmt"
    "log/slog"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/route53"
    "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// DNSManager manages Route53 DNS records for SES validation
type DNSManager struct {
    client *route53.Client
    logger *slog.Logger
    dryRun bool
}

// DNSConfig holds Route53 configuration
type DNSConfig struct {
    ZoneID   string
    ZoneName string // Looked up from ZoneID via GetHostedZone API
}

// OrganizationDNS holds DNS configuration for one organization
type OrganizationDNS struct {
    Name              string
    DKIMTokens        []string
    VerificationToken string
}

// NewDNSManager creates a new DNS manager
func NewDNSManager(cfg aws.Config, dryRun bool, logger *slog.Logger) *DNSManager

// ConfigureRecords creates DNS records for SES validation
// Looks up zone name from zone ID before creating records
func (dm *DNSManager) ConfigureRecords(ctx context.Context, dnsConfig DNSConfig, orgs []OrganizationDNS) error

// getHostedZoneName retrieves the zone name from a zone ID
func (dm *DNSManager) getHostedZoneName(ctx context.Context, zoneID string) (string, error)

// createDKIMRecords creates CNAME records for DKIM tokens
func (dm *DNSManager) createDKIMRecords(ctx context.Context, zoneName string, dkimTokens []string) []types.Change

// createVerificationRecords creates TXT records for domain verification (one per organization)
// Multiple TXT records with the same name are supported by Route53
func (dm *DNSManager) createVerificationRecords(ctx context.Context, zoneName string, orgs []OrganizationDNS) []types.Change

// applyChanges applies DNS changes in batches
func (dm *DNSManager) applyChanges(ctx context.Context, zoneID string, changes []types.Change) error

// validateOrganization validates organization DNS configuration
func (dm *DNSManager) validateOrganization(org OrganizationDNS) error
```

### 4. Configuration Extension (internal/config/types.go)

Extend existing configuration structures in config.json:

```go
// Extend existing Config struct with Route53 configuration
type Config struct {
    AWSRegion         string                     `json:"aws_region"`
    LogLevel          string                     `json:"log_level"`
    S3Config          S3Config                   `json:"s3_config"`
    CustomerMappings  map[string]CustomerMapping `json:"customer_mappings"`
    EmailConfig       EmailConfig                `json:"email_config"`
    ContactConfig     ContactConfig              `json:"contact_config"`
    Route53Config     *Route53Config             `json:"route53_config,omitempty"` // NEW
}

// Route53Config holds Route53 zone information for SES domain validation
type Route53Config struct {
    ZoneID  string `json:"zone_id"`  // Hosted zone ID (zone name will be looked up from this)
    RoleArn string `json:"role_arn"` // IAM role to assume in DNS account
}

// Extend CustomerMapping to include SES domain validation tokens (optional)
type CustomerMapping struct {
    CustomerCode      string   `json:"customer_code"`
    Environment       string   `json:"environment"`
    CustomerName      string   `json:"customer_name"`
    Region            string   `json:"region"`
    SESRoleArn        string   `json:"ses_role_arn"`
    SQSQueueArn       string   `json:"sqs_queue_arn"`
    DKIMTokens        []string `json:"dkim_tokens,omitempty"`        // NEW (optional)
    VerificationToken string   `json:"verification_token,omitempty"` // NEW (optional)
}
```

## Data Models

### Configuration File Format (config.json)

**Extended Configuration (Integrated Workflow):**
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
      "ses_role_arn": "arn:aws:iam::869445953789:role/hts-ccoe-customer-contact-manager",
      "sqs_queue_arn": "arn:aws:sqs:us-east-1:730335533660:hts-prod-ccoe-customer-contact-manager-htsnonprod"
    }
  },
  "email_config": {
    "sender_address": "ccoe@hearst.com",
    "meeting_organizer": "ccoe@hearst.com",
    "portal_base_url": "https://change-management.ccoe.hearst.com"
  },
  "route53_config": {
    "zone_id": "Z1234567890ABC",
    "role_arn": "arn:aws:iam::123456789012:role/DNSManagementRole"
  }
}
```

**With Tokens (Standalone Route53 Workflow - Optional):**
```json
{
  "customer_mappings": {
    "htsnonprod": {
      "customer_code": "htsnonprod",
      "ses_role_arn": "arn:aws:iam::869445953789:role/hts-ccoe-customer-contact-manager",
      "dkim_tokens": ["token1", "token2", "token3"],
      "verification_token": "abc123xyz"
    }
  },
  "route53_config": {
    "zone_id": "Z1234567890ABC",
    "role_arn": "arn:aws:iam::123456789012:role/DNSManagementRole"
  }
}
```

Note: 
- Tokens in customer_mappings are optional. The integrated workflow captures them in memory and passes them directly to Route53 configuration.
- Zone name is looked up from zone_id using Route53 GetHostedZone API, so it doesn't need to be in the config.

### DNS Record Formats

**DKIM CNAME Records (3 per domain):**
```
{token1}._domainkey.ccoe.hearst.com -> {token1}.dkim.amazonses.com
{token2}._domainkey.ccoe.hearst.com -> {token2}.dkim.amazonses.com
{token3}._domainkey.ccoe.hearst.com -> {token3}.dkim.amazonses.com
```

**Verification TXT Records (one per customer account):**
```
_amazonses.ccoe.hearst.com -> "{verification-token-customer1}"
_amazonses.ccoe.hearst.com -> "{verification-token-customer2}"
_amazonses.ccoe.hearst.com -> "{verification-token-customer3}"
... (one for each of the 30 customer accounts)
```

Note: Multiple TXT records with the same name are supported by Route53. Each customer account gets its own verification token, and SES will check if any of the tokens in DNS matches what it expects.

## Error Handling

### Retry Strategy

Implement exponential backoff with jitter for all AWS API calls:

```go
type RetryConfig struct {
    MaxAttempts     int           // 5 attempts
    InitialDelay    time.Duration // 1 second
    MaxDelay        time.Duration // 30 seconds
    BackoffFactor   float64       // 2.0
    JitterFraction  float64       // 0.1
}

func retryWithBackoff(ctx context.Context, operation func() error, config RetryConfig) error {
    var lastErr error
    delay := config.InitialDelay
    
    for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
        if err := operation(); err == nil {
            return nil
        } else {
            lastErr = err
            
            // Check if error is retryable
            if !isRetryableError(err) {
                return err
            }
            
            // Apply exponential backoff with jitter
            jitter := time.Duration(float64(delay) * config.JitterFraction * (rand.Float64()*2 - 1))
            sleepTime := delay + jitter
            
            if sleepTime > config.MaxDelay {
                sleepTime = config.MaxDelay
            }
            
            time.Sleep(sleepTime)
            delay = time.Duration(float64(delay) * config.BackoffFactor)
        }
    }
    
    return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

func isRetryableError(err error) bool {
    // Check for throttling, rate limit, and transient errors
    // AWS SDK v2 provides error types for this
}
```

### Error Categories

1. **Configuration Errors**: Invalid JSON, missing required fields
   - Action: Log error and exit immediately
   - Exit code: 1

2. **Authentication Errors**: Failed role assumption, invalid credentials
   - Action: Log error with details and exit
   - Exit code: 2

3. **Validation Errors**: Invalid organization configuration (missing tokens, wrong count)
   - Action: Log warning, skip organization, continue processing
   - Track in summary

4. **API Errors**: AWS service errors, rate limiting
   - Action: Retry with exponential backoff
   - If all retries fail: log error, continue to next operation
   - Track in summary

5. **Resource Already Exists**: Idempotent operations encounter existing resources
   - Action: Update resource if possible, log info message
   - Continue processing

### Error Logging

Use structured logging with slog:

```go
logger.Error("failed to create email identity",
    "email", emailAddress,
    "customer", customerCode,
    "error", err,
    "attempt", attempt)

logger.Warn("skipping organization due to invalid configuration",
    "organization", orgName,
    "reason", "missing DKIM tokens",
    "expected", 3,
    "actual", len(tokens))
```

## Testing Strategy

### Unit Tests

1. **Configuration Loading**
   - Test valid JSON parsing
   - Test invalid JSON handling
   - Test missing required fields
   - Test default values

2. **Domain Manager**
   - Test email identity creation (mocked SESV2 client)
   - Test domain identity creation with DKIM
   - Test token retrieval
   - Test dry-run mode (no actual API calls)
   - Test idempotency (resource already exists)

3. **DNS Manager**
   - Test DKIM record generation
   - Test verification record generation
   - Test batch splitting (>1000 records)
   - Test organization validation
   - Test dry-run mode

4. **Retry Logic**
   - Test exponential backoff calculation
   - Test jitter application
   - Test max attempts limit
   - Test retryable vs non-retryable errors

### Integration Tests

1. **End-to-End SES Configuration**
   - Create test customer account
   - Run `ses configure-domain` command
   - Verify email identity created
   - Verify domain identity created
   - Verify DKIM enabled
   - Verify tokens retrieved

2. **End-to-End Route53 Configuration**
   - Create test hosted zone
   - Run `route53 configure` command
   - Verify DKIM CNAME records created
   - Verify verification TXT record created
   - Verify TTL values correct

3. **Multi-Organization Processing**
   - Configure multiple organizations
   - Verify all processed
   - Verify summary output correct
   - Test with one invalid organization (should continue)

4. **Idempotency**
   - Run commands twice
   - Verify no errors on second run
   - Verify resources updated, not duplicated

### Manual Testing Checklist

- [ ] Test with valid configuration file
- [ ] Test with missing configuration file
- [ ] Test with invalid JSON
- [ ] Test with single customer
- [ ] Test with all customers
- [ ] Test dry-run mode (no changes made)
- [ ] Test actual execution mode
- [ ] Test with existing SES resources
- [ ] Test with existing Route53 records
- [ ] Test role assumption in DNS account
- [ ] Test error handling (invalid credentials)
- [ ] Test error handling (rate limiting)
- [ ] Verify structured logging output
- [ ] Verify exit codes

## Implementation Phases

### Phase 1: Core SES Domain Configuration
- Implement `internal/ses/domain.go`
- Add `ses configure-domain` command handler
- Implement email identity creation
- Implement domain identity creation with DKIM
- Implement token retrieval
- Add unit tests

### Phase 2: Route53 DNS Management
- Implement `internal/route53/manager.go`
- Add `route53` command handler
- Implement DKIM record creation
- Implement verification record creation
- Implement batch processing
- Add unit tests

### Phase 3: Configuration and Integration
- Extend configuration structures
- Implement configuration loading
- Implement organization validation
- Add retry logic with exponential backoff
- Integrate with existing credential management

### Phase 4: Error Handling and Logging
- Implement comprehensive error handling
- Add structured logging throughout
- Implement dry-run mode
- Implement summary output
- Add integration tests

### Phase 5: Documentation and Polish
- Update README with new commands
- Add usage examples
- Add troubleshooting guide
- Final testing and validation

## Security Considerations

1. **Credential Management**
   - Use AWS SDK v2 credential chain
   - Support role assumption for cross-account access
   - Never log credentials or tokens
   - Use temporary credentials where possible

2. **IAM Permissions Required**

   **For SES Operations (Customer Accounts):**
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

   **For Route53 Operations (DNS Account):**
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
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

3. **Configuration File Security**
   - Store configuration files in version control
   - Tokens are public information (used in DNS)
   - Protect AWS credentials and role ARNs
   - Use least-privilege IAM roles

## Performance Considerations

1. **Concurrency**
   - Process organizations sequentially (simpler error handling)
   - Future enhancement: parallel processing with worker pool
   - Route53 batch operations (up to 1000 changes per request)

2. **Rate Limiting**
   - SESV2 API: 1 request per second (default)
   - Route53 API: 5 requests per second (default)
   - Implement exponential backoff for rate limit errors
   - Add configurable rate limiting if needed

3. **Optimization Opportunities**
   - Cache AWS clients (reuse connections)
   - Batch Route53 changes efficiently
   - Minimize API calls (check existence before create)

## Monitoring and Observability

1. **Structured Logging**
   - Use slog with JSON output for production
   - Include context: customer code, organization name, operation
   - Log levels: DEBUG, INFO, WARN, ERROR

2. **Metrics to Track**
   - Number of organizations processed
   - Number of successful operations
   - Number of failed operations
   - API call latency
   - Retry attempts

3. **Output Summary**
   ```
   SES Domain Configuration Summary:
   - Total organizations: 5
   - Successful: 4
   - Failed: 1
   - Email identities created: 4
   - Domain identities created: 4
   - DKIM configurations: 4
   
   Route53 Configuration Summary:
   - Total organizations: 5
   - Successful: 5
   - Failed: 0
   - DKIM records created: 15 (3 per org)
   - Verification records created: 5
   ```

## Dependencies

### New Go Packages Required

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/sesv2"
    "github.com/aws/aws-sdk-go-v2/service/route53"
)
```

### Existing Dependencies to Leverage

- `github.com/aws/aws-sdk-go-v2/aws` - Core AWS SDK
- `github.com/aws/aws-sdk-go-v2/config` - Configuration loading
- `github.com/aws/aws-sdk-go-v2/service/sts` - Role assumption
- `log/slog` - Structured logging
- Existing `internal/aws/utils.go` - AssumeRole, retry logic
- Existing `internal/config` - Configuration patterns

## Migration Path

### From Terraform to CLI

**Integrated Workflow (Recommended):**

1. **One-Step Setup**
   ```bash
   # Configure SES and DNS for all customers in one command
   ./ccoe-customer-contact-manager ses configure-domain \
     --config ./config.json \
     --configure-dns=true \
     --dns-role-arn arn:aws:iam::123456789012:role/DNSManagementRole
   ```

2. **Validation**
   - Check SES console for verified identities
   - Check Route53 console for DNS records
   - Wait for DNS propagation (5-10 minutes)
   - Test email sending

**Standalone Workflow (If Needed):**

1. **SES Setup Only**
   ```bash
   # Configure SES without DNS (tokens output to console)
   ./ccoe-customer-contact-manager ses configure-domain \
     --config ./config.json \
     --configure-dns=false
   ```

2. **DNS Configuration Later**
   ```bash
   # Configure DNS using tokens from config file
   ./ccoe-customer-contact-manager route53 configure \
     --config ./config.json \
     --role-arn arn:aws:iam::123456789012:role/DNSManagementRole
   ```

### Ongoing Operations

- **Add new customers**: Run integrated command for new customer
- **Update existing**: Re-run integrated command (idempotent)
- **Single customer**: Use `--customer CUST1` flag
- **Dry-run testing**: Use `--dry-run` flag to preview changes
- **Remove customers**: Manual cleanup (future enhancement)
