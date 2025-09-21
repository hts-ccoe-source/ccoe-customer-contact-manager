package main

import (
	"testing"
	"time"
)

func TestNewExecutionStatusTracker(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	tracker := NewExecutionStatusTracker(customerManager)

	if tracker.customerManager != customerManager {
		t.Errorf("Expected customer manager to be set")
	}

	if tracker.executions == nil {
		t.Errorf("Expected executions map to be initialized")
	}

	if tracker.persistenceType != "memory" {
		t.Errorf("Expected default persistence type to be 'memory', got '%s'", tracker.persistenceType)
	}
}

func TestStartExecution(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	customerCodes := []string{"hts", "cds"}
	execution, err := tracker.StartExecution("change-123", "Test Change", "Test description", "test-user", customerCodes)

	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	if execution.ExecutionID == "" {
		t.Errorf("Expected execution ID to be generated")
	}

	if execution.ChangeID != "change-123" {
		t.Errorf("Expected change ID 'change-123', got '%s'", execution.ChangeID)
	}

	if execution.Title != "Test Change" {
		t.Errorf("Expected title 'Test Change', got '%s'", execution.Title)
	}

	if execution.InitiatedBy != "test-user" {
		t.Errorf("Expected initiated by 'test-user', got '%s'", execution.InitiatedBy)
	}

	if execution.Status != StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", execution.Status)
	}

	if execution.TotalCustomers != 2 {
		t.Errorf("Expected 2 total customers, got %d", execution.TotalCustomers)
	}

	if len(execution.CustomerStatuses) != 2 {
		t.Errorf("Expected 2 customer statuses, got %d", len(execution.CustomerStatuses))
	}

	// Verify customer statuses
	for _, customerCode := range customerCodes {
		customerStatus, exists := execution.CustomerStatuses[customerCode]
		if !exists {
			t.Errorf("Expected customer status for '%s' to exist", customerCode)
			continue
		}

		if customerStatus.CustomerCode != customerCode {
			t.Errorf("Expected customer code '%s', got '%s'", customerCode, customerStatus.CustomerCode)
		}

		if customerStatus.Status != CustomerStatusPending {
			t.Errorf("Expected customer status 'pending', got '%s'", customerStatus.Status)
		}
	}

	// Test with invalid customer
	_, err = tracker.StartExecution("change-456", "Invalid Change", "Test", "test-user", []string{"invalid"})
	if err == nil {
		t.Errorf("Expected error for invalid customer code, got none")
	}
}

func TestUpdateExecutionStatus(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	execution, err := tracker.StartExecution("change-123", "Test Change", "Test", "test-user", []string{"hts"})
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// Update to running
	err = tracker.UpdateExecutionStatus(execution.ExecutionID, StatusRunning)
	if err != nil {
		t.Fatalf("Failed to update execution status: %v", err)
	}

	updatedExecution, err := tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	if updatedExecution.Status != StatusRunning {
		t.Errorf("Expected status 'running', got '%s'", updatedExecution.Status)
	}

	// Update to completed
	err = tracker.UpdateExecutionStatus(execution.ExecutionID, StatusCompleted)
	if err != nil {
		t.Fatalf("Failed to update execution status: %v", err)
	}

	updatedExecution, err = tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	if updatedExecution.Status != StatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", updatedExecution.Status)
	}

	if updatedExecution.CompletedAt == nil {
		t.Errorf("Expected completed at timestamp to be set")
	}

	if updatedExecution.ActualDuration == nil {
		t.Errorf("Expected actual duration to be calculated")
	}

	if updatedExecution.Metrics == nil {
		t.Errorf("Expected metrics to be calculated")
	}

	// Test with non-existent execution
	err = tracker.UpdateExecutionStatus("non-existent", StatusCompleted)
	if err == nil {
		t.Errorf("Expected error for non-existent execution, got none")
	}
}

func TestStartCustomerExecution(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	execution, err := tracker.StartExecution("change-123", "Test Change", "Test", "test-user", []string{"hts", "cds"})
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// Start customer execution
	err = tracker.StartCustomerExecution(execution.ExecutionID, "hts")
	if err != nil {
		t.Fatalf("Failed to start customer execution: %v", err)
	}

	updatedExecution, err := tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	// Check overall execution status
	if updatedExecution.Status != StatusRunning {
		t.Errorf("Expected overall status 'running', got '%s'", updatedExecution.Status)
	}

	// Check customer status
	htsStatus := updatedExecution.CustomerStatuses["hts"]
	if htsStatus.Status != CustomerStatusRunning {
		t.Errorf("Expected HTS status 'running', got '%s'", htsStatus.Status)
	}

	if htsStatus.StartedAt == nil {
		t.Errorf("Expected HTS started at timestamp to be set")
	}

	// Test with non-existent execution
	err = tracker.StartCustomerExecution("non-existent", "hts")
	if err == nil {
		t.Errorf("Expected error for non-existent execution, got none")
	}

	// Test with non-existent customer
	err = tracker.StartCustomerExecution(execution.ExecutionID, "non-existent")
	if err == nil {
		t.Errorf("Expected error for non-existent customer, got none")
	}
}

