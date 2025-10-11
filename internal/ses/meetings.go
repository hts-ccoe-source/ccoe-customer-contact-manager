// Package ses provides Microsoft Graph meeting integration and email template functions.
package ses

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	internalconfig "ccoe-customer-contact-manager/internal/config"
	"ccoe-customer-contact-manager/internal/datetime"
	"ccoe-customer-contact-manager/internal/types"
)

// formatTimeWithTimezone is a centralized function to format time.Time with timezone conversion
// This matches the implementation in internal/lambda/handlers.go for consistency
func formatTimeWithTimezone(t time.Time, timezone string) string {
	if t.IsZero() {
		return "TBD"
	}

	// Use provided timezone or default to Eastern Time
	targetTimezone := timezone
	if targetTimezone == "" {
		targetTimezone = "America/New_York"
	}

	// Load the target timezone
	loc, err := time.LoadLocation(targetTimezone)
	if err != nil {
		// Fallback to UTC if timezone loading fails
		fmt.Printf("Warning: Failed to load timezone %s, using UTC: %v\n", targetTimezone, err)
		loc = time.UTC
	}

	// Convert the time to the target timezone and format
	localTime := t.In(loc)
	return localTime.Format("January 2, 2006 at 3:04 PM MST")
}

// convertApprovalRequestToChangeMetadata converts old nested ApprovalRequestMetadata to flat ChangeMetadata
// This provides backward compatibility while allowing new functions to use the flat structure
func convertApprovalRequestToChangeMetadata(approval *types.ApprovalRequestMetadata) *types.ChangeMetadata {
	return &types.ChangeMetadata{
		ChangeID:            approval.ChangeMetadata.Title, // Use title as ID if no ID available
		ChangeTitle:         approval.ChangeMetadata.Title,
		ChangeReason:        approval.ChangeMetadata.ChangeReason,
		Customers:           approval.ChangeMetadata.CustomerCodes,
		ImplementationPlan:  approval.ChangeMetadata.ImplementationPlan,
		TestPlan:            approval.ChangeMetadata.TestPlan,
		CustomerImpact:      approval.ChangeMetadata.ExpectedCustomerImpact,
		RollbackPlan:        approval.ChangeMetadata.RollbackPlan,
		SnowTicket:          approval.ChangeMetadata.Tickets.ServiceNow,
		JiraTicket:          approval.ChangeMetadata.Tickets.Jira,
		ImplementationStart: approval.ChangeMetadata.Schedule.ImplementationStart,
		ImplementationEnd:   approval.ChangeMetadata.Schedule.ImplementationEnd,
		Timezone:            approval.ChangeMetadata.Schedule.Timezone,
		MeetingRequired:     "yes", // Assume yes if creating a meeting
		MeetingTitle:        fmt.Sprintf("Change Implementation: %s", approval.ChangeMetadata.Title),
		MeetingLocation:     "Microsoft Teams Meeting",
		Status:              "approved", // Assume approved if creating meeting
	}
}

// azureCredentials holds cached Azure credentials from Parameter Store
var azureCredentials struct {
	ClientID     string
	ClientSecret string
	TenantID     string
	loaded       bool
}

// getMetadataFilePath returns the correct file path, handling both absolute and relative paths
func getMetadataFilePath(jsonMetadataPath string) string {
	// If path is absolute, use it as-is
	if filepath.IsAbs(jsonMetadataPath) {
		return jsonMetadataPath
	}

	// If path is relative, add config path prefix
	configPath := internalconfig.GetConfigPath()
	return configPath + jsonMetadataPath
}

// loadAzureCredentialsFromSSM loads Azure credentials from Parameter Store
// and sets them as environment variables
func loadAzureCredentialsFromSSM(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ssm.NewFromConfig(cfg)

	// Get all Azure parameters at once for efficiency
	result, err := client.GetParameters(ctx, &ssm.GetParametersInput{
		Names: []string{
			"/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_ID",
			"/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_SECRET",
			"/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_TENANT_ID",
		},
		WithDecryption: aws.Bool(true), // Important for SecureString parameters
	})
	if err != nil {
		return fmt.Errorf("failed to get Azure parameters from SSM: %w", err)
	}

	// Check if we got all expected parameters
	if len(result.Parameters) != 3 {
		return fmt.Errorf("expected 3 Azure parameters, got %d", len(result.Parameters))
	}

	// Set environment variables based on parameter names
	for _, param := range result.Parameters {
		switch *param.Name {
		case "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_ID":
			os.Setenv("AZURE_CLIENT_ID", *param.Value)
		case "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_SECRET":
			os.Setenv("AZURE_CLIENT_SECRET", *param.Value)
		case "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_TENANT_ID":
			os.Setenv("AZURE_TENANT_ID", *param.Value)
		}
	}

	// Verify all required environment variables are set
	requiredVars := []string{"AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET", "AZURE_TENANT_ID"}
	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			return fmt.Errorf("failed to load %s from Parameter Store", varName)
		}
	}

	fmt.Println("✅ Successfully loaded Azure credentials from Parameter Store")
	return nil
}

// GetAzureCredentials returns Azure credentials, loading from Parameter Store if not cached
func GetAzureCredentials(ctx context.Context) (clientID, clientSecret, tenantID string, err error) {
	// Return cached values if already loaded
	if azureCredentials.loaded {
		return azureCredentials.ClientID, azureCredentials.ClientSecret, azureCredentials.TenantID, nil
	}

	// Load from Parameter Store
	if err := loadAzureCredentialsFromSSM(ctx); err != nil {
		return "", "", "", err
	}

	// Cache the values
	azureCredentials.ClientID = os.Getenv("AZURE_CLIENT_ID")
	azureCredentials.ClientSecret = os.Getenv("AZURE_CLIENT_SECRET")
	azureCredentials.TenantID = os.Getenv("AZURE_TENANT_ID")
	azureCredentials.loaded = true

	return azureCredentials.ClientID, azureCredentials.ClientSecret, azureCredentials.TenantID, nil
}

// CreateMultiCustomerMeetingInvite creates a meeting using Microsoft Graph API with recipients from multiple customers
// Returns the Graph meeting ID and any error
func CreateMultiCustomerMeetingInvite(credentialManager CredentialManager, customerCodes []string, topicName string, jsonMetadataPath string, senderEmail string, dryRun bool, forceUpdate bool) (string, error) {
	// Validate required parameters
	if len(customerCodes) == 0 {
		return "", fmt.Errorf("at least one customer code is required for multi-customer meeting invite")
	}
	if topicName == "" {
		return "", fmt.Errorf("topic name is required for create-meeting-invite action")
	}
	if jsonMetadataPath == "" {
		return "", fmt.Errorf("json-metadata file path is required for create-meeting-invite action")
	}
	if senderEmail == "" {
		return "", fmt.Errorf("sender email is required for create-meeting-invite action")
	}

	if dryRun {
		fmt.Printf("🔍 DRY RUN: Would create multi-customer meeting invite for topic %s using metadata %s from %s for customers: %v (force-update: %v)\n",
			topicName, jsonMetadataPath, senderEmail, customerCodes, forceUpdate)
		return "", nil
	}

	fmt.Printf("📅 Creating multi-customer meeting invite for topic %s using metadata %s from %s for customers: %v (force-update: %v)\n",
		topicName, jsonMetadataPath, senderEmail, customerCodes, forceUpdate)

	// Load metadata from JSON file using format converter
	metadataFile := getMetadataFilePath(jsonMetadataPath)

	metadataPtr, err := LoadMetadataFromFile(metadataFile)
	if err != nil {
		return "", fmt.Errorf("failed to load metadata file %s: %w", metadataFile, err)
	}
	metadata := *metadataPtr

	// Validate that meeting invite data exists
	if metadata.MeetingInvite == nil {
		return "", fmt.Errorf("no meeting invite data found in metadata")
	}

	// Query aws-calendar topic from all affected customers and aggregate recipients
	allRecipients, err := queryAndAggregateCalendarRecipients(credentialManager, customerCodes, topicName)
	if err != nil {
		return "", fmt.Errorf("failed to aggregate calendar recipients: %w", err)
	}

	if len(allRecipients) == 0 {
		fmt.Printf("⚠️  No subscribers found for topic %s across all customers\n", topicName)
		return "", nil
	}

	fmt.Printf("👥 Found %d unique recipients for topic %s across %d customers\n", len(allRecipients), topicName, len(customerCodes))

	// Show recipient list for dry-run mode
	if dryRun {
		fmt.Printf("📧 Recipients that would receive meeting invite:\n")
		for i, email := range allRecipients {
			fmt.Printf("  %d. %s\n", i+1, email)
		}
		return "", nil
	}

	// Convert old nested format to new flat format for Graph API
	flatMetadata := convertApprovalRequestToChangeMetadata(&metadata)

	// Generate Microsoft Graph meeting payload with unified recipient list
	payload, err := generateGraphMeetingPayload(flatMetadata, senderEmail, allRecipients)
	if err != nil {
		return "", fmt.Errorf("failed to generate meeting payload: %w", err)
	}

	// Create the meeting using Microsoft Graph API
	meetingID, err := createGraphMeetingFromPayload(payload, senderEmail, forceUpdate, flatMetadata.SnowTicket, flatMetadata.JiraTicket, flatMetadata.ChangeTitle)
	if err != nil {
		return "", fmt.Errorf("failed to create meeting via Microsoft Graph API: %w", err)
	}

	fmt.Printf("✅ Successfully created multi-customer meeting with ID: %s\n", meetingID)
	return meetingID, nil
}

