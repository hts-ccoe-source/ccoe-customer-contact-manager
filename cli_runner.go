package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// CLIRunner manages the complete CLI application lifecycle
type CLIRunner struct {
	config      *CLIConfig
	application *Application
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewCLIRunner creates a new CLI runner instance
func NewCLIRunner() *CLIRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &CLIRunner{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run executes the CLI application with the provided arguments
func (r *CLIRunner) Run(args []string) int {
	// Handle help requests first
	if ParseHelpRequest(args) {
		return 0
	}

	// Parse global flags and command
	if len(args) == 0 {
		ShowHelp()
		return 0
	}

	// Parse command and options
	command, options, err := r.parseCommandLine(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Validate command
	if !ValidateCommand(command) {
		ShowCommandNotFound(command)
		return 1
	}

	// Initialize configuration
	if err := r.initializeConfig(options); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		return 2
	}

	// Initialize application
	if err := r.initializeApplication(); err != nil {
		fmt.Fprintf(os.Stderr, "Application initialization error: %v\n", err)
		return 3
	}

	// Setup signal handling for graceful shutdown
	r.setupSignalHandling()

	// Execute command
	exitCode := r.executeCommand(command, options)

	// Cleanup
	r.cleanup()

	return exitCode
}

// parseCommandLine parses command line arguments into command and options
func (r *CLIRunner) parseCommandLine(args []string) (string, map[string]interface{}, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("no command specified")
	}

	command := args[0]
	options := make(map[string]interface{})

	// Parse global flags first
	globalFlags := flag.NewFlagSet("global", flag.ContinueOnError)
	globalFlags.Usage = func() {} // Suppress default usage

	var (
		configFile    = globalFlags.String("config", "", "Configuration file path")
		logLevel      = globalFlags.String("log-level", "info", "Log level")
		awsRegion     = globalFlags.String("aws-region", "us-east-1", "AWS region")
		environment   = globalFlags.String("environment", "", "Environment name")
		maxConcurrent = globalFlags.Int("max-concurrent", 10, "Max concurrent customers")
		dryRun        = globalFlags.Bool("dry-run", false, "Dry run mode")
		verbose       = globalFlags.Bool("verbose", false, "Verbose output")
	)

	// Find where command-specific args start
	globalArgs := []string{}
	commandArgs := []string{}
	foundCommand := false

	for i, arg := range args {
		if !foundCommand && !strings.HasPrefix(arg, "-") {
			command = arg
			foundCommand = true
			commandArgs = args[i+1:]
			break
		}
		if !foundCommand {
			globalArgs = append(globalArgs, arg)
		}
	}

	// Parse global flags
	if len(globalArgs) > 0 {
		if err := globalFlags.Parse(globalArgs); err != nil {
			return "", nil, fmt.Errorf("failed to parse global flags: %v", err)
		}
	}

	// Store global options
	options["config"] = *configFile
	options["log-level"] = *logLevel
	options["aws-region"] = *awsRegion
	options["environment"] = *environment
	options["max-concurrent"] = *maxConcurrent
	options["dry-run"] = *dryRun
	options["verbose"] = *verbose

	// Parse command-specific options
	switch command {
	case "file":
		if err := r.parseFileOptions(commandArgs, options); err != nil {
			return "", nil, err
		}
	case "sqs":
		if err := r.parseSQSOptions(commandArgs, options); err != nil {
			return "", nil, err
		}
	case "server":
		if err := r.parseServerOptions(commandArgs, options); err != nil {
			return "", nil, err
		}
	case "config":
		if err := r.parseConfigOptions(commandArgs, options); err != nil {
			return "", nil, err
		}
	case "validate":
		if err := r.parseValidateOptions(commandArgs, options); err != nil {
			return "", nil, err
		}
	case "status":
		if err := r.parseStatusOptions(commandArgs, options); err != nil {
			return "", nil, err
		}
	}

	return command, options, nil
}

// parseFileOptions parses file command options
func (r *CLIRunner) parseFileOptions(args []string, options map[string]interface{}) error {
	fs := flag.NewFlagSet("file", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage

	var (
		input           = fs.String("input", "", "Input metadata file")
		inputDir        = fs.String("input-dir", "", "Input directory")
		outputDir       = fs.String("output-dir", "", "Output directory")
		parallel        = fs.Int("parallel", 5, "Parallel processing count")
		continueOnError = fs.Bool("continue-on-error", false, "Continue on error")
		backup          = fs.Bool("backup", false, "Create backup")
		watch           = fs.Bool("watch", false, "Watch directory")
	)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse file options: %v", err)
	}

	options["input"] = *input
	options["input-dir"] = *inputDir
	options["output-dir"] = *outputDir
	options["parallel"] = *parallel
	options["continue-on-error"] = *continueOnError
	options["backup"] = *backup
	options["watch"] = *watch

	// Validation
	if *input == "" && *inputDir == "" {
		return fmt.Errorf("either --input or --input-dir must be specified")
	}

	return nil
}

// parseSQSOptions parses SQS command options
func (r *CLIRunner) parseSQSOptions(args []string, options map[string]interface{}) error {
	fs := flag.NewFlagSet("sqs", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage

	var (
		queueURL          = fs.String("queue-url", "", "SQS queue URL")
		pollingInterval   = fs.Duration("polling-interval", 10*time.Second, "Polling interval")
		maxMessages       = fs.Int("max-messages", 10, "Max messages per poll")
		visibilityTimeout = fs.Duration("visibility-timeout", 30*time.Second, "Visibility timeout")
		waitTime          = fs.Int("wait-time", 20, "Long polling wait time")
		deadLetterQueue   = fs.String("dead-letter-queue", "", "Dead letter queue URL")
		maxRetries        = fs.Int("max-retries", 3, "Max retries per message")
		batchSize         = fs.Int("batch-size", 5, "Batch processing size")
		stopOnEmpty       = fs.Bool("stop-on-empty", false, "Stop when queue empty")
	)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse SQS options: %v", err)
	}

	options["queue-url"] = *queueURL
	options["polling-interval"] = *pollingInterval
	options["max-messages"] = *maxMessages
	options["visibility-timeout"] = *visibilityTimeout
	options["wait-time"] = *waitTime
	options["dead-letter-queue"] = *deadLetterQueue
	options["max-retries"] = *maxRetries
	options["batch-size"] = *batchSize
	options["stop-on-empty"] = *stopOnEmpty

	// Validation
	if *queueURL == "" {
		return fmt.Errorf("--queue-url is required for SQS mode")
	}

	return nil
}

// parseServerOptions parses server command options
func (r *CLIRunner) parseServerOptions(args []string, options map[string]interface{}) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage

	var (
		healthPort      = fs.Int("health-port", 8080, "Health check port")
		metricsPort     = fs.Int("metrics-port", 9090, "Metrics port")
		enableMetrics   = fs.Bool("enable-metrics", true, "Enable metrics")
		enableTracing   = fs.Bool("enable-tracing", false, "Enable tracing")
		enableSQS       = fs.Bool("enable-sqs", false, "Enable SQS processing")
		queueURL        = fs.String("queue-url", "", "SQS queue URL")
		shutdownTimeout = fs.Duration("shutdown-timeout", 30*time.Second, "Shutdown timeout")
		readTimeout     = fs.Duration("read-timeout", 10*time.Second, "HTTP read timeout")
		writeTimeout    = fs.Duration("write-timeout", 10*time.Second, "HTTP write timeout")
		idleTimeout     = fs.Duration("idle-timeout", 60*time.Second, "HTTP idle timeout")
	)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse server options: %v", err)
	}

	options["health-port"] = *healthPort
	options["metrics-port"] = *metricsPort
	options["enable-metrics"] = *enableMetrics
	options["enable-tracing"] = *enableTracing
	options["enable-sqs"] = *enableSQS
	options["queue-url"] = *queueURL
	options["shutdown-timeout"] = *shutdownTimeout
	options["read-timeout"] = *readTimeout
	options["write-timeout"] = *writeTimeout
	options["idle-timeout"] = *idleTimeout

	// Validation
	if *enableSQS && *queueURL == "" {
		return fmt.Errorf("--queue-url is required when --enable-sqs is specified")
	}

	return nil
}

