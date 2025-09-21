package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// SQSMessageProcessor handles processing of SQS messages containing embedded metadata
type SQSMessageProcessor struct {
	CustomerCode string
	QueueURL     string
	SQSClient    interface{} // Would be *sqs.SQS in real implementation
}

// SQSMessage represents an SQS message structure
type SQSMessage struct {
	MessageID         string                 `json:"messageId"`
	ReceiptHandle     string                 `json:"receiptHandle"`
	Body              string                 `json:"body"`
	Attributes        map[string]interface{} `json:"attributes"`
	MessageAttributes map[string]interface{} `json:"messageAttributes"`
}

// S3EventRecord represents an S3 event record within an SQS message
type S3EventRecord struct {
	EventVersion string    `json:"eventVersion"`
	EventSource  string    `json:"eventSource"`
	EventTime    time.Time `json:"eventTime"`
	EventName    string    `json:"eventName"`
	S3           S3Event   `json:"s3"`
}

// S3Event represents the S3 portion of an event record
type S3Event struct {
	S3SchemaVersion string   `json:"s3SchemaVersion"`
	ConfigurationID string   `json:"configurationId"`
	Bucket          S3Bucket `json:"bucket"`
	Object          S3Object `json:"object"`
}

// S3Bucket represents S3 bucket information
type S3Bucket struct {
	Name          string `json:"name"`
	OwnerIdentity struct {
		PrincipalID string `json:"principalId"`
	} `json:"ownerIdentity"`
	ARN string `json:"arn"`
}

// S3Object represents S3 object information
type S3Object struct {
	Key       string `json:"key"`
	Size      int64  `json:"size"`
	ETag      string `json:"eTag"`
	Sequencer string `json:"sequencer"`
}

// SQSMessageBody represents the parsed SQS message body
type SQSMessageBody struct {
	Records []S3EventRecord `json:"Records"`
}

// ProcessingResult represents the result of processing an SQS message
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

// NewSQSMessageProcessor creates a new SQS message processor
func NewSQSMessageProcessor(customerCode, queueURL string) *SQSMessageProcessor {
	return &SQSMessageProcessor{
		CustomerCode: customerCode,
		QueueURL:     queueURL,
		// SQSClient would be initialized here in real implementation
	}
}

// ProcessSQSMessage processes a single SQS message with embedded metadata
func (p *SQSMessageProcessor) ProcessSQSMessage(message *SQSMessage) (*ProcessingResult, error) {
	startTime := time.Now()

	result := &ProcessingResult{
		MessageID:    message.MessageID,
		CustomerCode: p.CustomerCode,
		ProcessedAt:  startTime,
		Success:      false,
	}

	// Step 1: Parse SQS message body
	log.Printf("Processing SQS message %s for customer %s", message.MessageID, p.CustomerCode)

	messageBody, err := p.ParseSQSMessageBody(message.Body)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to parse SQS message body: %v", err)
		result.ProcessingTime = time.Since(startTime)
		return result, err
	}

	// Step 2: Extract S3 event information
	if len(messageBody.Records) == 0 {
		err := fmt.Errorf("no S3 event records found in SQS message")
		result.Error = err.Error()
		result.ProcessingTime = time.Since(startTime)
		return result, err
	}

	s3Record := messageBody.Records[0] // Process first record
	log.Printf("Processing S3 event: %s for object %s", s3Record.EventName, s3Record.S3.Object.Key)

	// Step 3: Validate customer code from S3 key
	if err := p.ValidateCustomerFromS3Key(s3Record.S3.Object.Key); err != nil {
		result.Error = fmt.Sprintf("Customer validation failed: %v", err)
		result.ProcessingTime = time.Since(startTime)
		return result, err
	}

	// Step 4: Extract embedded metadata (simulated - in real implementation would download from S3)
	metadata, err := p.ExtractEmbeddedMetadata(s3Record)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to extract metadata: %v", err)
		result.ProcessingTime = time.Since(startTime)
		return result, err
	}

	result.MetadataExtracted = true
	log.Printf("Successfully extracted metadata for change ID: %s", metadata.ChangeID)

	// Step 5: Process email notifications using existing CLI functions
	emailCount, err := p.ProcessEmailNotifications(metadata)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to process email notifications: %v", err)
		result.ProcessingTime = time.Since(startTime)
		return result, err
	}

	result.EmailsSent = emailCount
	result.Success = true
	result.ProcessingTime = time.Since(startTime)

	log.Printf("Successfully processed SQS message %s: %d emails sent in %v",
		message.MessageID, emailCount, result.ProcessingTime)

	return result, nil
}

