# Implementation Plan

- [x] 1. Create shared concurrent processing infrastructure
  - Create `CustomerResult` struct to hold operation results
  - Create `MultiCustomerSummary` struct for aggregated results
  - Implement `processCustomersConcurrently()` generic function with worker pool pattern
  - Implement `aggregateResults()` function to collect and summarize results
  - Implement `displayCustomerResult()` function for formatted output
  - _Requirements: 1.4, 3.1, 3.2, 3.3, 4.1_

- [x] 2. Implement configuration validation
  - Create `validateCustomerConfigs()` function to check config.json
  - Add validation for missing SES role ARNs
  - Add validation for empty customer mappings
  - Add warning logs for customers without SES role ARNs
  - _Requirements: 1.2, 3.4_

- [x] 3. Implement manage-topic-all action
  - Create `handleManageTopicAll()` function
  - Load config.json and validate customer configurations
  - Load SESConfig.json for topic definitions
  - Use `processCustomersConcurrently()` to process all customers
  - For each customer, assume SES role and call existing manage-topic logic
  - Display aggregated results with success/failure counts
  - Support `--dry-run` flag to preview changes
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 4. Implement describe-list-all action
  - Create `handleDescribeListAll()` function
  - Load config.json and validate customer configurations
  - Use `processCustomersConcurrently()` to process all customers
  - For each customer, assume SES role and call existing describe-list logic
  - Display results with customer labels
  - Handle failures gracefully and continue processing
  - _Requirements: 2.1, 2.4, 2.5_

- [x] 5. Implement list-contacts-all action
  - Create `handleListContactsAll()` function
  - Load config.json and validate customer configurations
  - Use `processCustomersConcurrently()` to process all customers
  - For each customer, assume SES role and call existing list-contacts logic
  - Display results with customer labels
  - Handle failures gracefully and continue processing
  - _Requirements: 2.2, 2.4, 2.5_

- [x] 6. Implement describe-topics-all action
  - Create `handleDescribeTopicsAll()` function
  - Load config.json and validate customer configurations
  - Use `processCustomersConcurrently()` to process all customers
  - For each customer, assume SES role and call existing describe-topic-all logic
  - Display results with customer labels
  - Handle failures gracefully and continue processing
  - _Requirements: 2.3, 2.4, 2.5_

- [x] 7. Add CLI routing for new actions
  - Add case statements in `handleSESCommand()` for all new `-all` actions
  - Add validation to reject `-customer-code` flag with `-all` actions
  - Add validation to reject `-ses-role-arn` flag with `-all` actions
  - Add validation to require config.json for `-all` actions
  - _Requirements: 5.4, 6.2, 6.5_

- [x] 8. Implement concurrency control
  - Add `--max-customer-concurrency` flag to CLI
  - Default to number of customers (unlimited concurrency)
  - Allow setting lower values to limit concurrency
  - Ignore values higher than number of customers
  - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [x] 9. Implement error handling and reporting
  - Ensure errors in one customer don't stop processing of others
  - Log errors with customer context
  - Display summary at end showing all successes and failures
  - Exit with non-zero code if any customer failed
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 10. Update help text and documentation
  - Add new `-all` actions to `showSESUsage()` function
  - Update README.md with new actions and usage examples
  - Add examples showing multi-customer operations
  - Document `--max-customer-concurrency` flag
  - Document that `-all` actions require config.json
  - _Requirements: 6.1, 6.3, 6.4, 6.5_

- [ ]* 11. Write unit tests
  - Test `processCustomersConcurrently()` with mock operations
  - Test `aggregateResults()` with various result combinations
  - Test `validateCustomerConfigs()` with valid and invalid configs
  - Test concurrency control with different limits
  - Test error handling and graceful degradation
  - _Requirements: All_

- [ ]* 12. Write integration tests
  - Test manage-topic-all with mock customers
  - Test describe-list-all with mock customers
  - Test error scenarios (one customer fails, all fail, none configured)
  - Test dry-run mode for all actions
  - Test backward compatibility of existing single-customer actions
  - _Requirements: All_
