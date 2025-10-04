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
		log.Printf("No recipients found for customer %s, skipping email notification", customerCode)
		return nil
	}

	// Send email
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String("ccoe@nonprod.ccoe.hearst.com"),
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
	// Get customer-specific SES client
	customerConfig, err := em.credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer config: %v", err)
	}

	return em.getTopicSubscribersWithConfig(customerCode, topicName, customerConfig)
}

// getTopicSubscribersWithConfig returns email addresses subscribed to a specific SES topic using provided config
func (em *EmailManager) getTopicSubscribersWithConfig(customerCode, topicName string, customerConfig aws.Config) ([]string, error) {
	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the account's main contact list dynamically
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get account contact list: %v", err)
	}

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
	// Use aws-announce as default topic for backward compatibility
	return em.getNotificationRecipientsForTopic(customerCode, "aws-announce")
}

// getNotificationRecipientsForTopic returns the list of email recipients for a specific topic
func (em *EmailManager) getNotificationRecipientsForTopic(customerCode string, topicName string) []string {
	// Try to get subscribers from SES topic first
	subscribers, err := em.getTopicSubscribers(customerCode, topicName)
	if err != nil {
		log.Printf("Failed to get topic subscribers for '%s': %v, no fallback emails will be used", topicName, err)
		return []string{}
	}

	if len(subscribers) == 0 {
		log.Printf("No subscribers found for topic '%s', no emails will be sent", topicName)
		return []string{}
	}

	log.Printf("Found %d subscribers for topic '%s'", len(subscribers), topicName)
	return subscribers
}

// getNotificationRecipientsForTopicWithConfig returns the list of email recipients for a specific topic using provided config
func (em *EmailManager) getNotificationRecipientsForTopicWithConfig(customerCode string, topicName string, customerConfig aws.Config) []string {
	// Try to get subscribers from SES topic first
	subscribers, err := em.getTopicSubscribersWithConfig(customerCode, topicName, customerConfig)
	if err != nil {
		log.Printf("Failed to get topic subscribers for '%s': %v, no fallback emails will be used", topicName, err)
		return []string{}
	}

	if len(subscribers) == 0 {
		log.Printf("No subscribers found for topic '%s', no emails will be sent", topicName)
		return []string{}
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

// getTopicForChangeStatus determines the appropriate SES topic based on change status
func (em *EmailManager) getTopicForChangeStatus(status string, changeType string) string {
	switch status {
	case "waiting for approval", "submitted":
		// For changes waiting for approval, send to approval topic for review
		return "aws-approval"
	case "approved", "completed", "implemented":
		// For approved/completed changes, send announcements
		return "aws-announce"
	case "cancelled", "rejected", "denied":
		// For cancelled/rejected changes, do not send emails
		return ""
	default:
		// Default to approval topic for unknown statuses (safer for review)
		return "aws-approval"
	}
}

// SendChangeNotification sends change notification to appropriate topic based on status
func (em *EmailManager) SendChangeNotification(customerCode string, changeMetadata map[string]interface{}) error {
	customerConfig, err := em.credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config: %v", err)
	}

	customerInfo, err := em.credentialManager.GetCustomerInfo(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer info: %v", err)
	}

	// Extract status and change type from metadata
	status, _ := changeMetadata["status"].(string)
	changeType, _ := changeMetadata["changeType"].(string)
	if status == "" {
		status = "waiting for approval" // Default status when submitted from UI
	}

	// Determine appropriate topic
	topicName := em.getTopicForChangeStatus(status, changeType)

	// If no topic is returned, skip sending email (for cancelled/rejected statuses)
	if topicName == "" {
		log.Printf("Skipping email notification for status '%s' - no email required", status)
		return nil
	}

	// Get recipients for the specific topic
	recipients := em.getNotificationRecipientsForTopicWithConfig(customerCode, topicName, customerConfig)
	if len(recipients) == 0 {
		log.Printf("No recipients found for customer %s topic %s, skipping email notification", customerCode, topicName)
		return nil
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Prepare email content based on status
	subject := em.generateSubjectForStatus(status, changeMetadata, customerInfo.CustomerName)
	body, err := em.generateChangeEmailBody(status, changeMetadata, &customerInfo)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %v", err)
	}

	// Send email
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String("ccoe@nonprod.ccoe.hearst.com"),
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
		return fmt.Errorf("failed to send email to topic %s: %v", topicName, err)
	}

	log.Printf("Successfully sent change notification to topic %s for customer %s (MessageId: %s)",
		topicName, customerCode, aws.ToString(result.MessageId))
	return nil
}

