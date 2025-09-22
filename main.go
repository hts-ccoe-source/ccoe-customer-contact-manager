package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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
	// Command line flags
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
			log.Printf("  %s: %s (%s)", code, info.CustomerName, info.AWSAccountID)
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

// showHelp displays help information
func showHelp() {
	fmt.Printf("AWS Alternate Contact Manager\n\n")
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
