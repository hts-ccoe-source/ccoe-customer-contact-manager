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

	// Process each SQS message
	var processingErrors []error
	successCount := 0

	for i, record := range sqsEvent.Records {
		log.Printf("Processing message %d/%d: %s", i+1, len(sqsEvent.Records), record.MessageId)

		err := ProcessSQSRecord(ctx, record, cfg)
		if err != nil {
			log.Printf("Error processing message %s: %v", record.MessageId, err)
			processingErrors = append(processingErrors, fmt.Errorf("message %s: %w", record.MessageId, err))
		} else {
			successCount++
			log.Printf("Successfully processed message %s", record.MessageId)
		}
	}

	// Log summary
	log.Printf("Processing complete: %d successful, %d errors", successCount, len(processingErrors))

	// If any messages failed, return error (Lambda will retry)
	if len(processingErrors) > 0 {
		return fmt.Errorf("failed to process %d messages: %v", len(processingErrors), processingErrors[0])
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
		return fmt.Errorf("failed to parse message body: %w", err)
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
		// Process as legacy SQS message
		return ProcessSQSMessage(ctx, sqsMsg, cfg)
	} else {
		log.Printf("Failed to parse as legacy SQS message: %v", err)
	}

	log.Printf("Message body type: %T, content: %+v", messageBody, messageBody)
	return fmt.Errorf("unrecognized message format")
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
			return fmt.Errorf("failed to extract customer code from S3 key %s: %w", objectKey, err)
		}

		// Validate customer code
		if err := ValidateCustomerCode(customerCode, cfg); err != nil {
			return fmt.Errorf("invalid customer code %s: %w", customerCode, err)
		}

		// Download and process metadata
		metadata, err := DownloadMetadataFromS3(ctx, bucketName, objectKey, cfg.AWSRegion)
		if err != nil {
			return fmt.Errorf("failed to download metadata from S3: %w", err)
		}

		// Process the change request
		err = ProcessChangeRequest(ctx, customerCode, metadata, cfg)
		if err != nil {
			return fmt.Errorf("failed to process change request: %w", err)
		}

		log.Printf("Successfully processed S3 event for customer %s", customerCode)
	}

	return nil
}

