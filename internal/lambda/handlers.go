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

	awsinternal "aws-alternate-contact-manager/internal/aws"
	"aws-alternate-contact-manager/internal/config"
	"aws-alternate-contact-manager/internal/ses"
	"aws-alternate-contact-manager/internal/types"
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

		log.Printf("Processing S3 event: s3://%s/%s", bucketName, objectKey)

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
			log.Printf("Found request-type in S3 metadata: %s", requestTypeFromS3)
		}
		if status, exists := result.Metadata["status"]; exists {
			log.Printf("Found status in S3 metadata: %s", status)
		}
	}

	// First, try to parse as standard ChangeMetadata (flat structure from frontend)
	var metadata types.ChangeMetadata
	if err := json.Unmarshal(contentBytes, &metadata); err == nil {
		// Validate that we have essential fields for a valid ChangeMetadata
		if metadata.ChangeID != "" || metadata.ChangeTitle != "" {
			log.Printf("Successfully parsed as flat ChangeMetadata structure")

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
	} else {
		log.Printf("Failed to parse as flat ChangeMetadata: %v", err)
	}

	// If that fails, try to parse as ApprovalRequestMetadata (nested structure)
	var approvalMetadata types.ApprovalRequestMetadata
	if err := json.Unmarshal(contentBytes, &approvalMetadata); err == nil && approvalMetadata.ChangeMetadata.Title != "" {
		log.Printf("Successfully parsed as ApprovalRequestMetadata, converting to ChangeMetadata")
		// Convert ApprovalRequestMetadata to ChangeMetadata
		converted := ConvertApprovalRequestToChangeMetadata(&approvalMetadata)

		// Apply request type from S3 metadata if available
		if requestTypeFromS3 != "" {
			if converted.Metadata == nil {
				converted.Metadata = make(map[string]interface{})
			}
			converted.Metadata["request_type"] = requestTypeFromS3
			log.Printf("Set request_type from S3 metadata: %s", requestTypeFromS3)
		}

		return converted, nil
	} else {
		log.Printf("Failed to parse as ApprovalRequestMetadata: %v", err)
	}

	// If both fail, return a detailed error with the content for debugging
	return nil, fmt.Errorf("failed to parse metadata as either ChangeMetadata or ApprovalRequestMetadata. Content: %s", string(contentBytes))
}

// ConvertApprovalRequestToChangeMetadata converts ApprovalRequestMetadata to ChangeMetadata
func ConvertApprovalRequestToChangeMetadata(approval *types.ApprovalRequestMetadata) *types.ChangeMetadata {
	// Generate a change ID if not present
	changeID := fmt.Sprintf("APPROVAL-%d", time.Now().Unix())

	metadata := &types.ChangeMetadata{
		ChangeID:                changeID,
		ChangeTitle:             approval.ChangeMetadata.Title,
		ChangeReason:            approval.ChangeMetadata.Description,
		Customers:               approval.ChangeMetadata.CustomerCodes,
		ImplementationPlan:      approval.ChangeMetadata.ImplementationPlan,
		TestPlan:                approval.ChangeMetadata.TestPlan,
		CustomerImpact:          approval.ChangeMetadata.ExpectedCustomerImpact,
		RollbackPlan:            approval.ChangeMetadata.RollbackPlan,
		SnowTicket:              approval.ChangeMetadata.Tickets.ServiceNow,
		JiraTicket:              approval.ChangeMetadata.Tickets.Jira,
		ImplementationBeginDate: approval.ChangeMetadata.Schedule.BeginDate,
		ImplementationBeginTime: approval.ChangeMetadata.Schedule.BeginTime,
		ImplementationEndDate:   approval.ChangeMetadata.Schedule.EndDate,
		ImplementationEndTime:   approval.ChangeMetadata.Schedule.EndTime,
		Timezone:                approval.ChangeMetadata.Schedule.Timezone,
		Status:                  "submitted",
		Version:                 1,
		CreatedAt:               approval.GeneratedAt,
		CreatedBy:               approval.GeneratedBy,
		ModifiedAt:              approval.GeneratedAt,
		ModifiedBy:              approval.GeneratedBy,
		SubmittedAt:             approval.GeneratedAt,
		SubmittedBy:             approval.GeneratedBy,
		Source:                  "approval_request", // Mark this as an approval request
		TestRun:                 false,              // Approval requests are not test runs
		Metadata: map[string]interface{}{
			"request_type":      "approval_request",
			"original_format":   "ApprovalRequestMetadata",
			"jira_ticket":       approval.ChangeMetadata.Tickets.Jira,
			"servicenow_ticket": approval.ChangeMetadata.Tickets.ServiceNow,
			"test_plan":         approval.ChangeMetadata.TestPlan,
			"customer_names":    approval.ChangeMetadata.CustomerNames,
			"generated_by":      approval.GeneratedBy,
			"generated_at":      approval.GeneratedAt,
		},
	}

	// Add meeting invite information if present
	if approval.MeetingInvite != nil {
		metadata.Metadata["meeting_required"] = true
		metadata.Metadata["meeting_title"] = approval.MeetingInvite.Title
		metadata.Metadata["meeting_start_time"] = approval.MeetingInvite.StartTime
		metadata.Metadata["meeting_duration"] = approval.MeetingInvite.Duration
		metadata.Metadata["meeting_attendees"] = approval.MeetingInvite.Attendees
		metadata.Metadata["meeting_location"] = approval.MeetingInvite.Location
	}

	return metadata
}

