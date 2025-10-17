// Package processors provides specialized processors for different object types
package processors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsinternal "ccoe-customer-contact-manager/internal/aws"
	"ccoe-customer-contact-manager/internal/ses"
	"ccoe-customer-contact-manager/internal/types"
)

// AnnouncementProcessor handles announcement-specific processing logic
type AnnouncementProcessor struct {
	S3Client   *s3.Client
	SESClient  *sesv2.Client
	GraphToken string
	Config     *types.Config
}

// NewAnnouncementProcessor creates a new announcement processor with required clients
func NewAnnouncementProcessor(s3Client *s3.Client, sesClient *sesv2.Client, graphToken string, cfg *types.Config) *AnnouncementProcessor {
	return &AnnouncementProcessor{
		S3Client:   s3Client,
		SESClient:  sesClient,
		GraphToken: graphToken,
		Config:     cfg,
	}
}

// ProcessAnnouncement processes an announcement based on its status
func (p *AnnouncementProcessor) ProcessAnnouncement(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	log.Printf("📢 Processing announcement for customer %s: %s (type: %s, status: %s)",
		customerCode, announcement.AnnouncementID, announcement.AnnouncementType, announcement.Status)

	switch announcement.Status {
	case "submitted":
		return p.handleSubmitted(ctx, customerCode, announcement)
	case "approved":
		return p.handleApproved(ctx, customerCode, announcement, s3Bucket, s3Key)
	case "cancelled":
		return p.handleCancelled(ctx, customerCode, announcement, s3Bucket, s3Key)
	case "completed":
		return p.handleCompleted(ctx, customerCode, announcement)
	default:
		log.Printf("⏭️  Skipping announcement %s - status is '%s' (not submitted/approved/cancelled/completed)",
			announcement.AnnouncementID, announcement.Status)
		return nil
	}
}

// handleSubmitted processes a submitted announcement (sends approval request)
func (p *AnnouncementProcessor) handleSubmitted(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	log.Printf("📧 Sending approval request for announcement %s", announcement.AnnouncementID)
	return p.sendApprovalRequest(ctx, customerCode, announcement)
}

// handleApproved processes an approved announcement (schedules meeting if needed, sends emails)
func (p *AnnouncementProcessor) handleApproved(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	log.Printf("✅ Announcement %s is approved, proceeding with processing", announcement.AnnouncementID)

	// Schedule meeting if requested
	if announcement.IncludeMeeting {
		log.Printf("📅 Scheduling meeting for announcement %s", announcement.AnnouncementID)
		err := p.scheduleMeeting(ctx, announcement, s3Bucket, s3Key)
		if err != nil {
			log.Printf("❌ Failed to schedule meeting for announcement %s: %v", announcement.AnnouncementID, err)
			// Don't fail the entire process if meeting scheduling fails
		} else {
			log.Printf("✅ Successfully scheduled meeting for announcement %s", announcement.AnnouncementID)
		}
	} else {
		log.Printf("⏭️  No meeting required for announcement %s", announcement.AnnouncementID)
	}

	// Send announcement emails
	log.Printf("📧 Sending announcement emails for %s", announcement.AnnouncementID)
	err := p.sendAnnouncementEmails(ctx, customerCode, announcement)
	if err != nil {
		// Check if error is due to no subscribers (not a real error)
		if strings.Contains(err.Error(), "no subscribers found") {
			log.Printf("ℹ️  %v", err)
		} else {
			log.Printf("❌ Failed to send announcement emails for customer %s: %v", customerCode, err)
			return fmt.Errorf("failed to send announcement emails: %w", err)
		}
	}

	log.Printf("✅ Announcement processing completed for customer %s: %s", customerCode, announcement.AnnouncementID)
	return nil
}

