package main

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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// LambdaHandler handles SQS events from Lambda
func LambdaHandler(ctx context.Context, sqsEvent events.SQSEvent) error {
	log.Printf("Received %d SQS messages", len(sqsEvent.Records))

	// Load configuration
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	// Initialize credential and email managers
	credentialManager, err := NewCredentialManager(config.AWSRegion, config.CustomerMappings)
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %v", err)
	}

	emailManager := NewEmailManager(credentialManager, config.ContactConfig)

	// Process each SQS message
	var processingErrors []error
	successCount := 0

	for i, record := range sqsEvent.Records {
		log.Printf("Processing message %d/%d: %s", i+1, len(sqsEvent.Records), record.MessageId)

		err := processLambdaSQSRecord(ctx, record, credentialManager, emailManager, config)
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

// processLambdaSQSRecord processes a single SQS record from Lambda
func processLambdaSQSRecord(ctx context.Context, record events.SQSMessage, credentialManager *CredentialManager, emailManager *EmailManager, config *Config) error {
	// Log the raw message for debugging
	log.Printf("Processing SQS message: %s", record.Body)

	// Check if this is an S3 test event and skip it
	if isS3TestEvent(record.Body) {
		log.Printf("Skipping S3 test event")
		return nil
	}

	// Parse the message body
	var messageBody interface{}
	if err := json.Unmarshal([]byte(record.Body), &messageBody); err != nil {
		return fmt.Errorf("failed to parse message body: %w", err)
	}

	// Check if it's an S3 event notification
	var s3Event S3EventNotification
	if err := json.Unmarshal([]byte(record.Body), &s3Event); err == nil {
		log.Printf("Successfully parsed S3 event, Records count: %d", len(s3Event.Records))
		if len(s3Event.Records) > 0 {
			log.Printf("Processing as S3 event notification with %d records", len(s3Event.Records))
			for i, rec := range s3Event.Records {
				log.Printf("Record %d: EventSource=%s, S3.Bucket.Name=%s, S3.Object.Key=%s",
					i, rec.EventSource, rec.S3.Bucket.Name, rec.S3.Object.Key)
			}
			// Process as S3 event
			return processLambdaS3Event(ctx, s3Event, credentialManager, emailManager, config)
		} else {
			log.Printf("S3 event parsed successfully but has no records")
		}
	} else {
		log.Printf("Failed to parse as S3 event: %v", err)
	}

	// Try to parse as legacy SQS message
	var sqsMsg SQSMessage
	if err := json.Unmarshal([]byte(record.Body), &sqsMsg); err == nil && sqsMsg.CustomerCode != "" {
		log.Printf("Processing as legacy SQS message for customer: %s", sqsMsg.CustomerCode)
		// Process as legacy SQS message
		return processLambdaSQSMessage(ctx, sqsMsg, credentialManager, emailManager, config)
	} else {
		log.Printf("Failed to parse as legacy SQS message: %v", err)
	}

	log.Printf("Message body type: %T, content: %+v", messageBody, messageBody)
	return fmt.Errorf("unrecognized message format")
}

// isS3TestEvent checks if the message is an S3 test event
func isS3TestEvent(messageBody string) bool {
	// Check for S3 test event patterns
	return strings.Contains(messageBody, `"Event": "s3:TestEvent"`) ||
		strings.Contains(messageBody, `"Service": "Amazon S3"`) && strings.Contains(messageBody, `"s3:TestEvent"`)
}

// processLambdaS3Event processes an S3 event notification in Lambda context
func processLambdaS3Event(ctx context.Context, s3Event S3EventNotification, credentialManager *CredentialManager, emailManager *EmailManager, config *Config) error {
	for _, record := range s3Event.Records {
		if record.EventSource != "aws:s3" {
			log.Printf("Skipping non-S3 event: %s", record.EventSource)
			continue
		}

		bucketName := record.S3.Bucket.Name
		objectKey := record.S3.Object.Key

		log.Printf("Processing S3 event: s3://%s/%s", bucketName, objectKey)

		// Extract customer code from S3 key
		customerCode, err := extractCustomerCodeFromS3Key(objectKey)
		if err != nil {
			return fmt.Errorf("failed to extract customer code from S3 key %s: %w", objectKey, err)
		}

		// Validate customer code
		if err := validateCustomerCode(customerCode, credentialManager); err != nil {
			return fmt.Errorf("invalid customer code %s: %w", customerCode, err)
		}

		// Download and process metadata
		metadata, err := downloadMetadataFromS3(ctx, bucketName, objectKey, config.AWSRegion)
		if err != nil {
			return fmt.Errorf("failed to download metadata from S3: %w", err)
		}

		// Process the change request
		err = processChangeRequest(ctx, customerCode, metadata, credentialManager, emailManager)
		if err != nil {
			return fmt.Errorf("failed to process change request: %w", err)
		}

		log.Printf("Successfully processed S3 event for customer %s", customerCode)
	}

	return nil
}

// processLambdaSQSMessage processes a legacy SQS message in Lambda context
func processLambdaSQSMessage(ctx context.Context, sqsMsg SQSMessage, credentialManager *CredentialManager, emailManager *EmailManager, config *Config) error {
	// Validate the message
	if err := validateSQSMessage(sqsMsg); err != nil {
		return fmt.Errorf("invalid SQS message: %w", err)
	}

	// Validate customer code
	if err := validateCustomerCode(sqsMsg.CustomerCode, credentialManager); err != nil {
		return fmt.Errorf("invalid customer code %s: %w", sqsMsg.CustomerCode, err)
	}

	// Download metadata from S3
	metadata, err := downloadMetadataFromS3(ctx, sqsMsg.S3Bucket, sqsMsg.S3Key, config.AWSRegion)
	if err != nil {
		return fmt.Errorf("failed to download metadata from S3: %w", err)
	}

	// Process the change request
	err = processChangeRequest(ctx, sqsMsg.CustomerCode, metadata, credentialManager, emailManager)
	if err != nil {
		return fmt.Errorf("failed to process change request: %w", err)
	}

	log.Printf("Successfully processed legacy SQS message for customer %s", sqsMsg.CustomerCode)
	return nil
}

// Helper functions (extracted from existing SQS processor)

func extractCustomerCodeFromS3Key(s3Key string) (string, error) {
	// Expected format: customers/{customer-code}/filename.json
	parts := strings.Split(s3Key, "/")
	if len(parts) < 2 || parts[0] != "customers" {
		return "", fmt.Errorf("invalid S3 key format, expected customers/{customer-code}/filename.json")
	}
	return parts[1], nil
}

func validateCustomerCode(customerCode string, credentialManager *CredentialManager) error {
	_, err := credentialManager.GetCustomerInfo(customerCode)
	if err != nil {
		return fmt.Errorf("customer code not found: %s", customerCode)
	}
	return nil
}

func validateSQSMessage(msg SQSMessage) error {
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

func downloadMetadataFromS3(ctx context.Context, bucket, key, region string) (*ChangeMetadata, error) {
	// Create S3 client
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)

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
	var metadata ChangeMetadata
	if err := json.NewDecoder(result.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	return &metadata, nil
}

func processChangeRequest(ctx context.Context, customerCode string, metadata *ChangeMetadata, credentialManager *CredentialManager, emailManager *EmailManager) error {
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
	if err := emailManager.SendAlternateContactNotification(customerCode, changeDetails); err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}

	// If this is not a test run, we could also update alternate contacts here
	if !metadata.TestRun {
		log.Printf("Processing non-test change request - would update alternate contacts for customer %s", customerCode)
		// TODO: Add actual alternate contact update logic here if needed
	} else {
		log.Printf("Test run - skipping alternate contact updates for customer %s", customerCode)
	}

	return nil
}

// runLambdaMode starts the Lambda handler
func runLambdaMode() {
	lambda.Start(LambdaHandler)
}
