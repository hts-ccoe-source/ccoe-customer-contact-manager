// Package ses provides SES domain validation and management functionality.
package ses

import (
	"context"
	"fmt"
	"log/slog"

	"ccoe-customer-contact-manager/internal/aws"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

// DomainManager manages SES domain validation resources
type DomainManager struct {
	client *sesv2.Client
	logger *slog.Logger
	dryRun bool
}

// DomainConfig holds configuration for domain setup
type DomainConfig struct {
	EmailAddress string // e.g., "ccoe@hearst.com"
	DomainName   string // e.g., "ccoe.hearst.com"
}

// DomainTokens holds the tokens needed for DNS configuration
type DomainTokens struct {
	VerificationToken string
	DKIMTokens        []string // Always 3 tokens
}

// NewDomainManager creates a new domain manager
func NewDomainManager(cfg awssdk.Config, dryRun bool, logger *slog.Logger) *DomainManager {
	return &DomainManager{
		client: sesv2.NewFromConfig(cfg),
		logger: logger,
		dryRun: dryRun,
	}
}

// ConfigureDomain sets up SES email and domain identities with DKIM
func (dm *DomainManager) ConfigureDomain(ctx context.Context, config DomainConfig) (*DomainTokens, error) {
	dm.logger.Info("configuring SES domain",
		"email", config.EmailAddress,
		"domain", config.DomainName,
		"dry_run", dm.dryRun)

	// In dry-run mode, check if domain identity already exists
	if dm.dryRun {
		existingTokens, err := dm.getDomainTokens(ctx, config.DomainName)
		if err != nil {
			// Domain identity doesn't exist
			dm.logger.Info("dry-run: domain identity does not exist, would create",
				"domain", config.DomainName)
			dm.logger.Info("dry-run: would create email identity",
				"email", config.EmailAddress)
			dm.logger.Info("dry-run: would create domain identity with DKIM",
				"domain", config.DomainName)
			dm.logger.Info("dry-run: cannot check DNS records without existing domain identity",
				"domain", config.DomainName)
			// Return nil to skip DNS configuration
			return nil, nil
		}
		// Domain identity exists, proceed with real tokens
		dm.logger.Info("dry-run: domain identity already exists, checking DNS records",
			"domain", config.DomainName,
			"verification_token", existingTokens.VerificationToken,
			"dkim_token_count", len(existingTokens.DKIMTokens))
		return existingTokens, nil
	}

	// Normal mode: create identities
	// Create email identity
	if err := dm.createEmailIdentity(ctx, config.EmailAddress); err != nil {
		return nil, fmt.Errorf("failed to create email identity: %w", err)
	}

	// Create domain identity with DKIM
	if err := dm.createDomainIdentity(ctx, config.DomainName); err != nil {
		return nil, fmt.Errorf("failed to create domain identity: %w", err)
	}

	// Retrieve tokens
	tokens, err := dm.getDomainTokens(ctx, config.DomainName)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain tokens: %w", err)
	}

	if dm.dryRun {
		dm.logger.Info("dry-run: would configure SES domain",
			"email", config.EmailAddress,
			"domain", config.DomainName,
			"verification_token", tokens.VerificationToken,
			"dkim_token_count", len(tokens.DKIMTokens))
	} else {
		dm.logger.Info("successfully configured SES domain",
			"email", config.EmailAddress,
			"domain", config.DomainName,
			"verification_token", tokens.VerificationToken,
			"dkim_token_count", len(tokens.DKIMTokens))
	}

	return tokens, nil
}

// createEmailIdentity creates an SESV2 email identity
func (dm *DomainManager) createEmailIdentity(ctx context.Context, emailAddress string) error {
	dm.logger.Info("creating email identity",
		"email", emailAddress,
		"dry_run", dm.dryRun)

	if dm.dryRun {
		dm.logger.Info("dry-run mode: would create email identity",
			"email", emailAddress)
		return nil
	}

	input := &sesv2.CreateEmailIdentityInput{
		EmailIdentity: awssdk.String(emailAddress),
	}

	// Wrap API call with retry logic
	err := aws.RetryableOperation(ctx, "CreateEmailIdentity", func() error {
		_, err := dm.client.CreateEmailIdentity(ctx, input)
		if err != nil {
			// Check if identity already exists (idempotent behavior)
			if isAlreadyExistsError(err) {
				dm.logger.Info("email identity already exists",
					"email", emailAddress)
				return nil
			}
			return err
		}
		return nil
	}, dm.logger)

	if err != nil {
		return aws.WrapAWSError(err, fmt.Sprintf("create email identity %s", emailAddress))
	}

	dm.logger.Info("successfully created email identity",
		"email", emailAddress)
	return nil
}

