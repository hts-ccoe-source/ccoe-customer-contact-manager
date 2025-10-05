package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"aws-alternate-contact-manager/internal/aws"
	"aws-alternate-contact-manager/internal/config"
	"aws-alternate-contact-manager/internal/contacts"
	"aws-alternate-contact-manager/internal/lambda"
	"aws-alternate-contact-manager/internal/ses"
	"aws-alternate-contact-manager/internal/types"
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
	fmt.Printf("AWS Alternate Contact Manager\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("  aws-alternate-contact-manager <command> [options]\n\n")
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
	fmt.Printf("Use 'aws-alternate-contact-manager <command> --help' for command-specific help.\n")
}

func showVersion() {
	fmt.Printf("AWS Alternate Contact Manager\n")
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

	// Add unused flags for compatibility
	fs.String("config", "", "Configuration file path")
	fs.String("log-level", "info", "Log level")

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

func handleSESCommand() {
	fs := flag.NewFlagSet("ses", flag.ExitOnError)

	action := fs.String("action", "", "SES action to perform")
	configFile := fs.String("config-file", "", "Configuration file path")
	customerCode := fs.String("customer-code", "", "Customer code")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	email := fs.String("email", "", "Email address")
	identityCenterID := fs.String("identity-center-id", "", "Identity Center instance ID (format: d-xxxxxxxxxx)")
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

	// Load configuration if provided
	var cfg *types.Config
	var err error
	if *configFile != "" {
		cfg, err = config.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
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
	case "create-contact-list":
		handleCreateContactList(customerCode, credentialManager, *dryRun)
	case "list-contact-lists":
		handleListContactLists(customerCode, credentialManager)
	case "describe-contact-list":
		handleDescribeContactList(customerCode, credentialManager)
	case "add-contact":
		handleAddContact(customerCode, credentialManager, email, topics, *dryRun)
	case "remove-contact":
		handleRemoveContact(customerCode, credentialManager, email, *dryRun)
	case "list-contacts":
		handleListContacts(customerCode, credentialManager)
	case "describe-contact":
		handleDescribeContact(customerCode, credentialManager, email)
	case "add-contact-topics":
		handleAddContactTopics(customerCode, credentialManager, email, topics, *dryRun)
	case "remove-contact-topics":
		handleRemoveContactTopics(customerCode, credentialManager, email, topics, *dryRun)
	case "describe-topic":
		handleDescribeTopic(customerCode, credentialManager, topicName)
	case "add-to-suppression":
		handleAddToSuppression(customerCode, credentialManager, email, suppressionReason, *dryRun)
	case "remove-from-suppression":
		handleRemoveFromSuppression(customerCode, credentialManager, email, *dryRun)
	case "backup-contact-list":
		handleBackupContactList(customerCode, credentialManager, *action)
	case "remove-all-contacts":
		handleRemoveAllContacts(customerCode, credentialManager, *dryRun)
	case "send-test-email":
		handleSendTestEmail(emailManager, customerCode, senderEmail, email, *dryRun)
	case "validate-customer":
		handleValidateCustomer(credentialManager, customerCode)
	case "describe-topic-all":
		handleDescribeTopicAll(customerCode, credentialManager)
	case "send-topic-test":
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
	case "create-meeting-invite":
		handleCreateMeetingInvite(customerCode, credentialManager, topicName, jsonMetadata, senderEmail, *dryRun, *forceUpdate)
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
		handleImportAWSContactAll(customerCode, credentialManager, mgmtRoleArn, identityCenterID, *maxConcurrency, *requestsPerSecond, *dryRun, configFile)
	default:
		fmt.Printf("Unknown SES action: %s\n", *action)
		showSESUsage()
		os.Exit(1)
	}

	// Suppress unused variable warnings for now
	_ = backupFile
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
	fmt.Printf("  describe-contact-list   Show detailed contact list information\n")
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
	fmt.Printf("  create-meeting-invite   Create meeting via Microsoft Graph API\n\n")
	fmt.Printf("👥 IDENTITY CENTER INTEGRATION:\n")
	fmt.Printf("  list-identity-center-user     List specific user from Identity Center\n")
	fmt.Printf("  list-identity-center-user-all List ALL users from Identity Center\n")
	fmt.Printf("  list-group-membership         List group memberships for specific user\n")
	fmt.Printf("  list-group-membership-all     List group memberships for ALL users\n\n")
	fmt.Printf("📥 AWS CONTACT IMPORT:\n")
	fmt.Printf("  import-aws-contact            Import specific user to SES based on group memberships\n")
	fmt.Printf("  import-aws-contact-all        Import ALL users to SES based on group memberships\n\n")
	fmt.Printf("🔧 UTILITIES:\n")
	fmt.Printf("  validate-customer       Validate customer access\n")
	fmt.Printf("  help                    Show this help message\n\n")
	fmt.Printf("COMMON FLAGS:\n")
	fmt.Printf("  --customer-code string  Customer code (required for most actions)\n")
	fmt.Printf("  --config-file string    Configuration file path\n")
	fmt.Printf("  --email string          Email address\n")
	fmt.Printf("  --topics string         Comma-separated topic names\n")
	fmt.Printf("  --topic-name string     Single topic name\n")
	fmt.Printf("  --sender-email string   Sender email address for test emails\n")
	fmt.Printf("  --json-metadata string  Path to JSON metadata file\n")
	fmt.Printf("  --html-template string  Path to HTML email template file\n")
	fmt.Printf("  --mgmt-role-arn string  Management account IAM role ARN for Identity Center\n")
	fmt.Printf("  --identity-center-id    Identity Center instance ID (d-xxxxxxxxxx)\n")
	fmt.Printf("  --username string       Username to search in Identity Center\n")
	fmt.Printf("  --max-concurrency int   Max concurrent workers (default: 10)\n")
	fmt.Printf("  --requests-per-second   API requests per second rate limit (default: 10)\n")
	fmt.Printf("  --force-update          Force update existing meetings\n")
	fmt.Printf("  --dry-run               Show what would be done without making changes\n")
	fmt.Printf("  --log-level string      Log level (default: info)\n")
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
		log.Fatal("Customer code is required for describe-contact-list action")
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

	if dryRun {
		fmt.Printf("DRY RUN: Would update topics for customer %s using config %s\n", *customerCode, *configFile)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Updating topics for customer %s using config %s\n", *customerCode, *configFile)
	fmt.Printf("Note: This action requires the ManageTopics function to be implemented in internal/ses\n")
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
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Sending approval request to topic %s using metadata %s from %s\n", *topicName, *jsonMetadata, *senderEmail)
	fmt.Printf("Note: This action requires the SendApprovalRequest function to be implemented in internal/ses\n")
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
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Sending change notification to topic %s using metadata %s from %s\n", *topicName, *jsonMetadata, *senderEmail)
	fmt.Printf("Note: This action requires the SendChangeNotification function to be implemented in internal/ses\n")
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
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Creating ICS invite for topic %s using metadata %s from %s\n", *topicName, *jsonMetadata, *senderEmail)
	fmt.Printf("Note: This action requires the CreateICSInvite function to be implemented in internal/ses\n")
}

func handleCreateMeetingInvite(customerCode *string, credentialManager *aws.CredentialManager, topicName *string, jsonMetadata *string, senderEmail *string, dryRun bool, forceUpdate bool) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for create-meeting-invite action")
	}
	if *topicName == "" {
		log.Fatal("Topic name is required for create-meeting-invite action")
	}
	if *jsonMetadata == "" {
		log.Fatal("JSON metadata file is required for create-meeting-invite action")
	}
	if *senderEmail == "" {
		log.Fatal("Sender email is required for create-meeting-invite action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would create meeting invite for topic %s using metadata %s from %s for customer %s (force-update: %v)\n", *topicName, *jsonMetadata, *senderEmail, *customerCode, forceUpdate)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Creating meeting invite for topic %s using metadata %s from %s (force-update: %v)\n", *topicName, *jsonMetadata, *senderEmail, forceUpdate)
	fmt.Printf("Note: This action requires the CreateMeetingInvite function to be implemented in internal/ses\n")
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

	if dryRun {
		fmt.Printf("DRY RUN: Would import AWS contact for user %s to customer %s\n", *username, *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Importing AWS contact for user %s to customer %s\n", *username, *customerCode)
	if *mgmtRoleArn != "" {
		fmt.Printf("Using management role: %s\n", *mgmtRoleArn)
	}
	if *identityCenterID != "" {
		fmt.Printf("Using Identity Center ID: %s\n", *identityCenterID)
	}
	fmt.Printf("Config file: %s\n", *configFile)
	fmt.Printf("Max concurrency: %d, Requests per second: %d\n", maxConcurrency, requestsPerSecond)
	fmt.Printf("Note: This action requires AWS contact import functions to be implemented in internal/ses\n")
}

func handleImportAWSContactAll(customerCode *string, credentialManager *aws.CredentialManager, mgmtRoleArn *string, identityCenterID *string, maxConcurrency int, requestsPerSecond int, dryRun bool, configFile *string) {
	if *customerCode == "" {
		log.Fatal("Customer code is required for import-aws-contact-all action")
	}

	if dryRun {
		fmt.Printf("DRY RUN: Would import ALL AWS contacts to customer %s\n", *customerCode)
		return
	}

	customerConfig, err := credentialManager.GetCustomerConfig(*customerCode)
	if err != nil {
		log.Fatalf("Failed to get customer config: %v", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)
	_ = sesClient // Suppress unused variable warning

	fmt.Printf("Importing ALL AWS contacts to customer %s\n", *customerCode)
	if *mgmtRoleArn != "" {
		fmt.Printf("Using management role: %s\n", *mgmtRoleArn)
	}
	if *identityCenterID != "" {
		fmt.Printf("Using Identity Center ID: %s\n", *identityCenterID)
	}
	fmt.Printf("Config file: %s\n", *configFile)
	fmt.Printf("Max concurrency: %d, Requests per second: %d\n", maxConcurrency, requestsPerSecond)
	fmt.Printf("Note: This action requires AWS contact import functions to be implemented in internal/ses\n")
}
