package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ExecutionStatusTracker manages execution status across multi-customer operations
type ExecutionStatusTracker struct {
	executions        map[string]*ExecutionStatus
	customerManager   *CustomerCredentialManager
	mutex             sync.RWMutex
	persistenceType   string // "memory", "s3", "dynamodb"
	persistenceConfig map[string]interface{}
}

// ExecutionStatus represents the status of a multi-customer execution
type ExecutionStatus struct {
	ExecutionID       string                        `json:"executionId"`
	ChangeID          string                        `json:"changeId"`
	Title             string                        `json:"title"`
	Description       string                        `json:"description"`
	InitiatedBy       string                        `json:"initiatedBy"`
	InitiatedAt       time.Time                     `json:"initiatedAt"`
	CompletedAt       *time.Time                    `json:"completedAt,omitempty"`
	Status            ExecutionStatusType           `json:"status"`
	TotalCustomers    int                           `json:"totalCustomers"`
	CustomerStatuses  map[string]*CustomerExecution `json:"customerStatuses"`
	Metadata          map[string]interface{}        `json:"metadata,omitempty"`
	ErrorSummary      *ErrorSummary                 `json:"errorSummary,omitempty"`
	Metrics           *ExecutionMetrics             `json:"metrics,omitempty"`
	Tags              map[string]string             `json:"tags,omitempty"`
	Priority          string                        `json:"priority"` // high, normal, low
	EstimatedDuration time.Duration                 `json:"estimatedDuration,omitempty"`
	ActualDuration    *time.Duration                `json:"actualDuration,omitempty"`
}

// ExecutionStatusType represents the overall execution status
type ExecutionStatusType string

const (
	StatusPending   ExecutionStatusType = "pending"
	StatusRunning   ExecutionStatusType = "running"
	StatusCompleted ExecutionStatusType = "completed"
	StatusFailed    ExecutionStatusType = "failed"
	StatusCancelled ExecutionStatusType = "cancelled"
	StatusPartial   ExecutionStatusType = "partial" // Some customers succeeded, some failed
)

// CustomerExecution represents the execution status for a specific customer
type CustomerExecution struct {
	CustomerCode      string                 `json:"customerCode"`
	CustomerName      string                 `json:"customerName"`
	Status            CustomerStatusType     `json:"status"`
	StartedAt         *time.Time             `json:"startedAt,omitempty"`
	CompletedAt       *time.Time             `json:"completedAt,omitempty"`
	Duration          *time.Duration         `json:"duration,omitempty"`
	Steps             []ExecutionStep        `json:"steps"`
	ErrorMessage      string                 `json:"errorMessage,omitempty"`
	ErrorCode         string                 `json:"errorCode,omitempty"`
	RetryCount        int                    `json:"retryCount"`
	LastRetryAt       *time.Time             `json:"lastRetryAt,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ResourcesAffected []string               `json:"resourcesAffected,omitempty"`
	EmailsSent        int                    `json:"emailsSent"`
	EmailsDelivered   int                    `json:"emailsDelivered"`
	EmailsFailed      int                    `json:"emailsFailed"`
}

// CustomerStatusType represents the status for a customer's execution
type CustomerStatusType string

const (
	CustomerStatusPending   CustomerStatusType = "pending"
	CustomerStatusRunning   CustomerStatusType = "running"
	CustomerStatusCompleted CustomerStatusType = "completed"
	CustomerStatusFailed    CustomerStatusType = "failed"
	CustomerStatusSkipped   CustomerStatusType = "skipped"
	CustomerStatusRetrying  CustomerStatusType = "retrying"
)

// ExecutionStep represents a step in the customer execution process
type ExecutionStep struct {
	StepID       string                 `json:"stepId"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Status       StepStatusType         `json:"status"`
	StartedAt    *time.Time             `json:"startedAt,omitempty"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
	Duration     *time.Duration         `json:"duration,omitempty"`
	ErrorMessage string                 `json:"errorMessage,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	RetryCount   int                    `json:"retryCount"`
}

