package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewEmailTemplateManager(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	manager := NewEmailTemplateManager(customerManager)

	if manager.CustomerManager != customerManager {
		t.Errorf("Expected customer manager to be set")
	}

	if len(manager.DefaultTemplates) == 0 {
		t.Errorf("Expected default templates to be loaded")
	}

	// Check that default templates are loaded
	expectedTemplates := []string{"notification", "welcome", "error"}
	for _, templateID := range expectedTemplates {
		if _, exists := manager.DefaultTemplates[templateID]; !exists {
			t.Errorf("Expected default template '%s' to be loaded", templateID)
		}
	}
}

func TestCreateTemplate(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Test valid template creation
	template := &EmailTemplate{
		ID:          "test-template",
		Name:        "Test Template",
		Description: "A test template",
		Subject:     "Test Subject: {{.Title}}",
		HTMLBody:    "<h1>{{.Title}}</h1><p>{{.Message}}</p>",
		TextBody:    "{{.Title}}\n\n{{.Message}}",
		Variables: []TemplateVariable{
			{Name: "Title", Type: "string", Required: true},
			{Name: "Message", Type: "string", Required: true},
		},
	}

	err := manager.CreateTemplate(template)
	if err != nil {
		t.Fatalf("Failed to create valid template: %v", err)
	}

	// Verify template was created
	if _, exists := manager.Templates["test-template"]; !exists {
		t.Errorf("Template was not stored")
	}

	// Test template with customer code
	customerTemplate := &EmailTemplate{
		ID:           "customer-template",
		Name:         "Customer Template",
		Subject:      "Customer: {{.Title}}",
		HTMLBody:     "<p>{{.Message}}</p>",
		CustomerCode: "hts",
	}

	err = manager.CreateTemplate(customerTemplate)
	if err != nil {
		t.Fatalf("Failed to create customer template: %v", err)
	}

	// Verify customer template was stored with full ID
	if _, exists := manager.Templates["hts-customer-template"]; !exists {
		t.Errorf("Customer template was not stored with correct ID")
	}

	// Test invalid templates
	invalidTemplates := []*EmailTemplate{
		{Name: "No ID", Subject: "Test", HTMLBody: "Test"},                    // Missing ID
		{ID: "no-name", Subject: "Test", HTMLBody: "Test"},                    // Missing name
		{ID: "no-subject", Name: "Test", HTMLBody: "Test"},                    // Missing subject
		{ID: "no-body", Name: "Test", Subject: "Test"},                        // Missing body
		{ID: "bad-syntax", Name: "Test", Subject: "{{.Bad", HTMLBody: "Test"}, // Bad syntax
	}

	for i, invalidTemplate := range invalidTemplates {
		err = manager.CreateTemplate(invalidTemplate)
		if err == nil {
			t.Errorf("Expected error for invalid template %d, got none", i)
		}
	}
}

