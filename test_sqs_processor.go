package main

import (
	"fmt"
	"log"
	"strings"
)

// Simple test runner for SQS processor functionality

func main() {
	fmt.Println("=== SQS Message Processor Tests ===")

	// Test 1: Create processor
	fmt.Println("\n🧪 Test 1: Create SQS Message Processor")
	testCreateProcessor()

	// Test 2: Validate customer from S3 key
	fmt.Println("\n🧪 Test 2: Validate Customer from S3 Key")
	testValidateCustomer()

	// Test 3: Extract change ID from S3 key
	fmt.Println("\n🧪 Test 3: Extract Change ID from S3 Key")
	testExtractChangeID()

	// Test 4: Customer display names
	fmt.Println("\n🧪 Test 4: Customer Display Names")
	testCustomerDisplayNames()

	fmt.Println("\n✅ All tests completed successfully!")
}

func testCreateProcessor() {
	processor := NewSQSMessageProcessor("hts", "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications")

	if processor.CustomerCode != "hts" {
		log.Fatalf("❌ Expected customer code 'hts', got '%s'", processor.CustomerCode)
	}

	if processor.QueueURL != "https://sqs.us-east-1.amazonaws.com/123456789012/hts-notifications" {
		log.Fatalf("❌ Queue URL not set correctly")
	}

	fmt.Printf("   ✅ Processor created successfully for customer: %s\n", processor.CustomerCode)
}

func testValidateCustomer() {
	processor := NewSQSMessageProcessor("hts", "test-queue")

	tests := []struct {
		name        string
		s3Key       string
		expectError bool
	}{
		{"Valid customer key", "customers/hts/change-123.json", false},
		{"Wrong customer code", "customers/cds/change-123.json", true},
		{"Invalid prefix", "archive/hts/change-123.json", true},
		{"Missing json extension", "customers/hts/change-123.txt", true},
		{"Invalid format", "invalid-key", true},
	}

	for _, tt := range tests {
		err := processor.ValidateCustomerFromS3Key(tt.s3Key)

		if tt.expectError && err == nil {
			log.Fatalf("❌ %s: Expected error but got none", tt.name)
		}

		if !tt.expectError && err != nil {
			log.Fatalf("❌ %s: Unexpected error: %v", tt.name, err)
		}

		status := "✅"
		if tt.expectError {
			status = "❌ (expected)"
		}
		fmt.Printf("   %s %s\n", status, tt.name)
	}
}

func testExtractChangeID() {
	tests := []struct {
		name     string
		s3Key    string
		expected string
	}{
		{"Standard format", "customers/hts/change-123-2025-09-20T15-30-00.json", "change-123"},
		{"UUID format", "customers/cds/550e8400-e29b-41d4-a716-446655440000-2025-09-20T15-30-00.json", "550e8400-e29b-41d4-a716-446655440000"},
		{"Simple format", "customers/motor/simple-change.json", "simple"},
		{"Invalid format", "invalid-key", "unknown"},
	}

	for _, tt := range tests {
		result := extractChangeIDFromS3Key(tt.s3Key)
		if result != tt.expected {
			log.Fatalf("❌ %s: Expected '%s', got '%s'", tt.name, tt.expected, result)
		}
		fmt.Printf("   ✅ %s: %s\n", tt.name, result)
	}
}

func testCustomerDisplayNames() {
	tests := []struct {
		customerCode string
		expected     string
	}{
		{"hts", "HTS Prod"},
		{"cds", "CDS Global"},
		{"motor", "Motor"},
		{"unknown", "UNKNOWN"},
	}

	for _, tt := range tests {
		result := getCustomerDisplayName(tt.customerCode)
		if result != tt.expected {
			log.Fatalf("❌ Expected '%s', got '%s'", tt.expected, result)
		}
		fmt.Printf("   ✅ %s -> %s\n", tt.customerCode, result)
	}
}

// Include required types and functions from the main implementation

type SQSMessageProcessor struct {
	CustomerCode string
	QueueURL     string
	SQSClient    interface{}
}

func NewSQSMessageProcessor(customerCode, queueURL string) *SQSMessageProcessor {
	return &SQSMessageProcessor{
		CustomerCode: customerCode,
		QueueURL:     queueURL,
	}
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

func extractChangeIDFromS3Key(s3Key string) string {
	parts := strings.Split(s3Key, "/")
	if len(parts) < 3 {
		return "unknown"
	}

	filename := parts[len(parts)-1]
	filename = strings.TrimSuffix(filename, ".json")

	// Look for timestamp pattern and extract everything before it
	timestampPattern := "-2025-"
	if idx := strings.Index(filename, timestampPattern); idx > 0 {
		return filename[:idx]
	}

	// Fallback: extract everything before the last dash
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
