package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Version information
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Check if running in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		// Running in Lambda - start Lambda handler immediately
		runLambdaMode()
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
	case "help", "-h", "--help":
		showUsage()
	default:
		// Fallback to old mode-based system for backward compatibility
		handleLegacyModeCommand()
	}
}

func handleLegacyModeCommand() {
	// Command line flags for legacy mode
	var (
		mode       = flag.String("mode", "update", "Operation mode: update, sqs, validate, version, help")
		configFile = flag.String("config", "", "Configuration file path")
		customer   = flag.String("customer", "", "Customer code (for update mode)")
		sqsQueue   = flag.String("sqs-queue", "", "SQS queue URL (for sqs mode)")
		logLevel   = flag.String("log-level", "info", "Log level: debug, info, warn, error")
		dryRun     = flag.Bool("dry-run", false, "Dry run mode (don't make actual changes)")
	)
	flag.Parse()

	// Setup logging
	SetupLogging(*logLevel)

	// Handle special modes
	switch *mode {
	case "version":
		showVersion()
		return
	case "help":
		showHelp()
		return
	}

	// Load configuration
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize credential manager
	credentialManager, err := NewCredentialManager(config.AWSRegion, config.CustomerMappings)
	if err != nil {
		log.Fatalf("Failed to initialize credential manager: %v", err)
	}

	// Initialize email manager
	emailManager := NewEmailManager(credentialManager, config.ContactConfig)

	// Run based on mode
	switch *mode {
	case "update":
		err = runUpdateMode(*customer, *dryRun, credentialManager, emailManager, config)
	case "sqs":
		err = runSQSMode(*sqsQueue, credentialManager, emailManager, config)
	case "lambda":
		// Manual lambda mode for testing
		runLambdaMode()
		return
	case "validate":
		err = runValidateMode(*customer, credentialManager, emailManager)
	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}

	if err != nil {
		log.Fatalf("Operation failed: %v", err)
	}
}

// runUpdateMode updates alternate contacts for a customer
func runUpdateMode(customerCode string, dryRun bool, credentialManager *CredentialManager, emailManager *EmailManager, config *Config) error {
	if customerCode == "" {
		log.Println("Available customers:")
		for code, info := range config.CustomerMappings {
			log.Printf("  %s: %s (%s)", code, info.CustomerName, info.GetAccountID())
		}
		return fmt.Errorf("customer code is required (use -customer flag)")
	}

	// Validate customer code
	if err := ValidateCustomerCode(customerCode); err != nil {
		return fmt.Errorf("invalid customer code: %v", err)
	}

	log.Printf("Updating alternate contacts for customer: %s", customerCode)

	if dryRun {
		log.Println("DRY RUN MODE - No actual changes will be made")

		// Validate access
		if err := credentialManager.ValidateCustomerAccess(customerCode); err != nil {
			return fmt.Errorf("access validation failed: %v", err)
		}

		// Validate email configuration
		if err := emailManager.ValidateEmailConfiguration(customerCode); err != nil {
			return fmt.Errorf("email validation failed: %v", err)
		}

		log.Println("Validation successful - would update contacts and send notification")
		return nil
	}

	// Update alternate contacts
	if err := UpdateAlternateContacts(customerCode, credentialManager, config.ContactConfig); err != nil {
		return fmt.Errorf("failed to update contacts: %v", err)
	}

	// Send notification email
	changeDetails := map[string]interface{}{
		"security_updated":   true,
		"billing_updated":    true,
		"operations_updated": true,
		"timestamp":          time.Now(),
		"source":             "cli",
	}

	if err := emailManager.SendAlternateContactNotification(customerCode, changeDetails); err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}

	log.Printf("Successfully updated alternate contacts for customer %s", customerCode)
	return nil
}

