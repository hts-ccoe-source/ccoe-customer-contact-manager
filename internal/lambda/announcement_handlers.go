package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"ccoe-customer-contact-manager/internal/processors"
	"ccoe-customer-contact-manager/internal/typeform"
	"ccoe-customer-contact-manager/internal/types"
)

// handleAnnouncementEventNew processes an announcement object from S3 using AnnouncementMetadata
// This is the new implementation that works directly with AnnouncementMetadata
func handleAnnouncementEventNew(ctx context.Context, customerCode string, s3Bucket, s3Key string, cfg *types.Config) error {
	log.Printf("📢 Processing announcement event for customer %s from S3: %s/%s", customerCode, s3Bucket, s3Key)

	// Download announcement from S3
	announcement, err := downloadAnnouncementFromS3(ctx, s3Bucket, s3Key, cfg.AWSRegion)
	if err != nil {
		return fmt.Errorf("failed to download announcement from S3: %w", err)
	}

	// Validate announcement
	if err := validateAnnouncement(announcement); err != nil {
		return fmt.Errorf("invalid announcement: %w", err)
	}

	// Create AWS clients for the processor
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)
	sesClient := sesv2.NewFromConfig(awsCfg)

	// Get Microsoft Graph token (if needed for meeting scheduling)
	graphToken := "" // TODO: Implement token retrieval

	// Create announcement processor
	processor := processors.NewAnnouncementProcessor(s3Client, sesClient, graphToken, cfg)

	// Process the announcement
	return processor.ProcessAnnouncement(ctx, customerCode, announcement, s3Bucket, s3Key)
}

// downloadAnnouncementFromS3 downloads and parses announcement metadata from S3
func downloadAnnouncementFromS3(ctx context.Context, bucket, key, region string) (*types.AnnouncementMetadata, error) {
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

	// Read the content
	contentBytes, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object content: %w", err)
	}

	// Parse as AnnouncementMetadata
	var announcement types.AnnouncementMetadata
	if err := json.Unmarshal(contentBytes, &announcement); err != nil {
		return nil, fmt.Errorf("failed to parse announcement metadata: %w", err)
	}

	// Extract survey metadata from S3 object metadata if present
	if result.Metadata != nil {
		if surveyID, ok := result.Metadata["survey_id"]; ok {
			announcement.SurveyID = surveyID
		}
		if surveyURL, ok := result.Metadata["survey_url"]; ok {
			announcement.SurveyURL = surveyURL
		}
		if surveyCreatedAt, ok := result.Metadata["survey_created_at"]; ok {
			announcement.SurveyCreatedAt = surveyCreatedAt
		}
		if announcement.SurveyID != "" {
			log.Printf("📋 Survey metadata found for announcement: ID=%s", announcement.SurveyID)
		}
	}

	return &announcement, nil
}

// validateAnnouncement validates the announcement metadata structure
func validateAnnouncement(announcement *types.AnnouncementMetadata) error {
	if announcement == nil {
		return fmt.Errorf("announcement cannot be nil")
	}

	if strings.TrimSpace(announcement.AnnouncementID) == "" {
		return fmt.Errorf("announcement_id is required")
	}

	if strings.TrimSpace(announcement.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if strings.TrimSpace(announcement.AnnouncementType) == "" {
		return fmt.Errorf("announcement_type is required")
	}

	if !strings.HasPrefix(announcement.ObjectType, "announcement_") {
		return fmt.Errorf("object_type must start with 'announcement_', got: %s", announcement.ObjectType)
	}

	if len(announcement.Customers) == 0 {
		return fmt.Errorf("at least one customer is required")
	}

	// Validate that object doesn't contain legacy metadata map
	if err := announcement.ValidateLegacyMetadata(); err != nil {
		log.Printf("❌ Legacy metadata detected in announcement %s: %v", announcement.AnnouncementID, err)
		return fmt.Errorf("legacy metadata validation failed: %w", err)
	}

	return nil
}

// handleAnnouncementSubmitted processes a submitted announcement
func handleAnnouncementSubmitted(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, cfg *types.Config) error {
	log.Printf("📧 Handling submitted announcement %s for customer %s", announcement.AnnouncementID, customerCode)

	// Create AWS clients
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)
	sesClient := sesv2.NewFromConfig(awsCfg)

	// Create processor
	processor := processors.NewAnnouncementProcessor(s3Client, sesClient, "", cfg)

	// Send approval request
	return processor.ProcessAnnouncement(ctx, customerCode, announcement, "", "")
}

