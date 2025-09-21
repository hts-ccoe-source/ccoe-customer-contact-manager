package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Version information (set during build)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
	GitBranch = "unknown"
)

// CLI configuration
type CLIConfig struct {
	Mode                    string        `json:"mode"` // "file", "sqs", "server"
	ConfigFile              string        `json:"configFile"`
	LogLevel                string        `json:"logLevel"`
	SQSQueueURL             string        `json:"sqsQueueUrl"`
	SQSPollingInterval      time.Duration `json:"sqsPollingInterval"`
	SQSMaxMessages          int           `json:"sqsMaxMessages"`
	SQSVisibilityTimeout    time.Duration `json:"sqsVisibilityTimeout"`
	SQSWaitTimeSeconds      int           `json:"sqsWaitTimeSeconds"`
	MaxConcurrentCustomers  int           `json:"maxConcurrentCustomers"`
	GracefulShutdownTimeout time.Duration `json:"gracefulShutdownTimeout"`
	HealthCheckPort         int           `json:"healthCheckPort"`
	MetricsPort             int           `json:"metricsPort"`
	EnableMetrics           bool          `json:"enableMetrics"`
	EnableTracing           bool          `json:"enableTracing"`
	AWSRegion               string        `json:"awsRegion"`
	Environment             string        `json:"environment"`
	DryRun                  bool          `json:"dryRun"`
	Verbose                 bool          `json:"verbose"`
}

// Application represents the main application
type Application struct {
	config           *CLIConfig
	customerManager  *CustomerCredentialManager
	templateManager  *EmailTemplateManager
	sesManager       *SESIntegrationManager
	statusTracker    *ExecutionStatusTracker
	errorHandler     *ErrorHandler
	monitoringSystem *MonitoringSystem
	sqsProcessor     *EnhancedSQSProcessor
	configManager    *ConfigurationManager
	shutdownChan     chan os.Signal
	ctx              context.Context
	cancel           context.CancelFunc
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Show version if requested
	if config.Mode == "version" {
		showVersion()
		return
	}

	// Show help if requested
	if config.Mode == "help" {
		showHelp()
		return
	}

	// Initialize application
	app, err := NewApplication(config)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Run application based on mode
	switch config.Mode {
	case "file":
		err = app.RunFileMode()
	case "sqs":
		err = app.RunSQSMode()
	case "server":
		err = app.RunServerMode()
	default:
		log.Fatalf("Unknown mode: %s", config.Mode)
	}

	if err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}

// parseFlags parses command line flags and returns configuration
func parseFlags() *CLIConfig {
	config := &CLIConfig{
		Mode:                    "file",
		LogLevel:                "info",
		SQSPollingInterval:      5 * time.Second,
		SQSMaxMessages:          10,
		SQSVisibilityTimeout:    30 * time.Second,
		SQSWaitTimeSeconds:      20,
		MaxConcurrentCustomers:  10,
		GracefulShutdownTimeout: 30 * time.Second,
		HealthCheckPort:         8081,
		MetricsPort:             9090,
		EnableMetrics:           true,
		EnableTracing:           false,
		AWSRegion:               "us-east-1",
		Environment:             "production",
	}

	// Define flags
	flag.StringVar(&config.Mode, "mode", config.Mode, "Execution mode: file, sqs, server, version, help")
	flag.StringVar(&config.ConfigFile, "config", "", "Path to configuration file")
	flag.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level: debug, info, warn, error")
	flag.StringVar(&config.SQSQueueURL, "sqs-queue-url", "", "SQS queue URL for message processing")
	flag.DurationVar(&config.SQSPollingInterval, "sqs-polling-interval", config.SQSPollingInterval, "SQS polling interval")
	flag.IntVar(&config.SQSMaxMessages, "sqs-max-messages", config.SQSMaxMessages, "Maximum messages to receive per SQS poll")
	flag.DurationVar(&config.SQSVisibilityTimeout, "sqs-visibility-timeout", config.SQSVisibilityTimeout, "SQS message visibility timeout")
	flag.IntVar(&config.SQSWaitTimeSeconds, "sqs-wait-time", config.SQSWaitTimeSeconds, "SQS long polling wait time")
	flag.IntVar(&config.MaxConcurrentCustomers, "max-concurrent-customers", config.MaxConcurrentCustomers, "Maximum concurrent customer processing")
	flag.DurationVar(&config.GracefulShutdownTimeout, "shutdown-timeout", config.GracefulShutdownTimeout, "Graceful shutdown timeout")
	flag.IntVar(&config.HealthCheckPort, "health-port", config.HealthCheckPort, "Health check port")
	flag.IntVar(&config.MetricsPort, "metrics-port", config.MetricsPort, "Metrics port")
	flag.BoolVar(&config.EnableMetrics, "enable-metrics", config.EnableMetrics, "Enable metrics collection")
	flag.BoolVar(&config.EnableTracing, "enable-tracing", config.EnableTracing, "Enable distributed tracing")
	flag.StringVar(&config.AWSRegion, "aws-region", config.AWSRegion, "AWS region")
	flag.StringVar(&config.Environment, "environment", config.Environment, "Environment (development, staging, production)")

	flag.Parse()

	// Load configuration from environment variables
	loadConfigFromEnv(config)

	return config
}

