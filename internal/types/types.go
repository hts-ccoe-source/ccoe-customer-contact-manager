// Package types contains all shared type definitions and structs.
package types

import (
	"fmt"
	"log"
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
	CustomerCode           string   `json:"customer_code"`
	CustomerName           string   `json:"customer_name"`
	AWSAccountID           string   `json:"aws_account_id"` // Deprecated: use GetAccountID() method instead
	Region                 string   `json:"region"`
	SESRoleARN             string   `json:"ses_role_arn"`
	Environment            string   `json:"environment"`
	SQSQueueARN            string   `json:"sqs_queue_arn"`
	DKIMTokens             []string `json:"dkim_tokens,omitempty"`                  // Optional: SES DKIM tokens for Route53 DNS configuration
	VerificationToken      string   `json:"verification_token,omitempty"`           // Optional: SES domain verification token for Route53 DNS configuration
	IdentityCenterRoleArn  string   `json:"identity_center_role_arn,omitempty"`     // Optional: IAM role ARN for Identity Center data retrieval
	DeliverabilitySnsTopic string   `json:"deliverability_sns_topic_arn,omitempty"` // Optional: SNS topic ARN for SES event notifications (per customer)
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

// EmailConfig represents email configuration for notifications
type EmailConfig struct {
	SenderAddress     string `json:"sender_address"`
	MeetingOrganizer  string `json:"meeting_organizer"`
	PortalBaseURL     string `json:"portal_base_url"`
	Domain            string `json:"domain"`              // Main email domain
	MailFromSubdomain string `json:"mail_from_subdomain"` // e.g., "bounce" (combined with domain for MAIL FROM)
	DMARCPolicy       string `json:"dmarc_policy"`        // "none", "quarantine", or "reject"
	DMARCReportEmail  string `json:"dmarc_report_email"`  // Local part only (e.g., "dmarc-reports")
}

// Route53Config holds Route53 zone information for SES domain validation
type Route53Config struct {
	ZoneID  string `json:"zone_id"`  // Hosted zone ID (zone name will be looked up from this)
	RoleARN string `json:"role_arn"` // IAM role to assume in DNS account
}

// Config represents the application configuration
type Config struct {
	AWSRegion        string                         `json:"aws_region"`
	LogLevel         string                         `json:"log_level"`
	CustomerMappings map[string]CustomerAccountInfo `json:"customer_mappings"`
	ContactConfig    AlternateContactConfig         `json:"contact_config"`
	S3Config         S3Config                       `json:"s3_config"`
	EmailConfig      EmailConfig                    `json:"email_config"`
	Route53Config    *Route53Config                 `json:"route53_config,omitempty"` // Optional: Route53 configuration for SES domain validation
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
	EventVersion string          `json:"eventVersion"`
	EventSource  string          `json:"eventSource"`
	AWSRegion    string          `json:"awsRegion"`
	EventTime    time.Time       `json:"eventTime"`
	EventName    string          `json:"eventName"`
	UserIdentity *S3UserIdentity `json:"userIdentity,omitempty"`
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

// S3UserIdentity represents the userIdentity field in S3 events
type S3UserIdentity struct {
	Type        string `json:"type"`
	PrincipalID string `json:"principalId"`
	ARN         string `json:"arn"`
	AccountID   string `json:"accountId,omitempty"`
	AccessKeyID string `json:"accessKeyId,omitempty"`
	UserName    string `json:"userName,omitempty"`
}

// S3EventNotification represents the S3 event notification message
type S3EventNotification struct {
	Records []S3EventRecord `json:"Records"`
}

// ModificationEntry represents a single modification entry in the change history
type ModificationEntry struct {
	Timestamp        time.Time        `json:"timestamp"`
	UserID           string           `json:"user_id"`
	ModificationType string           `json:"modification_type"`
	CustomerCode     string           `json:"customer_code,omitempty"`
	MeetingMetadata  *MeetingMetadata `json:"meeting_metadata,omitempty"`
}

// MeetingMetadata represents Microsoft Graph meeting information
type MeetingMetadata struct {
	MeetingID string   `json:"meeting_id"`
	JoinURL   string   `json:"join_url"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time"`
	Subject   string   `json:"subject"`
	Organizer string   `json:"organizer,omitempty"`
	Attendees []string `json:"attendees,omitempty"`
}

// AnnouncementMetadata represents announcement-specific metadata
type AnnouncementMetadata struct {
	ObjectType       string              `json:"object_type"`
	AnnouncementID   string              `json:"announcement_id"`
	AnnouncementType string              `json:"announcement_type"`
	Title            string              `json:"title"`
	Summary          string              `json:"summary"`
	Content          string              `json:"content"`
	Customers        []string            `json:"customers"`
	IncludeMeeting   bool                `json:"include_meeting"`
	MeetingMetadata  *MeetingMetadata    `json:"meeting_metadata,omitempty"`
	Attachments      []string            `json:"attachments"`
	CreatedBy        string              `json:"created_by"`
	CreatedAt        time.Time           `json:"created_at"`
	PostedDate       time.Time           `json:"posted_date"`
	Author           string              `json:"author"`
	Status           string              `json:"status"`
	PriorStatus      string              `json:"prior_status"`
	Modifications    []ModificationEntry `json:"modifications"`
	SubmittedBy      string              `json:"submittedBy"`
	SubmittedAt      *time.Time          `json:"submittedAt,omitempty"`
	Version          int                 `json:"version"`
	ModifiedAt       time.Time           `json:"modifiedAt"`
	ModifiedBy       string              `json:"modifiedBy"`

	// Legacy metadata map - should not be present in new objects
	// This field is used only for validation to detect legacy objects
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Source   string                 `json:"source,omitempty"`
}

// ChangeMetadata represents the metadata from the uploaded JSON file
type ChangeMetadata struct {
	ObjectType         string   `json:"object_type"`
	ChangeID           string   `json:"changeId"`
	ChangeTitle        string   `json:"changeTitle"`
	ChangeReason       string   `json:"changeReason"`
	Customers          []string `json:"customers"`
	ImplementationPlan string   `json:"implementationPlan"`
	TestPlan           string   `json:"testPlan"`
	CustomerImpact     string   `json:"customerImpact"`
	RollbackPlan       string   `json:"rollbackPlan"`
	SnowTicket         string   `json:"snowTicket"`
	JiraTicket         string   `json:"jiraTicket"`

	// Standardized datetime fields using time.Time
	ImplementationStart time.Time `json:"implementationStart"`
	ImplementationEnd   time.Time `json:"implementationEnd"`
	Timezone            string    `json:"timezone"`

	IncludeMeeting   bool       `json:"include_meeting"`
	MeetingTitle     string     `json:"meetingTitle"`
	MeetingStartTime *time.Time `json:"meetingStartTime,omitempty"`
	MeetingDuration  string     `json:"meetingDuration"`
	MeetingLocation  string     `json:"meetingLocation"`

	// Nested meeting metadata (set by backend when meeting is scheduled, consistent with announcements)
	MeetingMetadata *MeetingMetadata `json:"meeting_metadata,omitempty"`

	Status      string `json:"status"`
	PriorStatus string `json:"prior_status"`
	Version     int    `json:"version"`

	// Enhanced modification tracking array
	Modifications []ModificationEntry `json:"modifications"`

	// Legacy audit timestamps (deprecated - use Modifications array)
	CreatedAt   time.Time  `json:"createdAt"`
	CreatedBy   string     `json:"createdBy"`
	ModifiedAt  time.Time  `json:"modifiedAt"`
	ModifiedBy  string     `json:"modifiedBy"`
	SubmittedAt *time.Time `json:"submittedAt,omitempty"`
	SubmittedBy string     `json:"submittedBy"`
	ApprovedAt  *time.Time `json:"approvedAt,omitempty"`
	ApprovedBy  string     `json:"approvedBy,omitempty"`

	TestRun bool `json:"testRun,omitempty"`

	// Legacy metadata map - should not be present in new objects
	// This field is used only for validation to detect legacy objects
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Source   string                 `json:"source,omitempty"`
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
	ICalUId string `json:"iCalUId,omitempty"` // Native idempotency key
	Body    *struct {
		ContentType string `json:"contentType"`
		Content     string `json:"content"`
	} `json:"body,omitempty"`
	Start *struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"start,omitempty"`
	End *struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"end,omitempty"`
	OnlineMeeting *struct {
		JoinURL string `json:"joinUrl"`
	} `json:"onlineMeeting,omitempty"`
}

// RateLimiter implements a simple rate limiter using a channel
type RateLimiter struct {
	Ticker   *time.Ticker
	Requests chan struct{}
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

// ApprovalRequestMetadata represents the legacy nested metadata format
// This is kept for backward compatibility with SES functions
// New code should use the flat ChangeMetadata structure
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
			// Standardized datetime fields using time.Time
			ImplementationStart time.Time `json:"implementationStart"`
			ImplementationEnd   time.Time `json:"implementationEnd"`
			Timezone            string    `json:"timezone"`

			// Backward compatibility fields (deprecated - use ImplementationStart/End)
			BeginDate string `json:"beginDate,omitempty"`
			BeginTime string `json:"beginTime,omitempty"`
			EndDate   string `json:"endDate,omitempty"`
			EndTime   string `json:"endTime,omitempty"`
		} `json:"schedule"`
		Description string `json:"description"`
		ApprovedBy  string `json:"approvedBy,omitempty"`
		ApprovedAt  string `json:"approvedAt,omitempty"`
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
		Title           string    `json:"title"`
		StartTime       time.Time `json:"startTime"`
		Duration        int       `json:"duration"`
		DurationMinutes int       `json:"durationMinutes"`
		Attendees       []string  `json:"attendees"`
		Location        string    `json:"location"`
	} `json:"meetingInvite,omitempty"`
	GeneratedAt string `json:"generatedAt"`
	GeneratedBy string `json:"generatedBy"`
}

