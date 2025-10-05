// Package ses provides email templates, Microsoft Graph integration, and S3 payload processing.
package ses

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// This file contains functions extracted from aws-alternate-contact-manager-original.go
// that are missing from the modular codebase. These functions provide:
// 1. Microsoft Graph meeting functionality
// 2. Email template functionality for approval requests and announcements
// 3. ICS calendar invite functionality
// 4. Supporting helper functions

// Microsoft Graph API structures
type GraphAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
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
	WebLink string `json:"webLink"`
	Start   struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"start"`
	End struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"end"`
	Location struct {
		DisplayName string `json:"displayName"`
	} `json:"location"`
	Body struct {
		Content     string `json:"content"`
		ContentType string `json:"contentType"`
	} `json:"body"`
	Attendees []struct {
		EmailAddress struct {
			Address string `json:"address"`
			Name    string `json:"name"`
		} `json:"emailAddress"`
		Type string `json:"type"`
	} `json:"attendees"`
	OnlineMeeting struct {
		JoinURL string `json:"joinUrl"`
	} `json:"onlineMeeting"`
}

// ApprovalRequestMetadata represents the structure of approval request metadata
// This type should be moved to internal/types eventually
type ApprovalRequestMetadata struct {
	GeneratedAt       string            `json:"generated_at"`
	ChangeMetadata    ChangeMetadata    `json:"change_metadata"`
	EmailNotification EmailNotification `json:"email_notification"`
	MeetingInvite     *MeetingInvite    `json:"meeting_invite,omitempty"`
}

type ChangeMetadata struct {
	Title                  string   `json:"title"`
	CustomerNames          []string `json:"customer_names"`
	CustomerCodes          []string `json:"customer_codes"`
	ChangeReason           string   `json:"change_reason"`
	ImplementationPlan     string   `json:"implementation_plan"`
	TestPlan               string   `json:"test_plan"`
	ExpectedCustomerImpact string   `json:"expected_customer_impact"`
	RollbackPlan           string   `json:"rollback_plan"`
	Schedule               Schedule `json:"schedule"`
	Tickets                Tickets  `json:"tickets"`
}

type Schedule struct {
	BeginDate           string `json:"begin_date"`
	BeginTime           string `json:"begin_time"`
	EndDate             string `json:"end_date"`
	EndTime             string `json:"end_time"`
	Timezone            string `json:"timezone"`
	ImplementationStart string `json:"implementation_start"`
	ImplementationEnd   string `json:"implementation_end"`
}

type Tickets struct {
	ServiceNow string `json:"servicenow"`
	Jira       string `json:"jira"`
}

type EmailNotification struct {
	Subject string `json:"subject"`
}

type MeetingInvite struct {
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	Duration  int    `json:"duration"`
	Location  string `json:"location"`
}

