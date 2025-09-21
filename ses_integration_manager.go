package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SESIntegrationManager handles SES operations for multi-customer environments
type SESIntegrationManager struct {
	CustomerManager  *CustomerCredentialManager
	TemplateManager  *EmailTemplateManager
	DefaultFromEmail string
	DefaultFromName  string
	RetryConfig      SESRetryConfig
}

// SESRetryConfig defines retry behavior for SES operations
type SESRetryConfig struct {
	MaxRetries    int           `json:"maxRetries"`
	InitialDelay  time.Duration `json:"initialDelay"`
	MaxDelay      time.Duration `json:"maxDelay"`
	BackoffFactor float64       `json:"backoffFactor"`
}

// EmailRequest represents a request to send an email
type EmailRequest struct {
	CustomerCode  string                 `json:"customerCode"`
	TemplateID    string                 `json:"templateId"`
	Recipients    []EmailRecipient       `json:"recipients"`
	TemplateData  map[string]interface{} `json:"templateData"`
	FromEmail     string                 `json:"fromEmail,omitempty"`
	FromName      string                 `json:"fromName,omitempty"`
	ReplyToEmail  string                 `json:"replyToEmail,omitempty"`
	Tags          map[string]string      `json:"tags,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Priority      string                 `json:"priority,omitempty"` // high, normal, low
	ScheduledTime *time.Time             `json:"scheduledTime,omitempty"`
}

// EmailRecipient represents an email recipient
type EmailRecipient struct {
	Email       string            `json:"email"`
	Name        string            `json:"name,omitempty"`
	Type        string            `json:"type"` // to, cc, bcc
	Variables   map[string]string `json:"variables,omitempty"`
	Unsubscribe bool              `json:"unsubscribe,omitempty"`
}

// EmailResponse represents the response from sending an email
type EmailResponse struct {
	MessageID    string                 `json:"messageId"`
	CustomerCode string                 `json:"customerCode"`
	Recipients   []EmailRecipient       `json:"recipients"`
	Status       string                 `json:"status"` // sent, failed, queued
	SentAt       time.Time              `json:"sentAt"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	SESResponse  map[string]interface{} `json:"sesResponse,omitempty"`
}

// BulkEmailRequest represents a request to send emails to multiple customers
type BulkEmailRequest struct {
	TemplateID     string                            `json:"templateId"`
	CustomerEmails map[string][]EmailRecipient       `json:"customerEmails"` // customerCode -> recipients
	TemplateData   map[string]interface{}            `json:"templateData"`
	CustomerData   map[string]map[string]interface{} `json:"customerData,omitempty"` // customer-specific data
	FromEmail      string                            `json:"fromEmail,omitempty"`
	FromName       string                            `json:"fromName,omitempty"`
	Tags           map[string]string                 `json:"tags,omitempty"`
	Priority       string                            `json:"priority,omitempty"`
	ScheduledTime  *time.Time                        `json:"scheduledTime,omitempty"`
}

// BulkEmailResponse represents the response from bulk email sending
type BulkEmailResponse struct {
	TotalCustomers int                       `json:"totalCustomers"`
	TotalEmails    int                       `json:"totalEmails"`
	SuccessCount   int                       `json:"successCount"`
	FailureCount   int                       `json:"failureCount"`
	Responses      map[string]*EmailResponse `json:"responses"` // customerCode -> response
	Errors         map[string]string         `json:"errors,omitempty"`
	ProcessedAt    time.Time                 `json:"processedAt"`
	Metadata       map[string]interface{}    `json:"metadata,omitempty"`
}

// SESConfiguration represents SES configuration for a customer
type SESConfiguration struct {
	CustomerCode         string            `json:"customerCode"`
	Region               string            `json:"region"`
	FromEmail            string            `json:"fromEmail"`
	FromName             string            `json:"fromName"`
	ReplyToEmail         string            `json:"replyToEmail,omitempty"`
	ConfigurationSetName string            `json:"configurationSetName,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty"`
	SendingQuota         int64             `json:"sendingQuota,omitempty"`
	SendingRate          float64           `json:"sendingRate,omitempty"`
	VerifiedDomains      []string          `json:"verifiedDomains,omitempty"`
	VerifiedEmails       []string          `json:"verifiedEmails,omitempty"`
}

// EmailValidationResult represents email validation results
type EmailValidationResult struct {
	Email       string `json:"email"`
	Valid       bool   `json:"valid"`
	Reason      string `json:"reason,omitempty"`
	Deliverable bool   `json:"deliverable"`
	Risky       bool   `json:"risky"`
}

