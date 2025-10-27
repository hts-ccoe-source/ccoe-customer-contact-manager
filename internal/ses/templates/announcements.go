package templates

import (
	"ccoe-customer-contact-manager/internal/types"
	"fmt"
	"strings"
)

// categoryColors maps announcement categories to their header background colors
var categoryColors = map[CategoryType]string{
	CategoryCIC:         "#0066cc", // Blue
	CategoryFinOps:      "#28a745", // Green
	CategoryInnerSource: "#6f42c1", // Purple
	CategoryGeneral:     "#007bff", // Light blue
}

// AnnouncementTemplateBuilder builds email templates for announcements
type AnnouncementTemplateBuilder struct {
	config types.EmailConfig
}

// NewAnnouncementTemplateBuilder creates a new announcement template builder
func NewAnnouncementTemplateBuilder(config types.EmailConfig) *AnnouncementTemplateBuilder {
	return &AnnouncementTemplateBuilder{
		config: config,
	}
}

// BuildApprovalRequest builds an approval request email for announcements
func (b *AnnouncementTemplateBuilder) BuildApprovalRequest(data ApprovalRequestData) EmailTemplate {
	category := CategoryType(data.Category)
	emoji := GetEmojiForNotification(NotificationApprovalRequest, category)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildApprovalRequestHTML(data, emoji, category)
	textBody := b.buildApprovalRequestText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildApprovalRequestHTML builds the HTML body for approval request
func (b *AnnouncementTemplateBuilder) buildApprovalRequestHTML(data ApprovalRequestData, emoji string, category CategoryType) string {
	backgroundColor := categoryColors[category]
	if backgroundColor == "" {
		backgroundColor = "#007bff" // Default blue
	}

	var sb strings.Builder

	// HTML structure
	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            line-height: 1.6; 
            color: #333;
            margin: 0;
            padding: 0;
        }
        .email-container {
            max-width: 600px;
            margin: 0 auto;
        }
        .content {
            padding: 20px;
            background-color: #ffffff;
        }
        @media only screen and (max-width: 600px) {
            .email-container {
                width: 100% !important;
            }
        }
    </style>
</head>
<body>
`)

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	statusWord := getStatusWordForNotification(NotificationApprovalRequest)
	sb.WriteString(renderHTMLHeader(statusWord, data.Title, backgroundColor))
	sb.WriteString("\n")

	// Content section
	sb.WriteString(`        <div class="content">` + "\n")

	// Status subtitle
	sb.WriteString("            ")
	sb.WriteString(renderStatusSubtitle(data.Status))
	sb.WriteString("\n")

	// Summary
	if data.Summary != "" {
		sb.WriteString(fmt.Sprintf(`            <p style="font-weight: bold; margin-bottom: 15px;">%s</p>`, formatContentForHTML(data.Summary)))
		sb.WriteString("\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(fmt.Sprintf(`            <div style="margin-bottom: 20px;">%s</div>`, formatContentForHTML(data.Content)))
		sb.WriteString("\n")
	}

	// Approval URL
	if data.ApprovalURL != "" {
		sb.WriteString(fmt.Sprintf(`            <div style="margin: 20px 0;">
                <a href="%s" style="display: inline-block; padding: 12px 24px; background-color: %s; color: white; text-decoration: none; border-radius: 4px; font-weight: bold;">Review and Approve</a>
            </div>`, data.ApprovalURL, backgroundColor))
		sb.WriteString("\n")
	}

	// Customers
	if len(data.Customers) > 0 {
		sb.WriteString(`            <div style="margin-top: 20px;">
                <h3 style="font-size: 1em; color: #495057; margin-bottom: 10px;">Affected Customers</h3>
                <ul style="margin: 0; padding-left: 20px;">`)
		sb.WriteString("\n")
		for _, customer := range data.Customers {
			sb.WriteString(fmt.Sprintf(`                    <li>%s</li>`, formatContentForHTML(customer)))
			sb.WriteString("\n")
		}
		sb.WriteString(`                </ul>
            </div>`)
		sb.WriteString("\n")
	}

	// Attachments
	if len(data.Attachments) > 0 {
		sb.WriteString("            ")
		sb.WriteString(renderAttachments(data.Attachments))
		sb.WriteString("\n")
	}

	sb.WriteString(`        </div>` + "\n")

	// Footer
	sb.WriteString("        ")
	sb.WriteString(renderHTMLFooter(data.EventID, data.EventType, b.config.PortalBaseURL))
	sb.WriteString("\n")

	// SES Macro
	sb.WriteString("        ")
	sb.WriteString(renderSESMacro(data.Timestamp))
	sb.WriteString("\n")

	sb.WriteString(`    </div>` + "\n")

	// Hidden metadata (at end for email client compatibility)
	sb.WriteString("    ")
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationApprovalRequest)))
	sb.WriteString("\n")

	sb.WriteString(`</body>
</html>`)

	return sb.String()
}

// buildApprovalRequestText builds the plain text body for approval request
func (b *AnnouncementTemplateBuilder) buildApprovalRequestText(data ApprovalRequestData, emoji string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(renderTextHeader(emoji, data.Title))

	// Status
	sb.WriteString(renderTextStatusLine(data.Status))

	// Summary
	if data.Summary != "" {
		sb.WriteString(data.Summary)
		sb.WriteString("\n\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(data.Content)
		sb.WriteString("\n\n")
	}

	// Approval URL
	if data.ApprovalURL != "" {
		sb.WriteString("Review and Approve: ")
		sb.WriteString(data.ApprovalURL)
		sb.WriteString("\n\n")
	}

	// Customers
	if len(data.Customers) > 0 {
		sb.WriteString("Affected Customers:\n")
		for _, customer := range data.Customers {
			sb.WriteString(fmt.Sprintf("  - %s\n", customer))
		}
		sb.WriteString("\n")
	}

	// Attachments
	sb.WriteString(renderTextAttachments(data.Attachments))

	// Footer
	sb.WriteString(renderTextFooter(data.EventID, data.EventType, b.config.PortalBaseURL, data.Timestamp))

	return sb.String()
}

// BuildApprovedNotification builds an approved notification email for announcements
func (b *AnnouncementTemplateBuilder) BuildApprovedNotification(data ApprovedNotificationData) EmailTemplate {
	category := CategoryType(data.Category)
	emoji := GetEmojiForNotification(NotificationApproved, category)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildApprovedNotificationHTML(data, emoji, category)
	textBody := b.buildApprovedNotificationText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildApprovedNotificationHTML builds the HTML body for approved notification
func (b *AnnouncementTemplateBuilder) buildApprovedNotificationHTML(data ApprovedNotificationData, emoji string, category CategoryType) string {
	backgroundColor := categoryColors[category]
	if backgroundColor == "" {
		backgroundColor = "#007bff"
	}

	var sb strings.Builder

	// HTML structure
	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            line-height: 1.6; 
            color: #333;
            margin: 0;
            padding: 0;
        }
        .email-container {
            max-width: 600px;
            margin: 0 auto;
        }
        .content {
            padding: 20px;
            background-color: #ffffff;
        }
        @media only screen and (max-width: 600px) {
            .email-container {
                width: 100% !important;
            }
        }
    </style>
</head>
<body>
`)

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	statusWord := getStatusWordForNotification(NotificationApproved)
	sb.WriteString(renderHTMLHeader(statusWord, data.Title, backgroundColor))
	sb.WriteString("\n")

	// Content section
	sb.WriteString(`        <div class="content">` + "\n")

	// Status subtitle
	sb.WriteString("            ")
	sb.WriteString(renderStatusSubtitle(data.Status))
	sb.WriteString("\n")

	// Summary
	if data.Summary != "" {
		sb.WriteString(fmt.Sprintf(`            <p style="font-weight: bold; margin-bottom: 15px;">%s</p>`, formatContentForHTML(data.Summary)))
		sb.WriteString("\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(fmt.Sprintf(`            <div style="margin-bottom: 20px;">%s</div>`, formatContentForHTML(data.Content)))
		sb.WriteString("\n")
	}

	// Approvals section
	if len(data.Approvals) > 0 {
		sb.WriteString(`            <div style="margin-top: 20px; padding: 15px; background-color: #f8f9fa; border-left: 4px solid `)
		sb.WriteString(backgroundColor)
		sb.WriteString(`;">
                <h3 style="font-size: 1em; color: #495057; margin: 0 0 10px 0;">Approved By</h3>`)
		sb.WriteString("\n")
		for _, approval := range data.Approvals {
			formattedTime := approval.ApprovedAt.Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf(`                <div style="margin-bottom: 8px;">
                    <strong>%s</strong>`, formatContentForHTML(approval.ApprovedBy)))
			if approval.ApproverEmail != "" {
				sb.WriteString(fmt.Sprintf(` (%s)`, formatContentForHTML(approval.ApproverEmail)))
			}
			sb.WriteString(fmt.Sprintf(`<br>
                    <span style="color: #6c757d; font-size: 0.9em;">%s</span>
                </div>`, formattedTime))
			sb.WriteString("\n")
		}
		sb.WriteString(`            </div>`)
		sb.WriteString("\n")
	}

	// Attachments
	if len(data.Attachments) > 0 {
		sb.WriteString("            ")
		sb.WriteString(renderAttachments(data.Attachments))
		sb.WriteString("\n")
	}

	sb.WriteString(`        </div>` + "\n")

	// Footer
	sb.WriteString("        ")
	sb.WriteString(renderHTMLFooter(data.EventID, data.EventType, b.config.PortalBaseURL))
	sb.WriteString("\n")

	// SES Macro
	sb.WriteString("        ")
	sb.WriteString(renderSESMacro(data.Timestamp))
	sb.WriteString("\n")

	sb.WriteString(`    </div>` + "\n")

	// Hidden metadata (at end for email client compatibility)
	sb.WriteString("    ")
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationApproved)))
	sb.WriteString("\n")

	sb.WriteString(`</body>
</html>`)

	return sb.String()
}

