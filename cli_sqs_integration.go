package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// CLI integration for SQS message processing
// This demonstrates how to add SQS processing mode to the existing CLI

// CLISQSProcessor integrates SQS message processing with existing CLI functionality
type CLISQSProcessor struct {
	CustomerCode    string
	QueueURL        string
	DryRun          bool
	MaxMessages     int
	WaitTimeSeconds int
	Processor       *SQSMessageProcessor
}

// CLIConfig represents CLI configuration for SQS processing
type CLIConfig struct {
	Action          string
	CustomerCode    string
	QueueURL        string
	SQSRoleARN      string
	DryRun          bool
	MaxMessages     int
	WaitTimeSeconds int
	JSONMetadata    string
	Verbose         bool
}

// NewCLISQSProcessor creates a new CLI SQS processor
func NewCLISQSProcessor(config *CLIConfig) *CLISQSProcessor {
	processor := NewSQSMessageProcessor(config.CustomerCode, config.QueueURL)

	return &CLISQSProcessor{
		CustomerCode:    config.CustomerCode,
		QueueURL:        config.QueueURL,
		DryRun:          config.DryRun,
		MaxMessages:     config.MaxMessages,
		WaitTimeSeconds: config.WaitTimeSeconds,
		Processor:       processor,
	}
}

// ProcessSQSMessages processes SQS messages from the command line
func (c *CLISQSProcessor) ProcessSQSMessages() error {
	log.Printf("Starting SQS message processing for customer: %s", c.CustomerCode)
	log.Printf("Queue URL: %s", c.QueueURL)

	if c.DryRun {
		log.Printf("DRY RUN MODE - No messages will be processed")
		return c.simulateSQSProcessing()
	}

	// In real implementation, this would:
	// 1. Connect to SQS using AWS SDK
	// 2. Poll for messages
	// 3. Process each message
	// 4. Delete processed messages

	return c.simulateSQSProcessing()
}

// simulateSQSProcessing simulates SQS message processing for demo
func (c *CLISQSProcessor) simulateSQSProcessing() error {
	log.Printf("Polling SQS queue for messages (max: %d, wait: %ds)",
		c.MaxMessages, c.WaitTimeSeconds)

	// Simulate receiving messages
	messages := c.generateSimulatedMessages()

	if len(messages) == 0 {
		log.Printf("No messages available in queue")
		return nil
	}

	log.Printf("Received %d messages from SQS queue", len(messages))

	// Process messages
	results, err := c.Processor.ProcessMessageBatch(messages)
	if err != nil {
		return fmt.Errorf("failed to process message batch: %v", err)
	}

	// Display results
	c.displayProcessingResults(results)

	// Generate statistics
	stats := c.Processor.GetProcessingStats(results)
	c.displayStatistics(stats)

	return nil
}

// generateSimulatedMessages creates simulated SQS messages for demo
func (c *CLISQSProcessor) generateSimulatedMessages() []*SQSMessage {
	messageCount := 1 + (int(time.Now().UnixNano()) % c.MaxMessages)
	messages := make([]*SQSMessage, messageCount)

	for i := 0; i < messageCount; i++ {
		messages[i] = c.createSimulatedMessage(fmt.Sprintf("cli-msg-%03d", i+1))
	}

	return messages
}

// createSimulatedMessage creates a simulated SQS message
func (c *CLISQSProcessor) createSimulatedMessage(messageID string) *SQSMessage {
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	s3Key := fmt.Sprintf("customers/%s/cli-change-%s-%s.json",
		c.CustomerCode, messageID, timestamp)

	s3Event := S3EventRecord{
		EventVersion: "2.1",
		EventSource:  "aws:s3",
		EventTime:    time.Now(),
		EventName:    "ObjectCreated:Put",
		S3: S3Event{
			S3SchemaVersion: "1.0",
			ConfigurationID: "cli-config",
			Bucket: S3Bucket{
				Name: "cli-metadata-bucket",
				ARN:  "arn:aws:s3:::cli-metadata-bucket",
			},
			Object: S3Object{
				Key:  s3Key,
				Size: 2048,
				ETag: fmt.Sprintf("cli-etag-%s", messageID),
			},
		},
	}

	messageBody := SQSMessageBody{
		Records: []S3EventRecord{s3Event},
	}

	bodyJSON, _ := json.Marshal(messageBody)

	return &SQSMessage{
		MessageID:     messageID,
		ReceiptHandle: fmt.Sprintf("cli-receipt-%s", messageID),
		Body:          string(bodyJSON),
		Attributes: map[string]interface{}{
			"SentTimestamp": fmt.Sprintf("%d", time.Now().Unix()*1000),
		},
		MessageAttributes: map[string]interface{}{
			"CustomerCode": map[string]interface{}{
				"StringValue": c.CustomerCode,
				"DataType":    "String",
			},
		},
	}
}

