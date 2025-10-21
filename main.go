package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"ccoe-customer-contact-manager/internal/aws"
	"ccoe-customer-contact-manager/internal/config"
	"ccoe-customer-contact-manager/internal/contacts"
	"ccoe-customer-contact-manager/internal/lambda"
	"ccoe-customer-contact-manager/internal/route53"
	"ccoe-customer-contact-manager/internal/ses"
	"ccoe-customer-contact-manager/internal/types"
)

// Version information
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// CustomerImportResult represents the result of importing contacts for a single customer
type CustomerImportResult struct {
	CustomerCode    string `json:"customer_code"`
	Success         bool   `json:"success"`
	Error           error  `json:"error,omitempty"`
	UsersProcessed  int    `json:"users_processed"`
	ContactsAdded   int    `json:"contacts_added"`
	ContactsUpdated int    `json:"contacts_updated"`
	ContactsSkipped int    `json:"contacts_skipped"`
}

// assumeSESRole assumes an SES role for a customer and returns an AWS config with the assumed credentials
func assumeSESRole(sesRoleArn string, customerCode string, region string) (awssdk.Config, error) {
	// Load base AWS config
	baseConfig, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region))
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(baseConfig)

	// Assume SES role
	sessionName := fmt.Sprintf("ses-import-%s", customerCode)
	assumedCreds, err := aws.AssumeRole(stsClient, sesRoleArn, sessionName)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("failed to assume SES role: %w", err)
	}

	// Create AWS config with assumed credentials
	sesAwsCreds := awssdk.Credentials{
		AccessKeyID:     *assumedCreds.AccessKeyId,
		SecretAccessKey: *assumedCreds.SecretAccessKey,
		SessionToken:    *assumedCreds.SessionToken,
		Source:          "AssumeRole",
	}

	sesConfig, err := aws.CreateConnectionConfiguration(sesAwsCreds)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("failed to create SES config: %w", err)
	}

	// Set the region for the SES config
	sesConfig.Region = region

	return sesConfig, nil
}

func main() {
	// Check if running in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		// Running in Lambda - start Lambda handler immediately
		lambda.StartLambdaMode()
		return
	}

	// Check for subcommands
	if len(os.Args) < 2 {
		showUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "alt-contact":
		handleAltContactCommand()
	case "ses":
		handleSESCommand()
	case "validate-customers":
		handleValidateCustomersCommand()
	case "extract-customers":
		handleExtractCustomersCommand()
	case "configure-s3-events":
		handleConfigureS3EventsCommand()
	case "test-s3-events":
		handleTestS3EventsCommand()
	case "validate-s3-events":
		handleValidateS3EventsCommand()
	case "version":
		showVersion()
	case "help", "--help", "-h":
		showUsage()
	default:
		fmt.Printf("Unknown command: %s\n", subcommand)
		showUsage()
		os.Exit(1)
	}
}

