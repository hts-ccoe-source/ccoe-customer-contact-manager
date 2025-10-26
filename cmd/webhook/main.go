package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"ccoe-customer-contact-manager/internal/typeform"
)

// Version information
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Response represents the Lambda response
type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// ErrorResponse represents an error response body
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a success response body
type SuccessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// webhookSecretCache holds the cached webhook secret from Parameter Store
var webhookSecretCache string

// loadWebhookSecretFromSSM loads the Typeform webhook secret from Parameter Store
func loadWebhookSecretFromSSM(ctx context.Context) (string, error) {
	// Return cached value if already loaded
	if webhookSecretCache != "" {
		return webhookSecretCache, nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}

	client := ssm.NewFromConfig(cfg)

	// Get the parameter path from environment variable
	parameterPath := os.Getenv("TYPEFORM_WEBHOOK_SECRET_PARAMETER")
	if parameterPath == "" {
		parameterPath = "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_WEBHOOK_SECRET"
	}

	result, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(parameterPath),
		WithDecryption: aws.Bool(true), // Important for SecureString parameters
	})
	if err != nil {
		return "", err
	}

	// Cache the secret
	webhookSecretCache = *result.Parameter.Value
	log.Printf("✅ Successfully loaded Typeform webhook secret from Parameter Store")

	return webhookSecretCache, nil
}

func main() {
	// Display version information at startup
	log.Printf("CCOE Typeform Webhook Handler v%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)

	// Start Lambda handler
	lambda.Start(handleRequest)
}

// handleRequest processes API Gateway proxy requests
func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	// Setup structured logging
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	var slogLevel slog.Level
	switch logLevel {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	}))

	logger.Info("webhook request received",
		"method", request.HTTPMethod,
		"path", request.Path,
		"request_id", request.RequestContext.RequestID)

	// Validate HTTP method
	if request.HTTPMethod != "POST" {
		logger.Warn("invalid http method",
			"method", request.HTTPMethod,
			"expected", "POST")
		return createErrorResponse(405, "Method not allowed", "Only POST requests are supported"), nil
	}

	// Extract Typeform signature from headers
	signature := request.Headers["Typeform-Signature"]
	if signature == "" {
		// Try lowercase header name (API Gateway may normalize headers)
		signature = request.Headers["typeform-signature"]
	}

	if signature == "" {
		logger.Warn("missing typeform signature header")
		return createErrorResponse(401, "Unauthorized", "Missing Typeform-Signature header"), nil
	}

	// Load webhook secret from Parameter Store
	secret, err := loadWebhookSecretFromSSM(ctx)
	if err != nil {
		logger.Error("failed to load webhook secret from parameter store",
			"error", err)
		return createErrorResponse(500, "Internal server error", "Failed to load webhook secret"), nil
	}

	// Validate signature
	payload := []byte(request.Body)
	if !typeform.ValidateWebhookSignature(payload, signature, secret) {
		logger.Warn("invalid webhook signature",
			"signature", signature)
		return createErrorResponse(401, "Unauthorized", "Invalid webhook signature"), nil
	}

	logger.Info("webhook signature validated successfully")

	// Parse webhook payload
	var webhook typeform.WebhookPayload
	if err := json.Unmarshal(payload, &webhook); err != nil {
		logger.Error("failed to parse webhook payload",
			"error", err)
		return createErrorResponse(400, "Bad request", "Invalid webhook payload"), nil
	}

	logger.Info("webhook payload parsed",
		"event_id", webhook.EventID,
		"event_type", webhook.EventType,
		"form_id", webhook.FormResponse.FormID)

	// Extract hidden fields
	customerCode := webhook.FormResponse.Hidden["customer_code"]
	year := webhook.FormResponse.Hidden["year"]
	quarter := webhook.FormResponse.Hidden["quarter"]
	userLogin := webhook.FormResponse.Hidden["user_login"]
	eventType := webhook.FormResponse.Hidden["event_type"]
	eventSubtype := webhook.FormResponse.Hidden["event_subtype"]

	logger.Info("extracted hidden fields",
		"customer_code", customerCode,
		"year", year,
		"quarter", quarter,
		"user_login", userLogin,
		"event_type", eventType,
		"event_subtype", eventSubtype)

	// Validate required hidden fields
	if customerCode == "" || year == "" || quarter == "" {
		logger.Warn("missing required hidden fields",
			"customer_code", customerCode,
			"year", year,
			"quarter", quarter)
		// Continue processing but log the warning
	}

	// Load AWS configuration
	bucketName := os.Getenv("S3_BUCKET")
	if bucketName == "" {
		logger.Error("s3 bucket not configured")
		return createErrorResponse(500, "Internal server error", "S3 bucket not configured"), nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("failed to load aws config",
			"error", err)
		return createErrorResponse(500, "Internal server error", "Failed to load AWS configuration"), nil
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg)

	// Create webhook handler
	webhookHandler := typeform.NewWebhookHandler(s3Client, bucketName, logger)

	// Process webhook (stores results in S3)
	if err := webhookHandler.HandleWebhook(ctx, payload, signature); err != nil {
		logger.Error("failed to process webhook",
			"error", err)
		return createErrorResponse(500, "Internal server error", "Failed to process webhook"), nil
	}

	logger.Info("webhook processed successfully",
		"form_id", webhook.FormResponse.FormID,
		"customer_code", customerCode)

	// Return success response
	return createSuccessResponse("Webhook processed successfully"), nil
}

// createErrorResponse creates an error response
func createErrorResponse(statusCode int, error string, message string) Response {
	body := ErrorResponse{
		Error:   error,
		Message: message,
	}

	bodyJSON, _ := json.Marshal(body)

	return Response{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(bodyJSON),
	}
}

// createSuccessResponse creates a success response
func createSuccessResponse(message string) Response {
	body := SuccessResponse{
		Status:  "success",
		Message: message,
	}

	bodyJSON, _ := json.Marshal(body)

	return Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(bodyJSON),
	}
}
