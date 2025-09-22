package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// CredentialManager handles AWS credential management for multiple customers
type CredentialManager struct {
	region           string
	customerMappings map[string]CustomerAccountInfo
	baseConfig       aws.Config
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(region string, customerMappings map[string]CustomerAccountInfo) (*CredentialManager, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
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

	log.Printf("Assuming role %s for customer %s", customer.SESRoleARN, customerCode)

	// Create STS client with base config
	stsClient := sts.NewFromConfig(cm.baseConfig)

	// Create credentials provider for the customer role
	creds := stscreds.NewAssumeRoleProvider(stsClient, customer.SESRoleARN)

	// Create customer-specific config
	customerConfig := cm.baseConfig.Copy()
	customerConfig.Credentials = creds
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
	_, err = stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to validate credentials for customer %s: %v", customerCode, err)
	}

	log.Printf("Successfully validated access for customer %s", customerCode)
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
func (cm *CredentialManager) GetCustomerInfo(customerCode string) (CustomerAccountInfo, error) {
	customer, exists := cm.customerMappings[customerCode]
	if !exists {
		return CustomerAccountInfo{}, fmt.Errorf("customer %s not found", customerCode)
	}
	return customer, nil
}