func showUsage() {
	fmt.Printf("CCOE Customer Contact Manager\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("  ccoe-customer-contact-manager <command> [options]\n\n")
	fmt.Printf("COMMANDS:\n")
	fmt.Printf("  alt-contact           Manage AWS alternate contacts\n")
	fmt.Printf("  ses                   Manage SES contact lists and emails\n")
	fmt.Printf("  validate-customers    Validate customer codes\n")
	fmt.Printf("  extract-customers     Extract customers from metadata\n")
	fmt.Printf("  configure-s3-events   Configure S3 event notifications\n")
	fmt.Printf("  test-s3-events        Test S3 event delivery\n")
	fmt.Printf("  validate-s3-events    Validate S3 event configuration\n")
	fmt.Printf("  version               Show version information\n")
	fmt.Printf("  help                  Show this help message\n\n")
	fmt.Printf("Use 'ccoe-customer-contact-manager <command> --help' for command-specific help.\n")
}

func showVersion() {
	fmt.Printf("CCOE Customer Contact Manager\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Build Time: %s\n", BuildTime)
	fmt.Printf("Git Commit: %s\n", GitCommit)
}

func handleAltContactCommand() {
	fs := flag.NewFlagSet("alt-contact", flag.ExitOnError)

	action := fs.String("action", "", "Action to perform: set-all, set-one, delete")
	contactConfigFile := fs.String("contact-config-file", "ContactConfig.json", "Contact configuration file")
	orgPrefix := fs.String("org-prefix", "", "Organization prefix (required for set-one and delete)")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing contacts")
	contactTypes := fs.String("contact-types", "", "Comma-separated contact types for delete action")

	fs.Parse(os.Args[2:])

	if *action == "" {
		fmt.Printf("alt-contact command usage:\n")
		fmt.Printf("  --action string         Action to perform: set-all, set-one, delete\n")
		fmt.Printf("  --contact-config-file   Contact configuration file (default: ContactConfig.json)\n")
		fmt.Printf("  --org-prefix string     Organization prefix (required for set-one and delete)\n")
		fmt.Printf("  --overwrite             Overwrite existing contacts\n")
		fmt.Printf("  --contact-types string  Comma-separated contact types for delete action\n")
		return
	}

	switch *action {
	case "set-all":
		contacts.SetContactsForAllOrganizations(contactConfigFile, overwrite)
	case "set-one":
		if *orgPrefix == "" {
			fmt.Println("Error: org-prefix is required for set-one action")
			return
		}
		contacts.SetContactsForSingleOrganization(contactConfigFile, orgPrefix, overwrite)
	case "delete":
		if *orgPrefix == "" {
			fmt.Println("Error: org-prefix is required for delete action")
			return
		}
		contacts.DeleteContactsFromOrganization(orgPrefix, contactTypes)
	default:
		fmt.Printf("Unknown action: %s\n", *action)
	}
}

func handleSESConfigureDomainAction(cfg *types.Config, customerCode *string, dryRun, configureDNS *bool, dnsRoleArn, logLevel, logFormat *string) {
	// Validate configuration
	if err := config.ValidateRoute53Config(cfg); err != nil {
		if *configureDNS {
			log.Fatalf("Configuration validation failed: %v", err)
		}
	}

	// Setup structured logging
	slogLevel := slog.LevelInfo
	logLevelStr := cfg.LogLevel
	if *logLevel != "" && *logLevel != "info" {
		logLevelStr = *logLevel
	}
	switch strings.ToLower(logLevelStr) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	}

	// Create logger with appropriate handler based on format
	var handler slog.Handler
	if strings.ToLower(*logFormat) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slogLevel,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slogLevel,
		})
	}
	logger := slog.New(handler)

	// Allow command-line override of DNS role ARN
	if *configureDNS && *dnsRoleArn != "" {
		cfg.Route53Config.RoleARN = *dnsRoleArn
	}

	// Determine which customers to process
	var customersToProcess []string
	if *customerCode != "" {
		// Single customer mode - validate customer code
		if err := config.ValidateCustomerCode(cfg, *customerCode); err != nil {
			log.Fatalf("Customer validation failed: %v", err)
		}
		customersToProcess = []string{*customerCode}
	} else {
		// All customers mode
		for code := range cfg.CustomerMappings {
			customersToProcess = append(customersToProcess, code)
		}
	}

	fmt.Printf("🔧 SES Domain Configuration\n")
	fmt.Printf("Customers to process: %d\n", len(customersToProcess))
	fmt.Printf("Configure DNS: %v\n", *configureDNS)
	fmt.Printf("Dry run: %v\n\n", *dryRun)

	// Collect tokens in memory (map[customerCode]DomainTokens)
	type DomainTokens struct {
		VerificationToken string
		DKIMTokens        []string
	}
	tokensMap := make(map[string]DomainTokens)

	successCount := 0
	errorCount := 0
	var errors []string

	// Create credential manager
	credentialManager, err := aws.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		logger.Error("failed to create credential manager", "error", err)
		os.Exit(1)
	}

	logger.Info("starting SES domain configuration",
		"customers_to_process", len(customersToProcess),
		"configure_dns", *configureDNS,
		"dry_run", *dryRun)

	// Get domain name from Route53 zone if DNS configuration is enabled
	var domainName string
	if *configureDNS {
		// Load base AWS config to get zone name
		baseConfig, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.AWSRegion),
		)
		if err != nil {
			logger.Error("failed to load AWS config", "error", err)
			os.Exit(1)
		}

		// Create STS client and assume DNS role
		stsClient := sts.NewFromConfig(baseConfig)
		sessionName := "ses-domain-validation-dns-lookup"

		logger.Info("assuming DNS role to lookup zone name",
			"role_arn", cfg.Route53Config.RoleARN,
			"session_name", sessionName)

		assumedCreds, err := aws.AssumeRole(stsClient, cfg.Route53Config.RoleARN, sessionName)
		if err != nil {
			logger.Error("failed to assume DNS role for zone lookup",
				"role_arn", cfg.Route53Config.RoleARN,
				"error", err)
			os.Exit(1)
		}

		// Create DNS config from assumed credentials
		dnsAwsCreds := awssdk.Credentials{
			AccessKeyID:     *assumedCreds.AccessKeyId,
			SecretAccessKey: *assumedCreds.SecretAccessKey,
			SessionToken:    *assumedCreds.SessionToken,
			Source:          "AssumeRole",
		}

		dnsConfig, err := aws.CreateConnectionConfiguration(dnsAwsCreds)
		if err != nil {
			logger.Error("failed to create DNS config", "error", err)
			os.Exit(1)
		}

		// Create temporary DNS manager to get zone name
		tempDNSManager := route53.NewDNSManager(dnsConfig, *dryRun, logger)
		domainName, err = tempDNSManager.GetHostedZoneName(context.Background(), cfg.Route53Config.ZoneID)
		if err != nil {
			logger.Error("failed to get hosted zone name", "error", err)
			os.Exit(1)
		}

		logger.Info("using domain from Route53 zone",
			"zone_id", cfg.Route53Config.ZoneID,
			"domain_name", domainName)
	} else {
		// If not configuring DNS, extract domain from email
		domainName = extractDomainFromEmail(cfg.EmailConfig.SenderAddress)
		logger.Info("using domain from email address",
			"email", cfg.EmailConfig.SenderAddress,
			"domain_name", domainName)
	}

	// Process each customer
	for _, custCode := range customersToProcess {
		customerInfo := cfg.CustomerMappings[custCode]

		logger.Info("processing customer",
			"customer_code", custCode,
			"customer_name", customerInfo.CustomerName,
			"environment", customerInfo.Environment)

		// Get customer AWS config (assumes role automatically)
		customerConfig, err := credentialManager.GetCustomerConfig(custCode)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get customer config for %s: %v", custCode, err)
			logger.Error("failed to get customer config",
				"customer_code", custCode,
				"error", err)
			errors = append(errors, errMsg)
			errorCount++
			continue
		}

		// Create domain manager
		domainManager := ses.NewDomainManager(customerConfig, *dryRun, logger)

		// Configure domain (email identity + domain identity with DKIM)
		domainConfig := ses.DomainConfig{
			EmailAddress: cfg.EmailConfig.SenderAddress,
			DomainName:   domainName,
		}

		logger.Info("configuring SES domain for customer",
			"customer_code", custCode,
			"email_address", domainConfig.EmailAddress,
			"domain_name", domainConfig.DomainName,
			"dry_run", *dryRun)

		tokens, err := domainManager.ConfigureDomain(context.Background(), domainConfig)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to configure domain for customer %s: %v", custCode, err)
			logger.Error("failed to configure domain",
				"customer_code", custCode,
				"error", err)
			errors = append(errors, errMsg)
			errorCount++
			continue
		}

		// Store tokens for DNS configuration
		if tokens != nil {
			tokensMap[custCode] = DomainTokens{
				VerificationToken: tokens.VerificationToken,
				DKIMTokens:        tokens.DKIMTokens,
			}
			if *dryRun {
				logger.Info("dry-run: would configure SES domain for customer",
					"customer_code", custCode,
					"verification_token", tokens.VerificationToken,
					"dkim_token_count", len(tokens.DKIMTokens))
			} else {
				logger.Info("successfully configured SES domain",
					"customer_code", custCode,
					"verification_token", tokens.VerificationToken,
					"dkim_token_count", len(tokens.DKIMTokens))
			}
		}

		successCount++
	}

	// Configure DNS if requested
	if *configureDNS && len(tokensMap) > 0 {
		logger.Info("starting DNS configuration",
			"zone_id", cfg.Route53Config.ZoneID,
			"role_arn", cfg.Route53Config.RoleARN,
			"organizations", len(tokensMap),
			"dry_run", *dryRun)

		// Load base AWS config
		baseConfig, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.AWSRegion),
		)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to load AWS config: %v", err)
			logger.Error("failed to load AWS config", "error", err)
			errors = append(errors, errMsg)
			errorCount++
		} else {
			// Create STS client
			stsClient := sts.NewFromConfig(baseConfig)

			// Assume DNS role
			sessionName := "ses-domain-validation-dns"
			logger.Info("assuming DNS role",
				"role_arn", cfg.Route53Config.RoleARN,
				"session_name", sessionName)

			assumedCreds, err := aws.AssumeRole(stsClient, cfg.Route53Config.RoleARN, sessionName)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to assume DNS role: %v", err)
				logger.Error("failed to assume DNS role",
					"role_arn", cfg.Route53Config.RoleARN,
					"error", err)
				errors = append(errors, errMsg)
				errorCount++
			} else {
				// Create DNS config from assumed credentials
				dnsAwsCreds := awssdk.Credentials{
					AccessKeyID:     *assumedCreds.AccessKeyId,
					SecretAccessKey: *assumedCreds.SecretAccessKey,
					SessionToken:    *assumedCreds.SessionToken,
					Source:          "AssumeRole",
				}

				dnsConfig, err := aws.CreateConnectionConfiguration(dnsAwsCreds)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to create DNS config: %v", err)
					logger.Error("failed to create DNS config", "error", err)
					errors = append(errors, errMsg)
					errorCount++
				} else {
					// Create DNS manager
					dnsManager := route53.NewDNSManager(dnsConfig, *dryRun, logger)

					// Build organization DNS configurations
					var orgs []route53.OrganizationDNS
					for custCode, tokens := range tokensMap {
						orgs = append(orgs, route53.OrganizationDNS{
							Name:              custCode,
							DKIMTokens:        tokens.DKIMTokens,
							VerificationToken: tokens.VerificationToken,
						})
					}

					// Configure DNS records
					dnsConfigStruct := route53.DNSConfig{
						ZoneID: cfg.Route53Config.ZoneID,
					}

					err = dnsManager.ConfigureRecords(context.Background(), dnsConfigStruct, orgs)
					if err != nil {
						errMsg := fmt.Sprintf("Failed to configure DNS records: %v", err)
						logger.Error("failed to configure DNS records", "error", err)
						errors = append(errors, errMsg)
						errorCount++
					}
					// Success logging is handled by the DNSManager
				}
			}
		}
	}

	// Calculate DNS records created
	dnsRecordsCreated := 0
	if *configureDNS && len(tokensMap) > 0 {
		dnsRecordsCreated = len(tokensMap) * 4 // 3 DKIM + 1 verification per customer
	}

	// Output summary
	logger.Info("SES domain configuration summary",
		"total_customers", len(customersToProcess),
		"successful", successCount,
		"failed", errorCount,
		"identities_created", successCount*2, // email + domain per customer
		"tokens_retrieved", len(tokensMap),
		"dns_configured", *configureDNS,
		"dns_records_created", dnsRecordsCreated,
		"dry_run", *dryRun)

	if errorCount > 0 {
		logger.Error("operation completed with errors",
			"error_count", errorCount,
			"errors", errors)
		os.Exit(1)
	}
}

// extractDomainFromEmail extracts the domain part from an email address
func extractDomainFromEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return email
}

