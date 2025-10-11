package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"ccoe-customer-contact-manager/internal/types"
)

// S3UpdateManager handles S3 object updates for change metadata
type S3UpdateManager struct {
	s3Client *s3.Client
	region   string
}

// NewS3UpdateManager creates a new S3UpdateManager
func NewS3UpdateManager(region string) (*S3UpdateManager, error) {
	// Create AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &S3UpdateManager{
		s3Client: s3.NewFromConfig(awsCfg),
		region:   region,
	}, nil
}

// NewS3UpdateManagerWithClient creates a new S3UpdateManager with existing S3 client
func NewS3UpdateManagerWithClient(s3Client *s3.Client, region string) *S3UpdateManager {
	return &S3UpdateManager{
		s3Client: s3Client,
		region:   region,
	}
}

// UpdateChangeObjectInS3 updates a change object in S3 with modification entries
func (s *S3UpdateManager) UpdateChangeObjectInS3(ctx context.Context, bucket, key string, changeMetadata *types.ChangeMetadata) error {
	log.Printf("📤 Updating change object in S3: s3://%s/%s", bucket, key)

	// Validate input parameters
	if bucket == "" {
		return fmt.Errorf("bucket name cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("object key cannot be empty")
	}
	if changeMetadata == nil {
		return fmt.Errorf("change metadata cannot be nil")
	}

	// Serialize the change metadata to JSON
	jsonData, err := json.MarshalIndent(changeMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal change metadata to JSON: %w", err)
	}

	log.Printf("📋 Serialized change metadata: %d bytes", len(jsonData))

	// Create the S3 PUT request
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
		Metadata: map[string]string{
			"updated-by":   "backend-lambda",
			"updated-at":   time.Now().Format(time.RFC3339),
			"content-type": "change-metadata",
		},
	}

	// Perform the S3 PUT operation
	_, err = s.s3Client.PutObject(ctx, putInput)
	if err != nil {
		return fmt.Errorf("failed to update S3 object s3://%s/%s: %w", bucket, key, err)
	}

	log.Printf("✅ Successfully updated change object in S3: s3://%s/%s", bucket, key)
	return nil
}

// UpdateChangeObjectWithRetry updates a change object in S3 with exponential backoff retry
func (s *S3UpdateManager) UpdateChangeObjectWithRetry(ctx context.Context, bucket, key string, changeMetadata *types.ChangeMetadata, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay: 1s, 2s, 4s, 8s, 16s
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("🔄 Retrying S3 update in %v (attempt %d/%d)", delay, attempt+1, maxRetries+1)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		err := s.UpdateChangeObjectInS3(ctx, bucket, key, changeMetadata)
		if err == nil {
			if attempt > 0 {
				log.Printf("✅ S3 update succeeded on attempt %d", attempt+1)
			}
			return nil
		}

		lastErr = err
		log.Printf("❌ S3 update attempt %d failed: %v", attempt+1, err)
	}

	return fmt.Errorf("failed to update S3 object after %d attempts: %w", maxRetries+1, lastErr)
}

