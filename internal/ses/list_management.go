// Package ses provides email templates, Microsoft Graph integration, and S3 payload processing.
package ses

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	apptypes "ccoe-customer-contact-manager/internal/types"
)

// This file contains functions extracted from ccoe-customer-contact-manager-original.go
// that are missing from the modular codebase. These functions provide:
// 1. Microsoft Graph meeting functionality
// 2. Email template functionality for approval requests and announcements
// 3. ICS calendar invite functionality
// 4. Supporting helper functions

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
		fmt.Printf(" No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	// Create subject with "APPROVED" prefix and shorten "Notification:" to make it more concise
	originalSubject := metadata.EmailNotification.Subject
	shortenedSubject := strings.Replace(originalSubject, "CCOE Change:", "ITSM Change:", 1)
	subject := fmt.Sprintf(" APPROVED %s", shortenedSubject)

	fmt.Printf(" Sending change notification to topic '%s' (%d subscribers)\n", topicName, len(subscribedContacts))
	fmt.Printf(" Using SES contact list: %s\n", accountListName)
	fmt.Printf(" Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("👤 Customer: %s\n", strings.Join(metadata.ChangeMetadata.CustomerNames, ", "))

	if dryRun {
		fmt.Printf(" DRY RUN MODE - No emails will be sent\n")
		fmt.Printf(" Change Notification Summary (DRY RUN):\n")
		fmt.Printf("    Would send to: %d recipients\n", len(subscribedContacts))
		fmt.Printf("    Recipients:\n")
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
			fmt.Printf("    Failed to send to %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			fmt.Printf("    Sent to %s\n", *contact.EmailAddress)
			successCount++
		}
	}

	fmt.Printf("\n Change Notification Summary:\n")
	fmt.Printf("    Successful: %d\n", successCount)
	fmt.Printf("    Errors: %d\n", errorCount)
	fmt.Printf("    Total recipients: %d\n", len(subscribedContacts))

	if errorCount > 0 {
		return fmt.Errorf("failed to send change notification to %d recipients", errorCount)
	}

	return nil
}

// Helper functions for email templates and metadata processing

func loadApprovalMetadata(filePath string) (*apptypes.ApprovalRequestMetadata, error) {
	// Use the new format converter that handles both nested and flat formats
	return LoadMetadataFromFile(filePath)
}

func loadHtmlTemplate(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}
	return string(data), nil
}

func generateDefaultHtmlTemplate(metadata *apptypes.ApprovalRequestMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Approval Request</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .unsubscribe-prominent { margin-top: 10px; }
        .unsubscribe-prominent a { color: #007bff; text-decoration: none; font-weight: bold; }
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
        <p><strong>Test Plan:</strong> %s</p>
        <p><strong>Expected Impact:</strong> %s</p>
        <p><strong>ServiceNow:</strong> %s</p>
        <p><strong>JIRA:</strong> %s</p>
    </div>
    
    <div class="unsubscribe">
        <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
        <p>Request sent at %s</p>
        <div class="unsubscribe-prominent"><a href="{{amazonSESUnsubscribeUrl}}"> Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.ChangeReason,
		metadata.ChangeMetadata.ImplementationPlan,
		metadata.ChangeMetadata.TestPlan,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		time.Now().Format("January 2, 2006 at 3:04 PM MST"),
	)
}

func generateChangeNotificationHtml(metadata *apptypes.ApprovalRequestMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Notification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #d4edda; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .unsubscribe-prominent { margin-top: 10px; }
        .unsubscribe-prominent a { color: #007bff; text-decoration: none; font-weight: bold; }
    </style>
</head>
<body>
    <div class="header">
        <h2> CHANGE APPROVED & SCHEDULED</h2>
        <p>This change has been approved and is scheduled for implementation.</p>
    </div>
    <div class="section">
        <h3>Change Details</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer:</strong> %s</p>
        <p><strong>Reason:</strong> %s</p>
        <p><strong>Implementation Plan:</strong> %s</p>
        <p><strong>Test Plan:</strong> %s</p>
        <p><strong>Expected Impact:</strong> %s</p>
        <p><strong>ServiceNow:</strong> %s</p>
        <p><strong>JIRA:</strong> %s</p>
    </div>
    
    <div class="unsubscribe">
        <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
        <p>Notification sent at %s</p>
        <div class="unsubscribe-prominent"><a href="{{amazonSESUnsubscribeUrl}}"> Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.ChangeReason,
		metadata.ChangeMetadata.ImplementationPlan,
		metadata.ChangeMetadata.TestPlan,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		time.Now().Format("January 2, 2006 at 3:04 PM MST"),
	)
}

func processTemplate(template string, metadata *apptypes.ApprovalRequestMetadata, topicName string) string {
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
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_START}}", formatTimeForTemplate(metadata.ChangeMetadata.Schedule.ImplementationStart, metadata.ChangeMetadata.Schedule.Timezone))
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_END}}", formatTimeForTemplate(metadata.ChangeMetadata.Schedule.ImplementationEnd, metadata.ChangeMetadata.Schedule.Timezone))
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
	ExecutionID  string                            `json:"execution_id"`
	ActionType   string                            `json:"action_type"`
	CustomerCode string                            `json:"customer_code"`
	Timestamp    string                            `json:"timestamp"`
	RetryCount   int                               `json:"retry_count"`
	Metadata     *apptypes.ApprovalRequestMetadata `json:"metadata"`
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
func (s *SQSMessageSender) CreateSQSMessage(customerCode, actionType string, metadata *apptypes.ApprovalRequestMetadata) (*SQSMessage, error) {
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
func MapS3ChangeMetadataToEmailMessage(s3Key string, metadata *apptypes.ApprovalRequestMetadata) (*EmailMessage, error) {
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
			Start:    formatTimeForTemplate(metadata.ChangeMetadata.Schedule.ImplementationStart, metadata.ChangeMetadata.Schedule.Timezone),
			End:      formatTimeForTemplate(metadata.ChangeMetadata.Schedule.ImplementationEnd, metadata.ChangeMetadata.Schedule.Timezone),
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

// getTimezoneForMeeting returns the timezone to use for meeting creation
func getTimezoneForMeeting(metadata *apptypes.ApprovalRequestMetadata) string {
	if metadata.ChangeMetadata.Schedule.Timezone != "" {
		return metadata.ChangeMetadata.Schedule.Timezone
	}
	return "America/New_York" // Default to Eastern Time
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

// ICS Calendar generation functions

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

// formatTimeForTemplate formats a time.Time for use in email templates
func formatTimeForTemplate(t time.Time, timezone string) string {
	if t.IsZero() {
		return "TBD"
	}

	// Apply timezone if specified
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			t = t.In(loc)
		}
	}

	// Format in human-readable format
	return t.Format("January 2, 2006 at 3:04 PM MST")
}
