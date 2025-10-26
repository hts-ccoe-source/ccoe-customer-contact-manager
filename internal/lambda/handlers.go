package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsinternal "ccoe-customer-contact-manager/internal/aws"
	"ccoe-customer-contact-manager/internal/config"
	"ccoe-customer-contact-manager/internal/datetime"
	"ccoe-customer-contact-manager/internal/ses"
	"ccoe-customer-contact-manager/internal/ses/templates"
	"ccoe-customer-contact-manager/internal/typeform"
	"ccoe-customer-contact-manager/internal/types"
)

const (
	// defaultSenderEmail is the default sender email address for all notifications
	// Note: ccoe@hearst.com should ONLY be used for meeting invites
	defaultSenderEmail = "ccoe@ccoe.hearst.com"
)

// currentCustomerInfo holds the customer info for the current request
// This is set at the beginning of processing and used for recipient restrictions
var currentCustomerInfo *types.CustomerAccountInfo

// getBackendRoleARN returns the backend Lambda's execution role ARN from environment variables
func getBackendRoleARN() string {
	// Try multiple environment variable names for flexibility
	roleARN := os.Getenv("BACKEND_ROLE_ARN")
	if roleARN == "" {
		roleARN = os.Getenv("AWS_LAMBDA_ROLE_ARN")
	}
	if roleARN == "" {
		roleARN = os.Getenv("LAMBDA_EXECUTION_ROLE_ARN")
	}

	if roleARN == "" {
		log.Printf("⚠️  Backend role ARN not configured - event loop prevention may not work correctly")
	} else {
		log.Printf("🔧 Using backend role ARN: %s", roleARN)
	}

	return roleARN
}

// getFrontendRoleARN returns the frontend Lambda's execution role ARN from environment variables
func getFrontendRoleARN() string {
	roleARN := os.Getenv("FRONTEND_ROLE_ARN")

	if roleARN == "" {
		log.Printf("⚠️  Frontend role ARN not configured - may not be able to identify frontend events")
	} else {
		log.Printf("🔧 Using frontend role ARN: %s", roleARN)
	}

	return roleARN
}

// formatTextForHTML converts plain text to HTML-safe format with line breaks
// Double newlines are converted to paragraph breaks, single newlines to <br> tags
func formatTextForHTML(text string) string {
	// Replace double newlines with paragraph breaks
	text = strings.ReplaceAll(text, "\r\n\r\n", "</p><p>")
	text = strings.ReplaceAll(text, "\n\n", "</p><p>")

	// Replace single newlines with <br> tags
	text = strings.ReplaceAll(text, "\r\n", "<br>")
	text = strings.ReplaceAll(text, "\n", "<br>")

	// Wrap in paragraph tags if not empty
	if text != "" && !strings.HasPrefix(text, "<p>") {
		text = "<p>" + text + "</p>"
	}

	return text
}

// formatDateTimeWithTimezone is a centralized function to format datetime with timezone conversion
// This eliminates duplicate timezone formatting logic across different email templates
func formatDateTimeWithTimezone(t time.Time, timezone string) string {
	if t.IsZero() {
		return "TBD"
	}

	// Use provided timezone or default to Eastern Time
	targetTimezone := timezone
	if targetTimezone == "" {
		targetTimezone = "America/New_York"
	}

	// Load the target timezone
	loc, err := time.LoadLocation(targetTimezone)
	if err != nil {
		// Fallback to UTC if timezone loading fails
		log.Printf("Warning: Failed to load timezone %s, using UTC: %v", targetTimezone, err)
		loc = time.UTC
	}

	// Convert the time to the target timezone and format
	localTime := t.In(loc)
	return localTime.Format("January 2, 2006 at 3:04 PM MST")
}

// Handler handles SQS events from Lambda
func Handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	// Load all credentials from Parameter Store (Azure + Typeform)
	if err := ses.LoadAllCredentialsFromSSM(ctx); err != nil {
		log.Printf("⚠️  Warning: Failed to load credentials from Parameter Store: %v", err)
		// Don't fail the entire handler - some operations may not need these credentials
	}

	// Load configuration
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	// Process each SQS message with proper error handling
	var retryableErrors []error
	var nonRetryableErrors []error
	successCount := 0

	for _, record := range sqsEvent.Records {
		err := ProcessSQSRecord(ctx, record, cfg)
		if err != nil {
			// Log the error with proper classification
			LogError(err, record.MessageId)

			// Determine if this error should cause a retry
			if ShouldDeleteMessage(err) {
				nonRetryableErrors = append(nonRetryableErrors, fmt.Errorf("message %s (non-retryable): %w", record.MessageId, err))
			} else {
				retryableErrors = append(retryableErrors, fmt.Errorf("message %s (retryable): %w", record.MessageId, err))
			}
		} else {
			successCount++
		}
	}

	// Log summary only
	if len(retryableErrors) > 0 || len(nonRetryableErrors) > 0 {
		log.Printf("Processed %d messages: %d successful, %d retryable errors, %d non-retryable errors",
			len(sqsEvent.Records), successCount, len(retryableErrors), len(nonRetryableErrors))
	}

	// Only return error for retryable failures
	if len(retryableErrors) > 0 {
		return fmt.Errorf("failed to process %d retryable messages: %v", len(retryableErrors), retryableErrors[0])
	}

	return nil
}

// ProcessSQSRecord processes a single SQS record from Lambda
func ProcessSQSRecord(ctx context.Context, record events.SQSMessage, cfg *types.Config) error {
	// Check if this is an S3 test event and skip it
	if IsS3TestEvent(record.Body) {
		return nil
	}

	// Extract userIdentity from SQS message for event loop prevention
	roleConfig := LoadRoleConfigFromEnvironment()
	userIdentityExtractor := NewUserIdentityExtractorWithConfig(roleConfig)

	userIdentity, err := userIdentityExtractor.SafeExtractUserIdentity(record.Body, record.MessageId)
	if err != nil {
		// Continue processing - missing userIdentity shouldn't block legitimate events
		log.Printf("Warning: Failed to extract userIdentity from message %s: %v", record.MessageId, err)
	} else {
		// Check if this event should be discarded (backend-generated event)
		shouldDiscard, reason := userIdentityExtractor.ShouldDiscardEvent(userIdentity)
		if shouldDiscard {
			log.Printf("Discarding event from backend: %s", reason)
			return nil // Successfully processed (by discarding)
		}
	}

	// Parse the message body
	var messageBody interface{}
	if err := json.Unmarshal([]byte(record.Body), &messageBody); err != nil {
		// JSON parsing failure is non-retryable
		return NewProcessingError(
			ErrorTypeInvalidFormat,
			fmt.Sprintf("Failed to parse message body as JSON: %v", err),
			false, // Not retryable
			err,
			record.MessageId,
			"", // No S3 info yet
			"",
		)
	}

	// Check if it's an S3 event notification
	var s3Event types.S3EventNotification
	if err := json.Unmarshal([]byte(record.Body), &s3Event); err == nil {
		if len(s3Event.Records) > 0 {
			// Process as S3 event
			return ProcessS3Event(ctx, s3Event, cfg)
		}
	}

	// Try to parse as legacy SQS message
	var sqsMsg types.SQSMessage
	if err := json.Unmarshal([]byte(record.Body), &sqsMsg); err == nil && sqsMsg.CustomerCode != "" {
		// Set the message ID from the SQS record
		sqsMsg.MessageID = record.MessageId
		// Process as legacy SQS message
		return ProcessSQSMessage(ctx, sqsMsg, cfg)
	}
	// Unrecognized message format is non-retryable
	return NewProcessingError(
		ErrorTypeInvalidFormat,
		"Unrecognized message format - not S3 event or legacy SQS message",
		false, // Not retryable
		fmt.Errorf("unrecognized message format"),
		record.MessageId,
		"",
		"",
	)
}

// IsS3TestEvent checks if the message is an S3 test event
func IsS3TestEvent(messageBody string) bool {
	// Check for S3 test event patterns - handle various JSON formatting
	isTestEvent := strings.Contains(messageBody, `"Event": "s3:TestEvent"`) ||
		strings.Contains(messageBody, `"Event":"s3:TestEvent"`) ||
		strings.Contains(messageBody, `"Event" : "s3:TestEvent"`) ||
		(strings.Contains(messageBody, `"Service": "Amazon S3"`) && strings.Contains(messageBody, `s3:TestEvent`)) ||
		(strings.Contains(messageBody, `"Service":"Amazon S3"`) && strings.Contains(messageBody, `s3:TestEvent`)) ||
		(strings.Contains(messageBody, `"Service" : "Amazon S3"`) && strings.Contains(messageBody, `s3:TestEvent`))

	if isTestEvent {
		log.Printf("Detected S3 test event, skipping processing")
	} else {
		// Debug logging to help troubleshoot if test events are still getting through
		if strings.Contains(messageBody, "s3:TestEvent") || strings.Contains(messageBody, "Amazon S3") {
			log.Printf("Message contains S3 test indicators but didn't match patterns: %s", messageBody)
		}
	}

	return isTestEvent
}

// ProcessS3Event processes an S3 event notification in Lambda context with Transient Trigger Pattern
func ProcessS3Event(ctx context.Context, s3Event types.S3EventNotification, cfg *types.Config) error {
	// Check userIdentity for event loop prevention before processing any records
	roleConfig := LoadRoleConfigFromEnvironment()
	userIdentityExtractor := NewUserIdentityExtractorWithConfig(roleConfig)

	userIdentity, err := userIdentityExtractor.ExtractUserIdentityFromS3Event(s3Event)
	if err == nil {
		// Check if this event should be discarded (backend-generated event)
		shouldDiscard, reason := userIdentityExtractor.ShouldDiscardEvent(userIdentity)
		if shouldDiscard {
			log.Printf("Discarding event from backend: %s", reason)
			return nil // Successfully processed (by discarding)
		}
	}

	for _, record := range s3Event.Records {
		if record.EventSource != "aws:s3" {
			continue
		}

		bucketName := record.S3.Bucket.Name
		objectKey := record.S3.Object.Key

		// Extract customer code and change ID from S3 key
		customerCode, changeID, err := ExtractCustomerCodeAndChangeIDFromS3Key(objectKey)
		if err != nil {
			// Customer code extraction failure is non-retryable (bad S3 key format)
			return NewProcessingError(
				ErrorTypeInvalidFormat,
				fmt.Sprintf("Failed to extract customer code from S3 key %s: %v", objectKey, err),
				false, // Not retryable
				err,
				"", // No message ID for S3 events
				bucketName,
				objectKey,
			)
		}

		// Validate customer code
		if err := ValidateCustomerCode(customerCode, cfg); err != nil {
			// Invalid customer code is non-retryable
			return NewProcessingError(
				ErrorTypeInvalidCustomer,
				fmt.Sprintf("Invalid customer code %s: %v", customerCode, err),
				false, // Not retryable
				err,
				"",
				bucketName,
				objectKey,
			)
		}

		// TRANSIENT TRIGGER PATTERN: Process with idempotency check and archive-first loading
		err = ProcessTransientTrigger(ctx, bucketName, objectKey, customerCode, changeID, cfg)
		if err != nil {
			// Check if this is a "trigger already processed" case (not an error)
			if strings.Contains(err.Error(), "trigger already processed") {
				log.Printf("ℹ️  Trigger already processed, skipping: %s", objectKey)
				return nil // Successfully handled (idempotent)
			}

			// Classify the error appropriately
			return ClassifyError(err, "", bucketName, objectKey)
		}
	}

	return nil
}

// ProcessSQSMessage processes a legacy SQS message in Lambda context
func ProcessSQSMessage(ctx context.Context, sqsMsg types.SQSMessage, cfg *types.Config) error {
	// Validate the message
	if err := ValidateSQSMessage(sqsMsg); err != nil {
		// Invalid message format is non-retryable
		return NewProcessingError(
			ErrorTypeInvalidFormat,
			fmt.Sprintf("Invalid SQS message format: %v", err),
			false, // Not retryable
			err,
			sqsMsg.MessageID,
			sqsMsg.S3Bucket,
			sqsMsg.S3Key,
		)
	}

	// Validate customer code
	if err := ValidateCustomerCode(sqsMsg.CustomerCode, cfg); err != nil {
		// Invalid customer code is non-retryable
		return NewProcessingError(
			ErrorTypeInvalidCustomer,
			fmt.Sprintf("Invalid customer code %s: %v", sqsMsg.CustomerCode, err),
			false, // Not retryable
			err,
			sqsMsg.MessageID,
			sqsMsg.S3Bucket,
			sqsMsg.S3Key,
		)
	}

	// Download metadata from S3
	metadata, err := DownloadMetadataFromS3(ctx, sqsMsg.S3Bucket, sqsMsg.S3Key, cfg.AWSRegion)
	if err != nil {
		// Classify the S3 error appropriately
		return ClassifyError(err, sqsMsg.MessageID, sqsMsg.S3Bucket, sqsMsg.S3Key)
	}

	// Process the change request
	err = ProcessChangeRequest(ctx, sqsMsg.CustomerCode, metadata, cfg, sqsMsg.S3Bucket, sqsMsg.S3Key)
	if err != nil {
		// Email/processing errors are typically retryable
		return NewProcessingError(
			ErrorTypeEmailError,
			fmt.Sprintf("Failed to process change request for customer %s: %v", sqsMsg.CustomerCode, err),
			true, // Retryable
			err,
			sqsMsg.MessageID,
			sqsMsg.S3Bucket,
			sqsMsg.S3Key,
		)
	}

	return nil
}

// ExtractCustomerCodeFromS3Key extracts customer code from S3 object key
func ExtractCustomerCodeFromS3Key(s3Key string) (string, error) {
	// Expected format: customers/{customer-code}/filename.json
	parts := strings.Split(s3Key, "/")
	if len(parts) < 2 || parts[0] != "customers" {
		return "", fmt.Errorf("invalid S3 key format, expected customers/{customer-code}/filename.json")
	}
	return parts[1], nil
}

// ExtractCustomerCodeAndChangeIDFromS3Key extracts both customer code and change ID from S3 object key
func ExtractCustomerCodeAndChangeIDFromS3Key(s3Key string) (string, string, error) {
	// Expected format: customers/{customer-code}/{changeId}.json
	parts := strings.Split(s3Key, "/")
	if len(parts) < 3 || parts[0] != "customers" {
		return "", "", fmt.Errorf("invalid S3 key format, expected customers/{customer-code}/{changeId}.json")
	}

	customerCode := parts[1]
	filename := parts[2]

	// Extract change ID from filename (remove .json extension)
	changeID := strings.TrimSuffix(filename, ".json")
	if changeID == "" {
		return "", "", fmt.Errorf("invalid filename format, expected {changeId}.json")
	}

	return customerCode, changeID, nil
}

// hasBeenProcessedForCurrentStatus checks if a change has already been processed for its current status
// This prevents duplicate processing when triggers are created multiple times for the same status
func hasBeenProcessedForCurrentStatus(metadata *types.ChangeMetadata) bool {
	// For cancelled and completed statuses, check if there's already a "processed" entry
	// after the status change modification
	if metadata.Status != "cancelled" && metadata.Status != "completed" {
		return false // Only check for final states
	}

	// Find the most recent status change modification
	var statusChangeIndex = -1
	for i := len(metadata.Modifications) - 1; i >= 0; i-- {
		mod := metadata.Modifications[i]
		if mod.ModificationType == "cancelled" ||
			mod.ModificationType == "completed" {
			statusChangeIndex = i
			break
		}
	}

	// If no status change found, not processed yet
	if statusChangeIndex == -1 {
		return false
	}

	// Check if there's a "processed" entry after the status change
	for i := statusChangeIndex + 1; i < len(metadata.Modifications); i++ {
		if metadata.Modifications[i].ModificationType == types.ModificationTypeProcessed {
			log.Printf("✓ Found processed entry after %s status change", metadata.Status)
			return true
		}
	}

	return false
}

