package lambda

import (
	"errors"
	"log/slog" // Required for slog.Default() calls
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"ccoe-customer-contact-manager/internal/types"
)

func TestUserIdentityExtractor_ExtractUserIdentityFromSQSMessage(t *testing.T) {
	extractor := NewUserIdentityExtractor(
		"arn:aws:iam::123456789012:role/backend-lambda-role",
		"arn:aws:iam::123456789012:role/frontend-lambda-role",
	)

	tests := []struct {
		name        string
		sqsMessage  events.SQSMessage
		expectError bool
		expectARN   string
	}{
		{
			name: "valid S3 event with userIdentity",
			sqsMessage: events.SQSMessage{
				MessageId: "test-message-1",
				Body: `{
					"Records": [{
						"eventSource": "aws:s3",
						"eventName": "ObjectCreated:Put",
						"userIdentity": {
							"type": "AssumedRole",
							"principalId": "AIDACKCEVSQ6C2EXAMPLE:backend-lambda-function",
							"arn": "arn:aws:sts::123456789012:assumed-role/backend-lambda-role/backend-lambda-function"
						},
						"s3": {
							"bucket": {"name": "test-bucket"},
							"object": {"key": "customers/test-customer/change.json"}
						}
					}]
				}`,
			},
			expectError: false,
			expectARN:   "arn:aws:sts::123456789012:assumed-role/backend-lambda-role/backend-lambda-function",
		},
		{
			name: "S3 event without userIdentity",
			sqsMessage: events.SQSMessage{
				MessageId: "test-message-2",
				Body: `{
					"Records": [{
						"eventSource": "aws:s3",
						"eventName": "ObjectCreated:Put",
						"s3": {
							"bucket": {"name": "test-bucket"},
							"object": {"key": "customers/test-customer/change.json"}
						}
					}]
				}`,
			},
			expectError: true,
		},
		{
			name: "invalid JSON",
			sqsMessage: events.SQSMessage{
				MessageId: "test-message-3",
				Body:      `invalid json`,
			},
			expectError: true,
		},
		{
			name: "empty records array",
			sqsMessage: events.SQSMessage{
				MessageId: "test-message-4",
				Body:      `{"Records": []}`,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userIdentity, err := extractor.ExtractUserIdentityFromSQSMessage(tt.sqsMessage, slog.Default())

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if userIdentity == nil {
				t.Errorf("Expected userIdentity but got nil")
				return
			}

			if userIdentity.ARN != tt.expectARN {
				t.Errorf("Expected ARN %s, got %s", tt.expectARN, userIdentity.ARN)
			}
		})
	}
}

func TestUserIdentityExtractor_IsBackendGeneratedEvent(t *testing.T) {
	extractor := NewUserIdentityExtractor(
		"arn:aws:iam::123456789012:role/backend-lambda-role",
		"arn:aws:iam::123456789012:role/frontend-lambda-role",
	)

	tests := []struct {
		name          string
		userIdentity  *types.S3UserIdentity
		expectBackend bool
	}{
		{
			name: "backend role ARN match",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/backend-lambda-role/backend-lambda-function",
			},
			expectBackend: true,
		},
		{
			name: "frontend role ARN",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/frontend-lambda-role/frontend-lambda-function",
			},
			expectBackend: false,
		},
		{
			name: "backend role in PrincipalID",
			userIdentity: &types.S3UserIdentity{
				Type:        "AssumedRole",
				PrincipalID: "AIDACKCEVSQ6C2EXAMPLE:backend-lambda-role",
			},
			expectBackend: true,
		},
		{
			name: "external user",
			userIdentity: &types.S3UserIdentity{
				Type: "IAMUser",
				ARN:  "arn:aws:iam::123456789012:user/external-user",
			},
			expectBackend: false,
		},
		{
			name:          "nil userIdentity",
			userIdentity:  nil,
			expectBackend: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.IsBackendGeneratedEvent(tt.userIdentity, slog.Default())
			if result != tt.expectBackend {
				t.Errorf("Expected backend=%v, got %v", tt.expectBackend, result)
			}
		})
	}
}

func TestUserIdentityExtractor_ShouldDiscardEvent(t *testing.T) {
	extractor := NewUserIdentityExtractor(
		"arn:aws:iam::123456789012:role/backend-lambda-role",
		"arn:aws:iam::123456789012:role/frontend-lambda-role",
	)

	tests := []struct {
		name          string
		userIdentity  *types.S3UserIdentity
		expectDiscard bool
	}{
		{
			name: "backend event should be discarded",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/backend-lambda-role/backend-lambda-function",
			},
			expectDiscard: true,
		},
		{
			name: "frontend event should be processed",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/frontend-lambda-role/frontend-lambda-function",
			},
			expectDiscard: false,
		},
		{
			name:          "nil userIdentity should be processed",
			userIdentity:  nil,
			expectDiscard: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldDiscard, reason := extractor.ShouldDiscardEvent(tt.userIdentity, slog.Default())
			if shouldDiscard != tt.expectDiscard {
				t.Errorf("Expected discard=%v, got %v (reason: %s)", tt.expectDiscard, shouldDiscard, reason)
			}
		})
	}
}

