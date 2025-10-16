// Package ses provides announcement email template functions.
package ses

import (
	"fmt"
	"strings"
	"time"

	"ccoe-customer-contact-manager/internal/types"
)

// AnnouncementEmailTemplate represents an email template for announcements
type AnnouncementEmailTemplate struct {
	Type     string
	Subject  string
	HTMLBody string
	TextBody string
}

// AnnouncementData represents the data needed to render announcement templates
type AnnouncementData struct {
	AnnouncementID   string
	AnnouncementType string
	Title            string
	Summary          string
	Content          string
	Customers        []string
	MeetingMetadata  *types.MeetingMetadata
	Attachments      []AttachmentInfo
	CreatedBy        string
	CreatedAt        time.Time
}

// AttachmentInfo represents information about a file attachment
type AttachmentInfo struct {
	Name string
	URL  string
	Size string
}

// GetAnnouncementTemplate returns the appropriate email template based on announcement type
func GetAnnouncementTemplate(announcementType string, data AnnouncementData) AnnouncementEmailTemplate {
	switch strings.ToLower(announcementType) {
	case "cic":
		return getCICTemplate(data)
	case "finops":
		return getFinOpsTemplate(data)
	case "innersource":
		return getInnerSourceTemplate(data)
	default:
		return getGenericTemplate(data)
	}
}

// getCICTemplate returns the CIC (Cloud Innovator Community) email template
func getCICTemplate(data AnnouncementData) AnnouncementEmailTemplate {
	return AnnouncementEmailTemplate{
		Type:     "cic",
		Subject:  fmt.Sprintf("CIC Announcement: %s", data.Title),
		HTMLBody: renderCICHTMLTemplate(data),
		TextBody: renderCICTextTemplate(data),
	}
}

// getFinOpsTemplate returns the FinOps email template
func getFinOpsTemplate(data AnnouncementData) AnnouncementEmailTemplate {
	return AnnouncementEmailTemplate{
		Type:     "finops",
		Subject:  fmt.Sprintf("FinOps Update: %s", data.Title),
		HTMLBody: renderFinOpsHTMLTemplate(data),
		TextBody: renderFinOpsTextTemplate(data),
	}
}

// getInnerSourceTemplate returns the InnerSource Guild email template
func getInnerSourceTemplate(data AnnouncementData) AnnouncementEmailTemplate {
	return AnnouncementEmailTemplate{
		Type:     "innersource",
		Subject:  fmt.Sprintf("InnerSource Guild: %s", data.Title),
		HTMLBody: renderInnerSourceHTMLTemplate(data),
		TextBody: renderInnerSourceTextTemplate(data),
	}
}

// getGenericTemplate returns a generic announcement email template
func getGenericTemplate(data AnnouncementData) AnnouncementEmailTemplate {
	return AnnouncementEmailTemplate{
		Type:     "general",
		Subject:  fmt.Sprintf("Announcement: %s", data.Title),
		HTMLBody: renderGenericHTMLTemplate(data),
		TextBody: renderGenericTextTemplate(data),
	}
}