// loadConfigFromEnv loads configuration from environment variables
func loadConfigFromEnv(config *CLIConfig) {
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		config.LogLevel = val
	}
	if val := os.Getenv("SQS_QUEUE_URL"); val != "" {
		config.SQSQueueURL = val
	}
	if val := os.Getenv("AWS_REGION"); val != "" {
		config.AWSRegion = val
	}
	if val := os.Getenv("ENVIRONMENT"); val != "" {
		config.Environment = val
	}
	if val := os.Getenv("ENABLE_METRICS"); val == "false" {
		config.EnableMetrics = false
	}
	if val := os.Getenv("ENABLE_TRACING"); val == "true" {
		config.EnableTracing = true
	}
}

// NewApplication creates a new application instance
func NewApplication(config *CLIConfig) (*Application, error) {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize core components
	customerManager := NewCustomerCredentialManager(config.AWSRegion)

	// Load customer configuration if config file provided
	if config.ConfigFile != "" {
		configManager := NewConfigurationManager(config.ConfigFile)
		if err := configManager.LoadConfiguration(); err != nil {
			return nil, fmt.Errorf("failed to load configuration: %v", err)
		}

		// Convert configuration to customer mappings
		for customerCode, customerConfig := range configManager.Configuration.CustomerMappings {
			accountInfo := CustomerAccountInfo{
				CustomerCode: customerCode,
				CustomerName: customerConfig.CustomerName,
				AWSAccountID: customerConfig.AWSAccountID,
				Region:       customerConfig.Region,
				SESRoleARN:   customerConfig.RoleARNs["ses"],
				SQSRoleARN:   customerConfig.RoleARNs["sqs"],
				S3RoleARN:    customerConfig.RoleARNs["s3"],
				Environment:  customerConfig.Environment,
			}
			customerManager.CustomerMappings[customerCode] = accountInfo
		}
	}

	// Initialize other components
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	// Initialize monitoring system
	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch:    config.EnableMetrics,
		EnableXRay:          config.EnableTracing,
		MetricsNamespace:    "EmailDistribution",
		HealthCheckInterval: 30 * time.Second,
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	// Initialize SQS processor if in SQS mode
	var sqsProcessor *EnhancedSQSProcessor
	if config.Mode == "sqs" || config.Mode == "server" {
		sqsConfig := SQSProcessorConfig{
			QueueURL:          config.SQSQueueURL,
			MaxMessages:       int32(config.SQSMaxMessages),
			VisibilityTimeout: int32(config.SQSVisibilityTimeout.Seconds()),
			WaitTimeSeconds:   int32(config.SQSWaitTimeSeconds),
			PollingInterval:   config.SQSPollingInterval,
			MaxRetries:        3,
			Region:            config.AWSRegion,
		}

		var err error
		sqsProcessor, err = NewEnhancedSQSProcessor(sqsConfig, customerManager, templateManager, sesManager, statusTracker, errorHandler, monitoringSystem)
		if err != nil {
			return nil, fmt.Errorf("failed to create SQS processor: %v", err)
		}
	}

	// Set up shutdown signal handling
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	app := &Application{
		config:           config,
		customerManager:  customerManager,
		templateManager:  templateManager,
		sesManager:       sesManager,
		statusTracker:    statusTracker,
		errorHandler:     errorHandler,
		monitoringSystem: monitoringSystem,
		sqsProcessor:     sqsProcessor,
		shutdownChan:     shutdownChan,
		ctx:              ctx,
		cancel:           cancel,
	}

	return app, nil
}