// runSQSMode processes messages from SQS queue
func runSQSMode(queueURL string, credentialManager *CredentialManager, emailManager *EmailManager, config *Config) error {
	if queueURL == "" {
		return fmt.Errorf("SQS queue URL is required (use -sqs-queue flag)")
	}

	log.Printf("Starting SQS processing from queue: %s", queueURL)

	// Initialize SQS processor
	sqsProcessor, err := NewSQSProcessor(queueURL, credentialManager, emailManager, config.AWSRegion)
	if err != nil {
		return fmt.Errorf("failed to initialize SQS processor: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping SQS processing...")
		cancel()
	}()

	// Start processing
	return sqsProcessor.ProcessMessages(ctx)
}

// runValidateMode validates configuration and access
func runValidateMode(customerCode string, credentialManager *CredentialManager, emailManager *EmailManager) error {
	log.Println("Validating configuration and access...")

	customers := credentialManager.ListCustomers()
	if customerCode != "" {
		customers = []string{customerCode}
	}

	for _, code := range customers {
		log.Printf("Validating customer: %s", code)

		// Validate customer access
		if err := credentialManager.ValidateCustomerAccess(code); err != nil {
			log.Printf("❌ Access validation failed for %s: %v", code, err)
			continue
		}

		// Validate email configuration
		if err := emailManager.ValidateEmailConfiguration(code); err != nil {
			log.Printf("❌ Email validation failed for %s: %v", code, err)
			continue
		}

		// Get current contacts
		contacts, err := GetAlternateContacts(code, credentialManager)
		if err != nil {
			log.Printf("❌ Failed to get contacts for %s: %v", code, err)
			continue
		}

		log.Printf("✅ Customer %s validation successful", code)
		log.Printf("   Current contacts: %d configured", len(contacts))
	}

	log.Println("Validation complete")
	return nil
}

// showVersion displays version information
func showVersion() {
	fmt.Printf("AWS Alternate Contact Manager\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Build Time: %s\n", BuildTime)
	fmt.Printf("Git Commit: %s\n", GitCommit)
}

// showHelp displays legacy help information
func showHelp() {
	fmt.Printf("AWS Alternate Contact Manager (Legacy Mode)\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s [OPTIONS]\n\n", os.Args[0])
	fmt.Printf("MODES:\n")
	fmt.Printf("  update      Update alternate contacts for a customer\n")
	fmt.Printf("  sqs         Process messages from SQS queue\n")
	fmt.Printf("  validate    Validate configuration and access\n")
	fmt.Printf("  version     Show version information\n")
	fmt.Printf("  help        Show this help message\n\n")
	fmt.Printf("OPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("  # Update contacts for a specific customer\n")
	fmt.Printf("  %s -mode=update -customer=hts\n\n", os.Args[0])
	fmt.Printf("  # Dry run to test configuration\n")
	fmt.Printf("  %s -mode=update -customer=hts -dry-run\n\n", os.Args[0])
	fmt.Printf("  # Process SQS messages\n")
	fmt.Printf("  %s -mode=sqs -sqs-queue=https://sqs.us-east-1.amazonaws.com/123456789012/contact-updates\n\n", os.Args[0])
	fmt.Printf("  # Validate all customers\n")
	fmt.Printf("  %s -mode=validate\n\n", os.Args[0])
}

func showUsage() {
	fmt.Println("AWS Alternate Contact Manager")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  aws-alternate-contact-manager <command> [options]")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  alt-contact           Manage AWS alternate contacts")
	fmt.Println("  ses                   Manage SES contact lists and emails")
	fmt.Println("  validate-customers    Validate customer codes")
	fmt.Println("  extract-customers     Extract customers from metadata")
	fmt.Println("  configure-s3-events   Configure S3 event notifications")
	fmt.Println("  test-s3-events        Test S3 event delivery")
	fmt.Println("  validate-s3-events    Validate S3 event configuration")
	fmt.Println("  version               Show version information")
	fmt.Println("  help                  Show this help message")
	fmt.Println()
	fmt.Println("Use 'aws-alternate-contact-manager <command> --help' for command-specific help.")
}

