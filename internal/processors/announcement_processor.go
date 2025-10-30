// Package processors provides specialized processors for different object types
package processors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsinternal "ccoe-customer-contact-manager/internal/aws"
	"ccoe-customer-contact-manager/internal/ses"
	"ccoe-customer-contact-manager/internal/ses/templates"
	"ccoe-customer-contact-manager/internal/typeform"
	"ccoe-customer-contact-manager/internal/types"
)

// AnnouncementProcessor handles announcement-specific processing logic
type AnnouncementProcessor struct {
	S3Client   *s3.Client
	SESClient  *sesv2.Client
	GraphToken string
	Config     *types.Config
	logger     *slog.Logger
}

// NewAnnouncementProcessor creates a new announcement processor with required clients
func NewAnnouncementProcessor(s3Client *s3.Client, sesClient *sesv2.Client, graphToken string, cfg *types.Config, logger *slog.Logger) *AnnouncementProcessor {
	return &AnnouncementProcessor{
		S3Client:   s3Client,
		SESClient:  sesClient,
		GraphToken: graphToken,
		Config:     cfg,
		logger:     logger,
	}
}

// ProcessAnnouncement processes an announcement based on its status
func (p *AnnouncementProcessor) ProcessAnnouncement(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	switch announcement.Status {
	case "submitted":
		return p.handleSubmitted(ctx, customerCode, announcement)
	case "approved":
		return p.handleApproved(ctx, customerCode, announcement, s3Bucket, s3Key)
	case "cancelled":
		return p.handleCancelled(ctx, customerCode, announcement, s3Bucket, s3Key)
	case "completed":
		return p.handleCompleted(ctx, customerCode, announcement, s3Bucket, s3Key)
	default:
		return nil
	}
}

// handleSubmitted processes a submitted announcement (sends approval request)
func (p *AnnouncementProcessor) handleSubmitted(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	return p.sendApprovalRequest(ctx, customerCode, announcement)
}

// handleApproved processes an approved announcement (schedules meeting if needed, sends emails)
func (p *AnnouncementProcessor) handleApproved(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	// Schedule meeting if requested
	if announcement.IncludeMeeting {
		err := p.scheduleMeeting(ctx, announcement, s3Bucket, s3Key)
		if err != nil {
			p.logger.Error("failed to schedule meeting", "announcement_id", announcement.AnnouncementID, "error", err)
		}
	}

	// Send announcement emails
	err := p.sendAnnouncementEmails(ctx, customerCode, announcement)
	if err != nil {
		// Check if error is due to no subscribers (not a real error)
		if !strings.Contains(err.Error(), "no subscribers found") {
			p.logger.Error("failed to send announcement emails", "customer", customerCode, "announcement_id", announcement.AnnouncementID, "error", err)
			return fmt.Errorf("failed to send announcement emails: %w", err)
		}
	}

	return nil
}

// handleCancelled processes a cancelled announcement (cancels meeting, sends cancellation email)
func (p *AnnouncementProcessor) handleCancelled(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	// Cancel meeting if scheduled
	if announcement.MeetingMetadata != nil && announcement.MeetingMetadata.MeetingID != "" {
		err := p.cancelMeeting(ctx, announcement, s3Bucket, s3Key)
		if err != nil {
			p.logger.Error("failed to cancel meeting", "announcement_id", announcement.AnnouncementID, "meeting_id", announcement.MeetingMetadata.MeetingID, "error", err)
		}
	}

	// Check if announcement was previously approved by looking at modifications
	wasApproved := false
	for _, mod := range announcement.Modifications {
		if mod.ModificationType == types.ModificationTypeApproved {
			wasApproved = true
			break
		}
	}

	// Only send cancellation email if announcement was previously approved
	if wasApproved {
		err := p.sendCancellationEmail(ctx, customerCode, announcement)
		if err != nil {
			// Check if error is due to no subscribers (not a real error)
			if !strings.Contains(err.Error(), "no subscribers found") {
				p.logger.Error("failed to send cancellation email", "announcement_id", announcement.AnnouncementID, "error", err)
			}
		}
	}

	return nil
}

