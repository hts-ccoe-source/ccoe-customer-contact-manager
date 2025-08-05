# GitHub Copilot Instructions for Breakglass Role Creator

This file contains custom instructions for GitHub Copilot to align with the coding practices and requirements of the Breakglass Role Creator project.

## Project Overview

This is a Go-based AWS automation tool that creates breakglass roles and manages CloudFormation stacks with GitSync integration. The tool processes `*-config.yaml` files to create, update, or delete CloudFormation stacks and sync configurations.

## Language and Framework Preferences

- **Go Version**: Use Go 1.21+ (current: 1.23.2)
- **AWS SDK**: Use AWS SDK for Go v2 (github.com/aws/aws-sdk-go-v2)
- **Go Modules**: Always use Go modules for dependency management

## Code Style and Conventions

### Naming Conventions
- **Functions**: Use PascalCase for exported functions (e.g., `CreateStack`, `AssumeRole`)
- **Variables**: Use camelCase for variables (e.g., `stackName`, `currentAccountId`)
- **Constants**: Use ALL_CAPS with underscores for constants
- **Stack Names**: Always lowercase stack names for consistency with existing functions
- **File Naming**: Use kebab-case for config files (`*-config.yaml`)

### Error Handling
- Always return descriptive errors using `fmt.Errorf` with context
- Use wrapped errors with `%w` verb for error chaining
- Handle AWS API errors gracefully with appropriate logging
- Example:
  ```go
  if err != nil {
      return fmt.Errorf("failed to create stack %s: %w", stackName, err)
  }
  ```

### AWS SDK Patterns
- Use context.Background() for AWS API calls
- Always check for nil pointers when dereferencing AWS SDK response fields
- Use `aws.String()` for string pointers when required by AWS APIs
- Prefer AWS SDK v2 clients over v1
- Use `aws.Config` for credential management

### Concurrency Patterns
- Use `sync.WaitGroup` for coordinating goroutines
- Use `sync.Mutex` for protecting shared state
- Use channels for collecting results from goroutines
- Implement rate limiting with `time.Ticker` for AWS API calls
- Example pattern:
  ```go
  var wg sync.WaitGroup
  var mu sync.Mutex
  ticker := time.NewTicker(3 * time.Second)
  defer ticker.Stop()
  
  for _, item := range items {
      wg.Add(1)
      go processItem(item, &wg, &mu, ticker)
  }
  wg.Wait()
  ```

### Logging and Output
- Use `fmt.Println` for user-facing status messages
- Use `log.Printf` for error logging
- Use `log.Fatalf` for fatal errors that should terminate the program
- Include relevant context in log messages (stack names, account IDs, etc.)

## AWS-Specific Patterns

### Credential Management
- Support both environment variables and role assumption
- Transform between `*ststypes.Credentials` and `aws.Credentials` as needed
- Use `CreateConnectionConfiguration` function for creating AWS configs
- Example:
  ```go
  awsCreds := aws.Credentials{
      AccessKeyID:     *assumedCreds.AccessKeyId,
      SecretAccessKey: *assumedCreds.SecretAccessKey,
      SessionToken:    *assumedCreds.SessionToken,
      Source:          "AssumeRole",
  }
  ```

### CloudFormation Operations
- Always use waiters for async operations (CreateStack, UpdateStack, DeleteStack)
- Include `CAPABILITY_NAMED_IAM` capability for stack operations
- Compare templates as strings before updating stacks
- Use lowercase stack names consistently
- Set appropriate timeouts (5 minutes for stack operations)

### CodeConnections (GitSync)
- Check for existing connections/links before creating new ones
- Use proper resource naming patterns
- Handle sync configuration updates when branches change

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

## File and Directory Patterns

### Configuration Files
- Process `*-config.yaml` files from repository directories
- Extract template paths from config file contents
- Use relative paths like `../{repoName}/` for file access
- Skip YAML frontmatter (`---`) when processing config files

### File Operations
- Use `filepath.Walk` for recursive file discovery
- Use `os.ReadFile` for reading file contents
- Handle file path operations with `filepath` package functions
- Convert file paths to stack names by removing suffixes

## Command Line Interface

### Flag Management
- Use separate `flag.NewFlagSet` for each subcommand
- Provide clear usage examples in help text
- Validate required flags and exit with usage on missing parameters
- Support subcommands: `both` (GitSync + CloudFormation) and `only-cfn` (CloudFormation only)

### Default Values
- Use sensible defaults (e.g., `main` branch for GitSync)
- Make organization name required for all operations
- Require role ARN for cross-account operations

## Security Considerations

### IAM and Permissions
- Always assume roles when working across accounts
- Use least privilege principles
- Include proper capability declarations for CloudFormation
- Validate management account status before operations

### Credential Handling
- Check for required environment variables at startup
- Never log sensitive credential information (mask with "xxxx")
- Use temporary credentials from role assumption

## Performance Optimization

### Concurrent Operations
- Process stack configurations concurrently using goroutines
- Use rate limiting to avoid AWS API throttling
- Implement proper synchronization for shared state
- Batch operations where possible

### Resource Management
- Always clean up tickers and channels
- Use defer statements for cleanup operations
- Close channels after all goroutines complete

## Project-Specific Requirements

### Stack Management
- Support both creation and deletion of stacks
- Compare existing templates before updating
- Use organization-based prefixes for stack filtering
- Handle stack status filtering for active stacks only

### GitSync Integration
- Create repository links automatically if missing
- Update sync configurations when branches change
- Delete sync configurations before deleting stacks
- Validate GitHub connections exist before proceeding

## Docker and Deployment

### Container Patterns
- Use multi-stage builds with Alpine Linux
- Include AWS CLI in runtime container
- Build static binaries for container deployment
- Support environment variable configuration

### Build Process
- Use `go get . && go build` for local builds
- Support Docker builds via buildspec.yml
- Tag container images appropriately for ECR

## Code Organization

### Function Responsibilities
- Keep functions focused on single responsibilities
- Use worker functions for concurrent operations
- Separate AWS client creation from business logic
- Return structured results from worker functions

### Struct Usage
- Define result structs for aggregating worker outputs
- Use channels to collect results from goroutines
- Embed context and configuration in function parameters

When suggesting code changes or new features, follow these patterns and conventions to maintain consistency with the existing codebase.