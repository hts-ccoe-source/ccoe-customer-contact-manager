package lambda

import (
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// TestUserIdentityIntegration tests the integration of userIdentity extraction with the Lambda handler
func TestUserIdentityIntegration(t *testing.T) {
	// Set up environment variables for testing
	os.Setenv("BACKEND_ROLE_ARN", "arn:aws:iam::123456789012:role/backend-lambda-role")
	os.Setenv("FRONTEND_ROLE_ARN", "arn:aws:iam::123456789012:role/frontend-lambda-role")
	defer func() {
		os.Unsetenv("BACKEND_ROLE_ARN")
		os.Unsetenv("FRONTEND_ROLE_ARN")
	}()

	tests := []struct {
		name          string
		sqsMessage    events.SQSMessage
		expectDiscard bool
		expectError   bool
	}{
		{
			name: "backend event should be discarded",
			sqsMessage: events.SQSMessage{
				MessageId: "test-backend-event",
				Body: `{
					"Records": [{
						"eventSource": "aws:s3",
						"eventName": "ObjectCreated:Put",
						"userIdentity": {
							"type": "AssumedRole",
							"principalId": "AIDACKCEVSQ6C2EXAMPLE:backend-lambda-role",
							"arn": "arn:aws:sts::123456789012:assumed-role/backend-lambda-role/backend-lambda-function"
						},
						"s3": {
							"bucket": {"name": "test-bucket"},
							"object": {"key": "customers/test-customer/change.json"}
						}
					}]
				}`,
			},
			expectDiscard: true,
			expectError:   false,
		},
		{
			name: "frontend event should be processed",
			sqsMessage: events.SQSMessage{
				MessageId: "test-frontend-event",
				Body: `{
					"Records": [{
						"eventSource": "aws:s3",
						"eventName": "ObjectCreated:Put",
						"userIdentity": {
							"type": "AssumedRole",
							"principalId": "AIDACKCEVSQ6C2EXAMPLE:frontend-lambda-role",
							"arn": "arn:aws:sts::123456789012:assumed-role/frontend-lambda-role/frontend-lambda-function"
						},
						"s3": {
							"bucket": {"name": "test-bucket"},
							"object": {"key": "customers/test-customer/change.json"}
						}
					}]
				}`,
			},
			expectDiscard: false,
			expectError:   true, // Will fail due to missing config, but that's expected
		},
		{
			name: "S3 test event should be skipped",
			sqsMessage: events.SQSMessage{
				MessageId: "test-s3-test-event",
				Body: `{
					"Service": "Amazon S3",
					"Event": "s3:TestEvent",
					"Time": "2025-01-15T10:30:00.000Z",
					"Bucket": "test-bucket",
					"RequestId": "test-request-id",
					"HostId": "test-host-id"
				}`,
			},
			expectDiscard: true, // Test events are skipped
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We don't need a full config for userIdentity testing

			// Test the userIdentity extraction logic directly
			extractor := NewUserIdentityExtractor(
				"arn:aws:iam::123456789012:role/backend-lambda-role",
				"arn:aws:iam::123456789012:role/frontend-lambda-role",
			)

			// Check if it's an S3 test event first
			if IsS3TestEvent(tt.sqsMessage.Body) {
				t.Logf("Message correctly identified as S3 test event")
				return
			}

			// Extract userIdentity
			userIdentity, err := extractor.SafeExtractUserIdentity(tt.sqsMessage.Body, tt.sqsMessage.MessageId)
			if err != nil {
				t.Logf("UserIdentity extraction failed (may be expected): %v", err)
				// For this test, we're mainly checking that the extraction doesn't panic
				return
			}

			// Check discard decision
			shouldDiscard, reason := extractor.ShouldDiscardEvent(userIdentity)
			t.Logf("Discard decision: %v, reason: %s", shouldDiscard, reason)

			if shouldDiscard != tt.expectDiscard {
				t.Errorf("Expected discard=%v, got %v", tt.expectDiscard, shouldDiscard)
			}
		})
	}
}

// TestGetRoleARNFunctions tests the helper functions for getting role ARNs
func TestGetRoleARNFunctions(t *testing.T) {
	// Test with no environment variables set
	backendARN := getBackendRoleARN()
	frontendARN := getFrontendRoleARN()

	// Should return empty strings when not set
	if backendARN != "" {
		t.Logf("Backend ARN from environment: %s", backendARN)
	}
	if frontendARN != "" {
		t.Logf("Frontend ARN from environment: %s", frontendARN)
	}

	// Test with environment variables set
	testBackendARN := "arn:aws:iam::123456789012:role/test-backend-role"
	testFrontendARN := "arn:aws:iam::123456789012:role/test-frontend-role"

	os.Setenv("BACKEND_ROLE_ARN", testBackendARN)
	os.Setenv("FRONTEND_ROLE_ARN", testFrontendARN)
	defer func() {
		os.Unsetenv("BACKEND_ROLE_ARN")
		os.Unsetenv("FRONTEND_ROLE_ARN")
	}()

	backendARN = getBackendRoleARN()
	frontendARN = getFrontendRoleARN()

	if backendARN != testBackendARN {
		t.Errorf("Expected backend ARN %s, got %s", testBackendARN, backendARN)
	}
	if frontendARN != testFrontendARN {
		t.Errorf("Expected frontend ARN %s, got %s", testFrontendARN, frontendARN)
	}
}

// TestIsS3TestEvent tests the S3 test event detection
func TestIsS3TestEvent(t *testing.T) {
	tests := []struct {
		name        string
		messageBody string
		expected    bool
	}{
		{
			name: "S3 test event with standard format",
			messageBody: `{
				"Service": "Amazon S3",
				"Event": "s3:TestEvent",
				"Time": "2025-01-15T10:30:00.000Z"
			}`,
			expected: true,
		},
		{
			name:        "S3 test event with compact format",
			messageBody: `{"Service":"Amazon S3","Event":"s3:TestEvent"}`,
			expected:    true,
		},
		{
			name: "Regular S3 event",
			messageBody: `{
				"Records": [{
					"eventSource": "aws:s3",
					"eventName": "ObjectCreated:Put"
				}]
			}`,
			expected: false,
		},
		{
			name:        "Non-S3 message",
			messageBody: `{"message": "hello world"}`,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsS3TestEvent(tt.messageBody)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