// RunFileMode runs the application in file processing mode
func (app *Application) RunFileMode() error {
	app.monitoringSystem.logger.Info("Starting file processing mode", map[string]interface{}{
		"version": Version,
		"mode":    "file",
	})

	// Get remaining command line arguments (file paths)
	args := flag.Args()
	if len(args) == 0 {
		return fmt.Errorf("no input files specified")
	}

	// Process each file
	for _, filePath := range args {
		app.monitoringSystem.logger.Info("Processing file", map[string]interface{}{
			"file": filePath,
		})

		// Read and process file
		if err := app.processFile(filePath); err != nil {
			app.monitoringSystem.logger.Error("Failed to process file", err, map[string]interface{}{
				"file": filePath,
			})
			return err
		}

		app.monitoringSystem.logger.Info("File processed successfully", map[string]interface{}{
			"file": filePath,
		})
	}

	return nil
}

// RunSQSMode runs the application in SQS message processing mode
func (app *Application) RunSQSMode() error {
	app.monitoringSystem.logger.Info("Starting SQS processing mode", map[string]interface{}{
		"version":         Version,
		"mode":            "sqs",
		"queueUrl":        app.config.SQSQueueURL,
		"pollingInterval": app.config.SQSPollingInterval.String(),
	})

	if app.sqsProcessor == nil {
		return fmt.Errorf("SQS processor not initialized")
	}

	if app.config.SQSQueueURL == "" {
		return fmt.Errorf("SQS queue URL not specified")
	}

	// Start SQS processing in a goroutine
	processingDone := make(chan error, 1)
	go func() {
		processingDone <- app.sqsProcessor.StartProcessing()
	}()

	// Wait for shutdown signal or processing error
	select {
	case <-app.shutdownChan:
		app.monitoringSystem.logger.Info("Received shutdown signal, initiating graceful shutdown", nil)
		return app.gracefulShutdown()
	case err := <-processingDone:
		if err != nil {
			app.monitoringSystem.logger.Error("SQS processing failed", err, nil)
			return err
		}
		return nil
	}
}

// RunServerMode runs the application in server mode (HTTP + SQS)
func (app *Application) RunServerMode() error {
	app.monitoringSystem.logger.Info("Starting server mode", map[string]interface{}{
		"version":       Version,
		"mode":          "server",
		"healthPort":    app.config.HealthCheckPort,
		"metricsPort":   app.config.MetricsPort,
		"enableMetrics": app.config.EnableMetrics,
		"enableTracing": app.config.EnableTracing,
	})

	// Start health check server
	go app.startHealthCheckServer()

	// Start metrics server if enabled
	if app.config.EnableMetrics {
		go app.startMetricsServer()
	}

	// Start SQS processing if queue URL provided
	if app.config.SQSQueueURL != "" && app.sqsProcessor != nil {
		go func() {
			if err := app.sqsProcessor.StartProcessing(); err != nil {
				app.monitoringSystem.logger.Error("SQS processing failed", err, nil)
			}
		}()
	}

	// Wait for shutdown signal
	<-app.shutdownChan
	app.monitoringSystem.logger.Info("Received shutdown signal, initiating graceful shutdown", nil)
	return app.gracefulShutdown()
}