// handleCancelled processes a cancelled announcement (cancels meeting, sends cancellation email)
func (p *AnnouncementProcessor) handleCancelled(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	log.Printf("❌ Announcement %s cancelled, cancelling meeting if scheduled", announcement.AnnouncementID)

	// Cancel the meeting if one was scheduled
	if announcement.MeetingMetadata != nil && announcement.MeetingMetadata.MeetingID != "" {
		err := p.cancelMeeting(ctx, announcement, s3Bucket, s3Key)
		if err != nil {
			log.Printf("ERROR: Failed to cancel meeting for announcement %s: %v", announcement.AnnouncementID, err)
		}
	}

	// Send cancellation email
	log.Printf("📧 Sending cancellation email for announcement %s", announcement.AnnouncementID)
	err := p.sendCancellationEmail(ctx, customerCode, announcement)
	if err != nil {
		// Check if error is due to no subscribers (not a real error)
		if strings.Contains(err.Error(), "no subscribers found") {
			log.Printf("ℹ️  %v", err)
		} else {
			log.Printf("ERROR: Failed to send cancellation email for announcement %s: %v", announcement.AnnouncementID, err)
		}
	}

	log.Printf("✅ Announcement cancellation processing completed for customer %s: %s", customerCode, announcement.AnnouncementID)
	return nil
}

// handleCompleted processes a completed announcement (sends completion email)
func (p *AnnouncementProcessor) handleCompleted(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	log.Printf("🎉 Announcement %s marked as completed", announcement.AnnouncementID)

	// Send completion email
	log.Printf("📧 Sending completion email for announcement %s", announcement.AnnouncementID)
	err := p.sendCompletionEmail(ctx, customerCode, announcement)
	if err != nil {
		// Check if error is due to no subscribers (not a real error)
		if strings.Contains(err.Error(), "no subscribers found") {
			log.Printf("ℹ️  %v", err)
		} else {
			log.Printf("ERROR: Failed to send completion email for announcement %s: %v", announcement.AnnouncementID, err)
		}
	}

	log.Printf("✅ Announcement completion processing completed for customer %s: %s", customerCode, announcement.AnnouncementID)
	return nil
}

// sendApprovalRequest sends approval request email for the announcement
func (p *AnnouncementProcessor) sendApprovalRequest(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Get announcement data for email template
	data := p.convertToAnnouncementData(announcement)

	// Get appropriate template based on announcement type
	template := ses.GetAnnouncementApprovalRequestTemplate(announcement.AnnouncementType, data)

	// Send via SES topic management
	return p.sendEmailViaSES(ctx, customerCode, template)
}

// sendAnnouncementEmails sends type-specific announcement emails
func (p *AnnouncementProcessor) sendAnnouncementEmails(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Get announcement data for email template
	data := p.convertToAnnouncementData(announcement)

	// Get appropriate template based on announcement type
	template := ses.GetAnnouncementTemplate(announcement.AnnouncementType, data)

	// Send via SES topic management
	return p.sendEmailViaSES(ctx, customerCode, template)
}

// sendCancellationEmail sends cancellation notification email
func (p *AnnouncementProcessor) sendCancellationEmail(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Get announcement data for email template
	data := p.convertToAnnouncementData(announcement)

	// Get cancellation template
	template := ses.GetAnnouncementCancellationTemplate(announcement.AnnouncementType, data)

	// Send via SES topic management
	return p.sendEmailViaSES(ctx, customerCode, template)
}

// sendCompletionEmail sends completion notification email
func (p *AnnouncementProcessor) sendCompletionEmail(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Get announcement data for email template
	data := p.convertToAnnouncementData(announcement)

	// Get completion template
	template := ses.GetAnnouncementCompletionTemplate(announcement.AnnouncementType, data)

	// Send via SES topic management
	return p.sendEmailViaSES(ctx, customerCode, template)
}