// ProcessTransientTrigger implements the Transient Trigger Pattern for backend processing
// This function:
// 1. Checks if trigger still exists (idempotency)
// 2. Loads authoritative data from archive/
// 3. Processes the change (sends emails, schedules meetings)
// 4. Updates archive/ with processing results
// 5. Deletes the trigger from customers/
func ProcessTransientTrigger(ctx context.Context, bucketName, triggerKey, customerCode, changeID string, cfg *types.Config) error {
	log.Printf("🔄 Processing transient trigger: customer=%s, changeID=%s", customerCode, changeID)

	// Step 1: Idempotency check - verify trigger still exists
	if exists, err := CheckTriggerExists(ctx, bucketName, triggerKey, cfg.AWSRegion); err != nil {
		return fmt.Errorf("failed to check trigger existence: %w", err)
	} else if !exists {
		log.Printf("ℹ️  Trigger already processed, skipping: %s", triggerKey)
		return fmt.Errorf("trigger already processed: %s", triggerKey)
	}

	// Step 2: Load authoritative data from archive
	archiveKey := fmt.Sprintf("archive/%s.json", changeID)
	metadata, err := DownloadMetadataFromS3(ctx, bucketName, archiveKey, cfg.AWSRegion)
	if err != nil {
		return fmt.Errorf("failed to load from archive: %w", err)
	}

	log.Printf("✅ Loaded change from archive: %s (status: %s)", changeID, metadata.Status)

	// Step 2.1: Validate that object doesn't contain legacy metadata map
	if err := metadata.ValidateLegacyMetadata(); err != nil {
		log.Printf("❌ Legacy metadata detected in object %s: %v", changeID, err)
		return fmt.Errorf("legacy metadata validation failed: %w", err)
	}

	// Step 2.5: Check if change has already been processed for its current status
	// This prevents duplicate emails when triggers are created for already-processed changes
	// We check the modifications array to see if there's already a "processed" entry for this status
	if hasBeenProcessedForCurrentStatus(metadata) {
		log.Printf("⏭️  Change %s has already been processed for status '%s', skipping", changeID, metadata.Status)
		// Still delete the trigger to clean up
		_ = DeleteTrigger(ctx, bucketName, triggerKey, cfg.AWSRegion)
		return fmt.Errorf("trigger already processed: change already processed for status %s", metadata.Status)
	}

	// Step 3: Process the change (send emails, schedule meetings, etc.)
	var processingErr error
	if strings.HasPrefix(metadata.ObjectType, "announcement_") {
		// Route to announcement handler
		processingErr = handleAnnouncementEventNew(ctx, customerCode, bucketName, archiveKey, cfg)
	} else {
		// Process as change request
		processingErr = ProcessChangeRequest(ctx, customerCode, metadata, cfg, bucketName, archiveKey)
	}

	if processingErr != nil {
		log.Printf("❌ Failed to process change: %v", processingErr)
		// Don't delete trigger - allow retry
		return fmt.Errorf("failed to process change: %w", processingErr)
	}

	log.Printf("✅ Successfully processed change: %s", changeID)

	// Step 4: Update archive with processing results (only for changes, not announcements)
	// Announcements are updated by the AnnouncementProcessor to preserve AnnouncementMetadata structure
	if !strings.HasPrefix(metadata.ObjectType, "announcement_") {
		err = UpdateArchiveWithProcessingResult(ctx, bucketName, archiveKey, customerCode, cfg.AWSRegion)
		if err != nil {
			log.Printf("❌ Failed to update archive: %v", err)
			// CRITICAL: Delete trigger but do NOT acknowledge SQS message
			// This allows retry while preventing duplicate triggers
			_ = DeleteTrigger(ctx, bucketName, triggerKey, cfg.AWSRegion)
			return fmt.Errorf("failed to update archive: %w", err)
		}

		log.Printf("✅ Successfully updated archive with processing results")
	} else {
		log.Printf("✅ Skipping archive update for announcement (handled by AnnouncementProcessor)")
	}

	// Step 5: Delete trigger (cleanup)
	err = DeleteTrigger(ctx, bucketName, triggerKey, cfg.AWSRegion)
	if err != nil {
		log.Printf("⚠️  Failed to delete trigger, but processing complete: %v", err)
		// Non-fatal - processing is complete, archive is updated
	} else {
		log.Printf("✅ Successfully deleted trigger: %s", triggerKey)
	}

	log.Printf("✅ Transient trigger processing completed for customer %s: %s", customerCode, changeID)
	return nil
}

// CheckTriggerExists checks if a trigger object still exists in S3 (for idempotency)
func CheckTriggerExists(ctx context.Context, bucket, key, region string) (bool, error) {
	// Create S3 client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return false, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	// Use HeadObject to check if object exists
	_, err = s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// Check if error is "not found"
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return false, nil // Object doesn't exist (already processed)
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil // Object exists
}

// UpdateArchiveWithProcessingResult updates the archive object with processing metadata using ETag-based optimistic locking
func UpdateArchiveWithProcessingResult(ctx context.Context, bucket, archiveKey, customerCode, region string) error {
	// NOTE: Removed the "processed by backend-system" modification entry update
	// This was causing unnecessary ETag changes that interfered with frontend polling
	// for meeting details. The meeting details are added separately by the processor.
	log.Printf("✅ Successfully processed change (skipping modification entry to preserve ETag for customer=%s)", customerCode)
	return nil
}

// DeleteTrigger deletes a trigger object from S3 (cleanup after processing)
func DeleteTrigger(ctx context.Context, bucket, key, region string) error {
	log.Printf("🗑️  Deleting trigger: s3://%s/%s", bucket, key)

	// Create S3 client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	// Delete the object
	_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete trigger object: %w", err)
	}

	log.Printf("✅ Successfully deleted trigger: %s", key)
	return nil
}

// ValidateCustomerCode validates that a customer code exists in the configuration
func ValidateCustomerCode(customerCode string, cfg *types.Config) error {
	if customerCode == "" {
		return fmt.Errorf("customer code cannot be empty")
	}

	if _, exists := cfg.CustomerMappings[customerCode]; !exists {
		return fmt.Errorf("customer code %s not found in configuration", customerCode)
	}

	return nil
}

// ValidateSQSMessage validates the structure of an SQS message
func ValidateSQSMessage(msg types.SQSMessage) error {
	if msg.CustomerCode == "" {
		return fmt.Errorf("customer_code is required")
	}
	if msg.S3Bucket == "" {
		return fmt.Errorf("s3_bucket is required")
	}
	if msg.S3Key == "" {
		return fmt.Errorf("s3_key is required")
	}
	return nil
}

// DownloadMetadataFromS3 downloads and parses metadata from S3
func DownloadMetadataFromS3(ctx context.Context, bucket, key, region string) (*types.ChangeMetadata, error) {
	// Create S3 client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	// Download object
	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download S3 object: %w", err)
	}
	defer result.Body.Close()

	// Read the content into a byte slice so we can try multiple parsing approaches
	contentBytes, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object content: %w", err)
	}

	// Extract request type from S3 object metadata if available
	var requestTypeFromS3 string
	if result.Metadata != nil {
		if reqType, exists := result.Metadata["request-type"]; exists {
			requestTypeFromS3 = reqType
		}
	}

	// First, check if this is an announcement by looking at object_type
	var rawMetadata map[string]interface{}
	if err := json.Unmarshal(contentBytes, &rawMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	objectType, _ := rawMetadata["object_type"].(string)

	// Check if this is an announcement (object_type starts with "announcement_")
	if strings.HasPrefix(objectType, "announcement_") {
		// Announcements are handled by handleAnnouncementEventNew which parses AnnouncementMetadata directly
		// Return a minimal ChangeMetadata with just ObjectType for routing purposes
		// The actual announcement processing will read from S3 again using AnnouncementMetadata
		metadata := types.ChangeMetadata{
			ObjectType: objectType,
		}
		return &metadata, nil
	}

	// Parse as standard ChangeMetadata (flat structure)
	var metadata types.ChangeMetadata
	if err := json.Unmarshal(contentBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata as ChangeMetadata: %w", err)
	}

	// Validate that we have essential fields for a valid ChangeMetadata
	if metadata.ChangeID == "" && metadata.ChangeTitle == "" {
		return nil, fmt.Errorf("invalid ChangeMetadata: missing both ChangeID and ChangeTitle")
	}

	// Ensure we have a ChangeID if missing
	if metadata.ChangeID == "" && metadata.ChangeTitle != "" {
		metadata.ChangeID = fmt.Sprintf("CHG-%d", time.Now().Unix())
	}

	// Set default status if missing
	if metadata.Status == "" {
		metadata.Status = "submitted"
	}

	// Apply request type from S3 metadata if available
	if requestTypeFromS3 != "" {
		if metadata.Metadata == nil {
			metadata.Metadata = make(map[string]interface{})
		}
		metadata.Metadata["request_type"] = requestTypeFromS3
	}

	return &metadata, nil
}

// ProcessChangeRequest processes a change request with metadata
func ProcessChangeRequest(ctx context.Context, customerCode string, metadata *types.ChangeMetadata, cfg *types.Config, s3Bucket, s3Key string) error {
	// Set the current customer info for recipient restrictions
	if customerInfo, exists := cfg.CustomerMappings[customerCode]; exists {
		currentCustomerInfo = &customerInfo
		defer func() { currentCustomerInfo = nil }() // Clear after processing
	}

	// Determine the request type based on the metadata structure and source
	requestType := DetermineRequestType(metadata)

	// CRITICAL: If request_type is not set but status is approved, default to approved_announcement
	// This handles the case where approved changes need to send approval notifications
	if requestType == "" && metadata.Status == "approved" {
		requestType = "approved_announcement"
		log.Printf("✅ Defaulting to approved_announcement for approved change with no explicit request_type")
	}

	// Create change details for email notification (same as SQS processor)
	changeDetails := map[string]interface{}{
		"change_id":            metadata.ChangeID,
		"changeTitle":          metadata.ChangeTitle,
		"changeReason":         metadata.ChangeReason,
		"implementationPlan":   metadata.ImplementationPlan,
		"testPlan":             metadata.TestPlan,
		"customerImpact":       metadata.CustomerImpact,
		"rollbackPlan":         metadata.RollbackPlan,
		"snowTicket":           metadata.SnowTicket,
		"jiraTicket":           metadata.JiraTicket,
		"implementationStart":  metadata.ImplementationStart.Format(time.RFC3339),
		"implementationEnd":    metadata.ImplementationEnd.Format(time.RFC3339),
		"timezone":             metadata.Timezone,
		"status":               metadata.Status,
		"version":              metadata.Version,
		"createdAt":            metadata.CreatedAt,
		"createdBy":            metadata.CreatedBy,
		"modifiedAt":           metadata.ModifiedAt,
		"modifiedBy":           metadata.ModifiedBy,
		"submittedAt":          metadata.SubmittedAt,
		"submittedBy":          metadata.SubmittedBy,
		"approvedAt":           metadata.ApprovedAt,
		"approvedBy":           metadata.ApprovedBy,
		"source":               metadata.Source,
		"testRun":              metadata.TestRun,
		"customers":            metadata.Customers,
		"request_type":         requestType,
		"processing_timestamp": datetime.FormatRFC3339(time.Now()),
	}

	// Add any additional metadata
	if metadata.Metadata != nil {
		for key, value := range metadata.Metadata {
			changeDetails[fmt.Sprintf("meta_%s", key)] = value
		}
	}

	// Send appropriate notification based on request type
	switch requestType {
	case "approval_request":
		err := SendApprovalRequestEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("ERROR: Failed to send approval request email for customer %s: %v", customerCode, err)
		}

	case "approved_announcement":
		err := SendApprovedAnnouncementEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("ERROR: Failed to send approved announcement email for customer %s: %v", customerCode, err)
		}

		// Check if this approved change has meeting settings and schedule multi-customer meeting
		err = ScheduleMultiCustomerMeetingIfNeeded(ctx, metadata, cfg, s3Bucket, s3Key)
		if err != nil {
			log.Printf("ERROR: Failed to schedule meeting for change %s: %v", metadata.ChangeID, err)
		}

	case "change_complete":
		err := SendChangeCompleteEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("ERROR: Failed to send change complete email for customer %s: %v", customerCode, err)
		}

		// Create Typeform survey for completed changes
		err = CreateSurveyForCompletedChange(ctx, metadata, cfg, s3Bucket, s3Key)
		if err != nil {
			log.Printf("ERROR: Failed to create survey for change %s: %v", metadata.ChangeID, err)
			// Don't fail the entire workflow if survey creation fails
		}
	case "change_cancelled":
		err := SendChangeCancelledEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("ERROR: Failed to send change cancelled email for customer %s: %v", customerCode, err)
		}

		// Cancel the meeting if one was scheduled
		err = CancelScheduledMeetingIfNeeded(ctx, metadata, cfg, s3Bucket, s3Key)
		if err != nil {
			log.Printf("ERROR: Failed to cancel meeting for change %s: %v", metadata.ChangeID, err)
		}
	default:
		log.Printf("WARNING: Unknown event type '%s' - ignoring", requestType)
		return nil
	}

	// Note: This system handles change notifications only, not AWS account modifications
	log.Printf("Change notification processing completed for customer %s", customerCode)

	return nil
}

// isMeetingRequired checks if a meeting is required based on the include_meeting field
// This is a DRY helper used by both changes and announcements
func isMeetingRequired(metadata *types.ChangeMetadata) bool {
	// Only check the top-level IncludeMeeting field
	// This field is used by both changes and announcements
	return metadata.IncludeMeeting
}

// formatFileSize formats a file size in bytes to a human-readable string
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// DetermineRequestTypeFromStatus determines request type from status field only
// Works for both ChangeMetadata and AnnouncementMetadata
func DetermineRequestTypeFromStatus(status string) string {
	// ONLY check status parameter (no nested fields, no metadata map)
	switch status {
	case "submitted":
		return "approval_request"
	case "approved":
		return "approved_announcement"
	case "completed":
		return "change_complete"
	case "cancelled":
		return "change_cancelled"
	default:
		log.Printf("⚠️  Unknown status: %s", status)
		return "unknown"
	}
}

// DetermineRequestType determines the type of request for changes
func DetermineRequestType(metadata *types.ChangeMetadata) string {
	return DetermineRequestTypeFromStatus(metadata.Status)
}

// DetermineAnnouncementRequestType determines the type of request for announcements
func DetermineAnnouncementRequestType(metadata *types.AnnouncementMetadata) string {
	return DetermineRequestTypeFromStatus(metadata.Status)
}

// ScheduleMultiCustomerMeetingIfNeeded schedules a Microsoft Graph meeting if the change requires it
func ScheduleMultiCustomerMeetingIfNeeded(ctx context.Context, metadata *types.ChangeMetadata, cfg *types.Config, s3Bucket, s3Key string) error {
	log.Printf("🔍 Checking if change %s requires meeting scheduling", metadata.ChangeID)
	log.Printf("📋 Change details - Title: %s, Customers: %v, Status: %s", metadata.ChangeTitle, metadata.Customers, metadata.Status)
	log.Printf("📅 Implementation schedule - Begin: %s, End: %s",
		metadata.ImplementationStart.Format("2006-01-02 15:04:05 MST"),
		metadata.ImplementationEnd.Format("2006-01-02 15:04:05 MST"))

	// Debug: Show metadata structure
	if metadata.Metadata == nil {
		log.Printf("⚠️  metadata.Metadata is nil")
	} else {
		log.Printf("📋 metadata.Metadata contains %d fields:", len(metadata.Metadata))
		for key, value := range metadata.Metadata {
			log.Printf("  - %s: %v (type: %T)", key, value, value)
		}
	}

	// Use DRY helper to check if meeting is required
	if !isMeetingRequired(metadata) {
		log.Printf("⏭️  No meeting required for change %s", metadata.ChangeID)
		return nil
	}

	log.Printf("📅 Meeting is required for change %s", metadata.ChangeID)

	// Create meeting scheduler with idempotency support
	scheduler := NewMeetingScheduler(cfg.AWSRegion)

	// Schedule or update the meeting (idempotency is handled within ScheduleMeetingWithMetadata)
	meetingMetadata, err := scheduler.ScheduleMeetingWithMetadata(ctx, metadata, s3Bucket, s3Key)
	if err != nil {
		return fmt.Errorf("failed to schedule meeting for change %s: %w", metadata.ChangeID, err)
	}

	log.Printf("✅ Successfully scheduled meeting for change %s: ID=%s", metadata.ChangeID, meetingMetadata.MeetingID)
	return nil
}