// StepStatusType represents the status of an execution step
type StepStatusType string

const (
	StepStatusPending   StepStatusType = "pending"
	StepStatusRunning   StepStatusType = "running"
	StepStatusCompleted StepStatusType = "completed"
	StepStatusFailed    StepStatusType = "failed"
	StepStatusSkipped   StepStatusType = "skipped"
)

// ErrorSummary provides a summary of errors across the execution
type ErrorSummary struct {
	TotalErrors      int            `json:"totalErrors"`
	ErrorsByType     map[string]int `json:"errorsByType"`
	ErrorsByCustomer map[string]int `json:"errorsByCustomer"`
	CriticalErrors   int            `json:"criticalErrors"`
	RetryableErrors  int            `json:"retryableErrors"`
	PermanentErrors  int            `json:"permanentErrors"`
	MostCommonError  string         `json:"mostCommonError,omitempty"`
	ErrorDetails     []ErrorDetail  `json:"errorDetails,omitempty"`
}

// ErrorDetail represents detailed error information
type ErrorDetail struct {
	CustomerCode string                 `json:"customerCode"`
	StepID       string                 `json:"stepId"`
	ErrorType    string                 `json:"errorType"`
	ErrorMessage string                 `json:"errorMessage"`
	ErrorCode    string                 `json:"errorCode"`
	Timestamp    time.Time              `json:"timestamp"`
	Retryable    bool                   `json:"retryable"`
	Severity     string                 `json:"severity"` // critical, high, medium, low
	Context      map[string]interface{} `json:"context,omitempty"`
}

// ExecutionMetrics provides performance and operational metrics
type ExecutionMetrics struct {
	TotalDuration              time.Duration            `json:"totalDuration"`
	AverageDurationPerCustomer time.Duration            `json:"averageDurationPerCustomer"`
	FastestCustomer            string                   `json:"fastestCustomer"`
	SlowestCustomer            string                   `json:"slowestCustomer"`
	SuccessRate                float64                  `json:"successRate"`
	ThroughputPerMinute        float64                  `json:"throughputPerMinute"`
	ResourceUtilization        map[string]interface{}   `json:"resourceUtilization,omitempty"`
	PerformanceBreakdown       map[string]time.Duration `json:"performanceBreakdown,omitempty"`
}

// ExecutionQuery represents query parameters for searching executions
type ExecutionQuery struct {
	Status       []ExecutionStatusType `json:"status,omitempty"`
	CustomerCode string                `json:"customerCode,omitempty"`
	InitiatedBy  string                `json:"initiatedBy,omitempty"`
	StartTime    *time.Time            `json:"startTime,omitempty"`
	EndTime      *time.Time            `json:"endTime,omitempty"`
	Tags         map[string]string     `json:"tags,omitempty"`
	Priority     string                `json:"priority,omitempty"`
	Limit        int                   `json:"limit,omitempty"`
	Offset       int                   `json:"offset,omitempty"`
}

// NewExecutionStatusTracker creates a new execution status tracker
func NewExecutionStatusTracker(customerManager *CustomerCredentialManager) *ExecutionStatusTracker {
	return &ExecutionStatusTracker{
		executions:        make(map[string]*ExecutionStatus),
		customerManager:   customerManager,
		persistenceType:   "memory", // Default to in-memory
		persistenceConfig: make(map[string]interface{}),
	}
}

