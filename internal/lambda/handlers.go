package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"aws-alternate-contact-manager/internal/config"
	"aws-alternate-contact-manager/internal/types"
)

// Handler handles SQS events from Lambda
func Handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	log.Printf("Received %d SQS messages", len(sqsEvent.Records))

	// Load configuration
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	// Process each SQS message
	var processingErrors []error
	successCount := 0

	for i, record := range sqsEvent.Records {
		log.Printf("Processing message %d/%d: %s", i+1, len(sqsEvent.Records), record.MessageId)

		err := ProcessSQSRecord(ctx, record, cfg)
		if err != nil {
			log.Printf("Error processing message %s: %v", record.MessageId, err)
			processingErrors = append(processingErrors, fmt.Errorf("message %s: %w", record.MessageId, err))
		} else {
			successCount++
			log.Printf("Successfully processed message %s", record.MessageId)
		}
	}

	// Log summary
	log.Printf("Processing complete: %d successful, %d errors", successCount, len(processingErrors))

	// If any messages failed, return error (Lambda will retry)
	if len(processingErrors) > 0 {
		return fmt.Errorf("failed to process %d messages: %v", len(processingErrors), processingErrors[0])
	}

	return nil
}

// ProcessSQSRecord processes a single SQS record from Lambda
func ProcessSQSRecord(ctx context.Context, record events.SQSMessage, cfg *types.Config) error {
	// Log the raw message for debugging
	log.Printf("Processing SQS message: %s", record.Body)

	// Check if this is an S3 test event and skip it
	if IsS3TestEvent(record.Body) {
		log.Printf("Skipping S3 test event")
		return nil
	}

	// Parse the message body
	var messageBody interface{}
	if err := json.Unmarshal([]byte(record.Body), &messageBody); err != nil {
		return fmt.Errorf("failed to parse message body: %w", err)
	}

	// Check if it's an S3 event notification
	var s3Event types.S3EventNotification
	if err := json.Unmarshal([]byte(record.Body), &s3Event); err == nil {
		log.Printf("Successfully parsed S3 event, Records count: %d", len(s3Event.Records))
		if len(s3Event.Records) > 0 {
			log.Printf("Processing as S3 event notification with %d records", len(s3Event.Records))
			for i, rec := range s3Event.Records {
				log.Printf("Record %d: EventSource=%s, S3.Bucket.Name=%s, S3.Object.Key=%s",
					i, rec.EventSource, rec.S3.Bucket.Name, rec.S3.Object.Key)
			}
			// Process as S3 event
			return ProcessS3Event(ctx, s3Event, cfg)
		} else {
			log.Printf("S3 event parsed successfully but has no records")
		}
	} else {
		log.Printf("Failed to parse as S3 event: %v", err)
	}

	// Try to parse as legacy SQS message
	var sqsMsg types.SQSMessage
	if err := json.Unmarshal([]byte(record.Body), &sqsMsg); err == nil && sqsMsg.CustomerCode != "" {
		log.Printf("Processing as legacy SQS message for customer: %s", sqsMsg.CustomerCode)
		// Process as legacy SQS message
		return ProcessSQSMessage(ctx, sqsMsg, cfg)
	} else {
		log.Printf("Failed to parse as legacy SQS message: %v", err)
	}

	log.Printf("Message body type: %T, content: %+v", messageBody, messageBody)
	return fmt.Errorf("unrecognized message format")
}

// IsS3TestEvent checks if the message is an S3 test event
func IsS3TestEvent(messageBody string) bool {
	// Check for S3 test event patterns
	return strings.Contains(messageBody, `"Event": "s3:TestEvent"`) ||
		strings.Contains(messageBody, `"Service": "Amazon S3"`) && strings.Contains(messageBody, `"s3:TestEvent"`)
}