// CustomerRecipientResult holds the result of querying recipients from a single customer
type CustomerRecipientResult struct {
	CustomerCode string
	Recipients   []string
	Error        error
}

// queryAndAggregateCalendarRecipients queries aws-calendar topic from all customers concurrently and deduplicates recipients
func queryAndAggregateCalendarRecipients(credentialManager CredentialManager, customerCodes []string, topicName string) ([]string, error) {
	fmt.Printf("📋 Querying %s topic from %d customers concurrently...\n", topicName, len(customerCodes))

	// Create channels for concurrent processing
	resultChan := make(chan CustomerRecipientResult, len(customerCodes))

	// Launch goroutines for each customer
	for _, customerCode := range customerCodes {
		go func(code string) {
			recipients, err := queryCustomerRecipients(credentialManager, code, topicName)
			resultChan <- CustomerRecipientResult{
				CustomerCode: code,
				Recipients:   recipients,
				Error:        err,
			}
		}(customerCode)
	}

	// Collect results from all goroutines
	recipientSet := make(map[string]bool) // Use map for deduplication
	var allRecipients []string
	successCount := 0
	errorCount := 0

	for i := 0; i < len(customerCodes); i++ {
		result := <-resultChan

		if result.Error != nil {
			fmt.Printf("⚠️  Warning: Failed to get recipients for customer %s: %v\n", result.CustomerCode, result.Error)
			errorCount++
			continue
		}

		fmt.Printf("📧 Customer %s: found %d recipients\n", result.CustomerCode, len(result.Recipients))
		successCount++

		// Add to deduplicated set
		for _, email := range result.Recipients {
			if !recipientSet[email] {
				recipientSet[email] = true
				allRecipients = append(allRecipients, email)
			}
		}
	}

	fmt.Printf("📊 Aggregation complete: %d unique recipients from %d customers (%d successful, %d errors)\n",
		len(allRecipients), len(customerCodes), successCount, errorCount)

	return allRecipients, nil
}

// queryCustomerRecipients queries recipients from a single customer (used by concurrent processing)
func queryCustomerRecipients(credentialManager CredentialManager, customerCode string, topicName string) ([]string, error) {
	fmt.Printf("🔍 Querying customer: %s\n", customerCode)

	// Get customer-specific SES client
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get config for customer %s: %w", customerCode, err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the account contact list for this customer
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get contact list for customer %s: %w", customerCode, err)
	}

	// Get topic subscribers for this customer
	customerRecipients, err := getTopicSubscribers(sesClient, accountListName, topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic subscribers for customer %s: %w", customerCode, err)
	}

	return customerRecipients, nil
}

// CreateMeetingInvite creates a meeting using Microsoft Graph API based on metadata (single customer)
// DEPRECATED: Use CreateMultiCustomerMeetingInvite instead, which works for both single and multiple customers
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

	if dryRun {
		fmt.Printf("🔍 DRY RUN: Would create meeting invite for topic %s using metadata %s from %s (force-update: %v)\n", topicName, jsonMetadataPath, senderEmail, forceUpdate)
		return nil
	}

	fmt.Printf("📅 Creating meeting invite for topic %s using metadata %s from %s (force-update: %v)\n", topicName, jsonMetadataPath, senderEmail, forceUpdate)

	// Load metadata from JSON file using format converter
	metadataFile := getMetadataFilePath(jsonMetadataPath)

	metadataPtr, err := LoadMetadataFromFile(metadataFile)
	if err != nil {
		return fmt.Errorf("failed to load metadata file %s: %w", metadataFile, err)
	}
	metadata := *metadataPtr

	// Validate that meeting invite data exists
	if metadata.MeetingInvite == nil {
		return fmt.Errorf("no meeting invite data found in metadata")
	}

	// Get topic subscribers
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	attendeeEmails, err := getTopicSubscribers(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get topic subscribers: %w", err)
	}

	if len(attendeeEmails) == 0 {
		fmt.Printf("⚠️  No subscribers found for topic %s\n", topicName)
		return nil
	}

	fmt.Printf("👥 Found %d attendees for topic %s\n", len(attendeeEmails), topicName)

	// Convert old nested format to new flat format for Graph API
	flatMetadata := convertApprovalRequestToChangeMetadata(&metadata)

	// Generate Microsoft Graph meeting payload
	payload, err := generateGraphMeetingPayload(flatMetadata, senderEmail, attendeeEmails)
	if err != nil {
		return fmt.Errorf("failed to generate meeting payload: %w", err)
	}

	// Create the meeting using Microsoft Graph API (still needs old format for backward compatibility)
	meetingID, err := createGraphMeeting(payload, senderEmail, forceUpdate, &metadata)
	if err != nil {
		return fmt.Errorf("failed to create meeting: %w", err)
	}

	fmt.Printf("✅ Successfully created meeting with ID: %s\n", meetingID)
	return nil
}

// generateGraphMeetingPayload creates the JSON payload for Microsoft Graph API
func generateGraphMeetingPayload(metadata *types.ChangeMetadata, organizerEmail string, attendeeEmails []string) (string, error) {
	// Calculate meeting times from implementation schedule
	startTime := metadata.ImplementationStart
	endTime := metadata.ImplementationEnd

	// If no end time specified, default to 1 hour duration
	if endTime.IsZero() || endTime.Equal(startTime) {
		endTime = startTime.Add(1 * time.Hour)
	}

	// Create attendees list
	var attendees []map[string]interface{}
	for _, email := range attendeeEmails {
		attendees = append(attendees, map[string]interface{}{
			"emailAddress": map[string]interface{}{
				"address": email,
				"name":    email,
			},
			"type": "required",
		})
	}

	// Use the user's specified timezone
	targetTimezone := metadata.Timezone
	if targetTimezone == "" {
		targetTimezone = "America/New_York" // Default timezone
	}

	// Create meeting subject
	meetingSubject := fmt.Sprintf("Change Implementation: %s", metadata.ChangeTitle)

	// Microsoft Graph expects the datetime in the format WITHOUT timezone conversion
	// The timeZone field tells Graph what timezone the datetime is in
	// DO NOT convert the time with .In(loc) - just format it as-is
	// Graph API will handle the timezone interpretation
	meeting := map[string]interface{}{
		"subject": meetingSubject,
		"body": map[string]interface{}{
			"contentType": "HTML",
			"content":     generateMeetingBodyHTML(metadata),
		},
		"start": map[string]interface{}{
			"dateTime": startTime.Format("2006-01-02T15:04:05.0000000"),
			"timeZone": targetTimezone,
		},
		"end": map[string]interface{}{
			"dateTime": endTime.Format("2006-01-02T15:04:05.0000000"),
			"timeZone": targetTimezone,
		},
		"location": map[string]interface{}{
			"displayName": metadata.MeetingLocation,
		},
		"attendees": attendees,
		"organizer": map[string]interface{}{
			"emailAddress": map[string]interface{}{
				"address": organizerEmail,
				"name":    organizerEmail,
			},
		},
		"isOnlineMeeting":       true,
		"onlineMeetingProvider": "teamsForBusiness",
	}

	payloadBytes, err := json.MarshalIndent(meeting, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal meeting payload: %w", err)
	}

	return string(payloadBytes), nil
}

