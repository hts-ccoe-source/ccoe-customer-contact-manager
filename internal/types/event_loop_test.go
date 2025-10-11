package types

import (
	"encoding/json"
	"testing"
	"time"
)

// TestS3EventUserIdentityExtraction tests extraction of user identity from S3 events
func TestS3EventUserIdentityExtraction(t *testing.T) {
	tests := []struct {
		name           string
		event          *S3EventRecord
		expectedARN    string
		expectedType   string
		shouldHaveUser bool
	}{
		{
			name: "backend lambda event",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type:        "AssumedRole",
					PrincipalID: "AIDACKCEVSQ6C2EXAMPLE:backend-lambda",
					ARN:         "arn:aws:iam::123456789012:role/backend-lambda-role",
					AccountID:   "123456789012",
				},
			},
			expectedARN:    "arn:aws:iam::123456789012:role/backend-lambda-role",
			expectedType:   "AssumedRole",
			shouldHaveUser: true,
		},
		{
			name: "frontend lambda event",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type:        "AssumedRole",
					PrincipalID: "AIDACKCEVSQ6C2EXAMPLE:frontend-lambda",
					ARN:         "arn:aws:iam::123456789012:role/frontend-lambda-role",
					AccountID:   "123456789012",
				},
			},
			expectedARN:    "arn:aws:iam::123456789012:role/frontend-lambda-role",
			expectedType:   "AssumedRole",
			shouldHaveUser: true,
		},
		{
			name: "user upload event",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type:        "IAMUser",
					PrincipalID: "AIDACKCEVSQ6C2EXAMPLE",
					ARN:         "arn:aws:iam::123456789012:user/test-user",
					AccountID:   "123456789012",
					UserName:    "test-user",
				},
			},
			expectedARN:    "arn:aws:iam::123456789012:user/test-user",
			expectedType:   "IAMUser",
			shouldHaveUser: true,
		},
		{
			name: "event without user identity",
			event: &S3EventRecord{
				EventName:    "s3:ObjectCreated:Put",
				UserIdentity: nil,
			},
			expectedARN:    "",
			expectedType:   "",
			shouldHaveUser: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldHaveUser {
				if tt.event.UserIdentity == nil {
					t.Error("Expected user identity to be present")
					return
				}

				if tt.event.UserIdentity.ARN != tt.expectedARN {
					t.Errorf("Expected ARN %s, got %s", tt.expectedARN, tt.event.UserIdentity.ARN)
				}

				if tt.event.UserIdentity.Type != tt.expectedType {
					t.Errorf("Expected type %s, got %s", tt.expectedType, tt.event.UserIdentity.Type)
				}
			} else {
				if tt.event.UserIdentity != nil {
					t.Error("Expected user identity to be nil")
				}
			}
		})
	}
}

// TestEventLoopPreventionLogic tests the complete event loop prevention logic
func TestEventLoopPreventionLogic(t *testing.T) {
	backendRoleARN := "arn:aws:iam::123456789012:role/backend-lambda-role"
	frontendRoleARN := "arn:aws:iam::123456789012:role/frontend-lambda-role"
	userARN := "arn:aws:iam::123456789012:user/test-user"

	tests := []struct {
		name          string
		event         *S3EventRecord
		backendARN    string
		shouldProcess bool
		description   string
	}{
		{
			name: "backend generated event - should discard",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type: "AssumedRole",
					ARN:  backendRoleARN,
				},
			},
			backendARN:    backendRoleARN,
			shouldProcess: false,
			description:   "Backend should discard its own events to prevent loops",
		},
		{
			name: "frontend generated event - should process",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type: "AssumedRole",
					ARN:  frontendRoleARN,
				},
			},
			backendARN:    backendRoleARN,
			shouldProcess: true,
			description:   "Backend should process frontend events normally",
		},
		{
			name: "user generated event - should process",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type: "IAMUser",
					ARN:  userARN,
				},
			},
			backendARN:    backendRoleARN,
			shouldProcess: true,
			description:   "Backend should process user events normally",
		},
		{
			name: "event without user identity - should process",
			event: &S3EventRecord{
				EventName:    "s3:ObjectCreated:Put",
				UserIdentity: nil,
			},
			backendARN:    backendRoleARN,
			shouldProcess: true,
			description:   "Backend should process events without user identity (err on side of processing)",
		},
		{
			name: "malformed user identity - should process",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type: "AssumedRole",
					ARN:  "", // Empty ARN
				},
			},
			backendARN:    backendRoleARN,
			shouldProcess: true,
			description:   "Backend should process events with malformed user identity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldProcess := !isBackendGeneratedEvent(tt.event, tt.backendARN)

			if shouldProcess != tt.shouldProcess {
				t.Errorf("Expected shouldProcess=%v, got %v. %s", tt.shouldProcess, shouldProcess, tt.description)
			}
		})
	}
}