// CancelScheduledMeetingIfNeeded cancels a Microsoft Graph meeting if one was scheduled for this change
func CancelScheduledMeetingIfNeeded(ctx context.Context, metadata *types.ChangeMetadata, cfg *types.Config, s3Bucket, s3Key string) error {
	log.Printf("🔍 Checking if change %s has a scheduled meeting to cancel", metadata.ChangeID)
	log.Printf("📊 Metadata has %d modification entries", len(metadata.Modifications))

	// Debug: Log all modification types
	if len(metadata.Modifications) > 0 {
		log.Printf("📋 Modification types in metadata:")
		for i, mod := range metadata.Modifications {
			log.Printf("  %d. Type: %s, Timestamp: %s", i+1, mod.ModificationType, mod.Timestamp.Format("2006-01-02 15:04:05"))
			if mod.ModificationType == types.ModificationTypeMeetingScheduled && mod.MeetingMetadata != nil {
				log.Printf("     Meeting ID: %s, Join URL: %s", mod.MeetingMetadata.MeetingID, mod.MeetingMetadata.JoinURL)
			}
		}
	}

	// FIRST: Check nested meeting_metadata field (most reliable)
	var meetingID string
	if metadata.MeetingMetadata != nil && metadata.MeetingMetadata.MeetingID != "" {
		meetingID = metadata.MeetingMetadata.MeetingID
		log.Printf("✅ Found meeting_id in meeting_metadata: %s", meetingID)
	} else {
		// FALLBACK: Check modifications array
		latestMeeting := metadata.GetLatestMeetingMetadata()
		if latestMeeting == nil {
			log.Printf("⚠️  No scheduled meeting found for change %s, nothing to cancel", metadata.ChangeID)
			return nil
		}
		meetingID = latestMeeting.MeetingID
		log.Printf("📅 Found meeting_id in modifications array: %s", meetingID)
	}

	log.Printf("📅 Cancelling meeting for change %s: ID=%s", metadata.ChangeID, meetingID)

	// Get organizer email from environment or config
	organizerEmail := os.Getenv("MEETING_ORGANIZER_EMAIL")
	if organizerEmail == "" {
		organizerEmail = "ccoe@hearst.com" // Default organizer
		log.Printf("⚠️  MEETING_ORGANIZER_EMAIL not set, using default: %s", organizerEmail)
	}

	// Cancel the meeting via Microsoft Graph API
	err := cancelGraphMeeting(meetingID, organizerEmail)
	if err != nil {
		log.Printf("❌ Failed to cancel Graph meeting %s: %v", meetingID, err)
		// Don't return error - we still want to update S3 with cancellation entry
	} else {
		log.Printf("✅ Successfully cancelled Graph meeting %s", meetingID)
	}

	// Update S3 object with meeting cancellation entry
	if s3Bucket != "" && s3Key != "" {
		modManager := NewModificationManager()
		s3UpdateManager, err := NewS3UpdateManager(cfg.AWSRegion)
		if err != nil {
			log.Printf("⚠️  Failed to create S3UpdateManager: %v", err)
			return nil
		}

		// Create meeting cancelled entry
		cancelledEntry, err := modManager.CreateMeetingCancelledEntry()
		if err != nil {
			log.Printf("⚠️  Failed to create meeting cancelled entry: %v", err)
		} else {
			// Update S3 with the new modification entry
			err = s3UpdateManager.UpdateChangeObjectWithModification(ctx, s3Bucket, s3Key, cancelledEntry)
			if err != nil {
				log.Printf("⚠️  Failed to update S3 object with meeting cancelled entry: %v", err)
			} else {
				log.Printf("✅ Updated S3 object with meeting cancelled entry")
			}
		}
	}

	return nil
}

// CreateSurveyForCompletedChange creates a Typeform survey for a completed change
func CreateSurveyForCompletedChange(ctx context.Context, metadata *types.ChangeMetadata, cfg *types.Config, s3Bucket, s3Key string) error {
	log.Printf("🔍 Creating survey for completed change %s", metadata.ChangeID)

	// Import typeform package
	typeformClient, err := typeform.NewClient(slog.Default())
	if err != nil {
		log.Printf("⚠️  Failed to create Typeform client: %v", err)
		return fmt.Errorf("failed to create typeform client: %w", err)
	}

	// Create S3 client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	s3Client := s3.NewFromConfig(awsCfg)

	// Determine survey type based on object type
	surveyType := determineSurveyType(metadata.ObjectType)

	// Extract metadata for survey creation
	year, quarter := extractYearQuarter(metadata.ImplementationStart)
	eventType, eventSubtype := extractEventTypeAndSubtype(metadata.ObjectType)

	// Get customer code from the first customer in the list
	customerCode := ""
	if len(metadata.Customers) > 0 {
		customerCode = metadata.Customers[0]
	}

	surveyMetadata := &typeform.SurveyMetadata{
		CustomerCode: customerCode,
		ObjectID:     metadata.ChangeID,
		Year:         year,
		Quarter:      quarter,
		EventType:    eventType,
		EventSubtype: eventSubtype,
	}

	// Create the survey
	response, err := typeformClient.CreateSurvey(ctx, s3Client, s3Bucket, surveyMetadata, surveyType)
	if err != nil {
		log.Printf("❌ Failed to create survey for change %s: %v", metadata.ChangeID, err)
		return fmt.Errorf("failed to create survey: %w", err)
	}

	log.Printf("✅ Successfully created survey for change %s: ID=%s", metadata.ChangeID, response.ID)
	return nil
}

// determineSurveyType determines the survey type based on object type
func determineSurveyType(objectType string) typeform.SurveyType {
	// For changes, use change survey type
	if objectType == "" || objectType == "change" {
		return typeform.SurveyTypeChange
	}

	// For announcements, determine based on announcement type
	switch objectType {
	case "announcement_cic":
		return typeform.SurveyTypeCIC
	case "announcement_innersource":
		return typeform.SurveyTypeInnerSource
	case "announcement_finops":
		return typeform.SurveyTypeFinOps
	case "announcement_general":
		return typeform.SurveyTypeGeneral
	default:
		return typeform.SurveyTypeGeneral
	}
}

// extractYearQuarter extracts year and quarter from a timestamp
func extractYearQuarter(t time.Time) (string, string) {
	year := t.Format("2006")
	month := t.Month()

	var quarter string
	switch {
	case month >= 1 && month <= 3:
		quarter = "Q1"
	case month >= 4 && month <= 6:
		quarter = "Q2"
	case month >= 7 && month <= 9:
		quarter = "Q3"
	case month >= 10 && month <= 12:
		quarter = "Q4"
	}

	return year, quarter
}

// extractEventTypeAndSubtype extracts event type and subtype from object type
func extractEventTypeAndSubtype(objectType string) (string, string) {
	// For changes
	if objectType == "" || objectType == "change" {
		return "change", "general"
	}

	// For announcements
	if strings.HasPrefix(objectType, "announcement_") {
		subtype := strings.TrimPrefix(objectType, "announcement_")
		return "announcement", subtype
	}

	return "general", "general"
}

// generateSurveyURLAndQRCode generates a Typeform survey URL with hidden parameters and QR code
func generateSurveyURLAndQRCode(metadata *types.ChangeMetadata, cfg *types.Config) (string, string) {
	// Get survey ID from S3 metadata (stored when survey was created)
	// For now, return empty strings - this will be populated when survey is created
	// The survey URL will be stored in S3 metadata and retrieved here

	// TODO: Retrieve survey URL from S3 object metadata
	// For now, return empty to avoid blocking email generation
	return "", ""
}

// findMeetingBySubject searches for a meeting by subject in Microsoft Graph
// Returns the meeting ID if found, empty string if not found
func findMeetingBySubject(subject, organizerEmail string) (string, error) {
	log.Printf("🔍 Searching for meeting with subject: '%s'", subject)

	// Get access token
	accessToken, err := ses.GetGraphAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get Graph access token: %w", err)
	}

	// Search for recent meetings with this subject
	// Get last 50 meetings and filter by subject
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events?$top=50&$select=id,subject&$orderby=start/dateTime desc", organizerEmail)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to search meetings: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("meeting search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Value []struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
		} `json:"value"`
	}

	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return "", fmt.Errorf("failed to parse search response: %w", err)
	}

	log.Printf("📊 Found %d recent meetings, searching for subject match...", len(searchResponse.Value))

	// Look for exact subject match
	for _, meeting := range searchResponse.Value {
		if meeting.Subject == subject {
			log.Printf("✅ Found matching meeting: ID=%s", meeting.ID)
			return meeting.ID, nil
		}
	}

	log.Printf("⚠️  No meeting found with exact subject match")
	return "", nil
}

// cancelGraphMeeting cancels a Microsoft Graph meeting by ID
func cancelGraphMeeting(meetingID, organizerEmail string) error {
	log.Printf("🗑️  Attempting to cancel Graph meeting: ID=%s, Organizer=%s", meetingID, organizerEmail)

	// Validate inputs
	if meetingID == "" {
		return fmt.Errorf("meeting ID cannot be empty")
	}
	if organizerEmail == "" {
		return fmt.Errorf("organizer email cannot be empty")
	}

	// Get access token
	log.Printf("🔑 Getting Graph API access token...")
	accessToken, err := ses.GetGraphAccessToken()
	if err != nil {
		log.Printf("❌ Failed to get Graph access token: %v", err)
		return fmt.Errorf("failed to get Graph access token: %w", err)
	}
	log.Printf("✅ Successfully obtained Graph API access token")

	// Delete the meeting using Microsoft Graph API
	// DELETE https://graph.microsoft.com/v1.0/users/{user-id}/events/{event-id}
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events/%s", organizerEmail, meetingID)
	log.Printf("🌐 DELETE request URL: %s", url)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("❌ Failed to create DELETE request: %v", err)
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	log.Printf("📤 Sending DELETE request to Microsoft Graph API...")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ HTTP request failed: %v", err)
		return fmt.Errorf("failed to delete meeting: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, _ := io.ReadAll(resp.Body)
	log.Printf("📥 Graph API response: Status=%d, Body=%s", resp.StatusCode, string(body))

	// 204 No Content is the success response for DELETE
	if resp.StatusCode == http.StatusNoContent {
		log.Printf("✅ Successfully deleted Graph meeting %s (HTTP 204)", meetingID)
		return nil
	}

	// 404 Not Found means the meeting was already deleted or doesn't exist
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("⚠️  Meeting %s not found (may have been already deleted)", meetingID)
		return nil // Not an error - meeting is gone either way
	}

	// Any other status code is an error
	return fmt.Errorf("failed to delete meeting (status %d): %s", resp.StatusCode, string(body))
}