// handleAnnouncementApproved processes an approved announcement
func handleAnnouncementApproved(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string, cfg *types.Config) error {
	log.Printf("✅ Handling approved announcement %s for customer %s", announcement.AnnouncementID, customerCode)

	// Create AWS clients
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)
	sesClient := sesv2.NewFromConfig(awsCfg)

	// Get Microsoft Graph token (if needed)
	graphToken := "" // TODO: Implement token retrieval

	// Create processor
	processor := processors.NewAnnouncementProcessor(s3Client, sesClient, graphToken, cfg)

	// Process approved announcement (schedule meeting if needed, send emails)
	return processor.ProcessAnnouncement(ctx, customerCode, announcement, s3Bucket, s3Key)
}

// handleAnnouncementCancelled processes a cancelled announcement
func handleAnnouncementCancelled(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, s3Bucket, s3Key string, cfg *types.Config) error {
	log.Printf("❌ Handling cancelled announcement %s for customer %s", announcement.AnnouncementID, customerCode)

	// Create AWS clients
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)
	sesClient := sesv2.NewFromConfig(awsCfg)

	// Get Microsoft Graph token (if needed)
	graphToken := "" // TODO: Implement token retrieval

	// Create processor
	processor := processors.NewAnnouncementProcessor(s3Client, sesClient, graphToken, cfg)

	// Process cancelled announcement (cancel meeting if scheduled, send cancellation email)
	return processor.ProcessAnnouncement(ctx, customerCode, announcement, s3Bucket, s3Key)
}

// handleAnnouncementCompleted processes a completed announcement
func handleAnnouncementCompleted(ctx context.Context, customerCode string, announcement *types.AnnouncementMetadata, cfg *types.Config) error {
	log.Printf("🎉 Handling completed announcement %s for customer %s", announcement.AnnouncementID, customerCode)

	// Create AWS clients
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)
	sesClient := sesv2.NewFromConfig(awsCfg)

	// Create processor
	processor := processors.NewAnnouncementProcessor(s3Client, sesClient, "", cfg)

	// Process completed announcement (send completion email)
	return processor.ProcessAnnouncement(ctx, customerCode, announcement, "", "")
}

// CreateSurveyForCompletedAnnouncement creates a Typeform survey for a completed announcement
func CreateSurveyForCompletedAnnouncement(ctx context.Context, announcement *types.AnnouncementMetadata, cfg *types.Config, s3Bucket, s3Key string) error {
	log.Printf("🔍 Creating survey for completed announcement %s", announcement.AnnouncementID)

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

	// Determine survey type based on announcement type
	surveyType := determineSurveyTypeFromAnnouncement(announcement.ObjectType, announcement.AnnouncementType)

	// Extract metadata for survey creation
	year, quarter := extractYearQuarterFromTime(announcement.PostedDate)
	eventType := "announcement"
	eventSubtype := announcement.AnnouncementType

	// Get customer code from the first customer in the list
	customerCode := ""
	if len(announcement.Customers) > 0 {
		customerCode = announcement.Customers[0]
	}

	surveyMetadata := &typeform.SurveyMetadata{
		CustomerCode: customerCode,
		ObjectID:     announcement.AnnouncementID,
		ObjectTitle:  announcement.Title,
		Year:         year,
		Quarter:      quarter,
		EventType:    eventType,
		EventSubtype: eventSubtype,
	}

	// Create the survey
	response, err := typeformClient.CreateSurvey(ctx, s3Client, s3Bucket, surveyMetadata, surveyType)
	if err != nil {
		log.Printf("❌ Failed to create survey for announcement %s: %v", announcement.AnnouncementID, err)
		return fmt.Errorf("failed to create survey: %w", err)
	}

	log.Printf("✅ Successfully created survey for announcement %s: ID=%s", announcement.AnnouncementID, response.ID)
	return nil
}

// determineSurveyTypeFromAnnouncement determines the survey type based on announcement type
func determineSurveyTypeFromAnnouncement(objectType, announcementType string) typeform.SurveyType {
	// Check object type first
	switch objectType {
	case "announcement_cic":
		return typeform.SurveyTypeCIC
	case "announcement_innersource":
		return typeform.SurveyTypeInnerSource
	case "announcement_finops":
		return typeform.SurveyTypeFinOps
	case "announcement_general":
		return typeform.SurveyTypeGeneral
	}

	// Fallback to announcement type
	switch announcementType {
	case "cic":
		return typeform.SurveyTypeCIC
	case "innersource":
		return typeform.SurveyTypeInnerSource
	case "finops":
		return typeform.SurveyTypeFinOps
	default:
		return typeform.SurveyTypeGeneral
	}
}

// extractYearQuarterFromTime extracts year and quarter from a time.Time
func extractYearQuarterFromTime(t time.Time) (string, string) {
	year := t.Format("2006")
	quarter := fmt.Sprintf("Q%d", (int(t.Month())-1)/3+1)
	return year, quarter
}
