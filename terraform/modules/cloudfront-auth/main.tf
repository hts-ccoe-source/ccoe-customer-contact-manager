# CloudFront + Lambda@Edge Module for Identity Center Authentication
# Provides secure access to the multi-customer email distribution portal

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# CloudFront Origin Access Control for S3
resource "aws_cloudfront_origin_access_control" "s3_oac" {
  name                              = "${var.distribution_name}-s3-oac"
  description                       = "OAC for ${var.distribution_name} S3 origin"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# CloudFront distribution
resource "aws_cloudfront_distribution" "portal_distribution" {
  origin {
    domain_name              = var.s3_bucket_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.s3_oac.id
    origin_id                = "S3-${var.s3_bucket_name}"

    # Custom headers for Lambda@Edge
    custom_header {
      name  = "X-Origin-Verify"
      value = var.origin_verify_header
    }
  }

  enabled             = true
  is_ipv6_enabled     = true
  comment             = "Multi-customer email distribution portal"
  default_root_object = var.default_root_object

  # Aliases (custom domains)
  aliases = var.domain_names

  # Default cache behavior
  default_cache_behavior {
    allowed_methods  = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "S3-${var.s3_bucket_name}"

    forwarded_values {
      query_string = true
      headers      = ["Authorization", "CloudFront-Viewer-Country", "CloudFront-Is-Mobile-Viewer"]

      cookies {
        forward = "all"
      }
    }

    viewer_protocol_policy = "redirect-to-https"
    min_ttl                = 0
    default_ttl            = var.default_cache_ttl
    max_ttl                = var.max_cache_ttl
    compress               = true

    # Lambda@Edge functions
    lambda_function_association {
      event_type   = "viewer-request"
      lambda_arn   = aws_lambda_function.auth_lambda.qualified_arn
      include_body = false
    }

    lambda_function_association {
      event_type   = "origin-response"
      lambda_arn   = aws_lambda_function.security_headers_lambda.qualified_arn
      include_body = false
    }
  }

  # Cache behavior for API endpoints
  ordered_cache_behavior {
    path_pattern     = "/api/*"
    allowed_methods  = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods   = ["GET", "HEAD", "OPTIONS"]
    target_origin_id = "S3-${var.s3_bucket_name}"

    forwarded_values {
      query_string = true
      headers      = ["*"]

      cookies {
        forward = "all"
      }
    }

    min_ttl                = 0
    default_ttl            = 0
    max_ttl                = 0
    compress               = true
    viewer_protocol_policy = "redirect-to-https"

    lambda_function_association {
      event_type   = "viewer-request"
      lambda_arn   = aws_lambda_function.auth_lambda.qualified_arn
      include_body = true
    }
  }

  # Cache behavior for static assets
  ordered_cache_behavior {
    path_pattern     = "/static/*"
    allowed_methods  = ["GET", "HEAD"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "S3-${var.s3_bucket_name}"

    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }

    min_ttl                = var.static_cache_ttl
    default_ttl            = var.static_cache_ttl
    max_ttl                = var.static_max_cache_ttl
    compress               = true
    viewer_protocol_policy = "redirect-to-https"
  }

  # Price class
  price_class = var.price_class

  # Restrictions
  restrictions {
    geo_restriction {
      restriction_type = var.geo_restriction_type
      locations        = var.geo_restriction_locations
    }
  }

  # SSL certificate
  viewer_certificate {
    acm_certificate_arn            = var.ssl_certificate_arn
    ssl_support_method             = "sni-only"
    minimum_protocol_version       = "TLSv1.2_2021"
    cloudfront_default_certificate = var.ssl_certificate_arn == null
  }

  # Custom error responses
  dynamic "custom_error_response" {
    for_each = var.custom_error_responses
    content {
      error_code         = custom_error_response.value.error_code
      response_code      = custom_error_response.value.response_code
      response_page_path = custom_error_response.value.response_page_path
      error_caching_min_ttl = custom_error_response.value.error_caching_min_ttl
    }
  }

  # Logging
  logging_config {
    include_cookies = false
    bucket          = var.logging_bucket
    prefix          = var.logging_prefix
  }

  tags = merge(var.common_tags, {
    Name        = var.distribution_name
    Purpose     = "Multi-customer email distribution portal"
    Environment = var.environment
  })
}

# Lambda@Edge function for authentication
resource "aws_lambda_function" "auth_lambda" {
  provider         = aws.us_east_1 # Lambda@Edge must be in us-east-1
  filename         = data.archive_file.auth_lambda_zip.output_path
  function_name    = "${var.distribution_name}-auth"
  role            = aws_iam_role.lambda_edge_role.arn
  handler         = "index.handler"
  source_code_hash = data.archive_file.auth_lambda_zip.output_base64sha256
  runtime         = "nodejs18.x"
  timeout         = 5
  memory_size     = 128
  publish         = true

  environment {
    variables = {
      IDENTITY_CENTER_DOMAIN = var.identity_center_domain
      ALLOWED_GROUPS         = jsonencode(var.allowed_groups)
      SESSION_DURATION       = var.session_duration
      COOKIE_DOMAIN          = var.cookie_domain
      REDIRECT_URI           = var.redirect_uri
    }
  }

  tags = merge(var.common_tags, {
    Name        = "${var.distribution_name}-auth"
    Purpose     = "Identity Center authentication for CloudFront"
    Environment = var.environment
  })
}

# Lambda@Edge function for security headers
resource "aws_lambda_function" "security_headers_lambda" {
  provider         = aws.us_east_1 # Lambda@Edge must be in us-east-1
  filename         = data.archive_file.security_headers_lambda_zip.output_path
  function_name    = "${var.distribution_name}-security-headers"
  role            = aws_iam_role.lambda_edge_role.arn
  handler         = "index.handler"
  source_code_hash = data.archive_file.security_headers_lambda_zip.output_base64sha256
  runtime         = "nodejs18.x"
  timeout         = 5
  memory_size     = 128
  publish         = true

  environment {
    variables = {
      CSP_POLICY = var.content_security_policy
      HSTS_MAX_AGE = var.hsts_max_age
    }
  }

  tags = merge(var.common_tags, {
    Name        = "${var.distribution_name}-security-headers"
    Purpose     = "Security headers for CloudFront responses"
    Environment = var.environment
  })
}

# IAM role for Lambda@Edge
resource "aws_iam_role" "lambda_edge_role" {
  name = "${var.distribution_name}-lambda-edge-role"

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

  tags = var.common_tags
}

# IAM policy for Lambda@Edge
resource "aws_iam_role_policy" "lambda_edge_policy" {
  name = "${var.distribution_name}-lambda-edge-policy"
  role = aws_iam_role.lambda_edge_role.id

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
      },
      {
        Effect = "Allow"
        Action = [
          "sso:GetManagedApplicationInstance",
          "sso:DescribePermissionSet",
          "sso:ListAccountAssignments"
        ]
        Resource = "*"
      }
    ]
  })
}