func TestCompleteCustomerExecution(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	execution, err := tracker.StartExecution("change-123", "Test Change", "Test", "test-user", []string{"hts", "cds"})
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// Start both customers
	tracker.StartCustomerExecution(execution.ExecutionID, "hts")
	tracker.StartCustomerExecution(execution.ExecutionID, "cds")

	// Complete HTS successfully
	err = tracker.CompleteCustomerExecution(execution.ExecutionID, "hts", true, "")
	if err != nil {
		t.Fatalf("Failed to complete HTS execution: %v", err)
	}

	updatedExecution, err := tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	// Check HTS status
	htsStatus := updatedExecution.CustomerStatuses["hts"]
	if htsStatus.Status != CustomerStatusCompleted {
		t.Errorf("Expected HTS status 'completed', got '%s'", htsStatus.Status)
	}

	if htsStatus.CompletedAt == nil {
		t.Errorf("Expected HTS completed at timestamp to be set")
	}

	if htsStatus.Duration == nil {
		t.Errorf("Expected HTS duration to be calculated")
	}

	// Overall execution should still be running
	if updatedExecution.Status != StatusRunning {
		t.Errorf("Expected overall status 'running', got '%s'", updatedExecution.Status)
	}

	// Complete CDS with failure
	err = tracker.CompleteCustomerExecution(execution.ExecutionID, "cds", false, "Connection timeout")
	if err != nil {
		t.Fatalf("Failed to complete CDS execution: %v", err)
	}

	updatedExecution, err = tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	// Check CDS status
	cdsStatus := updatedExecution.CustomerStatuses["cds"]
	if cdsStatus.Status != CustomerStatusFailed {
		t.Errorf("Expected CDS status 'failed', got '%s'", cdsStatus.Status)
	}

	if cdsStatus.ErrorMessage != "Connection timeout" {
		t.Errorf("Expected CDS error message 'Connection timeout', got '%s'", cdsStatus.ErrorMessage)
	}

	// Overall execution should be partial (some succeeded, some failed)
	if updatedExecution.Status != StatusPartial {
		t.Errorf("Expected overall status 'partial', got '%s'", updatedExecution.Status)
	}

	if updatedExecution.CompletedAt == nil {
		t.Errorf("Expected completed at timestamp to be set")
	}

	if updatedExecution.ErrorSummary == nil {
		t.Errorf("Expected error summary to be generated")
	}
}

func TestAddExecutionStep(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	execution, err := tracker.StartExecution("change-123", "Test Change", "Test", "test-user", []string{"hts"})
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// Add execution step
	err = tracker.AddExecutionStep(execution.ExecutionID, "hts", "step-1", "Send Email", "Send notification email")
	if err != nil {
		t.Fatalf("Failed to add execution step: %v", err)
	}

	updatedExecution, err := tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	htsStatus := updatedExecution.CustomerStatuses["hts"]
	if len(htsStatus.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(htsStatus.Steps))
	}

	step := htsStatus.Steps[0]
	if step.StepID != "step-1" {
		t.Errorf("Expected step ID 'step-1', got '%s'", step.StepID)
	}

	if step.Name != "Send Email" {
		t.Errorf("Expected step name 'Send Email', got '%s'", step.Name)
	}

	if step.Status != StepStatusPending {
		t.Errorf("Expected step status 'pending', got '%s'", step.Status)
	}
}