// CreateMeetingInvite creates a meeting using Microsoft Graph API based on metadata
func CreateMeetingInvite(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, senderEmail string, dryRun bool, forceUpdate bool) error {
	// Validate required parameters
	if topicName == "" {
		return fmt.Errorf("topic name is required for create-meeting-invite action")
	}
	if jsonMetadataPath == "" {
		return fmt.Errorf("json-metadata file path is required for create-meeting-invite action")
	}
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for create-meeting-invite action")
	}

	// Load metadata from JSON file
	metadata, err := loadApprovalMetadata(jsonMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Check if meeting information exists
	if metadata.MeetingInvite == nil {
		return fmt.Errorf("no meeting information found in metadata - meeting invite cannot be created")
	}

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get contacts subscribed to the specified topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &types.ListContactsFilter{
			FilteredStatus: types.SubscriptionStatusOptIn,
			TopicFilter: &types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	if len(contactsResult.Contacts) == 0 {
		fmt.Printf("⚠️  No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	// Extract attendee emails
	var attendeeEmails []string
	for _, contact := range contactsResult.Contacts {
		attendeeEmails = append(attendeeEmails, *contact.EmailAddress)
	}

	fmt.Printf("📅 Creating Microsoft Graph meeting for topic '%s' (%d attendees)\n", topicName, len(contactsResult.Contacts))
	fmt.Printf("📋 Using SES contact list: %s\n", accountListName)
	fmt.Printf("📄 Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("🕐 Meeting: %s\n", metadata.MeetingInvite.Title)

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No meeting will be created\n")
		fmt.Printf("📊 Meeting Invite Summary (DRY RUN):\n")
		fmt.Printf("   📧 Would create meeting for: %d attendees\n", len(contactsResult.Contacts))
		fmt.Printf("   📋 Attendees:\n")
		for _, contact := range contactsResult.Contacts {
			fmt.Printf("      - %s\n", *contact.EmailAddress)
		}
		return nil
	}

	// Create meeting request payload
	meetingPayload, err := generateGraphMeetingPayload(metadata, senderEmail, attendeeEmails)
	if err != nil {
		return fmt.Errorf("failed to generate meeting payload: %w", err)
	}

	// Create the meeting using Microsoft Graph API
	action, err := createGraphMeeting(meetingPayload, senderEmail, forceUpdate)
	if err != nil {
		return fmt.Errorf("failed to create Microsoft Graph meeting: %w", err)
	}

	// Only show additional success messages if a meeting was actually created
	if action == "created" {
		fmt.Printf("   ✅ Successfully created Microsoft Graph meeting for %d attendees\n", len(attendeeEmails))
		fmt.Printf("\n📊 Meeting Invite Summary:\n")
		fmt.Printf("   📧 Meeting created for: %d attendees\n", len(attendeeEmails))
		fmt.Printf("   📋 Meeting created via Microsoft Graph API\n")
	}

	return nil
}

// SendApprovalRequest sends an approval request email using metadata and template
func SendApprovalRequest(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, htmlTemplatePath string, senderEmail string, dryRun bool) error {
	// Validate required parameters
	if topicName == "" {
		return fmt.Errorf("topic name is required for send-approval-request action")
	}
	if jsonMetadataPath == "" {
		return fmt.Errorf("json-metadata file path is required for send-approval-request action")
	}
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for send-approval-request action")
	}

	// Load metadata from JSON file
	metadata, err := loadApprovalMetadata(jsonMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Generate or load HTML template
	var htmlContent string
	if htmlTemplatePath != "" {
		htmlContent, err = loadHtmlTemplate(htmlTemplatePath)
		if err != nil {
			return fmt.Errorf("failed to load HTML template: %w", err)
		}
	} else {
		htmlContent = generateDefaultHtmlTemplate(metadata)
	}

	// Process template with metadata
	processedHtml := processTemplate(htmlContent, metadata, topicName)

	// Create subject with question mark emoji and change "Notification:" to "Approval:"
	originalSubject := metadata.EmailNotification.Subject
	modifiedSubject := strings.Replace(originalSubject, "ITSM Change Notification:", "Change Approval:", 1)
	subject := fmt.Sprintf("❓ %s", modifiedSubject)

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get contacts subscribed to the specified topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &types.ListContactsFilter{
			FilteredStatus: types.SubscriptionStatusOptIn,
			TopicFilter: &types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	if len(contactsResult.Contacts) == 0 {
		fmt.Printf("⚠️  No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	fmt.Printf("📧 Sending approval request to topic '%s' (%d subscribers)\n", topicName, len(contactsResult.Contacts))
	fmt.Printf("📋 Using SES contact list: %s\n", accountListName)
	fmt.Printf("📄 Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("👤 Customer: %s\n", strings.Join(metadata.ChangeMetadata.CustomerNames, ", "))

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No emails will be sent\n")
		fmt.Printf("📊 Approval Request Summary (DRY RUN):\n")
		fmt.Printf("   📧 Would send to: %d recipients\n", len(contactsResult.Contacts))
		fmt.Printf("   📋 Recipients:\n")
		for _, contact := range contactsResult.Contacts {
			fmt.Printf("      - %s\n", *contact.EmailAddress)
		}
		return nil
	}

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(subject),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data: aws.String(processedHtml),
					},
				},
			},
		},
		ListManagementOptions: &types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	successCount := 0
	errorCount := 0

	// Send to each subscribed contact
	for _, contact := range contactsResult.Contacts {
		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			fmt.Printf("   ❌ Failed to send to %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			fmt.Printf("   ✅ Sent to %s\n", *contact.EmailAddress)
			successCount++
		}
	}

	fmt.Printf("\n📊 Approval Request Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   📋 Total recipients: %d\n", len(contactsResult.Contacts))

	if errorCount > 0 {
		return fmt.Errorf("failed to send approval request to %d recipients", errorCount)
	}

	return nil
}

// SendChangeNotificationWithTemplate sends a change notification email indicating the change has been approved and scheduled
func SendChangeNotificationWithTemplate(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, senderEmail string, dryRun bool) error {
	// Validate required parameters
	if topicName == "" {
		return fmt.Errorf("topic name is required for send-change-notification action")
	}
	if jsonMetadataPath == "" {
		return fmt.Errorf("json-metadata file path is required for send-change-notification action")
	}
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for send-change-notification action")
	}

	// Load metadata from JSON file
	metadata, err := loadApprovalMetadata(jsonMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Generate HTML content for change notification
	htmlContent := generateChangeNotificationHtml(metadata)

	// Process template with metadata to handle macros like {{amazonSESUnsubscribeUrl}}
	processedHtml := processTemplate(htmlContent, metadata, topicName)

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts that should receive emails for this topic (explicit opt-in + default opt-in)
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		fmt.Printf("⚠️  No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	// Create subject with "APPROVED" prefix and shorten "Notification:" to make it more concise
	originalSubject := metadata.EmailNotification.Subject
	shortenedSubject := strings.Replace(originalSubject, "ITSM Change Notification:", "ITSM Change:", 1)
	subject := fmt.Sprintf("✅ APPROVED %s", shortenedSubject)

	fmt.Printf("📧 Sending change notification to topic '%s' (%d subscribers)\n", topicName, len(subscribedContacts))
	fmt.Printf("📋 Using SES contact list: %s\n", accountListName)
	fmt.Printf("📄 Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("👤 Customer: %s\n", strings.Join(metadata.ChangeMetadata.CustomerNames, ", "))

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No emails will be sent\n")
		fmt.Printf("📊 Change Notification Summary (DRY RUN):\n")
		fmt.Printf("   📧 Would send to: %d recipients\n", len(subscribedContacts))
		fmt.Printf("   📋 Recipients:\n")
		for _, contact := range subscribedContacts {
			fmt.Printf("      - %s\n", *contact.EmailAddress)
		}
		return nil
	}

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(subject),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data: aws.String(processedHtml),
					},
				},
			},
		},
		ListManagementOptions: &types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	successCount := 0
	errorCount := 0

	// Send to each subscribed contact
	for _, contact := range subscribedContacts {
		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			fmt.Printf("   ❌ Failed to send to %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			fmt.Printf("   ✅ Sent to %s\n", *contact.EmailAddress)
			successCount++
		}
	}

	fmt.Printf("\n📊 Change Notification Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   📋 Total recipients: %d\n", len(subscribedContacts))

	if errorCount > 0 {
		return fmt.Errorf("failed to send change notification to %d recipients", errorCount)
	}

	return nil
}

// Helper functions for email templates and metadata processing

func loadApprovalMetadata(filePath string) (*ApprovalRequestMetadata, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata ApprovalRequestMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

func loadHtmlTemplate(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}
	return string(data), nil
}

func generateDefaultHtmlTemplate(metadata *ApprovalRequestMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Approval Request</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
    </style>
</head>
<body>
    <div class="header">
        <h2>❓ CHANGE APPROVAL REQUEST</h2>
        <p>This change has been reviewed, tentatively scheduled, and is ready for your approval.</p>
    </div>
    <div class="section">
        <h3>Change Details</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer:</strong> %s</p>
        <p><strong>Reason:</strong> %s</p>
        <p><strong>Implementation Plan:</strong> %s</p>
        <p><strong>Expected Impact:</strong> %s</p>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.ChangeReason,
		metadata.ChangeMetadata.ImplementationPlan,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
	)
}

func generateChangeNotificationHtml(metadata *ApprovalRequestMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Notification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #d4edda; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
    </style>
</head>
<body>
    <div class="header">
        <h2>✅ CHANGE APPROVED & SCHEDULED</h2>
        <p>This change has been approved and is scheduled for implementation.</p>
    </div>
    <div class="section">
        <h3>Change Details</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer:</strong> %s</p>
        <p><strong>Reason:</strong> %s</p>
        <p><strong>Implementation Plan:</strong> %s</p>
        <p><strong>Expected Impact:</strong> %s</p>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.ChangeReason,
		metadata.ChangeMetadata.ImplementationPlan,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
	)
}

func processTemplate(template string, metadata *ApprovalRequestMetadata, topicName string) string {
	// Simple template processing - replace common placeholders
	processed := template
	processed = strings.ReplaceAll(processed, "{{CHANGE_TITLE}}", metadata.ChangeMetadata.Title)
	processed = strings.ReplaceAll(processed, "{{CUSTOMER_NAME}}", strings.Join(metadata.ChangeMetadata.CustomerNames, ", "))
	processed = strings.ReplaceAll(processed, "{{CUSTOMER_CODES}}", strings.Join(metadata.ChangeMetadata.CustomerCodes, ", "))
	processed = strings.ReplaceAll(processed, "{{TOPIC_NAME}}", topicName)
	processed = strings.ReplaceAll(processed, "{{CHANGE_REASON}}", metadata.ChangeMetadata.ChangeReason)
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_PLAN}}", metadata.ChangeMetadata.ImplementationPlan)
	processed = strings.ReplaceAll(processed, "{{TEST_PLAN}}", metadata.ChangeMetadata.TestPlan)
	processed = strings.ReplaceAll(processed, "{{CUSTOMER_IMPACT}}", metadata.ChangeMetadata.ExpectedCustomerImpact)
	processed = strings.ReplaceAll(processed, "{{ROLLBACK_PLAN}}", metadata.ChangeMetadata.RollbackPlan)
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_START}}", metadata.ChangeMetadata.Schedule.ImplementationStart)
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_END}}", metadata.ChangeMetadata.Schedule.ImplementationEnd)
	processed = strings.ReplaceAll(processed, "{{TIMEZONE}}", metadata.ChangeMetadata.Schedule.Timezone)
	processed = strings.ReplaceAll(processed, "{{GENERATED_AT}}", metadata.GeneratedAt)

	return processed
}

