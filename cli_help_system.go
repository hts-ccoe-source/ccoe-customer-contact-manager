package main

import (
	"fmt"
	"os"
	"strings"
)

// CLI Help System for Multi-Customer Email Distribution

const (
	CLIVersion = "1.0.0"
	CLIName    = "multi-customer-email-distribution"
)

// ShowHelp displays comprehensive help information
func ShowHelp() {
	fmt.Printf(`%s v%s - Multi-Customer Email Distribution CLI

DESCRIPTION:
    A comprehensive CLI tool for managing email distribution across multiple customer 
    AWS accounts. Supports file processing, SQS message processing, and server modes
    with advanced monitoring, error handling, and configuration management.

USAGE:
    %s [GLOBAL OPTIONS] COMMAND [COMMAND OPTIONS]

GLOBAL OPTIONS:
    --help, -h                     Show help information
    --version, -v                  Show version information
    --config FILE                  Load configuration from FILE
    --log-level LEVEL             Set logging level (debug, info, warn, error)
    --aws-region REGION           AWS region to use (default: us-east-1)
    --environment ENV             Environment name (dev, staging, prod)
    --max-concurrent INT          Maximum concurrent customer processing (default: 10)
    --dry-run                     Simulate operations without executing
    --verbose                     Enable verbose output

COMMANDS:
    file        Process email distribution from metadata files
    sqs         Process email distribution from SQS messages
    server      Run as a server with health checks and metrics
    config      Configuration management commands
    validate    Validate configuration and metadata files
    status      Check execution status and history
    help        Show detailed help for commands

EXAMPLES:
    # Process a single metadata file
    %s file --input metadata.json

    # Process multiple files with custom configuration
    %s --config config.json file --input-dir ./metadata/

    # Start SQS processing mode
    %s sqs --queue-url https://sqs.us-east-1.amazonaws.com/123456789012/email-queue

    # Run server mode with custom ports
    %s server --health-port 8080 --metrics-port 9090

    # Validate configuration
    %s validate --config config.json

    # Check execution status
    %s status --execution-id abc123

For detailed command help, use: %s help COMMAND

`, CLIName, CLIVersion, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName)
}

// ShowCommandHelp displays help for specific commands
func ShowCommandHelp(command string) {
	switch strings.ToLower(command) {
	case "file":
		showFileCommandHelp()
	case "sqs":
		showSQSCommandHelp()
	case "server":
		showServerCommandHelp()
	case "config":
		showConfigCommandHelp()
	case "validate":
		showValidateCommandHelp()
	case "status":
		showStatusCommandHelp()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		ShowHelp()
	}
}

func showFileCommandHelp() {
	fmt.Printf(`FILE COMMAND - Process email distribution from metadata files

USAGE:
    %s file [OPTIONS]

DESCRIPTION:
    Process email distribution requests from JSON metadata files. Supports single
    file processing or batch processing from a directory. Each file should contain
    customer codes, email template information, and distribution metadata.

OPTIONS:
    --input FILE, -i FILE         Process a single metadata file
    --input-dir DIR, -d DIR       Process all JSON files in directory
    --output-dir DIR              Directory for processing results
    --parallel INT                Number of files to process in parallel (default: 5)
    --continue-on-error           Continue processing other files if one fails
    --backup                      Create backup of processed files
    --watch                       Watch directory for new files (continuous mode)

METADATA FILE FORMAT:
    {
      "customer_codes": ["hts", "cds"],           // Required: Customer identifiers
      "change_id": "CHANGE-001",                  // Required: Unique change identifier
      "title": "Monthly Newsletter",              // Required: Distribution title
      "description": "Newsletter distribution",   // Optional: Description
      "template_id": "newsletter",                // Required: Email template ID
      "priority": "normal",                       // Optional: Priority (low, normal, high)
      "email_data": {                            // Required: Template data
        "subject": "Monthly Update",
        "message": "Your monthly newsletter..."
      },
      "scheduled_time": "2024-12-15T10:00:00Z"  // Optional: Scheduled delivery
    }

EXAMPLES:
    # Process single file
    %s file --input newsletter.json

    # Process all files in directory
    %s file --input-dir ./metadata --parallel 3

    # Watch directory for new files
    %s file --input-dir ./metadata --watch

    # Process with backup and error handling
    %s file --input-dir ./metadata --backup --continue-on-error

SUPPORTED CUSTOMER CODE FORMATS:
    - Array: "customer_codes": ["hts", "cds", "motor"]
    - Single: "customer_codes": "hts"
    - Singular: "customer_code": "hts"

EXIT CODES:
    0    Success - All files processed successfully
    1    Partial failure - Some files failed
    2    Complete failure - All files failed
    3    Configuration error
    4    File not found or permission error

`, CLIName, CLIName, CLIName, CLIName, CLIName)
}

