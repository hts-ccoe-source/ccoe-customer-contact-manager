package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Demo application for SQS message processing workflow

func main() {
	fmt.Println("=== SQS Message Processing Demo ===")

	// Demo 1: Single message processing
	fmt.Println("\n🧪 Demo 1: Single SQS Message Processing")
	demoSingleMessageProcessing()

	// Demo 2: Batch message processing
	fmt.Println("\n🧪 Demo 2: Batch SQS Message Processing")
	demoBatchMessageProcessing()

	// Demo 3: Error handling scenarios
	fmt.Println("\n🧪 Demo 3: Error Handling Scenarios")
	demoErrorHandling()

	// Demo 4: Multi-customer processing
	fmt.Println("\n🧪 Demo 4: Multi-Customer Processing")
	demoMultiCustomerProcessing()

	// Demo 5: Performance and statistics
	fmt.Println("\n🧪 Demo 5: Performance and Statistics")
	demoPerformanceStats()

	fmt.Println("\n=== Demo Complete ===")
}

func demoSingleMessageProcessing() {
	// Create processor for HTS customer
	processor := NewSQSMessageProcessor("hts", "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications")

	// Create a realistic SQS message
	message := createDemoSQSMessage(
		"msg-001",
		"customers/hts/deploy-monitoring-system-2025-09-20T15-30-00.json",
		"test-metadata-bucket",
	)

	fmt.Printf("📨 Processing SQS message: %s\n", message.MessageID)
	fmt.Printf("   S3 Object: %s\n", extractS3KeyFromMessage(message))

	// Process the message
	result, err := processor.ProcessSQSMessage(message)
	if err != nil {
		log.Printf("❌ Processing failed: %v", err)
		return
	}

	// Display results
	fmt.Printf("✅ Processing successful!\n")
	fmt.Printf("   Customer: %s\n", result.CustomerCode)
	fmt.Printf("   Change ID: %s\n", extractChangeIDFromS3Key(extractS3KeyFromMessage(message)))
	fmt.Printf("   Emails sent: %d\n", result.EmailsSent)
	fmt.Printf("   Processing time: %v\n", result.ProcessingTime)
	fmt.Printf("   Metadata extracted: %t\n", result.MetadataExtracted)
}

func demoBatchMessageProcessing() {
	processor := NewSQSMessageProcessor("cds", "https://sqs.us-east-1.amazonaws.com/234567890123/cds-notifications")

	// Create batch of messages
	messages := []*SQSMessage{
		createDemoSQSMessage("batch-001", "customers/cds/security-update-2025-09-20T10-00-00.json", "metadata-bucket"),
		createDemoSQSMessage("batch-002", "customers/cds/network-maintenance-2025-09-20T14-00-00.json", "metadata-bucket"),
		createDemoSQSMessage("batch-003", "customers/cds/software-deployment-2025-09-20T16-00-00.json", "metadata-bucket"),
	}

	fmt.Printf("📦 Processing batch of %d messages for customer: %s\n", len(messages), processor.CustomerCode)

	// Process batch
	results, err := processor.ProcessMessageBatch(messages)
	if err != nil {
		log.Printf("❌ Batch processing failed: %v", err)
		return
	}

	// Display batch results
	successCount := 0
	totalEmails := 0

	for i, result := range results {
		status := "✅"
		if !result.Success {
			status = "❌"
		} else {
			successCount++
		}
		totalEmails += result.EmailsSent

		fmt.Printf("   %s Message %d: %s (%d emails, %v)\n",
			status, i+1, result.MessageID, result.EmailsSent, result.ProcessingTime)
	}

	fmt.Printf("📊 Batch Summary: %d/%d successful, %d total emails sent\n",
		successCount, len(messages), totalEmails)
}

func demoErrorHandling() {
	processor := NewSQSMessageProcessor("motor", "https://sqs.us-east-1.amazonaws.com/345678901234/motor-notifications")

	// Test various error scenarios
	errorScenarios := []struct {
		name    string
		message *SQSMessage
	}{
		{
			name:    "Wrong customer code",
			message: createDemoSQSMessage("error-001", "customers/hts/change-123.json", "bucket"),
		},
		{
			name:    "Invalid S3 key format",
			message: createDemoSQSMessage("error-002", "invalid-key-format", "bucket"),
		},
		{
			name:    "Non-JSON file",
			message: createDemoSQSMessage("error-003", "customers/motor/change-123.txt", "bucket"),
		},
		{
			name:    "Archive prefix (not customer)",
			message: createDemoSQSMessage("error-004", "archive/change-123.json", "bucket"),
		},
	}

	fmt.Printf("🔍 Testing error handling scenarios for customer: %s\n", processor.CustomerCode)

	for _, scenario := range errorScenarios {
		fmt.Printf("   Testing: %s\n", scenario.name)

		result, err := processor.ProcessSQSMessage(scenario.message)

		if err != nil {
			fmt.Printf("      ❌ Expected error caught: %v\n", err)
		} else if !result.Success {
			fmt.Printf("      ❌ Processing failed as expected: %s\n", result.Error)
		} else {
			fmt.Printf("      ⚠️  Unexpected success - this should have failed!\n")
		}
	}
}