// sendEmailViaSES sends email using SES topic management
func (p *AnnouncementProcessor) sendEmailViaSES(ctx context.Context, customerCode string, template ses.AnnouncementEmailTemplate) error {
	// Validate customer code exists
	if _, exists := p.Config.CustomerMappings[customerCode]; !exists {
		return fmt.Errorf("customer code %s not found in configuration", customerCode)
	}

	// Get customer info
	customerInfo := p.Config.CustomerMappings[customerCode]

	// Determine sender email
	senderEmail := "ccoe@nonprod.ccoe.hearst.com" // Default sender

	// Determine topic name based on email type
	var topicName string
	if template.Subject != "" && strings.Contains(template.Subject, "Approval Request") {
		// Approval requests go to unified announce-approval topic
		topicName = "announce-approval"
		log.Printf("Sending approval request to unified topic %s", topicName)
	} else {
		// Approved announcements go to customer-specific announcement topics
		topicName = p.getTopicNameForAnnouncementType(customerCode, template.Type)
		log.Printf("Sending %s announcement email to topic %s for customer %s", template.Type, topicName, customerCode)
	}

	// Create customer-specific SES client with role chaining
	customerSESClient, err := p.getCustomerSESClient(ctx, customerCode)
	if err != nil {
		return fmt.Errorf("failed to create customer SES client: %w", err)
	}

	// Get account contact list using customer-specific client
	accountListName, err := ses.GetAccountContactList(customerSESClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts subscribed to this topic using customer-specific client
	subscribedContacts, err := p.getSubscribedContactsForTopic(customerSESClient, accountListName, topicName)
	if err != nil {
		// Check if error is due to topic not existing
		if strings.Contains(err.Error(), "doesn't contain Topic") || strings.Contains(err.Error(), "NotFoundException") {
			log.Printf("⚠️  Topic '%s' does not exist in contact list - skipping email send", topicName)
			return nil // Don't treat missing topic as an error
		}
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s' - skipping email send", topicName)
		return nil // Don't treat no subscribers as an error
	}

	log.Printf("📧 Sending announcement to topic '%s' (%d subscribers)", topicName, len(subscribedContacts))

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(template.Subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(template.HTMLBody),
					},
					Text: &sesv2Types.Content{
						Data: aws.String(template.TextBody),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	// Send to each subscribed contact
	successCount := 0
	errorCount := 0

	for _, contact := range subscribedContacts {
		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := customerSESClient.SendEmail(ctx, sendInput)
		if err != nil {
			log.Printf("❌ Failed to send email to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			successCount++
		}
	}

	log.Printf("✅ Successfully sent %s announcement email for customer %s (%d sent, %d errors)",
		template.Type, customerInfo.CustomerName, successCount, errorCount)

	if errorCount > 0 && successCount == 0 {
		return fmt.Errorf("failed to send email to all %d subscribers", errorCount)
	}

	return nil
}

// getTopicNameForAnnouncementType returns the appropriate SES topic name for an announcement type
func (p *AnnouncementProcessor) getTopicNameForAnnouncementType(customerCode, announcementType string) string {
	// Map announcement types to SES topics
	// Format: {type}-announce
	topicMap := map[string]string{
		"cic":         "cic-announce",
		"finops":      "finops-announce",
		"innersource": "inner-announce",
		"general":     "general-announce",
	}

	topicName := topicMap[strings.ToLower(announcementType)]
	if topicName == "" {
		topicName = "general-announce"
	}

	return topicName
}

// convertToAnnouncementData converts AnnouncementMetadata to AnnouncementData for email templates
func (p *AnnouncementProcessor) convertToAnnouncementData(announcement *types.AnnouncementMetadata) ses.AnnouncementData {
	data := ses.AnnouncementData{
		AnnouncementID: announcement.AnnouncementID,
		Title:          announcement.Title,
		Summary:        announcement.Summary,
		Content:        announcement.Content,
		Author:         announcement.Author,
		PostedDate:     announcement.PostedDate,
		Customers:      announcement.Customers,
	}

	// Add meeting metadata if present
	if announcement.MeetingMetadata != nil {
		data.MeetingMetadata = announcement.MeetingMetadata
	}

	// Add attachments if present
	if len(announcement.Attachments) > 0 {
		data.Attachments = announcement.Attachments
	}

	return data
}

// scheduleMeeting schedules a Microsoft Teams meeting for the announcement
func (p *AnnouncementProcessor) scheduleMeeting(ctx context.Context, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	log.Printf("📅 Scheduling meeting for announcement %s", announcement.AnnouncementID)

	// Convert announcement to ChangeMetadata format for meeting scheduling
	// This reuses the existing Microsoft Graph API integration
	changeMetadata := p.convertAnnouncementToChangeForMeeting(announcement)

	// Determine organizer email - announcements use ccoe@hearst.com
	organizerEmail := "ccoe@hearst.com"

	// Get all attendees from customers using SES topic subscriptions
	allAttendees, err := p.getAnnouncementAttendees(ctx, announcement)
	if err != nil {
		return fmt.Errorf("failed to get announcement attendees: %w", err)
	}

	if len(allAttendees) == 0 {
		log.Printf("⚠️  No attendees found for announcement %s, skipping meeting creation", announcement.AnnouncementID)
		return nil
	}

	log.Printf("👥 Found %d attendees for announcement meeting", len(allAttendees))

	// Generate Microsoft Graph meeting payload
	payload, err := ses.GenerateGraphMeetingPayloadFromChangeMetadata(changeMetadata, organizerEmail, allAttendees)
	if err != nil {
		return fmt.Errorf("failed to generate meeting payload: %w", err)
	}

	// Create the meeting using Microsoft Graph API
	meetingID, meetingURL, err := p.createGraphMeetingForAnnouncement(ctx, payload, organizerEmail, announcement)
	if err != nil {
		return fmt.Errorf("failed to create meeting via Microsoft Graph API: %w", err)
	}

	log.Printf("✅ Successfully created meeting with ID: %s", meetingID)

	// Update announcement with meeting metadata
	announcement.MeetingMetadata = &types.MeetingMetadata{
		MeetingID: meetingID,
		JoinURL:   meetingURL,
	}

	// Add modification entry for meeting scheduled
	modificationEntry, err := types.NewMeetingScheduledEntry(types.BackendUserID, announcement.MeetingMetadata)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to create meeting scheduled modification entry: %v", err)
	} else {
		announcement.Modifications = append(announcement.Modifications, modificationEntry)
	}

	// Save updated announcement back to S3
	err = p.SaveAnnouncementToS3(ctx, announcement, s3Bucket, s3Key)
	if err != nil {
		return fmt.Errorf("failed to save announcement with meeting metadata: %w", err)
	}

	log.Printf("✅ Meeting metadata saved to announcement %s", announcement.AnnouncementID)
	return nil
}

// cancelMeeting cancels a scheduled Microsoft Teams meeting
func (p *AnnouncementProcessor) cancelMeeting(ctx context.Context, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	log.Printf("❌ Cancelling meeting for announcement %s", announcement.AnnouncementID)

	if announcement.MeetingMetadata == nil || announcement.MeetingMetadata.MeetingID == "" {
		log.Printf("⚠️  No scheduled meeting found for announcement %s, nothing to cancel", announcement.AnnouncementID)
		return nil
	}

	meetingID := announcement.MeetingMetadata.MeetingID
	log.Printf("📅 Cancelling meeting ID: %s", meetingID)

	// Determine organizer email from config
	organizerEmail := "ccoe@nonprod.ccoe.hearst.com" // Default organizer

	// Cancel the meeting using Microsoft Graph API
	err := p.cancelGraphMeeting(ctx, meetingID, organizerEmail)
	if err != nil {
		return fmt.Errorf("failed to cancel meeting via Microsoft Graph API: %w", err)
	}

	log.Printf("✅ Successfully cancelled meeting %s", meetingID)

	// Add modification entry for meeting cancelled
	modificationEntry, err := types.NewMeetingCancelledEntry(types.BackendUserID)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to create meeting cancelled modification entry: %v", err)
	} else {
		announcement.Modifications = append(announcement.Modifications, modificationEntry)
	}

	// Clear meeting metadata
	announcement.MeetingMetadata = nil

	// Save updated announcement back to S3
	err = p.SaveAnnouncementToS3(ctx, announcement, s3Bucket, s3Key)
	if err != nil {
		return fmt.Errorf("failed to save announcement after meeting cancellation: %w", err)
	}

	log.Printf("✅ Meeting cancellation saved to announcement %s", announcement.AnnouncementID)
	return nil
}

// convertAnnouncementToChangeForMeeting converts AnnouncementMetadata to ChangeMetadata for meeting scheduling
// This is a temporary conversion ONLY for meeting scheduling, and the announcement remains as AnnouncementMetadata
func (p *AnnouncementProcessor) convertAnnouncementToChangeForMeeting(announcement *types.AnnouncementMetadata) *types.ChangeMetadata {
	metadata := &types.ChangeMetadata{
		ObjectType:    announcement.ObjectType,
		ChangeID:      announcement.AnnouncementID,
		ChangeTitle:   announcement.Title,
		ChangeReason:  announcement.Summary,
		Customers:     announcement.Customers,
		Status:        announcement.Status,
		Modifications: announcement.Modifications,
	}

	// Set meeting-related fields if meeting is included
	if announcement.IncludeMeeting {
		metadata.MeetingRequired = "yes"

		// Set implementation times from announcement posted date or current time
		if !announcement.PostedDate.IsZero() {
			metadata.ImplementationStart = announcement.PostedDate
			metadata.ImplementationEnd = announcement.PostedDate.Add(1 * time.Hour) // Default 1 hour duration
		} else {
			metadata.ImplementationStart = time.Now()
			metadata.ImplementationEnd = time.Now().Add(1 * time.Hour)
		}

		metadata.Timezone = "America/New_York" // Default timezone
		metadata.MeetingTitle = announcement.Title
	}

	return metadata
}

// getCustomerSESClient creates a customer-specific SES client with role chaining
func (p *AnnouncementProcessor) getCustomerSESClient(ctx context.Context, customerCode string) (*sesv2.Client, error) {
	// Import the AWS internal package for credential manager
	credentialManager, err := awsinternal.NewCredentialManager(p.Config.AWSRegion, p.Config.CustomerMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential manager: %w", err)
	}

	// Get customer-specific AWS config (assumes customer's SES role)
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer config for %s: %w", customerCode, err)
	}

	// Create SES client with assumed role credentials
	return sesv2.NewFromConfig(customerConfig), nil
}

