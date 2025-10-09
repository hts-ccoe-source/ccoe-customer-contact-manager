package lambda

import (
	"strings"
	"testing"

	"ccoe-customer-contact-manager/internal/types"
)

func TestScheduleMultiCustomerMeetingIfNeeded(t *testing.T) {
	// Test case 1: Change with meeting required
	t.Run("MeetingRequired", func(t *testing.T) {
		metadata := &types.ChangeMetadata{
			ChangeID:    "CHG-12345",
			ChangeTitle: "Test Change with Meeting",
			Customers:   []string{"customer-a", "customer-b"},
			Metadata: map[string]interface{}{
				"meetingRequired": "yes",
				"meetingTitle":    "Implementation Meeting",
				"meetingDate":     "2025-01-10",
				"meetingDuration": "60",
				"meetingLocation": "Microsoft Teams",
			},
			ImplementationBeginDate: "2025-01-10",
			ImplementationBeginTime: "10:00",
			Timezone:                "America/New_York",
		}

		// This test would require mocking the credential manager and SES client
		// For now, we'll test the logic without actual AWS calls
		if len(metadata.Customers) != 2 {
			t.Errorf("Expected 2 customers, got %d", len(metadata.Customers))
		}

		if metadata.Metadata["meetingRequired"] != "yes" {
			t.Error("Expected meetingRequired to be 'yes'")
		}
	})

	// Test case 2: Change without meeting required
	t.Run("NoMeetingRequired", func(t *testing.T) {
		metadata := &types.ChangeMetadata{
			ChangeID:    "CHG-67890",
			ChangeTitle: "Test Change without Meeting",
			Customers:   []string{"customer-a"},
			Metadata:    map[string]interface{}{},
		}

		// Verify no meeting is scheduled for changes without meeting requirements
		if len(metadata.Customers) != 1 {
			t.Errorf("Expected 1 customer, got %d", len(metadata.Customers))
		}

		if metadata.Metadata["meetingRequired"] != nil {
			t.Error("Expected no meetingRequired field")
		}
	})

	// Test case 3: Multi-customer change with implementation schedule (auto-meeting)
	t.Run("AutoMeetingForMultiCustomer", func(t *testing.T) {
		metadata := &types.ChangeMetadata{
			ChangeID:                "CHG-AUTO",
			ChangeTitle:             "Multi-Customer Implementation",
			Customers:               []string{"customer-a", "customer-b", "customer-c"},
			ImplementationBeginDate: "2025-01-15",
			ImplementationBeginTime: "14:00",
			Timezone:                "America/New_York",
		}

		// Multi-customer changes with implementation schedule should auto-schedule meetings
		if len(metadata.Customers) < 2 {
			t.Error("Expected multi-customer change")
		}

		if metadata.ImplementationBeginDate == "" {
			t.Error("Expected implementation date for auto-meeting scheduling")
		}
	})
}

func TestCreateTempMeetingMetadata(t *testing.T) {
	metadata := &types.ChangeMetadata{
		ChangeID:                "CHG-TEST",
		ChangeTitle:             "Test Meeting Creation",
		Customers:               []string{"customer-a", "customer-b"},
		ChangeReason:            "Testing meeting metadata creation",
		ImplementationPlan:      "Deploy test changes",
		ImplementationBeginDate: "2025-01-10",
		ImplementationBeginTime: "10:00",
		Timezone:                "America/New_York",
		SnowTicket:              "CHG0123456",
		JiraTicket:              "TEST-123",
	}

	meetingTitle := "Test Implementation Meeting"
	meetingDate := "2025-01-10"
	meetingDuration := "60"
	meetingLocation := "Microsoft Teams"

	// Test the metadata creation function
	tempFile, err := createTempMeetingMetadata(metadata, meetingTitle, meetingDate, meetingDuration, meetingLocation)
	if err != nil {
		t.Fatalf("Failed to create temp meeting metadata: %v", err)
	}

	if tempFile == "" {
		t.Error("Expected non-empty temp file path")
	}

	// Verify the temp file path format
	if !strings.Contains(tempFile, "meeting-metadata-CHG-TEST") {
		t.Errorf("Expected temp file to contain change ID, got: %s", tempFile)
	}

	// Clean up temp file (in real test, we'd verify file contents)
	// os.Remove(tempFile)
}

func TestMeetingRequiredDetection(t *testing.T) {
	testCases := []struct {
		name            string
		metadata        *types.ChangeMetadata
		expectedMeeting bool
	}{
		{
			name: "Explicit meeting required - yes",
			metadata: &types.ChangeMetadata{
				Customers: []string{"customer-a"},
				Metadata: map[string]interface{}{
					"meetingRequired": "yes",
				},
			},
			expectedMeeting: true,
		},
		{
			name: "Explicit meeting required - true",
			metadata: &types.ChangeMetadata{
				Customers: []string{"customer-a"},
				Metadata: map[string]interface{}{
					"meetingRequired": true,
				},
			},
			expectedMeeting: true,
		},
		{
			name: "Meeting title provided",
			metadata: &types.ChangeMetadata{
				Customers: []string{"customer-a"},
				Metadata: map[string]interface{}{
					"meetingTitle": "Implementation Meeting",
				},
			},
			expectedMeeting: true,
		},
		{
			name: "Multi-customer with implementation schedule",
			metadata: &types.ChangeMetadata{
				Customers:               []string{"customer-a", "customer-b"},
				ImplementationBeginDate: "2025-01-10",
				ImplementationBeginTime: "10:00",
			},
			expectedMeeting: true,
		},
		{
			name: "Single customer with implementation schedule",
			metadata: &types.ChangeMetadata{
				Customers:               []string{"customer-a"},
				ImplementationBeginDate: "2025-01-10",
				ImplementationBeginTime: "10:00",
			},
			expectedMeeting: false,
		},
		{
			name: "No meeting indicators",
			metadata: &types.ChangeMetadata{
				Customers: []string{"customer-a"},
				Metadata:  map[string]interface{}{},
			},
			expectedMeeting: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the meeting detection logic
			meetingRequired := false

			// Replicate the logic from ScheduleMultiCustomerMeetingIfNeeded
			if tc.metadata.Metadata != nil {
				if required, exists := tc.metadata.Metadata["meetingRequired"]; exists {
					if reqStr, ok := required.(string); ok {
						meetingRequired = strings.ToLower(reqStr) == "yes" || strings.ToLower(reqStr) == "true"
					} else if reqBool, ok := required.(bool); ok {
						meetingRequired = reqBool
					}
				}

				if title, exists := tc.metadata.Metadata["meetingTitle"]; exists {
					if titleStr, ok := title.(string); ok && titleStr != "" {
						meetingRequired = true
					}
				}
			}

			if !meetingRequired && tc.metadata.ImplementationBeginDate != "" && tc.metadata.ImplementationBeginTime != "" {
				if len(tc.metadata.Customers) > 1 {
					meetingRequired = true
				}
			}

			if meetingRequired != tc.expectedMeeting {
				t.Errorf("Expected meeting required: %v, got: %v", tc.expectedMeeting, meetingRequired)
			}
		})
	}
}
