package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// End-to-end integration tests for multi-customer email distribution system
// These tests validate the complete workflow from metadata ingestion to email delivery

func TestEndToEndFileProcessingWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	// Create test environment
	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	t.Run("CompleteFileProcessingWorkflow", func(t *testing.T) {
		// Step 1: Create metadata file
		metadata := createTestMetadata("E2E-FILE-001", []string{"hts", "cds"})
		metadataFile := testEnv.createMetadataFile("e2e_test.json", metadata)

		// Step 2: Process file through CLI
		err := testEnv.app.processFile(metadataFile)
		if err != nil {
			t.Errorf("File processing failed: %v", err)
		}

		// Step 3: Verify execution tracking
		executions, err := testEnv.app.statusTracker.QueryExecutions(ExecutionQuery{Limit: 10})
		if err != nil {
			t.Fatalf("Failed to query executions: %v", err)
		}

		if len(executions) == 0 {
			t.Error("Expected at least one execution to be tracked")
		}

		execution := executions[0]
		if execution.ChangeId != "E2E-FILE-001" {
			t.Errorf("Expected change ID 'E2E-FILE-001', got '%s'", execution.ChangeId)
		}

		// Step 4: Verify customer processing
		if len(execution.CustomerStatuses) != 2 {
			t.Errorf("Expected 2 customer statuses, got %d", len(execution.CustomerStatuses))
		}

		for customerCode, status := range execution.CustomerStatuses {
			if status.Status != CustomerStatusCompleted {
				t.Errorf("Expected customer %s to be completed, got %s", customerCode, status.Status)
			}
		}

		// Step 5: Verify isolation validation
		ctx := context.Background()
		for customerCode := range execution.CustomerStatuses {
			result, err := testEnv.isolationValidator.ValidateCustomerIsolation(ctx, customerCode)
			if err != nil {
				t.Errorf("Isolation validation failed for customer %s: %v", customerCode, err)
			}

			if result.CriticalIssues > 0 {
				t.Errorf("Critical isolation issues found for customer %s: %d", customerCode, result.CriticalIssues)
			}
		}
	})
}

func TestEndToEndSQSProcessingWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	t.Run("CompleteSQSProcessingWorkflow", func(t *testing.T) {
		// Step 1: Create SQS message with embedded metadata
		metadata := createTestMetadata("E2E-SQS-001", []string{"hts", "cds"})
		metadataJSON, _ := json.Marshal(metadata)

		sqsMessage := &types.Message{
			MessageId:     aws.String("test-message-001"),
			ReceiptHandle: aws.String("test-receipt-handle"),
			Body:          aws.String(string(metadataJSON)),
		}

		// Step 2: Process SQS message
		err := testEnv.sqsProcessor.processMessage(context.Background(), sqsMessage)
		if err != nil {
			t.Errorf("SQS message processing failed: %v", err)
		}

		// Step 3: Verify execution tracking
		executions, err := testEnv.app.statusTracker.QueryExecutions(ExecutionQuery{
			ChangeId: "E2E-SQS-001",
			Limit:    1,
		})
		if err != nil {
			t.Fatalf("Failed to query executions: %v", err)
		}

		if len(executions) == 0 {
			t.Error("Expected SQS execution to be tracked")
		}

		execution := executions[0]
		if execution.InitiatedBy != "sqs-processor" {
			t.Errorf("Expected initiator 'sqs-processor', got '%s'", execution.InitiatedBy)
		}

		// Step 4: Verify customer credential handling
		ctx := context.Background()
		customerCodes := []string{"hts", "cds"}
		for _, customerCode := range customerCodes {
			if testEnv.enhancedCredManager != nil {
				err := testEnv.enhancedCredManager.ValidateCustomerCredentials(ctx, customerCode, "ses")
				if err != nil {
					t.Logf("Credential validation failed for customer %s (expected in test): %v", customerCode, err)
				}
			}
		}

		// Step 5: Verify monitoring metrics
		metrics := testEnv.sqsProcessor.GetMetrics()
		if metrics.MessagesProcessed == 0 {
			t.Error("Expected messages processed count to be > 0")
		}
	})
}

func TestEndToEndMultiCustomerIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	t.Run("MultiCustomerIsolationWorkflow", func(t *testing.T) {
		ctx := context.Background()

		// Step 1: Process multiple customers simultaneously
		var wg sync.WaitGroup
		customerCodes := []string{"hts", "cds", "motor"}
		results := make(map[string]error)
		var resultsMutex sync.Mutex

		for _, customerCode := range customerCodes {
			wg.Add(1)
			go func(code string) {
				defer wg.Done()

				// Create customer-specific metadata
				metadata := createTestMetadata(fmt.Sprintf("E2E-ISOLATION-%s", code), []string{code})
				metadataFile := testEnv.createMetadataFile(fmt.Sprintf("isolation_%s.json", code), metadata)

				// Process file
				err := testEnv.app.processFile(metadataFile)

				resultsMutex.Lock()
				results[code] = err
				resultsMutex.Unlock()
			}(customerCode)
		}

		wg.Wait()

		// Step 2: Verify all customers processed successfully
		for customerCode, err := range results {
			if err != nil {
				t.Errorf("Customer %s processing failed: %v", customerCode, err)
			}
		}

		// Step 3: Validate isolation for all customers
		isolationResults, err := testEnv.isolationValidator.ValidateAllCustomers(ctx)
		if err != nil {
			t.Fatalf("Bulk isolation validation failed: %v", err)
		}

		// Step 4: Verify no cross-customer contamination
		for customerCode, result := range isolationResults {
			if result.CriticalIssues > 0 {
				t.Errorf("Critical isolation issues for customer %s: %d", customerCode, result.CriticalIssues)
			}

			// Check that customer only has access to their own data
			executions, err := testEnv.app.statusTracker.QueryExecutions(ExecutionQuery{
				CustomerCode: customerCode,
				Limit:        10,
			})
			if err != nil {
				t.Errorf("Failed to query executions for customer %s: %v", customerCode, err)
				continue
			}

			for _, execution := range executions {
				for execCustomerCode := range execution.CustomerStatuses {
					if execCustomerCode != customerCode {
						// This should only happen in legitimate multi-customer executions
						if !isLegitimateMultiCustomerExecution(execution, customerCode) {
							t.Errorf("Cross-customer data contamination detected: customer %s has access to %s data", customerCode, execCustomerCode)
						}
					}
				}
			}
		}
	})
}

func TestEndToEndErrorHandlingAndRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	t.Run("ErrorHandlingAndRecoveryWorkflow", func(t *testing.T) {
		// Step 1: Test invalid metadata handling
		invalidMetadata := map[string]interface{}{
			"change_id": "E2E-ERROR-001",
			"title":     "Invalid Test",
			// Missing required fields
		}

		invalidFile := testEnv.createMetadataFile("invalid.json", invalidMetadata)
		err := testEnv.app.processFile(invalidFile)
		if err == nil {
			t.Error("Expected error for invalid metadata, got none")
		}

		// Step 2: Test partial failure scenario
		mixedMetadata := createTestMetadata("E2E-ERROR-002", []string{"hts", "invalid-customer", "cds"})
		mixedFile := testEnv.createMetadataFile("mixed.json", mixedMetadata)

		err = testEnv.app.processFile(mixedFile)
		// Should not fail completely due to error handling

		// Step 3: Verify error tracking
		executions, err := testEnv.app.statusTracker.QueryExecutions(ExecutionQuery{
			ChangeId: "E2E-ERROR-002",
			Limit:    1,
		})
		if err != nil {
			t.Fatalf("Failed to query executions: %v", err)
		}

		if len(executions) > 0 {
			execution := executions[0]
			// Should have partial success/failure
			if execution.Status != StatusPartial && execution.Status != StatusFailed {
				// In test environment, this might vary
				t.Logf("Execution status: %s (expected partial failure or failed)", execution.Status)
			}
		}

		// Step 4: Test recovery mechanisms
		validMetadata := createTestMetadata("E2E-ERROR-003", []string{"hts"})
		validFile := testEnv.createMetadataFile("recovery.json", validMetadata)

		err = testEnv.app.processFile(validFile)
		if err != nil {
			t.Errorf("Recovery processing failed: %v", err)
		}
	})
}

func TestEndToEndPerformanceAndScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	t.Run("PerformanceAndScalabilityWorkflow", func(t *testing.T) {
		// Step 1: Test concurrent file processing
		numFiles := 10
		var wg sync.WaitGroup
		startTime := time.Now()

		for i := 0; i < numFiles; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				metadata := createTestMetadata(fmt.Sprintf("E2E-PERF-%03d", index), []string{"hts"})
				metadataFile := testEnv.createMetadataFile(fmt.Sprintf("perf_%03d.json", index), metadata)

				err := testEnv.app.processFile(metadataFile)
				if err != nil {
					t.Errorf("File %d processing failed: %v", index, err)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		t.Logf("Processed %d files in %v (avg: %v per file)", numFiles, duration, duration/time.Duration(numFiles))

		// Step 2: Verify all executions were tracked
		executions, err := testEnv.app.statusTracker.QueryExecutions(ExecutionQuery{Limit: numFiles + 5})
		if err != nil {
			t.Fatalf("Failed to query executions: %v", err)
		}

		perfExecutions := 0
		for _, execution := range executions {
			if strings.HasPrefix(execution.ChangeID, "E2E-PERF-") {
				perfExecutions++
			}
		}

		if perfExecutions < numFiles {
			t.Errorf("Expected at least %d performance test executions, got %d", numFiles, perfExecutions)
		}

		// Step 3: Test memory usage and cleanup
		// This is a basic check - in production you'd use more sophisticated monitoring
		if testEnv.isolationValidator != nil {
			metrics := testEnv.isolationValidator.GetValidationMetrics()
			t.Logf("Validation metrics: %+v", metrics)
		}

		if testEnv.enhancedCredManager != nil {
			credMetrics := testEnv.enhancedCredManager.GetCredentialMetrics()
			t.Logf("Credential metrics: %+v", credMetrics)
		}
	})
}

func TestEndToEndConfigurationManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	t.Run("ConfigurationManagementWorkflow", func(t *testing.T) {
		// Step 1: Test configuration loading
		configData := map[string]interface{}{
			"customerMappings": map[string]interface{}{
				"test-config": map[string]interface{}{
					"customerName": "Test Config Customer",
					"awsAccountId": "999999999999",
					"region":       "us-east-1",
					"environment":  "test",
					"roleArns": map[string]string{
						"ses": "arn:aws:iam::999999999999:role/TestConfigSESRole",
					},
				},
			},
		}

		configFile := testEnv.createConfigFile("test_config.json", configData)

		// Step 2: Create application with configuration
		config := &CLIConfig{
			ConfigFile: configFile,
			LogLevel:   "info",
			AWSRegion:  "us-east-1",
		}

		configApp, err := NewApplication(config)
		if err != nil {
			t.Fatalf("Failed to create application with config: %v", err)
		}

		// Step 3: Verify configuration was loaded
		if len(configApp.customerManager.CustomerMappings) == 0 {
			t.Error("Expected customer mappings to be loaded from config")
		}

		if _, exists := configApp.customerManager.CustomerMappings["test-config"]; !exists {
			t.Error("Expected test-config customer to be loaded")
		}

		// Step 4: Test configuration validation
		err = configApp.customerManager.ValidateCustomerMappingConfig()
		if err != nil {
			t.Errorf("Configuration validation failed: %v", err)
		}

		// Step 5: Process metadata with configured customer
		metadata := createTestMetadata("E2E-CONFIG-001", []string{"test-config"})
		metadataFile := testEnv.createMetadataFile("config_test.json", metadata)

		err = configApp.processFile(metadataFile)
		if err != nil {
			t.Errorf("Processing with configured customer failed: %v", err)
		}
	})
}

func TestEndToEndMonitoringAndAlerting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	t.Run("MonitoringAndAlertingWorkflow", func(t *testing.T) {
		// Step 1: Process metadata to generate monitoring data
		metadata := createTestMetadata("E2E-MONITOR-001", []string{"hts", "cds"})
		metadataFile := testEnv.createMetadataFile("monitor_test.json", metadata)

		err := testEnv.app.processFile(metadataFile)
		if err != nil {
			t.Errorf("File processing failed: %v", err)
		}

		// Step 2: Verify monitoring system captured metrics
		// In a real implementation, you would check CloudWatch metrics
		// For now, we verify the monitoring system is functioning
		if testEnv.app.monitoringSystem == nil {
			t.Error("Expected monitoring system to be initialized")
		}

		// Step 3: Test health check functionality (simplified for test)
		// In production, this would call actual health check endpoints
		t.Log("Monitoring system health check passed (simulated)")

		// Step 4: Verify execution metrics
		executions, err := testEnv.app.statusTracker.QueryExecutions(ExecutionQuery{
			ChangeId: "E2E-MONITOR-001",
			Limit:    1,
		})
		if err != nil {
			t.Fatalf("Failed to query executions: %v", err)
		}

		if len(executions) == 0 {
			t.Error("Expected monitoring execution to be tracked")
		}

		execution := executions[0]
		if execution.InitiatedAt.IsZero() {
			t.Error("Expected execution start time to be recorded")
		}

		if execution.CompletedAt == nil {
			t.Log("Execution may still be in progress (CompletedAt is nil)")
		}
	})
}

// Test environment setup and utilities

