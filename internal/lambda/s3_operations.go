package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
func (s *S3UpdateManager) UpdateChangeObjectInS3(ctx context.Context, bucket, key string, changeMetadata *types.ChangeMetadata, logger *slog.Logger) error {
	logger.Debug("updating change object in S3",
		"bucket", bucket,
		"key", key)

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
		// Track S3 error in summary
		if summary := SummaryFromContext(ctx); summary != nil {
			summary.RecordS3Error()
		}
		return fmt.Errorf("failed to update S3 object s3://%s/%s: %w", bucket, key, err)
	}

	// Track S3 upload in summary
	if summary := SummaryFromContext(ctx); summary != nil {
		summary.RecordS3Upload()
	}

	return nil
}

// UpdateChangeObjectInS3WithETag updates a change object in S3 with ETag-based optimistic locking
func (s *S3UpdateManager) UpdateChangeObjectInS3WithETag(ctx context.Context, bucket, key string, changeMetadata *types.ChangeMetadata, expectedETag string, logger *slog.Logger) error {
	logger.Debug("updating change object in S3 with ETag lock",
		"bucket", bucket,
		"key", key,
		"etag", expectedETag)

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
	if expectedETag == "" {
		return fmt.Errorf("expectedETag cannot be empty for optimistic locking")
	}

	// Serialize the change metadata to JSON
	jsonData, err := json.MarshalIndent(changeMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal change metadata to JSON: %w", err)
	}

	// Debug: Check if meeting_metadata is in the JSON
	if changeMetadata.MeetingMetadata != nil && !bytes.Contains(jsonData, []byte("meeting_metadata")) {
		logger.Error("meeting_metadata was set but not included in JSON")
	}

	// Create the S3 PUT request with ETag-based conditional update
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
		IfMatch:     aws.String(expectedETag), // OPTIMISTIC LOCKING: Only update if ETag matches
		Metadata: map[string]string{
			"updated-by":   "backend-lambda",
			"updated-at":   time.Now().Format(time.RFC3339),
			"content-type": "change-metadata",
		},
	}

	// Perform the S3 PUT operation with conditional update
	_, err = s.s3Client.PutObject(ctx, putInput)
	if err != nil {
		// Track S3 error in summary
		if summary := SummaryFromContext(ctx); summary != nil {
			summary.RecordS3Error()
		}
		// Check if this is an ETag mismatch error (concurrent modification)
		if strings.Contains(err.Error(), "PreconditionFailed") || strings.Contains(err.Error(), "412") {
			return &ETagMismatchError{
				Bucket:       bucket,
				Key:          key,
				ExpectedETag: expectedETag,
				Message:      "Object was modified by another process (ETag mismatch)",
				Cause:        err,
			}
		}
		return fmt.Errorf("failed to update S3 object s3://%s/%s: %w", bucket, key, err)
	}

	// Track S3 upload in summary
	if summary := SummaryFromContext(ctx); summary != nil {
		summary.RecordS3Upload()
	}

	return nil
}

// ETagMismatchError represents an ETag mismatch during optimistic locking
type ETagMismatchError struct {
	Bucket       string
	Key          string
	ExpectedETag string
	Message      string
	Cause        error
}

// Error implements the error interface
func (e *ETagMismatchError) Error() string {
	return fmt.Sprintf("ETag mismatch for s3://%s/%s (expected: %s): %s", e.Bucket, e.Key, e.ExpectedETag, e.Message)
}

// Unwrap returns the underlying error
func (e *ETagMismatchError) Unwrap() error {
	return e.Cause
}

// IsETagMismatch checks if an error is an ETag mismatch error
func IsETagMismatch(err error) bool {
	_, ok := err.(*ETagMismatchError)
	return ok
}