func showSQSCommandHelp() {
	fmt.Printf(`SQS COMMAND - Process email distribution from SQS messages

USAGE:
    %s sqs [OPTIONS]

DESCRIPTION:
    Start SQS message processing mode. Continuously polls an SQS queue for email
    distribution requests and processes them. Supports long polling, batch processing,
    and automatic message deletion on successful processing.

OPTIONS:
    --queue-url URL, -q URL       SQS queue URL (required)
    --polling-interval DURATION   Polling interval (default: 10s)
    --max-messages INT            Max messages per poll (default: 10, max: 10)
    --visibility-timeout DURATION Message visibility timeout (default: 30s)
    --wait-time INT               Long polling wait time (default: 20s, max: 20s)
    --dead-letter-queue URL       Dead letter queue URL for failed messages
    --max-retries INT             Max processing retries per message (default: 3)
    --batch-size INT              Batch processing size (default: 5)
    --stop-on-empty               Stop processing when queue is empty
    --message-retention DURATION  How long to retain processed message info

SQS MESSAGE FORMAT:
    {
      "messageType": "email_distribution",       // Required: Message type
      "changeId": "SQS-001",                    // Required: Unique identifier
      "title": "Newsletter Distribution",        // Required: Distribution title
      "description": "Monthly newsletter",       // Optional: Description
      "customerCodes": ["hts", "cds"],          // Required: Target customers
      "templateId": "newsletter",               // Required: Email template
      "priority": "normal",                     // Optional: Priority level
      "templateData": {                         // Required: Template variables
        "title": "Monthly Update",
        "message": "Newsletter content..."
      },
      "scheduledAt": "2024-12-15T10:00:00Z"    // Optional: Scheduled time
    }

EXAMPLES:
    # Basic SQS processing
    %s sqs --queue-url https://sqs.us-east-1.amazonaws.com/123456789012/email-queue

    # Custom polling configuration
    %s sqs --queue-url URL --polling-interval 5s --max-messages 5

    # With dead letter queue and retries
    %s sqs --queue-url URL --dead-letter-queue DLQ_URL --max-retries 5

    # Batch processing mode
    %s sqs --queue-url URL --batch-size 10 --stop-on-empty

MONITORING:
    The SQS processor provides metrics for:
    - Messages received and processed
    - Processing success/failure rates
    - Average processing time
    - Queue depth and message age
    - Error rates and retry counts

ERROR HANDLING:
    - Failed messages are retried up to max-retries times
    - Messages exceeding retry limit are sent to dead letter queue
    - Processing errors are logged with full context
    - Partial failures (some customers succeed) are handled gracefully

EXIT CODES:
    0    Graceful shutdown
    1    Processing error
    2    Configuration error
    3    AWS service error
    4    Queue access error

`, CLIName, CLIName, CLIName, CLIName, CLIName)
}