func TestUpdateExecutionStep(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	execution, err := tracker.StartExecution("change-123", "Test Change", "Test", "test-user", []string{"hts"})
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// Add and update execution step
	tracker.AddExecutionStep(execution.ExecutionID, "hts", "step-1", "Send Email", "Send notification email")

	// Update step to running
	err = tracker.UpdateExecutionStep(execution.ExecutionID, "hts", "step-1", StepStatusRunning, "")
	if err != nil {
		t.Fatalf("Failed to update execution step: %v", err)
	}

	updatedExecution, err := tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	step := updatedExecution.CustomerStatuses["hts"].Steps[0]
	if step.Status != StepStatusRunning {
		t.Errorf("Expected step status 'running', got '%s'", step.Status)
	}

	if step.StartedAt == nil {
		t.Errorf("Expected step started at timestamp to be set")
	}

	// Update step to completed
	err = tracker.UpdateExecutionStep(execution.ExecutionID, "hts", "step-1", StepStatusCompleted, "")
	if err != nil {
		t.Fatalf("Failed to update execution step: %v", err)
	}

	updatedExecution, err = tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	step = updatedExecution.CustomerStatuses["hts"].Steps[0]
	if step.Status != StepStatusCompleted {
		t.Errorf("Expected step status 'completed', got '%s'", step.Status)
	}

	if step.CompletedAt == nil {
		t.Errorf("Expected step completed at timestamp to be set")
	}

	if step.Duration == nil {
		t.Errorf("Expected step duration to be calculated")
	}

	// Update step to failed
	tracker.AddExecutionStep(execution.ExecutionID, "hts", "step-2", "Validate", "Validate data")
	err = tracker.UpdateExecutionStep(execution.ExecutionID, "hts", "step-2", StepStatusFailed, "Validation error")
	if err != nil {
		t.Fatalf("Failed to update execution step: %v", err)
	}

	updatedExecution, err = tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	step2 := updatedExecution.CustomerStatuses["hts"].Steps[1]
	if step2.Status != StepStatusFailed {
		t.Errorf("Expected step status 'failed', got '%s'", step2.Status)
	}

	if step2.ErrorMessage != "Validation error" {
		t.Errorf("Expected step error message 'Validation error', got '%s'", step2.ErrorMessage)
	}
}

func TestQueryExecutions(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	// Create multiple executions
	exec1, _ := tracker.StartExecution("change-1", "Change 1", "Test 1", "user1", []string{"hts"})
	exec2, _ := tracker.StartExecution("change-2", "Change 2", "Test 2", "user2", []string{"cds"})
	exec3, _ := tracker.StartExecution("change-3", "Change 3", "Test 3", "user1", []string{"hts", "cds"})

	// Update statuses
	tracker.UpdateExecutionStatus(exec1.ExecutionID, StatusCompleted)
	tracker.UpdateExecutionStatus(exec2.ExecutionID, StatusFailed)
	// exec3 remains pending

	// Query all executions
	query := ExecutionQuery{}
	results, err := tracker.QueryExecutions(query)
	if err != nil {
		t.Fatalf("Failed to query executions: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 executions, got %d", len(results))
	}

	// Query by status
	query = ExecutionQuery{Status: []ExecutionStatusType{StatusCompleted}}
	results, err = tracker.QueryExecutions(query)
	if err != nil {
		t.Fatalf("Failed to query executions by status: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 completed execution, got %d", len(results))
	}

	if results[0].ExecutionID != exec1.ExecutionID {
		t.Errorf("Expected execution ID '%s', got '%s'", exec1.ExecutionID, results[0].ExecutionID)
	}

	// Query by initiator
	query = ExecutionQuery{InitiatedBy: "user1"}
	results, err = tracker.QueryExecutions(query)
	if err != nil {
		t.Fatalf("Failed to query executions by initiator: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 executions by user1, got %d", len(results))
	}

	// Query by customer
	query = ExecutionQuery{CustomerCode: "hts"}
	results, err = tracker.QueryExecutions(query)
	if err != nil {
		t.Fatalf("Failed to query executions by customer: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 executions for HTS, got %d", len(results))
	}

	// Query with limit
	query = ExecutionQuery{Limit: 2}
	results, err = tracker.QueryExecutions(query)
	if err != nil {
		t.Fatalf("Failed to query executions with limit: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 executions with limit, got %d", len(results))
	}
}