func TestGetTemplate(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Create test templates
	globalTemplate := &EmailTemplate{
		ID:       "global-template",
		Name:     "Global Template",
		Subject:  "Global: {{.Title}}",
		HTMLBody: "<p>{{.Message}}</p>",
	}

	customerTemplate := &EmailTemplate{
		ID:           "customer-template",
		Name:         "Customer Template",
		Subject:      "Customer: {{.Title}}",
		HTMLBody:     "<p>{{.Message}}</p>",
		CustomerCode: "hts",
	}

	manager.CreateTemplate(globalTemplate)
	manager.CreateTemplate(customerTemplate)

	// Test getting global template
	template, err := manager.GetTemplate("global-template", "")
	if err != nil {
		t.Fatalf("Failed to get global template: %v", err)
	}
	if template.ID != "global-template" {
		t.Errorf("Expected template ID 'global-template', got '%s'", template.ID)
	}

	// Test getting customer-specific template
	template, err = manager.GetTemplate("customer-template", "hts")
	if err != nil {
		t.Fatalf("Failed to get customer template: %v", err)
	}
	if template.CustomerCode != "hts" {
		t.Errorf("Expected customer code 'hts', got '%s'", template.CustomerCode)
	}

	// Test getting default template
	template, err = manager.GetTemplate("notification", "")
	if err != nil {
		t.Fatalf("Failed to get default template: %v", err)
	}
	if template.ID != "default-notification" {
		t.Errorf("Expected template ID 'default-notification', got '%s'", template.ID)
	}

	// Test customer-specific template takes precedence
	globalWithSameID := &EmailTemplate{
		ID:       "same-id",
		Name:     "Global Same ID",
		Subject:  "Global",
		HTMLBody: "<p>Global</p>",
	}

	customerWithSameID := &EmailTemplate{
		ID:           "same-id",
		Name:         "Customer Same ID",
		Subject:      "Customer",
		HTMLBody:     "<p>Customer</p>",
		CustomerCode: "hts",
	}

	manager.CreateTemplate(globalWithSameID)
	manager.CreateTemplate(customerWithSameID)

	template, err = manager.GetTemplate("same-id", "hts")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}
	if template.Subject != "Customer" {
		t.Errorf("Expected customer template to take precedence, got subject '%s'", template.Subject)
	}

	// Test non-existent template
	_, err = manager.GetTemplate("non-existent", "")
	if err == nil {
		t.Errorf("Expected error for non-existent template, got none")
	}
}

func TestUpdateTemplate(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Create initial template
	template := &EmailTemplate{
		ID:       "update-test",
		Name:     "Original Name",
		Subject:  "Original Subject",
		HTMLBody: "<p>Original</p>",
	}

	err := manager.CreateTemplate(template)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	originalVersion := manager.Templates["update-test"].Version
	originalUpdatedAt := manager.Templates["update-test"].UpdatedAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update template
	updates := &EmailTemplate{
		Name:     "Updated Name",
		Subject:  "Updated Subject: {{.Title}}",
		HTMLBody: "<h1>{{.Title}}</h1><p>Updated</p>",
		Variables: []TemplateVariable{
			{Name: "Title", Type: "string", Required: true},
		},
	}

	err = manager.UpdateTemplate("update-test", updates)
	if err != nil {
		t.Fatalf("Failed to update template: %v", err)
	}

	// Verify updates
	updatedTemplate := manager.Templates["update-test"]
	if updatedTemplate.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", updatedTemplate.Name)
	}
	if updatedTemplate.Subject != "Updated Subject: {{.Title}}" {
		t.Errorf("Expected updated subject, got '%s'", updatedTemplate.Subject)
	}
	if updatedTemplate.Version != originalVersion+1 {
		t.Errorf("Expected version to increment, got %d", updatedTemplate.Version)
	}
	if !updatedTemplate.UpdatedAt.After(originalUpdatedAt) {
		t.Errorf("Expected UpdatedAt to be updated")
	}

	// Test updating non-existent template
	err = manager.UpdateTemplate("non-existent", updates)
	if err == nil {
		t.Errorf("Expected error for non-existent template, got none")
	}

	// Test invalid syntax update
	invalidUpdates := &EmailTemplate{
		Subject: "{{.Bad",
	}

	err = manager.UpdateTemplate("update-test", invalidUpdates)
	if err == nil {
		t.Errorf("Expected error for invalid syntax, got none")
	}
}

func TestDeleteTemplate(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Create template
	template := &EmailTemplate{
		ID:       "delete-test",
		Name:     "Delete Test",
		Subject:  "Test",
		HTMLBody: "<p>Test</p>",
	}

	err := manager.CreateTemplate(template)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Verify template exists
	if _, exists := manager.Templates["delete-test"]; !exists {
		t.Errorf("Template should exist before deletion")
	}

	// Delete template
	err = manager.DeleteTemplate("delete-test")
	if err != nil {
		t.Fatalf("Failed to delete template: %v", err)
	}

	// Verify template is deleted
	if _, exists := manager.Templates["delete-test"]; exists {
		t.Errorf("Template should not exist after deletion")
	}

	// Test deleting non-existent template
	err = manager.DeleteTemplate("non-existent")
	if err == nil {
		t.Errorf("Expected error for non-existent template, got none")
	}
}

