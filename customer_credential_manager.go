package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// CustomerCredentialManager handles AWS credential management for multi-customer operations
type CustomerCredentialManager struct {
	CustomerMappings map[string]CustomerAccountInfo
	DefaultRegion    string
	AssumeRoleConfig AssumeRoleConfig
}

// CustomerAccountInfo contains AWS account information for a customer
type CustomerAccountInfo struct {
	CustomerCode string `json:"customerCode"`
	CustomerName string `json:"customerName"`
	AWSAccountID string `json:"awsAccountId"`
	Region       string `json:"region"`
	SESRoleARN   string `json:"sesRoleArn"`
	SQSRoleARN   string `json:"sqsRoleArn"`
	S3RoleARN    string `json:"s3RoleArn"`
	Environment  string `json:"environment"` // prod, staging, dev
}

// AssumeRoleConfig contains configuration for role assumption
type AssumeRoleConfig struct {
	SessionName     string        `json:"sessionName"`
	DurationSeconds int32         `json:"durationSeconds"`
	ExternalID      string        `json:"externalId,omitempty"`
	MFASerialNumber string        `json:"mfaSerialNumber,omitempty"`
	TokenCode       string        `json:"tokenCode,omitempty"`
	RetryAttempts   int           `json:"retryAttempts"`
	RetryDelay      time.Duration `json:"retryDelay"`
}

// AWSCredentials represents temporary AWS credentials
type AWSCredentials struct {
	AccessKeyID     string    `json:"accessKeyId"`
	SecretAccessKey string    `json:"secretAccessKey"`
	SessionToken    string    `json:"sessionToken"`
	Expiration      time.Time `json:"expiration"`
	Region          string    `json:"region"`
	CustomerCode    string    `json:"customerCode"`
	RoleARN         string    `json:"roleArn"`
}

