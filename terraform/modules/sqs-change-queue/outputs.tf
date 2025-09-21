# Outputs for SQS Change Queue Module

output "queue_id" {
  description = "ID of the SQS queue"
  value       = aws_sqs_queue.change_queue.id
}

output "queue_arn" {
  description = "ARN of the SQS queue"
  value       = aws_sqs_queue.change_queue.arn
}

output "queue_url" {
  description = "URL of the SQS queue"
  value       = aws_sqs_queue.change_queue.url
}

output "queue_name" {
  description = "Name of the SQS queue"
  value       = aws_sqs_queue.change_queue.name
}

# Dead Letter Queue Outputs
output "dlq_id" {
  description = "ID of the dead letter queue"
  value       = var.enable_dlq ? aws_sqs_queue.dlq[0].id : null
}

output "dlq_arn" {
  description = "ARN of the dead letter queue"
  value       = var.enable_dlq ? aws_sqs_queue.dlq[0].arn : null
}

output "dlq_url" {
  description = "URL of the dead letter queue"
  value       = var.enable_dlq ? aws_sqs_queue.dlq[0].url : null
}

output "dlq_name" {
  description = "Name of the dead letter queue"
  value       = var.enable_dlq ? aws_sqs_queue.dlq[0].name : null
}

# Configuration Outputs
output "queue_configuration" {
  description = "Queue configuration summary"
  value = {
    fifo_queue                = var.fifo_queue
    delay_seconds            = var.delay_seconds
    max_message_size         = var.max_message_size
    message_retention_seconds = var.message_retention_seconds
    visibility_timeout_seconds = var.visibility_timeout_seconds
    receive_wait_time_seconds = var.receive_wait_time_seconds
    kms_key_id               = var.kms_key_id
  }
}

output "dlq_configuration" {
  description = "Dead letter queue configuration"
  value = var.enable_dlq ? {
    enabled               = true
    max_receive_count     = var.max_receive_count
    message_retention_seconds = var.dlq_message_retention_seconds
  } : {
    enabled = false
  }
}

# Security Outputs
output "cross_account_access" {
  description = "Cross-account access configuration"
  value = {
    orchestrator_role_arn      = var.orchestrator_role_arn
    customer_processor_role_arn = var.customer_processor_role_arn
    orchestrator_account_id    = var.orchestrator_account_id
    s3_bucket_arn             = var.s3_bucket_arn
  }
}

output "encryption_configuration" {
  description = "Encryption configuration"
  value = {
    kms_key_id                    = var.kms_key_id
    kms_data_key_reuse_period    = var.kms_data_key_reuse_period
  }
}

# Monitoring Outputs
output "cloudwatch_alarms" {
  description = "CloudWatch alarms configuration"
  value = var.enable_cloudwatch_alarms ? {
    queue_depth_alarm = {
      name      = "${var.queue_name}-depth-alarm"
      threshold = var.queue_depth_alarm_threshold
    }
    message_age_alarm = {
      name      = "${var.queue_name}-message-age-alarm"
      threshold = var.message_age_alarm_threshold
    }
    dlq_messages_alarm = var.enable_dlq ? {
      name      = "${var.queue_name}-dlq-messages-alarm"
      threshold = 0
    } : null
  } : null
}

output "cloudwatch_dashboard_name" {
  description = "Name of the CloudWatch dashboard"
  value       = var.enable_cloudwatch_dashboard ? "${var.queue_name}-dashboard" : null
}

# SNS Outputs
output "sns_topic_arn" {
  description = "ARN of the SNS topic for notifications"
  value       = var.enable_sns_notifications ? aws_sns_topic.queue_notifications[0].arn : null
}

output "sns_topic_name" {
  description = "Name of the SNS topic for notifications"
  value       = var.enable_sns_notifications ? aws_sns_topic.queue_notifications[0].name : null
}

output "notification_emails" {
  description = "List of email addresses configured for notifications"
  value       = var.notification_emails
}

# Lambda Processor Outputs
output "lambda_function_name" {
  description = "Name of the Lambda processor function"
  value       = var.enable_lambda_processor ? aws_lambda_function.queue_processor[0].function_name : null
}

output "lambda_function_arn" {
  description = "ARN of the Lambda processor function"
  value       = var.enable_lambda_processor ? aws_lambda_function.queue_processor[0].arn : null
}

output "lambda_role_arn" {
  description = "ARN of the Lambda execution role"
  value       = var.enable_lambda_processor ? aws_iam_role.lambda_processor_role[0].arn : null
}

output "lambda_configuration" {
  description = "Lambda processor configuration"
  value = var.enable_lambda_processor ? {
    timeout      = var.lambda_timeout
    memory_size  = var.lambda_memory_size
    batch_size   = var.lambda_batch_size
    batching_window = var.lambda_batching_window
    log_level    = var.lambda_log_level
  } : null
}

# Customer Information
output "customer_code" {
  description = "Customer code for this queue"
  value       = var.customer_code
}

output "environment" {
  description = "Environment name"
  value       = var.environment
}

# Queue Metrics
output "queue_metrics" {
  description = "Available CloudWatch metrics for the queue"
  value = [
    "NumberOfMessagesSent",
    "NumberOfMessagesReceived",
    "NumberOfMessagesDeleted",
    "ApproximateNumberOfVisibleMessages",
    "ApproximateNumberOfMessagesNotVisible",
    "ApproximateAgeOfOldestMessage"
  ]
}

# Integration Information
output "s3_event_configuration" {
  description = "Configuration for S3 event notifications"
  value = {
    queue_arn = aws_sqs_queue.change_queue.arn
    events    = ["s3:ObjectCreated:*"]
    filter_prefix = "customers/${var.customer_code}/"
    filter_suffix = ".json"
  }
}

output "policy_statements" {
  description = "Summary of IAM policy statements applied to the queue"
  value = {
    orchestrator_permissions = [
      "sqs:SendMessage",
      "sqs:GetQueueAttributes",
      "sqs:GetQueueUrl"
    ]
    customer_processor_permissions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes",
      "sqs:GetQueueUrl",
      "sqs:ChangeMessageVisibility"
    ]
    s3_permissions = [
      "sqs:SendMessage"
    ]
  }
}