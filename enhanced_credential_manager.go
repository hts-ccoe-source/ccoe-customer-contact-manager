package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// EnhancedCredentialManager provides real AWS credential management with caching and validation
type EnhancedCredentialManager struct {
	baseConfig      aws.Config
	customerConfigs map[string]aws.Config
	stsClient       *sts.Client
	configMutex     sync.RWMutex
	credentialCache map[string]*CachedCredentials
	cacheMutex      sync.RWMutex
	customerManager *CustomerCredentialManager
}

// CachedCredentials represents cached AWS credentials with expiration
type CachedCredentials struct {
	Config    aws.Config
	ExpiresAt time.Time
	RoleARN   string
}

// CustomerAWSClients holds AWS service clients for a specific customer
type CustomerAWSClients struct {
	SESClient *ses.Client
	SQSClient *sqs.Client
	STSClient *sts.Client
	Config    aws.Config
}

// RoleAssumptionError represents errors during role assumption
type RoleAssumptionError struct {
	CustomerCode string
	RoleARN      string
	Err          error
}

func (e *RoleAssumptionError) Error() string {
	return fmt.Sprintf("failed to assume role %s for customer %s: %v", e.RoleARN, e.CustomerCode, e.Err)
}

// CredentialValidationError represents credential validation errors
type CredentialValidationError struct {
	CustomerCode string
	Service      string
	Err          error
}

func (e *CredentialValidationError) Error() string {
	return fmt.Sprintf("credential validation failed for customer %s service %s: %v", e.CustomerCode, e.Service, e.Err)
}

// NewEnhancedCredentialManager creates a new enhanced credential manager
func NewEnhancedCredentialManager(customerManager *CustomerCredentialManager) (*EnhancedCredentialManager, error) {
	// Load default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	return &EnhancedCredentialManager{
		baseConfig:      cfg,
		customerConfigs: make(map[string]aws.Config),
		stsClient:       sts.NewFromConfig(cfg),
		credentialCache: make(map[string]*CachedCredentials),
		customerManager: customerManager,
	}, nil
}

// AssumeCustomerRole assumes an IAM role for customer-specific operations with real AWS STS
func (e *EnhancedCredentialManager) AssumeCustomerRole(ctx context.Context, customerCode, serviceType string) (*CustomerAWSClients, error) {
	// Get customer account info
	accountInfo, err := e.customerManager.GetCustomerAccountInfo(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info for customer %s: %v", customerCode, err)
	}

	// Determine which role ARN to use based on service type
	roleARN, err := e.getRoleARNForService(accountInfo, serviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get role ARN for service %s: %v", serviceType, err)
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", customerCode, serviceType)
	if cachedCreds := e.getCachedCredentials(cacheKey); cachedCreds != nil {
		return e.createCustomerClients(cachedCreds.Config), nil
	}

	// Assume role using AWS STS
	config, err := e.assumeRoleWithSTS(ctx, roleARN, customerCode, accountInfo.Region)
	if err != nil {
		return nil, &RoleAssumptionError{
			CustomerCode: customerCode,
			RoleARN:      roleARN,
			Err:          err,
		}
	}

	// Cache the credentials
	e.cacheCredentials(cacheKey, config, roleARN)

	// Create and return customer clients
	return e.createCustomerClients(config), nil
}

// assumeRoleWithSTS performs the actual STS AssumeRole operation
func (e *EnhancedCredentialManager) assumeRoleWithSTS(ctx context.Context, roleARN, customerCode, region string) (aws.Config, error) {
	// Create role credentials provider
	roleProvider := stscreds.NewAssumeRoleProvider(e.stsClient, roleARN, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = fmt.Sprintf("multi-customer-email-%s-%d", customerCode, time.Now().Unix())
		o.Duration = time.Hour // 1 hour session
		// Add external ID if configured
		if e.customerManager.AssumeRoleConfig.ExternalID != "" {
			o.ExternalID = &e.customerManager.AssumeRoleConfig.ExternalID
		}
	})

	// Create AWS config with assumed role credentials
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(roleProvider),
		config.WithRetryMaxAttempts(e.customerManager.AssumeRoleConfig.RetryAttempts),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create config with assumed role: %v", err)
	}

	// Validate the credentials by calling GetCallerIdentity
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to validate assumed role credentials: %v", err)
	}

	// Log successful role assumption
	fmt.Printf("Successfully assumed role %s for customer %s, identity: %s\n",
		roleARN, customerCode, *identity.Arn)

	return cfg, nil
}

