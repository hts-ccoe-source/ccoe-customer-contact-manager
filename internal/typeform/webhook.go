package typeform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// WebhookPayload represents the Typeform webhook payload
type WebhookPayload struct {
	EventID      string       `json:"event_id"`
	EventType    string       `json:"event_type"`
	FormResponse FormResponse `json:"form_response"`
}

// FormResponse represents the survey response data
type FormResponse struct {
	FormID      string            `json:"form_id"`
	Token       string            `json:"token"`
	SubmittedAt string            `json:"submitted_at"`
	Hidden      map[string]string `json:"hidden"`
	Answers     []Answer          `json:"answers"`
}

// Answer represents a single survey answer
type Answer struct {
	Type    string                 `json:"type"`
	Field   Field                  `json:"field"`
	Number  *int                   `json:"number,omitempty"`
	Boolean *bool                  `json:"boolean,omitempty"`
	Text    *string                `json:"text,omitempty"`
	Choice  map[string]interface{} `json:"choice,omitempty"`
}

// ValidateWebhookSignature validates the HMAC-SHA256 signature
func ValidateWebhookSignature(payload []byte, signature string, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// WebhookHandler handles Typeform webhook processing
type WebhookHandler struct {
	s3Client   *s3.Client
	bucketName string
	logger     *slog.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(s3Client *s3.Client, bucketName string, logger *slog.Logger) *WebhookHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &WebhookHandler{
		s3Client:   s3Client,
		bucketName: bucketName,
		logger:     logger,
	}
}

// HandleWebhook processes incoming Typeform webhooks
func (h *WebhookHandler) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	// 1. Validate signature
	secret := os.Getenv("TYPEFORM_WEBHOOK_SECRET")
	if secret == "" {
		return fmt.Errorf("TYPEFORM_WEBHOOK_SECRET environment variable not set")
	}

	if !ValidateWebhookSignature(payload, signature, secret) {
		h.logger.Warn("invalid webhook signature received")
		return fmt.Errorf("invalid webhook signature")
	}

	// 2. Parse payload
	var webhook WebhookPayload
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	h.logger.Info("webhook received",
		"event_id", webhook.EventID,
		"event_type", webhook.EventType,
		"form_id", webhook.FormResponse.FormID)

	// 3. Extract metadata from hidden fields
	customerCode := webhook.FormResponse.Hidden["customer_code"]
	year := webhook.FormResponse.Hidden["year"]
	quarter := webhook.FormResponse.Hidden["quarter"]

	if customerCode == "" || year == "" || quarter == "" {
		h.logger.Warn("missing required hidden fields",
			"customer_code", customerCode,
			"year", year,
			"quarter", quarter)
		// Continue processing but log the issue
	}

	// 4. Store survey results in S3
	if err := h.storeSurveyResults(ctx, &webhook, customerCode, year, quarter); err != nil {
		return fmt.Errorf("failed to store survey results: %w", err)
	}

	return nil
}

// storeSurveyResults stores survey results in S3 with retry logic
func (h *WebhookHandler) storeSurveyResults(ctx context.Context, webhook *WebhookPayload, customerCode, year, quarter string) error {
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("surveys/results/%s/%s/%s/%d-%s.json",
		customerCode, year, quarter, timestamp, webhook.FormResponse.FormID)

	data, err := json.Marshal(webhook)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Retry logic with exponential backoff
	maxRetries := 3
	backoff := time.Second

	for i := 0; i < maxRetries; i++ {
		_, err = h.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(h.bucketName),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String("application/json"),
		})

		if err == nil {
			h.logger.Info("survey results stored in s3",
				"key", key,
				"form_id", webhook.FormResponse.FormID)
			return nil
		}

		h.logger.Warn("s3 storage attempt failed",
			"attempt", i+1,
			"max_retries", maxRetries,
			"error", err)

		if i < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("failed to store after %d retries: %w", maxRetries, err)
}

// GetWebhookSecret returns the webhook secret from environment
func GetWebhookSecret() string {
	return os.Getenv("TYPEFORM_WEBHOOK_SECRET")
}