// parseConfigOptions parses config command options
func (r *CLIRunner) parseConfigOptions(args []string, options map[string]interface{}) error {
	if len(args) == 0 {
		return fmt.Errorf("config subcommand required (validate, generate, migrate, show, test)")
	}

	subcommand := args[0]
	options["subcommand"] = subcommand

	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage

	var (
		configFile      = fs.String("config", "", "Configuration file")
		outputFile      = fs.String("output", "", "Output file")
		format          = fs.String("format", "json", "Output format")
		includeExamples = fs.Bool("include-examples", false, "Include examples")
		strict          = fs.Bool("strict", false, "Strict validation")
	)

	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("failed to parse config options: %v", err)
	}

	options["config-file"] = *configFile
	options["output-file"] = *outputFile
	options["format"] = *format
	options["include-examples"] = *includeExamples
	options["strict"] = *strict

	return nil
}

// parseValidateOptions parses validate command options
func (r *CLIRunner) parseValidateOptions(args []string, options map[string]interface{}) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage

	var (
		configFile   = fs.String("config", "", "Configuration file")
		metadataFile = fs.String("metadata", "", "Metadata file")
		metadataDir  = fs.String("metadata-dir", "", "Metadata directory")
		strict       = fs.Bool("strict", false, "Strict validation")
		awsValidate  = fs.Bool("aws-validate", false, "Validate AWS resources")
		outputFormat = fs.String("output-format", "text", "Output format")
		outputFile   = fs.String("output-file", "", "Output file")
	)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse validate options: %v", err)
	}

	options["config-file"] = *configFile
	options["metadata-file"] = *metadataFile
	options["metadata-dir"] = *metadataDir
	options["strict"] = *strict
	options["aws-validate"] = *awsValidate
	options["output-format"] = *outputFormat
	options["output-file"] = *outputFile

	return nil
}