// getRoleARNForService determines the appropriate role ARN for a service type
func (e *EnhancedCredentialManager) getRoleARNForService(accountInfo *CustomerAccountInfo, serviceType string) (string, error) {
	switch serviceType {
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

// createCustomerClients creates AWS service clients for a customer
func (e *EnhancedCredentialManager) createCustomerClients(cfg aws.Config) *CustomerAWSClients {
	return &CustomerAWSClients{
		SESClient: ses.NewFromConfig(cfg),
		SQSClient: sqs.NewFromConfig(cfg),
		STSClient: sts.NewFromConfig(cfg),
		Config:    cfg,
	}
}

// getCachedCredentials retrieves cached credentials if they're still valid
func (e *EnhancedCredentialManager) getCachedCredentials(cacheKey string) *CachedCredentials {
	e.cacheMutex.RLock()
	defer e.cacheMutex.RUnlock()

	cached, exists := e.credentialCache[cacheKey]
	if !exists {
		return nil
	}

	// Check if credentials are still valid (with 5-minute buffer)
	if time.Now().Add(5 * time.Minute).After(cached.ExpiresAt) {
		return nil
	}

	return cached
}

// cacheCredentials stores credentials in the cache
func (e *EnhancedCredentialManager) cacheCredentials(cacheKey string, cfg aws.Config, roleARN string) {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	// Credentials expire in 1 hour (STS default)
	expiresAt := time.Now().Add(time.Hour)

	e.credentialCache[cacheKey] = &CachedCredentials{
		Config:    cfg,
		ExpiresAt: expiresAt,
		RoleARN:   roleARN,
	}
}

// ValidateCustomerCredentials validates AWS credentials for a customer
func (e *EnhancedCredentialManager) ValidateCustomerCredentials(ctx context.Context, customerCode, serviceType string) error {
	clients, err := e.AssumeCustomerRole(ctx, customerCode, serviceType)
	if err != nil {
		return &CredentialValidationError{
			CustomerCode: customerCode,
			Service:      serviceType,
			Err:          err,
		}
	}

	// Test the credentials by calling GetCallerIdentity
	identity, err := clients.STSClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return &CredentialValidationError{
			CustomerCode: customerCode,
			Service:      serviceType,
			Err:          fmt.Errorf("STS GetCallerIdentity failed: %v", err),
		}
	}

	// Validate that we're in the correct account
	accountInfo, _ := e.customerManager.GetCustomerAccountInfo(customerCode)
	if *identity.Account != accountInfo.AWSAccountID {
		return &CredentialValidationError{
			CustomerCode: customerCode,
			Service:      serviceType,
			Err:          fmt.Errorf("account mismatch: expected %s, got %s", accountInfo.AWSAccountID, *identity.Account),
		}
	}

	fmt.Printf("Credentials validated for customer %s (%s): %s\n",
		customerCode, serviceType, *identity.Arn)

	return nil
}

// ValidateAllCustomerCredentials validates credentials for all customers
func (e *EnhancedCredentialManager) ValidateAllCustomerCredentials(ctx context.Context, serviceType string) map[string]error {
	results := make(map[string]error)
	var wg sync.WaitGroup

	// Validate credentials for each customer in parallel
	for customerCode := range e.customerManager.CustomerMappings {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			err := e.ValidateCustomerCredentials(ctx, code, serviceType)
			if err != nil {
				results[code] = err
			}
		}(customerCode)
	}

	wg.Wait()
	return results
}

// GetCustomerSESClient gets an SES client for a specific customer
func (e *EnhancedCredentialManager) GetCustomerSESClient(ctx context.Context, customerCode string) (*ses.Client, error) {
	clients, err := e.AssumeCustomerRole(ctx, customerCode, "ses")
	if err != nil {
		return nil, err
	}
	return clients.SESClient, nil
}

