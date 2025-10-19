package templates

import (
	"ccoe-customer-contact-manager/internal/types"
	"fmt"
	"strings"
)

// changeColor is the header background color for change notifications
const changeColor = "#fd7e14" // Orange

// ChangeTemplateBuilder builds email templates for changes
type ChangeTemplateBuilder struct {
	config types.EmailConfig
}

// NewChangeTemplateBuilder creates a new change template builder
func NewChangeTemplateBuilder(config types.EmailConfig) *ChangeTemplateBuilder {
	return &ChangeTemplateBuilder{
		config: config,
	}
}

// BuildApprovalRequest builds an approval request email for changes
func (b *ChangeTemplateBuilder) BuildApprovalRequest(data ApprovalRequestData) EmailTemplate {
	emoji := GetEmojiForNotification(NotificationApprovalRequest, CategoryChange)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildApprovalRequestHTML(data, emoji)
	textBody := b.buildApprovalRequestText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildApprovalRequestHTML builds the HTML body for approval request
func (b *ChangeTemplateBuilder) buildApprovalRequestHTML(data ApprovalRequestData, emoji string) string {
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

	// Hidden metadata
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationApprovalRequest)))
	sb.WriteString("\n")

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	sb.WriteString(renderHTMLHeader(emoji, data.Title, changeColor))
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
            </div>`, data.ApprovalURL, changeColor))
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

	sb.WriteString(`    </div>
</body>
</html>`)

	return sb.String()
}

// buildApprovalRequestText builds the plain text body for approval request
func (b *ChangeTemplateBuilder) buildApprovalRequestText(data ApprovalRequestData, emoji string) string {
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

// BuildApprovedNotification builds an approved notification email for changes
func (b *ChangeTemplateBuilder) BuildApprovedNotification(data ApprovedNotificationData) EmailTemplate {
	emoji := GetEmojiForNotification(NotificationApproved, CategoryChange)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildApprovedNotificationHTML(data, emoji)
	textBody := b.buildApprovedNotificationText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildApprovedNotificationHTML builds the HTML body for approved notification
func (b *ChangeTemplateBuilder) buildApprovedNotificationHTML(data ApprovedNotificationData, emoji string) string {
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

	// Hidden metadata
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationApproved)))
	sb.WriteString("\n")

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	sb.WriteString(renderHTMLHeader(emoji, data.Title, changeColor))
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
		sb.WriteString(changeColor)
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

	sb.WriteString(`    </div>
</body>
</html>`)

	return sb.String()
}

// buildApprovedNotificationText builds the plain text body for approved notification
func (b *ChangeTemplateBuilder) buildApprovedNotificationText(data ApprovedNotificationData, emoji string) string {
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

// BuildMeetingInvitation builds a meeting invitation email for changes
func (b *ChangeTemplateBuilder) BuildMeetingInvitation(data MeetingData) EmailTemplate {
	emoji := GetEmojiForNotification(NotificationMeeting, CategoryChange)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildMeetingInvitationHTML(data, emoji)
	textBody := b.buildMeetingInvitationText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildMeetingInvitationHTML builds the HTML body for meeting invitation
func (b *ChangeTemplateBuilder) buildMeetingInvitationHTML(data MeetingData, emoji string) string {
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

	// Hidden metadata
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationMeeting)))
	sb.WriteString("\n")

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	sb.WriteString(renderHTMLHeader(emoji, data.Title, changeColor))
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
		sb.WriteString(changeColor)
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
                </div>`, data.MeetingMetadata.JoinURL, changeColor))
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

	sb.WriteString(`    </div>
</body>
</html>`)

	return sb.String()
}

// buildMeetingInvitationText builds the plain text body for meeting invitation
func (b *ChangeTemplateBuilder) buildMeetingInvitationText(data MeetingData, emoji string) string {
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

// BuildCompletion builds a completion notification email for changes
func (b *ChangeTemplateBuilder) BuildCompletion(data CompletionData) EmailTemplate {
	emoji := GetEmojiForNotification(NotificationCompleted, CategoryChange)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildCompletionHTML(data, emoji)
	textBody := b.buildCompletionText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildCompletionHTML builds the HTML body for completion notification
func (b *ChangeTemplateBuilder) buildCompletionHTML(data CompletionData, emoji string) string {
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

	// Hidden metadata
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationCompleted)))
	sb.WriteString("\n")

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	sb.WriteString(renderHTMLHeader(emoji, data.Title, changeColor))
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

	sb.WriteString(`    </div>
</body>
</html>`)

	return sb.String()
}

// buildCompletionText builds the plain text body for completion notification
func (b *ChangeTemplateBuilder) buildCompletionText(data CompletionData, emoji string) string {
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

// BuildCancellation builds a cancellation notification email for changes
func (b *ChangeTemplateBuilder) BuildCancellation(data CancellationData) EmailTemplate {
	emoji := GetEmojiForNotification(NotificationCancelled, CategoryChange)
	subject := buildSubject(emoji, data.Title)

	htmlBody := b.buildCancellationHTML(data, emoji)
	textBody := b.buildCancellationText(data, emoji)

	return EmailTemplate{
		Subject:  sanitizeSubject(subject),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
}

// buildCancellationHTML builds the HTML body for cancellation notification
func (b *ChangeTemplateBuilder) buildCancellationHTML(data CancellationData, emoji string) string {
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

	// Hidden metadata
	sb.WriteString(renderHiddenMetadata(data.EventID, data.EventType, string(NotificationCancelled)))
	sb.WriteString("\n")

	// Email container
	sb.WriteString(`    <div class="email-container">` + "\n")

	// Header
	sb.WriteString("        ")
	sb.WriteString(renderHTMLHeader(emoji, data.Title, changeColor))
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

	sb.WriteString(`    </div>
</body>
</html>`)

	return sb.String()
}

// buildCancellationText builds the plain text body for cancellation notification
func (b *ChangeTemplateBuilder) buildCancellationText(data CancellationData, emoji string) string {
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