// NewSESIntegrationManager creates a new SES integration manager
func NewSESIntegrationManager(customerManager *CustomerCredentialManager, templateManager *EmailTemplateManager) *SESIntegrationManager {
	return &SESIntegrationManager{
		CustomerManager:  customerManager,
		TemplateManager:  templateManager,
		DefaultFromEmail: "noreply@example.com",
		DefaultFromName:  "Email Distribution System",
		RetryConfig: SESRetryConfig{
			MaxRetries:    3,
			InitialDelay:  1 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
		},
	}
}

// SendEmail sends an email using the specified template and customer credentials
func (sim *SESIntegrationManager) SendEmail(request *EmailRequest) (*EmailResponse, error) {
	// Validate request
	if err := sim.validateEmailRequest(request); err != nil {
		return nil, fmt.Errorf("invalid email request: %v", err)
	}

	// Get customer credentials
	credentials, err := sim.CustomerManager.AssumeCustomerRole(request.CustomerCode, "ses")
	if err != nil {
		return nil, fmt.Errorf("failed to assume customer role: %v", err)
	}

	// Render email template
	templateData := &EmailTemplateData{
		Variables:    request.TemplateData,
		CustomerCode: request.CustomerCode,
		Metadata:     request.Metadata,
	}

	renderedEmail, err := sim.TemplateManager.RenderTemplate(request.TemplateID, request.CustomerCode, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render email template: %v", err)
	}

	// Prepare SES email parameters
	sesParams := sim.prepareSESParams(request, renderedEmail, credentials)

	// Send email with retry logic
	response, err := sim.sendEmailWithRetry(sesParams, credentials)
	if err != nil {
		return &EmailResponse{
			CustomerCode: request.CustomerCode,
			Recipients:   request.Recipients,
			Status:       "failed",
			SentAt:       time.Now(),
			Error:        err.Error(),
			Metadata:     request.Metadata,
		}, err
	}

	return &EmailResponse{
		MessageID:    response["MessageId"].(string),
		CustomerCode: request.CustomerCode,
		Recipients:   request.Recipients,
		Status:       "sent",
		SentAt:       time.Now(),
		Metadata:     request.Metadata,
		SESResponse:  response,
	}, nil
}

// SendBulkEmail sends emails to multiple customers
func (sim *SESIntegrationManager) SendBulkEmail(request *BulkEmailRequest) (*BulkEmailResponse, error) {
	response := &BulkEmailResponse{
		TotalCustomers: len(request.CustomerEmails),
		Responses:      make(map[string]*EmailResponse),
		Errors:         make(map[string]string),
		ProcessedAt:    time.Now(),
		Metadata:       make(map[string]interface{}),
	}

	// Process each customer
	for customerCode, recipients := range request.CustomerEmails {
		// Prepare customer-specific template data
		templateData := make(map[string]interface{})
		for key, value := range request.TemplateData {
			templateData[key] = value
		}

		// Add customer-specific data if available
		if customerData, exists := request.CustomerData[customerCode]; exists {
			for key, value := range customerData {
				templateData[key] = value
			}
		}

		// Create email request for this customer
		emailRequest := &EmailRequest{
			CustomerCode:  customerCode,
			TemplateID:    request.TemplateID,
			Recipients:    recipients,
			TemplateData:  templateData,
			FromEmail:     request.FromEmail,
			FromName:      request.FromName,
			Tags:          request.Tags,
			Priority:      request.Priority,
			ScheduledTime: request.ScheduledTime,
		}

		// Send email for this customer
		emailResponse, err := sim.SendEmail(emailRequest)
		if err != nil {
			response.Errors[customerCode] = err.Error()
			response.FailureCount++
		} else {
			response.Responses[customerCode] = emailResponse
			response.SuccessCount++
		}

		response.TotalEmails += len(recipients)
	}

	return response, nil
}

// ValidateEmailAddresses validates email addresses using SES
func (sim *SESIntegrationManager) ValidateEmailAddresses(customerCode string, emails []string) ([]EmailValidationResult, error) {
	// Get customer credentials
	_, err := sim.CustomerManager.AssumeCustomerRole(customerCode, "ses")
	if err != nil {
		return nil, fmt.Errorf("failed to assume customer role: %v", err)
	}

	var results []EmailValidationResult

	for _, email := range emails {
		result := EmailValidationResult{
			Email: email,
		}

		// Basic email format validation
		if !sim.isValidEmailFormat(email) {
			result.Valid = false
			result.Reason = "Invalid email format"
			result.Deliverable = false
			results = append(results, result)
			continue
		}

		// SES validation (simulated - would use actual SES API)
		result.Valid = true
		result.Deliverable = true
		result.Risky = false

		// Check against known problematic domains (example logic)
		domain := strings.Split(email, "@")[1]
		if sim.isRiskyDomain(domain) {
			result.Risky = true
			result.Reason = "Domain flagged as risky"
		}

		results = append(results, result)
	}

	return results, nil
}

