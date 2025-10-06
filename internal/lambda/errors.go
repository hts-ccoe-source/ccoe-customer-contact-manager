package lambda

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// ProcessingError represents different types of processing errors
type ProcessingError struct {
	Type       ErrorType
	Message    string
	Retryable  bool
	Underlying error
	MessageID  string
	S3Bucket   string
	S3Key      string
}

// ErrorType represents the category of error
type ErrorType string

const (
	ErrorTypeS3NotFound      ErrorType = "s3_not_found"
	ErrorTypeS3AccessDenied  ErrorType = "s3_access_denied"
	ErrorTypeS3NetworkError  ErrorType = "s3_network_error"
	ErrorTypeInvalidFormat   ErrorType = "invalid_format"
	ErrorTypeInvalidCustomer ErrorType = "invalid_customer"
	ErrorTypeConfigError     ErrorType = "config_error"
	ErrorTypeEmailError      ErrorType = "email_error"
	ErrorTypeUnknown         ErrorType = "unknown"
)

// Error implements the error interface
func (pe *ProcessingError) Error() string {
	return fmt.Sprintf("[%s] %s", pe.Type, pe.Message)
}

// IsRetryable returns whether this error should trigger a retry
func (pe *ProcessingError) IsRetryable() bool {
	return pe.Retryable
}

// NewProcessingError creates a new ProcessingError with context
func NewProcessingError(errorType ErrorType, message string, retryable bool, underlying error, messageID, s3Bucket, s3Key string) *ProcessingError {
	return &ProcessingError{
		Type:       errorType,
		Message:    message,
		Retryable:  retryable,
		Underlying: underlying,
		MessageID:  messageID,
		S3Bucket:   s3Bucket,
		S3Key:      s3Key,
	}
}

