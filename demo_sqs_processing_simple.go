package main

import (
	"fmt"
	"strings"
	"time"
)

// Simple demo for SQS message processing

func main() {
	fmt.Println("=== SQS Message Processing Demo ===")

	// Demo 1: Single message processing
	fmt.Println("\n🧪 Demo 1: Single SQS Message Processing")
	demoSingleMessage()

	// Demo 2: Error handling
	fmt.Println("\n🧪 Demo 2: Error Handling")
	demoErrorHandling()

	// Demo 3: Multi-customer processing
	fmt.Println("\n🧪 Demo 3: Multi-Customer Processing")
	demoMultiCustomer()

	fmt.Println("\n=== Demo Complete ===")
}

func demoSingleMessage() {
	fmt.Printf("📨 Processing SQS message for customer: hts\n")
	fmt.Printf("   S3 Object: customers/hts/deploy-monitoring-system-2025-09-20T15-30-00.json\n")

	// Simulate message processing
	startTime := time.Now()

	// Step 1: Parse SQS message
	fmt.Printf("   ✅ Step 1: Parsed SQS message body\n")

	// Step 2: Validate customer code
	fmt.Printf("   ✅ Step 2: Validated customer code from S3 key\n")

	// Step 3: Extract metadata
	fmt.Printf("   ✅ Step 3: Extracted embedded metadata\n")

	// Step 4: Process emails
	emailCount := 2
	fmt.Printf("   ✅ Step 4: Processed email notifications (%d emails)\n", emailCount)

	processingTime := time.Since(startTime)

	fmt.Printf("✅ Processing successful!\n")
	fmt.Printf("   Customer: hts (HTS Prod)\n")
	fmt.Printf("   Change ID: deploy-monitoring-system\n")
	fmt.Printf("   Emails sent: %d\n", emailCount)
	fmt.Printf("   Processing time: %v\n", processingTime)
}

func demoErrorHandling() {
	errorScenarios := []struct {
		name     string
		s3Key    string
		customer string
	}{
		{"Wrong customer code", "customers/cds/change-123.json", "hts"},
		{"Invalid S3 key format", "invalid-key-format", "hts"},
		{"Non-JSON file", "customers/hts/change-123.txt", "hts"},
		{"Archive prefix", "archive/change-123.json", "hts"},
	}

	fmt.Printf("🔍 Testing error handling scenarios:\n")

	for _, scenario := range errorScenarios {
		fmt.Printf("   Testing: %s\n", scenario.name)

		// Simulate validation
		err := validateCustomerFromS3Key(scenario.s3Key, scenario.customer)
		if err != nil {
			fmt.Printf("      ❌ Expected error caught: %v\n", err)
		} else {
			fmt.Printf("      ⚠️  Unexpected success - this should have failed!\n")
		}
	}
}

func demoMultiCustomer() {
	customers := []struct {
		code        string
		displayName string
	}{
		{"hts", "HTS Prod"},
		{"cds", "CDS Global"},
		{"motor", "Motor"},
		{"bat", "Bring A Trailer"},
	}

	fmt.Printf("🏢 Simulating multi-customer processing for %d customers\n", len(customers))

	totalEmails := 0
	successCount := 0

	for _, customer := range customers {
		fmt.Printf("   Processing for %s (%s)...\n", customer.displayName, customer.code)

		// Simulate processing
		time.Sleep(50 * time.Millisecond)
		emailCount := 1 + (len(customer.code) % 3) // 1-3 emails based on customer

		fmt.Printf("      ✅ Success: %d emails sent\n", emailCount)

		totalEmails += emailCount
		successCount++
	}

	fmt.Printf("📈 Multi-Customer Summary:\n")
	fmt.Printf("   Customers processed: %d/%d\n", successCount, len(customers))
	fmt.Printf("   Total emails sent: %d\n", totalEmails)
	fmt.Printf("   Average emails per customer: %.1f\n", float64(totalEmails)/float64(len(customers)))
}

// Helper function to validate customer from S3 key
func validateCustomerFromS3Key(s3Key, expectedCustomer string) error {
	parts := strings.Split(s3Key, "/")

	if len(parts) < 3 {
		return fmt.Errorf("invalid S3 key format: %s", s3Key)
	}

	if parts[0] != "customers" {
		return fmt.Errorf("S3 key does not start with 'customers/': %s", s3Key)
	}

	keyCustomerCode := parts[1]
	if keyCustomerCode != expectedCustomer {
		return fmt.Errorf("S3 key customer code '%s' does not match expected '%s'",
			keyCustomerCode, expectedCustomer)
	}

	if !strings.HasSuffix(s3Key, ".json") {
		return fmt.Errorf("S3 key does not end with '.json': %s", s3Key)
	}

	return nil
}
