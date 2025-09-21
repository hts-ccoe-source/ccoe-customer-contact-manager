package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Demo application showcasing Tasks 6 and 12 integration
// Task 6: Customer-specific AWS credential handling
// Task 12: Enhanced CLI for SQS integration

func main() {
	fmt.Println("=== Tasks 6 & 12 Integration Demo ===")
	fmt.Println("Task 6: Customer-specific AWS credential handling")
	fmt.Println("Task 12: Enhanced CLI for SQS integration")

	// Demo 1: Customer credential handling
	fmt.Println("\n🔐 Demo 1: Customer-Specific AWS Credential Handling")
	demoCustomerCredentialHandling()

	// Demo 2: Enhanced SQS integration
	fmt.Println("\n📨 Demo 2: Enhanced SQS Integration with CLI")
	demoEnhancedSQSIntegration()

	// Demo 3: Combined workflow
	fmt.Println("\n🔄 Demo 3: Combined Workflow - SQS + Cross-Account Credentials")
	demoCombinedWorkflow()

	fmt.Println("\n=== Tasks 6 & 12 Integration Demo Complete ===")
}

func demoCustomerCredentialHandling() {
	fmt.Printf("🔐 Customer-Specific AWS Credential Handling Demo\n")

	// Create customer credential manager
	customerManager := NewCustomerCredentialManager("us-east-1")

	// Add test customer mappings with cross-account roles
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/HTSSQSRole",
			S3RoleARN:    "arn:aws:iam::123456789012:role/HTSS3Role",
			Environment:  "production",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Global",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			SQSRoleARN:   "arn:aws:iam::234567890123:role/CDSSQSRole",
			Environment:  "production",
		},
		"motor": {
			CustomerCode: "motor",
			CustomerName: "Motor Staging",
			AWSAccountID: "345678901234",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::345678901234:role/MotorSESRole",
			Environment:  "staging",
		},
	}

	fmt.Printf("   📋 Loaded %d customer account mappings\n", len(customerManager.CustomerMappings))

	// Create enhanced credential manager
	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		fmt.Printf("   ⚠️  Enhanced credential manager creation failed (expected in demo): %v\n", err)
		fmt.Printf("   📝 In production, this would use real AWS STS for cross-account role assumption\n")

		// Continue with simulation using the basic manager
		demoBasicCredentialHandling(customerManager)
		return
	}

	fmt.Printf("   ✅ Enhanced credential manager created successfully\n")

	// Demo real AWS credential operations
	demoRealCredentialHandling(enhancedManager)
}

func demoBasicCredentialHandling(customerManager *CustomerCredentialManager) {
	fmt.Printf("\n   🔄 Demonstrating basic credential handling (simulation mode)\n")

	// Test credential operations for each customer
	for customerCode := range customerManager.CustomerMappings {
		fmt.Printf("      🏢 Processing customer: %s\n", customerCode)

		// Assume role for SES service
		credentials, err := customerManager.AssumeCustomerRole(customerCode, "ses")
		if err != nil {
			fmt.Printf("         ❌ Failed to assume SES role: %v\n", err)
			continue
		}

		fmt.Printf("         ✅ SES role assumed successfully\n")
		fmt.Printf("            Account ID: %s\n", extractAccountIDFromRoleARN(credentials.RoleARN))
		fmt.Printf("            Region: %s\n", credentials.Region)
		fmt.Printf("            Expires: %v\n", credentials.Expiration.Format("2006-01-02 15:04:05"))

		// Validate credentials
		validation, err := customerManager.ValidateCredentials(credentials)
		if err != nil {
			fmt.Printf("         ⚠️  Credential validation failed: %v\n", err)
		} else {
			fmt.Printf("         ✅ Credentials validated: %s\n", validation.UserARN)
		}

		// Create AWS client configuration
		clientConfig, err := customerManager.CreateAWSClientConfig(credentials, "ses")
		if err != nil {
			fmt.Printf("         ❌ Failed to create client config: %v\n", err)
		} else {
			fmt.Printf("         ✅ AWS client config created for %s service\n", clientConfig["serviceType"])
		}
	}

	// Demo bulk validation
	fmt.Printf("\n   📊 Bulk credential validation for all customers:\n")
	results, err := customerManager.ValidateAllCustomerCredentials("ses")
	if err != nil {
		fmt.Printf("      ❌ Bulk validation failed: %v\n", err)
		return
	}

	successCount := 0
	for customerCode, result := range results {
		if result.Valid {
			successCount++
			fmt.Printf("      ✅ %s: Valid (expires %v)\n", customerCode, result.ExpiresAt.Format("15:04:05"))
		} else {
			fmt.Printf("      ❌ %s: Invalid - %s\n", customerCode, result.Error)
		}
	}

	fmt.Printf("   📈 Validation Summary: %d/%d customers successful\n", successCount, len(results))
}