func showServerCommandHelp() {
	fmt.Printf(`SERVER COMMAND - Run as a server with health checks and metrics

USAGE:
    %s server [OPTIONS]

DESCRIPTION:
    Run the application in server mode with HTTP endpoints for health checks,
    metrics, and optional SQS processing. Designed for containerized deployments
    with proper observability and graceful shutdown handling.

OPTIONS:
    --health-port PORT            Health check endpoint port (default: 8080)
    --metrics-port PORT           Metrics endpoint port (default: 9090)
    --enable-metrics              Enable Prometheus metrics (default: true)
    --enable-tracing              Enable distributed tracing (default: false)
    --enable-sqs                  Enable SQS processing in server mode
    --queue-url URL               SQS queue URL (required if --enable-sqs)
    --shutdown-timeout DURATION   Graceful shutdown timeout (default: 30s)
    --read-timeout DURATION       HTTP read timeout (default: 10s)
    --write-timeout DURATION      HTTP write timeout (default: 10s)
    --idle-timeout DURATION       HTTP idle timeout (default: 60s)

ENDPOINTS:
    Health Check Endpoints:
      GET /health                 Overall health status
      GET /health/ready           Readiness probe
      GET /health/live            Liveness probe

    Metrics Endpoints:
      GET /metrics                Prometheus metrics
      GET /metrics/json           JSON formatted metrics

    Management Endpoints:
      GET /info                   Application information
      GET /config                 Configuration summary (sanitized)
      POST /shutdown              Graceful shutdown trigger

HEALTH CHECK RESPONSE:
    {
      "status": "healthy",
      "timestamp": "2024-12-15T10:00:00Z",
      "version": "1.0.0",
      "uptime": "5m30s",
      "checks": {
        "database": "healthy",
        "sqs": "healthy",
        "ses": "healthy",
        "memory": "healthy",
        "disk_space": "healthy"
      }
    }

METRICS:
    Application Metrics:
    - emails_sent_total           Total emails sent
    - emails_failed_total         Total email failures
    - customers_active            Number of active customers
    - processing_duration_seconds Processing time histogram
    - sqs_messages_processed      SQS messages processed
    - error_rate                  Error rate percentage

    System Metrics:
    - memory_usage_bytes          Memory usage
    - cpu_usage_percent           CPU utilization
    - disk_usage_bytes            Disk usage
    - goroutines_count            Active goroutines

EXAMPLES:
    # Basic server mode
    %s server

    # Custom ports
    %s server --health-port 8081 --metrics-port 9091

    # Server with SQS processing
    %s server --enable-sqs --queue-url https://sqs.us-east-1.amazonaws.com/123456789012/queue

    # Production configuration
    %s server --enable-metrics --enable-tracing --shutdown-timeout 60s

DOCKER DEPLOYMENT:
    # Dockerfile health check
    HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
      CMD curl -f http://localhost:8080/health || exit 1

    # Kubernetes probes
    livenessProbe:
      httpGet:
        path: /health/live
        port: 8080
    readinessProbe:
      httpGet:
        path: /health/ready
        port: 8080

GRACEFUL SHUTDOWN:
    1. Stop accepting new requests
    2. Complete in-flight SQS message processing
    3. Close database connections
    4. Flush metrics and logs
    5. Exit with code 0

EXIT CODES:
    0    Graceful shutdown
    1    Server startup error
    2    Configuration error
    3    Port binding error
    4    Service initialization error

`, CLIName, CLIName, CLIName, CLIName, CLIName)
}

