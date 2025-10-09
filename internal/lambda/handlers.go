package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	"ccoe-customer-contact-manager/internal/ses"
	"ccoe-customer-contact-manager/internal/types"
)

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
		err = ProcessChangeRequest(ctx, customerCode, metadata, cfg)
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
	err = ProcessChangeRequest(ctx, sqsMsg.CustomerCode, metadata, cfg)
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
func ProcessChangeRequest(ctx context.Context, customerCode string, metadata *types.ChangeMetadata, cfg *types.Config) error {
	// Generate a unique processing ID for this request to help track duplicates
	processingID := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	log.Printf("[%s] Processing change request %s for customer %s", processingID, metadata.ChangeID, customerCode)

	// Determine the request type based on the metadata structure and source
	requestType := DetermineRequestType(metadata)
	log.Printf("[%s] Determined request type: %s", processingID, requestType)

	// Create change details for email notification (same as SQS processor)
	changeDetails := map[string]interface{}{
		"change_id":               metadata.ChangeID,
		"changeTitle":             metadata.ChangeTitle,
		"changeReason":            metadata.ChangeReason,
		"implementationPlan":      metadata.ImplementationPlan,
		"testPlan":                metadata.TestPlan,
		"customerImpact":          metadata.CustomerImpact,
		"rollbackPlan":            metadata.RollbackPlan,
		"snowTicket":              metadata.SnowTicket,
		"jiraTicket":              metadata.JiraTicket,
		"implementationBeginDate": metadata.ImplementationBeginDate,
		"implementationBeginTime": metadata.ImplementationBeginTime,
		"implementationEndDate":   metadata.ImplementationEndDate,
		"implementationEndTime":   metadata.ImplementationEndTime,
		"timezone":                metadata.Timezone,
		"status":                  metadata.Status,
		"version":                 metadata.Version,
		"createdAt":               metadata.CreatedAt,
		"createdBy":               metadata.CreatedBy,
		"modifiedAt":              metadata.ModifiedAt,
		"modifiedBy":              metadata.ModifiedBy,
		"submittedAt":             metadata.SubmittedAt,
		"submittedBy":             metadata.SubmittedBy,
		"approvedAt":              metadata.ApprovedAt,
		"approvedBy":              metadata.ApprovedBy,
		"source":                  metadata.Source,
		"testRun":                 metadata.TestRun,
		"customers":               metadata.Customers,
		"request_type":            requestType,
		"processing_timestamp":    time.Now(),
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
		err = ScheduleMultiCustomerMeetingIfNeeded(ctx, metadata, cfg)
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
		ChangeID:                getString("change_id"),
		ChangeTitle:             getString("changeTitle"),
		ChangeReason:            getString("changeReason"),
		Customers:               getStringSlice("customers"),
		ImplementationPlan:      getString("implementationPlan"),
		TestPlan:                getString("testPlan"),
		CustomerImpact:          getString("customerImpact"),
		RollbackPlan:            getString("rollbackPlan"),
		SnowTicket:              getString("snowTicket"),
		JiraTicket:              getString("jiraTicket"),
		ImplementationBeginDate: getString("implementationBeginDate"),
		ImplementationBeginTime: getString("implementationBeginTime"),
		ImplementationEndDate:   getString("implementationEndDate"),
		ImplementationEndTime:   getString("implementationEndTime"),
		Timezone:                getString("timezone"),
		Status:                  getString("status"),
		Version:                 1, // Default version
		CreatedAt:               getString("createdAt"),
		CreatedBy:               getString("createdBy"),
		ModifiedAt:              getString("modifiedAt"),
		ModifiedBy:              getString("modifiedBy"),
		SubmittedAt:             getString("submittedAt"),
		SubmittedBy:             getString("submittedBy"),
		ApprovedAt:              getString("approvedAt"),
		ApprovedBy:              getString("approvedBy"),
		Source:                  getString("source"),
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

// ScheduleMultiCustomerMeetingIfNeeded checks if the approved change has meeting settings and schedules a multi-customer meeting
func ScheduleMultiCustomerMeetingIfNeeded(ctx context.Context, metadata *types.ChangeMetadata, cfg *types.Config) error {
	log.Printf("🔍 Checking if change %s requires meeting scheduling", metadata.ChangeID)

	// Check if meeting is required based on metadata
	meetingRequired := false
	var meetingTitle, meetingDate, meetingDuration, meetingLocation string

	// Check for meeting settings in various possible fields
	if metadata.Metadata != nil {
		// Check meetingRequired field
		if required, exists := metadata.Metadata["meetingRequired"]; exists {
			if reqStr, ok := required.(string); ok {
				meetingRequired = strings.ToLower(reqStr) == "yes" || strings.ToLower(reqStr) == "true"
			} else if reqBool, ok := required.(bool); ok {
				meetingRequired = reqBool
			}
		}

		// Extract meeting details if available
		if title, exists := metadata.Metadata["meetingTitle"]; exists {
			if titleStr, ok := title.(string); ok && titleStr != "" {
				meetingTitle = titleStr
				meetingRequired = true // If we have meeting details, assume meeting is required
			}
		}

		if date, exists := metadata.Metadata["meetingDate"]; exists {
			if dateStr, ok := date.(string); ok {
				meetingDate = dateStr
			}
		}

		if duration, exists := metadata.Metadata["meetingDuration"]; exists {
			if durationStr, ok := duration.(string); ok {
				meetingDuration = durationStr
			}
		}

		if location, exists := metadata.Metadata["meetingLocation"]; exists {
			if locationStr, ok := location.(string); ok {
				meetingLocation = locationStr
			}
		}
	}

	// Also check if we have implementation dates that could be used for meeting scheduling
	if !meetingRequired && metadata.ImplementationBeginDate != "" && metadata.ImplementationBeginTime != "" {
		// If we have implementation schedule but no explicit meeting, check if we should auto-schedule
		if len(metadata.Customers) > 1 {
			log.Printf("📅 Multi-customer change with implementation schedule detected, considering meeting scheduling")
			meetingRequired = true
			if meetingTitle == "" {
				meetingTitle = fmt.Sprintf("Implementation Meeting: %s", metadata.ChangeTitle)
			}
			if meetingDate == "" {
				meetingDate = metadata.ImplementationBeginDate
			}
			if meetingDuration == "" {
				meetingDuration = "60" // Default 60 minutes
			}
			if meetingLocation == "" {
				meetingLocation = "Microsoft Teams"
			}
		}
	}

	if !meetingRequired {
		log.Printf("📋 No meeting required for change %s", metadata.ChangeID)
		return nil
	}

	if len(metadata.Customers) == 0 {
		log.Printf("⚠️  No customers specified for change %s, cannot schedule meeting", metadata.ChangeID)
		return nil
	}

	log.Printf("📅 Meeting required for change %s with %d customers: %v", metadata.ChangeID, len(metadata.Customers), metadata.Customers)
	log.Printf("📋 Meeting details - Title: %s, Date: %s, Duration: %s, Location: %s",
		meetingTitle, meetingDate, meetingDuration, meetingLocation)

	// Create a temporary metadata file for the meeting functionality
	tempMetadata, err := createTempMeetingMetadata(metadata, meetingTitle, meetingDate, meetingDuration, meetingLocation)
	if err != nil {
		return fmt.Errorf("failed to create temporary meeting metadata: %w", err)
	}

	// Create credential manager
	credentialManager, err := awsinternal.NewCredentialManager(cfg.AWSRegion, cfg.CustomerMappings)
	if err != nil {
		return fmt.Errorf("failed to create credential manager: %w", err)
	}

	// Schedule multi-customer meeting using the SES meeting functionality
	topicName := "aws-calendar"
	senderEmail := "ccoe@hearst.com"

	log.Printf("🚀 Scheduling multi-customer meeting for change %s", metadata.ChangeID)

	err = ses.CreateMultiCustomerMeetingInvite(
		credentialManager,
		metadata.Customers,
		topicName,
		tempMetadata,
		senderEmail,
		false, // not dry-run
		false, // not force-update
	)

	if err != nil {
		log.Printf("❌ Failed to schedule multi-customer meeting: %v", err)
		return fmt.Errorf("failed to schedule multi-customer meeting: %w", err)
	}

	log.Printf("✅ Successfully scheduled multi-customer meeting for change %s with %d customers",
		metadata.ChangeID, len(metadata.Customers))

	return nil
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
				ImplementationStart string `json:"implementationStart"`
				ImplementationEnd   string `json:"implementationEnd"`
				BeginDate           string `json:"beginDate"`
				BeginTime           string `json:"beginTime"`
				EndDate             string `json:"endDate"`
				EndTime             string `json:"endTime"`
				Timezone            string `json:"timezone"`
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
			Description:            fmt.Sprintf("Implementation meeting for change: %s", metadata.ChangeTitle),
			ApprovedBy:             metadata.ApprovedBy,
			ApprovedAt:             metadata.ApprovedAt,
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
		GeneratedAt: time.Now().Format(time.RFC3339),
		GeneratedBy: "lambda-auto-scheduler",
	}

	// Set tickets
	meetingMetadata.ChangeMetadata.Tickets.ServiceNow = metadata.SnowTicket
	meetingMetadata.ChangeMetadata.Tickets.Jira = metadata.JiraTicket

	// Set schedule information
	meetingMetadata.ChangeMetadata.Schedule.BeginDate = metadata.ImplementationBeginDate
	meetingMetadata.ChangeMetadata.Schedule.BeginTime = metadata.ImplementationBeginTime
	meetingMetadata.ChangeMetadata.Schedule.EndDate = metadata.ImplementationEndDate
	meetingMetadata.ChangeMetadata.Schedule.EndTime = metadata.ImplementationEndTime
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
	meetingInvite := &struct {
		Title           string   `json:"title"`
		StartTime       string   `json:"startTime"`
		Duration        int      `json:"duration"`
		DurationMinutes int      `json:"durationMinutes"`
		Attendees       []string `json:"attendees"`
		Location        string   `json:"location"`
	}{
		Title:           meetingTitle,
		StartTime:       fmt.Sprintf("%sT%s:00", meetingDate, metadata.ImplementationBeginTime),
		Duration:        durationMinutes,
		DurationMinutes: durationMinutes,
		Location:        meetingLocation,
	}

	meetingMetadata.MeetingInvite = meetingInvite

	// Create temporary file
	tempFileName := fmt.Sprintf("/tmp/meeting-metadata-%s-%d.json", metadata.ChangeID, time.Now().Unix())

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(meetingMetadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal meeting metadata: %w", err)
	}

	// Write to temporary file
	err = os.WriteFile(tempFileName, jsonData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write temporary meeting metadata file: %w", err)
	}

	log.Printf("📄 Created temporary meeting metadata file: %s", tempFileName)
	return tempFileName, nil
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
        <div class="info-grid">
            <div class="info-label">Title:</div>
            <div>%s</div>
            <div class="info-label">Customer:</div>
            <div>%s</div>
        </div>
       
        <div class="tickets">
            <strong>Tracking Numbers:</strong><br>
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
		metadata.ChangeTitle,
		strings.Join(getCustomerNames(metadata.Customers), ", "),
		metadata.SnowTicket,
		metadata.JiraTicket,
		formatScheduleDateTime(metadata.ImplementationBeginDate, metadata.ImplementationBeginTime),
		formatScheduleDateTime(metadata.ImplementationEndDate, metadata.ImplementationEndTime),
		metadata.Timezone,
		metadata.ChangeReason,
		strings.ReplaceAll(metadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.TestPlan, "\n", "<br>"),
		metadata.CustomerImpact,
		strings.ReplaceAll(metadata.RollbackPlan, "\n", "<br>"),
		metadata.CreatedAt,
	)
}

// generateAnnouncementHTML generates HTML content for approved announcement emails
func generateAnnouncementHTML(metadata *types.ChangeMetadata) string {
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
		formatScheduleDateTime(metadata.ImplementationBeginDate, metadata.ImplementationBeginTime),
		formatScheduleDateTime(metadata.ImplementationEndDate, metadata.ImplementationEndTime),
		metadata.Timezone,
		metadata.CustomerImpact,
		strings.ReplaceAll(metadata.RollbackPlan, "\n", "<br>"),
		metadata.SnowTicket,
		metadata.JiraTicket,
		metadata.CreatedAt,
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
		time.Now().Format("January 2, 2006 at 3:04 PM MST"),
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

// formatScheduleDateTime combines date and time for display
func formatScheduleDateTime(date, time string) string {
	if date == "" || time == "" {
		return "TBD"
	}
	return fmt.Sprintf("%s at %s", date, time)
}