// handleCompleted processes a completed announcement (sends completion email)
func (p *AnnouncementProcessor) handleCompleted(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	// Create Typeform survey for completed announcement FIRST (before sending email)
	// This ensures the survey URL is available in S3 metadata when the email is generated
	err := p.createSurveyForAnnouncement(ctx, announcement, s3Bucket, s3Key)
	if err != nil {
		p.logger.Error("failed to create survey", "announcement_id", announcement.AnnouncementID, "error", err)
		// Don't fail the entire workflow if survey creation fails
	}

	// Reload announcement from S3 to get survey metadata
	announcement, err = p.reloadAnnouncementFromS3(ctx, s3Bucket, s3Key)
	if err != nil {
		p.logger.Warn("failed to reload announcement from S3", "announcement_id", announcement.AnnouncementID, "error", err)
		// Continue with existing announcement data
	}

	// Send completion email (which will include the survey link if survey was created successfully)
	err = p.sendCompletionEmail(ctx, customerCode, announcement)
	if err != nil {
		// Check if error is due to no subscribers (not a real error)
		if !strings.Contains(err.Error(), "no subscribers found") {
			p.logger.Error("failed to send completion email", "announcement_id", announcement.AnnouncementID, "error", err)
		}
	}

	return nil
}

// sendApprovalRequest sends approval request email for the announcement
func (p *AnnouncementProcessor) sendApprovalRequest(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Prepare data for template
	data := templates.ApprovalRequestData{
		BaseTemplateData: templates.BaseTemplateData{
			EventID:       announcement.AnnouncementID,
			EventType:     "announcement",
			Category:      announcement.AnnouncementType,
			Status:        announcement.Status,
			Title:         announcement.Title,
			Summary:       announcement.Summary,
			Content:       announcement.Content,
			SenderAddress: p.Config.EmailConfig.SenderAddress,
			Timestamp:     time.Now(),
			Attachments:   announcement.Attachments,
		},
		ApprovalURL: fmt.Sprintf("%s/approvals.html?customerCode=%s&objectId=%s", p.Config.EmailConfig.PortalBaseURL, customerCode, announcement.AnnouncementID),
		Customers:   announcement.Customers,
	}

	// Send via new template system
	return p.sendEmailWithNewTemplates(ctx, customerCode, "announcement", templates.NotificationApprovalRequest, data)
}

// sendAnnouncementEmails sends type-specific announcement emails
func (p *AnnouncementProcessor) sendAnnouncementEmails(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Extract approval information from modifications
	approvals := []templates.ApprovalRecord{}
	for _, mod := range announcement.Modifications {
		if mod.ModificationType == types.ModificationTypeApproved {
			approvals = append(approvals, templates.ApprovalRecord{
				ApprovedBy:    mod.UserID,
				ApprovedAt:    mod.Timestamp,
				ApproverEmail: "", // Email not stored in modifications
			})
		}
	}

	data := templates.ApprovedNotificationData{
		BaseTemplateData: templates.BaseTemplateData{
			EventID:       announcement.AnnouncementID,
			EventType:     "announcement",
			Category:      announcement.AnnouncementType,
			Status:        announcement.Status,
			Title:         announcement.Title,
			Summary:       announcement.Summary,
			Content:       announcement.Content,
			SenderAddress: p.Config.EmailConfig.SenderAddress,
			Timestamp:     time.Now(),
			Attachments:   announcement.Attachments,
		},
		Approvals: approvals,
	}

	// Send via new template system
	return p.sendEmailWithNewTemplates(ctx, customerCode, "announcement", templates.NotificationApproved, data)
}

// sendCancellationEmail sends cancellation notification email
func (p *AnnouncementProcessor) sendCancellationEmail(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Extract cancellation information from modifications (find the "cancelled" entry, not "meeting_cancelled")
	var cancelledBy string
	var cancelledAt time.Time
	if len(announcement.Modifications) > 0 {
		// Look for the "cancelled" modification entry (user-initiated action)
		for i := len(announcement.Modifications) - 1; i >= 0; i-- {
			if announcement.Modifications[i].ModificationType == "cancelled" {
				cancelledBy = announcement.Modifications[i].UserID
				cancelledAt = announcement.Modifications[i].Timestamp
				break
			}
		}
		// Fallback to last modification if no "cancelled" entry found
		if cancelledBy == "" {
			lastMod := announcement.Modifications[len(announcement.Modifications)-1]
			cancelledBy = lastMod.UserID
			cancelledAt = lastMod.Timestamp
		}
	}

	data := templates.CancellationData{
		BaseTemplateData: templates.BaseTemplateData{
			EventID:       announcement.AnnouncementID,
			EventType:     "announcement",
			Category:      announcement.AnnouncementType,
			Status:        announcement.Status,
			Title:         announcement.Title,
			Summary:       announcement.Summary,
			Content:       announcement.Content,
			SenderAddress: p.Config.EmailConfig.SenderAddress,
			Timestamp:     time.Now(),
			Attachments:   announcement.Attachments,
		},
		CancelledBy:      cancelledBy,
		CancelledByEmail: "", // Email not stored in modifications
		CancelledAt:      cancelledAt,
	}

	// Send via new template system
	return p.sendEmailWithNewTemplates(ctx, customerCode, "announcement", templates.NotificationCancelled, data)
}

