# Design Document

## Overview

This design enhances the CLI to retrieve AWS Identity Center user and group membership data in-memory and pass it directly to the `import-aws-contact-all` command. The enhancement eliminates the dependency on pre-generated JSON files while maintaining backward compatibility with file-based workflows.

The key architectural change is to support concurrent per-customer processing where each customer's workflow consists of:
1. Assume Identity Center role
2. Discover Identity Center instance ID
3. Retrieve user and group membership data
4. Assume SES role
5. Import contacts to SES

## Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────┐
│ import-aws-contact-all Command                              │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Load Configuration & Determine Customers                    │
│ - Read config.json                                          │
│ - Get identity_center_role_arn (CLI flag or config)        │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Concurrent Customer Processing (max-concurrency workers)   │
└─────────────────────────────────────────────────────────────┘
                          │
          ┌───────────────┴───────────────┐
          │                               │
          ▼                               ▼
┌──────────────────────┐        ┌──────────────────────┐
│ Customer A           │        │ Customer B           │
│                      │        │                      │
│ 1. Assume IC Role    │        │ 1. Assume IC Role    │
│ 2. Discover IC ID    │        │ 2. Discover IC ID    │
│ 3. Retrieve Users    │        │ 3. Retrieve Users    │
│ 4. Retrieve Groups   │        │ 4. Retrieve Groups   │
│ 5. Assume SES Role   │        │ 5. Assume SES Role   │
│ 6. Import Contacts   │        │ 6. Import Contacts   │
└──────────────────────┘        └──────────────────────┘
          │                               │
          └───────────────┬───────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Aggregate Results & Report Summary                          │
└─────────────────────────────────────────────────────────────┘
```

### Configuration Changes

Add optional fields to customer mapping in `config.json`:

```json
{
  "customer_mappings": {
    "htsnonprod": {
      "customer_code": "htsnonprod",
      "ses_role_arn": "arn:aws:iam::869445953789:role/...",
      "identity_center_role_arn": "arn:aws:iam::978660766591:role/..."
    }
  }
}
```


## Components and Interfaces

### 1. Configuration Structure Updates

**File:** `internal/types/types.go`

Add new field to `CustomerInfo` struct:

```go
type CustomerInfo struct {
    CustomerCode          string `json:"customer_code"`
    Environment           string `json:"environment"`
    CustomerName          string `json:"customer_name"`
    Region                string `json:"region"`
    SESRoleArn            string `json:"ses_role_arn"`
    SQSQueueArn           string `json:"sqs_queue_arn"`
    IdentityCenterRoleArn string `json:"identity_center_role_arn,omitempty"` // NEW
}
```

### 2. Identity Center Data Structures

**File:** `internal/aws/identity_center.go`

Define return types for Identity Center operations:

```go
// IdentityCenterData holds user and group membership information
type IdentityCenterData struct {
    Users       []types.IdentityCenterUser
    Memberships []types.IdentityCenterGroupMembership
    InstanceID  string
}
```

### 3. Enhanced Identity Center Functions

**File:** `internal/aws/identity_center.go`

Modify existing functions to return data instead of only writing files:

```go
// RetrieveIdentityCenterData retrieves users and group memberships in-memory
func RetrieveIdentityCenterData(
    roleArn string,
    maxConcurrency int,
    requestsPerSecond int,
) (*IdentityCenterData, error)

