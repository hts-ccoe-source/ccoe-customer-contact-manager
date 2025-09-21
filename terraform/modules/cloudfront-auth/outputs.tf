# Outputs for CloudFront + Lambda@Edge Authentication Module

output "distribution_id" {
  description = "ID of the CloudFront distribution"
  value       = aws_cloudfront_distribution.portal_distribution.id
}

output "distribution_arn" {
  description = "ARN of the CloudFront distribution"
  value       = aws_cloudfront_distribution.portal_distribution.arn
}

output "distribution_domain_name" {
  description = "Domain name of the CloudFront distribution"
  value       = aws_cloudfront_distribution.portal_distribution.domain_name
}

output "distribution_hosted_zone_id" {
  description = "CloudFront hosted zone ID"
  value       = aws_cloudfront_distribution.portal_distribution.hosted_zone_id
}

output "distribution_status" {
  description = "Status of the CloudFront distribution"
  value       = aws_cloudfront_distribution.portal_distribution.status
}

# Lambda@Edge Function Outputs
output "auth_lambda_function_name" {
  description = "Name of the authentication Lambda@Edge function"
  value       = aws_lambda_function.auth_lambda.function_name
}

output "auth_lambda_function_arn" {
  description = "ARN of the authentication Lambda@Edge function"
  value       = aws_lambda_function.auth_lambda.arn
}

output "auth_lambda_qualified_arn" {
  description = "Qualified ARN of the authentication Lambda@Edge function"
  value       = aws_lambda_function.auth_lambda.qualified_arn
}

output "security_headers_lambda_function_name" {
  description = "Name of the security headers Lambda@Edge function"
  value       = aws_lambda_function.security_headers_lambda.function_name
}

output "security_headers_lambda_function_arn" {
  description = "ARN of the security headers Lambda@Edge function"
  value       = aws_lambda_function.security_headers_lambda.arn
}

output "security_headers_lambda_qualified_arn" {
  description = "Qualified ARN of the security headers Lambda@Edge function"
  value       = aws_lambda_function.security_headers_lambda.qualified_arn
}

# IAM Role Outputs
output "lambda_edge_role_arn" {
  description = "ARN of the Lambda@Edge execution role"
  value       = aws_iam_role.lambda_edge_role.arn
}

output "lambda_edge_role_name" {
  description = "Name of the Lambda@Edge execution role"
  value       = aws_iam_role.lambda_edge_role.name
}

# Origin Access Control Outputs
output "origin_access_control_id" {
  description = "ID of the CloudFront Origin Access Control"
  value       = aws_cloudfront_origin_access_control.s3_oac.id
}

output "origin_access_control_etag" {
  description = "ETag of the CloudFront Origin Access Control"
  value       = aws_cloudfront_origin_access_control.s3_oac.etag
}

# Domain and SSL Outputs
output "custom_domain_names" {
  description = "List of custom domain names configured"
  value       = var.domain_names
}

output "ssl_certificate_arn" {
  description = "ARN of the SSL certificate used"
  value       = var.ssl_certificate_arn
}

# Route53 Outputs
output "route53_records_created" {
  description = "Whether Route53 records were created"
  value       = var.create_route53_records
}

output "route53_zone_id" {
  description = "Route53 hosted zone ID used"
  value       = var.route53_zone_id
}

# Cache Configuration Outputs
output "cache_behaviors" {
  description = "Summary of cache behaviors configured"
  value = {
    default = {
      target_origin_id       = "S3-${var.s3_bucket_name}"
      viewer_protocol_policy = "redirect-to-https"
      default_ttl           = var.default_cache_ttl
      max_ttl               = var.max_cache_ttl
    }
    api = {
      path_pattern          = "/api/*"
      viewer_protocol_policy = "redirect-to-https"
      default_ttl           = 0
      max_ttl               = 0
    }
    static = {
      path_pattern          = "/static/*"
      viewer_protocol_policy = "redirect-to-https"
      default_ttl           = var.static_cache_ttl
      max_ttl               = var.static_max_cache_ttl
    }
  }
}

# Security Configuration Outputs
output "security_headers_configured" {
  description = "List of security headers configured"
  value = [
    "Content-Security-Policy",
    "Strict-Transport-Security",
    "X-Frame-Options",
    "X-Content-Type-Options",
    "X-XSS-Protection",
    "Referrer-Policy",
    "Permissions-Policy",
    "Cross-Origin-Embedder-Policy",
    "Cross-Origin-Opener-Policy",
    "Cross-Origin-Resource-Policy"
  ]
}

output "geo_restrictions" {
  description = "Geographic restriction configuration"
  value = {
    restriction_type = var.geo_restriction_type
    locations       = var.geo_restriction_locations
  }
}

# Authentication Configuration Outputs
output "identity_center_domain" {
  description = "Identity Center domain configured"
  value       = var.identity_center_domain
}

output "allowed_groups" {
  description = "List of allowed Identity Center groups"
  value       = var.allowed_groups
}

output "session_duration" {
  description = "Configured session duration in seconds"
  value       = var.session_duration
}

output "cookie_domain" {
  description = "Domain configured for authentication cookies"
  value       = var.cookie_domain
}

output "redirect_uri" {
  description = "OAuth redirect URI configured"
  value       = var.redirect_uri
}

# Monitoring and Logging Outputs
output "logging_configuration" {
  description = "CloudFront logging configuration"
  value = var.logging_bucket != "" ? {
    bucket = var.logging_bucket
    prefix = var.logging_prefix
  } : null
}

output "real_time_logs_enabled" {
  description = "Whether real-time logs are enabled"
  value       = var.enable_real_time_logs
}

# Performance Outputs
output "price_class" {
  description = "CloudFront price class configured"
  value       = var.price_class
}

output "origin_shield_enabled" {
  description = "Whether Origin Shield is enabled"
  value       = var.enable_origin_shield
}

output "origin_shield_region" {
  description = "Origin Shield region configured"
  value       = var.enable_origin_shield ? var.origin_shield_region : null
}

# Error Handling Outputs
output "custom_error_responses" {
  description = "Custom error response configurations"
  value       = var.custom_error_responses
}

# WAF Integration Output
output "web_acl_id" {
  description = "AWS WAF Web ACL ID associated with the distribution"
  value       = var.web_acl_id != "" ? var.web_acl_id : null
}