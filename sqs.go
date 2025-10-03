package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSProcessor handles SQS message processing
type SQSProcessor struct {
	queueURL          string
	credentialManager *CredentialManager
	emailManager      *EmailManager
	sqsClient         *sqs.Client
	s3Client          *s3.Client
}

// NewSQSProcessor creates a new SQS processor
func NewSQSProcessor(queueURL string, credentialManager *CredentialManager, emailManager *EmailManager, region string) (*SQSProcessor, error) {
	// Use base AWS config for SQS client (assuming queue is in the same account)
	cfg := credentialManager.baseConfig

	sqsClient := sqs.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	return &SQSProcessor{
		queueURL:          queueURL,
		credentialManager: credentialManager,
		emailManager:      emailManager,
		sqsClient:         sqsClient,
		s3Client:          s3Client,
	}, nil
}

// ProcessMessages processes messages from the SQS queue
func (sp *SQSProcessor) ProcessMessages(ctx context.Context) error {
	log.Printf("Starting SQS message processing from queue: %s", sp.queueURL)

	for {
		select {
		case <-ctx.Done():
			log.Println("SQS processing stopped")
			return ctx.Err()
		default:
			if err := sp.processMessageBatch(ctx); err != nil {
				log.Printf("Error processing message batch: %v", err)
				// Continue processing despite errors
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// processMessageBatch processes a batch of messages from SQS
func (sp *SQSProcessor) processMessageBatch(ctx context.Context) error {
	// Receive messages from SQS
	result, err := sp.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(sp.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     20, // Long polling
		VisibilityTimeout:   30,
	})
	if err != nil {
		return fmt.Errorf("failed to receive messages: %v", err)
	}

	if len(result.Messages) == 0 {
		// No messages, continue polling
		return nil
	}

	log.Printf("Received %d messages from SQS", len(result.Messages))

	// Process each message
	for _, message := range result.Messages {
		if err := sp.processMessage(ctx, message); err != nil {
			log.Printf("Failed to process message %s: %v", *message.MessageId, err)
			// Don't delete the message if processing failed
			continue
		}

		// Delete the message after successful processing
		if err := sp.deleteMessage(ctx, message); err != nil {
			log.Printf("Failed to delete message %s: %v", *message.MessageId, err)
		}
	}

	return nil
}

// processMessage processes a single SQS message
func (sp *SQSProcessor) processMessage(ctx context.Context, message types.Message) error {
	log.Printf("Processing message: %s", *message.MessageId)

	// Try to parse as S3 event notification first
	var s3Event S3EventNotification
	if err := json.Unmarshal([]byte(*message.Body), &s3Event); err == nil && len(s3Event.Records) > 0 {
		return sp.processS3Event(ctx, s3Event)
	}

	// Fallback to legacy SQS message format
	var sqsMsg SQSMessage
	if err := json.Unmarshal([]byte(*message.Body), &sqsMsg); err != nil {
		return fmt.Errorf("failed to parse message body as S3 event or SQS message: %v", err)
	}

	return sp.processLegacySQSMessage(ctx, sqsMsg)
}

// processS3Event processes an S3 event notification
func (sp *SQSProcessor) processS3Event(ctx context.Context, s3Event S3EventNotification) error {
	for _, record := range s3Event.Records {
		if record.EventSource != "aws:s3" {
			log.Printf("Skipping non-S3 event: %s", record.EventSource)
			continue
		}

		// Extract customer code from S3 key path
		customerCode, err := sp.extractCustomerCodeFromS3Key(record.S3.Object.Key)
		if err != nil {
			log.Printf("Failed to extract customer code from S3 key %s: %v", record.S3.Object.Key, err)
			continue
		}

		log.Printf("Processing S3 event for customer %s: %s", customerCode, record.S3.Object.Key)

		// Download and parse the metadata file from S3
		metadata, err := sp.downloadMetadataFromS3(ctx, record.S3.Bucket.Name, record.S3.Object.Key)
		if err != nil {
			return fmt.Errorf("failed to download metadata from S3: %v", err)
		}

		// Validate customer exists
		if err := sp.validateCustomerCode(customerCode); err != nil {
			return fmt.Errorf("invalid customer: %v", err)
		}

		// Process the change request
		if err := sp.processChangeRequest(ctx, customerCode, metadata); err != nil {
			return fmt.Errorf("failed to process change request: %v", err)
		}

		log.Printf("Successfully processed S3 event for customer %s", customerCode)
	}

	return nil
}

// processLegacySQSMessage processes legacy SQS message format
func (sp *SQSProcessor) processLegacySQSMessage(ctx context.Context, sqsMsg SQSMessage) error {
	// Validate the message
	if err := sp.validateMessage(sqsMsg); err != nil {
		return fmt.Errorf("invalid message: %v", err)
	}

	// Process the alternate contact update
	changeDetails := map[string]interface{}{
		"security_updated":   true,
		"billing_updated":    true,
		"operations_updated": true,
		"timestamp":          time.Now(),
		"source":             "sqs",
	}

	// Add metadata from the message
	for key, value := range sqsMsg.Metadata {
		changeDetails[key] = value
	}

	// Send notification email
	if err := sp.emailManager.SendAlternateContactNotification(sqsMsg.CustomerCode, changeDetails); err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}

	log.Printf("Successfully processed legacy SQS message for customer %s", sqsMsg.CustomerCode)
	return nil
}

// extractCustomerCodeFromS3Key extracts customer code from S3 object key
func (sp *SQSProcessor) extractCustomerCodeFromS3Key(s3Key string) (string, error) {
	// Expected format: customers/{customer-code}/filename.json
	parts := strings.Split(s3Key, "/")
	if len(parts) < 3 || parts[0] != "customers" {
		return "", fmt.Errorf("invalid S3 key format, expected customers/{customer-code}/filename.json, got: %s", s3Key)
	}

	customerCode := parts[1]
	if customerCode == "" {
		return "", fmt.Errorf("empty customer code in S3 key: %s", s3Key)
	}

	return customerCode, nil
}

// downloadMetadataFromS3 downloads and parses metadata file from S3
func (sp *SQSProcessor) downloadMetadataFromS3(ctx context.Context, bucket, key string) (*ChangeMetadata, error) {
	log.Printf("Downloading metadata from S3: s3://%s/%s", bucket, key)

	// Download the object from S3
	result, err := sp.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download object from S3: %v", err)
	}
	defer result.Body.Close()

	// Read the content
	content, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object content: %v", err)
	}

	// Parse the JSON metadata
	var metadata ChangeMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %v", err)
	}

	log.Printf("Successfully downloaded and parsed metadata: ChangeID=%s, Title=%s", metadata.ChangeID, metadata.Title)
	return &metadata, nil
}

