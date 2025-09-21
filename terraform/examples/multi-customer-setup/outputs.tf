# Outputs for Multi-Customer Email Distribution Example

# S3 Metadata Bucket Outputs
output "metadata_bucket_name" {
  description = "Name of the S3 metadata bucket"
  value       = module.metadata_bucket.bucket_id
}

output "metadata_bucket_arn" {
  description = "ARN of the S3 metadata bucket"
  value       = module.metadata_bucket.bucket_arn
}

output "metadata_bucket_domain_name" {
  description = "Domain name of the S3 metadata bucket"
  value       = module.metadata_bucket.bucket_domain_name
}

# CloudFront Distribution Outputs
output "cloudfront_distribution_id" {
  description = "ID of the CloudFront distribution"
  value       = module.cloudfront_portal.distribution_id
}

output "cloudfront_distribution_domain_name" {
  description = "Domain name of the CloudFront distribution"
  value       = module.cloudfront_portal.distribution_domain_name
}

output "portal_urls" {
  description = "URLs for accessing the portal"
  value = length(var.portal_domain_names) > 0 ? [
    for domain in var.portal_domain_names : "https://${domain}"
  ] : ["https://${module.cloudfront_portal.distribution_domain_name}"]
}

# Customer SQS Queue Outputs
output "customer_queue_urls" {
  description = "Map of customer codes to SQS queue URLs"
  value = {
    for customer_code, queue in module.customer_sqs_queues :
    customer_code => queue.queue_url
  }
}

output "customer_queue_arns" {
  description = "Map of customer codes to SQS queue ARNs"
  value = {
    for customer_code, queue in module.customer_sqs_queues :
    customer_code => queue.queue_arn
  }
}

output "customer_dlq_urls" {
  description = "Map of customer codes to dead letter queue URLs"
  value = {
    for customer_code, queue in module.customer_sqs_queues :
    customer_code => queue.dlq_url
  }
}

# IAM Role Outputs
output "orchestrator_role_arn" {
  description = "ARN of the orchestrator IAM role"
  value       = aws_iam_role.orchestrator_role.arn
}

output "orchestrator_role_name" {
  description = "Name of the orchestrator IAM role"
  value       = aws_iam_role.orchestrator_role.name
}

# Lambda@Edge Function Outputs
output "auth_lambda_function_arn" {
  description = "ARN of the authentication Lambda@Edge function"
  value       = module.cloudfront_portal.auth_lambda_qualified_arn
}

output "security_headers_lambda_function_arn" {
  description = "ARN of the security headers Lambda@Edge function"
  value       = module.cloudfront_portal.security_headers_lambda_qualified_arn
}

# KMS Key Outputs
output "kms_key_id" {
  description = "ID of the KMS encryption key"
  value       = aws_kms_key.encryption_key.key_id
}

output "kms_key_arn" {
  description = "ARN of the KMS encryption key"
  value       = aws_kms_key.encryption_key.arn
}

output "kms_key_alias" {
  description = "Alias of the KMS encryption key"
  value       = aws_kms_alias.encryption_key_alias.name
}

# Customer Configuration Outputs
output "customer_configurations" {
  description = "Summary of customer configurations"
  value = {
    for customer_code, customer in local.customers :
    customer_code => {
      name        = customer.name
      account_id  = customer.account_id
      region      = customer.region
      environment = customer.environment
      queue_url   = module.customer_sqs_queues[customer_code].queue_url
      queue_arn   = module.customer_sqs_queues[customer_code].queue_arn
      s3_prefix   = "customers/${customer_code}/"
    }
  }
}

# S3 Event Configuration Outputs
output "s3_event_configurations" {
  description = "S3 event notification configurations for each customer"
  value = {
    for customer_code, queue in module.customer_sqs_queues :
    customer_code => {
      queue_arn     = queue.queue_arn
      events        = ["s3:ObjectCreated:*"]
      filter_prefix = "customers/${customer_code}/"
      filter_suffix = ".json"
    }
  }
}

# Monitoring Outputs
output "cloudwatch_dashboard_urls" {
  description = "URLs for CloudWatch dashboards"
  value = {
    for customer_code, queue in module.customer_sqs_queues :
    customer_code => queue.cloudwatch_dashboard_name != null ?
    "https://${data.aws_region.current.name}.console.aws.amazon.com/cloudwatch/home?region=${data.aws_region.current.name}#dashboards:name=${queue.cloudwatch_dashboard_name}" :
    null
  }
}

output "alarm_configurations" {
  description = "CloudWatch alarm configurations"
  value = {
    for customer_code, queue in module.customer_sqs_queues :
    customer_code => queue.cloudwatch_alarms
  }
}

# Security Configuration Outputs
output "security_configuration" {
  description = "Security configuration summary"
  value = {
    encryption = {
      kms_key_id             = aws_kms_key.encryption_key.key_id
      s3_encryption_enabled  = true
      sqs_encryption_enabled = true
    }
    access_control = {
      cross_account_roles_configured = true
      identity_center_integration    = true
      cloudfront_oac_enabled         = true
    }
    network_security = {
      geo_restrictions = {
        type      = var.geo_restriction_type
        locations = var.geo_restriction_locations
      }
      waf_enabled = var.web_acl_id != ""
    }
  }
}

# Integration Endpoints
output "integration_endpoints" {
  description = "Key integration endpoints and configurations"
  value = {
    portal_url = length(var.portal_domain_names) > 0 ? "https://${var.portal_domain_names[0]}" : "https://${module.cloudfront_portal.distribution_domain_name}"

    metadata_upload_endpoint = "https://${module.metadata_bucket.bucket_regional_domain_name}"

    identity_center_domain = var.identity_center_domain

    customer_queue_endpoints = {
      for customer_code, queue in module.customer_sqs_queues :
      customer_code => queue.queue_url
    }
  }
}

# Deployment Information
output "deployment_info" {
  description = "Deployment information and next steps"
  value = {
    terraform_workspace = terraform.workspace
    aws_region          = data.aws_region.current.name
    aws_account_id      = data.aws_caller_identity.current.account_id

    next_steps = [
      "Configure Identity Center groups and users",
      "Set up customer-specific IAM roles in customer accounts",
      "Deploy SQS processors to customer accounts",
      "Configure DNS records for custom domains (if using)",
      "Set up monitoring and alerting",
      "Test end-to-end workflow with sample metadata"
    ]

    customer_account_setup_required = {
      for customer_code, customer in local.customers :
      customer_code => {
        account_id = customer.account_id
        region     = customer.region
        required_roles = [
          "${local.project_name}-${customer_code}-processor-role"
        ]
        required_policies = [
          "SQS access for queue processing",
          "SES access for email sending",
          "CloudWatch access for logging"
        ]
      }
    }
  }
}