// CredentialValidationResult represents the result of credential validation
type CredentialValidationResult struct {
	CustomerCode string    `json:"customerCode"`
	Valid        bool      `json:"valid"`
	Error        string    `json:"error,omitempty"`
	ValidatedAt  time.Time `json:"validatedAt"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	AccountID    string    `json:"accountId,omitempty"`
	UserARN      string    `json:"userArn,omitempty"`
}

// NewCustomerCredentialManager creates a new credential manager
func NewCustomerCredentialManager(defaultRegion string) *CustomerCredentialManager {
	return &CustomerCredentialManager{
		CustomerMappings: make(map[string]CustomerAccountInfo),
		DefaultRegion:    defaultRegion,
		AssumeRoleConfig: AssumeRoleConfig{
			SessionName:     "multi-customer-email-distribution",
			DurationSeconds: 3600, // 1 hour
			RetryAttempts:   3,
			RetryDelay:      2 * time.Second,
		},
	}
}

// LoadCustomerMappings loads customer account mappings from configuration
func (c *CustomerCredentialManager) LoadCustomerMappings(configData map[string]interface{}) error {
	log.Printf("Loading customer account mappings...")

	// Parse customer mappings from configuration
	mappings, ok := configData["customerMappings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("customerMappings not found in configuration")
	}

	for customerCode, mappingData := range mappings {
		mapping, ok := mappingData.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid mapping data for customer %s", customerCode)
		}

		accountInfo := CustomerAccountInfo{
			CustomerCode: customerCode,
			CustomerName: getStringValue(mapping, "customerName", strings.ToUpper(customerCode)),
			AWSAccountID: getStringValue(mapping, "awsAccountId", ""),
			Region:       getStringValue(mapping, "region", c.DefaultRegion),
			SESRoleARN:   getStringValue(mapping, "sesRoleArn", ""),
			SQSRoleARN:   getStringValue(mapping, "sqsRoleArn", ""),
			S3RoleARN:    getStringValue(mapping, "s3RoleArn", ""),
			Environment:  getStringValue(mapping, "environment", "production"),
		}

		// Validate required fields
		if accountInfo.AWSAccountID == "" {
			return fmt.Errorf("awsAccountId is required for customer %s", customerCode)
		}

		if accountInfo.SESRoleARN == "" {
			return fmt.Errorf("sesRoleArn is required for customer %s", customerCode)
		}

		c.CustomerMappings[customerCode] = accountInfo
		log.Printf("Loaded mapping for customer %s: %s (%s)",
			customerCode, accountInfo.CustomerName, accountInfo.AWSAccountID)
	}

	log.Printf("Successfully loaded %d customer mappings", len(c.CustomerMappings))
	return nil
}

// GetCustomerAccountInfo retrieves account information for a customer
func (c *CustomerCredentialManager) GetCustomerAccountInfo(customerCode string) (*CustomerAccountInfo, error) {
	accountInfo, exists := c.CustomerMappings[customerCode]
	if !exists {
		return nil, fmt.Errorf("no account mapping found for customer code: %s", customerCode)
	}

	return &accountInfo, nil
}

// AssumeCustomerRole assumes an IAM role for customer-specific operations
func (c *CustomerCredentialManager) AssumeCustomerRole(customerCode, serviceType string) (*AWSCredentials, error) {
	// Get customer account info
	accountInfo, err := c.GetCustomerAccountInfo(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info for customer %s: %v", customerCode, err)
	}

	// Determine which role ARN to use based on service type
	roleARN, err := c.getRoleARNForService(accountInfo, serviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get role ARN for service %s: %v", serviceType, err)
	}

	log.Printf("Assuming role %s for customer %s (%s service)",
		roleARN, customerCode, serviceType)

	// Simulate role assumption (in real implementation, would use AWS STS)
	credentials, err := c.simulateAssumeRole(roleARN, accountInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role %s: %v", roleARN, err)
	}

	log.Printf("Successfully assumed role for customer %s, expires at %v",
		customerCode, credentials.Expiration)

	return credentials, nil
}

// getRoleARNForService determines the appropriate role ARN for a service type
func (c *CustomerCredentialManager) getRoleARNForService(accountInfo *CustomerAccountInfo, serviceType string) (string, error) {
	switch strings.ToLower(serviceType) {
	case "ses", "email":
		if accountInfo.SESRoleARN == "" {
			return "", fmt.Errorf("SES role ARN not configured for customer %s", accountInfo.CustomerCode)
		}
		return accountInfo.SESRoleARN, nil

	case "sqs", "queue":
		if accountInfo.SQSRoleARN == "" {
			return "", fmt.Errorf("SQS role ARN not configured for customer %s", accountInfo.CustomerCode)
		}
		return accountInfo.SQSRoleARN, nil

	case "s3", "storage":
		if accountInfo.S3RoleARN == "" {
			return "", fmt.Errorf("S3 role ARN not configured for customer %s", accountInfo.CustomerCode)
		}
		return accountInfo.S3RoleARN, nil

	default:
		return "", fmt.Errorf("unsupported service type: %s", serviceType)
	}
}

// simulateAssumeRole simulates AWS STS AssumeRole operation
func (c *CustomerCredentialManager) simulateAssumeRole(roleARN string, accountInfo *CustomerAccountInfo) (*AWSCredentials, error) {
	// Simulate network delay
	time.Sleep(100 * time.Millisecond)

	// Simulate occasional failures (5% failure rate)
	if time.Now().UnixNano()%20 == 0 {
		return nil, fmt.Errorf("simulated STS assume role failure for %s", roleARN)
	}

	// In real implementation, this would be:
	// stsClient := sts.NewFromConfig(cfg)
	// result, err := stsClient.AssumeRole(context.TODO(), &sts.AssumeRoleInput{
	//     RoleArn:         aws.String(roleARN),
	//     RoleSessionName: aws.String(c.AssumeRoleConfig.SessionName),
	//     DurationSeconds: aws.Int32(c.AssumeRoleConfig.DurationSeconds),
	//     ExternalId:      aws.String(c.AssumeRoleConfig.ExternalID),
	// })
	// if err != nil {
	//     return nil, err
	// }
	//
	// return &AWSCredentials{
	//     AccessKeyID:     *result.Credentials.AccessKeyId,
	//     SecretAccessKey: *result.Credentials.SecretAccessKey,
	//     SessionToken:    *result.Credentials.SessionToken,
	//     Expiration:      *result.Credentials.Expiration,
	//     Region:          accountInfo.Region,
	//     CustomerCode:    accountInfo.CustomerCode,
	//     RoleARN:         roleARN,
	// }, nil

	// Simulate successful credential generation
	expiration := time.Now().Add(time.Duration(c.AssumeRoleConfig.DurationSeconds) * time.Second)

	return &AWSCredentials{
		AccessKeyID:     fmt.Sprintf("ASIA%s%d", strings.ToUpper(accountInfo.CustomerCode), time.Now().Unix()),
		SecretAccessKey: fmt.Sprintf("simulated-secret-key-%s", accountInfo.CustomerCode),
		SessionToken:    fmt.Sprintf("simulated-session-token-%s-%d", accountInfo.CustomerCode, time.Now().Unix()),
		Expiration:      expiration,
		Region:          accountInfo.Region,
		CustomerCode:    accountInfo.CustomerCode,
		RoleARN:         roleARN,
	}, nil
}

// ValidateCredentials validates AWS credentials for a customer
func (c *CustomerCredentialManager) ValidateCredentials(credentials *AWSCredentials) (*CredentialValidationResult, error) {
	result := &CredentialValidationResult{
		CustomerCode: credentials.CustomerCode,
		ValidatedAt:  time.Now(),
		Valid:        false,
	}

	// Check if credentials are expired
	if time.Now().After(credentials.Expiration) {
		result.Error = "credentials have expired"
		return result, fmt.Errorf("credentials expired at %v", credentials.Expiration)
	}

	// Simulate credential validation (in real implementation, would call STS GetCallerIdentity)
	time.Sleep(50 * time.Millisecond)

	// Simulate occasional validation failures (2% failure rate)
	if time.Now().UnixNano()%50 == 0 {
		result.Error = "simulated credential validation failure"
		return result, fmt.Errorf("credential validation failed")
	}

	// In real implementation, this would be:
	// stsClient := sts.NewFromConfig(cfg)
	// identity, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	// if err != nil {
	//     result.Error = err.Error()
	//     return result, err
	// }
	//
	// result.AccountID = *identity.Account
	// result.UserARN = *identity.Arn

	// Simulate successful validation
	result.Valid = true
	result.ExpiresAt = credentials.Expiration
	result.AccountID = extractAccountIDFromRoleARN(credentials.RoleARN)
	result.UserARN = credentials.RoleARN

	log.Printf("Credentials validated for customer %s, expires at %v",
		credentials.CustomerCode, result.ExpiresAt)

	return result, nil
}

// RefreshCredentials refreshes expired or soon-to-expire credentials
func (c *CustomerCredentialManager) RefreshCredentials(credentials *AWSCredentials, serviceType string) (*AWSCredentials, error) {
	// Check if refresh is needed (refresh if expiring within 5 minutes)
	refreshThreshold := time.Now().Add(5 * time.Minute)
	if credentials.Expiration.After(refreshThreshold) {
		log.Printf("Credentials for customer %s still valid until %v, no refresh needed",
			credentials.CustomerCode, credentials.Expiration)
		return credentials, nil
	}

	log.Printf("Refreshing credentials for customer %s (expires at %v)",
		credentials.CustomerCode, credentials.Expiration)

	// Assume role again to get fresh credentials
	return c.AssumeCustomerRole(credentials.CustomerCode, serviceType)
}

// DetermineCustomerAccount determines AWS account ID from customer code
func (c *CustomerCredentialManager) DetermineCustomerAccount(customerCode string) (string, error) {
	accountInfo, err := c.GetCustomerAccountInfo(customerCode)
	if err != nil {
		return "", err
	}

	return accountInfo.AWSAccountID, nil
}

// CreateAWSClientConfig creates AWS client configuration with customer credentials
func (c *CustomerCredentialManager) CreateAWSClientConfig(credentials *AWSCredentials, serviceType string) (map[string]interface{}, error) {
	// Validate credentials before creating client config
	validation, err := c.ValidateCredentials(credentials)
	if err != nil {
		return nil, fmt.Errorf("credential validation failed: %v", err)
	}

	if !validation.Valid {
		return nil, fmt.Errorf("credentials are not valid: %s", validation.Error)
	}

	// In real implementation, this would create AWS SDK config:
	// cfg, err := config.LoadDefaultConfig(context.TODO(),
	//     config.WithRegion(credentials.Region),
	//     config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
	//         credentials.AccessKeyID,
	//         credentials.SecretAccessKey,
	//         credentials.SessionToken,
	//     )),
	// )

	// Return simulated client configuration
	clientConfig := map[string]interface{}{
		"region":          credentials.Region,
		"accessKeyId":     credentials.AccessKeyID,
		"secretAccessKey": credentials.SecretAccessKey,
		"sessionToken":    credentials.SessionToken,
		"customerCode":    credentials.CustomerCode,
		"serviceType":     serviceType,
		"expiresAt":       credentials.Expiration,
	}

	log.Printf("Created AWS client config for customer %s (%s service)",
		credentials.CustomerCode, serviceType)

	return clientConfig, nil
}

// GetAllCustomerAccounts returns all configured customer account IDs
func (c *CustomerCredentialManager) GetAllCustomerAccounts() map[string]string {
	accounts := make(map[string]string)

	for customerCode, accountInfo := range c.CustomerMappings {
		accounts[customerCode] = accountInfo.AWSAccountID
	}

	return accounts
}

// ValidateAllCustomerCredentials validates credentials for all customers
func (c *CustomerCredentialManager) ValidateAllCustomerCredentials(serviceType string) (map[string]*CredentialValidationResult, error) {
	results := make(map[string]*CredentialValidationResult)

	log.Printf("Validating credentials for all %d customers (%s service)",
		len(c.CustomerMappings), serviceType)

	for customerCode := range c.CustomerMappings {
		// Assume role for customer
		credentials, err := c.AssumeCustomerRole(customerCode, serviceType)
		if err != nil {
			results[customerCode] = &CredentialValidationResult{
				CustomerCode: customerCode,
				Valid:        false,
				Error:        err.Error(),
				ValidatedAt:  time.Now(),
			}
			continue
		}

		// Validate credentials
		validation, err := c.ValidateCredentials(credentials)
		if err != nil {
			validation = &CredentialValidationResult{
				CustomerCode: customerCode,
				Valid:        false,
				Error:        err.Error(),
				ValidatedAt:  time.Now(),
			}
		}

		results[customerCode] = validation
	}

	// Count successful validations
	successCount := 0
	for _, result := range results {
		if result.Valid {
			successCount++
		}
	}

	log.Printf("Credential validation complete: %d/%d customers successful",
		successCount, len(c.CustomerMappings))

	return results, nil
}

// Helper functions

func getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}

func extractAccountIDFromRoleARN(roleARN string) string {
	// Extract account ID from role ARN: arn:aws:iam::123456789012:role/RoleName
	parts := strings.Split(roleARN, ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return "unknown"
}

// Configuration validation functions

// ValidateCustomerMappingConfig validates the customer mapping configuration
func (c *CustomerCredentialManager) ValidateCustomerMappingConfig() error {
	if len(c.CustomerMappings) == 0 {
		return fmt.Errorf("no customer mappings configured")
	}

	for customerCode, accountInfo := range c.CustomerMappings {
		// Validate customer code format
		if !isValidCustomerCode(customerCode) {
			return fmt.Errorf("invalid customer code format: %s", customerCode)
		}

		// Validate AWS account ID format
		if !isValidAWSAccountID(accountInfo.AWSAccountID) {
			return fmt.Errorf("invalid AWS account ID for customer %s: %s",
				customerCode, accountInfo.AWSAccountID)
		}

		// Validate role ARN format
		if !isValidRoleARN(accountInfo.SESRoleARN) {
			return fmt.Errorf("invalid SES role ARN for customer %s: %s",
				customerCode, accountInfo.SESRoleARN)
		}

		// Validate region
		if !isValidAWSRegion(accountInfo.Region) {
			return fmt.Errorf("invalid AWS region for customer %s: %s",
				customerCode, accountInfo.Region)
		}
	}

	return nil
}

// Validation helper functions

func isValidCustomerCode(customerCode string) bool {
	// Customer codes should be lowercase alphanumeric with hyphens
	if len(customerCode) == 0 || len(customerCode) > 20 {
		return false
	}

	for _, char := range customerCode {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return false
		}
	}

	return true
}

func isValidAWSAccountID(accountID string) bool {
	// AWS account IDs are 12-digit numbers
	if len(accountID) != 12 {
		return false
	}

	for _, char := range accountID {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

func isValidRoleARN(roleARN string) bool {
	// Role ARN format: arn:aws:iam::123456789012:role/RoleName
	if !strings.HasPrefix(roleARN, "arn:aws:iam::") {
		return false
	}

	if !strings.Contains(roleARN, ":role/") {
		return false
	}

	parts := strings.Split(roleARN, ":")
	if len(parts) != 6 {
		return false
	}

	// Validate account ID in ARN
	accountID := parts[4]
	return isValidAWSAccountID(accountID)
}

func isValidAWSRegion(region string) bool {
	// Common AWS regions
	validRegions := map[string]bool{
		"us-east-1":      true,
		"us-east-2":      true,
		"us-west-1":      true,
		"us-west-2":      true,
		"eu-west-1":      true,
		"eu-west-2":      true,
		"eu-central-1":   true,
		"ap-southeast-1": true,
		"ap-southeast-2": true,
		"ap-northeast-1": true,
	}

	return validRegions[region]
}