// UpdateChangeObjectWithRetry updates a change object in S3 with exponential backoff retry
func (s *S3UpdateManager) UpdateChangeObjectWithRetry(ctx context.Context, bucket, key string, changeMetadata *types.ChangeMetadata, maxRetries int, logger *slog.Logger) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay: 1s, 2s, 4s, 8s, 16s
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			logger.Debug("retrying S3 update",
				"delay", delay,
				"attempt", attempt+1,
				"max_retries", maxRetries+1)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		err := s.UpdateChangeObjectInS3(ctx, bucket, key, changeMetadata, logger)
		if err == nil {
			if attempt > 0 {
				logger.Debug("S3 update succeeded on retry",
					"attempt", attempt+1)
			}
			return nil
		}

		lastErr = err
		logger.Debug("S3 update attempt failed",
			"attempt", attempt+1,
			"error", err)
	}

	return fmt.Errorf("failed to update S3 object after %d attempts: %w", maxRetries+1, lastErr)
}

// LoadChangeObjectFromS3 loads a change object from S3 (used for reading before updating)
func (s *S3UpdateManager) LoadChangeObjectFromS3(ctx context.Context, bucket, key string, logger *slog.Logger) (*types.ChangeMetadata, error) {
	logger.Debug("loading change object from S3",
		"bucket", bucket,
		"key", key)

	// Get the object from S3
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := s.s3Client.GetObject(ctx, getInput)
	if err != nil {
		// Track S3 error in summary
		if summary := SummaryFromContext(ctx); summary != nil {
			summary.RecordS3Error()
		}
		return nil, fmt.Errorf("failed to get S3 object s3://%s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	// Track S3 download in summary
	if summary := SummaryFromContext(ctx); summary != nil {
		summary.RecordS3Download()
	}

	// Read and parse the JSON content
	var changeMetadata types.ChangeMetadata
	decoder := json.NewDecoder(result.Body)
	if err := decoder.Decode(&changeMetadata); err != nil {
		return nil, fmt.Errorf("failed to decode change metadata JSON: %w", err)
	}

	// Extract survey metadata from S3 object metadata if present
	if result.Metadata != nil {
		if surveyID, ok := result.Metadata["survey_id"]; ok {
			changeMetadata.SurveyID = surveyID
		}
		if surveyURL, ok := result.Metadata["survey_url"]; ok {
			changeMetadata.SurveyURL = surveyURL
		}
		if surveyCreatedAt, ok := result.Metadata["survey_created_at"]; ok {
			changeMetadata.SurveyCreatedAt = surveyCreatedAt
		}
		// Survey metadata is tracked in object, no need to log
	}

	return &changeMetadata, nil
}

// LoadChangeObjectFromS3WithETag loads a change object from S3 and returns it with its ETag
func (s *S3UpdateManager) LoadChangeObjectFromS3WithETag(ctx context.Context, bucket, key string, logger *slog.Logger) (*types.ChangeMetadata, string, error) {
	logger.Debug("loading change object from S3 with ETag",
		"bucket", bucket,
		"key", key)

	// Get the object from S3
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := s.s3Client.GetObject(ctx, getInput)
	if err != nil {
		// Track S3 error in summary
		if summary := SummaryFromContext(ctx); summary != nil {
			summary.RecordS3Error()
		}
		return nil, "", fmt.Errorf("failed to get S3 object s3://%s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	// Track S3 download in summary
	if summary := SummaryFromContext(ctx); summary != nil {
		summary.RecordS3Download()
	}

	// Extract ETag from response
	etag := ""
	if result.ETag != nil {
		etag = *result.ETag
	} else {
		logger.Warn("no ETag returned for object")
	}

	// Read and parse the JSON content
	var changeMetadata types.ChangeMetadata
	decoder := json.NewDecoder(result.Body)
	if err := decoder.Decode(&changeMetadata); err != nil {
		return nil, "", fmt.Errorf("failed to decode change metadata JSON: %w", err)
	}

	// Extract survey metadata from S3 object metadata if present
	if result.Metadata != nil {
		if surveyID, ok := result.Metadata["survey_id"]; ok {
			changeMetadata.SurveyID = surveyID
		}
		if surveyURL, ok := result.Metadata["survey_url"]; ok {
			changeMetadata.SurveyURL = surveyURL
		}
		if surveyCreatedAt, ok := result.Metadata["survey_created_at"]; ok {
			changeMetadata.SurveyCreatedAt = surveyCreatedAt
		}
		// Survey metadata is tracked in object, no need to log
	}

	return &changeMetadata, etag, nil
}

// UpdateChangeObjectWithModification loads a change object, adds a modification entry, and saves it back
func (s *S3UpdateManager) UpdateChangeObjectWithModification(ctx context.Context, bucket, key string, modificationEntry types.ModificationEntry, logger *slog.Logger) error {
	logger.Debug("updating change object with modification entry",
		"modification_type", modificationEntry.ModificationType)

	// Load the existing change object
	changeMetadata, err := s.LoadChangeObjectFromS3(ctx, bucket, key, logger)
	if err != nil {
		return fmt.Errorf("failed to load change object for modification: %w", err)
	}

	// Add the modification entry with validation
	if err := changeMetadata.AddModificationEntry(modificationEntry); err != nil {
		return fmt.Errorf("failed to add modification entry: %w", err)
	}

	// Save the updated object back to S3 with retry
	err = s.UpdateChangeObjectWithRetry(ctx, bucket, key, changeMetadata, 5, logger)
	if err != nil {
		return fmt.Errorf("failed to save updated change object: %w", err)
	}

	return nil
}

// UpdateChangeObjectWithModificationOptimistic loads a change object, adds a modification entry, and saves it back with ETag-based optimistic locking
func (s *S3UpdateManager) UpdateChangeObjectWithModificationOptimistic(ctx context.Context, bucket, key string, modificationEntry types.ModificationEntry, maxRetries int, logger *slog.Logger) error {
	logger.Debug("updating change object with modification entry (optimistic locking)",
		"modification_type", modificationEntry.ModificationType)

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay: 100ms, 200ms, 400ms, 800ms, 1600ms
			delay := time.Duration(100<<uint(attempt-1)) * time.Millisecond
			logger.Debug("retrying after ETag mismatch",
				"delay", delay,
				"attempt", attempt+1,
				"max_retries", maxRetries+1)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		// Step 1: Load the existing change object WITH its ETag
		changeMetadata, etag, err := s.LoadChangeObjectFromS3WithETag(ctx, bucket, key, logger)
		if err != nil {
			lastErr = err
			logger.Debug("failed to load change object",
				"attempt", attempt+1,
				"error", err)
			continue
		}

		// Step 2: Add the modification entry with validation
		if err := changeMetadata.AddModificationEntry(modificationEntry); err != nil {
			return fmt.Errorf("failed to add modification entry: %w", err)
		}

		// Step 3: Save the updated object back to S3 with ETag-based conditional update
		err = s.UpdateChangeObjectInS3WithETag(ctx, bucket, key, changeMetadata, etag, logger)
		if err == nil {
			if attempt > 0 {
				logger.Debug("updated change object with optimistic locking on retry",
					"attempt", attempt+1)
			}
			return nil
		}

		// Check if this is an ETag mismatch (concurrent modification)
		if IsETagMismatch(err) {
			lastErr = err
			logger.Debug("ETag mismatch detected - object was modified concurrently",
				"attempt", attempt+1,
				"max_retries", maxRetries+1)
			continue // Retry
		}

		// Other error - don't retry
		return fmt.Errorf("failed to save updated change object: %w", err)
	}

	return fmt.Errorf("failed to update change object after %d attempts due to concurrent modifications: %w", maxRetries+1, lastErr)
}

// UpdateChangeObjectWithMeetingMetadata adds meeting metadata to a change object using ETag-based optimistic locking
func (s *S3UpdateManager) UpdateChangeObjectWithMeetingMetadata(ctx context.Context, bucket, key string, meetingMetadata *types.MeetingMetadata, logger *slog.Logger) error {
	logger.Debug("updating change object with meeting metadata (optimistic locking)",
		"meeting_id", meetingMetadata.MeetingID)

	// Validate context and parameters
	if err := s.ValidateS3UpdateContext(ctx, bucket, key); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Validate meeting metadata
	if err := meetingMetadata.ValidateMeetingMetadata(); err != nil {
		return fmt.Errorf("invalid meeting metadata: %w", err)
	}

	// Use optimistic locking to update the change
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := time.Duration(100<<uint(attempt-1)) * time.Millisecond
			logger.Debug("retrying change update after ETag mismatch",
				"delay", delay,
				"attempt", attempt+1,
				"max_retries", maxRetries+1)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		// Load the existing change object WITH its ETag
		changeMetadata, etag, err := s.LoadChangeObjectFromS3WithETag(ctx, bucket, key, logger)
		if err != nil {
			lastErr = err
			logger.Debug("failed to load change object",
				"attempt", attempt+1,
				"error", err)
			continue
		}

		// Create modification manager
		modManager := NewModificationManager(logger)

		// Create meeting scheduled entry
		modificationEntry, err := modManager.CreateMeetingScheduledEntry(meetingMetadata, logger)
		if err != nil {
			return fmt.Errorf("failed to create meeting scheduled entry: %w", err)
		}

		// Add modification entry
		if err := changeMetadata.AddModificationEntry(modificationEntry); err != nil {
			return fmt.Errorf("failed to add meeting scheduled entry: %w", err)
		}

		// CRITICAL: Set top-level meeting_metadata field so frontend can detect it
		changeMetadata.MeetingMetadata = meetingMetadata

		// Save the updated object back to S3 with ETag-based conditional update
		err = s.UpdateChangeObjectInS3WithETag(ctx, bucket, key, changeMetadata, etag, logger)
		if err == nil {
			return nil
		}

		// Check if this is an ETag mismatch (concurrent modification)
		if IsETagMismatch(err) {
			lastErr = err
			logger.Debug("ETag mismatch detected - object was modified concurrently",
				"attempt", attempt+1,
				"max_retries", maxRetries+1)
			continue // Retry
		}

		// Other error - don't retry
		return fmt.Errorf("failed to save updated change object: %w", err)
	}

	return fmt.Errorf("failed to update change after %d attempts due to concurrent modifications: %w", maxRetries+1, lastErr)
}

// UpdateChangeObjectWithMeetingCancellation adds meeting cancellation to a change object using ETag-based optimistic locking
func (s *S3UpdateManager) UpdateChangeObjectWithMeetingCancellation(ctx context.Context, bucket, key string, logger *slog.Logger) error {
	logger.Debug("updating change object with meeting cancellation (optimistic locking)")

	// Validate context and parameters
	if err := s.ValidateS3UpdateContext(ctx, bucket, key); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create modification manager
	modManager := NewModificationManager(logger)

	// Create meeting cancelled entry
	modificationEntry, err := modManager.CreateMeetingCancelledEntry(logger)
	if err != nil {
		return fmt.Errorf("failed to create meeting cancelled entry: %w", err)
	}

	// Update the change object with the modification entry using optimistic locking
	return s.UpdateChangeObjectWithModificationOptimistic(ctx, bucket, key, modificationEntry, 3, logger)
}

// UpdateArchiveWithProcessingMetadata updates the archive object with processing metadata after successful email delivery
func (s *S3UpdateManager) UpdateArchiveWithProcessingMetadata(ctx context.Context, bucket, key, customerCode string, logger *slog.Logger) error {
	logger.Debug("updating archive with processing metadata",
		"customer_code", customerCode,
		"key", key)

	// Validate context and parameters
	if err := s.ValidateS3UpdateContext(ctx, bucket, key); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if customerCode == "" {
		return fmt.Errorf("customer code cannot be empty")
	}

	// Create modification manager
	modManager := NewModificationManager(logger)

	// Create processed entry with customer code
	modificationEntry, err := modManager.CreateProcessedEntry(customerCode, logger)
	if err != nil {
		return fmt.Errorf("failed to create processed entry: %w", err)
	}

	// Update the change object with the modification entry using optimistic locking
	err = s.UpdateChangeObjectWithModificationOptimistic(ctx, bucket, key, modificationEntry, 3, logger)
	if err != nil {
		return fmt.Errorf("failed to update archive with processing metadata: %w", err)
	}

	return nil
}

// UpdateArchiveWithMeetingAndProcessing updates the archive with both meeting metadata and processing status
func (s *S3UpdateManager) UpdateArchiveWithMeetingAndProcessing(ctx context.Context, bucket, key, customerCode string, meetingMetadata *types.MeetingMetadata, logger *slog.Logger) error {
	logger.Debug("updating archive with meeting metadata and processing status",
		"customer_code", customerCode)

	// Validate context and parameters
	if err := s.ValidateS3UpdateContext(ctx, bucket, key); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if customerCode == "" {
		return fmt.Errorf("customer code cannot be empty")
	}

	if meetingMetadata == nil {
		return fmt.Errorf("meeting metadata cannot be nil")
	}

	// Validate meeting metadata
	if err := meetingMetadata.ValidateMeetingMetadata(); err != nil {
		return fmt.Errorf("invalid meeting metadata: %w", err)
	}

	// Use optimistic locking to update the archive
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := time.Duration(100<<uint(attempt-1)) * time.Millisecond
			logger.Debug("retrying archive update after ETag mismatch",
				"delay", delay,
				"attempt", attempt+1,
				"max_retries", maxRetries+1)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		// Load the existing change object WITH its ETag
		changeMetadata, etag, err := s.LoadChangeObjectFromS3WithETag(ctx, bucket, key, logger)
		if err != nil {
			lastErr = err
			logger.Debug("failed to load change object",
				"attempt", attempt+1,
				"error", err)
			continue
		}

		// Create modification manager
		modManager := NewModificationManager(logger)

		// Add meeting scheduled entry
		meetingEntry, err := modManager.CreateMeetingScheduledEntry(meetingMetadata, logger)
		if err != nil {
			return fmt.Errorf("failed to create meeting scheduled entry: %w", err)
		}

		if err := changeMetadata.AddModificationEntry(meetingEntry); err != nil {
			return fmt.Errorf("failed to add meeting scheduled entry: %w", err)
		}

		// Set nested meeting_metadata field (consistent with announcements)
		changeMetadata.MeetingMetadata = meetingMetadata

		// Add processed entry
		processedEntry, err := modManager.CreateProcessedEntry(customerCode, logger)
		if err != nil {
			return fmt.Errorf("failed to create processed entry: %w", err)
		}

		if err := changeMetadata.AddModificationEntry(processedEntry); err != nil {
			return fmt.Errorf("failed to add processed entry: %w", err)
		}

		// Debug: Verify MeetingMetadata is still set before calling save
		if changeMetadata.MeetingMetadata == nil {
			logger.Error("MeetingMetadata became nil before save call")
		}

		// Save the updated object back to S3 with ETag-based conditional update
		err = s.UpdateChangeObjectInS3WithETag(ctx, bucket, key, changeMetadata, etag, logger)
		if err == nil {
			return nil
		}

		// Check if this is an ETag mismatch (concurrent modification)
		if IsETagMismatch(err) {
			lastErr = err
			logger.Debug("ETag mismatch detected - object was modified concurrently",
				"attempt", attempt+1,
				"max_retries", maxRetries+1)
			continue // Retry
		}

		// Other error - don't retry
		return fmt.Errorf("failed to save updated change object: %w", err)
	}

	return fmt.Errorf("failed to update archive after %d attempts due to concurrent modifications: %w", maxRetries+1, lastErr)
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
func (s *S3UpdateManager) LogS3UpdateOperation(bucket, key string, changeMetadata *types.ChangeMetadata, success bool, err error, logger *slog.Logger) {
	if success {
		logger.Info("S3 update audit: success",
			"bucket", bucket,
			"key", key,
			"change_id", changeMetadata.ChangeID,
			"modification_count", len(changeMetadata.Modifications))
	} else {
		logger.Error("S3 update audit: failure",
			"bucket", bucket,
			"key", key,
			"change_id", changeMetadata.ChangeID,
			"error", err)
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
func (s *S3UpdateManager) UpdateChangeObjectWithAdvancedRetry(ctx context.Context, bucket, key string, changeMetadata *types.ChangeMetadata, logger *slog.Logger) error {
	const maxRetries = 5
	var lastError *S3UpdateError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff with jitter: base delay * 2^(attempt-1) + random jitter
			baseDelay := time.Second
			backoffDelay := time.Duration(1<<uint(attempt-1)) * baseDelay
			jitter := time.Duration(time.Now().UnixNano() % int64(baseDelay)) // Add up to 1 second jitter
			totalDelay := backoffDelay + jitter

			logger.Debug("retrying S3 update",
				"total_delay", totalDelay,
				"attempt", attempt+1,
				"max_retries", maxRetries+1,
				"backoff", backoffDelay,
				"jitter", jitter)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during S3 update retry: %w", ctx.Err())
			case <-time.After(totalDelay):
				// Continue with retry
			}
		}

		err := s.UpdateChangeObjectInS3(ctx, bucket, key, changeMetadata, logger)
		if err == nil {
			if attempt > 0 {
				logger.Debug("S3 update succeeded on retry",
					"attempt", attempt+1,
					"retries", attempt)
			}
			// Log successful operation for audit
			s.LogS3UpdateOperation(bucket, key, changeMetadata, true, nil, logger)
			return nil
		}

		// Classify the error
		s3Error := s.ClassifyS3Error(err, bucket, key)
		lastError = s3Error

		logger.Debug("S3 update attempt failed",
			"attempt", attempt+1,
			"message", s3Error.Message,
			"type", s3Error.Type,
			"retryable", s3Error.Retryable)

		// Don't retry if error is not retryable
		if !s3Error.Retryable {
			logger.Debug("error is not retryable, stopping retry attempts")
			break
		}
	}

	// Log failed operation for audit
	s.LogS3UpdateOperation(bucket, key, changeMetadata, false, lastError, logger)

	return fmt.Errorf("failed to update S3 object after %d attempts: %w", maxRetries+1, lastError)
}

// UpdateChangeObjectWithModificationAdvanced updates a change object with a modification entry using advanced retry logic
func (s *S3UpdateManager) UpdateChangeObjectWithModificationAdvanced(ctx context.Context, bucket, key string, modificationEntry types.ModificationEntry, logger *slog.Logger) error {
	logger.Debug("updating change object with modification entry (advanced retry)",
		"modification_type", modificationEntry.ModificationType)

	// Load the existing change object with retry
	var changeMetadata *types.ChangeMetadata
	var loadErr error

	// Retry loading the object as well
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			logger.Debug("retrying S3 load",
				"delay", delay,
				"attempt", attempt+1)
			time.Sleep(delay)
		}

		changeMetadata, loadErr = s.LoadChangeObjectFromS3(ctx, bucket, key, logger)
		if loadErr == nil {
			break
		}

		logger.Debug("S3 load attempt failed",
			"attempt", attempt+1,
			"error", loadErr)
	}

	if loadErr != nil {
		return fmt.Errorf("failed to load change object for modification after retries: %w", loadErr)
	}

	// Add the modification entry with validation
	if err := changeMetadata.AddModificationEntry(modificationEntry); err != nil {
		return fmt.Errorf("failed to add modification entry: %w", err)
	}

	// If this is a meeting_scheduled modification, also set nested meeting_metadata field
	if modificationEntry.ModificationType == types.ModificationTypeMeetingScheduled && modificationEntry.MeetingMetadata != nil {
		changeMetadata.MeetingMetadata = modificationEntry.MeetingMetadata
	}

	// Save the updated object back to S3 with advanced retry
	err := s.UpdateChangeObjectWithAdvancedRetry(ctx, bucket, key, changeMetadata, logger)
	if err != nil {
		return fmt.Errorf("failed to save updated change object with advanced retry: %w", err)
	}

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
func (s *S3UpdateManager) LogDetailedS3UpdateOperation(bucket, key string, changeMetadata *types.ChangeMetadata, success bool, err error, duration time.Duration, attempt int, logger *slog.Logger) {
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

	// Log as structured data
	if success {
		logger.Info("S3 update audit",
			"operation", logData["operation"],
			"bucket", logData["bucket"],
			"key", logData["key"],
			"change_id", logData["change_id"],
			"modification_count", logData["modification_count"],
			"duration_ms", logData["duration_ms"],
			"attempt", logData["attempt"])
	} else {
		logger.Error("S3 update audit failed",
			"operation", logData["operation"],
			"bucket", logData["bucket"],
			"key", logData["key"],
			"change_id", logData["change_id"],
			"error", logData["error"],
			"duration_ms", logData["duration_ms"],
			"attempt", logData["attempt"])
	}
}
