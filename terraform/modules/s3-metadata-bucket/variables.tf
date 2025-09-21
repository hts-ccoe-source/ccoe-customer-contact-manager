# Variables for S3 Metadata Bucket Module

variable "bucket_name" {
  description = "Name of the S3 bucket for metadata storage"
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]*[a-z0-9]$", var.bucket_name))
    error_message = "Bucket name must be lowercase alphanumeric with hyphens, starting and ending with alphanumeric characters."
  }
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "force_destroy" {
  description = "Allow Terraform to destroy the bucket even if it contains objects"
  type        = bool
  default     = false
}

variable "enable_versioning" {
  description = "Enable S3 bucket versioning"
  type        = bool
  default     = true
}

variable "kms_key_id" {
  description = "KMS key ID for bucket encryption (optional)"
  type        = string
  default     = null
}

variable "common_tags" {
  description = "Common tags to apply to all resources"
  type        = map(string)
  default     = {}
}

# Lifecycle configuration variables
variable "archive_after_days" {
  description = "Number of days after which to move objects to Standard-IA"
  type        = number
  default     = 30
}

variable "glacier_after_days" {
  description = "Number of days after which to move objects to Glacier"
  type        = number
  default     = 90
}

variable "delete_after_days" {
  description = "Number of days after which to delete objects"
  type        = number
  default     = 2555 # ~7 years
}

variable "delete_noncurrent_after_days" {
  description = "Number of days after which to delete noncurrent versions"
  type        = number
  default     = 90
}

variable "archive_retention_ia_days" {
  description = "Days to move archive files to Standard-IA"
  type        = number
  default     = 90
}

variable "archive_retention_glacier_days" {
  description = "Days to move archive files to Glacier"
  type        = number
  default     = 365
}

variable "archive_retention_delete_days" {
  description = "Days to delete archive files"
  type        = number
  default     = 3650 # 10 years
}

# Event notification variables
variable "customer_sqs_queues" {
  description = "Map of customer codes to SQS queue configurations"
  type = map(object({
    queue_arn = string
    queue_url = string
  }))
  default = {}
}

# Cross-account access variables
variable "enable_cross_account_access" {
  description = "Enable cross-account access policies"
  type        = bool
  default     = false
}

variable "orchestrator_role_arn" {
  description = "ARN of the orchestrator role for cross-account access"
  type        = string
  default     = ""
}

variable "customer_role_arns" {
  description = "List of customer role ARNs for cross-account access"
  type        = list(string)
  default     = []
}

# Logging variables
variable "enable_access_logging" {
  description = "Enable S3 access logging"
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 90
}

# Monitoring variables
variable "enable_inventory" {
  description = "Enable S3 inventory configuration"
  type        = bool
  default     = false
}

# Security variables
variable "enable_mfa_delete" {
  description = "Enable MFA delete (requires root account)"
  type        = bool
  default     = false
}

variable "enable_object_lock" {
  description = "Enable S3 object lock"
  type        = bool
  default     = false
}

variable "object_lock_retention_days" {
  description = "Object lock retention period in days"
  type        = number
  default     = 365
}

# Performance variables
variable "enable_transfer_acceleration" {
  description = "Enable S3 transfer acceleration"
  type        = bool
  default     = false
}

variable "enable_intelligent_tiering" {
  description = "Enable S3 intelligent tiering"
  type        = bool
  default     = false
}

# Replication variables
variable "enable_replication" {
  description = "Enable cross-region replication"
  type        = bool
  default     = false
}

variable "replication_destination_bucket" {
  description = "Destination bucket for replication"
  type        = string
  default     = ""
}

variable "replication_destination_region" {
  description = "Destination region for replication"
  type        = string
  default     = ""
}

variable "replication_role_arn" {
  description = "IAM role ARN for replication"
  type        = string
  default     = ""
}