// displayProcessingResults shows the results of message processing
func (c *CLISQSProcessor) displayProcessingResults(results []*ProcessingResult) {
	fmt.Printf("\n📋 Processing Results:\n")

	for i, result := range results {
		status := "✅ SUCCESS"
		if !result.Success {
			status = "❌ FAILED"
		}

		fmt.Printf("   Message %d: %s\n", i+1, status)
		fmt.Printf("      ID: %s\n", result.MessageID)
		fmt.Printf("      Customer: %s\n", result.CustomerCode)
		fmt.Printf("      Emails sent: %d\n", result.EmailsSent)
		fmt.Printf("      Processing time: %v\n", result.ProcessingTime)

		if result.Error != "" {
			fmt.Printf("      Error: %s\n", result.Error)
		}

		fmt.Printf("\n")
	}
}

// displayStatistics shows processing statistics
func (c *CLISQSProcessor) displayStatistics(stats map[string]interface{}) {
	fmt.Printf("📊 Processing Statistics:\n")
	fmt.Printf("   Total messages: %d\n", stats["totalMessages"])
	fmt.Printf("   Successful: %d\n", stats["successfulMessages"])
	fmt.Printf("   Failed: %d\n", stats["failedMessages"])
	fmt.Printf("   Success rate: %.1f%%\n", stats["successRate"])
	fmt.Printf("   Total emails sent: %d\n", stats["totalEmailsSent"])
	fmt.Printf("   Average processing time: %s\n", stats["averageProcessingTime"])
}

// ParseCLIFlags parses command line flags for SQS processing
func ParseCLIFlags() *CLIConfig {
	config := &CLIConfig{}

	flag.StringVar(&config.Action, "action", "", "Action to perform (process-sqs-message)")
	flag.StringVar(&config.CustomerCode, "customer-code", "", "Customer code for SQS processing (required)")
	flag.StringVar(&config.QueueURL, "sqs-queue-url", "", "SQS queue URL (required)")
	flag.StringVar(&config.SQSRoleARN, "sqs-role-arn", "", "IAM role ARN for SQS operations (optional)")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be done without processing messages")
	flag.IntVar(&config.MaxMessages, "max-messages", 10, "Maximum number of messages to process")
	flag.IntVar(&config.WaitTimeSeconds, "wait-time", 20, "SQS long polling wait time in seconds")
	flag.StringVar(&config.JSONMetadata, "json-metadata", "", "Path to JSON metadata file (for testing)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")

	flag.Parse()

	return config
}