// GetSESConfiguration retrieves SES configuration for a customer
func (sim *SESIntegrationManager) GetSESConfiguration(customerCode string) (*SESConfiguration, error) {
	// Get customer account info
	accountInfo, err := sim.CustomerManager.GetCustomerAccountInfo(customerCode)
	if err != nil {
		return nil, err
	}

	// Get customer credentials
	credentials, err := sim.CustomerManager.AssumeCustomerRole(customerCode, "ses")
	if err != nil {
		return nil, fmt.Errorf("failed to assume customer role: %v", err)
	}

	// Simulate SES configuration retrieval
	config := &SESConfiguration{
		CustomerCode:         customerCode,
		Region:               credentials.Region,
		FromEmail:            fmt.Sprintf("noreply@%s.example.com", customerCode),
		FromName:             accountInfo.CustomerName,
		ConfigurationSetName: fmt.Sprintf("%s-config-set", customerCode),
		SendingQuota:         1000,
		SendingRate:          5.0,
		VerifiedDomains:      []string{fmt.Sprintf("%s.example.com", customerCode)},
		VerifiedEmails:       []string{fmt.Sprintf("noreply@%s.example.com", customerCode)},
		Tags: map[string]string{
			"Customer":    customerCode,
			"Environment": accountInfo.Environment,
		},
	}

	return config, nil
}

// GetSendingStatistics retrieves sending statistics for a customer
func (sim *SESIntegrationManager) GetSendingStatistics(customerCode string, startTime, endTime time.Time) (map[string]interface{}, error) {
	// Get customer credentials
	_, err := sim.CustomerManager.AssumeCustomerRole(customerCode, "ses")
	if err != nil {
		return nil, fmt.Errorf("failed to assume customer role: %v", err)
	}

	// Simulate statistics retrieval
	stats := map[string]interface{}{
		"customerCode": customerCode,
		"period": map[string]string{
			"start": startTime.Format("2006-01-02T15:04:05Z"),
			"end":   endTime.Format("2006-01-02T15:04:05Z"),
		},
		"sent":          150,
		"delivered":     145,
		"bounced":       3,
		"complained":    1,
		"rejected":      1,
		"deliveryRate":  96.7,
		"bounceRate":    2.0,
		"complaintRate": 0.7,
		"reputationMetrics": map[string]interface{}{
			"deliverabilityDelay": 0,
			"ipWarmupPercentage":  100,
			"reputationStatus":    "healthy",
		},
	}

	return stats, nil
}

// ListSuppressedAddresses retrieves suppressed email addresses for a customer
func (sim *SESIntegrationManager) ListSuppressedAddresses(customerCode string) ([]map[string]interface{}, error) {
	// Get customer credentials
	_, err := sim.CustomerManager.AssumeCustomerRole(customerCode, "ses")
	if err != nil {
		return nil, fmt.Errorf("failed to assume customer role: %v", err)
	}

	// Simulate suppressed addresses retrieval
	suppressedAddresses := []map[string]interface{}{
		{
			"email":          "bounced@example.com",
			"reason":         "BOUNCE",
			"lastUpdateTime": time.Now().Add(-24 * time.Hour).Format("2006-01-02T15:04:05Z"),
		},
		{
			"email":          "complained@example.com",
			"reason":         "COMPLAINT",
			"lastUpdateTime": time.Now().Add(-48 * time.Hour).Format("2006-01-02T15:04:05Z"),
		},
	}

	return suppressedAddresses, nil
}