# Lambda function code for authentication
data "archive_file" "auth_lambda_zip" {
  type        = "zip"
  output_path = "${path.module}/auth_lambda.zip"
  source {
    content = templatefile("${path.module}/lambda/auth.js", {
      identity_center_domain = var.identity_center_domain
      allowed_groups         = jsonencode(var.allowed_groups)
      session_duration       = var.session_duration
      cookie_domain          = var.cookie_domain
      redirect_uri           = var.redirect_uri
    })
    filename = "index.js"
  }
}

# Lambda function code for security headers
data "archive_file" "security_headers_lambda_zip" {
  type        = "zip"
  output_path = "${path.module}/security_headers_lambda.zip"
  source {
    content = templatefile("${path.module}/lambda/security-headers.js", {
      csp_policy   = var.content_security_policy
      hsts_max_age = var.hsts_max_age
    })
    filename = "index.js"
  }
}

# S3 bucket policy to allow CloudFront OAC
resource "aws_s3_bucket_policy" "cloudfront_oac_policy" {
  bucket = var.s3_bucket_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowCloudFrontServicePrincipal"
        Effect = "Allow"
        Principal = {
          Service = "cloudfront.amazonaws.com"
        }
        Action   = "s3:GetObject"
        Resource = "${var.s3_bucket_arn}/*"
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = aws_cloudfront_distribution.portal_distribution.arn
          }
        }
      }
    ]
  })
}

# Route53 records for custom domains
resource "aws_route53_record" "cloudfront_alias" {
  for_each = var.create_route53_records ? toset(var.domain_names) : []
  
  zone_id = var.route53_zone_id
  name    = each.value
  type    = "A"

  alias {
    name                   = aws_cloudfront_distribution.portal_distribution.domain_name
    zone_id                = aws_cloudfront_distribution.portal_distribution.hosted_zone_id
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "cloudfront_alias_ipv6" {
  for_each = var.create_route53_records ? toset(var.domain_names) : []
  
  zone_id = var.route53_zone_id
  name    = each.value
  type    = "AAAA"

  alias {
    name                   = aws_cloudfront_distribution.portal_distribution.domain_name
    zone_id                = aws_cloudfront_distribution.portal_distribution.hosted_zone_id
    evaluate_target_health = false
  }
}