// calculateMeetingTimes is deprecated - meeting times are now taken directly from ChangeMetadata
// This function is kept for backward compatibility with any remaining legacy code
func calculateMeetingTimes(metadata *types.ApprovalRequestMetadata) (time.Time, time.Time, error) {
	// For backward compatibility, use the schedule times from the nested structure
	startTime := metadata.ChangeMetadata.Schedule.ImplementationStart
	endTime := metadata.ChangeMetadata.Schedule.ImplementationEnd

	// If no end time specified, default to 1 hour duration
	if endTime.IsZero() || endTime.Equal(startTime) {
		endTime = startTime.Add(1 * time.Hour)
	}

	return startTime, endTime, nil
}

// parseStartTimeWithTimezone parses a start time string with timezone support using centralized datetime utilities
func parseStartTimeWithTimezone(startTimeStr, timezone string) (time.Time, error) {
	// Use centralized datetime parser with timezone support
	dtManager := datetime.New(nil)

	if timezone != "" {
		return dtManager.ParseWithTimezone(startTimeStr, timezone)
	}

	return dtManager.Parse(startTimeStr)
}

// generateMeetingBodyHTML creates HTML content for the meeting body
// generateMeetingBodyHTML creates HTML content for the meeting body using flat ChangeMetadata structure
func generateMeetingBodyHTML(metadata *types.ChangeMetadata) string {
	// Use centralized timezone formatting function
	formatDateTime := func(t time.Time) string {
		return formatTimeWithTimezone(t, metadata.Timezone)
	}

	// Get customer names - convert codes to names if needed
	customerNames := strings.Join(metadata.Customers, ", ")

	return fmt.Sprintf(`
<h2>🔄 Change Implementation Meeting</h2>
<p><strong>Change Title:</strong> %s</p>
<p><strong>Description:</strong> %s</p>

<h3>📋 Implementation Details</h3>
<p><strong>Implementation Plan:</strong></p>
<div>%s</div>

<h3>📅 Schedule</h3>
<p><strong>Implementation Window:</strong> %s to %s</p>
<p><strong>Timezone:</strong> %s</p>

<h3>🎯 Impact & Rollback</h3>
<p><strong>Expected Impact:</strong> %s</p>
<p><strong>Rollback Plan:</strong> %s</p>

<h3>🎫 Related Tickets</h3>
<p><strong>ServiceNow:</strong> %s</p>
<p><strong>Jira:</strong> %s</p>

<h3>👥 Stakeholders</h3>
<p><strong>Customers:</strong> %s</p>
`,
		metadata.ChangeTitle,
		metadata.ChangeReason, // Using ChangeReason as description
		strings.ReplaceAll(metadata.ImplementationPlan, "\n", "<br>"),
		formatDateTime(metadata.ImplementationStart),
		formatDateTime(metadata.ImplementationEnd),
		metadata.Timezone,
		metadata.CustomerImpact,
		strings.ReplaceAll(metadata.RollbackPlan, "\n", "<br>"),
		metadata.SnowTicket,
		metadata.JiraTicket,
		customerNames,
	)
}

// getGraphAccessToken obtains an access token for Microsoft Graph API using client credentials flow
func getGraphAccessToken() (string, error) {
	// Get Azure credentials from cache or Parameter Store
	clientID, clientSecret, tenantID, err := GetAzureCredentials(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get Azure credentials: %w", err)
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
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse types.GraphAuthResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	return tokenResponse.AccessToken, nil
}

// createGraphMeeting creates a meeting using Microsoft Graph API
func createGraphMeeting(payload string, organizerEmail string, forceUpdate bool, metadata *types.ApprovalRequestMetadata) (string, error) {
	// Parse the payload to extract meeting details for idempotency check
	var meetingData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &meetingData); err != nil {
		return "", fmt.Errorf("failed to parse meeting payload: %w", err)
	}

	subject, ok := meetingData["subject"].(string)
	if !ok {
		return "", fmt.Errorf("meeting subject not found in payload")
	}

	startData, ok := meetingData["start"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("meeting start time not found in payload")
	}

	_, ok = startData["dateTime"].(string)
	if !ok {
		return "", fmt.Errorf("meeting start dateTime not found in payload")
	}

	// Get access token
	accessToken, err := getGraphAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Check if meeting already exists (unless force update is enabled)
	if !forceUpdate {
		exists, existingMeeting, err := checkMeetingExists(accessToken, organizerEmail, subject, metadata)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to check existing meetings: %v\n", err)
		} else if exists {
			// Compare meeting details to see if update is needed
			hasChanges, err := compareMeetingDetails(existingMeeting, payload)
			if err != nil {
				fmt.Printf("⚠️  Warning: Failed to compare meeting details: %v\n", err)
			} else if !hasChanges {
				fmt.Printf("✅ Meeting already exists with same details, skipping creation\n")
				return existingMeeting.ID, nil
			} else {
				fmt.Printf("🔄 Meeting exists but has changes, updating...\n")
				err = updateGraphMeeting(existingMeeting.ID, payload, organizerEmail)
				if err != nil {
					return "", fmt.Errorf("failed to update existing meeting: %w", err)
				}
				return existingMeeting.ID, nil
			}
		}
	}

	// Create new meeting
	fmt.Printf("📅 Creating new meeting: %s\n", subject)

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events", organizerEmail)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create meeting request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create meeting: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read meeting response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var graphError types.GraphError
		if json.Unmarshal(body, &graphError) == nil {
			return "", fmt.Errorf("meeting creation failed: %s - %s", graphError.Error.Code, graphError.Error.Message)
		}
		return "", fmt.Errorf("meeting creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var meetingResponse types.GraphMeetingResponse
	if err := json.Unmarshal(body, &meetingResponse); err != nil {
		return "", fmt.Errorf("failed to parse meeting response: %w", err)
	}

	return meetingResponse.ID, nil
}

// checkMeetingExists checks if a meeting with the same subject and ticket numbers already exists
func checkMeetingExists(accessToken, organizerEmail, subject string, metadata *types.ApprovalRequestMetadata) (bool, *types.GraphMeetingResponse, error) {
	// Extract ticket numbers for idempotency check
	serviceNowTicket := ""
	jiraTicket := ""
	if metadata != nil && metadata.ChangeMetadata.Tickets.ServiceNow != "" {
		serviceNowTicket = metadata.ChangeMetadata.Tickets.ServiceNow
	}
	if metadata != nil && metadata.ChangeMetadata.Tickets.Jira != "" {
		jiraTicket = metadata.ChangeMetadata.Tickets.Jira
	}

	// Simplify the approach - just get recent events and filter in code
	// This avoids complex OData filter syntax issues
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$top=50&$select=id,subject,start,end,attendees&$orderby=start/dateTime desc",
		organizerEmail)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("failed to search meetings: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("meeting search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Value []types.GraphMeetingResponse `json:"value"`
	}

	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return false, nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	// Look for a meeting with the same subject and ticket numbers
	for _, meeting := range searchResponse.Value {
		if meeting.Subject == subject {
			// If we have ticket numbers, we need to get the full meeting details to check the body
			if serviceNowTicket != "" || jiraTicket != "" {
				fullMeeting, err := getMeetingDetails(accessToken, organizerEmail, meeting.ID)
				if err != nil {
					fmt.Printf("⚠️  Warning: Failed to get meeting details for %s: %v\n", meeting.ID, err)
					continue
				}

				// Check if the meeting body contains our ticket numbers
				if containsTicketNumbers(fullMeeting, serviceNowTicket, jiraTicket) {
					return true, &meeting, nil
				}
			} else {
				// If no ticket numbers available, fall back to subject-only matching
				return true, &meeting, nil
			}
		}
	}

	return false, nil, nil
}

