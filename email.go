package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// EmailManager handles email operations
type EmailManager struct {
	credentialManager *CredentialManager
	contactConfig     AlternateContactConfig
}

// NewEmailManager creates a new email manager
func NewEmailManager(credentialManager *CredentialManager, contactConfig AlternateContactConfig) *EmailManager {
	return &EmailManager{
		credentialManager: credentialManager,
		contactConfig:     contactConfig,
	}
}

// SendAlternateContactNotification sends alternate contact update notification
func (em *EmailManager) SendAlternateContactNotification(customerCode string, changeDetails map[string]interface{}) error {
	customerConfig, err := em.credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config: %v", err)
	}

	customerInfo, err := em.credentialManager.GetCustomerInfo(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer info: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Prepare email content
	subject := fmt.Sprintf("AWS Alternate Contact Update - %s", customerInfo.CustomerName)
	body, err := em.generateEmailBody(customerInfo, changeDetails)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %v", err)
	}

	// Determine recipients
	recipients := em.getNotificationRecipients(customerCode)
	if len(recipients) == 0 {
		return fmt.Errorf("no recipients configured for customer %s", customerCode)
	}

	// Send email
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(em.contactConfig.SecurityEmail), // Use verified security email instead of operations
		Destination: &types.Destination{
			ToAddresses: recipients,
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(subject),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data: aws.String(body),
					},
				},
			},
		},
	}

	result, err := sesClient.SendEmail(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	log.Printf("Email sent successfully to %s for customer %s (MessageId: %s)",
		strings.Join(recipients, ", "), customerCode, *result.MessageId)

	return nil
}

// generateEmailBody generates the email body content
func (em *EmailManager) generateEmailBody(customerInfo CustomerAccountInfo, changeDetails map[string]interface{}) (string, error) {
	tmplText := `
AWS Alternate Contact Update Notification

Customer: {{.CustomerName}}
Account ID: {{.AccountID}}
Environment: {{.Environment}}

The following alternate contacts have been updated:

{{if .SecurityUpdated}}
Security Contact:
- Email: {{.SecurityEmail}}
- Name: {{.SecurityName}}
- Title: {{.SecurityTitle}}
- Phone: {{.SecurityPhone}}
{{end}}

{{if .BillingUpdated}}
Billing Contact:
- Email: {{.BillingEmail}}
- Name: {{.BillingName}}
- Title: {{.BillingTitle}}
- Phone: {{.BillingPhone}}
{{end}}

{{if .OperationsUpdated}}
Operations Contact:
- Email: {{.OperationsEmail}}
- Name: {{.OperationsName}}
- Title: {{.OperationsTitle}}
- Phone: {{.OperationsPhone}}
{{end}}

This update was performed automatically by the AWS Alternate Contact Manager.

If you have any questions, please contact the operations team at {{.ContactEmail}}.

Best regards,
AWS Operations Team
`

	tmpl, err := template.New("email").Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("failed to parse email template: %v", err)
	}

	// Prepare template data
	data := map[string]interface{}{
		"CustomerName":      customerInfo.CustomerName,
		"AccountID":         customerInfo.GetAccountID(),
		"Environment":       customerInfo.Environment,
		"ContactEmail":      em.contactConfig.OperationsEmail,
		"SecurityEmail":     em.contactConfig.SecurityEmail,
		"SecurityName":      em.contactConfig.SecurityName,
		"SecurityTitle":     em.contactConfig.SecurityTitle,
		"SecurityPhone":     em.contactConfig.SecurityPhone,
		"BillingEmail":      em.contactConfig.BillingEmail,
		"BillingName":       em.contactConfig.BillingName,
		"BillingTitle":      em.contactConfig.BillingTitle,
		"BillingPhone":      em.contactConfig.BillingPhone,
		"OperationsEmail":   em.contactConfig.OperationsEmail,
		"OperationsName":    em.contactConfig.OperationsName,
		"OperationsTitle":   em.contactConfig.OperationsTitle,
		"OperationsPhone":   em.contactConfig.OperationsPhone,
		"SecurityUpdated":   changeDetails["security_updated"],
		"BillingUpdated":    changeDetails["billing_updated"],
		"OperationsUpdated": changeDetails["operations_updated"],
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute email template: %v", err)
	}

	return buf.String(), nil
}

// getTopicSubscribers returns email addresses subscribed to a specific SES topic
func (em *EmailManager) getTopicSubscribers(customerCode, topicName string) ([]string, error) {
	// Get the account list name (assuming it follows the pattern)
	accountListName := "hts-prod-ccoe-change-management-contacts"

	// Get contacts subscribed to this topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	// Get customer config to access SES
	customerConfig, err := em.credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	contactsResult, err := sesClient.ListContacts(context.TODO(), contactsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	var subscribers []string
	for _, contact := range contactsResult.Contacts {
		if contact.EmailAddress != nil {
			subscribers = append(subscribers, *contact.EmailAddress)
		}
	}

	return subscribers, nil
}

// getNotificationRecipients returns the list of email recipients for notifications
func (em *EmailManager) getNotificationRecipients(customerCode string) []string {
	// Try to get subscribers from SES topic first
	topicName := "change-notifications" // or "announce" based on your topic configuration

	subscribers, err := em.getTopicSubscribers(customerCode, topicName)
	if err != nil {
		log.Printf("Failed to get topic subscribers for '%s': %v, falling back to config emails", topicName, err)
		// Fallback to config-based emails
		recipients := []string{
			em.contactConfig.SecurityEmail, // Use security email instead of operations for change mgmt
		}

		// Remove duplicates and empty emails
		seen := make(map[string]bool)
		var result []string
		for _, email := range recipients {
			if email != "" && !seen[email] {
				seen[email] = true
				result = append(result, email)
			}
		}
		return result
	}

	if len(subscribers) == 0 {
		log.Printf("No subscribers found for topic '%s', using fallback emails", topicName)
		return []string{em.contactConfig.SecurityEmail}
	}

	log.Printf("Found %d subscribers for topic '%s'", len(subscribers), topicName)
	return subscribers
}

// ValidateEmailConfiguration validates the email configuration
func (em *EmailManager) ValidateEmailConfiguration(customerCode string) error {
	// Validate customer exists
	_, err := em.credentialManager.GetCustomerInfo(customerCode)
	if err != nil {
		return err
	}

	// Validate we can get SES client
	customerConfig, err := em.credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Test SES access by getting account sending enabled status
	_, err = sesClient.GetAccount(context.TODO(), &sesv2.GetAccountInput{})
	if err != nil {
		return fmt.Errorf("failed to access SES for customer %s: %v", customerCode, err)
	}

	log.Printf("Email configuration validated for customer %s", customerCode)
	return nil
}
