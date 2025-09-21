package main

import (
	"fmt"
	"log"
	// "strings"
)

// Simple test runner for SQS processor functionality

func TestSQSProcessor() {
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
