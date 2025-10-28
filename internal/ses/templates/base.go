package templates

import (
	"fmt"
	"time"

	"ccoe-customer-contact-manager/internal/types"
)

// BaseTemplateData contains common fields for all email templates
type BaseTemplateData struct {
	EventID       string
	EventType     string // "announcement" or "change"
	Category      string // "cic", "finops", "innersource", "general", "change"
	Status        string // Workflow state: "pending_approval", "approved", "completed", "cancelled", etc.
	Title         string
	Summary       string
	Content       string
	SenderAddress string
	Timestamp     time.Time
	Attachments   []string // URLs to attachments
}

// ApprovalRecord tracks who approved and when
type ApprovalRecord struct {
	ApprovedBy    string
	ApprovedAt    time.Time
	ApproverEmail string
}

// ApprovalRequestData contains data for approval request notifications
type ApprovalRequestData struct {
	BaseTemplateData
	ApprovalURL string
	Customers   []string
}

// ApprovedNotificationData contains data for approved notifications
type ApprovedNotificationData struct {
	BaseTemplateData
	Approvals []ApprovalRecord // Multiple approvers with timestamps
}

// MeetingData contains data for meeting invitation notifications
type MeetingData struct {
	BaseTemplateData
	MeetingMetadata *types.MeetingMetadata
	OrganizerEmail  string
}

// CompletionData contains data for completion notifications
type CompletionData struct {
	BaseTemplateData
	CompletedBy      string
	CompletedByEmail string
	CompletedAt      time.Time
	SurveyURL        string // Typeform survey URL with hidden parameters
	SurveyQRCode     string // Base64-encoded QR code image for survey
}

// CancellationData contains data for cancellation notifications
type CancellationData struct {
	BaseTemplateData
	CancelledBy      string
	CancelledByEmail string
	CancelledAt      time.Time
}

// EmailTemplate represents a complete email with subject and body
type EmailTemplate struct {
	Subject  string
	HTMLBody string
	TextBody string
}

// TemplateBuilder defines the interface for building email templates
type TemplateBuilder interface {
	BuildApprovalRequest(data ApprovalRequestData) EmailTemplate
	BuildApprovedNotification(data ApprovedNotificationData) EmailTemplate
	BuildMeetingInvitation(data MeetingData) EmailTemplate
	BuildCompletion(data CompletionData) EmailTemplate
	BuildCancellation(data CancellationData) EmailTemplate
}

// TemplateRegistry manages template builders for different event types
type TemplateRegistry struct {
	announcementBuilder TemplateBuilder
	changeBuilder       TemplateBuilder
	config              types.EmailConfig
}

// NewTemplateRegistry creates a new template registry with the given configuration
func NewTemplateRegistry(config types.EmailConfig) *TemplateRegistry {
	return &TemplateRegistry{
		announcementBuilder: NewAnnouncementTemplateBuilder(config),
		changeBuilder:       NewChangeTemplateBuilder(config),
		config:              config,
	}
}

// GetTemplate routes to the appropriate builder based on event type and notification type
func (r *TemplateRegistry) GetTemplate(
	eventType string,
	notificationType NotificationType,
	data interface{},
) (EmailTemplate, error) {
	var builder TemplateBuilder

	// Select the appropriate builder based on event type
	switch eventType {
	case "announcement":
		builder = r.announcementBuilder
	case "change":
		builder = r.changeBuilder
	default:
		return EmailTemplate{}, fmt.Errorf("unknown event type: %s", eventType)
	}

	// Route to the appropriate builder method based on notification type
	switch notificationType {
	case NotificationApprovalRequest:
		approvalData, ok := data.(ApprovalRequestData)
		if !ok {
			return EmailTemplate{}, fmt.Errorf("invalid data type for approval request: expected ApprovalRequestData")
		}
		return builder.BuildApprovalRequest(approvalData), nil

	case NotificationApproved:
		approvedData, ok := data.(ApprovedNotificationData)
		if !ok {
			return EmailTemplate{}, fmt.Errorf("invalid data type for approved notification: expected ApprovedNotificationData")
		}
		return builder.BuildApprovedNotification(approvedData), nil

	case NotificationMeeting:
		meetingData, ok := data.(MeetingData)
		if !ok {
			return EmailTemplate{}, fmt.Errorf("invalid data type for meeting invitation: expected MeetingData")
		}
		return builder.BuildMeetingInvitation(meetingData), nil

	case NotificationCompleted:
		completionData, ok := data.(CompletionData)
		if !ok {
			return EmailTemplate{}, fmt.Errorf("invalid data type for completion notification: expected CompletionData")
		}
		return builder.BuildCompletion(completionData), nil

	case NotificationCancelled:
		cancellationData, ok := data.(CancellationData)
		if !ok {
			return EmailTemplate{}, fmt.Errorf("invalid data type for cancellation notification: expected CancellationData")
		}
		return builder.BuildCancellation(cancellationData), nil

	default:
		return EmailTemplate{}, fmt.Errorf("unknown notification type: %s", notificationType)
	}
}