func showConfigCommandHelp() {
	fmt.Printf(`CONFIG COMMAND - Configuration management

USAGE:
    %s config SUBCOMMAND [OPTIONS]

DESCRIPTION:
    Manage application configuration including customer mappings, email settings,
    and service configuration. Supports validation, generation, and migration.

SUBCOMMANDS:
    validate    Validate configuration file
    generate    Generate sample configuration
    migrate     Migrate configuration to new version
    show        Display current configuration (sanitized)
    test        Test configuration with dry-run

OPTIONS:
    --config FILE, -c FILE        Configuration file path
    --output FILE, -o FILE        Output file for generated config
    --format FORMAT               Output format (json, yaml) (default: json)
    --include-examples            Include example values in generated config
    --strict                      Enable strict validation mode

CONFIGURATION FILE STRUCTURE:
    {
      "version": "1.0.0",
      "environment": "production",
      "region": "us-east-1",
      "serviceSettings": {
        "serviceName": "multi-customer-email-distribution",
        "serviceVersion": "1.0.0",
        "logLevel": "info",
        "enableMetrics": true,
        "healthCheckPath": "/health"
      },
      "customerMappings": {
        "hts": {
          "customerName": "HTS Production",
          "awsAccountId": "123456789012",
          "region": "us-east-1",
          "environment": "production",
          "roleArns": {
            "ses": "arn:aws:iam::123456789012:role/HTSSESRole"
          },
          "emailSettings": {
            "fromEmail": "noreply@hts.example.com",
            "fromName": "HTS Team"
          }
        }
      },
      "emailSettings": {
        "defaultFromEmail": "noreply@system.example.com",
        "maxRetries": 3,
        "retryDelay": "2s"
      }
    }

EXAMPLES:
    # Validate configuration
    %s config validate --config config.json

    # Generate sample configuration
    %s config generate --output sample-config.json --include-examples

    # Test configuration
    %s config test --config config.json --strict

    # Show current configuration
    %s config show --config config.json

VALIDATION RULES:
    - Customer codes must be unique and non-empty
    - AWS account IDs must be valid 12-digit numbers
    - ARNs must follow proper AWS ARN format
    - Email addresses must be valid format
    - Regions must be valid AWS regions
    - Required fields must be present

EXIT CODES:
    0    Configuration valid
    1    Validation errors found
    2    File not found or permission error
    3    Invalid JSON/YAML format
    4    Missing required fields

`, CLIName, CLIName, CLIName, CLIName, CLIName)
}

func showValidateCommandHelp() {
	fmt.Printf(`VALIDATE COMMAND - Validate configuration and metadata files

USAGE:
    %s validate [OPTIONS]

DESCRIPTION:
    Validate configuration files, metadata files, and customer mappings.
    Performs comprehensive validation including format, required fields,
    AWS resource validation, and cross-reference checks.

OPTIONS:
    --config FILE                 Validate configuration file
    --metadata FILE               Validate metadata file
    --metadata-dir DIR            Validate all metadata files in directory
    --customer-mapping FILE       Validate customer mapping file
    --strict                      Enable strict validation mode
    --aws-validate                Validate AWS resources (requires credentials)
    --output-format FORMAT        Output format (text, json, junit) (default: text)
    --output-file FILE            Write validation results to file

VALIDATION TYPES:
    Configuration Validation:
    - JSON/YAML syntax validation
    - Required field presence
    - Data type validation
    - AWS ARN format validation
    - Email address format validation
    - Region and account ID validation

    Metadata Validation:
    - Customer code existence
    - Template ID validation
    - Email data completeness
    - Priority level validation
    - Scheduled time format validation

    AWS Resource Validation (--aws-validate):
    - IAM role existence and permissions
    - SES domain verification status
    - SQS queue accessibility
    - S3 bucket permissions

EXAMPLES:
    # Validate configuration only
    %s validate --config config.json

    # Validate metadata file
    %s validate --metadata newsletter.json

    # Validate all metadata in directory
    %s validate --metadata-dir ./metadata --strict

    # Full validation with AWS checks
    %s validate --config config.json --metadata-dir ./metadata --aws-validate

    # Output validation results to JUnit XML
    %s validate --config config.json --output-format junit --output-file results.xml

VALIDATION OUTPUT:
    Text Format:
    ✅ Configuration validation passed
    ❌ Metadata validation failed:
       - Missing required field: customer_codes
       - Invalid email format: invalid@email

    JSON Format:
    {
      "status": "failed",
      "errors": [
        {
          "type": "missing_field",
          "field": "customer_codes",
          "message": "Required field missing"
        }
      ],
      "warnings": [],
      "summary": {
        "total_files": 5,
        "passed": 3,
        "failed": 2
      }
    }

EXIT CODES:
    0    All validations passed
    1    Validation errors found
    2    File access errors
    3    AWS validation errors
    4    Invalid command options

`, CLIName, CLIName, CLIName, CLIName, CLIName)
}

