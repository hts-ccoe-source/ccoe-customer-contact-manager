// Package config provides configuration validation functionality.
package config

import (
	"fmt"
	"strings"

	"ccoe-customer-contact-manager/internal/types"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError
}

// Error implements the error interface
func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	var messages []string
	for _, err := range e.Errors {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("multiple validation errors:\n  - %s", strings.Join(messages, "\n  - "))
}

// Add adds a validation error
func (e *ValidationErrors) Add(field, message string) {
	e.Errors = append(e.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// ValidateSESConfig validates the configuration for SES domain validation operations
func ValidateSESConfig(config *types.Config, requireRoute53 bool) error {
	errors := &ValidationErrors{}

	// Validate required fields
	if config.AWSRegion == "" {
		errors.Add("aws_region", "is required")
	}

	if len(config.CustomerMappings) == 0 {
		errors.Add("customer_mappings", "at least one customer mapping is required")
	}

	// Validate customer mappings
	for code, customer := range config.CustomerMappings {
		prefix := fmt.Sprintf("customer_mappings[%s]", code)

		if customer.CustomerCode == "" {
			errors.Add(prefix+".customer_code", "is required")
		} else if customer.CustomerCode != code {
			errors.Add(prefix+".customer_code", fmt.Sprintf("must match map key (expected: %s, got: %s)", code, customer.CustomerCode))
		}

		if customer.SESRoleARN == "" {
			errors.Add(prefix+".ses_role_arn", "is required")
		} else if !isValidARN(customer.SESRoleARN) {
			errors.Add(prefix+".ses_role_arn", fmt.Sprintf("invalid ARN format: %s", customer.SESRoleARN))
		}

		// Validate Identity Center role ARN if provided (optional)
		if customer.IdentityCenterRoleArn != "" {
			if err := ValidateIdentityCenterRoleArn(customer.IdentityCenterRoleArn); err != nil {
				errors.Add(prefix+".identity_center_role_arn", err.Error())
			}
		}
	}

	// Validate Route53 config if required
	if requireRoute53 {
		if config.Route53Config == nil {
			errors.Add("route53_config", "is required when configure-dns is enabled")
		} else {
			if config.Route53Config.ZoneID == "" {
				errors.Add("route53_config.zone_id", "is required")
			}
			if config.Route53Config.RoleARN == "" {
				errors.Add("route53_config.role_arn", "is required")
			} else if !isValidARN(config.Route53Config.RoleARN) {
				errors.Add("route53_config.role_arn", fmt.Sprintf("invalid ARN format: %s", config.Route53Config.RoleARN))
			}
		}
	}

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// ValidateRoute53Config validates the configuration for Route53 operations
func ValidateRoute53Config(config *types.Config) error {
	errors := &ValidationErrors{}

	// Validate required fields
	if config.AWSRegion == "" {
		errors.Add("aws_region", "is required")
	}

	if len(config.CustomerMappings) == 0 {
		errors.Add("customer_mappings", "at least one customer mapping is required")
	}

	// Validate Route53 config
	if config.Route53Config == nil {
		errors.Add("route53_config", "is required for Route53 operations")
	} else {
		if config.Route53Config.ZoneID == "" {
			errors.Add("route53_config.zone_id", "is required")
		}
		if config.Route53Config.RoleARN == "" {
			errors.Add("route53_config.role_arn", "is required")
		} else if !isValidARN(config.Route53Config.RoleARN) {
			errors.Add("route53_config.role_arn", fmt.Sprintf("invalid ARN format: %s", config.Route53Config.RoleARN))
		}
	}

	// Validate customer mappings have required fields
	for code, customer := range config.CustomerMappings {
		prefix := fmt.Sprintf("customer_mappings[%s]", code)

		if customer.CustomerCode == "" {
			errors.Add(prefix+".customer_code", "is required")
		}

		if customer.SESRoleARN == "" {
			errors.Add(prefix+".ses_role_arn", "is required")
		} else if !isValidARN(customer.SESRoleARN) {
			errors.Add(prefix+".ses_role_arn", fmt.Sprintf("invalid ARN format: %s", customer.SESRoleARN))
		}

		// Validate Identity Center role ARN if provided (optional)
		if customer.IdentityCenterRoleArn != "" {
			if err := ValidateIdentityCenterRoleArn(customer.IdentityCenterRoleArn); err != nil {
				errors.Add(prefix+".identity_center_role_arn", err.Error())
			}
		}
	}

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// ValidateCustomerCode validates that a customer code exists in the configuration
func ValidateCustomerCode(config *types.Config, customerCode string) error {
	if customerCode == "" {
		return fmt.Errorf("customer code cannot be empty")
	}

	customer, exists := config.CustomerMappings[customerCode]
	if !exists {
		availableCodes := make([]string, 0, len(config.CustomerMappings))
		for code := range config.CustomerMappings {
			availableCodes = append(availableCodes, code)
		}
		return fmt.Errorf("customer code '%s' not found in configuration (available: %s)",
			customerCode, strings.Join(availableCodes, ", "))
	}

	// Validate SES role ARN format
	if customer.SESRoleARN == "" {
		return fmt.Errorf("customer '%s' has no SES role ARN configured", customerCode)
	}

	if !isValidARN(customer.SESRoleARN) {
		return fmt.Errorf("customer '%s' has invalid SES role ARN format: %s", customerCode, customer.SESRoleARN)
	}

	// Validate Identity Center role ARN format if provided (optional field)
	if customer.IdentityCenterRoleArn != "" && !isValidARN(customer.IdentityCenterRoleArn) {
		return fmt.Errorf("customer '%s' has invalid Identity Center role ARN format: %s", customerCode, customer.IdentityCenterRoleArn)
	}

	return nil
}

// ValidateIdentityCenterRoleArn validates the Identity Center role ARN format
// This is an optional field, so empty values are valid
func ValidateIdentityCenterRoleArn(roleArn string) error {
	// Empty is valid (optional field)
	if roleArn == "" {
		return nil
	}

	// If provided, must be a valid ARN
	if !isValidARN(roleArn) {
		return fmt.Errorf("invalid Identity Center role ARN format: %s", roleArn)
	}

	// Additional validation: should be an IAM role ARN
	if !strings.Contains(roleArn, ":iam::") || !strings.Contains(roleArn, ":role/") {
		return fmt.Errorf("Identity Center role ARN must be an IAM role ARN (format: arn:aws:iam::account-id:role/role-name): %s", roleArn)
	}

	return nil
}

// isValidARN validates AWS ARN format
// ARN format: arn:partition:service:region:account-id:resource-type/resource-id
// Example: arn:aws:iam::123456789012:role/MyRole
func isValidARN(arn string) bool {
	if arn == "" {
		return false
	}

	// Must start with "arn:"
	if !strings.HasPrefix(arn, "arn:") {
		return false
	}

	// Split by colons
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return false
	}

	// Validate partition (usually "aws")
	if parts[1] == "" {
		return false
	}

	// Validate service (e.g., "iam", "sts")
	if parts[2] == "" {
		return false
	}

	// Region can be empty for global services like IAM
	// Account ID should be present (parts[4])
	if parts[4] == "" {
		return false
	}

	// Resource should be present (parts[5])
	if parts[5] == "" {
		return false
	}

	return true
}
