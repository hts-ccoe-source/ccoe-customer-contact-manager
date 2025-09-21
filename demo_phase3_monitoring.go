package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

// Demo application for Phase 3 monitoring, error handling, and execution tracking

func main() {
	fmt.Println("=== Phase 3: Monitoring, Error Handling & Execution Tracking Demo ===")

	// Demo 1: Execution Status Tracking
	fmt.Println("\n🧪 Demo 1: Execution Status Tracking")
	demoExecutionStatusTracking()

	// Demo 2: Error Handling and Retry Logic
	fmt.Println("\n🧪 Demo 2: Error Handling & Retry Logic")
	demoErrorHandlingAndRetry()

	// Demo 3: Monitoring and Observability
	fmt.Println("\n🧪 Demo 3: Monitoring & Observability")
	demoMonitoringAndObservability()

	// Demo 4: Circuit Breaker and Customer Isolation
	fmt.Println("\n🧪 Demo 4: Circuit Breaker & Customer Isolation")
	demoCircuitBreakerAndIsolation()

	// Demo 5: End-to-End Multi-Customer Execution
	fmt.Println("\n🧪 Demo 5: End-to-End Multi-Customer Execution")
	demoEndToEndExecution()

	fmt.Println("\n=== Phase 3 Demo Complete ===")
}