// LoadChangeObjectFromS3 loads a change object from S3 (used for reading before updating)
func (s *S3UpdateManager) LoadChangeObjectFromS3(ctx context.Context, bucket, key string) (*types.ChangeMetadata, error) {
	log.Printf("📥 Loading change object from S3: s3://%s/%s", bucket, key)

	// Get the object from S3
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := s.s3Client.GetObject(ctx, getInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 object s3://%s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	// Read and parse the JSON content
	var changeMetadata types.ChangeMetadata
	decoder := json.NewDecoder(result.Body)
	if err := decoder.Decode(&changeMetadata); err != nil {
		return nil, fmt.Errorf("failed to decode change metadata JSON: %w", err)
	}

	log.Printf("✅ Successfully loaded change object: %s", changeMetadata.ChangeID)
	return &changeMetadata, nil
}

// UpdateChangeObjectWithModification loads a change object, adds a modification entry, and saves it back
func (s *S3UpdateManager) UpdateChangeObjectWithModification(ctx context.Context, bucket, key string, modificationEntry types.ModificationEntry) error {
	log.Printf("🔄 Updating change object with modification entry: type=%s", modificationEntry.ModificationType)

	// Load the existing change object
	changeMetadata, err := s.LoadChangeObjectFromS3(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("failed to load change object for modification: %w", err)
	}

	// Add the modification entry with validation
	if err := changeMetadata.AddModificationEntry(modificationEntry); err != nil {
		return fmt.Errorf("failed to add modification entry: %w", err)
	}

	log.Printf("📝 Added modification entry to change %s (total entries: %d)",
		changeMetadata.ChangeID, len(changeMetadata.Modifications))

	// Save the updated object back to S3 with retry
	err = s.UpdateChangeObjectWithRetry(ctx, bucket, key, changeMetadata, 5)
	if err != nil {
		return fmt.Errorf("failed to save updated change object: %w", err)
	}

	log.Printf("✅ Successfully updated change object with modification entry")
	return nil
}

// UpdateChangeObjectWithMeetingMetadata adds meeting metadata to a change object
func (s *S3UpdateManager) UpdateChangeObjectWithMeetingMetadata(ctx context.Context, bucket, key string, meetingMetadata *types.MeetingMetadata) error {
	log.Printf("📅 Updating change object with meeting metadata: meeting_id=%s", meetingMetadata.MeetingID)

	// Validate context and parameters
	if err := s.ValidateS3UpdateContext(ctx, bucket, key); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create modification manager
	modManager := NewModificationManager()

	// Create meeting scheduled entry
	modificationEntry, err := modManager.CreateMeetingScheduledEntry(meetingMetadata)
	if err != nil {
		return fmt.Errorf("failed to create meeting scheduled entry: %w", err)
	}

	// Update the change object with the modification entry using advanced retry
	return s.UpdateChangeObjectWithModificationAdvanced(ctx, bucket, key, modificationEntry)
}

// UpdateChangeObjectWithMeetingCancellation adds meeting cancellation to a change object
func (s *S3UpdateManager) UpdateChangeObjectWithMeetingCancellation(ctx context.Context, bucket, key string) error {
	log.Printf("❌ Updating change object with meeting cancellation")

	// Validate context and parameters
	if err := s.ValidateS3UpdateContext(ctx, bucket, key); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create modification manager
	modManager := NewModificationManager()

	// Create meeting cancelled entry
	modificationEntry, err := modManager.CreateMeetingCancelledEntry()
	if err != nil {
		return fmt.Errorf("failed to create meeting cancelled entry: %w", err)
	}

	// Update the change object with the modification entry using advanced retry
	return s.UpdateChangeObjectWithModificationAdvanced(ctx, bucket, key, modificationEntry)
}

// ValidateS3UpdateParameters validates parameters for S3 update operations
func (s *S3UpdateManager) ValidateS3UpdateParameters(bucket, key string, changeMetadata *types.ChangeMetadata) error {
	if bucket == "" {
		return fmt.Errorf("S3 bucket name cannot be empty")
	}

	if key == "" {
		return fmt.Errorf("S3 object key cannot be empty")
	}

	if changeMetadata == nil {
		return fmt.Errorf("change metadata cannot be nil")
	}

	if changeMetadata.ChangeID == "" {
		return fmt.Errorf("change metadata must have a change ID")
	}

	return nil
}

// LogS3UpdateOperation logs details about an S3 update operation for audit purposes
func (s *S3UpdateManager) LogS3UpdateOperation(bucket, key string, changeMetadata *types.ChangeMetadata, success bool, err error) {
	if success {
		log.Printf("📊 S3 Update Audit: SUCCESS - Bucket=%s, Key=%s, ChangeID=%s, ModificationCount=%d",
			bucket, key, changeMetadata.ChangeID, len(changeMetadata.Modifications))
	} else {
		log.Printf("📊 S3 Update Audit: FAILURE - Bucket=%s, Key=%s, ChangeID=%s, Error=%v",
			bucket, key, changeMetadata.ChangeID, err)
	}
}

// S3UpdateError represents different types of S3 update errors
type S3UpdateError struct {
	Type      S3ErrorType
	Message   string
	Cause     error
	Bucket    string
	Key       string
	Retryable bool
}

// S3ErrorType represents the type of S3 error
type S3ErrorType string

const (
	S3ErrorTypePermission S3ErrorType = "permission"
	S3ErrorTypeNotFound   S3ErrorType = "not_found"
	S3ErrorTypeThrottling S3ErrorType = "throttling"
	S3ErrorTypeNetwork    S3ErrorType = "network"
	S3ErrorTypeValidation S3ErrorType = "validation"
	S3ErrorTypeUnknown    S3ErrorType = "unknown"
)

// Error implements the error interface
func (e *S3UpdateError) Error() string {
	return fmt.Sprintf("S3 update error (%s): %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *S3UpdateError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error should be retried
func (e *S3UpdateError) IsRetryable() bool {
	return e.Retryable
}

// ClassifyS3Error classifies an S3 error and determines if it should be retried
func (s *S3UpdateManager) ClassifyS3Error(err error, bucket, key string) *S3UpdateError {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for specific AWS error types
	switch {
	case strings.Contains(errStr, "NoSuchBucket") || strings.Contains(errStr, "NoSuchKey"):
		return &S3UpdateError{
			Type:      S3ErrorTypeNotFound,
			Message:   fmt.Sprintf("S3 object not found: s3://%s/%s", bucket, key),
			Cause:     err,
			Bucket:    bucket,
			Key:       key,
			Retryable: false, // Don't retry if object doesn't exist
		}

	case strings.Contains(errStr, "AccessDenied") || strings.Contains(errStr, "Forbidden"):
		return &S3UpdateError{
			Type:      S3ErrorTypePermission,
			Message:   fmt.Sprintf("Access denied to S3 object: s3://%s/%s", bucket, key),
			Cause:     err,
			Bucket:    bucket,
			Key:       key,
			Retryable: false, // Don't retry permission errors
		}

	case strings.Contains(errStr, "SlowDown") || strings.Contains(errStr, "RequestLimitExceeded") || strings.Contains(errStr, "ServiceUnavailable"):
		return &S3UpdateError{
			Type:      S3ErrorTypeThrottling,
			Message:   fmt.Sprintf("S3 throttling error for: s3://%s/%s", bucket, key),
			Cause:     err,
			Bucket:    bucket,
			Key:       key,
			Retryable: true, // Retry throttling errors
		}

	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") || strings.Contains(errStr, "network"):
		return &S3UpdateError{
			Type:      S3ErrorTypeNetwork,
			Message:   fmt.Sprintf("Network error accessing S3: s3://%s/%s", bucket, key),
			Cause:     err,
			Bucket:    bucket,
			Key:       key,
			Retryable: true, // Retry network errors
		}

	default:
		return &S3UpdateError{
			Type:      S3ErrorTypeUnknown,
			Message:   fmt.Sprintf("Unknown S3 error for: s3://%s/%s - %s", bucket, key, errStr),
			Cause:     err,
			Bucket:    bucket,
			Key:       key,
			Retryable: true, // Default to retryable for unknown errors
		}
	}
}

// UpdateChangeObjectWithAdvancedRetry updates a change object with advanced error handling and retry logic
func (s *S3UpdateManager) UpdateChangeObjectWithAdvancedRetry(ctx context.Context, bucket, key string, changeMetadata *types.ChangeMetadata) error {
	const maxRetries = 5
	var lastError *S3UpdateError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff with jitter: base delay * 2^(attempt-1) + random jitter
			baseDelay := time.Second
			backoffDelay := time.Duration(1<<uint(attempt-1)) * baseDelay
			jitter := time.Duration(time.Now().UnixNano() % int64(baseDelay)) // Add up to 1 second jitter
			totalDelay := backoffDelay + jitter

			log.Printf("🔄 Retrying S3 update in %v (attempt %d/%d, backoff: %v, jitter: %v)",
				totalDelay, attempt+1, maxRetries+1, backoffDelay, jitter)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during S3 update retry: %w", ctx.Err())
			case <-time.After(totalDelay):
				// Continue with retry
			}
		}

		err := s.UpdateChangeObjectInS3(ctx, bucket, key, changeMetadata)
		if err == nil {
			if attempt > 0 {
				log.Printf("✅ S3 update succeeded on attempt %d after %d retries", attempt+1, attempt)
			}
			// Log successful operation for audit
			s.LogS3UpdateOperation(bucket, key, changeMetadata, true, nil)
			return nil
		}

		// Classify the error
		s3Error := s.ClassifyS3Error(err, bucket, key)
		lastError = s3Error

		log.Printf("❌ S3 update attempt %d failed: %s (type: %s, retryable: %v)",
			attempt+1, s3Error.Message, s3Error.Type, s3Error.Retryable)

		// Don't retry if error is not retryable
		if !s3Error.Retryable {
			log.Printf("🚫 Error is not retryable, stopping retry attempts")
			break
		}
	}

	// Log failed operation for audit
	s.LogS3UpdateOperation(bucket, key, changeMetadata, false, lastError)

	return fmt.Errorf("failed to update S3 object after %d attempts: %w", maxRetries+1, lastError)
}