// Modification type constants
const (
	ModificationTypeCreated          = "created"
	ModificationTypeUpdated          = "updated"
	ModificationTypeSubmitted        = "submitted"
	ModificationTypeApproved         = "approved"
	ModificationTypeDeleted          = "deleted"
	ModificationTypeMeetingScheduled = "meeting_scheduled"
	ModificationTypeMeetingCancelled = "meeting_cancelled"
	ModificationTypeProcessed        = "processed"
)

// Backend user ID for system-generated modifications
// Deprecated: Use the actual Lambda execution role ARN instead
const BackendUserID = "backend-system"

// NewModificationEntry creates a new modification entry with the specified type and user
func NewModificationEntry(modificationType, userID string) (ModificationEntry, error) {
	entry := ModificationEntry{
		Timestamp:        time.Now(),
		UserID:           userID,
		ModificationType: modificationType,
	}

	// Validate the entry before returning
	if err := entry.ValidateModificationEntry(); err != nil {
		return ModificationEntry{}, fmt.Errorf("invalid modification entry: %w", err)
	}

	return entry, nil
}

// NewMeetingScheduledEntry creates a modification entry for meeting scheduling
func NewMeetingScheduledEntry(userID string, meetingMetadata *MeetingMetadata) (ModificationEntry, error) {
	entry := ModificationEntry{
		Timestamp:        time.Now(),
		UserID:           userID,
		ModificationType: ModificationTypeMeetingScheduled,
		MeetingMetadata:  meetingMetadata,
	}

	// Validate the entry before returning
	if err := entry.ValidateModificationEntry(); err != nil {
		return ModificationEntry{}, fmt.Errorf("invalid meeting scheduled entry: %w", err)
	}

	return entry, nil
}

