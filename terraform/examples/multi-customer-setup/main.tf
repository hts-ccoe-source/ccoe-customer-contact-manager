# Example Multi-Customer Email Distribution Setup
# This example shows how to use the Terraform modules together

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Configure AWS providers
provider "aws" {
  region = var.aws_region
}

provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1" # Required for Lambda@Edge
}

# Local values for configuration
locals {
  environment = "prod"
  project_name = "multi-customer-email"
  
  common_tags = {
    Project     = local.project_name
    Environment = local.environment
    ManagedBy   = "terraform"
    Owner       = "platform-team"
  }
  
  # Customer configuration
  customers = {
    hts = {
      name         = "HTS Customer"
      account_id   = "123456789012"
      region       = "us-east-1"
      environment  = "prod"
    }
    cds = {
      name         = "CDS Customer"
      account_id   = "234567890123"
      region       = "us-west-2"
      environment  = "prod"
    }
    motor = {
      name         = "Motor Customer"
      account_id   = "345678901234"
      region       = "us-east-1"
      environment  = "prod"
    }
  }
}

# S3 bucket for metadata storage
module "metadata_bucket" {
  source = "../../modules/s3-metadata-bucket"
  
  bucket_name     = "${local.project_name}-metadata-${local.environment}"
  environment     = local.environment
  force_destroy   = false
  enable_versioning = true
  
  # Lifecycle configuration
  archive_after_days  = 30
  glacier_after_days  = 90
  delete_after_days   = 2555 # ~7 years
  
  # Archive retention (longer for compliance)
  archive_retention_ia_days     = 90
  archive_retention_glacier_days = 365
  archive_retention_delete_days  = 3650 # 10 years
  
  # Customer SQS queues for event notifications
  customer_sqs_queues = {
    for customer_code, customer in local.customers :
    customer_code => {
      queue_arn = "arn:aws:sqs:${customer.region}:${customer.account_id}:${local.project_name}-${customer_code}-queue"
      queue_url = "https://sqs.${customer.region}.amazonaws.com/${customer.account_id}/${local.project_name}-${customer_code}-queue"
    }
  }
  
  # Cross-account access
  enable_cross_account_access = true
  orchestrator_role_arn      = aws_iam_role.orchestrator_role.arn
  customer_role_arns = [
    for customer_code, customer in local.customers :
    "arn:aws:iam::${customer.account_id}:role/${local.project_name}-${customer_code}-processor-role"
  ]
  
  # Monitoring and logging
  enable_access_logging = true
  log_retention_days   = 90
  enable_inventory     = true
  
  common_tags = local.common_tags
}

# CloudFront distribution with authentication
module "cloudfront_portal" {
  source = "../../modules/cloudfront-auth"
  
  providers = {
    aws.us_east_1 = aws.us_east_1
  }
  
  distribution_name       = "${local.project_name}-portal-${local.environment}"
  environment            = local.environment
  s3_bucket_name         = module.metadata_bucket.bucket_id
  s3_bucket_arn          = module.metadata_bucket.bucket_arn
  s3_bucket_domain_name  = module.metadata_bucket.bucket_regional_domain_name
  
  # Custom domain configuration
  domain_names           = var.portal_domain_names
  ssl_certificate_arn    = var.ssl_certificate_arn
  create_route53_records = var.create_route53_records
  route53_zone_id        = var.route53_zone_id
  
  # Cache configuration
  default_cache_ttl     = 86400    # 1 day
  max_cache_ttl         = 31536000 # 1 year
  static_cache_ttl      = 604800   # 1 week
  static_max_cache_ttl  = 31536000 # 1 year
  
  # Identity Center configuration
  identity_center_domain = var.identity_center_domain
  allowed_groups = [
    "ChangeManagers",
    "CustomerManagers-HTS",
    "CustomerManagers-CDS",
    "CustomerManagers-Motor",
    "Auditors"
  ]
  session_duration = 3600 # 1 hour
  cookie_domain    = var.cookie_domain
  redirect_uri     = var.redirect_uri
  