func TestListTemplates(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Create test templates
	globalTemplate := &EmailTemplate{
		ID:       "global",
		Name:     "Global Template",
		Subject:  "Global",
		HTMLBody: "<p>Global</p>",
	}

	htsTemplate := &EmailTemplate{
		ID:           "hts-specific",
		Name:         "HTS Template",
		Subject:      "HTS",
		HTMLBody:     "<p>HTS</p>",
		CustomerCode: "hts",
	}

	cdsTemplate := &EmailTemplate{
		ID:           "cds-specific",
		Name:         "CDS Template",
		Subject:      "CDS",
		HTMLBody:     "<p>CDS</p>",
		CustomerCode: "cds",
	}

	manager.CreateTemplate(globalTemplate)
	manager.CreateTemplate(htsTemplate)
	manager.CreateTemplate(cdsTemplate)

	// Test listing all templates (no customer filter)
	allTemplates := manager.ListTemplates("")
	if len(allTemplates) < 4 { // 3 default + 1 global
		t.Errorf("Expected at least 4 templates, got %d", len(allTemplates))
	}

	// Test listing templates for specific customer
	htsTemplates := manager.ListTemplates("hts")
	if len(htsTemplates) < 5 { // 3 default + 1 global + 1 hts-specific
		t.Errorf("Expected at least 5 templates for HTS, got %d", len(htsTemplates))
	}

	// Verify HTS-specific template is included
	htsFound := false
	for _, template := range htsTemplates {
		if template.CustomerCode == "hts" {
			htsFound = true
			break
		}
	}
	if !htsFound {
		t.Errorf("HTS-specific template not found in HTS template list")
	}

	// Verify CDS-specific template is not included in HTS list
	cdsFound := false
	for _, template := range htsTemplates {
		if template.CustomerCode == "cds" {
			cdsFound = true
			break
		}
	}
	if cdsFound {
		t.Errorf("CDS-specific template should not be in HTS template list")
	}
}

func TestRenderTemplate(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")

	// Add test customer
	customerManager.CustomerMappings["hts"] = CustomerAccountInfo{
		CustomerCode: "hts",
		CustomerName: "HTS Production",
		Environment:  "production",
		Region:       "us-east-1",
	}

	manager := NewEmailTemplateManager(customerManager)

	// Create test template
	template := &EmailTemplate{
		ID:      "render-test",
		Name:    "Render Test",
		Subject: "Hello {{.Name}} - {{.Title}}",
		HTMLBody: `<html>
<body>
<h1>{{.Title}}</h1>
<p>Hello {{.Name}},</p>
<p>{{.Message}}</p>
<p>Customer: {{.CustomerName}} ({{.CustomerCode}})</p>
<p>Generated: {{.GeneratedAt}}</p>
</body>
</html>`,
		TextBody: `{{.Title}}

Hello {{.Name}},

{{.Message}}

Customer: {{.CustomerName}} ({{.CustomerCode}})
Generated: {{.GeneratedAt}}`,
		Variables: []TemplateVariable{
			{Name: "Name", Type: "string", Required: true},
			{Name: "Title", Type: "string", Required: true},
			{Name: "Message", Type: "string", Required: true},
		},
	}

	err := manager.CreateTemplate(template)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Test successful rendering
	templateData := &EmailTemplateData{
		Variables: map[string]interface{}{
			"Name":    "John Doe",
			"Title":   "Welcome",
			"Message": "Welcome to our service!",
		},
		CustomerCode: "hts",
	}

	rendered, err := manager.RenderTemplate("render-test", "hts", templateData)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Verify rendered content
	expectedSubject := "Hello John Doe - Welcome"
	if rendered.Subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, rendered.Subject)
	}

	if !strings.Contains(rendered.HTMLBody, "Hello John Doe") {
		t.Errorf("HTML body should contain 'Hello John Doe'")
	}

	if !strings.Contains(rendered.HTMLBody, "HTS Production") {
		t.Errorf("HTML body should contain customer name 'HTS Production'")
	}

	if !strings.Contains(rendered.TextBody, "Welcome to our service!") {
		t.Errorf("Text body should contain message")
	}

	if rendered.TemplateID != "render-test" {
		t.Errorf("Expected template ID 'render-test', got '%s'", rendered.TemplateID)
	}

	// Test missing required variable
	incompleteData := &EmailTemplateData{
		Variables: map[string]interface{}{
			"Name":  "John Doe",
			"Title": "Welcome",
			// Missing "Message"
		},
		CustomerCode: "hts",
	}

	_, err = manager.RenderTemplate("render-test", "hts", incompleteData)
	if err == nil {
		t.Errorf("Expected error for missing required variable, got none")
	}

	// Test non-existent template
	_, err = manager.RenderTemplate("non-existent", "hts", templateData)
	if err == nil {
		t.Errorf("Expected error for non-existent template, got none")
	}
}