// isAlreadyExistsError checks if the error indicates the resource already exists
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	// Check for AlreadyExistsException in error message
	errMsg := err.Error()
	return contains(errMsg, "AlreadyExistsException") || contains(errMsg, "already exists")
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

// stringContains is a simple substring check
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createDomainIdentity creates an SESV2 domain identity with DKIM enabled
func (dm *DomainManager) createDomainIdentity(ctx context.Context, domainName string) error {
	dm.logger.Info("creating domain identity with DKIM",
		"domain", domainName,
		"dry_run", dm.dryRun)

	if dm.dryRun {
		dm.logger.Info("dry-run mode: would create domain identity with DKIM",
			"domain", domainName)
		return nil
	}

	input := &sesv2.CreateEmailIdentityInput{
		EmailIdentity: awssdk.String(domainName),
		// Use AWS-managed DKIM (omit DkimSigningAttributes for AWS_SES signing)
	}

	// Wrap API call with retry logic
	err := aws.RetryableOperation(ctx, "CreateDomainIdentity", func() error {
		_, err := dm.client.CreateEmailIdentity(ctx, input)
		if err != nil {
			// Check if identity already exists (idempotent behavior)
			if isAlreadyExistsError(err) {
				dm.logger.Info("domain identity already exists",
					"domain", domainName)
				return nil
			}
			return err
		}
		return nil
	}, dm.logger)

	if err != nil {
		return aws.WrapAWSError(err, fmt.Sprintf("create domain identity %s", domainName))
	}

	dm.logger.Info("successfully created domain identity with DKIM",
		"domain", domainName)
	return nil
}

// getDomainTokens retrieves verification and DKIM tokens for a domain
// Note: This is a read-only operation, so it runs even in dry-run mode to provide accurate information
func (dm *DomainManager) getDomainTokens(ctx context.Context, domainName string) (*DomainTokens, error) {
	dm.logger.Info("retrieving domain tokens",
		"domain", domainName,
		"dry_run", dm.dryRun)

	input := &sesv2.GetEmailIdentityInput{
		EmailIdentity: awssdk.String(domainName),
	}

	var tokens *DomainTokens

	// Wrap API call with retry logic
	err := aws.RetryableOperation(ctx, "GetEmailIdentity", func() error {
		result, err := dm.client.GetEmailIdentity(ctx, input)
		if err != nil {
			return err
		}

		// Extract DKIM attributes
		if result.DkimAttributes == nil {
			return fmt.Errorf("no DKIM attributes found for domain %s", domainName)
		}

		// Extract verification token
		verificationToken := ""
		if len(result.DkimAttributes.Tokens) > 0 {
			verificationToken = result.DkimAttributes.Tokens[0]
		}

		// Extract DKIM tokens
		dkimTokens := result.DkimAttributes.Tokens
		if len(dkimTokens) == 0 {
			dkimTokens = []string{}
		}

		// Validate that exactly 3 DKIM tokens are returned
		if len(dkimTokens) != 3 {
			return fmt.Errorf("expected 3 DKIM tokens for domain %s, got %d", domainName, len(dkimTokens))
		}

		tokens = &DomainTokens{
			VerificationToken: verificationToken,
			DKIMTokens:        dkimTokens,
		}

		return nil
	}, dm.logger)

	if err != nil {
		return nil, aws.WrapAWSError(err, fmt.Sprintf("get email identity %s", domainName))
	}

	dm.logger.Info("successfully retrieved domain tokens",
		"domain", domainName,
		"verification_token", tokens.VerificationToken,
		"dkim_token_count", len(tokens.DKIMTokens))

	return tokens, nil
}