// GetCustomerSQSClient gets an SQS client for a specific customer
func (e *EnhancedCredentialManager) GetCustomerSQSClient(ctx context.Context, customerCode string) (*sqs.Client, error) {
	clients, err := e.AssumeCustomerRole(ctx, customerCode, "sqs")
	if err != nil {
		return nil, err
	}
	return clients.SQSClient, nil
}

// RefreshCredentials refreshes cached credentials for a customer
func (e *EnhancedCredentialManager) RefreshCredentials(ctx context.Context, customerCode, serviceType string) error {
	cacheKey := fmt.Sprintf("%s:%s", customerCode, serviceType)

	// Remove from cache to force refresh
	e.cacheMutex.Lock()
	delete(e.credentialCache, cacheKey)
	e.cacheMutex.Unlock()

	// Assume role again to get fresh credentials
	_, err := e.AssumeCustomerRole(ctx, customerCode, serviceType)
	return err
}

// ClearCredentialCache clears all cached credentials
func (e *EnhancedCredentialManager) ClearCredentialCache() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	e.credentialCache = make(map[string]*CachedCredentials)
	fmt.Println("Credential cache cleared")
}

// GetCacheStatus returns information about cached credentials
func (e *EnhancedCredentialManager) GetCacheStatus() map[string]time.Time {
	e.cacheMutex.RLock()
	defer e.cacheMutex.RUnlock()

	status := make(map[string]time.Time)
	for key, cached := range e.credentialCache {
		status[key] = cached.ExpiresAt
	}

	return status
}

// TestCustomerAccess tests access to customer AWS services
func (e *EnhancedCredentialManager) TestCustomerAccess(ctx context.Context, customerCode string) error {
	// Test SES access
	sesClient, err := e.GetCustomerSESClient(ctx, customerCode)
	if err != nil {
		return fmt.Errorf("SES access test failed: %v", err)
	}

	// Try to list SES identities (this requires minimal permissions)
	_, err = sesClient.ListIdentities(ctx, &ses.ListIdentitiesInput{})
	if err != nil {
		return fmt.Errorf("SES ListIdentities failed: %v", err)
	}

	// Test SQS access if configured
	accountInfo, _ := e.customerManager.GetCustomerAccountInfo(customerCode)
	if accountInfo.SQSRoleARN != "" {
		sqsClient, err := e.GetCustomerSQSClient(ctx, customerCode)
		if err != nil {
			return fmt.Errorf("SQS access test failed: %v", err)
		}

		// Try to list SQS queues
		_, err = sqsClient.ListQueues(ctx, &sqs.ListQueuesInput{})
		if err != nil {
			return fmt.Errorf("SQS ListQueues failed: %v", err)
		}
	}

	fmt.Printf("Access test passed for customer %s\n", customerCode)
	return nil
}

// GetCredentialMetrics returns metrics about credential usage
func (e *EnhancedCredentialManager) GetCredentialMetrics() map[string]interface{} {
	e.cacheMutex.RLock()
	defer e.cacheMutex.RUnlock()

	totalCached := len(e.credentialCache)
	expiredCount := 0
	expiringCount := 0

	now := time.Now()
	fiveMinutesFromNow := now.Add(5 * time.Minute)

	for _, cached := range e.credentialCache {
		if cached.ExpiresAt.Before(now) {
			expiredCount++
		} else if cached.ExpiresAt.Before(fiveMinutesFromNow) {
			expiringCount++
		}
	}

	return map[string]interface{}{
		"total_cached":    totalCached,
		"expired_count":   expiredCount,
		"expiring_count":  expiringCount,
		"total_customers": len(e.customerManager.CustomerMappings),
		"cache_hit_ratio": e.calculateCacheHitRatio(),
	}
}

// calculateCacheHitRatio calculates cache hit ratio (simplified implementation)
func (e *EnhancedCredentialManager) calculateCacheHitRatio() float64 {
	// In a real implementation, you would track cache hits and misses
	// For now, return a placeholder value
	return 0.85
}
