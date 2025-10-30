package lambda

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"ccoe-customer-contact-manager/internal/types"
)

// RoleConfig represents configuration for known IAM roles
type RoleConfig struct {
	BackendRoleARN  string
	FrontendRoleARN string
	// Additional role patterns that should be considered backend roles
	BackendRolePatterns []string
	// Additional role patterns that should be considered frontend roles
	FrontendRolePatterns []string
}

// UserIdentityExtractor handles extraction and validation of userIdentity from S3 events
type UserIdentityExtractor struct {
	Config *RoleConfig
}

// NewUserIdentityExtractor creates a new UserIdentityExtractor with configured role ARNs
func NewUserIdentityExtractor(backendRoleARN, frontendRoleARN string) *UserIdentityExtractor {
	return &UserIdentityExtractor{
		Config: &RoleConfig{
			BackendRoleARN:  backendRoleARN,
			FrontendRoleARN: frontendRoleARN,
		},
	}
}

// NewUserIdentityExtractorWithConfig creates a new UserIdentityExtractor with a RoleConfig
func NewUserIdentityExtractorWithConfig(config *RoleConfig) *UserIdentityExtractor {
	return &UserIdentityExtractor{
		Config: config,
	}
}

// LoadRoleConfigFromEnvironment loads role configuration from environment variables
func LoadRoleConfigFromEnvironment() *RoleConfig {
	config := &RoleConfig{
		BackendRoleARN:  getBackendRoleARNFromEnv(),
		FrontendRoleARN: getFrontendRoleARNFromEnv(),
	}

	// Add default backend role patterns
	config.BackendRolePatterns = []string{
		"backend-lambda",
		"backend-processor",
		"change-processor",
		"event-processor",
	}

	// Add default frontend role patterns
	config.FrontendRolePatterns = []string{
		"frontend-lambda",
		"web-lambda",
		"api-lambda",
		"user-interface",
	}

	return config
}

// getBackendRoleARNFromEnv returns the backend Lambda's execution role ARN from environment variables
func getBackendRoleARNFromEnv() string {
	// Try multiple environment variable names for flexibility
	roleARN := os.Getenv("BACKEND_ROLE_ARN")
	if roleARN == "" {
		roleARN = os.Getenv("AWS_LAMBDA_ROLE_ARN")
	}
	if roleARN == "" {
		roleARN = os.Getenv("LAMBDA_EXECUTION_ROLE_ARN")
	}

	if roleARN == "" {
		slog.Warn("backend role ARN not configured - event loop prevention may not work correctly")
	}

	return roleARN
}

// getFrontendRoleARNFromEnv returns the frontend Lambda's execution role ARN from environment variables
func getFrontendRoleARNFromEnv() string {
	roleARN := os.Getenv("FRONTEND_ROLE_ARN")

	if roleARN == "" {
		slog.Warn("frontend role ARN not configured - may not be able to identify frontend events")
	}

	return roleARN
}

// ExtractUserIdentityFromSQSMessage extracts userIdentity from an SQS message containing S3 event payload
func (u *UserIdentityExtractor) ExtractUserIdentityFromSQSMessage(sqsRecord events.SQSMessage, logger *slog.Logger) (*types.S3UserIdentity, error) {
	logger.Debug("extracting userIdentity from SQS message",
		"message_id", sqsRecord.MessageId)

	// Parse the SQS message body as S3 event notification
	var s3Event types.S3EventNotification
	if err := json.Unmarshal([]byte(sqsRecord.Body), &s3Event); err != nil {
		return nil, NewUserIdentityError(
			"failed to parse SQS message body as S3 event",
			err,
			sqsRecord.MessageId,
			sqsRecord.Body,
		)
	}

	// Check if we have any S3 event records
	if len(s3Event.Records) == 0 {
		return nil, NewUserIdentityError(
			"no S3 event records found in SQS message",
			fmt.Errorf("empty records array"),
			sqsRecord.MessageId,
			sqsRecord.Body,
		)
	}

	// Extract userIdentity from the first S3 event record
	// In practice, SQS messages typically contain a single S3 event record
	s3Record := s3Event.Records[0]

	if s3Record.UserIdentity == nil {
		logger.Debug("no userIdentity field found in S3 event record",
			"message_id", sqsRecord.MessageId)
		return nil, NewUserIdentityError(
			"userIdentity field is missing from S3 event record",
			fmt.Errorf("userIdentity is nil"),
			sqsRecord.MessageId,
			fmt.Sprintf("S3 Record: %+v", s3Record),
		)
	}

	logger.Debug("extracted userIdentity from message",
		"message_id", sqsRecord.MessageId,
		"type", s3Record.UserIdentity.Type,
		"arn", s3Record.UserIdentity.ARN,
		"principal_id", s3Record.UserIdentity.PrincipalID)

	return s3Record.UserIdentity, nil
}