func TestGetExecutionSummary(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	// Create executions with different statuses
	exec1, _ := tracker.StartExecution("change-1", "Change 1", "Test 1", "user1", []string{"hts"})
	exec2, _ := tracker.StartExecution("change-2", "Change 2", "Test 2", "user2", []string{"cds"})
	exec3, _ := tracker.StartExecution("change-3", "Change 3", "Test 3", "user1", []string{"hts", "cds"})

	// Complete executions
	tracker.UpdateExecutionStatus(exec1.ExecutionID, StatusCompleted)
	tracker.UpdateExecutionStatus(exec2.ExecutionID, StatusFailed)
	tracker.UpdateExecutionStatus(exec3.ExecutionID, StatusCompleted)

	// Get summary
	summary, err := tracker.GetExecutionSummary(24 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to get execution summary: %v", err)
	}

	if summary.TotalExecutions != 3 {
		t.Errorf("Expected 3 total executions, got %d", summary.TotalExecutions)
	}

	if summary.StatusCounts[StatusCompleted] != 2 {
		t.Errorf("Expected 2 completed executions, got %d", summary.StatusCounts[StatusCompleted])
	}

	if summary.StatusCounts[StatusFailed] != 1 {
		t.Errorf("Expected 1 failed execution, got %d", summary.StatusCounts[StatusFailed])
	}

	if len(summary.CustomerStats) == 0 {
		t.Errorf("Expected customer statistics to be populated")
	}

	if summary.PerformanceStats == nil {
		t.Errorf("Expected performance statistics to be calculated")
	}

	// Check customer statistics
	if htsStats, exists := summary.CustomerStats["hts"]; exists {
		if htsStats.TotalExecutions != 2 {
			t.Errorf("Expected 2 HTS executions, got %d", htsStats.TotalExecutions)
		}
	} else {
		t.Errorf("Expected HTS customer statistics")
	}
}

func TestErrorSummaryGeneration(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	execution, err := tracker.StartExecution("change-123", "Test Change", "Test", "test-user", []string{"hts", "cds"})
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// Start customers
	tracker.StartCustomerExecution(execution.ExecutionID, "hts")
	tracker.StartCustomerExecution(execution.ExecutionID, "cds")

	// Complete with different errors
	tracker.CompleteCustomerExecution(execution.ExecutionID, "hts", false, "Authentication failed")
	tracker.CompleteCustomerExecution(execution.ExecutionID, "cds", false, "Network timeout")

	updatedExecution, err := tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	if updatedExecution.ErrorSummary == nil {
		t.Fatalf("Expected error summary to be generated")
	}

	errorSummary := updatedExecution.ErrorSummary

	if errorSummary.TotalErrors != 2 {
		t.Errorf("Expected 2 total errors, got %d", errorSummary.TotalErrors)
	}

	if len(errorSummary.ErrorsByCustomer) != 2 {
		t.Errorf("Expected errors for 2 customers, got %d", len(errorSummary.ErrorsByCustomer))
	}

	if errorSummary.ErrorsByCustomer["hts"] != 1 {
		t.Errorf("Expected 1 error for HTS, got %d", errorSummary.ErrorsByCustomer["hts"])
	}

	if len(errorSummary.ErrorDetails) != 2 {
		t.Errorf("Expected 2 error details, got %d", len(errorSummary.ErrorDetails))
	}

	// Check error categorization
	if len(errorSummary.ErrorsByType) == 0 {
		t.Errorf("Expected error types to be categorized")
	}
}

func TestMetricsCalculation(t *testing.T) {
	customerManager := setupTestCustomerManager()
	tracker := NewExecutionStatusTracker(customerManager)

	execution, err := tracker.StartExecution("change-123", "Test Change", "Test", "test-user", []string{"hts", "cds"})
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// Start and complete customers with different durations
	tracker.StartCustomerExecution(execution.ExecutionID, "hts")
	time.Sleep(10 * time.Millisecond) // Simulate processing time
	tracker.CompleteCustomerExecution(execution.ExecutionID, "hts", true, "")

	tracker.StartCustomerExecution(execution.ExecutionID, "cds")
	time.Sleep(20 * time.Millisecond) // Simulate longer processing time
	tracker.CompleteCustomerExecution(execution.ExecutionID, "cds", true, "")

	updatedExecution, err := tracker.GetExecution(execution.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}

	if updatedExecution.Metrics == nil {
		t.Fatalf("Expected metrics to be calculated")
	}

	metrics := updatedExecution.Metrics

	if metrics.TotalDuration == 0 {
		t.Errorf("Expected total duration to be calculated")
	}

	if metrics.AverageDurationPerCustomer == 0 {
		t.Errorf("Expected average duration per customer to be calculated")
	}

	if metrics.SuccessRate != 100.0 {
		t.Errorf("Expected 100%% success rate, got %.1f%%", metrics.SuccessRate)
	}

	if metrics.FastestCustomer == "" {
		t.Errorf("Expected fastest customer to be identified")
	}

	if metrics.SlowestCustomer == "" {
		t.Errorf("Expected slowest customer to be identified")
	}

	if metrics.ThroughputPerMinute == 0 {
		t.Errorf("Expected throughput per minute to be calculated")
	}
}

// Helper function to set up test customer manager
func setupTestCustomerManager() *CustomerCredentialManager {
	customerManager := NewCustomerCredentialManager("us-east-1")

	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Global",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
		},
	}

	return customerManager
}