func showStatusCommandHelp() {
	fmt.Printf(`STATUS COMMAND - Check execution status and history

USAGE:
    %s status [OPTIONS]

DESCRIPTION:
    Query and display execution status, history, and statistics for email
    distribution operations. Supports filtering, searching, and detailed
    execution information.

OPTIONS:
    --execution-id ID             Show specific execution details
    --customer CODE               Filter by customer code
    --status STATUS               Filter by status (pending, running, completed, failed)
    --since DURATION              Show executions since duration ago (e.g., 24h, 7d)
    --limit INT                   Limit number of results (default: 20)
    --format FORMAT               Output format (table, json, csv) (default: table)
    --follow                      Follow execution status in real-time
    --export FILE                 Export results to file

EXECUTION STATUSES:
    pending         Execution queued but not started
    running         Currently processing
    completed       Successfully completed all customers
    partial         Some customers succeeded, others failed
    failed          All customers failed
    cancelled       Execution was cancelled

EXAMPLES:
    # Show recent executions
    %s status

    # Show specific execution
    %s status --execution-id abc123

    # Filter by customer and status
    %s status --customer hts --status completed --limit 10

    # Show executions from last 24 hours
    %s status --since 24h

    # Follow execution in real-time
    %s status --execution-id abc123 --follow

    # Export to CSV
    %s status --since 7d --format csv --export executions.csv

OUTPUT FORMATS:
    Table Format:
    ID       Title                    Status      Customers  Started             Duration
    abc123   Monthly Newsletter       completed   3/3        2024-12-15 10:00:00 2m30s
    def456   System Alert            partial     2/3        2024-12-15 09:30:00 1m45s

    JSON Format:
    {
      "executions": [
        {
          "id": "abc123",
          "title": "Monthly Newsletter",
          "status": "completed",
          "startTime": "2024-12-15T10:00:00Z",
          "endTime": "2024-12-15T10:02:30Z",
          "duration": "2m30s",
          "customerStatuses": {
            "hts": {"status": "completed", "emailsSent": 150},
            "cds": {"status": "completed", "emailsSent": 89},
            "motor": {"status": "completed", "emailsSent": 45}
          }
        }
      ]
    }

REAL-TIME FOLLOWING:
    When using --follow, the command will continuously update the display
    with the latest execution status until completion or cancellation.

EXIT CODES:
    0    Status retrieved successfully
    1    Execution not found
    2    Database connection error
    3    Invalid filter parameters
    4    Export file error

`, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName)
}

// ShowVersion displays version information
func ShowVersion() {
	fmt.Printf(`%s version %s

Build Information:
  Go Version:    %s
  Git Commit:    %s
  Build Date:    %s
  Platform:      %s

`, CLIName, CLIVersion,
		"go1.21",
		"abc123def",
		"2024-12-15T10:00:00Z",
		"linux/amd64")
}