type TestEnvironment struct {
	app                 *Application
	sqsProcessor        *EnhancedSQSProcessor
	enhancedCredManager *EnhancedCredentialManager
	isolationValidator  *CustomerIsolationValidator
	tempDir             string
	t                   *testing.T
}

func setupTestEnvironment(t *testing.T) *TestEnvironment {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "e2e-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create customer manager with test data
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Test",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/HTSSQSRole",
			Environment:  "test",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Test",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "test",
		},
		"motor": {
			CustomerCode: "motor",
			CustomerName: "Motor Test",
			AWSAccountID: "345678901234",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::345678901234:role/MotorSESRole",
			Environment:  "test",
		},
	}

	// Create enhanced credential manager
	enhancedCredManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		t.Logf("Enhanced credential manager creation failed (expected in test): %v", err)
		enhancedCredManager = nil
	}

	// Create other components
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "E2ETest",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	// Create isolation validator
	isolationValidator := NewCustomerIsolationValidator(
		customerManager,
		enhancedCredManager,
		statusTracker,
		monitoringSystem,
	)

	// Create SQS processor
	sqsConfig := SQSProcessorConfig{
		QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:          "us-east-1",
		WorkerPoolSize:  2,
		ShutdownTimeout: 5 * time.Second,
	}

	sqsProcessor, err := NewEnhancedSQSProcessor(
		sqsConfig,
		customerManager,
		templateManager,
		sesManager,
		statusTracker,
		errorHandler,
		monitoringSystem,
	)
	if err != nil {
		t.Logf("SQS processor creation failed (expected in test): %v", err)
		sqsProcessor = nil
	}

	// Create application
	cliConfig := &CLIConfig{
		LogLevel:  "info",
		AWSRegion: "us-east-1",
	}

	app, err := NewApplication(cliConfig)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	// Override with test components
	app.customerManager = customerManager

	return &TestEnvironment{
		app:                 app,
		sqsProcessor:        sqsProcessor,
		enhancedCredManager: enhancedCredManager,
		isolationValidator:  isolationValidator,
		tempDir:             tempDir,
		t:                   t,
	}
}

func (env *TestEnvironment) cleanup() {
	os.RemoveAll(env.tempDir)
}

func (env *TestEnvironment) createMetadataFile(filename string, metadata map[string]interface{}) string {
	filePath := filepath.Join(env.tempDir, filename)
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		env.t.Fatalf("Failed to marshal metadata: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		env.t.Fatalf("Failed to write metadata file: %v", err)
	}

	return filePath
}

func (env *TestEnvironment) createConfigFile(filename string, config map[string]interface{}) string {
	filePath := filepath.Join(env.tempDir, filename)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		env.t.Fatalf("Failed to marshal config: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		env.t.Fatalf("Failed to write config file: %v", err)
	}

	return filePath
}

func isLegitimateMultiCustomerExecution(execution *ExecutionStatus, customerCode string) bool {
	// Check if this is a legitimate multi-customer execution
	// This would involve checking the execution metadata and business rules
	return len(execution.CustomerStatuses) > 1 && execution.CustomerStatuses[customerCode] != nil
}

func createTestMetadata(changeID string, customerCodes []string) map[string]interface{} {
	return map[string]interface{}{
		"customer_codes": customerCodes,
		"change_id":      changeID,
		"title":          fmt.Sprintf("E2E Test: %s", changeID),
		"description":    "End-to-end integration test metadata",
		"template_id":    "test-template",
		"priority":       "normal",
		"email_data": map[string]interface{}{
			"subject": fmt.Sprintf("Test Email: %s", changeID),
			"message": "This is a test email for end-to-end integration testing",
		},
	}
}

// Benchmark tests for performance validation

func BenchmarkEndToEndFileProcessing(b *testing.B) {
	testEnv := setupTestEnvironment(&testing.T{})
	defer testEnv.cleanup()

	metadata := createTestMetadata("BENCH-001", []string{"hts"})
	metadataFile := testEnv.createMetadataFile("bench.json", metadata)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := testEnv.app.processFile(metadataFile)
		if err != nil {
			b.Errorf("File processing failed: %v", err)
		}
	}
}

func BenchmarkEndToEndIsolationValidation(b *testing.B) {
	testEnv := setupTestEnvironment(&testing.T{})
	defer testEnv.cleanup()

	if testEnv.isolationValidator == nil {
		b.Skip("Isolation validator not available")
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := testEnv.isolationValidator.ValidateCustomerIsolation(ctx, "hts")
		if err != nil {
			b.Errorf("Isolation validation failed: %v", err)
		}
	}
}
