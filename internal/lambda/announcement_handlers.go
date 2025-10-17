package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"ccoe-customer-contact-manager/internal/processors"
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
