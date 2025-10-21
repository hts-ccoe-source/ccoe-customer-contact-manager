# SES Domain Validation CLI - Error Handling and Validation Implementation

## Summary

Implemented comprehensive error handling and validation for the SES domain validation CLI feature, ensuring robust configuration validation, customer validation, organization validation in batch processing, and error aggregation across all operations.

## Implementation Details

### 1. Configuration Validation (Task 7.1)

Created `internal/config/validation.go` with comprehensive validation functions:

**Key Features:**
- `ValidateSESConfig()`: Validates configuration for SES domain validation operations
  - Checks required fields (aws_region, customer_mappings)
  - Validates Route53 config when DNS configuration is enabled
  - Validates customer mapping fields (customer_code, ses_role_arn)
  - Validates ARN formats

- `ValidateRoute53Config()`: Validates configuration for Route53 operations
  - Ensures route53_config is present
  - Validates zone_id and role_arn
  - Validates customer mappings

- `ValidationErrors` type: Collects multiple validation errors with descriptive messages
  - Provides clear error messages indicating which field failed validation
  - Supports multiple error aggregation

- `isValidARN()`: Helper function to validate AWS ARN format
  - Checks ARN structure (arn:partition:service:region:account-id:resource)
  - Ensures all required components are present

**Integration:**
- Updated `handleSESConfigureDomainCommand()` to call `ValidateSESConfig()` before processing
- Updated `handleRoute53Command()` to call `ValidateRoute53Config()` before processing
- Configuration validation happens early, preventing wasted processing time

### 2. Customer Validation (Task 7.2)

Implemented `ValidateCustomerCode()` function in `internal/config/validation.go`:

**Key Features:**
- Checks that customer code exists in customer_mappings
- Validates SES role ARN format for the customer
- Returns descriptive error messages with available customer codes
- Provides clear guidance when customer not found

**Integration:**
- Used in `handleSESConfigureDomainCommand()` when processing single customer
- Validates customer before attempting to process

### 3. Organization Validation in Batch Processing (Task 7.3)

Organization validation already implemented in `internal/route53/manager.go`:

**Key Features:**
- `validateOrganization()`: Validates organization DNS configuration
  - Checks exactly 3 DKIM tokens are present
  - Validates verification token is non-empty
  - Returns descriptive error messages

- `ConfigureRecords()`: Implements batch processing with validation
  - Validates each organization before processing
  - Logs warnings for invalid organizations
  - Skips invalid organizations and continues with valid ones
  - Tracks skipped count in summary
  - Returns error only if no valid organizations remain

**Behavior:**
- Invalid organizations are logged with warnings
- Processing continues for remaining valid organizations
- Summary includes both processed and skipped counts

### 4. Error Aggregation (Task 7.4)

Error aggregation implemented in command handlers:

**SES Configure Domain Command:**
- Collects errors in `errors []string` slice
- Tracks `successCount` and `errorCount`
- Continues processing on individual customer failures (using `continue`)
- Aggregates errors from:
  - Customer config retrieval failures
  - Domain configuration failures
  - DNS configuration failures (if enabled)
- Logs all errors in summary output
- Returns exit code 1 if any failures occurred

**Route53 Command:**
- Tracks invalid organizations in `invalidOrgs []string`
- Logs warnings for each skipped organization
- Continues processing valid organizations
- Includes skipped count in summary
- Returns exit code 1 on failure

**Summary Output:**
Both commands provide comprehensive summaries including:
- Total items processed
- Success count
- Failure count
- Detailed error messages (for SES command)
- Skipped organization count (for Route53 command)

## Error Handling Patterns

### Configuration Errors
- Validated early before any processing
- Descriptive error messages indicating missing/invalid fields
- Exit immediately with code 1

### Customer/Organization Validation Errors
- Logged as warnings
- Processing continues for remaining items
- Tracked in summary

### API Errors
- Wrapped with context using `aws.WrapAWSError()`
- Retried with exponential backoff (already implemented in tasks 4.x)
- Logged with structured logging
- Collected in error aggregation

### Exit Codes
- 0: Success (all operations completed successfully)
- 1: Failure (one or more operations failed)

## Testing Recommendations

1. **Configuration Validation:**
   - Test with missing required fields
   - Test with invalid ARN formats
   - Test with missing Route53 config when DNS enabled
   - Test with invalid customer codes

2. **Customer Validation:**
   - Test with non-existent customer code
   - Test with invalid SES role ARN
   - Test with valid customer code

3. **Organization Validation:**
   - Test with missing DKIM tokens
   - Test with wrong number of DKIM tokens (not 3)
   - Test with empty verification token
   - Test with mix of valid and invalid organizations

4. **Error Aggregation:**
   - Test with multiple customer failures
   - Test with partial failures (some succeed, some fail)
   - Verify exit code is 1 when any failure occurs
   - Verify summary includes all error details

## Files Modified

- `internal/config/validation.go` (NEW): Comprehensive validation functions
- `main.go`: Updated command handlers to use validation functions

## Requirements Satisfied

- ✅ 6.3: Validate config.json exists and is valid JSON
- ✅ 6.4: Validate required fields present
- ✅ 4.1: Check customer code exists in customer_mappings
- ✅ 3.2: Validate DKIM tokens count (exactly 3)
- ✅ 3.3: Validate verification token non-empty
- ✅ 3.4: Log warning and skip invalid organizations
- ✅ 4.3: Continue processing on individual failures
- ✅ 4.4: Return non-zero exit code if any failures occurred
- ✅ 5.4: Include error details in summary output