// ExtractUserIdentityFromS3Event extracts userIdentity from a parsed S3 event
func (u *UserIdentityExtractor) ExtractUserIdentityFromS3Event(s3Event types.S3EventNotification, logger *slog.Logger) (*types.S3UserIdentity, error) {
	if len(s3Event.Records) == 0 {
		return nil, NewUserIdentityError(
			"no S3 event records found",
			fmt.Errorf("empty records array"),
			"",
			fmt.Sprintf("S3Event: %+v", s3Event),
		)
	}

	s3Record := s3Event.Records[0]

	if s3Record.UserIdentity == nil {
		logger.Debug("no userIdentity field found in S3 event record")
		return nil, NewUserIdentityError(
			"userIdentity field is missing from S3 event record",
			fmt.Errorf("userIdentity is nil"),
			"",
			fmt.Sprintf("S3 Record: %+v", s3Record),
		)
	}

	logger.Debug("extracted userIdentity from S3 event",
		"type", s3Record.UserIdentity.Type,
		"arn", s3Record.UserIdentity.ARN,
		"principal_id", s3Record.UserIdentity.PrincipalID)

	return s3Record.UserIdentity, nil
}

// SafeExtractUserIdentity safely extracts userIdentity with comprehensive error handling
func (u *UserIdentityExtractor) SafeExtractUserIdentity(messageBody string, messageID string, logger *slog.Logger) (*types.S3UserIdentity, error) {
	logger.Debug("safely extracting userIdentity from message",
		"message_id", messageID)

	// First, try to parse as S3 event notification
	var s3Event types.S3EventNotification
	if err := json.Unmarshal([]byte(messageBody), &s3Event); err != nil {
		return nil, NewUserIdentityError(
			"failed to parse message body as S3 event notification",
			err,
			messageID,
			messageBody,
		)
	}

	// Validate S3 event structure
	if len(s3Event.Records) == 0 {
		return nil, NewUserIdentityError(
			"S3 event notification contains no records",
			fmt.Errorf("empty records array"),
			messageID,
			messageBody,
		)
	}

	// Extract userIdentity from first record
	s3Record := s3Event.Records[0]

	// Check if userIdentity exists
	if s3Record.UserIdentity == nil {
		logger.Debug("userIdentity field is missing from S3 event",
			"message_id", messageID,
			"event_name", s3Record.EventName,
			"event_source", s3Record.EventSource)
		return nil, NewUserIdentityError(
			"userIdentity field is missing from S3 event record",
			fmt.Errorf("userIdentity is nil"),
			messageID,
			fmt.Sprintf("Event: %s, Source: %s", s3Record.EventName, s3Record.EventSource),
		)
	}

	// Validate userIdentity structure
	userIdentity := s3Record.UserIdentity
	if userIdentity.Type == "" && userIdentity.ARN == "" && userIdentity.PrincipalID == "" {
		return nil, NewUserIdentityError(
			"userIdentity contains no identifying information",
			fmt.Errorf("all userIdentity fields are empty"),
			messageID,
			fmt.Sprintf("UserIdentity: %+v", userIdentity),
		)
	}

	logger.Debug("extracted userIdentity",
		"type", userIdentity.Type,
		"arn", userIdentity.ARN,
		"principal_id", userIdentity.PrincipalID)

	return userIdentity, nil
}