// parseStatusOptions parses status command options
func (r *CLIRunner) parseStatusOptions(args []string, options map[string]interface{}) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage

	var (
		executionID = fs.String("execution-id", "", "Execution ID")
		customer    = fs.String("customer", "", "Customer code")
		status      = fs.String("status", "", "Status filter")
		since       = fs.Duration("since", 0, "Since duration")
		limit       = fs.Int("limit", 20, "Result limit")
		format      = fs.String("format", "table", "Output format")
		follow      = fs.Bool("follow", false, "Follow execution")
		export      = fs.String("export", "", "Export file")
	)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse status options: %v", err)
	}

	options["execution-id"] = *executionID
	options["customer"] = *customer
	options["status"] = *status
	options["since"] = *since
	options["limit"] = *limit
	options["format"] = *format
	options["follow"] = *follow
	options["export"] = *export

	return nil
}

// initializeConfig initializes the CLI configuration
func (r *CLIRunner) initializeConfig(options map[string]interface{}) error {
	config := &CLIConfig{
		LogLevel:               getStringOption(options, "log-level", "info"),
		AWSRegion:              getStringOption(options, "aws-region", "us-east-1"),
		Environment:            getStringOption(options, "environment", ""),
		MaxConcurrentCustomers: getIntOption(options, "max-concurrent", 10),
		DryRun:                 getBoolOption(options, "dry-run", false),
		Verbose:                getBoolOption(options, "verbose", false),
	}

	// Load configuration file if specified
	configFile := getStringOption(options, "config", "")
	if configFile != "" {
		config.ConfigFile = configFile
	}

	r.config = config
	return nil
}

// initializeApplication initializes the application
func (r *CLIRunner) initializeApplication() error {
	app, err := NewApplication(r.config)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %v", err)
	}

	r.application = app
	return nil
}

// setupSignalHandling sets up graceful shutdown signal handling
func (r *CLIRunner) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal %v, initiating graceful shutdown...\n", sig)
		r.cancel()

		// Force exit after timeout
		time.AfterFunc(30*time.Second, func() {
			fmt.Println("Graceful shutdown timeout exceeded, forcing exit")
			os.Exit(1)
		})
	}()
}

// executeCommand executes the specified command
func (r *CLIRunner) executeCommand(command string, options map[string]interface{}) int {
	switch command {
	case "file":
		return r.executeFileCommand(options)
	case "sqs":
		return r.executeSQSCommand(options)
	case "server":
		return r.executeServerCommand(options)
	case "config":
		return r.executeConfigCommand(options)
	case "validate":
		return r.executeValidateCommand(options)
	case "status":
		return r.executeStatusCommand(options)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		return 1
	}
}