// ProcessS3Event processes an S3 event notification in Lambda context
func ProcessS3Event(ctx context.Context, s3Event types.S3EventNotification, cfg *types.Config) error {
	for _, record := range s3Event.Records {
		if record.EventSource != "aws:s3" {
			log.Printf("Skipping non-S3 event: %s", record.EventSource)
			continue
		}

		bucketName := record.S3.Bucket.Name
		objectKey := record.S3.Object.Key

		log.Printf("Processing S3 event: s3://%s/%s", bucketName, objectKey)

		// Extract customer code from S3 key
		customerCode, err := ExtractCustomerCodeFromS3Key(objectKey)
		if err != nil {
			return fmt.Errorf("failed to extract customer code from S3 key %s: %w", objectKey, err)
		}

		// Validate customer code
		if err := ValidateCustomerCode(customerCode, cfg); err != nil {
			return fmt.Errorf("invalid customer code %s: %w", customerCode, err)
		}

		// Download and process metadata
		metadata, err := DownloadMetadataFromS3(ctx, bucketName, objectKey, cfg.AWSRegion)
		if err != nil {
			return fmt.Errorf("failed to download metadata from S3: %w", err)
		}

		// Process the change request
		err = ProcessChangeRequest(ctx, customerCode, metadata, cfg)
		if err != nil {
			return fmt.Errorf("failed to process change request: %w", err)
		}

		log.Printf("Successfully processed S3 event for customer %s", customerCode)
	}

	return nil
}

// ProcessSQSMessage processes a legacy SQS message in Lambda context
func ProcessSQSMessage(ctx context.Context, sqsMsg types.SQSMessage, cfg *types.Config) error {
	// Validate the message
	if err := ValidateSQSMessage(sqsMsg); err != nil {
		return fmt.Errorf("invalid SQS message: %w", err)
	}

	// Validate customer code
	if err := ValidateCustomerCode(sqsMsg.CustomerCode, cfg); err != nil {
		return fmt.Errorf("invalid customer code %s: %w", sqsMsg.CustomerCode, err)
	}

	// Download metadata from S3
	metadata, err := DownloadMetadataFromS3(ctx, sqsMsg.S3Bucket, sqsMsg.S3Key, cfg.AWSRegion)
	if err != nil {
		return fmt.Errorf("failed to download metadata from S3: %w", err)
	}

	// Process the change request
	err = ProcessChangeRequest(ctx, sqsMsg.CustomerCode, metadata, cfg)
	if err != nil {
		return fmt.Errorf("failed to process change request: %w", err)
	}

	log.Printf("Successfully processed legacy SQS message for customer %s", sqsMsg.CustomerCode)
	return nil
}

// ExtractCustomerCodeFromS3Key extracts customer code from S3 object key
func ExtractCustomerCodeFromS3Key(s3Key string) (string, error) {
	// Expected format: customers/{customer-code}/filename.json
	parts := strings.Split(s3Key, "/")
	if len(parts) < 2 || parts[0] != "customers" {
		return "", fmt.Errorf("invalid S3 key format, expected customers/{customer-code}/filename.json")
	}
	return parts[1], nil
}

// ValidateCustomerCode validates that a customer code exists in the configuration
func ValidateCustomerCode(customerCode string, cfg *types.Config) error {
	if customerCode == "" {
		return fmt.Errorf("customer code cannot be empty")
	}

	if _, exists := cfg.CustomerMappings[customerCode]; !exists {
		return fmt.Errorf("customer code %s not found in configuration", customerCode)
	}

	return nil
}

// ValidateSQSMessage validates the structure of an SQS message
func ValidateSQSMessage(msg types.SQSMessage) error {
	if msg.CustomerCode == "" {
		return fmt.Errorf("customer_code is required")
	}
	if msg.S3Bucket == "" {
		return fmt.Errorf("s3_bucket is required")
	}
	if msg.S3Key == "" {
		return fmt.Errorf("s3_key is required")
	}
	return nil
}