// processFile processes a single metadata file
func (app *Application) processFile(filePath string) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	// Parse metadata
	var metadata map[string]interface{}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %v", filePath, err)
	}

	// Extract customer codes
	customerCodes, err := extractCustomerCodes(metadata)
	if err != nil {
		return fmt.Errorf("failed to extract customer codes from %s: %v", filePath, err)
	}

	if len(customerCodes) == 0 {
		app.monitoringSystem.logger.Warn("No customer codes found in file", map[string]interface{}{
			"file": filePath,
		})
		return nil
	}

	// Start execution tracking
	execution, err := app.statusTracker.StartExecution(
		fmt.Sprintf("FILE-%d", time.Now().Unix()),
		fmt.Sprintf("Process file: %s", filePath),
		fmt.Sprintf("Processing metadata file %s for customers: %s", filePath, strings.Join(customerCodes, ", ")),
		"cli-user",
		customerCodes,
	)
	if err != nil {
		return fmt.Errorf("failed to start execution tracking: %v", err)
	}

	// Process each customer
	for _, customerCode := range customerCodes {
		app.monitoringSystem.logger.Info("Processing customer", map[string]interface{}{
			"customer":    customerCode,
			"executionId": execution.ExecutionID,
		})

		err := app.processCustomerFromFile(execution.ExecutionID, customerCode, metadata)
		if err != nil {
			app.monitoringSystem.logger.Error("Customer processing failed", err, map[string]interface{}{
				"customer":    customerCode,
				"executionId": execution.ExecutionID,
			})
		}
	}

	return nil
}

// processCustomerFromFile processes a customer's email distribution from file metadata
func (app *Application) processCustomerFromFile(executionID, customerCode string, metadata map[string]interface{}) error {
	// Start customer execution
	if err := app.statusTracker.StartCustomerExecution(executionID, customerCode); err != nil {
		return err
	}

	// Use error handler for retry logic
	operation := func(ctx context.Context) error {
		// Add execution steps
		steps := []struct {
			id, name, description string
		}{
			{"validate", "Validate Metadata", "Validate customer metadata and configuration"},
			{"render", "Render Templates", "Render email templates with customer data"},
			{"send", "Send Emails", "Send emails via SES"},
			{"verify", "Verify Delivery", "Verify email delivery status"},
		}

		for _, step := range steps {
			app.statusTracker.AddExecutionStep(executionID, customerCode, step.id, step.name, step.description)
			app.statusTracker.UpdateExecutionStep(executionID, customerCode, step.id, StepStatusRunning, "")

			// Simulate step processing
			time.Sleep(100 * time.Millisecond)

			// Complete step
			app.statusTracker.UpdateExecutionStep(executionID, customerCode, step.id, StepStatusCompleted, "")
		}

		return nil
	}

	// Execute with retry logic
	err := app.errorHandler.ExecuteWithRetry(context.Background(), customerCode, operation)

	// Complete customer execution
	success := err == nil
	var errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}

	return app.statusTracker.CompleteCustomerExecution(executionID, customerCode, success, errorMessage)
}

// startHealthCheckServer starts the health check HTTP server
func (app *Application) startHealthCheckServer() {
	app.monitoringSystem.logger.Info("Starting health check server", map[string]interface{}{
		"port": app.config.HealthCheckPort,
	})

	// Simple HTTP server for health checks
	// In a real implementation, you would use a proper HTTP framework
	app.monitoringSystem.logger.Info("Health check server would start here", map[string]interface{}{
		"port":     app.config.HealthCheckPort,
		"endpoint": "/health",
	})
}

// startMetricsServer starts the metrics HTTP server
func (app *Application) startMetricsServer() {
	app.monitoringSystem.logger.Info("Starting metrics server", map[string]interface{}{
		"port": app.config.MetricsPort,
	})

	// Simple HTTP server for metrics
	// In a real implementation, you would use a proper HTTP framework
	app.monitoringSystem.logger.Info("Metrics server would start here", map[string]interface{}{
		"port":     app.config.MetricsPort,
		"endpoint": "/metrics",
	})
}