// sendCompletionEmail sends completion notification email
func (p *AnnouncementProcessor) sendCompletionEmail(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata) error {
	// Extract completion information from modifications (find the "completed" entry)
	var completedBy string
	var completedAt time.Time
	if len(announcement.Modifications) > 0 {
		// Look for the "completed" modification entry (user-initiated action)
		for i := len(announcement.Modifications) - 1; i >= 0; i-- {
			if announcement.Modifications[i].ModificationType == "completed" {
				completedBy = announcement.Modifications[i].UserID
				completedAt = announcement.Modifications[i].Timestamp
				break
			}
		}
		// Fallback to last modification if no "completed" entry found
		if completedBy == "" {
			lastMod := announcement.Modifications[len(announcement.Modifications)-1]
			completedBy = lastMod.UserID
			completedAt = lastMod.Timestamp
		}
	}

	// Generate survey URL with hidden parameters
	surveyURL, qrCode := p.generateSurveyURLAndQRCode(announcement)

	data := templates.CompletionData{
		BaseTemplateData: templates.BaseTemplateData{
			EventID:       announcement.AnnouncementID,
			EventType:     "announcement",
			Category:      announcement.AnnouncementType,
			Status:        announcement.Status,
			Title:         announcement.Title,
			Summary:       announcement.Summary,
			Content:       announcement.Content,
			SenderAddress: p.Config.EmailConfig.SenderAddress,
			Timestamp:     time.Now(),
			Attachments:   announcement.Attachments,
		},
		CompletedBy:      completedBy,
		CompletedByEmail: "", // Email not stored in modifications
		CompletedAt:      completedAt,
		SurveyURL:        surveyURL,
		SurveyQRCode:     qrCode,
	}

	// Send via new template system
	return p.sendEmailWithNewTemplates(ctx, customerCode, "announcement", templates.NotificationCompleted, data)
}

// sendEmailWithNewTemplates sends email using the new template system
func (p *AnnouncementProcessor) sendEmailWithNewTemplates(ctx context.Context, customerCode string, eventType string, notificationType templates.NotificationType, data interface{}) error {
	// Validate customer code exists
	if _, exists := p.Config.CustomerMappings[customerCode]; !exists {
		return fmt.Errorf("customer code %s not found in configuration", customerCode)
	}

	// Initialize template registry with email config
	registry := templates.NewTemplateRegistry(p.Config.EmailConfig)

	// Get the template
	emailTemplate, err := registry.GetTemplate(eventType, notificationType, data)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	// Determine topic name based on notification type and event type
	var topicName string
	if notificationType == templates.NotificationApprovalRequest {
		// Approval requests go to unified announce-approval topic
		topicName = "announce-approval"
	} else {
		// Extract category from data
		var category string
		switch d := data.(type) {
		case templates.ApprovalRequestData:
			category = d.Category
		case templates.ApprovedNotificationData:
			category = d.Category
		case templates.CancellationData:
			category = d.Category
		case templates.CompletionData:
			category = d.Category
		default:
			category = "general"
		}

		// Get topic name for the category
		topicName = p.getTopicNameForAnnouncementType(customerCode, category)
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
			return nil // Don't treat missing topic as an error
		}
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		return nil // Don't treat no subscribers as an error
	}

	// Extract email addresses from contacts
	var allRecipients []string
	for _, contact := range subscribedContacts {
		if contact.EmailAddress != nil {
			allRecipients = append(allRecipients, *contact.EmailAddress)
		}
	}

	// Get customer config for filtering
	customerInfo, exists := p.Config.CustomerMappings[customerCode]
	if exists && len(customerInfo.RestrictedRecipients) > 0 {
		// Apply restricted_recipients filtering
		filteredRecipients, _ := customerInfo.FilterRecipients(allRecipients)
		allRecipients = filteredRecipients
	}

	// Check if any recipients remain after filtering
	if len(allRecipients) == 0 {
		return nil
	}

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(p.Config.EmailConfig.SenderAddress),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(emailTemplate.Subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(emailTemplate.HTMLBody),
					},
					Text: &sesv2Types.Content{
						Data: aws.String(emailTemplate.TextBody),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	// Send to each allowed recipient
	successCount := 0
	errorCount := 0

	for _, email := range allRecipients {
		sendInput.Destination.ToAddresses = []string{email}

		_, err := customerSESClient.SendEmail(ctx, sendInput)
		if err != nil {
			p.logger.Error("failed to send email to recipient", "recipient", email, "error", err)
			errorCount++
		} else {
			successCount++
		}
	}

	p.logger.Info("email sent", "customer", customerCode, "topic", topicName, "notification_type", notificationType, "sent", successCount, "errors", errorCount)

	if errorCount > 0 && successCount == 0 {
		return fmt.Errorf("failed to send email to all %d subscribers", errorCount)
	}

	return nil
}

