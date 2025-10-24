# DNS Code Alignment Analysis

## Existing DNS Infrastructure

### Current Implementation (`internal/route53/manager.go`)

The existing code has a well-structured Route53 DNS manager that:

1. **Manages DKIM and Domain Verification Records**
   - Creates 3 CNAME records for DKIM tokens
   - Creates TXT records for domain verification
   - Supports idempotent operations (checks existing records before creating)
   - Handles batch operations (up to 1000 changes per batch)

2. **Key Features**
   - Role assumption for cross-account DNS management
   - Retry logic with exponential backoff
   - Dry-run support
   - Structured logging with slog
   - Zone name lookup from Zone ID

3. **DNS Record Types Handled**
   - CNAME records for DKIM (`{token}._domainkey.{domain}`)
   - TXT records for verification (`_amazonses.{domain}`)

### Current CLI Integration (`main.go`)

The `configure-domain` action:
- Configures SES domain identity (DKIM + verification)
- Optionally configures Route53 DNS records
- Supports multi-customer operations
- Uses `Route53Config` from `config.json`

## New Deliverability Code (`internal/ses/deliverability.go`)

### What It Adds

1. **SPF Records** - TXT records for sender authorization
2. **DMARC Records** - TXT records for email authentication policy
3. **Custom MAIL FROM Domain** - MX and TXT records for bounce handling
4. **Configuration Sets** - SES tracking configuration
5. **Event Destinations** - SNS integration for bounce/complaint tracking

### DNS Record Types Needed

```go
// New record types not currently handled:
- MX records (for custom MAIL FROM domain)
- TXT records on main domain (for SPF)
- TXT records on _dmarc subdomain (for DMARC)
- TXT records on bounce subdomain (for SPF)
```

## Alignment Strategy

### ✅ What Aligns Well

1. **DNSManager Pattern**
   - Both use the same Route53 client pattern
   - Both support dry-run mode
   - Both use structured logging
   - Both handle idempotent operations

2. **Configuration Structure**
   - Both use `Route53Config` from config.json
   - Both support role assumption
   - Both use zone ID lookup

3. **Error Handling**
   - Both use retry logic with exponential backoff
   - Both wrap AWS errors with context

### 🔧 What Needs Integration

1. **DNSRecord Type Definition**
   - New code defines `DNSRecord` struct in `deliverability.go`
   - Should be moved to `internal/route53/manager.go` or `internal/types/types.go`
   - Should be used by both existing and new code

2. **Record Type Support**
   - Existing code only handles CNAME and TXT for DKIM/verification
   - Need to add MX record support
   - Need to add TXT records for different purposes (SPF, DMARC)

3. **Function Organization**
   - Existing: `ConfigureRecords()` for DKIM/verification
   - New: Multiple helper functions for different record types
   - Should consolidate into unified DNS management

## Recommended Integration Approach

### Phase 1: Extend Existing DNSManager

Add new methods to `internal/route53/manager.go`:

```go
// Add to DNSManager struct
func (dm *DNSManager) CreateMXRecord(ctx context.Context, zoneID, name, value string, ttl int64) error
func (dm *DNSManager) CreateTXTRecord(ctx context.Context, zoneID, name, value string, ttl int64) error
func (dm *DNSManager) ConfigureDeliverabilityRecords(ctx context.Context, config DeliverabilityDNSConfig) error
```

### Phase 2: Define Common Types

Add to `internal/types/types.go`:

```go
// DeliverabilityConfig holds deliverability settings per customer
type DeliverabilityConfig struct {
    MailFromDomain       string `json:"mail_from_domain"`
    ConfigurationSetName string `json:"configuration_set_name"`
    DMARCPolicy          string `json:"dmarc_policy"`
    DMARCReportEmail     string `json:"dmarc_report_email"`
    SNSTopicARN          string `json:"sns_topic_arn,omitempty"`
}

// Add to CustomerAccountInfo
type CustomerAccountInfo struct {
    // ... existing fields ...
    Deliverability *DeliverabilityConfig `json:"deliverability,omitempty"`
}
```

### Phase 3: Extend CLI Commands

Add new actions to `handleSESCommand()`:

