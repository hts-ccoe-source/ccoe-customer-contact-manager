package main

import "time"

// Organization represents an AWS organization
type Organization struct {
	FriendlyName        string `json:"mocb_org_friendly_name"`
	Prefix              string `json:"mocb_org_prefix"`
	Environment         string `json:"environment"`
	ManagementAccountId string `json:"management_account_id"`
}

// AlternateContactConfig represents alternate contact configuration
type AlternateContactConfig struct {
	SecurityEmail   string `json:"security_email"`
	SecurityName    string `json:"security_name"`
	SecurityTitle   string `json:"security_title"`
	SecurityPhone   string `json:"security_phone"`
	BillingEmail    string `json:"billing_email"`
	BillingName     string `json:"billing_name"`
	BillingTitle    string `json:"billing_title"`
	BillingPhone    string `json:"billing_phone"`
	OperationsEmail string `json:"operations_email"`
	OperationsName  string `json:"operations_name"`
	OperationsTitle string `json:"operations_title"`
	OperationsPhone string `json:"operations_phone"`
}

// CustomerAccountInfo represents customer account information
type CustomerAccountInfo struct {
	CustomerCode string `json:"customer_code"`
	CustomerName string `json:"customer_name"`
	AWSAccountID string `json:"aws_account_id"`
	Region       string `json:"region"`
	SESRoleARN   string `json:"ses_role_arn"`
	Environment  string `json:"environment"`
	SQSQueueARN  string `json:"sqs_queue_arn"`
}

// EmailRequest represents an email sending request
type EmailRequest struct {
	CustomerCode string                 `json:"customer_code"`
	ToAddresses  []string               `json:"to_addresses"`
	Subject      string                 `json:"subject"`
	Body         string                 `json:"body"`
	TemplateData map[string]interface{} `json:"template_data,omitempty"`
}

// SQSMessage represents an SQS message for processing
type SQSMessage struct {
	MessageID    string                 `json:"message_id"`
	CustomerCode string                 `json:"customer_code"`
	S3Bucket     string                 `json:"s3_bucket"`
	S3Key        string                 `json:"s3_key"`
	EventTime    time.Time              `json:"event_time"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// S3Config represents S3 configuration
type S3Config struct {
	BucketName string `json:"bucket_name"`
}

// Config represents the application configuration
type Config struct {
	AWSRegion        string                         `json:"aws_region"`
	LogLevel         string                         `json:"log_level"`
	CustomerMappings map[string]CustomerAccountInfo `json:"customer_mappings"`
	ContactConfig    AlternateContactConfig         `json:"contact_config"`
	S3Config         S3Config                       `json:"s3_config"`
}
