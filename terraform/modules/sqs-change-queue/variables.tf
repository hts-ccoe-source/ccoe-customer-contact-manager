# Variables for SQS Change Queue Module

variable "queue_name" {
  description = "Name of the SQS queue"
  type        = string
}

variable "customer_code" {
  description = "Customer code for this queue"
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

# Queue Configuration
variable "delay_seconds" {
  description = "The time in seconds that the delivery of all messages in the queue will be delayed"
  type        = number
  default     = 0
}

variable "max_message_size" {
  description = "The limit of how many bytes a message can contain before Amazon SQS rejects it"
  type        = number
  default     = 262144 # 256 KB
}

variable "message_retention_seconds" {
  description = "The number of seconds Amazon SQS retains a message"
  type        = number
  default     = 1209600 # 14 days
}

variable "receive_wait_time_seconds" {
  description = "The time for which a ReceiveMessage call will wait for a message to arrive"
  type        = number
  default     = 20 # Enable long polling
}

variable "visibility_timeout_seconds" {
  description = "The visibility timeout for the queue"
  type        = number
  default     = 300 # 5 minutes
}

# Encryption Configuration
variable "kms_key_id" {
  description = "The ID of an AWS-managed customer master key (CMK) for Amazon SQS"
  type        = string
  default     = "alias/aws/sqs"
}

variable "kms_data_key_reuse_period" {
  description = "The length of time, in seconds, for which Amazon SQS can reuse a data key"
  type        = number
  default     = 300
}

# Dead Letter Queue Configuration
variable "enable_dlq" {
  description = "Enable dead letter queue"
  type        = bool
  default     = true
}

variable "max_receive_count" {
  description = "The number of times a message is delivered to the source queue before being moved to the dead-letter queue"
  type        = number
  default     = 3
}

variable "dlq_message_retention_seconds" {
  description = "The number of seconds Amazon SQS retains a message in the dead letter queue"
  type        = number
  default     = 1209600 # 14 days
}

# FIFO Configuration
variable "fifo_queue" {
  description = "Boolean designating a FIFO queue"
  type        = bool
  default     = false
}

variable "content_based_deduplication" {
  description = "Enables content-based deduplication for FIFO queues"
  type        = bool
  default     = false
}

variable "deduplication_scope" {
  description = "Specifies whether message deduplication occurs at the message group or queue level"
  type        = string
  default     = "queue"
  validation {
    condition = contains([
      "messageGroup",
      "queue"
    ], var.deduplication_scope)
    error_message = "Deduplication scope must be messageGroup or queue."
  }
}

variable "fifo_throughput_limit" {
  description = "Specifies whether the FIFO queue throughput quota applies to the entire queue or per message group"
  type        = string
  default     = "perQueue"
  validation {
    condition = contains([
      "perQueue",
      "perMessageGroupId"
    ], var.fifo_throughput_limit)
    error_message = "FIFO throughput limit must be perQueue or perMessageGroupId."
  }
}

# Cross-Account Access Configuration
variable "orchestrator_role_arn" {
  description = "ARN of the orchestrator role that can send messages to this queue"
  type        = string
}

variable "customer_processor_role_arn" {
  description = "ARN of the customer processor role that can receive messages from this queue"
  type        = string
}

variable "orchestrator_account_id" {
  description = "AWS account ID of the orchestrator account"
  type        = string
}

variable "s3_bucket_arn" {
  description = "ARN of the S3 bucket that can send notifications to this queue"
  type        = string
}

# Security Configuration
variable "enable_source_ip_restriction" {
  description = "Enable source IP restrictions for the queue"
  type        = bool
  default     = false
}

variable "allowed_source_ips" {
  description = "List of allowed source IP addresses/CIDR blocks"
  type        = list(string)
  default     = []
}

# Monitoring Configuration
variable "enable_cloudwatch_alarms" {
  description = "Enable CloudWatch alarms for the queue"
  type        = bool
  default     = true
}

variable "queue_depth_alarm_threshold" {
  description = "Threshold for queue depth alarm"
  type        = number
  default     = 100
}

variable "message_age_alarm_threshold" {
  description = "Threshold for message age alarm (seconds)"
  type        = number
  default     = 3600 # 1 hour
}

variable "alarm_actions" {
  description = "List of ARNs to notify when alarm triggers"
  type        = list(string)
  default     = []
}

variable "enable_cloudwatch_dashboard" {
  description = "Enable CloudWatch dashboard for the queue"
  type        = bool
  default     = false
}

# SNS Notifications Configuration
variable "enable_sns_notifications" {
  description = "Enable SNS notifications for queue events"
  type        = bool
  default     = false
}

variable "notification_emails" {
  description = "List of email addresses to receive notifications"
  type        = list(string)
  default     = []
}

# Lambda Processor Configuration
variable "enable_lambda_processor" {
  description = "Enable Lambda function to process queue messages"
  type        = bool
  default     = false
}

variable "lambda_timeout" {
  description = "Lambda function timeout in seconds"
  type        = number
  default     = 300
}

variable "lambda_memory_size" {
  description = "Lambda function memory size in MB"
  type        = number
  default     = 512
}

variable "lambda_batch_size" {
  description = "Maximum number of messages to process in a single Lambda invocation"
  type        = number
  default     = 10
}

variable "lambda_batching_window" {
  description = "Maximum amount of time to gather records before invoking Lambda"
  type        = number
  default     = 5
}

variable "lambda_log_level" {
  description = "Log level for Lambda function"
  type        = string
  default     = "info"
  validation {
    condition = contains([
      "debug",
      "info",
      "warn",
      "error"
    ], var.lambda_log_level)
    error_message = "Log level must be debug, info, warn, or error."
  }
}