// UpdateChangeObjectWithModificationAdvanced updates a change object with a modification entry using advanced retry logic
func (s *S3UpdateManager) UpdateChangeObjectWithModificationAdvanced(ctx context.Context, bucket, key string, modificationEntry types.ModificationEntry) error {
	log.Printf("🔄 Updating change object with modification entry (advanced retry): type=%s", modificationEntry.ModificationType)

	// Load the existing change object with retry
	var changeMetadata *types.ChangeMetadata
	var loadErr error

	// Retry loading the object as well
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("🔄 Retrying S3 load in %v (attempt %d/3)", delay, attempt+1)
			time.Sleep(delay)
		}

		changeMetadata, loadErr = s.LoadChangeObjectFromS3(ctx, bucket, key)
		if loadErr == nil {
			break
		}

		log.Printf("❌ S3 load attempt %d failed: %v", attempt+1, loadErr)
	}

	if loadErr != nil {
		return fmt.Errorf("failed to load change object for modification after retries: %w", loadErr)
	}

	// Add the modification entry with validation
	if err := changeMetadata.AddModificationEntry(modificationEntry); err != nil {
		return fmt.Errorf("failed to add modification entry: %w", err)
	}

	log.Printf("📝 Added modification entry to change %s (total entries: %d)",
		changeMetadata.ChangeID, len(changeMetadata.Modifications))

	// Save the updated object back to S3 with advanced retry
	err := s.UpdateChangeObjectWithAdvancedRetry(ctx, bucket, key, changeMetadata)
	if err != nil {
		return fmt.Errorf("failed to save updated change object with advanced retry: %w", err)
	}

	log.Printf("✅ Successfully updated change object with modification entry using advanced retry")
	return nil
}