// gracefulShutdown performs graceful shutdown
func (app *Application) gracefulShutdown() error {
	app.monitoringSystem.logger.Info("Starting graceful shutdown", map[string]interface{}{
		"timeout": app.config.GracefulShutdownTimeout.String(),
	})

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), app.config.GracefulShutdownTimeout)
	defer shutdownCancel()

	// Cancel main context to stop all operations
	app.cancel()

	// Stop SQS processing
	if app.sqsProcessor != nil {
		app.monitoringSystem.logger.Info("Stopping SQS processor", nil)
		app.sqsProcessor.StopProcessing()
	}

	// Wait for operations to complete or timeout
	done := make(chan struct{})
	go func() {
		// Wait for all operations to complete
		// In a real implementation, you would wait for all goroutines to finish
		time.Sleep(1 * time.Second)
		close(done)
	}()

	select {
	case <-done:
		app.monitoringSystem.logger.Info("Graceful shutdown completed", nil)
		return nil
	case <-shutdownCtx.Done():
		app.monitoringSystem.logger.Warn("Graceful shutdown timed out, forcing exit", nil)
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// showVersion displays version information
func showVersion() {
	fmt.Printf("Multi-Customer Email Distribution CLI\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Build Time: %s\n", BuildTime)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Git Branch: %s\n", GitBranch)
}

// showHelp displays help information
func showHelp() {
	fmt.Printf("Multi-Customer Email Distribution CLI\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s [OPTIONS] [FILES...]\n\n", os.Args[0])
	fmt.Printf("MODES:\n")
	fmt.Printf("  file    Process metadata files directly (default)\n")
	fmt.Printf("  sqs     Process messages from SQS queue\n")
	fmt.Printf("  server  Run as HTTP server with SQS processing\n")
	fmt.Printf("  version Show version information\n")
	fmt.Printf("  help    Show this help message\n\n")
	fmt.Printf("OPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("  # Process metadata files\n")
	fmt.Printf("  %s --mode=file metadata1.json metadata2.json\n\n", os.Args[0])
	fmt.Printf("  # Process SQS messages\n")
	fmt.Printf("  %s --mode=sqs --sqs-queue-url=https://sqs.us-east-1.amazonaws.com/123456789012/email-queue\n\n", os.Args[0])
	fmt.Printf("  # Run as server\n")
	fmt.Printf("  %s --mode=server --health-port=8081 --metrics-port=9090\n\n", os.Args[0])
	fmt.Printf("ENVIRONMENT VARIABLES:\n")
	fmt.Printf("  LOG_LEVEL           Log level (debug, info, warn, error)\n")
	fmt.Printf("  SQS_QUEUE_URL       SQS queue URL for message processing\n")
	fmt.Printf("  AWS_REGION          AWS region\n")
	fmt.Printf("  ENVIRONMENT         Environment (development, staging, production)\n")
	fmt.Printf("  ENABLE_METRICS      Enable metrics collection (true/false)\n")
	fmt.Printf("  ENABLE_TRACING      Enable distributed tracing (true/false)\n")
}

// extractCustomerCodes extracts customer codes from metadata
func extractCustomerCodes(metadata map[string]interface{}) ([]string, error) {
	// Look for customer_codes field
	if codes, ok := metadata["customer_codes"]; ok {
		switch v := codes.(type) {
		case []interface{}:
			var customerCodes []string
			for _, code := range v {
				if str, ok := code.(string); ok {
					customerCodes = append(customerCodes, str)
				}
			}
			return customerCodes, nil
		case []string:
			return v, nil
		case string:
			return []string{v}, nil
		}
	}

	// Look for customer_code field (singular)
	if code, ok := metadata["customer_code"]; ok {
		if str, ok := code.(string); ok {
			return []string{str}, nil
		}
	}

	return nil, fmt.Errorf("no customer codes found in metadata")
}
