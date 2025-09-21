package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// Test functions for SQS message processing

func TestNewSQSMessageProcessor(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications")

	if processor.CustomerCode != "hts" {
		t.Errorf("Expected customer code 'hts', got '%s'", processor.CustomerCode)
	}

	if processor.QueueURL != "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications" {
		t.Errorf("Expected queue URL not set correctly")
	}
}

func TestParseSQSMessageBody(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "test-queue")

	// Test valid S3 event message body
	validBody := `{
		"Records": [
			{
				"eventVersion": "2.1",
				"eventSource": "aws:s3",
				"eventTime": "2025-09-20T15:30:00.000Z",
				"eventName": "ObjectCreated:Put",
				"s3": {
					"s3SchemaVersion": "1.0",
					"configurationId": "test-config",
					"bucket": {
						"name": "test-bucket",
						"arn": "arn:aws:s3:::test-bucket"
					},
					"object": {
						"key": "customers/hts/test-change-123.json",
						"size": 1024,
						"eTag": "test-etag"
					}
				}
			}
		]
	}`

	messageBody, err := processor.ParseSQSMessageBody(validBody)
	if err != nil {
		t.Fatalf("Failed to parse valid SQS message body: %v", err)
	}

	if len(messageBody.Records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(messageBody.Records))
	}

	record := messageBody.Records[0]
	if record.EventSource != "aws:s3" {
		t.Errorf("Expected eventSource 'aws:s3', got '%s'", record.EventSource)
	}

	if record.S3.Object.Key != "customers/hts/test-change-123.json" {
		t.Errorf("Expected S3 key 'customers/hts/test-change-123.json', got '%s'", record.S3.Object.Key)
	}

	// Test invalid JSON
	invalidBody := `{"invalid": json}`
	_, err = processor.ParseSQSMessageBody(invalidBody)
	if err == nil {
		t.Errorf("Expected error for invalid JSON, got none")
	}
}

func TestValidateCustomerFromS3Key(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "test-queue")

	tests := []struct {
		name        string
		s3Key       string
		expectError bool
	}{
		{
			name:        "Valid customer key",
			s3Key:       "customers/hts/change-123.json",
			expectError: false,
		},
		{
			name:        "Wrong customer code",
			s3Key:       "customers/cds/change-123.json",
			expectError: true,
		},
		{
			name:        "Invalid prefix",
			s3Key:       "archive/hts/change-123.json",
			expectError: true,
		},
		{
			name:        "Missing json extension",
			s3Key:       "customers/hts/change-123.txt",
			expectError: true,
		},
		{
			name:        "Invalid format",
			s3Key:       "invalid-key",
			expectError: true,
		},
		{
			name:        "Empty key",
			s3Key:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidateCustomerFromS3Key(tt.s3Key)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for S3 key '%s', got none", tt.s3Key)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for S3 key '%s': %v", tt.s3Key, err)
			}
		})
	}
}

func TestValidateSQSMessage(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "test-queue")

	// Test valid message
	validMessage := createTestSQSMessage("test-msg-1", "customers/hts/change-123.json")
	err := processor.ValidateSQSMessage(validMessage)
	if err != nil {
		t.Errorf("Expected no error for valid message, got: %v", err)
	}

	// Test nil message
	err = processor.ValidateSQSMessage(nil)
	if err == nil {
		t.Errorf("Expected error for nil message, got none")
	}

	// Test message with empty MessageID
	invalidMessage := &SQSMessage{
		MessageID: "",
		Body:      "test-body",
	}
	err = processor.ValidateSQSMessage(invalidMessage)
	if err == nil {
		t.Errorf("Expected error for empty MessageID, got none")
	}

	// Test message with empty Body
	invalidMessage = &SQSMessage{
		MessageID: "test-msg",
		Body:      "",
	}
	err = processor.ValidateSQSMessage(invalidMessage)
	if err == nil {
		t.Errorf("Expected error for empty Body, got none")
	}

	// Test message with invalid JSON body
	invalidMessage = &SQSMessage{
		MessageID: "test-msg",
		Body:      "invalid json",
	}
	err = processor.ValidateSQSMessage(invalidMessage)
	if err == nil {
		t.Errorf("Expected error for invalid JSON body, got none")
	}

	// Test message with non-S3 event
	nonS3Body := `{
		"Records": [
			{
				"eventSource": "aws:sns",
				"eventName": "Notification"
			}
		]
	}`
	invalidMessage = &SQSMessage{
		MessageID: "test-msg",
		Body:      nonS3Body,
	}
	err = processor.ValidateSQSMessage(invalidMessage)
	if err == nil {
		t.Errorf("Expected error for non-S3 event, got none")
	}
}