// ProcessSQSMessage processes a legacy SQS message in Lambda context
func ProcessSQSMessage(ctx context.Context, sqsMsg types.SQSMessage, cfg *types.Config) error {
	// Validate the message
	if err := ValidateSQSMessage(sqsMsg); err != nil {
		return fmt.Errorf("invalid SQS message: %w", err)
	}

	// Validate customer code
	if err := ValidateCustomerCode(sqsMsg.CustomerCode, cfg); err != nil {
		return fmt.Errorf("invalid customer code %s: %w", sqsMsg.CustomerCode, err)
	}

	// Download metadata from S3
	metadata, err := DownloadMetadataFromS3(ctx, sqsMsg.S3Bucket, sqsMsg.S3Key, cfg.AWSRegion)
	if err != nil {
		return fmt.Errorf("failed to download metadata from S3: %w", err)
	}

	// Process the change request
	err = ProcessChangeRequest(ctx, sqsMsg.CustomerCode, metadata, cfg)
	if err != nil {
		return fmt.Errorf("failed to process change request: %w", err)
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

	// First, try to parse as standard ChangeMetadata
	var metadata types.ChangeMetadata
	if err := json.Unmarshal(contentBytes, &metadata); err == nil && metadata.ChangeID != "" {
		log.Printf("Successfully parsed as ChangeMetadata")
		return &metadata, nil
	}

	// If that fails, try to parse as ApprovalRequestMetadata
	var approvalMetadata types.ApprovalRequestMetadata
	if err := json.Unmarshal(contentBytes, &approvalMetadata); err == nil && approvalMetadata.ChangeMetadata.Title != "" {
		log.Printf("Successfully parsed as ApprovalRequestMetadata, converting to ChangeMetadata")
		// Convert ApprovalRequestMetadata to ChangeMetadata
		converted := ConvertApprovalRequestToChangeMetadata(&approvalMetadata)
		return converted, nil
	}

	// If both fail, return the original parsing error
	return nil, fmt.Errorf("failed to parse metadata as either ChangeMetadata or ApprovalRequestMetadata")
}

// ConvertApprovalRequestToChangeMetadata converts ApprovalRequestMetadata to ChangeMetadata
func ConvertApprovalRequestToChangeMetadata(approval *types.ApprovalRequestMetadata) *types.ChangeMetadata {
	// Generate a change ID if not present
	changeID := fmt.Sprintf("APPROVAL-%d", time.Now().Unix())

	metadata := &types.ChangeMetadata{
		ChangeID:           changeID,
		Title:              approval.ChangeMetadata.Title,
		Description:        approval.ChangeMetadata.Description,
		Customers:          approval.ChangeMetadata.CustomerCodes,
		ImplementationPlan: approval.ChangeMetadata.ImplementationPlan,
		Schedule: struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
		}{
			StartDate: approval.ChangeMetadata.Schedule.ImplementationStart,
			EndDate:   approval.ChangeMetadata.Schedule.ImplementationEnd,
		},
		Impact:            approval.ChangeMetadata.ExpectedCustomerImpact,
		RollbackPlan:      approval.ChangeMetadata.RollbackPlan,
		CommunicationPlan: "Approval request workflow",
		Approver:          "approval-team@hearst.com", // Default approver for approval requests
		Implementer:       approval.GeneratedBy,
		Timestamp:         approval.GeneratedAt,
		Source:            "approval_request", // Mark this as an approval request
		TestRun:           false,              // Approval requests are not test runs
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
		"change_id":            metadata.ChangeID,
		"title":                metadata.Title,
		"description":          metadata.Description,
		"implementation_plan":  metadata.ImplementationPlan,
		"schedule_start":       metadata.Schedule.StartDate,
		"schedule_end":         metadata.Schedule.EndDate,
		"impact":               metadata.Impact,
		"rollback_plan":        metadata.RollbackPlan,
		"communication_plan":   metadata.CommunicationPlan,
		"approver":             metadata.Approver,
		"implementer":          metadata.Implementer,
		"timestamp":            metadata.Timestamp,
		"source":               metadata.Source,
		"test_run":             metadata.TestRun,
		"customers":            metadata.Customers,
		"request_type":         requestType,
		"security_updated":     true,
		"billing_updated":      true,
		"operations_updated":   true,
		"processing_timestamp": time.Now(),
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
	case "contact_update":
		log.Printf("Sending contact update notification for customer %s", customerCode)
		// TODO: Implement contact update notification when email manager is available
		// This should send the alternate contact update notification
		log.Printf("Would send contact update notification for customer %s with change details: %+v", customerCode, changeDetails)
	default:
		log.Printf("Unknown request type %s, defaulting to contact update notification", requestType)
		log.Printf("Would send contact update notification for customer %s with change details: %+v", customerCode, changeDetails)
	}

	// If this is not a test run, we could also update alternate contacts here
	if !metadata.TestRun {
		log.Printf("Processing non-test change request - would update alternate contacts for customer %s", customerCode)
		// TODO: Add actual alternate contact update logic here if needed
	} else {
		log.Printf("Test run - skipping alternate contact updates for customer %s", customerCode)
	}

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
		if strings.Contains(source, "contact") || strings.Contains(source, "update") {
			return "contact_update"
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

	// Check if approver field is set (indicates approval workflow)
	if metadata.Approver != "" {
		return "approval_request"
	}

	// Default to contact_update if we can't determine
	return "contact_update"
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

	// Create the metadata structure
	metadata := &types.ApprovalRequestMetadata{
		ChangeMetadata: struct {
			Title         string   `json:"title"`
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
			Title:                  getString("title"),
			CustomerNames:          getStringSlice("customer_names"),
			CustomerCodes:          getStringSlice("customers"),
			ChangeReason:           getString("change_reason"),
			ImplementationPlan:     getString("implementation_plan"),
			TestPlan:               getString("test_plan"),
			ExpectedCustomerImpact: getString("impact"),
			RollbackPlan:           getString("rollback_plan"),
			Description:            getString("description"),
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
			Subject:       fmt.Sprintf("ITSM Change Notification: %s", getString("title")),
			CustomerNames: getStringSlice("customer_names"),
			CustomerCodes: getStringSlice("customers"),
		},
		GeneratedAt: getString("timestamp"),
		GeneratedBy: getString("implementer"),
	}

	// Set tickets
	metadata.ChangeMetadata.Tickets.ServiceNow = getString("meta_servicenow_ticket")
	metadata.ChangeMetadata.Tickets.Jira = getString("meta_jira_ticket")
	metadata.EmailNotification.Tickets.Snow = getString("meta_servicenow_ticket")
	metadata.EmailNotification.Tickets.Jira = getString("meta_jira_ticket")

	// Set schedule
	metadata.ChangeMetadata.Schedule.ImplementationStart = getString("schedule_start")
	metadata.ChangeMetadata.Schedule.ImplementationEnd = getString("schedule_end")
	metadata.ChangeMetadata.Schedule.BeginDate = getString("schedule_start")
	metadata.ChangeMetadata.Schedule.EndDate = getString("schedule_end")
	metadata.ChangeMetadata.Schedule.Timezone = "America/New_York" // Default timezone
	metadata.EmailNotification.ScheduledWindow.Start = getString("schedule_start")
	metadata.EmailNotification.ScheduledWindow.End = getString("schedule_end")

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

	// Create AWS config and SES client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	sesClient := sesv2.NewFromConfig(awsCfg)

	// Create ApprovalRequestMetadata from changeDetails
	metadata := createApprovalMetadataFromChangeDetails(changeDetails)

	// Create a temporary JSON file with the metadata for the SES function
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Write to a temporary file (in Lambda, we can use /tmp)
	tmpFile := "/tmp/approval_metadata.json"
	err = os.WriteFile(tmpFile, metadataJSON, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temporary metadata file: %w", err)
	}
	defer os.Remove(tmpFile) // Clean up

	// Use the existing SES function for approval requests
	topicName := "aws-approval"
	senderEmail := "aws-contact-manager@hearst.com" // Default sender

	// Use SendApprovalRequest from the SES package
	// Parameters: sesClient, topicName, jsonMetadataPath, htmlTemplatePath, senderEmail, dryRun
	return ses.SendApprovalRequest(sesClient, topicName, tmpFile, "", senderEmail, false)
}

// SendApprovedAnnouncementEmail sends approved announcement email using existing SES template system
func SendApprovedAnnouncementEmail(ctx context.Context, customerCode string, changeDetails map[string]interface{}, cfg *types.Config) error {
	log.Printf("Sending approved announcement email for customer %s", customerCode)

	// Create AWS config and SES client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	sesClient := sesv2.NewFromConfig(awsCfg)

	// Create ApprovalRequestMetadata from changeDetails
	metadata := createApprovalMetadataFromChangeDetails(changeDetails)

	// Create a temporary JSON file with the metadata for the SES function
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Write to a temporary file (in Lambda, we can use /tmp)
	tmpFile := "/tmp/announcement_metadata.json"
	err = os.WriteFile(tmpFile, metadataJSON, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temporary metadata file: %w", err)
	}
	defer os.Remove(tmpFile) // Clean up

	// Use the existing SES function for change notifications (announcements)
	topicName := "aws-announce"
	senderEmail := "aws-contact-manager@hearst.com" // Default sender

	// Use SendChangeNotificationWithTemplate from the SES package
	return ses.SendChangeNotificationWithTemplate(sesClient, topicName, tmpFile, senderEmail, false)
}

// generateApprovalRequestHTML generates HTML content for approval request emails
func generateApprovalRequestHTML(metadata *types.ApprovalRequestMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Approval Request</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #fff3cd; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #ffc107; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
        .highlight { background-color: #e7f3ff; padding: 10px; border-radius: 3px; }
        .ticket { display: inline-block; margin-right: 15px; padding: 5px 10px; background-color: #e9ecef; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>❓ CHANGE APPROVAL REQUEST</h2>
        <p>This change has been reviewed, tentatively scheduled, and is ready for your approval.</p>
    </div>
    
    <div class="section">
        <h3>📋 Change Details</h3>
        <p><strong>Title:</strong> %s</p>
        <p><strong>Customer(s):</strong> %s</p>
        <p><strong>Description:</strong> %s</p>
    </div>
    
    <div class="section">
        <h3>🔧 Implementation Plan</h3>
        <div class="highlight">%s</div>
    </div>
    
    <div class="section">
        <h3>📅 Proposed Schedule</h3>
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
    
    <div class="section" style="background-color: #d1ecf1; border-left: 4px solid #bee5eb;">
        <h3>✅ Action Required</h3>
        <p>Please review this change request and provide your approval or feedback.</p>
    </div>
    
    <hr style="margin: 30px 0;">
    <p style="font-size: 12px; color: #666;">
        This is an automated message from the AWS Contact Manager system.<br>
        Generated at: %s
    </p>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Description,
		strings.ReplaceAll(metadata.ChangeMetadata.ImplementationPlan, "\n", "<br>"),
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

// generateAnnouncementHTML generates HTML content for approved announcement emails
func generateAnnouncementHTML(metadata *types.ApprovalRequestMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Change Approved & Scheduled</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #d4edda; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #28a745; }
        .section { margin-bottom: 20px; padding: 15px; border-radius: 5px; background-color: #f8f9fa; }
        .highlight { background-color: #e7f3ff; padding: 10px; border-radius: 3px; }
        .ticket { display: inline-block; margin-right: 15px; padding: 5px 10px; background-color: #e9ecef; border-radius: 3px; }
        .approved { background-color: #d1ecf1; border-left: 4px solid #17a2b8; }
    </style>
</head>
<body>
    <div class="header">
        <h2>✅ CHANGE APPROVED & SCHEDULED</h2>
        <p>This change has been approved and is scheduled for implementation.</p>
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
    
    <div class="section" style="background-color: #fff3cd; border-left: 4px solid #ffc107;">
        <h3>📢 Next Steps</h3>
        <p>Implementation will proceed as scheduled. You will receive updates as the change progresses.</p>
    </div>
    
    <hr style="margin: 30px 0;">
    <p style="font-size: 12px; color: #666;">
        This is an automated message from the AWS Contact Manager system.<br>
        Generated at: %s
    </p>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		strings.Join(metadata.ChangeMetadata.CustomerNames, ", "),
		metadata.ChangeMetadata.Description,
		strings.ReplaceAll(metadata.ChangeMetadata.ImplementationPlan, "\n", "<br>"),
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

	// Default sender email - this should be configured
	senderEmail := "aws-contact-manager@hearst.com"

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