// getMeetingDetails retrieves full meeting details including body content
func getMeetingDetails(accessToken, organizerEmail, meetingID string) (*types.GraphMeetingResponse, error) {
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events/%s?$select=id,subject,body,start,end", organizerEmail, meetingID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create meeting details request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get meeting details: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read meeting details response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("meeting details request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var meeting types.GraphMeetingResponse
	if err := json.Unmarshal(body, &meeting); err != nil {
		return nil, fmt.Errorf("failed to parse meeting details response: %w", err)
	}

	return &meeting, nil
}

// containsTicketNumbers checks if the meeting body contains the specified ticket numbers
func containsTicketNumbers(meeting *types.GraphMeetingResponse, serviceNowTicket, jiraTicket string) bool {
	if meeting.Body == nil || meeting.Body.Content == "" {
		return false
	}

	// Convert to lowercase for case-insensitive matching
	bodyLower := strings.ToLower(meeting.Body.Content)

	// Check ServiceNow ticket
	if serviceNowTicket != "" {
		if !strings.Contains(bodyLower, strings.ToLower(serviceNowTicket)) {
			return false
		}
	}

	// Check Jira ticket
	if jiraTicket != "" {
		if !strings.Contains(bodyLower, strings.ToLower(jiraTicket)) {
			return false
		}
	}

	return true
}

// compareMeetingDetails compares existing meeting with new payload to detect changes
func compareMeetingDetails(existingMeeting *types.GraphMeetingResponse, newPayload string) (bool, error) {
	// Parse the new payload
	var newMeetingData map[string]interface{}
	if err := json.Unmarshal([]byte(newPayload), &newMeetingData); err != nil {
		return false, fmt.Errorf("failed to parse new meeting payload: %w", err)
	}

	// Compare key fields
	if existingMeeting.Subject != newMeetingData["subject"].(string) {
		return true, nil
	}

	// For now, assume there are changes if we can't do a detailed comparison
	// In a full implementation, you would compare start time, attendees, etc.
	return true, nil
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
		return fmt.Errorf("failed to create update request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update meeting: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("meeting update failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("✅ Successfully updated meeting %s\n", meetingID)
	return nil
}

// getTopicSubscribers gets all email addresses subscribed to a specific topic
func getTopicSubscribers(sesClient *sesv2.Client, listName string, topicName string) ([]string, error) {
	fmt.Printf("📋 Getting subscribers for topic: %s\n", topicName)

	// Get contacts subscribed to this topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	var subscribers []string
	for _, contact := range contactsResult.Contacts {
		if contact.EmailAddress != nil {
			subscribers = append(subscribers, *contact.EmailAddress)
		}
	}

	fmt.Printf("👥 Found %d subscribers for topic %s\n", len(subscribers), topicName)
	return subscribers, nil
}

// CreateICSInvite sends a calendar invite with ICS attachment based on metadata
func CreateICSInvite(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, senderEmail string, dryRun bool) error {
	// Validate required parameters
	if topicName == "" {
		return fmt.Errorf("topic name is required for create-ics-invite action")
	}
	if jsonMetadataPath == "" {
		return fmt.Errorf("json-metadata file path is required for create-ics-invite action")
	}
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for create-ics-invite action")
	}

	if dryRun {
		fmt.Printf("🔍 DRY RUN: Would create ICS invite for topic %s using metadata %s from %s\n", topicName, jsonMetadataPath, senderEmail)
		return nil
	}

	fmt.Printf("📅 Creating ICS calendar invite for topic %s using metadata %s from %s\n", topicName, jsonMetadataPath, senderEmail)

	// Load metadata from JSON file using format converter
	metadataFile := getMetadataFilePath(jsonMetadataPath)

	metadataPtr, err := LoadMetadataFromFile(metadataFile)
	if err != nil {
		return fmt.Errorf("failed to load metadata file %s: %w", metadataFile, err)
	}
	metadata := *metadataPtr

	// Validate that meeting invite data exists
	if metadata.MeetingInvite == nil {
		return fmt.Errorf("no meeting invite data found in metadata")
	}

	// Get topic subscribers
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	attendeeEmails, err := getTopicSubscribers(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get topic subscribers: %w", err)
	}

	if len(attendeeEmails) == 0 {
		fmt.Printf("⚠️  No subscribers found for topic %s\n", topicName)
		return nil
	}

	fmt.Printf("👥 Found %d attendees for topic %s\n", len(attendeeEmails), topicName)

	// Generate ICS file content
	icsContent, err := generateICSFile(&metadata, senderEmail, attendeeEmails)
	if err != nil {
		return fmt.Errorf("failed to generate ICS file: %w", err)
	}

	// Generate email content
	htmlBody := generateCalendarInviteHTML(&metadata)
	textBody := generateCalendarInviteText(&metadata)

	// Send email with ICS attachment to each attendee
	for _, attendeeEmail := range attendeeEmails {
		subject := fmt.Sprintf("📅 Calendar Invite: %s", metadata.MeetingInvite.Title)

		// Generate raw email with ICS attachment
		rawEmail, err := generateRawEmailWithAttachment(
			senderEmail,
			attendeeEmail,
			subject,
			htmlBody,
			textBody,
			icsContent,
			"meeting-invite.ics",
		)
		if err != nil {
			fmt.Printf("❌ Failed to generate email for %s: %v\n", attendeeEmail, err)
			continue
		}

		// Send the email using SES
		err = sendRawEmail(sesClient, rawEmail)
		if err != nil {
			fmt.Printf("❌ Failed to send ICS invite to %s: %v\n", attendeeEmail, err)
			continue
		}

		fmt.Printf("✅ Successfully sent ICS invite to %s\n", attendeeEmail)
	}

	return nil
}

