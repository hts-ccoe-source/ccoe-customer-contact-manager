# Variables for Multi-Customer Email Distribution Example

variable "aws_region" {
  description = "AWS region for the main resources"
  type        = string
  default     = "us-east-1"
}

# Portal Configuration
variable "portal_domain_names" {
  description = "List of custom domain names for the portal"
  type        = list(string)
  default     = []
}

variable "ssl_certificate_arn" {
  description = "ARN of the SSL certificate in ACM (us-east-1)"
  type        = string
  default     = null
}

variable "create_route53_records" {
  description = "Whether to create Route53 records for custom domains"
  type        = bool
  default     = false
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID for custom domains"
  type        = string
  default     = ""
}

# Identity Center Configuration
variable "identity_center_domain" {
  description = "Identity Center domain for authentication"
  type        = string
}

variable "cookie_domain" {
  description = "Domain for authentication cookies"
  type        = string
}

variable "redirect_uri" {
  description = "OAuth redirect URI for Identity Center"
  type        = string
}

# Security Configuration
variable "geo_restriction_type" {
  description = "Type of geographic restriction (none, whitelist, blacklist)"
  type        = string
  default     = "none"
}

variable "geo_restriction_locations" {
  description = "List of country codes for geographic restrictions"
  type        = list(string)
  default     = []
}

variable "web_acl_id" {
  description = "AWS WAF Web ACL ID to associate with CloudFront"
  type        = string
  default     = ""
}

# Logging Configuration
variable "cloudfront_logging_bucket" {
  description = "S3 bucket for CloudFront access logs"
  type        = string
  default     = ""
}

# Monitoring Configuration
variable "alarm_sns_topic_arns" {
  description = "List of SNS topic ARNs for CloudWatch alarms"
  type        = list(string)
  default     = []
}

variable "notification_emails" {
  description = "List of email addresses for notifications"
  type        = list(string)
  default     = []
}

# Lambda Configuration
variable "enable_lambda_processors" {
  description = "Enable Lambda processors for SQS queues"
  type        = bool
  default     = false
}

# Customer Configuration
variable "customer_configurations" {
  description = "Additional customer-specific configurations"
  type = map(object({
    enable_enhanced_monitoring = bool
    custom_retention_days     = number
    priority_queue_enabled    = bool
  }))
  default = {}
}

# Environment Configuration
variable "enable_development_features" {
  description = "Enable development and testing features"
  type        = bool
  default     = false
}

variable "enable_cost_optimization" {
  description = "Enable cost optimization features"
  type        = bool
  default     = true
}

# Backup and Recovery Configuration
variable "enable_cross_region_backup" {
  description = "Enable cross-region backup for critical resources"
  type        = bool
  default     = false
}

variable "backup_retention_days" {
  description = "Number of days to retain backups"
  type        = number
  default     = 30
}

# Compliance Configuration
variable "enable_compliance_logging" {
  description = "Enable additional logging for compliance requirements"
  type        = bool
  default     = false
}

variable "compliance_log_retention_years" {
  description = "Number of years to retain compliance logs"
  type        = number
  default     = 7
}

# Performance Configuration
variable "enable_performance_insights" {
  description = "Enable performance monitoring and insights"
  type        = bool
  default     = false
}

variable "cloudfront_price_class" {
  description = "CloudFront price class for cost optimization"
  type        = string
  default     = "PriceClass_100"
}