// ProcessChangeRequest processes a change request with metadata
func ProcessChangeRequest(ctx context.Context, customerCode string, metadata *types.ChangeMetadata, cfg *types.Config) error {
	log.Printf("Processing change request %s for customer %s", metadata.ChangeID, customerCode)

	// Determine the request type based on the metadata structure and source
	requestType := DetermineRequestType(metadata)
	log.Printf("Determined request type: %s", requestType)

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
		log.Printf("Sending approval request email for customer %s", customerCode)
		err := SendApprovalRequestEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("Failed to send approval request email for customer %s: %v", customerCode, err)
		} else {
			log.Printf("Successfully sent approval request email for customer %s", customerCode)
		}
	case "approved_announcement":
		log.Printf("Sending approved announcement email for customer %s", customerCode)
		err := SendApprovedAnnouncementEmail(ctx, customerCode, changeDetails, cfg)
		if err != nil {
			log.Printf("Failed to send approved announcement email for customer %s: %v", customerCode, err)
		} else {
			log.Printf("Successfully sent approved announcement email for customer %s", customerCode)
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
	// Check status field in metadata map first for common cases
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
			}
		}
	}

	// Check the source field
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

	// Check metadata for approval-related fields
	if metadata.Metadata != nil {
		if requestType, exists := metadata.Metadata["request_type"]; exists {
			if rt, ok := requestType.(string); ok {
				return strings.ToLower(rt)
			}
		}

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

	// Default to approval_request for unknown cases (most common workflow)
	return "approval_request"
}

