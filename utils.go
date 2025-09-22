package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/account/types"
)

// UpdateAlternateContacts updates alternate contacts for a customer account
func UpdateAlternateContacts(customerCode string, credentialManager *CredentialManager, contactConfig AlternateContactConfig) error {
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config: %v", err)
	}

	accountClient := account.NewFromConfig(customerConfig)

	// Update Security Contact
	if err := updateContact(accountClient, types.AlternateContactTypeSecurity, contactConfig.SecurityEmail, contactConfig.SecurityName, contactConfig.SecurityTitle, contactConfig.SecurityPhone); err != nil {
		return fmt.Errorf("failed to update security contact: %v", err)
	}

	// Update Billing Contact
	if err := updateContact(accountClient, types.AlternateContactTypeBilling, contactConfig.BillingEmail, contactConfig.BillingName, contactConfig.BillingTitle, contactConfig.BillingPhone); err != nil {
		return fmt.Errorf("failed to update billing contact: %v", err)
	}

	// Update Operations Contact
	if err := updateContact(accountClient, types.AlternateContactTypeOperations, contactConfig.OperationsEmail, contactConfig.OperationsName, contactConfig.OperationsTitle, contactConfig.OperationsPhone); err != nil {
		return fmt.Errorf("failed to update operations contact: %v", err)
	}

	log.Printf("Successfully updated alternate contacts for customer %s", customerCode)
	return nil
}

// updateContact updates a specific alternate contact
func updateContact(client *account.Client, contactType types.AlternateContactType, email, name, title, phone string) error {
	input := &account.PutAlternateContactInput{
		AlternateContactType: contactType,
		EmailAddress:         &email,
		Name:                 &name,
		Title:                &title,
		PhoneNumber:          &phone,
	}

	_, err := client.PutAlternateContact(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to put alternate contact %s: %v", contactType, err)
	}

	log.Printf("Updated %s contact: %s (%s)", contactType, name, email)
	return nil
}

// GetAlternateContacts retrieves current alternate contacts for a customer
func GetAlternateContacts(customerCode string, credentialManager *CredentialManager) (map[string]interface{}, error) {
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer config: %v", err)
	}

	accountClient := account.NewFromConfig(customerConfig)
	contacts := make(map[string]interface{})

	// Get Security Contact
	if contact, err := getContact(accountClient, types.AlternateContactTypeSecurity); err == nil {
		contacts["security"] = contact
	}

	// Get Billing Contact
	if contact, err := getContact(accountClient, types.AlternateContactTypeBilling); err == nil {
		contacts["billing"] = contact
	}

	// Get Operations Contact
	if contact, err := getContact(accountClient, types.AlternateContactTypeOperations); err == nil {
		contacts["operations"] = contact
	}

	return contacts, nil
}

// getContact retrieves a specific alternate contact
func getContact(client *account.Client, contactType types.AlternateContactType) (map[string]string, error) {
	input := &account.GetAlternateContactInput{
		AlternateContactType: contactType,
	}

	result, err := client.GetAlternateContact(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	contact := map[string]string{
		"type":  string(contactType),
		"email": *result.AlternateContact.EmailAddress,
		"name":  *result.AlternateContact.Name,
		"title": *result.AlternateContact.Title,
		"phone": *result.AlternateContact.PhoneNumber,
	}

	return contact, nil
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

// ValidateEmail validates an email address format
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Basic email validation
	matched, err := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
	if err != nil {
		return fmt.Errorf("failed to validate email: %v", err)
	}

	if !matched {
		return fmt.Errorf("invalid email format")
	}

	return nil
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

// Contains checks if a slice contains a string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveDuplicates removes duplicate strings from a slice
func RemoveDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