// getSubscribedContactsForTopic gets all contacts that should receive emails for a topic
func getSubscribedContactsForTopic(sesClient *sesv2.Client, listName string, topicName string) ([]types.Contact, error) {
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
		Filter: &types.ListContactsFilter{
			FilteredStatus: types.SubscriptionStatusOptIn,
			TopicFilter: &types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	return contactsResult.Contacts, nil
}

// S3 Payload Processing Functions for Lambda Mode

// S3EventNotificationConfig represents S3 event notification configuration
type S3EventNotificationConfig struct {
	BucketName            string                       `json:"bucketName"`
	CustomerNotifications []CustomerNotificationConfig `json:"customerNotifications"`
}

// CustomerNotificationConfig represents customer-specific notification configuration
type CustomerNotificationConfig struct {
	CustomerCode string `json:"customerCode"`
	SQSQueueArn  string `json:"sqsQueueArn"`
	Prefix       string `json:"prefix"`
	Suffix       string `json:"suffix"`
}

// S3EventConfigManager handles S3 event notification configuration
type S3EventConfigManager struct {
	Config S3EventNotificationConfig
}

// NewS3EventConfigManager creates a new S3 event configuration manager
func NewS3EventConfigManager(bucketName string) *S3EventConfigManager {
	return &S3EventConfigManager{
		Config: S3EventNotificationConfig{
			BucketName:            bucketName,
			CustomerNotifications: make([]CustomerNotificationConfig, 0),
		},
	}
}

// AddCustomerNotification adds a customer notification configuration
func (m *S3EventConfigManager) AddCustomerNotification(customerCode, sqsQueueArn string) error {
	if customerCode == "" {
		return fmt.Errorf("customer code cannot be empty")
	}
	if sqsQueueArn == "" {
		return fmt.Errorf("SQS queue ARN cannot be empty")
	}

	// Check if customer already exists
	for _, notification := range m.Config.CustomerNotifications {
		if notification.CustomerCode == customerCode {
			return fmt.Errorf("customer code '%s' already exists", customerCode)
		}
	}

	// Add new customer notification
	notification := CustomerNotificationConfig{
		CustomerCode: customerCode,
		SQSQueueArn:  sqsQueueArn,
		Prefix:       fmt.Sprintf("customers/%s/", customerCode),
		Suffix:       ".json",
	}

	m.Config.CustomerNotifications = append(m.Config.CustomerNotifications, notification)
	return nil
}

// GetCustomerNotification gets notification config for a specific customer
func (m *S3EventConfigManager) GetCustomerNotification(customerCode string) (*CustomerNotificationConfig, error) {
	for _, notification := range m.Config.CustomerNotifications {
		if notification.CustomerCode == customerCode {
			return &notification, nil
		}
	}
	return nil, fmt.Errorf("customer code '%s' not found", customerCode)
}

// ValidateConfiguration validates the S3 event notification configuration
func (m *S3EventConfigManager) ValidateConfiguration() error {
	if m.Config.BucketName == "" {
		return fmt.Errorf("bucket name cannot be empty")
	}

	if len(m.Config.CustomerNotifications) == 0 {
		return fmt.Errorf("at least one customer notification must be configured")
	}

	// Validate each customer notification
	for _, notification := range m.Config.CustomerNotifications {
		if notification.CustomerCode == "" {
			return fmt.Errorf("customer code cannot be empty")
		}
		if notification.SQSQueueArn == "" {
			return fmt.Errorf("SQS queue ARN cannot be empty for customer '%s'", notification.CustomerCode)
		}
		if notification.Prefix == "" {
			return fmt.Errorf("prefix cannot be empty for customer '%s'", notification.CustomerCode)
		}
		if notification.Suffix == "" {
			return fmt.Errorf("suffix cannot be empty for customer '%s'", notification.CustomerCode)
		}
	}

	return nil
}

// SQSMessage represents the message format sent to customer SQS queues
type SQSMessage struct {
	ExecutionID  string                   `json:"execution_id"`
	ActionType   string                   `json:"action_type"`
	CustomerCode string                   `json:"customer_code"`
	Timestamp    string                   `json:"timestamp"`
	RetryCount   int                      `json:"retry_count"`
	Metadata     *ApprovalRequestMetadata `json:"metadata"`
}

// SQSMessageSender handles SQS message creation and sending
type SQSMessageSender struct {
	Config *S3EventConfigManager
}

// NewSQSMessageSender creates a new SQS message sender
func NewSQSMessageSender(config *S3EventConfigManager) *SQSMessageSender {
	return &SQSMessageSender{
		Config: config,
	}
}

// CreateSQSMessage creates a properly formatted SQS message for a customer
func (s *SQSMessageSender) CreateSQSMessage(customerCode, actionType string, metadata *ApprovalRequestMetadata) (*SQSMessage, error) {
	if customerCode == "" {
		return nil, fmt.Errorf("customer code cannot be empty")
	}
	if actionType == "" {
		return nil, fmt.Errorf("action type cannot be empty")
	}
	if metadata == nil {
		return nil, fmt.Errorf("metadata cannot be nil")
	}

	// Validate action type
	validActionTypes := []string{
		"send-change-notification",
		"send-approval-request",
		"create-meeting-invite",
		"create-ics-invite",
	}

	isValidAction := false
	for _, validType := range validActionTypes {
		if actionType == validType {
			isValidAction = true
			break
		}
	}

	if !isValidAction {
		return nil, fmt.Errorf("invalid action type: %s, valid types: %v", actionType, validActionTypes)
	}

	// Generate execution ID
	executionID := fmt.Sprintf("%s-%s-%d", customerCode, actionType, time.Now().Unix())

	message := &SQSMessage{
		ExecutionID:  executionID,
		ActionType:   actionType,
		CustomerCode: customerCode,
		Timestamp:    time.Now().Format(time.RFC3339),
		RetryCount:   0,
		Metadata:     metadata,
	}

	return message, nil
}

// ValidateSQSMessage validates an SQS message structure
func (s *SQSMessageSender) ValidateSQSMessage(message *SQSMessage) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	if message.ExecutionID == "" {
		return fmt.Errorf("execution ID cannot be empty")
	}

	if message.ActionType == "" {
		return fmt.Errorf("action type cannot be empty")
	}

	if message.CustomerCode == "" {
		return fmt.Errorf("customer code cannot be empty")
	}

	if message.Timestamp == "" {
		return fmt.Errorf("timestamp cannot be empty")
	}

	if message.Metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	// Validate timestamp format
	_, err := time.Parse(time.RFC3339, message.Timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %s", message.Timestamp)
	}

	// Validate retry count
	if message.RetryCount < 0 {
		return fmt.Errorf("retry count cannot be negative: %d", message.RetryCount)
	}

	return nil
}

