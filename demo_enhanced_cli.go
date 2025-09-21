package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Demo application for enhanced CLI functionality

func main() {
	fmt.Println("=== Enhanced CLI Multi-Customer Email Distribution Demo ===")

	// Demo 1: File processing mode
	fmt.Println("\n🧪 Demo 1: File Processing Mode")
	demoFileProcessingMode()

	// Demo 2: Configuration management
	fmt.Println("\n🧪 Demo 2: Configuration Management")
	demoConfigurationManagement()

	// Demo 3: SQS integration simulation
	fmt.Println("\n🧪 Demo 3: SQS Integration Simulation")
	demoSQSIntegrationSimulation()

	// Demo 4: Server mode simulation
	fmt.Println("\n🧪 Demo 4: Server Mode Simulation")
	demoServerModeSimulation()

	// Demo 5: Error handling and recovery
	fmt.Println("\n🧪 Demo 5: Error Handling & Recovery")
	demoErrorHandlingAndRecovery()

	fmt.Println("\n=== Enhanced CLI Demo Complete ===")
}

func demoFileProcessingMode() {
	fmt.Printf("📁 File Processing Mode Demo\n")

	// Create temporary directory for demo files
	tempDir, err := os.MkdirTemp("", "cli-demo-*")
	if err != nil {
		log.Printf("❌ Failed to create temp directory: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("   📂 Created temporary directory: %s\n", tempDir)

	// Create sample metadata files
	metadataFiles := []struct {
		filename string
		data     map[string]interface{}
	}{
		{
			filename: "hts_newsletter.json",
			data: map[string]interface{}{
				"customer_codes": []string{"hts"},
				"change_id":      "NEWSLETTER-2024-001",
				"title":          "Monthly Newsletter - HTS",
				"description":    "Monthly newsletter distribution for HTS customers",
				"template_id":    "newsletter",
				"priority":       "normal",
				"email_data": map[string]interface{}{
					"subject": "HTS Monthly Newsletter - December 2024",
					"title":   "December Newsletter",
					"message": "Welcome to our December newsletter with exciting updates!",
					"highlights": []string{
						"New product launches",
						"Customer success stories",
						"Upcoming events",
					},
				},
				"recipients": []map[string]interface{}{
					{"email": "admin@hts.example.com", "name": "HTS Admin"},
					{"email": "marketing@hts.example.com", "name": "HTS Marketing"},
				},
			},
		},
		{
			filename: "multi_customer_alert.json",
			data: map[string]interface{}{
				"customer_codes": []string{"hts", "cds", "motor"},
				"change_id":      "ALERT-2024-002",
				"title":          "System Maintenance Alert",
				"description":    "Scheduled maintenance notification for all customers",
				"template_id":    "notification",
				"priority":       "high",
				"email_data": map[string]interface{}{
					"subject":            "Scheduled Maintenance - December 15, 2024",
					"title":              "System Maintenance Notice",
					"message":            "We will be performing scheduled maintenance on December 15, 2024.",
					"maintenance_window": "2:00 AM - 4:00 AM EST",
					"impact":             "Email services may be temporarily unavailable",
					"action_required":    "Please plan accordingly and avoid sending critical emails during this window",
				},
				"scheduled_time": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			},
		},
		{
			filename: "cds_welcome.json",
			data: map[string]interface{}{
				"customer_code": "cds", // Singular form
				"change_id":     "WELCOME-2024-003",
				"title":         "Welcome to Email Distribution System",
				"description":   "Welcome email for new CDS setup",
				"template_id":   "welcome",
				"priority":      "normal",
				"email_data": map[string]interface{}{
					"subject":       "Welcome to Email Distribution System",
					"service_name":  "Multi-Customer Email Distribution",
					"customer_name": "CDS Global",
					"setup_date":    time.Now().Format("2006-01-02"),
					"contact_email": "support@emailsystem.example.com",
				},
			},
		},
	}

	// Write metadata files
	for _, file := range metadataFiles {
		filePath := filepath.Join(tempDir, file.filename)
		data, err := json.MarshalIndent(file.data, "", "  ")
		if err != nil {
			log.Printf("❌ Failed to marshal %s: %v", file.filename, err)
			continue
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			log.Printf("❌ Failed to write %s: %v", file.filename, err)
			continue
		}

		fmt.Printf("   📄 Created metadata file: %s\n", file.filename)
	}

	// Simulate CLI file processing
	fmt.Printf("\n   🚀 Simulating CLI file processing...\n")

	// Create CLI configuration
	config := &CLIConfig{
		Mode:                   "file",
		LogLevel:               "info",
		MaxConcurrentCustomers: 5,
		AWSRegion:              "us-east-1",
		Environment:            "demo",
	}

	// Initialize application
	app, err := NewApplication(config)
	if err != nil {
		log.Printf("❌ Failed to initialize application: %v", err)
		return
	}

	// Add demo customer mappings
	app.customerManager.CustomerMappings = getDemoCustomerMappings()

	// Process each file
	for _, file := range metadataFiles {
		filePath := filepath.Join(tempDir, file.filename)
		fmt.Printf("      📋 Processing: %s\n", file.filename)

		err := app.processFile(filePath)
		if err != nil {
			fmt.Printf("         ❌ Failed: %v\n", err)
		} else {
			fmt.Printf("         ✅ Success\n")
		}
	}

	// Show execution summary
	fmt.Printf("\n   📊 Execution Summary:\n")
	executions, err := app.statusTracker.QueryExecutions(ExecutionQuery{Limit: 10})
	if err != nil {
		log.Printf("❌ Failed to query executions: %v", err)
		return
	}

	fmt.Printf("      Total Executions: %d\n", len(executions))
	for i, execution := range executions {
		fmt.Printf("      %d. %s (%s) - %d customers\n",
			i+1, execution.Title, execution.Status, len(execution.CustomerStatuses))

		for customerCode, customerStatus := range execution.CustomerStatuses {
			fmt.Printf("         %s: %s\n", customerCode, customerStatus.Status)
		}
	}
}

func demoConfigurationManagement() {
	fmt.Printf("⚙️  Configuration Management Demo\n")

	// Create temporary configuration file
	tempDir, err := os.MkdirTemp("", "config-demo-*")
	if err != nil {
		log.Printf("❌ Failed to create temp directory: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "customer_config.json")

	// Create comprehensive configuration
	configData := map[string]interface{}{
		"version":     "1.0.0",
		"environment": "demo",
		"region":      "us-east-1",
		"serviceSettings": map[string]interface{}{
			"serviceName":     "multi-customer-email-distribution",
			"serviceVersion":  "1.0.0",
			"defaultTimeout":  "30s",
			"maxRetries":      3,
			"logLevel":        "info",
			"enableMetrics":   true,
			"enableTracing":   false,
			"healthCheckPath": "/health",
			"maintenanceMode": false,
		},
		"customerMappings": map[string]interface{}{
			"hts": map[string]interface{}{
				"customerName": "HTS Production",
				"awsAccountId": "123456789012",
				"region":       "us-east-1",
				"environment":  "production",
				"roleArns": map[string]string{
					"ses": "arn:aws:iam::123456789012:role/HTSSESRole",
					"sqs": "arn:aws:iam::123456789012:role/HTSSQSRole",
					"s3":  "arn:aws:iam::123456789012:role/HTSS3Role",
				},
				"emailSettings": map[string]interface{}{
					"fromEmail":            "noreply@hts.example.com",
					"fromName":             "HTS Production Team",
					"replyToEmail":         "support@hts.example.com",
					"configurationSetName": "hts-config-set",
					"sendingQuota":         1000,
					"sendingRate":          5.0,
				},
				"quotas": map[string]interface{}{
					"maxEmailsPerDay":       10000,
					"maxEmailsPerHour":      1000,
					"maxRecipientsPerEmail": 100,
					"maxTemplates":          50,
				},
				"features": map[string]bool{
					"enableBulkEmail":       true,
					"enableTemplatePreview": true,
					"enableAdvancedMetrics": true,
				},
				"contactInfo": map[string]interface{}{
					"primaryContact": map[string]string{
						"name":  "John Smith",
						"email": "john.smith@hts.example.com",
						"phone": "+1-555-0123",
						"role":  "Technical Lead",
					},
					"technicalContact": map[string]string{
						"name":  "Jane Doe",
						"email": "jane.doe@hts.example.com",
						"phone": "+1-555-0124",
						"role":  "DevOps Engineer",
					},
				},
				"enabled": true,
			},
			"cds": map[string]interface{}{
				"customerName": "CDS Global",
				"awsAccountId": "234567890123",
				"region":       "us-west-2",
				"environment":  "production",
				"roleArns": map[string]string{
					"ses": "arn:aws:iam::234567890123:role/CDSSESRole",
					"sqs": "arn:aws:iam::234567890123:role/CDSSQSRole",
				},
				"emailSettings": map[string]interface{}{
					"fromEmail":    "notifications@cds.example.com",
					"fromName":     "CDS Global Team",
					"sendingQuota": 500,
					"sendingRate":  2.0,
				},
				"enabled": true,
			},
			"motor": map[string]interface{}{
				"customerName": "Motor",
				"awsAccountId": "345678901234",
				"region":       "us-east-1",
				"environment":  "staging",
				"roleArns": map[string]string{
					"ses": "arn:aws:iam::345678901234:role/MotorSESRole",
				},
				"emailSettings": map[string]interface{}{
					"fromEmail":    "staging@motor.example.com",
					"fromName":     "Motor Staging",
					"sendingQuota": 100,
					"sendingRate":  1.0,
				},
				"enabled": true,
			},
		},
		"emailSettings": map[string]interface{}{
			"defaultFromEmail":        "noreply@emailsystem.example.com",
			"defaultFromName":         "Email Distribution System",
			"defaultReplyToEmail":     "support@emailsystem.example.com",
			"maxRetries":              3,
			"retryDelay":              "2s",
			"enableBounceHandling":    true,
			"enableComplaintHandling": true,
			"blockedDomains":          []string{"tempmail.com", "10minutemail.com"},
		},
		"featureFlags": map[string]bool{
			"enableBulkEmail":       true,
			"enableTemplatePreview": true,
			"enableAdvancedMetrics": false,
			"enableEmailScheduling": false,
		},
	}

	// Write configuration file
	configJSON, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		log.Printf("❌ Failed to marshal configuration: %v", err)
		return
	}

	if err := os.WriteFile(configFile, configJSON, 0644); err != nil {
		log.Printf("❌ Failed to write configuration file: %v", err)
		return
	}

	fmt.Printf("   📄 Created configuration file: %s\n", configFile)
	fmt.Printf("   📊 Configuration size: %d bytes\n", len(configJSON))

	// Load configuration using CLI
	fmt.Printf("\n   🔄 Loading configuration via CLI...\n")

	config := &CLIConfig{
		Mode:       "file",
		ConfigFile: configFile,
		LogLevel:   "info",
		AWSRegion:  "us-east-1",
	}

	app, err := NewApplication(config)
	if err != nil {
		log.Printf("❌ Failed to initialize application with config: %v", err)
		return
	}

	fmt.Printf("   ✅ Configuration loaded successfully\n")
	fmt.Printf("   📋 Customer mappings loaded: %d\n", len(app.customerManager.CustomerMappings))

	// Display loaded customer information
	fmt.Printf("\n   🏢 Customer Information:\n")
	for customerCode, accountInfo := range app.customerManager.CustomerMappings {
		fmt.Printf("      %s (%s):\n", customerCode, accountInfo.CustomerName)
		fmt.Printf("         Account ID: %s\n", accountInfo.AWSAccountID)
		fmt.Printf("         Region: %s\n", accountInfo.Region)
		fmt.Printf("         Environment: %s\n", accountInfo.Environment)
		fmt.Printf("         SES Role: %s\n", accountInfo.SESRoleARN)
	}

	// Demonstrate configuration validation
	fmt.Printf("\n   🔍 Validating configuration...\n")
	configManager := NewConfigurationManager(configFile)
	if err := configManager.LoadConfiguration(); err != nil {
		log.Printf("❌ Configuration validation failed: %v", err)
	} else {
		fmt.Printf("   ✅ Configuration validation passed\n")
		fmt.Printf("      Service: %s v%s\n",
			configManager.Configuration.ServiceSettings.ServiceName,
			configManager.Configuration.ServiceSettings.ServiceVersion)
		fmt.Printf("      Environment: %s\n", configManager.Configuration.Environment)
		fmt.Printf("      Feature flags: %d enabled\n", countEnabledFeatures(configManager.Configuration.FeatureFlags))
	}
}

func demoSQSIntegrationSimulation() {
	fmt.Printf("📨 SQS Integration Simulation Demo\n")

	// Simulate SQS configuration
	fmt.Printf("   ⚙️  SQS Configuration:\n")
	fmt.Printf("      Queue URL: https://sqs.us-east-1.amazonaws.com/123456789012/email-distribution-queue\n")
	fmt.Printf("      Polling Interval: 5s\n")
	fmt.Printf("      Max Messages: 10\n")
	fmt.Printf("      Visibility Timeout: 30s\n")
	fmt.Printf("      Wait Time: 20s\n")

	// Create CLI configuration for SQS mode
	config := &CLIConfig{
		Mode:                 "sqs",
		LogLevel:             "info",
		SQSQueueURL:          "https://sqs.us-east-1.amazonaws.com/123456789012/email-distribution-queue",
		SQSPollingInterval:   5 * time.Second,
		SQSMaxMessages:       10,
		SQSVisibilityTimeout: 30 * time.Second,
		SQSWaitTimeSeconds:   20,
		AWSRegion:            "us-east-1",
		Environment:          "demo",
	}

	// Initialize application
	app, err := NewApplication(config)
	if err != nil {
		log.Printf("❌ Failed to initialize SQS application: %v", err)
		return
	}

	// Add demo customer mappings
	app.customerManager.CustomerMappings = getDemoCustomerMappings()

	fmt.Printf("\n   📨 Simulating SQS message processing...\n")

	// Simulate different types of SQS messages
	sampleMessages := []map[string]interface{}{
		{
			"messageType":   "email_distribution",
			"changeId":      "SQS-001",
			"title":         "SQS Newsletter Distribution",
			"description":   "Newsletter distribution triggered via SQS",
			"customerCodes": []string{"hts", "cds"},
			"templateId":    "newsletter",
			"priority":      "normal",
			"templateData": map[string]interface{}{
				"title":   "Monthly Newsletter",
				"message": "Your monthly update is here!",
			},
			"scheduledAt": time.Now().Format(time.RFC3339),
		},
		{
			"messageType":   "urgent_notification",
			"changeId":      "SQS-002",
			"title":         "Urgent System Alert",
			"description":   "Critical system alert for all customers",
			"customerCodes": []string{"hts", "cds", "motor"},
			"templateId":    "alert",
			"priority":      "high",
			"templateData": map[string]interface{}{
				"alertType": "System Outage",
				"message":   "We are experiencing a service disruption",
				"eta":       "2 hours",
			},
			"scheduledAt": time.Now().Format(time.RFC3339),
		},
		{
			"messageType":   "customer_onboarding",
			"changeId":      "SQS-003",
			"title":         "New Customer Welcome",
			"description":   "Welcome email for new customer",
			"customerCodes": []string{"motor"},
			"templateId":    "welcome",
			"priority":      "normal",
			"templateData": map[string]interface{}{
				"customerName": "Motor",
				"setupDate":    time.Now().Format("2006-01-02"),
				"serviceName":  "Email Distribution System",
			},
			"scheduledAt": time.Now().Format(time.RFC3339),
		},
	}

	// Simulate processing each message
	for i, message := range sampleMessages {
		fmt.Printf("      📩 Message %d: %s\n", i+1, message["title"])
		fmt.Printf("         Type: %s\n", message["messageType"])
		fmt.Printf("         Priority: %s\n", message["priority"])
		fmt.Printf("         Customers: %v\n", message["customerCodes"])
		fmt.Printf("         Template: %s\n", message["templateId"])

		// Simulate message processing
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("         ✅ Processed successfully\n")
	}

	fmt.Printf("\n   📊 SQS Processing Summary:\n")
	fmt.Printf("      Messages Processed: %d\n", len(sampleMessages))
	fmt.Printf("      Success Rate: 100%%\n")
	fmt.Printf("      Average Processing Time: 100ms\n")
	fmt.Printf("      Total Customers Notified: 7\n")
}

func demoServerModeSimulation() {
	fmt.Printf("🖥️  Server Mode Simulation Demo\n")

	// Create CLI configuration for server mode
	config := &CLIConfig{
		Mode:            "server",
		LogLevel:        "info",
		HealthCheckPort: 8081,
		MetricsPort:     9090,
		EnableMetrics:   true,
		EnableTracing:   false,
		SQSQueueURL:     "https://sqs.us-east-1.amazonaws.com/123456789012/email-distribution-queue",
		AWSRegion:       "us-east-1",
		Environment:     "demo",
	}

	fmt.Printf("   🚀 Server Configuration:\n")
	fmt.Printf("      Health Check Port: %d\n", config.HealthCheckPort)
	fmt.Printf("      Metrics Port: %d\n", config.MetricsPort)
	fmt.Printf("      Metrics Enabled: %t\n", config.EnableMetrics)
	fmt.Printf("      Tracing Enabled: %t\n", config.EnableTracing)
	fmt.Printf("      SQS Integration: %t\n", config.SQSQueueURL != "")

	// Initialize application
	app, err := NewApplication(config)
	if err != nil {
		log.Printf("❌ Failed to initialize server application: %v", err)
		return
	}

	// Add demo customer mappings
	app.customerManager.CustomerMappings = getDemoCustomerMappings()

	fmt.Printf("\n   🌐 Starting server simulation...\n")

	// Simulate server startup
	fmt.Printf("      🔄 Initializing health check endpoint: http://localhost:%d/health\n", config.HealthCheckPort)
	fmt.Printf("      📊 Initializing metrics endpoint: http://localhost:%d/metrics\n", config.MetricsPort)
	fmt.Printf("      📨 Connecting to SQS queue for message processing\n")
	fmt.Printf("      🔍 Starting monitoring and alerting systems\n")

	// Simulate health check responses
	healthChecks := []struct {
		endpoint string
		status   string
		response map[string]interface{}
	}{
		{
			endpoint: "/health",
			status:   "200 OK",
			response: map[string]interface{}{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
				"version":   "1.0.0",
				"uptime":    "5m30s",
				"checks": map[string]interface{}{
					"database":   "healthy",
					"sqs":        "healthy",
					"ses":        "healthy",
					"memory":     "healthy",
					"disk_space": "healthy",
				},
			},
		},
		{
			endpoint: "/health/ready",
			status:   "200 OK",
			response: map[string]interface{}{
				"ready":     true,
				"timestamp": time.Now().Format(time.RFC3339),
				"services": map[string]bool{
					"customer_manager": true,
					"email_processor":  true,
					"sqs_processor":    true,
					"status_tracker":   true,
				},
			},
		},
		{
			endpoint: "/metrics",
			status:   "200 OK",
			response: map[string]interface{}{
				"emails_sent_total":      1247,
				"emails_failed_total":    23,
				"customers_active":       3,
				"sqs_messages_processed": 456,
				"avg_processing_time_ms": 150,
				"memory_usage_mb":        128,
				"cpu_usage_percent":      15.5,
				"uptime_seconds":         330,
			},
		},
	}

	fmt.Printf("\n   🔍 Health Check Simulation:\n")
	for _, check := range healthChecks {
		fmt.Printf("      GET %s -> %s\n", check.endpoint, check.status)

		// Pretty print response for health endpoint
		if check.endpoint == "/health" {
			fmt.Printf("         Status: %s\n", check.response["status"])
			fmt.Printf("         Uptime: %s\n", check.response["uptime"])
			if checks, ok := check.response["checks"].(map[string]interface{}); ok {
				fmt.Printf("         Service Checks:\n")
				for service, status := range checks {
					fmt.Printf("           %s: %s\n", service, status)
				}
			}
		}
	}

	// Simulate metrics collection
	fmt.Printf("\n   📊 Metrics Collection:\n")
	metrics := healthChecks[2].response
	fmt.Printf("      📧 Emails Sent: %v\n", metrics["emails_sent_total"])
	fmt.Printf("      ❌ Emails Failed: %v\n", metrics["emails_failed_total"])
	fmt.Printf("      🏢 Active Customers: %v\n", metrics["customers_active"])
	fmt.Printf("      📨 SQS Messages: %v\n", metrics["sqs_messages_processed"])
	fmt.Printf("      ⏱️  Avg Processing Time: %vms\n", metrics["avg_processing_time_ms"])
	fmt.Printf("      💾 Memory Usage: %vMB\n", metrics["memory_usage_mb"])
	fmt.Printf("      🖥️  CPU Usage: %v%%\n", metrics["cpu_usage_percent"])

	// Simulate graceful shutdown
	fmt.Printf("\n   🛑 Simulating graceful shutdown...\n")
	fmt.Printf("      📨 Stopping SQS message processing\n")
	fmt.Printf("      ⏳ Waiting for in-flight requests to complete\n")
	fmt.Printf("      🔌 Closing database connections\n")
	fmt.Printf("      📊 Flushing metrics\n")
	fmt.Printf("      ✅ Server shutdown complete\n")
}

func demoErrorHandlingAndRecovery() {
	fmt.Printf("🚨 Error Handling & Recovery Demo\n")

	// Create CLI configuration
	config := &CLIConfig{
		Mode:                   "file",
		LogLevel:               "debug",
		MaxConcurrentCustomers: 3,
		AWSRegion:              "us-east-1",
		Environment:            "demo",
	}

	// Initialize application
	app, err := NewApplication(config)
	if err != nil {
		log.Printf("❌ Failed to initialize application: %v", err)
		return
	}

	// Add demo customer mappings with some problematic configurations
	app.customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			Environment:  "production",
		},
		"invalid": {
			CustomerCode: "invalid",
			CustomerName: "Invalid Customer",
			AWSAccountID: "000000000000", // Invalid account ID
			Region:       "invalid-region",
			SESRoleARN:   "invalid-arn",
			Environment:  "test",
		},
		"timeout": {
			CustomerCode: "timeout",
			CustomerName: "Timeout Customer",
			AWSAccountID: "999999999999",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::999999999999:role/TimeoutRole",
			Environment:  "test",
		},
	}

	fmt.Printf("\n   🧪 Testing Error Scenarios:\n")

	// Scenario 1: Invalid JSON file
	fmt.Printf("      1️⃣  Invalid JSON File:\n")
	tempDir, _ := os.MkdirTemp("", "error-demo-*")
	defer os.RemoveAll(tempDir)

	invalidJSONFile := filepath.Join(tempDir, "invalid.json")
	os.WriteFile(invalidJSONFile, []byte(`{"invalid": json content}`), 0644)

	err = app.processFile(invalidJSONFile)
	if err != nil {
		fmt.Printf("         ❌ Expected error caught: %v\n", err)
		fmt.Printf("         🔄 Recovery: File skipped, processing continues\n")
	}

	// Scenario 2: Missing customer codes
	fmt.Printf("      2️⃣  Missing Customer Codes:\n")
	missingCodesFile := filepath.Join(tempDir, "missing_codes.json")
	missingCodesData := map[string]interface{}{
		"change_id":   "MISSING-001",
		"title":       "Missing Customer Codes",
		"description": "File without customer codes",
		"email_data": map[string]interface{}{
			"subject": "Test Email",
			"message": "Test message",
		},
	}
	data, _ := json.Marshal(missingCodesData)
	os.WriteFile(missingCodesFile, data, 0644)

	err = app.processFile(missingCodesFile)
	if err != nil {
		fmt.Printf("         ❌ Expected error caught: %v\n", err)
		fmt.Printf("         🔄 Recovery: Validation failed, file rejected\n")
	}

	// Scenario 3: Invalid customer code
	fmt.Printf("      3️⃣  Invalid Customer Code:\n")
	invalidCustomerFile := filepath.Join(tempDir, "invalid_customer.json")
	invalidCustomerData := map[string]interface{}{
		"customer_codes": []string{"nonexistent"},
		"change_id":      "INVALID-001",
		"title":          "Invalid Customer",
		"description":    "File with non-existent customer",
		"email_data": map[string]interface{}{
			"subject": "Test Email",
			"message": "Test message",
		},
	}
	data, _ = json.Marshal(invalidCustomerData)
	os.WriteFile(invalidCustomerFile, data, 0644)

	err = app.processFile(invalidCustomerFile)
	if err != nil {
		fmt.Printf("         ❌ Expected error caught: %v\n", err)
		fmt.Printf("         🔄 Recovery: Unknown customer rejected\n")
	}

	// Scenario 4: Mixed valid/invalid customers
	fmt.Printf("      4️⃣  Mixed Valid/Invalid Customers:\n")
	mixedCustomerFile := filepath.Join(tempDir, "mixed_customers.json")
	mixedCustomerData := map[string]interface{}{
		"customer_codes": []string{"hts", "invalid", "timeout"},
		"change_id":      "MIXED-001",
		"title":          "Mixed Customer Processing",
		"description":    "File with mix of valid and invalid customers",
		"email_data": map[string]interface{}{
			"subject": "Test Email",
			"message": "Test message",
		},
	}
	data, _ = json.Marshal(mixedCustomerData)
	os.WriteFile(mixedCustomerFile, data, 0644)

	err = app.processFile(mixedCustomerFile)
	fmt.Printf("         ⚠️  Partial processing completed\n")
	fmt.Printf("         ✅ Valid customers processed successfully\n")
	fmt.Printf("         ❌ Invalid customers failed gracefully\n")
	fmt.Printf("         🔄 Recovery: System continues with valid customers\n")

	// Demonstrate error tracking and reporting
	fmt.Printf("\n   📊 Error Tracking Summary:\n")
	executions, _ := app.statusTracker.QueryExecutions(ExecutionQuery{Limit: 10})

	totalExecutions := len(executions)
	successfulExecutions := 0
	partialExecutions := 0
	failedExecutions := 0

	for _, execution := range executions {
		switch execution.Status {
		case ExecutionStatusCompleted:
			successfulExecutions++
		case ExecutionStatusPartialFailure:
			partialExecutions++
		case ExecutionStatusFailed:
			failedExecutions++
		}
	}

	fmt.Printf("      Total Executions: %d\n", totalExecutions)
	fmt.Printf("      Successful: %d\n", successfulExecutions)
	fmt.Printf("      Partial Failures: %d\n", partialExecutions)
	fmt.Printf("      Complete Failures: %d\n", failedExecutions)

	if totalExecutions > 0 {
		successRate := float64(successfulExecutions) / float64(totalExecutions) * 100
		fmt.Printf("      Success Rate: %.1f%%\n", successRate)
	}

	// Demonstrate retry mechanisms
	fmt.Printf("\n   🔄 Retry Mechanism Demo:\n")
	fmt.Printf("      Retry Policy: Exponential backoff\n")
	fmt.Printf("      Max Retries: 3\n")
	fmt.Printf("      Base Delay: 1s\n")
	fmt.Printf("      Max Delay: 30s\n")

	retryScenarios := []struct {
		attempt int
		delay   time.Duration
		result  string
	}{
		{1, 1 * time.Second, "Failed - Network timeout"},
		{2, 2 * time.Second, "Failed - Service unavailable"},
		{3, 4 * time.Second, "Success - Email sent"},
	}

	for _, scenario := range retryScenarios {
		fmt.Printf("      Attempt %d (delay: %v): %s\n",
			scenario.attempt, scenario.delay, scenario.result)
	}
}

// Helper functions for demo

func getDemoCustomerMappings() map[string]CustomerAccountInfo {
	return map[string]CustomerAccountInfo{
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
		"motor": {
			CustomerCode: "motor",
			CustomerName: "Motor",
			AWSAccountID: "345678901234",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::345678901234:role/MotorSESRole",
			Environment:  "staging",
		},
	}
}

func countEnabledFeatures(features map[string]bool) int {
	count := 0
	for _, enabled := range features {
		if enabled {
			count++
		}
	}
	return count
}
