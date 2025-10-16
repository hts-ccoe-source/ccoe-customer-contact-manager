// Package config provides configuration loading and management functionality.
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"ccoe-customer-contact-manager/internal/types"
)

// GetConfigPath returns the CONFIG_PATH environment variable or defaults to current directory
func GetConfigPath() string {
	configPath, exists := os.LookupEnv("CONFIG_PATH")
	if !exists || configPath == "" {
		return "./"
	}
	// Ensure the path ends with a slash for proper file concatenation
	if !strings.HasSuffix(configPath, "/") {
		configPath += "/"
	}
	return configPath
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(configPath string) (*types.Config, error) {
	if configPath == "" {
		return getDefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", configPath, err)
	}

	var config types.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", configPath, err)
	}

	// Set defaults if not specified
	if config.AWSRegion == "" {
		config.AWSRegion = "us-east-1"
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	return &config, nil
}

// LoadSESConfig loads SES configuration from a JSON file
func LoadSESConfig(configPath string) (*types.SESConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SES config file %s: %v", configPath, err)
	}

	var sesConfig types.SESConfig
	if err := json.Unmarshal(data, &sesConfig); err != nil {
		return nil, fmt.Errorf("failed to parse SES config file %s: %v", configPath, err)
	}

	return &sesConfig, nil
}

// SaveConfig saves configuration to a JSON file
func SaveConfig(config *types.Config, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %v", configPath, err)
	}

	return nil
}

// ValidateConfig validates the configuration
func ValidateConfig(config *types.Config) error {
	if config.AWSRegion == "" {
		return fmt.Errorf("aws_region is required")
	}

	if len(config.CustomerMappings) == 0 {
		return fmt.Errorf("at least one customer mapping is required")
	}

	for code, customer := range config.CustomerMappings {
		if customer.CustomerCode != code {
			return fmt.Errorf("customer code mismatch: key=%s, code=%s", code, customer.CustomerCode)
		}
		if customer.SESRoleARN == "" {
			return fmt.Errorf("ses_role_arn is required for customer %s", code)
		}
		if customer.GetAccountID() == "" {
			return fmt.Errorf("unable to extract account ID from ses_role_arn for customer %s", code)
		}
	}

	return nil
}

// LoadSubscriptionConfig loads the subscription configuration from a JSON file
func LoadSubscriptionConfig(configPath string) (types.SubscriptionConfig, error) {
	configJson, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription config file %s: %w", configPath, err)
	}

	var config types.SubscriptionConfig
	err = json.Unmarshal(configJson, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subscription config: %w", err)
	}

	return config, nil
}

// getDefaultConfig returns a default configuration
func getDefaultConfig() *types.Config {
	return &types.Config{
		AWSRegion: "us-east-1",
		LogLevel:  "info",
		CustomerMappings: map[string]types.CustomerAccountInfo{
			"hts": {
				CustomerCode: "hts",
				CustomerName: "Hearst Technology Services",
				Region:       "us-east-1",
				SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
				Environment:  "production",
				SQSQueueARN:  "arn:aws:sqs:us-east-1:123456789012:hts-queue",
			},
		},
		ContactConfig: types.AlternateContactConfig{
			SecurityEmail:   "security@hearst.com",
			SecurityName:    "Security Team",
			SecurityTitle:   "Security Operations",
			SecurityPhone:   "+1-555-0123",
			BillingEmail:    "billing@hearst.com",
			BillingName:     "Billing Team",
			BillingTitle:    "Financial Operations",
			BillingPhone:    "+1-555-0124",
			OperationsEmail: "operations@hearst.com",
			OperationsName:  "Operations Team",
			OperationsTitle: "Technical Operations",
			OperationsPhone: "+1-555-0125",
		},
		S3Config: types.S3Config{
			BucketName: "example-bucket",
		},
	}
}

// SetupLogging configures logging based on log level
func SetupLogging(logLevel string) {
	switch strings.ToLower(logLevel) {
	case "debug":
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	case "info":
		log.SetFlags(log.LstdFlags)
	case "warn", "error":
		log.SetFlags(log.LstdFlags)
	default:
		log.SetFlags(log.LstdFlags)
	}

	log.SetOutput(os.Stdout)
}