// ExtractCustomerCodeFromS3Key extracts customer code from S3 object key
func ExtractCustomerCodeFromS3Key(s3Key string) (string, error) {
	// Expected format: customers/{customer-code}/filename.json
	parts := strings.Split(s3Key, "/")
	if len(parts) < 2 || parts[0] != "customers" {
		return "", fmt.Errorf("invalid S3 key format, expected customers/{customer-code}/filename.json")
	}
	return parts[1], nil
}

// ParseS3EventPayload parses S3 event payload and extracts metadata
func ParseS3EventPayload(eventPayload []byte) (*S3EventPayload, error) {
	var payload S3EventPayload
	if err := json.Unmarshal(eventPayload, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse S3 event payload: %w", err)
	}

	// Validate payload structure
	if len(payload.Records) == 0 {
		return nil, fmt.Errorf("S3 event payload contains no records")
	}

	for i, record := range payload.Records {
		if record.EventSource != "aws:s3" {
			return nil, fmt.Errorf("record %d is not an S3 event: %s", i, record.EventSource)
		}
		if record.S3.Bucket.Name == "" {
			return nil, fmt.Errorf("record %d missing bucket name", i)
		}
		if record.S3.Object.Key == "" {
			return nil, fmt.Errorf("record %d missing object key", i)
		}
	}

	return &payload, nil
}

// MapS3ChangeMetadataToEmailMessage maps S3 change metadata to email message format
func MapS3ChangeMetadataToEmailMessage(s3Key string, metadata *ApprovalRequestMetadata) (*EmailMessage, error) {
	if metadata == nil {
		return nil, fmt.Errorf("metadata cannot be nil")
	}

	// Extract customer code from S3 key
	customerCode, err := ExtractCustomerCodeFromS3Key(s3Key)
	if err != nil {
		return nil, fmt.Errorf("failed to extract customer code: %w", err)
	}

	// Create email message from metadata
	emailMessage := &EmailMessage{
		CustomerCode:       customerCode,
		Subject:            metadata.EmailNotification.Subject,
		ChangeTitle:        metadata.ChangeMetadata.Title,
		CustomerNames:      metadata.ChangeMetadata.CustomerNames,
		ChangeReason:       metadata.ChangeMetadata.ChangeReason,
		ImplementationPlan: metadata.ChangeMetadata.ImplementationPlan,
		ExpectedImpact:     metadata.ChangeMetadata.ExpectedCustomerImpact,
		Schedule: EmailSchedule{
			Start:    metadata.ChangeMetadata.Schedule.ImplementationStart,
			End:      metadata.ChangeMetadata.Schedule.ImplementationEnd,
			Timezone: metadata.ChangeMetadata.Schedule.Timezone,
		},
		Tickets: EmailTickets{
			ServiceNow: metadata.ChangeMetadata.Tickets.ServiceNow,
			Jira:       metadata.ChangeMetadata.Tickets.Jira,
		},
		GeneratedAt: metadata.GeneratedAt,
	}

	return emailMessage, nil
}

// S3EventPayload represents the structure of S3 event payload
type S3EventPayload struct {
	Records []S3EventRecord `json:"Records"`
}

// S3EventRecord represents a single S3 event record
type S3EventRecord struct {
	EventSource string `json:"eventSource"`
	EventName   string `json:"eventName"`
	S3          struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key  string `json:"key"`
			Size int64  `json:"size"`
		} `json:"object"`
	} `json:"s3"`
}

// EmailMessage represents the email message structure for notifications
type EmailMessage struct {
	CustomerCode       string        `json:"customer_code"`
	Subject            string        `json:"subject"`
	ChangeTitle        string        `json:"change_title"`
	CustomerNames      []string      `json:"customer_names"`
	ChangeReason       string        `json:"change_reason"`
	ImplementationPlan string        `json:"implementation_plan"`
	ExpectedImpact     string        `json:"expected_impact"`
	Schedule           EmailSchedule `json:"schedule"`
	Tickets            EmailTickets  `json:"tickets"`
	GeneratedAt        string        `json:"generated_at"`
}

// EmailSchedule represents schedule information in email messages
type EmailSchedule struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// EmailTickets represents ticket information in email messages
type EmailTickets struct {
	ServiceNow string `json:"servicenow"`
	Jira       string `json:"jira"`
}

// Microsoft Graph functionality

// generateGraphMeetingPayload creates the JSON payload for Microsoft Graph API
func generateGraphMeetingPayload(metadata *ApprovalRequestMetadata, organizerEmail string, attendeeEmails []string) (string, error) {
	// Parse start time and calculate end time
	startTime, endTime, err := calculateMeetingTimes(metadata)
	if err != nil {
		return "", err
	}

	// Build attendees array
	var attendees []map[string]interface{}
	for _, email := range attendeeEmails {
		attendees = append(attendees, map[string]interface{}{
			"emailAddress": map[string]string{
				"address": email,
				"name":    email,
			},
			"type": "required",
		})
	}

	// Create enhanced subject for idempotency (include ticket numbers for uniqueness)
	enhancedSubject := metadata.MeetingInvite.Title

	// Add ticket numbers with different formatting: ServiceNow in [brackets], JIRA in (parentheses)
	if metadata.ChangeMetadata.Tickets.ServiceNow != "" {
		enhancedSubject = fmt.Sprintf("%s [%s]", enhancedSubject, metadata.ChangeMetadata.Tickets.ServiceNow)
	}
	if metadata.ChangeMetadata.Tickets.Jira != "" {
		enhancedSubject = fmt.Sprintf("%s (%s)", enhancedSubject, metadata.ChangeMetadata.Tickets.Jira)
	}

	// Create meeting payload
	meetingData := map[string]interface{}{
		"subject": enhancedSubject,
		"body": map[string]interface{}{
			"contentType": "HTML",
			"content":     generateMeetingBodyHTML(metadata),
		},
		"start": map[string]string{
			"dateTime": startTime.Format("2006-01-02T15:04:05"),
			"timeZone": getTimezoneForMeeting(metadata),
		},
		"end": map[string]string{
			"dateTime": endTime.Format("2006-01-02T15:04:05"),
			"timeZone": getTimezoneForMeeting(metadata),
		},
		"location": map[string]string{
			"displayName": metadata.MeetingInvite.Location,
		},
		"attendees":             attendees,
		"allowNewTimeProposals": false,
		"isOnlineMeeting":       true,
		"onlineMeetingProvider": "teamsForBusiness",
		"responseRequested":     true,
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(meetingData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal meeting data to JSON: %w", err)
	}

	return string(jsonData), nil
}

// Helper functions for Microsoft Graph functionality

// parseStartTime parses the start time from various formats
func parseStartTime(startTimeStr string) (time.Time, error) {
	// Try multiple time formats
	timeFormats := []string{
		time.RFC3339,          // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05", // 2006-01-02T15:04:05
		"2006-01-02T15:04",    // 2006-01-02T15:04
		time.RFC3339Nano,      // 2006-01-02T15:04:05.999999999Z07:00
	}

	for _, format := range timeFormats {
		if startTime, err := time.Parse(format, startTimeStr); err == nil {
			return startTime, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse start time '%s' with any supported format", startTimeStr)
}

// parseStartTimeWithTimezone parses the start time with timezone support
func parseStartTimeWithTimezone(startTimeStr, timezone string) (time.Time, error) {
	// First try to parse the time
	startTime, err := parseStartTime(startTimeStr)
	if err != nil {
		return time.Time{}, err
	}

	// If timezone is specified, interpret the time in that timezone
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			// If the parsed time doesn't have timezone info, assume it's in the specified timezone
			if startTime.Location() == time.UTC {
				startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(),
					startTime.Hour(), startTime.Minute(), startTime.Second(), startTime.Nanosecond(), loc)
			}
		}
	}

	return startTime, nil
}