// getSubscribedContactsForTopic gets all contacts that should receive emails for a topic
func (p *AnnouncementProcessor) getSubscribedContactsForTopic(sesClient *sesv2.Client, listName string, topicName string) ([]sesv2Types.Contact, error) {
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

	return contactsResult.Contacts, nil
}

// getAnnouncementAttendees gets all attendees for an announcement from SES topic subscriptions
func (p *AnnouncementProcessor) getAnnouncementAttendees(ctx context.Context, announcement *types.AnnouncementMetadata) ([]string, error) {
	// Get the appropriate topic name based on announcement type
	customerCode := announcement.Customers[0]
	topicName := p.getTopicNameForAnnouncementType(customerCode, announcement.AnnouncementType)

	// Create customer-specific SES client with role chaining
	customerSESClient, err := p.getCustomerSESClient(ctx, customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer SES client: %w", err)
	}

	// Get account contact list using customer-specific client
	accountListName, err := ses.GetAccountContactList(customerSESClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts subscribed to this topic using customer-specific client
	subscribedContacts, err := p.getSubscribedContactsForTopic(customerSESClient, accountListName, topicName)
	if err != nil {
		// Check if error is due to topic not existing
		if strings.Contains(err.Error(), "doesn't contain Topic") || strings.Contains(err.Error(), "NotFoundException") {
			log.Printf("⚠️  Topic '%s' does not exist in contact list - no attendees for meeting", topicName)
			return []string{}, nil // Return empty list, not an error
		}
		return nil, fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	// Extract email addresses
	var attendees []string
	for _, contact := range subscribedContacts {
		if contact.EmailAddress != nil {
			attendees = append(attendees, *contact.EmailAddress)
		}
	}

	return attendees, nil
}

// createGraphMeetingForAnnouncement creates a meeting using Microsoft Graph API
func (p *AnnouncementProcessor) createGraphMeetingForAnnouncement(ctx context.Context, payload string, organizerEmail string, announcement *types.AnnouncementMetadata) (string, string, error) {
	// Get access token
	accessToken, err := ses.GetGraphAccessToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Check if meeting already exists (idempotency check)
	exists, existingMeeting, err := p.checkAnnouncementMeetingExists(ctx, accessToken, organizerEmail, announcement)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to check existing meetings: %v", err)
	} else if exists {
		log.Printf("✅ Meeting already exists for announcement, reusing existing meeting")
		meetingURL := ""
		if existingMeeting.OnlineMeeting != nil && existingMeeting.OnlineMeeting.JoinURL != "" {
			meetingURL = existingMeeting.OnlineMeeting.JoinURL
		}
		return existingMeeting.ID, meetingURL, nil
	}

	// Create new meeting
	log.Printf("📅 Creating new meeting for announcement: %s", announcement.Title)

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events", organizerEmail)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return "", "", fmt.Errorf("failed to create meeting request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to create meeting: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read meeting response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var graphError types.GraphError
		if json.Unmarshal(body, &graphError) == nil {
			return "", "", fmt.Errorf("meeting creation failed: %s - %s", graphError.Error.Code, graphError.Error.Message)
		}
		return "", "", fmt.Errorf("meeting creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var meetingResponse types.GraphMeetingResponse
	if err := json.Unmarshal(body, &meetingResponse); err != nil {
		return "", "", fmt.Errorf("failed to parse meeting response: %w", err)
	}

	// Extract meeting URL
	meetingURL := ""
	if meetingResponse.OnlineMeeting != nil && meetingResponse.OnlineMeeting.JoinURL != "" {
		meetingURL = meetingResponse.OnlineMeeting.JoinURL
	}

	return meetingResponse.ID, meetingURL, nil
}

// checkAnnouncementMeetingExists checks if a meeting for this announcement already exists
func (p *AnnouncementProcessor) checkAnnouncementMeetingExists(ctx context.Context, accessToken, organizerEmail string, announcement *types.AnnouncementMetadata) (bool, *types.GraphMeetingResponse, error) {
	// Search for meetings with the announcement title
	subject := fmt.Sprintf("%s Event: %s", strings.ToUpper(announcement.AnnouncementType), announcement.Title)

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$top=50&$select=id,subject,start,end,onlineMeeting&$orderby=start/dateTime desc",
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

	// Look for a meeting with the same subject
	for _, meeting := range searchResponse.Value {
		if meeting.Subject == subject {
			return true, &meeting, nil
		}
	}

	return false, nil, nil
}

// cancelGraphMeeting cancels a meeting using Microsoft Graph API
func (p *AnnouncementProcessor) cancelGraphMeeting(ctx context.Context, meetingID, organizerEmail string) error {
	// Get access token
	accessToken, err := ses.GetGraphAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Delete the meeting
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events/%s", organizerEmail, meetingID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete meeting: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("meeting deletion failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SaveAnnouncementToS3 saves the announcement metadata back to S3
func (p *AnnouncementProcessor) SaveAnnouncementToS3(ctx context.Context, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	log.Printf("💾 Saving announcement %s to S3: %s/%s", announcement.AnnouncementID, s3Bucket, s3Key)

	// Marshal announcement to JSON
	announcementJSON, err := json.MarshalIndent(announcement, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal announcement: %w", err)
	}

	// Upload to S3
	_, err = p.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(s3Key),
		Body:        strings.NewReader(string(announcementJSON)),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload announcement to S3: %w", err)
	}

	log.Printf("✅ Successfully saved announcement %s to S3", announcement.AnnouncementID)
	return nil
}