func handleSESCommand() {
	fs := flag.NewFlagSet("ses", flag.ExitOnError)

	action := fs.String("action", "", "SES action to perform")
	configFile := fs.String("config-file", "", "Configuration file path")
	customerCode := fs.String("customer-code", "", "Customer code")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	email := fs.String("email", "", "Email address")
	identityCenterID := fs.String("identity-center-id", "", "Identity Center instance ID (format: d-xxxxxxxxxx)")
	identityCenterRoleArn := fs.String("identity-center-role-arn", "", "Identity Center role ARN for in-memory data retrieval (overrides config)")
	logLevel := fs.String("log-level", "info", "Log level")
	maxConcurrency := fs.Int("max-concurrency", 10, "Maximum concurrent workers for Identity Center operations")
	mgmtRoleArn := fs.String("mgmt-role-arn", "", "Management account IAM role ARN for Identity Center operations")
	requestsPerSecond := fs.Int("requests-per-second", 10, "API requests per second rate limit")
	senderEmail := fs.String("sender-email", "", "Sender email address for test emails")
	suppressionReason := fs.String("suppression-reason", "bounce", "Suppression reason: bounce or complaint")
	topicName := fs.String("topic-name", "", "Topic name")
	topics := fs.String("topics", "", "Comma-separated topics")
	username := fs.String("username", "", "Username to search for in Identity Center")
	backupFile := fs.String("backup-file", "", "Backup file for restore operations")
	jsonMetadata := fs.String("json-metadata", "", "Path to JSON metadata file from metadata collector")
	htmlTemplate := fs.String("html-template", "", "Path to HTML email template file")
	forceUpdate := fs.Bool("force-update", false, "Force update existing meetings regardless of detected changes")
	// Flags for configure-domain action
	configureDNS := fs.Bool("configure-dns", true, "Automatically configure Route53 DNS records (for configure-domain action)")
	dnsRoleArn := fs.String("dns-role-arn", "", "IAM role ARN for DNS account (for configure-domain action)")
	logFormat := fs.String("log-format", "text", "Log output format: json or text")

	fs.Parse(os.Args[2:])

	if *action == "" {
		showSESUsage()
		return
	}

	// Setup logging
	config.SetupLogging(*logLevel)

	// Handle actions that don't require customer code first
	switch *action {
	case "help":
		showSESUsage()
		return
	}

	// Load configuration - default to ./config.json if not specified
	var cfg *types.Config
	var err error
	configPath := *configFile
	if configPath == "" {
		configPath = "./config.json"
	}

	// Load config file (required for most SES operations)
	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config from %s: %v", configPath, err)
	}

	// For customer-specific actions, validate customer code
	if *customerCode != "" && cfg != nil {
		if _, exists := cfg.CustomerMappings[*customerCode]; !exists {
			log.Fatalf("Customer code %s not found in configuration", *customerCode)
		}
	}

	// Create credential manager if we have config and customer code
	var credentialManager *aws.CredentialManager
	if cfg != nil && *customerCode != "" {
		credentialManager, err = aws.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
		if err != nil {
			log.Fatalf("Failed to create credential manager: %v", err)
		}
	}

	// Create email manager if we have credential manager
	var emailManager *ses.EmailManager
	if credentialManager != nil {
		emailManager = ses.NewEmailManager(credentialManager, cfg.ContactConfig)
	}

	// Execute the requested action
	switch *action {
	case "configure-domain":
		handleSESConfigureDomainAction(cfg, customerCode, dryRun, configureDNS, dnsRoleArn, logLevel, logFormat)
		return
	case "create-contact-list":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for create-contact-list action")
		}
		handleCreateContactList(customerCode, credentialManager, *dryRun)
	case "list-contact-lists":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for list-contact-lists action")
		}
		handleListContactLists(customerCode, credentialManager)
	case "describe-list":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for describe-list action")
		}
		handleDescribeContactList(customerCode, credentialManager)
	case "add-contact":
		handleAddContact(customerCode, credentialManager, email, topics, *dryRun)
	case "remove-contact":
		handleRemoveContact(customerCode, credentialManager, email, *dryRun)
	case "list-contacts":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for list-contacts action")
		}
		handleListContacts(customerCode, credentialManager)
	case "describe-contact":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for describe-contact action")
		}
		handleDescribeContact(customerCode, credentialManager, email)
	case "add-contact-topics":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for add-contact-topics action")
		}
		handleAddContactTopics(customerCode, credentialManager, email, topics, *dryRun)
	case "remove-contact-topics":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for remove-contact-topics action")
		}
		handleRemoveContactTopics(customerCode, credentialManager, email, topics, *dryRun)
	case "describe-topic":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for describe-topic action")
		}
		handleDescribeTopic(customerCode, credentialManager, topicName)
	case "add-to-suppression":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for add-to-suppression action")
		}
		handleAddToSuppression(customerCode, credentialManager, email, suppressionReason, *dryRun)
	case "remove-from-suppression":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for remove-from-suppression action")
		}
		handleRemoveFromSuppression(customerCode, credentialManager, email, *dryRun)
	case "backup-contact-list":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for backup-contact-list action")
		}
		handleBackupContactList(customerCode, credentialManager, *action)
	case "remove-all-contacts":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for remove-all-contacts action")
		}
		handleRemoveAllContacts(customerCode, credentialManager, *dryRun)
	case "send-test-email":
		handleSendTestEmail(emailManager, customerCode, senderEmail, email, *dryRun)
	case "validate-customer":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for validate-customer action")
		}
		handleValidateCustomer(credentialManager, customerCode)
	case "describe-topic-all":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for describe-topic-all action")
		}
		handleDescribeTopicAll(customerCode, credentialManager)
	case "send-topic-test":
		if credentialManager == nil {
			log.Fatal("Configuration file and customer code are required for send-topic-test action")
		}
		handleSendTopicTest(customerCode, credentialManager, topicName, senderEmail, *dryRun)
	case "update-topic":
		handleUpdateTopic(customerCode, credentialManager, configFile, *dryRun)
	case "subscribe":
		handleSubscribe(customerCode, credentialManager, configFile, *dryRun)
	case "unsubscribe":
		handleUnsubscribe(customerCode, credentialManager, configFile, *dryRun)
	case "send-general-preferences":
		handleSendGeneralPreferences(customerCode, credentialManager, senderEmail, *dryRun)
	case "send-approval-request":
		handleSendApprovalRequest(customerCode, credentialManager, topicName, jsonMetadata, htmlTemplate, senderEmail, *dryRun)
	case "send-change-notification":
		handleSendChangeNotification(customerCode, credentialManager, topicName, jsonMetadata, senderEmail, *dryRun)
	case "create-ics-invite":
		handleCreateICSInvite(customerCode, credentialManager, topicName, jsonMetadata, senderEmail, *dryRun)

	case "create-multi-customer-meeting-invite":
		handleCreateMultiCustomerMeetingInvite(credentialManager, jsonMetadata, topicName, senderEmail, *dryRun, *forceUpdate)
	case "list-identity-center-user":
		handleListIdentityCenterUser(mgmtRoleArn, identityCenterID, username, *maxConcurrency, *requestsPerSecond)
	case "list-identity-center-user-all":
		handleListIdentityCenterUserAll(mgmtRoleArn, identityCenterID, *maxConcurrency, *requestsPerSecond)
	case "list-group-membership":
		handleListGroupMembership(mgmtRoleArn, identityCenterID, username, *maxConcurrency, *requestsPerSecond)
	case "list-group-membership-all":
		handleListGroupMembershipAll(mgmtRoleArn, identityCenterID, *maxConcurrency, *requestsPerSecond)
	case "import-aws-contact":
		handleImportAWSContact(customerCode, credentialManager, mgmtRoleArn, identityCenterID, username, *maxConcurrency, *requestsPerSecond, *dryRun, configFile)
	case "import-aws-contact-all":
		handleImportAWSContactAll(cfg, customerCode, identityCenterRoleArn, *maxConcurrency, *requestsPerSecond, *dryRun)
	default:
		fmt.Printf("Unknown SES action: %s\n", *action)
		showSESUsage()
		os.Exit(1)
	}

	// Suppress unused variable warnings for now
	_ = backupFile
}