// calculateMeetingTimes parses start time and calculates end time from meeting metadata with timezone support
func calculateMeetingTimes(metadata *ApprovalRequestMetadata) (time.Time, time.Time, error) {
	startTime, err := parseStartTimeWithTimezone(metadata.MeetingInvite.StartTime, metadata.ChangeMetadata.Schedule.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse meeting start time: %w", err)
	}

	endTime := startTime.Add(time.Duration(metadata.MeetingInvite.Duration) * time.Minute)
	return startTime, endTime, nil
}

// getTimezoneForMeeting returns the timezone to use for meeting creation
func getTimezoneForMeeting(metadata *ApprovalRequestMetadata) string {
	if metadata.ChangeMetadata.Schedule.Timezone != "" {
		return metadata.ChangeMetadata.Schedule.Timezone
	}
	return "America/New_York" // Default to Eastern Time
}

// formatScheduleTime formats schedule time for display
func formatScheduleTime(date, timeStr, timezone string) string {
	if date == "" || timeStr == "" {
		return "Not specified"
	}

	// Combine date and time
	dateTimeStr := fmt.Sprintf("%s %s", date, timeStr)

	// Parse the combined datetime
	layout := "2006-01-02 15:04"
	t, err := time.Parse(layout, dateTimeStr)
	if err != nil {
		return dateTimeStr // Return as-is if parsing fails
	}

	// Apply timezone if specified
	if timezone != "" {
		if loc, locErr := time.LoadLocation(timezone); locErr == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
		}
	}

	return t.Format("January 2, 2006 at 3:04 PM MST")
}

// generateMeetingBodyHTML creates HTML content for the meeting body
func generateMeetingBodyHTML(metadata *ApprovalRequestMetadata) string {
	return fmt.Sprintf(`
<div style="background: linear-gradient(135deg, #28a745, #20c997); color: white; padding: 25px; border-radius: 10px; margin-bottom: 25px; text-align: center;">
    <h2 style="margin: 0 0 10px 0; font-size: 28px; font-weight: bold;">📅 CHANGE APPROVED & SCHEDULED</h2>
    <p style="margin: 0; font-size: 16px;">The change has been approved and scheduled.<br>You are welcome but not required to join the coordination bridge during the implementation window.</p>
</div>

<div style="background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #28a745;">
    <h2 style="color: #28a745;">📋 Change Details</h2>
    <p><strong>%s</strong></p>
    <p>Customer: %s</p>
</div>

<div style="margin-bottom: 25px;">
    <h3 style="color: #28a745; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px;">📋 Change Information</h3>
    <div style="background-color: #f8f9fa; padding: 10px; border-radius: 5px;">
        <strong>Tracking Numbers:</strong><br>
        ServiceNow: %s<br>
        JIRA: %s
    </div>
</div>

<div style="margin-bottom: 25px;">
    <h3 style="color: #28a745; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px;">📅 Implementation Schedule</h3>
    <div style="background-color: #d4edda; padding: 15px; border-radius: 5px; border-left: 4px solid #28a745;">
        <strong>🕐 Start:</strong> %s<br>
        <strong>🕐 End:</strong> %s
    </div>
</div>

<div style="margin-bottom: 25px;">
    <h3 style="color: #28a745; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px;">📝 Change Reason</h3>
    <p>%s</p>
</div>

<div style="margin-bottom: 25px;">
    <h3 style="color: #28a745; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px;">🔧 Implementation Plan</h3>
    <p>%s</p>
</div>

<div style="margin-bottom: 25px;">
    <h3 style="color: #28a745; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px;">🧪 Test Plan</h3>
    <p>%s</p>
</div>

<div style="margin-bottom: 25px;">
    <h3 style="color: #28a745; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px;">👥 Expected Customer Impact</h3>
    <p>%s</p>
</div>

<div style="margin-bottom: 25px;">
    <h3 style="color: #28a745; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px;">🔄 Rollback Plan</h3>
    <p>%s</p>
</div>

<p style="margin-top: 30px; padding-top: 20px; border-top: 1px solid #dee2e6; font-size: 12px; color: #6c757d;">This meeting is for the approved change implementation.</p>
`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		formatScheduleTime(metadata.ChangeMetadata.Schedule.BeginDate, metadata.ChangeMetadata.Schedule.BeginTime, metadata.ChangeMetadata.Schedule.Timezone),
		formatScheduleTime(metadata.ChangeMetadata.Schedule.EndDate, metadata.ChangeMetadata.Schedule.EndTime, metadata.ChangeMetadata.Schedule.Timezone),
		strings.ReplaceAll(metadata.ChangeMetadata.ChangeReason, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.TestPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.ExpectedCustomerImpact, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.RollbackPlan, "\n", "<br>"),
	)
}

// getGraphAccessToken obtains an access token for Microsoft Graph API using client credentials flow
func getGraphAccessToken() (string, error) {
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")

	// Use default client ID if not provided via environment variable
	if clientID == "" {
		clientID = "071af76c-1e5a-4423-bb57-c9e7573b4bc0"
	}

	// Use default tenant ID if not provided via environment variable
	if tenantID == "" {
		tenantID = "a84894e7-87c5-40e3-9783-320d0334b3cc"
	}

	if clientSecret == "" {
		return "", fmt.Errorf("missing required environment variable: AZURE_CLIENT_SECRET")
	}

	// Prepare the request
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("scope", "https://graph.microsoft.com/.default")
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var graphErr GraphError
		if err := json.Unmarshal(body, &graphErr); err == nil {
			return "", fmt.Errorf("authentication failed: %s - %s", graphErr.Error.Code, graphErr.Error.Message)
		}
		return "", fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp GraphAuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return authResp.AccessToken, nil
}

