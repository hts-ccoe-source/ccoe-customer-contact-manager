package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ConfigurationManager handles system configuration for multi-customer environments
type ConfigurationManager struct {
	ConfigPath      string
	Configuration   *SystemConfiguration
	CustomerManager *CustomerCredentialManager
	TemplateManager *EmailTemplateManager
	SESManager      *SESIntegrationManager
	ValidationRules map[string]ValidationRule
}

// SystemConfiguration represents the complete system configuration
type SystemConfiguration struct {
	Version          string                    `json:"version"`
	Environment      string                    `json:"environment"`
	Region           string                    `json:"region"`
	ServiceSettings  ServiceSettings           `json:"serviceSettings"`
	CustomerMappings map[string]CustomerConfig `json:"customerMappings"`
	EmailSettings    EmailConfiguration        `json:"emailSettings"`
	SQSSettings      SQSConfiguration          `json:"sqsSettings"`
	S3Settings       S3Configuration           `json:"s3Settings"`
	MonitoringConfig MonitoringConfiguration   `json:"monitoringConfig"`
	SecurityConfig   SecurityConfiguration     `json:"securityConfig"`
	FeatureFlags     map[string]bool           `json:"featureFlags"`
	CreatedAt        time.Time                 `json:"createdAt"`
	UpdatedAt        time.Time                 `json:"updatedAt"`
	Metadata         map[string]interface{}    `json:"metadata,omitempty"`
}

// ServiceSettings contains general service configuration
type ServiceSettings struct {
	ServiceName     string        `json:"serviceName"`
	ServiceVersion  string        `json:"serviceVersion"`
	DefaultTimeout  time.Duration `json:"defaultTimeout"`
	MaxRetries      int           `json:"maxRetries"`
	LogLevel        string        `json:"logLevel"`
	EnableMetrics   bool          `json:"enableMetrics"`
	EnableTracing   bool          `json:"enableTracing"`
	HealthCheckPath string        `json:"healthCheckPath"`
	MaintenanceMode bool          `json:"maintenanceMode"`
}

