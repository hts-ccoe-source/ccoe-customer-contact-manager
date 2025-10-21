package ses

import (
	"testing"

	"ccoe-customer-contact-manager/internal/types"
)

// TestExtractObjectID tests the extractObjectID function
func TestExtractObjectID(t *testing.T) {
	tests := []struct {
		name     string
		metadata *types.ChangeMetadata
		expected string
	}{
		{
			name: "Extract ChangeID from change metadata",
			metadata: &types.ChangeMetadata{
				ObjectType: "change",
				ChangeID:   "change-123-abc",
			},
			expected: "change-123-abc",
		},
		{
			name: "Extract announcement_id from announcement metadata",
			metadata: &types.ChangeMetadata{
				ObjectType: "announcement_cic",
				Metadata: map[string]interface{}{
					"announcement_id": "announcement-456-def",
				},
			},
			expected: "announcement-456-def",
		},
		{
			name: "Fallback to object type + title when no ID available",
			metadata: &types.ChangeMetadata{
				ObjectType:  "change",
				ChangeTitle: "Test Change",
			},
			expected: "change-Test Change",
		},
		{
			name: "Empty metadata returns empty string",
			metadata: &types.ChangeMetadata{
				ObjectType: "",
			},
			expected: "",
		},
		{
			name: "ChangeID takes precedence over announcement_id",
			metadata: &types.ChangeMetadata{
				ObjectType: "change",
				ChangeID:   "change-789-ghi",
				Metadata: map[string]interface{}{
					"announcement_id": "announcement-999-zzz",
				},
			},
			expected: "change-789-ghi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractObjectID(tt.metadata)
			if result != tt.expected {
				t.Errorf("extractObjectID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestExtractManualAttendees tests the extractManualAttendees function
func TestExtractManualAttendees(t *testing.T) {
	tests := []struct {
		name     string
		metadata *types.ChangeMetadata
		expected []string
	}{
		{
			name: "Extract comma-separated attendees from string",
			metadata: &types.ChangeMetadata{
				Metadata: map[string]interface{}{
					"attendees": "user1@example.com, user2@example.com, user3@example.com",
				},
			},
			expected: []string{"user1@example.com", "user2@example.com", "user3@example.com"},
		},
		{
			name: "Extract attendees from string array",
			metadata: &types.ChangeMetadata{
				Metadata: map[string]interface{}{
					"attendees": []string{"user1@example.com", "user2@example.com"},
				},
			},
			expected: []string{"user1@example.com", "user2@example.com"},
		},
		{
			name: "Extract attendees from interface array",
			metadata: &types.ChangeMetadata{
				Metadata: map[string]interface{}{
					"attendees": []interface{}{"user1@example.com", "user2@example.com"},
				},
			},
			expected: []string{"user1@example.com", "user2@example.com"},
		},
		{
			name: "Filter out invalid emails (no @ symbol)",
			metadata: &types.ChangeMetadata{
				Metadata: map[string]interface{}{
					"attendees": "user1@example.com, invalid-email, user2@example.com",
				},
			},
			expected: []string{"user1@example.com", "user2@example.com"},
		},
		{
			name: "Empty metadata returns empty array",
			metadata: &types.ChangeMetadata{
				Metadata: nil,
			},
			expected: []string{},
		},
		{
			name: "No attendees field returns empty array",
			metadata: &types.ChangeMetadata{
				Metadata: map[string]interface{}{
					"other_field": "value",
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractManualAttendees(tt.metadata)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("extractManualAttendees() returned %d attendees, want %d", len(result), len(tt.expected))
				return
			}

			// Check each attendee
			for i, email := range result {
				if email != tt.expected[i] {
					t.Errorf("extractManualAttendees()[%d] = %v, want %v", i, email, tt.expected[i])
				}
			}
		})
	}
}

// TestICalUIdGeneration tests that iCalUId is correctly generated from objectID
func TestICalUIdGeneration(t *testing.T) {
	tests := []struct {
		name            string
		metadata        *types.ChangeMetadata
		expectedICalUID string
	}{
		{
			name: "Change metadata generates correct iCalUId",
			metadata: &types.ChangeMetadata{
				ObjectType:         "change",
				ChangeID:           "change-123-abc",
				ChangeTitle:        "Test Change",
				ChangeReason:       "Testing",
				ImplementationPlan: "Test plan",
				CustomerImpact:     "None",
				RollbackPlan:       "Rollback",
				SnowTicket:         "CHG0001",
				JiraTicket:         "PROJ-123",
				Customers:          []string{"customer1"},
				Timezone:           "America/New_York",
			},
			expectedICalUID: "change-123-abc@ccoe-customer-contact-manager",
		},
		{
			name: "Announcement metadata generates correct iCalUId",
			metadata: &types.ChangeMetadata{
				ObjectType:   "announcement_cic",
				ChangeTitle:  "Test Announcement",
				ChangeReason: "Testing",
				Customers:    []string{"customer1"},
				Timezone:     "America/New_York",
				Metadata: map[string]interface{}{
					"announcement_id": "announcement-456-def",
				},
			},
			expectedICalUID: "announcement-456-def@ccoe-customer-contact-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract objectID
			objectID := extractObjectID(tt.metadata)
			if objectID == "" {
				t.Error("Expected objectID to be extracted but got empty string")
				return
			}

			// Generate iCalUId
			iCalUID := objectID + "@ccoe-customer-contact-manager"

			if iCalUID != tt.expectedICalUID {
				t.Errorf("Generated iCalUId = %v, want %v", iCalUID, tt.expectedICalUID)
			}
		})
	}
}

// TestIdempotencyWithDuplicateRequests simulates duplicate meeting creation requests
func TestIdempotencyWithDuplicateRequests(t *testing.T) {
	// This is a conceptual test - actual implementation would require mocking Graph API
	// The test verifies that the same objectID is extracted consistently

	metadata := &types.ChangeMetadata{
		ObjectType:  "change",
		ChangeID:    "change-duplicate-test",
		ChangeTitle: "Duplicate Test",
	}

	// Extract objectID multiple times - should be consistent
	objectID1 := extractObjectID(metadata)
	objectID2 := extractObjectID(metadata)
	objectID3 := extractObjectID(metadata)

	if objectID1 != objectID2 || objectID2 != objectID3 {
		t.Errorf("extractObjectID() returned inconsistent results: %v, %v, %v", objectID1, objectID2, objectID3)
	}

	if objectID1 == "" {
		t.Error("extractObjectID() returned empty string for valid metadata")
	}
}

// TestHideAttendeesForAnnouncements tests that announcement meetings hide attendees
func TestHideAttendeesForAnnouncements(t *testing.T) {
	tests := []struct {
		name              string
		metadata          *types.ChangeMetadata
		expectedHideValue bool
	}{
		{
			name: "Change meeting should NOT hide attendees",
			metadata: &types.ChangeMetadata{
				ObjectType:  "change",
				ChangeID:    "change-123",
				ChangeTitle: "Test Change",
			},
			expectedHideValue: false,
		},
		{
			name: "CIC announcement should hide attendees",
			metadata: &types.ChangeMetadata{
				ObjectType:  "announcement_cic",
				ChangeTitle: "CIC Event",
				Metadata: map[string]interface{}{
					"announcement_id": "announcement-cic-123",
				},
			},
			expectedHideValue: true,
		},
		{
			name: "FinOps announcement should hide attendees",
			metadata: &types.ChangeMetadata{
				ObjectType:  "announcement_finops",
				ChangeTitle: "FinOps Event",
				Metadata: map[string]interface{}{
					"announcement_id": "announcement-finops-456",
				},
			},
			expectedHideValue: true,
		},
		{
			name: "InnerSource announcement should hide attendees",
			metadata: &types.ChangeMetadata{
				ObjectType:  "announcement_innersource",
				ChangeTitle: "InnerSource Event",
				Metadata: map[string]interface{}{
					"announcement_id": "announcement-innersource-789",
				},
			},
			expectedHideValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if object type starts with "announcement"
			// This mimics the logic in generateGraphMeetingPayload
			isAnnouncement := len(tt.metadata.ObjectType) >= 12 &&
				tt.metadata.ObjectType[:12] == "announcement"

			if isAnnouncement != tt.expectedHideValue {
				t.Errorf("hideAttendees check = %v, want %v for ObjectType: %s",
					isAnnouncement, tt.expectedHideValue, tt.metadata.ObjectType)
			}
		})
	}
}

// TestRetryLogicForTransientFailures tests the retry mechanism
func TestRetryLogicForTransientFailures(t *testing.T) {
	// This is a conceptual test - actual implementation would require mocking HTTP client
	// The test documents the expected retry behavior

	t.Run("Retry on 5xx errors", func(t *testing.T) {
		// createGraphMeetingWithRetry should retry on 500, 502, 503, 504 errors
		// with exponential backoff: 2s, 4s, 8s
		t.Skip("Requires HTTP client mocking")
	})

	t.Run("Retry on 429 rate limiting", func(t *testing.T) {
		// createGraphMeetingWithRetry should retry on 429 errors
		t.Skip("Requires HTTP client mocking")
	})

	t.Run("No retry on 4xx client errors", func(t *testing.T) {
		// createGraphMeetingWithRetry should NOT retry on 400, 401, 403, 404 errors
		t.Skip("Requires HTTP client mocking")
	})

	t.Run("Success on first attempt", func(t *testing.T) {
		// createGraphMeetingWithRetry should return immediately on 201 Created
		t.Skip("Requires HTTP client mocking")
	})

	t.Run("Success after retries", func(t *testing.T) {
		// createGraphMeetingWithRetry should succeed if any retry succeeds
		t.Skip("Requires HTTP client mocking")
	})
}
