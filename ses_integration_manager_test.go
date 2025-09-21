package main

import (
	"fmt"
	"testing"
	"time"
)

func TestNewSESIntegrationManager(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)

	if sesManager.CustomerManager != customerManager {
		t.Errorf("Expected customer manager to be set")
	}

	if sesManager.TemplateManager != templateManager {
		t.Errorf("Expected template manager to be set")
	}

	if sesManager.DefaultFromEmail == "" {
		t.Errorf("Expected default from email to be set")
	}

	if sesManager.RetryConfig.MaxRetries == 0 {
		t.Errorf("Expected retry config to be initialized")
	}
}

func TestValidateEmailRequest(t *testing.T) {
	sesManager := setupSESManager()

	// Test valid request
	validRequest := &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Type: "to"},
		},
		TemplateData: map[string]interface{}{
			"Title":   "Test",
			"Message": "Test message",
		},
	}

	err := sesManager.validateEmailRequest(validRequest)
	if err != nil {
		t.Errorf("Expected valid request to pass validation, got: %v", err)
	}

	// Test missing customer code
	invalidRequest := &EmailRequest{
		TemplateID: "notification",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Type: "to"},
		},
	}

	err = sesManager.validateEmailRequest(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for missing customer code, got none")
	}

	// Test missing template ID
	invalidRequest = &EmailRequest{
		CustomerCode: "hts",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Type: "to"},
		},
	}

	err = sesManager.validateEmailRequest(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for missing template ID, got none")
	}

	// Test no recipients
	invalidRequest = &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients:   []EmailRecipient{},
	}

	err = sesManager.validateEmailRequest(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for no recipients, got none")
	}

	// Test invalid email format
	invalidRequest = &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "invalid-email", Type: "to"},
		},
	}

	err = sesManager.validateEmailRequest(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for invalid email format, got none")
	}

	// Test invalid recipient type
	invalidRequest = &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Type: "invalid"},
		},
	}

	err = sesManager.validateEmailRequest(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for invalid recipient type, got none")
	}

	// Test invalid from email
	invalidRequest = &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Type: "to"},
		},
		FromEmail: "invalid-email",
	}

	err = sesManager.validateEmailRequest(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for invalid from email, got none")
	}

	// Test invalid priority
	invalidRequest = &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Type: "to"},
		},
		Priority: "invalid",
	}

	err = sesManager.validateEmailRequest(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for invalid priority, got none")
	}
}

func TestSendEmail(t *testing.T) {
	sesManager := setupSESManager()

	// Test successful email sending
	request := &EmailRequest{
		CustomerCode: "hts",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Name: "Test User", Type: "to"},
		},
		TemplateData: map[string]interface{}{
			"Title":   "Test Email",
			"Message": "This is a test email",
		},
		FromEmail: "sender@example.com",
		FromName:  "Test Sender",
		Tags: map[string]string{
			"Campaign": "test",
		},
	}

	response, err := sesManager.SendEmail(request)
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	if response.Status != "sent" {
		t.Errorf("Expected status 'sent', got '%s'", response.Status)
	}

	if response.CustomerCode != "hts" {
		t.Errorf("Expected customer code 'hts', got '%s'", response.CustomerCode)
	}

	if response.MessageID == "" {
		t.Errorf("Expected message ID to be set")
	}

	if len(response.Recipients) != 1 {
		t.Errorf("Expected 1 recipient, got %d", len(response.Recipients))
	}

	// Test with invalid customer
	invalidRequest := &EmailRequest{
		CustomerCode: "nonexistent",
		TemplateID:   "notification",
		Recipients: []EmailRecipient{
			{Email: "test@example.com", Type: "to"},
		},
		TemplateData: map[string]interface{}{
			"Title":   "Test",
			"Message": "Test",
		},
	}

	_, err = sesManager.SendEmail(invalidRequest)
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

