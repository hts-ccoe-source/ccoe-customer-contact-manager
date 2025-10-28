package typeform

import (
	"bytes"
	"context"
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
	Title     string        `json:"title"`
	Type      string        `json:"type,omitempty"`
	Workspace *WorkspaceRef `json:"workspace,omitempty"`
	Theme     *Theme        `json:"theme,omitempty"`
	Fields    []Field       `json:"fields"`
	Hidden    []string      `json:"hidden,omitempty"`
}

// WorkspaceRef represents a reference to a Typeform workspace
type WorkspaceRef struct {
	Href string `json:"href"`
}

// Theme represents the form theme reference
type Theme struct {
	Href string `json:"href,omitempty"` // Reference to a theme with logo
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
	ObjectTitle  string // Title of the change or announcement
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
		c.logger.Warn("failed to retrieve logo, continuing without logo",
			"customer_code", metadata.CustomerCode,
			"error", err)
	}

	// 2. Create or get theme with logo
	var theme *Theme
	if len(logoData) > 0 {
		// Upload image to Typeform and get image ID and src URL
		fileName := fmt.Sprintf("%s-logo.png", metadata.CustomerCode)
		imageID, imageSrc, err := c.UploadImage(ctx, logoData, fileName)
		if err != nil {
			c.logger.Warn("failed to upload logo to typeform, continuing without logo",
				"customer_code", metadata.CustomerCode,
				"error", err)
		} else {
			c.logger.Info("logo uploaded to typeform",
				"customer_code", metadata.CustomerCode,
				"image_id", imageID,
				"image_src", imageSrc,
				"size_bytes", len(logoData))

			// Create theme with the logo using the image src URL
			// Theme name format: {event_type}-{event_subtype} (e.g., "announcement-cic", "change-general")
			themeName := fmt.Sprintf("%s-%s", metadata.EventType, metadata.EventSubtype)
			themeResponse, err := c.CreateTheme(ctx, themeName, imageSrc, surveyType)
			if err != nil {
				c.logger.Warn("failed to create theme, continuing without theme",
					"theme_name", themeName,
					"error", err)
			} else {
				// Use the theme href from the response
				if themeResponse.Links != nil {
					if href, ok := themeResponse.Links["self"]; ok {
						theme = &Theme{
							Href: href,
						}
						c.logger.Info("theme created and will be applied to form",
							"theme_id", themeResponse.ID,
							"theme_name", themeName)
					}
				}
			}
		}
	}

	// 3. Get survey template for type
	template := GetSurveyTemplate(surveyType)

	// 4. Customize the "Was this excellent?" question with the actual title
	fields := make([]Field, len(template.Fields))
	copy(fields, template.Fields)

	// Update the first field (yes/no question) with the title as description
	if len(fields) > 0 && metadata.ObjectTitle != "" {
		if fields[0].Properties == nil {
			fields[0].Properties = make(map[string]interface{})
		}
		fields[0].Properties["description"] = metadata.ObjectTitle
	}

	// 5. Build create request
	// Use ObjectTitle if available, otherwise fall back to ObjectID
	title := metadata.ObjectTitle
	if title == "" {
		title = metadata.ObjectID
	}

	// Determine workspace based on survey type
	workspaceID := getWorkspaceNameForSurveyType(surveyType)
	var workspace *WorkspaceRef
	if workspaceID != "" {
		workspace = &WorkspaceRef{
			Href: fmt.Sprintf("https://api.typeform.com/workspaces/%s", workspaceID),
		}
	}

	request := &CreateFormRequest{
		Title:     title,
		Type:      "form",
		Workspace: workspace,
		Theme:     theme, // Include theme with logo if available
		Fields:    fields,
		Hidden: []string{
			"user_login",
			"customer_code",
			"year",
			"quarter",
			"event_type",
			"event_subtype",
			"object_id",
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

	// Check content type to ensure it's an image
	contentType := ""
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	// Validate it's an image type
	validImageTypes := []string{"image/png", "image/jpeg", "image/jpg", "image/gif", "image/svg+xml"}
	isValidImage := false
	for _, validType := range validImageTypes {
		if contentType == validType {
			isValidImage = true
			break
		}
	}

	if !isValidImage {
		return nil, fmt.Errorf("invalid image content type: %s (expected image/png, image/jpeg, etc.)", contentType)
	}

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

	c.logger.Debug("retrieved image from s3",
		"key", key,
		"content_type", contentType,
		"size_bytes", len(data))

	return data, nil
}

// storeSurveyForm stores the survey form definition in S3
func (c *Client) storeSurveyForm(ctx context.Context, s3Client *s3.Client, bucketName, customerCode, objectID string, survey *CreateFormResponse) error {
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("surveys/forms/%s/%s/%d-%s.json", customerCode, objectID, timestamp, survey.ID)

	// Use MarshalIndent for pretty-printed JSON
	data, err := json.MarshalIndent(survey, "", "  ")
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
	// The object is stored in the archive folder
	key := fmt.Sprintf("archive/%s.json", objectID)
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

// workspaceNames maps survey types to their Typeform workspace IDs
var workspaceNames = map[SurveyType]string{
	SurveyTypeChange:      "7zvRPv", // Changes workspace
	SurveyTypeCIC:         "SUXyVp", // CIC workspace
	SurveyTypeInnerSource: "8sJuTN", // InnerSource workspace
	SurveyTypeFinOps:      "uVr3cK", // FinOps workspace
	SurveyTypeGeneral:     "Apxhwx", // General workspace
}

// getWorkspaceNameForSurveyType returns the workspace name for a given survey type
func getWorkspaceNameForSurveyType(surveyType SurveyType) string {
	if name, ok := workspaceNames[surveyType]; ok {
		return name
	}
	return "general" // Default to general workspace
}

// themeIDs maps event type/subtype combinations to pre-created Typeform theme IDs
// These themes must be created manually in the Typeform UI with logos
// Theme naming convention: {event_type}-{event_subtype}
var themeIDs = map[string]string{
	"change-general":           "", // TODO: Create theme in Typeform UI and add ID here
	"announcement-cic":         "", // TODO: Create theme in Typeform UI and add ID here
	"announcement-finops":      "", // TODO: Create theme in Typeform UI and add ID here
	"announcement-innersource": "", // TODO: Create theme in Typeform UI and add ID here
	"announcement-general":     "", // TODO: Create theme in Typeform UI and add ID here
}

// getThemeIDForEventType returns the theme ID for a given event type/subtype
func getThemeIDForEventType(eventType, eventSubtype string) string {
	key := fmt.Sprintf("%s-%s", eventType, eventSubtype)
	if themeID, ok := themeIDs[key]; ok && themeID != "" {
		return themeID
	}
	return "" // No theme configured
}