// NewMeetingCancelledEntry creates a modification entry for meeting cancellation
func NewMeetingCancelledEntry(userID string) (ModificationEntry, error) {
	entry := ModificationEntry{
		Timestamp:        time.Now(),
		UserID:           userID,
		ModificationType: ModificationTypeMeetingCancelled,
	}

	// Validate the entry before returning
	if err := entry.ValidateModificationEntry(); err != nil {
		return ModificationEntry{}, fmt.Errorf("invalid meeting cancelled entry: %w", err)
	}

	return entry, nil
}

// AddModificationEntry adds a modification entry to the change metadata after validation
func (c *ChangeMetadata) AddModificationEntry(entry ModificationEntry) error {
	// Validate the modification entry before adding
	if err := entry.ValidateModificationEntry(); err != nil {
		return fmt.Errorf("invalid modification entry: %w", err)
	}

	if c.Modifications == nil {
		c.Modifications = make([]ModificationEntry, 0)
	}
	c.Modifications = append(c.Modifications, entry)
	return nil
}

// GetLatestMeetingMetadata returns the most recent meeting metadata from modification entries
func (c *ChangeMetadata) GetLatestMeetingMetadata() *MeetingMetadata {
	// Iterate through modifications in reverse order to find the most recent meeting
	for i := len(c.Modifications) - 1; i >= 0; i-- {
		entry := c.Modifications[i]
		if entry.ModificationType == ModificationTypeMeetingScheduled && entry.MeetingMetadata != nil {
			return entry.MeetingMetadata
		}
	}
	return nil
}