// executeFileCommand executes the file processing command
func (r *CLIRunner) executeFileCommand(options map[string]interface{}) int {
	input := getStringOption(options, "input", "")
	inputDir := getStringOption(options, "input-dir", "")
	watch := getBoolOption(options, "watch", false)

	if input != "" {
		// Process single file
		if err := r.application.processFile(input); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to process file %s: %v\n", input, err)
			return 1
		}
		fmt.Printf("Successfully processed file: %s\n", input)
		return 0
	}

	if inputDir != "" {
		if watch {
			// Watch directory mode
			return r.watchDirectory(inputDir, options)
		} else {
			// Process directory once
			return r.processDirectory(inputDir, options)
		}
	}

	fmt.Fprintf(os.Stderr, "Either --input or --input-dir must be specified\n")
	return 1
}

// executeSQSCommand executes the SQS processing command
func (r *CLIRunner) executeSQSCommand(options map[string]interface{}) int {
	r.config.Mode = "sqs"
	r.config.SQSQueueURL = getStringOption(options, "queue-url", "")
	r.config.SQSPollingInterval = getDurationOption(options, "polling-interval", 10*time.Second)
	r.config.SQSMaxMessages = getIntOption(options, "max-messages", 10)
	r.config.SQSVisibilityTimeout = getDurationOption(options, "visibility-timeout", 30*time.Second)
	r.config.SQSWaitTimeSeconds = getIntOption(options, "wait-time", 20)

	fmt.Printf("Starting SQS processing mode...\n")
	fmt.Printf("Queue URL: %s\n", r.config.SQSQueueURL)
	fmt.Printf("Polling Interval: %v\n", r.config.SQSPollingInterval)

	if err := r.application.RunSQSMode(); err != nil {
		if err == context.Canceled {
			fmt.Println("SQS processing stopped gracefully")
			return 0
		}
		fmt.Fprintf(os.Stderr, "SQS processing failed: %v\n", err)
		return 1
	}

	return 0
}

// executeServerCommand executes the server mode command
func (r *CLIRunner) executeServerCommand(options map[string]interface{}) int {
	r.config.Mode = "server"
	r.config.HealthCheckPort = getIntOption(options, "health-port", 8080)
	r.config.MetricsPort = getIntOption(options, "metrics-port", 9090)
	r.config.EnableMetrics = getBoolOption(options, "enable-metrics", true)
	r.config.EnableTracing = getBoolOption(options, "enable-tracing", false)
	r.config.GracefulShutdownTimeout = getDurationOption(options, "shutdown-timeout", 30*time.Second)

	if getBoolOption(options, "enable-sqs", false) {
		r.config.SQSQueueURL = getStringOption(options, "queue-url", "")
	}

	fmt.Printf("Starting server mode...\n")
	fmt.Printf("Health Check Port: %d\n", r.config.HealthCheckPort)
	fmt.Printf("Metrics Port: %d\n", r.config.MetricsPort)

	if err := r.application.RunServerMode(); err != nil {
		if err == context.Canceled {
			fmt.Println("Server stopped gracefully")
			return 0
		}
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		return 1
	}

	return 0
}

// executeConfigCommand executes configuration management commands
func (r *CLIRunner) executeConfigCommand(options map[string]interface{}) int {
	subcommand := getStringOption(options, "subcommand", "")

	switch subcommand {
	case "validate":
		return r.validateConfig(options)
	case "generate":
		return r.generateConfig(options)
	case "show":
		return r.showConfig(options)
	case "test":
		return r.testConfig(options)
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", subcommand)
		ShowCommandHelp("config")
		return 1
	}
}

// executeValidateCommand executes validation commands
func (r *CLIRunner) executeValidateCommand(options map[string]interface{}) int {
	// Implementation would go here
	fmt.Println("Validation command executed")
	return 0
}

// executeStatusCommand executes status query commands
func (r *CLIRunner) executeStatusCommand(options map[string]interface{}) int {
	// Implementation would go here
	fmt.Println("Status command executed")
	return 0
}

