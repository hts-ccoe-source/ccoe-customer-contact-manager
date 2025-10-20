package templates

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// renderHiddenMetadata generates hidden HTML fields for email tracking
func renderHiddenMetadata(eventID string, eventType string, notificationType string) string {
	return fmt.Sprintf(`<div style="display:none; max-height:0px; overflow:hidden;">
    <span id="event-id">%s</span>
    <span id="event-type">%s</span>
    <span id="notification-type">%s</span>
</div>`,
		html.EscapeString(eventID),
		html.EscapeString(eventType),
		html.EscapeString(notificationType),
	)
}

// renderHTMLHeader generates the HTML header section with status word and title (no emoji)
func renderHTMLHeader(statusWord string, title string, backgroundColor string) string {
	return fmt.Sprintf(`<div class="header" style="padding: 20px; color: white; background-color: %s;">
    <h1 style="margin: 0; font-size: 1.5em;">%s: %s</h1>
</div>`,
		backgroundColor,
		html.EscapeString(statusWord),
		html.EscapeString(title),
	)
}

// getStatusWordForNotification returns the status word to display in the header
func getStatusWordForNotification(notificationType NotificationType) string {
	switch notificationType {
	case NotificationApprovalRequest:
		return "Approval Required"
	case NotificationApproved:
		return "Approved"
	case NotificationCompleted:
		return "Completed"
	case NotificationCancelled:
		return "Cancelled"
	case NotificationMeeting:
		return "Meeting Invitation"
	default:
		return "Notification"
	}
}

// renderStatusSubtitle generates a status subtitle for the email body
func renderStatusSubtitle(status string) string {
	statusDisplay := getStatusDisplay(status)
	return fmt.Sprintf(`<div class="status-subtitle" style="color: #6c757d; font-size: 0.9em; margin-bottom: 15px;">
    Status: %s
</div>`, html.EscapeString(statusDisplay))
}

// renderAttachments generates HTML for attachment links
func renderAttachments(attachments []string) string {
	if len(attachments) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(`<div class="attachments" style="margin-top: 20px;">
    <h3 style="font-size: 1em; color: #495057; margin-bottom: 10px;">📎 Attachments</h3>
    <ul style="list-style-type: none; padding-left: 0;">`)

	for _, attachment := range attachments {
		sb.WriteString(fmt.Sprintf(`
        <li style="margin-bottom: 5px;">
            <a href="%s" style="color: #007bff; text-decoration: none;">%s</a>
        </li>`,
			html.EscapeString(attachment),
			html.EscapeString(attachment),
		))
	}

	sb.WriteString(`
    </ul>
</div>`)

	return sb.String()
}

// buildTagline generates the tagline with hyperlinked event ID for HTML emails
func buildTagline(eventID string, eventType string, baseURL string) string {
	var url string
	if eventType == "announcement" {
		url = fmt.Sprintf("%s/edit-announcement.html?announcementId=%s", baseURL, eventID)
	} else {
		url = fmt.Sprintf("%s/edit-change.html?changeId=%s", baseURL, eventID)
	}

	return fmt.Sprintf(`event ID <a href="%s" style="color: #007bff; text-decoration: none;">%s</a> sent by the <a href="https://github.com/hts-ccoe-source/ccoe-customer-contact-manager" style="color: #007bff; text-decoration: none;">CCOE customer contact manager</a>`,
		html.EscapeString(url),
		html.EscapeString(eventID),
	)
}

// buildTaglineText generates the tagline for plain text emails
func buildTaglineText(eventID string, eventType string, baseURL string) string {
	var url string
	if eventType == "announcement" {
		url = fmt.Sprintf("%s/edit-announcement.html?announcementId=%s", baseURL, eventID)
	} else {
		url = fmt.Sprintf("%s/edit-change.html?changeId=%s", baseURL, eventID)
	}

	return fmt.Sprintf("event ID %s (%s) sent by the CCOE customer contact manager (https://github.com/hts-ccoe-source/ccoe-customer-contact-manager)", eventID, url)
}