func TestValidateTemplateSyntax(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Test valid syntax
	validTemplate := &EmailTemplate{
		ID:       "valid-syntax",
		Name:     "Valid Syntax",
		Subject:  "Hello {{.Name}}",
		HTMLBody: "<h1>{{.Title}}</h1><p>{{.Message}}</p>",
		TextBody: "{{.Title}}\n\n{{.Message}}",
	}

	err := manager.validateTemplateSyntax(validTemplate)
	if err != nil {
		t.Errorf("Expected no error for valid syntax, got: %v", err)
	}

	// Test invalid syntax
	invalidTemplates := []*EmailTemplate{
		{
			Subject: "{{.Bad",
		},
		{
			HTMLBody: "<h1>{{.Title</h1>",
		},
		{
			TextBody: "{{.Message}}{{",
		},
	}

	for i, invalidTemplate := range invalidTemplates {
		err = manager.validateTemplateSyntax(invalidTemplate)
		if err == nil {
			t.Errorf("Expected error for invalid syntax %d, got none", i)
		}
	}
}

func TestGetTemplateVariables(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	templateStr := "Hello {{.Name}}, your {{.OrderType}} order #{{.OrderID}} is {{.Status}}. {{.Name}} again!"

	variables, err := manager.GetTemplateVariables(templateStr)
	if err != nil {
		t.Fatalf("Failed to get template variables: %v", err)
	}

	expectedVariables := []string{"Name", "OrderType", "OrderID", "Status"}
	if len(variables) != len(expectedVariables) {
		t.Errorf("Expected %d variables, got %d", len(expectedVariables), len(variables))
	}

	// Check that all expected variables are found (order may vary)
	variableMap := make(map[string]bool)
	for _, variable := range variables {
		variableMap[variable] = true
	}

	for _, expected := range expectedVariables {
		if !variableMap[expected] {
			t.Errorf("Expected variable '%s' not found", expected)
		}
	}

	// Verify no duplicates (Name appears twice but should only be listed once)
	if len(variables) != 4 {
		t.Errorf("Expected 4 unique variables, got %d", len(variables))
	}
}

