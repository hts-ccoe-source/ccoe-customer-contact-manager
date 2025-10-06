// Package types contains all shared type definitions and structs.
package types

import (
	"strings"
	"time"
)

// Organization represents an AWS organization configuration
type Organization struct {
	FriendlyName        string `json:"mocb_org_friendly_name"`
	Prefix              string `json:"mocb_org_prefix"`
	Environment         string `json:"environment"`
	ManagementAccountId string `json:"management_account_id"`
}

// AlternateContactConfig represents the configuration for alternate contacts
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

// SESTopicConfig represents the configuration for SES topics
type SESTopicConfig struct {
	TopicName                 string   `json:"TopicName"`
	DisplayName               string   `json:"DisplayName"`
	Description               string   `json:"Description"`
	DefaultSubscriptionStatus string   `json:"DefaultSubscriptionStatus"`
	OptInRoles                []string `json:"OptInRoles"`
}

// SESConfig represents the configuration for SES operations
type SESConfig struct {
	TopicGroupPrefix  []string         `json:"topic_group_prefix"`
	TopicGroupMembers []SESTopicConfig `json:"topic_group_members"`
	Topics            []SESTopicConfig `json:"topics"`
}

// ContactImportConfig represents configuration for importing contacts
type ContactImportConfig struct {
	DefaultTopics []string           `json:"default_topics"`
	RoleMappings  []RoleTopicMapping `json:"role_mappings"`
}

// RoleTopicMapping represents mapping between roles and topics
type RoleTopicMapping struct {
	Role   string   `json:"role"`
	Topics []string `json:"topics"`
}

// SESBackup represents a backup of SES contact list data
type SESBackup struct {
	ContactList struct {
		Name        string      `json:"name"`
		Description *string     `json:"description"`
		Topics      interface{} `json:"topics"` // Using interface{} to avoid import cycle
		CreatedAt   string      `json:"created_at"`
		UpdatedAt   string      `json:"updated_at"`
	} `json:"contact_list"`
	Contacts       []SESContactBackup `json:"contacts"`
	BackupMetadata struct {
		Timestamp string `json:"timestamp"`
		Tool      string `json:"tool"`
		Action    string `json:"action"`
	} `json:"backup_metadata"`
}

// SESContactBackup represents a contact in the backup
type SESContactBackup struct {
	EmailAddress     string      `json:"email_address"`
	TopicPreferences interface{} `json:"topic_preferences"` // Using interface{} to avoid import cycle
	UnsubscribeAll   bool        `json:"unsubscribe_all"`
}

// SubscriptionConfig represents subscription configuration mapping
type SubscriptionConfig map[string][]string

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
	ChangeID                string                 `json:"changeId"`
	ChangeTitle             string                 `json:"changeTitle"`
	ChangeReason            string                 `json:"changeReason"`
	Customers               []string               `json:"customers"`
	ImplementationPlan      string                 `json:"implementationPlan"`
	TestPlan                string                 `json:"testPlan"`
	CustomerImpact          string                 `json:"customerImpact"`
	RollbackPlan            string                 `json:"rollbackPlan"`
	SnowTicket              string                 `json:"snowTicket"`
	JiraTicket              string                 `json:"jiraTicket"`
	ImplementationBeginDate string                 `json:"implementationBeginDate"`
	ImplementationBeginTime string                 `json:"implementationBeginTime"`
	ImplementationEndDate   string                 `json:"implementationEndDate"`
	ImplementationEndTime   string                 `json:"implementationEndTime"`
	Timezone                string                 `json:"timezone"`
	MeetingRequired         string                 `json:"meetingRequired"`
	MeetingTitle            string                 `json:"meetingTitle"`
	MeetingDate             string                 `json:"meetingDate"`
	MeetingDuration         string                 `json:"meetingDuration"`
	MeetingLocation         string                 `json:"meetingLocation"`
	Status                  string                 `json:"status"`
	Version                 int                    `json:"version"`
	CreatedAt               string                 `json:"createdAt"`
	CreatedBy               string                 `json:"createdBy"`
	ModifiedAt              string                 `json:"modifiedAt"`
	ModifiedBy              string                 `json:"modifiedBy"`
	SubmittedAt             string                 `json:"submittedAt"`
	SubmittedBy             string                 `json:"submittedBy"`
	Source                  string                 `json:"source"`
	TestRun                 bool                   `json:"testRun,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// ApprovalRequestMetadata represents the metadata from the collector
type ApprovalRequestMetadata struct {
	ChangeMetadata struct {
		Title         string   `json:"changeTitle"`
		CustomerNames []string `json:"customerNames"`
		CustomerCodes []string `json:"customerCodes"`
		Tickets       struct {
			ServiceNow string `json:"serviceNow"`
			Jira       string `json:"jira"`
		} `json:"tickets"`
		ChangeReason           string `json:"changeReason"`
		ImplementationPlan     string `json:"implementationPlan"`
		TestPlan               string `json:"testPlan"`
		ExpectedCustomerImpact string `json:"expectedCustomerImpact"`
		RollbackPlan           string `json:"rollbackPlan"`
		Schedule               struct {
			ImplementationStart string `json:"implementationStart"`
			ImplementationEnd   string `json:"implementationEnd"`
			BeginDate           string `json:"beginDate"`
			BeginTime           string `json:"beginTime"`
			EndDate             string `json:"endDate"`
			EndTime             string `json:"endTime"`
			Timezone            string `json:"timezone"`
		} `json:"schedule"`
		Description string `json:"description"`
	} `json:"changeMetadata"`
	EmailNotification struct {
		Subject         string   `json:"subject"`
		CustomerNames   []string `json:"customerNames"`
		CustomerCodes   []string `json:"customerCodes"`
		ScheduledWindow struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"scheduledWindow"`
		Tickets struct {
			Snow string `json:"snow"`
			Jira string `json:"jira"`
		} `json:"tickets"`
	} `json:"emailNotification"`
	MeetingInvite *struct {
		Title           string   `json:"title"`
		StartTime       string   `json:"startTime"`
		Duration        int      `json:"duration"`
		DurationMinutes int      `json:"durationMinutes"`
		Attendees       []string `json:"attendees"`
		Location        string   `json:"location"`
	} `json:"meetingInvite,omitempty"`
	GeneratedAt string `json:"generatedAt"`
	GeneratedBy string `json:"generatedBy"`
}