func demoRealCredentialHandling(enhancedManager *EnhancedCredentialManager) {
	fmt.Printf("\n   🔄 Demonstrating real AWS credential handling\n")

	ctx := context.Background()

	// Test credential operations for each customer
	for customerCode := range enhancedManager.customerManager.CustomerMappings {
		fmt.Printf("      🏢 Processing customer: %s\n", customerCode)

		// Get SES client for customer
		sesClient, err := enhancedManager.GetCustomerSESClient(ctx, customerCode)
		if err != nil {
			fmt.Printf("         ⚠️  Failed to get SES client (expected in demo): %v\n", err)
			continue
		}

		fmt.Printf("         ✅ SES client created successfully\n")

		// Test customer access
		err = enhancedManager.TestCustomerAccess(ctx, customerCode)
		if err != nil {
			fmt.Printf("         ⚠️  Access test failed (expected in demo): %v\n", err)
		} else {
			fmt.Printf("         ✅ Customer access test passed\n")
		}
	}

	// Demo credential caching
	fmt.Printf("\n   💾 Credential caching demonstration:\n")
	cacheStatus := enhancedManager.GetCacheStatus()
	fmt.Printf("      Cached credentials: %d\n", len(cacheStatus))

	for key, expiresAt := range cacheStatus {
		fmt.Printf("      %s: expires %v\n", key, expiresAt.Format("15:04:05"))
	}

	// Demo metrics
	metrics := enhancedManager.GetCredentialMetrics()
	fmt.Printf("\n   📊 Credential metrics:\n")
	fmt.Printf("      Total cached: %v\n", metrics["total_cached"])
	fmt.Printf("      Expired count: %v\n", metrics["expired_count"])
	fmt.Printf("      Expiring count: %v\n", metrics["expiring_count"])
	fmt.Printf("      Total customers: %v\n", metrics["total_customers"])
	fmt.Printf("      Cache hit ratio: %.1f%%\n", metrics["cache_hit_ratio"].(float64)*100)
}

