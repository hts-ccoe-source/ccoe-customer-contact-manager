# CCOE Customer Contact Manager - Testing Session

This directory contains comprehensive tests for the CCOE Customer Contact Manager application.

## Test Categories

### 1. Configuration Tests
- Validate all JSON configuration files
- Test configuration loading and parsing
- Verify customer code mappings

### 2. AWS Infrastructure Tests
- Test AWS credentials and permissions
- Validate SQS queue access
- Test S3 bucket operations
- Verify SES configuration

### 3. Application Mode Tests
- Test update mode with dry-run
- Test SQS message processing
- Test validation mode
- Test multi-customer operations

### 4. Integration Tests
- End-to-end workflow testing
- Multi-customer email distribution
- Error handling and recovery

## Running Tests

Execute tests in order:

```bash
# 1. Configuration validation
./test-config.sh

# 2. AWS infrastructure validation
./test-aws-infrastructure.sh

# 3. Application functionality
./test-app-modes.sh

# 4. Integration testing
./test-integration.sh
```

## Test Results

Results are logged to `test-results/` directory with timestamps.