func demoExecutionStatusTracking() {
	customerManager := setupDemoCustomerManager()
	statusTracker := NewExecutionStatusTracker(customerManager)

	fmt.Printf("📊 Execution Status Tracking Demo\n")

	// Start a multi-customer execution
	fmt.Printf("   🚀 Starting multi-customer execution...\n")
	customerCodes := []string{"hts", "cds", "motor", "bat"}

	execution, err := statusTracker.StartExecution(
		"CHANGE-2024-001",
		"Security Update Rollout",
		"Deploy security patches across all customer environments",
		"admin@example.com",
		customerCodes,
	)
	if err != nil {
		log.Printf("❌ Failed to start execution: %v", err)
		return
	}

	fmt.Printf("   ✅ Execution started: %s\n", execution.ExecutionID)
	fmt.Printf("      Change ID: %s\n", execution.ChangeID)
	fmt.Printf("      Title: %s\n", execution.Title)
	fmt.Printf("      Total Customers: %d\n", execution.TotalCustomers)
	fmt.Printf("      Status: %s\n", execution.Status)

	// Simulate customer executions with steps
	fmt.Printf("\n   📋 Simulating customer executions...\n")

	for i, customerCode := range customerCodes {
		fmt.Printf("      🏢 Processing customer: %s\n", customerCode)

		// Start customer execution
		err := statusTracker.StartCustomerExecution(execution.ExecutionID, customerCode)
		if err != nil {
			log.Printf("❌ Failed to start customer execution: %v", err)
			continue
		}

		// Add execution steps
		steps := []struct {
			id          string
			name        string
			description string
		}{
			{"validate", "Validate Environment", "Check environment prerequisites"},
			{"backup", "Create Backup", "Create system backup before changes"},
			{"deploy", "Deploy Changes", "Apply security patches"},
			{"verify", "Verify Deployment", "Verify changes were applied correctly"},
			{"notify", "Send Notifications", "Notify stakeholders of completion"},
		}

		for _, step := range steps {
			statusTracker.AddExecutionStep(execution.ExecutionID, customerCode, step.id, step.name, step.description)
		}

		// Simulate step execution
		for _, step := range steps {
			// Start step
			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, step.id, StepStatusRunning, "")

			// Simulate processing time
			processingTime := time.Duration(rand.Intn(100)+50) * time.Millisecond
			time.Sleep(processingTime)

			// Complete step (simulate occasional failures)
			if rand.Float32() < 0.9 { // 90% success rate
				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, step.id, StepStatusCompleted, "")
			} else {
				errorMsg := fmt.Sprintf("Step failed: %s", step.name)
				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, step.id, StepStatusFailed, errorMsg)
				break // Stop processing remaining steps for this customer
			}
		}

		// Complete customer execution
		success := rand.Float32() < 0.8 // 80% success rate
		var errorMessage string
		if !success {
			errorMessages := []string{
				"Network connectivity issues",
				"Authentication timeout",
				"Insufficient permissions",
				"Service temporarily unavailable",
			}
			errorMessage = errorMessages[rand.Intn(len(errorMessages))]
		}

		statusTracker.CompleteCustomerExecution(execution.ExecutionID, customerCode, success, errorMessage)

		fmt.Printf("         Status: %s\n", map[bool]string{true: "✅ Success", false: "❌ Failed"}[success])
		if !success {
			fmt.Printf("         Error: %s\n", errorMessage)
		}

		// Add some delay between customers
		if i < len(customerCodes)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Get final execution status
	finalExecution, err := statusTracker.GetExecution(execution.ExecutionID)
	if err != nil {
		log.Printf("❌ Failed to get final execution: %v", err)
		return
	}

	fmt.Printf("\n   📈 Final Execution Results:\n")
	fmt.Printf("      Overall Status: %s\n", finalExecution.Status)
	fmt.Printf("      Duration: %v\n", finalExecution.ActualDuration)
	fmt.Printf("      Success Rate: %.1f%%\n", finalExecution.Metrics.SuccessRate)
	fmt.Printf("      Fastest Customer: %s\n", finalExecution.Metrics.FastestCustomer)
	fmt.Printf("      Slowest Customer: %s\n", finalExecution.Metrics.SlowestCustomer)

	if finalExecution.ErrorSummary != nil {
		fmt.Printf("      Total Errors: %d\n", finalExecution.ErrorSummary.TotalErrors)
		fmt.Printf("      Most Common Error: %s\n", finalExecution.ErrorSummary.MostCommonError)
	}

	// Query executions
	fmt.Printf("\n   🔍 Querying executions...\n")
	query := ExecutionQuery{
		Status: []ExecutionStatusType{StatusCompleted, StatusPartial, StatusFailed},
		Limit:  10,
	}

	executions, err := statusTracker.QueryExecutions(query)
	if err != nil {
		log.Printf("❌ Failed to query executions: %v", err)
		return
	}

	fmt.Printf("      Found %d executions\n", len(executions))
	for _, exec := range executions {
		fmt.Printf("         %s: %s (%s)\n", exec.ExecutionID, exec.Title, exec.Status)
	}

	// Get execution summary
	fmt.Printf("\n   📊 Execution Summary (24h):\n")
	summary, err := statusTracker.GetExecutionSummary(24 * time.Hour)
	if err != nil {
		log.Printf("❌ Failed to get execution summary: %v", err)
		return
	}

	fmt.Printf("      Total Executions: %d\n", summary.TotalExecutions)
	fmt.Printf("      Status Distribution:\n")
	for status, count := range summary.StatusCounts {
		fmt.Printf("         %s: %d\n", status, count)
	}

	if summary.PerformanceStats != nil {
		fmt.Printf("      Performance:\n")
		fmt.Printf("         Average Duration: %v\n", summary.PerformanceStats.AverageDuration)
		fmt.Printf("         Success Rate: %.1f%%\n", summary.PerformanceStats.SuccessRate)
	}
}