func TestSendBulkEmail(t *testing.T) {
	sesManager := setupSESManager()

	// Test bulk email sending
	request := &BulkEmailRequest{
		TemplateID: "notification",
		CustomerEmails: map[string][]EmailRecipient{
			"hts": {
				{Email: "hts1@example.com", Type: "to"},
				{Email: "hts2@example.com", Type: "to"},
			},
			"cds": {
				{Email: "cds1@example.com", Type: "to"},
			},
		},
		TemplateData: map[string]interface{}{
			"Title":   "Bulk Email Test",
			"Message": "This is a bulk email test",
		},
		CustomerData: map[string]map[string]interface{}{
			"hts": {
				"CustomMessage": "HTS specific message",
			},
			"cds": {
				"CustomMessage": "CDS specific message",
			},
		},
		FromEmail: "bulk@example.com",
		Tags: map[string]string{
			"Campaign": "bulk-test",
		},
	}

	response, err := sesManager.SendBulkEmail(request)
	if err != nil {
		t.Fatalf("Failed to send bulk email: %v", err)
	}

	if response.TotalCustomers != 2 {
		t.Errorf("Expected 2 customers, got %d", response.TotalCustomers)
	}

	if response.TotalEmails != 3 {
		t.Errorf("Expected 3 total emails, got %d", response.TotalEmails)
	}

	if response.SuccessCount != 2 {
		t.Errorf("Expected 2 successful sends, got %d", response.SuccessCount)
	}

	if response.FailureCount != 0 {
		t.Errorf("Expected 0 failures, got %d", response.FailureCount)
	}

	// Verify responses for each customer
	if _, exists := response.Responses["hts"]; !exists {
		t.Errorf("Expected response for HTS customer")
	}

	if _, exists := response.Responses["cds"]; !exists {
		t.Errorf("Expected response for CDS customer")
	}
}

