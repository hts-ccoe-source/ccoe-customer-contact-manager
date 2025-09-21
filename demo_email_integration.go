package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// Demo application for email template management and SES integration

func main() {
	fmt.Println("=== Email Template Management & SES Integration Demo ===")

	// Demo 1: Email template management
	fmt.Println("\n🧪 Demo 1: Email Template Management")
	demoEmailTemplateManagement()

	// Demo 2: SES integration
	fmt.Println("\n🧪 Demo 2: SES Integration")
	demoSESIntegration()

	// Demo 3: Multi-customer email sending
	fmt.Println("\n🧪 Demo 3: Multi-Customer Email Sending")
	demoMultiCustomerEmailSending()

	// Demo 4: Email validation and statistics
	fmt.Println("\n🧪 Demo 4: Email Validation & Statistics")
	demoEmailValidationAndStats()

	// Demo 5: Template rendering and customization
	fmt.Println("\n🧪 Demo 5: Template Rendering & Customization")
	demoTemplateRenderingAndCustomization()

	fmt.Println("\n=== Demo Complete ===")
}

func demoEmailTemplateManagement() {
	customerManager := setupDemoCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)

	fmt.Printf("📋 Email Template Management Demo\n")

	// List default templates
	fmt.Printf("   📊 Default templates available:\n")
	defaultTemplates := templateManager.ListTemplates("")
	for _, template := range defaultTemplates {
		if template.IsDefault {
			fmt.Printf("      - %s: %s\n", template.ID, template.Name)
		}
	}

	// Create custom template
	fmt.Printf("\n   📝 Creating custom notification template...\n")
	customTemplate := &EmailTemplate{
		ID:          "custom-notification",
		Name:        "Custom Notification Template",
		Description: "Customized notification template with branding",
		Subject:     "[{{.CustomerName}}] {{.NotificationType}}: {{.Title}}",
		HTMLBody: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px 20px; text-align: center; }
        .content { padding: 30px 20px; }
        .footer { background-color: #f8f9fa; padding: 20px; text-align: center; font-size: 14px; color: #6c757d; }
        .highlight { background-color: #e3f2fd; padding: 15px; border-left: 4px solid #2196f3; margin: 20px 0; }
        .button { display: inline-block; padding: 12px 24px; background-color: #2196f3; color: white; text-decoration: none; border-radius: 6px; font-weight: bold; }
        .changes-list { background-color: #f5f5f5; padding: 15px; border-radius: 6px; margin: 15px 0; }
        .priority-high { border-left-color: #f44336; }
        .priority-normal { border-left-color: #ff9800; }
        .priority-low { border-left-color: #4caf50; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.Title}}</h1>
            <p>{{.CustomerName}} - {{.Environment}} Environment</p>
        </div>
        <div class="content">
            <p>Hello {{.RecipientName}},</p>
            <p>{{.Message}}</p>
            
            {{if .Changes}}
            <div class="highlight priority-{{.Priority}}">
                <h3>📋 Changes Summary ({{len .Changes}} items):</h3>
                <div class="changes-list">
                    <ul>
                    {{range .Changes}}
                        <li>{{.}}</li>
                    {{end}}
                    </ul>
                </div>
            </div>
            {{end}}
            
            {{if .ActionRequired}}
            <div class="highlight">
                <h3>⚠️ Action Required:</h3>
                <p>{{.ActionRequired}}</p>
                {{if .ActionDeadline}}
                <p><strong>Deadline:</strong> {{.ActionDeadline}}</p>
                {{end}}
            </div>
            {{end}}
            
            {{if .ActionURL}}
            <p style="text-align: center; margin: 30px 0;">
                <a href="{{.ActionURL}}" class="button">Take Action Now</a>
            </p>
            {{end}}
            
            <p>If you have any questions or need assistance, please don't hesitate to contact our support team.</p>
            
            <p>Best regards,<br>
            The {{.CustomerName}} Team</p>
        </div>
        <div class="footer">
            <p>This is an automated message from {{.ServiceName}}.</p>
            <p>Generated on {{.GeneratedAt}} | Customer: {{.CustomerCode}} | Environment: {{.Environment}}</p>
            {{if .UnsubscribeURL}}
            <p><a href="{{.UnsubscribeURL}}">Unsubscribe from these notifications</a></p>
            {{end}}
        </div>
    </div>
</body>
</html>`,
		TextBody: `{{.Title}}
{{.CustomerName}} - {{.Environment}} Environment

Hello {{.RecipientName}},

{{.Message}}

{{if .Changes}}
Changes Summary ({{len .Changes}} items):
{{range .Changes}}
- {{.}}
{{end}}
{{end}}

{{if .ActionRequired}}
⚠️ Action Required:
{{.ActionRequired}}
{{if .ActionDeadline}}
Deadline: {{.ActionDeadline}}
{{end}}
{{end}}

{{if .ActionURL}}
Take Action: {{.ActionURL}}
{{end}}

If you have any questions or need assistance, please don't hesitate to contact our support team.

Best regards,
The {{.CustomerName}} Team

---
This is an automated message from {{.ServiceName}}.
Generated on {{.GeneratedAt}} | Customer: {{.CustomerCode}} | Environment: {{.Environment}}
{{if .UnsubscribeURL}}
Unsubscribe: {{.UnsubscribeURL}}
{{end}}`,
		Variables: []TemplateVariable{
			{Name: "Title", Type: "string", Description: "Notification title", Required: true, Example: "Email List Updated"},
			{Name: "NotificationType", Type: "string", Description: "Type of notification", Required: true, Example: "UPDATE"},
			{Name: "CustomerName", Type: "string", Description: "Customer organization name", Required: true, Example: "HTS Production"},
			{Name: "RecipientName", Type: "string", Description: "Recipient's name", Required: false, Example: "John Doe"},
			{Name: "Message", Type: "string", Description: "Main notification message", Required: true, Example: "Your email distribution list has been updated."},
			{Name: "Changes", Type: "array", Description: "List of changes made", Required: false, Example: "Added 5 new recipients"},
			{Name: "Priority", Type: "string", Description: "Priority level (high, normal, low)", Required: false, DefaultValue: "normal", Example: "high"},
			{Name: "ActionRequired", Type: "string", Description: "Action required from recipient", Required: false, Example: "Please review and approve the changes"},
			{Name: "ActionDeadline", Type: "string", Description: "Deadline for action", Required: false, Example: "2024-01-20 17:00 EST"},
			{Name: "ActionURL", Type: "string", Description: "URL for taking action", Required: false, Example: "https://portal.example.com/approve/12345"},
			{Name: "ServiceName", Type: "string", Description: "Name of the service", Required: false, DefaultValue: "Email Distribution System", Example: "Email Distribution System"},
			{Name: "UnsubscribeURL", Type: "string", Description: "Unsubscribe URL", Required: false, Example: "https://portal.example.com/unsubscribe"},
		},
	}

	err := templateManager.CreateTemplate(customTemplate)
	if err != nil {
		log.Printf("❌ Failed to create custom template: %v", err)
		return
	}

	fmt.Printf("   ✅ Custom template created successfully\n")

	// Create customer-specific template
	fmt.Printf("   📝 Creating HTS-specific template...\n")
	htsTemplate := &EmailTemplate{
		ID:           "hts-branded",
		Name:         "HTS Branded Template",
		Description:  "HTS-specific branded template",
		Subject:      "[HTS Alert] {{.Title}}",
		HTMLBody:     `<div style="background-color: #1e3a8a; color: white; padding: 20px;"><h1>HTS Production Alert</h1></div><div style="padding: 20px;"><p>{{.Message}}</p></div>`,
		TextBody:     `HTS Production Alert\n\n{{.Message}}`,
		CustomerCode: "hts",
		Variables: []TemplateVariable{
			{Name: "Title", Type: "string", Required: true},
			{Name: "Message", Type: "string", Required: true},
		},
	}

	err = templateManager.CreateTemplate(htsTemplate)
	if err != nil {
		log.Printf("❌ Failed to create HTS template: %v", err)
		return
	}

	fmt.Printf("   ✅ HTS-specific template created successfully\n")

	// List all templates for HTS
	fmt.Printf("\n   📊 Templates available for HTS customer:\n")
	htsTemplates := templateManager.ListTemplates("hts")
	for _, template := range htsTemplates {
		templateType := "Global"
		if template.IsDefault {
			templateType = "Default"
		} else if template.CustomerCode == "hts" {
			templateType = "HTS-Specific"
		}
		fmt.Printf("      - %s (%s): %s\n", template.ID, templateType, template.Name)
	}

	// Export and import template
	fmt.Printf("\n   📤 Exporting custom template...\n")
	exportData, err := templateManager.ExportTemplate("custom-notification")
	if err != nil {
		log.Printf("❌ Failed to export template: %v", err)
		return
	}

	fmt.Printf("   ✅ Template exported (%d bytes)\n", len(exportData))

	// Delete and reimport
	templateManager.DeleteTemplate("custom-notification")
	fmt.Printf("   🗑️  Template deleted\n")

	err = templateManager.ImportTemplate(exportData)
	if err != nil {
		log.Printf("❌ Failed to import template: %v", err)
		return
	}

	fmt.Printf("   📥 Template imported successfully\n")
}

func demoSESIntegration() {
	customerManager := setupDemoCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)

	fmt.Printf("📧 SES Integration Demo\n")

	// Get SES configuration for customer
	fmt.Printf("   🔧 Getting SES configuration for HTS...\n")
	config, err := sesManager.GetSESConfiguration("hts")
	if err != nil {
		log.Printf("❌ Failed to get SES configuration: %v", err)
		return
	}

	fmt.Printf("   ✅ SES Configuration:\n")
	fmt.Printf("      Customer: %s (%s)\n", config.CustomerCode, config.Region)
	fmt.Printf("      From Email: %s\n", config.FromEmail)
	fmt.Printf("      From Name: %s\n", config.FromName)
	fmt.Printf("      Sending Quota: %d emails\n", config.SendingQuota)
	fmt.Printf("      Sending Rate: %.1f emails/second\n", config.SendingRate)
	fmt.Printf("      Verified Domains: %v\n", config.VerifiedDomains)

	// Validate email addresses
	fmt.Printf("\n   🔍 Validating email addresses...\n")
	testEmails := []string{
		"valid.user@example.com",
		"invalid-email",
		"test@tempmail.com",
		"admin@hts.example.com",
	}

	validationResults, err := sesManager.ValidateEmailAddresses("hts", testEmails)
	if err != nil {
		log.Printf("❌ Failed to validate emails: %v", err)
		return
	}

	for _, result := range validationResults {
		status := "❌"
		if result.Valid {
			status = "✅"
		}
		riskFlag := ""
		if result.Risky {
			riskFlag = " ⚠️ RISKY"
		}

		fmt.Printf("      %s %s - Valid: %t, Deliverable: %t%s\n",
			status, result.Email, result.Valid, result.Deliverable, riskFlag)
		if result.Reason != "" {
			fmt.Printf("         Reason: %s\n", result.Reason)
		}
	}

	// Send test email
	fmt.Printf("\n   📤 Sending test email...\n")
	emailRequest := &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "admin@hts.example.com", Name: "HTS Admin", Type: "to"},
			{Email: "manager@hts.example.com", Name: "HTS Manager", Type: "cc"},
		},
		TemplateData: map[string]interface{}{
			"Title":   "System Maintenance Notification",
			"Message": "Scheduled maintenance will occur tonight from 2:00 AM to 4:00 AM EST.",
			"Changes": []string{
				"Email service will be temporarily unavailable",
				"All scheduled emails will be queued and sent after maintenance",
				"Emergency notifications will continue to function",
			},
			"ActionRequired": "Please inform your team about the maintenance window",
			"ActionURL":      "https://portal.hts.example.com/maintenance/details",
		},
		FromEmail: "noreply@hts.example.com",
		FromName:  "HTS Operations Team",
		Tags: map[string]string{
			"Type":     "maintenance",
			"Priority": "high",
			"Customer": "hts",
		},
		Priority: "high",
	}

	response, err := sesManager.SendEmail(emailRequest)
	if err != nil {
		log.Printf("❌ Failed to send email: %v", err)
		return
	}

	fmt.Printf("   ✅ Email sent successfully\n")
	fmt.Printf("      Message ID: %s\n", response.MessageID)
	fmt.Printf("      Status: %s\n", response.Status)
	fmt.Printf("      Recipients: %d\n", len(response.Recipients))
	fmt.Printf("      Sent At: %s\n", response.SentAt.Format("2006-01-02 15:04:05"))

	// Get sending statistics
	fmt.Printf("\n   📊 Getting sending statistics...\n")
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	stats, err := sesManager.GetSendingStatistics("hts", startTime, endTime)
	if err != nil {
		log.Printf("❌ Failed to get statistics: %v", err)
		return
	}

	fmt.Printf("   ✅ Sending Statistics (24h):\n")
	fmt.Printf("      Sent: %v emails\n", stats["sent"])
	fmt.Printf("      Delivered: %v emails\n", stats["delivered"])
	fmt.Printf("      Bounced: %v emails\n", stats["bounced"])
	fmt.Printf("      Complained: %v emails\n", stats["complained"])
	fmt.Printf("      Delivery Rate: %.1f%%\n", stats["deliveryRate"])
	fmt.Printf("      Bounce Rate: %.1f%%\n", stats["bounceRate"])
	fmt.Printf("      Complaint Rate: %.1f%%\n", stats["complaintRate"])

	// List suppressed addresses
	fmt.Printf("\n   🚫 Checking suppressed addresses...\n")
	suppressedAddresses, err := sesManager.ListSuppressedAddresses("hts")
	if err != nil {
		log.Printf("❌ Failed to get suppressed addresses: %v", err)
		return
	}

	if len(suppressedAddresses) > 0 {
		fmt.Printf("   ⚠️  Found %d suppressed addresses:\n", len(suppressedAddresses))
		for _, addr := range suppressedAddresses {
			fmt.Printf("      - %s (Reason: %s, Updated: %s)\n",
				addr["email"], addr["reason"], addr["lastUpdateTime"])
		}
	} else {
		fmt.Printf("   ✅ No suppressed addresses found\n")
	}
}

func demoMultiCustomerEmailSending() {
	customerManager := setupDemoCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)

	fmt.Printf("🏢 Multi-Customer Email Sending Demo\n")

	// Prepare bulk email request
	fmt.Printf("   📋 Preparing bulk email for multiple customers...\n")
	bulkRequest := &BulkEmailRequest{
		TemplateID: "notification",
		CustomerEmails: map[string][]EmailRecipient{
			"hts": {
				{Email: "admin@hts.example.com", Name: "HTS Admin", Type: "to"},
				{Email: "manager@hts.example.com", Name: "HTS Manager", Type: "to"},
			},
			"cds": {
				{Email: "admin@cds.example.com", Name: "CDS Admin", Type: "to"},
			},
			"motor": {
				{Email: "admin@motor.example.com", Name: "Motor Admin", Type: "to"},
				{Email: "ops@motor.example.com", Name: "Motor Ops", Type: "to"},
			},
		},
		TemplateData: map[string]interface{}{
			"Title":   "Security Update Notification",
			"Message": "A security update has been applied to all email distribution systems.",
			"Changes": []string{
				"Enhanced encryption for email transmission",
				"Updated authentication protocols",
				"Improved spam filtering",
			},
		},
		CustomerData: map[string]map[string]interface{}{
			"hts": {
				"ActionRequired": "Please update your API keys by January 31st",
				"ActionURL":      "https://portal.hts.example.com/security/update",
			},
			"cds": {
				"ActionRequired": "No action required - update applied automatically",
			},
			"motor": {
				"ActionRequired": "Please review new security settings",
				"ActionURL":      "https://portal.motor.example.com/security/review",
			},
		},
		FromEmail: "security@emailsystem.example.com",
		FromName:  "Email System Security Team",
		Tags: map[string]string{
			"Type":     "security",
			"Priority": "high",
			"Bulk":     "true",
		},
		Priority: "high",
	}

	// Send bulk email
	fmt.Printf("   📤 Sending bulk email to %d customers...\n", len(bulkRequest.CustomerEmails))
	bulkResponse, err := sesManager.SendBulkEmail(bulkRequest)
	if err != nil {
		log.Printf("❌ Failed to send bulk email: %v", err)
		return
	}

	fmt.Printf("   ✅ Bulk email processing complete\n")
	fmt.Printf("      Total Customers: %d\n", bulkResponse.TotalCustomers)
	fmt.Printf("      Total Emails: %d\n", bulkResponse.TotalEmails)
	fmt.Printf("      Successful: %d\n", bulkResponse.SuccessCount)
	fmt.Printf("      Failed: %d\n", bulkResponse.FailureCount)
	fmt.Printf("      Processed At: %s\n", bulkResponse.ProcessedAt.Format("2006-01-02 15:04:05"))

	// Show results per customer
	fmt.Printf("\n   📊 Results by customer:\n")
	for customerCode, response := range bulkResponse.Responses {
		fmt.Printf("      ✅ %s: %s (Message ID: %s)\n",
			customerCode, response.Status, response.MessageID)
	}

	if len(bulkResponse.Errors) > 0 {
		fmt.Printf("\n   ❌ Errors:\n")
		for customerCode, errorMsg := range bulkResponse.Errors {
			fmt.Printf("      %s: %s\n", customerCode, errorMsg)
		}
	}

	// Demonstrate delivery status tracking
	fmt.Printf("\n   📋 Tracking delivery status...\n")
	for customerCode, response := range bulkResponse.Responses {
		if response.Status == "sent" {
			status, err := sesManager.GetEmailDeliveryStatus(customerCode, response.MessageID)
			if err != nil {
				fmt.Printf("      ❌ %s: Failed to get status - %v\n", customerCode, err)
				continue
			}

			fmt.Printf("      📧 %s: %s (Last event: %s)\n",
				customerCode, status["status"], status["timestamp"])
		}
	}
}

func demoEmailValidationAndStats() {
	customerManager := setupDemoCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)

	fmt.Printf("🔍 Email Validation & Statistics Demo\n")

	customers := []string{"hts", "cds", "motor"}

	// Validate emails for each customer
	fmt.Printf("   📧 Validating email addresses across customers...\n")

	testEmailSets := map[string][]string{
		"hts": {
			"admin@hts.example.com",
			"manager@hts.example.com",
			"invalid.email",
			"test@tempmail.com",
		},
		"cds": {
			"admin@cds.example.com",
			"support@cds.example.com",
			"user@10minutemail.com",
		},
		"motor": {
			"admin@motor.example.com",
			"ops@motor.example.com",
			"test@guerrillamail.com",
		},
	}

	validationSummary := make(map[string]map[string]int)

	for _, customer := range customers {
		fmt.Printf("\n      🏢 %s customer validation:\n", strings.ToUpper(customer))

		emails := testEmailSets[customer]
		results, err := sesManager.ValidateEmailAddresses(customer, emails)
		if err != nil {
			log.Printf("❌ Failed to validate emails for %s: %v", customer, err)
			continue
		}

		summary := map[string]int{
			"total":       len(results),
			"valid":       0,
			"invalid":     0,
			"deliverable": 0,
			"risky":       0,
		}

		for _, result := range results {
			status := "❌"
			if result.Valid {
				status = "✅"
				summary["valid"]++
			} else {
				summary["invalid"]++
			}

			if result.Deliverable {
				summary["deliverable"]++
			}

			if result.Risky {
				summary["risky"]++
			}

			riskFlag := ""
			if result.Risky {
				riskFlag = " ⚠️"
			}

			fmt.Printf("         %s %s%s\n", status, result.Email, riskFlag)
			if result.Reason != "" {
				fmt.Printf("            Reason: %s\n", result.Reason)
			}
		}

		validationSummary[customer] = summary

		fmt.Printf("         Summary: %d valid, %d invalid, %d risky\n",
			summary["valid"], summary["invalid"], summary["risky"])
	}

	// Get statistics for each customer
	fmt.Printf("\n   📊 Getting sending statistics for all customers...\n")
	startTime := time.Now().Add(-7 * 24 * time.Hour) // Last 7 days
	endTime := time.Now()

	for _, customer := range customers {
		fmt.Printf("\n      📈 %s Statistics (7 days):\n", strings.ToUpper(customer))

		stats, err := sesManager.GetSendingStatistics(customer, startTime, endTime)
		if err != nil {
			log.Printf("❌ Failed to get statistics for %s: %v", customer, err)
			continue
		}

		fmt.Printf("         Sent: %v | Delivered: %v | Bounced: %v | Complained: %v\n",
			stats["sent"], stats["delivered"], stats["bounced"], stats["complained"])
		fmt.Printf("         Delivery Rate: %.1f%% | Bounce Rate: %.1f%% | Complaint Rate: %.1f%%\n",
			stats["deliveryRate"], stats["bounceRate"], stats["complaintRate"])

		// Check reputation
		if reputation, ok := stats["reputationMetrics"].(map[string]interface{}); ok {
			fmt.Printf("         Reputation: %s | IP Warmup: %.0f%%\n",
				reputation["reputationStatus"], reputation["ipWarmupPercentage"])
		}
	}

	// Overall validation summary
	fmt.Printf("\n   📋 Overall Validation Summary:\n")
	totalEmails := 0
	totalValid := 0
	totalRisky := 0

	for customer, summary := range validationSummary {
		totalEmails += summary["total"]
		totalValid += summary["valid"]
		totalRisky += summary["risky"]

		validRate := float64(summary["valid"]) / float64(summary["total"]) * 100
		fmt.Printf("      %s: %.1f%% valid (%d/%d), %d risky\n",
			strings.ToUpper(customer), validRate, summary["valid"], summary["total"], summary["risky"])
	}

	overallValidRate := float64(totalValid) / float64(totalEmails) * 100
	fmt.Printf("      Overall: %.1f%% valid (%d/%d), %d risky addresses detected\n",
		overallValidRate, totalValid, totalEmails, totalRisky)
}

func demoTemplateRenderingAndCustomization() {
	customerManager := setupDemoCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)

	fmt.Printf("🎨 Template Rendering & Customization Demo\n")

	// Create a complex template with conditional content
	fmt.Printf("   📝 Creating advanced template with conditional content...\n")
	advancedTemplate := &EmailTemplate{
		ID:          "advanced-notification",
		Name:        "Advanced Notification Template",
		Description: "Template with conditional content and loops",
		Subject:     "[{{.Priority}}] {{.Title}} - {{.CustomerName}}",
		HTMLBody: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        .priority-high { border-left: 5px solid #f44336; background-color: #ffebee; }
        .priority-normal { border-left: 5px solid #ff9800; background-color: #fff3e0; }
        .priority-low { border-left: 5px solid #4caf50; background-color: #e8f5e9; }
        .change-item { margin: 10px 0; padding: 10px; background-color: #f5f5f5; border-radius: 4px; }
        .stats-table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        .stats-table th, .stats-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        .stats-table th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <div class="priority-{{.Priority}}">
        <h1>{{.Title}}</h1>
        <p><strong>Customer:</strong> {{.CustomerName}} ({{.CustomerCode}})</p>
        <p><strong>Environment:</strong> {{.Environment}}</p>
        <p><strong>Priority:</strong> {{.Priority | title}}</p>
    </div>
    
    <div style="padding: 20px;">
        <p>Hello {{.RecipientName}},</p>
        <p>{{.Message}}</p>
        
        {{if .Changes}}
        <h3>📋 Changes Made ({{len .Changes}} items):</h3>
        {{range $index, $change := .Changes}}
        <div class="change-item">
            <strong>{{add $index 1}}.</strong> {{$change}}
        </div>
        {{end}}
        {{end}}
        
        {{if .Statistics}}
        <h3>📊 Statistics:</h3>
        <table class="stats-table">
            <thead>
                <tr>
                    <th>Metric</th>
                    <th>Value</th>
                    <th>Change</th>
                </tr>
            </thead>
            <tbody>
                {{range $metric, $data := .Statistics}}
                <tr>
                    <td>{{$metric}}</td>
                    <td>{{$data.value}}</td>
                    <td style="color: {{if gt $data.change 0}}green{{else if lt $data.change 0}}red{{else}}gray{{end}};">
                        {{if gt $data.change 0}}+{{end}}{{$data.change}}
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}
        
        {{if .ActionRequired}}
        <div style="background-color: #fff3cd; border: 1px solid #ffeaa7; padding: 15px; margin: 20px 0; border-radius: 4px;">
            <h3>⚠️ Action Required:</h3>
            <p>{{.ActionRequired}}</p>
            {{if .ActionDeadline}}
            <p><strong>Deadline:</strong> {{.ActionDeadline}}</p>
            {{end}}
            {{if .ActionURL}}
            <p><a href="{{.ActionURL}}" style="background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px;">Take Action</a></p>
            {{end}}
        </div>
        {{end}}
        
        <p>Generated on {{.GeneratedAt}} by {{.ServiceName}}</p>
    </div>
</body>
</html>`,
		TextBody: `[{{.Priority}}] {{.Title}} - {{.CustomerName}}

Customer: {{.CustomerName}} ({{.CustomerCode}})
Environment: {{.Environment}}
Priority: {{.Priority}}

Hello {{.RecipientName}},

{{.Message}}

{{if .Changes}}
Changes Made ({{len .Changes}} items):
{{range $index, $change := .Changes}}
{{add $index 1}}. {{$change}}
{{end}}
{{end}}

{{if .Statistics}}
Statistics:
{{range $metric, $data := .Statistics}}
- {{$metric}}: {{$data.value}} ({{if gt $data.change 0}}+{{end}}{{$data.change}})
{{end}}
{{end}}

{{if .ActionRequired}}
⚠️ Action Required:
{{.ActionRequired}}
{{if .ActionDeadline}}
Deadline: {{.ActionDeadline}}
{{end}}
{{if .ActionURL}}
Action URL: {{.ActionURL}}
{{end}}
{{end}}

Generated on {{.GeneratedAt}} by {{.ServiceName}}`,
		Variables: []TemplateVariable{
			{Name: "Title", Type: "string", Required: true},
			{Name: "Priority", Type: "string", Required: true},
			{Name: "RecipientName", Type: "string", Required: false},
			{Name: "Message", Type: "string", Required: true},
			{Name: "Changes", Type: "array", Required: false},
			{Name: "Statistics", Type: "object", Required: false},
			{Name: "ActionRequired", Type: "string", Required: false},
			{Name: "ActionDeadline", Type: "string", Required: false},
			{Name: "ActionURL", Type: "string", Required: false},
		},
	}

	err := templateManager.CreateTemplate(advancedTemplate)
	if err != nil {
		log.Printf("❌ Failed to create advanced template: %v", err)
		return
	}

	fmt.Printf("   ✅ Advanced template created\n")

	// Render template with complex data
	fmt.Printf("   🎨 Rendering template with complex data...\n")

	templateData := &EmailTemplateData{
		Variables: map[string]interface{}{
			"Title":         "Monthly Email Distribution Report",
			"Priority":      "normal",
			"RecipientName": "Sarah Johnson",
			"Message":       "Here's your monthly summary of email distribution activities and performance metrics.",
			"Changes": []string{
				"Added 25 new recipients to the HTS distribution list",
				"Updated email templates with new branding guidelines",
				"Implemented enhanced spam filtering",
				"Migrated 3 customer accounts to new infrastructure",
				"Optimized delivery performance for high-volume campaigns",
			},
			"Statistics": map[string]interface{}{
				"Total Emails Sent": map[string]interface{}{
					"value":  "12,450",
					"change": 1250,
				},
				"Delivery Rate": map[string]interface{}{
					"value":  "98.2%",
					"change": 0.5,
				},
				"Bounce Rate": map[string]interface{}{
					"value":  "1.1%",
					"change": -0.3,
				},
				"Open Rate": map[string]interface{}{
					"value":  "24.7%",
					"change": 2.1,
				},
				"Click Rate": map[string]interface{}{
					"value":  "3.8%",
					"change": 0.2,
				},
			},
			"ActionRequired": "Please review the new compliance requirements and update your email preferences",
			"ActionDeadline": "February 15, 2024",
			"ActionURL":      "https://portal.example.com/compliance/review",
		},
		CustomerCode: "hts",
		Metadata: map[string]interface{}{
			"reportType": "monthly",
			"reportId":   "RPT-2024-01",
		},
	}

	renderedEmail, err := templateManager.RenderTemplate("advanced-notification", "hts", templateData)
	if err != nil {
		log.Printf("❌ Failed to render template: %v", err)
		return
	}

	fmt.Printf("   ✅ Template rendered successfully\n")
	fmt.Printf("      Subject: %s\n", renderedEmail.Subject)
	fmt.Printf("      HTML Body: %d characters\n", len(renderedEmail.HTMLBody))
	fmt.Printf("      Text Body: %d characters\n", len(renderedEmail.TextBody))
	fmt.Printf("      Rendered At: %s\n", renderedEmail.RenderedAt.Format("2006-01-02 15:04:05"))

	// Show template variables extraction
	fmt.Printf("\n   🔍 Extracting variables from template...\n")
	variables, err := templateManager.GetTemplateVariables(advancedTemplate.Subject + " " + advancedTemplate.HTMLBody)
	if err != nil {
		log.Printf("❌ Failed to extract variables: %v", err)
		return
	}

	fmt.Printf("   📋 Found %d unique variables:\n", len(variables))
	for i, variable := range variables {
		fmt.Printf("      %d. {{.%s}}\n", i+1, variable)
	}

	// Demonstrate customer-specific template override
	fmt.Printf("\n   🎯 Creating customer-specific template override...\n")
	htsOverride := &EmailTemplate{
		ID:           "advanced-notification",
		Name:         "HTS Advanced Notification",
		Description:  "HTS-specific version with custom branding",
		Subject:      "[HTS-{{.Priority}}] {{.Title}}",
		HTMLBody:     `<div style="background: linear-gradient(135deg, #1e3a8a 0%, #3b82f6 100%); color: white; padding: 20px;"><h1>🏢 HTS Production System</h1><h2>{{.Title}}</h2></div>` + advancedTemplate.HTMLBody,
		TextBody:     "HTS Production System\n" + advancedTemplate.TextBody,
		CustomerCode: "hts",
		Variables:    advancedTemplate.Variables,
	}

	err = templateManager.CreateTemplate(htsOverride)
	if err != nil {
		log.Printf("❌ Failed to create HTS override: %v", err)
		return
	}

	fmt.Printf("   ✅ HTS-specific template created\n")

	// Render with customer-specific template
	fmt.Printf("   🎨 Rendering with HTS-specific template...\n")
	htsRendered, err := templateManager.RenderTemplate("advanced-notification", "hts", templateData)
	if err != nil {
		log.Printf("❌ Failed to render HTS template: %v", err)
		return
	}

	fmt.Printf("   ✅ HTS template rendered\n")
	fmt.Printf("      Subject: %s\n", htsRendered.Subject)
	fmt.Printf("      Contains HTS branding: %t\n",
		strings.Contains(htsRendered.HTMLBody, "HTS Production System"))

	// Compare template sizes
	fmt.Printf("\n   📏 Template comparison:\n")
	fmt.Printf("      Global template HTML: %d chars\n", len(renderedEmail.HTMLBody))
	fmt.Printf("      HTS template HTML: %d chars\n", len(htsRendered.HTMLBody))
	fmt.Printf("      Size difference: %d chars\n", len(htsRendered.HTMLBody)-len(renderedEmail.HTMLBody))
}

// Helper function to set up demo customer manager
func setupDemoCustomerManager() *CustomerCredentialManager {
	customerManager := NewCustomerCredentialManager("us-east-1")

	// Add demo customers
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/HTSSQSRole",
			S3RoleARN:    "arn:aws:iam::123456789012:role/HTSS3Role",
			Environment:  "production",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Global",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			SQSRoleARN:   "arn:aws:iam::234567890123:role/CDSSQSRole",
			Environment:  "production",
		},
		"motor": {
			CustomerCode: "motor",
			CustomerName: "Motor",
			AWSAccountID: "345678901234",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::345678901234:role/MotorSESRole",
			Environment:  "staging",
		},
	}

	return customerManager
}