func demoErrorHandlingAndRetry() {
	customerManager := setupDemoCustomerManager()
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	fmt.Printf("🔧 Error Handling & Retry Logic Demo\n")

	// Demo retry logic with different error types
	fmt.Printf("   🔄 Testing retry logic with different error types...\n")

	testCases := []struct {
		name         string
		customerCode string
		errorType    string
		shouldRetry  bool
	}{
		{"Network Timeout", "hts", "network timeout", true},
		{"Rate Limiting", "cds", "rate exceeded", true},
		{"Authentication Error", "motor", "invalid credentials", false},
		{"Service Unavailable", "bat", "service unavailable", true},
		{"Invalid Request", "hts", "malformed request", false},
	}

	for _, testCase := range testCases {
		fmt.Printf("      🧪 Test: %s (Customer: %s)\n", testCase.name, testCase.customerCode)

		attemptCount := 0
		operation := func(ctx context.Context) error {
			attemptCount++
			fmt.Printf("         Attempt %d...", attemptCount)

			// Simulate operation that might fail
			if attemptCount <= 2 && testCase.shouldRetry {
				fmt.Printf(" ❌ Failed: %s\n", testCase.errorType)
				return fmt.Errorf(testCase.errorType)
			}

			if !testCase.shouldRetry {
				fmt.Printf(" ❌ Failed: %s\n", testCase.errorType)
				return fmt.Errorf(testCase.errorType)
			}

			fmt.Printf(" ✅ Success\n")
			return nil
		}

		ctx := context.Background()
		err := errorHandler.ExecuteWithRetry(ctx, testCase.customerCode, operation)

		if err != nil {
			fmt.Printf("         Final Result: ❌ Failed after retries - %v\n", err)
		} else {
			fmt.Printf("         Final Result: ✅ Success after %d attempts\n", attemptCount)
		}

		fmt.Printf("         Total Attempts: %d\n", attemptCount)
	}

	// Demo circuit breaker
	fmt.Printf("\n   ⚡ Testing circuit breaker functionality...\n")

	// Simulate multiple failures to trigger circuit breaker
	fmt.Printf("      Simulating multiple failures for customer 'hts'...\n")

	for i := 1; i <= 7; i++ {
		fmt.Printf("         Failure %d...", i)

		operation := func(ctx context.Context) error {
			return fmt.Errorf("simulated failure %d", i)
		}

		ctx := context.Background()
		err := errorHandler.ExecuteWithRetry(ctx, "hts", operation)

		if err != nil {
			if err.Error() == "circuit breaker is open for customer hts" {
				fmt.Printf(" ⚡ Circuit breaker opened!\n")
				break
			} else {
				fmt.Printf(" ❌ Failed\n")
			}
		}
	}

	// Check circuit breaker status
	fmt.Printf("\n   📊 Circuit Breaker Status:\n")
	cbStatus := errorHandler.GetCircuitBreakerStatus()
	for customerCode, status := range cbStatus {
		fmt.Printf("      %s: %s (Failures: %d, Successes: %d)\n",
			customerCode, status.State, status.FailureCount, status.SuccessCount)
	}

	// Demo customer isolation
	fmt.Printf("\n   🏢 Testing customer isolation...\n")

	customerCodes := []string{"hts", "cds", "motor", "bat"}

	operation := func(customerCode string) error {
		// Simulate different success rates per customer
		successRates := map[string]float32{
			"hts":   0.9, // 90% success
			"cds":   0.7, // 70% success
			"motor": 0.5, // 50% success
			"bat":   0.8, // 80% success
		}

		if rand.Float32() < successRates[customerCode] {
			return nil // Success
		}

		return fmt.Errorf("operation failed for customer %s", customerCode)
	}

	ctx := context.Background()
	results := errorHandler.ExecuteWithCustomerIsolation(ctx, customerCodes, operation)

	fmt.Printf("      Customer Isolation Results:\n")
	successCount := 0
	for customerCode, err := range results {
		if err == nil {
			fmt.Printf("         %s: ✅ Success\n", customerCode)
			successCount++
		} else {
			fmt.Printf("         %s: ❌ Failed - %v\n", customerCode, err)
		}
	}

	fmt.Printf("      Overall Success Rate: %.1f%% (%d/%d)\n",
		float64(successCount)/float64(len(customerCodes))*100, successCount, len(customerCodes))

	// Show error metrics
	fmt.Printf("\n   📈 Error Metrics:\n")
	metrics := errorHandler.GetErrorMetrics()
	fmt.Printf("      Total Errors: %d\n", metrics.TotalErrors)
	fmt.Printf("      Circuit Breaker Trips: %d\n", metrics.CircuitBreakerTrips)
	fmt.Printf("      Dead Letter Count: %d\n", metrics.DeadLetterCount)

	if len(metrics.ErrorsByType) > 0 {
		fmt.Printf("      Errors by Type:\n")
		for errorType, count := range metrics.ErrorsByType {
			fmt.Printf("         %s: %d\n", errorType, count)
		}
	}

	if len(metrics.ErrorsByCustomer) > 0 {
		fmt.Printf("      Errors by Customer:\n")
		for customerCode, count := range metrics.ErrorsByCustomer {
			fmt.Printf("         %s: %d\n", customerCode, count)
		}
	}
}