// Microsoft Graph API structures
type GraphAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type GraphError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type GraphMeetingResponse struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
}

// RateLimiter implements a simple rate limiter using a channel
type RateLimiter struct {
	Ticker   *time.Ticker
	Requests chan struct{}
}

// IdentityCenterUser represents a user from Identity Center
type IdentityCenterUser struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
}

// IdentityCenterGroupMembership represents a user's group membership
type IdentityCenterGroupMembership struct {
	UserId      string   `json:"user_id"`
	UserName    string   `json:"user_name"`
	DisplayName string   `json:"display_name"`
	Groups      []string `json:"groups"`
}

// IdentityCenterGroupCentric represents groups with their members
type IdentityCenterGroupCentric struct {
	GroupName string                   `json:"group_name"`
	Members   []IdentityCenterUserInfo `json:"members"`
}

// IdentityCenterUserInfo represents user info for group membership
type IdentityCenterUserInfo struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
}

// CCOECloudGroupInfo represents parsed information from ccoe-cloud group names
type CCOECloudGroupInfo struct {
	GroupName        string `json:"group_name"`
	AccountName      string `json:"account_name"`
	Role             string `json:"role"`
	Environment      string `json:"environment"`
	CustomerCode     string `json:"customer_code"`
	IsValidCCOEGroup bool   `json:"is_valid_ccoe_group"`
}

// CustomerCodeExtractor handles extraction and validation of customer codes from metadata
type CustomerCodeExtractor struct {
	ValidCustomerCodes map[string]bool
}

// S3EventNotificationConfig represents S3 event notification configuration
type S3EventNotificationConfig struct {
	BucketName            string                       `json:"bucketName"`
	CustomerNotifications []CustomerNotificationConfig `json:"customerNotifications"`
}

// CustomerNotificationConfig represents notification config for a single customer
type CustomerNotificationConfig struct {
	CustomerCode string `json:"customerCode"`
	SQSQueueArn  string `json:"sqsQueueArn"`
	Prefix       string `json:"prefix"`
}

// S3EventConfigManager handles S3 event notification configuration
type S3EventConfigManager struct {
	Config S3EventNotificationConfig
}

// S3EventTester provides functionality to test S3 event delivery to SQS queues
type S3EventTester struct {
	S3Client  interface{} // Will be *s3.Client in real implementation
	SQSClient interface{} // Will be *sqs.Client in real implementation
	Config    *S3EventConfigManager
}

// SQSMessageSender handles SQS message creation and sending
type SQSMessageSender struct {
	SQSClient interface{} // Will be *sqs.Client in real implementation
	Config    *S3EventConfigManager
}

// MultiCustomerUploadManager handles multi-customer uploads with S3 integration
type MultiCustomerUploadManager struct {
	S3Client interface{} // Mock S3 client
	Config   *S3EventConfigManager
}

// UploadResult represents the result of an S3 upload operation
type UploadResult struct {
	Success      bool   `json:"success"`
	Key          string `json:"key,omitempty"`
	Error        string `json:"error,omitempty"`
	CustomerCode string `json:"customer_code,omitempty"`
}

// MultiCustomerUploadResults represents the results of a multi-customer upload
type MultiCustomerUploadResults struct {
	CustomerUploads map[string]UploadResult `json:"customerUploads"`
	ArchiveUpload   *UploadResult           `json:"archiveUpload,omitempty"`
}

// Wait blocks until a request can be made
func (rl *RateLimiter) Wait() {
	<-rl.Requests
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	rl.Ticker.Stop()
}
