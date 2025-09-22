package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSProcessor handles SQS message processing
type SQSProcessor struct {
	queueURL          string
	credentialManager *CredentialManager
	emailManager      *EmailManager
	sqsClient         *sqs.Client
}

// NewSQSProcessor creates a new SQS processor
func NewSQSProcessor(queueURL string, credentialManager *CredentialManager, emailManager *EmailManager, region string) (*SQSProcessor, error) {
	// Use base AWS config for SQS client (assuming queue is in the same account)
	cfg := credentialManager.baseConfig

	sqsClient := sqs.NewFromConfig(cfg)

	return &SQSProcessor{
		queueURL:          queueURL,
		credentialManager: credentialManager,
		emailManager:      emailManager,
		sqsClient:         sqsClient,
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

	// Parse the message body
	var sqsMsg SQSMessage
	if err := json.Unmarshal([]byte(*message.Body), &sqsMsg); err != nil {
		return fmt.Errorf("failed to parse message body: %v", err)
	}

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

	log.Printf("Successfully processed message for customer %s", sqsMsg.CustomerCode)
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