func handleAltContactCommand() {
	fs := flag.NewFlagSet("alt-contact", flag.ExitOnError)

	action := fs.String("action", "", "Action to perform: set-all, set-one, delete")
	configFile := fs.String("config", "", "Configuration file path")
	contactConfigFile := fs.String("contact-config-file", "ContactConfig.json", "Contact configuration file")
	orgPrefix := fs.String("org-prefix", "", "Organization prefix (required for set-one and delete)")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing contacts")
	contactTypes := fs.String("contact-types", "", "Comma-separated contact types for delete action")
	logLevel := fs.String("log-level", "info", "Log level")

	fs.Parse(os.Args[2:])

	if *action == "" {
		fmt.Println("Error: -action is required")
		fmt.Println()
		fmt.Println("Available actions: set-all, set-one, delete")
		os.Exit(1)
	}

	SetupLogging(*logLevel)

	// Load configuration
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Handle alt-contact actions
	switch *action {
	case "set-all":
		err = handleSetAllContacts(config, *contactConfigFile, *overwrite)
	case "set-one":
		if *orgPrefix == "" {
			log.Fatal("Error: -org-prefix is required for set-one action")
		}
		err = handleSetOneContact(config, *contactConfigFile, *orgPrefix, *overwrite)
	case "delete":
		if *orgPrefix == "" || *contactTypes == "" {
			log.Fatal("Error: -org-prefix and -contact-types are required for delete action")
		}
		err = handleDeleteContacts(*orgPrefix, *contactTypes)
	default:
		log.Fatalf("Unknown action: %s", *action)
	}

	if err != nil {
		log.Fatalf("Operation failed: %v", err)
	}
}

func handleSESCommand() {
	fs := flag.NewFlagSet("ses", flag.ExitOnError)

	action := fs.String("action", "", "SES action to perform")
	configFile := fs.String("config-file", "", "Configuration file path")
	backupFile := fs.String("backup-file", "", "Backup file for restore operations")
	email := fs.String("email", "", "Email address")
	topics := fs.String("topics", "", "Comma-separated topics")
	suppressionReason := fs.String("suppression-reason", "bounce", "Suppression reason: bounce or complaint")
	topicName := fs.String("topic-name", "", "Topic name")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	customerCode := fs.String("customer-code", "", "Customer code")
	logLevel := fs.String("log-level", "info", "Log level")

	// Identity Center parameters
	mgmtRoleArn := fs.String("mgmt-role-arn", "", "Management account IAM role ARN for Identity Center operations")
	identityCenterId := fs.String("identity-center-id", "", "Identity Center instance ID (format: d-xxxxxxxxxx)")
	username := fs.String("username", "", "Username to search for in Identity Center")
	maxConcurrency := fs.Int("max-concurrency", 10, "Maximum concurrent workers for Identity Center operations")
	requestsPerSecond := fs.Int("requests-per-second", 10, "API requests per second rate limit")

	// Email sending parameters
	senderEmail := fs.String("sender-email", "", "Sender email address for test emails")

	fs.Parse(os.Args[2:])

	if *action == "" || *action == "help" {
		showSESHelp()
		return
	}

	SetupLogging(*logLevel)

	// Handle SES actions
	err := handleSESAction(*action, *configFile, *backupFile, *email, *topics,
		*suppressionReason, *topicName, *dryRun, *customerCode, *mgmtRoleArn,
		*identityCenterId, *username, *maxConcurrency, *requestsPerSecond, *senderEmail)

	if err != nil {
		log.Fatalf("SES operation failed: %v", err)
	}
}