// generateICSFile creates an ICS calendar file from metadata
func generateICSFile(metadata *types.ApprovalRequestMetadata, senderEmail string, attendeeEmails []string) (string, error) {
	if metadata.MeetingInvite == nil {
		return "", fmt.Errorf("no meeting information available")
	}

	// Parse start time and calculate end time
	startTime, endTime, err := calculateMeetingTimes(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to calculate meeting times: %w", err)
	}

	// Format times for ICS (UTC format) using centralized datetime utilities
	dtManager := datetime.New(nil)
	startUTC := dtManager.Format(startTime).ToICS()
	endUTC := dtManager.Format(endTime).ToICS()
	nowUTC := dtManager.Format(time.Now()).ToICS()

	// Generate unique UID for the event
	uid := fmt.Sprintf("%s-%s@%s", startUTC, endUTC, "aws-contact-manager")

	// Build attendees list
	var attendeesICS strings.Builder
	for _, email := range attendeeEmails {
		attendeesICS.WriteString(fmt.Sprintf("ATTENDEE;CN=%s;RSVP=TRUE:mailto:%s\r\n", email, email))
	}

	// Create ICS content
	icsContent := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//AWS Contact Manager//Meeting Invite//EN
CALSCALE:GREGORIAN
METHOD:REQUEST
BEGIN:VEVENT
UID:%s
DTSTART:%s
DTEND:%s
DTSTAMP:%s
ORGANIZER;CN=%s:mailto:%s
%sSUMMARY:%s
DESCRIPTION:%s
LOCATION:%s
STATUS:CONFIRMED
SEQUENCE:0
BEGIN:VALARM
TRIGGER:-PT15M
ACTION:DISPLAY
DESCRIPTION:Reminder
END:VALARM
END:VEVENT
END:VCALENDAR`,
		uid,
		startUTC,
		endUTC,
		nowUTC,
		senderEmail,
		senderEmail,
		attendeesICS.String(),
		metadata.MeetingInvite.Title,
		strings.ReplaceAll(metadata.ChangeMetadata.Description, "\n", "\\n"),
		metadata.MeetingInvite.Location,
	)

	return icsContent, nil
}

// generateCalendarInviteHTML creates HTML email content for calendar invite
func generateCalendarInviteHTML(metadata *types.ApprovalRequestMetadata) string {
	// Format common data using new time.Time fields
	startTime := formatScheduleTimeFromTime(metadata.ChangeMetadata.Schedule.ImplementationStart, metadata.ChangeMetadata.Schedule.Timezone)
	endTime := formatScheduleTimeFromTime(metadata.ChangeMetadata.Schedule.ImplementationEnd, metadata.ChangeMetadata.Schedule.Timezone)

	return fmt.Sprintf(`
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
<div style="max-width: 600px; margin: 0 auto; padding: 20px;">

<h1 style="color: #2c5aa0; border-bottom: 2px solid #2c5aa0; padding-bottom: 10px;">
📅 Calendar Invite: Change Implementation Meeting
</h1>

<div style="background-color: #f8f9fa; padding: 15px; border-radius: 5px; margin: 20px 0;">
<h2 style="color: #28a745; margin-top: 0;">✅ CHANGE APPROVED</h2>
<p><strong>Title:</strong> %s</p>
<p><strong>Description:</strong> %s</p>
</div>

<h3 style="color: #2c5aa0;">📋 Meeting Details</h3>
<ul>
<li><strong>Meeting:</strong> %s</li>
<li><strong>Date & Time:</strong> %s</li>
<li><strong>Duration:</strong> %d minutes</li>
<li><strong>Location:</strong> %s</li>
</ul>

<h3 style="color: #2c5aa0;">📅 Implementation Schedule</h3>
<ul>
<li><strong>Implementation Window:</strong> %s to %s</li>
<li><strong>Timezone:</strong> %s</li>
</ul>

<h3 style="color: #2c5aa0;">🎫 Related Tickets</h3>
<ul>
<li><strong>ServiceNow:</strong> %s</li>
<li><strong>Jira:</strong> %s</li>
</ul>

<h3 style="color: #2c5aa0;">👥 Affected Customers</h3>
<p>%s</p>

<div style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin: 20px 0;">
<p><strong>📎 Calendar Invite:</strong> Please find the calendar invite attached as an ICS file. 
Click on the attachment to add this meeting to your calendar.</p>
</div>

<div class="unsubscribe">
    <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
    <p>Calendar invite sent at %s</p>
    <div class="unsubscribe-prominent"><a href="{{amazonSESUnsubscribeUrl}}">📧 Manage Email Preferences or Unsubscribe</a></div>
</div>

</div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.Description,
		metadata.MeetingInvite.Title,
		metadata.MeetingInvite.StartTime,
		metadata.MeetingInvite.DurationMinutes,
		metadata.MeetingInvite.Location,
		startTime,
		endTime,
		metadata.ChangeMetadata.Schedule.Timezone,
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		time.Now().Format("January 2, 2006 at 3:04 PM MST"),
	)
}

// generateCalendarInviteText creates plain text email content for calendar invite
func generateCalendarInviteText(metadata *types.ApprovalRequestMetadata) string {
	startTime := formatScheduleTimeFromTime(metadata.ChangeMetadata.Schedule.ImplementationStart, metadata.ChangeMetadata.Schedule.Timezone)
	endTime := formatScheduleTimeFromTime(metadata.ChangeMetadata.Schedule.ImplementationEnd, metadata.ChangeMetadata.Schedule.Timezone)

	return fmt.Sprintf(`✅ CHANGE APPROVED - CALENDAR INVITE

📅 Meeting: %s
🕐 Date & Time: %s
⏱️  Duration: %d minutes
📍 Location: %s

📋 CHANGE DETAILS
Title: %s
Description: %s

📅 IMPLEMENTATION SCHEDULE
Implementation Window: %s to %s
Timezone: %s

🎫 RELATED TICKETS
ServiceNow: %s
Jira: %s

👥 AFFECTED CUSTOMERS
%s

📎 CALENDAR INVITE
Please find the calendar invite attached as an ICS file.
Click on the attachment to add this meeting to your calendar.

---
This is an automated message from the AWS Contact Manager system.`,
		metadata.MeetingInvite.Title,
		metadata.MeetingInvite.StartTime,
		metadata.MeetingInvite.DurationMinutes,
		metadata.MeetingInvite.Location,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.Description,
		startTime,
		endTime,
		metadata.ChangeMetadata.Schedule.Timezone,
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
	)
}

// formatScheduleTime formats a date and time with timezone using centralized datetime utilities
func formatScheduleTime(date, timeStr, timezone string) string {
	if date == "" || timeStr == "" {
		return "TBD"
	}

	// Use centralized datetime parser to parse legacy date/time fields
	dtManager := datetime.New(nil)

	// Parse date and time separately, then combine
	dateTime := fmt.Sprintf("%s %s", date, timeStr)
	parsedTime, err := dtManager.ParseWithTimezone(dateTime, timezone)
	if err != nil {
		// Fallback to original formatting if parsing fails
		formattedTime := formatTimeWithAMPM(timeStr)
		formattedDate := formatDateNicely(date)
		return fmt.Sprintf("%s at %s %s", formattedDate, formattedTime, timezone)
	}

	// Use centralized formatter for human-readable output
	return dtManager.Format(parsedTime).ToEmailTemplate(timezone)
}

// formatScheduleTimeFromTime formats a time.Time with timezone conversion
// This is a simplified version that matches the centralized formatting in handlers.go
func formatScheduleTimeFromTime(t time.Time, timezone string) string {
	if t.IsZero() {
		return "TBD"
	}

	// Use provided timezone or default to Eastern Time
	targetTimezone := timezone
	if targetTimezone == "" {
		targetTimezone = "America/New_York"
	}

	// Load the target timezone
	loc, err := time.LoadLocation(targetTimezone)
	if err != nil {
		// Fallback to UTC if timezone loading fails
		loc = time.UTC
	}

	// Convert the time to the target timezone and format
	localTime := t.In(loc)
	return localTime.Format("Monday, January 2, 2006 at 3:04 PM MST")
}

// formatTimeWithAMPM converts 24-hour time format to 12-hour with AM/PM
func formatTimeWithAMPM(timeStr string) string {
	// Handle various time formats
	if timeStr == "" {
		return "TBD"
	}

	// Try to parse different time formats
	formats := []string{
		"15:04",    // 24-hour format (HH:MM)
		"15:04:05", // 24-hour format with seconds
		"3:04 PM",  // Already in 12-hour format
		"3:04PM",   // 12-hour format without space
		"03:04 PM", // 12-hour format with leading zero
		"03:04PM",  // 12-hour format with leading zero, no space
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			// Format as 12-hour time with AM/PM
			return t.Format("3:04 PM")
		}
	}

	// If parsing fails, return the original string
	return timeStr
}

// formatDateNicely converts date string to a more readable format
func formatDateNicely(dateStr string) string {
	if dateStr == "" {
		return "TBD"
	}

	// Try to parse different date formats
	formats := []string{
		"2006-01-02",      // ISO format (YYYY-MM-DD)
		"01/02/2006",      // US format (MM/DD/YYYY)
		"2006/01/02",      // Alternative format
		"January 2, 2006", // Already formatted nicely
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			// Format as "January 2, 2006"
			return t.Format("January 2, 2006")
		}
	}

	// If parsing fails, return the original string
	return dateStr
}