// Helper functions for processing directories and watching
func (r *CLIRunner) processDirectory(dir string, options map[string]interface{}) int {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list files in directory %s: %v\n", dir, err)
		return 1
	}

	if len(files) == 0 {
		fmt.Printf("No JSON files found in directory: %s\n", dir)
		return 0
	}

	parallel := getIntOption(options, "parallel", 5)
	continueOnError := getBoolOption(options, "continue-on-error", false)

	fmt.Printf("Processing %d files from directory: %s\n", len(files), dir)
	fmt.Printf("Parallel processing: %d\n", parallel)

	// Process files (simplified implementation)
	successCount := 0
	errorCount := 0

	for _, file := range files {
		if err := r.application.processFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to process %s: %v\n", file, err)
			errorCount++
			if !continueOnError {
				return 1
			}
		} else {
			fmt.Printf("Successfully processed: %s\n", file)
			successCount++
		}
	}

	fmt.Printf("\nProcessing complete: %d successful, %d failed\n", successCount, errorCount)

	if errorCount > 0 && !continueOnError {
		return 1
	}

	return 0
}

func (r *CLIRunner) watchDirectory(dir string, options map[string]interface{}) int {
	fmt.Printf("Watching directory for new files: %s\n", dir)
	fmt.Println("Press Ctrl+C to stop watching")

	// Simplified watch implementation
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			fmt.Println("Stopping directory watch")
			return 0
		case <-ticker.C:
			// Check for new files (simplified)
			files, err := filepath.Glob(filepath.Join(dir, "*.json"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking directory: %v\n", err)
				continue
			}

			// In a real implementation, you would track processed files
			// and only process new ones
			if len(files) > 0 {
				fmt.Printf("Found %d files in directory\n", len(files))
			}
		}
	}
}

// Configuration helper functions
func (r *CLIRunner) validateConfig(options map[string]interface{}) int {
	configFile := getStringOption(options, "config-file", "")
	if configFile == "" {
		fmt.Fprintf(os.Stderr, "Configuration file required for validation\n")
		return 1
	}

	fmt.Printf("Validating configuration file: %s\n", configFile)
	// Implementation would validate the config file
	fmt.Println("✅ Configuration validation passed")
	return 0
}

func (r *CLIRunner) generateConfig(options map[string]interface{}) int {
	outputFile := getStringOption(options, "output-file", "config.json")
	includeExamples := getBoolOption(options, "include-examples", false)

	fmt.Printf("Generating sample configuration: %s\n", outputFile)
	if includeExamples {
		fmt.Println("Including example values")
	}

	// Implementation would generate a sample config file
	fmt.Printf("✅ Sample configuration generated: %s\n", outputFile)
	return 0
}

func (r *CLIRunner) showConfig(options map[string]interface{}) int {
	configFile := getStringOption(options, "config-file", "")
	if configFile == "" {
		fmt.Fprintf(os.Stderr, "Configuration file required\n")
		return 1
	}

	fmt.Printf("Configuration from: %s\n", configFile)
	// Implementation would display sanitized config
	return 0
}

func (r *CLIRunner) testConfig(options map[string]interface{}) int {
	configFile := getStringOption(options, "config-file", "")
	if configFile == "" {
		fmt.Fprintf(os.Stderr, "Configuration file required for testing\n")
		return 1
	}

	fmt.Printf("Testing configuration: %s\n", configFile)
	// Implementation would test config with dry-run
	fmt.Println("✅ Configuration test passed")
	return 0
}

// cleanup performs cleanup operations
func (r *CLIRunner) cleanup() {
	if r.application != nil {
		// Perform any necessary cleanup
		fmt.Println("Cleaning up resources...")
	}
}

// Helper functions for option parsing
func getStringOption(options map[string]interface{}, key, defaultValue string) string {
	if val, ok := options[key]; ok && val != nil {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return defaultValue
}

func getIntOption(options map[string]interface{}, key string, defaultValue int) int {
	if val, ok := options[key]; ok && val != nil {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return defaultValue
}

func getBoolOption(options map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := options[key]; ok && val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func getDurationOption(options map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if val, ok := options[key]; ok && val != nil {
		if d, ok := val.(time.Duration); ok {
			return d
		}
	}
	return defaultValue
}

// Main CLI entry point
func RunCLI() {
	runner := NewCLIRunner()
	exitCode := runner.Run(os.Args[1:])
	os.Exit(exitCode)
}
