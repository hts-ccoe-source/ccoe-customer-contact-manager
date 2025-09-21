package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// ErrorHandler provides comprehensive error handling and retry logic for multi-customer operations
type ErrorHandler struct {
	retryConfig     RetryConfiguration
	circuitBreakers map[string]*CircuitBreaker
	deadLetterQueue DeadLetterQueue
	errorClassifier *ErrorClassifier
	customerManager *CustomerCredentialManager
	statusTracker   *ExecutionStatusTracker
	mutex           sync.RWMutex
	errorMetrics    *ErrorMetrics
}

// RetryConfiguration defines retry behavior
type RetryConfiguration struct {
	MaxRetries         int                           `json:"maxRetries"`
	InitialDelay       time.Duration                 `json:"initialDelay"`
	MaxDelay           time.Duration                 `json:"maxDelay"`
	BackoffMultiplier  float64                       `json:"backoffMultiplier"`
	Jitter             bool                          `json:"jitter"`
	RetryableErrors    []string                      `json:"retryableErrors"`
	NonRetryableErrors []string                      `json:"nonRetryableErrors"`
	CustomerSpecific   map[string]RetryConfiguration `json:"customerSpecific,omitempty"`
}

// CircuitBreaker implements circuit breaker pattern for customer isolation
type CircuitBreaker struct {
	CustomerCode     string
	State            CircuitBreakerState
	FailureCount     int
	SuccessCount     int
	LastFailureTime  time.Time
	LastSuccessTime  time.Time
	FailureThreshold int
	RecoveryTimeout  time.Duration
	HalfOpenMaxCalls int
	HalfOpenCalls    int
	mutex            sync.RWMutex
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"
	CircuitBreakerOpen     CircuitBreakerState = "open"
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open"
)

// DeadLetterQueue handles messages that cannot be processed after retries
type DeadLetterQueue interface {
	SendToDeadLetter(message *FailedMessage) error
	ProcessDeadLetterQueue() ([]*FailedMessage, error)
	GetDeadLetterStats() (*DeadLetterStats, error)
}