// createApprovalMetadataFromChangeDetails converts changeDetails map to ApprovalRequestMetadata
func createApprovalMetadataFromChangeDetails(changeDetails map[string]interface{}) *types.ApprovalRequestMetadata {
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

	// Helper function to convert customer codes to friendly names
	getCustomerNames := func() []string {
		customerCodes := getStringSlice("customers")
		if len(customerCodes) == 0 {
			return []string{}
		}

		// Customer code to friendly name mapping
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

	// Create the metadata structure
	metadata := &types.ApprovalRequestMetadata{
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
		}{
			Title:                  getString("changeTitle"),
			CustomerNames:          getCustomerNames(),
			CustomerCodes:          getStringSlice("customers"),
			ChangeReason:           getString("changeReason"),
			ImplementationPlan:     getString("implementationPlan"),
			TestPlan:               getString("testPlan"),
			ExpectedCustomerImpact: getString("customerImpact"),
			RollbackPlan:           getString("rollbackPlan"),
			Description:            getString("changeReason"),
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
			Subject:       fmt.Sprintf("ITSM Change Notification: %s", getString("changeTitle")),
			CustomerNames: getCustomerNames(),
			CustomerCodes: getStringSlice("customers"),
		},
		GeneratedAt: getString("timestamp"),
		GeneratedBy: getString("implementer"),
	}

	// Set tickets - use consistent field names
	metadata.ChangeMetadata.Tickets.ServiceNow = getString("snowTicket")
	metadata.ChangeMetadata.Tickets.Jira = getString("jiraTicket")
	metadata.EmailNotification.Tickets.Snow = getString("snowTicket")
	metadata.EmailNotification.Tickets.Jira = getString("jiraTicket")

	// Set schedule
	metadata.ChangeMetadata.Schedule.ImplementationStart = getString("implementationBeginDate") + "T" + getString("implementationBeginTime")
	metadata.ChangeMetadata.Schedule.ImplementationEnd = getString("implementationEndDate") + "T" + getString("implementationEndTime")
	metadata.ChangeMetadata.Schedule.BeginDate = getString("implementationBeginDate")
	metadata.ChangeMetadata.Schedule.BeginTime = getString("implementationBeginTime")
	metadata.ChangeMetadata.Schedule.EndDate = getString("implementationEndDate")
	metadata.ChangeMetadata.Schedule.EndTime = getString("implementationEndTime")
	metadata.ChangeMetadata.Schedule.Timezone = getString("timezone")
	if metadata.ChangeMetadata.Schedule.Timezone == "" {
		metadata.ChangeMetadata.Schedule.Timezone = "America/New_York" // Default timezone
	}
	metadata.EmailNotification.ScheduledWindow.Start = metadata.ChangeMetadata.Schedule.ImplementationStart
	metadata.EmailNotification.ScheduledWindow.End = metadata.ChangeMetadata.Schedule.ImplementationEnd

	// Add meeting invite if present
	if getString("meta_meeting_required") == "true" {
		metadata.MeetingInvite = &struct {
			Title           string   `json:"title"`
			StartTime       string   `json:"startTime"`
			Duration        int      `json:"duration"`
			DurationMinutes int      `json:"durationMinutes"`
			Attendees       []string `json:"attendees"`
			Location        string   `json:"location"`
		}{
			Title:           getString("meta_meeting_title"),
			StartTime:       getString("meta_meeting_start_time"),
			DurationMinutes: 60, // Default duration
			Location:        getString("meta_meeting_location"),
		}
		if metadata.MeetingInvite.Location == "" {
			metadata.MeetingInvite.Location = "Microsoft Teams"
		}
	}

	return metadata
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

	// Create a temporary JSON file with the change details for the SES function
	tempFile, err := createTempMetadataFile(changeDetails)
	if err != nil {
		return fmt.Errorf("failed to create temporary metadata file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up temp file

	// Load metadata using the format converter directly
	metadata, err := ses.LoadMetadataFromFile(tempFile)
	if err != nil {
		return fmt.Errorf("failed to load metadata from temp file: %w", err)
	}

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

	// Create a temporary JSON file with the change details for the SES function
	tempFile, err := createTempMetadataFile(changeDetails)
	if err != nil {
		return fmt.Errorf("failed to create temporary metadata file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up temp file

	// Load metadata using the format converter directly
	metadata, err := ses.LoadMetadataFromFile(tempFile)
	if err != nil {
		return fmt.Errorf("failed to load metadata from temp file: %w", err)
	}

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

// generateApprovalRequestHTML generates HTML content for approval request emails
func generateApprovalRequestHTML(metadata *types.ApprovalRequestMetadata) string {
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
        <p>This is an automated notification from the AWS Alternate Contact Manager.</p>
        <p>Generated at: %s</p>
        <div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		metadata.ChangeMetadata.Schedule.ImplementationStart,
		metadata.ChangeMetadata.Schedule.ImplementationEnd,
		metadata.ChangeMetadata.Schedule.Timezone,
		metadata.ChangeMetadata.Description,
		strings.ReplaceAll(metadata.ChangeMetadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.TestPlan, "\n", "<br>"),
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		strings.ReplaceAll(metadata.ChangeMetadata.RollbackPlan, "\n", "<br>"),
		metadata.GeneratedAt,
	)
}

// generateAnnouncementHTML generates HTML content for approved announcement emails
func generateAnnouncementHTML(metadata *types.ApprovalRequestMetadata) string {
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
        <p>This change has been approved and is scheduled for implementation during the specified window.<br>You will receive additional notifications as the implementation progresses.</p>
    </div>
    
    <div class="section approved">
        <h3>📋 Change Details</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer(s):</strong> %s</p>
        <p><strong>Description:</strong> %s</p>
        <p><strong>Status:</strong> <span style="color: #28a745; font-weight: bold;">APPROVED</span></p>
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
        <p>Implementation will proceed as scheduled. You will receive updates as the change progresses.</p>
    </div>
    
    <div class="unsubscribe" style="background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px;">
        <p>This is an automated notification from the AWS Alternate Contact Manager.</p>
        <p>Generated at: %s</p>
        <div class="unsubscribe-prominent" style="margin-top: 10px;"><a href="{{amazonSESUnsubscribeUrl}}" style="color: #28a745; text-decoration: none; font-weight: bold;">📧 Manage Email Preferences or Unsubscribe</a></div>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Description,
		strings.ReplaceAll(metadata.ChangeMetadata.ImplementationPlan, "\n", "<br>"),
		strings.ReplaceAll(metadata.ChangeMetadata.TestPlan, "\n", "<br>"),
		metadata.ChangeMetadata.Schedule.ImplementationStart,
		metadata.ChangeMetadata.Schedule.ImplementationEnd,
		metadata.ChangeMetadata.Schedule.Timezone,
		metadata.ChangeMetadata.ExpectedCustomerImpact,
		strings.ReplaceAll(metadata.ChangeMetadata.RollbackPlan, "\n", "<br>"),
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira,
		metadata.GeneratedAt,
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

// createTempMetadataFile creates a temporary JSON file with change details for SES functions
func createTempMetadataFile(changeDetails map[string]interface{}) (string, error) {
	// Convert changeDetails to ApprovalRequestMetadata format for SES functions
	metadata := createApprovalMetadataFromChangeDetails(changeDetails)

	// Create temporary file in /tmp (writable in Lambda)
	tempFile, err := os.CreateTemp("/tmp", "change-metadata-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Write metadata to file
	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write metadata to temp file: %w", err)
	}

	// Return the full absolute path since we'll use LoadMetadataFromFile directly
	return tempFile.Name(), nil
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
func sendApprovalRequestEmailDirect(sesClient *sesv2.Client, topicName, senderEmail string, metadata *types.ApprovalRequestMetadata) error {
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
	subject := fmt.Sprintf("❓ APPROVAL REQUEST: %s", metadata.ChangeMetadata.Title)

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
func sendApprovedAnnouncementEmailDirect(sesClient *sesv2.Client, topicName, senderEmail string, metadata *types.ApprovalRequestMetadata) error {
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
	originalSubject := fmt.Sprintf("ITSM Change Notification: %s", metadata.ChangeMetadata.Title)
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
