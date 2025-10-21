# Implementation Plan

- [x] 1. Extend configuration structures for Route53 and SES domain validation
  - Add Route53Config type to internal/config with zone_id and role_arn fields
  - Extend CustomerMapping type with optional dkim_tokens and verification_token fields
  - Update Config struct to include optional Route53Config
  - _Requirements: 6.3, 6.5_

- [x] 2. Implement SES domain manager
  - [x] 2.1 Create internal/ses/domain.go with DomainManager struct
    - Define DomainManager with sesv2 client, logger, and dryRun flag
    - Define DomainConfig struct for email address and domain name
    - Define DomainTokens struct for verification token and DKIM tokens array
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 2.2 Implement NewDomainManager constructor
    - Accept aws.Config, dryRun bool, and slog.Logger parameters
    - Create SESV2 client from config
    - Return initialized DomainManager
    - _Requirements: 1.1_

  - [x] 2.3 Implement ConfigureDomain method
    - Call createEmailIdentity for email address (<ccoe@hearst.com>)
    - Call createDomainIdentity for domain (ccoe.hearst.com)
    - Call getDomainTokens to retrieve verification and DKIM tokens
    - Return DomainTokens struct with captured tokens
    - Log each operation with structured logging
    - _Requirements: 1.1, 1.2, 1.3, 1.5, 7.2_

  - [x] 2.4 Implement createEmailIdentity method
    - Use SESV2 CreateEmailIdentity API for email address
    - Handle dry-run mode (log without API call)
    - Implement idempotent behavior (handle AlreadyExists error)
    - Return error if API call fails after retries
    - _Requirements: 1.1, 1.4, 5.1_

  - [x] 2.5 Implement createDomainIdentity method
    - Use SESV2 CreateEmailIdentity API for domain
    - Configure DKIM signing attributes (DkimSigningAttributesOrigin: AWS_SES)
    - Handle dry-run mode (log without API call)
    - Implement idempotent behavior (handle AlreadyExists error)
    - Return error if API call fails after retries
    - _Requirements: 1.2, 1.3, 1.4, 5.1_

  - [x] 2.6 Implement getDomainTokens method
    - Use SESV2 GetEmailIdentity API to retrieve domain identity details
    - Extract verification token from DkimAttributes
    - Extract three DKIM tokens from DkimAttributes
    - Validate that exactly 3 DKIM tokens are returned
    - Return DomainTokens struct
    - _Requirements: 1.5, 3.2_

- [x] 3. Implement Route53 DNS manager
  - [x] 3.1 Create internal/route53/manager.go with DNSManager struct
    - Define DNSManager with route53 client, logger, and dryRun flag
    - Define DNSConfig struct for zone_id and zone_name
    - Define OrganizationDNS struct for name, dkim_tokens, and verification_token
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 3.2 Implement NewDNSManager constructor
    - Accept aws.Config, dryRun bool, and slog.Logger parameters
    - Create Route53 client from config
    - Return initialized DNSManager
    - _Requirements: 2.1_

  - [x] 3.3 Implement getHostedZoneName method
    - Use Route53 GetHostedZone API with zone_id
    - Extract zone name from response
    - Strip trailing dot if present
    - Return zone name string
    - _Requirements: 6.3_

  - [x] 3.4 Implement ConfigureRecords method
    - Call getHostedZoneName to look up zone name from zone_id
    - Validate each organization configuration (3 DKIM tokens, verification token present)
    - Build array of Route53 Change objects for all organizations
    - Call createDKIMRecords for each organization
    - Call createVerificationRecords for all organizations
    - Call applyChanges with all changes
    - Log summary of records created
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 3.1, 3.2, 3.3, 3.4, 3.5, 7.3, 7.4_

  - [x] 3.5 Implement createDKIMRecords method
    - For each of 3 DKIM tokens, create CNAME record
    - Record name format: {token}._domainkey.{zoneName}
    - Record value format: {token}.dkim.amazonses.com
    - Set TTL to 600 seconds
    - Use UPSERT action for idempotency
    - Return array of Change objects
    - _Requirements: 2.2, 2.4, 5.2_

  - [x] 3.6 Implement createVerificationRecords method
    - For each organization, create TXT record with same name
    - Record name format: _amazonses.{zoneName}
    - Record value: "{verificationToken}" (quoted)
    - Set TTL to 600 seconds
    - Use UPSERT action for idempotency
    - Return array of Change objects (one per organization)
    - _Requirements: 2.3, 2.4, 5.2_

  - [x] 3.7 Implement applyChanges method
    - Split changes into batches of 1000 (Route53 limit)
    - For each batch, call Route53 ChangeResourceRecordSets API
    - Handle dry-run mode (log without API call)
    - Log each batch applied
    - Return error if any batch fails after retries
    - _Requirements: 2.5, 5.5, 7.3_

  - [x] 3.8 Implement validateOrganization method
    - Check that exactly 3 DKIM tokens are present
    - Check that verification token is non-empty
    - Return error with descriptive message if validation fails
    - _Requirements: 3.2, 3.3_

