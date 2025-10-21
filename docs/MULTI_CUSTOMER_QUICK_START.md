# Multi-Customer Email Distribution - Quick Start Guide

## 🚀 Overview

The CCOE Customer Contact Manager now supports **multi-customer email distribution**, allowing you to send change notifications to multiple customer organizations simultaneously with perfect isolation and scalability.

## 📋 Quick Setup

### 1. Configuration Files

Create these configuration files in your config directory:

#### `CustomerCodes.json`
```json
{
  "validCustomers": ["hts", "cds", "motor", "bat", "icx"],
  "customerMapping": {
    "hts": "HTS Prod",
    "cds": "CDS Global", 
    "motor": "Motor",
    "bat": "Bring A Trailer",
    "icx": "iCrossing"
  }
}
```

#### `S3EventConfig.json`
```json
{
  "bucketName": "multi-customer-metadata-bucket",
  "eventNotifications": [
    {
      "customerCode": "hts",
      "sqsQueueArn": "arn:aws:sqs:us-east-1:123456789012:hts-notifications",
      "prefix": "customers/hts/",
      "suffix": ".json"
    }
  ]
}
```

### 2. Web Interface

Open `html/index.html` in your browser (locally) or access the deployed site:

1. **Select customers**: Check boxes for affected customers
2. **Fill details**: Add change title, implementation plan, schedule
3. **Submit**: Watch real-time upload progress
4. **Monitor**: See success/failure for each upload
5. **Retry**: Use retry button for any failures

## 🔧 CLI Usage

### SQS Message Processing

Process customer-specific SQS messages:

```bash
# Process SQS messages for a customer
./ccoe-customer-contact-manager ses -action process-sqs-message \
  -sqs-queue-url "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications" \
  -customer-code "hts"
```

### Customer Validation

```bash
# Validate customer codes
./ccoe-customer-contact-manager validate-customers \
  -json-metadata "change-metadata.json"

# Extract affected customers
./ccoe-customer-contact-manager extract-customers \
  -json-metadata "change-metadata.json"
```

### S3 Event Configuration

```bash
# Configure S3 events for all customers
./ccoe-customer-contact-manager configure-s3-events \
  -config-file "S3EventConfig.json"

# Test S3 event delivery
./ccoe-customer-contact-manager test-s3-events \
  -customer-code "hts" \
  -test-file "test-metadata.json"
```

## 🧪 Testing & Demos

### Run Demo Applications

```bash
# Multi-customer upload demo
go run demo_multi_customer_upload.go

# Integration testing demo  
go run multi_customer_integration_test.go

# Comprehensive validation tests
go run multi_customer_upload_validation.go
```

### Test Results
All demos include comprehensive testing:
- ✅ Customer determination logic
- ✅ Upload queue creation  
- ✅ Progress indicators
- ✅ Error handling with retry
- ✅ Upload validation
- ✅ Backend-driven cleanup

## 📊 Architecture Flow

```
1. Web Interface → Select multiple customers
2. Form Submit → Generate metadata with customer codes
3. Multi-Upload → Upload to customers/{code}/ + archive/
4. S3 Events → Trigger customer-specific SQS queues
5. SQS Processing → Customer CLI processes embedded metadata
6. Email Delivery → Customer SES sends notifications
```

## 🎯 Key Benefits

- **Perfect Isolation**: Each customer only sees their changes
- **No Single Point of Failure**: Direct S3 → SQS integration
- **Scalable**: Handles 30+ customers efficiently  
- **Cost Effective**: Minimal infrastructure overhead
- **Reliable**: Built-in retry and error handling
- **Real-time Progress**: Visual upload tracking
- **Immediate Cleanup**: Backend deletes trigger files after processing

## 🔍 Troubleshooting

### Common Issues

1. **Upload Failures**: Use retry mechanism in web interface
2. **Invalid Customer Codes**: Check CustomerCodes.json configuration
3. **SQS Permission Issues**: Verify cross-account SQS permissions
4. **S3 Event Configuration**: Use validate-s3-events command

### Debug Commands

```bash
# Validate configuration
./ccoe-customer-contact-manager validate-s3-events -config-file S3EventConfig.json

# Test with dry-run
./ccoe-customer-contact-manager configure-s3-events --dry-run

# Check customer validation
./ccoe-customer-contact-manager validate-customers -json-metadata metadata.json
```

## 📚 Next Steps

1. **Configure your customer codes** in CustomerCodes.json
2. **Set up S3 event notifications** using S3EventConfig.json  
3. **Test with the web interface** using html/index.html (local) or the deployed URL
4. **Run demo applications** to validate functionality
5. **Deploy to production** with proper IAM roles and permissions

For detailed configuration and advanced usage, see the main [README.md](README.md).