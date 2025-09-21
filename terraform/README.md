# Multi-Customer Email Distribution - Terraform Infrastructure

This directory contains Terraform modules and examples for deploying the multi-customer email distribution system infrastructure on AWS.

## Architecture Overview

The infrastructure consists of several key components:

1. **S3 Metadata Bucket** - Centralized storage for email metadata with lifecycle policies and event notifications
2. **CloudFront + Lambda@Edge** - Secure web portal with Identity Center authentication
3. **SQS Change Queues** - Customer-specific queues for change notifications (deployed to customer accounts)
4. **Cross-Account IAM Roles** - Secure access between orchestrator and customer accounts
5. **Monitoring and Alerting** - CloudWatch dashboards, alarms, and SNS notifications

## Directory Structure

```
terraform/
├── modules/                    # Reusable Terraform modules
│   ├── s3-metadata-bucket/    # S3 bucket with lifecycle and notifications
│   ├── cloudfront-auth/       # CloudFront with Lambda@Edge authentication
│   └── sqs-change-queue/      # SQS queues for customer accounts
├── examples/                  # Example configurations
│   └── multi-customer-setup/  # Complete multi-customer setup
└── README.md                  # This file
```

## Modules

### S3 Metadata Bucket Module

Creates an S3 bucket for storing email metadata with:
- Lifecycle policies for automatic archiving and cleanup
- Event notifications to customer SQS queues
- Cross-account access policies
- Encryption and security controls
- CloudWatch metrics and monitoring

**Usage:**
```hcl
module "metadata_bucket" {
  source = "./modules/s3-metadata-bucket"
  
  bucket_name     = "my-email-metadata-bucket"
  environment     = "prod"
  enable_versioning = true
  
  customer_sqs_queues = {
    customer1 = {
      queue_arn = "arn:aws:sqs:us-east-1:123456789012:customer1-queue"
      queue_url = "https://sqs.us-east-1.amazonaws.com/123456789012/customer1-queue"
    }
  }
  
  common_tags = {
    Project = "multi-customer-email"
    Environment = "prod"
  }
}
```

### CloudFront + Lambda@Edge Authentication Module

Creates a CloudFront distribution with Lambda@Edge functions for:
- Identity Center authentication
- Security headers injection
- Origin access control for S3
- Custom domain support with SSL
- Geographic restrictions and WAF integration

**Usage:**
```hcl
module "cloudfront_portal" {
  source = "./modules/cloudfront-auth"
  
  distribution_name       = "email-portal"
  s3_bucket_name         = "my-portal-bucket"
  s3_bucket_arn          = "arn:aws:s3:::my-portal-bucket"
  s3_bucket_domain_name  = "my-portal-bucket.s3.amazonaws.com"
  
  identity_center_domain = "my-org.awsapps.com"
  allowed_groups        = ["ChangeManagers", "CustomerManagers"]
  cookie_domain         = ".example.com"
  redirect_uri          = "https://portal.example.com/auth/callback"
  
  common_tags = {
    Project = "multi-customer-email"
  }
}
```

### SQS Change Queue Module

Creates SQS queues in customer accounts for:
- Change notification processing
- Dead letter queue handling
- Cross-account access from orchestrator
- CloudWatch monitoring and alarms
- Optional Lambda processor integration

**Usage:**
```hcl
module "customer_queue" {
  source = "./modules/sqs-change-queue"
  
  queue_name    = "customer1-change-queue"
  customer_code = "customer1"
  environment   = "prod"
  
  orchestrator_role_arn       = "arn:aws:iam::999999999999:role/orchestrator-role"
  customer_processor_role_arn = "arn:aws:iam::123456789012:role/processor-role"
  orchestrator_account_id     = "999999999999"
  s3_bucket_arn              = "arn:aws:s3:::metadata-bucket"
  
  enable_cloudwatch_alarms = true
  enable_lambda_processor  = true
  
  common_tags = {
    Customer = "customer1"
  }
}
```

## Example Deployment

The `examples/multi-customer-setup` directory contains a complete example that demonstrates how to use all modules together for a production deployment.

### Prerequisites

1. **AWS CLI configured** with appropriate permissions
2. **Terraform >= 1.0** installed
3. **Identity Center configured** in your AWS organization
4. **Customer AWS accounts** set up and accessible
5. **Domain and SSL certificate** (optional, for custom domains)

### Deployment Steps