// CustomerConfig represents configuration for a specific customer
type CustomerConfig struct {
	CustomerCode  string                `json:"customerCode"`
	CustomerName  string                `json:"customerName"`
	AWSAccountID  string                `json:"awsAccountId"`
	Region        string                `json:"region"`
	Environment   string                `json:"environment"`
	RoleARNs      map[string]string     `json:"roleArns"`
	SQSQueueURLs  map[string]string     `json:"sqsQueueUrls"`
	S3Buckets     map[string]string     `json:"s3Buckets"`
	EmailSettings CustomerEmailSettings `json:"emailSettings"`
	Quotas        CustomerQuotas        `json:"quotas"`
	Features      map[string]bool       `json:"features"`
	Tags          map[string]string     `json:"tags"`
	ContactInfo   CustomerContactInfo   `json:"contactInfo"`
	Enabled       bool                  `json:"enabled"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
}

// CustomerEmailSettings contains email-specific settings for a customer
type CustomerEmailSettings struct {
	FromEmail            string            `json:"fromEmail"`
	FromName             string            `json:"fromName"`
	ReplyToEmail         string            `json:"replyToEmail"`
	ConfigurationSetName string            `json:"configurationSetName"`
	DefaultTemplateID    string            `json:"defaultTemplateId"`
	SendingQuota         int64             `json:"sendingQuota"`
	SendingRate          float64           `json:"sendingRate"`
	VerifiedDomains      []string          `json:"verifiedDomains"`
	VerifiedEmails       []string          `json:"verifiedEmails"`
	SuppressedAddresses  []string          `json:"suppressedAddresses"`
	CustomHeaders        map[string]string `json:"customHeaders"`
	EnableTracking       bool              `json:"enableTracking"`
	EnableSuppression    bool              `json:"enableSuppression"`
}

// CustomerQuotas defines resource quotas for a customer
type CustomerQuotas struct {
	MaxEmailsPerDay       int64 `json:"maxEmailsPerDay"`
	MaxEmailsPerHour      int64 `json:"maxEmailsPerHour"`
	MaxRecipientsPerEmail int   `json:"maxRecipientsPerEmail"`
	MaxTemplates          int   `json:"maxTemplates"`
	MaxS3Objects          int64 `json:"maxS3Objects"`
	MaxSQSMessages        int64 `json:"maxSqsMessages"`
}

// CustomerContactInfo contains customer contact information
type CustomerContactInfo struct {
	PrimaryContact   ContactPerson `json:"primaryContact"`
	TechnicalContact ContactPerson `json:"technicalContact"`
	BillingContact   ContactPerson `json:"billingContact"`
	EmergencyContact ContactPerson `json:"emergencyContact"`
}

// ContactPerson represents a contact person
type ContactPerson struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Role  string `json:"role"`
}

// EmailConfiguration contains global email settings
type EmailConfiguration struct {
	DefaultFromEmail        string            `json:"defaultFromEmail"`
	DefaultFromName         string            `json:"defaultFromName"`
	DefaultReplyToEmail     string            `json:"defaultReplyToEmail"`
	DefaultTemplateID       string            `json:"defaultTemplateId"`
	MaxRetries              int               `json:"maxRetries"`
	RetryDelay              time.Duration     `json:"retryDelay"`
	EnableBounceHandling    bool              `json:"enableBounceHandling"`
	EnableComplaintHandling bool              `json:"enableComplaintHandling"`
	GlobalSuppressList      []string          `json:"globalSuppressList"`
	AllowedDomains          []string          `json:"allowedDomains"`
	BlockedDomains          []string          `json:"blockedDomains"`
	CustomHeaders           map[string]string `json:"customHeaders"`
}

// SQSConfiguration contains SQS settings
type SQSConfiguration struct {
	DefaultVisibilityTimeout time.Duration `json:"defaultVisibilityTimeout"`
	DefaultMessageRetention  time.Duration `json:"defaultMessageRetention"`
	MaxReceiveCount          int           `json:"maxReceiveCount"`
	EnableDeadLetterQueue    bool          `json:"enableDeadLetterQueue"`
	PollingInterval          time.Duration `json:"pollingInterval"`
	BatchSize                int           `json:"batchSize"`
	WaitTimeSeconds          int           `json:"waitTimeSeconds"`
}

// S3Configuration contains S3 settings
type S3Configuration struct {
	DefaultBucket       string        `json:"defaultBucket"`
	DefaultPrefix       string        `json:"defaultPrefix"`
	EnableVersioning    bool          `json:"enableVersioning"`
	EnableEncryption    bool          `json:"enableEncryption"`
	LifecyclePolicies   []string      `json:"lifecyclePolicies"`
	DefaultStorageClass string        `json:"defaultStorageClass"`
	MaxObjectSize       int64         `json:"maxObjectSize"`
	AllowedFileTypes    []string      `json:"allowedFileTypes"`
	ScanForMalware      bool          `json:"scanForMalware"`
	RetentionPeriod     time.Duration `json:"retentionPeriod"`
}

// MonitoringConfiguration contains monitoring and observability settings
type MonitoringConfiguration struct {
	EnableCloudWatch    bool               `json:"enableCloudWatch"`
	EnableXRay          bool               `json:"enableXRay"`
	MetricsNamespace    string             `json:"metricsNamespace"`
	LogGroup            string             `json:"logGroup"`
	LogRetentionDays    int                `json:"logRetentionDays"`
	AlarmTopicARN       string             `json:"alarmTopicArn"`
	DashboardName       string             `json:"dashboardName"`
	CustomMetrics       []string           `json:"customMetrics"`
	AlertThresholds     map[string]float64 `json:"alertThresholds"`
	HealthCheckInterval time.Duration      `json:"healthCheckInterval"`
}

// SecurityConfiguration contains security settings
type SecurityConfiguration struct {
	EnableEncryption     bool           `json:"enableEncryption"`
	KMSKeyID             string         `json:"kmsKeyId"`
	EnableAuditLogging   bool           `json:"enableAuditLogging"`
	AuditLogBucket       string         `json:"auditLogBucket"`
	AllowedIPRanges      []string       `json:"allowedIpRanges"`
	RequireMFA           bool           `json:"requireMfa"`
	SessionTimeout       time.Duration  `json:"sessionTimeout"`
	PasswordPolicy       PasswordPolicy `json:"passwordPolicy"`
	APIRateLimit         int            `json:"apiRateLimit"`
	EnableCSRFProtection bool           `json:"enableCsrfProtection"`
}

// PasswordPolicy defines password requirements
type PasswordPolicy struct {
	MinLength        int  `json:"minLength"`
	RequireUppercase bool `json:"requireUppercase"`
	RequireLowercase bool `json:"requireLowercase"`
	RequireNumbers   bool `json:"requireNumbers"`
	RequireSymbols   bool `json:"requireSymbols"`
	MaxAge           int  `json:"maxAge"`       // days
	PreventReuse     int  `json:"preventReuse"` // number of previous passwords
}

// ValidationRule defines a configuration validation rule
type ValidationRule struct {
	Field         string        `json:"field"`
	Type          string        `json:"type"` // required, format, range, enum
	Pattern       string        `json:"pattern,omitempty"`
	MinValue      interface{}   `json:"minValue,omitempty"`
	MaxValue      interface{}   `json:"maxValue,omitempty"`
	AllowedValues []interface{} `json:"allowedValues,omitempty"`
	Message       string        `json:"message"`
	Severity      string        `json:"severity"` // error, warning, info
}

// NewConfigurationManager creates a new configuration manager
func NewConfigurationManager(configPath string) *ConfigurationManager {
	manager := &ConfigurationManager{
		ConfigPath:      configPath,
		ValidationRules: make(map[string]ValidationRule),
	}

	// Load default validation rules
	manager.loadDefaultValidationRules()

	return manager
}

// LoadConfiguration loads configuration from file
func (cm *ConfigurationManager) LoadConfiguration() error {
	if _, err := os.Stat(cm.ConfigPath); os.IsNotExist(err) {
		// Create default configuration if file doesn't exist
		return cm.CreateDefaultConfiguration()
	}

	data, err := os.ReadFile(cm.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %v", err)
	}

	var config SystemConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse configuration: %v", err)
	}

	cm.Configuration = &config

	// Validate configuration
	if err := cm.ValidateConfiguration(); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	return nil
}

// SaveConfiguration saves configuration to file
func (cm *ConfigurationManager) SaveConfiguration() error {
	if cm.Configuration == nil {
		return fmt.Errorf("no configuration to save")
	}

	// Update timestamp
	cm.Configuration.UpdatedAt = time.Now()

	// Validate before saving
	if err := cm.ValidateConfiguration(); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(cm.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Marshal configuration
	data, err := json.MarshalIndent(cm.Configuration, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %v", err)
	}

	// Write to file
	if err := os.WriteFile(cm.ConfigPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %v", err)
	}

	return nil
}

// CreateDefaultConfiguration creates a default configuration
func (cm *ConfigurationManager) CreateDefaultConfiguration() error {
	config := &SystemConfiguration{
		Version:     "1.0.0",
		Environment: "development",
		Region:      "us-east-1",
		ServiceSettings: ServiceSettings{
			ServiceName:     "multi-customer-email-distribution",
			ServiceVersion:  "1.0.0",
			DefaultTimeout:  30 * time.Second,
			MaxRetries:      3,
			LogLevel:        "info",
			EnableMetrics:   true,
			EnableTracing:   false,
			HealthCheckPath: "/health",
			MaintenanceMode: false,
		},
		CustomerMappings: make(map[string]CustomerConfig),
		EmailSettings: EmailConfiguration{
			DefaultFromEmail:        "noreply@example.com",
			DefaultFromName:         "Email Distribution System",
			DefaultReplyToEmail:     "support@example.com",
			DefaultTemplateID:       "notification",
			MaxRetries:              3,
			RetryDelay:              2 * time.Second,
			EnableBounceHandling:    true,
			EnableComplaintHandling: true,
			GlobalSuppressList:      []string{},
			AllowedDomains:          []string{},
			BlockedDomains:          []string{"tempmail.com", "10minutemail.com"},
			CustomHeaders:           make(map[string]string),
		},
		SQSSettings: SQSConfiguration{
			DefaultVisibilityTimeout: 30 * time.Second,
			DefaultMessageRetention:  14 * 24 * time.Hour,
			MaxReceiveCount:          3,
			EnableDeadLetterQueue:    true,
			PollingInterval:          5 * time.Second,
			BatchSize:                10,
			WaitTimeSeconds:          20,
		},
		S3Settings: S3Configuration{
			DefaultBucket:       "email-distribution-metadata",
			DefaultPrefix:       "customers/",
			EnableVersioning:    true,
			EnableEncryption:    true,
			LifecyclePolicies:   []string{"delete-after-90-days"},
			DefaultStorageClass: "STANDARD",
			MaxObjectSize:       10 * 1024 * 1024, // 10MB
			AllowedFileTypes:    []string{".json", ".csv", ".txt"},
			ScanForMalware:      true,
			RetentionPeriod:     90 * 24 * time.Hour,
		},
		MonitoringConfig: MonitoringConfiguration{
			EnableCloudWatch:    true,
			EnableXRay:          false,
			MetricsNamespace:    "EmailDistribution",
			LogGroup:            "/aws/lambda/email-distribution",
			LogRetentionDays:    30,
			AlarmTopicARN:       "",
			DashboardName:       "EmailDistributionDashboard",
			CustomMetrics:       []string{"EmailsSent", "EmailsDelivered", "EmailsBounced"},
			AlertThresholds:     map[string]float64{"ErrorRate": 5.0, "LatencyP99": 1000.0},
			HealthCheckInterval: 1 * time.Minute,
		},
		SecurityConfig: SecurityConfiguration{
			EnableEncryption:   true,
			KMSKeyID:           "",
			EnableAuditLogging: true,
			AuditLogBucket:     "email-distribution-audit-logs",
			AllowedIPRanges:    []string{},
			RequireMFA:         false,
			SessionTimeout:     8 * time.Hour,
			PasswordPolicy: PasswordPolicy{
				MinLength:        12,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
				RequireSymbols:   true,
				MaxAge:           90,
				PreventReuse:     5,
			},
			APIRateLimit:         1000,
			EnableCSRFProtection: true,
		},
		FeatureFlags: map[string]bool{
			"enableBulkEmail":       true,
			"enableTemplatePreview": true,
			"enableAdvancedMetrics": false,
			"enableA/BTesting":      false,
			"enableEmailScheduling": false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	cm.Configuration = config
	return cm.SaveConfiguration()
}

// AddCustomer adds a new customer configuration
func (cm *ConfigurationManager) AddCustomer(customerConfig CustomerConfig) error {
	if cm.Configuration == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Validate customer configuration
	if err := cm.validateCustomerConfig(&customerConfig); err != nil {
		return fmt.Errorf("invalid customer configuration: %v", err)
	}

	// Set timestamps
	customerConfig.CreatedAt = time.Now()
	customerConfig.UpdatedAt = time.Now()

	// Add to configuration
	cm.Configuration.CustomerMappings[customerConfig.CustomerCode] = customerConfig

	return cm.SaveConfiguration()
}

// UpdateCustomer updates an existing customer configuration
func (cm *ConfigurationManager) UpdateCustomer(customerCode string, updates CustomerConfig) error {
	if cm.Configuration == nil {
		return fmt.Errorf("configuration not loaded")
	}

	existing, exists := cm.Configuration.CustomerMappings[customerCode]
	if !exists {
		return fmt.Errorf("customer not found: %s", customerCode)
	}

	// Preserve creation time and update modification time
	updates.CreatedAt = existing.CreatedAt
	updates.UpdatedAt = time.Now()

	// Validate updated configuration
	if err := cm.validateCustomerConfig(&updates); err != nil {
		return fmt.Errorf("invalid customer configuration: %v", err)
	}

	// Update configuration
	cm.Configuration.CustomerMappings[customerCode] = updates

	return cm.SaveConfiguration()
}

// RemoveCustomer removes a customer configuration
func (cm *ConfigurationManager) RemoveCustomer(customerCode string) error {
	if cm.Configuration == nil {
		return fmt.Errorf("configuration not loaded")
	}

	if _, exists := cm.Configuration.CustomerMappings[customerCode]; !exists {
		return fmt.Errorf("customer not found: %s", customerCode)
	}

	delete(cm.Configuration.CustomerMappings, customerCode)

	return cm.SaveConfiguration()
}

// GetCustomer retrieves a customer configuration
func (cm *ConfigurationManager) GetCustomer(customerCode string) (*CustomerConfig, error) {
	if cm.Configuration == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}

	customer, exists := cm.Configuration.CustomerMappings[customerCode]
	if !exists {
		return nil, fmt.Errorf("customer not found: %s", customerCode)
	}

	return &customer, nil
}

// ListCustomers returns all customer configurations
func (cm *ConfigurationManager) ListCustomers() map[string]CustomerConfig {
	if cm.Configuration == nil {
		return make(map[string]CustomerConfig)
	}

	return cm.Configuration.CustomerMappings
}

// ValidateConfiguration validates the entire configuration
func (cm *ConfigurationManager) ValidateConfiguration() error {
	if cm.Configuration == nil {
		return fmt.Errorf("no configuration to validate")
	}

	var errors []string

	// Validate service settings
	if err := cm.validateServiceSettings(&cm.Configuration.ServiceSettings); err != nil {
		errors = append(errors, fmt.Sprintf("service settings: %v", err))
	}

	// Validate email settings
	if err := cm.validateEmailSettings(&cm.Configuration.EmailSettings); err != nil {
		errors = append(errors, fmt.Sprintf("email settings: %v", err))
	}

	// Validate customer configurations
	for customerCode, customerConfig := range cm.Configuration.CustomerMappings {
		if err := cm.validateCustomerConfig(&customerConfig); err != nil {
			errors = append(errors, fmt.Sprintf("customer %s: %v", customerCode, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// validateServiceSettings validates service settings
func (cm *ConfigurationManager) validateServiceSettings(settings *ServiceSettings) error {
	if settings.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}

	if settings.ServiceVersion == "" {
		return fmt.Errorf("service version is required")
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, settings.LogLevel) {
		return fmt.Errorf("invalid log level: %s (must be one of: %s)",
			settings.LogLevel, strings.Join(validLogLevels, ", "))
	}

	if settings.DefaultTimeout <= 0 {
		return fmt.Errorf("default timeout must be positive")
	}

	if settings.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	return nil
}

// validateEmailSettings validates email settings
func (cm *ConfigurationManager) validateEmailSettings(settings *EmailConfiguration) error {
	if settings.DefaultFromEmail == "" {
		return fmt.Errorf("default from email is required")
	}

	if !isValidEmail(settings.DefaultFromEmail) {
		return fmt.Errorf("invalid default from email format")
	}

	if settings.DefaultReplyToEmail != "" && !isValidEmail(settings.DefaultReplyToEmail) {
		return fmt.Errorf("invalid default reply-to email format")
	}

	if settings.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	if settings.RetryDelay <= 0 {
		return fmt.Errorf("retry delay must be positive")
	}

	return nil
}

// validateCustomerConfig validates a customer configuration
func (cm *ConfigurationManager) validateCustomerConfig(config *CustomerConfig) error {
	if config.CustomerCode == "" {
		return fmt.Errorf("customer code is required")
	}

	if !isValidCustomerCode(config.CustomerCode) {
		return fmt.Errorf("invalid customer code format")
	}

	if config.CustomerName == "" {
		return fmt.Errorf("customer name is required")
	}

	if config.AWSAccountID == "" {
		return fmt.Errorf("AWS account ID is required")
	}

	if !isValidAWSAccountID(config.AWSAccountID) {
		return fmt.Errorf("invalid AWS account ID format")
	}

	if config.Region == "" {
		return fmt.Errorf("region is required")
	}

	if !isValidAWSRegion(config.Region) {
		return fmt.Errorf("invalid AWS region")
	}

	// Validate email settings
	if config.EmailSettings.FromEmail != "" && !isValidEmail(config.EmailSettings.FromEmail) {
		return fmt.Errorf("invalid from email format")
	}

	if config.EmailSettings.ReplyToEmail != "" && !isValidEmail(config.EmailSettings.ReplyToEmail) {
		return fmt.Errorf("invalid reply-to email format")
	}

	// Validate contact information
	if err := cm.validateContactInfo(&config.ContactInfo); err != nil {
		return fmt.Errorf("invalid contact info: %v", err)
	}

	return nil
}

// validateContactInfo validates customer contact information
func (cm *ConfigurationManager) validateContactInfo(contactInfo *CustomerContactInfo) error {
	contacts := map[string]ContactPerson{
		"primary":   contactInfo.PrimaryContact,
		"technical": contactInfo.TechnicalContact,
		"billing":   contactInfo.BillingContact,
		"emergency": contactInfo.EmergencyContact,
	}

	for contactType, contact := range contacts {
		if contact.Email != "" && !isValidEmail(contact.Email) {
			return fmt.Errorf("invalid %s contact email format", contactType)
		}

		if contact.Phone != "" && !isValidPhoneNumber(contact.Phone) {
			return fmt.Errorf("invalid %s contact phone format", contactType)
		}
	}

	return nil
}

// loadDefaultValidationRules loads default validation rules
func (cm *ConfigurationManager) loadDefaultValidationRules() {
	cm.ValidationRules = map[string]ValidationRule{
		"customerCode": {
			Field:    "customerCode",
			Type:     "format",
			Pattern:  "^[a-z0-9-]{2,20}$",
			Message:  "Customer code must be 2-20 characters, lowercase alphanumeric with hyphens",
			Severity: "error",
		},
		"awsAccountId": {
			Field:    "awsAccountId",
			Type:     "format",
			Pattern:  "^[0-9]{12}$",
			Message:  "AWS Account ID must be exactly 12 digits",
			Severity: "error",
		},
		"email": {
			Field:    "email",
			Type:     "format",
			Pattern:  "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
			Message:  "Invalid email format",
			Severity: "error",
		},
		"region": {
			Field: "region",
			Type:  "enum",
			AllowedValues: []interface{}{
				"us-east-1", "us-east-2", "us-west-1", "us-west-2",
				"eu-west-1", "eu-west-2", "eu-central-1",
				"ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
			},
			Message:  "Invalid AWS region",
			Severity: "error",
		},
		"environment": {
			Field: "environment",
			Type:  "enum",
			AllowedValues: []interface{}{
				"development", "staging", "production",
			},
			Message:  "Environment must be development, staging, or production",
			Severity: "error",
		},
	}
}

// GetFeatureFlag returns the value of a feature flag
func (cm *ConfigurationManager) GetFeatureFlag(flagName string) bool {
	if cm.Configuration == nil {
		return false
	}

	value, exists := cm.Configuration.FeatureFlags[flagName]
	return exists && value
}

// SetFeatureFlag sets the value of a feature flag
func (cm *ConfigurationManager) SetFeatureFlag(flagName string, enabled bool) error {
	if cm.Configuration == nil {
		return fmt.Errorf("configuration not loaded")
	}

	cm.Configuration.FeatureFlags[flagName] = enabled
	return cm.SaveConfiguration()
}

// GetEnvironmentConfig returns environment-specific configuration
func (cm *ConfigurationManager) GetEnvironmentConfig() map[string]interface{} {
	if cm.Configuration == nil {
		return make(map[string]interface{})
	}

	envConfig := map[string]interface{}{
		"environment":    cm.Configuration.Environment,
		"region":         cm.Configuration.Region,
		"serviceName":    cm.Configuration.ServiceSettings.ServiceName,
		"serviceVersion": cm.Configuration.ServiceSettings.ServiceVersion,
		"logLevel":       cm.Configuration.ServiceSettings.LogLevel,
		"enableMetrics":  cm.Configuration.ServiceSettings.EnableMetrics,
		"enableTracing":  cm.Configuration.ServiceSettings.EnableTracing,
		"featureFlags":   cm.Configuration.FeatureFlags,
	}

	return envConfig
}

// ExportConfiguration exports configuration to JSON
func (cm *ConfigurationManager) ExportConfiguration() ([]byte, error) {
	if cm.Configuration == nil {
		return nil, fmt.Errorf("no configuration to export")
	}

	return json.MarshalIndent(cm.Configuration, "", "  ")
}

// ImportConfiguration imports configuration from JSON
func (cm *ConfigurationManager) ImportConfiguration(data []byte) error {
	var config SystemConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse configuration: %v", err)
	}

	cm.Configuration = &config

	// Validate imported configuration
	if err := cm.ValidateConfiguration(); err != nil {
		return fmt.Errorf("imported configuration is invalid: %v", err)
	}

	return cm.SaveConfiguration()
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

func isValidPhoneNumber(phone string) bool {
	// Simple phone number validation (can be enhanced)
	re := regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
	return re.MatchString(strings.ReplaceAll(phone, " ", ""))
}

// These functions are already defined in customer_credential_manager.go
// but included here for completeness in case this file is used standalone

func isValidCustomerCode(code string) bool {
	if len(code) < 2 || len(code) > 20 {
		return false
	}
	re := regexp.MustCompile(`^[a-z0-9-]+$`)
	return re.MatchString(code)
}

func isValidAWSAccountID(accountID string) bool {
	if len(accountID) != 12 {
		return false
	}
	re := regexp.MustCompile(`^[0-9]{12}$`)
	return re.MatchString(accountID)
}

func isValidAWSRegion(region string) bool {
	validRegions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-central-1", "eu-north-1",
		"ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-northeast-2",
		"ap-south-1", "ca-central-1", "sa-east-1",
	}

	for _, validRegion := range validRegions {
		if region == validRegion {
			return true
		}
	}
	return false
}