// IsBackendGeneratedEvent checks if the userIdentity indicates the event was generated by the backend
func (u *UserIdentityExtractor) IsBackendGeneratedEvent(userIdentity *types.S3UserIdentity, logger *slog.Logger) bool {
	if userIdentity == nil {
		logger.Debug("cannot determine event source: userIdentity is nil")
		return false
	}

	if u.Config == nil {
		logger.Debug("cannot determine event source: RoleConfig is nil")
		return false
	}

	// Check if the ARN matches the backend Lambda's execution role
	if u.Config.BackendRoleARN != "" && userIdentity.ARN != "" {
		isBackend := u.compareRoleARNs(userIdentity.ARN, u.Config.BackendRoleARN)
		if isBackend {
			logger.Debug("event identified as backend-generated: ARN matches backend role",
				"arn", userIdentity.ARN)
			return true
		}
	}

	// Check PrincipalID for backend role patterns
	// PrincipalID format: AWS:{RoleUniqueId}:{SessionName}
	// For Lambda, SessionName is typically the function name or role name
	if userIdentity.PrincipalID != "" {
		if u.Config.BackendRoleARN != "" {
			backendRoleName := extractRoleNameFromARN(u.Config.BackendRoleARN)
			if backendRoleName != "" && strings.Contains(userIdentity.PrincipalID, backendRoleName) {
				logger.Debug("event identified as backend-generated: PrincipalID contains backend role name",
					"principal_id", userIdentity.PrincipalID)
				return true
			}
		}

		// Also check if PrincipalID contains "backend" keyword (common in Lambda session names)
		// This check works even if BackendRoleARN is not configured
		if strings.Contains(strings.ToLower(userIdentity.PrincipalID), "backend") {
			logger.Debug("event identified as backend-generated: PrincipalID contains 'backend' keyword",
				"principal_id", userIdentity.PrincipalID)
			return true
		}
	}

	// Check against backend role patterns
	if u.matchesRolePatterns(userIdentity, u.Config.BackendRolePatterns, logger) {
		logger.Debug("event identified as backend-generated: matches backend role pattern")
		return true
	}

	logger.Debug("event identified as frontend-generated or external",
		"arn", userIdentity.ARN,
		"principal_id", userIdentity.PrincipalID)
	return false
}

// IsFrontendGeneratedEvent checks if the userIdentity indicates the event was generated by the frontend
func (u *UserIdentityExtractor) IsFrontendGeneratedEvent(userIdentity *types.S3UserIdentity, logger *slog.Logger) bool {
	if userIdentity == nil {
		logger.Debug("cannot determine event source: userIdentity is nil")
		return false
	}

	if u.Config == nil {
		logger.Debug("cannot determine event source: RoleConfig is nil")
		return false
	}

	// Check if the ARN matches the frontend Lambda's execution role
	if u.Config.FrontendRoleARN != "" && userIdentity.ARN != "" {
		isFrontend := u.compareRoleARNs(userIdentity.ARN, u.Config.FrontendRoleARN)
		if isFrontend {
			logger.Debug("event identified as frontend-generated: ARN matches frontend role",
				"arn", userIdentity.ARN)
			return true
		}
	}

	// Check PrincipalID for frontend role patterns
	if userIdentity.PrincipalID != "" && u.Config.FrontendRoleARN != "" {
		frontendRoleName := extractRoleNameFromARN(u.Config.FrontendRoleARN)
		if frontendRoleName != "" && strings.Contains(userIdentity.PrincipalID, frontendRoleName) {
			logger.Debug("event identified as frontend-generated: PrincipalID contains frontend role name",
				"principal_id", userIdentity.PrincipalID)
			return true
		}
	}

	// Check against frontend role patterns
	if u.matchesRolePatterns(userIdentity, u.Config.FrontendRolePatterns, logger) {
		logger.Debug("event identified as frontend-generated: matches frontend role pattern")
		return true
	}

	return false
}

// ShouldDiscardEvent determines if an event should be discarded based on userIdentity
func (u *UserIdentityExtractor) ShouldDiscardEvent(userIdentity *types.S3UserIdentity, logger *slog.Logger) (bool, string) {
	if userIdentity == nil {
		// If userIdentity is missing, process the event to be safe
		return false, "userIdentity is nil - processing event to be safe"
	}

	if u.IsBackendGeneratedEvent(userIdentity, logger) {
		return true, fmt.Sprintf("event generated by backend (ARN: %s, PrincipalID: %s)",
			userIdentity.ARN, userIdentity.PrincipalID)
	}

	return false, fmt.Sprintf("event should be processed (ARN: %s, PrincipalID: %s)",
		userIdentity.ARN, userIdentity.PrincipalID)
}

