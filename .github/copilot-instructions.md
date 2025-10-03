# GitHub Copilot Instructions for AWS Alternate Contact Manager

This file contains custom instructions for GitHub Copilot to align with the coding practices and requirements of the AWS Alternate Contact Manager project.

## Project Overview

This is a Go-based AWS automation tool that manages AWS alternate contacts and SES contact lists with Identity Center integration. The tool provides two main subcommands:
- `alt-contact`: Manages AWS alternate contacts across organizations
- `ses`: Manages SES contact lists, topics, and subscriptions with Identity Center user import capabilities

## Language and Framework Preferences

- **Go Version**: Use Go 1.21+ (current: 1.23.2)
- **AWS SDK**: Use AWS SDK for Go v2 (github.com/aws/aws-sdk-go-v2)
- **Go Modules**: Always use Go modules for dependency management

## Code Style and Conventions

### Naming Conventions
- **Functions**: Use PascalCase for exported functions (e.g., `CreateContactList`, `AssumeRole`, `ImportAllAWSContacts`)
- **Variables**: Use camelCase for variables (e.g., `contactListName`, `identityCenterId`, `topicName`)
- **Constants**: Use ALL_CAPS with underscores for constants
- **Contact List Names**: Use descriptive names with account context (e.g., `AppCommonNonProd`)
- **File Naming**: Use kebab-case for config files (`config.json`, `SESConfig.json`)

### Error Handling
- Always return descriptive errors using `fmt.Errorf` with context
- Use wrapped errors with `%w` verb for error chaining
- Handle AWS API errors gracefully with appropriate logging
- Example:
  ```go
  if err != nil {
      return fmt.Errorf("failed to create contact list %s: %w", listName, err)
  }
  ```

### AWS SDK Patterns
- Use context.Background() for AWS API calls
- Always check for nil pointers when dereferencing AWS SDK response fields
- Use `aws.String()` for string pointers when required by AWS APIs
- Prefer AWS SDK v2 clients over v1
- Use `aws.Config` for credential management
- Use proper service clients: `sesv2.Client`, `account.Client`, `organizations.Client`, `identitystore.Client`

### Concurrency Patterns
- Use `sync.WaitGroup` for coordinating goroutines
- Use `sync.Mutex` for protecting shared state
- Use channels for collecting results from goroutines
- Implement rate limiting with `time.Ticker` for AWS API calls
- Support configurable concurrency and rate limiting parameters
- Example pattern:
  ```go
  var wg sync.WaitGroup
  var mu sync.Mutex
  ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
  defer ticker.Stop()
  
  semaphore := make(chan struct{}, maxConcurrency)
  
  for _, user := range users {
      wg.Add(1)
      go processUser(user, &wg, &mu, ticker, semaphore)
  }
  wg.Wait()
  ```

### Logging and Output
- Use `fmt.Printf` with emojis for user-facing status messages (📧, 📋, ✅, ❌, 🔍, 👥)
- Use descriptive progress indicators and summaries
- Include relevant context in log messages (contact list names, topic names, user counts, etc.)
- Provide clear error messages with actionable information
- Use consistent formatting for summaries and reports

## AWS-Specific Patterns

### Credential Management
- Support both environment variables and role assumption
- Transform between `*ststypes.Credentials` and `aws.Credentials` as needed
- Use separate role ARNs for different operations (SES vs Identity Center)
- Example:
  ```go
  awsCreds := aws.Credentials{
      AccessKeyID:     *assumedCreds.AccessKeyId,
      SecretAccessKey: *assumedCreds.SecretAccessKey,
      SessionToken:    *assumedCreds.SessionToken,
      Source:          "AssumeRole",
  }
  ```

### SES Operations
- Use `sesv2.Client` for all SES operations
- Always include `ListManagementOptions` when sending emails through contact lists
- Create backups before destructive operations (remove-contact-all, manage-topic, delete-list)
- Use proper topic subscription status types (`SubscriptionStatusOptIn`, `SubscriptionStatusOptOut`)
- Handle pagination for large contact lists

### Identity Center Integration
- Use `identitystore.Client` for user and group operations
- Support auto-detection of Identity Center ID from existing files
- Generate timestamped JSON files for data persistence
- Parse CCOE cloud group naming conventions (`ccoe-cloud-{account}-{role}`)
- Map user roles to topic subscriptions based on configuration

### Account Management
- Use `account.Client` for alternate contact operations
- Use `organizations.Client` for organization-wide operations
- Support both single account and organization-wide contact management

## Testing Patterns