// processChangeRequest processes a change request with full metadata
func (sp *SQSProcessor) processChangeRequest(ctx context.Context, customerCode string, metadata *ChangeMetadata) error {
	log.Printf("Processing change request %s for customer %s", metadata.ChangeID, customerCode)

	// Create change details for email notification
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
	if err := sp.emailManager.SendAlternateContactNotification(customerCode, changeDetails); err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}

	// If this is not a test run, we could also update alternate contacts here
	if !metadata.TestRun {
		log.Printf("Processing non-test change request - would update alternate contacts for customer %s", customerCode)
		// TODO: Add actual alternate contact update logic here if needed
		// if err := UpdateAlternateContacts(customerCode, sp.credentialManager, contactConfig); err != nil {
		//     return fmt.Errorf("failed to update alternate contacts: %v", err)
		// }
	} else {
		log.Printf("Test run detected - skipping alternate contact updates")
	}

	return nil
}

// validateCustomerCode validates a customer code
func (sp *SQSProcessor) validateCustomerCode(customerCode string) error {
	_, err := sp.credentialManager.GetCustomerInfo(customerCode)
	if err != nil {
		return fmt.Errorf("invalid customer: %v", err)
	}
	return nil
}

// validateMessage validates an SQS message
func (sp *SQSProcessor) validateMessage(msg SQSMessage) error {
	if msg.CustomerCode == "" {
		return fmt.Errorf("customer_code is required")
	}

	// Validate customer exists
	_, err := sp.credentialManager.GetCustomerInfo(msg.CustomerCode)
	if err != nil {
		return fmt.Errorf("invalid customer: %v", err)
	}

	return nil
}

// deleteMessage deletes a message from SQS
func (sp *SQSProcessor) deleteMessage(ctx context.Context, message types.Message) error {
	_, err := sp.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(sp.queueURL),
		ReceiptHandle: message.ReceiptHandle,
	})
	return err
}

// SendMessage sends a message to the SQS queue (for testing)
func (sp *SQSProcessor) SendMessage(ctx context.Context, msg SQSMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	_, err = sp.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(sp.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	log.Printf("Message sent to SQS for customer %s", msg.CustomerCode)
	return nil
}