// renderCICHTMLTemplate renders the CIC HTML email template
func renderCICHTMLTemplate(data AnnouncementData) string {
	meetingSection := ""
	if data.MeetingMetadata != nil {
		meetingSection = fmt.Sprintf(`
		<div class="meeting-info">
			<h3>📅 Meeting Information</h3>
			<p><strong>Join URL:</strong> <a href="%s">Click to Join</a></p>
			<p><strong>Date/Time:</strong> %s</p>
			<p><strong>Subject:</strong> %s</p>
		</div>
		`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	attachmentsSection := ""
	if len(data.Attachments) > 0 {
		attachmentsSection = `<div class="attachments"><h3>📎 Attachments</h3>`
		for _, attachment := range data.Attachments {
			attachmentsSection += fmt.Sprintf(`<p><a href="%s">%s</a> (%s)</p>`, attachment.URL, attachment.Name, attachment.Size)
		}
		attachmentsSection += `</div>`
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.cic-header { background-color: #0066cc; color: white; padding: 20px; }
		.cic-content { padding: 20px; }
		.meeting-info { background-color: #f0f8ff; padding: 15px; margin: 20px 0; border-left: 4px solid #0066cc; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #0066cc; text-decoration: none; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<div class="cic-header">
		<h1>☁️ Cloud Innovator Community</h1>
	</div>
	<div class="cic-content">
		<h2>%s</h2>
		<p><strong>Summary:</strong> %s</p>
		<div>%s</div>
		%s
		%s
	</div>
	<div class="footer">
		<p>This event notification was sent by the CCOE Customer Contact Manager.</p>
		<p>If you have questions, please contact your Cloud Innovator Community team.</p>
	</div>
	<div class="unsubscribe" style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px;">
		<p>Event notification sent at %s</p>
		<div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
	</div>
</body>
</html>
`, data.Title, data.Summary, formatContentForHTML(data.Content), meetingSection, attachmentsSection, time.Now().Format("January 2, 2006 at 3:04 PM MST"))
}

// renderFinOpsHTMLTemplate renders the FinOps HTML email template
func renderFinOpsHTMLTemplate(data AnnouncementData) string {
	meetingSection := ""
	if data.MeetingMetadata != nil {
		meetingSection = fmt.Sprintf(`
		<div class="meeting-info">
			<h3>📅 Meeting Information</h3>
			<p><strong>Join URL:</strong> <a href="%s">Click to Join</a></p>
			<p><strong>Date/Time:</strong> %s</p>
			<p><strong>Subject:</strong> %s</p>
		</div>
		`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	attachmentsSection := ""
	if len(data.Attachments) > 0 {
		attachmentsSection = `<div class="attachments"><h3>📎 Attachments</h3>`
		for _, attachment := range data.Attachments {
			attachmentsSection += fmt.Sprintf(`<p><a href="%s">%s</a> (%s)</p>`, attachment.URL, attachment.Name, attachment.Size)
		}
		attachmentsSection += `</div>`
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.finops-header { background-color: #28a745; color: white; padding: 20px; }
		.finops-content { padding: 20px; }
		.cost-highlight { background-color: #d4edda; padding: 10px; margin: 10px 0; border-left: 4px solid #28a745; }
		.meeting-info { background-color: #d4edda; padding: 15px; margin: 20px 0; border-left: 4px solid #28a745; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #28a745; text-decoration: none; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<div class="finops-header">
		<h1>💰 FinOps Update</h1>
	</div>
	<div class="finops-content">
		<h2>%s</h2>
		<p><strong>Summary:</strong> %s</p>
		<div>%s</div>
		%s
		%s
	</div>
	<div class="footer">
		<p>This event notification was sent by the CCOE Customer Contact Manager.</p>
		<p>For questions about cost optimization, please contact your FinOps team.</p>
	</div>
	<div class="unsubscribe" style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px;">
		<p>Event notification sent at %s</p>
		<div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
	</div>
</body>
</html>
`, data.Title, data.Summary, formatContentForHTML(data.Content), meetingSection, attachmentsSection, time.Now().Format("January 2, 2006 at 3:04 PM MST"))
}

// renderInnerSourceHTMLTemplate renders the InnerSource HTML email template
func renderInnerSourceHTMLTemplate(data AnnouncementData) string {
	meetingSection := ""
	if data.MeetingMetadata != nil {
		meetingSection = fmt.Sprintf(`
		<div class="meeting-info">
			<h3>📅 Meeting Information</h3>
			<p><strong>Join URL:</strong> <a href="%s">Click to Join</a></p>
			<p><strong>Date/Time:</strong> %s</p>
			<p><strong>Subject:</strong> %s</p>
		</div>
		`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	attachmentsSection := ""
	if len(data.Attachments) > 0 {
		attachmentsSection = `<div class="attachments"><h3>📎 Attachments</h3>`
		for _, attachment := range data.Attachments {
			attachmentsSection += fmt.Sprintf(`<p><a href="%s">%s</a> (%s)</p>`, attachment.URL, attachment.Name, attachment.Size)
		}
		attachmentsSection += `</div>`
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.innersource-header { background-color: #6f42c1; color: white; padding: 20px; }
		.innersource-content { padding: 20px; }
		.project-highlight { background-color: #e7d9f7; padding: 10px; margin: 10px 0; border-left: 4px solid #6f42c1; }
		.meeting-info { background-color: #e7d9f7; padding: 15px; margin: 20px 0; border-left: 4px solid #6f42c1; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #6f42c1; text-decoration: none; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<div class="innersource-header">
		<h1>🔧 InnerSource Guild</h1>
	</div>
	<div class="innersource-content">
		<h2>%s</h2>
		<p><strong>Summary:</strong> %s</p>
		<div>%s</div>
		%s
		%s
	</div>
	<div class="footer">
		<p>This event notification was sent by the CCOE Customer Contact Manager.</p>
		<p>For questions about InnerSource projects, please contact the InnerSource Guild.</p>
	</div>
	<div class="unsubscribe" style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px;">
		<p>Event notification sent at %s</p>
		<div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
	</div>
</body>
</html>
`, data.Title, data.Summary, formatContentForHTML(data.Content), meetingSection, attachmentsSection, time.Now().Format("January 2, 2006 at 3:04 PM MST"))
}

// renderGenericHTMLTemplate renders a generic HTML email template
func renderGenericHTMLTemplate(data AnnouncementData) string {
	meetingSection := ""
	if data.MeetingMetadata != nil {
		meetingSection = fmt.Sprintf(`
		<div class="meeting-info">
			<h3>📅 Meeting Information</h3>
			<p><strong>Join URL:</strong> <a href="%s">Click to Join</a></p>
			<p><strong>Date/Time:</strong> %s</p>
			<p><strong>Subject:</strong> %s</p>
		</div>
		`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	attachmentsSection := ""
	if len(data.Attachments) > 0 {
		attachmentsSection = `<div class="attachments"><h3>📎 Attachments</h3>`
		for _, attachment := range data.Attachments {
			attachmentsSection += fmt.Sprintf(`<p><a href="%s">%s</a> (%s)</p>`, attachment.URL, attachment.Name, attachment.Size)
		}
		attachmentsSection += `</div>`
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		.header { background-color: #007bff; color: white; padding: 20px; }
		.content { padding: 20px; }
		.meeting-info { background-color: #e7f3ff; padding: 15px; margin: 20px 0; border-left: 4px solid #007bff; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #007bff; text-decoration: none; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<div class="header">
		<h1>📢 Announcement</h1>
	</div>
	<div class="content">
		<h2>%s</h2>
		<p><strong>Summary:</strong> %s</p>
		<div>%s</div>
		%s
		%s
	</div>
	<div class="footer">
		<p>This event notification was sent by the CCOE Customer Contact Manager.</p>
	</div>
	<div class="unsubscribe" style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px;">
		<p>Event notification sent at %s</p>
		<div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
	</div>
</body>
</html>
`, data.Title, data.Summary, formatContentForHTML(data.Content), meetingSection, attachmentsSection, time.Now().Format("January 2, 2006 at 3:04 PM MST"))
}

// renderCICTextTemplate renders the CIC plain text email template
func renderCICTextTemplate(data AnnouncementData) string {
	text := fmt.Sprintf(`
☁️ CLOUD INNOVATOR COMMUNITY EVENT

%s

Summary: %s

%s
`, data.Title, data.Summary, data.Content)

	if data.MeetingMetadata != nil {
		text += fmt.Sprintf(`

📅 MEETING INFORMATION
Join URL: %s
Date/Time: %s
Subject: %s
`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	if len(data.Attachments) > 0 {
		text += "\n\n📎 ATTACHMENTS\n"
		for _, attachment := range data.Attachments {
			text += fmt.Sprintf("%s (%s): %s\n", attachment.Name, attachment.Size, attachment.URL)
		}
	}

	text += "\n\nThis event notification was sent by the CCOE Customer Contact Manager.\nIf you have questions, please contact your Cloud Innovator Community team."

	return text
}

// renderFinOpsTextTemplate renders the FinOps plain text email template
func renderFinOpsTextTemplate(data AnnouncementData) string {
	text := fmt.Sprintf(`
💰 FINOPS EVENT

%s

Summary: %s

%s
`, data.Title, data.Summary, data.Content)

	if data.MeetingMetadata != nil {
		text += fmt.Sprintf(`

📅 MEETING INFORMATION
Join URL: %s
Date/Time: %s
Subject: %s
`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	if len(data.Attachments) > 0 {
		text += "\n\n📎 ATTACHMENTS\n"
		for _, attachment := range data.Attachments {
			text += fmt.Sprintf("%s (%s): %s\n", attachment.Name, attachment.Size, attachment.URL)
		}
	}

	text += "\n\nThis event notification was sent by the CCOE Customer Contact Manager.\nFor questions about cost optimization, please contact your FinOps team."

	return text
}

// renderInnerSourceTextTemplate renders the InnerSource plain text email template
func renderInnerSourceTextTemplate(data AnnouncementData) string {
	text := fmt.Sprintf(`
🔧 INNERSOURCE GUILD EVENT

%s

Summary: %s

%s
`, data.Title, data.Summary, data.Content)

	if data.MeetingMetadata != nil {
		text += fmt.Sprintf(`

📅 MEETING INFORMATION
Join URL: %s
Date/Time: %s
Subject: %s
`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	if len(data.Attachments) > 0 {
		text += "\n\n📎 ATTACHMENTS\n"
		for _, attachment := range data.Attachments {
			text += fmt.Sprintf("%s (%s): %s\n", attachment.Name, attachment.Size, attachment.URL)
		}
	}

	text += "\n\nThis event notification was sent by the CCOE Customer Contact Manager.\nFor questions about InnerSource projects, please contact the InnerSource Guild."

	return text
}

// renderGenericTextTemplate renders a generic plain text email template
func renderGenericTextTemplate(data AnnouncementData) string {
	text := fmt.Sprintf(`
📢 EVENT NOTIFICATION

%s

Summary: %s

%s
`, data.Title, data.Summary, data.Content)

	if data.MeetingMetadata != nil {
		text += fmt.Sprintf(`

📅 MEETING INFORMATION
Join URL: %s
Date/Time: %s
Subject: %s
`, data.MeetingMetadata.JoinURL, formatMeetingTime(data.MeetingMetadata.StartTime), data.MeetingMetadata.Subject)
	}

	if len(data.Attachments) > 0 {
		text += "\n\n📎 ATTACHMENTS\n"
		for _, attachment := range data.Attachments {
			text += fmt.Sprintf("%s (%s): %s\n", attachment.Name, attachment.Size, attachment.URL)
		}
	}

	text += "\n\nThis event notification was sent by the CCOE Customer Contact Manager."

	return text
}

// formatContentForHTML converts plain text content to HTML-friendly format
func formatContentForHTML(content string) string {
	// Replace newlines with <br> tags
	content = strings.ReplaceAll(content, "\n", "<br>")
	return content
}

// formatMeetingTime formats a meeting time string for display
func formatMeetingTime(timeStr string) string {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return timeStr // Return original if parsing fails
	}
	return t.Format("January 2, 2006 at 3:04 PM MST")
}
