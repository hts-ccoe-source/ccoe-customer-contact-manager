# Outputs for S3 Metadata Bucket Module

output "bucket_id" {
  description = "ID of the S3 bucket"
  value       = aws_s3_bucket.metadata_bucket.id
}

output "bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.metadata_bucket.arn
}

output "bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  value       = aws_s3_bucket.metadata_bucket.bucket_domain_name
}

output "bucket_regional_domain_name" {
  description = "Regional domain name of the S3 bucket"
  value       = aws_s3_bucket.metadata_bucket.bucket_regional_domain_name
}

output "bucket_hosted_zone_id" {
  description = "Hosted zone ID of the S3 bucket"
  value       = aws_s3_bucket.metadata_bucket.hosted_zone_id
}

output "bucket_region" {
  description = "Region of the S3 bucket"
  value       = aws_s3_bucket.metadata_bucket.region
}

output "access_logs_bucket_id" {
  description = "ID of the access logs bucket"
  value       = var.enable_access_logging ? aws_s3_bucket.access_logs_bucket[0].id : null
}

output "access_logs_bucket_arn" {
  description = "ARN of the access logs bucket"
  value       = var.enable_access_logging ? aws_s3_bucket.access_logs_bucket[0].arn : null
}

output "cloudwatch_log_group_name" {
  description = "Name of the CloudWatch log group for S3 access logs"
  value       = var.enable_access_logging ? aws_cloudwatch_log_group.s3_access_logs[0].name : null
}

output "cloudwatch_log_group_arn" {
  description = "ARN of the CloudWatch log group for S3 access logs"
  value       = var.enable_access_logging ? aws_cloudwatch_log_group.s3_access_logs[0].arn : null
}

# Customer-specific outputs
output "customer_prefixes" {
  description = "Map of customer codes to their S3 prefixes"
  value = {
    for customer_code in keys(var.customer_sqs_queues) :
    customer_code => "customers/${customer_code}/"
  }
}

output "archive_prefix" {
  description = "S3 prefix for archive files"
  value       = "archive/"
}

# Lifecycle configuration outputs
output "lifecycle_rules" {
  description = "Summary of lifecycle rules applied to the bucket"
  value = {
    customer_files = {
      archive_after_days = var.archive_after_days
      glacier_after_days = var.glacier_after_days
      delete_after_days  = var.delete_after_days
    }
    archive_files = {
      ia_after_days     = var.archive_retention_ia_days
      glacier_after_days = var.archive_retention_glacier_days
      delete_after_days  = var.archive_retention_delete_days
    }
  }
}

# Security outputs
output "bucket_encryption" {
  description = "Bucket encryption configuration"
  value = {
    algorithm = var.kms_key_id != null ? "aws:kms" : "AES256"
    kms_key_id = var.kms_key_id
  }
}

output "public_access_blocked" {
  description = "Public access block configuration"
  value = {
    block_public_acls       = true
    block_public_policy     = true
    ignore_public_acls      = true
    restrict_public_buckets = true
  }
}

# Monitoring outputs
output "metrics_configurations" {
  description = "S3 metrics configurations"
  value = {
    entire_bucket = "EntireBucket"
    customer_metrics = {
      for customer_code in keys(var.customer_sqs_queues) :
      customer_code => "Customer-${customer_code}"
    }
  }
}

# Event notification outputs
output "event_notifications" {
  description = "S3 event notification configurations"
  value = {
    for customer_code, queue_config in var.customer_sqs_queues :
    customer_code => {
      queue_arn     = queue_config.queue_arn
      events        = ["s3:ObjectCreated:*"]
      filter_prefix = "customers/${customer_code}/"
      filter_suffix = ".json"
    }
  }
}

# Policy outputs
output "bucket_policy_enabled" {
  description = "Whether bucket policy is enabled for cross-account access"
  value       = var.enable_cross_account_access
}

output "cross_account_roles" {
  description = "Cross-account role configurations"
  value = var.enable_cross_account_access ? {
    orchestrator_role = var.orchestrator_role_arn
    customer_roles    = var.customer_role_arns
  } : null
}