func TestSafeExtractUserIdentity(t *testing.T) {
	extractor := NewUserIdentityExtractor(
		"arn:aws:iam::123456789012:role/backend-lambda-role",
		"arn:aws:iam::123456789012:role/frontend-lambda-role",
	)

	tests := []struct {
		name        string
		messageBody string
		messageID   string
		expectError bool
		expectARN   string
	}{
		{
			name: "valid S3 event",
			messageBody: `{
				"Records": [{
					"eventSource": "aws:s3",
					"eventName": "ObjectCreated:Put",
					"userIdentity": {
						"type": "AssumedRole",
						"principalId": "AIDACKCEVSQ6C2EXAMPLE:test-role",
						"arn": "arn:aws:sts::123456789012:assumed-role/test-role/test-function"
					},
					"s3": {
						"bucket": {"name": "test-bucket"},
						"object": {"key": "test-key"}
					}
				}]
			}`,
			messageID:   "test-msg-1",
			expectError: false,
			expectARN:   "arn:aws:sts::123456789012:assumed-role/test-role/test-function",
		},
		{
			name:        "invalid JSON",
			messageBody: `invalid json`,
			messageID:   "test-msg-2",
			expectError: true,
		},
		{
			name: "missing userIdentity",
			messageBody: `{
				"Records": [{
					"eventSource": "aws:s3",
					"eventName": "ObjectCreated:Put",
					"s3": {
						"bucket": {"name": "test-bucket"},
						"object": {"key": "test-key"}
					}
				}]
			}`,
			messageID:   "test-msg-3",
			expectError: true,
		},
		{
			name: "empty userIdentity",
			messageBody: `{
				"Records": [{
					"eventSource": "aws:s3",
					"eventName": "ObjectCreated:Put",
					"userIdentity": {},
					"s3": {
						"bucket": {"name": "test-bucket"},
						"object": {"key": "test-key"}
					}
				}]
			}`,
			messageID:   "test-msg-4",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userIdentity, err := extractor.SafeExtractUserIdentity(tt.messageBody, tt.messageID, slog.Default())

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if userIdentity == nil {
				t.Errorf("Expected userIdentity but got nil")
				return
			}

			if userIdentity.ARN != tt.expectARN {
				t.Errorf("Expected ARN %s, got %s", tt.expectARN, userIdentity.ARN)
			}
		})
	}
}