// generateSubjectForStatus generates email subject based on change status
func (em *EmailManager) generateSubjectForStatus(status string, changeMetadata map[string]interface{}, customerName string) string {
	title, _ := changeMetadata["title"].(string)
	if title == "" {
		title = "Change Request"
	}

	switch status {
	case "waiting for approval", "submitted":
		return fmt.Sprintf("APPROVAL REQUIRED: %s - %s", title, customerName)
	case "approved":
		return fmt.Sprintf("CHANGE APPROVED: %s - %s", title, customerName)
	case "completed", "implemented":
		return fmt.Sprintf("CHANGE COMPLETED: %s - %s", title, customerName)
	case "cancelled":
		return fmt.Sprintf("CHANGE CANCELLED: %s - %s", title, customerName)
	case "rejected", "denied":
		return fmt.Sprintf("CHANGE REJECTED: %s - %s", title, customerName)
	default:
		return fmt.Sprintf("APPROVAL REQUIRED: %s - %s", title, customerName)
	}
}

// generateChangeEmailBody generates email body for change notifications
func (em *EmailManager) generateChangeEmailBody(status string, changeMetadata map[string]interface{}, customerInfo *CustomerAccountInfo) (string, error) {
	var body strings.Builder

	title, _ := changeMetadata["title"].(string)
	createdBy, _ := changeMetadata["createdBy"].(string)
	createdAt, _ := changeMetadata["createdAt"].(string)

	// Header based on status
	switch status {
	case "waiting for approval", "submitted":
		body.WriteString("A new change request requires your approval.\n\n")
	case "approved":
		body.WriteString("A change request has been approved and will be implemented.\n\n")
	case "completed", "implemented":
		body.WriteString("A change request has been completed successfully.\n\n")
	case "cancelled":
		body.WriteString("A change request has been cancelled.\n\n")
	case "rejected", "denied":
		body.WriteString("A change request has been rejected.\n\n")
	default:
		body.WriteString("A change request requires your approval.\n\n")
	}

	// Change details
	body.WriteString(fmt.Sprintf("Customer: %s\n", customerInfo.CustomerName))
	body.WriteString(fmt.Sprintf("Title: %s\n", title))
	body.WriteString(fmt.Sprintf("Status: %s\n", strings.ToUpper(status)))
	body.WriteString(fmt.Sprintf("Created By: %s\n", createdBy))
	body.WriteString(fmt.Sprintf("Created At: %s\n", createdAt))
	body.WriteString("\n")

	// Add change metadata if available
	if changeData, ok := changeMetadata["changeMetadata"].(map[string]interface{}); ok {
		if reason, ok := changeData["changeReason"].(string); ok && reason != "" {
			body.WriteString(fmt.Sprintf("Reason: %s\n", reason))
		}
		if plan, ok := changeData["implementationPlan"].(string); ok && plan != "" {
			body.WriteString(fmt.Sprintf("Implementation Plan: %s\n", plan))
		}
		if impact, ok := changeData["expectedCustomerImpact"].(string); ok && impact != "" {
			body.WriteString(fmt.Sprintf("Expected Impact: %s\n", impact))
		}
		if schedule, ok := changeData["schedule"].(map[string]interface{}); ok {
			if start, ok := schedule["implementationStart"].(string); ok && start != "" {
				body.WriteString(fmt.Sprintf("Scheduled Start: %s\n", start))
			}
			if end, ok := schedule["implementationEnd"].(string); ok && end != "" {
				body.WriteString(fmt.Sprintf("Scheduled End: %s\n", end))
			}
		}
	}

	body.WriteString("\n")
	body.WriteString("This is an automated notification from the CCOE Change Management System.\n")

	return body.String(), nil
}