// buildApprovedNotificationText builds the plain text body for approved notification
func (b *AnnouncementTemplateBuilder) buildApprovedNotificationText(data ApprovedNotificationData, emoji string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(renderTextHeader(emoji, data.Title))

	// Status
	sb.WriteString(renderTextStatusLine(data.Status))

	// Summary
	if data.Summary != "" {
		sb.WriteString(data.Summary)
		sb.WriteString("\n\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(data.Content)
		sb.WriteString("\n\n")
	}

	// Approvals
	if len(data.Approvals) > 0 {
		sb.WriteString("Approved By:\n")
		for _, approval := range data.Approvals {
			formattedTime := approval.ApprovedAt.Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf("  - %s", approval.ApprovedBy))
			if approval.ApproverEmail != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", approval.ApproverEmail))
			}
			sb.WriteString(fmt.Sprintf(" at %s\n", formattedTime))
		}
		sb.WriteString("\n")
	}

	// Attachments
	sb.WriteString(renderTextAttachments(data.Attachments))

	// Footer
	sb.WriteString(renderTextFooter(data.EventID, data.EventType, b.config.PortalBaseURL, data.Timestamp))

	return sb.String()
}

// BuildMeetingInvitation builds a meeting invitation email for announcements
func (b *AnnouncementTemplateBuilder) BuildMeetingInvitation(data MeetingData) EmailTemplate {
	category := CategoryType(data.Category)
	emoji := GetEmojiForNotification(NotificationMeeting, category)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildMeetingInvitationHTML(data, emoji, category)
	textBody := b.buildMeetingInvitationText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildMeetingInvitationHTML builds the HTML body for meeting invitation
func (b *AnnouncementTemplateBuilder) buildMeetingInvitationHTML(data MeetingData, emoji string, category CategoryType) string {
	backgroundColor := categoryColors[category]
	if backgroundColor == "" {
		backgroundColor = "#007bff"
	}

	var sb strings.Builder

	// HTML structure
	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            line-height: 1.6; 
            color: #333;
            margin: 0;
            padding: 0;
        }
        .email-container {
            max-width: 600px;
            margin: 0 auto;
        }
        .content {
            padding: 20px;
            background-color: #ffffff;
        }
        @media only screen and (max-width: 600px) {
            .email-container {
                width: 100% !important;
            }
        }
    </style>
</head>
<body>
`)

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	statusWord := getStatusWordForNotification(NotificationMeeting)
	sb.WriteString(renderHTMLHeader(statusWord, data.Title, backgroundColor))
	sb.WriteString("\n")

	// Content section
	sb.WriteString(`        <div class="content">` + "\n")

	// Status subtitle
	sb.WriteString("            ")
	sb.WriteString(renderStatusSubtitle(data.Status))
	sb.WriteString("\n")

	// Summary
	if data.Summary != "" {
		sb.WriteString(fmt.Sprintf(`            <p style="font-weight: bold; margin-bottom: 15px;">%s</p>`, formatContentForHTML(data.Summary)))
		sb.WriteString("\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(fmt.Sprintf(`            <div style="margin-bottom: 20px;">%s</div>`, formatContentForHTML(data.Content)))
		sb.WriteString("\n")
	}

	// Meeting details
	if data.MeetingMetadata != nil {
		sb.WriteString(`            <div style="margin-top: 20px; padding: 15px; background-color: #f8f9fa; border-left: 4px solid `)
		sb.WriteString(backgroundColor)
		sb.WriteString(`;">
                <h3 style="font-size: 1em; color: #495057; margin: 0 0 10px 0;">📅 Meeting Details</h3>`)
		sb.WriteString("\n")

		if data.MeetingMetadata.StartTime != "" {
			sb.WriteString(fmt.Sprintf(`                <div style="margin-bottom: 8px;">
                    <strong>Start:</strong> %s
                </div>`, formatContentForHTML(data.MeetingMetadata.StartTime)))
			sb.WriteString("\n")
		}

		if data.MeetingMetadata.EndTime != "" {
			sb.WriteString(fmt.Sprintf(`                <div style="margin-bottom: 8px;">
                    <strong>End:</strong> %s
                </div>`, formatContentForHTML(data.MeetingMetadata.EndTime)))
			sb.WriteString("\n")
		}

		if data.MeetingMetadata.JoinURL != "" {
			sb.WriteString(fmt.Sprintf(`                <div style="margin-top: 15px;">
                    <a href="%s" style="display: inline-block; padding: 10px 20px; background-color: %s; color: white; text-decoration: none; border-radius: 4px; font-weight: bold;">Join Meeting</a>
                </div>`, data.MeetingMetadata.JoinURL, backgroundColor))
			sb.WriteString("\n")
		}

		sb.WriteString(`            </div>`)
		sb.WriteString("\n")
	}

	// Attachments
	if len(data.Attachments) > 0 {
		sb.WriteString("            ")
		sb.WriteString(renderAttachments(data.Attachments))
		sb.WriteString("\n")
	}

	sb.WriteString(`        </div>` + "\n")

	// Footer
	sb.WriteString("        ")
	sb.WriteString(renderHTMLFooter(data.EventID, data.EventType, b.config.PortalBaseURL))
	sb.WriteString("\n")

	// SES Macro
	sb.WriteString("        ")
	sb.WriteString(renderSESMacro(data.Timestamp))
	sb.WriteString("\n")

	sb.WriteString(`    </div>` + "\n")

	// Hidden metadata (at end for email client compatibility)
	sb.WriteString("    ")
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationMeeting)))
	sb.WriteString("\n")

	sb.WriteString(`</body>
</html>`)

	return sb.String()
}

