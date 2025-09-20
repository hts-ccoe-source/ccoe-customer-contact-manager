package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// DemoSQSMessageSending demonstrates the SQS message creation and sending functionality
func DemoSQSMessageSending() {
	fmt.Println("📨 SQS Message Creation and Sending Demo")
	fmt.Println("========================================")

	// Load S3 event configuration (from previous task)
	config := NewS3EventConfigManager("multi-customer-change-metadata")

	// Add customer configurations
	customers := map[string]string{
		"customer-a": "arn:aws:sqs:us-east-1:123456789012:customer-a-change-queue",
		"customer-b": "arn:aws:sqs:us-east-1:123456789012:customer-b-change-queue",
		"customer-c": "arn:aws:sqs:us-east-1:123456789012:customer-c-change-queue",
	}

	fmt.Println("🔧 Setting up SQS configuration...")
	for customerCode, sqsArn := range customers {
		err := config.AddCustomerNotification(customerCode, sqsArn)
		if err != nil {
			fmt.Printf("❌ Error adding %s: %v\n", customerCode, err)
			return
		}
		fmt.Printf("   ✅ %s → %s\n", customerCode, sqsArn)
	}
	fmt.Println()

	// Create SQS message sender
	sender := NewSQSMessageSender(config)

	// Load example metadata (from Task 1)
	fmt.Println("📄 Loading example metadata...")
	data, err := os.ReadFile("example_metadata.json")
	if err != nil {
		fmt.Printf("❌ Error reading example metadata: %v\n", err)
		fmt.Println("📝 Creating sample metadata instead...")

		// Create sample metadata
		sampleMetadata := &ApprovalRequestMetadata{
			ChangeMetadata: struct {
				Title         string   `json:"title"`
				CustomerNames []string `json:"customerNames"`
				CustomerCodes []string `json:"customerCodes"`
				Tickets       struct {
					ServiceNow string `json:"serviceNow"`
					Jira       string `json:"jira"`
				} `json:"tickets"`
				ChangeReason           string `json:"changeReason"`
				ImplementationPlan     string `json:"implementationPlan"`
				TestPlan               string `json:"testPlan"`
				ExpectedCustomerImpact string `json:"expectedCustomerImpact"`
				RollbackPlan           string `json:"rollbackPlan"`
				Schedule               struct {
					ImplementationStart string `json:"implementationStart"`
					ImplementationEnd   string `json:"implementationEnd"`
					BeginDate           string `json:"beginDate"`
					BeginTime           string `json:"beginTime"`
					EndDate             string `json:"endDate"`
					EndTime             string `json:"endTime"`
					Timezone            string `json:"timezone"`
				} `json:"schedule"`
			}{
				Title:         "Demo Change: Configure Proof-of-Value exercise",
				CustomerNames: []string{"Customer A", "Customer B"},
				CustomerCodes: []string{"customer-a", "customer-b"},
				Tickets: struct {
					ServiceNow string `json:"serviceNow"`
					Jira       string `json:"jira"`
				}{
					ServiceNow: "CHG0123456",
					Jira:       "INFRA-2847",
				},
				ChangeReason:           "Evaluate new cost management platform",
				ImplementationPlan:     "Deploy FinOut platform in test environment",
				TestPlan:               "Validate cost data ingestion and reporting",
				ExpectedCustomerImpact: "No customer impact expected",
				RollbackPlan:           "Remove FinOut configuration if issues arise",
				Schedule: struct {
					ImplementationStart string `json:"implementationStart"`
					ImplementationEnd   string `json:"implementationEnd"`
					BeginDate           string `json:"beginDate"`
					BeginTime           string `json:"beginTime"`
					EndDate             string `json:"endDate"`
					EndTime             string `json:"endTime"`
					Timezone            string `json:"timezone"`
				}{
					ImplementationStart: "2025-09-20T10:00",
					ImplementationEnd:   "2025-09-20T17:00",
					BeginDate:           "2025-09-20",
					BeginTime:           "10:00",
					EndDate:             "2025-09-20",
					EndTime:             "17:00",
					Timezone:            "America/New_York",
				},
			},
			GeneratedAt: time.Now().Format(time.RFC3339),
			GeneratedBy: "demo-sqs-messages",
		}

		// Use sample metadata
		data, _ = json.Marshal(sampleMetadata)
	}

	var metadata ApprovalRequestMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		fmt.Printf("❌ Error parsing metadata: %v\n", err)
		return
	}

	fmt.Printf("✅ Loaded metadata for change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("📋 Affected customers: %v\n", metadata.ChangeMetadata.CustomerCodes)
	fmt.Println()

	// Demonstrate single customer message creation
	fmt.Println("🔨 Creating single customer SQS message...")
	singleMessage, err := sender.CreateSQSMessage("customer-a", "send-change-notification", &metadata)
	if err != nil {
		fmt.Printf("❌ Error creating message: %v\n", err)
		return
	}

	fmt.Printf("✅ Created message for customer-a:\n")
	fmt.Printf("   🆔 Execution ID: %s\n", singleMessage.ExecutionID)
	fmt.Printf("   ⚡ Action Type: %s\n", singleMessage.ActionType)
	fmt.Printf("   🕐 Timestamp: %s\n", singleMessage.Timestamp)
	fmt.Printf("   🔄 Retry Count: %d\n", singleMessage.RetryCount)
	fmt.Println()

	// Demonstrate message validation
	fmt.Println("🔍 Validating SQS message...")
	err = sender.ValidateSQSMessage(singleMessage)
	if err != nil {
		fmt.Printf("❌ Message validation failed: %v\n", err)
		return
	}
	fmt.Println("✅ Message validation passed!")
	fmt.Println()

	// Demonstrate single message sending (dry run)
	fmt.Println("📤 Sending single SQS message (dry run)...")
	err = sender.SendSQSMessage(singleMessage, true)
	if err != nil {
		fmt.Printf("❌ Error sending message: %v\n", err)
		return
	}
	fmt.Println()

	// Demonstrate multi-customer message sending
	fmt.Println("📨 Sending multi-customer SQS messages (dry run)...")
	customerCodes := []string{"customer-a", "customer-b"}
	results, err := sender.SendMultiCustomerMessages(customerCodes, "send-change-notification", &metadata, true)
	if err != nil {
		fmt.Printf("⚠️  Some messages failed to send: %v\n", err)
	}

	// Show results summary
	fmt.Println("📊 Multi-customer sending results:")
	for customerCode, result := range results {
		status := "✅ SUCCESS"
		if result != nil {
			status = fmt.Sprintf("❌ FAILED: %v", result)
		}
		fmt.Printf("   %s %s\n", status, customerCode)
	}
	fmt.Println()

	// Demonstrate retry functionality
	fmt.Println("🔄 Demonstrating retry functionality...")
	retryMessage, err := sender.RetryFailedMessage(singleMessage)
	if err != nil {
		fmt.Printf("❌ Error creating retry message: %v\n", err)
		return
	}

	fmt.Printf("✅ Created retry message:\n")
	fmt.Printf("   🆔 Original ID: %s\n", singleMessage.ExecutionID)
	fmt.Printf("   🆔 Retry ID: %s\n", retryMessage.ExecutionID)
	fmt.Printf("   🔄 Retry Count: %d → %d\n", singleMessage.RetryCount, retryMessage.RetryCount)
	fmt.Println()

	// Generate message template
	fmt.Println("📋 Generating SQS message template...")
	template, err := sender.GenerateSQSMessageTemplate()
	if err != nil {
		fmt.Printf("❌ Error generating template: %v\n", err)
		return
	}

	fmt.Println("✅ Generated SQS message template:")
	fmt.Println("```json")
	// Show first few lines of template
	lines := strings.Split(template, "\n")
	for i, line := range lines {
		if i >= 15 { // Show first 15 lines
			fmt.Println("  ... (truncated)")
			break
		}
		fmt.Printf("  %s\n", line)
	}
	fmt.Println("```")
	fmt.Println()

	// Show integration with previous tasks
	fmt.Println("🔗 Integration with Previous Tasks:")
	fmt.Println("   Task 1 (Customer Code Extraction):")
	fmt.Println("      → Extract customer codes: [customer-a, customer-b]")
	fmt.Println("   Task 2 (S3 Event Configuration):")
	fmt.Println("      → Map customer codes to SQS queues")
	fmt.Println("   Task 3 (SQS Message Creation):")
	fmt.Println("      → Create and send SQS messages to customer queues")
	fmt.Println("      → Each customer receives their own message")
	fmt.Println("      → Messages contain complete metadata for processing")
	fmt.Println()

	fmt.Println("🚀 Next Steps:")
	fmt.Println("   - Implement actual AWS SQS client integration")
	fmt.Println("   - Add retry logic with exponential backoff")
	fmt.Println("   - Integrate with S3 upload functionality (Task 4)")
	fmt.Println("   - Add dead letter queue handling")
	fmt.Println()

	fmt.Println("✨ SQS message creation and sending completed successfully!")
}

// Run the demo if this file is executed directly
func init() {
	// This will run when the package is imported, but only if we're running the demo
	if len(os.Args) > 1 && os.Args[1] == "demo-sqs-messages" {
		DemoSQSMessageSending()
		os.Exit(0)
	}
}