// ValidateS3UpdateContext validates the context and parameters before S3 operations
func (s *S3UpdateManager) ValidateS3UpdateContext(ctx context.Context, bucket, key string) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}

	if ctx.Err() != nil {
		return fmt.Errorf("context is already cancelled or expired: %w", ctx.Err())
	}

	if bucket == "" {
		return fmt.Errorf("S3 bucket name cannot be empty")
	}

	if key == "" {
		return fmt.Errorf("S3 object key cannot be empty")
	}

	return nil
}

// LogDetailedS3UpdateOperation logs comprehensive details about S3 update operations
func (s *S3UpdateManager) LogDetailedS3UpdateOperation(bucket, key string, changeMetadata *types.ChangeMetadata, success bool, err error, duration time.Duration, attempt int) {
	logData := map[string]interface{}{
		"operation":   "s3_update",
		"bucket":      bucket,
		"key":         key,
		"success":     success,
		"duration_ms": duration.Milliseconds(),
		"attempt":     attempt,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	if changeMetadata != nil {
		logData["change_id"] = changeMetadata.ChangeID
		logData["modification_count"] = len(changeMetadata.Modifications)

		// Log the latest modification type
		if len(changeMetadata.Modifications) > 0 {
			latest := changeMetadata.Modifications[len(changeMetadata.Modifications)-1]
			logData["latest_modification_type"] = latest.ModificationType
			logData["latest_modification_user"] = latest.UserID
		}
	}

	if err != nil {
		logData["error"] = err.Error()
		if s3Error, ok := err.(*S3UpdateError); ok {
			logData["error_type"] = string(s3Error.Type)
			logData["retryable"] = s3Error.Retryable
		}
	}

	// Log as structured JSON for better parsing by log aggregation systems
	logJSON, _ := json.Marshal(logData)
	log.Printf("📊 S3_UPDATE_AUDIT: %s", string(logJSON))
}