func TestValidateEmailAddresses(t *testing.T) {
	sesManager := setupSESManager()

	emails := []string{
		"valid@example.com",
		"invalid-email",
		"test@tempmail.com", // Risky domain
		"another@example.org",
	}

	results, err := sesManager.ValidateEmailAddresses("hts", emails)
	if err != nil {
		t.Fatalf("Failed to validate email addresses: %v", err)
	}

	if len(results) != len(emails) {
		t.Errorf("Expected %d results, got %d", len(emails), len(results))
	}

	// Check valid email
	if !results[0].Valid {
		t.Errorf("Expected first email to be valid")
	}

	if !results[0].Deliverable {
		t.Errorf("Expected first email to be deliverable")
	}

	// Check invalid email
	if results[1].Valid {
		t.Errorf("Expected second email to be invalid")
	}

	if results[1].Deliverable {
		t.Errorf("Expected second email to be undeliverable")
	}

	// Check risky domain
	if !results[2].Risky {
		t.Errorf("Expected third email to be risky")
	}

	// Test with invalid customer
	_, err = sesManager.ValidateEmailAddresses("nonexistent", emails)
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

func TestGetSESConfiguration(t *testing.T) {
	sesManager := setupSESManager()

	config, err := sesManager.GetSESConfiguration("hts")
	if err != nil {
		t.Fatalf("Failed to get SES configuration: %v", err)
	}

	if config.CustomerCode != "hts" {
		t.Errorf("Expected customer code 'hts', got '%s'", config.CustomerCode)
	}

	if config.Region == "" {
		t.Errorf("Expected region to be set")
	}

	if config.FromEmail == "" {
		t.Errorf("Expected from email to be set")
	}

	if config.SendingQuota == 0 {
		t.Errorf("Expected sending quota to be set")
	}

	if len(config.VerifiedDomains) == 0 {
		t.Errorf("Expected verified domains to be set")
	}

	// Test with invalid customer
	_, err = sesManager.GetSESConfiguration("nonexistent")
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

func TestGetSendingStatistics(t *testing.T) {
	sesManager := setupSESManager()

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	stats, err := sesManager.GetSendingStatistics("hts", startTime, endTime)
	if err != nil {
		t.Fatalf("Failed to get sending statistics: %v", err)
	}

	if stats["customerCode"] != "hts" {
		t.Errorf("Expected customer code 'hts', got '%v'", stats["customerCode"])
	}

	if stats["sent"] == nil {
		t.Errorf("Expected sent count to be present")
	}

	if stats["delivered"] == nil {
		t.Errorf("Expected delivered count to be present")
	}

	if stats["deliveryRate"] == nil {
		t.Errorf("Expected delivery rate to be present")
	}

	// Test with invalid customer
	_, err = sesManager.GetSendingStatistics("nonexistent", startTime, endTime)
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

func TestListSuppressedAddresses(t *testing.T) {
	sesManager := setupSESManager()

	addresses, err := sesManager.ListSuppressedAddresses("hts")
	if err != nil {
		t.Fatalf("Failed to list suppressed addresses: %v", err)
	}

	if len(addresses) == 0 {
		t.Errorf("Expected some suppressed addresses in test data")
	}

	// Check structure of first address
	if len(addresses) > 0 {
		address := addresses[0]
		if address["email"] == nil {
			t.Errorf("Expected email field in suppressed address")
		}

		if address["reason"] == nil {
			t.Errorf("Expected reason field in suppressed address")
		}

		if address["lastUpdateTime"] == nil {
			t.Errorf("Expected lastUpdateTime field in suppressed address")
		}
	}

	// Test with invalid customer
	_, err = sesManager.ListSuppressedAddresses("nonexistent")
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

func TestIsValidEmailFormat(t *testing.T) {
	sesManager := setupSESManager()

	validEmails := []string{
		"test@example.com",
		"user.name@example.org",
		"user+tag@example.net",
		"123@example.com",
		"test@sub.example.com",
	}

	for _, email := range validEmails {
		if !sesManager.isValidEmailFormat(email) {
			t.Errorf("Expected email '%s' to be valid", email)
		}
	}

	invalidEmails := []string{
		"",
		"invalid",
		"@example.com",
		"test@",
		"test..test@example.com",
		"test@example",
		"test@.com",
	}

	for _, email := range invalidEmails {
		if sesManager.isValidEmailFormat(email) {
			t.Errorf("Expected email '%s' to be invalid", email)
		}
	}
}

func TestIsRiskyDomain(t *testing.T) {
	sesManager := setupSESManager()

	riskyDomains := []string{
		"tempmail.com",
		"10minutemail.com",
		"guerrillamail.com",
		"mailinator.com",
		"TEMPMAIL.COM", // Case insensitive
	}

	for _, domain := range riskyDomains {
		if !sesManager.isRiskyDomain(domain) {
			t.Errorf("Expected domain '%s' to be risky", domain)
		}
	}

	safeDomains := []string{
		"example.com",
		"gmail.com",
		"yahoo.com",
		"outlook.com",
	}

	for _, domain := range safeDomains {
		if sesManager.isRiskyDomain(domain) {
			t.Errorf("Expected domain '%s' to be safe", domain)
		}
	}
}

func TestPrepareSESParams(t *testing.T) {
	sesManager := setupSESManager()

	request := &EmailRequest{
		CustomerCode: "hts",
		Recipients: []EmailRecipient{
			{Email: "to1@example.com", Type: "to"},
			{Email: "to2@example.com", Type: "to"},
			{Email: "cc@example.com", Type: "cc"},
			{Email: "bcc@example.com", Type: "bcc"},
		},
		FromEmail:    "sender@example.com",
		FromName:     "Test Sender",
		ReplyToEmail: "reply@example.com",
		Tags: map[string]string{
			"Campaign": "test",
			"Type":     "notification",
		},
	}

	renderedEmail := &RenderedEmail{
		Subject:  "Test Subject",
		HTMLBody: "<h1>Test HTML</h1>",
		TextBody: "Test Text",
	}

	credentials := &AWSCredentials{
		CustomerCode: "hts",
		Region:       "us-east-1",
	}

	params := sesManager.prepareSESParams(request, renderedEmail, credentials)

	// Check source
	expectedSource := "Test Sender <sender@example.com>"
	if params["Source"] != expectedSource {
		t.Errorf("Expected source '%s', got '%v'", expectedSource, params["Source"])
	}

	// Check destinations
	destination := params["Destination"].(map[string]interface{})

	toAddresses := destination["ToAddresses"].([]string)
	if len(toAddresses) != 2 {
		t.Errorf("Expected 2 TO addresses, got %d", len(toAddresses))
	}

	ccAddresses := destination["CcAddresses"].([]string)
	if len(ccAddresses) != 1 {
		t.Errorf("Expected 1 CC address, got %d", len(ccAddresses))
	}

	bccAddresses := destination["BccAddresses"].([]string)
	if len(bccAddresses) != 1 {
		t.Errorf("Expected 1 BCC address, got %d", len(bccAddresses))
	}

	// Check message
	message := params["Message"].(map[string]interface{})
	subject := message["Subject"].(map[string]interface{})
	if subject["Data"] != "Test Subject" {
		t.Errorf("Expected subject 'Test Subject', got '%v'", subject["Data"])
	}

	body := message["Body"].(map[string]interface{})
	htmlBody := body["Html"].(map[string]interface{})
	if htmlBody["Data"] != "<h1>Test HTML</h1>" {
		t.Errorf("Expected HTML body '<h1>Test HTML</h1>', got '%v'", htmlBody["Data"])
	}

	textBody := body["Text"].(map[string]interface{})
	if textBody["Data"] != "Test Text" {
		t.Errorf("Expected text body 'Test Text', got '%v'", textBody["Data"])
	}

	// Check reply-to
	replyToAddresses := params["ReplyToAddresses"].([]string)
	if len(replyToAddresses) != 1 || replyToAddresses[0] != "reply@example.com" {
		t.Errorf("Expected reply-to 'reply@example.com', got %v", replyToAddresses)
	}

	// Check tags
	tags := params["Tags"].([]map[string]interface{})
	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}
}

func TestIsRetryableError(t *testing.T) {
	sesManager := setupSESManager()

	retryableErrors := []error{
		fmt.Errorf("Throttling exception"),
		fmt.Errorf("Rate exceeded"),
		fmt.Errorf("Service unavailable"),
		fmt.Errorf("Internal error occurred"),
		fmt.Errorf("Connection timeout"),
	}

	for _, err := range retryableErrors {
		if !sesManager.isRetryableError(err) {
			t.Errorf("Expected error '%v' to be retryable", err)
		}
	}

	nonRetryableErrors := []error{
		fmt.Errorf("Invalid email address"),
		fmt.Errorf("Message rejected"),
		fmt.Errorf("Account suspended"),
		fmt.Errorf("Invalid credentials"),
	}

	for _, err := range nonRetryableErrors {
		if sesManager.isRetryableError(err) {
			t.Errorf("Expected error '%v' to be non-retryable", err)
		}
	}
}

func TestGetEmailDeliveryStatus(t *testing.T) {
	sesManager := setupSESManager()

	messageID := "test-message-123"

	status, err := sesManager.GetEmailDeliveryStatus("hts", messageID)
	if err != nil {
		t.Fatalf("Failed to get email delivery status: %v", err)
	}

	if status["messageId"] != messageID {
		t.Errorf("Expected message ID '%s', got '%v'", messageID, status["messageId"])
	}

	if status["customerCode"] != "hts" {
		t.Errorf("Expected customer code 'hts', got '%v'", status["customerCode"])
	}

	if status["status"] == nil {
		t.Errorf("Expected status to be present")
	}

	if status["events"] == nil {
		t.Errorf("Expected events to be present")
	}

	// Test with invalid customer
	_, err = sesManager.GetEmailDeliveryStatus("nonexistent", messageID)
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

func TestCreateConfigurationSet(t *testing.T) {
	sesManager := setupSESManager()

	configSetName := "test-config-set"

	err := sesManager.CreateConfigurationSet("hts", configSetName)
	if err != nil {
		t.Fatalf("Failed to create configuration set: %v", err)
	}

	// Test with invalid customer
	err = sesManager.CreateConfigurationSet("nonexistent", configSetName)
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

func TestSetupEventPublishing(t *testing.T) {
	sesManager := setupSESManager()

	configSetName := "test-config-set"
	eventTypes := []string{"send", "delivery", "bounce", "complaint"}

	err := sesManager.SetupEventPublishing("hts", configSetName, eventTypes)
	if err != nil {
		t.Fatalf("Failed to setup event publishing: %v", err)
	}

	// Test with invalid customer
	err = sesManager.SetupEventPublishing("nonexistent", configSetName, eventTypes)
	if err == nil {
		t.Errorf("Expected error for invalid customer, got none")
	}
}

// Helper function to set up SES manager with test data
func setupSESManager() *SESIntegrationManager {
	customerManager := NewCustomerCredentialManager("us-east-1")

	// Add test customers
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			Environment:  "production",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Global",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "production",
		},
	}

	templateManager := NewEmailTemplateManager(customerManager)
	return NewSESIntegrationManager(customerManager, templateManager)
}