func TestExtractRoleNameFromARN(t *testing.T) {
	tests := []struct {
		name     string
		roleARN  string
		expected string
	}{
		{
			name:     "valid role ARN",
			roleARN:  "arn:aws:iam::123456789012:role/backend-lambda-role",
			expected: "backend-lambda-role",
		},
		{
			name:     "role ARN with path",
			roleARN:  "arn:aws:iam::123456789012:role/service-role/backend-lambda-role",
			expected: "backend-lambda-role",
		},
		{
			name:     "empty ARN",
			roleARN:  "",
			expected: "",
		},
		{
			name:     "invalid ARN format",
			roleARN:  "invalid-arn",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRoleNameFromARN(tt.roleARN)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestUserIdentityError tests the custom error type
func TestUserIdentityError(t *testing.T) {
	testError := errors.New("test underlying error")
	err := NewUserIdentityError(
		"test error message",
		testError,
		"test-message-id",
		"test context",
	)

	expectedMessage := "userIdentity extraction error for message test-message-id: test error message"
	if err.Error() != expectedMessage {
		t.Errorf("Expected error message %s, got %s", expectedMessage, err.Error())
	}

	if err.Unwrap() != testError {
		t.Errorf("Expected unwrapped error to be testError, got %v", err.Unwrap())
	}
}

// TestRoleConfigFromEnvironment tests the environment-based configuration
func TestRoleConfigFromEnvironment(t *testing.T) {
	// Save original environment
	originalBackend := os.Getenv("BACKEND_ROLE_ARN")
	originalFrontend := os.Getenv("FRONTEND_ROLE_ARN")
	defer func() {
		if originalBackend != "" {
			os.Setenv("BACKEND_ROLE_ARN", originalBackend)
		} else {
			os.Unsetenv("BACKEND_ROLE_ARN")
		}
		if originalFrontend != "" {
			os.Setenv("FRONTEND_ROLE_ARN", originalFrontend)
		} else {
			os.Unsetenv("FRONTEND_ROLE_ARN")
		}
	}()

	// Set test environment variables
	testBackendARN := "arn:aws:iam::123456789012:role/test-backend-role"
	testFrontendARN := "arn:aws:iam::123456789012:role/test-frontend-role"

	os.Setenv("BACKEND_ROLE_ARN", testBackendARN)
	os.Setenv("FRONTEND_ROLE_ARN", testFrontendARN)

	// Load configuration from environment
	config := LoadRoleConfigFromEnvironment()

	if config.BackendRoleARN != testBackendARN {
		t.Errorf("Expected backend ARN %s, got %s", testBackendARN, config.BackendRoleARN)
	}

	if config.FrontendRoleARN != testFrontendARN {
		t.Errorf("Expected frontend ARN %s, got %s", testFrontendARN, config.FrontendRoleARN)
	}

	// Check that default patterns are loaded
	if len(config.BackendRolePatterns) == 0 {
		t.Error("Expected backend role patterns to be loaded")
	}

	if len(config.FrontendRolePatterns) == 0 {
		t.Error("Expected frontend role patterns to be loaded")
	}
}

// TestPatternMatching tests the enhanced pattern matching functionality
func TestPatternMatching(t *testing.T) {
	config := &RoleConfig{
		BackendRoleARN:  "arn:aws:iam::123456789012:role/backend-lambda-role",
		FrontendRoleARN: "arn:aws:iam::123456789012:role/frontend-lambda-role",
		BackendRolePatterns: []string{
			"backend-lambda",
			"event-processor",
		},
		FrontendRolePatterns: []string{
			"frontend-lambda",
			"api-lambda",
		},
	}

	extractor := NewUserIdentityExtractorWithConfig(config)

	tests := []struct {
		name           string
		userIdentity   *types.S3UserIdentity
		expectBackend  bool
		expectFrontend bool
	}{
		{
			name: "exact backend ARN match",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/backend-lambda-role/function",
			},
			expectBackend:  true,
			expectFrontend: false,
		},
		{
			name: "backend pattern match",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/my-event-processor-role/function",
			},
			expectBackend:  true,
			expectFrontend: false,
		},
		{
			name: "frontend pattern match",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/my-api-lambda-service/function",
			},
			expectBackend:  false,
			expectFrontend: true,
		},
		{
			name: "no pattern match",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/unknown-service/function",
			},
			expectBackend:  false,
			expectFrontend: false,
		},
		{
			name: "PrincipalID pattern match",
			userIdentity: &types.S3UserIdentity{
				Type:        "AssumedRole",
				PrincipalID: "AIDACKCEVSQ6C2EXAMPLE:backend-lambda-processor",
			},
			expectBackend:  true,
			expectFrontend: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBackend := extractor.IsBackendGeneratedEvent(tt.userIdentity, slog.Default())
			isFrontend := extractor.IsFrontendGeneratedEvent(tt.userIdentity, slog.Default())

			if isBackend != tt.expectBackend {
				t.Errorf("Expected backend=%v, got %v", tt.expectBackend, isBackend)
			}

			if isFrontend != tt.expectFrontend {
				t.Errorf("Expected frontend=%v, got %v", tt.expectFrontend, isFrontend)
			}
		})
	}
}

// TestEnhancedRoleComparison tests the improved ARN comparison logic
func TestEnhancedRoleComparison(t *testing.T) {
	config := &RoleConfig{
		BackendRoleARN:  "arn:aws:iam::123456789012:role/backend-lambda-role",
		FrontendRoleARN: "arn:aws:iam::123456789012:role/frontend-lambda-role",
	}

	extractor := NewUserIdentityExtractorWithConfig(config)

	tests := []struct {
		name          string
		userIdentity  *types.S3UserIdentity
		expectBackend bool
	}{
		{
			name: "assumed role ARN with same role name",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::123456789012:assumed-role/backend-lambda-role/backend-lambda-function",
			},
			expectBackend: true,
		},
		{
			name: "different account should not match",
			userIdentity: &types.S3UserIdentity{
				Type: "AssumedRole",
				ARN:  "arn:aws:sts::999999999999:assumed-role/backend-lambda-role/function",
			},
			expectBackend: false,
		},
		{
			name: "IAM user ARN should not match role",
			userIdentity: &types.S3UserIdentity{
				Type: "IAMUser",
				ARN:  "arn:aws:iam::123456789012:user/backend-lambda-role",
			},
			expectBackend: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.IsBackendGeneratedEvent(tt.userIdentity, slog.Default())
			if result != tt.expectBackend {
				t.Errorf("Expected backend=%v, got %v", tt.expectBackend, result)
			}
		})
	}
}