// validateEmailRequest validates an email request
func (sim *SESIntegrationManager) validateEmailRequest(request *EmailRequest) error {
	if request.CustomerCode == "" {
		return fmt.Errorf("customer code is required")
	}

	if request.TemplateID == "" {
		return fmt.Errorf("template ID is required")
	}

	if len(request.Recipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	// Validate recipients
	for i, recipient := range request.Recipients {
		if recipient.Email == "" {
			return fmt.Errorf("recipient %d: email is required", i)
		}

		if !sim.isValidEmailFormat(recipient.Email) {
			return fmt.Errorf("recipient %d: invalid email format: %s", i, recipient.Email)
		}

		if recipient.Type == "" {
			recipient.Type = "to" // Default to "to"
		}

		if recipient.Type != "to" && recipient.Type != "cc" && recipient.Type != "bcc" {
			return fmt.Errorf("recipient %d: invalid type: %s (must be to, cc, or bcc)", i, recipient.Type)
		}
	}

	// Validate from email if provided
	if request.FromEmail != "" && !sim.isValidEmailFormat(request.FromEmail) {
		return fmt.Errorf("invalid from email format: %s", request.FromEmail)
	}

	// Validate reply-to email if provided
	if request.ReplyToEmail != "" && !sim.isValidEmailFormat(request.ReplyToEmail) {
		return fmt.Errorf("invalid reply-to email format: %s", request.ReplyToEmail)
	}

	// Validate priority
	if request.Priority != "" {
		validPriorities := []string{"high", "normal", "low"}
		valid := false
		for _, p := range validPriorities {
			if request.Priority == p {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid priority: %s (must be high, normal, or low)", request.Priority)
		}
	}

	return nil
}

// prepareSESParams prepares parameters for SES API call
func (sim *SESIntegrationManager) prepareSESParams(request *EmailRequest, renderedEmail *RenderedEmail, credentials *AWSCredentials) map[string]interface{} {
	// Prepare recipient lists
	toAddresses := []string{}
	ccAddresses := []string{}
	bccAddresses := []string{}

	for _, recipient := range request.Recipients {
		switch recipient.Type {
		case "to":
			toAddresses = append(toAddresses, recipient.Email)
		case "cc":
			ccAddresses = append(ccAddresses, recipient.Email)
		case "bcc":
			bccAddresses = append(bccAddresses, recipient.Email)
		}
	}

	// Determine from email
	fromEmail := request.FromEmail
	if fromEmail == "" {
		fromEmail = sim.DefaultFromEmail
	}

	// Determine from name
	fromName := request.FromName
	if fromName == "" {
		fromName = sim.DefaultFromName
	}

	// Prepare source address
	source := fromEmail
	if fromName != "" {
		source = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}

	params := map[string]interface{}{
		"Source": source,
		"Destination": map[string]interface{}{
			"ToAddresses":  toAddresses,
			"CcAddresses":  ccAddresses,
			"BccAddresses": bccAddresses,
		},
		"Message": map[string]interface{}{
			"Subject": map[string]interface{}{
				"Data":    renderedEmail.Subject,
				"Charset": "UTF-8",
			},
			"Body": map[string]interface{}{},
		},
	}

	// Add HTML body if available
	if renderedEmail.HTMLBody != "" {
		params["Message"].(map[string]interface{})["Body"].(map[string]interface{})["Html"] = map[string]interface{}{
			"Data":    renderedEmail.HTMLBody,
			"Charset": "UTF-8",
		}
	}

	// Add text body if available
	if renderedEmail.TextBody != "" {
		params["Message"].(map[string]interface{})["Body"].(map[string]interface{})["Text"] = map[string]interface{}{
			"Data":    renderedEmail.TextBody,
			"Charset": "UTF-8",
		}
	}

	// Add reply-to if specified
	if request.ReplyToEmail != "" {
		params["ReplyToAddresses"] = []string{request.ReplyToEmail}
	}

	// Add tags if specified
	if request.Tags != nil && len(request.Tags) > 0 {
		var tags []map[string]interface{}
		for key, value := range request.Tags {
			tags = append(tags, map[string]interface{}{
				"Name":  key,
				"Value": value,
			})
		}
		params["Tags"] = tags
	}

	return params
}

// sendEmailWithRetry sends email with retry logic
func (sim *SESIntegrationManager) sendEmailWithRetry(params map[string]interface{}, credentials *AWSCredentials) (map[string]interface{}, error) {
	var lastErr error
	delay := sim.RetryConfig.InitialDelay

	for attempt := 0; attempt <= sim.RetryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * sim.RetryConfig.BackoffFactor)
			if delay > sim.RetryConfig.MaxDelay {
				delay = sim.RetryConfig.MaxDelay
			}
		}

		// Simulate SES API call
		response, err := sim.callSESAPI("SendEmail", params, credentials)
		if err != nil {
			lastErr = err

			// Check if error is retryable
			if !sim.isRetryableError(err) {
				return nil, err
			}

			continue
		}

		return response, nil
	}

	return nil, fmt.Errorf("failed to send email after %d attempts: %v", sim.RetryConfig.MaxRetries+1, lastErr)
}