// getTopicNameForAnnouncementType returns the appropriate SES topic name for an announcement type
func (p *AnnouncementProcessor) getTopicNameForAnnouncementType(customerCode, announcementType string) string {
	// Map announcement types to SES topics
	// Must match topic names defined in SESConfig.json
	topicMap := map[string]string{
		"cic":         "cic-announce",
		"finops":      "finops-announce",
		"innersource": "inner-announce",
		"general":     "general-updates", // Matches SESConfig.json topic name
	}

	topicName := topicMap[strings.ToLower(announcementType)]
	if topicName == "" {
		topicName = "general-updates" // Default to general-updates
	}

	return topicName
}

// scheduleMeeting schedules a Microsoft Teams meeting for the announcement
func (p *AnnouncementProcessor) scheduleMeeting(ctx context.Context, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
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
		return nil
	}

	// Generate Microsoft Graph meeting payload
	payload, err := ses.GenerateGraphMeetingPayloadFromChangeMetadata(changeMetadata, organizerEmail, allAttendees)
	if err != nil {
		return fmt.Errorf("failed to generate meeting payload: %w", err)
	}

	// Create the meeting using Microsoft Graph API
	// Note: createGraphMeetingForAnnouncement populates announcement.MeetingMetadata directly
	meetingID, _, err := p.createGraphMeetingForAnnouncement(ctx, payload, organizerEmail, announcement)
	if err != nil {
		return fmt.Errorf("failed to create meeting via Microsoft Graph API: %w", err)
	}

	p.logger.Info("meeting scheduled", "announcement_id", announcement.AnnouncementID, "meeting_id", meetingID, "attendees", len(allAttendees))

	// Meeting metadata is already populated by createGraphMeetingForAnnouncement
	// Add modification entry for meeting scheduled
	modificationEntry, err := types.NewMeetingScheduledEntry(types.BackendUserID, announcement.MeetingMetadata)
	if err != nil {
		p.logger.Warn("failed to create meeting scheduled modification entry", "error", err)
	} else {
		announcement.Modifications = append(announcement.Modifications, modificationEntry)
	}

	// Save updated announcement back to S3
	err = p.SaveAnnouncementToS3(ctx, announcement, s3Bucket, s3Key)
	if err != nil {
		return fmt.Errorf("failed to save announcement with meeting metadata: %w", err)
	}

	return nil
}

