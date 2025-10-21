// Package aws provides core AWS service interactions and credential management utilities.
package aws

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationsTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"

	"ccoe-customer-contact-manager/internal/types"
)

// CreateConnectionConfiguration creates an AWS configuration using the provided credentials.
func CreateConnectionConfiguration(creds aws.Credentials) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: creds,
		}),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	return cfg, nil
}

// GetManagementAccountIdByPrefix gets management account ID by organization prefix
func GetManagementAccountIdByPrefix(prefix string, orgConfig []types.Organization) (string, error) {
	for _, org := range orgConfig {
		if org.Prefix == prefix {
			return org.ManagementAccountId, nil
		}
	}
	return "", fmt.Errorf("management account ID not found for prefix: %s", prefix)
}

// GetCurrentAccountId gets the current AWS account ID
func GetCurrentAccountId(StsServiceConnection *sts.Client) string {
	stsinput := &sts.GetCallerIdentityInput{}
	stsresult, stserr := StsServiceConnection.GetCallerIdentity(context.Background(), stsinput)
	if stserr != nil {
		fmt.Println("Error getting current Account ID")
		os.Exit(1)
	} else {
		fmt.Println("Account ID: " + *stsresult.Account)
		fmt.Println()
	}
	return *stsresult.Account
}

// IsManagementAccount checks if the current account is the management account
func IsManagementAccount(OrganizationsServiceConnection *organizations.Client, AccountId string) bool {
	input := &organizations.DescribeOrganizationInput{}
	result, err := OrganizationsServiceConnection.DescribeOrganization(context.Background(), input)
	if err != nil {
		fmt.Println("Error", err)
	}

	if *result.Organization.MasterAccountId == AccountId {
		return true
	} else {
		return false
	}
}

// AssumeRole assumes an AWS IAM role and returns the assumed role's credentials.
func AssumeRole(stsClient *sts.Client, roleArn string, sessionName string) (*ststypes.Credentials, error) {
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
	}

	result, err := stsClient.AssumeRole(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}

	return result.Credentials, nil
}

// ValidateRoleAssumption validates that a role can be assumed by attempting to assume it
// This is useful for validating Identity Center role ARNs before using them
func ValidateRoleAssumption(roleArn string, sessionName string) error {
	// Empty role ARN is valid (optional field)
	if roleArn == "" {
		return nil
	}

	// Load default AWS config
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Attempt to assume the role
	_, err = AssumeRole(stsClient, roleArn, sessionName)
	if err != nil {
		// Provide clear error message for permission issues
		return fmt.Errorf("failed to assume role %s: %w\n"+
			"Please ensure:\n"+
			"  1. The role exists in the target account\n"+
			"  2. The role's trust policy allows your current credentials to assume it\n"+
			"  3. Your current credentials have sts:AssumeRole permission\n"+
			"  4. The role ARN is correct", roleArn, err)
	}

	return nil
}

// GetAllAccountsInOrganization lists all accounts in the organization
func GetAllAccountsInOrganization(OrganizationsServiceConnection *organizations.Client) ([]organizationsTypes.Account, error) {
	input := &organizations.ListAccountsInput{}
	result, err := OrganizationsServiceConnection.ListAccounts(context.Background(), input)
	if err != nil {
		fmt.Println("Error", err)
	}

	OrgAccounts := result.Accounts

	return OrgAccounts, nil
}

// ValidateCustomerCode validates a customer code format
func ValidateCustomerCode(code string) error {
	if code == "" {
		return fmt.Errorf("customer code cannot be empty")
	}

	// Customer codes should be lowercase alphanumeric with hyphens
	matched, err := regexp.MatchString("^[a-z0-9-]+$", code)
	if err != nil {
		return fmt.Errorf("failed to validate customer code: %v", err)
	}

	if !matched {
		return fmt.Errorf("customer code must contain only lowercase letters, numbers, and hyphens")
	}

	if len(code) > 50 {
		return fmt.Errorf("customer code must be 50 characters or less")
	}

	return nil
}

// CredentialManager handles AWS credential management for multiple customers
type CredentialManager struct {
	region           string
	customerMappings map[string]types.CustomerAccountInfo
	baseConfig       aws.Config
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(region string, customerMappings map[string]types.CustomerAccountInfo) (*CredentialManager, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &CredentialManager{
		region:           region,
		customerMappings: customerMappings,
		baseConfig:       cfg,
	}, nil
}

// GetCustomerConfig returns AWS config for a specific customer
func (cm *CredentialManager) GetCustomerConfig(customerCode string) (aws.Config, error) {
	customer, exists := cm.customerMappings[customerCode]
	if !exists {
		return aws.Config{}, fmt.Errorf("customer %s not found", customerCode)
	}

	// Create STS client with base config
	stsClient := sts.NewFromConfig(cm.baseConfig)

	// Create credentials provider for the customer role
	sessionName := fmt.Sprintf("%s-ses-session", customerCode)

	assumedCreds, err := AssumeRole(stsClient, customer.SESRoleARN, sessionName)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to assume role: %w", err)
	}

	fmt.Printf("✅ Successfully assumed role %s for customer %s\n", customer.SESRoleARN, customerCode)

	// Create customer-specific config
	awsCreds := aws.Credentials{
		AccessKeyID:     *assumedCreds.AccessKeyId,
		SecretAccessKey: *assumedCreds.SecretAccessKey,
		SessionToken:    *assumedCreds.SessionToken,
		Source:          "AssumeRole",
	}

	customerConfig, err := CreateConnectionConfiguration(awsCreds)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create customer config: %w", err)
	}

	customerConfig.Region = customer.Region
	return customerConfig, nil
}

// ValidateCustomerAccess validates that we can assume the customer role
func (cm *CredentialManager) ValidateCustomerAccess(customerCode string) error {
	customerConfig, err := cm.GetCustomerConfig(customerCode)
	if err != nil {
		return err
	}

	// Test the credentials by calling GetCallerIdentity
	stsClient := sts.NewFromConfig(customerConfig)
	_, err = stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to validate credentials for customer %s: %w", customerCode, err)
	}

	fmt.Printf("Successfully validated access for customer %s\n", customerCode)
	return nil
}

// ListCustomers returns a list of configured customer codes
func (cm *CredentialManager) ListCustomers() []string {
	customers := make([]string, 0, len(cm.customerMappings))
	for code := range cm.customerMappings {
		customers = append(customers, code)
	}
	return customers
}

// GetCustomerInfo returns customer information
func (cm *CredentialManager) GetCustomerInfo(customerCode string) (types.CustomerAccountInfo, error) {
	customer, exists := cm.customerMappings[customerCode]
	if !exists {
		return types.CustomerAccountInfo{}, fmt.Errorf("customer %s not found", customerCode)
	}
	return customer, nil
}
