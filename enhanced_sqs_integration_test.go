package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func TestEnhancedSQSProcessor_Creation(t *testing.T) {
	config := SQSProcessorConfig{
		QueueURL:          "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:            "us-east-1",
		MaxMessages:       10,
		VisibilityTimeout: 30,
		WaitTimeSeconds:   20,
		PollingInterval:   5 * time.Second,
		WorkerPoolSize:    5,
		ShutdownTimeout:   30 * time.Second,
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		t.Skipf("Failed to create SQS processor (likely no AWS credentials): %v", err)
	}

	if processor == nil {
		t.Error("Expected processor to be non-nil")
	}

	if !processor.IsRunning() == true {
		// Processor should not be running initially
	}
}

func TestEnhancedSQSProcessor_ValidateMessage(t *testing.T) {
	config := SQSProcessorConfig{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:   "us-east-1",
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		t.Skipf("Failed to create SQS processor: %v", err)
	}

	tests := []struct {
		name      string
		message   SQSMessage
		expectErr bool
	}{
		{
			name: "Valid message",
			message: SQSMessage{
				ChangeID:      "TEST-001",
				Title:         "Test Message",
				Description:   "Test description",
				CustomerCodes: []string{"hts"},
				TemplateID:    "test-template",
				TemplateData: map[string]interface{}{
					"subject": "Test Subject",
					"message": "Test Message",
				},
			},
			expectErr: false,
		},
		{
			name: "Missing change ID",
			message: SQSMessage{
				Title:         "Test Message",
				CustomerCodes: []string{"hts"},
				TemplateID:    "test-template",
			},
			expectErr: true,
		},
		{
			name: "Missing title",
			message: SQSMessage{
				ChangeID:      "TEST-002",
				CustomerCodes: []string{"hts"},
				TemplateID:    "test-template",
			},
			expectErr: true,
		},
		{
			name: "Empty customer codes",
			message: SQSMessage{
				ChangeID:      "TEST-003",
				Title:         "Test Message",
				CustomerCodes: []string{},
				TemplateID:    "test-template",
			},
			expectErr: true,
		},
		{
			name: "Invalid customer code",
			message: SQSMessage{
				ChangeID:      "TEST-004",
				Title:         "Test Message",
				CustomerCodes: []string{"invalid"},
				TemplateID:    "test-template",
			},
			expectErr: true,
		},
		{
			name: "Missing template ID",
			message: SQSMessage{
				ChangeID:      "TEST-005",
				Title:         "Test Message",
				CustomerCodes: []string{"hts"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateSQSMessage(&tt.message)
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestEnhancedSQSProcessor_ProcessMessage(t *testing.T) {
	config := SQSProcessorConfig{
		QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:          "us-east-1",
		WorkerPoolSize:  2,
		ShutdownTimeout: 5 * time.Second,
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		t.Skipf("Failed to create SQS processor: %v", err)
	}

	// Create test message
	sqsMessage := SQSMessage{
		ChangeID:      "TEST-PROCESS-001",
		Title:         "Test Processing",
		Description:   "Test message processing",
		CustomerCodes: []string{"hts"},
		TemplateID:    "test-template",
		TemplateData: map[string]interface{}{
			"subject": "Test Subject",
			"message": "Test Message",
		},
	}

	messageBody, err := json.Marshal(sqsMessage)
	if err != nil {
		t.Fatalf("Failed to marshal test message: %v", err)
	}

	messageID := "test-message-id"
	receiptHandle := "test-receipt-handle"

	message := &types.Message{
		MessageId:     &messageID,
		Body:          stringPtr(string(messageBody)),
		ReceiptHandle: &receiptHandle,
	}

	// Test message processing
	t.Run("ProcessValidMessage", func(t *testing.T) {
		// This would normally process the message
		// In a real test, you might mock the SQS client
		processor.processMessage(0, message)

		// Check metrics
		metrics := processor.GetMetrics()
		if metrics.MessagesProcessed == 0 {
			t.Error("Expected messages processed count to be > 0")
		}
	})
}

func TestEnhancedSQSProcessor_Metrics(t *testing.T) {
	config := SQSProcessorConfig{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:   "us-east-1",
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		t.Skipf("Failed to create SQS processor: %v", err)
	}

	// Test initial metrics
	metrics := processor.GetMetrics()
	if metrics.MessagesProcessed != 0 {
		t.Errorf("Expected 0 messages processed initially, got %d", metrics.MessagesProcessed)
	}
	if metrics.ErrorRate != 0 {
		t.Errorf("Expected 0%% error rate initially, got %.2f%%", metrics.ErrorRate)
	}

	// Simulate some processing
	processor.metricsLock.Lock()
	processor.messagesProcessed = 10
	processor.messagesSucceeded = 8
	processor.messagesFailed = 2
	processor.metricsLock.Unlock()

	// Check updated metrics
	metrics = processor.GetMetrics()
	if metrics.MessagesProcessed != 10 {
		t.Errorf("Expected 10 messages processed, got %d", metrics.MessagesProcessed)
	}
	if metrics.MessagesSucceeded != 8 {
		t.Errorf("Expected 8 messages succeeded, got %d", metrics.MessagesSucceeded)
	}
	if metrics.MessagesFailed != 2 {
		t.Errorf("Expected 2 messages failed, got %d", metrics.MessagesFailed)
	}
	if metrics.ErrorRate != 20.0 {
		t.Errorf("Expected 20%% error rate, got %.2f%%", metrics.ErrorRate)
	}
}

func TestEnhancedSQSProcessor_GracefulShutdown(t *testing.T) {
	config := SQSProcessorConfig{
		QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:          "us-east-1",
		ShutdownTimeout: 2 * time.Second,
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		t.Skipf("Failed to create SQS processor: %v", err)
	}

	// Test graceful shutdown when not running
	err = processor.GracefulShutdown()
	if err != nil {
		t.Errorf("Graceful shutdown failed when not running: %v", err)
	}

	// Test stop processing
	processor.StopProcessing()

	// Test IsRunning
	if processor.IsRunning() {
		t.Error("Expected processor to not be running after stop")
	}
}

func TestSQSProcessorConfig_Defaults(t *testing.T) {
	config := SQSProcessorConfig{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:   "us-east-1",
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		t.Skipf("Failed to create SQS processor: %v", err)
	}

	// Check that defaults were applied
	if processor.config.WorkerPoolSize != 10 {
		t.Errorf("Expected default WorkerPoolSize 10, got %d", processor.config.WorkerPoolSize)
	}
	if processor.config.MessageBufferSize != 100 {
		t.Errorf("Expected default MessageBufferSize 100, got %d", processor.config.MessageBufferSize)
	}
	if processor.config.ShutdownTimeout != 30*time.Second {
		t.Errorf("Expected default ShutdownTimeout 30s, got %v", processor.config.ShutdownTimeout)
	}
	if processor.config.MaxMessages != 10 {
		t.Errorf("Expected default MaxMessages 10, got %d", processor.config.MaxMessages)
	}
	if processor.config.VisibilityTimeout != 30 {
		t.Errorf("Expected default VisibilityTimeout 30, got %d", processor.config.VisibilityTimeout)
	}
	if processor.config.WaitTimeSeconds != 20 {
		t.Errorf("Expected default WaitTimeSeconds 20, got %d", processor.config.WaitTimeSeconds)
	}
	if processor.config.PollingInterval != 5*time.Second {
		t.Errorf("Expected default PollingInterval 5s, got %v", processor.config.PollingInterval)
	}
}

func TestEnhancedSQSProcessor_ProcessEmailDistribution(t *testing.T) {
	config := SQSProcessorConfig{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:   "us-east-1",
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		t.Skipf("Failed to create SQS processor: %v", err)
	}

	sqsMessage := &SQSMessage{
		ChangeID:      "TEST-DIST-001",
		Title:         "Test Distribution",
		Description:   "Test email distribution",
		CustomerCodes: []string{"hts", "cds"},
		TemplateID:    "test-template",
		TemplateData: map[string]interface{}{
			"subject": "Test Subject",
			"message": "Test Message",
		},
	}

	err = processor.processEmailDistribution(sqsMessage)
	if err != nil {
		t.Errorf("Email distribution processing failed: %v", err)
	}

	// Check that execution was tracked
	executions, err := statusTracker.QueryExecutions(ExecutionQuery{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to query executions: %v", err)
	}

	if len(executions) == 0 {
		t.Error("Expected at least one execution to be tracked")
	}

	// Check that customers were processed
	if len(executions) > 0 {
		execution := executions[0]
		if len(execution.CustomerStatuses) != 2 {
			t.Errorf("Expected 2 customer statuses, got %d", len(execution.CustomerStatuses))
		}
	}
}

// Benchmark tests

func BenchmarkEnhancedSQSProcessor_ValidateMessage(b *testing.B) {
	config := SQSProcessorConfig{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:   "us-east-1",
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		b.Skipf("Failed to create SQS processor: %v", err)
	}

	message := &SQSMessage{
		ChangeID:      "BENCH-001",
		Title:         "Benchmark Message",
		CustomerCodes: []string{"hts"},
		TemplateID:    "test-template",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.validateSQSMessage(message)
	}
}

func BenchmarkEnhancedSQSProcessor_GetMetrics(b *testing.B) {
	config := SQSProcessorConfig{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
		Region:   "us-east-1",
	}

	customerManager := createTestCustomerManager()
	templateManager := NewEmailTemplateManager(customerManager)
	sesManager := NewSESIntegrationManager(customerManager, templateManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

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
		b.Skipf("Failed to create SQS processor: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.GetMetrics()
	}
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}

func createTestSQSMessage() SQSMessage {
	return SQSMessage{
		ChangeID:      "TEST-001",
		Title:         "Test Message",
		Description:   "Test SQS message",
		CustomerCodes: []string{"hts", "cds"},
		TemplateID:    "test-template",
		Priority:      "normal",
		TemplateData: map[string]interface{}{
			"subject": "Test Subject",
			"message": "Test Message Content",
		},
		ScheduledAt: time.Now(),
	}
}