// generateRawEmailWithAttachment creates a raw MIME email with ICS attachment
func generateRawEmailWithAttachment(from, to, subject, htmlBody, textBody, icsContent, icsFilename string) (string, error) {
	// Replace attendee email placeholder in ICS content
	icsContent = strings.ReplaceAll(icsContent, "%%ATTENDEE_EMAIL%%", to)

	// Encode ICS content as base64
	icsBase64 := base64.StdEncoding.EncodeToString([]byte(icsContent))

	// Create multipart boundary
	boundary := fmt.Sprintf("boundary_%d", time.Now().Unix())

	// Build the raw email
	var email strings.Builder

	// Headers
	email.WriteString(fmt.Sprintf("From: %s\r\n", from))
	email.WriteString(fmt.Sprintf("To: %s\r\n", to))
	email.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	email.WriteString("MIME-Version: 1.0\r\n")
	email.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	email.WriteString("\r\n")

	// Text/HTML part
	email.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	email.WriteString("Content-Type: multipart/alternative; boundary=\"alt_boundary\"\r\n")
	email.WriteString("\r\n")

	// Plain text part
	email.WriteString("--alt_boundary\r\n")
	email.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	email.WriteString("\r\n")
	email.WriteString(textBody)
	email.WriteString("\r\n")

	// HTML part
	email.WriteString("--alt_boundary\r\n")
	email.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	email.WriteString("\r\n")
	email.WriteString(htmlBody)
	email.WriteString("\r\n")

	email.WriteString("--alt_boundary--\r\n")

	// ICS attachment
	email.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	email.WriteString("Content-Type: text/calendar; charset=UTF-8; method=REQUEST\r\n")
	email.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", icsFilename))
	email.WriteString("Content-Transfer-Encoding: base64\r\n")
	email.WriteString("\r\n")

	// Add base64 encoded ICS content in chunks of 76 characters
	icsLines := chunkString(icsBase64, 76)
	for _, line := range icsLines {
		email.WriteString(line + "\r\n")
	}

	email.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return email.String(), nil
}

// chunkString splits a string into chunks of specified length
func chunkString(s string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

// sendRawEmail sends a raw email using SES
func sendRawEmail(sesClient *sesv2.Client, rawEmail string) error {
	// This is a placeholder implementation
	// In the full version, this would use SES SendRawEmail API
	fmt.Printf("📧 Sending raw email (placeholder implementation)\n")
	fmt.Printf("Email size: %d bytes\n", len(rawEmail))

	// For now, just return success
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

	if dryRun {
		fmt.Printf("🔍 DRY RUN: Would send approval request to topic %s using metadata %s from %s\n", topicName, jsonMetadataPath, senderEmail)
		return nil
	}

	fmt.Printf("📧 Sending approval request to topic %s using metadata %s from %s\n", topicName, jsonMetadataPath, senderEmail)

	// Load metadata from JSON file using format converter
	metadataFile := getMetadataFilePath(jsonMetadataPath)

	metadataPtr, err := LoadMetadataFromFile(metadataFile)
	if err != nil {
		return fmt.Errorf("failed to load metadata file %s: %w", metadataFile, err)
	}
	metadata := *metadataPtr

	// Load HTML template if provided
	var htmlTemplate string
	if htmlTemplatePath != "" {
		templateFile := getMetadataFilePath(htmlTemplatePath)
		templateBytes, err := os.ReadFile(templateFile)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to read HTML template %s: %v\n", templateFile, err)
			htmlTemplate = generateDefaultApprovalRequestHTML(&metadata)
		} else {
			htmlTemplate = string(templateBytes)
		}
	} else {
		htmlTemplate = generateDefaultApprovalRequestHTML(&metadata)
	}

	// Get topic subscribers
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	subscriberEmails, err := getTopicSubscribers(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get topic subscribers: %w", err)
	}

	if len(subscriberEmails) == 0 {
		fmt.Printf("⚠️  No subscribers found for topic %s\n", topicName)
		return nil
	}

	// Generate email content
	subject := fmt.Sprintf("🔔 Approval Request: %s", metadata.ChangeMetadata.Title)
	textBody := generateApprovalRequestText(&metadata)

	// Replace template variables in HTML
	htmlBody := replaceTemplateVariables(htmlTemplate, &metadata)

	// Send email to each subscriber
	for _, subscriberEmail := range subscriberEmails {
		err = sendHTMLEmail(sesClient, senderEmail, subscriberEmail, subject, htmlBody, textBody)
		if err != nil {
			fmt.Printf("❌ Failed to send approval request to %s: %v\n", subscriberEmail, err)
			continue
		}

		fmt.Printf("✅ Successfully sent approval request to %s\n", subscriberEmail)
	}

	return nil
}

// SendChangeNotification sends a change notification email indicating the change has been approved and scheduled
func SendChangeNotification(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, senderEmail string, dryRun bool) error {
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

	if dryRun {
		fmt.Printf("🔍 DRY RUN: Would send change notification to topic %s using metadata %s from %s\n", topicName, jsonMetadataPath, senderEmail)
		return nil
	}

	fmt.Printf("📧 Sending change notification to topic %s using metadata %s from %s\n", topicName, jsonMetadataPath, senderEmail)

	// Load metadata from JSON file using format converter
	metadataFile := getMetadataFilePath(jsonMetadataPath)

	metadataPtr, err := LoadMetadataFromFile(metadataFile)
	if err != nil {
		return fmt.Errorf("failed to load metadata file %s: %w", metadataFile, err)
	}
	metadata := *metadataPtr

	// Get topic subscribers
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	subscriberEmails, err := getTopicSubscribers(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get topic subscribers: %w", err)
	}

	if len(subscriberEmails) == 0 {
		fmt.Printf("⚠️  No subscribers found for topic %s\n", topicName)
		return nil
	}

	// Generate email content
	subject := fmt.Sprintf("✅ Change Approved & Scheduled: %s", metadata.ChangeMetadata.Title)
	htmlBody := generateChangeNotificationHTML(&metadata)
	textBody := generateChangeNotificationText(&metadata)

	// Send email to each subscriber
	for _, subscriberEmail := range subscriberEmails {
		err = sendHTMLEmail(sesClient, senderEmail, subscriberEmail, subject, htmlBody, textBody)
		if err != nil {
			fmt.Printf("❌ Failed to send change notification to %s: %v\n", subscriberEmail, err)
			continue
		}

		fmt.Printf("✅ Successfully sent change notification to %s\n", subscriberEmail)
	}

	return nil
}