// StartExecution creates and starts tracking a new execution
func (est *ExecutionStatusTracker) StartExecution(changeID, title, description, initiatedBy string, customerCodes []string) (*ExecutionStatus, error) {
	est.mutex.Lock()
	defer est.mutex.Unlock()

	executionID := generateExecutionID()

	// Initialize customer statuses
	customerStatuses := make(map[string]*CustomerExecution)
	for _, customerCode := range customerCodes {
		customerInfo, err := est.customerManager.GetCustomerAccountInfo(customerCode)
		if err != nil {
			return nil, fmt.Errorf("invalid customer code %s: %v", customerCode, err)
		}

		customerStatuses[customerCode] = &CustomerExecution{
			CustomerCode: customerCode,
			CustomerName: customerInfo.CustomerName,
			Status:       CustomerStatusPending,
			Steps:        []ExecutionStep{},
			RetryCount:   0,
			Metadata:     make(map[string]interface{}),
		}
	}

	execution := &ExecutionStatus{
		ExecutionID:      executionID,
		ChangeID:         changeID,
		Title:            title,
		Description:      description,
		InitiatedBy:      initiatedBy,
		InitiatedAt:      time.Now(),
		Status:           StatusPending,
		TotalCustomers:   len(customerCodes),
		CustomerStatuses: customerStatuses,
		Metadata:         make(map[string]interface{}),
		Tags:             make(map[string]string),
		Priority:         "normal",
	}

	est.executions[executionID] = execution

	// Persist to storage
	if err := est.persistExecution(execution); err != nil {
		return nil, fmt.Errorf("failed to persist execution: %v", err)
	}

	return execution, nil
}

// UpdateExecutionStatus updates the overall execution status
func (est *ExecutionStatusTracker) UpdateExecutionStatus(executionID string, status ExecutionStatusType) error {
	est.mutex.Lock()
	defer est.mutex.Unlock()

	execution, exists := est.executions[executionID]
	if !exists {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	execution.Status = status

	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		now := time.Now()
		execution.CompletedAt = &now
		duration := now.Sub(execution.InitiatedAt)
		execution.ActualDuration = &duration

		// Calculate metrics
		execution.Metrics = est.calculateMetrics(execution)

		// Generate error summary if there were errors
		if status == StatusFailed || status == StatusPartial {
			execution.ErrorSummary = est.generateErrorSummary(execution)
		}
	}

	return est.persistExecution(execution)
}