func demoMonitoringAndObservability() {
	customerManager := setupDemoCustomerManager()
	errorHandler := NewErrorHandler(customerManager, nil)
	statusTracker := NewExecutionStatusTracker(customerManager)

	// Create monitoring configuration
	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch:    true,
		EnableXRay:          false,
		MetricsNamespace:    "EmailDistribution",
		LogGroup:            "/aws/lambda/email-distribution",
		LogRetentionDays:    30,
		HealthCheckInterval: 30 * time.Second,
	}

	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	fmt.Printf("📊 Monitoring & Observability Demo\n")

	// Demo metrics collection
	fmt.Printf("   📈 Collecting metrics...\n")

	// Simulate various metrics
	metrics := []struct {
		name  string
		value float64
		unit  string
		tags  map[string]string
	}{
		{"emails_sent", 1250, "count", map[string]string{"customer": "hts"}},
		{"emails_delivered", 1200, "count", map[string]string{"customer": "hts"}},
		{"emails_bounced", 25, "count", map[string]string{"customer": "hts"}},
		{"response_time", 150, "milliseconds", map[string]string{"operation": "send_email"}},
		{"error_rate", 2.5, "percent", map[string]string{"service": "ses"}},
		{"queue_depth", 45, "count", map[string]string{"queue": "email_processing"}},
		{"cpu_utilization", 65.5, "percent", map[string]string{"instance": "web-01"}},
		{"memory_usage", 78.2, "percent", map[string]string{"instance": "web-01"}},
	}

	for _, metric := range metrics {
		monitoringSystem.metricsCollector.EmitMetric(
			metric.name,
			metric.value,
			MetricTypeGauge,
			metric.unit,
			metric.tags,
		)
		fmt.Printf("      📊 %s: %.1f %s %v\n", metric.name, metric.value, metric.unit, metric.tags)
	}

	// Demo structured logging
	fmt.Printf("\n   📝 Structured logging examples...\n")

	logger := monitoringSystem.logger

	// Different log levels with structured data
	logger.Info("Email distribution started", map[string]interface{}{
		"executionId":   "exec-123456",
		"customerCount": 4,
		"changeId":      "CHANGE-2024-001",
	})

	logger.Warn("High queue depth detected", map[string]interface{}{
		"queueName": "email_processing",
		"depth":     45,
		"threshold": 40,
		"customer":  "hts",
	})

	logger.Error("Failed to send email", fmt.Errorf("SES rate limit exceeded"), map[string]interface{}{
		"customer":       "cds",
		"recipientCount": 150,
		"retryAttempt":   3,
	})

	logger.Debug("Processing customer batch", map[string]interface{}{
		"customer":       "motor",
		"batchSize":      25,
		"processingTime": "1.2s",
	})

	// Demo health checks
	fmt.Printf("\n   🏥 Running health checks...\n")

	healthResults := monitoringSystem.healthChecker.RunHealthChecks()
	overallStatus, lastCheck := monitoringSystem.healthChecker.GetHealthStatus()

	fmt.Printf("      Overall Status: %s (Last Check: %s)\n", overallStatus, lastCheck.Format("15:04:05"))
	fmt.Printf("      Individual Checks:\n")

	for checkName, result := range healthResults {
		statusIcon := map[HealthStatus]string{
			HealthStatusHealthy:   "✅",
			HealthStatusDegraded:  "⚠️",
			HealthStatusUnhealthy: "❌",
			HealthStatusUnknown:   "❓",
		}[result.Status]

		fmt.Printf("         %s %s: %s (%v)\n",
			statusIcon, checkName, result.Message, result.Duration)
	}

	// Demo alerting
	fmt.Printf("\n   🚨 Testing alert system...\n")

	// Simulate metrics that would trigger alerts
	alertTestMetrics := map[string]*Metric{
		"error_rate": {
			Name:      "error_rate",
			Type:      MetricTypeGauge,
			Value:     7.5, // Above threshold of 5.0
			Unit:      "percent",
			Timestamp: time.Now(),
		},
		"response_time": {
			Name:      "response_time",
			Type:      MetricTypeGauge,
			Value:     1200, // Above threshold of 1000ms
			Unit:      "milliseconds",
			Timestamp: time.Now(),
		},
	}

	fmt.Printf("      Evaluating alert rules...\n")
	monitoringSystem.alertManager.EvaluateAlerts(alertTestMetrics)

	// Show active alerts (simulated)
	fmt.Printf("      Active Alerts:\n")
	fmt.Printf("         🚨 HIGH: Error rate is above threshold (7.5% > 5.0%)\n")
	fmt.Printf("         🚨 MEDIUM: Response latency is above threshold (1200ms > 1000ms)\n")

	// Demo dashboard configuration
	fmt.Printf("\n   📊 Dashboard Configuration:\n")
	dashboard := monitoringSystem.dashboardConfig
	fmt.Printf("      Name: %s\n", dashboard.Name)
	fmt.Printf("      Refresh Rate: %v\n", dashboard.RefreshRate)
	fmt.Printf("      Widgets: %d\n", len(dashboard.Widgets))

	for _, widget := range dashboard.Widgets {
		fmt.Printf("         📈 %s (%s): %s\n", widget.Title, widget.Type, widget.Description)
	}

	// Simulate real-time metrics update
	fmt.Printf("\n   🔄 Simulating real-time metrics (5 seconds)...\n")

	for i := 0; i < 5; i++ {
		// Generate random metrics
		emailsSent := float64(rand.Intn(100) + 200)
		errorRate := rand.Float64() * 10
		responseTime := float64(rand.Intn(500) + 100)

		monitoringSystem.metricsCollector.SetGauge("emails_sent_rate", emailsSent, "count/min", nil)
		monitoringSystem.metricsCollector.SetGauge("current_error_rate", errorRate, "percent", nil)
		monitoringSystem.metricsCollector.SetGauge("current_response_time", responseTime, "milliseconds", nil)

		fmt.Printf("      [%ds] Emails: %.0f/min, Errors: %.1f%%, Latency: %.0fms\n",
			i+1, emailsSent, errorRate, responseTime)

		time.Sleep(1 * time.Second)
	}
}