// TestSQSMessageProcessingWithEventLoop tests SQS message processing with event loop prevention
func TestSQSMessageProcessingWithEventLoop(t *testing.T) {
	backendRoleARN := "arn:aws:iam::123456789012:role/backend-lambda-role"

	// Create a complete S3 event notification that would come through SQS
	s3Event := &S3EventNotification{
		Records: []S3EventRecord{
			{
				EventVersion: "2.1",
				EventSource:  "aws:s3",
				AWSRegion:    "us-east-1",
				EventTime:    time.Now(),
				EventName:    "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type:        "AssumedRole",
					PrincipalID: "AIDACKCEVSQ6C2EXAMPLE:backend-lambda",
					ARN:         backendRoleARN,
					AccountID:   "123456789012",
				},
				S3: struct {
					S3SchemaVersion string `json:"s3SchemaVersion"`
					ConfigurationID string `json:"configurationId"`
					Bucket          struct {
						Name string `json:"name"`
						ARN  string `json:"arn"`
					} `json:"bucket"`
					Object struct {
						Key       string `json:"key"`
						Size      int64  `json:"size"`
						ETag      string `json:"eTag"`
						VersionID string `json:"versionId,omitempty"`
					} `json:"object"`
				}{
					S3SchemaVersion: "1.0",
					ConfigurationID: "test-config",
					Bucket: struct {
						Name string `json:"name"`
						ARN  string `json:"arn"`
					}{
						Name: "test-change-bucket",
						ARN:  "arn:aws:s3:::test-change-bucket",
					},
					Object: struct {
						Key       string `json:"key"`
						Size      int64  `json:"size"`
						ETag      string `json:"eTag"`
						VersionID string `json:"versionId,omitempty"`
					}{
						Key:  "changes/TEST-LOOP-001.json",
						Size: 2048,
						ETag: "d41d8cd98f00b204e9800998ecf8427e",
					},
				},
			},
		},
	}

	// Serialize to JSON (simulating SQS message body)
	eventJSON, err := json.Marshal(s3Event)
	if err != nil {
		t.Fatalf("Failed to marshal S3 event: %v", err)
	}

	// Deserialize (simulating SQS message processing)
	var parsedEvent S3EventNotification
	err = json.Unmarshal(eventJSON, &parsedEvent)
	if err != nil {
		t.Fatalf("Failed to unmarshal S3 event: %v", err)
	}

	// Test event loop prevention logic
	if len(parsedEvent.Records) != 1 {
		t.Fatalf("Expected 1 S3 record, got %d", len(parsedEvent.Records))
	}

	record := parsedEvent.Records[0]
	shouldDiscard := isBackendGeneratedEvent(&record, backendRoleARN)

	if !shouldDiscard {
		t.Error("Expected backend-generated event to be discarded")
	}

	// Verify the event details are preserved correctly
	if record.UserIdentity.ARN != backendRoleARN {
		t.Errorf("Expected user identity ARN %s, got %s", backendRoleARN, record.UserIdentity.ARN)
	}

	if record.S3.Object.Key != "changes/TEST-LOOP-001.json" {
		t.Errorf("Expected object key 'changes/TEST-LOOP-001.json', got %s", record.S3.Object.Key)
	}
}

