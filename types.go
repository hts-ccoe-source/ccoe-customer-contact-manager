package main

import (
	"strings"
	"time"
)

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
	AWSAccountID string `json:"aws_account_id"` // Deprecated: use GetAccountID() method instead
	Region       string `json:"region"`
	SESRoleARN   string `json:"ses_role_arn"`
	Environment  string `json:"environment"`
	SQSQueueARN  string `json:"sqs_queue_arn"`
}

// GetAccountID extracts the AWS account ID from the SES role ARN
// ARN format: arn:aws:iam::123456789012:role/RoleName
func (c *CustomerAccountInfo) GetAccountID() string {
	if c.SESRoleARN == "" {
		return c.AWSAccountID // fallback to deprecated field
	}

	parts := strings.Split(c.SESRoleARN, ":")
	if len(parts) >= 5 {
		return parts[4] // account ID is the 5th part (index 4)
	}

	return c.AWSAccountID // fallback to deprecated field
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

// S3EventRecord represents a single S3 event record
type S3EventRecord struct {
	EventVersion string    `json:"eventVersion"`
	EventSource  string    `json:"eventSource"`
	AWSRegion    string    `json:"awsRegion"`
	EventTime    time.Time `json:"eventTime"`
	EventName    string    `json:"eventName"`
	S3           struct {
		S3SchemaVersion string `json:"s3SchemaVersion"`
		ConfigurationID string `json:"configurationId"`
		Bucket          struct {
			Name string `json:"name"`
			ARN  string `json:"arn"`
		} `json:"bucket"`
		Object struct {
			Key       string `json:"key"`
			Size      int64  `json:"size"`
			ETag      string `json:"eTag"`
			VersionID string `json:"versionId,omitempty"`
		} `json:"object"`
	} `json:"s3"`
}

// S3EventNotification represents the S3 event notification message
type S3EventNotification struct {
	Records []S3EventRecord `json:"Records"`
}

// ChangeMetadata represents the metadata from the uploaded JSON file
type ChangeMetadata struct {
	ChangeID           string   `json:"changeId"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Customers          []string `json:"customers"`
	ImplementationPlan string   `json:"implementationPlan"`
	Schedule           struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	} `json:"schedule"`
	Impact            string                 `json:"impact"`
	RollbackPlan      string                 `json:"rollbackPlan"`
	CommunicationPlan string                 `json:"communicationPlan"`
	Approver          string                 `json:"approver"`
	Implementer       string                 `json:"implementer"`
	Timestamp         string                 `json:"timestamp"`
	Source            string                 `json:"source"`
	TestRun           bool                   `json:"testRun,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
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