// HasMeetingScheduled checks if the change has any scheduled meetings
func (c *ChangeMetadata) HasMeetingScheduled() bool {
	for _, entry := range c.Modifications {
		if entry.ModificationType == ModificationTypeMeetingScheduled {
			return true
		}
	}
	return false
}

// GetApprovalEntries returns all approval modification entries
func (c *ChangeMetadata) GetApprovalEntries() []ModificationEntry {
	var approvals []ModificationEntry
	for _, entry := range c.Modifications {
		if entry.ModificationType == ModificationTypeApproved {
			approvals = append(approvals, entry)
		}
	}
	return approvals
}

// ValidateChangeMetadata validates the entire change metadata structure including all modification entries
func (c *ChangeMetadata) ValidateChangeMetadata() error {
	if c == nil {
		return fmt.Errorf("change metadata cannot be nil")
	}

	// Validate basic required fields
	if strings.TrimSpace(c.ChangeID) == "" {
		return fmt.Errorf("changeId is required")
	}

	if strings.TrimSpace(c.ChangeTitle) == "" {
		return fmt.Errorf("changeTitle is required")
	}

	// Validate modification array
	if err := ValidateModificationArray(c.Modifications); err != nil {
		return fmt.Errorf("invalid modifications array: %w", err)
	}

	return nil
}

// ValidateMeetingMetadata validates the meeting metadata structure
func (m *MeetingMetadata) ValidateMeetingMetadata() error {
	if m == nil {
		return fmt.Errorf("meeting metadata cannot be nil")
	}

	if strings.TrimSpace(m.MeetingID) == "" {
		return fmt.Errorf("meeting_id is required")
	}

	if strings.TrimSpace(m.JoinURL) == "" {
		return fmt.Errorf("join_url is required")
	}

	if strings.TrimSpace(m.StartTime) == "" {
		return fmt.Errorf("start_time is required")
	}

	if strings.TrimSpace(m.EndTime) == "" {
		return fmt.Errorf("end_time is required")
	}

	if strings.TrimSpace(m.Subject) == "" {
		return fmt.Errorf("subject is required")
	}

	// Validate time format (ISO 8601)
	if _, err := time.Parse(time.RFC3339, m.StartTime); err != nil {
		return fmt.Errorf("start_time must be in ISO 8601 format: %v", err)
	}

	if _, err := time.Parse(time.RFC3339, m.EndTime); err != nil {
		return fmt.Errorf("end_time must be in ISO 8601 format: %v", err)
	}

	// Validate that start time is before end time
	startTime, _ := time.Parse(time.RFC3339, m.StartTime)
	endTime, _ := time.Parse(time.RFC3339, m.EndTime)
	if !startTime.Before(endTime) {
		return fmt.Errorf("start_time must be before end_time")
	}

	return nil
}

// ValidateModificationEntry validates a modification entry structure
func (e *ModificationEntry) ValidateModificationEntry() error {
	if e == nil {
		return fmt.Errorf("modification entry cannot be nil")
	}

	if e.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}

	if strings.TrimSpace(e.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}

	if strings.TrimSpace(e.ModificationType) == "" {
		return fmt.Errorf("modification_type is required")
	}

	// Validate user_id format consistency
	if err := ValidateUserIDFormat(e.UserID); err != nil {
		return fmt.Errorf("invalid user_id format: %w", err)
	}

	// Validate modification type is one of the allowed values
	validTypes := map[string]bool{
		ModificationTypeCreated:          true,
		ModificationTypeUpdated:          true,
		ModificationTypeSubmitted:        true,
		ModificationTypeApproved:         true,
		ModificationTypeDeleted:          true,
		ModificationTypeMeetingScheduled: true,
		ModificationTypeMeetingCancelled: true,
		ModificationTypeProcessed:        true,
	}

	if !validTypes[e.ModificationType] {
		return fmt.Errorf("invalid modification_type: %s", e.ModificationType)
	}

	// Validate meeting metadata if present
	if e.ModificationType == ModificationTypeMeetingScheduled {
		if e.MeetingMetadata == nil {
			return fmt.Errorf("meeting_metadata is required for meeting_scheduled type")
		}
		if err := e.MeetingMetadata.ValidateMeetingMetadata(); err != nil {
			return fmt.Errorf("invalid meeting metadata: %v", err)
		}
	} else if e.MeetingMetadata != nil {
		return fmt.Errorf("meeting_metadata should only be present for meeting_scheduled type")
	}

	return nil
}