// ShowExamples displays comprehensive usage examples
func ShowExamples() {
	fmt.Printf(`%s - Usage Examples

BASIC USAGE:
    # Process a single metadata file
    %s file --input newsletter.json

    # Process all files in a directory
    %s file --input-dir ./metadata

    # Start SQS processing
    %s sqs --queue-url https://sqs.us-east-1.amazonaws.com/123456789012/email-queue

    # Run server mode
    %s server --health-port 8080 --metrics-port 9090

ADVANCED USAGE:
    # Process with custom configuration
    %s --config production.json file --input-dir ./metadata --parallel 10

    # SQS with dead letter queue
    %s sqs --queue-url MAIN_QUEUE --dead-letter-queue DLQ_URL --max-retries 5

    # Server with SQS processing
    %s server --enable-sqs --queue-url QUEUE_URL --enable-metrics

CONFIGURATION MANAGEMENT:
    # Generate sample configuration
    %s config generate --output config.json --include-examples

    # Validate configuration
    %s config validate --config config.json --strict

    # Test configuration with dry-run
    %s config test --config config.json

VALIDATION AND MONITORING:
    # Validate metadata files
    %s validate --metadata-dir ./metadata --aws-validate

    # Check execution status
    %s status --since 24h --customer hts

    # Follow execution in real-time
    %s status --execution-id abc123 --follow

DOCKER USAGE:
    # Build Docker image
    docker build -t email-distribution .

    # Run file processing
    docker run -v $(pwd)/metadata:/data email-distribution file --input-dir /data

    # Run server mode
    docker run -p 8080:8080 -p 9090:9090 email-distribution server

KUBERNETES DEPLOYMENT:
    # Deploy as a job for file processing
    kubectl create job email-dist --image=email-distribution -- file --input-dir /data

    # Deploy as a service for server mode
    kubectl create deployment email-dist --image=email-distribution
    kubectl expose deployment email-dist --port=8080 --type=LoadBalancer

ENVIRONMENT VARIABLES:
    AWS_REGION                    Default AWS region
    AWS_PROFILE                   AWS profile to use
    CONFIG_FILE                   Default configuration file path
    LOG_LEVEL                     Default log level
    MAX_CONCURRENT_CUSTOMERS      Default concurrency limit
    SQS_QUEUE_URL                Default SQS queue URL
    HEALTH_CHECK_PORT            Default health check port
    METRICS_PORT                 Default metrics port

METADATA FILE EXAMPLES:
    # Simple newsletter
    {
      "customer_codes": ["hts", "cds"],
      "change_id": "NEWSLETTER-001",
      "title": "Monthly Newsletter",
      "template_id": "newsletter",
      "email_data": {
        "subject": "December Newsletter",
        "message": "Monthly updates and news..."
      }
    }

    # Scheduled maintenance alert
    {
      "customer_codes": ["hts", "cds", "motor"],
      "change_id": "MAINT-001",
      "title": "Scheduled Maintenance",
      "template_id": "alert",
      "priority": "high",
      "email_data": {
        "subject": "Maintenance Window - Dec 15",
        "maintenance_window": "2:00 AM - 4:00 AM EST",
        "impact": "Email services temporarily unavailable"
      },
      "scheduled_time": "2024-12-15T06:00:00Z"
    }

TROUBLESHOOTING:
    # Enable debug logging
    %s --log-level debug file --input metadata.json

    # Dry run to test without execution
    %s --dry-run file --input metadata.json

    # Validate before processing
    %s validate --metadata metadata.json && %s file --input metadata.json

For more information, visit: https://github.com/your-org/multi-customer-email-distribution

`, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName, CLIName)
}

// ParseHelpRequest checks if help is requested and shows appropriate help
func ParseHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}

	// Check for global help flags
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			if len(args) > 1 && args[0] == "help" && len(args) > 1 {
				// Command-specific help: help COMMAND
				ShowCommandHelp(args[1])
			} else if len(args) > 2 && (args[1] == "--help" || args[1] == "-h") {
				// Command-specific help: COMMAND --help
				ShowCommandHelp(args[0])
			} else {
				// General help
				ShowHelp()
			}
			return true
		}
		if arg == "--version" || arg == "-v" {
			ShowVersion()
			return true
		}
		if arg == "--examples" {
			ShowExamples()
			return true
		}
	}

	return false
}

// ValidateCommand checks if the command is valid
func ValidateCommand(command string) bool {
	validCommands := []string{"file", "sqs", "server", "config", "validate", "status", "help", "version"}
	for _, valid := range validCommands {
		if strings.ToLower(command) == valid {
			return true
		}
	}
	return false
}

// SuggestCommand suggests similar commands for typos
func SuggestCommand(command string) []string {
	suggestions := []string{}
	validCommands := []string{"file", "sqs", "server", "config", "validate", "status"}

	command = strings.ToLower(command)

	// Simple similarity check
	for _, valid := range validCommands {
		if strings.Contains(valid, command) || strings.Contains(command, valid) {
			suggestions = append(suggestions, valid)
		}
	}

	// If no suggestions, return most common commands
	if len(suggestions) == 0 {
		suggestions = []string{"file", "sqs", "server"}
	}

	return suggestions
}

// ShowCommandNotFound displays error and suggestions for invalid commands
func ShowCommandNotFound(command string) {
	fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n\n", command)

	suggestions := SuggestCommand(command)
	if len(suggestions) > 0 {
		fmt.Fprintf(os.Stderr, "Did you mean one of these?\n")
		for _, suggestion := range suggestions {
			fmt.Fprintf(os.Stderr, "    %s\n", suggestion)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	fmt.Fprintf(os.Stderr, "Run '%s --help' for usage information.\n", CLIName)
	os.Exit(1)
}