func TestExportImportTemplate(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Create test template
	originalTemplate := &EmailTemplate{
		ID:          "export-test",
		Name:        "Export Test",
		Description: "Template for export testing",
		Subject:     "Export Test: {{.Title}}",
		HTMLBody:    "<h1>{{.Title}}</h1><p>{{.Message}}</p>",
		TextBody:    "{{.Title}}\n\n{{.Message}}",
		Variables: []TemplateVariable{
			{Name: "Title", Type: "string", Required: true, Description: "Email title"},
			{Name: "Message", Type: "string", Required: true, Description: "Email message"},
		},
		CustomerCode: "hts",
	}

	err := manager.CreateTemplate(originalTemplate)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Export template
	jsonData, err := manager.ExportTemplate("hts-export-test")
	if err != nil {
		t.Fatalf("Failed to export template: %v", err)
	}

	// Verify JSON is valid
	var exportedTemplate EmailTemplate
	err = json.Unmarshal(jsonData, &exportedTemplate)
	if err != nil {
		t.Fatalf("Failed to parse exported JSON: %v", err)
	}

	if exportedTemplate.Name != originalTemplate.Name {
		t.Errorf("Expected name '%s', got '%s'", originalTemplate.Name, exportedTemplate.Name)
	}

	// Delete original template
	manager.DeleteTemplate("hts-export-test")

	// Import template back
	err = manager.ImportTemplate(jsonData)
	if err != nil {
		t.Fatalf("Failed to import template: %v", err)
	}

	// Verify imported template
	importedTemplate, err := manager.GetTemplate("export-test", "hts")
	if err != nil {
		t.Fatalf("Failed to get imported template: %v", err)
	}

	if importedTemplate.Name != originalTemplate.Name {
		t.Errorf("Expected imported name '%s', got '%s'", originalTemplate.Name, importedTemplate.Name)
	}

	if importedTemplate.Subject != originalTemplate.Subject {
		t.Errorf("Expected imported subject '%s', got '%s'", originalTemplate.Subject, importedTemplate.Subject)
	}

	// Test export non-existent template
	_, err = manager.ExportTemplate("non-existent")
	if err == nil {
		t.Errorf("Expected error for non-existent template export, got none")
	}

	// Test import invalid JSON
	err = manager.ImportTemplate([]byte("invalid json"))
	if err == nil {
		t.Errorf("Expected error for invalid JSON import, got none")
	}
}

func TestValidateTemplateID(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Test valid IDs
	validIDs := []string{
		"valid-id",
		"valid_id",
		"ValidID123",
		"test-template-1",
		"a",
		"123",
	}

	for _, id := range validIDs {
		err := manager.ValidateTemplateID(id)
		if err != nil {
			t.Errorf("Expected ID '%s' to be valid, got error: %v", id, err)
		}
	}

	// Test invalid IDs
	invalidIDs := []string{
		"",                       // Empty
		"invalid.id",             // Contains dot
		"invalid id",             // Contains space
		"invalid@id",             // Contains @
		"invalid/id",             // Contains slash
		strings.Repeat("a", 101), // Too long
	}

	for _, id := range invalidIDs {
		err := manager.ValidateTemplateID(id)
		if err == nil {
			t.Errorf("Expected ID '%s' to be invalid, got no error", id)
		}
	}
}

func TestDefaultTemplateContent(t *testing.T) {
	manager := NewEmailTemplateManager(nil)

	// Test notification template
	notificationTemplate, err := manager.GetTemplate("notification", "")
	if err != nil {
		t.Fatalf("Failed to get notification template: %v", err)
	}

	if !strings.Contains(notificationTemplate.Subject, "{{.Title}}") {
		t.Errorf("Notification template subject should contain {{.Title}}")
	}

	if !strings.Contains(notificationTemplate.HTMLBody, "{{.Message}}") {
		t.Errorf("Notification template HTML body should contain {{.Message}}")
	}

	if len(notificationTemplate.Variables) == 0 {
		t.Errorf("Notification template should have variables defined")
	}

	// Test welcome template
	welcomeTemplate, err := manager.GetTemplate("welcome", "")
	if err != nil {
		t.Fatalf("Failed to get welcome template: %v", err)
	}

	if !strings.Contains(welcomeTemplate.Subject, "{{.ServiceName}}") {
		t.Errorf("Welcome template subject should contain {{.ServiceName}}")
	}

	// Test error template
	errorTemplate, err := manager.GetTemplate("error", "")
	if err != nil {
		t.Fatalf("Failed to get error template: %v", err)
	}

	if !strings.Contains(errorTemplate.Subject, "{{.ErrorType}}") {
		t.Errorf("Error template subject should contain {{.ErrorType}}")
	}
}