// renderHTMLFooter generates the HTML footer with tagline
func renderHTMLFooter(eventID string, eventType string, baseURL string) string {
	tagline := buildTagline(eventID, eventType, baseURL)
	return fmt.Sprintf(`<div class="footer" style="background-color: #f5f5f5; padding: 15px 20px; font-size: 0.9em; color: #666;">
    <p style="margin: 0;">%s</p>
</div>`, tagline)
}

// renderSESMacro generates the SES unsubscribe macro section
func renderSESMacro(timestamp time.Time) string {
	formattedTime := timestamp.Format("2006-01-02 15:04:05 MST")
	return fmt.Sprintf(`<div class="unsubscribe" style="background-color: #e9ecef; padding: 15px 20px; margin-top: 20px;">
    <p style="margin: 0 0 10px 0; font-size: 0.9em; color: #666;">Notification sent at %s</p>
    <p style="margin: 0; font-size: 0.9em;">
        <a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none;">📧 Manage Email Preferences or Unsubscribe</a>
    </p>
</div>`, html.EscapeString(formattedTime))
}

// renderTextHeader generates the plain text header
func renderTextHeader(emoji string, title string) string {
	return fmt.Sprintf("%s %s\n%s\n\n", emoji, title, strings.Repeat("=", len(title)+4))
}

// renderTextStatusLine generates the status line for plain text emails
func renderTextStatusLine(status string) string {
	statusDisplay := getStatusDisplay(status)
	return fmt.Sprintf("Status: %s\n\n", statusDisplay)
}

// renderTextAttachments generates attachment list for plain text emails
func renderTextAttachments(attachments []string) string {
	if len(attachments) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n📎 Attachments:\n")
	for _, attachment := range attachments {
		sb.WriteString(fmt.Sprintf("  - %s\n", attachment))
	}

	return sb.String()
}

// renderTextFooter generates the plain text footer
func renderTextFooter(eventID string, eventType string, baseURL string, timestamp time.Time) string {
	tagline := buildTaglineText(eventID, eventType, baseURL)
	formattedTime := timestamp.Format("2006-01-02 15:04:05 MST")

	return fmt.Sprintf(`
%s
%s

Notification sent at %s

Manage Email Preferences or Unsubscribe: {{amazonSESUnsubscribeUrl}}
`,
		strings.Repeat("-", 70),
		tagline,
		formattedTime,
	)
}

// buildSubject generates a mobile-optimized subject line
func buildSubject(emoji string, title string) string {
	// The emoji conveys the notification type, title provides context
	// No truncation needed - portal UI enforces title length limits
	return fmt.Sprintf("%s %s", emoji, title)
}

// getStatusDisplay maps internal status codes to user-friendly display text
func getStatusDisplay(status string) string {
	statusMap := map[string]string{
		"pending_approval": "Pending Approval",
		"approved":         "Approved",
		"completed":        "Completed",
		"cancelled":        "Cancelled",
		"in_progress":      "In Progress",
		"scheduled":        "Scheduled",
		"draft":            "Draft",
	}

	if display, ok := statusMap[status]; ok {
		return display
	}

	// Fallback: capitalize first letter and replace underscores with spaces
	return strings.Title(strings.ReplaceAll(status, "_", " "))
}

// formatContentForHTML converts plain text content to HTML-safe format
func formatContentForHTML(content string) string {
	// Escape HTML special characters
	escaped := html.EscapeString(content)

	// Convert newlines to <br> tags
	formatted := strings.ReplaceAll(escaped, "\n", "<br>")

	return formatted
}

// sanitizeHTML removes potentially dangerous HTML while preserving safe formatting
func sanitizeHTML(input string) string {
	// For now, we escape all HTML to prevent XSS
	// In the future, we could use a proper HTML sanitizer library
	// to allow safe tags like <b>, <i>, <a>, etc.
	return html.EscapeString(input)
}

// sanitizeSubject removes newlines and control characters from subject lines
func sanitizeSubject(subject string) string {
	// Remove newlines and carriage returns to prevent header injection
	cleaned := strings.ReplaceAll(subject, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", " ")

	// Remove other control characters
	var sb strings.Builder
	for _, r := range cleaned {
		if r >= 32 || r == '\t' {
			sb.WriteRune(r)
		}
	}

	return strings.TrimSpace(sb.String())
}