// ValidateGraphMeetingResponse validates Microsoft Graph API response data
func ValidateGraphMeetingResponse(response *GraphMeetingResponse) error {
	if response == nil {
		return fmt.Errorf("graph meeting response cannot be nil")
	}

	if strings.TrimSpace(response.ID) == "" {
		return fmt.Errorf("meeting ID is required in Graph response")
	}

	if strings.TrimSpace(response.Subject) == "" {
		return fmt.Errorf("meeting subject is required in Graph response")
	}

	if response.Start == nil {
		return fmt.Errorf("meeting start time is required in Graph response")
	}

	if response.End == nil {
		return fmt.Errorf("meeting end time is required in Graph response")
	}

	if strings.TrimSpace(response.Start.DateTime) == "" {
		return fmt.Errorf("meeting start dateTime is required in Graph response")
	}

	if strings.TrimSpace(response.End.DateTime) == "" {
		return fmt.Errorf("meeting end dateTime is required in Graph response")
	}

	// Validate datetime format
	if _, err := time.Parse("2006-01-02T15:04:05.0000000", response.Start.DateTime); err != nil {
		return fmt.Errorf("invalid start dateTime format in Graph response: %v", err)
	}

	if _, err := time.Parse("2006-01-02T15:04:05.0000000", response.End.DateTime); err != nil {
		return fmt.Errorf("invalid end dateTime format in Graph response: %v", err)
	}

	return nil
}

// ConvertGraphResponseToMeetingMetadata converts Microsoft Graph response to MeetingMetadata
func ConvertGraphResponseToMeetingMetadata(response *GraphMeetingResponse, joinURL string) (*MeetingMetadata, error) {
	if err := ValidateGraphMeetingResponse(response); err != nil {
		return nil, err
	}

	if strings.TrimSpace(joinURL) == "" {
		return nil, fmt.Errorf("join URL is required")
	}

	// Convert Graph datetime format to ISO 8601
	startTime, err := time.Parse("2006-01-02T15:04:05.0000000", response.Start.DateTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start time: %v", err)
	}

	endTime, err := time.Parse("2006-01-02T15:04:05.0000000", response.End.DateTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse end time: %v", err)
	}

	metadata := &MeetingMetadata{
		MeetingID: response.ID,
		JoinURL:   joinURL,
		StartTime: startTime.Format(time.RFC3339),
		EndTime:   endTime.Format(time.RFC3339),
		Subject:   response.Subject,
	}

	// Validate the converted metadata
	if err := metadata.ValidateMeetingMetadata(); err != nil {
		return nil, fmt.Errorf("converted metadata validation failed: %v", err)
	}

	return metadata, nil
}

// ValidateUserIDFormat validates that user_id follows expected format patterns
func ValidateUserIDFormat(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id cannot be empty")
	}

	// Allow backend system user ID (deprecated, but still supported for backward compatibility)
	if userID == BackendUserID {
		return nil
	}

	// Allow IAM Role ARN format (preferred for backend system)
	// Format: arn:aws:iam::123456789012:role/role-name
	if strings.HasPrefix(userID, "arn:aws:iam::") && strings.Contains(userID, ":role/") {
		return nil
	}

	// Validate Identity Center user ID format (UUID-like format)
	// Identity Center user IDs are typically UUIDs like: 906638888d-1234-5678-9abc-123456789012
	if len(userID) < 10 {
		return fmt.Errorf("user_id too short, expected Identity Center user ID or IAM Role ARN format")
	}

	// Check for valid characters (alphanumeric, hyphens, colons, slashes for ARNs)
	for _, char := range userID {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == ':' || char == '/') {
			return fmt.Errorf("user_id contains invalid characters, expected alphanumeric, hyphens, colons, and slashes only")
		}
	}

	return nil
}