func showSESHelp() {
	fmt.Println("AWS SES Contact List Management - Available Actions")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Println()
	fmt.Println("📋 CONTACT LIST MANAGEMENT:")
	fmt.Println("  create-list          Create a new contact list")
	fmt.Println("                       • From config: -config-file SESConfig.json")
	fmt.Println("                       • From backup: -backup-file backup.json")
	fmt.Println()
	fmt.Println("  describe-list        Show contact list details and topics")
	fmt.Println("  delete-list          Delete contact list (creates backup first)")
	fmt.Println("                       • Use --dry-run to preview deletion")
	fmt.Println("  describe-account     Show account's main contact list details")
	fmt.Println()
	fmt.Println("👥 CONTACT MANAGEMENT:")
	fmt.Println("  add-contact          Add email to contact list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println("                       • Optional: -topics topic1,topic2")
	fmt.Println()
	fmt.Println("  remove-contact       Remove specific email from list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println()
	fmt.Println("  remove-contact-all   Remove ALL contacts from list (creates backup)")
	fmt.Println("                       • ⚠️  Creates automatic backup before removal")
	fmt.Println("                       • 📁 Backup: ses-backup-{list}-{timestamp}.json")
	fmt.Println()
	fmt.Println("  list-contacts        List all contacts in the contact list")
	fmt.Println()
	fmt.Println("🔍 CONTACT INFORMATION:")
	fmt.Println("  describe-contact     Show contact details and subscriptions")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println()
	fmt.Println("📧 SUPPRESSION LIST:")
	fmt.Println("  suppress             Add email to suppression list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println("                       • Optional: -suppression-reason bounce|complaint")
	fmt.Println()
	fmt.Println("  unsuppress           Remove email from suppression list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println()
	fmt.Println("🏷️  TOPIC MANAGEMENT:")
	fmt.Println("  describe-topic       Show specific topic details")
	fmt.Println("                       • Required: -topic-name topic-name")
	fmt.Println()
	fmt.Println("  describe-topic-all   Show all topics and subscription stats")
	fmt.Println()
	fmt.Println("  send-topic-test      Send test email to specific topic subscribers")
	fmt.Println("                       • Required: -topic-name topic-name")
	fmt.Println("                       • Required: -sender-email verified@domain.com")
	fmt.Println("                       • Sends test email to all subscribed contacts")
	fmt.Println()
	fmt.Println("  manage-topic         Update contact list topics (creates backup)")
	fmt.Println("                       • Uses: -config-file SESConfig.json")
	fmt.Println("                       • Optional: -dry-run (preview changes)")
	fmt.Println()
	fmt.Println("👥 IDENTITY CENTER INTEGRATION:")
	fmt.Println("  list-identity-center-user     List specific user from Identity Center")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Required: -username john.doe")
	fmt.Println("                                • Outputs: JSON file with user data")
	fmt.Println()
	fmt.Println("  list-identity-center-user-all List ALL users from Identity Center")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Optional: -max-concurrency 10 (workers)")
	fmt.Println("                                • Optional: -requests-per-second 10 (rate limit)")
	fmt.Println("                                • Outputs: JSON file with all users data")
	fmt.Println()
	fmt.Println("  list-group-membership         List group memberships for specific user")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Required: -username john.doe")
	fmt.Println("                                • Outputs: JSON file with user's group memberships")
	fmt.Println()
	fmt.Println("  list-group-membership-all     List group memberships for ALL users")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Optional: -max-concurrency 10 (workers)")
	fmt.Println("                                • Optional: -requests-per-second 10 (rate limit)")
	fmt.Println("                                • Outputs: Three JSON files (user-centric, group-centric, and CCOE cloud groups)")
	fmt.Println()
	fmt.Println("📥 AWS CONTACT IMPORT:")
	fmt.Println("  import-aws-contact            Import specific user to SES based on group memberships")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Required: -username john.doe")
	fmt.Println("                                • Optional: -mgmt-role-arn (if data files don't exist)")
	fmt.Println("                                • Optional: -dry-run (preview import)")
	fmt.Println()
	fmt.Println("  import-aws-contact-all        Import ALL users to SES based on group memberships")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Optional: -mgmt-role-arn (if data files don't exist)")
	fmt.Println("                                • Optional: -dry-run (preview import)")
	fmt.Println("                                • Optional: -max-concurrency 10 (for data generation)")
	fmt.Println("                                • Optional: -requests-per-second 10 (for data generation)")
	fmt.Println()
	fmt.Println("📧 EMAIL TEMPLATES & NOTIFICATIONS:")
	fmt.Println("  send-approval-request         Send approval request emails using templates")
	fmt.Println("                                • Required: -topic-name approval-topic")
	fmt.Println("                                • Required: -sender-email verified@domain.com")
	fmt.Println("                                • Required: -config-file metadata.json (JSON metadata)")
	fmt.Println("                                • Optional: -backup-file template.html (HTML template)")
	fmt.Println("                                • Optional: -dry-run (preview email)")
	fmt.Println()
	fmt.Println("  send-change-notification      Send change notification/announcement emails")
	fmt.Println("                                • Required: -topic-name announce-topic")
	fmt.Println("                                • Required: -sender-email verified@domain.com")
	fmt.Println("                                • Required: -config-file metadata.json (JSON metadata)")
	fmt.Println("                                • Optional: -dry-run (preview email)")
	fmt.Println()
	fmt.Println("📅 CALENDAR & MEETING INVITES:")
	fmt.Println("  create-meeting-invite         Create Microsoft Graph/Teams meeting invites")
	fmt.Println("                                • Required: -topic-name calendar-topic")
	fmt.Println("                                • Required: -sender-email verified@domain.com")
	fmt.Println("                                • Required: -config-file metadata.json (JSON metadata)")
	fmt.Println("                                • Optional: -dry-run (preview meeting)")
	fmt.Println("                                • Requires: Azure app registration with Graph API permissions")
	fmt.Println()
	fmt.Println("  create-ics-invite             Create ICS calendar file invites")
	fmt.Println("                                • Required: -topic-name calendar-topic")
	fmt.Println("                                • Required: -sender-email verified@domain.com")
	fmt.Println("                                • Required: -config-file metadata.json (JSON metadata)")
	fmt.Println("                                • Optional: -dry-run (preview calendar invite)")
	fmt.Println("                                • Sends ICS attachments via email")
}

// Placeholder implementations - these need to be implemented based on the original functionality
func handleSetAllContacts(config *Config, contactConfigFile string, overwrite bool) error {
	return fmt.Errorf("set-all contacts not yet implemented")
}

func handleSetOneContact(config *Config, contactConfigFile string, orgPrefix string, overwrite bool) error {
	return fmt.Errorf("set-one contact not yet implemented")
}

func handleDeleteContacts(orgPrefix string, contactTypes string) error {
	return fmt.Errorf("delete contacts not yet implemented")
}

func handleSESAction(action, configFile, backupFile, email, topics, suppressionReason, topicName string, dryRun bool, customerCode, mgmtRoleArn, identityCenterId, username string, maxConcurrency, requestsPerSecond int, senderEmail string) error {
	// Parse topics string into slice
	var topicSlice []string
	if topics != "" {
		topicSlice = strings.Split(topics, ",")
		// Trim whitespace from each topic
		for i, topic := range topicSlice {
			topicSlice[i] = strings.TrimSpace(topic)
		}
	}

	// Load customer-specific SES role if customer code is provided
	var sesRoleArn string
	if customerCode != "" {
		// Always load the main config file (config.json) to get customer mappings
		// The configFile parameter is for SES-specific config, not the main config
		config, err := LoadConfig("config.json")
		if err == nil {
			if customerInfo, exists := config.CustomerMappings[customerCode]; exists {
				sesRoleArn = customerInfo.SESRoleARN
			}
		}
	}

	return ManageSESLists(action, configFile, backupFile, email, topicSlice, suppressionReason, topicName, dryRun, sesRoleArn, mgmtRoleArn, identityCenterId, username, maxConcurrency, requestsPerSecond, senderEmail)
}

func handleValidateCustomersCommand() {
	fmt.Println("validate-customers command not yet implemented")
}

func handleExtractCustomersCommand() {
	fmt.Println("extract-customers command not yet implemented")
}

func handleConfigureS3EventsCommand() {
	fmt.Println("configure-s3-events command not yet implemented")
}

func handleTestS3EventsCommand() {
	fmt.Println("test-s3-events command not yet implemented")
}

func handleValidateS3EventsCommand() {
	fmt.Println("validate-s3-events command not yet implemented")
}