func TestProcessSQSMessage(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "test-queue")

	// Test successful message processing
	validMessage := createTestSQSMessage("test-msg-1", "customers/hts/change-123.json")

	result, err := processor.ProcessSQSMessage(validMessage)
	if err != nil {
		t.Fatalf("Failed to process valid SQS message: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful processing, got failure: %s", result.Error)
	}

	if result.MessageID != "test-msg-1" {
		t.Errorf("Expected MessageID 'test-msg-1', got '%s'", result.MessageID)
	}

	if result.CustomerCode != "hts" {
		t.Errorf("Expected CustomerCode 'hts', got '%s'", result.CustomerCode)
	}

	if !result.MetadataExtracted {
		t.Errorf("Expected MetadataExtracted to be true")
	}

	if result.EmailsSent <= 0 {
		t.Errorf("Expected EmailsSent > 0, got %d", result.EmailsSent)
	}

	if result.ProcessingTime <= 0 {
		t.Errorf("Expected ProcessingTime > 0, got %v", result.ProcessingTime)
	}

	// Test message with wrong customer code
	wrongCustomerMessage := createTestSQSMessage("test-msg-2", "customers/cds/change-123.json")

	result, err = processor.ProcessSQSMessage(wrongCustomerMessage)
	if err == nil {
		t.Errorf("Expected error for wrong customer code, got none")
	}

	if result.Success {
		t.Errorf("Expected failed processing for wrong customer code")
	}

	// Test message with invalid S3 key format
	invalidKeyMessage := createTestSQSMessage("test-msg-3", "invalid-key-format")

	result, err = processor.ProcessSQSMessage(invalidKeyMessage)
	if err == nil {
		t.Errorf("Expected error for invalid S3 key format, got none")
	}

	if result.Success {
		t.Errorf("Expected failed processing for invalid S3 key format")
	}
}

func TestProcessMessageBatch(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "test-queue")

	// Create batch of test messages
	messages := []*SQSMessage{
		createTestSQSMessage("msg-1", "customers/hts/change-1.json"),
		createTestSQSMessage("msg-2", "customers/hts/change-2.json"),
		createTestSQSMessage("msg-3", "customers/hts/change-3.json"),
		createTestSQSMessage("msg-4", "customers/cds/change-4.json"), // Wrong customer - should fail
	}

	results, err := processor.ProcessMessageBatch(messages)
	if err != nil {
		t.Fatalf("Failed to process message batch: %v", err)
	}

	if len(results) != len(messages) {
		t.Errorf("Expected %d results, got %d", len(messages), len(results))
	}

	// Check that first 3 messages succeeded and last one failed
	for i := 0; i < 3; i++ {
		if !results[i].Success {
			t.Errorf("Expected message %d to succeed, got failure: %s", i+1, results[i].Error)
		}
	}

	if results[3].Success {
		t.Errorf("Expected message 4 to fail (wrong customer), got success")
	}

	// Test empty batch
	_, err = processor.ProcessMessageBatch([]*SQSMessage{})
	if err == nil {
		t.Errorf("Expected error for empty batch, got none")
	}
}

func TestGetProcessingStats(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "test-queue")

	// Create test results
	results := []*ProcessingResult{
		{
			MessageID:      "msg-1",
			Success:        true,
			EmailsSent:     2,
			ProcessingTime: 100 * time.Millisecond,
		},
		{
			MessageID:      "msg-2",
			Success:        true,
			EmailsSent:     1,
			ProcessingTime: 150 * time.Millisecond,
		},
		{
			MessageID:      "msg-3",
			Success:        false,
			EmailsSent:     0,
			ProcessingTime: 50 * time.Millisecond,
		},
	}

	stats := processor.GetProcessingStats(results)

	if stats["totalMessages"] != 3 {
		t.Errorf("Expected totalMessages 3, got %v", stats["totalMessages"])
	}

	if stats["successfulMessages"] != 2 {
		t.Errorf("Expected successfulMessages 2, got %v", stats["successfulMessages"])
	}

	if stats["failedMessages"] != 1 {
		t.Errorf("Expected failedMessages 1, got %v", stats["failedMessages"])
	}

	if stats["totalEmailsSent"] != 3 {
		t.Errorf("Expected totalEmailsSent 3, got %v", stats["totalEmailsSent"])
	}

	successRate := stats["successRate"].(float64)
	expectedRate := 66.66666666666667 // 2/3 * 100
	if successRate != expectedRate {
		t.Errorf("Expected successRate %.2f, got %.2f", expectedRate, successRate)
	}

	// Test empty results
	emptyStats := processor.GetProcessingStats([]*ProcessingResult{})
	if emptyStats["totalMessages"] != 0 {
		t.Errorf("Expected totalMessages 0 for empty results, got %v", emptyStats["totalMessages"])
	}
}