// extractCustomerCodesFromMetadata extracts customer codes from a metadata JSON file
func extractCustomerCodesFromMetadata(jsonMetadataPath string) ([]string, error) {
	// Read the metadata file
	data, err := os.ReadFile(jsonMetadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file %s: %w", jsonMetadataPath, err)
	}

	// Try to parse as ChangeMetadata (flat format)
	var changeMetadata types.ChangeMetadata
	if err := json.Unmarshal(data, &changeMetadata); err == nil && len(changeMetadata.Customers) > 0 {
		return changeMetadata.Customers, nil
	}

	// Try to parse as ApprovalRequestMetadata (nested format)
	var approvalMetadata types.ApprovalRequestMetadata
	if err := json.Unmarshal(data, &approvalMetadata); err == nil && len(approvalMetadata.ChangeMetadata.CustomerCodes) > 0 {
		return approvalMetadata.ChangeMetadata.CustomerCodes, nil
	}

	// Try to parse as generic JSON and look for customer codes in various fields
	var genericData map[string]interface{}
	if err := json.Unmarshal(data, &genericData); err != nil {
		return nil, fmt.Errorf("failed to parse metadata as JSON: %w", err)
	}

	// Look for customer codes in common field names
	customerFields := []string{"customers", "customerCodes", "customer_codes", "affectedCustomers"}

	for _, field := range customerFields {
		if value, exists := genericData[field]; exists {
			switch v := value.(type) {
			case []interface{}:
				var customerCodes []string
				for _, item := range v {
					if str, ok := item.(string); ok {
						customerCodes = append(customerCodes, str)
					}
				}
				if len(customerCodes) > 0 {
					return customerCodes, nil
				}
			case []string:
				if len(v) > 0 {
					return v, nil
				}
			}
		}
	}

	// Check nested changeMetadata field
	if changeMetadataField, exists := genericData["changeMetadata"]; exists {
		if changeMetadataMap, ok := changeMetadataField.(map[string]interface{}); ok {
			for _, field := range customerFields {
				if value, exists := changeMetadataMap[field]; exists {
					switch v := value.(type) {
					case []interface{}:
						var customerCodes []string
						for _, item := range v {
							if str, ok := item.(string); ok {
								customerCodes = append(customerCodes, str)
							}
						}
						if len(customerCodes) > 0 {
							return customerCodes, nil
						}
					case []string:
						if len(v) > 0 {
							return v, nil
						}
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("no customer codes found in metadata file %s", jsonMetadataPath)
}

func handleValidateCustomersCommand() {
	fs := flag.NewFlagSet("validate-customers", flag.ExitOnError)
	configFile := fs.String("config-file", "", "Configuration file path")
	logLevel := fs.String("log-level", "info", "Log level")

	fs.Parse(os.Args[2:])

	// Setup logging
	config.SetupLogging(*logLevel)

	if *configFile == "" {
		log.Fatal("Configuration file is required for validate-customers command")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate configuration
	err = config.ValidateConfig(cfg)
	if err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Create credential manager
	credentialManager, err := aws.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		log.Fatalf("Failed to create credential manager: %v", err)
	}

	// Validate each customer
	customers := credentialManager.ListCustomers()
	fmt.Printf("Validating %d customers...\n\n", len(customers))

	successCount := 0
	errorCount := 0

	for _, customerCode := range customers {
		fmt.Printf("Validating customer: %s\n", customerCode)

		err := credentialManager.ValidateCustomerAccess(customerCode)
		if err != nil {
			fmt.Printf("❌ Customer %s validation failed: %v\n", customerCode, err)
			errorCount++
		} else {
			fmt.Printf("✅ Customer %s validation successful\n", customerCode)
			successCount++
		}
		fmt.Println()
	}

	fmt.Printf("Validation complete: %d successful, %d errors\n", successCount, errorCount)
	if errorCount > 0 {
		os.Exit(1)
	}
}

func handleExtractCustomersCommand() {
	fs := flag.NewFlagSet("extract-customers", flag.ExitOnError)
	inputFile := fs.String("input-file", "", "Input metadata file path")
	outputFile := fs.String("output-file", "extracted-customers.json", "Output file path")
	logLevel := fs.String("log-level", "info", "Log level")

	fs.Parse(os.Args[2:])

	// Setup logging
	config.SetupLogging(*logLevel)

	if *inputFile == "" {
		log.Fatal("Input file is required for extract-customers command")
	}

	// Read input file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Try to parse as different metadata formats
	var metadata interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		log.Fatalf("Failed to parse input file as JSON: %v", err)
	}

	// Extract customer codes (this is a simplified implementation)
	// In a real implementation, this would parse specific metadata formats
	fmt.Printf("Extracting customer codes from %s...\n", *inputFile)

	// For now, just show that we processed the file
	fmt.Printf("Processed metadata file: %s\n", *inputFile)
	fmt.Printf("Results would be saved to: %s\n", *outputFile)
	fmt.Println("Note: This is a placeholder implementation")
}

func handleConfigureS3EventsCommand() {
	fs := flag.NewFlagSet("configure-s3-events", flag.ExitOnError)
	configFile := fs.String("config-file", "", "Configuration file path")
	bucketName := fs.String("bucket-name", "", "S3 bucket name")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	logLevel := fs.String("log-level", "info", "Log level")

	fs.Parse(os.Args[2:])

	// Setup logging
	config.SetupLogging(*logLevel)

	if *bucketName == "" {
		log.Fatal("Bucket name is required for configure-s3-events command")
	}

	if *dryRun {
		fmt.Printf("DRY RUN: Would configure S3 events for bucket: %s\n", *bucketName)
		if *configFile != "" {
			fmt.Printf("Using configuration file: %s\n", *configFile)
		}
		return
	}

	fmt.Printf("Configuring S3 events for bucket: %s\n", *bucketName)
	fmt.Println("Note: This is a placeholder implementation")

	// In a real implementation, this would:
	// 1. Load customer configuration
	// 2. Create SQS queues for each customer
	// 3. Configure S3 event notifications
	// 4. Set up proper IAM permissions
}

func handleTestS3EventsCommand() {
	fs := flag.NewFlagSet("test-s3-events", flag.ExitOnError)
	configFile := fs.String("config-file", "", "Configuration file path")
	bucketName := fs.String("bucket-name", "", "S3 bucket name")
	customerCode := fs.String("customer-code", "", "Customer code to test")
	logLevel := fs.String("log-level", "info", "Log level")

	fs.Parse(os.Args[2:])

	// Setup logging
	config.SetupLogging(*logLevel)

	if *bucketName == "" {
		log.Fatal("Bucket name is required for test-s3-events command")
	}

	fmt.Printf("Testing S3 events for bucket: %s\n", *bucketName)
	if *customerCode != "" {
		fmt.Printf("Testing for customer: %s\n", *customerCode)
	}
	if *configFile != "" {
		fmt.Printf("Using configuration file: %s\n", *configFile)
	}

	fmt.Println("Note: This is a placeholder implementation")

	// In a real implementation, this would:
	// 1. Upload test files to S3
	// 2. Verify SQS messages are received
	// 3. Test Lambda processing
	// 4. Validate end-to-end flow
}

func handleValidateS3EventsCommand() {
	fs := flag.NewFlagSet("validate-s3-events", flag.ExitOnError)
	configFile := fs.String("config-file", "", "Configuration file path")
	bucketName := fs.String("bucket-name", "", "S3 bucket name")
	logLevel := fs.String("log-level", "info", "Log level")

	fs.Parse(os.Args[2:])

	// Setup logging
	config.SetupLogging(*logLevel)

	if *bucketName == "" {
		log.Fatal("Bucket name is required for validate-s3-events command")
	}

	fmt.Printf("Validating S3 events configuration for bucket: %s\n", *bucketName)
	if *configFile != "" {
		fmt.Printf("Using configuration file: %s\n", *configFile)
	}

	fmt.Println("Note: This is a placeholder implementation")

	// In a real implementation, this would:
	// 1. Check S3 bucket event configuration
	// 2. Verify SQS queues exist and have proper permissions
	// 3. Test Lambda function configuration
	// 4. Validate IAM roles and policies
	// 5. Report any configuration issues
}
func showSESUsage() {
	fmt.Printf("SES command usage:\n\n")
	fmt.Printf("📧 CONTACT LIST MANAGEMENT:\n")
	fmt.Printf("  create-contact-list     Create a new contact list\n")
	fmt.Printf("  list-contact-lists      List all contact lists\n")
	fmt.Printf("  describe-list           Show detailed contact list information\n")
	fmt.Printf("  add-contact             Add email to contact list\n")
	fmt.Printf("  remove-contact          Remove email from contact list\n")
	fmt.Printf("  list-contacts           List all contacts in contact list\n")
	fmt.Printf("  describe-contact        Show detailed contact information\n")
	fmt.Printf("  add-contact-topics      Add topic subscriptions to contact\n")
	fmt.Printf("  remove-contact-topics   Remove topic subscriptions from contact\n")
	fmt.Printf("  remove-all-contacts     Remove all contacts from list (with backup)\n")
	fmt.Printf("  backup-contact-list     Create backup of contact list\n\n")
	fmt.Printf("🏷️  TOPIC MANAGEMENT:\n")
	fmt.Printf("  describe-topic          Show detailed topic information\n")
	fmt.Printf("  describe-topic-all      Show all topics with statistics\n")
	fmt.Printf("  send-topic-test         Send test email to topic subscribers\n")
	fmt.Printf("  update-topic            Update topics from configuration\n")
	fmt.Printf("  subscribe               Subscribe users based on configuration\n")
	fmt.Printf("  unsubscribe             Unsubscribe users based on configuration\n\n")
	fmt.Printf("🚫 SUPPRESSION MANAGEMENT:\n")
	fmt.Printf("  add-to-suppression      Add email to suppression list\n")
	fmt.Printf("  remove-from-suppression Remove email from suppression list\n\n")
	fmt.Printf("📨 EMAIL & NOTIFICATIONS:\n")
	fmt.Printf("  send-test-email         Send test email\n")
	fmt.Printf("  send-general-preferences Send general preferences email\n")
	fmt.Printf("  send-approval-request   Send approval request email to topic subscribers\n")
	fmt.Printf("  send-change-notification Send change approved/scheduled notification email\n")
	fmt.Printf("  create-ics-invite       Send calendar invite with ICS attachment\n")
	fmt.Printf("  create-multi-customer-meeting-invite Create meeting with recipients from single or multiple customers\n")
	fmt.Printf("  create-multi-customer-meeting-invite Create meeting with recipients from single or multiple customers\n\n")
	fmt.Printf("👥 IDENTITY CENTER INTEGRATION:\n")
	fmt.Printf("  list-identity-center-user     List specific user from Identity Center\n")
	fmt.Printf("  list-identity-center-user-all List ALL users from Identity Center\n")
	fmt.Printf("  list-group-membership         List group memberships for specific user\n")
	fmt.Printf("  list-group-membership-all     List group memberships for ALL users\n\n")
	fmt.Printf("📥 AWS CONTACT IMPORT:\n")
	fmt.Printf("  import-aws-contact            Import specific user to SES based on group memberships\n")
	fmt.Printf("  import-aws-contact-all        Import ALL users to SES based on group memberships\n")
	fmt.Printf("                                Supports in-memory retrieval with --identity-center-role-arn\n")
	fmt.Printf("                                or falls back to file-based import\n\n")
	fmt.Printf("🔧 UTILITIES:\n")
	fmt.Printf("  validate-customer       Validate customer access\n")
	fmt.Printf("  help                    Show this help message\n\n")
	fmt.Printf("COMMON FLAGS:\n")
	fmt.Printf("  --customer-code string          Customer code (optional for import-aws-contact-all)\n")
	fmt.Printf("  --config-file string            Configuration file path\n")
	fmt.Printf("  --email string                  Email address\n")
	fmt.Printf("  --topics string                 Comma-separated topic names\n")
	fmt.Printf("  --topic-name string             Single topic name\n")
	fmt.Printf("  --sender-email string           Sender email address for test emails\n")
	fmt.Printf("  --json-metadata string          Path to JSON metadata file\n")
	fmt.Printf("  --html-template string          Path to HTML email template file\n")
	fmt.Printf("  --mgmt-role-arn string          Management account IAM role ARN for Identity Center\n")
	fmt.Printf("  --identity-center-id            Identity Center instance ID (d-xxxxxxxxxx)\n")
	fmt.Printf("  --identity-center-role-arn      Identity Center role ARN for in-memory retrieval\n")
	fmt.Printf("                                  (overrides config, enables concurrent processing)\n")
	fmt.Printf("  --username string               Username to search in Identity Center\n")
	fmt.Printf("  --max-concurrency int           Max concurrent workers (default: 10)\n")
	fmt.Printf("  --requests-per-second           API requests per second rate limit (default: 10)\n")
	fmt.Printf("  --force-update                  Force update existing meetings\n")
	fmt.Printf("  --dry-run                       Show what would be done without making changes\n")
	fmt.Printf("  --log-level string              Log level (default: info)\n\n")
	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("  # Import contacts for single customer using in-memory retrieval\n")
	fmt.Printf("  ccoe-customer-contact-manager ses --action import-aws-contact-all \\\n")
	fmt.Printf("    --customer-code htsnonprod \\\n")
	fmt.Printf("    --identity-center-role-arn arn:aws:iam::123456789012:role/IdentityCenterRole\n\n")
	fmt.Printf("  # Import contacts for all customers concurrently (uses config)\n")
	fmt.Printf("  ccoe-customer-contact-manager ses --action import-aws-contact-all \\\n")
	fmt.Printf("    --config-file config.json\n\n")
	fmt.Printf("  # Import with CLI override of Identity Center role\n")
	fmt.Printf("  ccoe-customer-contact-manager ses --action import-aws-contact-all \\\n")
	fmt.Printf("    --identity-center-role-arn arn:aws:iam::123456789012:role/IdentityCenterRole \\\n")
	fmt.Printf("    --max-concurrency 5 --dry-run\n")
}

func handleCreateContactList(customerCode *string, credentialManager *aws.CredentialManager, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for create-contact-list action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would create contact list for customer %s\n", *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// For now, create a basic contact list - this could be enhanced to read from config
	listName := fmt.Sprintf("%s-contact-list", *customerCode)
	description := fmt.Sprintf("Contact list for customer %s", *customerCode)

	err = ses.CreateContactList(sesClient, listName, description, []types.SESTopicConfig{})
	if err != nil {
		log.Fatalf("Failed to create contact list: %v", err)
	}

	fmt.Printf("Successfully created contact list: %s\n", listName)
}

func handleListContactLists(customerCode *string, credentialManager *aws.CredentialManager) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for list-contact-lists action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	input := &sesv2.ListContactListsInput{}
	result, err := sesClient.ListContactLists(context.Background(), input)
	if err != nil {
		log.Fatalf("Failed to list contact lists: %v", err)
	}

	if len(result.ContactLists) == 0 {
		fmt.Printf("No contact lists found for customer %s\n", *customerCode)
		return
	}

	fmt.Printf("Contact lists for customer %s:\n", *customerCode)
	for i, list := range result.ContactLists {
		fmt.Printf("%d. %s", i+1, *list.ContactListName)
		// Note: ContactList doesn't have Description field in the list response
		// Description is only available in GetContactList response
		fmt.Printf("\n")
	}
}

func handleDescribeContactList(customerCode *string, credentialManager *aws.CredentialManager) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for describe-list action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	err = ses.DescribeContactList(sesClient, listName)
	if err != nil {
		log.Fatalf("Failed to describe contact list: %v", err)
	}
}

func handleAddContact(customerCode *string, credentialManager *aws.CredentialManager, email *string, topics *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for add-contact action")
	}
	if *email == "" {
		log.Fatal("Email address is required for add-contact action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would add contact %s to customer %s with topics: %s\n", *email, *customerCode, *topics)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	// Parse topics
	var topicList []string
	if *topics != "" {
		topicList = strings.Split(*topics, ",")
		for i, topic := range topicList {
			topicList[i] = strings.TrimSpace(topic)
		}
	}

	action, err := ses.AddOrUpdateContactToList(sesClient, listName, *email, topicList)
	if err != nil {
		log.Fatalf("Failed to add contact: %v", err)
	}

	fmt.Printf("Contact %s %s successfully\n", *email, action)
}

func handleRemoveContact(customerCode *string, credentialManager *aws.CredentialManager, email *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for remove-contact action")
	}
	if *email == "" {
		log.Fatal("Email address is required for remove-contact action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would remove contact %s from customer %s\n", *email, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	err = ses.RemoveContactFromList(sesClient, listName, *email)
	if err != nil {
		log.Fatalf("Failed to remove contact: %v", err)
	}
}

func handleListContacts(customerCode *string, credentialManager *aws.CredentialManager) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for list-contacts action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	err = ses.ListContactsInList(sesClient, listName)
	if err != nil {
		log.Fatalf("Failed to list contacts: %v", err)
	}
}

func handleDescribeContact(customerCode *string, credentialManager *aws.CredentialManager, email *string) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for describe-contact action")
	}
	if *email == "" {
		log.Fatal("Email address is required for describe-contact action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	err = ses.DescribeContact(sesClient, *email)
	if err != nil {
		log.Fatalf("Failed to describe contact: %v", err)
	}
}

func handleAddContactTopics(customerCode *string, credentialManager *aws.CredentialManager, email *string, topics *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for add-contact-topics action")
	}
	if *email == "" {
		log.Fatal("Email address is required for add-contact-topics action")
	}
	if *topics == "" {
		log.Fatal("Topics are required for add-contact-topics action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would add topics %s to contact %s for customer %s\n", *topics, *email, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	// Parse topics
	topicList := strings.Split(*topics, ",")
	for i, topic := range topicList {
		topicList[i] = strings.TrimSpace(topic)
	}

	err = ses.AddContactTopics(sesClient, listName, *email, topicList)
	if err != nil {
		log.Fatalf("Failed to add contact topics: %v", err)
	}
}

func handleRemoveContactTopics(customerCode *string, credentialManager *aws.CredentialManager, email *string, topics *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for remove-contact-topics action")
	}
	if *email == "" {
		log.Fatal("Email address is required for remove-contact-topics action")
	}
	if *topics == "" {
		log.Fatal("Topics are required for remove-contact-topics action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would remove topics %s from contact %s for customer %s\n", *topics, *email, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	// Parse topics
	topicList := strings.Split(*topics, ",")
	for i, topic := range topicList {
		topicList[i] = strings.TrimSpace(topic)
	}

	err = ses.RemoveContactTopics(sesClient, listName, *email, topicList)
	if err != nil {
		log.Fatalf("Failed to remove contact topics: %v", err)
	}
}

func handleDescribeTopic(customerCode *string, credentialManager *aws.CredentialManager, topicName *string) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for describe-topic action")
	}
	if *topicName == "" {
		log.Fatal("Topic name is required for describe-topic action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	err = ses.DescribeTopic(sesClient, *topicName)
	if err != nil {
		log.Fatalf("Failed to describe topic: %v", err)
	}
}

func handleAddToSuppression(customerCode *string, credentialManager *aws.CredentialManager, email *string, suppressionReason *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for add-to-suppression action")
	}
	if *email == "" {
		log.Fatal("Email address is required for add-to-suppression action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would add %s to suppression list for customer %s with reason: %s\n", *email, *customerCode, *suppressionReason)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	var reason sesv2Types.SuppressionListReason
	switch strings.ToLower(*suppressionReason) {
	case "bounce":
		reason = sesv2Types.SuppressionListReasonBounce
	case "complaint":
		reason = sesv2Types.SuppressionListReasonComplaint
	default:
		log.Fatalf("Invalid suppression reason: %s (must be 'bounce' or 'complaint')", *suppressionReason)
	}

	err = ses.AddToSuppressionList(sesClient, *email, reason)
	if err != nil {
		log.Fatalf("Failed to add to suppression list: %v", err)
	}
}

func handleRemoveFromSuppression(customerCode *string, credentialManager *aws.CredentialManager, email *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for remove-from-suppression action")
	}
	if *email == "" {
		log.Fatal("Email address is required for remove-from-suppression action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would remove %s from suppression list for customer %s\n", *email, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	err = ses.RemoveFromSuppressionList(sesClient, *email)
	if err != nil {
		log.Fatalf("Failed to remove from suppression list: %v", err)
	}
}

func handleBackupContactList(customerCode *string, credentialManager *aws.CredentialManager, action string) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for backup-contact-list action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	_, err = ses.CreateContactListBackup(sesClient, listName, action)
	if err != nil {
		log.Fatalf("Failed to create backup: %v", err)
	}
}

func handleRemoveAllContacts(customerCode *string, credentialManager *aws.CredentialManager, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for remove-all-contacts action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would remove all contacts from customer %s (with backup)\n", *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	err = ses.RemoveAllContactsFromList(sesClient, listName)
	if err != nil {
		log.Fatalf("Failed to remove all contacts: %v", err)
	}
}

func handleSendTestEmail(emailManager *ses.EmailManager, customerCode *string, senderEmail *string, email *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for send-test-email action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for send-test-email action")
	}
	if *email == "" {
		log.Fatal("Recipient email is required for send-test-email action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would send test email from %s to %s for customer %s\n", *senderEmail, *email, *customerCode)
		return
	}

	// Create test change details
	changeDetails := map[string]interface{}{
		"change_id":            "TEST-" + strconv.FormatInt(time.Now().Unix(), 10),
		"title":                "Test Email Notification",
		"description":          "This is a test email to verify SES functionality",
		"implementation_plan":  "No implementation required - this is a test",
		"schedule_start":       time.Now().Format("2006-01-02 15:04:05"),
		"schedule_end":         time.Now().Add(1 * time.Hour).Format("2006-01-02 15:04:05"),
		"impact":               "No impact - test only",
		"rollback_plan":        "No rollback required - test only",
		"communication_plan":   "Test email notification",
		"approver":             "Test User",
		"implementer":          "Test System",
		"timestamp":            time.Now().Format("2006-01-02T15:04:05Z"),
		"source":               "CLI Test",
		"test_run":             true,
		"customers":            []string{*customerCode},
		"processing_timestamp": time.Now(),
	}

	err := emailManager.SendAlternateContactNotification(*customerCode, changeDetails)
	if err != nil {
		log.Fatalf("Failed to send test email: %v", err)
	}

	fmt.Printf("Test email sent successfully to %s\n", *email)
}

func handleValidateCustomer(credentialManager *aws.CredentialManager, customerCode *string) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for validate-customer action")
	}

	err := credentialManager.ValidateCustomerAccess(*customerCode)
	if err != nil {
		log.Fatalf("Customer validation failed: %v", err)
	}

	fmt.Printf("Customer %s validation successful\n", *customerCode)
}
func handleDescribeTopicAll(customerCode *string, credentialManager *aws.CredentialManager) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for describe-topic-all action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the main contact list for the account
	listName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		log.Fatalf("Failed to get account contact list: %v", err)
	}

	fmt.Printf("Describing all topics for customer %s (list: %s)\n\n", *customerCode, listName)

	// This would call a function to describe all topics
	// For now, show that it's implemented but needs the internal function
	fmt.Printf("Note: This action requires the DescribeAllTopics function to be implemented in internal/ses\n")
}