// createGraphMeeting creates a meeting using Microsoft Graph API
func createGraphMeeting(payload string, organizerEmail string, forceUpdate bool) (string, error) {
	// Parse the payload to extract meeting details for idempotency check
	var meetingData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &meetingData); err != nil {
		return "", fmt.Errorf("failed to parse meeting payload: %w", err)
	}

	subject, _ := meetingData["subject"].(string)
	startTimeData, _ := meetingData["start"].(map[string]interface{})
	startTimeStr, _ := startTimeData["dateTime"].(string)

	// Get access token
	accessToken, err := getGraphAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Check if meeting already exists (idempotency check)
	exists, existingMeeting, err := checkMeetingExists(accessToken, organizerEmail, subject, startTimeStr)
	if err != nil {
		return "", fmt.Errorf("failed to check existing meetings: %w", err)
	}

	if exists {
		fmt.Printf("✅ Meeting already exists (idempotent):\n")
		fmt.Printf("   Meeting ID: %s\n", existingMeeting.ID)
		fmt.Printf("   Subject: %s\n", existingMeeting.Subject)

		if forceUpdate {
			fmt.Printf("🔄 Force update requested - updating meeting details...\n")
			// Update the existing meeting with new details
			err = updateGraphMeeting(existingMeeting.ID, payload, organizerEmail)
			if err != nil {
				return "", fmt.Errorf("failed to update existing meeting: %w", err)
			}
			fmt.Printf("✅ Meeting updated successfully (forced)\n")
			return "updated", nil
		}

		// Check if there are any changes to apply (excluding body content)
		hasChanges, err := compareMeetingDetails(existingMeeting, payload)
		if err != nil {
			return "", fmt.Errorf("failed to compare meeting details: %w", err)
		}

		if hasChanges {
			fmt.Printf("🔄 Detected changes - updating meeting details...\n")
			// Update the existing meeting with new details
			err = updateGraphMeeting(existingMeeting.ID, payload, organizerEmail)
			if err != nil {
				return "", fmt.Errorf("failed to update existing meeting: %w", err)
			}
			fmt.Printf("✅ Meeting updated successfully\n")
			return "updated", nil
		} else {
			fmt.Printf("📋 No changes detected - meeting is already up to date\n")
			fmt.Printf("   💡 Use --force-update to update anyway (e.g., for body content changes)\n")
			if existingMeeting.WebLink != "" {
				fmt.Printf("   Web Link: %s\n", existingMeeting.WebLink)
			}
			if existingMeeting.OnlineMeeting.JoinURL != "" {
				fmt.Printf("   Teams Join URL: %s\n", existingMeeting.OnlineMeeting.JoinURL)
			}
			return "unchanged", nil
		}
	}

	// Create HTTP request - use the organizer's email to create the meeting in their calendar
	graphURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events", organizerEmail)
	req, err := http.NewRequest("POST", graphURL, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create meeting: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var graphError GraphError
		if err := json.Unmarshal(body, &graphError); err == nil {
			return "", fmt.Errorf("meeting creation failed: %s - %s", graphError.Error.Code, graphError.Error.Message)
		}
		return "", fmt.Errorf("meeting creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var meetingResponse GraphMeetingResponse
	if err := json.Unmarshal(body, &meetingResponse); err != nil {
		return "", fmt.Errorf("failed to parse meeting response: %w", err)
	}

	fmt.Printf("✅ Meeting created successfully:\n")
	fmt.Printf("   Meeting ID: %s\n", meetingResponse.ID)
	fmt.Printf("   Subject: %s\n", meetingResponse.Subject)
	if meetingResponse.WebLink != "" {
		fmt.Printf("   Web Link: %s\n", meetingResponse.WebLink)
	}
	if meetingResponse.OnlineMeeting.JoinURL != "" {
		fmt.Printf("   Teams Join URL: %s\n", meetingResponse.OnlineMeeting.JoinURL)
	}

	return "created", nil
}

// checkMeetingExists checks if a meeting with the same subject already exists (regardless of time)
func checkMeetingExists(accessToken, organizerEmail, subject, startTime string) (bool, *GraphMeetingResponse, error) {
	// Parse the start time to create a date range for the query
	startDateTime, err := time.Parse("2006-01-02T15:04:05", startTime)
	if err != nil {
		return false, nil, fmt.Errorf("failed to parse start time: %w", err)
	}

	// Create a date range (same day)
	startOfDay := time.Date(startDateTime.Year(), startDateTime.Month(), startDateTime.Day(), 0, 0, 0, 0, startDateTime.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Format for Microsoft Graph API
	startFilter := startOfDay.Format("2006-01-02T15:04:05.000Z")
	endFilter := endOfDay.Format("2006-01-02T15:04:05.000Z")

	// Query for events on the same day
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$filter=start/dateTime ge '%s' and start/dateTime lt '%s'&$select=id,subject,webLink,start,end,location,body,attendees,onlineMeeting",
		organizerEmail, startFilter, endFilter)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("failed to query events with status %d: %s", resp.StatusCode, string(body))
	}

	var eventsResp struct {
		Value []GraphMeetingResponse `json:"value"`
	}

	if err := json.Unmarshal(body, &eventsResp); err != nil {
		return false, nil, fmt.Errorf("failed to parse events response: %w", err)
	}

	// Check if any meeting has the same subject
	for _, event := range eventsResp.Value {
		if event.Subject == subject {
			return true, &event, nil
		}
	}

	return false, nil, nil
}