- [x] 4. Implement retry logic with exponential backoff
  - [x] 4.1 Create internal/aws/retry.go with retry utilities
    - Define RetryConfig struct with max attempts, delays, backoff factor, jitter
    - Implement retryWithBackoff function accepting operation and config
    - Implement exponential backoff calculation with jitter
    - Implement isRetryableError function to check AWS error types
    - _Requirements: 5.3, 5.4_

  - [x] 4.2 Integrate retry logic into SES operations
    - Wrap SESV2 API calls with retryWithBackoff
    - Use default RetryConfig (5 attempts, 1s initial, 30s max, 2.0 factor)
    - Log retry attempts with structured logging
    - _Requirements: 5.3, 5.4, 7.1_

  - [x] 4.3 Integrate retry logic into Route53 operations
    - Wrap Route53 API calls with retryWithBackoff
    - Use default RetryConfig (5 attempts, 1s initial, 30s max, 2.0 factor)
    - Log retry attempts with structured logging
    - _Requirements: 5.3, 5.4, 7.1_

- [x] 5. Implement CLI command handlers
  - [x] 5.1 Add ses configure-domain command handler in main.go
    - Define handleSESConfigureDomainCommand function
    - Parse flags: config, customer, dry-run, profile, region, configure-dns, dns-role-arn
    - Load configuration from config.json
    - Validate route53_config present if configure-dns=true
    - Iterate through customer_mappings (all or single customer)
    - For each customer, assume SES role and call DomainManager.ConfigureDomain
    - Collect tokens in memory (map[customerCode]DomainTokens)
    - If configure-dns=true, assume DNS role and call DNSManager.ConfigureRecords
    - Output summary of operations
    - Return appropriate exit code
    - _Requirements: 1.1, 1.4, 4.1, 4.2, 4.3, 4.4, 4.5, 6.1, 6.2, 7.4, 8.1, 8.3_

  - [x] 5.2 Add route53 configure command handler in main.go
    - Define handleRoute53Command function
    - Parse flags: action, config, role-arn, dry-run, profile, region
    - Load configuration from config.json
    - Read tokens from customer_mappings in config
    - Validate that all customers have tokens present
    - Assume DNS role
    - Call DNSManager.ConfigureRecords with tokens from config
    - Output summary of operations
    - Return appropriate exit code
    - _Requirements: 2.1, 2.5, 4.1, 4.2, 4.3, 4.4, 4.5, 6.1, 6.2, 7.4, 8.2, 8.5_

  - [x] 5.3 Update showUsage function in main.go
    - Add ses configure-domain command description
    - Add route53 command description
    - _Requirements: 8.1, 8.2_

- [x] 6. Implement structured logging
  - [x] 6.1 Initialize slog logger in command handlers
    - Create logger with appropriate log level from config
    - Use JSON format for structured output
    - Pass logger to DomainManager and DNSManager
    - _Requirements: 7.1_

  - [x] 6.2 Add logging to SES operations
    - Log customer being processed
    - Log email identity creation
    - Log domain identity creation
    - Log DKIM configuration
    - Log tokens retrieved
    - Log dry-run mode indicator
    - _Requirements: 7.1, 7.2, 7.5_

  - [x] 6.3 Add logging to Route53 operations
    - Log organization being processed
    - Log each DNS record being created (name, type, value)
    - Log batch operations
    - Log dry-run mode indicator
    - _Requirements: 7.1, 7.3, 7.5_

  - [x] 6.4 Implement summary output
    - Count successful and failed operations
    - Output summary for SES operations (identities created, tokens retrieved)
    - Output summary for Route53 operations (records created)
    - Include counts in structured format
    - _Requirements: 3.5, 7.4_

- [x] 7. Add error handling and validation
  - [x] 7.1 Implement configuration validation
    - Validate config.json exists and is valid JSON
    - Validate required fields present (aws_region, customer_mappings)
    - Validate route53_config present when needed
    - Validate customer_mappings have required fields (customer_code, ses_role_arn)
    - Return descriptive errors for missing/invalid fields
    - _Requirements: 6.3, 6.4_

  - [x] 7.2 Implement customer validation
    - Check that specified customer code exists in customer_mappings
    - Validate SES role ARN format
    - Return descriptive error if customer not found
    - _Requirements: 4.1, 6.3_

  - [x] 7.3 Implement organization validation in batch processing
    - Validate DKIM tokens count (exactly 3)
    - Validate verification token non-empty
    - Log warning and skip invalid organizations
    - Continue processing remaining organizations
    - Track skipped organizations in summary
    - _Requirements: 3.2, 3.3, 3.4_

  - [x] 7.4 Implement error aggregation
    - Collect errors from all customer/organization processing
    - Continue processing on individual failures
    - Return non-zero exit code if any failures occurred
    - Include error details in summary output
    - _Requirements: 4.3, 4.4, 5.4_

- [x] 8. Update Makefile and build configuration
  - [x] 8.1 Add Go module dependencies
    - Add github.com/aws/aws-sdk-go-v2/service/sesv2 to go.mod
    - Add github.com/aws/aws-sdk-go-v2/service/route53 to go.mod
    - Run go mod tidy
    - _Requirements: 1.1, 2.1_

  - [x] 8.2 Update Makefile targets
    - Ensure existing build targets work with new code
    - Verify package-golang-lambda target includes new packages
    - _Requirements: 8.1, 8.2_

- [x] 9. Create example configuration
  - [x] 9.1 Create example config.json snippet
    - Document route53_config structure
    - Document optional token fields in customer_mappings
    - Add comments explaining each field
    - Include example with multiple customers
    - _Requirements: 6.1, 6.2, 6.3, 6.5_

  - [x] 9.2 Update README with usage examples
    - Document ses configure-domain command with all flags
    - Document route53 configure command with all flags
    - Provide integrated workflow example
    - Provide standalone workflow examples
    - Document dry-run usage
    - Document single customer vs all customers usage
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_
