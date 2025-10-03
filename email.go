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
		FromEmailAddress: aws.String(em.contactConfig.OperationsEmail),
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

// getNotificationRecipients returns the list of email recipients for notifications
func (em *EmailManager) getNotificationRecipients(customerCode string) []string {
	// For now, send to the operations team
	// In a real implementation, this could be customer-specific
	recipients := []string{
		em.contactConfig.OperationsEmail,
		em.contactConfig.SecurityEmail,
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
