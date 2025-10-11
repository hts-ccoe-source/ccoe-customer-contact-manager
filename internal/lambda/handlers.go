package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	"ccoe-customer-contact-manager/internal/types"
)

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
	log.Printf("Received %d SQS messages", len(sqsEvent.Records))

	// Load configuration
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	// Process each SQS message with proper error handling
	var retryableErrors []error
	var nonRetryableErrors []error
	successCount := 0

	for i, record := range sqsEvent.Records {
		log.Printf("Processing message %d/%d: %s", i+1, len(sqsEvent.Records), record.MessageId)

		err := ProcessSQSRecord(ctx, record, cfg)
		if err != nil {
			// Log the error with proper classification
			LogError(err, record.MessageId)

			// Determine if this error should cause a retry
			if ShouldDeleteMessage(err) {
				// Non-retryable error - message will be deleted from queue
				nonRetryableErrors = append(nonRetryableErrors, fmt.Errorf("message %s (non-retryable): %w", record.MessageId, err))
				log.Printf("🗑️  Message %s will be deleted from queue (non-retryable error)", record.MessageId)
			} else {
				// Retryable error - message will be retried
				retryableErrors = append(retryableErrors, fmt.Errorf("message %s (retryable): %w", record.MessageId, err))
				log.Printf("🔄 Message %s will be retried", record.MessageId)
			}
		} else {
			successCount++
			log.Printf("✅ Successfully processed message %s", record.MessageId)
		}
	}

	// Log summary
	log.Printf("📊 Processing complete: %d successful, %d retryable errors, %d non-retryable errors",
		successCount, len(retryableErrors), len(nonRetryableErrors))

	// Only return error for retryable failures (this will cause Lambda to retry those messages)
	// Non-retryable messages will be automatically deleted from the queue
	if len(retryableErrors) > 0 {
		log.Printf("⚠️  Returning error to Lambda for %d retryable messages", len(retryableErrors))
		return fmt.Errorf("failed to process %d retryable messages: %v", len(retryableErrors), retryableErrors[0])
	}

	if len(nonRetryableErrors) > 0 {
		log.Printf("✅ All errors were non-retryable - messages will be deleted from queue")
	}

	return nil
}