// TestEventProcessingDecisionLogging tests that event processing decisions are properly structured for logging
func TestEventProcessingDecisionLogging(t *testing.T) {
	backendRoleARN := "arn:aws:iam::123456789012:role/backend-lambda-role"

	tests := []struct {
		name           string
		event          *S3EventRecord
		expectedAction string
		expectedReason string
	}{
		{
			name: "backend event discarded",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type: "AssumedRole",
					ARN:  backendRoleARN,
				},
				S3: struct {
					S3SchemaVersion string `json:"s3SchemaVersion"`
					ConfigurationID string `json:"configurationId"`
					Bucket          struct {
						Name string `json:"name"`
						ARN  string `json:"arn"`
					} `json:"bucket"`
					Object struct {
						Key       string `json:"key"`
						Size      int64  `json:"size"`
						ETag      string `json:"eTag"`
						VersionID string `json:"versionId,omitempty"`
					} `json:"object"`
				}{
					Object: struct {
						Key       string `json:"key"`
						Size      int64  `json:"size"`
						ETag      string `json:"eTag"`
						VersionID string `json:"versionId,omitempty"`
					}{
						Key: "changes/BACKEND-001.json",
					},
				},
			},
			expectedAction: "discard",
			expectedReason: "backend_generated_event",
		},
		{
			name: "frontend event processed",
			event: &S3EventRecord{
				EventName: "s3:ObjectCreated:Put",
				UserIdentity: &S3UserIdentity{
					Type: "AssumedRole",
					ARN:  "arn:aws:iam::123456789012:role/frontend-lambda-role",
				},
				S3: struct {
					S3SchemaVersion string `json:"s3SchemaVersion"`
					ConfigurationID string `json:"configurationId"`
					Bucket          struct {
						Name string `json:"name"`
						ARN  string `json:"arn"`
					} `json:"bucket"`
					Object struct {
						Key       string `json:"key"`
						Size      int64  `json:"size"`
						ETag      string `json:"eTag"`
						VersionID string `json:"versionId,omitempty"`
					} `json:"object"`
				}{
					Object: struct {
						Key       string `json:"key"`
						Size      int64  `json:"size"`
						ETag      string `json:"eTag"`
						VersionID string `json:"versionId,omitempty"`
					}{
						Key: "changes/FRONTEND-001.json",
					},
				},
			},
			expectedAction: "process",
			expectedReason: "frontend_generated_event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := makeEventProcessingDecision(tt.event, backendRoleARN)

			if decision.Action != tt.expectedAction {
				t.Errorf("Expected action %s, got %s", tt.expectedAction, decision.Action)
			}

			if decision.Reason != tt.expectedReason {
				t.Errorf("Expected reason %s, got %s", tt.expectedReason, decision.Reason)
			}

			// Verify decision contains event details for logging
			if decision.EventName != tt.event.EventName {
				t.Errorf("Expected event name %s, got %s", tt.event.EventName, decision.EventName)
			}

			if decision.ObjectKey != tt.event.S3.Object.Key {
				t.Errorf("Expected object key %s, got %s", tt.event.S3.Object.Key, decision.ObjectKey)
			}
		})
	}
}

// EventProcessingDecision represents a decision about whether to process an S3 event
type EventProcessingDecision struct {
	Action    string `json:"action"`    // "process" or "discard"
	Reason    string `json:"reason"`    // reason for the decision
	EventName string `json:"eventName"` // S3 event name for logging
	ObjectKey string `json:"objectKey"` // S3 object key for logging
	UserARN   string `json:"userArn"`   // User ARN for logging
}

// makeEventProcessingDecision creates a structured decision for event processing
func makeEventProcessingDecision(event *S3EventRecord, backendRoleARN string) EventProcessingDecision {
	decision := EventProcessingDecision{
		EventName: event.EventName,
		ObjectKey: event.S3.Object.Key,
	}

	if event.UserIdentity != nil {
		decision.UserARN = event.UserIdentity.ARN
	}

	if isBackendGeneratedEvent(event, backendRoleARN) {
		decision.Action = "discard"
		decision.Reason = "backend_generated_event"
	} else {
		decision.Action = "process"
		if event.UserIdentity == nil {
			decision.Reason = "no_user_identity"
		} else if event.UserIdentity.ARN == "arn:aws:iam::123456789012:role/frontend-lambda-role" {
			decision.Reason = "frontend_generated_event"
		} else {
			decision.Reason = "user_generated_event"
		}
	}

	return decision
}

// TestEventLoopPreventionPerformance tests that event loop prevention is efficient
func TestEventLoopPreventionPerformance(t *testing.T) {
	backendRoleARN := "arn:aws:iam::123456789012:role/backend-lambda-role"

	// Create test event
	event := &S3EventRecord{
		EventName: "s3:ObjectCreated:Put",
		UserIdentity: &S3UserIdentity{
			Type: "AssumedRole",
			ARN:  backendRoleARN,
		},
	}

	// Test that event loop prevention is fast (should be simple string comparison)
	start := time.Now()
	for i := 0; i < 10000; i++ {
		isBackendGeneratedEvent(event, backendRoleARN)
	}
	duration := time.Since(start)

	// Should complete 10,000 checks in well under a second
	if duration > time.Second {
		t.Errorf("Event loop prevention too slow: %v for 10,000 checks", duration)
	}

	t.Logf("Event loop prevention performance: %v for 10,000 checks", duration)
}