// DownloadMetadataFromS3 downloads and parses metadata from S3
func DownloadMetadataFromS3(ctx context.Context, bucket, key, region string) (*types.ChangeMetadata, error) {
	// Create S3 client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	// Download object
	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download S3 object: %w", err)
	}
	defer result.Body.Close()

	// Parse metadata
	var metadata types.ChangeMetadata
	if err := json.NewDecoder(result.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	return &metadata, nil
}

// ProcessChangeRequest processes a change request with metadata
func ProcessChangeRequest(ctx context.Context, customerCode string, metadata *types.ChangeMetadata, cfg *types.Config) error {
	log.Printf("Processing change request %s for customer %s", metadata.ChangeID, customerCode)

	// Create change details for email notification (same as SQS processor)
	changeDetails := map[string]interface{}{
		"change_id":            metadata.ChangeID,
		"title":                metadata.Title,
		"description":          metadata.Description,
		"implementation_plan":  metadata.ImplementationPlan,
		"schedule_start":       metadata.Schedule.StartDate,
		"schedule_end":         metadata.Schedule.EndDate,
		"impact":               metadata.Impact,
		"rollback_plan":        metadata.RollbackPlan,
		"communication_plan":   metadata.CommunicationPlan,
		"approver":             metadata.Approver,
		"implementer":          metadata.Implementer,
		"timestamp":            metadata.Timestamp,
		"source":               metadata.Source,
		"test_run":             metadata.TestRun,
		"customers":            metadata.Customers,
		"security_updated":     true,
		"billing_updated":      true,
		"operations_updated":   true,
		"processing_timestamp": time.Now(),
	}

	// Add any additional metadata
	if metadata.Metadata != nil {
		for key, value := range metadata.Metadata {
			changeDetails[fmt.Sprintf("meta_%s", key)] = value
		}
	}

	// Send notification email with full change details
	// TODO: Implement email notification when email manager is available
	log.Printf("Would send notification email for customer %s with change details: %+v", customerCode, changeDetails)

	// If this is not a test run, we could also update alternate contacts here
	if !metadata.TestRun {
		log.Printf("Processing non-test change request - would update alternate contacts for customer %s", customerCode)
		// TODO: Add actual alternate contact update logic here if needed
	} else {
		log.Printf("Test run - skipping alternate contact updates for customer %s", customerCode)
	}

	return nil
}

// StartLambdaMode starts the Lambda handler
func StartLambdaMode() {
	lambda.Start(Handler)
}

// SQSProcessor handles SQS message processing
type SQSProcessor struct {
	queueURL          string
	credentialManager CredentialManager
	emailManager      EmailManager
	sqsClient         interface{} // Will be *sqs.Client in real implementation
	s3Client          interface{} // Will be *s3.Client in real implementation
}

// CredentialManager interface for dependency injection
type CredentialManager interface {
	GetCustomerConfig(customerCode string) (aws.Config, error)
	GetCustomerInfo(customerCode string) (types.CustomerAccountInfo, error)
}

// EmailManager interface for dependency injection
type EmailManager interface {
	SendAlternateContactNotification(customerCode string, changeDetails map[string]interface{}) error
}

// NewSQSProcessor creates a new SQS processor
func NewSQSProcessor(queueURL string, credentialManager CredentialManager, emailManager EmailManager, region string) (*SQSProcessor, error) {
	return &SQSProcessor{
		queueURL:          queueURL,
		credentialManager: credentialManager,
		emailManager:      emailManager,
	}, nil
}

// ProcessMessages processes messages from the SQS queue
func (sp *SQSProcessor) ProcessMessages(ctx context.Context) error {
	fmt.Printf("Starting SQS message processing from queue: %s\n", sp.queueURL)

	// This is a simplified implementation for the integration
	// The full implementation would include message polling and processing
	return fmt.Errorf("SQS processing not fully implemented in internal package yet")
}