1. **Clone and navigate to the example:**
   ```bash
   cd terraform/examples/multi-customer-setup
   ```

2. **Create terraform.tfvars:**
   ```hcl
   aws_region = "us-east-1"
   
   # Identity Center configuration
   identity_center_domain = "your-org.awsapps.com"
   cookie_domain         = ".your-domain.com"
   redirect_uri          = "https://portal.your-domain.com/auth/callback"
   
   # Optional: Custom domain
   portal_domain_names     = ["portal.your-domain.com"]
   ssl_certificate_arn     = "arn:aws:acm:us-east-1:123456789012:certificate/..."
   create_route53_records  = true
   route53_zone_id        = "Z1234567890ABC"
   
   # Monitoring
   notification_emails = ["admin@your-domain.com"]
   alarm_sns_topic_arns = ["arn:aws:sns:us-east-1:123456789012:alerts"]
   ```

3. **Initialize and plan:**
   ```bash
   terraform init
   terraform plan
   ```

4. **Deploy infrastructure:**
   ```bash
   terraform apply
   ```

5. **Configure customer accounts:**
   - Deploy SQS queues to customer accounts using the SQS module
   - Set up IAM roles for cross-account access
   - Configure SES for email sending in customer accounts

## Security Considerations

### Encryption
- All S3 buckets use server-side encryption (SSE-S3 or SSE-KMS)
- SQS queues use KMS encryption for messages
- CloudFront uses TLS 1.2+ for all connections

### Access Control
- Cross-account access uses IAM roles with least privilege
- S3 bucket policies restrict access by customer code
- SQS queue policies allow only specific operations from specific roles
- Lambda@Edge functions validate Identity Center group membership

### Network Security
- CloudFront Origin Access Control prevents direct S3 access
- Security headers protect against common web vulnerabilities
- Optional geographic restrictions and WAF integration
- VPC endpoints can be used for private connectivity

### Monitoring and Auditing
- CloudTrail logs all API calls
- CloudWatch alarms monitor queue depths and message ages
- S3 access logging tracks all bucket operations
- Lambda@Edge logs authentication events

## Cost Optimization

### S3 Storage Classes
- Automatic transition to Standard-IA after 30 days
- Glacier storage for long-term archival
- Intelligent Tiering for unpredictable access patterns

### CloudFront
- Price Class 100 for cost-effective global distribution
- Origin Shield for reduced origin requests
- Compression enabled for bandwidth savings

### SQS and Lambda
- Long polling reduces empty receive requests
- Batch processing in Lambda reduces invocation costs
- Dead letter queues prevent infinite retry costs

## Monitoring and Troubleshooting

### CloudWatch Dashboards
Each customer gets a dedicated dashboard showing:
- Queue depth and message age
- Processing rates and error rates
- Lambda function metrics (if enabled)

### Alarms and Notifications
- Queue depth exceeds threshold
- Messages in dead letter queue
- Old messages not being processed
- Lambda function errors

### Common Issues

1. **Cross-account access denied:**
   - Verify IAM roles exist in customer accounts
   - Check role trust policies allow orchestrator account
   - Ensure role policies grant required permissions

2. **S3 event notifications not working:**
   - Verify SQS queue policies allow S3 service
   - Check S3 bucket notification configuration
   - Ensure queue ARNs are correct

3. **Authentication failures:**
   - Verify Identity Center domain and groups
   - Check Lambda@Edge function logs in CloudWatch
   - Ensure redirect URI matches configuration

4. **Email delivery issues:**
   - Check SES configuration in customer accounts
   - Verify SES sending limits and reputation
   - Review bounce and complaint handling

## Maintenance

### Regular Tasks
- Review and update lifecycle policies
- Monitor costs and optimize storage classes
- Update Lambda@Edge functions for security patches
- Review and rotate access keys/roles

### Scaling Considerations
- S3 can handle unlimited objects and requests
- SQS scales automatically with demand
- CloudFront scales globally automatically
- Lambda@Edge has regional limits to consider

### Backup and Recovery
- S3 versioning provides object-level backup
- Cross-region replication for disaster recovery
- Infrastructure as Code enables rapid rebuilding
- Regular testing of recovery procedures

## Support and Contributing

For issues and questions:
1. Check the troubleshooting section above
2. Review AWS service documentation
3. Check Terraform module documentation
4. Contact the platform team

When contributing:
1. Follow Terraform best practices
2. Update documentation for any changes
3. Test changes in development environment
4. Follow security review process