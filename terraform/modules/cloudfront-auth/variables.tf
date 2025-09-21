# Variables for CloudFront + Lambda@Edge Authentication Module

variable "distribution_name" {
  description = "Name for the CloudFront distribution"
  type        = string
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "common_tags" {
  description = "Common tags to apply to all resources"
  type        = map(string)
  default     = {}
}

# S3 Origin Configuration
variable "s3_bucket_name" {
  description = "Name of the S3 bucket serving as origin"
  type        = string
}

variable "s3_bucket_arn" {
  description = "ARN of the S3 bucket serving as origin"
  type        = string
}

variable "s3_bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  type        = string
}

variable "origin_verify_header" {
  description = "Custom header value for origin verification"
  type        = string
  default     = "cloudfront-origin-verify"
  sensitive   = true
}

# Domain Configuration
variable "domain_names" {
  description = "List of domain names for the CloudFront distribution"
  type        = list(string)
  default     = []
}

variable "ssl_certificate_arn" {
  description = "ARN of the SSL certificate in ACM (us-east-1)"
  type        = string
  default     = null
}

variable "default_root_object" {
  description = "Default root object for the distribution"
  type        = string
  default     = "index.html"
}

# Route53 Configuration
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

# Cache Configuration
variable "default_cache_ttl" {
  description = "Default TTL for cached objects (seconds)"
  type        = number
  default     = 86400 # 1 day
}

variable "max_cache_ttl" {
  description = "Maximum TTL for cached objects (seconds)"
  type        = number
  default     = 31536000 # 1 year
}

variable "static_cache_ttl" {
  description = "TTL for static assets (seconds)"
  type        = number
  default     = 604800 # 1 week
}

variable "static_max_cache_ttl" {
  description = "Maximum TTL for static assets (seconds)"
  type        = number
  default     = 31536000 # 1 year
}

variable "price_class" {
  description = "CloudFront price class"
  type        = string
  default     = "PriceClass_100"
  validation {
    condition = contains([
      "PriceClass_All",
      "PriceClass_200",
      "PriceClass_100"
    ], var.price_class)
    error_message = "Price class must be PriceClass_All, PriceClass_200, or PriceClass_100."
  }
}

# Geographic Restrictions
variable "geo_restriction_type" {
  description = "Type of geographic restriction (none, whitelist, blacklist)"
  type        = string
  default     = "none"
  validation {
    condition = contains([
      "none",
      "whitelist",
      "blacklist"
    ], var.geo_restriction_type)
    error_message = "Geo restriction type must be none, whitelist, or blacklist."
  }
}

variable "geo_restriction_locations" {
  description = "List of country codes for geographic restrictions"
  type        = list(string)
  default     = []
}

# Custom Error Responses
variable "custom_error_responses" {
  description = "List of custom error response configurations"
  type = list(object({
    error_code            = number
    response_code         = number
    response_page_path    = string
    error_caching_min_ttl = number
  }))
  default = [
    {
      error_code            = 403
      response_code         = 200
      response_page_path    = "/error.html"
      error_caching_min_ttl = 300
    },
    {
      error_code            = 404
      response_code         = 200
      response_page_path    = "/error.html"
      error_caching_min_ttl = 300
    }
  ]
}

# Logging Configuration
variable "logging_bucket" {
  description = "S3 bucket for CloudFront access logs"
  type        = string
  default     = ""
}

variable "logging_prefix" {
  description = "Prefix for CloudFront access logs"
  type        = string
  default     = "cloudfront-logs/"
}

# Identity Center Configuration
variable "identity_center_domain" {
  description = "Identity Center domain for authentication"
  type        = string
}

variable "allowed_groups" {
  description = "List of Identity Center groups allowed to access the portal"
  type        = list(string)
  default     = ["ChangeManagers", "CustomerManagers"]
}

variable "session_duration" {
  description = "Session duration in seconds"
  type        = number
  default     = 3600 # 1 hour
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
variable "content_security_policy" {
  description = "Content Security Policy header value"
  type        = string
  default     = "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https:; frame-ancestors 'none';"
}

variable "hsts_max_age" {
  description = "HSTS max-age value in seconds"
  type        = number
  default     = 31536000 # 1 year
}

# Lambda@Edge Configuration
variable "lambda_memory_size" {
  description = "Memory size for Lambda@Edge functions"
  type        = number
  default     = 128
}

variable "lambda_timeout" {
  description = "Timeout for Lambda@Edge functions"
  type        = number
  default     = 5
}

# Monitoring Configuration
variable "enable_real_time_logs" {
  description = "Enable CloudFront real-time logs"
  type        = bool
  default     = false
}

variable "real_time_log_config_arn" {
  description = "ARN of the real-time log configuration"
  type        = string
  default     = ""
}

# WAF Configuration
variable "web_acl_id" {
  description = "AWS WAF Web ACL ID to associate with the distribution"
  type        = string
  default     = ""
}

# Origin Shield Configuration
variable "enable_origin_shield" {
  description = "Enable CloudFront Origin Shield"
  type        = bool
  default     = false
}

variable "origin_shield_region" {
  description = "AWS region for Origin Shield"
  type        = string
  default     = "us-east-1"
}