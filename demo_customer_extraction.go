package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// DemoCustomerCodeExtraction demonstrates the customer code extraction functionality
func DemoCustomerCodeExtraction() {
	fmt.Println("🔍 Customer Code Extraction Demo")
	fmt.Println("================================")

	// Load example metadata
	data, err := os.ReadFile("example_metadata.json")
	if err != nil {
		fmt.Printf("Error reading example metadata: %v\n", err)
		return
	}

	var metadata ApprovalRequestMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		fmt.Printf("Error parsing metadata: %v\n", err)
		return
	}

	fmt.Printf("📄 Loaded metadata for change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("📋 Raw customer codes from metadata: %v\n", metadata.ChangeMetadata.CustomerCodes)
	fmt.Println()

	// Load valid customer codes from configuration
	validCodes, err := LoadValidCustomerCodes()
	if err != nil {
		fmt.Printf("⚠️  Could not load valid customer codes: %v\n", err)
		fmt.Println("📝 Using validation disabled mode")
		validCodes = []string{} // Empty means no validation
	} else {
		fmt.Printf("✅ Loaded valid customer codes: %v\n", validCodes)
	}
	fmt.Println()

	// Create customer code extractor
	extractor := NewCustomerCodeExtractor(validCodes)

	// Extract and validate customer codes
	extractedCodes, err := extractor.ExtractCustomerCodes(&metadata)
	if err != nil {
		fmt.Printf("❌ Error extracting customer codes: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully extracted customer codes: %v\n", extractedCodes)
	fmt.Printf("📊 Number of affected customers: %d\n", len(extractedCodes))
	fmt.Println()

	// Show what would happen next
	fmt.Println("🚀 Next Steps (Multi-Customer Distribution):")
	for i, code := range extractedCodes {
		fmt.Printf("   %d. Upload to S3: customers/%s/changeId-v1.json\n", i+1, code)
		fmt.Printf("      → Triggers SQS queue for %s\n", code)
		fmt.Printf("      → Starts ECS task in %s organization\n", code)
	}
	fmt.Printf("   %d. Upload to S3: archive/changeId.json (permanent storage)\n", len(extractedCodes)+1)
	fmt.Println()

	fmt.Println("✨ Customer code extraction completed successfully!")
}

// Run the demo if this file is executed directly
func init() {
	// This will run when the package is imported, but only if we're running the demo
	if len(os.Args) > 1 && os.Args[1] == "demo-extraction" {
		DemoCustomerCodeExtraction()
		os.Exit(0)
	}
}
