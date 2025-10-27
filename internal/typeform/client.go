package typeform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const (
	// TypeformAPIBaseURL is the base URL for Typeform API
	TypeformAPIBaseURL = "https://api.typeform.com"

	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
)

// Client represents a Typeform API client
type Client struct {
	apiToken       string
	httpClient     *http.Client
	logger         *slog.Logger
	workspaceCache map[string]string // Cache of workspace name -> ID
	themeCache     map[string]string // Cache of theme name -> theme href
}

// NewClient creates a new Typeform API client
func NewClient(logger *slog.Logger) (*Client, error) {
	apiToken := os.Getenv("TYPEFORM_API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("TYPEFORM_API_TOKEN environment variable not set")
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		logger:         logger,
		workspaceCache: make(map[string]string),
		themeCache:     make(map[string]string),
	}, nil
}

// doRequest performs an HTTP request with authentication and error handling
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := TypeformAPIBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("making typeform api request",
		"method", method,
		"url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for API errors
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.Error("typeform api error",
			"status", resp.StatusCode,
			"response", string(bodyBytes))
		return nil, fmt.Errorf("typeform api error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

// ImageUploadRequest represents a request to upload an image to Typeform
type ImageUploadRequest struct {
	Image    string `json:"image"`     // Base64-encoded image data (without data:image prefix)
	FileName string `json:"file_name"` // Optional filename
}

// ImageUploadResponse represents the response from uploading an image
type ImageUploadResponse struct {
	ID  string `json:"id"`
	Src string `json:"src"`
}

// CreateThemeRequest represents a request to create a theme
type CreateThemeRequest struct {
	Name       string           `json:"name"`
	Background *ThemeBackground `json:"background,omitempty"`
	Colors     *ThemeColors     `json:"colors,omitempty"`
	Fields     *ThemeFields     `json:"fields,omitempty"`
	Screens    *ThemeScreens    `json:"screens,omitempty"`
	Font       string           `json:"font,omitempty"`
}

// ThemeBackground represents theme background settings
type ThemeBackground struct {
	Href string `json:"href,omitempty"` // URL reference to background image
}

// ThemeColors represents theme color settings
type ThemeColors struct {
	Answer     string `json:"answer,omitempty"`
	Background string `json:"background,omitempty"`
	Button     string `json:"button,omitempty"`
	Question   string `json:"question,omitempty"`
}

// ThemeFields represents theme field settings
type ThemeFields struct {
	Alignment string `json:"alignment"` // Required: "left" or "center"
	FontSize  string `json:"font_size"` // Required: "small", "medium", or "large"
}

// ThemeScreens represents theme screen settings for welcome and thankyou screens
type ThemeScreens struct {
	Alignment string `json:"alignment"` // Required: "left" or "center"
	FontSize  string `json:"font_size"` // Required: "small", "medium", or "large"
}

// CreateThemeResponse represents the response from creating a theme
type CreateThemeResponse struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Background map[string]interface{} `json:"background,omitempty"`
	Colors     map[string]interface{} `json:"colors,omitempty"`
	Font       string                 `json:"font,omitempty"`
	Links      map[string]string      `json:"_links,omitempty"`
}

// UploadImage uploads an image to Typeform and returns the image ID and src URL
func (c *Client) UploadImage(ctx context.Context, imageData []byte, fileName string) (string, string, error) {
	// Base64 encode the image data
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	request := &ImageUploadRequest{
		Image:    imageBase64,
		FileName: fileName,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/images", request)
	if err != nil {
		return "", "", fmt.Errorf("failed to upload image: %w", err)
	}
	defer resp.Body.Close()

	var result ImageUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to decode image upload response: %w", err)
	}

	c.logger.Info("image uploaded to typeform",
		"image_id", result.ID,
		"image_src", result.Src,
		"file_name", fileName)

	return result.ID, result.Src, nil
}

// ListThemesResponse represents the response from listing themes
type ListThemesResponse struct {
	Items      []ThemeItem `json:"items"`
	PageCount  int         `json:"page_count"`
	TotalItems int         `json:"total_items"`
}

// ThemeItem represents a theme in the list response
type ThemeItem struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Links map[string]string `json:"_links,omitempty"`
}