func demoEnhancedSQSIntegration() {
	fmt.Printf("📨 Enhanced SQS Integration Demo\n")

	// Create SQS processor configuration
	config := SQSProcessorConfig{
		QueueURL:           "https://sqs.us-east-1.amazonaws.com/123456789012/email-distribution-queue",
		Region:             "us-east-1",
		MaxMessages:        10,
		VisibilityTimeout:  30,
		WaitTimeSeconds:    20,
		PollingInterval:    5 * time.Second,
		WorkerPoolSize:     5,
		MessageBufferSize:  50,
		ShutdownTimeout:    30 * time.Second,
		EnableLongPolling:  true,
		EnableBatchDelete:  true,
		DeadLetterQueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/email-distribution-dlq",
	}

	fmt.Printf("   ⚙️  SQS Configuration:\n")
	fmt.Printf("      Queue URL: %s\n", config.QueueURL)
	fmt.Printf("      Worker Pool Size: %d\n", config.WorkerPoolSize)
	fmt.Printf("      Polling Interval: %v\n", config.PollingInterval)
	fmt.Printf("      Message Buffer: %d\n", config.MessageBufferSize)
	fmt.Printf("      Shutdown Timeout: %v\n", config.ShutdownTimeout)
	fmt.Printf("      Long Polling: %t\n", config.EnableLongPolling)
	fmt.Printf("      Dead Letter Queue: %s\n", config.DeadLetterQueueURL)

	// Create required components
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			Environment:  "production",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Global",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "production",
		},
	}

	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "EmailDistribution",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	// Create enhanced SQS processor
	processor, err := NewEnhancedSQSProcessor(
		config,
		customerManager,
		templateManager,
		sesManager,
		statusTracker,
		errorHandler,
		monitoringSystem,
	)

	if err != nil {
		fmt.Printf("   ⚠️  Failed to create SQS processor (expected in demo): %v\n", err)
		fmt.Printf("   📝 In production, this would connect to real SQS queues\n")

		// Continue with simulation
		demoSQSProcessingSimulation(processor)
		return
	}

	fmt.Printf("   ✅ Enhanced SQS processor created successfully\n")

	// Demo SQS operations
	demoSQSOperations(processor)
}

func demoSQSProcessingSimulation(processor *EnhancedSQSProcessor) {
	fmt.Printf("\n   🔄 Simulating SQS message processing\n")

	// Create sample SQS messages
	sampleMessages := []SQSMessage{
		{
			ChangeID:      "SQS-DEMO-001",
			Title:         "Monthly Newsletter Distribution",
			Description:   "Distribute monthly newsletter to all customers",
			CustomerCodes: []string{"hts", "cds"},
			TemplateID:    "newsletter",
			Priority:      "normal",
			TemplateData: map[string]interface{}{
				"subject":    "Monthly Newsletter - December 2024",
				"title":      "December Updates",
				"message":    "Welcome to our December newsletter!",
				"highlights": []string{"New features", "Customer stories", "Upcoming events"},
			},
			ScheduledAt: time.Now(),
		},
		{
			ChangeID:      "SQS-DEMO-002",
			Title:         "Security Alert Notification",
			Description:   "Critical security alert for all customers",
			CustomerCodes: []string{"hts", "cds"},
			TemplateID:    "security-alert",
			Priority:      "high",
			TemplateData: map[string]interface{}{
				"subject":     "URGENT: Security Alert",
				"alert_type":  "Security Vulnerability",
				"severity":    "High",
				"description": "A security vulnerability has been identified",
				"action":      "Please update your systems immediately",
			},
			ScheduledAt: time.Now(),
		},
		{
			ChangeID:      "SQS-DEMO-003",
			Title:         "Customer Onboarding Welcome",
			Description:   "Welcome email for new customer onboarding",
			CustomerCodes: []string{"hts"},
			TemplateID:    "welcome",
			Priority:      "normal",
			TemplateData: map[string]interface{}{
				"subject":       "Welcome to Email Distribution System",
				"customer_name": "HTS Production",
				"setup_date":    time.Now().Format("2006-01-02"),
				"contact_email": "support@emailsystem.example.com",
			},
			ScheduledAt: time.Now(),
		},
	}

	fmt.Printf("   📨 Processing %d sample messages:\n", len(sampleMessages))

	for i, message := range sampleMessages {
		fmt.Printf("      %d. %s (Priority: %s)\n", i+1, message.Title, message.Priority)
		fmt.Printf("         Change ID: %s\n", message.ChangeID)
		fmt.Printf("         Customers: %v\n", message.CustomerCodes)
		fmt.Printf("         Template: %s\n", message.TemplateID)

		// Validate message
		if processor != nil {
			err := processor.validateSQSMessage(&message)
			if err != nil {
				fmt.Printf("         ❌ Validation failed: %v\n", err)
			} else {
				fmt.Printf("         ✅ Message validation passed\n")
			}

			// Simulate processing
			err = processor.processEmailDistribution(&message)
			if err != nil {
				fmt.Printf("         ❌ Processing failed: %v\n", err)
			} else {
				fmt.Printf("         ✅ Processing completed successfully\n")
			}
		} else {
			fmt.Printf("         ⚠️  Processor not available, skipping validation\n")
		}

		fmt.Printf("\n")
	}

	// Show processing metrics
	if processor != nil {
		fmt.Printf("   📊 Processing Metrics:\n")
		metrics := processor.GetMetrics()
		fmt.Printf("      Messages Processed: %d\n", metrics.MessagesProcessed)
		fmt.Printf("      Messages Succeeded: %d\n", metrics.MessagesSucceeded)
		fmt.Printf("      Messages Failed: %d\n", metrics.MessagesFailed)
		fmt.Printf("      Error Rate: %.1f%%\n", metrics.ErrorRate)
		fmt.Printf("      Current Workers: %d\n", metrics.CurrentWorkers)
	}
}

