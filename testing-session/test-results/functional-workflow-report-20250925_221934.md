# Functional Workflow Test Report

**Test Run:** Thu Sep 25 22:19:35 EDT 2025
**Pipeline:** Web UI → S3 → SQS → Application Processing

## Configuration Tested

- **S3 Bucket:** 
- **SQS Queue:** 
- **Customer Code:** 
- **Customer Prefix:** 

## Summary

- **Total Tests:** 2
- **Passed:** 1
- **Failed:** 1
- **Success Rate:** 50%

## Test Categories

1. **S3 Infrastructure** - Bucket access and prefix validation
2. **SQS Infrastructure** - Queue access and message handling
3. **Metadata Creation** - Test file generation and validation
4. **S3 Upload Simulation** - Web UI upload workflow simulation
5. **S3 Event Processing** - S3 → SQS event trigger validation
6. **Application Processing** - SQS message processing by CLI
7. **End-to-End Validation** - Complete pipeline test
8. **Web UI Validation** - HTML file and deployment check

## Detailed Results

See full log: `functional-workflow-test-20250925_221934.log`

## Next Steps

⚠️ **Some functional tests failed.** Review the issues before production deployment.

**Common issues to check:**
- S3 bucket permissions and event notifications
- SQS queue permissions and visibility
- Application configuration and AWS credentials
- Web UI deployment and accessibility