// cancelMeeting cancels a scheduled Microsoft Teams meeting
func (p *AnnouncementProcessor) cancelMeeting(ctx context.Context, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	if announcement.MeetingMetadata == nil || announcement.MeetingMetadata.MeetingID == "" {
		return nil
	}

	meetingID := announcement.MeetingMetadata.MeetingID

	// Determine organizer email from config
	organizerEmail := "ccoe@hearst.com" // Default organizer

	// Cancel the meeting using Microsoft Graph API
	err := p.cancelGraphMeeting(ctx, meetingID, organizerEmail)
	if err != nil {
		return fmt.Errorf("failed to cancel meeting via Microsoft Graph API: %w", err)
	}

	p.logger.Info("meeting cancelled", "announcement_id", announcement.AnnouncementID, "meeting_id", meetingID)

	// Add modification entry for meeting cancelled
	modificationEntry, err := types.NewMeetingCancelledEntry(types.BackendUserID)
	if err != nil {
		p.logger.Warn("failed to create meeting cancelled modification entry", "error", err)
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
		Metadata:      make(map[string]interface{}),
	}

	// Add announcement content to metadata for meeting body
	if announcement.Content != "" {
		metadata.Metadata["content"] = announcement.Content
	}

	// Set meeting-related fields if meeting is included
	if announcement.IncludeMeeting {
		metadata.IncludeMeeting = true

		// Parse meeting date and timezone from announcement
		if announcement.MeetingDate != "" {
			// Parse the ISO 8601 datetime string
			meetingTime, err := time.Parse(time.RFC3339, announcement.MeetingDate)
			if err != nil {
				meetingTime = time.Now()
			}

			// Load the timezone if specified
			timezone := announcement.MeetingTimezone
			if timezone == "" {
				timezone = "America/New_York" // Default timezone
			}

			loc, err := time.LoadLocation(timezone)
			if err != nil {
				loc = time.UTC
			}

			// Convert meeting time to the specified timezone
			meetingTimeInZone := meetingTime.In(loc)
			metadata.ImplementationStart = meetingTimeInZone

			// Calculate end time from duration
			duration := 60 // Default 60 minutes
			if announcement.MeetingDuration != "" {
				// Parse duration string (should be a number in minutes)
				var parsedDuration int
				_, err := fmt.Sscanf(announcement.MeetingDuration, "%d", &parsedDuration)
				if err == nil && parsedDuration > 0 {
					duration = parsedDuration
				}
			}

			metadata.ImplementationEnd = meetingTimeInZone.Add(time.Duration(duration) * time.Minute)
			metadata.Timezone = timezone
		} else {
			// Fallback: use posted date or current time
			if !announcement.PostedDate.IsZero() {
				metadata.ImplementationStart = announcement.PostedDate
				metadata.ImplementationEnd = announcement.PostedDate.Add(1 * time.Hour)
			} else {
				metadata.ImplementationStart = time.Now()
				metadata.ImplementationEnd = time.Now().Add(1 * time.Hour)
			}
			metadata.Timezone = "America/New_York" // Default timezone
		}

		// Set meeting title
		if announcement.MeetingTitle != "" {
			metadata.MeetingTitle = announcement.MeetingTitle
		} else {
			metadata.MeetingTitle = announcement.Title
		}
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
// Handles pagination to retrieve all subscribers (not just the first 100)
func (p *AnnouncementProcessor) getSubscribedContactsForTopic(sesClient *sesv2.Client, listName string, topicName string) ([]sesv2Types.Contact, error) {
	var allContacts []sesv2Types.Contact
	var nextToken *string

	// Paginate through all contacts
	for {
		contactsInput := &sesv2.ListContactsInput{
			ContactListName: aws.String(listName),
			Filter: &sesv2Types.ListContactsFilter{
				FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
				TopicFilter: &sesv2Types.TopicFilter{
					TopicName: aws.String(topicName),
				},
			},
			NextToken: nextToken,
		}

		contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
		if err != nil {
			return nil, fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
		}

		allContacts = append(allContacts, contactsResult.Contacts...)

		// Check if there are more pages
		if contactsResult.NextToken == nil || *contactsResult.NextToken == "" {
			break
		}
		nextToken = contactsResult.NextToken
	}

	return allContacts, nil
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
			return []string{}, nil // Return empty list, not an error
		}
		return nil, fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	// Extract email addresses from topic subscribers
	var attendees []string
	for _, contact := range subscribedContacts {
		if contact.EmailAddress != nil {
			attendees = append(attendees, *contact.EmailAddress)
		}
	}

	// Add manually specified attendees from the announcement (if any)
	if announcement.Attendees != "" {
		// Parse comma-separated email addresses
		manualAttendees := strings.Split(announcement.Attendees, ",")
		for _, email := range manualAttendees {
			trimmedEmail := strings.TrimSpace(email)
			if trimmedEmail != "" {
				// Check if not already in the list (avoid duplicates)
				found := false
				for _, existing := range attendees {
					if strings.EqualFold(existing, trimmedEmail) {
						found = true
						break
					}
				}
				if !found {
					attendees = append(attendees, trimmedEmail)
				}
			}
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
		p.logger.Warn("failed to check existing meetings", "error", err)
	} else if exists {
		meetingURL := ""
		if existingMeeting.OnlineMeeting != nil && existingMeeting.OnlineMeeting.JoinURL != "" {
			meetingURL = existingMeeting.OnlineMeeting.JoinURL
		}
		return existingMeeting.ID, meetingURL, nil
	}

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

	// Store the full meeting response in announcement for later use
	announcement.MeetingMetadata = &types.MeetingMetadata{
		MeetingID: meetingResponse.ID,
		JoinURL:   meetingURL,
		Subject:   meetingResponse.Subject,
		Organizer: organizerEmail,
	}

	// Extract start and end times if available
	// Graph API returns datetime without timezone, so we need to parse and convert to RFC3339
	if meetingResponse.Start != nil && meetingResponse.Start.DateTime != "" {
		startTime, err := parseGraphDateTime(meetingResponse.Start.DateTime, meetingResponse.Start.TimeZone)
		if err == nil {
			announcement.MeetingMetadata.StartTime = startTime
		}
	}
	if meetingResponse.End != nil && meetingResponse.End.DateTime != "" {
		endTime, err := parseGraphDateTime(meetingResponse.End.DateTime, meetingResponse.End.TimeZone)
		if err == nil {
			announcement.MeetingMetadata.EndTime = endTime
		}
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

// parseGraphDateTime converts Microsoft Graph API datetime format to RFC3339
// Graph API returns datetime without timezone suffix, so we need to add it
func parseGraphDateTime(dateTimeStr, timeZone string) (string, error) {
	// Graph API datetime format: "2025-10-17T15:16:26.1718365"
	// We need to convert to RFC3339: "2025-10-17T15:16:26.171Z"

	// Parse the datetime string
	// Try multiple formats since Graph API can return with or without fractional seconds
	var parsedTime time.Time
	var err error

	formats := []string{
		"2006-01-02T15:04:05.9999999",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05.99999",
		"2006-01-02T15:04:05.9999",
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05.99",
		"2006-01-02T15:04:05.9",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		parsedTime, err = time.Parse(format, dateTimeStr)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to parse datetime %s: %w", dateTimeStr, err)
	}

	// Load the timezone location
	// Default to UTC if timezone is not specified or invalid
	loc := time.UTC
	if timeZone != "" && timeZone != "UTC" {
		loadedLoc, err := time.LoadLocation(timeZone)
		if err == nil {
			loc = loadedLoc
		}
	}

	// Convert to the specified timezone and format as RFC3339
	timeInZone := time.Date(
		parsedTime.Year(), parsedTime.Month(), parsedTime.Day(),
		parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(),
		parsedTime.Nanosecond(), loc,
	)

	return timeInZone.Format(time.RFC3339), nil
}

// SaveAnnouncementToS3 saves the announcement metadata back to S3
// SaveAnnouncementToS3 saves announcement to archive/ path (permanent storage)
// The archive/ path is the source of truth for all announcements
// The customers/ path is only for transient trigger files
func (p *AnnouncementProcessor) SaveAnnouncementToS3(ctx context.Context, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	// Marshal announcement to JSON
	announcementJSON, err := json.MarshalIndent(announcement, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal announcement: %w", err)
	}

	// Save to archive/ path (permanent storage)
	archiveKey := fmt.Sprintf("archive/%s.json", announcement.AnnouncementID)

	_, err = p.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(archiveKey),
		Body:        strings.NewReader(string(announcementJSON)),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload announcement to S3 %s: %w", archiveKey, err)
	}

	return nil
}

// createSurveyForAnnouncement creates a Typeform survey for the announcement
func (p *AnnouncementProcessor) createSurveyForAnnouncement(ctx context.Context, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string) error {
	// Create typeform client
	typeformClient, err := typeform.NewClient(slog.Default())
	if err != nil {
		return fmt.Errorf("failed to create typeform client: %w", err)
	}

	// Determine survey type based on announcement type
	surveyType := p.determineSurveyType(announcement.ObjectType, announcement.AnnouncementType)

	// Extract metadata for survey creation
	year, quarter := p.extractYearQuarter(announcement.PostedDate)
	eventType := "announcement"
	eventSubtype := announcement.AnnouncementType

	// Get customer code from the first customer in the list
	customerCode := ""
	if len(announcement.Customers) > 0 {
		customerCode = announcement.Customers[0]
	}

	surveyMetadata := &typeform.SurveyMetadata{
		CustomerCode: customerCode,
		ObjectID:     announcement.AnnouncementID,
		ObjectTitle:  announcement.Title,
		Year:         year,
		Quarter:      quarter,
		EventType:    eventType,
		EventSubtype: eventSubtype,
	}

	// Create the survey
	response, err := typeformClient.CreateSurvey(ctx, p.S3Client, s3Bucket, surveyMetadata, surveyType)
	if err != nil {
		return fmt.Errorf("failed to create survey: %w", err)
	}

	p.logger.Info("survey created", "announcement_id", announcement.AnnouncementID, "survey_id", response.ID)
	return nil
}

// determineSurveyType determines the survey type based on announcement type
func (p *AnnouncementProcessor) determineSurveyType(objectType, announcementType string) typeform.SurveyType {
	// Check object type first
	switch objectType {
	case "announcement_cic":
		return typeform.SurveyTypeCIC
	case "announcement_innersource":
		return typeform.SurveyTypeInnerSource
	case "announcement_finops":
		return typeform.SurveyTypeFinOps
	case "announcement_general":
		return typeform.SurveyTypeGeneral
	}

	// Fallback to announcement type
	switch announcementType {
	case "cic":
		return typeform.SurveyTypeCIC
	case "innersource":
		return typeform.SurveyTypeInnerSource
	case "finops":
		return typeform.SurveyTypeFinOps
	default:
		return typeform.SurveyTypeGeneral
	}
}

// extractYearQuarter extracts year and quarter from a time.Time
func (p *AnnouncementProcessor) extractYearQuarter(t time.Time) (string, string) {
	year := t.Format("2006")
	quarter := fmt.Sprintf("Q%d", (int(t.Month())-1)/3+1)
	return year, quarter
}

// reloadAnnouncementFromS3 reloads the announcement from S3 to get updated metadata
func (p *AnnouncementProcessor) reloadAnnouncementFromS3(ctx context.Context, s3Bucket, s3Key string) (*types.AnnouncementMetadata, error) {
	result, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 object: %w", err)
	}
	defer result.Body.Close()

	// Read and parse the JSON content
	var announcement types.AnnouncementMetadata
	decoder := json.NewDecoder(result.Body)
	if err := decoder.Decode(&announcement); err != nil {
		return nil, fmt.Errorf("failed to decode announcement metadata: %w", err)
	}

	// Extract survey metadata from S3 object metadata if present
	if result.Metadata != nil {
		if surveyID, ok := result.Metadata["survey_id"]; ok {
			announcement.SurveyID = surveyID
		}
		if surveyURL, ok := result.Metadata["survey_url"]; ok {
			announcement.SurveyURL = surveyURL
		}
		if surveyCreatedAt, ok := result.Metadata["survey_created_at"]; ok {
			announcement.SurveyCreatedAt = surveyCreatedAt
		}
	}

	return &announcement, nil
}

// generateSurveyURLAndQRCode generates a Typeform survey URL with hidden parameters and QR code
func (p *AnnouncementProcessor) generateSurveyURLAndQRCode(announcement *types.AnnouncementMetadata) (string, string) {
	// Check if survey URL exists in metadata
	if announcement.SurveyURL == "" {
		return "", ""
	}

	// Get customer code (use first customer if multiple)
	customerCode := ""
	if len(announcement.Customers) > 0 {
		customerCode = announcement.Customers[0]
	}

	// Calculate year and quarter from current time
	now := time.Now()
	year := fmt.Sprintf("%d", now.Year())
	quarter := fmt.Sprintf("Q%d", (now.Month()-1)/3+1)

	// Determine event type and subtype
	eventType := "announcement"
	eventSubtype := announcement.AnnouncementType // e.g., "cic", "finops", "innersource", "general"

	// Build Typeform URL directly with all hidden field parameters
	// The base URL is already in announcement.SurveyURL (e.g., https://form.typeform.com/to/{surveyId})
	// Hidden fields: user_login, customer_code, year, quarter, event_type, event_subtype, object_id
	surveyURL := fmt.Sprintf("%s?customer_code=%s&object_id=%s&year=%s&quarter=%s&event_type=%s&event_subtype=%s",
		announcement.SurveyURL,
		url.QueryEscape(customerCode),
		url.QueryEscape(announcement.AnnouncementID),
		url.QueryEscape(year),
		url.QueryEscape(quarter),
		url.QueryEscape(eventType),
		url.QueryEscape(eventSubtype),
	)

	// TODO: Generate QR code from survey URL
	// For now, return empty string for QR code
	qrCode := ""

	return surveyURL, qrCode
}
