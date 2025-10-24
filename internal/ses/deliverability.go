// Package ses provides SES deliverability configuration functionality.
package ses

import (
	"ccoe-customer-contact-manager/internal/types"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// ConfigureCustomMailFrom configures a custom MAIL FROM domain for better deliverability
func ConfigureCustomMailFrom(sesClient *sesv2.Client, domainName string, mailFromDomain string, dryRun bool, logger *slog.Logger) error {
	if dryRun {
		logger.Info("dry-run: would configure custom MAIL FROM domain",
			"domain", domainName,
			"mail_from_domain", mailFromDomain)
		return nil
	}

	input := &sesv2.PutEmailIdentityMailFromAttributesInput{
		EmailIdentity:       aws.String(domainName),
		MailFromDomain:      aws.String(mailFromDomain),
		BehaviorOnMxFailure: sesv2Types.BehaviorOnMxFailureUseDefaultValue,
	}

	_, err := sesClient.PutEmailIdentityMailFromAttributes(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to configure custom MAIL FROM domain: %w", err)
	}

	logger.Info("successfully configured custom MAIL FROM domain",
		"domain", domainName,
		"mail_from_domain", mailFromDomain)

	return nil
}

// GetMailFromDNSRecords returns the DNS records needed for custom MAIL FROM domain
// This is a helper function that returns records in a display-friendly format
func GetMailFromDNSRecords(mailFromDomain string, region string) []types.DNSRecord {
	// MX record for bounce handling
	mxValue := fmt.Sprintf("10 feedback-smtp.%s.amazonses.com", region)

	// SPF record for MAIL FROM domain
	spfValue := "v=spf1 include:amazonses.com ~all"

	return []types.DNSRecord{
		{
			Type:  "MX",
			Name:  mailFromDomain,
			Value: mxValue,
			TTL:   300,
		},
		{
			Type:  "TXT",
			Name:  mailFromDomain,
			Value: spfValue,
			TTL:   300,
		},
	}
}

// CreateConfigurationSet creates a configuration set for tracking email metrics
func CreateConfigurationSet(sesClient *sesv2.Client, configSetName string, dryRun bool, logger *slog.Logger) error {
	if dryRun {
		logger.Info("dry-run: would create configuration set",
			"configuration_set", configSetName)
		return nil
	}

	input := &sesv2.CreateConfigurationSetInput{
		ConfigurationSetName: aws.String(configSetName),
		DeliveryOptions: &sesv2Types.DeliveryOptions{
			TlsPolicy: sesv2Types.TlsPolicyRequire, // Require TLS for better security
		},
		ReputationOptions: &sesv2Types.ReputationOptions{
			ReputationMetricsEnabled: true,
		},
		SendingOptions: &sesv2Types.SendingOptions{
			SendingEnabled: true,
		},
		SuppressionOptions: &sesv2Types.SuppressionOptions{
			SuppressedReasons: []sesv2Types.SuppressionListReason{
				sesv2Types.SuppressionListReasonBounce,
				sesv2Types.SuppressionListReasonComplaint,
			},
		},
	}

	_, err := sesClient.CreateConfigurationSet(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to create configuration set: %w", err)
	}

	logger.Info("successfully created configuration set",
		"configuration_set", configSetName)

	return nil
}

// ConfigureEventDestination adds event tracking to a configuration set
func ConfigureEventDestination(sesClient *sesv2.Client, configSetName string, destinationName string, snsTopicArn string, dryRun bool, logger *slog.Logger) error {
	if dryRun {
		logger.Info("dry-run: would configure event destination",
			"configuration_set", configSetName,
			"destination_name", destinationName,
			"sns_topic_arn", snsTopicArn)
		return nil
	}

	input := &sesv2.CreateConfigurationSetEventDestinationInput{
		ConfigurationSetName: aws.String(configSetName),
		EventDestinationName: aws.String(destinationName),
		EventDestination: &sesv2Types.EventDestinationDefinition{
			Enabled: true,
			MatchingEventTypes: []sesv2Types.EventType{
				sesv2Types.EventTypeBounce,
				sesv2Types.EventTypeComplaint,
				sesv2Types.EventTypeDelivery,
				sesv2Types.EventTypeSend,
				sesv2Types.EventTypeReject,
				sesv2Types.EventTypeOpen,
				sesv2Types.EventTypeClick,
			},
			SnsDestination: &sesv2Types.SnsDestination{
				TopicArn: aws.String(snsTopicArn),
			},
		},
	}

	_, err := sesClient.CreateConfigurationSetEventDestination(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to configure event destination: %w", err)
	}

	logger.Info("successfully configured event destination",
		"configuration_set", configSetName,
		"destination_name", destinationName)

	return nil
}

// GetSPFRecord returns the SPF DNS record for the domain
func GetSPFRecord(domainName string, includeExisting []string) types.DNSRecord {
	// Build SPF record with SES authorization
	includes := []string{"include:amazonses.com"}
	includes = append(includes, includeExisting...)

	spfValue := fmt.Sprintf("v=spf1 %s ~all", strings.Join(includes, " "))

	return types.DNSRecord{
		Type:  "TXT",
		Name:  domainName,
		Value: spfValue,
		TTL:   300,
	}
}

// GetDMARCRecord returns the DMARC DNS record for the domain
func GetDMARCRecord(domainName string, policy string, reportEmail string) types.DNSRecord {
	// Validate policy
	if policy != "none" && policy != "quarantine" && policy != "reject" {
		policy = "none" // Default to monitoring mode
	}

	dmarcValue := fmt.Sprintf("v=DMARC1; p=%s; rua=mailto:%s; pct=100; adkim=s; aspf=s",
		policy, reportEmail)

	return types.DNSRecord{
		Type:  "TXT",
		Name:  "_dmarc." + domainName,
		Value: dmarcValue,
		TTL:   300,
	}
}

// GetDeliverabilityDNSRecords returns all DNS records needed for optimal deliverability
// Deprecated: This function is no longer used. Domain-level deliverability settings are now in EmailConfig.
// Use GetDeliverabilityDNSRecordsWithMailFrom directly with values from EmailConfig.
func GetDeliverabilityDNSRecords(config types.DeliverabilityConfig, domainName string, region string, existingSPFIncludes []string) []types.DNSRecord {
	// This function is deprecated and should not be called
	// Domain-level settings are now in EmailConfig, not DeliverabilityConfig
	return []types.DNSRecord{}
}

// GetDeliverabilityDNSRecordsWithMailFrom returns all DNS records needed for optimal deliverability
// This version accepts the full mailFromDomain and dmarcReportEmail as parameters for better control
func GetDeliverabilityDNSRecordsWithMailFrom(domainName string, mailFromDomain string, dmarcReportEmail string, dmarcPolicy string, region string, existingSPFIncludes []string) []types.DNSRecord {
	records := []types.DNSRecord{}

	// SPF record for main domain
	records = append(records, GetSPFRecord(domainName, existingSPFIncludes))

	// DMARC record
	records = append(records, GetDMARCRecord(domainName, dmarcPolicy, dmarcReportEmail))

	// Custom MAIL FROM domain records (if configured)
	if mailFromDomain != "" {
		records = append(records, GetMailFromDNSRecords(mailFromDomain, region)...)
	}

	return records
}

// VerifyDeliverabilitySetup checks if deliverability features are properly configured
func VerifyDeliverabilitySetup(sesClient *sesv2.Client, domainName string, logger *slog.Logger) error {
	// Get email identity details
	input := &sesv2.GetEmailIdentityInput{
		EmailIdentity: aws.String(domainName),
	}

	result, err := sesClient.GetEmailIdentity(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to get email identity: %w", err)
	}

	logger.Info("deliverability setup verification",
		"domain", domainName,
		"verified", result.VerifiedForSendingStatus,
		"dkim_enabled", result.DkimAttributes != nil && result.DkimAttributes.Status == sesv2Types.DkimStatusSuccess,
		"mail_from_configured", result.MailFromAttributes != nil && result.MailFromAttributes.MailFromDomain != nil)

	// Check DKIM
	if result.DkimAttributes == nil || result.DkimAttributes.Status != sesv2Types.DkimStatusSuccess {
		logger.Warn("DKIM not properly configured", "domain", domainName)
	}

	// Check custom MAIL FROM
	if result.MailFromAttributes == nil || result.MailFromAttributes.MailFromDomain == nil {
		logger.Warn("custom MAIL FROM domain not configured", "domain", domainName)
	}

	// Check configuration set
	if result.ConfigurationSetName == nil {
		logger.Warn("no configuration set assigned to domain", "domain", domainName)
	}

	return nil
}

// AssignConfigurationSetToDomain assigns a configuration set to an email identity
func AssignConfigurationSetToDomain(sesClient *sesv2.Client, domainName string, configSetName string, dryRun bool, logger *slog.Logger) error {
	if dryRun {
		logger.Info("dry-run: would assign configuration set to domain",
			"domain", domainName,
			"configuration_set", configSetName)
		return nil
	}

	input := &sesv2.PutEmailIdentityConfigurationSetAttributesInput{
		EmailIdentity:        aws.String(domainName),
		ConfigurationSetName: aws.String(configSetName),
	}

	_, err := sesClient.PutEmailIdentityConfigurationSetAttributes(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to assign configuration set: %w", err)
	}

	logger.Info("successfully assigned configuration set to domain",
		"domain", domainName,
		"configuration_set", configSetName)

	return nil
}

// GetReputationMetrics retrieves sender reputation metrics
func GetReputationMetrics(sesClient *sesv2.Client, logger *slog.Logger) error {
	input := &sesv2.GetAccountInput{}

	result, err := sesClient.GetAccount(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to get account reputation metrics: %w", err)
	}

	logger.Info("SES account reputation metrics",
		"sending_enabled", result.SendingEnabled,
		"production_access_enabled", result.ProductionAccessEnabled)

	if result.SendQuota != nil {
		logger.Info("send quota",
			"max_24_hour_send", result.SendQuota.Max24HourSend,
			"max_send_rate", result.SendQuota.MaxSendRate,
			"sent_last_24_hours", result.SendQuota.SentLast24Hours)
	}

	return nil
}