```go
case "configure-deliverability":
    // Configure SPF, DMARC, MAIL FROM, Config Sets
case "show-deliverability-dns":
    // Show what DNS records are needed
case "verify-deliverability":
    // Check current deliverability setup
case "configure-deliverability-all":
    // Multi-customer deliverability setup
```

### Phase 4: Unified DNS Configuration

Create a single command that handles everything:

```bash
./ccoe-customer-contact-manager ses \
  --action configure-domain-complete \
  --customer-code hts \
  --configure-dns \
  --configure-deliverability
```

This would:
1. Configure SES domain identity (existing)
2. Create DKIM records (existing)
3. Create verification records (existing)
4. Create SPF records (new)
5. Create DMARC records (new)
6. Configure custom MAIL FROM (new)
7. Create MAIL FROM DNS records (new)
8. Create configuration set (new)
9. Assign config set to domain (new)

## Code Reuse Opportunities

### 1. Batch Change Application

The existing `applyChanges()` method can be reused:

```go
// Existing method handles batching and retries
func (dm *DNSManager) applyChanges(ctx context.Context, zoneID string, changes []types.Change) error
```

### 2. Existing Record Check

The existing `getExistingRecords()` pattern can be extended:

```go
// Extend to check for MX, SPF, DMARC records
func (dm *DNSManager) getExistingRecords(ctx context.Context, zoneID, zoneName string) (map[string]string, error)
```

### 3. Idempotent Operations

The existing pattern of checking before creating can be reused:

```go
// Pattern from existing code
if existingValue, exists := existingRecords[recordName]; exists && existingValue == recordValue {
    // Skip - already exists
    continue
}
```

## Implementation Plan

### Step 1: Move DNSRecord Type (5 min)
```go
// Add to internal/types/types.go
type DNSRecord struct {
    Type  string
    Name  string
    Value string
    TTL   int
}
```

### Step 2: Extend DNSManager (30 min)
- Add MX record support
- Add generic TXT record support
- Add deliverability-specific methods

### Step 3: Update deliverability.go (15 min)
- Remove DNSRecord definition
- Import from types package
- Use DNSManager methods instead of direct SES calls

### Step 4: Add CLI Commands (30 min)
- Add new action handlers
- Integrate with existing configure-domain flow
- Add multi-customer support

### Step 5: Update Config Schema (10 min)
- Add deliverability section to config.json
- Update validation
- Update documentation

## Benefits of This Approach

1. **Code Reuse**: Leverages existing DNS management infrastructure
2. **Consistency**: Same patterns for all DNS operations
3. **Maintainability**: Single source of truth for DNS operations
4. **Testing**: Can test DNS operations in isolation
5. **Idempotency**: All operations are safe to re-run
6. **Dry-Run**: Preview all changes before applying
7. **Multi-Customer**: Works with existing multi-customer patterns

## Example Usage After Integration

```bash
# Show what DNS records are needed
./ccoe-customer-contact-manager ses \
  --action show-deliverability-dns \
  --customer-code hts

# Configure everything (DKIM + Deliverability)
./ccoe-customer-contact-manager ses \
  --action configure-domain-complete \
  --customer-code hts \
  --configure-dns \
  --dry-run

# Apply changes
./ccoe-customer-contact-manager ses \
  --action configure-domain-complete \
  --customer-code hts \
  --configure-dns

# Verify setup
./ccoe-customer-contact-manager ses \
  --action verify-deliverability \
  --customer-code hts

# Multi-customer setup
./ccoe-customer-contact-manager ses \
  --action configure-deliverability-all \
  --config-file config.json \
  --configure-dns
```

## Next Steps

1. Review this alignment analysis
2. Decide on integration approach
3. Implement Phase 1 (extend DNSManager)
4. Test with single customer
5. Roll out to all customers
6. Update documentation

## Files to Modify

1. `internal/types/types.go` - Add DNSRecord and DeliverabilityConfig types
2. `internal/route53/manager.go` - Add MX and deliverability methods
3. `internal/ses/deliverability.go` - Update to use DNSManager
4. `main.go` - Add new CLI commands
5. `config.json` - Add deliverability configuration
6. `README.md` - Update documentation

## Estimated Time

- Phase 1-2: 1 hour (extend infrastructure)
- Phase 3-4: 1 hour (CLI integration)
- Phase 5: 30 min (config updates)
- Testing: 1 hour
- Documentation: 30 min

**Total: ~4 hours for complete integration**