// ParseSQSMessageBody parses the JSON body of an SQS message
func (p *SQSMessageProcessor) ParseSQSMessageBody(body string) (*SQSMessageBody, error) {
	var messageBody SQSMessageBody

	if err := json.Unmarshal([]byte(body), &messageBody); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SQS message body: %v", err)
	}

	return &messageBody, nil
}

// ValidateCustomerFromS3Key validates that the S3 key matches the expected customer
func (p *SQSMessageProcessor) ValidateCustomerFromS3Key(s3Key string) error {
	// Expected format: customers/{customer-code}/filename.json
	parts := strings.Split(s3Key, "/")

	if len(parts) < 3 {
		return fmt.Errorf("invalid S3 key format: %s", s3Key)
	}

	if parts[0] != "customers" {
		return fmt.Errorf("S3 key does not start with 'customers/': %s", s3Key)
	}

	keyCustomerCode := parts[1]
	if keyCustomerCode != p.CustomerCode {
		return fmt.Errorf("S3 key customer code '%s' does not match processor customer code '%s'",
			keyCustomerCode, p.CustomerCode)
	}

	if !strings.HasSuffix(s3Key, ".json") {
		return fmt.Errorf("S3 key does not end with '.json': %s", s3Key)
	}

	return nil
}

// ChangeMetadata represents the structure of change metadata
type ChangeMetadata struct {
	ChangeID          string            `json:"changeId"`
	Version           int               `json:"version"`
	CreatedAt         string            `json:"createdAt"`
	ModifiedAt        string            `json:"modifiedAt"`
	CreatedBy         string            `json:"createdBy"`
	Status            string            `json:"status"`
	ChangeMetadata    ChangeDetails     `json:"changeMetadata"`
	EmailNotification EmailNotification `json:"emailNotification"`
}

type ChangeDetails struct {
	Title         string   `json:"title"`
	CustomerNames []string `json:"customerNames"`
	CustomerCodes []string `json:"customerCodes"`
	ChangeReason  string   `json:"changeReason"`
}

type EmailNotification struct {
	Subject string `json:"subject"`
}

// ExtractEmbeddedMetadata extracts metadata from S3 event (simulated)
func (p *SQSMessageProcessor) ExtractEmbeddedMetadata(s3Record S3EventRecord) (*ChangeMetadata, error) {
	// In real implementation, this would download the file from S3
	// For now, we'll simulate with sample metadata

	log.Printf("Extracting metadata from S3 object: s3://%s/%s",
		s3Record.S3.Bucket.Name, s3Record.S3.Object.Key)

	// Simulate metadata extraction
	metadata := &ChangeMetadata{
		ChangeID:   extractChangeIDFromS3Key(s3Record.S3.Object.Key),
		Version:    1,
		CreatedAt:  s3Record.EventTime.Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
		CreatedBy:  "sqs-processor",
		Status:     "processing",
		ChangeMetadata: ChangeDetails{
			Title:         "SQS-triggered Change Processing",
			CustomerCodes: []string{p.CustomerCode},
			CustomerNames: []string{getCustomerDisplayName(p.CustomerCode)},
			ChangeReason:  "Processing change notification from SQS message",
		},
		EmailNotification: EmailNotification{
			Subject: fmt.Sprintf("Change Notification for %s", getCustomerDisplayName(p.CustomerCode)),
		},
	}

	return metadata, nil
}

// ProcessEmailNotifications processes email notifications using existing CLI functions
func (p *SQSMessageProcessor) ProcessEmailNotifications(metadata *ChangeMetadata) (int, error) {
	log.Printf("Processing email notifications for change %s affecting customer %s",
		metadata.ChangeID, p.CustomerCode)

	// In real implementation, this would call existing CLI email functions
	// For now, we'll simulate the email processing

	emailCount := 0

	// Simulate different email types based on metadata
	if metadata.EmailNotification.Subject != "" {
		// Send change notification email
		log.Printf("Sending change notification email: %s", metadata.EmailNotification.Subject)
		emailCount++
	}

	// Simulate additional emails based on change type
	if strings.Contains(strings.ToLower(metadata.ChangeMetadata.Title), "approval") {
		log.Printf("Sending approval request email for change %s", metadata.ChangeID)
		emailCount++
	}

	if strings.Contains(strings.ToLower(metadata.ChangeMetadata.Title), "meeting") {
		log.Printf("Sending meeting invite for change %s", metadata.ChangeID)
		emailCount++
	}

	// Simulate processing delay
	time.Sleep(100 * time.Millisecond)

	return emailCount, nil
}

