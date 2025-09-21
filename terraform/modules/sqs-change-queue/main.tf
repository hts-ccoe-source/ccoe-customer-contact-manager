# SQS Change Queue Module for Customer Accounts
# Creates SQS queues for receiving change notifications in customer accounts

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Main SQS queue for change notifications
resource "aws_sqs_queue" "change_queue" {
  name                       = var.queue_name
  delay_seconds              = var.delay_seconds
  max_message_size           = var.max_message_size
  message_retention_seconds  = var.message_retention_seconds
  receive_wait_time_seconds  = var.receive_wait_time_seconds
  visibility_timeout_seconds = var.visibility_timeout_seconds

  # Enable server-side encryption
  kms_master_key_id                 = var.kms_key_id
  kms_data_key_reuse_period_seconds = var.kms_data_key_reuse_period

  # Dead letter queue configuration
  redrive_policy = var.enable_dlq ? jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[0].arn
    maxReceiveCount     = var.max_receive_count
  }) : null

  # FIFO configuration (if enabled)
  fifo_queue                  = var.fifo_queue
  content_based_deduplication = var.fifo_queue ? var.content_based_deduplication : null
  deduplication_scope         = var.fifo_queue ? var.deduplication_scope : null
  fifo_throughput_limit       = var.fifo_queue ? var.fifo_throughput_limit : null

  tags = merge(var.common_tags, {
    Name         = var.queue_name
    Purpose      = "Multi-customer email distribution change notifications"
    CustomerCode = var.customer_code
    Environment  = var.environment
  })
}

# Dead letter queue (optional)
resource "aws_sqs_queue" "dlq" {
  count                     = var.enable_dlq ? 1 : 0
  name                      = "${var.queue_name}-dlq"
  message_retention_seconds = var.dlq_message_retention_seconds

  # Enable server-side encryption
  kms_master_key_id                 = var.kms_key_id
  kms_data_key_reuse_period_seconds = var.kms_data_key_reuse_period

  # FIFO configuration (if main queue is FIFO)
  fifo_queue                  = var.fifo_queue
  content_based_deduplication = var.fifo_queue ? var.content_based_deduplication : null

  tags = merge(var.common_tags, {
    Name         = "${var.queue_name}-dlq"
    Purpose      = "Dead letter queue for change notifications"
    CustomerCode = var.customer_code
    Environment  = var.environment
  })
}

# Queue policy for cross-account access
resource "aws_sqs_queue_policy" "change_queue_policy" {
  queue_url = aws_sqs_queue.change_queue.id

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
          "sqs:SendMessage",
          "sqs:GetQueueAttributes",
          "sqs:GetQueueUrl"
        ]
        Resource = aws_sqs_queue.change_queue.arn
        Condition = var.enable_source_ip_restriction ? {
          IpAddress = {
            "aws:SourceIp" = var.allowed_source_ips
          }
        } : {}
      },
      {
        Sid    = "AllowCustomerProcessor"
        Effect = "Allow"
        Principal = {
          AWS = var.customer_processor_role_arn
        }
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes",
          "sqs:GetQueueUrl",
          "sqs:ChangeMessageVisibility"
        ]
        Resource = aws_sqs_queue.change_queue.arn
      },
      {
        Sid    = "AllowS3Notifications"
        Effect = "Allow"
        Principal = {
          Service = "s3.amazonaws.com"
        }
        Action   = "sqs:SendMessage"
        Resource = aws_sqs_queue.change_queue.arn
        Condition = {
          StringEquals = {
            "aws:SourceAccount" = var.orchestrator_account_id
          }
          ArnEquals = {
            "aws:SourceArn" = var.s3_bucket_arn
          }
        }
      }
    ]
  })
}

# CloudWatch alarms for queue monitoring
resource "aws_cloudwatch_metric_alarm" "queue_depth_alarm" {
  count               = var.enable_cloudwatch_alarms ? 1 : 0
  alarm_name          = "${var.queue_name}-depth-alarm"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "ApproximateNumberOfVisibleMessages"
  namespace           = "AWS/SQS"
  period              = "300"
  statistic           = "Average"
  threshold           = var.queue_depth_alarm_threshold
  alarm_description   = "This metric monitors SQS queue depth"
  alarm_actions       = var.alarm_actions

  dimensions = {
    QueueName = aws_sqs_queue.change_queue.name
  }

  tags = var.common_tags
}

resource "aws_cloudwatch_metric_alarm" "dlq_messages_alarm" {
  count               = var.enable_dlq && var.enable_cloudwatch_alarms ? 1 : 0
  alarm_name          = "${var.queue_name}-dlq-messages-alarm"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "ApproximateNumberOfVisibleMessages"
  namespace           = "AWS/SQS"
  period              = "300"
  statistic           = "Average"
  threshold           = "0"
  alarm_description   = "This metric monitors messages in dead letter queue"
  alarm_actions       = var.alarm_actions

  dimensions = {
    QueueName = aws_sqs_queue.dlq[0].name
  }

  tags = var.common_tags
}

resource "aws_cloudwatch_metric_alarm" "message_age_alarm" {
  count               = var.enable_cloudwatch_alarms ? 1 : 0
  alarm_name          = "${var.queue_name}-message-age-alarm"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "ApproximateAgeOfOldestMessage"
  namespace           = "AWS/SQS"
  period              = "300"
  statistic           = "Maximum"
  threshold           = var.message_age_alarm_threshold
  alarm_description   = "This metric monitors age of oldest message in queue"
  alarm_actions       = var.alarm_actions

  dimensions = {
    QueueName = aws_sqs_queue.change_queue.name
  }

  tags = var.common_tags
}