func demoCircuitBreakerAndIsolation() {
	customerManager := setupDemoCustomerManager()
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	fmt.Printf("⚡ Circuit Breaker & Customer Isolation Demo\n")

	customers := []string{"hts", "cds", "motor", "bat"}

	// Simulate different failure patterns for each customer
	fmt.Printf("   🧪 Simulating different failure patterns per customer...\n")

	failurePatterns := map[string]struct {
		failureRate float32
		description string
	}{
		"hts":   {0.1, "Stable customer (10% failure rate)"},
		"cds":   {0.3, "Moderate issues (30% failure rate)"},
		"motor": {0.7, "High failure rate (70% failure rate)"},
		"bat":   {0.9, "Critical issues (90% failure rate)"},
	}

	for customer, pattern := range failurePatterns {
		fmt.Printf("      🏢 %s: %s\n", customer, pattern.description)
	}

	// Run operations for each customer
	fmt.Printf("\n   🔄 Running operations (20 attempts per customer)...\n")

	for attempt := 1; attempt <= 20; attempt++ {
		fmt.Printf("      Attempt %d:\n", attempt)

		for _, customer := range customers {
			operation := func(ctx context.Context) error {
				// Simulate operation based on failure pattern
				if rand.Float32() < failurePatterns[customer].failureRate {
					return fmt.Errorf("operation failed for %s", customer)
				}
				return nil
			}

			ctx := context.Background()
			err := errorHandler.ExecuteWithRetry(ctx, customer, operation)

			status := "✅"
			message := "Success"
			if err != nil {
				status = "❌"
				message = "Failed"
				if err.Error() == fmt.Sprintf("circuit breaker is open for customer %s", customer) {
					status = "⚡"
					message = "Circuit Breaker Open"
				}
			}

			fmt.Printf("         %s %s: %s\n", status, customer, message)
		}

		// Show circuit breaker status every 5 attempts
		if attempt%5 == 0 {
			fmt.Printf("      Circuit Breaker Status:\n")
			cbStatus := errorHandler.GetCircuitBreakerStatus()
			for _, customer := range customers {
				if status, exists := cbStatus[customer]; exists {
					stateIcon := map[CircuitBreakerState]string{
						CircuitBreakerClosed:   "🟢",
						CircuitBreakerOpen:     "🔴",
						CircuitBreakerHalfOpen: "🟡",
					}[status.State]

					fmt.Printf("         %s %s: %s (F:%d/S:%d)\n",
						stateIcon, customer, status.State, status.FailureCount, status.SuccessCount)
				}
			}
		}

		// Small delay between attempts
		time.Sleep(100 * time.Millisecond)
	}

	// Final circuit breaker status
	fmt.Printf("\n   📊 Final Circuit Breaker Status:\n")
	cbStatus := errorHandler.GetCircuitBreakerStatus()
	for _, customer := range customers {
		if status, exists := cbStatus[customer]; exists {
			fmt.Printf("      %s:\n", customer)
			fmt.Printf("         State: %s\n", status.State)
			fmt.Printf("         Failures: %d\n", status.FailureCount)
			fmt.Printf("         Successes: %d\n", status.SuccessCount)
			fmt.Printf("         Last Failure: %s\n", status.LastFailureTime.Format("15:04:05"))
			if !status.LastSuccessTime.IsZero() {
				fmt.Printf("         Last Success: %s\n", status.LastSuccessTime.Format("15:04:05"))
			}
		}
	}

	// Demo customer isolation effectiveness
	fmt.Printf("\n   🛡️  Customer Isolation Effectiveness:\n")

	// Show how one customer's failures don't affect others
	fmt.Printf("      Testing isolation: Simulating critical failure in 'motor' customer...\n")

	// Force multiple failures for motor to open circuit breaker
	for i := 0; i < 10; i++ {
		operation := func(ctx context.Context) error {
			return fmt.Errorf("critical system failure")
		}

		ctx := context.Background()
		errorHandler.ExecuteWithRetry(ctx, "motor", operation)
	}

	// Now test that other customers are unaffected
	fmt.Printf("      Testing other customers after 'motor' circuit breaker opened...\n")

	for _, customer := range []string{"hts", "cds", "bat"} {
		operation := func(ctx context.Context) error {
			return nil // Simulate success
		}

		ctx := context.Background()
		err := errorHandler.ExecuteWithRetry(ctx, customer, operation)

		if err == nil {
			fmt.Printf("         ✅ %s: Still operational (isolated from motor failures)\n", customer)
		} else {
			fmt.Printf("         ❌ %s: Affected by motor failures (isolation failed)\n", customer)
		}
	}

	// Show error metrics by customer
	fmt.Printf("\n   📈 Error Distribution by Customer:\n")
	metrics := errorHandler.GetErrorMetrics()

	for customer, errorCount := range metrics.ErrorsByCustomer {
		fmt.Printf("      %s: %d errors\n", customer, errorCount)
	}

	fmt.Printf("      Total Circuit Breaker Trips: %d\n", metrics.CircuitBreakerTrips)
}

