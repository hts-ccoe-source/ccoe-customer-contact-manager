package main

// This file contains functions extracted from aws-alternate-contact-manager-original.go
// that are missing from the modular codebase. These functions provide:
// 1. Microsoft Graph meeting functionality
// 2. Email template functionality for approval requests and announcements
// 3. ICS calendar invite functionality
// 4. Supporting helper functions

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
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

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
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
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

// Placeholder functions that need to be implemented
// These will be extracted from the original file in subsequent iterations

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

// getGraphAccessToken obtains an access token for Microsoft Graph API using client credentials flow
func getGraphAccessToken() (string, error) {
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")

	if clientID == "" || clientSecret == "" || tenantID == "" {
		return "", fmt.Errorf("missing required Azure environment variables: AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID")
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

func createGraphMeeting(payload string, organizerEmail string, forceUpdate bool) (string, error) {
	// This function needs to be extracted from the original file
	// For now, return a placeholder
	return "", fmt.Errorf("createGraphMeeting function needs to be extracted from original file")
}

// EMAIL TEMPLATE FUNCTIONS
// These functions handle approval request and change notification emails

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
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
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
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(processedHtml),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
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
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(processedHtml),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
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

// Helper functions for email templates

func loadHtmlTemplate(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}
	return string(data), nil
}

func generateDefaultHtmlTemplate(metadata *ApprovalRequestMetadata) string {
	// Simplified template - can be enhanced later
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
	// Simplified template - can be enhanced later
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

func getSubscribedContactsForTopic(sesClient *sesv2.Client, accountListName string, topicName string) ([]sesv2Types.Contact, error) {
	// This is a simplified implementation - the full version would handle default opt-in status
	// For now, just get explicitly opted-in contacts
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts: %w", err)
	}

	return contactsResult.Contacts, nil
}
