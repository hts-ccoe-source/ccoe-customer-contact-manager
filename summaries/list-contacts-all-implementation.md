# List Contacts All Implementation

## Summary

Implemented the `list-contacts-all` action that lists contacts across all customer accounts concurrently using their respective SES role ARNs.

## Changes Made

### 1. New Function: `handleListContactsAll()`
**Location**: `main.go` (line 1283)

The function follows the same pattern as `handleDescribeListAll()`:
- Validates customer configurations from config.json
- Builds list of customers with SES role ARNs
- Skips customers without SES role ARN with warning
- Processes customers concurrently using the shared infrastructure
- For each customer:
  - Assumes SES role
  - Creates SES client
  - Gets account contact list
  - Lists all contacts with topic subscriptions
- Displays results with customer labels
- Shows aggregated summary with success/failure counts
- Exits with error code if any customer failed

### 2. CLI Routing
**Location**: `main.go` (line 640)

Added case statement for `list-contacts-all` action:
```go
case "list-contacts-all":
    handleListContactsAll(cfg)
```

### 3. Help Text Updates
**Location**: `main.go` (showSESUsage function)

Added to contact list management section:
- Action description: "list-contacts-all       List contacts across ALL customers concurrently"
- Usage example showing how to use the action with config.json

## Key Features

1. **Concurrent Processing**: Uses the shared `concurrent.ProcessCustomersConcurrently()` infrastructure
2. **Error Handling**: Gracefully handles failures - one customer failure doesn't stop others
3. **Clear Output**: Each customer's contacts are clearly labeled with visual separators
4. **Configuration Validation**: Validates config.json and skips customers without SES role ARN
5. **Summary Display**: Shows aggregated results with success/failure counts and timing

## Requirements Satisfied

- ✅ Requirement 2.2: Retrieve and display all contacts from all customers concurrently
- ✅ Requirement 2.4: Clearly label which customer each result belongs to
- ✅ Requirement 2.5: Handle failures gracefully and continue processing

## Usage Example

```bash
# List contacts across all customers
ccoe-customer-contact-manager ses --action list-contacts-all \
  --config-file config.json
```

## Output Format

```
🔄 Listing contacts across 3 customer(s)

======================================================================
📋 CUSTOMER: htsnonprod (HTS Non-Production)
======================================================================
Contacts in list 'ccoe-customer-contacts' (150 total):

1. user1@example.com
   Last Updated: 2024-10-20 15:30:00 UTC
   Topic Subscriptions:
     - aws-calendar: OPT_IN
     - aws-announce: OPT_IN

[... more contacts ...]

======================================================================
📊 OPERATION SUMMARY
======================================================================
Total customers: 3
✅ Successful: 3
❌ Failed: 0
⏭️  Skipped: 0
⏱️  Total processing time: 2.50s
```

## Testing

- ✅ Code compiles successfully
- ✅ Help text displays correctly
- ✅ Function is properly routed in CLI
- ✅ Follows established patterns from `handleDescribeListAll()`