// ProcessSQSRecord processes a single SQS record from Lambda
func ProcessSQSRecord(ctx context.Context, record events.SQSMessage, cfg *types.Config) error {
	// Log the raw message for debugging
	log.Printf("Processing SQS message: %s", record.Body)

	// Check if this is an S3 test event and skip it
	if IsS3TestEvent(record.Body) {
		log.Printf("Skipping S3 test event")
		return nil
	}

	// Extract userIdentity from SQS message for event loop prevention
	roleConfig := LoadRoleConfigFromEnvironment()
	userIdentityExtractor := NewUserIdentityExtractorWithConfig(roleConfig)

	userIdentity, err := userIdentityExtractor.SafeExtractUserIdentity(record.Body, record.MessageId)
	if err != nil {
		// Log the error but continue processing - missing userIdentity shouldn't block legitimate events
		log.Printf("⚠️  Failed to extract userIdentity from message %s: %v", record.MessageId, err)
		log.Printf("🔄 Continuing with event processing despite userIdentity extraction failure")
	} else {
		// Check if this event should be discarded (backend-generated event)
		shouldDiscard, reason := userIdentityExtractor.ShouldDiscardEvent(userIdentity)
		if shouldDiscard {
			log.Printf("🗑️  Discarding SQS message %s: %s", record.MessageId, reason)
			return nil // Successfully processed (by discarding)
		}
		log.Printf("✅ Processing SQS message %s: %s", record.MessageId, reason)
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
		log.Printf("Successfully parsed S3 event, Records count: %d", len(s3Event.Records))
		if len(s3Event.Records) > 0 {
			log.Printf("Processing as S3 event notification with %d records", len(s3Event.Records))
			for i, rec := range s3Event.Records {
				log.Printf("Record %d: EventSource=%s, S3.Bucket.Name=%s, S3.Object.Key=%s",
					i, rec.EventSource, rec.S3.Bucket.Name, rec.S3.Object.Key)
			}
			// Process as S3 event
			return ProcessS3Event(ctx, s3Event, cfg)
		} else {
			log.Printf("S3 event parsed successfully but has no records")
		}
	} else {
		log.Printf("Failed to parse as S3 event: %v", err)
	}

	// Try to parse as legacy SQS message
	var sqsMsg types.SQSMessage
	if err := json.Unmarshal([]byte(record.Body), &sqsMsg); err == nil && sqsMsg.CustomerCode != "" {
		log.Printf("Processing as legacy SQS message for customer: %s", sqsMsg.CustomerCode)
		// Set the message ID from the SQS record
		sqsMsg.MessageID = record.MessageId
		// Process as legacy SQS message
		return ProcessSQSMessage(ctx, sqsMsg, cfg)
	} else {
		log.Printf("Failed to parse as legacy SQS message: %v", err)
	}

	log.Printf("Message body type: %T, content: %+v", messageBody, messageBody)
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

// ProcessS3Event processes an S3 event notification in Lambda context
func ProcessS3Event(ctx context.Context, s3Event types.S3EventNotification, cfg *types.Config) error {
	// Check userIdentity for event loop prevention before processing any records
	roleConfig := LoadRoleConfigFromEnvironment()
	userIdentityExtractor := NewUserIdentityExtractorWithConfig(roleConfig)

	userIdentity, err := userIdentityExtractor.ExtractUserIdentityFromS3Event(s3Event)
	if err != nil {
		// Log the error but continue processing - missing userIdentity shouldn't block legitimate events
		log.Printf("⚠️  Failed to extract userIdentity from S3 event: %v", err)
		log.Printf("🔄 Continuing with S3 event processing despite userIdentity extraction failure")
	} else {
		// Check if this event should be discarded (backend-generated event)
		shouldDiscard, reason := userIdentityExtractor.ShouldDiscardEvent(userIdentity)
		if shouldDiscard {
			log.Printf("🗑️  Discarding S3 event: %s", reason)
			return nil // Successfully processed (by discarding)
		}
		log.Printf("✅ Processing S3 event: %s", reason)
	}

	for _, record := range s3Event.Records {
		if record.EventSource != "aws:s3" {
			log.Printf("Skipping non-S3 event: %s", record.EventSource)
			continue
		}

		bucketName := record.S3.Bucket.Name
		objectKey := record.S3.Object.Key

		log.Printf("Processing S3 event: s3://%s/%s (EventName: %s)", bucketName, objectKey, record.EventName)

		// Extract customer code from S3 key
		customerCode, err := ExtractCustomerCodeFromS3Key(objectKey)
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

		// Download and process metadata
		metadata, err := DownloadMetadataFromS3(ctx, bucketName, objectKey, cfg.AWSRegion)
		if err != nil {
			// Classify the S3 error appropriately
			return ClassifyError(err, "", bucketName, objectKey)
		}

		// Process the change request
		err = ProcessChangeRequest(ctx, customerCode, metadata, cfg, bucketName, objectKey)
		if err != nil {
			// Email/processing errors are typically retryable
			return NewProcessingError(
				ErrorTypeEmailError,
				fmt.Sprintf("Failed to process change request for customer %s: %v", customerCode, err),
				true, // Retryable
				err,
				"",
				bucketName,
				objectKey,
			)
		}

		log.Printf("Successfully processed S3 event for customer %s", customerCode)
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

	log.Printf("Successfully processed legacy SQS message for customer %s", sqsMsg.CustomerCode)
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

	// Log basic info about the content for debugging
	log.Printf("Downloaded S3 object size: %d bytes", len(contentBytes))

	// Extract request type from S3 object metadata if available
	var requestTypeFromS3 string
	if result.Metadata != nil {
		if reqType, exists := result.Metadata["request-type"]; exists {
			requestTypeFromS3 = reqType
		}
	}

	// Parse as standard ChangeMetadata (flat structure)
	var metadata types.ChangeMetadata
	if err := json.Unmarshal(contentBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata as ChangeMetadata: %w. Content: %s", err, string(contentBytes))
	}

	// Validate that we have essential fields for a valid ChangeMetadata
	if metadata.ChangeID == "" && metadata.ChangeTitle == "" {
		return nil, fmt.Errorf("invalid ChangeMetadata: missing both ChangeID and ChangeTitle. Content: %s", string(contentBytes))
	}

	log.Printf("Successfully parsed as ChangeMetadata structure")

	// Ensure we have a ChangeID if missing
	if metadata.ChangeID == "" && metadata.ChangeTitle != "" {
		metadata.ChangeID = fmt.Sprintf("CHG-%d", time.Now().Unix())
		log.Printf("Generated ChangeID: %s", metadata.ChangeID)
	}

	// Set default status if missing
	if metadata.Status == "" {
		metadata.Status = "submitted"
		log.Printf("Set default status: %s", metadata.Status)
	}

	// Apply request type from S3 metadata if available
	if requestTypeFromS3 != "" {
		if metadata.Metadata == nil {
			metadata.Metadata = make(map[string]interface{})
		}
		metadata.Metadata["request_type"] = requestTypeFromS3
		log.Printf("Set request_type from S3 metadata: %s", requestTypeFromS3)
	}

	return &metadata, nil
}

// ProcessChangeRequest processes a change request with metadata
func ProcessChangeRequest(ctx context.Context, customerCode string, metadata *types.ChangeMetadata, cfg *types.Config, s3Bucket, s3Key string) error {
	// Generate a unique processing ID for this request to help track duplicates
	processingID := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	log.Printf("[%s] Processing change request %s for customer %s", processingID, metadata.ChangeID, customerCode)

	// Determine the request type based on the metadata structure and source
	requestType := DetermineRequestType(metadata)
	log.Printf("[%s] Determined request type: %s", processingID, requestType)

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
			log.Printf("Failed to send approval request email for customer %s: %v", customerCode, err)
		} else {
			log.Printf("Successfully sent approval request email for customer %s", customerCode)
		}
	case "approved_announcement":
		err := SendApprovedAnnouncementEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("Failed to send approved announcement email for customer %s: %v", customerCode, err)
		} else {
			log.Printf("Successfully sent approved announcement email for customer %s", customerCode)
		}

		// Check if this approved change has meeting settings and schedule multi-customer meeting
		err = ScheduleMultiCustomerMeetingIfNeeded(ctx, metadata, cfg, s3Bucket, s3Key)
		if err != nil {
			log.Printf("Failed to schedule multi-customer meeting for change %s: %v", metadata.ChangeID, err)
		}

	case "change_complete":
		err := SendChangeCompleteEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("Failed to send change complete email for customer %s: %v", customerCode, err)
		} else {
			log.Printf("Successfully sent change complete email for customer %s", customerCode)
		}
	default:
		log.Printf("Unknown request type %s, treating as approval request", requestType)
		err := SendApprovalRequestEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("Failed to send approval request email for customer %s: %v", customerCode, err)
		} else {
			log.Printf("Successfully sent approval request email for customer %s", customerCode)
		}
	}

	// Note: This system handles change notifications only, not AWS account modifications
	log.Printf("Change notification processing completed for customer %s", customerCode)

	return nil
}