// compareRoleARNs compares a userIdentity ARN with a configured role ARN
// Handles both IAM role ARNs and STS assumed role ARNs
func (u *UserIdentityExtractor) compareRoleARNs(userIdentityARN, configuredRoleARN string) bool {
	if userIdentityARN == "" || configuredRoleARN == "" {
		return false
	}

	// Direct match
	if userIdentityARN == configuredRoleARN {
		return true
	}

	// Extract account IDs first to ensure we're comparing within the same account
	userAccountID := extractAccountIDFromARN(userIdentityARN)
	configAccountID := extractAccountIDFromARN(configuredRoleARN)

	// Only proceed if both ARNs are from the same AWS account
	if userAccountID == "" || configAccountID == "" || userAccountID != configAccountID {
		return false
	}

	// Extract role name from configured ARN and check if it appears in userIdentity ARN
	configuredRoleName := extractRoleNameFromARN(configuredRoleARN)
	if configuredRoleName != "" {
		// Check if the role name appears in the userIdentity ARN
		// This handles cases where userIdentity ARN is an assumed role ARN
		// e.g., arn:aws:sts::123456789012:assumed-role/backend-lambda-role/backend-lambda-function
		if strings.Contains(userIdentityARN, configuredRoleName) {
			return true
		}
	}

	return false
}

// matchesRolePatterns checks if userIdentity matches any of the provided role patterns
func (u *UserIdentityExtractor) matchesRolePatterns(userIdentity *types.S3UserIdentity, patterns []string, logger *slog.Logger) bool {
	if userIdentity == nil || len(patterns) == 0 {
		return false
	}

	// Check ARN against patterns
	if userIdentity.ARN != "" {
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(userIdentity.ARN), strings.ToLower(pattern)) {
				logger.Debug("userIdentity ARN matches pattern",
					"arn", userIdentity.ARN,
					"pattern", pattern)
				return true
			}
		}
	}

	// Check PrincipalID against patterns
	if userIdentity.PrincipalID != "" {
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(userIdentity.PrincipalID), strings.ToLower(pattern)) {
				logger.Debug("userIdentity PrincipalID matches pattern",
					"principal_id", userIdentity.PrincipalID,
					"pattern", pattern)
				return true
			}
		}
	}

	return false
}

// extractAccountIDFromARN extracts the AWS account ID from an ARN
func extractAccountIDFromARN(arn string) string {
	if arn == "" {
		return ""
	}

	parts := strings.Split(arn, ":")
	if len(parts) >= 5 {
		return parts[4] // Account ID is the 5th part (index 4)
	}

	return ""
}

// extractRoleNameFromARN extracts the role name from an IAM role ARN
// ARN format: arn:aws:iam::123456789012:role/RoleName
// Also handles assumed role ARNs: arn:aws:sts::123456789012:assumed-role/RoleName/SessionName
func extractRoleNameFromARN(roleARN string) string {
	if roleARN == "" {
		return ""
	}

	parts := strings.Split(roleARN, "/")
	if len(parts) >= 2 {
		// For assumed role ARNs, the role name is the second-to-last part
		// For IAM role ARNs, the role name is the last part
		if strings.Contains(roleARN, "assumed-role") && len(parts) >= 3 {
			return parts[len(parts)-2] // Return the role name (second-to-last part)
		}
		return parts[len(parts)-1] // Return the last part (role name)
	}

	return ""
}

// UserIdentityError represents an error during userIdentity extraction
type UserIdentityError struct {
	Message   string
	Cause     error
	MessageID string
	Context   string
}

// Error implements the error interface
func (e *UserIdentityError) Error() string {
	if e.MessageID != "" {
		return fmt.Sprintf("userIdentity extraction error for message %s: %s", e.MessageID, e.Message)
	}
	return fmt.Sprintf("userIdentity extraction error: %s", e.Message)
}

// Unwrap returns the underlying error
func (e *UserIdentityError) Unwrap() error {
	return e.Cause
}

// NewUserIdentityError creates a new UserIdentityError
func NewUserIdentityError(message string, cause error, messageID, context string) *UserIdentityError {
	return &UserIdentityError{
		Message:   message,
		Cause:     cause,
		MessageID: messageID,
		Context:   context,
	}
}