### Unit Testing Structure
- Use `testify/mock` for mocking AWS clients
- Create interfaces for AWS clients to enable mocking
- Structure tests with Given/When/Then comments
- Example interface pattern:
  ```go
  type STSClientInterface interface {
      AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
  }
  ```

### Mock Setup
- Set up all expected method calls on mocks
- Use `mock.AnythingOfType()` for complex parameter matching
- Return appropriate mock responses that match AWS SDK types
- Mock SES, Identity Center, and Account service clients appropriately

## File and Directory Patterns

### Configuration Files
- Use consolidated JSON configuration file (`config.json`) with separate SES config (`SESConfig.json`)
- Support topic group expansion with prefixes and members
- Store Identity Center data in timestamped JSON files
- Use `GetConfigPath()` function for consistent file location handling

### File Operations
- Use `os.ReadFile` for reading configuration files
- Generate timestamped filenames for data exports
- Support auto-detection of existing data files by pattern matching
- Use `findMostRecentFile()` for loading latest data files
- Handle file path operations with proper error checking

## Command Line Interface

### Flag Management
- Use separate `flag.NewFlagSet` for each subcommand (`alt-contact`, `ses`)
- Provide comprehensive help text with examples and emoji indicators
- Validate required flags and provide clear error messages
- Support extensive SES actions: create-list, add-contact, remove-contact, delete-list, manage-topic, import-aws-contact-all, etc.

### Default Values
- Use sensible defaults (e.g., 10 max-concurrency, 10 requests-per-second)
- Make Identity Center ID optional when data files exist (auto-detection)
- Support dry-run mode for preview operations
- Provide backup file options for restore operations

## Security Considerations

### IAM and Permissions
- Support separate role ARNs for SES and Identity Center operations
- Use least privilege principles for AWS service access
- Validate management account access for Identity Center operations
- Handle cross-account alternate contact management securely

### Credential Handling
- Support optional role assumption for enhanced security
- Never log sensitive credential information
- Use temporary credentials from role assumption when specified
- Validate sender email addresses for SES operations

## Performance Optimization

### Concurrent Operations
- Process Identity Center users concurrently with configurable limits
- Use rate limiting to avoid AWS API throttling (configurable requests-per-second)
- Implement proper synchronization for shared state and progress tracking
- Support batch operations for contact list management

### Resource Management
- Always clean up tickers and channels
- Use defer statements for cleanup operations
- Implement idempotent operations (skip existing contacts with same topics)
- Provide progress indicators for long-running operations

## Project-Specific Requirements

### Contact Management
- Support both individual and organization-wide alternate contact operations
- Implement idempotent SES contact imports (skip existing, update changed)
- Create automatic backups before destructive operations
- Support topic-based subscription management with role mappings

### Identity Center Integration
- Auto-detect Identity Center ID from existing data files
- Parse CCOE cloud group naming conventions for role extraction
- Generate comprehensive user and group membership data files
- Map user roles to topic subscriptions based on configuration
- Support both individual user and bulk import operations

### SES Features
- Implement comprehensive contact list management (create, describe, manage topics)
- Support test email functionality with proper unsubscribe compliance
- Handle topic subscription statistics and reporting
- Provide suppression list management capabilities

## Build and Deployment

### Build Process
- Use `go build aws-alternate-contact-manager.go` for local builds
- Support Go modules for dependency management
- Build single binary with embedded configuration support
- Test with both dry-run and actual operations

### Configuration Management
- Support JSON configuration files for contacts and SES settings
- Handle topic group expansion and role mappings
- Provide clear configuration examples and documentation
- Support file-based data persistence for Identity Center operations

## Code Organization

### Function Responsibilities
- Keep functions focused on single responsibilities (e.g., `ImportAllAWSContacts`, `SendTopicTestEmail`)
- Use worker functions for concurrent Identity Center operations
- Separate AWS client creation from business logic
- Implement comprehensive help functions with clear examples

### Struct Usage
- Define configuration structs for contacts and SES settings (`ContactImportConfig`, `SESTopicConfig`)
- Use structured data types for Identity Center users and group memberships
- Implement result aggregation for bulk operations
- Support JSON marshaling/unmarshaling for data persistence

### Data Flow Patterns
- Load Identity Center data from files with auto-detection
- Transform user group memberships into topic subscriptions
- Implement idempotent operations with existing contact checking
- Provide comprehensive progress reporting and error handling

When suggesting code changes or new features, follow these patterns and conventions to maintain consistency with the existing codebase. Focus on AWS service integration, user experience with clear progress indicators, and robust error handling for production use.