func demoMultiCustomerProcessing() {
	// Simulate processing for multiple customers
	customers := []struct {
		code     string
		queueURL string
	}{
		{"hts", "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications"},
		{"cds", "https://sqs.us-east-1.amazonaws.com/234567890123/cds-notifications"},
		{"motor", "https://sqs.us-east-1.amazonaws.com/345678901234/motor-notifications"},
		{"bat", "https://sqs.us-east-1.amazonaws.com/456789012345/bat-notifications"},
	}

	fmt.Printf("🏢 Simulating multi-customer processing for %d customers\n", len(customers))

	allResults := make(map[string][]*ProcessingResult)

	for _, customer := range customers {
		processor := NewSQSMessageProcessor(customer.code, customer.queueURL)

		// Create customer-specific message
		message := createDemoSQSMessage(
			fmt.Sprintf("%s-msg-001", customer.code),
			fmt.Sprintf("customers/%s/multi-customer-change-2025-09-20T15-30-00.json", customer.code),
			"multi-customer-bucket",
		)

		fmt.Printf("   Processing for %s (%s)...\n",
			getCustomerDisplayName(customer.code), customer.code)

		result, err := processor.ProcessSQSMessage(message)
		if err != nil {
			fmt.Printf("      ❌ Failed: %v\n", err)
			continue
		}

		allResults[customer.code] = []*ProcessingResult{result}
		fmt.Printf("      ✅ Success: %d emails sent in %v\n",
			result.EmailsSent, result.ProcessingTime)
	}

	// Generate overall statistics
	totalMessages := 0
	totalEmails := 0
	successfulCustomers := 0

	for customerCode, results := range allResults {
		stats := NewSQSMessageProcessor(customerCode, "").GetProcessingStats(results)
		totalMessages += stats["totalMessages"].(int)
		totalEmails += stats["totalEmailsSent"].(int)
		if stats["successfulMessages"].(int) > 0 {
			successfulCustomers++
		}
	}

	fmt.Printf("📈 Multi-Customer Summary:\n")
	fmt.Printf("   Customers processed: %d/%d\n", successfulCustomers, len(customers))
	fmt.Printf("   Total messages: %d\n", totalMessages)
	fmt.Printf("   Total emails sent: %d\n", totalEmails)
}

func demoPerformanceStats() {
	processor := NewSQSMessageProcessor("icx", "https://sqs.us-east-1.amazonaws.com/567890123456/icx-notifications")

	// Create a larger batch for performance testing
	batchSize := 10
	messages := make([]*SQSMessage, batchSize)

	for i := 0; i < batchSize; i++ {
		messages[i] = createDemoSQSMessage(
			fmt.Sprintf("perf-test-%03d", i+1),
			fmt.Sprintf("customers/icx/performance-test-%d-2025-09-20T15-30-%02d.json", i+1, i),
			"performance-test-bucket",
		)
	}

	fmt.Printf("⚡ Performance testing with batch of %d messages\n", batchSize)

	startTime := time.Now()
	results, err := processor.ProcessMessageBatch(messages)
	totalTime := time.Since(startTime)

	if err != nil {
		log.Printf("❌ Performance test failed: %v", err)
		return
	}

	// Generate detailed statistics
	stats := processor.GetProcessingStats(results)

	fmt.Printf("📊 Performance Results:\n")
	fmt.Printf("   Total processing time: %v\n", totalTime)
	fmt.Printf("   Average per message: %v\n", totalTime/time.Duration(batchSize))
	fmt.Printf("   Messages per second: %.2f\n", float64(batchSize)/totalTime.Seconds())
	fmt.Printf("   Success rate: %.1f%%\n", stats["successRate"])
	fmt.Printf("   Total emails sent: %d\n", stats["totalEmailsSent"])
	fmt.Printf("   Average processing time: %s\n", stats["averageProcessingTime"])

	// Show distribution of processing times
	fmt.Printf("📈 Processing Time Distribution:\n")
	for i, result := range results {
		status := "✅"
		if !result.Success {
			status = "❌"
		}
		fmt.Printf("   Message %2d: %s %v\n", i+1, status, result.ProcessingTime)
	}
}

// Helper functions for demo