func demoEndToEndExecution() {
	fmt.Printf("🎯 End-to-End Multi-Customer Execution Demo\n")

	// Set up all components
	customerManager := setupDemoCustomerManager()
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch:    true,
		MetricsNamespace:    "EmailDistribution",
		HealthCheckInterval: 30 * time.Second,
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	fmt.Printf("   🚀 Starting comprehensive multi-customer email distribution...\n")

	// Start execution tracking
	customerCodes := []string{"hts", "cds", "motor", "bat"}
	execution, err := statusTracker.StartExecution(
		"CHANGE-2024-002",
		"Monthly Newsletter Distribution",
		"Send monthly newsletter to all customer subscribers",
		"marketing@example.com",
		customerCodes,
	)
	if err != nil {
		log.Printf("❌ Failed to start execution: %v", err)
		return
	}

	fmt.Printf("      Execution ID: %s\n", execution.ExecutionID)

	// Process each customer with full error handling and monitoring
	for _, customerCode := range customerCodes {
		fmt.Printf("\n   🏢 Processing customer: %s\n", customerCode)

		// Start customer execution tracking
		statusTracker.StartCustomerExecution(execution.ExecutionID, customerCode)

		// Define the complete customer processing operation
		customerOperation := func(ctx context.Context) error {
			// Step 1: Validate customer configuration
			statusTracker.AddExecutionStep(execution.ExecutionID, customerCode, "validate", "Validate Configuration", "Validate customer settings and permissions")
			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "validate", StepStatusRunning, "")

			// Simulate validation
			time.Sleep(50 * time.Millisecond)

			// Simulate occasional validation failures
			if rand.Float32() < 0.1 {
				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "validate", StepStatusFailed, "Invalid configuration")
				return fmt.Errorf("validation failed for customer %s", customerCode)
			}

			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "validate", StepStatusCompleted, "")

			// Step 2: Prepare email content
			statusTracker.AddExecutionStep(execution.ExecutionID, customerCode, "prepare", "Prepare Content", "Render email templates with customer data")
			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "prepare", StepStatusRunning, "")

			// Simulate template rendering
			templateData := &EmailTemplateData{
				Variables: map[string]interface{}{
					"Title":   "Monthly Newsletter - December 2024",
					"Message": fmt.Sprintf("Dear %s team, here's your monthly update...", strings.ToUpper(customerCode)),
				},
				CustomerCode: customerCode,
			}

			_, err := templateManager.RenderTemplate("notification", customerCode, templateData)
			if err != nil {
				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "prepare", StepStatusFailed, err.Error())
				return fmt.Errorf("template rendering failed: %v", err)
			}

			time.Sleep(30 * time.Millisecond)
			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "prepare", StepStatusCompleted, "")

			// Step 3: Send emails
			statusTracker.AddExecutionStep(execution.ExecutionID, customerCode, "send", "Send Emails", "Send emails via SES")
			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "send", StepStatusRunning, "")

			// Simulate email sending with SES
			emailRequest := &EmailRequest{
				CustomerCode: customerCode,
				TemplateID:   "notification",
				Recipients: []EmailRecipient{
					{Email: fmt.Sprintf("admin@%s.example.com", customerCode), Type: "to"},
				},
				TemplateData: templateData.Variables,
			}

			_, err = sesManager.SendEmail(emailRequest)
			if err != nil {
				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "send", StepStatusFailed, err.Error())
				return fmt.Errorf("email sending failed: %v", err)
			}

			time.Sleep(100 * time.Millisecond)
			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "send", StepStatusCompleted, "")

			// Step 4: Verify delivery
			statusTracker.AddExecutionStep(execution.ExecutionID, customerCode, "verify", "Verify Delivery", "Check email delivery status")
			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "verify", StepStatusRunning, "")

			// Simulate delivery verification
			time.Sleep(75 * time.Millisecond)

			// Simulate occasional delivery issues
			if rand.Float32() < 0.05 {
				statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "verify", StepStatusFailed, "Delivery verification failed")
				return fmt.Errorf("delivery verification failed for customer %s", customerCode)
			}

			statusTracker.UpdateExecutionStep(execution.ExecutionID, customerCode, "verify", StepStatusCompleted, "")

			return nil
		}

		// Execute with error handling and retry logic
		ctx := context.Background()
		err := errorHandler.ExecuteWithRetry(ctx, customerCode, customerOperation)

		// Complete customer execution
		success := err == nil
		var errorMessage string
		if err != nil {
			errorMessage = err.Error()
		}

		statusTracker.CompleteCustomerExecution(execution.ExecutionID, customerCode, success, errorMessage)

		// Emit metrics
		if success {
			monitoringSystem.metricsCollector.IncrementCounter("customer_success", map[string]string{"customer": customerCode})
			fmt.Printf("      ✅ Customer %s completed successfully\n", customerCode)
		} else {
			monitoringSystem.metricsCollector.IncrementCounter("customer_failure", map[string]string{"customer": customerCode})
			fmt.Printf("      ❌ Customer %s failed: %s\n", customerCode, errorMessage)
		}

		// Log structured event
		monitoringSystem.logger.Info("Customer processing completed", map[string]interface{}{
			"executionId":  execution.ExecutionID,
			"customerCode": customerCode,
			"success":      success,
			"error":        errorMessage,
		})
	}

	// Get final results
	finalExecution, err := statusTracker.GetExecution(execution.ExecutionID)
	if err != nil {
		log.Printf("❌ Failed to get final execution: %v", err)
		return
	}

	fmt.Printf("\n   📊 Final Execution Summary:\n")
	fmt.Printf("      Status: %s\n", finalExecution.Status)
	fmt.Printf("      Duration: %v\n", finalExecution.ActualDuration)
	fmt.Printf("      Success Rate: %.1f%%\n", finalExecution.Metrics.SuccessRate)
	fmt.Printf("      Throughput: %.1f customers/minute\n", finalExecution.Metrics.ThroughputPerMinute)

	// Customer-specific results
	fmt.Printf("\n   🏢 Customer Results:\n")
	successCount := 0
	for customerCode, customerExec := range finalExecution.CustomerStatuses {
		status := "❌ Failed"
		if customerExec.Status == CustomerStatusCompleted {
			status = "✅ Success"
			successCount++
		}

		fmt.Printf("      %s: %s", customerCode, status)
		if customerExec.Duration != nil {
			fmt.Printf(" (%v)", *customerExec.Duration)
		}
		if customerExec.ErrorMessage != "" {
			fmt.Printf(" - %s", customerExec.ErrorMessage)
		}
		fmt.Printf("\n")
	}

	// Error analysis
	if finalExecution.ErrorSummary != nil && finalExecution.ErrorSummary.TotalErrors > 0 {
		fmt.Printf("\n   🔍 Error Analysis:\n")
		fmt.Printf("      Total Errors: %d\n", finalExecution.ErrorSummary.TotalErrors)
		fmt.Printf("      Retryable Errors: %d\n", finalExecution.ErrorSummary.RetryableErrors)
		fmt.Printf("      Permanent Errors: %d\n", finalExecution.ErrorSummary.PermanentErrors)

		if len(finalExecution.ErrorSummary.ErrorsByType) > 0 {
			fmt.Printf("      Error Types:\n")
			for errorType, count := range finalExecution.ErrorSummary.ErrorsByType {
				fmt.Printf("         %s: %d\n", errorType, count)
			}
		}
	}

	// System health check
	fmt.Printf("\n   🏥 System Health Check:\n")
	healthResults := monitoringSystem.healthChecker.RunHealthChecks()
	overallStatus, _ := monitoringSystem.healthChecker.GetHealthStatus()
	fmt.Printf("      Overall System Health: %s\n", overallStatus)

	// Final metrics
	fmt.Printf("\n   📈 System Metrics:\n")
	errorMetrics := errorHandler.GetErrorMetrics()
	fmt.Printf("      Total System Errors: %d\n", errorMetrics.TotalErrors)
	fmt.Printf("      Circuit Breaker Trips: %d\n", errorMetrics.CircuitBreakerTrips)

	cbStatus := errorHandler.GetCircuitBreakerStatus()
	openCircuits := 0
	for _, status := range cbStatus {
		if status.State == CircuitBreakerOpen {
			openCircuits++
		}
	}
	fmt.Printf("      Open Circuit Breakers: %d/%d\n", openCircuits, len(cbStatus))

	fmt.Printf("\n   🎯 Execution Complete: %d/%d customers successful\n", successCount, len(customerCodes))
}

// Helper function to set up demo customer manager
func setupDemoCustomerManager() *CustomerCredentialManager {
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
		"motor": {
			CustomerCode: "motor",
			CustomerName: "Motor",
			AWSAccountID: "345678901234",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::345678901234:role/MotorSESRole",
			Environment:  "staging",
		},
		"bat": {
			CustomerCode: "bat",
			CustomerName: "Bring A Trailer",
			AWSAccountID: "456789012345",
			Region:       "eu-west-1",
			SESRoleARN:   "arn:aws:iam::456789012345:role/BATSESRole",
			Environment:  "production",
		},
	}

	return customerManager
}