// ListThemes lists all themes in the account
func (c *Client) ListThemes(ctx context.Context) (*ListThemesResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/themes", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list themes: %w", err)
	}
	defer resp.Body.Close()

	var result ListThemesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode themes list response: %w", err)
	}

	return &result, nil
}

// FindThemeByName searches for a theme by name and returns its href
func (c *Client) FindThemeByName(ctx context.Context, name string) (string, error) {
	// Check cache first
	if href, ok := c.themeCache[name]; ok {
		c.logger.Debug("theme found in cache", "name", name, "href", href)
		return href, nil
	}

	// List all themes
	themes, err := c.ListThemes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list themes: %w", err)
	}

	// Search for theme by name
	for _, theme := range themes.Items {
		if theme.Name == name {
			if theme.Links != nil {
				if href, ok := theme.Links["self"]; ok {
					// Cache the result
					c.themeCache[name] = href
					c.logger.Info("theme found", "name", name, "theme_id", theme.ID)
					return href, nil
				}
			}
		}
	}

	return "", nil // Theme not found
}

// getColorForSurveyType returns the color that matches the email header color for each survey type
func getColorForSurveyType(surveyType SurveyType) string {
	switch surveyType {
	case SurveyTypeChange:
		return "#6c757d" // Gray - matches change emails
	case SurveyTypeCIC:
		return "#0066cc" // Blue - matches CIC announcement emails
	case SurveyTypeFinOps:
		return "#28a745" // Green - matches FinOps announcement emails
	case SurveyTypeInnerSource:
		return "#6f42c1" // Purple - matches InnerSource announcement emails
	case SurveyTypeGeneral:
		return "#007bff" // Light blue - matches general announcement emails
	default:
		return "#4FB0AE" // Teal - fallback color
	}
}

// CreateTheme creates a new theme with a logo and colors matching the survey type
// This function is idempotent - it will reuse an existing theme with the same name if found
func (c *Client) CreateTheme(ctx context.Context, name string, imageSrc string, surveyType SurveyType) (*CreateThemeResponse, error) {
	// Check if theme already exists
	existingHref, err := c.FindThemeByName(ctx, name)
	if err != nil {
		c.logger.Warn("error checking for existing theme, will create new one",
			"name", name,
			"error", err)
	} else if existingHref != "" {
		// Theme exists, return it
		c.logger.Info("reusing existing theme",
			"name", name,
			"href", existingHref)
		return &CreateThemeResponse{
			Name: name,
			Links: map[string]string{
				"self": existingHref,
			},
		}, nil
	}

	// Theme doesn't exist, create it
	c.logger.Info("creating new theme", "name", name)

	// Use the image src URL directly as the background href
	backgroundHref := imageSrc

	// Get colors based on survey type to match email colors
	buttonColor := getColorForSurveyType(surveyType)

	request := &CreateThemeRequest{
		Name: name,
		Background: &ThemeBackground{
			Href: backgroundHref,
		},
		Colors: &ThemeColors{
			Question:   "#3D3D3D",
			Answer:     buttonColor,
			Button:     buttonColor,
			Background: "#FFFFFF",
		},
		Fields: &ThemeFields{
			Alignment: "left",
			FontSize:  "medium",
		},
		Screens: &ThemeScreens{
			Alignment: "center",
			FontSize:  "small",
		},
		Font: "Source Sans Pro",
	}

	// Log the request payload for debugging
	requestJSON, _ := json.MarshalIndent(request, "", "  ")
	c.logger.Debug("creating theme with payload", "payload", string(requestJSON))

	resp, err := c.doRequest(ctx, http.MethodPost, "/themes", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create theme: %w", err)
	}
	defer resp.Body.Close()

	var result CreateThemeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode theme response: %w", err)
	}

	c.logger.Info("theme created in typeform",
		"theme_id", result.ID,
		"theme_name", name,
		"image_src", imageSrc)

	// Cache the newly created theme
	if result.Links != nil {
		if href, ok := result.Links["self"]; ok {
			c.themeCache[name] = href
		}
	}

	return &result, nil
}

// CreateForm creates a new Typeform survey
func (c *Client) CreateForm(ctx context.Context, request *CreateFormRequest) (*CreateFormResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/forms", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create form: %w", err)
	}
	defer resp.Body.Close()

	var result CreateFormResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("typeform survey created",
		"survey_id", result.ID,
		"title", result.Title)

	return &result, nil
}