func demoSQSOperations(processor *EnhancedSQSProcessor) {
	fmt.Printf("\n   🔄 Demonstrating SQS operations\n")

	// Check if processor is running
	fmt.Printf("      Processor running: %t\n", processor.IsRunning())

	// Get queue depth (would fail in demo without real queue)
	depth, err := processor.GetQueueDepth()
	if err != nil {
		fmt.Printf("      ⚠️  Queue depth check failed (expected in demo): %v\n", err)
	} else {
		fmt.Printf("      Queue depth: %d messages\n", depth)
	}

	// Demo graceful shutdown
	fmt.Printf("      Testing graceful shutdown...\n")
	err = processor.GracefulShutdown()
	if err != nil {
		fmt.Printf("      ⚠️  Graceful shutdown failed: %v\n", err)
	} else {
		fmt.Printf("      ✅ Graceful shutdown completed\n")
	}
}

func demoCombinedWorkflow() {
	fmt.Printf("🔄 Combined Workflow Demo - SQS + Cross-Account Credentials\n")

	// Create a complete workflow demonstration
	fmt.Printf("   📋 Workflow: SQS Message → Cross-Account Role → Email Distribution\n")

	// Step 1: Receive SQS message
	fmt.Printf("\n   1️⃣  Step 1: Receive SQS Message\n")
	sqsMessage := SQSMessage{
		ChangeID:      "WORKFLOW-001",
		Title:         "Cross-Account Email Distribution",
		Description:   "Demonstrate cross-account email distribution workflow",
		CustomerCodes: []string{"hts", "cds"},
		TemplateID:    "notification",
		Priority:      "normal",
		TemplateData: map[string]interface{}{
			"subject": "Cross-Account Distribution Test",
			"message": "Testing cross-account email distribution workflow",
		},
		ScheduledAt: time.Now(),
	}

	fmt.Printf("      📨 Message received: %s\n", sqsMessage.Title)
	fmt.Printf("      🎯 Target customers: %v\n", sqsMessage.CustomerCodes)

	// Step 2: Validate customer access
	fmt.Printf("\n   2️⃣  Step 2: Validate Customer Access\n")
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			Environment:  "production",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Global",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "production",
		},
	}

	for _, customerCode := range sqsMessage.CustomerCodes {
		accountInfo, err := customerManager.GetCustomerAccountInfo(customerCode)
		if err != nil {
			fmt.Printf("      ❌ Customer %s: Invalid - %v\n", customerCode, err)
			continue
		}

		fmt.Printf("      ✅ Customer %s: Valid\n", customerCode)
		fmt.Printf("         Account: %s (%s)\n", accountInfo.AWSAccountID, accountInfo.CustomerName)
		fmt.Printf("         Region: %s\n", accountInfo.Region)
		fmt.Printf("         SES Role: %s\n", accountInfo.SESRoleARN)
	}

	// Step 3: Assume cross-account roles
	fmt.Printf("\n   3️⃣  Step 3: Assume Cross-Account Roles\n")
	for _, customerCode := range sqsMessage.CustomerCodes {
		fmt.Printf("      🔐 Assuming SES role for customer %s...\n", customerCode)

		credentials, err := customerManager.AssumeCustomerRole(customerCode, "ses")
		if err != nil {
			fmt.Printf("         ❌ Role assumption failed: %v\n", err)
			continue
		}

		fmt.Printf("         ✅ Role assumed successfully\n")
		fmt.Printf("         Credentials expire: %v\n", credentials.Expiration.Format("15:04:05"))

		// Validate credentials
		validation, err := customerManager.ValidateCredentials(credentials)
		if err != nil {
			fmt.Printf("         ⚠️  Validation failed: %v\n", err)
		} else {
			fmt.Printf("         ✅ Credentials validated\n")
		}
	}

	// Step 4: Process email distribution
	fmt.Printf("\n   4️⃣  Step 4: Process Email Distribution\n")
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)

	// Start execution tracking
	execution, err := statusTracker.StartExecution(
		sqsMessage.ChangeID,
		sqsMessage.Title,
		sqsMessage.Description,
		"workflow-demo",
		sqsMessage.CustomerCodes,
	)

	if err != nil {
		fmt.Printf("      ❌ Failed to start execution tracking: %v\n", err)
	} else {
		fmt.Printf("      ✅ Execution tracking started: %s\n", execution.ExecutionID)
	}

	// Process each customer
	for _, customerCode := range sqsMessage.CustomerCodes {
		fmt.Printf("      📧 Processing customer %s...\n", customerCode)

		if execution != nil {
			statusTracker.StartCustomerExecution(execution.ExecutionID, customerCode)

			// Simulate processing steps
			steps := []string{"validate", "render", "send", "verify"}
			for _, step := range steps {
				statusTracker.AddExecutionStep(execution.ExecutionID, customerCode, step,
					fmt.Sprintf("Step: %s", step), fmt.Sprintf("Processing %s step", step))
				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, step, StepStatusRunning, "")

				// Simulate processing time
				time.Sleep(50 * time.Millisecond)

				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, step, StepStatusCompleted, "")
			}

			statusTracker.CompleteCustomerExecution(execution.ExecutionID, customerCode, true, "")
		}

		fmt.Printf("         ✅ Customer %s processed successfully\n", customerCode)
	}

	// Step 5: Complete and report
	fmt.Printf("\n   5️⃣  Step 5: Complete and Report\n")
	if execution != nil {
		executions, _ := statusTracker.QueryExecutions(ExecutionQuery{Limit: 1})
		if len(executions) > 0 {
			exec := executions[0]
			fmt.Printf("      📊 Execution Summary:\n")
			fmt.Printf("         Execution ID: %s\n", exec.ExecutionID)
			fmt.Printf("         Status: %s\n", exec.Status)
			fmt.Printf("         Customers: %d\n", len(exec.CustomerStatuses))

			successCount := 0
			for customerCode, customerStatus := range exec.CustomerStatuses {
				if customerStatus.Status == CustomerStatusCompleted {
					successCount++
				}
				fmt.Printf("         %s: %s\n", customerCode, customerStatus.Status)
			}

			fmt.Printf("         Success Rate: %d/%d (%.1f%%)\n",
				successCount, len(exec.CustomerStatuses),
				float64(successCount)/float64(len(exec.CustomerStatuses))*100)
		}
	}

	fmt.Printf("\n   ✅ Combined workflow demonstration complete!\n")
	fmt.Printf("   📝 This workflow shows how SQS messages trigger cross-account\n")
	fmt.Printf("      email distribution with proper credential management.\n")
}

// Helper function to extract account ID from role ARN
func extractAccountIDFromRoleARN(roleARN string) string {
	// Extract account ID from role ARN: arn:aws:iam::123456789012:role/RoleName
	parts := strings.Split(roleARN, ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return "unknown"
}