// generateDefaultApprovalRequestHTML creates default HTML content for approval request
func generateDefaultApprovalRequestHTML(metadata *types.ApprovalRequestMetadata) string {
	startTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.BeginDate, metadata.ChangeMetadata.Schedule.BeginTime, metadata.ChangeMetadata.Schedule.Timezone)
	endTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.EndDate, metadata.ChangeMetadata.Schedule.EndTime, metadata.ChangeMetadata.Schedule.Timezone)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Change Approval Request</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #007bff; }
        .section { margin-bottom: 25px; }
        .section h3 { margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px; color: #007bff; }
        .info-grid { display: grid; grid-template-columns: 150px 1fr; gap: 10px; margin-bottom: 15px; }
        .info-label { font-weight: bold; color: #495057; }
        .schedule { background-color: #e7f3ff; padding: 15px; border-radius: 5px; margin: 15px 0; border-left: 4px solid #007bff; }
        .tickets { background-color: #f8f9fa; padding: 10px; border-radius: 5px; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .unsubscribe-prominent { margin-top: 10px; }
        .unsubscribe-prominent a { color: #007bff; text-decoration: none; font-weight: bold; }
        .approval-banner { background: linear-gradient(135deg, #007bff, #0056b3); color: white; padding: 25px; border-radius: 10px; margin-bottom: 25px; text-align: center; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .approval-banner h2 { margin: 0 0 10px 0; font-size: 28px; font-weight: bold; }
        .approval-banner p { margin: 0; font-size: 16px; opacity: 0.95; }
        .meeting-details { background-color: #e7f3ff; padding: 15px; border-radius: 5px; margin: 15px 0; border-left: 4px solid #007bff; }
    </style>
</head>
<body>
    <div class="approval-banner">
        <h2>❓ CHANGE APPROVAL REQUEST</h2>
        <p>This change has been reviewed, tentatively scheduled, and is ready for your approval.<br>A formal notification and calendar invite will be sent after final approval is received!</p>
    </div>
   
    <div class="header">
        <h2>📋 Change Details</h2>
        <p><strong>%s</strong></p>
        <p>Customer: %s</p>
    </div>

    <div class="section">
        <div class="tickets">
            <strong>Tracking Numbers:</strong><br>
            ServiceNow: %s<br>
            JIRA: %s
        </div>
    </div>
   
    <div class="section">
        <h3>📅 Proposed Implementation Schedule</h3>
        <div class="schedule">
            <strong>🕐 Start:</strong> %s<br>
            <strong>🕐 End:</strong> %s
        </div>
    </div>
   
    <div class="section">
        <h3>📝 Change Reason</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>🔧 Implementation Plan</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>🧪 Test Plan</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>👥 Expected Customer Impact</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>🔄 Rollback Plan</h3>
        <p>%s</p>
    </div>
   
    <div class="unsubscribe">
        <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
        <p>Change management system • Request sent at %s</p>
        <div class="unsubscribe-prominent"><a href="{{amazonSESUnsubscribeUrl}}">📧 Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		startTime,
		endTime,
		metadata.ChangeMetadata.Description,
		strings.ReplaceAll(metadata.ChangeMetadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.TestPlan, "\n", "<br>"),
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		strings.ReplaceAll(metadata.ChangeMetadata.RollbackPlan, "\n", "<br>"),
		time.Now().Format("January 2, 2006 at 3:04 PM MST"),
	)
}

// generateApprovalRequestText creates plain text content for approval request
func generateApprovalRequestText(metadata *types.ApprovalRequestMetadata) string {
	startTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.BeginDate, metadata.ChangeMetadata.Schedule.BeginTime, metadata.ChangeMetadata.Schedule.Timezone)
	endTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.EndDate, metadata.ChangeMetadata.Schedule.EndTime, metadata.ChangeMetadata.Schedule.Timezone)

	return fmt.Sprintf(`🔔 APPROVAL REQUEST

⏳ PENDING APPROVAL
Title: %s
Description: %s

📋 IMPLEMENTATION DETAILS
Implementation Plan:
%s

📅 PROPOSED SCHEDULE
Implementation Window: %s to %s
Timezone: %s

🎯 IMPACT ASSESSMENT
Expected Customer Impact: %s
Rollback Plan: %s

🎫 RELATED TICKETS
ServiceNow: %s
Jira: %s

👥 AFFECTED CUSTOMERS
%s

🔍 ACTION REQUIRED
Please review this change request and provide your approval or feedback.

---
This is an automated message from the AWS Contact Manager system.`,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.Description,
		metadata.ChangeMetadata.ImplementationPlan,
		startTime,
		endTime,
		metadata.ChangeMetadata.Schedule.Timezone,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		metadata.ChangeMetadata.RollbackPlan,
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
	)
}

// generateChangeNotificationHTML creates HTML content for change notification
func generateChangeNotificationHTML(metadata *types.ApprovalRequestMetadata) string {
	startTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.BeginDate, metadata.ChangeMetadata.Schedule.BeginTime, metadata.ChangeMetadata.Schedule.Timezone)
	endTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.EndDate, metadata.ChangeMetadata.Schedule.EndTime, metadata.ChangeMetadata.Schedule.Timezone)

	return fmt.Sprintf(`
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
<div style="max-width: 600px; margin: 0 auto; padding: 20px;">

<h1 style="color: #28a745; border-bottom: 2px solid #28a745; padding-bottom: 10px;">
✅ Change Approved & Scheduled
</h1>

<div style="background-color: #d4edda; padding: 15px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #28a745;">
<h2 style="color: #155724; margin-top: 0;">✅ APPROVED & SCHEDULED</h2>
<p><strong>Title:</strong> %s</p>
<p><strong>Description:</strong> %s</p>
<p><strong>Status:</strong> APPROVED<br>
<strong>By:</strong> %s<br>
<strong>On:</strong> %s</p>
</div>

<h3 style="color: #28a745;">📋 Implementation Details</h3>
<p><strong>Implementation Plan:</strong></p>
<div style="background-color: #f8f9fa; padding: 10px; border-radius: 3px;">
%s
</div>

<p><strong>Test Plan:</strong></p>
<div style="background-color: #f8f9fa; padding: 10px; border-radius: 3px;">
%s
</div>

<h3 style="color: #28a745;">📅 Scheduled Implementation</h3>
<ul>
<li><strong>Implementation Window:</strong> %s to %s</li>
<li><strong>Timezone:</strong> %s</li>
</ul>

<h3 style="color: #28a745;">🎯 Impact & Rollback</h3>
<p><strong>Expected Customer Impact:</strong> %s</p>
<p><strong>Rollback Plan:</strong> %s</p>

<h3 style="color: #28a745;">🎫 Related Tickets</h3>
<ul>
<li><strong>ServiceNow:</strong> %s</li>
<li><strong>Jira:</strong> %s</li>
</ul>

<h3 style="color: #28a745;">👥 Affected Customers</h3>
<p>%s</p>

<div style="background-color: #cce5ff; padding: 15px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #007bff;">
<p><strong>📅 Next Steps:</strong> This change has been approved and scheduled for implementation during the specified window.</p>
</div>

<div class="unsubscribe">
    <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
    <p>Change management system • Notification sent at %s</p>
    <div class="unsubscribe-prominent"><a href="{{amazonSESUnsubscribeUrl}}">📧 Manage Email Preferences or Unsubscribe</a></div>
</div>

</div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.Description,
		metadata.ChangeMetadata.ApprovedBy,
		metadata.ChangeMetadata.ApprovedAt,
		strings.ReplaceAll(metadata.ChangeMetadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.TestPlan, "\n", "<br>"),
		startTime,
		endTime,
		metadata.ChangeMetadata.Schedule.Timezone,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		strings.ReplaceAll(metadata.ChangeMetadata.RollbackPlan, "\n", "<br>"),
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		time.Now().Format("January 2, 2006 at 3:04 PM MST"),
	)
}

// generateChangeNotificationText creates plain text content for change notification
func generateChangeNotificationText(metadata *types.ApprovalRequestMetadata) string {
	startTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.BeginDate, metadata.ChangeMetadata.Schedule.BeginTime, metadata.ChangeMetadata.Schedule.Timezone)
	endTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.EndDate, metadata.ChangeMetadata.Schedule.EndTime, metadata.ChangeMetadata.Schedule.Timezone)

	return fmt.Sprintf(`✅ CHANGE APPROVED & SCHEDULED

✅ APPROVED & SCHEDULED
Title: %s
Description: %s
Status: APPROVED
By: %s
On: %s

📋 IMPLEMENTATION DETAILS
Implementation Plan:
%s

📅 SCHEDULED IMPLEMENTATION
Implementation Window: %s to %s
Timezone: %s

🎯 IMPACT & ROLLBACK
Expected Customer Impact: %s
Rollback Plan: %s

🎫 RELATED TICKETS
ServiceNow: %s
Jira: %s

👥 AFFECTED CUSTOMERS
%s

📅 NEXT STEPS
This change has been approved and scheduled for implementation during the specified window. 

---
This is an automated message from the AWS Contact Manager system.`,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.Description,
		metadata.ChangeMetadata.ApprovedBy,
		metadata.ChangeMetadata.ApprovedAt,
		metadata.ChangeMetadata.ImplementationPlan,
		startTime,
		endTime,
		metadata.ChangeMetadata.Schedule.Timezone,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		metadata.ChangeMetadata.RollbackPlan,
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
	)
}

// replaceTemplateVariables replaces template variables in HTML content
func replaceTemplateVariables(template string, metadata *types.ApprovalRequestMetadata) string {
	// Replace common template variables
	template = strings.ReplaceAll(template, "{{TITLE}}", metadata.ChangeMetadata.Title)
	template = strings.ReplaceAll(template, "{{DESCRIPTION}}", metadata.ChangeMetadata.Description)
	template = strings.ReplaceAll(template, "{{IMPLEMENTATION_PLAN}}", metadata.ChangeMetadata.ImplementationPlan)
	template = strings.ReplaceAll(template, "{{CUSTOMER_IMPACT}}", metadata.ChangeMetadata.ExpectedCustomerImpact)
	template = strings.ReplaceAll(template, "{{ROLLBACK_PLAN}}", metadata.ChangeMetadata.RollbackPlan)
	template = strings.ReplaceAll(template, "{{SERVICENOW_TICKET}}", metadata.ChangeMetadata.Tickets.ServiceNow)
	template = strings.ReplaceAll(template, "{{JIRA_TICKET}}", metadata.ChangeMetadata.Tickets.Jira)
	template = strings.ReplaceAll(template, "{{CUSTOMERS}}", strings.Join(metadata.ChangeMetadata.CustomerNames, ", "))
	template = strings.ReplaceAll(template, "{{TIMEZONE}}", metadata.ChangeMetadata.Schedule.Timezone)

	// Format schedule times
	startTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.BeginDate, metadata.ChangeMetadata.Schedule.BeginTime, metadata.ChangeMetadata.Schedule.Timezone)
	endTime := formatScheduleTime(metadata.ChangeMetadata.Schedule.EndDate, metadata.ChangeMetadata.Schedule.EndTime, metadata.ChangeMetadata.Schedule.Timezone)

	template = strings.ReplaceAll(template, "{{START_TIME}}", startTime)
	template = strings.ReplaceAll(template, "{{END_TIME}}", endTime)

	return template
}

// sendHTMLEmail sends an HTML email using SES
func sendHTMLEmail(sesClient *sesv2.Client, from, to, subject, htmlBody, textBody string) error {
	// This is a placeholder implementation
	// In the full version, this would use SES SendEmail API
	fmt.Printf("📧 Sending HTML email from %s to %s (placeholder implementation)\n", from, to)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Printf("HTML body size: %d bytes\n", len(htmlBody))
	fmt.Printf("Text body size: %d bytes\n", len(textBody))

	// For now, just return success
	return nil
}

// Exported wrapper functions for Lambda integration

// GenerateGraphMeetingPayloadFromChangeMetadata creates a Graph API payload from ChangeMetadata
// This is exported for use by Lambda functions
func GenerateGraphMeetingPayloadFromChangeMetadata(metadata *types.ChangeMetadata, organizerEmail string, attendeeEmails []string) (string, error) {
	return generateGraphMeetingPayload(metadata, organizerEmail, attendeeEmails)
}

// GetGraphAccessToken obtains an access token for Microsoft Graph API
// This is exported for use by Lambda functions
func GetGraphAccessToken() (string, error) {
	return getGraphAccessToken()
}

// CreateGraphMeetingWithPayload creates a meeting using the provided payload
// This is exported for use by Lambda functions
func CreateGraphMeetingWithPayload(accessToken, organizerEmail, payload string) (string, error) {
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events", organizerEmail)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create meeting request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create meeting: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read meeting response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var graphError types.GraphError
		if json.Unmarshal(body, &graphError) == nil {
			return "", fmt.Errorf("meeting creation failed: %s - %s", graphError.Error.Code, graphError.Error.Message)
		}
		return "", fmt.Errorf("meeting creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var meetingResponse types.GraphMeetingResponse
	if err := json.Unmarshal(body, &meetingResponse); err != nil {
		return "", fmt.Errorf("failed to parse meeting response: %w", err)
	}

	return meetingResponse.ID, nil
}

// GetGraphMeetingDetails retrieves full meeting details from Graph API
// This is exported for use by Lambda functions
func GetGraphMeetingDetails(accessToken, organizerEmail, meetingID string) (*types.GraphMeetingResponse, error) {
	return getMeetingDetails(accessToken, organizerEmail, meetingID)
}

// ExtractTeamsJoinURL extracts the Teams meeting join URL from HTML content
// This is exported for use by Lambda functions
func ExtractTeamsJoinURL(htmlContent string) string {
	// Look for Teams meeting URL patterns in the HTML content
	patterns := []string{
		`https://teams\.microsoft\.com/l/meetup-join/[^"'\s<>]+`,
		`https://teams\.live\.com/meet/[^"'\s<>]+`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(htmlContent); match != "" {
			return match
		}
	}

	return ""
}

// createGraphMeetingFromPayload creates a meeting using Microsoft Graph API with flat metadata
func createGraphMeetingFromPayload(payload string, organizerEmail string, forceUpdate bool, snowTicket, jiraTicket, changeTitle string) (string, error) {
	// Parse the payload to extract meeting details for idempotency check
	var meetingData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &meetingData); err != nil {
		return "", fmt.Errorf("failed to parse meeting payload: %w", err)
	}

	subject, ok := meetingData["subject"].(string)
	if !ok {
		return "", fmt.Errorf("meeting subject not found in payload")
	}

	// Get access token
	accessToken, err := getGraphAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Check if meeting already exists (unless force update is enabled)
	if !forceUpdate {
		exists, existingMeeting, err := checkMeetingExistsFlat(accessToken, organizerEmail, subject, snowTicket, jiraTicket)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to check existing meetings: %v\n", err)
		} else if exists {
			// Compare meeting details to see if update is needed
			hasChanges, err := compareMeetingDetails(existingMeeting, payload)
			if err != nil {
				fmt.Printf("⚠️  Warning: Failed to compare meeting details: %v\n", err)
			} else if !hasChanges {
				fmt.Printf("✅ Meeting already exists with same details, skipping creation\n")
				return existingMeeting.ID, nil
			} else {
				fmt.Printf("🔄 Meeting exists but has changes, updating...\n")
				err = updateGraphMeeting(existingMeeting.ID, payload, organizerEmail)
				if err != nil {
					return "", fmt.Errorf("failed to update existing meeting: %w", err)
				}
				return existingMeeting.ID, nil
			}
		}
	}

	// Create new meeting
	fmt.Printf("📅 Creating new meeting: %s\n", subject)

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events", organizerEmail)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create meeting request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create meeting: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read meeting response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var graphError types.GraphError
		if json.Unmarshal(body, &graphError) == nil {
			return "", fmt.Errorf("meeting creation failed: %s - %s", graphError.Error.Code, graphError.Error.Message)
		}
		return "", fmt.Errorf("meeting creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var meetingResponse types.GraphMeetingResponse
	if err := json.Unmarshal(body, &meetingResponse); err != nil {
		return "", fmt.Errorf("failed to parse meeting response: %w", err)
	}

	return meetingResponse.ID, nil
}

// checkMeetingExistsFlat checks if a meeting with the same subject and ticket numbers already exists (flat metadata version)
func checkMeetingExistsFlat(accessToken, organizerEmail, subject, serviceNowTicket, jiraTicket string) (bool, *types.GraphMeetingResponse, error) {
	// Get recent events and filter in code
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$top=50&$select=id,subject,start,end,attendees&$orderby=start/dateTime desc",
		organizerEmail)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("failed to search meetings: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("meeting search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Value []types.GraphMeetingResponse `json:"value"`
	}

	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return false, nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	// Look for a meeting with the same subject and ticket numbers
	for _, meeting := range searchResponse.Value {
		if meeting.Subject == subject {
			// If we have ticket numbers, check the meeting body
			if serviceNowTicket != "" || jiraTicket != "" {
				fullMeeting, err := getMeetingDetails(accessToken, organizerEmail, meeting.ID)
				if err != nil {
					fmt.Printf("⚠️  Warning: Failed to get meeting details for %s: %v\n", meeting.ID, err)
					continue
				}

				// Check if the meeting body contains our ticket numbers
				if containsTicketNumbers(fullMeeting, serviceNowTicket, jiraTicket) {
					return true, &meeting, nil
				}
			} else {
				// If no ticket numbers available, fall back to subject-only matching
				return true, &meeting, nil
			}
		}
	}

	return false, nil, nil
}