  # Security configuration
  content_security_policy = "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: https:; font-src 'self' data: https://fonts.gstatic.com; connect-src 'self' https:; frame-ancestors 'none';"
  hsts_max_age           = 31536000 # 1 year
  
  # Geographic restrictions (optional)
  geo_restriction_type      = var.geo_restriction_type
  geo_restriction_locations = var.geo_restriction_locations
  
  # Logging
  logging_bucket = var.cloudfront_logging_bucket
  logging_prefix = "cloudfront-logs/"
  
  # WAF integration (optional)
  web_acl_id = var.web_acl_id
  
  common_tags = local.common_tags
}

# SQS queues for each customer (deployed to customer accounts)
module "customer_sqs_queues" {
  source = "../../modules/sqs-change-queue"
  
  for_each = local.customers
  
  # Note: This would typically be deployed to customer accounts
  # using separate Terraform configurations with cross-account roles
  
  queue_name    = "${local.project_name}-${each.key}-queue"
  customer_code = each.key
  environment   = local.environment
  
  # Queue configuration
  visibility_timeout_seconds = 300  # 5 minutes
  message_retention_seconds  = 1209600 # 14 days
  receive_wait_time_seconds  = 20   # Enable long polling
  
  # Dead letter queue
  enable_dlq        = true
  max_receive_count = 3
  
  # Encryption
  kms_key_id = "alias/aws/sqs"
  
  # Cross-account access
  orchestrator_role_arn       = aws_iam_role.orchestrator_role.arn
  customer_processor_role_arn = "arn:aws:iam::${each.value.account_id}:role/${local.project_name}-${each.key}-processor-role"
  orchestrator_account_id     = data.aws_caller_identity.current.account_id
  s3_bucket_arn              = module.metadata_bucket.bucket_arn
  
  # Monitoring
  enable_cloudwatch_alarms    = true
  queue_depth_alarm_threshold = 100
  message_age_alarm_threshold = 3600 # 1 hour
  alarm_actions              = var.alarm_sns_topic_arns
  
  # SNS notifications
  enable_sns_notifications = true
  notification_emails     = var.notification_emails
  
  # Lambda processor (optional)
  enable_lambda_processor = var.enable_lambda_processors
  lambda_timeout         = 300
  lambda_memory_size     = 512
  lambda_batch_size      = 10
  
  common_tags = local.common_tags
}

# IAM role for orchestrator service
resource "aws_iam_role" "orchestrator_role" {
  name = "${local.project_name}-orchestrator-role"
  
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      },
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
  
  tags = local.common_tags
}

# IAM policy for orchestrator role
resource "aws_iam_role_policy" "orchestrator_policy" {
  name = "${local.project_name}-orchestrator-policy"
  role = aws_iam_role.orchestrator_role.id
  
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          module.metadata_bucket.bucket_arn,
          "${module.metadata_bucket.bucket_arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "sqs:SendMessage",
          "sqs:GetQueueAttributes",
          "sqs:GetQueueUrl"
        ]
        Resource = [
          for queue in module.customer_sqs_queues :
          queue.queue_arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "sts:AssumeRole"
        ]
        Resource = [
          for customer_code, customer in local.customers :
          "arn:aws:iam::${customer.account_id}:role/${local.project_name}-${customer_code}-*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:*"
      }
    ]
  })
}

# KMS key for encryption
resource "aws_kms_key" "encryption_key" {
  description             = "KMS key for ${local.project_name} encryption"
  deletion_window_in_days = 7
  
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "Enable IAM User Permissions"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        }
        Action   = "kms:*"
        Resource = "*"
      },
      {
        Sid    = "Allow use of the key"
        Effect = "Allow"
        Principal = {
          AWS = [
            aws_iam_role.orchestrator_role.arn,
            module.cloudfront_portal.lambda_edge_role_arn
          ]
        }
        Action = [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey"
        ]
        Resource = "*"
      }
    ]
  })
  
  tags = local.common_tags
}

resource "aws_kms_alias" "encryption_key_alias" {
  name          = "alias/${local.project_name}-${local.environment}"
  target_key_id = aws_kms_key.encryption_key.key_id
}

# Data sources
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}