// createChangeMetadataFromChangeDetails converts changeDetails map to ChangeMetadata
func createChangeMetadataFromChangeDetails(changeDetails map[string]interface{}) *types.ChangeMetadata {
	// Helper function to safely get string from map
	getString := func(key string) string {
		if val, ok := changeDetails[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
		return ""
	}

	// Helper function to safely get string slice from map
	getStringSlice := func(key string) []string {
		if val, ok := changeDetails[key]; ok {
			if slice, ok := val.([]string); ok {
				return slice
			}
			// Try to convert from interface{} slice
			if interfaceSlice, ok := val.([]interface{}); ok {
				var result []string
				for _, item := range interfaceSlice {
					if str, ok := item.(string); ok {
						result = append(result, str)
					}
				}
				return result
			}
		}
		return []string{}
	}

	// Create flat ChangeMetadata structure
	metadata := &types.ChangeMetadata{
		ObjectType:          "change",
		ChangeID:            getString("change_id"),
		ChangeTitle:         getString("changeTitle"),
		ChangeReason:        getString("changeReason"),
		Customers:           getStringSlice("customers"),
		ImplementationPlan:  getString("implementationPlan"),
		TestPlan:            getString("testPlan"),
		CustomerImpact:      getString("customerImpact"),
		RollbackPlan:        getString("rollbackPlan"),
		SnowTicket:          getString("snowTicket"),
		JiraTicket:          getString("jiraTicket"),
		ImplementationStart: parseTimeString(getString("implementationStart")),
		ImplementationEnd:   parseTimeString(getString("implementationEnd")),
		Timezone:            getString("timezone"),
		Status:              getString("status"),
		Version:             1, // Default version
		CreatedAt:           parseTimeString(getString("createdAt")),
		CreatedBy:           getString("createdBy"),
		ModifiedAt:          parseTimeString(getString("modifiedAt")),
		ModifiedBy:          getString("modifiedBy"),
		SubmittedAt:         parseTimeStringPtr(getString("submittedAt")),
		SubmittedBy:         getString("submittedBy"),
		ApprovedAt:          parseTimeStringPtr(getString("approvedAt")),
		ApprovedBy:          getString("approvedBy"),
		Source:              getString("source"),
	}

	// Parse modifications array if present
	if modificationsVal, ok := changeDetails["modifications"]; ok {
		if modificationsSlice, ok := modificationsVal.([]interface{}); ok {
			for _, modVal := range modificationsSlice {
				if modMap, ok := modVal.(map[string]interface{}); ok {
					modEntry := types.ModificationEntry{}

					// Parse timestamp
					if tsStr, ok := modMap["timestamp"].(string); ok {
						if ts, err := time.Parse(time.RFC3339, tsStr); err == nil {
							modEntry.Timestamp = ts
						}
					}

					// Parse user ID
					if userID, ok := modMap["user_id"].(string); ok {
						modEntry.UserID = userID
					}

					// Parse modification type
					if modType, ok := modMap["modification_type"].(string); ok {
						modEntry.ModificationType = modType
					}

					// Parse meeting metadata if present
					if meetingMeta, ok := modMap["meeting_metadata"].(map[string]interface{}); ok {
						modEntry.MeetingMetadata = &types.MeetingMetadata{}
						if meetingID, ok := meetingMeta["meeting_id"].(string); ok {
							modEntry.MeetingMetadata.MeetingID = meetingID
						}
						if joinURL, ok := meetingMeta["join_url"].(string); ok {
							modEntry.MeetingMetadata.JoinURL = joinURL
						}
					}

					metadata.Modifications = append(metadata.Modifications, modEntry)
				}
			}
		}
	}

	// Set default timezone if empty
	if metadata.Timezone == "" {
		metadata.Timezone = "America/New_York"
	}

	// Set default status if empty
	if metadata.Status == "" {
		metadata.Status = "submitted"
	}

	return metadata
}

// createTempMeetingMetadata creates a temporary metadata file path for meeting scheduling
func createTempMeetingMetadata(metadata *types.ChangeMetadata, meetingTitle, meetingDate, meetingDuration, meetingLocation string) (string, error) {
	// Create ApprovalRequestMetadata structure for compatibility with existing meeting functions
	meetingMetadata := types.ApprovalRequestMetadata{
		ChangeMetadata: struct {
			Title         string   `json:"changeTitle"`
			CustomerNames []string `json:"customerNames"`
			CustomerCodes []string `json:"customerCodes"`
			Tickets       struct {
				ServiceNow string `json:"serviceNow"`
				Jira       string `json:"jira"`
			} `json:"tickets"`
			ChangeReason           string `json:"changeReason"`
			ImplementationPlan     string `json:"implementationPlan"`
			TestPlan               string `json:"testPlan"`
			ExpectedCustomerImpact string `json:"expectedCustomerImpact"`
			RollbackPlan           string `json:"rollbackPlan"`
			Schedule               struct {
				ImplementationStart time.Time `json:"implementationStart"`
				ImplementationEnd   time.Time `json:"implementationEnd"`
				Timezone            string    `json:"timezone"`
				BeginDate           string    `json:"beginDate,omitempty"`
				BeginTime           string    `json:"beginTime,omitempty"`
				EndDate             string    `json:"endDate,omitempty"`
				EndTime             string    `json:"endTime,omitempty"`
			} `json:"schedule"`
			Description string `json:"description"`
			ApprovedBy  string `json:"approvedBy,omitempty"`
			ApprovedAt  string `json:"approvedAt,omitempty"`
		}{
			Title:                  metadata.ChangeTitle,
			CustomerNames:          []string{}, // Will be populated from customer codes
			CustomerCodes:          metadata.Customers,
			ChangeReason:           metadata.ChangeReason,
			ImplementationPlan:     metadata.ImplementationPlan,
			TestPlan:               metadata.TestPlan,
			ExpectedCustomerImpact: metadata.CustomerImpact,
			RollbackPlan:           metadata.RollbackPlan,
			Schedule: struct {
				ImplementationStart time.Time `json:"implementationStart"`
				ImplementationEnd   time.Time `json:"implementationEnd"`
				Timezone            string    `json:"timezone"`
				BeginDate           string    `json:"beginDate,omitempty"`
				BeginTime           string    `json:"beginTime,omitempty"`
				EndDate             string    `json:"endDate,omitempty"`
				EndTime             string    `json:"endTime,omitempty"`
			}{
				ImplementationStart: metadata.ImplementationStart,
				ImplementationEnd:   metadata.ImplementationEnd,
				Timezone:            metadata.Timezone,
			},
			Description: fmt.Sprintf("Implementation meeting for change: %s", metadata.ChangeTitle),
			ApprovedBy:  metadata.ApprovedBy,
			ApprovedAt:  formatTimePtr(metadata.ApprovedAt),
		},
		EmailNotification: struct {
			Subject         string   `json:"subject"`
			CustomerNames   []string `json:"customerNames"`
			CustomerCodes   []string `json:"customerCodes"`
			ScheduledWindow struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"scheduledWindow"`
			Tickets struct {
				Snow string `json:"snow"`
				Jira string `json:"jira"`
			} `json:"tickets"`
		}{
			Subject:       fmt.Sprintf("Meeting: %s", meetingTitle),
			CustomerCodes: metadata.Customers,
		},
		GeneratedAt: datetime.FormatRFC3339(time.Now()),
		GeneratedBy: "lambda-auto-scheduler",
	}

	// Set tickets
	meetingMetadata.ChangeMetadata.Tickets.ServiceNow = metadata.SnowTicket
	meetingMetadata.ChangeMetadata.Tickets.Jira = metadata.JiraTicket

	// Set schedule information (backward compatibility fields)
	meetingMetadata.ChangeMetadata.Schedule.BeginDate = metadata.ImplementationStart.Format("2006-01-02")
	meetingMetadata.ChangeMetadata.Schedule.BeginTime = metadata.ImplementationStart.Format("15:04:05")
	meetingMetadata.ChangeMetadata.Schedule.EndDate = metadata.ImplementationEnd.Format("2006-01-02")
	meetingMetadata.ChangeMetadata.Schedule.EndTime = metadata.ImplementationEnd.Format("15:04:05")
	meetingMetadata.ChangeMetadata.Schedule.Timezone = metadata.Timezone

	if meetingMetadata.ChangeMetadata.Schedule.Timezone == "" {
		meetingMetadata.ChangeMetadata.Schedule.Timezone = "America/New_York"
	}

	// Parse meeting duration to get duration in minutes
	durationMinutes := 60 // default
	if meetingDuration != "" {
		if duration, err := time.ParseDuration(meetingDuration + "m"); err == nil {
			durationMinutes = int(duration.Minutes())
		}
	}

	// Create meeting invite structure
	// Parse the meeting start time
	meetingStartTimeStr := formatMeetingStartTime(meetingDate, metadata.ImplementationStart.Format("15:04:05"))
	meetingStartTime := parseTimeString(meetingStartTimeStr)

	meetingInvite := &struct {
		Title           string    `json:"title"`
		StartTime       time.Time `json:"startTime"`
		Duration        int       `json:"duration"`
		DurationMinutes int       `json:"durationMinutes"`
		Attendees       []string  `json:"attendees"`
		Location        string    `json:"location"`
	}{
		Title:           meetingTitle,
		StartTime:       meetingStartTime,
		Duration:        durationMinutes,
		DurationMinutes: durationMinutes,
		Location:        meetingLocation,
	}

	meetingMetadata.MeetingInvite = meetingInvite

	// Create temporary file in /tmp (only writable directory in Lambda)
	tempFileName := fmt.Sprintf("/tmp/meeting-metadata-%s-%d.json", metadata.ChangeID, time.Now().Unix())

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(meetingMetadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal meeting metadata: %w", err)
	}

	// Write to temporary file in current directory
	err = os.WriteFile(tempFileName, jsonData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write temporary meeting metadata file: %w", err)
	}

	log.Printf("📄 Created temporary meeting metadata file: %s", tempFileName)
	return tempFileName, nil
}

// formatMeetingStartTime properly formats meeting start time from date and time components using centralized datetime utilities
func formatMeetingStartTime(meetingDate, implementationBeginTime string) string {
	dtManager := datetime.New(nil)

	// If meetingDate already contains a full timestamp, parse and format it
	if strings.Contains(meetingDate, "T") {
		parsed, err := dtManager.Parse(meetingDate)
		if err == nil {
			return dtManager.Format(parsed).ToRFC3339()
		}
		// Fallback to original logic if parsing fails
		if strings.Count(meetingDate, ":") >= 2 {
			return meetingDate
		}
		return meetingDate + ":00"
	}

	// If meetingDate is just a date, combine with implementationBeginTime
	if implementationBeginTime == "" {
		implementationBeginTime = "10:00" // default time
	}

	// Parse date and time separately, then combine
	dateTime := fmt.Sprintf("%s %s", meetingDate, implementationBeginTime)
	parsed, err := dtManager.Parse(dateTime)
	if err == nil {
		return dtManager.Format(parsed).ToRFC3339()
	}

	// Fallback to original logic if parsing fails
	timePart := implementationBeginTime
	if strings.Count(timePart, ":") == 1 {
		timePart += ":00"
	}
	return fmt.Sprintf("%sT%s", meetingDate, timePart)
}

// StartLambdaMode starts the Lambda handler
func StartLambdaMode() {
	// Log version information at Lambda startup
	log.Printf("CCOE Customer Contact Manager Lambda v%s (commit: %s, built: %s)",
		getVersion(), getGitCommit(), getBuildTime())

	lambda.Start(Handler)
}

// Version information - these will be set by main package
var (
	version   = "unknown"
	gitCommit = "unknown"
	buildTime = "unknown"
)

// SetVersionInfo sets the version information from main package
func SetVersionInfo(v, commit, build string) {
	version = v
	gitCommit = commit
	buildTime = build
}

func getVersion() string   { return version }
func getGitCommit() string { return gitCommit }
func getBuildTime() string { return buildTime }

// SQSProcessor handles SQS message processing
type SQSProcessor struct {
	queueURL          string
	credentialManager CredentialManager
	emailManager      EmailManager
	sqsClient         interface{} // Will be *sqs.Client in real implementation
	s3Client          interface{} // Will be *s3.Client in real implementation
}

// CredentialManager interface for dependency injection
type CredentialManager interface {
	GetCustomerConfig(customerCode string) (aws.Config, error)
	GetCustomerInfo(customerCode string) (types.CustomerAccountInfo, error)
}

// EmailManager interface for dependency injection
type EmailManager interface {
	SendAlternateContactNotification(customerCode string, changeDetails map[string]interface{}) error
}

// NewSQSProcessor creates a new SQS processor
func NewSQSProcessor(queueURL string, credentialManager CredentialManager, emailManager EmailManager, region string) (*SQSProcessor, error) {
	return &SQSProcessor{
		queueURL:          queueURL,
		credentialManager: credentialManager,
		emailManager:      emailManager,
	}, nil
}

// ProcessMessages processes messages from the SQS queue
func (sp *SQSProcessor) ProcessMessages(ctx context.Context) error {
	fmt.Printf("Starting SQS message processing from queue: %s\n", sp.queueURL)

	// This is a simplified implementation for the integration
	// The full implementation would include message polling and processing
	return fmt.Errorf("SQS processing not fully implemented in internal package yet")
}

// SendApprovalRequestEmail sends approval request email using new template system
func SendApprovalRequestEmail(ctx context.Context, customerCode string, changeDetails map[string]interface{}, cfg *types.Config) error {
	log.Printf("Sending approval request email for customer %s", customerCode)

	// Create credential manager to assume customer role
	credentialManager, err := awsinternal.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		return fmt.Errorf("failed to create credential manager: %w", err)
	}

	// Get customer-specific AWS config (assumes SES role)
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config for %s: %w", customerCode, err)
	}

	// Create SES client with assumed role credentials
	sesClient := sesv2.NewFromConfig(customerConfig)

	// Configuration for approval request
	topicName := "aws-approval"

	changeID := "unknown"
	if id, ok := changeDetails["change_id"].(string); ok && id != "" {
		changeID = id
	}
	log.Printf("📧 Sending approval request email for change %s", changeID)

	// Convert changeDetails to ChangeMetadata format
	metadata := createChangeMetadataFromChangeDetails(changeDetails)

	// Send approval request email using new template system
	err = sendChangeEmailWithTemplate(ctx, sesClient, topicName, metadata, cfg, "approval_request")
	if err != nil {
		log.Printf("❌ Failed to send approval request email: %v", err)
		return fmt.Errorf("failed to send approval request email: %w", err)
	}

	// Get topic subscriber count for logging
	subscriberCount, err := getTopicSubscriberCount(sesClient, topicName)
	if err != nil {
		log.Printf("⚠️  Could not get subscriber count: %v", err)
		subscriberCount = "unknown"
	}

	log.Printf("✅ Approval request email sent to %s members of topic %s", subscriberCount, topicName)
	return nil
}

// SendApprovedAnnouncementEmail sends approved announcement email using existing SES template system
func SendApprovedAnnouncementEmail(ctx context.Context, customerCode string, changeDetails map[string]interface{}, cfg *types.Config) error {
	log.Printf("Sending approved announcement email for customer %s", customerCode)

	// Create credential manager to assume customer role
	credentialManager, err := awsinternal.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		return fmt.Errorf("failed to create credential manager: %w", err)
	}

	// Get customer-specific AWS config (assumes SES role)
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config for %s: %w", customerCode, err)
	}

	// Create SES client with assumed role credentials
	sesClient := sesv2.NewFromConfig(customerConfig)

	// Configuration for approved announcement
	topicName := "aws-announce"

	changeID := "unknown"
	if id, ok := changeDetails["change_id"].(string); ok && id != "" {
		changeID = id
	}
	log.Printf("📧 Sending approved announcement email for change %s", changeID)

	// Convert changeDetails to ChangeMetadata format for SES functions
	metadata := createChangeMetadataFromChangeDetails(changeDetails)

	// Populate ApprovedAt and ApprovedBy from modifications if not already set
	if (metadata.ApprovedAt == nil || metadata.ApprovedAt.IsZero()) && len(metadata.Modifications) > 0 {
		for _, mod := range metadata.Modifications {
			if mod.ModificationType == types.ModificationTypeApproved {
				metadata.ApprovedAt = &mod.Timestamp
				if metadata.ApprovedBy == "" {
					metadata.ApprovedBy = mod.UserID
				}
				break
			}
		}
	}

	// Send approved announcement email using new template system
	err = sendChangeEmailWithTemplate(ctx, sesClient, topicName, metadata, cfg, "approved")
	if err != nil {
		log.Printf("❌ Failed to send approved announcement email: %v", err)
		return fmt.Errorf("failed to send approved announcement email: %w", err)
	}

	// Get topic subscriber count for logging
	subscriberCount, err := getTopicSubscriberCount(sesClient, topicName)
	if err != nil {
		log.Printf("⚠️  Could not get subscriber count: %v", err)
		subscriberCount = "unknown"
	}

	log.Printf("✅ Approved announcement email sent to %s members of topic %s", subscriberCount, topicName)
	return nil
}

// SendChangeCompleteEmail sends change complete notification email using existing SES template system
func SendChangeCompleteEmail(ctx context.Context, customerCode string, changeDetails map[string]interface{}, cfg *types.Config) error {
	log.Printf("Sending change complete notification email for customer %s", customerCode)

	// Create credential manager to assume customer role
	credentialManager, err := awsinternal.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		return fmt.Errorf("failed to create credential manager: %w", err)
	}

	// Get customer-specific AWS config (assumes SES role)
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config for %s: %w", customerCode, err)
	}

	// Create SES client with assumed role credentials
	sesClient := sesv2.NewFromConfig(customerConfig)

	// Configuration for change complete notification
	topicName := "aws-announce" // Use announce topic for completion notifications

	changeID := "unknown"
	if id, ok := changeDetails["change_id"].(string); ok && id != "" {
		changeID = id
	}
	log.Printf("📧 Sending change complete notification email for change %s", changeID)

	// Convert changeDetails to ChangeMetadata format for SES functions
	metadata := createChangeMetadataFromChangeDetails(changeDetails)

	// Send change complete email using new template system
	err = sendChangeEmailWithTemplate(ctx, sesClient, topicName, metadata, cfg, "completed")
	if err != nil {
		log.Printf("❌ Failed to send change complete email: %v", err)
		return fmt.Errorf("failed to send change complete email: %w", err)
	}

	// Get topic subscriber count for logging
	subscriberCount, err := getTopicSubscriberCount(sesClient, topicName)
	if err != nil {
		log.Printf("⚠️  Could not get subscriber count: %v", err)
		subscriberCount = "unknown"
	}

	log.Printf("✅ Change complete notification email sent to %s members of topic %s", subscriberCount, topicName)
	return nil
}

// SendChangeCancelledEmail sends change cancelled notification email using existing SES template system
func SendChangeCancelledEmail(ctx context.Context, customerCode string, changeDetails map[string]interface{}, cfg *types.Config) error {
	log.Printf("Sending change cancelled notification email for customer %s", customerCode)

	// Create credential manager to assume customer role
	credentialManager, err := awsinternal.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		return fmt.Errorf("failed to create credential manager: %w", err)
	}

	// Get customer-specific AWS config (assumes SES role)
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config for %s: %w", customerCode, err)
	}

	// Create SES client with assumed role credentials
	sesClient := sesv2.NewFromConfig(customerConfig)

	changeID := "unknown"
	if id, ok := changeDetails["change_id"].(string); ok && id != "" {
		changeID = id
	}

	// Convert changeDetails to ChangeMetadata format for SES functions
	metadata := createChangeMetadataFromChangeDetails(changeDetails)

	// Determine topic based on whether change was approved
	// If approved, send to aws-announce (broader audience)
	// If not approved (submitted/waiting), send to aws-approval (approval team only)
	topicName := "aws-approval" // Default to approval topic
	wasApproved := false

	// Check if change was ever approved by looking at modifications array
	// Also populate ApprovedAt and ApprovedBy from the modification entry
	if len(metadata.Modifications) > 0 {
		for _, mod := range metadata.Modifications {
			if mod.ModificationType == types.ModificationTypeApproved {
				wasApproved = true
				// Populate ApprovedAt and ApprovedBy from modification entry if not already set
				if metadata.ApprovedAt == nil || metadata.ApprovedAt.IsZero() {
					metadata.ApprovedAt = &mod.Timestamp
				}
				if metadata.ApprovedBy == "" {
					metadata.ApprovedBy = mod.UserID
				}
				break
			}
		}
	}

	// Also check the status and approvedAt fields as fallback
	if !wasApproved {
		if metadata.Status == "approved" {
			wasApproved = true
		} else if metadata.ApprovedAt != nil && !metadata.ApprovedAt.IsZero() {
			wasApproved = true
		} else if metadata.ApprovedBy != "" {
			wasApproved = true
		}
	}

	if wasApproved {
		topicName = "aws-announce"
		log.Printf("📧 Change %s was approved - sending cancellation to aws-announce topic", changeID)
	} else {
		log.Printf("📧 Change %s was not approved - sending cancellation to aws-approval topic", changeID)
	}

	log.Printf("📧 Sending change cancelled notification email for change %s to topic %s", changeID, topicName)

	// Send change cancelled email using new template system
	err = sendChangeEmailWithTemplate(ctx, sesClient, topicName, metadata, cfg, "cancelled")
	if err != nil {
		log.Printf("❌ Failed to send change cancelled email: %v", err)
		return fmt.Errorf("failed to send change cancelled email: %w", err)
	}

	// Get topic subscriber count for logging
	subscriberCount, err := getTopicSubscriberCount(sesClient, topicName)
	if err != nil {
		log.Printf("⚠️  Could not get subscriber count: %v", err)
		subscriberCount = "unknown"
	}

	log.Printf("✅ Change cancelled notification email sent to %s members of topic %s", subscriberCount, topicName)
	return nil
}

// generateApprovalRequestHTML generates HTML content for approval request emails
func generateApprovalRequestHTML(metadata *types.ChangeMetadata) string {
	// Use centralized timezone formatting function
	formatDateTime := func(t time.Time) string {
		return formatDateTimeWithTimezone(t, metadata.Timezone)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Approval Request</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #007bff; }
        .section { margin-bottom: 25px; }
        .section h3 { margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px; color: #007bff; }
        .info-grid { display: grid; grid-template-columns: 150px 1fr; gap: 10px; margin-bottom: 15px; }
        .info-label { font-weight: bold; color: #495057; }
        .schedule { background-color: #e7f3ff; padding: 15px; border-radius: 5px; margin: 15px 0; border-left: 4px solid #007bff; }
        .tickets { background-color: #f8f9fa; padding: 10px; border-radius: 5px; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .unsubscribe-prominent { margin-top: 10px; }
        .unsubscribe-prominent a { color: #007bff; text-decoration: none; font-weight: bold; }
        .approval-banner { background: linear-gradient(135deg, #007bff, #0056b3); color: white; padding: 25px; border-radius: 10px; margin-bottom: 25px; text-align: center; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .approval-banner h2 { margin: 0 0 10px 0; font-size: 28px; font-weight: bold; }
        .approval-banner p { margin: 0; font-size: 16px; opacity: 0.95; }
        .meeting-details { background-color: #e7f3ff; padding: 15px; border-radius: 5px; margin: 15px 0; border-left: 4px solid #007bff; }
    </style>
</head>
<body>
    <div class="approval-banner">
        <h2>❓ CHANGE APPROVAL REQUEST</h2>
        <p>This change has been reviewed, tentatively scheduled, and is ready for your approval.<br>A notification and calendar invite will be sent after final approval is received!</p>
    </div>
   
    <div class="header">
        <h2>📋 Change Details</h2>
        <p><strong>%s</strong></p>
        <p>Customer: %s</p>
    </div>

    <div class="section">
        <h3>📋 Change Information</h3>       
        <div class="tickets">
            ServiceNow: %s<br>
            JIRA: %s
        </div>
    </div>
   
    <div class="section">
        <h3>📅 Proposed Implementation Schedule</h3>
        <div class="schedule">
            <strong>🕐 Start:</strong> %s<br>
            <strong>🕐 End:</strong> %s<br>
            <strong>🌍 Timezone:</strong> %s
        </div>
    </div>
   
    <div class="section">
        <h3>📝 Change Reason</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>🔧 Implementation Plan</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>🧪 Test Plan</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>👥 Expected Customer Impact</h3>
        <p>%s</p>
    </div>

    <div class="section">
        <h3>🔄 Rollback Plan</h3>
        <p>%s</p>
    </div>
    
    <div class="section" style="background-color: #d1ecf1; border-left: 4px solid #bee5eb;">
        <h3>✅ Action Required</h3>
        <p>Please review this change request and provide your approval or feedback.</p>
    </div>
    
    <div class="unsubscribe" style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px;">
        <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
        <p>Generated at: %s</p>
        <div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeTitle,
		strings.Join(getCustomerNames(metadata.Customers), ", "),
		metadata.SnowTicket,
		metadata.JiraTicket,
		formatDateTime(metadata.ImplementationStart),
		formatDateTime(metadata.ImplementationEnd),
		metadata.Timezone,
		metadata.ChangeReason,
		strings.ReplaceAll(metadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.TestPlan, "\n", "<br>"),
		metadata.CustomerImpact,
		strings.ReplaceAll(metadata.RollbackPlan, "\n", "<br>"),
		formatDateTime(time.Now()), // Use current time for "Generated at"
	)
}

// generateAnnouncementHTML generates HTML content for approved announcement emails
func generateAnnouncementHTML(metadata *types.ChangeMetadata) string {
	// Use centralized timezone formatting function
	formatDateTime := func(t time.Time) string {
		return formatDateTimeWithTimezone(t, metadata.Timezone)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Approved & Scheduled</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #007bff; }
        .section { margin-bottom: 25px; }
        .section h3 { margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px; color: #007bff; }
        .schedule { background-color: #e7f3ff; padding: 15px; border-radius: 5px; margin: 15px 0; border-left: 4px solid #007bff; }
        .tickets { background-color: #f8f9fa; padding: 10px; border-radius: 5px; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .unsubscribe-prominent { margin-top: 10px; }
        .unsubscribe-prominent a { color: #007bff; text-decoration: none; font-weight: bold; }
        .approval-banner { background: linear-gradient(135deg, #28a745, #20c997); color: white; padding: 25px; border-radius: 10px; margin-bottom: 25px; text-align: center; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .header { background-color: #d4edda; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #28a745; }
        .section h3 { margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px; color: #28a745; }
        .schedule { background-color: #d1ecf1; padding: 15px; border-radius: 5px; margin: 15px 0; border-left: 4px solid #28a745; }
        .approval-banner h2 { margin: 0 0 10px 0; font-size: 28px; font-weight: bold; }
        .approval-banner p { margin: 0; font-size: 16px; opacity: 0.95; }
    </style>
</head>
<body>
    <div class="approval-banner">
        <h2>✅ CHANGE APPROVED & SCHEDULED</h2>
        <p>This change has been approved and is scheduled for implementation during the specified window.</p>
    </div>
    
    <div class="section approved">
        <h3>📋 Change Details</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer(s):</strong> %s</p>
        <p><strong>Status:</strong> <span style="color: #28a745; font-weight: bold;">✅ APPROVED</span></p>
        <p><strong>Approved By:</strong> %s</p>
        <p><strong>Approved At:</strong> %s</p>
    </div>
    
    <div class="section">
        <h3>📝 Content</h3>
        <div class="highlight">%s</div>
    </div>
    
    <div class="section">
        <h3>🔧 Implementation Plan</h3>
        <div class="highlight">%s</div>
    </div>
    
    <div class="section">
        <h3>🧪 Test Plan</h3>
        <div class="highlight">%s</div>
    </div>
    
    <div class="section">
        <h3>📅 Scheduled Implementation</h3>
        <p><strong>Implementation Window:</strong> %s to %s</p>
        <p><strong>Timezone:</strong> %s</p>
    </div>
    
    <div class="section">
        <h3>⚠️ Expected Impact</h3>
        <p>%s</p>
    </div>
    
    <div class="section">
        <h3>🔄 Rollback Plan</h3>
        <p>%s</p>
    </div>
    
    <div class="section">
        <h3>🎫 Related Tickets</h3>
        <div class="ticket"><strong>ServiceNow:</strong> %s</div>
        <div class="ticket"><strong>Jira:</strong> %s</div>
    </div>
    
    <div class="section" style="background-color: #d4edda; padding: 15px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #28a745;">
        <h3>📢 Next Steps</h3>
        <p>Implementation will proceed as scheduled. You will receive at least one additional update once the change is complete.</p>
    </div>
    
    <div class="unsubscribe" style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px;">
        <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
        <p>Generated at: %s</p>
        <div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #28a745; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeTitle,
		strings.Join(getCustomerNames(metadata.Customers), ", "),
		metadata.ApprovedBy,
		func() string {
			if metadata.ApprovedAt != nil {
				return formatDateTime(*metadata.ApprovedAt)
			}
			return "N/A"
		}(),
		formatTextForHTML(metadata.ChangeReason),
		formatTextForHTML(metadata.ImplementationPlan),
		formatTextForHTML(metadata.TestPlan),
		formatDateTime(metadata.ImplementationStart),
		formatDateTime(metadata.ImplementationEnd),
		metadata.Timezone,
		formatTextForHTML(metadata.CustomerImpact),
		formatTextForHTML(metadata.RollbackPlan),
		metadata.SnowTicket,
		metadata.JiraTicket,
		formatDateTime(time.Now()), // Use current time for "Generated at"
	)
}

// generateChangeCompleteHTML generates HTML content for change complete notification emails (short and sweet)
func generateChangeCompleteHTML(metadata *types.ChangeMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Complete</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .complete-banner { background: linear-gradient(135deg, #28a745, #20c997); color: white; padding: 25px; border-radius: 10px; margin-bottom: 25px; text-align: center; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .complete-banner h2 { margin: 0 0 10px 0; font-size: 28px; font-weight: bold; }
        .complete-banner p { margin: 0; font-size: 16px; opacity: 0.95; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .unsubscribe-prominent { margin-top: 10px; }
        .unsubscribe-prominent a { color: #28a745; text-decoration: none; font-weight: bold; }
    </style>
</head>
<body>
    <div class="complete-banner">
        <h2>🎯 CHANGE COMPLETED</h2>
        <p>The scheduled change has been successfully completed.</p>
    </div>
    
    <div class="section">
        <h3>📋 Change Summary</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer(s):</strong> %s</p>
        <p><strong>Status:</strong> <span style="color: #28a745; font-weight: bold;">✅ COMPLETED</span></p>
    </div>
    
    <div class="unsubscribe">
        <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
        <p>Notification sent at: %s</p>
        <div class="unsubscribe-prominent"><a href="{{amazonSESUnsubscribeUrl}}">📧 Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeTitle,
		strings.Join(getCustomerNames(metadata.Customers), ", "),
		datetime.New(nil).Format(time.Now()).ToHumanReadable(""),
	)
}

// generateChangeCancelledHTML generates HTML content for change cancelled notification emails
func generateChangeCancelledHTML(metadata *types.ChangeMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Cancelled</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .cancelled-banner { background: linear-gradient(135deg, #dc3545, #c82333); color: white; padding: 25px; border-radius: 10px; margin-bottom: 25px; text-align: center; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .cancelled-banner h2 { margin: 0 0 10px 0; font-size: 28px; font-weight: bold; }
        .cancelled-banner p { margin: 0; font-size: 16px; opacity: 0.95; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .unsubscribe-prominent { margin-top: 10px; }
        .unsubscribe-prominent a { color: #dc3545; text-decoration: none; font-weight: bold; }
    </style>
</head>
<body>
    <div class="cancelled-banner">
        <h2>❌ CHANGE CANCELLED</h2>
        <p>The scheduled change has been cancelled.</p>
    </div>
    
    <div class="section">
        <h3>📋 Change Summary</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer(s):</strong> %s</p>
        <p><strong>Status:</strong> <span style="color: #dc3545; font-weight: bold;">❌ CANCELLED</span></p>
    </div>
    
    <div class="unsubscribe">
        <p>This is an automated notification from the CCOE Customer Contact Manager.</p>
        <p>Notification sent at: %s</p>
        <div class="unsubscribe-prominent"><a href="{{amazonSESUnsubscribeUrl}}">📧 Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeTitle,
		strings.Join(getCustomerNames(metadata.Customers), ", "),
		datetime.New(nil).Format(time.Now()).ToHumanReadable(""),
	)
}

// shouldSendToRecipient checks if an email should be sent to a recipient based on restricted_recipients config
func shouldSendToRecipient(email string) bool {
	// If no customer info is set, allow all (shouldn't happen, but safe default)
	if currentCustomerInfo == nil {
		return true
	}

	// Use the IsRecipientAllowed method from CustomerAccountInfo
	return currentCustomerInfo.IsRecipientAllowed(email)
}

// sendEmailToTopic sends an email to all subscribers of a specific SES topic
func sendEmailToTopic(ctx context.Context, sesClient *sesv2.Client, topicName, subject, htmlContent string) error {
	// Get the account's main contact list
	accountListName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts subscribed to the topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s'", topicName)
		return nil
	}

	log.Printf("📧 Sending email to topic '%s' (%d subscribers)", topicName, len(subscribedContacts))

	// Default sender email - CCOE email address
	senderEmail := defaultSenderEmail

	// Send email to each subscribed contact
	successCount := 0
	errorCount := 0

	for _, contact := range subscribedContacts {
		// Check if recipient is allowed based on restricted_recipients config
		if !shouldSendToRecipient(*contact.EmailAddress) {
			log.Printf("⏭️  Skipping %s (not on restricted recipient list)", *contact.EmailAddress)
			continue
		}

		sendInput := &sesv2.SendEmailInput{
			FromEmailAddress: aws.String(senderEmail),
			Destination: &sesv2Types.Destination{
				ToAddresses: []string{*contact.EmailAddress},
			},
			Content: &sesv2Types.EmailContent{
				Simple: &sesv2Types.Message{
					Subject: &sesv2Types.Content{
						Data: aws.String(subject),
					},
					Body: &sesv2Types.Body{
						Html: &sesv2Types.Content{
							Data: aws.String(htmlContent),
						},
					},
				},
			},
			ListManagementOptions: &sesv2Types.ListManagementOptions{
				ContactListName: aws.String(accountListName),
				TopicName:       aws.String(topicName),
			},
		}

		_, err := sesClient.SendEmail(ctx, sendInput)
		if err != nil {
			log.Printf("❌ Failed to send email to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			log.Printf("✅ Sent email to %s", *contact.EmailAddress)
			successCount++
		}
	}

	log.Printf("📊 Email Summary: %d successful, %d errors", successCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("failed to send email to %d recipients", errorCount)
	}

	return nil
}

// getSubscribedContactsForTopic gets all contacts that should receive emails for a topic
func getSubscribedContactsForTopic(sesClient *sesv2.Client, listName string, topicName string) ([]sesv2Types.Contact, error) {
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	return contactsResult.Contacts, nil
}

// getTopicSubscriberCount gets the number of subscribers for a topic
func getTopicSubscriberCount(sesClient *sesv2.Client, topicName string) (string, error) {
	// Get the account's main contact list
	accountListName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		return "unknown", fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get subscribed contacts for the topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return "unknown", fmt.Errorf("failed to get subscribed contacts: %w", err)
	}

	return fmt.Sprintf("%d", len(subscribedContacts)), nil
}

// sendApprovalRequestEmailDirect sends approval request email directly using SES without file path issues
func sendApprovalRequestEmailDirect(sesClient *sesv2.Client, topicName, senderEmail string, metadata *types.ChangeMetadata) error {
	// Get account contact list
	accountListName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts that should receive emails for this topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s'", topicName)
		return nil
	}

	// Generate HTML content for approval request
	htmlContent := generateApprovalRequestHTML(metadata)

	// Create subject
	subject := fmt.Sprintf("❓ APPROVAL REQUEST: %s", metadata.ChangeTitle)

	log.Printf("📧 Sending approval request to topic '%s' (%d subscribers)", topicName, len(subscribedContacts))

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(htmlContent),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	successCount := 0
	errorCount := 0
	skippedCount := 0

	// Send to each subscribed contact
	for _, contact := range subscribedContacts {
		// Check if recipient is allowed based on restricted_recipients config
		if !shouldSendToRecipient(*contact.EmailAddress) {
			log.Printf("   ⏭️  Skipping %s (not on restricted recipient list)", *contact.EmailAddress)
			skippedCount++
			continue
		}

		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			log.Printf("   ❌ Failed to send to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			log.Printf("   ✅ Sent to %s", *contact.EmailAddress)
			successCount++
		}
	}

	if skippedCount > 0 {
		log.Printf("📊 Approval Request Summary: ✅ %d successful, ❌ %d errors, ⏭️  %d skipped", successCount, errorCount, skippedCount)
	} else {
		log.Printf("📊 Approval Request Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)
	}

	if errorCount > 0 {
		return fmt.Errorf("failed to send approval request to %d recipients", errorCount)
	}

	return nil
}

// sendApprovedAnnouncementEmailDirect sends approved announcement email directly using SES without file path issues
func sendApprovedAnnouncementEmailDirect(sesClient *sesv2.Client, topicName, senderEmail string, metadata *types.ChangeMetadata) error {
	// Get account contact list
	accountListName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts that should receive emails for this topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s'", topicName)
		return nil
	}

	// Generate HTML content for approved announcement
	htmlContent := generateAnnouncementHTML(metadata)

	// Create subject with "APPROVED" prefix
	originalSubject := fmt.Sprintf("CCOE Change: %s", metadata.ChangeTitle)
	subject := fmt.Sprintf("✅ APPROVED %s", originalSubject)

	log.Printf("📧 Sending approved announcement to topic '%s' (%d subscribers)", topicName, len(subscribedContacts))

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(htmlContent),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	successCount := 0
	errorCount := 0
	skippedCount := 0

	// Send to each subscribed contact
	for _, contact := range subscribedContacts {
		// Check if recipient is allowed based on restricted_recipients config
		if !shouldSendToRecipient(*contact.EmailAddress) {
			log.Printf("   ⏭️  Skipping %s (not on restricted recipient list)", *contact.EmailAddress)
			skippedCount++
			continue
		}

		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			log.Printf("   ❌ Failed to send to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			log.Printf("   ✅ Sent to %s", *contact.EmailAddress)
			successCount++
		}
	}

	if skippedCount > 0 {
		log.Printf("📊 Approved Announcement Summary: ✅ %d successful, ❌ %d errors, ⏭️  %d skipped", successCount, errorCount, skippedCount)
	} else {
		log.Printf("📊 Approved Announcement Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)
	}

	if errorCount > 0 {
		return fmt.Errorf("failed to send approved announcement to %d recipients", errorCount)
	}

	return nil
}

// sendChangeCompleteEmailDirect sends change complete notification email directly using SES
func sendChangeCompleteEmailDirect(sesClient *sesv2.Client, topicName, senderEmail string, metadata *types.ChangeMetadata) error {
	// Get account contact list
	accountListName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts that should receive emails for this topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s'", topicName)
		return nil
	}

	// Generate HTML content for change complete notification (short and sweet)
	htmlContent := generateChangeCompleteHTML(metadata)

	// Create subject for completion notification
	subject := fmt.Sprintf("🎯 COMPLETED: %s", metadata.ChangeTitle)

	log.Printf("📧 Sending change complete notification to topic '%s' (%d subscribers)", topicName, len(subscribedContacts))
	log.Printf("📤 Using sender address: %s", senderEmail)

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(htmlContent),
					},
				},
			},
		},
	}

	// Send to each subscribed contact
	successCount := 0
	errorCount := 0
	skippedCount := 0

	for _, contact := range subscribedContacts {
		// Check if recipient is allowed based on restricted_recipients config
		if !shouldSendToRecipient(*contact.EmailAddress) {
			log.Printf("⏭️  Skipping %s (not on restricted recipient list)", *contact.EmailAddress)
			skippedCount++
			continue
		}

		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			log.Printf("❌ Failed to send change complete notification to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			log.Printf("✅ Sent change complete notification to %s", *contact.EmailAddress)
			successCount++
		}
	}

	if skippedCount > 0 {
		log.Printf("📊 Change Complete Summary: ✅ %d successful, ❌ %d errors, ⏭️  %d skipped", successCount, errorCount, skippedCount)
	} else {
		log.Printf("📊 Change Complete Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)
	}

	if errorCount > 0 {
		return fmt.Errorf("failed to send change complete notification to %d recipients", errorCount)
	}

	return nil
}

// sendChangeCancelledEmailDirect sends change cancelled notification email directly using SES
func sendChangeCancelledEmailDirect(sesClient *sesv2.Client, topicName, senderEmail string, metadata *types.ChangeMetadata) error {
	// Get account contact list
	accountListName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts that should receive emails for this topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s'", topicName)
		return nil
	}

	// Generate HTML content for change cancelled notification
	htmlContent := generateChangeCancelledHTML(metadata)

	// Create subject for cancellation notification
	subject := fmt.Sprintf("❌ CANCELLED: %s", metadata.ChangeTitle)

	log.Printf("📧 Sending change cancelled notification to topic '%s' (%d subscribers)", topicName, len(subscribedContacts))
	log.Printf("📤 Using sender address: %s", senderEmail)

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(htmlContent),
					},
				},
			},
		},
	}

	// Send to each subscribed contact
	successCount := 0
	errorCount := 0
	skippedCount := 0

	for _, contact := range subscribedContacts {
		// Check if recipient is allowed based on restricted_recipients config
		if !shouldSendToRecipient(*contact.EmailAddress) {
			log.Printf("⏭️  Skipping %s (not on restricted recipient list)", *contact.EmailAddress)
			skippedCount++
			continue
		}

		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			log.Printf("❌ Failed to send change cancelled notification to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			log.Printf("✅ Sent change cancelled notification to %s", *contact.EmailAddress)
			successCount++
		}
	}

	if skippedCount > 0 {
		log.Printf("📊 Change Cancelled Summary: ✅ %d successful, ❌ %d errors, ⏭️  %d skipped", successCount, errorCount, skippedCount)
	} else {
		log.Printf("📊 Change Cancelled Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)
	}

	if errorCount > 0 {
		return fmt.Errorf("failed to send change cancelled notification to %d recipients", errorCount)
	}

	return nil
}

// Helper functions for email templates

// extractTypeHeader extracts the type-specific header banner from announcement type
func extractTypeHeader(announcementType string) string {
	switch strings.ToLower(announcementType) {
	case "cic":
		return `<div class="cic-header"><h1>☁️ Cloud Innovator Community</h1></div>`
	case "finops":
		return `<div class="finops-header"><h1>💰 Cloud FinOps</h1></div>`
	case "innersource":
		return `<div class="innersource-header"><h1>🔧 Innersource Guild</h1></div>`
	default:
		return `<div class="header"><h1>📢 Announcement</h1></div>`
	}
}

// getTypeStyles returns CSS styles for the announcement type
func getTypeStyles(announcementType string) string {
	switch strings.ToLower(announcementType) {
	case "cic":
		return `.cic-header { background-color: #0066cc; color: white; padding: 20px; }
		.cic-content { padding: 20px; }
		.meeting-info { background-color: #f0f8ff; padding: 15px; margin: 20px 0; border-left: 4px solid #0066cc; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #0066cc; text-decoration: none; }
		a:hover { text-decoration: underline; }`
	case "finops":
		return `.finops-header { background-color: #28a745; color: white; padding: 20px; }
		.finops-content { padding: 20px; }
		.cost-highlight { background-color: #d4edda; padding: 10px; margin: 10px 0; border-left: 4px solid #28a745; }
		.meeting-info { background-color: #d4edda; padding: 15px; margin: 20px 0; border-left: 4px solid #28a745; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #28a745; text-decoration: none; }
		a:hover { text-decoration: underline; }`
	case "innersource":
		return `.innersource-header { background-color: #6f42c1; color: white; padding: 20px; }
		.innersource-content { padding: 20px; }
		.project-highlight { background-color: #e7d9f7; padding: 10px; margin: 10px 0; border-left: 4px solid #6f42c1; }
		.meeting-info { background-color: #e7d9f7; padding: 15px; margin: 20px 0; border-left: 4px solid #6f42c1; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #6f42c1; text-decoration: none; }
		a:hover { text-decoration: underline; }`
	default:
		return `.header { background-color: #007bff; color: white; padding: 20px; }
		.content { padding: 20px; }
		.meeting-info { background-color: #e7f3ff; padding: 15px; margin: 20px 0; border-left: 4px solid #007bff; }
		.attachments { margin: 20px 0; }
		.footer { background-color: #f5f5f5; padding: 15px; margin-top: 30px; font-size: 0.9em; color: #666; }
		a { color: #007bff; text-decoration: none; }
		a:hover { text-decoration: underline; }`
	}
}

// extractContentWithoutHeader extracts content from HTML body, removing the header div
func extractContentWithoutHeader(htmlBody string) string {
	// Remove the opening html, head, style, and body tags, and the header div
	// Keep only the content section and footer

	// Find the content section (after the header)
	contentStart := strings.Index(htmlBody, "<div class=\"cic-content\">")
	if contentStart == -1 {
		contentStart = strings.Index(htmlBody, "<div class=\"finops-content\">")
	}
	if contentStart == -1 {
		contentStart = strings.Index(htmlBody, "<div class=\"innersource-content\">")
	}
	if contentStart == -1 {
		contentStart = strings.Index(htmlBody, "<div class=\"content\">")
	}

	if contentStart == -1 {
		// Fallback: return body without header
		return htmlBody
	}

	// Find the end of body tag
	contentEnd := strings.LastIndex(htmlBody, "</body>")
	if contentEnd == -1 {
		contentEnd = len(htmlBody)
	}

	return htmlBody[contentStart:contentEnd]
}

// getCustomerNames converts customer codes to friendly names
func getCustomerNames(customerCodes []string) []string {
	customerMapping := map[string]string{
		"hts":         "HTS Prod",
		"htsnonprod":  "HTS NonProd",
		"cds":         "CDS Global",
		"fdbus":       "FDBUS",
		"hmiit":       "Hearst Magazines Italy",
		"hmies":       "Hearst Magazines Spain",
		"htvdigital":  "HTV Digital",
		"htv":         "HTV",
		"icx":         "iCrossing",
		"motor":       "Motor",
		"bat":         "Bring A Trailer",
		"mhk":         "MHK",
		"hdmautos":    "Autos",
		"hnpit":       "HNP IT",
		"hnpdigital":  "HNP Digital",
		"camp":        "CAMP Systems",
		"mcg":         "MCG",
		"hmuk":        "Hearst Magazines UK",
		"hmusdigital": "Hearst Magazines Digital",
		"hwp":         "Hearst Western Properties",
		"zynx":        "Zynx",
		"hchb":        "HCHB",
		"fdbuk":       "FDBUK",
		"hecom":       "Hearst ECommerce",
		"blkbook":     "Black Book",
	}

	var customerNames []string
	for _, code := range customerCodes {
		if name, exists := customerMapping[code]; exists {
			customerNames = append(customerNames, name)
		} else {
			customerNames = append(customerNames, code) // fallback to code if mapping not found
		}
	}
	return customerNames
}

// formatScheduleDateTime combines date and time for display using centralized datetime utilities
func formatScheduleDateTime(date, timeStr string) string {
	if date == "" || timeStr == "" {
		return "TBD"
	}

	dtManager := datetime.New(nil)

	// Parse date and time separately, then combine
	dateTime := fmt.Sprintf("%s %s", date, timeStr)
	parsed, err := dtManager.Parse(dateTime)
	if err == nil {
		return dtManager.Format(parsed).ToHumanReadable("")
	}

	// Fallback to original logic if parsing fails
	return fmt.Sprintf("%s at %s", date, timeStr)
}

// parseTimeString parses a time string and returns a time.Time
// Returns zero time if parsing fails
func parseTimeString(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}

	// Try multiple time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

// parseTimeStringPtr parses a time string and returns a *time.Time
// Returns nil if parsing fails or string is empty
func parseTimeStringPtr(timeStr string) *time.Time {
	if timeStr == "" {
		return nil
	}

	t := parseTimeString(timeStr)
	if t.IsZero() {
		return nil
	}

	return &t
}

// formatTimePtr formats a *time.Time to string, returns empty string if nil
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// MeetingScheduler handles Microsoft Graph meeting scheduling and S3 updates
type MeetingScheduler struct {
	s3UpdateManager *S3UpdateManager
	region          string
}

// NewMeetingScheduler creates a new MeetingScheduler
func NewMeetingScheduler(region string) *MeetingScheduler {
	s3Manager, err := NewS3UpdateManager(region)
	if err != nil {
		log.Printf("❌ CRITICAL: Failed to create S3UpdateManager: %v", err)
		log.Printf("❌ CRITICAL: Meeting metadata will NOT be written to S3!")
		log.Printf("❌ CRITICAL: This will prevent meeting cancellations from working!")
		// Still return a scheduler but with nil s3UpdateManager
		// The ScheduleMeetingWithMetadata function will log warnings
		return &MeetingScheduler{region: region}
	}

	log.Printf("✅ Successfully created S3UpdateManager for region: %s", region)
	return &MeetingScheduler{
		s3UpdateManager: s3Manager,
		region:          region,
	}
}

// ScheduleMeetingWithMetadata schedules a Microsoft Graph meeting and updates the change object with metadata
// Implements idempotency by checking for existing meetings and updating them instead of creating duplicates
func (ms *MeetingScheduler) ScheduleMeetingWithMetadata(ctx context.Context, changeMetadata *types.ChangeMetadata, s3Bucket, s3Key string) (*types.MeetingMetadata, error) {
	log.Printf("📅 Scheduling meeting for change %s with idempotency check", changeMetadata.ChangeID)

	// Check for existing meeting metadata to implement idempotency
	existingMeeting := changeMetadata.GetLatestMeetingMetadata()

	var meetingMetadata *types.MeetingMetadata
	var err error

	if existingMeeting != nil {
		log.Printf("🔍 Found existing meeting metadata: ID=%s, Subject=%s", existingMeeting.MeetingID, existingMeeting.Subject)

		// Check if meeting details have changed and need updating
		needsUpdate, updateReason := ms.checkIfMeetingNeedsUpdate(changeMetadata, existingMeeting)

		if needsUpdate {
			log.Printf("🔄 Meeting needs updating: %s", updateReason)
			meetingMetadata, err = ms.updateExistingGraphMeeting(ctx, changeMetadata, existingMeeting)
			if err != nil {
				log.Printf("⚠️  Failed to update existing meeting, creating new one: %v", err)
				// Fallback to creating new meeting if update fails
				meetingMetadata, err = ms.createGraphMeeting(ctx, changeMetadata)
				if err != nil {
					return nil, fmt.Errorf("failed to create fallback Microsoft Graph meeting: %w", err)
				}
				log.Printf("✅ Created fallback Microsoft Graph meeting: ID=%s", meetingMetadata.MeetingID)
			} else {
				log.Printf("✅ Updated existing Microsoft Graph meeting: ID=%s", meetingMetadata.MeetingID)
			}
		} else {
			log.Printf("✅ Existing meeting is up to date, no changes needed")
			return existingMeeting, nil
		}
	} else {
		log.Printf("🆕 No existing meeting found, creating new meeting")
		// Create new meeting when no existing meeting metadata exists
		meetingMetadata, err = ms.createGraphMeeting(ctx, changeMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed to create Microsoft Graph meeting: %w", err)
		}
		log.Printf("✅ Created new Microsoft Graph meeting: ID=%s", meetingMetadata.MeetingID)
	}

	// Update the change object in S3 with meeting metadata
	// NOTE: Only update the provided s3Key path (typically archive/).
	// The transient trigger pattern will handle propagating changes as needed.
	// Updating multiple paths causes duplicate modification entries.
	if ms.s3UpdateManager != nil {
		err = ms.s3UpdateManager.UpdateChangeObjectWithMeetingMetadata(ctx, s3Bucket, s3Key, meetingMetadata)
		if err != nil {
			log.Printf("❌ CRITICAL: Failed to update S3 object with meeting metadata: %v", err)
			log.Printf("❌ CRITICAL: Meeting was created but metadata NOT saved to S3!")
			log.Printf("❌ CRITICAL: Meeting ID: %s", meetingMetadata.MeetingID)
			log.Printf("❌ CRITICAL: S3 Location: s3://%s/%s", s3Bucket, s3Key)
			log.Printf("❌ CRITICAL: This will prevent meeting cancellations from working!")
			// Don't return error here - meeting was created/updated successfully
			// Log the issue but continue
		} else {
			log.Printf("✅ Successfully updated S3 object with meeting metadata")
			log.Printf("✅ Meeting ID: %s written to s3://%s/%s", meetingMetadata.MeetingID, s3Bucket, s3Key)
		}
	} else {
		log.Printf("❌ CRITICAL: S3UpdateManager not available, skipping S3 update")
		log.Printf("❌ CRITICAL: Meeting was created but metadata will NOT be saved to S3!")
		log.Printf("❌ CRITICAL: Meeting ID: %s", meetingMetadata.MeetingID)
		log.Printf("❌ CRITICAL: This will prevent meeting cancellations from working!")
	}

	return meetingMetadata, nil
}

// checkIfMeetingNeedsUpdate determines if an existing meeting needs to be updated
func (ms *MeetingScheduler) checkIfMeetingNeedsUpdate(changeMetadata *types.ChangeMetadata, existingMeeting *types.MeetingMetadata) (bool, string) {
	log.Printf("🔍 Checking if meeting needs update for change %s", changeMetadata.ChangeID)

	// Compare meeting title/subject
	expectedSubject := ""
	if strings.HasPrefix(changeMetadata.ObjectType, "announcement_") {
		// For announcements, use the announcement title directly
		expectedSubject = changeMetadata.ChangeTitle
	} else {
		// For changes, use "Change Implementation:" prefix
		expectedSubject = fmt.Sprintf("Change Implementation: %s", changeMetadata.ChangeTitle)
	}
	if changeMetadata.MeetingTitle != "" {
		expectedSubject = changeMetadata.MeetingTitle
	}

	if existingMeeting.Subject != expectedSubject {
		return true, fmt.Sprintf("subject changed: '%s' -> '%s'", existingMeeting.Subject, expectedSubject)
	}

	// Compare meeting times
	expectedStartTime := changeMetadata.ImplementationStart
	if changeMetadata.MeetingStartTime != nil && !changeMetadata.MeetingStartTime.IsZero() {
		expectedStartTime = *changeMetadata.MeetingStartTime
	}

	existingStartTime, err := time.Parse(time.RFC3339, existingMeeting.StartTime)
	if err != nil {
		log.Printf("⚠️  Failed to parse existing meeting start time: %v", err)
		return true, "failed to parse existing start time"
	}

	if !existingStartTime.Equal(expectedStartTime) {
		return true, fmt.Sprintf("start time changed: %s -> %s",
			existingStartTime.Format("2006-01-02 15:04:05 MST"),
			expectedStartTime.Format("2006-01-02 15:04:05 MST"))
	}

	// Compare meeting duration/end time
	expectedEndTime := changeMetadata.ImplementationEnd
	if changeMetadata.MeetingStartTime != nil && !changeMetadata.MeetingStartTime.IsZero() {
		// Calculate end time based on duration or default to 1 hour
		expectedEndTime = *changeMetadata.MeetingStartTime
		if changeMetadata.MeetingDuration != "" {
			// Parse duration (e.g., "60 minutes", "1 hour")
			if strings.Contains(changeMetadata.MeetingDuration, "hour") {
				expectedEndTime = expectedEndTime.Add(1 * time.Hour)
			} else if strings.Contains(changeMetadata.MeetingDuration, "minute") {
				// Extract number of minutes
				var minutes int
				fmt.Sscanf(changeMetadata.MeetingDuration, "%d", &minutes)
				expectedEndTime = expectedEndTime.Add(time.Duration(minutes) * time.Minute)
			} else {
				expectedEndTime = expectedEndTime.Add(1 * time.Hour) // Default to 1 hour
			}
		} else {
			expectedEndTime = expectedEndTime.Add(1 * time.Hour) // Default to 1 hour
		}
	}

	existingEndTime, err := time.Parse(time.RFC3339, existingMeeting.EndTime)
	if err != nil {
		log.Printf("⚠️  Failed to parse existing meeting end time: %v", err)
		return true, "failed to parse existing end time"
	}

	if !existingEndTime.Equal(expectedEndTime) {
		return true, fmt.Sprintf("end time changed: %s -> %s",
			existingEndTime.Format("2006-01-02 15:04:05 MST"),
			expectedEndTime.Format("2006-01-02 15:04:05 MST"))
	}

	log.Printf("✅ Meeting details are up to date, no update needed")
	return false, "meeting is up to date"
}

// updateExistingGraphMeeting updates an existing Microsoft Graph meeting
func (ms *MeetingScheduler) updateExistingGraphMeeting(ctx context.Context, changeMetadata *types.ChangeMetadata, existingMeeting *types.MeetingMetadata) (*types.MeetingMetadata, error) {
	log.Printf("🔄 Updating existing Microsoft Graph meeting: ID=%s", existingMeeting.MeetingID)

	// Create updated meeting metadata with the same meeting ID
	updatedMetadata := &types.MeetingMetadata{
		MeetingID: existingMeeting.MeetingID, // Keep the same meeting ID
		JoinURL:   existingMeeting.JoinURL,   // Keep the same join URL
		StartTime: changeMetadata.ImplementationStart.Format(time.RFC3339),
		EndTime:   changeMetadata.ImplementationEnd.Format(time.RFC3339),
		Organizer: existingMeeting.Organizer, // Keep existing organizer
		Attendees: existingMeeting.Attendees, // Keep existing attendees
	}

	// Set subject based on type
	if strings.HasPrefix(changeMetadata.ObjectType, "announcement_") {
		// For announcements, use the announcement title directly
		updatedMetadata.Subject = changeMetadata.ChangeTitle
	} else {
		// For changes, use "Change Implementation:" prefix
		updatedMetadata.Subject = fmt.Sprintf("Change Implementation: %s", changeMetadata.ChangeTitle)
	}

	// Use meeting title if specified
	if changeMetadata.MeetingTitle != "" {
		updatedMetadata.Subject = changeMetadata.MeetingTitle
	}

	// Use meeting start time if specified
	if changeMetadata.MeetingStartTime != nil && !changeMetadata.MeetingStartTime.IsZero() {
		updatedMetadata.StartTime = changeMetadata.MeetingStartTime.Format(time.RFC3339)

		// Calculate end time based on duration or default to 1 hour
		endTime := *changeMetadata.MeetingStartTime
		if changeMetadata.MeetingDuration != "" {
			// Parse duration (e.g., "60 minutes", "1 hour")
			if strings.Contains(changeMetadata.MeetingDuration, "hour") {
				endTime = endTime.Add(1 * time.Hour)
			} else if strings.Contains(changeMetadata.MeetingDuration, "minute") {
				// Extract number of minutes
				var minutes int
				fmt.Sscanf(changeMetadata.MeetingDuration, "%d", &minutes)
				endTime = endTime.Add(time.Duration(minutes) * time.Minute)
			} else {
				endTime = endTime.Add(1 * time.Hour) // Default to 1 hour
			}
		} else {
			endTime = endTime.Add(1 * time.Hour) // Default to 1 hour
		}

		updatedMetadata.EndTime = endTime.Format(time.RFC3339)
	}

	// Validate the updated meeting metadata
	if err := updatedMetadata.ValidateMeetingMetadata(); err != nil {
		return nil, fmt.Errorf("invalid updated meeting metadata: %w", err)
	}

	// In a full implementation, this would call the Microsoft Graph API to update the meeting
	// For now, we simulate the update by returning the updated metadata
	log.Printf("🔄 Simulating Microsoft Graph API call to update meeting %s", existingMeeting.MeetingID)
	log.Printf("📝 Updated meeting details: Subject=%s, Start=%s, End=%s",
		updatedMetadata.Subject, updatedMetadata.StartTime, updatedMetadata.EndTime)

	// TODO: Implement actual Microsoft Graph API call here
	// Example: PATCH https://graph.microsoft.com/v1.0/me/events/{meeting-id}

	log.Printf("✅ Successfully updated Microsoft Graph meeting: ID=%s", updatedMetadata.MeetingID)
	return updatedMetadata, nil
}

// createGraphMeeting creates a meeting using Microsoft Graph API by delegating to the SES package
// This function uses ses.CreateMultiCustomerMeetingFromChangeMetadata which handles:
// - Role assumption into each customer account
// - Querying aws-calendar topic subscribers from each customer's SES
// - Aggregating and deduplicating recipients
// - Creating the meeting via Microsoft Graph API
func (ms *MeetingScheduler) createGraphMeeting(ctx context.Context, changeMetadata *types.ChangeMetadata) (*types.MeetingMetadata, error) {
	log.Printf("🔄 Creating Microsoft Graph meeting for change %s", changeMetadata.ChangeID)

	// Get organizer email from environment or config
	organizerEmail := os.Getenv("MEETING_ORGANIZER_EMAIL")
	if organizerEmail == "" {
		organizerEmail = "ccoe@hearst.com" // Default organizer
		log.Printf("⚠️  MEETING_ORGANIZER_EMAIL not set, using default: %s", organizerEmail)
	}

	// Get the config to create credential manager
	cfg, err := ms.getConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// Create credential manager for multi-customer role assumption
	credentialManager, err := awsinternal.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential manager: %w", err)
	}

	// Use the SES function which handles all the complexity:
	// - Assumes roles into each customer account
	// - Queries topic subscribers from each customer's SES
	// - Aggregates and deduplicates recipients
	// - Creates meeting via Microsoft Graph API

	// Determine which topic to use based on object type
	var topicName string
	if strings.HasPrefix(changeMetadata.ObjectType, "announcement_") {
		// For announcements, use the announce topic (e.g., cic-announce, finops-announce)
		announcementType := strings.TrimPrefix(changeMetadata.ObjectType, "announcement_")
		// Map announcement types to their corresponding SES topics
		switch strings.ToLower(announcementType) {
		case "cic":
			topicName = "cic-announce"
		case "finops":
			topicName = "finops-announce"
		case "innersource", "inner":
			topicName = "inner-announce"
		case "general":
			topicName = "general-updates"
		default:
			log.Printf("⚠️  Unknown announcement type '%s', using general-updates topic", announcementType)
			topicName = "general-updates"
		}
		log.Printf("📢 Announcement detected, using topic: %s", topicName)
	} else {
		// For changes, use the calendar topic
		topicName = "aws-calendar"
		log.Printf("📋 Change detected, using topic: %s", topicName)
	}

	log.Printf("🚀 Calling ses.CreateMultiCustomerMeetingFromChangeMetadata for %d customers with topic %s", len(changeMetadata.Customers), topicName)

	graphMeetingID, err := ses.CreateMultiCustomerMeetingFromChangeMetadata(
		credentialManager,
		changeMetadata,
		topicName,
		organizerEmail,
		false, // not dry-run
		false, // not force-update
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create multi-customer meeting: %w", err)
	}

	log.Printf("✅ Successfully created multi-customer meeting with Graph ID: %s", graphMeetingID)

	// Get the full meeting details from Graph API to extract join URL
	accessToken, err := ses.GetGraphAccessToken()
	if err != nil {
		log.Printf("⚠️  Failed to get Graph access token for meeting details: %v", err)
		// Continue without join URL
		// Determine subject based on type
		subject := ""
		if strings.HasPrefix(changeMetadata.ObjectType, "announcement_") {
			subject = changeMetadata.ChangeTitle
		} else {
			subject = fmt.Sprintf("Change Implementation: %s", changeMetadata.ChangeTitle)
		}

		meetingMetadata := &types.MeetingMetadata{
			MeetingID: graphMeetingID,
			Subject:   subject,
			StartTime: changeMetadata.ImplementationStart.Format(time.RFC3339),
			EndTime:   changeMetadata.ImplementationEnd.Format(time.RFC3339),
			Organizer: organizerEmail,
			Attendees: []string{},
			JoinURL:   "https://teams.microsoft.com", // Fallback URL
		}
		return meetingMetadata, nil
	}

	graphResponse, err := ses.GetGraphMeetingDetails(accessToken, organizerEmail, graphMeetingID)
	if err != nil {
		log.Printf("⚠️  Failed to get meeting details from Graph API: %v", err)
		// Determine subject based on type
		subject := ""
		if strings.HasPrefix(changeMetadata.ObjectType, "announcement_") {
			subject = changeMetadata.ChangeTitle
		} else {
			subject = fmt.Sprintf("Change Implementation: %s", changeMetadata.ChangeTitle)
		}

		// Continue without join URL
		meetingMetadata := &types.MeetingMetadata{
			MeetingID: graphMeetingID,
			Subject:   subject,
			StartTime: changeMetadata.ImplementationStart.Format(time.RFC3339),
			EndTime:   changeMetadata.ImplementationEnd.Format(time.RFC3339),
			Organizer: organizerEmail,
			Attendees: []string{},
			JoinURL:   "https://teams.microsoft.com", // Fallback URL
		}
		return meetingMetadata, nil
	}

	// Extract join URL from online meeting info
	joinURL := ""
	if graphResponse.OnlineMeeting != nil && graphResponse.OnlineMeeting.JoinURL != "" {
		joinURL = graphResponse.OnlineMeeting.JoinURL
		log.Printf("✅ Extracted join URL from Graph response: %s", joinURL)
	} else {
		// Fallback: try to extract from meeting body content
		if graphResponse.Body != nil && graphResponse.Body.Content != "" {
			joinURL = ses.ExtractTeamsJoinURL(graphResponse.Body.Content)
			if joinURL != "" {
				log.Printf("✅ Extracted join URL from meeting body content")
			}
		}
	}
	if joinURL == "" {
		joinURL = "https://teams.microsoft.com" // Fallback URL
		log.Printf("⚠️  Could not extract Teams join URL from Graph response")
	}

	// Create meeting metadata with the actual Graph meeting ID and join URL
	meetingMetadata := &types.MeetingMetadata{
		MeetingID: graphMeetingID,
		Subject:   graphResponse.Subject,
		StartTime: changeMetadata.ImplementationStart.Format(time.RFC3339),
		EndTime:   changeMetadata.ImplementationEnd.Format(time.RFC3339),
		Organizer: organizerEmail,
		Attendees: []string{}, // Recipients were aggregated by SES function
		JoinURL:   joinURL,
	}

	log.Printf("📅 Meeting metadata created: ID=%s, JoinURL=%s", meetingMetadata.MeetingID, meetingMetadata.JoinURL)

	return meetingMetadata, nil
}

// getConfig retrieves the application configuration
func (ms *MeetingScheduler) getConfig(ctx context.Context) (*types.Config, error) {
	// Load config from environment or default location
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/var/task/" // Lambda default
	}

	configFile := configPath + "config.json"
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// CancelMeetingWithMetadata cancels an existing Microsoft Graph meeting and updates the change object
func (ms *MeetingScheduler) CancelMeetingWithMetadata(ctx context.Context, changeMetadata *types.ChangeMetadata, s3Bucket, s3Key string) error {
	log.Printf("❌ Cancelling meeting for change %s", changeMetadata.ChangeID)

	// Check for existing meeting metadata
	existingMeeting := changeMetadata.GetLatestMeetingMetadata()
	if existingMeeting == nil {
		log.Printf("ℹ️  No existing meeting found for change %s, nothing to cancel", changeMetadata.ChangeID)
		return nil
	}

	log.Printf("🔍 Found existing meeting to cancel: ID=%s, Subject=%s", existingMeeting.MeetingID, existingMeeting.Subject)

	// Cancel the meeting using Microsoft Graph API
	err := ms.cancelGraphMeeting(ctx, existingMeeting)
	if err != nil {
		log.Printf("⚠️  Failed to cancel Microsoft Graph meeting %s: %v", existingMeeting.MeetingID, err)

		// Handle the cancellation failure by adding appropriate modification entries
		HandleMeetingCancellationFailure(ctx, changeMetadata, existingMeeting.MeetingID, err, s3Bucket, s3Key, ms.s3UpdateManager)

		// Return the error to indicate the cancellation failed
		return fmt.Errorf("failed to cancel Microsoft Graph meeting %s: %w", existingMeeting.MeetingID, err)
	} else {
		log.Printf("✅ Successfully cancelled Microsoft Graph meeting: ID=%s", existingMeeting.MeetingID)
	}

	// Update the change object in S3 with meeting cancellation
	if ms.s3UpdateManager != nil {
		err = ms.s3UpdateManager.UpdateChangeObjectWithMeetingCancellation(ctx, s3Bucket, s3Key)
		if err != nil {
			log.Printf("⚠️  Failed to update S3 object with meeting cancellation: %v", err)
			return fmt.Errorf("failed to update S3 object with meeting cancellation: %w", err)
		} else {
			log.Printf("✅ Updated S3 object with meeting cancellation")
		}
	} else {
		log.Printf("⚠️  S3UpdateManager not available, skipping S3 update")
	}

	return nil
}

// getGraphAccessTokenForCancellation gets an access token for Microsoft Graph API operations
func getGraphAccessTokenForCancellation() (string, error) {
	// Get Azure credentials from environment or Parameter Store
	clientID, clientSecret, tenantID, err := ses.GetAzureCredentials(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	// Prepare the token request
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("scope", "https://graph.microsoft.com/.default")
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse types.GraphAuthResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	return tokenResponse.AccessToken, nil
}

// cancelGraphMeeting cancels a meeting using Microsoft Graph API
func (ms *MeetingScheduler) cancelGraphMeeting(ctx context.Context, meetingMetadata *types.MeetingMetadata) error {
	log.Printf("🔄 Cancelling Microsoft Graph meeting: ID=%s", meetingMetadata.MeetingID)

	// Get access token for Microsoft Graph API
	accessToken, err := getGraphAccessTokenForCancellation()
	if err != nil {
		return fmt.Errorf("failed to get Graph access token for meeting cancellation: %w", err)
	}

	// Get organizer email from environment or use a default
	organizerEmail := os.Getenv("MEETING_ORGANIZER_EMAIL")
	if organizerEmail == "" {
		organizerEmail = "ccoe-team@example.com" // Default organizer email
		log.Printf("⚠️  Using default organizer email: %s", organizerEmail)
	}

	// Cancel the meeting by updating it with isCancelled: true
	// This is preferred over DELETE as it preserves the meeting record and notifies attendees
	cancelPayload := map[string]interface{}{
		"isCancelled": true,
	}

	payloadBytes, err := json.Marshal(cancelPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal cancellation payload: %w", err)
	}

	// Create the PATCH request to cancel the meeting
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/events/%s", organizerEmail, meetingMetadata.MeetingID)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("failed to create meeting cancellation request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to cancel meeting via Microsoft Graph API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("⚠️  Failed to read cancellation response body: %v", err)
		body = []byte("unable to read response")
	}

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		// Try to parse Graph API error response
		var graphError types.GraphError
		if json.Unmarshal(body, &graphError) == nil {
			return fmt.Errorf("meeting cancellation failed: %s - %s (status: %d)",
				graphError.Error.Code, graphError.Error.Message, resp.StatusCode)
		}
		return fmt.Errorf("meeting cancellation failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Successfully cancelled Microsoft Graph meeting: ID=%s", meetingMetadata.MeetingID)
	return nil
}

// CancelMeetingsForDeletedChange cancels all meetings associated with a deleted change
func CancelMeetingsForDeletedChange(ctx context.Context, changeMetadata *types.ChangeMetadata, cfg *types.Config, s3Bucket, s3Key string) error {
	log.Printf("🗑️  Processing meeting cancellation for deleted change: %s", changeMetadata.ChangeID)

	// Check if the change has any scheduled meetings
	if !changeMetadata.HasMeetingScheduled() {
		log.Printf("ℹ️  No meetings found for deleted change %s, nothing to cancel", changeMetadata.ChangeID)
		return nil
	}

	// Get all meeting metadata from modification entries
	var meetingsToCancel []*types.MeetingMetadata
	for _, entry := range changeMetadata.Modifications {
		if entry.ModificationType == types.ModificationTypeMeetingScheduled && entry.MeetingMetadata != nil {
			meetingsToCancel = append(meetingsToCancel, entry.MeetingMetadata)
		}
	}

	if len(meetingsToCancel) == 0 {
		log.Printf("ℹ️  No meeting metadata found for deleted change %s", changeMetadata.ChangeID)
		return nil
	}

	log.Printf("📅 Found %d meeting(s) to cancel for deleted change %s", len(meetingsToCancel), changeMetadata.ChangeID)

	// Create meeting scheduler
	scheduler := NewMeetingScheduler(cfg.AWSRegion)

	// Cancel each meeting
	var cancelErrors []error
	successCount := 0

	for i, meetingMetadata := range meetingsToCancel {
		log.Printf("❌ Cancelling meeting %d/%d: ID=%s, Subject=%s",
			i+1, len(meetingsToCancel), meetingMetadata.MeetingID, meetingMetadata.Subject)

		err := scheduler.cancelGraphMeeting(ctx, meetingMetadata)
		if err != nil {
			log.Printf("⚠️  Failed to cancel meeting %s: %v", meetingMetadata.MeetingID, err)

			// Handle the cancellation failure
			HandleMeetingCancellationFailure(ctx, changeMetadata, meetingMetadata.MeetingID, err, s3Bucket, s3Key, scheduler.s3UpdateManager)

			cancelErrors = append(cancelErrors, fmt.Errorf("meeting %s: %w", meetingMetadata.MeetingID, err))
		} else {
			log.Printf("✅ Successfully cancelled meeting: ID=%s", meetingMetadata.MeetingID)
			successCount++
		}
	}

	// Update the change object with meeting cancellation entries
	if scheduler.s3UpdateManager != nil {
		log.Printf("📝 Adding meeting cancellation entries to change object")

		// Add a meeting_cancelled entry for each cancelled meeting
		for range meetingsToCancel {
			err := scheduler.s3UpdateManager.UpdateChangeObjectWithMeetingCancellation(ctx, s3Bucket, s3Key)
			if err != nil {
				log.Printf("⚠️  Failed to update S3 object with meeting cancellation: %v", err)
				cancelErrors = append(cancelErrors, fmt.Errorf("S3 update failed: %w", err))
			}
		}
	}

	// Log summary
	log.Printf("📊 Meeting cancellation summary for change %s: %d successful, %d errors",
		changeMetadata.ChangeID, successCount, len(cancelErrors))

	// Return error if any cancellations failed, but don't fail the entire operation
	if len(cancelErrors) > 0 {
		log.Printf("⚠️  Some meeting cancellations failed, but continuing with change deletion")
		// Log errors but don't return them to avoid blocking change deletion
		for _, err := range cancelErrors {
			log.Printf("❌ Meeting cancellation error: %v", err)
		}
	}

	return nil
}

// HandleMeetingCancellationFailure handles meeting cancellation failures by adding appropriate modification entries
func HandleMeetingCancellationFailure(ctx context.Context, changeMetadata *types.ChangeMetadata, meetingID string, err error, s3Bucket, s3Key string, s3UpdateManager *S3UpdateManager) {
	log.Printf("⚠️  Handling meeting cancellation failure for meeting %s: %v", meetingID, err)

	// Create a meeting_cancelled entry with error information in the metadata
	// We still mark it as cancelled to indicate the attempt was made
	entry := types.ModificationEntry{
		Timestamp:        time.Now(),
		UserID:           types.BackendUserID,
		ModificationType: types.ModificationTypeMeetingCancelled,
		// Note: We don't include MeetingMetadata here since the cancellation failed
	}

	// Add the entry to the change metadata with validation
	if err := changeMetadata.AddModificationEntry(entry); err != nil {
		log.Printf("❌ Failed to add meeting cancellation failure entry: %v", err)
		return // Continue with the rest of the function even if this fails
	}

	// Try to update S3 with the failure entry
	if s3UpdateManager != nil {
		updateErr := s3UpdateManager.UpdateChangeObjectInS3(ctx, s3Bucket, s3Key, changeMetadata)
		if updateErr != nil {
			log.Printf("❌ Failed to update S3 with meeting cancellation failure entry: %v", updateErr)
		} else {
			log.Printf("📝 Added meeting cancellation failure entry to S3")
		}
	}

	// Log the failure for audit purposes
	log.Printf("📊 MEETING_CANCELLATION_FAILURE: ChangeID=%s, MeetingID=%s, Error=%v, Timestamp=%s",
		changeMetadata.ChangeID, meetingID, err, time.Now().Format(time.RFC3339))
}

// sendChangeEmailWithTemplate sends change notification emails using the new template system
func sendChangeEmailWithTemplate(ctx context.Context, sesClient *sesv2.Client, topicName string, metadata *types.ChangeMetadata, cfg *types.Config, notificationType string) error {
	// Get account contact list
	accountListName, err := ses.GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get all contacts that should receive emails for this topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s'", topicName)
		return nil
	}

	// Initialize template registry with email config
	registry := templates.NewTemplateRegistry(cfg.EmailConfig)

	// Prepare template data based on notification type
	var template templates.EmailTemplate
	var templateErr error

	switch notificationType {
	case "approval_request":
		// Get customer code from metadata (use first customer if multiple)
		customerCode := ""
		if len(metadata.Customers) > 0 {
			customerCode = metadata.Customers[0]
		}

		data := templates.ApprovalRequestData{
			BaseTemplateData: templates.BaseTemplateData{
				EventID:       metadata.ChangeID,
				EventType:     "change",
				Category:      "change",
				Status:        metadata.Status,
				Title:         metadata.ChangeTitle,
				Summary:       metadata.ChangeReason,
				Content:       metadata.ImplementationPlan,
				SenderAddress: cfg.EmailConfig.SenderAddress,
				Timestamp:     time.Now(),
				Attachments:   extractAttachments(metadata),
			},
			ApprovalURL: fmt.Sprintf("%s/approvals.html?customerCode=%s&objectId=%s", cfg.EmailConfig.PortalBaseURL, customerCode, metadata.ChangeID),
			Customers:   metadata.Customers,
		}
		template, templateErr = registry.GetTemplate("change", templates.NotificationApprovalRequest, data)

	case "approved":
		// Extract approval records from metadata
		approvals := extractApprovalRecords(metadata)
		data := templates.ApprovedNotificationData{
			BaseTemplateData: templates.BaseTemplateData{
				EventID:       metadata.ChangeID,
				EventType:     "change",
				Category:      "change",
				Status:        metadata.Status,
				Title:         metadata.ChangeTitle,
				Summary:       metadata.ChangeReason,
				Content:       metadata.ImplementationPlan,
				SenderAddress: cfg.EmailConfig.SenderAddress,
				Timestamp:     time.Now(),
				Attachments:   extractAttachments(metadata),
			},
			Approvals: approvals,
		}
		template, templateErr = registry.GetTemplate("change", templates.NotificationApproved, data)

	case "completed":
		// Generate survey URL with hidden parameters
		surveyURL, qrCode := generateSurveyURLAndQRCode(metadata, cfg)

		data := templates.CompletionData{
			BaseTemplateData: templates.BaseTemplateData{
				EventID:       metadata.ChangeID,
				EventType:     "change",
				Category:      "change",
				Status:        metadata.Status,
				Title:         metadata.ChangeTitle,
				Summary:       metadata.ChangeReason,
				Content:       metadata.ImplementationPlan,
				SenderAddress: cfg.EmailConfig.SenderAddress,
				Timestamp:     time.Now(),
				Attachments:   extractAttachments(metadata),
			},
			CompletedBy:      metadata.ModifiedBy,
			CompletedByEmail: "", // Not available in current metadata
			CompletedAt:      metadata.ModifiedAt,
			SurveyURL:        surveyURL,
			SurveyQRCode:     qrCode,
		}
		template, templateErr = registry.GetTemplate("change", templates.NotificationCompleted, data)

	case "cancelled":
		data := templates.CancellationData{
			BaseTemplateData: templates.BaseTemplateData{
				EventID:       metadata.ChangeID,
				EventType:     "change",
				Category:      "change",
				Status:        metadata.Status,
				Title:         metadata.ChangeTitle,
				Summary:       metadata.ChangeReason,
				Content:       metadata.ImplementationPlan,
				SenderAddress: cfg.EmailConfig.SenderAddress,
				Timestamp:     time.Now(),
				Attachments:   extractAttachments(metadata),
			},
			CancelledBy:      metadata.ModifiedBy,
			CancelledByEmail: "", // Not available in current metadata
			CancelledAt:      metadata.ModifiedAt,
		}
		template, templateErr = registry.GetTemplate("change", templates.NotificationCancelled, data)

	default:
		return fmt.Errorf("unknown notification type: %s", notificationType)
	}

	if templateErr != nil {
		return fmt.Errorf("failed to generate template: %w", templateErr)
	}

	log.Printf("📧 Sending %s notification to topic '%s' (%d subscribers)", notificationType, topicName, len(subscribedContacts))
	log.Printf("📤 Using sender address: %s", cfg.EmailConfig.SenderAddress)

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(cfg.EmailConfig.SenderAddress),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(template.Subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(template.HTMLBody),
					},
					Text: &sesv2Types.Content{
						Data: aws.String(template.TextBody),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	successCount := 0
	errorCount := 0
	skippedCount := 0

	// Send to each subscribed contact
	for _, contact := range subscribedContacts {
		// Check if recipient is allowed based on restricted_recipients config
		if !shouldSendToRecipient(*contact.EmailAddress) {
			log.Printf("   ⏭️  Skipping %s (not on restricted recipient list)", *contact.EmailAddress)
			skippedCount++
			continue
		}

		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(ctx, sendInput)
		if err != nil {
			log.Printf("   ❌ Failed to send to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			log.Printf("   ✅ Sent to %s", *contact.EmailAddress)
			successCount++
		}
	}

	if skippedCount > 0 {
		log.Printf("📊 Email Summary: ✅ %d successful, ❌ %d errors, ⏭️  %d skipped", successCount, errorCount, skippedCount)
	} else {
		log.Printf("📊 Email Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)
	}

	if errorCount > 0 {
		return fmt.Errorf("failed to send email to %d recipients", errorCount)
	}

	return nil
}

// extractAttachments extracts attachment URLs from metadata
func extractAttachments(metadata *types.ChangeMetadata) []string {
	var attachments []string
	if metadata.Metadata != nil {
		if attachmentsVal, exists := metadata.Metadata["attachments"]; exists {
			if attachmentsList, ok := attachmentsVal.([]interface{}); ok {
				for _, att := range attachmentsList {
					if attMap, ok := att.(map[string]interface{}); ok {
						if url, ok := attMap["url"].(string); ok {
							attachments = append(attachments, url)
						} else if s3Key, ok := attMap["s3_key"].(string); ok {
							attachmentURL := fmt.Sprintf("https://s3.amazonaws.com/%s", s3Key)
							attachments = append(attachments, attachmentURL)
						}
					} else if attStr, ok := att.(string); ok {
						attachments = append(attachments, attStr)
					}
				}
			}
		}
	}
	return attachments
}

// extractApprovalRecords extracts approval records from metadata
func extractApprovalRecords(metadata *types.ChangeMetadata) []templates.ApprovalRecord {
	var approvals []templates.ApprovalRecord

	// Extract approvals from modifications array (preferred method)
	for _, mod := range metadata.Modifications {
		if mod.ModificationType == types.ModificationTypeApproved {
			approvals = append(approvals, templates.ApprovalRecord{
				ApprovedBy:    mod.UserID,
				ApprovedAt:    mod.Timestamp,
				ApproverEmail: "", // Email not stored in modifications
			})
		}
	}

	// Fallback to flat fields if no modification entries found (for legacy changes)
	if len(approvals) == 0 && metadata.ApprovedBy != "" && metadata.ApprovedAt != nil && !metadata.ApprovedAt.IsZero() {
		approval := templates.ApprovalRecord{
			ApprovedBy:    metadata.ApprovedBy,
			ApproverEmail: "", // Not available in current metadata
			ApprovedAt:    *metadata.ApprovedAt,
		}
		approvals = append(approvals, approval)
	}

	return approvals
}