func createDemoSQSMessage(messageID, s3Key, bucketName string) *SQSMessage {
	eventTime := time.Now()

	s3Event := S3EventRecord{
		EventVersion: "2.1",
		EventSource:  "aws:s3",
		EventTime:    eventTime,
		EventName:    "ObjectCreated:Put",
		S3: S3Event{
			S3SchemaVersion: "1.0",
			ConfigurationID: "demo-config",
			Bucket: S3Bucket{
				Name: bucketName,
				ARN:  fmt.Sprintf("arn:aws:s3:::%s", bucketName),
			},
			Object: S3Object{
				Key:  s3Key,
				Size: 2048,
				ETag: fmt.Sprintf("demo-etag-%s", messageID),
			},
		},
	}

	messageBody := SQSMessageBody{
		Records: []S3EventRecord{s3Event},
	}

	bodyJSON, _ := json.Marshal(messageBody)

	return &SQSMessage{
		MessageID:     messageID,
		ReceiptHandle: fmt.Sprintf("demo-receipt-%s", messageID),
		Body:          string(bodyJSON),
		Attributes: map[string]interface{}{
			"SentTimestamp": fmt.Sprintf("%d", eventTime.Unix()*1000),
		},
		MessageAttributes: map[string]interface{}{
			"CustomerCode": map[string]interface{}{
				"StringValue": extractCustomerFromS3Key(s3Key),
				"DataType":    "String",
			},
		},
	}
}

func extractS3KeyFromMessage(message *SQSMessage) string {
	var messageBody SQSMessageBody
	if err := json.Unmarshal([]byte(message.Body), &messageBody); err != nil {
		return "unknown"
	}

	if len(messageBody.Records) == 0 {
		return "unknown"
	}

	return messageBody.Records[0].S3.Object.Key
}

func extractCustomerFromS3Key(s3Key string) string {
	parts := strings.Split(s3Key, "/")
	if len(parts) >= 2 && parts[0] == "customers" {
		return parts[1]
	}
	return "unknown"
}

// Include the required types from the main implementation
// (These would normally be imported from a package)

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

// Include the required functions from the main implementation

func NewSQSMessageProcessor(customerCode, queueURL string) *SQSMessageProcessor {
	return &SQSMessageProcessor{
		CustomerCode: customerCode,
		QueueURL:     queueURL,
	}
}

func (p *SQSMessageProcessor) ProcessSQSMessage(message *SQSMessage) (*ProcessingResult, error) {
	startTime := time.Now()

	result := &ProcessingResult{
		MessageID:    message.MessageID,
		CustomerCode: p.CustomerCode,
		ProcessedAt:  startTime,
		Success:      false,
	}

	// Simulate processing
	time.Sleep(50 * time.Millisecond)

	// Extract S3 key and validate
	s3Key := extractS3KeyFromMessage(message)
	if err := p.ValidateCustomerFromS3Key(s3Key); err != nil {
		result.Error = err.Error()
		result.ProcessingTime = time.Since(startTime)
		return result, err
	}

	// Simulate successful processing
	result.Success = true
	result.MetadataExtracted = true
	result.EmailsSent = 1 + (int(time.Now().UnixNano()) % 3) // 1-3 emails
	result.ProcessingTime = time.Since(startTime)

	return result, nil
}

func (p *SQSMessageProcessor) ProcessMessageBatch(messages []*SQSMessage) ([]*ProcessingResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to process")
	}

	results := make([]*ProcessingResult, len(messages))

	for i, message := range messages {
		result, _ := p.ProcessSQSMessage(message)
		results[i] = result
	}

	return results, nil
}

func (p *SQSMessageProcessor) ValidateCustomerFromS3Key(s3Key string) error {
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

func (p *SQSMessageProcessor) GetProcessingStats(results []*ProcessingResult) map[string]interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"totalMessages":         0,
			"successfulMessages":    0,
			"failedMessages":        0,
			"totalEmailsSent":       0,
			"averageProcessingTime": "0s",
			"successRate":           0.0,
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

func extractChangeIDFromS3Key(s3Key string) string {
	parts := strings.Split(s3Key, "/")
	if len(parts) < 3 {
		return "unknown"
	}

	filename := parts[len(parts)-1]
	filename = strings.TrimSuffix(filename, ".json")

	dashIndex := strings.LastIndex(filename, "-")
	if dashIndex > 0 {
		return filename[:dashIndex]
	}

	return filename
}

func getCustomerDisplayName(customerCode string) string {
	customerMapping := map[string]string{
		"hts":   "HTS Prod",
		"cds":   "CDS Global",
		"motor": "Motor",
		"bat":   "Bring A Trailer",
		"icx":   "iCrossing",
	}

	if displayName, exists := customerMapping[customerCode]; exists {
		return displayName
	}

	return strings.ToUpper(customerCode)
}
