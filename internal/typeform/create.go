package typeform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// CreateFormRequest represents the Typeform Create API request
type CreateFormRequest struct {
	Title  string   `json:"title"`
	Type   string   `json:"type,omitempty"`
	Theme  *Theme   `json:"theme,omitempty"`
	Fields []Field  `json:"fields"`
	Hidden []string `json:"hidden,omitempty"`
}

// Theme represents the form theme with logo
type Theme struct {
	Logo *Logo `json:"logo,omitempty"`
}

// Logo represents the base64-encoded logo
type Logo struct {
	Image string `json:"image"` // base64-encoded image data
}

// CreateFormResponse represents the Typeform Create API response
type CreateFormResponse struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Type      string                 `json:"type"`
	Workspace map[string]interface{} `json:"workspace,omitempty"`
	Links     map[string]string      `json:"_links,omitempty"`
	Theme     map[string]interface{} `json:"theme,omitempty"`
	Fields    []Field                `json:"fields,omitempty"`
	Hidden    []string               `json:"hidden,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

// SurveyMetadata contains metadata for survey creation
type SurveyMetadata struct {
	CustomerCode string
	ObjectID     string
	Year         string
	Quarter      string
	EventType    string
	EventSubtype string
}

// CreateSurvey creates a Typeform survey for a completed object
func (c *Client) CreateSurvey(ctx context.Context, s3Client *s3.Client, bucketName string, metadata *SurveyMetadata, surveyType SurveyType) (*CreateFormResponse, error) {
	// 1. Retrieve customer logo from S3 with fallback to default
	logoData, err := c.getCustomerLogoWithFallback(ctx, s3Client, bucketName, metadata.CustomerCode)
	if err != nil {
		c.logger.Error("failed to get logo, survey will be created without logo",
			"customer_code", metadata.CustomerCode,
			"error", err)
		// Continue without logo rather than failing
		logoData = nil
	}

	// 2. Base64 encode logo if available
	var theme *Theme
	if logoData != nil {
		logoBase64 := base64.StdEncoding.EncodeToString(logoData)
		theme = &Theme{
			Logo: &Logo{
				Image: logoBase64,
			},
		}
	}

	// 3. Get survey template for type
	template := GetSurveyTemplate(surveyType)

	// 4. Build create request
	request := &CreateFormRequest{
		Title:  fmt.Sprintf("%s Feedback - %s", surveyType, metadata.ObjectID),
		Type:   "form",
		Theme:  theme,
		Fields: template.Fields,
		Hidden: []string{
			"user_login",
			"customer_code",
			"year",
			"quarter",
			"event_type",
			"event_subtype",
		},
	}

	// 5. Call Typeform Create API
	response, err := c.CreateForm(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create survey: %w", err)
	}

	// 6. Store survey form in S3
	if err := c.storeSurveyForm(ctx, s3Client, bucketName, metadata.CustomerCode, metadata.ObjectID, response); err != nil {
		c.logger.Error("failed to store survey form",
			"customer_code", metadata.CustomerCode,
			"object_id", metadata.ObjectID,
			"survey_id", response.ID,
			"error", err)
		// Don't fail the entire operation if storage fails
	}

	// 7. Update S3 object metadata with survey info
	surveyURL := ""
	if response.Links != nil {
		surveyURL = response.Links["display"]
	}
	if err := c.updateObjectMetadata(ctx, s3Client, bucketName, metadata.ObjectID, response.ID, surveyURL); err != nil {
		c.logger.Error("failed to update object metadata",
			"object_id", metadata.ObjectID,
			"survey_id", response.ID,
			"error", err)
		// Don't fail the entire operation if metadata update fails
	}

	return response, nil
}

// getCustomerLogoWithFallback retrieves customer logo with fallback to default
func (c *Client) getCustomerLogoWithFallback(ctx context.Context, s3Client *s3.Client, bucketName, customerCode string) ([]byte, error) {
	// Try customer-specific logo
	logoData, err := c.getCustomerLogo(ctx, s3Client, bucketName, customerCode)
	if err != nil {
		c.logger.Debug("customer logo not found, using default",
			"customer_code", customerCode,
			"error", err)
		// Fallback to default logo
		logoData, err = c.getDefaultLogo(ctx, s3Client, bucketName)
		if err != nil {
			return nil, fmt.Errorf("failed to get default logo: %w", err)
		}
	}
	return logoData, nil
}

// getCustomerLogo retrieves customer logo from S3
func (c *Client) getCustomerLogo(ctx context.Context, s3Client *s3.Client, bucketName, customerCode string) ([]byte, error) {
	// Try common image extensions
	extensions := []string{"png", "jpg", "jpeg", "gif", "svg"}

	for _, ext := range extensions {
		key := fmt.Sprintf("customers/%s/logo.%s", customerCode, ext)
		data, err := c.getS3Object(ctx, s3Client, bucketName, key)
		if err == nil {
			c.logger.Debug("customer logo found",
				"customer_code", customerCode,
				"key", key)
			return data, nil
		}
	}

	return nil, fmt.Errorf("no customer logo found for %s", customerCode)
}

// getDefaultLogo retrieves default placeholder logo
func (c *Client) getDefaultLogo(ctx context.Context, s3Client *s3.Client, bucketName string) ([]byte, error) {
	key := "assets/images/default-logo.png"
	return c.getS3Object(ctx, s3Client, bucketName, key)
}

// getS3Object retrieves an object from S3
func (c *Client) getS3Object(ctx context.Context, s3Client *s3.Client, bucketName, key string) ([]byte, error) {
	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get s3 object %s: %w", key, err)
	}
	defer result.Body.Close()

	data := make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, err := result.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return data, nil
}

// storeSurveyForm stores the survey form definition in S3
func (c *Client) storeSurveyForm(ctx context.Context, s3Client *s3.Client, bucketName, customerCode, objectID string, survey *CreateFormResponse) error {
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("surveys/forms/%s/%s/%d-%s.json", customerCode, objectID, timestamp, survey.ID)

	data, err := json.Marshal(survey)
	if err != nil {
		return fmt.Errorf("failed to marshal survey form: %w", err)
	}

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("failed to put s3 object: %w", err)
	}

	c.logger.Info("survey form stored in s3",
		"key", key,
		"survey_id", survey.ID)

	return nil
}

// updateObjectMetadata updates S3 object metadata with survey info
func (c *Client) updateObjectMetadata(ctx context.Context, s3Client *s3.Client, bucketName, objectID, surveyID, surveyURL string) error {
	// Get current object to preserve existing metadata
	key := objectID
	getResult, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Copy existing metadata and add survey info
	metadata := make(map[string]string)
	for k, v := range getResult.Metadata {
		metadata[k] = v
	}
	metadata["survey_id"] = surveyID
	metadata["survey_url"] = surveyURL
	metadata["survey_created_at"] = time.Now().Format(time.RFC3339)

	// Copy object to itself with updated metadata
	_, err = s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(bucketName),
		Key:               aws.String(key),
		CopySource:        aws.String(fmt.Sprintf("%s/%s", bucketName, key)),
		Metadata:          metadata,
		MetadataDirective: types.MetadataDirectiveReplace,
	})
	if err != nil {
		return fmt.Errorf("failed to update object metadata: %w", err)
	}

	c.logger.Info("object metadata updated with survey info",
		"object_id", objectID,
		"survey_id", surveyID)

	return nil
}

// GetBucketName returns the S3 bucket name from environment
func GetBucketName() string {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		bucket = "4cm-prod-ccoe-change-management-metadata"
	}
	return bucket
}
