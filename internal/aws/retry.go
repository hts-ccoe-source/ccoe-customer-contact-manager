// Package aws provides core AWS service interactions and credential management utilities.
package aws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/aws/smithy-go"
)

// RetryConfig holds configuration for retry behavior with exponential backoff
type RetryConfig struct {
	MaxAttempts    int           // Maximum number of retry attempts (default: 5)
	InitialDelay   time.Duration // Initial delay before first retry (default: 1s)
	MaxDelay       time.Duration // Maximum delay between retries (default: 30s)
	BackoffFactor  float64       // Multiplier for exponential backoff (default: 2.0)
	JitterFraction float64       // Fraction of delay to use for jitter (default: 0.1)
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    5,
		InitialDelay:   1 * time.Second,
		MaxDelay:       30 * time.Second,
		BackoffFactor:  2.0,
		JitterFraction: 0.1,
	}
}

// RetryWithBackoff executes an operation with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, operation func() error, config RetryConfig, logger *slog.Logger) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the operation
		err := operation()
		if err == nil {
			// Success
			if attempt > 1 {
				logger.Info("operation succeeded after retry",
					"attempt", attempt,
					"total_attempts", config.MaxAttempts)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			logger.Error("operation failed with non-retryable error",
				"attempt", attempt,
				"error", err)
			return err
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate sleep time with exponential backoff and jitter
		jitter := time.Duration(float64(delay) * config.JitterFraction * (rand.Float64()*2 - 1))
		sleepTime := delay + jitter

		// Cap at max delay
		if sleepTime > config.MaxDelay {
			sleepTime = config.MaxDelay
		}

		logger.Warn("operation failed, retrying with backoff",
			"attempt", attempt,
			"max_attempts", config.MaxAttempts,
			"delay", sleepTime,
			"error", err)

		// Sleep before next retry
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-time.After(sleepTime):
			// Continue to next attempt
		}

		// Increase delay for next iteration
		delay = time.Duration(float64(delay) * config.BackoffFactor)
	}

	logger.Error("operation failed after all retry attempts",
		"attempts", config.MaxAttempts,
		"error", lastErr)

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// isRetryableError determines if an AWS error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation - not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for AWS SDK v2 error types
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		errorCode := apiErr.ErrorCode()

		// Throttling errors - always retryable
		switch errorCode {
		case "Throttling",
			"ThrottlingException",
			"TooManyRequestsException",
			"RequestLimitExceeded",
			"ProvisionedThroughputExceededException":
			return true
		}

		// Service unavailable errors - retryable
		switch errorCode {
		case "ServiceUnavailable",
			"ServiceUnavailableException",
			"InternalError",
			"InternalServerError",
			"InternalFailure":
			return true
		}

		// Transient network errors - retryable
		switch errorCode {
		case "RequestTimeout",
			"RequestTimeoutException",
			"RequestExpired":
			return true
		}

		// Client errors (4xx) - generally not retryable
		// Server errors (5xx) - generally retryable
		// This is a fallback for unknown error codes
	}

	// Check for retryable interface from AWS SDK
	var retryable interface{ RetryableError() bool }
	if errors.As(err, &retryable) {
		return retryable.RetryableError()
	}

	// Check for temporary errors
	var temporary interface{ Temporary() bool }
	if errors.As(err, &temporary) {
		return temporary.Temporary()
	}

	// Default: don't retry unknown errors
	return false
}

// RetryableOperation wraps an AWS operation with retry logic using default configuration
func RetryableOperation(ctx context.Context, operationName string, operation func() error, logger *slog.Logger) error {
	config := DefaultRetryConfig()

	logger.Debug("executing operation with retry logic",
		"operation", operationName,
		"max_attempts", config.MaxAttempts)

	return RetryWithBackoff(ctx, operation, config, logger)
}

// IsThrottlingError checks if an error is a throttling error
func IsThrottlingError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		errorCode := apiErr.ErrorCode()
		switch errorCode {
		case "Throttling",
			"ThrottlingException",
			"TooManyRequestsException",
			"RequestLimitExceeded",
			"ProvisionedThroughputExceededException":
			return true
		}
	}

	return false
}

// IsServiceError checks if an error is a service unavailable error
func IsServiceError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		errorCode := apiErr.ErrorCode()
		switch errorCode {
		case "ServiceUnavailable",
			"ServiceUnavailableException",
			"InternalError",
			"InternalServerError",
			"InternalFailure":
			return true
		}
	}

	return false
}

// GetAWSErrorCode extracts the error code from an AWS error
func GetAWSErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}

	return ""
}

// GetAWSErrorMessage extracts the error message from an AWS error
func GetAWSErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorMessage()
	}

	return err.Error()
}

// WrapAWSError wraps an AWS error with additional context
func WrapAWSError(err error, operation string) error {
	if err == nil {
		return nil
	}

	errorCode := GetAWSErrorCode(err)
	errorMessage := GetAWSErrorMessage(err)

	if errorCode != "" {
		return fmt.Errorf("%s failed: [%s] %s", operation, errorCode, errorMessage)
	}

	return fmt.Errorf("%s failed: %w", operation, err)
}

// init initializes the random number generator for jitter
func init() {
	rand.Seed(time.Now().UnixNano())
}
