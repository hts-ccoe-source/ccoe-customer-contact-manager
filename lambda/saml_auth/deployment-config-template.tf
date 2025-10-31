# Lambda@Edge SAML Authentication - Terraform Configuration Template
# This template provides a complete example for deploying the SAML authentication
# Lambda@Edge function with CloudFront.

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Lambda@Edge functions MUST be created in us-east-1
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}

# Variables for session timeout configuration
variable "session_idle_timeout_ms" {
  description = "Session idle timeout in milliseconds (default: 3 hours)"
  type        = number
  default     = 10800000
}

variable "session_absolute_max_ms" {
  description = "Absolute maximum session duration in milliseconds (default: 12 hours)"
  type        = number
  default     = 43200000
}

variable "session_refresh_threshold_ms" {
  description = "Time before expiration when refresh is triggered in milliseconds (default: 10 minutes)"
  type        = number
  default     = 600000
}

variable "session_cookie_max_age" {
  description = "Browser cookie max-age in seconds (default: 3 hours)"
  type        = number
  default     = 10800
}

# IAM role for Lambda@Edge
resource "aws_iam_role" "lambda_edge_role" {
  provider = aws.us_east_1
  name     = "saml-auth-lambda-edge-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = [
            "lambda.amazonaws.com",
            "edgelambda.amazonaws.com"
          ]
        }
      }
    ]
  })
}

# IAM policy for Lambda@Edge CloudWatch Logs
# Note: Lambda@Edge creates log groups in multiple regions
resource "aws_iam_role_policy" "lambda_edge_logging" {
  provider = aws.us_east_1
  name     = "saml-auth-lambda-edge-logging"
  role     = aws_iam_role.lambda_edge_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
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

# Lambda@Edge function
resource "aws_lambda_function" "saml_auth" {
  provider = aws.us_east_1

  filename         = "${path.module}/lambda-edge-samlify.zip"
  function_name    = "saml-auth-edge"
  role            = aws_iam_role.lambda_edge_role.arn
  handler         = "lambda-edge-samlify.handler"
  runtime         = "nodejs18.x"
  source_code_hash = filebase64sha256("${path.module}/lambda-edge-samlify.zip")
  
  # REQUIRED: Must publish for Lambda@Edge
  publish = true
  
  # Lambda@Edge constraints
  timeout     = 5    # Maximum for viewer-request events
  memory_size = 128  # Minimum sufficient for this function
  
  # Session timeout configuration
  environment {
    variables = {
      SESSION_IDLE_TIMEOUT_MS      = tostring(var.session_idle_timeout_ms)
      SESSION_ABSOLUTE_MAX_MS      = tostring(var.session_absolute_max_ms)
      SESSION_REFRESH_THRESHOLD_MS = tostring(var.session_refresh_threshold_ms)
      SESSION_COOKIE_MAX_AGE       = tostring(var.session_cookie_max_age)
    }
  }

  tags = {
    Name        = "saml-auth-edge"
    Environment = "production"
    ManagedBy   = "terraform"
  }
}

# CloudFront Origin Access Identity (if using S3 origin)
resource "aws_cloudfront_origin_access_identity" "main" {
  comment = "OAI for SAML-protected content"
}

# CloudFront distribution with Lambda@Edge
resource "aws_cloudfront_distribution" "main" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = "SAML-protected CloudFront distribution"
  default_root_object = "index.html"
  price_class         = "PriceClass_100"

  # Example S3 origin
  origin {
    domain_name = "my-bucket.s3.amazonaws.com"
    origin_id   = "S3-my-bucket"

    s3_origin_config {
      origin_access_identity = aws_cloudfront_origin_access_identity.main.cloudfront_access_identity_path
    }
  }

  # Default cache behavior with SAML authentication
  default_cache_behavior {
    target_origin_id       = "S3-my-bucket"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods         = ["GET", "HEAD", "OPTIONS"]
    compress               = true

    # Cache policy
    cache_policy_id = "658327ea-f89d-4fab-a63d-7e88639e58f6" # CachingOptimized

    # REQUIRED: Forward authentication headers to origin
    origin_request_policy_id = "88a5eaf4-2fd4-4709-b370-b4c650ea3fcf" # CORS-S3Origin

    # Lambda@Edge association for SAML authentication
    lambda_function_association {
      event_type   = "viewer-request"
      lambda_arn   = aws_lambda_function.saml_auth.qualified_arn
      include_body = true
    }
  }

  # Viewer certificate
  viewer_certificate {
    cloudfront_default_certificate = true
    # For custom domain:
    # acm_certificate_arn      = aws_acm_certificate.cert.arn
    # ssl_support_method       = "sni-only"
    # minimum_protocol_version = "TLSv1.2_2021"
  }

  # Restrictions
  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  tags = {
    Name        = "saml-protected-distribution"
    Environment = "production"
    ManagedBy   = "terraform"
  }
}

# Outputs
output "lambda_function_arn" {
  description = "ARN of the Lambda@Edge function"
  value       = aws_lambda_function.saml_auth.arn
}

output "lambda_function_qualified_arn" {
  description = "Qualified ARN of the Lambda@Edge function (with version)"
  value       = aws_lambda_function.saml_auth.qualified_arn
}

output "cloudfront_distribution_id" {
  description = "ID of the CloudFront distribution"
  value       = aws_cloudfront_distribution.main.id
}

output "cloudfront_domain_name" {
  description = "Domain name of the CloudFront distribution"
  value       = aws_cloudfront_distribution.main.domain_name
}

output "session_configuration" {
  description = "Active session timeout configuration"
  value = {
    idle_timeout_hours    = var.session_idle_timeout_ms / 3600000
    absolute_max_hours    = var.session_absolute_max_ms / 3600000
    refresh_threshold_min = var.session_refresh_threshold_ms / 60000
    cookie_max_age_hours  = var.session_cookie_max_age / 3600
  }
}
