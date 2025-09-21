# S3 Metadata Bucket Module
# Creates S3 bucket with lifecycle policies and event notifications for multi-customer email distribution

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# S3 bucket for metadata storage
resource "aws_s3_bucket" "metadata_bucket" {
  bucket        = var.bucket_name
  force_destroy = var.force_destroy

  tags = merge(var.common_tags, {
    Name        = var.bucket_name
    Purpose     = "Multi-customer email distribution metadata"
    Environment = var.environment
  })
}

# Bucket versioning
resource "aws_s3_bucket_versioning" "metadata_bucket_versioning" {
  bucket = aws_s3_bucket.metadata_bucket.id
  versioning_configuration {
    status = var.enable_versioning ? "Enabled" : "Suspended"
  }
}

# Bucket encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "metadata_bucket_encryption" {
  bucket = aws_s3_bucket.metadata_bucket.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = var.kms_key_id != null ? "aws:kms" : "AES256"
      kms_master_key_id = var.kms_key_id
    }
    bucket_key_enabled = var.kms_key_id != null
  }
}

# Block public access
resource "aws_s3_bucket_public_access_block" "metadata_bucket_pab" {
  bucket = aws_s3_bucket.metadata_bucket.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Lifecycle configuration for cleanup
resource "aws_s3_bucket_lifecycle_configuration" "metadata_bucket_lifecycle" {
  bucket = aws_s3_bucket.metadata_bucket.id

  # Archive processed metadata files
  rule {
    id     = "archive-processed-metadata"
    status = "Enabled"

    filter {
      prefix = "customers/"
    }

    transition {
      days          = var.archive_after_days
      storage_class = "STANDARD_IA"
    }

    transition {
      days          = var.glacier_after_days
      storage_class = "GLACIER"
    }

    expiration {
      days = var.delete_after_days
    }

    noncurrent_version_transition {
      noncurrent_days = 30
      storage_class   = "STANDARD_IA"
    }

    noncurrent_version_expiration {
      noncurrent_days = var.delete_noncurrent_after_days
    }
  }

  # Keep archive files longer
  rule {
    id     = "archive-retention"
    status = "Enabled"

    filter {
      prefix = "archive/"
    }

    transition {
      days          = var.archive_retention_ia_days
      storage_class = "STANDARD_IA"
    }

    transition {
      days          = var.archive_retention_glacier_days
      storage_class = "GLACIER"
    }

    expiration {
      days = var.archive_retention_delete_days
    }
  }

  # Clean up incomplete multipart uploads
  rule {
    id     = "cleanup-incomplete-uploads"
    status = "Enabled"

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

# Event notifications for customer prefixes
resource "aws_s3_bucket_notification" "metadata_bucket_notification" {
  bucket = aws_s3_bucket.metadata_bucket.id

  # Dynamic event notifications for each customer
  dynamic "queue" {
    for_each = var.customer_sqs_queues
    content {
      queue_arn     = queue.value.queue_arn
      events        = ["s3:ObjectCreated:*"]
      filter_prefix = "customers/${queue.key}/"
      filter_suffix = ".json"
    }
  }

  depends_on = [aws_s3_bucket.metadata_bucket]
}

# Bucket policy for cross-account access
resource "aws_s3_bucket_policy" "metadata_bucket_policy" {
  count  = var.enable_cross_account_access ? 1 : 0
  bucket = aws_s3_bucket.metadata_bucket.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowOrchestrator"
        Effect = "Allow"
        Principal = {
          AWS = var.orchestrator_role_arn
        }
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.metadata_bucket.arn,
          "${aws_s3_bucket.metadata_bucket.arn}/*"
        ]
      },
      {
        Sid    = "AllowCustomerAccess"
        Effect = "Allow"
        Principal = {
          AWS = var.customer_role_arns
        }
        Action = [
          "s3:GetObject"
        ]
        Resource = "${aws_s3_bucket.metadata_bucket.arn}/customers/$${aws:PrincipalTag/CustomerCode}/*"
        Condition = {
          StringEquals = {
            "s3:ExistingObjectTag/CustomerCode" = "$${aws:PrincipalTag/CustomerCode}"
          }
        }
      }
    ]
  })
}

# CloudWatch log group for S3 access logs
resource "aws_cloudwatch_log_group" "s3_access_logs" {
  count             = var.enable_access_logging ? 1 : 0
  name              = "/aws/s3/${var.bucket_name}/access-logs"
  retention_in_days = var.log_retention_days

  tags = var.common_tags
}

# S3 bucket for access logs (if enabled)
resource "aws_s3_bucket" "access_logs_bucket" {
  count         = var.enable_access_logging ? 1 : 0
  bucket        = "${var.bucket_name}-access-logs"
  force_destroy = var.force_destroy

  tags = merge(var.common_tags, {
    Name    = "${var.bucket_name}-access-logs"
    Purpose = "S3 access logs for metadata bucket"
  })
}

resource "aws_s3_bucket_logging" "metadata_bucket_logging" {
  count  = var.enable_access_logging ? 1 : 0
  bucket = aws_s3_bucket.metadata_bucket.id

  target_bucket = aws_s3_bucket.access_logs_bucket[0].id
  target_prefix = "access-logs/"
}

# Bucket metrics configuration
resource "aws_s3_bucket_metric" "metadata_bucket_metrics" {
  bucket = aws_s3_bucket.metadata_bucket.id
  name   = "EntireBucket"
}

# Customer-specific metrics
resource "aws_s3_bucket_metric" "customer_metrics" {
  for_each = var.customer_sqs_queues
  bucket   = aws_s3_bucket.metadata_bucket.id
  name     = "Customer-${each.key}"

  filter {
    prefix = "customers/${each.key}/"
  }
}

# Bucket inventory configuration
resource "aws_s3_bucket_inventory" "metadata_bucket_inventory" {
  count  = var.enable_inventory ? 1 : 0
  bucket = aws_s3_bucket.metadata_bucket.id
  name   = "EntireBucketDaily"

  included_object_versions = "All"

  schedule {
    frequency = "Daily"
  }

  destination {
    bucket {
      format     = "CSV"
      bucket_arn = aws_s3_bucket.metadata_bucket.arn
      prefix     = "inventory/"
      encryption {
        sse_s3 {}
      }
    }
  }

  optional_fields = [
    "Size",
    "LastModifiedDate",
    "StorageClass",
    "ETag",
    "IsMultipartUploaded",
    "ReplicationStatus",
    "EncryptionStatus",
    "ObjectLockRetainUntilDate",
    "ObjectLockMode",
    "ObjectLockLegalHoldStatus",
    "IntelligentTieringAccessTier"
  ]
}