// callSESAPI simulates calling the SES API
func (sim *SESIntegrationManager) callSESAPI(operation string, params map[string]interface{}, credentials *AWSCredentials) (map[string]interface{}, error) {
	// Simulate API call delay
	time.Sleep(100 * time.Millisecond)

	// Simulate success response
	response := map[string]interface{}{
		"MessageId": fmt.Sprintf("msg-%s-%d", credentials.CustomerCode, time.Now().Unix()),
		"ResponseMetadata": map[string]interface{}{
			"RequestId": fmt.Sprintf("req-%d", time.Now().UnixNano()),
		},
	}

	return response, nil
}

// isRetryableError determines if an error is retryable
func (sim *SESIntegrationManager) isRetryableError(err error) bool {
	errorStr := strings.ToLower(err.Error())

	// Retryable errors
	retryableErrors := []string{
		"throttling",
		"rate exceeded",
		"service unavailable",
		"internal error",
		"timeout",
		"connection",
	}

	for _, retryableError := range retryableErrors {
		if strings.Contains(errorStr, retryableError) {
			return true
		}
	}

	return false
}

// isValidEmailFormat validates email format using regex
func (sim *SESIntegrationManager) isValidEmailFormat(email string) bool {
	// RFC 5322 compliant email regex (simplified)
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// isRiskyDomain checks if a domain is considered risky
func (sim *SESIntegrationManager) isRiskyDomain(domain string) bool {
	riskyDomains := []string{
		"tempmail.com",
		"10minutemail.com",
		"guerrillamail.com",
		"mailinator.com",
	}

	domain = strings.ToLower(domain)
	for _, riskyDomain := range riskyDomains {
		if domain == riskyDomain {
			return true
		}
	}

	return false
}

// GetEmailDeliveryStatus retrieves delivery status for a message
func (sim *SESIntegrationManager) GetEmailDeliveryStatus(customerCode, messageID string) (map[string]interface{}, error) {
	// Get customer credentials
	_, err := sim.CustomerManager.AssumeCustomerRole(customerCode, "ses")
	if err != nil {
		return nil, fmt.Errorf("failed to assume customer role: %v", err)
	}

	// Simulate delivery status retrieval
	status := map[string]interface{}{
		"messageId":    messageID,
		"customerCode": customerCode,
		"status":       "delivered",
		"timestamp":    time.Now().Add(-5 * time.Minute).Format("2006-01-02T15:04:05Z"),
		"events": []map[string]interface{}{
			{
				"eventType": "send",
				"timestamp": time.Now().Add(-10 * time.Minute).Format("2006-01-02T15:04:05Z"),
			},
			{
				"eventType": "delivery",
				"timestamp": time.Now().Add(-5 * time.Minute).Format("2006-01-02T15:04:05Z"),
			},
		},
	}

	return status, nil
}

// CreateConfigurationSet creates a SES configuration set for a customer
func (sim *SESIntegrationManager) CreateConfigurationSet(customerCode, configSetName string) error {
	// Get customer credentials
	credentials, err := sim.CustomerManager.AssumeCustomerRole(customerCode, "ses")
	if err != nil {
		return fmt.Errorf("failed to assume customer role: %v", err)
	}

	// Simulate configuration set creation
	params := map[string]interface{}{
		"ConfigurationSet": map[string]interface{}{
			"Name": configSetName,
		},
	}

	_, err = sim.callSESAPI("CreateConfigurationSet", params, credentials)
	return err
}

// SetupEventPublishing sets up event publishing for a customer
func (sim *SESIntegrationManager) SetupEventPublishing(customerCode, configSetName string, eventTypes []string) error {
	// Get customer credentials
	credentials, err := sim.CustomerManager.AssumeCustomerRole(customerCode, "ses")
	if err != nil {
		return fmt.Errorf("failed to assume customer role: %v", err)
	}

	// Simulate event publishing setup
	params := map[string]interface{}{
		"ConfigurationSetName": configSetName,
		"EventDestination": map[string]interface{}{
			"Name":               fmt.Sprintf("%s-events", customerCode),
			"Enabled":            true,
			"MatchingEventTypes": eventTypes,
			"CloudWatchDestination": map[string]interface{}{
				"DimensionConfigurations": []map[string]interface{}{
					{
						"DimensionName":         "CustomerCode",
						"DimensionValueSource":  "messageTag",
						"DefaultDimensionValue": customerCode,
					},
				},
			},
		},
	}

	_, err = sim.callSESAPI("CreateConfigurationSetEventDestination", params, credentials)
	return err
}