// ClassifyError analyzes an error and returns a ProcessingError with appropriate retry behavior
func ClassifyError(err error, messageID, s3Bucket, s3Key string) *ProcessingError {
	if err == nil {
		return nil
	}

	// Check for AWS SDK errors
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return classifyAWSError(apiErr, messageID, s3Bucket, s3Key, err)
	}

	// Check for S3-specific errors
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return NewProcessingError(
			ErrorTypeS3NotFound,
			fmt.Sprintf("S3 object not found: s3://%s/%s", s3Bucket, s3Key),
			false, // Not retryable - file doesn't exist
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	var noSuchBucket *types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return NewProcessingError(
			ErrorTypeS3NotFound,
			fmt.Sprintf("S3 bucket not found: %s", s3Bucket),
			false, // Not retryable - bucket doesn't exist
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	// Check error message for common patterns
	errMsg := strings.ToLower(err.Error())

	// S3 404 errors (file not found)
	if strings.Contains(errMsg, "nosuchkey") || strings.Contains(errMsg, "404") {
		return NewProcessingError(
			ErrorTypeS3NotFound,
			fmt.Sprintf("S3 object not found: s3://%s/%s", s3Bucket, s3Key),
			false, // Not retryable
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	// S3 403 errors (access denied)
	if strings.Contains(errMsg, "access denied") || strings.Contains(errMsg, "403") {
		return NewProcessingError(
			ErrorTypeS3AccessDenied,
			fmt.Sprintf("Access denied to S3 object: s3://%s/%s", s3Bucket, s3Key),
			false, // Not retryable - permission issue
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	// Network/timeout errors (retryable)
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "network") || strings.Contains(errMsg, "dns") {
		return NewProcessingError(
			ErrorTypeS3NetworkError,
			fmt.Sprintf("Network error accessing S3: %s", err.Error()),
			true, // Retryable
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	// JSON parsing errors (not retryable)
	if strings.Contains(errMsg, "json") || strings.Contains(errMsg, "unmarshal") ||
		strings.Contains(errMsg, "parse") {
		return NewProcessingError(
			ErrorTypeInvalidFormat,
			fmt.Sprintf("Invalid JSON format in S3 object: %s", err.Error()),
			false, // Not retryable - bad data
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	// Customer validation errors (not retryable)
	if strings.Contains(errMsg, "customer") && (strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "invalid")) {
		return NewProcessingError(
			ErrorTypeInvalidCustomer,
			fmt.Sprintf("Invalid customer code: %s", err.Error()),
			false, // Not retryable - bad customer code
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	// Configuration errors (retryable - might be temporary)
	if strings.Contains(errMsg, "config") || strings.Contains(errMsg, "credential") {
		return NewProcessingError(
			ErrorTypeConfigError,
			fmt.Sprintf("Configuration error: %s", err.Error()),
			true, // Retryable - config might be fixed
			err,
			messageID,
			s3Bucket,
			s3Key,
		)
	}

	// Default to unknown error (retryable to be safe)
	return NewProcessingError(
		ErrorTypeUnknown,
		fmt.Sprintf("Unknown error: %s", err.Error()),
		true, // Retryable by default
		err,
		messageID,
		s3Bucket,
		s3Key,
	)
}

// classifyAWSError handles AWS SDK specific errors
func classifyAWSError(apiErr smithy.APIError, messageID, s3Bucket, s3Key string, originalErr error) *ProcessingError {
	errorCode := apiErr.ErrorCode()

	switch errorCode {
	case "NoSuchKey":
		return NewProcessingError(
			ErrorTypeS3NotFound,
			fmt.Sprintf("S3 object not found: s3://%s/%s", s3Bucket, s3Key),
			false, // Not retryable
			originalErr,
			messageID,
			s3Bucket,
			s3Key,
		)
	case "NoSuchBucket":
		return NewProcessingError(
			ErrorTypeS3NotFound,
			fmt.Sprintf("S3 bucket not found: %s", s3Bucket),
			false, // Not retryable
			originalErr,
			messageID,
			s3Bucket,
			s3Key,
		)
	case "AccessDenied":
		return NewProcessingError(
			ErrorTypeS3AccessDenied,
			fmt.Sprintf("Access denied to S3 object: s3://%s/%s", s3Bucket, s3Key),
			false, // Not retryable
			originalErr,
			messageID,
			s3Bucket,
			s3Key,
		)
	case "InvalidBucketName":
		return NewProcessingError(
			ErrorTypeS3NotFound,
			fmt.Sprintf("Invalid S3 bucket name: %s", s3Bucket),
			false, // Not retryable
			originalErr,
			messageID,
			s3Bucket,
			s3Key,
		)
	case "RequestTimeout", "ServiceUnavailable", "SlowDown":
		return NewProcessingError(
			ErrorTypeS3NetworkError,
			fmt.Sprintf("S3 service error (retryable): %s", apiErr.ErrorMessage()),
			true, // Retryable
			originalErr,
			messageID,
			s3Bucket,
			s3Key,
		)
	default:
		// For unknown AWS errors, check if they're 4xx (client error, not retryable) or 5xx (server error, retryable)
		if len(errorCode) >= 3 {
			if errorCode[0] == '4' {
				return NewProcessingError(
					ErrorTypeUnknown,
					fmt.Sprintf("AWS client error: %s - %s", errorCode, apiErr.ErrorMessage()),
					false, // 4xx errors are not retryable
					originalErr,
					messageID,
					s3Bucket,
					s3Key,
				)
			} else if errorCode[0] == '5' {
				return NewProcessingError(
					ErrorTypeUnknown,
					fmt.Sprintf("AWS server error: %s - %s", errorCode, apiErr.ErrorMessage()),
					true, // 5xx errors are retryable
					originalErr,
					messageID,
					s3Bucket,
					s3Key,
				)
			}
		}

		return NewProcessingError(
			ErrorTypeUnknown,
			fmt.Sprintf("Unknown AWS error: %s - %s", errorCode, apiErr.ErrorMessage()),
			true, // Default to retryable
			originalErr,
			messageID,
			s3Bucket,
			s3Key,
		)
	}
}

// ShouldDeleteMessage determines if an SQS message should be deleted (not retried)
func ShouldDeleteMessage(err error) bool {
	var procErr *ProcessingError
	if errors.As(err, &procErr) {
		return !procErr.IsRetryable()
	}

	// If it's not a ProcessingError, be conservative and retry
	return false
}

// LogError logs an error with appropriate context and severity
func LogError(err error, messageID string) {
	var procErr *ProcessingError
	if errors.As(err, &procErr) {
		if procErr.IsRetryable() {
			log.Printf("⚠️  RETRYABLE ERROR [%s] Message %s: %s", procErr.Type, messageID, procErr.Message)
			if procErr.S3Bucket != "" && procErr.S3Key != "" {
				log.Printf("   S3 Location: s3://%s/%s", procErr.S3Bucket, procErr.S3Key)
			}
		} else {
			log.Printf("❌ NON-RETRYABLE ERROR [%s] Message %s: %s", procErr.Type, messageID, procErr.Message)
			log.Printf("   This message will be deleted from the queue to prevent infinite retries")
			if procErr.S3Bucket != "" && procErr.S3Key != "" {
				log.Printf("   S3 Location: s3://%s/%s", procErr.S3Bucket, procErr.S3Key)
			}
		}

		if procErr.Underlying != nil {
			log.Printf("   Underlying error: %v", procErr.Underlying)
		}
	} else {
		log.Printf("❓ UNKNOWN ERROR Message %s: %v", messageID, err)
	}
}