func handleSendTopicTest(customerCode *string, credentialManager *aws.CredentialManager, topicName *string, senderEmail *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for send-topic-test action")
	}
	if *topicName == "" {
		log.Fatal("Topic name is required for send-topic-test action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for send-topic-test action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would send test email to topic %s subscribers from %s for customer %s\n", *topicName, *senderEmail, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Sending test email to topic %s subscribers from %s\n", *topicName, *senderEmail)
	fmt.Printf("Note: This action requires the SendTopicTestEmail function to be implemented in internal/ses\n")
}

func handleUpdateTopic(customerCode *string, credentialManager *aws.CredentialManager, configFile *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for update-topic action")
	}

	// Load SES configuration
	sesConfigPath := "SESConfig.json"
	if *configFile != "" {
		sesConfigPath = *configFile
	}

	sesConfig, err := config.LoadSESConfig(sesConfigPath)
	if err != nil {
		log.Fatalf("Failed to load SES config from %s: %v", sesConfigPath, err)
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would update topics for customer %s using config %s\n", *customerCode, sesConfigPath)
		fmt.Printf("Topics to manage: %d\n", len(sesConfig.Topics))
		for _, topic := range sesConfig.Topics {
			fmt.Printf("  - %s: %s\n", topic.TopicName, topic.DisplayName)
		}
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	fmt.Printf("Updating topics for customer %s using config %s\n", *customerCode, sesConfigPath)

	// Expand topics with groups
	expandedTopics := ses.ExpandTopicsWithGroups(*sesConfig)

	// Manage topics
	err = ses.ManageTopics(sesClient, expandedTopics, dryRun)
	if err != nil {
		log.Fatalf("Failed to manage topics: %v", err)
	}

	fmt.Printf("✅ Successfully updated topics for customer %s\n", *customerCode)
}

func handleSubscribe(customerCode *string, credentialManager *aws.CredentialManager, configFile *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for subscribe action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would subscribe users for customer %s using config %s\n", *customerCode, *configFile)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Subscribing users for customer %s using config %s\n", *customerCode, *configFile)
	fmt.Printf("Note: This action requires the ManageSubscriptions function to be implemented in internal/ses\n")
}