// ValidateSQSMessage validates the structure and content of an SQS message
func (p *SQSMessageProcessor) ValidateSQSMessage(message *SQSMessage) error {
	if message == nil {
		return fmt.Errorf("SQS message is nil")
	}

	if message.MessageID == "" {
		return fmt.Errorf("SQS message missing MessageID")
	}

	if message.Body == "" {
		return fmt.Errorf("SQS message missing Body")
	}

	// Validate that body contains S3 event structure
	var testBody SQSMessageBody
	if err := json.Unmarshal([]byte(message.Body), &testBody); err != nil {
		return fmt.Errorf("SQS message body is not valid JSON: %v", err)
	}

	if len(testBody.Records) == 0 {
		return fmt.Errorf("SQS message body contains no S3 event records")
	}

	// Validate S3 event structure
	record := testBody.Records[0]
	if record.EventSource != "aws:s3" {
		return fmt.Errorf("SQS message does not contain S3 event (eventSource: %s)", record.EventSource)
	}

	if !strings.HasPrefix(record.EventName, "ObjectCreated:") {
		return fmt.Errorf("SQS message does not contain ObjectCreated event (eventName: %s)", record.EventName)
	}

	return nil
}

// ProcessMessageBatch processes multiple SQS messages in batch
func (p *SQSMessageProcessor) ProcessMessageBatch(messages []*SQSMessage) ([]*ProcessingResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to process")
	}

	log.Printf("Processing batch of %d SQS messages for customer %s", len(messages), p.CustomerCode)

	results := make([]*ProcessingResult, len(messages))
	successCount := 0

	for i, message := range messages {
		result, err := p.ProcessSQSMessage(message)
		results[i] = result

		if err != nil {
			log.Printf("Failed to process message %s: %v", message.MessageID, err)
		} else {
			successCount++
		}
	}

	log.Printf("Batch processing complete: %d/%d messages processed successfully",
		successCount, len(messages))

	return results, nil
}

// GetProcessingStats returns statistics about message processing
func (p *SQSMessageProcessor) GetProcessingStats(results []*ProcessingResult) map[string]interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"totalMessages":         0,
			"successfulMessages":    0,
			"failedMessages":        0,
			"totalEmailsSent":       0,
			"averageProcessingTime": "0s",
		}
	}

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

	avgProcessingTime := totalProcessingTime / time.Duration(len(results))

	return map[string]interface{}{
		"totalMessages":         len(results),
		"successfulMessages":    successCount,
		"failedMessages":        len(results) - successCount,
		"totalEmailsSent":       totalEmails,
		"averageProcessingTime": avgProcessingTime.String(),
		"successRate":           float64(successCount) / float64(len(results)) * 100,
	}
}

// Helper functions

func extractChangeIDFromS3Key(s3Key string) string {
	// Extract change ID from S3 key like: customers/hts/changeId-timestamp.json
	parts := strings.Split(s3Key, "/")
	if len(parts) < 3 {
		return "unknown"
	}

	filename := parts[len(parts)-1]
	// Remove .json extension
	filename = strings.TrimSuffix(filename, ".json")

	// Extract change ID (everything before the last dash)
	dashIndex := strings.LastIndex(filename, "-")
	if dashIndex > 0 {
		return filename[:dashIndex]
	}

	return filename
}

func getCustomerDisplayName(customerCode string) string {
	customerMapping := map[string]string{
		"hts":        "HTS Prod",
		"cds":        "CDS Global",
		"motor":      "Motor",
		"bat":        "Bring A Trailer",
		"icx":        "iCrossing",
		"fdbus":      "FDBUS",
		"hmiit":      "Hearst Magazines Italy",
		"hmies":      "Hearst Magazines Spain",
		"htvdigital": "HTV Digital",
		"htv":        "HTV",
	}

	if displayName, exists := customerMapping[customerCode]; exists {
		return displayName
	}

	return strings.ToUpper(customerCode)
}
