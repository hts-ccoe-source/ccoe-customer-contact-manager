package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
)

// EmailTemplateManager handles email template operations for multi-customer environments
type EmailTemplateManager struct {
	Templates        map[string]*EmailTemplate
	DefaultTemplates map[string]*EmailTemplate
	CustomerManager  *CustomerCredentialManager
}

// EmailTemplate represents an email template with metadata
type EmailTemplate struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Subject      string                 `json:"subject"`
	HTMLBody     string                 `json:"htmlBody"`
	TextBody     string                 `json:"textBody"`
	Variables    []TemplateVariable     `json:"variables"`
	CustomerCode string                 `json:"customerCode,omitempty"`
	IsDefault    bool                   `json:"isDefault"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
	Version      int                    `json:"version"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateVariable defines a variable that can be used in templates
type TemplateVariable struct {
	Name         string `json:"name"`
	Type         string `json:"type"` // string, number, date, boolean, array
	Description  string `json:"description"`
	Required     bool   `json:"required"`
	DefaultValue string `json:"defaultValue,omitempty"`
	Example      string `json:"example,omitempty"`
}

// EmailTemplateData contains data for template rendering
type EmailTemplateData struct {
	Variables    map[string]interface{} `json:"variables"`
	CustomerCode string                 `json:"customerCode"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	SystemData   map[string]interface{} `json:"systemData,omitempty"`
}

// RenderedEmail represents a rendered email ready for sending
type RenderedEmail struct {
	Subject    string                 `json:"subject"`
	HTMLBody   string                 `json:"htmlBody"`
	TextBody   string                 `json:"textBody"`
	TemplateID string                 `json:"templateId"`
	Variables  map[string]interface{} `json:"variables"`
	RenderedAt time.Time              `json:"renderedAt"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NewEmailTemplateManager creates a new email template manager
func NewEmailTemplateManager(customerManager *CustomerCredentialManager) *EmailTemplateManager {
	manager := &EmailTemplateManager{
		Templates:        make(map[string]*EmailTemplate),
		DefaultTemplates: make(map[string]*EmailTemplate),
		CustomerManager:  customerManager,
	}

	// Load default templates
	manager.loadDefaultTemplates()

	return manager
}

// loadDefaultTemplates loads system default email templates
func (etm *EmailTemplateManager) loadDefaultTemplates() {
	// Default notification template
	notificationTemplate := &EmailTemplate{
		ID:          "default-notification",
		Name:        "Default Notification",
		Description: "Standard notification template for email distribution changes",
		Subject:     "{{.Title}} - Email Distribution Update",
		HTMLBody: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .header { background-color: #f4f4f4; padding: 20px; text-align: center; }
        .content { padding: 20px; }
        .footer { background-color: #f4f4f4; padding: 10px; text-align: center; font-size: 12px; }
        .highlight { background-color: #fff3cd; padding: 10px; border-left: 4px solid #ffc107; }
        .button { display: inline-block; padding: 10px 20px; background-color: #007bff; color: white; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Title}}</h1>
    </div>
    <div class="content">
        <p>Hello,</p>
        <p>{{.Message}}</p>
        
        {{if .Changes}}
        <div class="highlight">
            <h3>Changes Summary:</h3>
            <ul>
            {{range .Changes}}
                <li>{{.}}</li>
            {{end}}
            </ul>
        </div>
        {{end}}
        
        {{if .ActionRequired}}
        <p><strong>Action Required:</strong> {{.ActionRequired}}</p>
        {{end}}
        
        {{if .ActionURL}}
        <p><a href="{{.ActionURL}}" class="button">Take Action</a></p>
        {{end}}
        
        <p>If you have any questions, please contact your system administrator.</p>
    </div>
    <div class="footer">
        <p>This is an automated message from the Email Distribution System.</p>
        <p>Generated on {{.GeneratedAt}} for customer {{.CustomerCode}}</p>
    </div>
</body>
</html>`,
		TextBody: `{{.Title}}

Hello,

{{.Message}}

{{if .Changes}}
Changes Summary:
{{range .Changes}}
- {{.}}
{{end}}
{{end}}

{{if .ActionRequired}}
Action Required: {{.ActionRequired}}
{{end}}

{{if .ActionURL}}
Action URL: {{.ActionURL}}
{{end}}

If you have any questions, please contact your system administrator.

---
This is an automated message from the Email Distribution System.
Generated on {{.GeneratedAt}} for customer {{.CustomerCode}}`,
		Variables: []TemplateVariable{
			{Name: "Title", Type: "string", Description: "Email title/subject", Required: true, Example: "Email List Update"},
			{Name: "Message", Type: "string", Description: "Main message content", Required: true, Example: "Your email list has been updated."},
			{Name: "Changes", Type: "array", Description: "List of changes made", Required: false, Example: "Added 5 new recipients"},
			{Name: "ActionRequired", Type: "string", Description: "Action required from recipient", Required: false, Example: "Please review the changes"},
			{Name: "ActionURL", Type: "string", Description: "URL for taking action", Required: false, Example: "https://portal.example.com/review"},
			{Name: "CustomerCode", Type: "string", Description: "Customer code", Required: true, Example: "hts"},
			{Name: "GeneratedAt", Type: "string", Description: "Generation timestamp", Required: true, Example: "2024-01-15 10:30:00"},
		},
		IsDefault: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	// Default welcome template
	welcomeTemplate := &EmailTemplate{
		ID:          "default-welcome",
		Name:        "Welcome Template",
		Description: "Welcome template for new email distribution setup",
		Subject:     "Welcome to {{.ServiceName}} - {{.CustomerName}}",
		HTMLBody: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Welcome to {{.ServiceName}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .header { background-color: #28a745; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; }
        .footer { background-color: #f4f4f4; padding: 10px; text-align: center; font-size: 12px; }
        .info-box { background-color: #e9ecef; padding: 15px; border-radius: 4px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Welcome to {{.ServiceName}}</h1>
    </div>
    <div class="content">
        <p>Dear {{.CustomerName}} Team,</p>
        <p>Welcome to our email distribution system! Your account has been successfully set up.</p>
        
        <div class="info-box">
            <h3>Account Information:</h3>
            <ul>
                <li><strong>Customer Code:</strong> {{.CustomerCode}}</li>
                <li><strong>Environment:</strong> {{.Environment}}</li>
                <li><strong>Setup Date:</strong> {{.SetupDate}}</li>
            </ul>
        </div>
        
        <p>You can now start using the email distribution system to manage your email communications.</p>
        
        {{if .ContactEmail}}
        <p>If you need assistance, please contact us at: <a href="mailto:{{.ContactEmail}}">{{.ContactEmail}}</a></p>
        {{end}}
    </div>
    <div class="footer">
        <p>Thank you for choosing our email distribution service.</p>
    </div>
</body>
</html>`,
		TextBody: `Welcome to {{.ServiceName}}

Dear {{.CustomerName}} Team,

Welcome to our email distribution system! Your account has been successfully set up.

Account Information:
- Customer Code: {{.CustomerCode}}
- Environment: {{.Environment}}
- Setup Date: {{.SetupDate}}

You can now start using the email distribution system to manage your email communications.

{{if .ContactEmail}}
If you need assistance, please contact us at: {{.ContactEmail}}
{{end}}

Thank you for choosing our email distribution service.`,
		Variables: []TemplateVariable{
			{Name: "ServiceName", Type: "string", Description: "Name of the service", Required: true, Example: "Email Distribution System"},
			{Name: "CustomerName", Type: "string", Description: "Customer organization name", Required: true, Example: "HTS Production"},
			{Name: "CustomerCode", Type: "string", Description: "Customer code", Required: true, Example: "hts"},
			{Name: "Environment", Type: "string", Description: "Environment type", Required: true, Example: "production"},
			{Name: "SetupDate", Type: "string", Description: "Account setup date", Required: true, Example: "2024-01-15"},
			{Name: "ContactEmail", Type: "string", Description: "Support contact email", Required: false, Example: "support@example.com"},
		},
		IsDefault: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	// Default error template
	errorTemplate := &EmailTemplate{
		ID:          "default-error",
		Name:        "Error Notification",
		Description: "Template for error notifications",
		Subject:     "Error: {{.ErrorType}} - {{.CustomerCode}}",
		HTMLBody: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Error Notification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .header { background-color: #dc3545; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; }
        .footer { background-color: #f4f4f4; padding: 10px; text-align: center; font-size: 12px; }
        .error-box { background-color: #f8d7da; border: 1px solid #f5c6cb; padding: 15px; border-radius: 4px; margin: 10px 0; }
        .code { background-color: #f8f9fa; padding: 10px; border-radius: 4px; font-family: monospace; }
    </style>
</head>
<body>
    <div class="header">
        <h1>⚠️ Error Notification</h1>
    </div>
    <div class="content">
        <p>An error has occurred in the email distribution system.</p>
        
        <div class="error-box">
            <h3>Error Details:</h3>
            <ul>
                <li><strong>Error Type:</strong> {{.ErrorType}}</li>
                <li><strong>Customer:</strong> {{.CustomerCode}}</li>
                <li><strong>Timestamp:</strong> {{.ErrorTimestamp}}</li>
                {{if .ErrorID}}<li><strong>Error ID:</strong> {{.ErrorID}}</li>{{end}}
            </ul>
        </div>
        
        {{if .ErrorMessage}}
        <p><strong>Error Message:</strong></p>
        <div class="code">{{.ErrorMessage}}</div>
        {{end}}
        
        {{if .Resolution}}
        <p><strong>Recommended Action:</strong> {{.Resolution}}</p>
        {{end}}
        
        <p>Please investigate this issue and take appropriate action.</p>
    </div>
    <div class="footer">
        <p>This is an automated error notification from the Email Distribution System.</p>
    </div>
</body>
</html>`,
		TextBody: `⚠️ Error Notification

An error has occurred in the email distribution system.

Error Details:
- Error Type: {{.ErrorType}}
- Customer: {{.CustomerCode}}
- Timestamp: {{.ErrorTimestamp}}
{{if .ErrorID}}- Error ID: {{.ErrorID}}{{end}}

{{if .ErrorMessage}}
Error Message:
{{.ErrorMessage}}
{{end}}

{{if .Resolution}}
Recommended Action: {{.Resolution}}
{{end}}

Please investigate this issue and take appropriate action.

---
This is an automated error notification from the Email Distribution System.`,
		Variables: []TemplateVariable{
			{Name: "ErrorType", Type: "string", Description: "Type of error", Required: true, Example: "SQS Processing Error"},
			{Name: "CustomerCode", Type: "string", Description: "Customer code", Required: true, Example: "hts"},
			{Name: "ErrorTimestamp", Type: "string", Description: "When the error occurred", Required: true, Example: "2024-01-15 10:30:00"},
			{Name: "ErrorID", Type: "string", Description: "Unique error identifier", Required: false, Example: "ERR-2024-001"},
			{Name: "ErrorMessage", Type: "string", Description: "Detailed error message", Required: false, Example: "Failed to process SQS message"},
			{Name: "Resolution", Type: "string", Description: "Recommended resolution", Required: false, Example: "Check SQS queue permissions"},
		},
		IsDefault: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	etm.DefaultTemplates["notification"] = notificationTemplate
	etm.DefaultTemplates["welcome"] = welcomeTemplate
	etm.DefaultTemplates["error"] = errorTemplate
}

// GetTemplate retrieves a template by ID, checking customer-specific templates first
func (etm *EmailTemplateManager) GetTemplate(templateID, customerCode string) (*EmailTemplate, error) {
	// Check for customer-specific template first
	if customerCode != "" {
		customerTemplateID := fmt.Sprintf("%s-%s", customerCode, templateID)
		if template, exists := etm.Templates[customerTemplateID]; exists {
			return template, nil
		}
	}

	// Check global templates
	if template, exists := etm.Templates[templateID]; exists {
		return template, nil
	}

	// Check default templates
	if template, exists := etm.DefaultTemplates[templateID]; exists {
		return template, nil
	}

	return nil, fmt.Errorf("template not found: %s", templateID)
}

// CreateTemplate creates a new email template
func (etm *EmailTemplateManager) CreateTemplate(template *EmailTemplate) error {
	if template.ID == "" {
		return fmt.Errorf("template ID is required")
	}

	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if template.Subject == "" {
		return fmt.Errorf("template subject is required")
	}

	if template.HTMLBody == "" && template.TextBody == "" {
		return fmt.Errorf("template must have either HTML or text body")
	}

	// Validate template syntax
	if err := etm.validateTemplateSyntax(template); err != nil {
		return fmt.Errorf("template syntax validation failed: %v", err)
	}

	// Set metadata
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	template.Version = 1

	// Generate full template ID for customer-specific templates
	fullID := template.ID
	if template.CustomerCode != "" {
		fullID = fmt.Sprintf("%s-%s", template.CustomerCode, template.ID)
	}

	etm.Templates[fullID] = template

	return nil
}

// UpdateTemplate updates an existing email template
func (etm *EmailTemplateManager) UpdateTemplate(templateID string, updates *EmailTemplate) error {
	template, exists := etm.Templates[templateID]
	if !exists {
		return fmt.Errorf("template not found: %s", templateID)
	}

	// Validate template syntax if body is being updated
	if updates.HTMLBody != "" || updates.TextBody != "" {
		if err := etm.validateTemplateSyntax(updates); err != nil {
			return fmt.Errorf("template syntax validation failed: %v", err)
		}
	}

	// Update fields
	if updates.Name != "" {
		template.Name = updates.Name
	}
	if updates.Description != "" {
		template.Description = updates.Description
	}
	if updates.Subject != "" {
		template.Subject = updates.Subject
	}
	if updates.HTMLBody != "" {
		template.HTMLBody = updates.HTMLBody
	}
	if updates.TextBody != "" {
		template.TextBody = updates.TextBody
	}
	if updates.Variables != nil {
		template.Variables = updates.Variables
	}
	if updates.Metadata != nil {
		template.Metadata = updates.Metadata
	}

	template.UpdatedAt = time.Now()
	template.Version++

	return nil
}

// DeleteTemplate deletes an email template
func (etm *EmailTemplateManager) DeleteTemplate(templateID string) error {
	if _, exists := etm.Templates[templateID]; !exists {
		return fmt.Errorf("template not found: %s", templateID)
	}

	delete(etm.Templates, templateID)
	return nil
}

// ListTemplates returns all templates, optionally filtered by customer
func (etm *EmailTemplateManager) ListTemplates(customerCode string) []*EmailTemplate {
	var templates []*EmailTemplate

	// Add default templates
	for _, template := range etm.DefaultTemplates {
		templates = append(templates, template)
	}

	// Add global templates
	for _, template := range etm.Templates {
		if template.CustomerCode == "" {
			templates = append(templates, template)
		}
	}

	// Add customer-specific templates if customer code provided
	if customerCode != "" {
		for _, template := range etm.Templates {
			if template.CustomerCode == customerCode {
				templates = append(templates, template)
			}
		}
	}

	return templates
}

// RenderTemplate renders a template with provided data
func (etm *EmailTemplateManager) RenderTemplate(templateID, customerCode string, data *EmailTemplateData) (*RenderedEmail, error) {
	emailTemplate, err := etm.GetTemplate(templateID, customerCode)
	if err != nil {
		return nil, err
	}

	// Validate required variables
	if err := etm.validateTemplateData(emailTemplate, data); err != nil {
		return nil, fmt.Errorf("template data validation failed: %v", err)
	}

	// Prepare template data
	templateData := etm.prepareTemplateData(data, customerCode)

	// Render subject
	subject, err := etm.renderTemplateString(emailTemplate.Subject, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render subject: %v", err)
	}

	// Render HTML body
	var htmlBody string
	if emailTemplate.HTMLBody != "" {
		htmlBody, err = etm.renderTemplateString(emailTemplate.HTMLBody, templateData)
		if err != nil {
			return nil, fmt.Errorf("failed to render HTML body: %v", err)
		}
	}

	// Render text body
	var textBody string
	if emailTemplate.TextBody != "" {
		textBody, err = etm.renderTemplateString(emailTemplate.TextBody, templateData)
		if err != nil {
			return nil, fmt.Errorf("failed to render text body: %v", err)
		}
	}

	return &RenderedEmail{
		Subject:    subject,
		HTMLBody:   htmlBody,
		TextBody:   textBody,
		TemplateID: templateID,
		Variables:  data.Variables,
		RenderedAt: time.Now(),
		Metadata:   data.Metadata,
	}, nil
}

// validateTemplateSyntax validates Go template syntax
func (etm *EmailTemplateManager) validateTemplateSyntax(emailTemplate *EmailTemplate) error {
	// Validate subject template
	if emailTemplate.Subject != "" {
		if _, err := template.New("subject").Parse(emailTemplate.Subject); err != nil {
			return fmt.Errorf("invalid subject template syntax: %v", err)
		}
	}

	// Validate HTML body template
	if emailTemplate.HTMLBody != "" {
		if _, err := template.New("html").Parse(emailTemplate.HTMLBody); err != nil {
			return fmt.Errorf("invalid HTML body template syntax: %v", err)
		}
	}

	// Validate text body template
	if emailTemplate.TextBody != "" {
		if _, err := template.New("text").Parse(emailTemplate.TextBody); err != nil {
			return fmt.Errorf("invalid text body template syntax: %v", err)
		}
	}

	return nil
}

// validateTemplateData validates that required template variables are provided
func (etm *EmailTemplateManager) validateTemplateData(emailTemplate *EmailTemplate, data *EmailTemplateData) error {
	for _, variable := range emailTemplate.Variables {
		if variable.Required {
			if data.Variables == nil {
				return fmt.Errorf("required variable '%s' not provided", variable.Name)
			}

			if _, exists := data.Variables[variable.Name]; !exists {
				return fmt.Errorf("required variable '%s' not provided", variable.Name)
			}
		}
	}

	return nil
}

// prepareTemplateData prepares data for template rendering
func (etm *EmailTemplateManager) prepareTemplateData(data *EmailTemplateData, customerCode string) map[string]interface{} {
	templateData := make(map[string]interface{})

	// Add user variables
	if data.Variables != nil {
		for key, value := range data.Variables {
			templateData[key] = value
		}
	}

	// Add system data
	templateData["CustomerCode"] = customerCode
	templateData["GeneratedAt"] = time.Now().Format("2006-01-02 15:04:05")

	// Add customer information if available
	if etm.CustomerManager != nil && customerCode != "" {
		if accountInfo, err := etm.CustomerManager.GetCustomerAccountInfo(customerCode); err == nil {
			templateData["CustomerName"] = accountInfo.CustomerName
			templateData["Environment"] = accountInfo.Environment
			templateData["Region"] = accountInfo.Region
		}
	}

	// Add system data
	if data.SystemData != nil {
		for key, value := range data.SystemData {
			templateData[key] = value
		}
	}

	// Add metadata
	if data.Metadata != nil {
		templateData["Metadata"] = data.Metadata
	}

	return templateData
}

// renderTemplateString renders a template string with data
func (etm *EmailTemplateManager) renderTemplateString(templateStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", err
	}

	return result.String(), nil
}

// GetTemplateVariables extracts variables used in a template
func (etm *EmailTemplateManager) GetTemplateVariables(templateStr string) ([]string, error) {
	// Simple regex to find template variables
	re := regexp.MustCompile(`\{\{\.(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(templateStr, -1)

	var variables []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			variable := match[1]
			if !seen[variable] {
				variables = append(variables, variable)
				seen[variable] = true
			}
		}
	}

	return variables, nil
}

// ExportTemplate exports a template to JSON
func (etm *EmailTemplateManager) ExportTemplate(templateID string) ([]byte, error) {
	template, exists := etm.Templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	return json.MarshalIndent(template, "", "  ")
}

// ImportTemplate imports a template from JSON
func (etm *EmailTemplateManager) ImportTemplate(jsonData []byte) error {
	var template EmailTemplate
	if err := json.Unmarshal(jsonData, &template); err != nil {
		return fmt.Errorf("failed to parse template JSON: %v", err)
	}

	return etm.CreateTemplate(&template)
}

// ValidateTemplateID validates template ID format
func (etm *EmailTemplateManager) ValidateTemplateID(templateID string) error {
	if templateID == "" {
		return fmt.Errorf("template ID cannot be empty")
	}

	// Template ID should be alphanumeric with hyphens and underscores
	re := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !re.MatchString(templateID) {
		return fmt.Errorf("template ID must contain only alphanumeric characters, hyphens, and underscores")
	}

	if len(templateID) > 100 {
		return fmt.Errorf("template ID must be 100 characters or less")
	}

	return nil
}