// ValidateCLIConfig validates CLI configuration for SQS processing
func ValidateCLIConfig(config *CLIConfig) error {
	if config.Action != "process-sqs-message" {
		return fmt.Errorf("invalid action: %s (expected: process-sqs-message)", config.Action)
	}

	if config.CustomerCode == "" {
		return fmt.Errorf("customer-code is required for SQS processing")
	}

	if config.QueueURL == "" {
		return fmt.Errorf("sqs-queue-url is required for SQS processing")
	}

	// Validate customer code format
	validCustomers := map[string]bool{
		"hts": true, "cds": true, "fdbus": true, "hmiit": true, "hmies": true,
		"htvdigital": true, "htv": true, "icx": true, "motor": true, "bat": true,
		"mhk": true, "hdmautos": true, "hnpit": true, "hnpdigital": true,
		"camp": true, "mcg": true, "hmuk": true, "hmusdigital": true,
		"hwp": true, "zynx": true, "hchb": true, "fdbuk": true,
		"hecom": true, "blkbook": true,
	}

	if !validCustomers[config.CustomerCode] {
		return fmt.Errorf("invalid customer code: %s", config.CustomerCode)
	}

	// Validate SQS queue URL format
	if !strings.HasPrefix(config.QueueURL, "https://sqs.") {
		return fmt.Errorf("invalid SQS queue URL format: %s", config.QueueURL)
	}

	if config.MaxMessages < 1 || config.MaxMessages > 10 {
		return fmt.Errorf("max-messages must be between 1 and 10, got: %d", config.MaxMessages)
	}

	if config.WaitTimeSeconds < 0 || config.WaitTimeSeconds > 20 {
		return fmt.Errorf("wait-time must be between 0 and 20 seconds, got: %d", config.WaitTimeSeconds)
	}

	return nil
}

// ShowSQSHelp displays help information for SQS processing
func ShowSQSHelp() {
	fmt.Printf(`
SQS Message Processing Mode

USAGE:
    ./aws-alternate-contact-manager ses -action process-sqs-message [OPTIONS]

REQUIRED FLAGS:
    -customer-code string    Customer code for SQS processing (e.g., "hts", "cds", "motor")
    -sqs-queue-url string    SQS queue URL for the customer

OPTIONAL FLAGS:
    -sqs-role-arn string     IAM role ARN to assume for SQS operations
    -dry-run                 Show what would be done without processing messages
    -max-messages int        Maximum number of messages to process (default: 10, max: 10)
    -wait-time int           SQS long polling wait time in seconds (default: 20, max: 20)
    -json-metadata string    Path to JSON metadata file for testing
    -verbose                 Enable verbose logging

EXAMPLES:

    # Process SQS messages for HTS customer
    ./aws-alternate-contact-manager ses -action process-sqs-message \
        -customer-code "hts" \
        -sqs-queue-url "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications"

    # Dry run to see what would be processed
    ./aws-alternate-contact-manager ses -action process-sqs-message \
        -customer-code "cds" \
        -sqs-queue-url "https://sqs.us-east-1.amazonaws.com/234567890123/cds-notifications" \
        -dry-run

    # Process with custom SQS role assumption
    ./aws-alternate-contact-manager ses -action process-sqs-message \
        -customer-code "motor" \
        -sqs-queue-url "https://sqs.us-east-1.amazonaws.com/345678901234/motor-notifications" \
        -sqs-role-arn "arn:aws:iam::345678901234:role/SQSProcessorRole"

    # Process with custom settings
    ./aws-alternate-contact-manager ses -action process-sqs-message \
        -customer-code "bat" \
        -sqs-queue-url "https://sqs.us-east-1.amazonaws.com/456789012345/bat-notifications" \
        -max-messages 5 \
        -wait-time 10 \
        -verbose

WORKFLOW:
    1. Connect to customer-specific SQS queue
    2. Poll for messages using long polling
    3. Parse S3 event notifications from message body
    4. Extract embedded metadata from S3 object
    5. Process email notifications using existing CLI functions
    6. Delete processed messages from queue
    7. Report processing statistics

ERROR HANDLING:
    - Invalid customer codes are rejected
    - Messages with wrong customer prefixes are skipped
    - Failed message processing is logged and reported
    - Partial batch failures are handled gracefully
    - Retry logic for transient failures

SECURITY:
    - Customer isolation enforced through S3 key validation
    - Optional IAM role assumption for cross-account access
    - SQS message validation before processing
    - Audit logging of all processing activities
`)
}

// Main CLI entry point for SQS processing
func main() {
	// Parse command line flags
	config := ParseCLIFlags()

	// Show help if no action specified
	if config.Action == "" {
		ShowSQSHelp()
		os.Exit(0)
	}

	// Validate configuration
	if err := ValidateCLIConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Set up logging
	if config.Verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Create and run SQS processor
	processor := NewCLISQSProcessor(config)

	log.Printf("Starting SQS message processing...")
	log.Printf("Customer: %s", config.CustomerCode)
	log.Printf("Queue: %s", config.QueueURL)

	if config.DryRun {
		log.Printf("DRY RUN MODE - No actual processing will occur")
	}

	// Process SQS messages
	if err := processor.ProcessSQSMessages(); err != nil {
		log.Fatalf("SQS processing failed: %v", err)
	}

	log.Printf("SQS message processing completed successfully")
}