// FailedMessage represents a message that failed processing
type FailedMessage struct {
	MessageID       string                 `json:"messageId"`
	CustomerCode    string                 `json:"customerCode"`
	OriginalMessage interface{}            `json:"originalMessage"`
	ErrorHistory    []ErrorRecord          `json:"errorHistory"`
	FirstFailedAt   time.Time              `json:"firstFailedAt"`
	LastFailedAt    time.Time              `json:"lastFailedAt"`
	RetryCount      int                    `json:"retryCount"`
	MaxRetries      int                    `json:"maxRetries"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Priority        string                 `json:"priority"`
	ExpiresAt       *time.Time             `json:"expiresAt,omitempty"`
}

// ErrorRecord represents a single error occurrence
type ErrorRecord struct {
	Timestamp    time.Time              `json:"timestamp"`
	ErrorType    string                 `json:"errorType"`
	ErrorMessage string                 `json:"errorMessage"`
	ErrorCode    string                 `json:"errorCode"`
	Retryable    bool                   `json:"retryable"`
	Severity     ErrorSeverity          `json:"severity"`
	Context      map[string]interface{} `json:"context,omitempty"`
	StackTrace   string                 `json:"stackTrace,omitempty"`
}

// ErrorSeverity represents the severity of an error
type ErrorSeverity string

const (
	SeverityCritical ErrorSeverity = "critical"
	SeverityHigh     ErrorSeverity = "high"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityLow      ErrorSeverity = "low"
)

// ErrorClassifier categorizes and analyzes errors
type ErrorClassifier struct {
	patterns map[string]ErrorPattern
}

// ErrorPattern defines how to classify an error
type ErrorPattern struct {
	Name       string        `json:"name"`
	Keywords   []string      `json:"keywords"`
	Retryable  bool          `json:"retryable"`
	Severity   ErrorSeverity `json:"severity"`
	Category   string        `json:"category"`
	Mitigation string        `json:"mitigation"`
}

// ErrorMetrics tracks error statistics
type ErrorMetrics struct {
	TotalErrors         int64                   `json:"totalErrors"`
	ErrorsByType        map[string]int64        `json:"errorsByType"`
	ErrorsByCustomer    map[string]int64        `json:"errorsByCustomer"`
	ErrorsBySeverity    map[ErrorSeverity]int64 `json:"errorsBySeverity"`
	RetrySuccessRate    float64                 `json:"retrySuccessRate"`
	CircuitBreakerTrips int64                   `json:"circuitBreakerTrips"`
	DeadLetterCount     int64                   `json:"deadLetterCount"`
	LastUpdated         time.Time               `json:"lastUpdated"`
	mutex               sync.RWMutex
}

// DeadLetterStats provides statistics about the dead letter queue
type DeadLetterStats struct {
	TotalMessages      int64            `json:"totalMessages"`
	MessagesByCustomer map[string]int64 `json:"messagesByCustomer"`
	MessagesByType     map[string]int64 `json:"messagesByType"`
	OldestMessage      *time.Time       `json:"oldestMessage,omitempty"`
	NewestMessage      *time.Time       `json:"newestMessage,omitempty"`
	ProcessingRate     float64          `json:"processingRate"`
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func(ctx context.Context) error

// NewErrorHandler creates a new error handler
func NewErrorHandler(customerManager *CustomerCredentialManager, statusTracker *ExecutionStatusTracker) *ErrorHandler {
	handler := &ErrorHandler{
		retryConfig: RetryConfiguration{
			MaxRetries:        3,
			InitialDelay:      1 * time.Second,
			MaxDelay:          30 * time.Second,
			BackoffMultiplier: 2.0,
			Jitter:            true,
			RetryableErrors: []string{
				"timeout", "throttling", "rate exceeded", "service unavailable",
				"internal error", "connection", "network", "temporary",
			},
			NonRetryableErrors: []string{
				"invalid credentials", "access denied", "not found",
				"invalid request", "malformed", "unauthorized",
			},
			CustomerSpecific: make(map[string]RetryConfiguration),
		},
		circuitBreakers: make(map[string]*CircuitBreaker),
		customerManager: customerManager,
		statusTracker:   statusTracker,
		errorMetrics: &ErrorMetrics{
			ErrorsByType:     make(map[string]int64),
			ErrorsByCustomer: make(map[string]int64),
			ErrorsBySeverity: make(map[ErrorSeverity]int64),
		},
	}

	// Initialize error classifier
	handler.errorClassifier = NewErrorClassifier()

	// Initialize dead letter queue (in-memory implementation)
	handler.deadLetterQueue = NewInMemoryDeadLetterQueue()

	return handler
}

// ExecuteWithRetry executes an operation with retry logic
func (eh *ErrorHandler) ExecuteWithRetry(ctx context.Context, customerCode string, operation RetryableOperation) error {
	// Check circuit breaker
	if !eh.isCircuitBreakerClosed(customerCode) {
		return fmt.Errorf("circuit breaker is open for customer %s", customerCode)
	}

	config := eh.getRetryConfig(customerCode)
	var lastError error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute operation
		err := operation(ctx)

		if err == nil {
			// Success - record and return
			eh.recordSuccess(customerCode)
			return nil
		}

		lastError = err

		// Classify error
		errorInfo := eh.errorClassifier.ClassifyError(err)

		// Record error
		eh.recordError(customerCode, errorInfo)

		// Check if error is retryable
		if !eh.isRetryable(err, config) {
			eh.recordFailure(customerCode)
			return fmt.Errorf("non-retryable error: %v", err)
		}

		// Check if we've reached max retries
		if attempt >= config.MaxRetries {
			eh.recordFailure(customerCode)
			break
		}

		// Calculate delay for next attempt
		delay := eh.calculateDelay(attempt, config)

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// All retries exhausted - send to dead letter queue if configured
	if eh.deadLetterQueue != nil {
		failedMessage := &FailedMessage{
			MessageID:       generateMessageID(),
			CustomerCode:    customerCode,
			OriginalMessage: operation,
			ErrorHistory:    []ErrorRecord{},
			FirstFailedAt:   time.Now(),
			LastFailedAt:    time.Now(),
			RetryCount:      config.MaxRetries,
			MaxRetries:      config.MaxRetries,
			Metadata:        make(map[string]interface{}),
			Priority:        "normal",
		}

		eh.deadLetterQueue.SendToDeadLetter(failedMessage)
	}

	return fmt.Errorf("operation failed after %d retries: %v", config.MaxRetries, lastError)
}

// ExecuteWithCustomerIsolation executes operations with customer isolation
func (eh *ErrorHandler) ExecuteWithCustomerIsolation(ctx context.Context, customerCodes []string, operation func(customerCode string) error) map[string]error {
	results := make(map[string]error)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, customerCode := range customerCodes {
		wg.Add(1)
		go func(custCode string) {
			defer wg.Done()

			err := eh.ExecuteWithRetry(ctx, custCode, func(ctx context.Context) error {
				return operation(custCode)
			})

			mutex.Lock()
			results[custCode] = err
			mutex.Unlock()
		}(customerCode)
	}

	wg.Wait()
	return results
}

// isCircuitBreakerClosed checks if the circuit breaker allows operations
func (eh *ErrorHandler) isCircuitBreakerClosed(customerCode string) bool {
	eh.mutex.RLock()
	breaker, exists := eh.circuitBreakers[customerCode]
	eh.mutex.RUnlock()

	if !exists {
		// Create new circuit breaker
		eh.mutex.Lock()
		eh.circuitBreakers[customerCode] = &CircuitBreaker{
			CustomerCode:     customerCode,
			State:            CircuitBreakerClosed,
			FailureThreshold: 5,
			RecoveryTimeout:  30 * time.Second,
			HalfOpenMaxCalls: 3,
		}
		eh.mutex.Unlock()
		return true
	}

	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()

	switch breaker.State {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if recovery timeout has passed
		if time.Since(breaker.LastFailureTime) > breaker.RecoveryTimeout {
			breaker.State = CircuitBreakerHalfOpen
			breaker.HalfOpenCalls = 0
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return breaker.HalfOpenCalls < breaker.HalfOpenMaxCalls
	default:
		return false
	}
}

// recordSuccess records a successful operation
func (eh *ErrorHandler) recordSuccess(customerCode string) {
	eh.mutex.Lock()
	breaker, exists := eh.circuitBreakers[customerCode]
	eh.mutex.Unlock()

	if !exists {
		return
	}

	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()

	breaker.SuccessCount++
	breaker.LastSuccessTime = time.Now()

	switch breaker.State {
	case CircuitBreakerHalfOpen:
		breaker.HalfOpenCalls++
		if breaker.HalfOpenCalls >= breaker.HalfOpenMaxCalls {
			breaker.State = CircuitBreakerClosed
			breaker.FailureCount = 0
		}
	case CircuitBreakerClosed:
		breaker.FailureCount = 0 // Reset failure count on success
	}
}

// recordFailure records a failed operation
func (eh *ErrorHandler) recordFailure(customerCode string) {
	eh.mutex.Lock()
	breaker, exists := eh.circuitBreakers[customerCode]
	eh.mutex.Unlock()

	if !exists {
		return
	}

	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()

	breaker.FailureCount++
	breaker.LastFailureTime = time.Now()

	switch breaker.State {
	case CircuitBreakerClosed:
		if breaker.FailureCount >= breaker.FailureThreshold {
			breaker.State = CircuitBreakerOpen
			eh.errorMetrics.mutex.Lock()
			eh.errorMetrics.CircuitBreakerTrips++
			eh.errorMetrics.mutex.Unlock()
		}
	case CircuitBreakerHalfOpen:
		breaker.State = CircuitBreakerOpen
		breaker.HalfOpenCalls = 0
	}
}

// recordError records error metrics
func (eh *ErrorHandler) recordError(customerCode string, errorInfo *ErrorInfo) {
	eh.errorMetrics.mutex.Lock()
	defer eh.errorMetrics.mutex.Unlock()

	eh.errorMetrics.TotalErrors++
	eh.errorMetrics.ErrorsByCustomer[customerCode]++
	eh.errorMetrics.ErrorsByType[errorInfo.Type]++
	eh.errorMetrics.ErrorsBySeverity[errorInfo.Severity]++
	eh.errorMetrics.LastUpdated = time.Now()
}

// getRetryConfig gets retry configuration for a customer
func (eh *ErrorHandler) getRetryConfig(customerCode string) RetryConfiguration {
	if customerConfig, exists := eh.retryConfig.CustomerSpecific[customerCode]; exists {
		return customerConfig
	}
	return eh.retryConfig
}

// isRetryable determines if an error is retryable
func (eh *ErrorHandler) isRetryable(err error, config RetryConfiguration) bool {
	errorMessage := strings.ToLower(err.Error())

	// Check non-retryable errors first
	for _, nonRetryable := range config.NonRetryableErrors {
		if strings.Contains(errorMessage, strings.ToLower(nonRetryable)) {
			return false
		}
	}

	// Check retryable errors
	for _, retryable := range config.RetryableErrors {
		if strings.Contains(errorMessage, strings.ToLower(retryable)) {
			return true
		}
	}

	// Default to non-retryable for unknown errors
	return false
}

// calculateDelay calculates the delay before the next retry attempt
func (eh *ErrorHandler) calculateDelay(attempt int, config RetryConfiguration) time.Duration {
	delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.BackoffMultiplier, float64(attempt)))

	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Add jitter if enabled
	if config.Jitter {
		jitterAmount := time.Duration(float64(delay) * 0.1) // 10% jitter
		delay += time.Duration(float64(jitterAmount) * (2*time.Now().UnixNano()%1000/1000.0 - 1))
	}

	return delay
}

// GetErrorMetrics returns current error metrics
func (eh *ErrorHandler) GetErrorMetrics() *ErrorMetrics {
	eh.errorMetrics.mutex.RLock()
	defer eh.errorMetrics.mutex.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &ErrorMetrics{
		TotalErrors:         eh.errorMetrics.TotalErrors,
		ErrorsByType:        make(map[string]int64),
		ErrorsByCustomer:    make(map[string]int64),
		ErrorsBySeverity:    make(map[ErrorSeverity]int64),
		RetrySuccessRate:    eh.errorMetrics.RetrySuccessRate,
		CircuitBreakerTrips: eh.errorMetrics.CircuitBreakerTrips,
		DeadLetterCount:     eh.errorMetrics.DeadLetterCount,
		LastUpdated:         eh.errorMetrics.LastUpdated,
	}

	for k, v := range eh.errorMetrics.ErrorsByType {
		metrics.ErrorsByType[k] = v
	}
	for k, v := range eh.errorMetrics.ErrorsByCustomer {
		metrics.ErrorsByCustomer[k] = v
	}
	for k, v := range eh.errorMetrics.ErrorsBySeverity {
		metrics.ErrorsBySeverity[k] = v
	}

	return metrics
}

// GetCircuitBreakerStatus returns the status of all circuit breakers
func (eh *ErrorHandler) GetCircuitBreakerStatus() map[string]*CircuitBreakerStatus {
	eh.mutex.RLock()
	defer eh.mutex.RUnlock()

	status := make(map[string]*CircuitBreakerStatus)

	for customerCode, breaker := range eh.circuitBreakers {
		breaker.mutex.RLock()
		status[customerCode] = &CircuitBreakerStatus{
			CustomerCode:    customerCode,
			State:           breaker.State,
			FailureCount:    breaker.FailureCount,
			SuccessCount:    breaker.SuccessCount,
			LastFailureTime: breaker.LastFailureTime,
			LastSuccessTime: breaker.LastSuccessTime,
		}
		breaker.mutex.RUnlock()
	}

	return status
}

// CircuitBreakerStatus represents the status of a circuit breaker
type CircuitBreakerStatus struct {
	CustomerCode    string              `json:"customerCode"`
	State           CircuitBreakerState `json:"state"`
	FailureCount    int                 `json:"failureCount"`
	SuccessCount    int                 `json:"successCount"`
	LastFailureTime time.Time           `json:"lastFailureTime"`
	LastSuccessTime time.Time           `json:"lastSuccessTime"`
}

// ErrorInfo represents classified error information
type ErrorInfo struct {
	Type       string        `json:"type"`
	Severity   ErrorSeverity `json:"severity"`
	Retryable  bool          `json:"retryable"`
	Category   string        `json:"category"`
	Mitigation string        `json:"mitigation"`
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	classifier := &ErrorClassifier{
		patterns: make(map[string]ErrorPattern),
	}

	// Define error patterns
	patterns := []ErrorPattern{
		{
			Name:       "authentication_error",
			Keywords:   []string{"credential", "authentication", "auth", "unauthorized"},
			Retryable:  false,
			Severity:   SeverityHigh,
			Category:   "security",
			Mitigation: "Check credentials and permissions",
		},
		{
			Name:       "authorization_error",
			Keywords:   []string{"permission", "access denied", "forbidden"},
			Retryable:  false,
			Severity:   SeverityHigh,
			Category:   "security",
			Mitigation: "Verify IAM roles and policies",
		},
		{
			Name:       "network_error",
			Keywords:   []string{"network", "connection", "timeout", "dns"},
			Retryable:  true,
			Severity:   SeverityMedium,
			Category:   "infrastructure",
			Mitigation: "Check network connectivity and DNS resolution",
		},
		{
			Name:       "rate_limit_error",
			Keywords:   []string{"rate", "throttl", "quota", "limit exceeded"},
			Retryable:  true,
			Severity:   SeverityMedium,
			Category:   "capacity",
			Mitigation: "Implement exponential backoff and reduce request rate",
		},
		{
			Name:       "service_unavailable",
			Keywords:   []string{"unavailable", "service down", "maintenance"},
			Retryable:  true,
			Severity:   SeverityHigh,
			Category:   "service",
			Mitigation: "Wait for service recovery or use alternative endpoint",
		},
		{
			Name:       "validation_error",
			Keywords:   []string{"validation", "invalid", "malformed", "bad request"},
			Retryable:  false,
			Severity:   SeverityMedium,
			Category:   "data",
			Mitigation: "Fix input data format and validation",
		},
		{
			Name:       "internal_error",
			Keywords:   []string{"internal error", "server error", "500"},
			Retryable:  true,
			Severity:   SeverityHigh,
			Category:   "service",
			Mitigation: "Retry with exponential backoff",
		},
	}

	for _, pattern := range patterns {
		classifier.patterns[pattern.Name] = pattern
	}

	return classifier
}

// ClassifyError classifies an error based on patterns
func (ec *ErrorClassifier) ClassifyError(err error) *ErrorInfo {
	errorMessage := strings.ToLower(err.Error())

	for _, pattern := range ec.patterns {
		for _, keyword := range pattern.Keywords {
			if strings.Contains(errorMessage, strings.ToLower(keyword)) {
				return &ErrorInfo{
					Type:       pattern.Name,
					Severity:   pattern.Severity,
					Retryable:  pattern.Retryable,
					Category:   pattern.Category,
					Mitigation: pattern.Mitigation,
				}
			}
		}
	}

	// Default classification for unknown errors
	return &ErrorInfo{
		Type:       "unknown_error",
		Severity:   SeverityMedium,
		Retryable:  false,
		Category:   "unknown",
		Mitigation: "Review error details and implement specific handling",
	}
}

// InMemoryDeadLetterQueue implements DeadLetterQueue interface
type InMemoryDeadLetterQueue struct {
	messages map[string]*FailedMessage
	mutex    sync.RWMutex
}

// NewInMemoryDeadLetterQueue creates a new in-memory dead letter queue
func NewInMemoryDeadLetterQueue() *InMemoryDeadLetterQueue {
	return &InMemoryDeadLetterQueue{
		messages: make(map[string]*FailedMessage),
	}
}

// SendToDeadLetter adds a message to the dead letter queue
func (dlq *InMemoryDeadLetterQueue) SendToDeadLetter(message *FailedMessage) error {
	dlq.mutex.Lock()
	defer dlq.mutex.Unlock()

	dlq.messages[message.MessageID] = message
	return nil
}

// ProcessDeadLetterQueue retrieves all messages from the dead letter queue
func (dlq *InMemoryDeadLetterQueue) ProcessDeadLetterQueue() ([]*FailedMessage, error) {
	dlq.mutex.RLock()
	defer dlq.mutex.RUnlock()

	var messages []*FailedMessage
	for _, message := range dlq.messages {
		messages = append(messages, message)
	}

	return messages, nil
}

// GetDeadLetterStats returns statistics about the dead letter queue
func (dlq *InMemoryDeadLetterQueue) GetDeadLetterStats() (*DeadLetterStats, error) {
	dlq.mutex.RLock()
	defer dlq.mutex.RUnlock()

	stats := &DeadLetterStats{
		TotalMessages:      int64(len(dlq.messages)),
		MessagesByCustomer: make(map[string]int64),
		MessagesByType:     make(map[string]int64),
	}

	var oldestTime, newestTime *time.Time

	for _, message := range dlq.messages {
		stats.MessagesByCustomer[message.CustomerCode]++

		if oldestTime == nil || message.FirstFailedAt.Before(*oldestTime) {
			oldestTime = &message.FirstFailedAt
		}

		if newestTime == nil || message.LastFailedAt.After(*newestTime) {
			newestTime = &message.LastFailedAt
		}
	}

	stats.OldestMessage = oldestTime
	stats.NewestMessage = newestTime

	return stats, nil
}

// Utility functions

func generateMessageID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}