// DiscoverIdentityCenterInstanceID discovers the Identity Center instance ID
func DiscoverIdentityCenterInstanceID(cfg aws.Config) (string, error)
```


### 4. Enhanced Import Function

**File:** `internal/ses/operations.go`

Modify `ImportAllAWSContacts` to accept optional in-memory data:

```go
// ImportAllAWSContacts imports contacts with optional in-memory Identity Center data
func ImportAllAWSContacts(
    sesClient *sesv2.Client,
    identityCenterData *aws.IdentityCenterData, // NEW: optional in-memory data
    dryRun bool,
    requestsPerSecond int,
) error
```

When `identityCenterData` is provided, use it directly. Otherwise, fall back to loading from files.

### 5. Main Command Handler

**File:** `main.go`

Create new handler for concurrent customer processing:

```go
// handleImportAWSContactAllEnhanced processes multiple customers concurrently
func handleImportAWSContactAllEnhanced(
    cfg *types.Config,
    identityCenterRoleArn *string, // CLI flag override
    maxConcurrency int,
    requestsPerSecond int,
    dryRun bool,
) error
```

## Data Models

### Identity Center Instance Discovery

Use AWS SSO Admin API to list instances:

```go
// ListInstances returns all Identity Center instances in the account
ssoAdminClient.ListInstances(ctx, &ssoadmin.ListInstancesInput{})
```

Expected response contains `InstanceArn` which includes the instance ID (format: `d-xxxxxxxxxx`).


### Customer Processing Result

```go
type CustomerImportResult struct {
    CustomerCode string
    Success      bool
    Error        error
    UsersProcessed int
    ContactsAdded  int
    ContactsUpdated int
    ContactsSkipped int
}
```

## Error Handling

### Per-Customer Error Isolation

Each customer's processing is isolated in a goroutine. Errors in one customer do not affect others:

1. **Identity Center Role Assumption Failure**: Log error, mark customer as failed, continue with other customers
2. **Identity Center Data Retrieval Failure**: Log error, attempt file-based fallback if configured
3. **SES Role Assumption Failure**: Log error, mark customer as failed, continue with other customers
4. **Contact Import Failure**: Log error with details, mark customer as failed, continue with other customers

### Aggregated Error Reporting

At the end of processing:
- Report total customers processed
- Report successes and failures
- Exit with non-zero status if any customer failed
- Provide detailed error messages for each failure

## Testing Strategy

### Unit Tests

1. **Configuration Loading**: Test parsing of new `identity_center_role_arn` field
2. **Identity Center Instance Discovery**: Test with mocked SSO Admin API responses
3. **Data Retrieval**: Test `RetrieveIdentityCenterData` with mocked Identity Store API
4. **Import Function**: Test `ImportAllAWSContacts` with both in-memory and file-based data sources


### Integration Tests

1. **End-to-End with Mock Roles**: Test full workflow with mocked AWS API calls
2. **Concurrent Processing**: Test with multiple customers to verify concurrency limits
3. **Fallback Behavior**: Test that file-based loading works when role ARN is not configured
4. **Dry-Run Mode**: Verify no actual changes are made in dry-run mode

### Manual Testing

1. **Single Customer**: Test with one customer configured with Identity Center role ARN
2. **Multiple Customers**: Test with multiple customers, some with role ARN, some without
3. **CLI Flag Override**: Test that `--identity-center-role-arn` flag overrides config
4. **Error Scenarios**: Test with invalid role ARNs, missing permissions, etc.

## Implementation Phases

### Phase 1: Configuration and Data Structures
- Add `identity_center_role_arn` field to `CustomerInfo`
- Create `IdentityCenterData` struct
- Create `CustomerImportResult` struct

### Phase 2: Identity Center Discovery and Retrieval
- Implement `DiscoverIdentityCenterInstanceID`
- Implement `RetrieveIdentityCenterData`
- Refactor existing functions to return data

### Phase 3: Enhanced Import Function
- Modify `ImportAllAWSContacts` to accept optional in-memory data
- Implement fallback logic for file-based loading

### Phase 4: Concurrent Customer Processing
- Implement `handleImportAWSContactAllEnhanced`
- Add worker pool for concurrent processing
- Implement result aggregation

### Phase 5: CLI Integration
- Add `--identity-center-role-arn` flag
- Wire up new handler to existing command
- Update help text and documentation


## Detailed Component Design

### Identity Center Instance Discovery

```go
func DiscoverIdentityCenterInstanceID(cfg aws.Config) (string, error) {
    ssoAdminClient := ssoadmin.NewFromConfig(cfg)
    
    result, err := ssoAdminClient.ListInstances(context.Background(), 
        &ssoadmin.ListInstancesInput{})
    if err != nil {
        return "", fmt.Errorf("failed to list Identity Center instances: %w", err)
    }
    
    if len(result.Instances) == 0 {
        return "", fmt.Errorf("no Identity Center instances found in account")
    }
    
    if len(result.Instances) > 1 {
        return "", fmt.Errorf("multiple Identity Center instances found, expected exactly one")
    }
    
    // Extract instance ID from ARN (format: arn:aws:sso:::instance/ssoins-xxxxxxxxxx)
    instanceArn := *result.Instances[0].InstanceArn
    instanceID := extractInstanceIDFromArn(instanceArn)
    
    return instanceID, nil
}
```

### Identity Center Data Retrieval

```go
func RetrieveIdentityCenterData(
    roleArn string,
    maxConcurrency int,
    requestsPerSecond int,
) (*IdentityCenterData, error) {
    // 1. Assume the Identity Center role
    cfg, err := assumeRoleAndGetConfig(roleArn, "identity-center-data-retrieval")
    if err != nil {
        return nil, fmt.Errorf("failed to assume Identity Center role: %w", err)
    }
    
    // 2. Discover Identity Center instance ID
    instanceID, err := DiscoverIdentityCenterInstanceID(cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to discover Identity Center instance: %w", err)
    }
    
    // 3. Retrieve users
    users, err := retrieveAllUsers(cfg, instanceID, maxConcurrency, requestsPerSecond)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve users: %w", err)
    }
    
    // 4. Retrieve group memberships
    memberships, err := retrieveAllGroupMemberships(cfg, instanceID, users, maxConcurrency, requestsPerSecond)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve group memberships: %w", err)
    }
    
    return &IdentityCenterData{
        Users:       users,
        Memberships: memberships,
        InstanceID:  instanceID,
    }, nil
}
```


### Concurrent Customer Processing

```go
func handleImportAWSContactAllEnhanced(
    cfg *types.Config,
    identityCenterRoleArn *string,
    maxConcurrency int,
    requestsPerSecond int,
    dryRun bool,
) error {
    customers := getCustomersToProcess(cfg)
    
    // Create worker pool
    semaphore := make(chan struct{}, maxConcurrency)
    results := make(chan CustomerImportResult, len(customers))
    var wg sync.WaitGroup
    
    // Process each customer concurrently
    for _, customerCode := range customers {
        wg.Add(1)
        go func(custCode string) {
            defer wg.Done()
            
            // Acquire semaphore
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            result := processCustomer(cfg, custCode, identityCenterRoleArn, 
                requestsPerSecond, dryRun)
            results <- result
        }(customerCode)
    }
    
    // Wait for all workers to complete
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Aggregate results
    return aggregateAndReportResults(results)
}