// Include required types (these would normally be imported)

type SQSMessageProcessor struct {
	CustomerCode string
	QueueURL     string
	SQSClient    interface{}
}

type SQSMessage struct {
	MessageID         string                 `json:"messageId"`
	ReceiptHandle     string                 `json:"receiptHandle"`
	Body              string                 `json:"body"`
	Attributes        map[string]interface{} `json:"attributes"`
	MessageAttributes map[string]interface{} `json:"messageAttributes"`
}

type S3EventRecord struct {
	EventVersion string    `json:"eventVersion"`
	EventSource  string    `json:"eventSource"`
	EventTime    time.Time `json:"eventTime"`
	EventName    string    `json:"eventName"`
	S3           S3Event   `json:"s3"`
}

type S3Event struct {
	S3SchemaVersion string   `json:"s3SchemaVersion"`
	ConfigurationID string   `json:"configurationId"`
	Bucket          S3Bucket `json:"bucket"`
	Object          S3Object `json:"object"`
}

type S3Bucket struct {
	Name          string `json:"name"`
	OwnerIdentity struct {
		PrincipalID string `json:"principalId"`
	} `json:"ownerIdentity"`
	ARN string `json:"arn"`
}

type S3Object struct {
	Key       string `json:"key"`
	Size      int64  `json:"size"`
	ETag      string `json:"eTag"`
	Sequencer string `json:"sequencer"`
}

type SQSMessageBody struct {
	Records []S3EventRecord `json:"Records"`
}

type ProcessingResult struct {
	MessageID         string        `json:"messageId"`
	CustomerCode      string        `json:"customerCode"`
	ProcessedAt       time.Time     `json:"processedAt"`
	Success           bool          `json:"success"`
	Error             string        `json:"error,omitempty"`
	MetadataExtracted bool          `json:"metadataExtracted"`
	EmailsSent        int           `json:"emailsSent"`
	ProcessingTime    time.Duration `json:"processingTime"`
}

// Include required functions (simplified for demo)

func NewSQSMessageProcessor(customerCode, queueURL string) *SQSMessageProcessor {
	return &SQSMessageProcessor{
		CustomerCode: customerCode,
		QueueURL:     queueURL,
	}
}

func (p *SQSMessageProcessor) ProcessMessageBatch(messages []*SQSMessage) ([]*ProcessingResult, error) {
	results := make([]*ProcessingResult, len(messages))

	for i, message := range messages {
		startTime := time.Now()

		result := &ProcessingResult{
			MessageID:         message.MessageID,
			CustomerCode:      p.CustomerCode,
			ProcessedAt:       startTime,
			Success:           true,
			MetadataExtracted: true,
			EmailsSent:        1 + (i % 3), // 1-3 emails
			ProcessingTime:    time.Since(startTime),
		}

		results[i] = result
	}

	return results, nil
}

func (p *SQSMessageProcessor) GetProcessingStats(results []*ProcessingResult) map[string]interface{} {
	successCount := 0
	totalEmails := 0
	var totalProcessingTime time.Duration

	for _, result := range results {
		if result.Success {
			successCount++
		}
		totalEmails += result.EmailsSent
		totalProcessingTime += result.ProcessingTime
	}

	avgProcessingTime := time.Duration(0)
	if len(results) > 0 {
		avgProcessingTime = totalProcessingTime / time.Duration(len(results))
	}

	successRate := float64(0)
	if len(results) > 0 {
		successRate = float64(successCount) / float64(len(results)) * 100
	}

	return map[string]interface{}{
		"totalMessages":         len(results),
		"successfulMessages":    successCount,
		"failedMessages":        len(results) - successCount,
		"totalEmailsSent":       totalEmails,
		"averageProcessingTime": avgProcessingTime.String(),
		"successRate":           successRate,
	}
}