// compareMeetingDetails compares existing meeting with new payload to detect changes
func compareMeetingDetails(existingMeeting *GraphMeetingResponse, newPayload string) (bool, error) {
	// Parse the new payload
	var newMeetingData map[string]interface{}
	if err := json.Unmarshal([]byte(newPayload), &newMeetingData); err != nil {
		return false, fmt.Errorf("failed to parse new payload: %w", err)
	}

	// Compare key fields that might change

	// 1. Check subject
	if newSubject, ok := newMeetingData["subject"].(string); ok {
		if existingMeeting.Subject != newSubject {
			return true, nil
		}
	}

	// 2. Check start time
	if startData, ok := newMeetingData["start"].(map[string]interface{}); ok {
		if newStartTime, ok := startData["dateTime"].(string); ok {
			// Parse existing start time
			existingStart, err := time.Parse("2006-01-02T15:04:05.0000000", existingMeeting.Start.DateTime)
			if err != nil {
				// Try alternative format
				existingStart, err = time.Parse("2006-01-02T15:04:05", existingMeeting.Start.DateTime)
				if err != nil {
					return false, fmt.Errorf("failed to parse existing start time: %w", err)
				}
			}

			// Parse new start time
			newStart, err := time.Parse("2006-01-02T15:04:05", newStartTime)
			if err != nil {
				return false, fmt.Errorf("failed to parse new start time: %w", err)
			}

			// Compare times (allow for small differences due to formatting)
			if existingStart.Sub(newStart).Abs() > time.Minute {
				return true, nil
			}
		}
	}

	// 3. Check end time
	if endData, ok := newMeetingData["end"].(map[string]interface{}); ok {
		if newEndTime, ok := endData["dateTime"].(string); ok {
			// Parse existing end time
			existingEnd, err := time.Parse("2006-01-02T15:04:05.0000000", existingMeeting.End.DateTime)
			if err != nil {
				// Try alternative format
				existingEnd, err = time.Parse("2006-01-02T15:04:05", existingMeeting.End.DateTime)
				if err != nil {
					return false, fmt.Errorf("failed to parse existing end time: %w", err)
				}
			}

			// Parse new end time
			newEnd, err := time.Parse("2006-01-02T15:04:05", newEndTime)
			if err != nil {
				return false, fmt.Errorf("failed to parse new end time: %w", err)
			}

			// Compare times (allow for small differences due to formatting)
			if existingEnd.Sub(newEnd).Abs() > time.Minute {
				return true, nil
			}
		}
	}

	// 4. Check location
	if locationData, ok := newMeetingData["location"].(map[string]interface{}); ok {
		if newLocation, ok := locationData["displayName"].(string); ok {
			if existingMeeting.Location.DisplayName != newLocation {
				return true, nil
			}
		}
	}

	// 5. Check body content
	if bodyData, ok := newMeetingData["body"].(map[string]interface{}); ok {
		if newContent, ok := bodyData["content"].(string); ok {
			if existingMeeting.Body.Content != newContent {
				return true, nil
			}
		}
	}

	// 6. Check attendees count (simplified check)
	if newAttendees, ok := newMeetingData["attendees"].([]interface{}); ok {
		if len(existingMeeting.Attendees) != len(newAttendees) {
			return true, nil
		}
	}

	return false, nil
}

// areTimesEqualWithTimezone compares two time strings with their respective timezones
func areTimesEqualWithTimezone(time1, tz1, time2, tz2 string) bool {
	// Parse both times with their timezone information
	t1, err1 := parseTimeWithTimezone(time1, tz1)
	t2, err2 := parseTimeWithTimezone(time2, tz2)

	if err1 != nil || err2 != nil {
		// Fall back to string comparison if parsing failed
		return time1 == time2 && tz1 == tz2
	}

	// Normalize times to minute precision (ignore seconds and milliseconds)
	t1UTC := t1.UTC().Truncate(time.Minute)
	t2UTC := t2.UTC().Truncate(time.Minute)
	diff := t1UTC.Sub(t2UTC)

	// Times are equal if they match to the minute
	return diff == 0
}

// parseTimeWithTimezone parses a time string with timezone information
func parseTimeWithTimezone(timeStr, timezoneStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Try different time formats
	timeFormats := []string{
		"2006-01-02T15:04:05.0000000", // Microsoft Graph format
		"2006-01-02T15:04:05",         // Our format
		time.RFC3339,
		time.RFC3339Nano,
	}

	var parsedTime time.Time
	var err error

	// Parse the time string
	for _, format := range timeFormats {
		if parsedTime, err = time.Parse(format, timeStr); err == nil {
			break
		}
	}

	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse time '%s': %w", timeStr, err)
	}

	// Handle timezone conversion
	if timezoneStr != "" {
		// Microsoft Graph sometimes returns timezone names like "UTC" or location names
		if timezoneStr == "UTC" {
			return parsedTime.UTC(), nil
		}

		// Try to load the timezone
		if loc, err := time.LoadLocation(timezoneStr); err == nil {
			// If the parsed time has no timezone info, interpret it in the specified timezone
			if parsedTime.Location() == time.UTC {
				parsedTime = time.Date(parsedTime.Year(), parsedTime.Month(), parsedTime.Day(),
					parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), parsedTime.Nanosecond(), loc)
			}
			return parsedTime, nil
		}
	}

	// If no timezone specified or loading failed, assume UTC for Microsoft Graph format
	if strings.Contains(timeStr, ".0000000") {
		return parsedTime.UTC(), nil
	}

	return parsedTime, nil
}

