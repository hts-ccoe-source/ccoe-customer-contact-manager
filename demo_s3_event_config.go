package main

import (
	"fmt"
	"os"
	"strings"
)

// DemoS3EventConfiguration demonstrates the S3 event notification configuration functionality
func DemoS3EventConfiguration() {
	fmt.Println("🔧 S3 Event Notification Configuration Demo")
	fmt.Println("===========================================")

	// Create S3 event configuration manager
	bucketName := "multi-customer-change-metadata"
	manager := NewS3EventConfigManager(bucketName)

	fmt.Printf("📦 Created S3 event configuration for bucket: %s\n", bucketName)
	fmt.Println()

	// Add customer notifications
	customers := map[string]string{
		"customer-a": "arn:aws:sqs:us-east-1:123456789012:customer-a-change-queue",
		"customer-b": "arn:aws:sqs:us-east-1:123456789012:customer-b-change-queue",
		"customer-c": "arn:aws:sqs:us-east-1:123456789012:customer-c-change-queue",
		"test-org":   "arn:aws:sqs:us-east-1:123456789012:test-org-change-queue",
	}

	fmt.Println("➕ Adding customer notifications:")
	for customerCode, sqsArn := range customers {
		err := manager.AddCustomerNotification(customerCode, sqsArn)
		if err != nil {
			fmt.Printf("❌ Error adding %s: %v\n", customerCode, err)
			continue
		}
		fmt.Printf("   ✅ %s → %s\n", customerCode, sqsArn)
	}
	fmt.Println()

	// Validate configuration
	fmt.Println("🔍 Validating configuration...")
	err := manager.ValidateConfiguration()
	if err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		return
	}
	fmt.Println("✅ Configuration is valid!")
	fmt.Println()

	// Show configuration details
	fmt.Println("📋 Configuration Summary:")
	fmt.Printf("   Bucket: %s\n", manager.Config.BucketName)
	fmt.Printf("   Customer Notifications: %d\n", len(manager.Config.CustomerNotifications))
	fmt.Println()

	for _, notification := range manager.Config.CustomerNotifications {
		fmt.Printf("   🏢 %s:\n", notification.CustomerCode)
		fmt.Printf("      Prefix: %s\n", notification.Prefix)
		fmt.Printf("      Suffix: %s\n", notification.Suffix)
		fmt.Printf("      SQS: %s\n", notification.SQSQueueArn)
		fmt.Println()
	}

	// Generate Terraform configuration
	fmt.Println("🏗️  Generating Terraform configuration...")
	terraformConfig, err := manager.GenerateTerraformConfig()
	if err != nil {
		fmt.Printf("❌ Error generating Terraform config: %v\n", err)
		return
	}

	fmt.Println("✅ Generated Terraform configuration:")
	fmt.Println("```hcl")
	fmt.Print(terraformConfig)
	fmt.Println("```")
	fmt.Println()

	// Save configuration to file
	fmt.Println("💾 Saving configuration to file...")
	err = manager.SaveConfiguration("S3EventConfig.json")
	if err != nil {
		fmt.Printf("❌ Error saving configuration: %v\n", err)
		return
	}

	// Demonstrate how this integrates with customer code extraction
	fmt.Println("🔗 Integration with Customer Code Extraction:")
	fmt.Println("   1. Extract customer codes from metadata: [customer-a, customer-b]")
	fmt.Println("   2. For each customer code:")
	fmt.Println("      → Upload to S3: customers/customer-a/changeId-v1.json")
	fmt.Println("      → S3 event triggers: customer-a-change-queue")
	fmt.Println("      → ECS task starts in customer-a organization")
	fmt.Println("   3. Parallel processing across all affected customers")
	fmt.Println()

	// Demonstrate S3 event testing functionality
	fmt.Println("🧪 Testing S3 Event Delivery:")
	tester := NewS3EventTester(manager)

	// Generate test plan
	testPlan, err := tester.GenerateS3EventTestPlan()
	if err != nil {
		fmt.Printf("❌ Error generating test plan: %v\n", err)
	} else {
		fmt.Println("📋 Generated test plan (first few lines):")
		lines := strings.Split(testPlan, "\n")
		for i, line := range lines {
			if i >= 10 { // Show first 10 lines
				fmt.Println("   ... (truncated)")
				break
			}
			fmt.Printf("   %s\n", line)
		}
		fmt.Println()
	}

	// Run dry-run test
	customerCodes := []string{"customer-a", "customer-b"}
	fmt.Println("🔍 Running dry-run test for sample customers...")
	err = tester.TestS3EventDelivery(customerCodes, true)
	if err != nil {
		fmt.Printf("❌ Test failed: %v\n", err)
	}

	fmt.Println("✨ S3 event notification configuration completed successfully!")
	fmt.Println()
	fmt.Println("🚀 Next Steps:")
	fmt.Println("   - Deploy Terraform configuration to AWS")
	fmt.Println("   - Test S3 event delivery to SQS queues")
	fmt.Println("   - Integrate with multi-customer upload logic")
}

// Run the demo if this file is executed directly
func init() {
	// This will run when the package is imported, but only if we're running the demo
	if len(os.Args) > 1 && os.Args[1] == "demo-s3-config" {
		DemoS3EventConfiguration()
		os.Exit(0)
	}
}