func TestExtractChangeIDFromS3Key(t *testing.T) {
	tests := []struct {
		name     string
		s3Key    string
		expected string
	}{
		{
			name:     "Standard format",
			s3Key:    "customers/hts/change-123-2025-09-20T15-30-00.json",
			expected: "change-123",
		},
		{
			name:     "UUID format",
			s3Key:    "customers/cds/550e8400-e29b-41d4-a716-446655440000-2025-09-20T15-30-00.json",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "Simple format",
			s3Key:    "customers/motor/simple-change.json",
			expected: "simple-change",
		},
		{
			name:     "No timestamp",
			s3Key:    "customers/bat/change-456.json",
			expected: "change-456",
		},
		{
			name:     "Invalid format",
			s3Key:    "invalid-key",
			expected: "invalid-key",
		},
		{
			name:     "Empty key",
			s3Key:    "",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractChangeIDFromS3Key(tt.s3Key)
			if result != tt.expected {
				t.Errorf("Expected change ID '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetCustomerDisplayName(t *testing.T) {
	tests := []struct {
		customerCode string
		expected     string
	}{
		{"hts", "HTS Prod"},
		{"cds", "CDS Global"},
		{"motor", "Motor"},
		{"unknown", "UNKNOWN"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.customerCode, func(t *testing.T) {
			result := getCustomerDisplayName(tt.customerCode)
			if result != tt.expected {
				t.Errorf("Expected display name '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// Helper functions for tests

func createTestSQSMessage(messageID, s3Key string) *SQSMessage {
	eventTime := time.Now()

	s3Event := S3EventRecord{
		EventVersion: "2.1",
		EventSource:  "aws:s3",
		EventTime:    eventTime,
		EventName:    "ObjectCreated:Put",
		S3: S3Event{
			S3SchemaVersion: "1.0",
			ConfigurationID: "test-config",
			Bucket: S3Bucket{
				Name: "test-bucket",
				ARN:  "arn:aws:s3:::test-bucket",
			},
			Object: S3Object{
				Key:  s3Key,
				Size: 1024,
				ETag: "test-etag",
			},
		},
	}

	messageBody := SQSMessageBody{
		Records: []S3EventRecord{s3Event},
	}

	bodyJSON, _ := json.Marshal(messageBody)

	return &SQSMessage{
		MessageID:         messageID,
		ReceiptHandle:     fmt.Sprintf("receipt-%s", messageID),
		Body:              string(bodyJSON),
		Attributes:        make(map[string]interface{}),
		MessageAttributes: make(map[string]interface{}),
	}
}

// Integration test for complete workflow
func TestSQSMessageProcessingWorkflow(t *testing.T) {
	processor := NewSQSMessageProcessor("hts", "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications")

	// Create a realistic SQS message
	message := createTestSQSMessage("integration-test-msg", "customers/hts/550e8400-e29b-41d4-a716-446655440000-2025-09-20T15-30-00.json")

	// Step 1: Validate message
	err := processor.ValidateSQSMessage(message)
	if err != nil {
		t.Fatalf("Message validation failed: %v", err)
	}

	// Step 2: Process message
	result, err := processor.ProcessSQSMessage(message)
	if err != nil {
		t.Fatalf("Message processing failed: %v", err)
	}

	// Step 3: Verify results
	if !result.Success {
		t.Errorf("Expected successful processing, got failure: %s", result.Error)
	}

	if result.CustomerCode != "hts" {
		t.Errorf("Expected customer code 'hts', got '%s'", result.CustomerCode)
	}

	if !result.MetadataExtracted {
		t.Errorf("Expected metadata to be extracted")
	}

	if result.EmailsSent <= 0 {
		t.Errorf("Expected emails to be sent, got %d", result.EmailsSent)
	}

	// Step 4: Generate stats
	stats := processor.GetProcessingStats([]*ProcessingResult{result})

	if stats["totalMessages"] != 1 {
		t.Errorf("Expected 1 total message in stats, got %v", stats["totalMessages"])
	}

	if stats["successfulMessages"] != 1 {
		t.Errorf("Expected 1 successful message in stats, got %v", stats["successfulMessages"])
	}

	t.Logf("Integration test completed successfully:")
	t.Logf("  Message ID: %s", result.MessageID)
	t.Logf("  Customer: %s", result.CustomerCode)
	t.Logf("  Emails sent: %d", result.EmailsSent)
	t.Logf("  Processing time: %v", result.ProcessingTime)
}