// updateGraphMeeting updates an existing meeting with new details
func updateGraphMeeting(meetingID, payload, organizerEmail string) error {
	// Get access token
	accessToken, err := getGraphAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events/%s", organizerEmail, meetingID)

	req, err := http.NewRequest("PATCH", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var graphErr GraphError
		if err := json.Unmarshal(body, &graphErr); err == nil {
			return fmt.Errorf("failed to update meeting: %s - %s", graphErr.Error.Code, graphErr.Error.Message)
		}
		return fmt.Errorf("failed to update meeting with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateICSInvite sends a calendar invite with ICS attachment based on metadata
func CreateICSInvite(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, senderEmail string, dryRun bool) error {
	// Validate required parameters
	if topicName == "" {
		return fmt.Errorf("topic name is required for send-calendar-invite action")
	}
	if jsonMetadataPath == "" {
		return fmt.Errorf("json-metadata file path is required for send-calendar-invite action")
	}
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for send-calendar-invite action")
	}

	// Load metadata from JSON file
	metadata, err := loadApprovalMetadata(jsonMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Check if meeting information exists
	if metadata.MeetingInvite == nil {
		return fmt.Errorf("no meeting information found in metadata - calendar invite cannot be created")
	}

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get contacts subscribed to the specified topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &types.ListContactsFilter{
			FilteredStatus: types.SubscriptionStatusOptIn,
			TopicFilter: &types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	if len(contactsResult.Contacts) == 0 {
		fmt.Printf("⚠️  No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	// Extract attendee emails
	var attendeeEmails []string
	for _, contact := range contactsResult.Contacts {
		attendeeEmails = append(attendeeEmails, *contact.EmailAddress)
	}

	// Generate ICS file content with all attendees
	icsContent, err := generateICSFile(metadata, senderEmail, attendeeEmails)
	if err != nil {
		return fmt.Errorf("failed to generate ICS file: %w", err)
	}

	fmt.Printf("📅 Sending calendar invite to topic '%s' (%d subscribers)\n", topicName, len(contactsResult.Contacts))
	fmt.Printf("📋 Using SES contact list: %s\n", accountListName)
	fmt.Printf("📄 Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("🕐 Meeting: %s\n", metadata.MeetingInvite.Title)

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No emails will be sent\n")
	}

	// Create email content
	subject := fmt.Sprintf("Calendar Invite: %s", metadata.MeetingInvite.Title)

	// Output raw email message to console for debugging
	fmt.Printf("\n📧 Calendar Invite Preview:\n")
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("From: %s\n", senderEmail)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Printf("Contact List: %s\n", accountListName)
	fmt.Printf("Topic: %s\n", topicName)
	fmt.Printf("Content-Type: text/calendar; method=REQUEST\n")
	fmt.Printf("\n--- CALENDAR INVITE (ICS) ---\n")
	fmt.Printf("%s\n", icsContent)
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n\n")

	if dryRun {
		fmt.Printf("📊 Calendar Invite Summary (DRY RUN):\n")
		fmt.Printf("   📧 Would send individual invites to: %d recipients\n", len(contactsResult.Contacts))
		fmt.Printf("   📋 Each invite shows all attendees:\n")
		for _, contact := range contactsResult.Contacts {
			fmt.Printf("      - %s\n", *contact.EmailAddress)
		}
		return nil
	}

	successCount := 0
	errorCount := 0

	// Send individual calendar invites to each attendee (but each invite contains all attendees)
	for _, contact := range contactsResult.Contacts {
		// Generate raw calendar invite email for this specific recipient
		rawEmail, err := generateCalendarInviteEmail(
			senderEmail,
			*contact.EmailAddress,
			subject,
			icsContent, // ICS already contains all attendees
		)
		if err != nil {
			fmt.Printf("   ❌ Failed to generate calendar invite for %s: %v\n", *contact.EmailAddress, err)
			errorCount++
			continue
		}

		// Send individual email with full attendee list in ICS
		sendRawInput := &sesv2.SendEmailInput{
			FromEmailAddress: aws.String(senderEmail),
			Destination: &types.Destination{
				ToAddresses: []string{*contact.EmailAddress},
			},
			Content: &types.EmailContent{
				Raw: &types.RawMessage{
					Data: []byte(rawEmail),
				},
			},
			ListManagementOptions: &types.ListManagementOptions{
				ContactListName: aws.String(accountListName),
				TopicName:       aws.String(topicName),
			},
		}

		_, err = sesClient.SendEmail(context.Background(), sendRawInput)
		if err != nil {
			fmt.Printf("   ❌ Failed to send to %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			fmt.Printf("   ✅ Sent to %s\n", *contact.EmailAddress)
			successCount++
		}
	}

	fmt.Printf("\n📊 Calendar Invite Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   📋 Each recipient received invite showing all %d attendees\n", len(attendeeEmails))

	if errorCount > 0 {
		return fmt.Errorf("failed to send calendar invite to %d recipients", errorCount)
	}

	return nil
}

// ICS Calendar generation functions

// generateICSFile creates an ICS calendar file from metadata
func generateICSFile(metadata *ApprovalRequestMetadata, senderEmail string, attendeeEmails []string) (string, error) {
	if metadata.MeetingInvite == nil {
		return "", fmt.Errorf("no meeting information available")
	}

	// Parse start time and calculate end time
	startTime, endTime, err := calculateMeetingTimes(metadata)
	if err != nil {
		return "", err
	}

	// Generate unique UID
	uid := fmt.Sprintf("%d@aws-alternate-contact-manager", time.Now().Unix())

	// Build attendee list
	var attendeeLines []string
	for _, email := range attendeeEmails {
		attendeeLines = append(attendeeLines, fmt.Sprintf("ATTENDEE;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:MAILTO:%s", email))
	}
	attendeeList := strings.Join(attendeeLines, "\n")

	// Create ICS content with proper attendee information
	icsContent := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//AWS Alternate Contact Manager//Calendar Invite//EN
CALSCALE:GREGORIAN
METHOD:REQUEST
BEGIN:VEVENT
UID:%s
DTSTART:%s
DTEND:%s
DTSTAMP:%s
SUMMARY:%s
DESCRIPTION:CHANGE IMPLEMENTATION MEETING\n\n📋 CHANGE DETAILS:\nTitle: %s\nCustomer: %s\n\n🎫 TRACKING:\nServiceNow: %s\nJIRA: %s\n\n📅 IMPLEMENTATION WINDOW:\nStart: %s\nEnd: %s\n\n❓ WHY THIS CHANGE:\n%s\n\n🔧 IMPLEMENTATION PLAN:\n%s\n\n🧪 TEST PLAN:\n%s\n\n👥 EXPECTED CUSTOMER IMPACT:\n%s\n\n🔄 ROLLBACK PLAN:\n%s\n\n📍 MEETING LOCATION:\n%s\n\nThis meeting is for the approved change implementation.
LOCATION:%s
ORGANIZER:MAILTO:%s
%s
STATUS:CONFIRMED
SEQUENCE:0
CREATED:%s
LAST-MODIFIED:%s
CLASS:PUBLIC
TRANSP:OPAQUE
END:VEVENT
END:VCALENDAR`,
		uid,
		startTime.UTC().Format("20060102T150405Z"),
		endTime.UTC().Format("20060102T150405Z"),
		time.Now().UTC().Format("20060102T150405Z"),
		metadata.MeetingInvite.Title,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		formatDateTimeWithTimezone(metadata.ChangeMetadata.Schedule.ImplementationStart, metadata.ChangeMetadata.Schedule.Timezone),
		formatDateTimeWithTimezone(metadata.ChangeMetadata.Schedule.ImplementationEnd, metadata.ChangeMetadata.Schedule.Timezone),
		convertTextForICS(metadata.ChangeMetadata.ChangeReason),
		convertTextForICS(metadata.ChangeMetadata.ImplementationPlan),
		convertTextForICS(metadata.ChangeMetadata.TestPlan),
		convertTextForICS(metadata.ChangeMetadata.ExpectedCustomerImpact),
		convertTextForICS(metadata.ChangeMetadata.RollbackPlan),
		metadata.MeetingInvite.Location,
		metadata.MeetingInvite.Location,
		senderEmail,
		attendeeList,
		time.Now().UTC().Format("20060102T150405Z"),
		time.Now().UTC().Format("20060102T150405Z"),
	)

	return icsContent, nil
}

// generateCalendarInviteEmail creates a raw email that is primarily a calendar invite
func generateCalendarInviteEmail(from, to, subject, icsContent string) (string, error) {
	// Create raw calendar invite email (primary content is the calendar invite)
	rawEmail := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/calendar; charset=UTF-8; method=REQUEST
Content-Transfer-Encoding: 7bit

%s`,
		from,
		to,
		subject,
		icsContent,
	)

	return rawEmail, nil
}

// convertTextForICS converts plain text for ICS format (preserves line breaks as \n)
func convertTextForICS(text string) string {
	if text == "" {
		return ""
	}

	// ICS format uses \n for line breaks, so we just need to ensure consistent line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	return text
}

// formatDateTimeWithTimezone formats datetime with timezone for display
func formatDateTimeWithTimezone(dateTimeStr string, timezone string) string {
	if dateTimeStr == "" {
		return "Not specified"
	}

	// Try to parse and format the datetime
	if t, err := time.Parse(time.RFC3339, dateTimeStr); err == nil {
		// Apply timezone if specified
		if timezone != "" {
			if loc, locErr := time.LoadLocation(timezone); locErr == nil {
				t = t.In(loc)
			}
		}
		return t.Format("January 2, 2006 at 3:04 PM MST")
	}

	// If parsing fails, return as-is
	return dateTimeStr
}
