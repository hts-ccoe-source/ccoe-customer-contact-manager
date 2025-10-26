package typeform

import (
	"bytes"
	"context"
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
	apiToken   string
	httpClient *http.Client
	logger     *slog.Logger
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
		logger: logger,
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