// StartCustomerExecution starts execution for a specific customer
func (est *ExecutionStatusTracker) StartCustomerExecution(executionID, customerCode string) error {
	est.mutex.Lock()
	defer est.mutex.Unlock()

	execution, exists := est.executions[executionID]
	if !exists {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	customerExecution, exists := execution.CustomerStatuses[customerCode]
	if !exists {
		return fmt.Errorf("customer not found in execution: %s", customerCode)
	}

	now := time.Now()
	customerExecution.Status = CustomerStatusRunning
	customerExecution.StartedAt = &now

	// Update overall execution status if this is the first customer to start
	if execution.Status == StatusPending {
		execution.Status = StatusRunning
	}

	return est.persistExecution(execution)
}

// CompleteCustomerExecution completes execution for a specific customer
func (est *ExecutionStatusTracker) CompleteCustomerExecution(executionID, customerCode string, success bool, errorMessage string) error {
	est.mutex.Lock()
	defer est.mutex.Unlock()

	execution, exists := est.executions[executionID]
	if !exists {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	customerExecution, exists := execution.CustomerStatuses[customerCode]
	if !exists {
		return fmt.Errorf("customer not found in execution: %s", customerCode)
	}

	now := time.Now()
	customerExecution.CompletedAt = &now

	if customerExecution.StartedAt != nil {
		duration := now.Sub(*customerExecution.StartedAt)
		customerExecution.Duration = &duration
	}

	if success {
		customerExecution.Status = CustomerStatusCompleted
	} else {
		customerExecution.Status = CustomerStatusFailed
		customerExecution.ErrorMessage = errorMessage
	}

	// Check if all customers are complete
	allComplete := true
	anyFailed := false

	for _, custExec := range execution.CustomerStatuses {
		if custExec.Status == CustomerStatusPending || custExec.Status == CustomerStatusRunning {
			allComplete = false
			break
		}
		if custExec.Status == CustomerStatusFailed {
			anyFailed = true
		}
	}

	if allComplete {
		if anyFailed {
			execution.Status = StatusPartial
		} else {
			execution.Status = StatusCompleted
		}

		now := time.Now()
		execution.CompletedAt = &now
		duration := now.Sub(execution.InitiatedAt)
		execution.ActualDuration = &duration

		// Calculate final metrics
		execution.Metrics = est.calculateMetrics(execution)

		// Generate error summary if needed
		if anyFailed {
			execution.ErrorSummary = est.generateErrorSummary(execution)
		}
	}

	return est.persistExecution(execution)
}

// AddExecutionStep adds a step to a customer's execution
func (est *ExecutionStatusTracker) AddExecutionStep(executionID, customerCode, stepID, name, description string) error {
	est.mutex.Lock()
	defer est.mutex.Unlock()

	execution, exists := est.executions[executionID]
	if !exists {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	customerExecution, exists := execution.CustomerStatuses[customerCode]
	if !exists {
		return fmt.Errorf("customer not found in execution: %s", customerCode)
	}

	step := ExecutionStep{
		StepID:      stepID,
		Name:        name,
		Description: description,
		Status:      StepStatusPending,
		Metadata:    make(map[string]interface{}),
		RetryCount:  0,
	}

	customerExecution.Steps = append(customerExecution.Steps, step)

	return est.persistExecution(execution)
}

// UpdateExecutionStep updates the status of an execution step
func (est *ExecutionStatusTracker) UpdateExecutionStep(executionID, customerCode, stepID string, status StepStatusType, errorMessage string) error {
	est.mutex.Lock()
	defer est.mutex.Unlock()

	execution, exists := est.executions[executionID]
	if !exists {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	customerExecution, exists := execution.CustomerStatuses[customerCode]
	if !exists {
		return fmt.Errorf("customer not found in execution: %s", customerCode)
	}

	// Find and update the step
	for i := range customerExecution.Steps {
		if customerExecution.Steps[i].StepID == stepID {
			step := &customerExecution.Steps[i]
			step.Status = status

			now := time.Now()

			if status == StepStatusRunning && step.StartedAt == nil {
				step.StartedAt = &now
			}

			if status == StepStatusCompleted || status == StepStatusFailed {
				step.CompletedAt = &now
				if step.StartedAt != nil {
					duration := now.Sub(*step.StartedAt)
					step.Duration = &duration
				}

				if status == StepStatusFailed {
					step.ErrorMessage = errorMessage
				}
			}

			break
		}
	}

	return est.persistExecution(execution)
}

// GetExecution retrieves an execution by ID
func (est *ExecutionStatusTracker) GetExecution(executionID string) (*ExecutionStatus, error) {
	est.mutex.RLock()
	defer est.mutex.RUnlock()

	execution, exists := est.executions[executionID]
	if !exists {
		// Try to load from persistence
		if loadedExecution, err := est.loadExecution(executionID); err == nil {
			est.executions[executionID] = loadedExecution
			return loadedExecution, nil
		}
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	return execution, nil
}

// QueryExecutions searches for executions based on criteria
func (est *ExecutionStatusTracker) QueryExecutions(query ExecutionQuery) ([]*ExecutionStatus, error) {
	est.mutex.RLock()
	defer est.mutex.RUnlock()

	var results []*ExecutionStatus

	for _, execution := range est.executions {
		if est.matchesQuery(execution, query) {
			results = append(results, execution)
		}
	}

	// Sort by initiation time (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].InitiatedAt.After(results[j].InitiatedAt)
	})

	// Apply pagination
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}

	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results, nil
}

// GetExecutionSummary returns a summary of execution statistics
func (est *ExecutionStatusTracker) GetExecutionSummary(timeRange time.Duration) (*ExecutionSummaryReport, error) {
	est.mutex.RLock()
	defer est.mutex.RUnlock()

	cutoffTime := time.Now().Add(-timeRange)

	summary := &ExecutionSummaryReport{
		TimeRange:        timeRange,
		GeneratedAt:      time.Now(),
		StatusCounts:     make(map[ExecutionStatusType]int),
		CustomerStats:    make(map[string]*CustomerStats),
		PerformanceStats: &PerformanceStats{},
	}

	var totalDurations []time.Duration
	var successfulExecutions int

	for _, execution := range est.executions {
		if execution.InitiatedAt.Before(cutoffTime) {
			continue
		}

		summary.TotalExecutions++
		summary.StatusCounts[execution.Status]++

		if execution.ActualDuration != nil {
			totalDurations = append(totalDurations, *execution.ActualDuration)
		}

		if execution.Status == StatusCompleted {
			successfulExecutions++
		}

		// Aggregate customer statistics
		for customerCode, customerExec := range execution.CustomerStatuses {
			if _, exists := summary.CustomerStats[customerCode]; !exists {
				summary.CustomerStats[customerCode] = &CustomerStats{
					CustomerCode: customerCode,
					StatusCounts: make(map[CustomerStatusType]int),
				}
			}

			customerStats := summary.CustomerStats[customerCode]
			customerStats.TotalExecutions++
			customerStats.StatusCounts[customerExec.Status]++

			if customerExec.Duration != nil {
				customerStats.TotalDuration += *customerExec.Duration
				customerStats.AverageDuration = customerStats.TotalDuration / time.Duration(customerStats.TotalExecutions)
			}
		}
	}

	// Calculate performance statistics
	if len(totalDurations) > 0 {
		var totalDuration time.Duration
		for _, duration := range totalDurations {
			totalDuration += duration
		}

		summary.PerformanceStats.AverageDuration = totalDuration / time.Duration(len(totalDurations))
		summary.PerformanceStats.SuccessRate = float64(successfulExecutions) / float64(summary.TotalExecutions) * 100

		// Find min and max durations
		sort.Slice(totalDurations, func(i, j int) bool {
			return totalDurations[i] < totalDurations[j]
		})

		summary.PerformanceStats.MinDuration = totalDurations[0]
		summary.PerformanceStats.MaxDuration = totalDurations[len(totalDurations)-1]

		// Calculate median
		mid := len(totalDurations) / 2
		if len(totalDurations)%2 == 0 {
			summary.PerformanceStats.MedianDuration = (totalDurations[mid-1] + totalDurations[mid]) / 2
		} else {
			summary.PerformanceStats.MedianDuration = totalDurations[mid]
		}
	}

	return summary, nil
}

// ExecutionSummaryReport provides summary statistics
type ExecutionSummaryReport struct {
	TimeRange        time.Duration               `json:"timeRange"`
	GeneratedAt      time.Time                   `json:"generatedAt"`
	TotalExecutions  int                         `json:"totalExecutions"`
	StatusCounts     map[ExecutionStatusType]int `json:"statusCounts"`
	CustomerStats    map[string]*CustomerStats   `json:"customerStats"`
	PerformanceStats *PerformanceStats           `json:"performanceStats"`
}

// CustomerStats provides statistics for a specific customer
type CustomerStats struct {
	CustomerCode    string                     `json:"customerCode"`
	TotalExecutions int                        `json:"totalExecutions"`
	StatusCounts    map[CustomerStatusType]int `json:"statusCounts"`
	TotalDuration   time.Duration              `json:"totalDuration"`
	AverageDuration time.Duration              `json:"averageDuration"`
}

// PerformanceStats provides performance statistics
type PerformanceStats struct {
	AverageDuration time.Duration `json:"averageDuration"`
	MedianDuration  time.Duration `json:"medianDuration"`
	MinDuration     time.Duration `json:"minDuration"`
	MaxDuration     time.Duration `json:"maxDuration"`
	SuccessRate     float64       `json:"successRate"`
}

// Helper methods

func (est *ExecutionStatusTracker) matchesQuery(execution *ExecutionStatus, query ExecutionQuery) bool {
	// Check status filter
	if len(query.Status) > 0 {
		found := false
		for _, status := range query.Status {
			if execution.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check customer code filter
	if query.CustomerCode != "" {
		if _, exists := execution.CustomerStatuses[query.CustomerCode]; !exists {
			return false
		}
	}

	// Check initiator filter
	if query.InitiatedBy != "" && execution.InitiatedBy != query.InitiatedBy {
		return false
	}

	// Check time range filters
	if query.StartTime != nil && execution.InitiatedAt.Before(*query.StartTime) {
		return false
	}

	if query.EndTime != nil && execution.InitiatedAt.After(*query.EndTime) {
		return false
	}

	// Check priority filter
	if query.Priority != "" && execution.Priority != query.Priority {
		return false
	}

	// Check tags filter
	if len(query.Tags) > 0 {
		for key, value := range query.Tags {
			if execution.Tags[key] != value {
				return false
			}
		}
	}

	return true
}

func (est *ExecutionStatusTracker) calculateMetrics(execution *ExecutionStatus) *ExecutionMetrics {
	metrics := &ExecutionMetrics{
		ResourceUtilization:  make(map[string]interface{}),
		PerformanceBreakdown: make(map[string]time.Duration),
	}

	if execution.ActualDuration != nil {
		metrics.TotalDuration = *execution.ActualDuration
	}

	var customerDurations []time.Duration
	var fastestDuration time.Duration
	var slowestDuration time.Duration
	var fastestCustomer, slowestCustomer string
	var successfulCustomers int

	for customerCode, customerExec := range execution.CustomerStatuses {
		if customerExec.Duration != nil {
			duration := *customerExec.Duration
			customerDurations = append(customerDurations, duration)

			if fastestDuration == 0 || duration < fastestDuration {
				fastestDuration = duration
				fastestCustomer = customerCode
			}

			if duration > slowestDuration {
				slowestDuration = duration
				slowestCustomer = customerCode
			}
		}

		if customerExec.Status == CustomerStatusCompleted {
			successfulCustomers++
		}
	}

	if len(customerDurations) > 0 {
		var totalDuration time.Duration
		for _, duration := range customerDurations {
			totalDuration += duration
		}
		metrics.AverageDurationPerCustomer = totalDuration / time.Duration(len(customerDurations))
	}

	metrics.FastestCustomer = fastestCustomer
	metrics.SlowestCustomer = slowestCustomer
	metrics.SuccessRate = float64(successfulCustomers) / float64(execution.TotalCustomers) * 100

	if metrics.TotalDuration > 0 {
		metrics.ThroughputPerMinute = float64(execution.TotalCustomers) / metrics.TotalDuration.Minutes()
	}

	return metrics
}

func (est *ExecutionStatusTracker) generateErrorSummary(execution *ExecutionStatus) *ErrorSummary {
	summary := &ErrorSummary{
		ErrorsByType:     make(map[string]int),
		ErrorsByCustomer: make(map[string]int),
		ErrorDetails:     []ErrorDetail{},
	}

	errorTypeCounts := make(map[string]int)

	for customerCode, customerExec := range execution.CustomerStatuses {
		if customerExec.Status == CustomerStatusFailed {
			summary.TotalErrors++
			summary.ErrorsByCustomer[customerCode]++

			// Analyze error message to determine type
			errorType := categorizeError(customerExec.ErrorMessage)
			summary.ErrorsByType[errorType]++
			errorTypeCounts[errorType]++

			// Add error detail
			detail := ErrorDetail{
				CustomerCode: customerCode,
				ErrorType:    errorType,
				ErrorMessage: customerExec.ErrorMessage,
				ErrorCode:    customerExec.ErrorCode,
				Timestamp:    time.Now(),
				Retryable:    isRetryableError(customerExec.ErrorMessage),
				Severity:     determineSeverity(customerExec.ErrorMessage),
				Context:      customerExec.Metadata,
			}

			summary.ErrorDetails = append(summary.ErrorDetails, detail)

			if detail.Retryable {
				summary.RetryableErrors++
			} else {
				summary.PermanentErrors++
			}

			if detail.Severity == "critical" {
				summary.CriticalErrors++
			}
		}
	}

	// Find most common error type
	maxCount := 0
	for errorType, count := range errorTypeCounts {
		if count > maxCount {
			maxCount = count
			summary.MostCommonError = errorType
		}
	}

	return summary
}

func (est *ExecutionStatusTracker) persistExecution(execution *ExecutionStatus) error {
	switch est.persistenceType {
	case "s3":
		return est.persistToS3(execution)
	case "dynamodb":
		return est.persistToDynamoDB(execution)
	case "memory":
		// Already in memory, no additional persistence needed
		return nil
	default:
		return fmt.Errorf("unsupported persistence type: %s", est.persistenceType)
	}
}

func (est *ExecutionStatusTracker) loadExecution(executionID string) (*ExecutionStatus, error) {
	switch est.persistenceType {
	case "s3":
		return est.loadFromS3(executionID)
	case "dynamodb":
		return est.loadFromDynamoDB(executionID)
	case "memory":
		return nil, fmt.Errorf("execution not found in memory: %s", executionID)
	default:
		return nil, fmt.Errorf("unsupported persistence type: %s", est.persistenceType)
	}
}

func (est *ExecutionStatusTracker) persistToS3(execution *ExecutionStatus) error {
	// Simulate S3 persistence
	// In real implementation, this would use AWS SDK to store in S3
	return nil
}

func (est *ExecutionStatusTracker) loadFromS3(executionID string) (*ExecutionStatus, error) {
	// Simulate S3 loading
	// In real implementation, this would use AWS SDK to load from S3
	return nil, fmt.Errorf("execution not found in S3: %s", executionID)
}

func (est *ExecutionStatusTracker) persistToDynamoDB(execution *ExecutionStatus) error {
	// Simulate DynamoDB persistence
	// In real implementation, this would use AWS SDK to store in DynamoDB
	return nil
}

func (est *ExecutionStatusTracker) loadFromDynamoDB(executionID string) (*ExecutionStatus, error) {
	// Simulate DynamoDB loading
	// In real implementation, this would use AWS SDK to load from DynamoDB
	return nil, fmt.Errorf("execution not found in DynamoDB: %s", executionID)
}

// Utility functions

func generateExecutionID() string {
	return fmt.Sprintf("exec-%d", time.Now().UnixNano())
}

func categorizeError(errorMessage string) string {
	errorMessage = strings.ToLower(errorMessage)

	if strings.Contains(errorMessage, "credential") || strings.Contains(errorMessage, "auth") {
		return "authentication"
	}
	if strings.Contains(errorMessage, "permission") || strings.Contains(errorMessage, "access") {
		return "authorization"
	}
	if strings.Contains(errorMessage, "network") || strings.Contains(errorMessage, "connection") {
		return "network"
	}
	if strings.Contains(errorMessage, "timeout") {
		return "timeout"
	}
	if strings.Contains(errorMessage, "rate") || strings.Contains(errorMessage, "throttl") {
		return "rate_limit"
	}
	if strings.Contains(errorMessage, "validation") || strings.Contains(errorMessage, "invalid") {
		return "validation"
	}

	return "unknown"
}

func isRetryableError(errorMessage string) bool {
	errorMessage = strings.ToLower(errorMessage)

	retryableKeywords := []string{
		"timeout", "throttl", "rate", "temporary", "unavailable",
		"connection", "network", "internal error",
	}

	for _, keyword := range retryableKeywords {
		if strings.Contains(errorMessage, keyword) {
			return true
		}
	}

	return false
}

func determineSeverity(errorMessage string) string {
	errorMessage = strings.ToLower(errorMessage)

	if strings.Contains(errorMessage, "critical") || strings.Contains(errorMessage, "fatal") {
		return "critical"
	}
	if strings.Contains(errorMessage, "security") || strings.Contains(errorMessage, "unauthorized") {
		return "high"
	}
	if strings.Contains(errorMessage, "warning") || strings.Contains(errorMessage, "deprecated") {
		return "medium"
	}

	return "low"
}