# CloudWatch dashboard for queue metrics
resource "aws_cloudwatch_dashboard" "queue_dashboard" {
  count          = var.enable_cloudwatch_dashboard ? 1 : 0
  dashboard_name = "${var.queue_name}-dashboard"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6

        properties = {
          metrics = [
            ["AWS/SQS", "NumberOfMessagesSent", "QueueName", aws_sqs_queue.change_queue.name],
            [".", "NumberOfMessagesReceived", ".", "."],
            [".", "NumberOfMessagesDeleted", ".", "."]
          ]
          view    = "timeSeries"
          stacked = false
          region  = data.aws_region.current.name
          title   = "Message Throughput"
          period  = 300
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6

        properties = {
          metrics = [
            ["AWS/SQS", "ApproximateNumberOfVisibleMessages", "QueueName", aws_sqs_queue.change_queue.name],
            [".", "ApproximateNumberOfMessagesNotVisible", ".", "."]
          ]
          view    = "timeSeries"
          stacked = false
          region  = data.aws_region.current.name
          title   = "Queue Depth"
          period  = 300
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 12
        width  = 12
        height = 6

        properties = {
          metrics = [
            ["AWS/SQS", "ApproximateAgeOfOldestMessage", "QueueName", aws_sqs_queue.change_queue.name]
          ]
          view    = "timeSeries"
          stacked = false
          region  = data.aws_region.current.name
          title   = "Message Age"
          period  = 300
        }
      }
    ]
  })
}

# SNS topic for queue notifications (optional)
resource "aws_sns_topic" "queue_notifications" {
  count = var.enable_sns_notifications ? 1 : 0
  name  = "${var.queue_name}-notifications"

  kms_master_key_id = var.kms_key_id

  tags = merge(var.common_tags, {
    Name         = "${var.queue_name}-notifications"
    Purpose      = "Notifications for SQS queue events"
    CustomerCode = var.customer_code
    Environment  = var.environment
  })
}

# SNS topic policy
resource "aws_sns_topic_policy" "queue_notifications_policy" {
  count = var.enable_sns_notifications ? 1 : 0
  arn   = aws_sns_topic.queue_notifications[0].arn

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowCloudWatchAlarms"
        Effect = "Allow"
        Principal = {
          Service = "cloudwatch.amazonaws.com"
        }
        Action   = "sns:Publish"
        Resource = aws_sns_topic.queue_notifications[0].arn
      }
    ]
  })
}

# Email subscription to SNS topic
resource "aws_sns_topic_subscription" "email_notifications" {
  count     = var.enable_sns_notifications && length(var.notification_emails) > 0 ? length(var.notification_emails) : 0
  topic_arn = aws_sns_topic.queue_notifications[0].arn
  protocol  = "email"
  endpoint  = var.notification_emails[count.index]
}

# Lambda function for custom queue processing (optional)
resource "aws_lambda_function" "queue_processor" {
  count            = var.enable_lambda_processor ? 1 : 0
  filename         = data.archive_file.queue_processor_zip[0].output_path
  function_name    = "${var.queue_name}-processor"
  role            = aws_iam_role.lambda_processor_role[0].arn
  handler         = "index.handler"
  source_code_hash = data.archive_file.queue_processor_zip[0].output_base64sha256
  runtime         = "nodejs18.x"
  timeout         = var.lambda_timeout
  memory_size     = var.lambda_memory_size

  environment {
    variables = {
      QUEUE_URL      = aws_sqs_queue.change_queue.id
      CUSTOMER_CODE  = var.customer_code
      LOG_LEVEL      = var.lambda_log_level
    }
  }

  tags = merge(var.common_tags, {
    Name         = "${var.queue_name}-processor"
    Purpose      = "Process SQS messages for customer"
    CustomerCode = var.customer_code
    Environment  = var.environment
  })
}

# Event source mapping for Lambda
resource "aws_lambda_event_source_mapping" "queue_processor_trigger" {
  count            = var.enable_lambda_processor ? 1 : 0
  event_source_arn = aws_sqs_queue.change_queue.arn
  function_name    = aws_lambda_function.queue_processor[0].arn
  batch_size       = var.lambda_batch_size
  maximum_batching_window_in_seconds = var.lambda_batching_window
}

# IAM role for Lambda processor
resource "aws_iam_role" "lambda_processor_role" {
  count = var.enable_lambda_processor ? 1 : 0
  name  = "${var.queue_name}-processor-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = var.common_tags
}

# IAM policy for Lambda processor
resource "aws_iam_role_policy" "lambda_processor_policy" {
  count = var.enable_lambda_processor ? 1 : 0
  name  = "${var.queue_name}-processor-policy"
  role  = aws_iam_role.lambda_processor_role[0].id

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
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes"
        ]
        Resource = aws_sqs_queue.change_queue.arn
      }
    ]
  })
}

# Lambda function code
data "archive_file" "queue_processor_zip" {
  count       = var.enable_lambda_processor ? 1 : 0
  type        = "zip"
  output_path = "${path.module}/queue_processor.zip"
  source {
    content = templatefile("${path.module}/lambda/queue-processor.js", {
      customer_code = var.customer_code
    })
    filename = "index.js"
  }
}

# Data sources
data "aws_region" "current" {}
data "aws_caller_identity" "current" {}