func processCustomer(
    cfg *types.Config,
    customerCode string,
    identityCenterRoleArn *string,
    requestsPerSecond int,
    dryRun bool,
) CustomerImportResult {
    result := CustomerImportResult{CustomerCode: customerCode}
    
    customerInfo := cfg.CustomerMappings[customerCode]
    
    // Determine Identity Center role ARN (CLI flag takes precedence)
    icRoleArn := ""
    if identityCenterRoleArn != nil && *identityCenterRoleArn != "" {
        icRoleArn = *identityCenterRoleArn
    } else if customerInfo.IdentityCenterRoleArn != "" {
        icRoleArn = customerInfo.IdentityCenterRoleArn
    }
    
    var icData *aws.IdentityCenterData
    var err error
    
    // Retrieve Identity Center data if role ARN is configured
    if icRoleArn != "" {
        log.Printf("🔐 Customer %s: Retrieving Identity Center data via role %s", 
            customerCode, icRoleArn)
        
        icData, err = aws.RetrieveIdentityCenterData(icRoleArn, 10, requestsPerSecond)
        if err != nil {
            log.Printf("❌ Customer %s: Failed to retrieve Identity Center data: %v", 
                customerCode, err)
            result.Error = err
            return result
        }
        
        log.Printf("✅ Customer %s: Retrieved %d users and %d group memberships", 
            customerCode, len(icData.Users), len(icData.Memberships))
    } else {
        log.Printf("📁 Customer %s: Using file-based Identity Center data", customerCode)
    }
    
    // Assume SES role for customer
    sesConfig, err := assumeSESRole(customerInfo.SESRoleArn, customerCode)
    if err != nil {
        log.Printf("❌ Customer %s: Failed to assume SES role: %v", customerCode, err)
        result.Error = err
        return result
    }
    
    sesClient := sesv2.NewFromConfig(sesConfig)
    
    // Import contacts
    err = ses.ImportAllAWSContacts(sesClient, icData, dryRun, requestsPerSecond)
    if err != nil {
        log.Printf("❌ Customer %s: Failed to import contacts: %v", customerCode, err)
        result.Error = err
        return result
    }
    
    result.Success = true
    log.Printf("✅ Customer %s: Successfully imported contacts", customerCode)
    
    return result
}
```


### Enhanced Import Function

```go
func ImportAllAWSContacts(
    sesClient *sesv2.Client,
    identityCenterData *aws.IdentityCenterData,
    dryRun bool,
    requestsPerSecond int,
) error {
    var users []types.IdentityCenterUser
    var memberships []types.IdentityCenterGroupMembership
    var identityCenterId string
    
    // Use in-memory data if provided, otherwise load from files
    if identityCenterData != nil {
        log.Printf("📊 Using in-memory Identity Center data")
        users = identityCenterData.Users
        memberships = identityCenterData.Memberships
        identityCenterId = identityCenterData.InstanceID
    } else {
        log.Printf("📁 Loading Identity Center data from files")
        var err error
        users, memberships, identityCenterId, err = LoadIdentityCenterDataFromFiles("")
        if err != nil {
            return fmt.Errorf("failed to load Identity Center data: %w", err)
        }
    }
    
    // Rest of the import logic remains the same
    // ... (existing implementation)
}
```

## Backward Compatibility

The design maintains full backward compatibility:

1. **File-based workflow**: If `identity_center_role_arn` is not configured, the system falls back to loading from JSON files
2. **Existing commands**: All existing commands continue to work without modification
3. **Configuration**: The new field is optional, existing configurations work without changes
4. **API signatures**: Enhanced functions accept optional parameters, maintaining compatibility with existing callers

## Performance Considerations

1. **Concurrent Processing**: Multiple customers processed in parallel (up to `max-concurrency` limit)
2. **Rate Limiting**: Applied per-customer to avoid throttling
3. **Memory Usage**: Identity Center data held in memory only during processing, released after import
4. **Role Assumption**: Credentials cached within each customer's processing goroutine

## Security Considerations

1. **Least Privilege**: Identity Center role should have read-only permissions
2. **Credential Isolation**: Each customer's credentials isolated in separate goroutines
3. **Audit Logging**: All role assumptions and API calls logged for audit trail
4. **Dry-Run Mode**: Allows testing without making actual changes