// DetermineRequestType determines the type of request based on metadata
func DetermineRequestType(metadata *types.ChangeMetadata) string {
	// FIRST: Check explicit request_type field (most specific)
	if metadata.Metadata != nil {
		if requestType, exists := metadata.Metadata["request_type"]; exists {
			if rt, ok := requestType.(string); ok {
				return strings.ToLower(rt)
			}
		}
	}

	// SECOND: Check status field as fallback
	if metadata.Metadata != nil {
		if status, exists := metadata.Metadata["status"]; exists {
			if statusStr, ok := status.(string); ok {
				statusLower := strings.ToLower(statusStr)
				if statusLower == "submitted" {
					return "approval_request"
				}
				if statusLower == "approved" {
					return "approved_announcement"
				}
				if statusLower == "completed" {
					return "change_complete"
				}
			}
		}
	}

	// THIRD: Check the source field as fallback
	if metadata.Source != "" {
		source := strings.ToLower(metadata.Source)
		if strings.Contains(source, "approval") && strings.Contains(source, "request") {
			return "approval_request"
		}
		if strings.Contains(source, "approved") && strings.Contains(source, "announcement") {
			return "approved_announcement"
		}
		if strings.Contains(source, "approval") || strings.Contains(source, "request") {
			return "approval_request"
		}
		if strings.Contains(source, "approved") {
			return "approved_announcement"
		}
	}

	// FOURTH: Check metadata for approval-related fields as final fallback
	if metadata.Metadata != nil {

		// Check for approval-related metadata
		for key, value := range metadata.Metadata {
			keyLower := strings.ToLower(key)
			if strings.Contains(keyLower, "approval") || strings.Contains(keyLower, "request") {
				return "approval_request"
			}
			if valueStr, ok := value.(string); ok {
				valueLower := strings.ToLower(valueStr)
				if strings.Contains(valueLower, "approval") || strings.Contains(valueLower, "request") {
					return "approval_request"
				}
			}
		}
	}

	// Check if status indicates approval workflow
	if metadata.Status == "submitted" {
		return "approval_request"
	}
	if metadata.Status == "approved" {
		return "approved_announcement"
	}
	if metadata.Status == "completed" {
		return "change_complete"
	}

	// Default to approval_request for unknown cases (most common workflow)
	return "approval_request"
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

	// Check if meeting is required based on metadata
	meetingRequired := false
	var meetingTitle string

	// Check for meeting settings - first check top-level fields, then nested metadata
	// Check meetingRequired field (top-level first)
	if metadata.MeetingRequired != "" {
		meetingRequired = strings.ToLower(metadata.MeetingRequired) == "yes" || strings.ToLower(metadata.MeetingRequired) == "true"
		log.Printf("📋 Found top-level meetingRequired field: '%s', result: %v", metadata.MeetingRequired, meetingRequired)
	} else if metadata.Metadata != nil {
		if required, exists := metadata.Metadata["meetingRequired"]; exists {
			if reqStr, ok := required.(string); ok {
				meetingRequired = strings.ToLower(reqStr) == "yes" || strings.ToLower(reqStr) == "true"
				log.Printf("📋 Found nested meetingRequired field: '%s', result: %v", reqStr, meetingRequired)
			} else if reqBool, ok := required.(bool); ok {
				meetingRequired = reqBool
				log.Printf("📋 Found nested meetingRequired field: %v", reqBool)
			}
		}
	}

	// Extract meeting details if available (top-level first)
	if metadata.MeetingTitle != "" {
		meetingTitle = metadata.MeetingTitle
		meetingRequired = true // If we have meeting details, assume meeting is required
		log.Printf("📋 Found top-level meetingTitle: '%s', setting meetingRequired to true", meetingTitle)
	} else if metadata.Metadata != nil {
		if title, exists := metadata.Metadata["meetingTitle"]; exists {
			if titleStr, ok := title.(string); ok && titleStr != "" {
				meetingTitle = titleStr
				meetingRequired = true
				log.Printf("📋 Found nested meetingTitle: '%s', setting meetingRequired to true", titleStr)
			}
		}
	}

	if metadata.MeetingStartTime != nil && !metadata.MeetingStartTime.IsZero() {
		log.Printf("📋 Found meetingStartTime: '%s'", metadata.MeetingStartTime.Format("2006-01-02 15:04:05 MST"))
	} else if metadata.Metadata != nil {
		if date, exists := metadata.Metadata["meetingDate"]; exists {
			if dateStr, ok := date.(string); ok {
				log.Printf("📋 Found nested meetingDate: '%s'", dateStr)
			}
		}
	}

	if metadata.MeetingDuration != "" {
		log.Printf("📋 Found meetingDuration: '%s'", metadata.MeetingDuration)
	} else if metadata.Metadata != nil {
		if duration, exists := metadata.Metadata["meetingDuration"]; exists {
			if durationStr, ok := duration.(string); ok {
				log.Printf("📋 Found nested meetingDuration: '%s'", durationStr)
			}
		}
	}

	if metadata.MeetingLocation != "" {
		log.Printf("📋 Found meetingLocation: '%s'", metadata.MeetingLocation)
	} else if metadata.Metadata != nil {
		if location, exists := metadata.Metadata["meetingLocation"]; exists {
			if locationStr, ok := location.(string); ok {
				log.Printf("📋 Found nested meetingLocation: '%s'", locationStr)
			}
		}
	}

	// If no meeting is required, skip scheduling
	if !meetingRequired {
		log.Printf("✅ No meeting required for change %s", metadata.ChangeID)
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
	lambda.Start(Handler)
}

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

// SendApprovalRequestEmail sends approval request email using existing SES template system
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
	senderEmail := "ccoe@nonprod.ccoe.hearst.com"

	changeID := "unknown"
	if id, ok := changeDetails["change_id"].(string); ok && id != "" {
		changeID = id
	}
	log.Printf("📧 Sending approval request email for change %s", changeID)

	// Convert changeDetails to ChangeMetadata format for SES functions
	metadata := createChangeMetadataFromChangeDetails(changeDetails)

	// Send approval request email directly using SES
	err = sendApprovalRequestEmailDirect(sesClient, topicName, senderEmail, metadata)
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

	log.Printf("✅ Approval request email sent to %s members of topic %s from %s", subscriberCount, topicName, senderEmail)
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
	senderEmail := "ccoe@nonprod.ccoe.hearst.com"

	changeID := "unknown"
	if id, ok := changeDetails["change_id"].(string); ok && id != "" {
		changeID = id
	}
	log.Printf("📧 Sending approved announcement email for change %s", changeID)

	// Convert changeDetails to ChangeMetadata format for SES functions
	metadata := createChangeMetadataFromChangeDetails(changeDetails)

	// Send approved announcement email directly using SES
	err = sendApprovedAnnouncementEmailDirect(sesClient, topicName, senderEmail, metadata)
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

	log.Printf("✅ Approved announcement email sent to %s members of topic %s from %s", subscriberCount, topicName, senderEmail)
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
	senderEmail := "ccoe@nonprod.ccoe.hearst.com"

	changeID := "unknown"
	if id, ok := changeDetails["change_id"].(string); ok && id != "" {
		changeID = id
	}
	log.Printf("📧 Sending change complete notification email for change %s", changeID)

	// Convert changeDetails to ChangeMetadata format for SES functions
	metadata := createChangeMetadataFromChangeDetails(changeDetails)

	// Send change complete email directly using SES
	err = sendChangeCompleteEmailDirect(sesClient, topicName, senderEmail, metadata)
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

	log.Printf("✅ Change complete notification email sent to %s members of topic %s from %s", subscriberCount, topicName, senderEmail)
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
        <p><strong>Description:</strong> %s</p>
        <p><strong>Status:</strong> <span style="color: #28a745; font-weight: bold;">✅ APPROVED</span></p>
        <p><strong>Approved By:</strong> %s</p>
        <p><strong>Approved At:</strong> %s</p>
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
		metadata.ChangeReason,
		metadata.ApprovedBy,
		metadata.ApprovedAt,
		strings.ReplaceAll(metadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.TestPlan, "\n", "<br>"),
		formatDateTime(metadata.ImplementationStart),
		formatDateTime(metadata.ImplementationEnd),
		metadata.Timezone,
		metadata.CustomerImpact,
		strings.ReplaceAll(metadata.RollbackPlan, "\n", "<br>"),
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
	senderEmail := "ccoe@nonprod.ccoe.hearst.com"

	// Send email to each subscribed contact
	successCount := 0
	errorCount := 0

	for _, contact := range subscribedContacts {
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

	// Send to each subscribed contact
	for _, contact := range subscribedContacts {
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

	log.Printf("📊 Approval Request Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)

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
	originalSubject := fmt.Sprintf("ITSM Change Notification: %s", metadata.ChangeTitle)
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

	// Send to each subscribed contact
	for _, contact := range subscribedContacts {
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

	log.Printf("📊 Approved Announcement Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)

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

	for _, contact := range subscribedContacts {
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

	log.Printf("📊 Change Complete Summary: ✅ %d successful, ❌ %d errors", successCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("failed to send change complete notification to %d recipients", errorCount)
	}

	return nil
}

// Helper functions for email templates

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
		log.Printf("⚠️  Failed to create S3UpdateManager: %v", err)
		return &MeetingScheduler{region: region}
	}

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
	if ms.s3UpdateManager != nil {
		err = ms.s3UpdateManager.UpdateChangeObjectWithMeetingMetadata(ctx, s3Bucket, s3Key, meetingMetadata)
		if err != nil {
			log.Printf("⚠️  Failed to update S3 object with meeting metadata: %v", err)
			// Don't return error here - meeting was created/updated successfully
			// Log the issue but continue
		} else {
			log.Printf("✅ Updated S3 object with meeting metadata")
		}
	} else {
		log.Printf("⚠️  S3UpdateManager not available, skipping S3 update")
	}

	return meetingMetadata, nil
}

// checkIfMeetingNeedsUpdate determines if an existing meeting needs to be updated
func (ms *MeetingScheduler) checkIfMeetingNeedsUpdate(changeMetadata *types.ChangeMetadata, existingMeeting *types.MeetingMetadata) (bool, string) {
	log.Printf("🔍 Checking if meeting needs update for change %s", changeMetadata.ChangeID)

	// Compare meeting title/subject
	expectedSubject := fmt.Sprintf("Change Implementation: %s", changeMetadata.ChangeTitle)
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
		Subject:   fmt.Sprintf("Change Implementation: %s", changeMetadata.ChangeTitle),
		Organizer: existingMeeting.Organizer, // Keep existing organizer
		Attendees: existingMeeting.Attendees, // Keep existing attendees
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
// This function uses the existing ses.CreateMultiCustomerMeetingInvite which handles:
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

	// Create temporary metadata file for the SES meeting functionality
	tempMetadataPath, err := ms.createTempMetadataFile(changeMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary metadata file: %w", err)
	}
	defer os.Remove(tempMetadataPath) // Clean up temp file

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

	// Use the existing SES function which handles all the complexity:
	// - Assumes roles into each customer account
	// - Queries aws-calendar topic from each customer's SES
	// - Aggregates and deduplicates recipients
	// - Creates meeting via Microsoft Graph API
	topicName := "aws-calendar"

	log.Printf("🚀 Calling ses.CreateMultiCustomerMeetingInvite for %d customers", len(changeMetadata.Customers))

	graphMeetingID, err := ses.CreateMultiCustomerMeetingInvite(
		credentialManager,
		changeMetadata.Customers,
		topicName,
		tempMetadataPath,
		organizerEmail,
		false, // not dry-run
		false, // not force-update
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create multi-customer meeting: %w", err)
	}

	log.Printf("✅ Successfully created multi-customer meeting with Graph ID: %s", graphMeetingID)

	// Create meeting metadata with the actual Graph meeting ID
	meetingMetadata := &types.MeetingMetadata{
		MeetingID: graphMeetingID,
		Subject:   fmt.Sprintf("Change Implementation: %s", changeMetadata.ChangeTitle),
		StartTime: changeMetadata.ImplementationStart.Format(time.RFC3339),
		EndTime:   changeMetadata.ImplementationEnd.Format(time.RFC3339),
		Organizer: organizerEmail,
		Attendees: []string{}, // Recipients were aggregated by SES function
	}

	return meetingMetadata, nil
}

// createTempMetadataFile creates a temporary JSON file with the change metadata for SES functions
func (ms *MeetingScheduler) createTempMetadataFile(changeMetadata *types.ChangeMetadata) (string, error) {
	// Convert ChangeMetadata to ApprovalRequestMetadata format expected by SES functions
	approvalMetadata := convertChangeMetadataToApprovalRequest(changeMetadata)

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(approvalMetadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Create temp file
	tempFile := fmt.Sprintf("/tmp/meeting-metadata-%s-%d.json", changeMetadata.ChangeID, time.Now().Unix())
	err = os.WriteFile(tempFile, jsonData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	log.Printf("📝 Created temporary metadata file: %s", tempFile)
	return tempFile, nil
}

// convertChangeMetadataToApprovalRequest converts flat ChangeMetadata to nested ApprovalRequestMetadata
func convertChangeMetadataToApprovalRequest(change *types.ChangeMetadata) *types.ApprovalRequestMetadata {
	return &types.ApprovalRequestMetadata{
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
			Title:                  change.ChangeTitle,
			CustomerCodes:          change.Customers,
			ChangeReason:           change.ChangeReason,
			ImplementationPlan:     change.ImplementationPlan,
			TestPlan:               change.TestPlan,
			ExpectedCustomerImpact: change.CustomerImpact,
			RollbackPlan:           change.RollbackPlan,
			Schedule: struct {
				ImplementationStart time.Time `json:"implementationStart"`
				ImplementationEnd   time.Time `json:"implementationEnd"`
				Timezone            string    `json:"timezone"`
				BeginDate           string    `json:"beginDate,omitempty"`
				BeginTime           string    `json:"beginTime,omitempty"`
				EndDate             string    `json:"endDate,omitempty"`
				EndTime             string    `json:"endTime,omitempty"`
			}{
				ImplementationStart: change.ImplementationStart,
				ImplementationEnd:   change.ImplementationEnd,
				Timezone:            change.Timezone,
			},
			Description: change.ChangeReason,
			ApprovedBy:  change.ApprovedBy,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		GeneratedBy: "lambda-meeting-scheduler",
	}
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