func handleUnsubscribe(customerCode *string, credentialManager *aws.CredentialManager, configFile *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for unsubscribe action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would unsubscribe users for customer %s using config %s\n", *customerCode, *configFile)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Unsubscribing users for customer %s using config %s\n", *customerCode, *configFile)
	fmt.Printf("Note: This action requires the ManageSubscriptions function to be implemented in internal/ses\n")
}

func handleSendGeneralPreferences(customerCode *string, credentialManager *aws.CredentialManager, senderEmail *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for send-general-preferences action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for send-general-preferences action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would send general preferences email from %s for customer %s\n", *senderEmail, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Sending general preferences email from %s for customer %s\n", *senderEmail, *customerCode)
	fmt.Printf("Note: This action requires the SendGeneralPreferences function to be implemented in internal/ses\n")
}

func handleSendApprovalRequest(customerCode *string, credentialManager *aws.CredentialManager, topicName *string, jsonMetadata *string, htmlTemplate *string, senderEmail *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for send-approval-request action")
	}
	if *topicName == "" {
		log.Fatal("Topic name is required for send-approval-request action")
	}
	if *jsonMetadata == "" {
		log.Fatal("JSON metadata file is required for send-approval-request action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for send-approval-request action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would send approval request to topic %s using metadata %s from %s for customer %s\n", *topicName, *jsonMetadata, *senderEmail, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Call the actual SES function
	err = ses.SendApprovalRequest(sesClient, *topicName, *jsonMetadata, *htmlTemplate, *senderEmail, dryRun)
	if err != nil {
		log.Fatalf("Failed to send approval request: %v", err)
	}
}

func handleSendChangeNotification(customerCode *string, credentialManager *aws.CredentialManager, topicName *string, jsonMetadata *string, senderEmail *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for send-change-notification action")
	}
	if *topicName == "" {
		log.Fatal("Topic name is required for send-change-notification action")
	}
	if *jsonMetadata == "" {
		log.Fatal("JSON metadata file is required for send-change-notification action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for send-change-notification action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would send change notification to topic %s using metadata %s from %s for customer %s\n", *topicName, *jsonMetadata, *senderEmail, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Call the actual SES function
	err = ses.SendChangeNotificationWithTemplate(sesClient, *topicName, *jsonMetadata, *senderEmail, dryRun)
	if err != nil {
		log.Fatalf("Failed to send change notification: %v", err)
	}
}

func handleCreateICSInvite(customerCode *string, credentialManager *aws.CredentialManager, topicName *string, jsonMetadata *string, senderEmail *string, dryRun bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for create-ics-invite action")
	}
	if *topicName == "" {
		log.Fatal("Topic name is required for create-ics-invite action")
	}
	if *jsonMetadata == "" {
		log.Fatal("JSON metadata file is required for create-ics-invite action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for create-ics-invite action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would create ICS invite for topic %s using metadata %s from %s for customer %s\n", *topicName, *jsonMetadata, *senderEmail, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Call the actual SES function
	err = ses.CreateICSInvite(sesClient, *topicName, *jsonMetadata, *senderEmail, dryRun)
	if err != nil {
		log.Fatalf("Failed to create ICS invite: %v", err)
	}
}

func handleCreateMultiCustomerMeetingInvite(credentialManager *aws.CredentialManager, jsonMetadata *string, topicName *string, senderEmail *string, dryRun bool, forceUpdate bool) {
	if *jsonMetadata == "" {
		log.Fatal("JSON metadata file is required for create-multi-customer-meeting-invite action")
	}
	if *topicName == "" {
		log.Fatal("Topic name is required for create-multi-customer-meeting-invite action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for create-multi-customer-meeting-invite action")
	}

	// Extract customer codes from metadata file
	customerCodes, err := extractCustomerCodesFromMetadata(*jsonMetadata)
	if err != nil {
		log.Fatalf("Failed to extract customer codes from metadata: %v", err)
	}

	if len(customerCodes) == 0 {
		log.Fatal("No customer codes found in metadata file")
	}

	fmt.Printf("📋 Extracted customer codes from metadata: %v\n", customerCodes)

	if dryRun {
		fmt.Printf("DRY RUN: Would create multi-customer meeting invite for topic %s using metadata %s from %s for customers: %v (force-update: %v)\n",
			*topicName, *jsonMetadata, *senderEmail, customerCodes, forceUpdate)
		return
	}

	// Call the multi-customer SES function
	meetingID, err := ses.CreateMultiCustomerMeetingInvite(credentialManager, customerCodes, *topicName, *jsonMetadata, *senderEmail, dryRun, forceUpdate)
	if err != nil {
		log.Fatalf("Failed to create multi-customer meeting invite: %v", err)
	}
	if meetingID != "" {
		fmt.Printf("📅 Meeting ID: %s\n", meetingID)
	}
}

func handleListIdentityCenterUser(mgmtRoleArn *string, identityCenterID *string, username *string, maxConcurrency int, requestsPerSecond int) {
	if *username == "" {
		log.Fatal("Username is required for list-identity-center-user action")
	}
	if *mgmtRoleArn == "" {
		log.Fatal("Management role ARN is required for list-identity-center-user action")
	}
	if *identityCenterID == "" {
		log.Fatal("Identity Center ID is required for list-identity-center-user action")
	}

	err := aws.HandleIdentityCenterUserListing(*mgmtRoleArn, *identityCenterID, *username, "single", maxConcurrency, requestsPerSecond)
	if err != nil {
		log.Fatalf("Failed to list Identity Center user: %v", err)
	}
}

func handleListIdentityCenterUserAll(mgmtRoleArn *string, identityCenterID *string, maxConcurrency int, requestsPerSecond int) {
	if *mgmtRoleArn == "" {
		log.Fatal("Management role ARN is required for list-identity-center-user-all action")
	}
	if *identityCenterID == "" {
		log.Fatal("Identity Center ID is required for list-identity-center-user-all action")
	}

	err := aws.HandleIdentityCenterUserListing(*mgmtRoleArn, *identityCenterID, "", "all", maxConcurrency, requestsPerSecond)
	if err != nil {
		log.Fatalf("Failed to list all Identity Center users: %v", err)
	}
}

func handleListGroupMembership(mgmtRoleArn *string, identityCenterID *string, username *string, maxConcurrency int, requestsPerSecond int) {
	if *username == "" {
		log.Fatal("Username is required for list-group-membership action")
	}
	if *mgmtRoleArn == "" {
		log.Fatal("Management role ARN is required for list-group-membership action")
	}
	if *identityCenterID == "" {
		log.Fatal("Identity Center ID is required for list-group-membership action")
	}

	err := aws.HandleIdentityCenterGroupMembership(*mgmtRoleArn, *identityCenterID, *username, "single", maxConcurrency, requestsPerSecond)
	if err != nil {
		log.Fatalf("Failed to list group membership: %v", err)
	}
}

func handleListGroupMembershipAll(mgmtRoleArn *string, identityCenterID *string, maxConcurrency int, requestsPerSecond int) {
	if *mgmtRoleArn == "" {
		log.Fatal("Management role ARN is required for list-group-membership-all action")
	}
	if *identityCenterID == "" {
		log.Fatal("Identity Center ID is required for list-group-membership-all action")
	}

	err := aws.HandleIdentityCenterGroupMembership(*mgmtRoleArn, *identityCenterID, "", "all", maxConcurrency, requestsPerSecond)
	if err != nil {
		log.Fatalf("Failed to list all group memberships: %v", err)
	}
}

func handleImportAWSContact(customerCode *string, credentialManager *aws.CredentialManager, mgmtRoleArn *string, identityCenterID *string, username *string, maxConcurrency int, requestsPerSecond int, dryRun bool, configFile *string) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for import-aws-contact action")
	}
	if *username == "" {
		log.Fatal("Username is required for import-aws-contact action")
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Call the actual import function
	err = ses.ImportSingleAWSContact(sesClient, *identityCenterID, *username, dryRun)
	if err != nil {
		log.Fatalf("Failed to import AWS contact: %v", err)
	}

	fmt.Printf("✅ Successfully imported AWS contact: %s\n", *username)
}

func handleImportAWSContactAll(cfg *types.Config, customerCode *string, identityCenterRoleArn *string, maxConcurrency int, requestsPerSecond int, dryRun bool) {
	// If a specific customer code is provided, process only that customer
	// Otherwise, process all customers concurrently
	if customerCode != nil && *customerCode != "" {
		// Single customer mode - validate customer exists
		if _, exists := cfg.CustomerMappings[*customerCode]; !exists {
			log.Fatalf("Customer code %s not found in configuration", *customerCode)
		}

		// Create a temporary config with only this customer
		singleCustomerConfig := &types.Config{
			AWSRegion: cfg.AWSRegion,
			CustomerMappings: map[string]types.CustomerAccountInfo{
				*customerCode: cfg.CustomerMappings[*customerCode],
			},
			ContactConfig: cfg.ContactConfig,
			EmailConfig:   cfg.EmailConfig,
			LogLevel:      cfg.LogLevel,
		}

		// Call enhanced handler with single customer
		err := handleImportAWSContactAllEnhanced(singleCustomerConfig, identityCenterRoleArn, maxConcurrency, requestsPerSecond, dryRun)
		if err != nil {
			log.Fatalf("Failed to import AWS contacts: %v", err)
		}
	} else {
		// Multi-customer mode - process all customers concurrently
		err := handleImportAWSContactAllEnhanced(cfg, identityCenterRoleArn, maxConcurrency, requestsPerSecond, dryRun)
		if err != nil {
			log.Fatalf("Failed to import AWS contacts: %v", err)
		}
	}

	fmt.Printf("✅ Successfully completed bulk import of AWS contacts\n")
}

// processCustomer processes a single customer's Identity Center data retrieval and SES import
// This function is designed to be called concurrently for multiple customers
func processCustomer(
	cfg *types.Config,
	customerCode string,
	identityCenterRoleArn *string,
	maxConcurrency int,
	requestsPerSecond int,
	dryRun bool,
) CustomerImportResult {
	result := CustomerImportResult{
		CustomerCode: customerCode,
		Success:      false,
	}

	customerInfo, exists := cfg.CustomerMappings[customerCode]
	if !exists {
		result.Error = fmt.Errorf("customer code %s not found in configuration", customerCode)
		log.Printf("❌ Customer %s: Not found in configuration", customerCode)
		return result
	}

	log.Printf("🔄 Customer %s: Starting processing", customerCode)

	// Determine Identity Center role ARN (CLI flag takes precedence over config)
	icRoleArn := ""
	dataSource := "file-based"
	if identityCenterRoleArn != nil && *identityCenterRoleArn != "" {
		icRoleArn = *identityCenterRoleArn
		dataSource = "in-memory"
		log.Printf("🔐 Customer %s: Using Identity Center role from CLI flag: %s", customerCode, icRoleArn)
	} else if customerInfo.IdentityCenterRoleArn != "" {
		icRoleArn = customerInfo.IdentityCenterRoleArn
		dataSource = "in-memory"
		log.Printf("🔐 Customer %s: Using Identity Center role from config: %s", customerCode, icRoleArn)
	}

	var icData *aws.IdentityCenterData
	var identityCenterID string

	// Retrieve Identity Center data if role ARN is configured
	if icRoleArn != "" {
		// Validate the Identity Center role ARN format
		if err := config.ValidateIdentityCenterRoleArn(icRoleArn); err != nil {
			result.Error = fmt.Errorf("invalid Identity Center role ARN: %w", err)
			log.Printf("❌ Customer %s: Invalid Identity Center role ARN: %v", customerCode, err)
			return result
		}

		log.Printf("📊 Customer %s: Retrieving Identity Center data via role assumption (data source: %s)", customerCode, dataSource)

		var err error
		icData, err = aws.RetrieveIdentityCenterData(icRoleArn, maxConcurrency, requestsPerSecond)
		if err != nil {
			// Provide clear error message for permission issues
			if strings.Contains(err.Error(), "AccessDenied") || strings.Contains(err.Error(), "not authorized") {
				result.Error = fmt.Errorf("failed to assume Identity Center role (permission denied): %w\n"+
					"Please ensure:\n"+
					"  1. The role exists in the target account\n"+
					"  2. The role's trust policy allows your current credentials to assume it\n"+
					"  3. Your current credentials have sts:AssumeRole permission\n"+
					"  4. The role has permissions to access Identity Center (identitystore:* and sso:ListInstances)", err)
			} else {
				result.Error = fmt.Errorf("failed to retrieve Identity Center data: %w", err)
			}
			log.Printf("❌ Customer %s: Failed to retrieve Identity Center data: %v", customerCode, result.Error)
			return result
		}

		identityCenterID = icData.InstanceID
		log.Printf("✅ Customer %s: Retrieved %d users and %d group memberships from Identity Center (instance: %s, data source: %s)",
			customerCode, len(icData.Users), len(icData.Memberships), identityCenterID, dataSource)

		result.UsersProcessed = len(icData.Users)
	} else {
		log.Printf("📁 Customer %s: No Identity Center role configured, will use file-based data (data source: %s)", customerCode, dataSource)
		// identityCenterID will be auto-detected from files by ImportAllAWSContacts
	}

	// Assume SES role for customer
	log.Printf("🔐 Customer %s: Assuming SES role: %s", customerCode, customerInfo.SESRoleARN)

	sesConfig, err := assumeSESRole(customerInfo.SESRoleARN, customerCode, customerInfo.Region)
	if err != nil {
		result.Error = fmt.Errorf("failed to assume SES role: %w", err)
		log.Printf("❌ Customer %s: Failed to assume SES role: %v", customerCode, err)
		return result
	}

	sesClient := sesv2.NewFromConfig(sesConfig)

	// Import contacts
	log.Printf("📥 Customer %s: Importing contacts to SES (data source: %s)", customerCode, dataSource)

	err = ses.ImportAllAWSContacts(sesClient, identityCenterID, icData, dryRun, requestsPerSecond)
	if err != nil {
		result.Error = fmt.Errorf("failed to import contacts: %w", err)
		log.Printf("❌ Customer %s: Failed to import contacts: %v", customerCode, err)
		return result
	}

	result.Success = true
	log.Printf("✅ Customer %s: Successfully imported contacts (data source: %s)", customerCode, dataSource)

	return result
}

// handleImportAWSContactAllEnhanced processes multiple customers concurrently with in-memory Identity Center data
func handleImportAWSContactAllEnhanced(
	cfg *types.Config,
	identityCenterRoleArn *string,
	maxConcurrency int,
	requestsPerSecond int,
	dryRun bool,
) error {
	// Get list of customers to process
	var customersToProcess []string
	for customerCode := range cfg.CustomerMappings {
		customersToProcess = append(customersToProcess, customerCode)
	}

	if len(customersToProcess) == 0 {
		return fmt.Errorf("no customers found in configuration")
	}

	// Determine data source mode
	dataSourceMode := "file-based"
	if identityCenterRoleArn != nil && *identityCenterRoleArn != "" {
		dataSourceMode = "in-memory (CLI override)"
	} else {
		// Check if any customer has Identity Center role configured
		for _, customerCode := range customersToProcess {
			if cfg.CustomerMappings[customerCode].IdentityCenterRoleArn != "" {
				dataSourceMode = "in-memory (from config)"
				break
			}
		}
	}

	log.Printf("🚀 Starting concurrent customer processing")
	log.Printf("📊 Total customers: %d", len(customersToProcess))
	log.Printf("⚙️  Max concurrency: %d", maxConcurrency)
	log.Printf("⚙️  Requests per second: %d", requestsPerSecond)
	log.Printf("📂 Data source mode: %s", dataSourceMode)
	log.Printf("🔧 Dry run: %v", dryRun)
	if identityCenterRoleArn != nil && *identityCenterRoleArn != "" {
		log.Printf("🔐 Identity Center role (CLI override): %s", *identityCenterRoleArn)
	}
	fmt.Println()

	// Create worker pool with semaphore for concurrency control
	semaphore := make(chan struct{}, maxConcurrency)
	results := make(chan CustomerImportResult, len(customersToProcess))
	var wg sync.WaitGroup

	// Launch goroutines for each customer
	for _, custCode := range customersToProcess {
		wg.Add(1)
		go func(customerCode string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process customer
			result := processCustomer(cfg, customerCode, identityCenterRoleArn, maxConcurrency, requestsPerSecond, dryRun)
			results <- result
		}(custCode)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results from all customers
	var allResults []CustomerImportResult
	for result := range results {
		allResults = append(allResults, result)
	}

	// Aggregate and report results
	return aggregateAndReportResults(allResults)
}

// aggregateAndReportResults aggregates results from all customer imports and reports summary
func aggregateAndReportResults(results []CustomerImportResult) error {
	successCount := 0
	failureCount := 0
	skippedCount := 0
	totalUsersProcessed := 0
	totalContactsAdded := 0
	totalContactsUpdated := 0
	totalContactsSkipped := 0
	var successfulCustomers []string
	var failedCustomers []string
	var errorMessages []string

	// Aggregate counts
	for _, result := range results {
		if result.Success {
			successCount++
			successfulCustomers = append(successfulCustomers, result.CustomerCode)
			totalUsersProcessed += result.UsersProcessed
			totalContactsAdded += result.ContactsAdded
			totalContactsUpdated += result.ContactsUpdated
			totalContactsSkipped += result.ContactsSkipped
		} else {
			failureCount++
			failedCustomers = append(failedCustomers, result.CustomerCode)
			if result.Error != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", result.CustomerCode, result.Error))
			}
		}
	}

	// Print summary
	fmt.Println()
	log.Printf("=" + strings.Repeat("=", 70))
	log.Printf("📊 IMPORT SUMMARY")
	log.Printf("=" + strings.Repeat("=", 70))
	log.Printf("Total customers processed: %d", len(results))
	log.Printf("✅ Successful: %d", successCount)
	log.Printf("❌ Failed: %d", failureCount)
	log.Printf("⏭️  Skipped: %d", skippedCount)
	log.Printf("")
	log.Printf("📈 Statistics:")
	log.Printf("   Users processed: %d", totalUsersProcessed)
	log.Printf("   Contacts added: %d", totalContactsAdded)
	log.Printf("   Contacts updated: %d", totalContactsUpdated)
	log.Printf("   Contacts skipped: %d", totalContactsSkipped)

	// Report successful customers
	if successCount > 0 {
		log.Printf("")
		log.Printf("✅ Successful customers:")
		for _, customerCode := range successfulCustomers {
			log.Printf("   - %s", customerCode)
		}
	}

	// Report failures if any
	if failureCount > 0 {
		log.Printf("")
		log.Printf("❌ Failed customers:")
		for _, customerCode := range failedCustomers {
			log.Printf("   - %s", customerCode)
		}

		log.Printf("")
		log.Printf("🔍 Detailed errors:")
		for _, errMsg := range errorMessages {
			log.Printf("   %s", errMsg)
		}
	}

	log.Printf("=" + strings.Repeat("=", 70))

	// Exit with error if any customer failed
	if failureCount > 0 {
		return fmt.Errorf("%d customer(s) failed to import", failureCount)
	}

	return nil
}