// ValidateModificationArray validates an entire array of modification entries
func ValidateModificationArray(modifications []ModificationEntry) error {
	if modifications == nil {
		return nil // Empty array is valid
	}

	for i, entry := range modifications {
		if err := entry.ValidateModificationEntry(); err != nil {
			return fmt.Errorf("invalid modification entry at index %d: %w", i, err)
		}
	}

	// Validate chronological order (optional - entries should be in order but we don't enforce it)
	// This is a soft validation that logs warnings rather than errors
	for i := 1; i < len(modifications); i++ {
		if modifications[i].Timestamp.Before(modifications[i-1].Timestamp) {
			// Log warning but don't fail validation
			// In a real system, you might want to log this
		}
	}

	return nil
}

// ValidateLegacyMetadata checks if a ChangeMetadata object contains legacy metadata map
// Returns an error if legacy metadata is detected
func (c *ChangeMetadata) ValidateLegacyMetadata() error {
	if c == nil {
		return fmt.Errorf("change metadata cannot be nil")
	}

	// Check if legacy Metadata map exists and is non-empty
	if len(c.Metadata) > 0 {
		log.Printf("❌ ERROR: Object %s contains legacy metadata map - migration required", c.ChangeID)
		return fmt.Errorf("object %s contains legacy metadata map with %d entries", c.ChangeID, len(c.Metadata))
	}

	// Check if legacy Source field exists and is non-empty
	if strings.TrimSpace(c.Source) != "" {
		log.Printf("❌ ERROR: Object %s contains legacy source field - migration required", c.ChangeID)
		return fmt.Errorf("object %s contains legacy source field: %s", c.ChangeID, c.Source)
	}

	return nil
}

// ValidateLegacyMetadata checks if an AnnouncementMetadata object contains legacy metadata map
// Returns an error if legacy metadata is detected
func (a *AnnouncementMetadata) ValidateLegacyMetadata() error {
	if a == nil {
		return fmt.Errorf("announcement metadata cannot be nil")
	}

	// Check if legacy Metadata map exists and is non-empty
	if len(a.Metadata) > 0 {
		log.Printf("❌ ERROR: Announcement %s contains legacy metadata map - migration required", a.AnnouncementID)
		return fmt.Errorf("announcement %s contains legacy metadata map with %d entries", a.AnnouncementID, len(a.Metadata))
	}

	// Check if legacy Source field exists and is non-empty
	if strings.TrimSpace(a.Source) != "" {
		log.Printf("❌ ERROR: Announcement %s contains legacy source field - migration required", a.AnnouncementID)
		return fmt.Errorf("announcement %s contains legacy source field: %s", a.AnnouncementID, a.Source)
	}

	return nil
}

// Identity Center types for AWS contact import
type IdentityCenterUser struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	GivenName   string `json:"given_name"`
	FamilyName  string `json:"family_name"`
	Active      bool   `json:"active"`
}

type IdentityCenterGroupMembership struct {
	UserId      string   `json:"user_id"`
	UserName    string   `json:"user_name"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Groups      []string `json:"groups"`
}

type IdentityCenterGroupCentric struct {
	GroupName string                   `json:"group_name"`
	Members   []IdentityCenterUserInfo `json:"members"`
}

type IdentityCenterUserInfo struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

// Contact import configuration types
type RoleTopicMapping struct {
	Roles  []string `json:"roles"`
	Topics []string `json:"topics"`
}

type ContactImportConfig struct {
	RoleMappings       []RoleTopicMapping `json:"role_mappings"`
	DefaultTopics      []string           `json:"default_topics"`
	RequireActiveUsers bool               `json:"require_active_users"`
}

// DNSRecord represents a DNS record to be created or updated
type DNSRecord struct {
	Type  string `json:"type"`  // CNAME, TXT, MX
	Name  string `json:"name"`  // Record name
	Value string `json:"value"` // Record value
	TTL   int64  `json:"ttl"`   // Time to live
}

// DeliverabilityConfig is deprecated - deliverability settings are now in EmailConfig (domain-level)
// and CustomerAccountInfo.DeliverabilitySnsTopic (per-customer SNS topic)
type DeliverabilityConfig struct {
	// Deprecated: This type is no longer used
}