// buildMeetingInvitationText builds the plain text body for meeting invitation
func (b *AnnouncementTemplateBuilder) buildMeetingInvitationText(data MeetingData, emoji string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(renderTextHeader(emoji, data.Title))

	// Status
	sb.WriteString(renderTextStatusLine(data.Status))

	// Summary
	if data.Summary != "" {
		sb.WriteString(data.Summary)
		sb.WriteString("\n\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(data.Content)
		sb.WriteString("\n\n")
	}

	// Meeting details
	if data.MeetingMetadata != nil {
		sb.WriteString("📅 Meeting Details:\n")
		if data.MeetingMetadata.StartTime != "" {
			sb.WriteString(fmt.Sprintf("  Start: %s\n", data.MeetingMetadata.StartTime))
		}
		if data.MeetingMetadata.EndTime != "" {
			sb.WriteString(fmt.Sprintf("  End: %s\n", data.MeetingMetadata.EndTime))
		}
		if data.MeetingMetadata.JoinURL != "" {
			sb.WriteString(fmt.Sprintf("  Join Meeting: %s\n", data.MeetingMetadata.JoinURL))
		}
		sb.WriteString("\n")
	}

	// Attachments
	sb.WriteString(renderTextAttachments(data.Attachments))

	// Footer
	sb.WriteString(renderTextFooter(data.EventID, data.EventType, b.config.PortalBaseURL, data.Timestamp))

	return sb.String()
}

// BuildCompletion builds a completion notification email for announcements
func (b *AnnouncementTemplateBuilder) BuildCompletion(data CompletionData) EmailTemplate {
	category := CategoryType(data.Category)
	emoji := GetEmojiForNotification(NotificationCompleted, category)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildCompletionHTML(data, emoji, category)
	textBody := b.buildCompletionText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildCompletionHTML builds the HTML body for completion notification
func (b *AnnouncementTemplateBuilder) buildCompletionHTML(data CompletionData, emoji string, category CategoryType) string {
	backgroundColor := categoryColors[category]
	if backgroundColor == "" {
		backgroundColor = "#007bff"
	}

	var sb strings.Builder

	// HTML structure
	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            line-height: 1.6; 
            color: #333;
            margin: 0;
            padding: 0;
        }
        .email-container {
            max-width: 600px;
            margin: 0 auto;
        }
        .content {
            padding: 20px;
            background-color: #ffffff;
        }
        @media only screen and (max-width: 600px) {
            .email-container {
                width: 100% !important;
            }
        }
    </style>
</head>
<body>
`)

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	statusWord := getStatusWordForNotification(NotificationCompleted)
	sb.WriteString(renderHTMLHeader(statusWord, data.Title, backgroundColor))
	sb.WriteString("\n")

	// Content section
	sb.WriteString(`        <div class="content">` + "\n")

	// Status subtitle
	sb.WriteString("            ")
	sb.WriteString(renderStatusSubtitle(data.Status))
	sb.WriteString("\n")

	// Survey section - MOVED TO TOP for visibility
	if data.SurveyURL != "" {
		sb.WriteString(`            <div style="margin-top: 20px; padding: 15px; background-color: #e7f3ff; border-left: 4px solid #0066cc;">
                <h3 style="font-size: 1em; color: #004085; margin: 0 0 10px 0;">📋 Share Your Feedback</h3>
                <p style="margin: 0 0 15px 0;">Help us improve by taking a quick survey about this announcement.</p>`)
		sb.WriteString("\n")

		// Survey button
		sb.WriteString(fmt.Sprintf(`                <div style="margin-bottom: 15px;">
                    <a href="%s" style="display: inline-block; padding: 12px 24px; background-color: #0066cc; color: white; text-decoration: none; border-radius: 4px; font-weight: bold;">Take Survey</a>
                </div>`, data.SurveyURL))
		sb.WriteString("\n")

		// QR code if available
		if data.SurveyQRCode != "" {
			sb.WriteString(fmt.Sprintf(`                <div style="margin-top: 15px;">
                    <p style="margin: 0 0 10px 0; font-size: 0.9em; color: #666;">Or scan this QR code:</p>
                    <img src="data:image/png;base64,%s" alt="Survey QR Code" style="width: 150px; height: 150px; border: 1px solid #ddd; padding: 5px; background: white;" />
                </div>`, data.SurveyQRCode))
			sb.WriteString("\n")
		}

		sb.WriteString(`            </div>`)
		sb.WriteString("\n")
	}

	// Summary
	if data.Summary != "" {
		sb.WriteString(fmt.Sprintf(`            <p style="font-weight: bold; margin-bottom: 15px;">%s</p>`, formatContentForHTML(data.Summary)))
		sb.WriteString("\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(fmt.Sprintf(`            <div style="margin-bottom: 20px;">%s</div>`, formatContentForHTML(data.Content)))
		sb.WriteString("\n")
	}

	// Completion info
	if data.CompletedBy != "" {
		sb.WriteString(`            <div style="margin-top: 20px; padding: 15px; background-color: #d4edda; border-left: 4px solid #28a745;">
                <h3 style="font-size: 1em; color: #155724; margin: 0 0 10px 0;">Completed</h3>`)
		sb.WriteString("\n")

		sb.WriteString(fmt.Sprintf(`                <div style="margin-bottom: 8px;">
                    <strong>By:</strong> %s`, formatContentForHTML(data.CompletedBy)))
		if data.CompletedByEmail != "" {
			sb.WriteString(fmt.Sprintf(` (%s)`, formatContentForHTML(data.CompletedByEmail)))
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")

		if !data.CompletedAt.IsZero() {
			formattedTime := data.CompletedAt.Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf(`                <div>
                    <strong>At:</strong> %s
                </div>`, formattedTime))
			sb.WriteString("\n")
		}

		sb.WriteString(`            </div>`)
		sb.WriteString("\n")
	}

	// Attachments
	if len(data.Attachments) > 0 {
		sb.WriteString("            ")
		sb.WriteString(renderAttachments(data.Attachments))
		sb.WriteString("\n")
	}

	sb.WriteString(`        </div>` + "\n")

	// Footer
	sb.WriteString("        ")
	sb.WriteString(renderHTMLFooter(data.EventID, data.EventType, b.config.PortalBaseURL))
	sb.WriteString("\n")

	// SES Macro
	sb.WriteString("        ")
	sb.WriteString(renderSESMacro(data.Timestamp))
	sb.WriteString("\n")

	sb.WriteString(`    </div>` + "\n")

	// Hidden metadata (at end for email client compatibility)
	sb.WriteString("    ")
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationCompleted)))
	sb.WriteString("\n")

	sb.WriteString(`</body>
</html>`)

	return sb.String()
}

// buildCompletionText builds the plain text body for completion notification
func (b *AnnouncementTemplateBuilder) buildCompletionText(data CompletionData, emoji string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(renderTextHeader(emoji, data.Title))

	// Status
	sb.WriteString(renderTextStatusLine(data.Status))

	// Summary
	if data.Summary != "" {
		sb.WriteString(data.Summary)
		sb.WriteString("\n\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(data.Content)
		sb.WriteString("\n\n")
	}

	// Completion info
	if data.CompletedBy != "" {
		sb.WriteString("Completed:\n")
		sb.WriteString(fmt.Sprintf("  By: %s", data.CompletedBy))
		if data.CompletedByEmail != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", data.CompletedByEmail))
		}
		sb.WriteString("\n")

		if !data.CompletedAt.IsZero() {
			formattedTime := data.CompletedAt.Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf("  At: %s\n", formattedTime))
		}
		sb.WriteString("\n")
	}

	// Attachments
	sb.WriteString(renderTextAttachments(data.Attachments))

	// Footer
	sb.WriteString(renderTextFooter(data.EventID, data.EventType, b.config.PortalBaseURL, data.Timestamp))

	return sb.String()
}

// BuildCancellation builds a cancellation notification email for announcements
func (b *AnnouncementTemplateBuilder) BuildCancellation(data CancellationData) EmailTemplate {
	category := CategoryType(data.Category)
	emoji := GetEmojiForNotification(NotificationCancelled, category)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildCancellationHTML(data, emoji, category)
	textBody := b.buildCancellationText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildCancellationHTML builds the HTML body for cancellation notification
func (b *AnnouncementTemplateBuilder) buildCancellationHTML(data CancellationData, emoji string, category CategoryType) string {
	backgroundColor := categoryColors[category]
	if backgroundColor == "" {
		backgroundColor = "#007bff"
	}

	var sb strings.Builder

	// HTML structure
	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            line-height: 1.6; 
            color: #333;
            margin: 0;
            padding: 0;
        }
        .email-container {
            max-width: 600px;
            margin: 0 auto;
        }
        .content {
            padding: 20px;
            background-color: #ffffff;
        }
        @media only screen and (max-width: 600px) {
            .email-container {
                width: 100% !important;
            }
        }
    </style>
</head>
<body>
`)

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	statusWord := getStatusWordForNotification(NotificationCancelled)
	sb.WriteString(renderHTMLHeader(statusWord, data.Title, backgroundColor))
	sb.WriteString("\n")

	// Content section
	sb.WriteString(`        <div class="content">` + "\n")

	// Status subtitle
	sb.WriteString("            ")
	sb.WriteString(renderStatusSubtitle(data.Status))
	sb.WriteString("\n")

	// Summary
	if data.Summary != "" {
		sb.WriteString(fmt.Sprintf(`            <p style="font-weight: bold; margin-bottom: 15px;">%s</p>`, formatContentForHTML(data.Summary)))
		sb.WriteString("\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(fmt.Sprintf(`            <div style="margin-bottom: 20px;">%s</div>`, formatContentForHTML(data.Content)))
		sb.WriteString("\n")
	}

	// Cancellation info
	if data.CancelledBy != "" {
		sb.WriteString(`            <div style="margin-top: 20px; padding: 15px; background-color: #f8d7da; border-left: 4px solid #dc3545;">
                <h3 style="font-size: 1em; color: #721c24; margin: 0 0 10px 0;">Cancelled</h3>`)
		sb.WriteString("\n")

		sb.WriteString(fmt.Sprintf(`                <div style="margin-bottom: 8px;">
                    <strong>By:</strong> %s`, formatContentForHTML(data.CancelledBy)))
		if data.CancelledByEmail != "" {
			sb.WriteString(fmt.Sprintf(` (%s)`, formatContentForHTML(data.CancelledByEmail)))
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")

		if !data.CancelledAt.IsZero() {
			formattedTime := data.CancelledAt.Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf(`                <div>
                    <strong>At:</strong> %s
                </div>`, formattedTime))
			sb.WriteString("\n")
		}

		sb.WriteString(`            </div>`)
		sb.WriteString("\n")
	}

	// Attachments
	if len(data.Attachments) > 0 {
		sb.WriteString("            ")
		sb.WriteString(renderAttachments(data.Attachments))
		sb.WriteString("\n")
	}

	sb.WriteString(`        </div>` + "\n")

	// Footer
	sb.WriteString("        ")
	sb.WriteString(renderHTMLFooter(data.EventID, data.EventType, b.config.PortalBaseURL))
	sb.WriteString("\n")

	// SES Macro
	sb.WriteString("        ")
	sb.WriteString(renderSESMacro(data.Timestamp))
	sb.WriteString("\n")

	sb.WriteString(`    </div>` + "\n")

	// Hidden metadata (at end for email client compatibility)
	sb.WriteString("    ")
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationCancelled)))
	sb.WriteString("\n")

	sb.WriteString(`</body>
</html>`)

	return sb.String()
}

// buildCancellationText builds the plain text body for cancellation notification
func (b *AnnouncementTemplateBuilder) buildCancellationText(data CancellationData, emoji string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(renderTextHeader(emoji, data.Title))

	// Status
	sb.WriteString(renderTextStatusLine(data.Status))

	// Summary
	if data.Summary != "" {
		sb.WriteString(data.Summary)
		sb.WriteString("\n\n")
	}

	// Content
	if data.Content != "" {
		sb.WriteString(data.Content)
		sb.WriteString("\n\n")
	}

	// Cancellation info
	if data.CancelledBy != "" {
		sb.WriteString("Cancelled:\n")
		sb.WriteString(fmt.Sprintf("  By: %s", data.CancelledBy))
		if data.CancelledByEmail != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", data.CancelledByEmail))
		}
		sb.WriteString("\n")

		if !data.CancelledAt.IsZero() {
			formattedTime := data.CancelledAt.Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf("  At: %s\n", formattedTime))
		}
		sb.WriteString("\n")
	}

	// Attachments
	sb.WriteString(renderTextAttachments(data.Attachments))

	// Footer
	sb.WriteString(renderTextFooter(data.EventID, data.EventType, b.config.PortalBaseURL, data.Timestamp))

	return sb.String()
}
