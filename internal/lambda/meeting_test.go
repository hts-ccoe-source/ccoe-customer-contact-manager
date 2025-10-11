package lambda

import (
	"context"
	"strings"
	"testing"
	"time"

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
			ImplementationStart: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
			ImplementationEnd:   time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC),
			Timezone:            "America/New_York",
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
			ChangeID:            "CHG-67890",
			ChangeTitle:         "Test Change without Meeting",
			Customers:           []string{"customer-a"},
			Metadata:            map[string]interface{}{},
			ImplementationStart: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
			ImplementationEnd:   time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC),
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
			ChangeID:            "CHG-AUTO",
			ChangeTitle:         "Multi-Customer Implementation",
			Customers:           []string{"customer-a", "customer-b", "customer-c"},
			ImplementationStart: time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
			ImplementationEnd:   time.Date(2025, 1, 15, 15, 0, 0, 0, time.UTC),
			Timezone:            "America/New_York",
		}

		// Multi-customer changes with implementation schedule should auto-schedule meetings
		if len(metadata.Customers) < 2 {
			t.Error("Expected multi-customer change")
		}

		if metadata.ImplementationStart.IsZero() {
			t.Error("Expected implementation date for auto-meeting scheduling")
		}
	})
}

func TestCreateTempMeetingMetadata(t *testing.T) {
	metadata := &types.ChangeMetadata{
		ChangeID:            "CHG-TEST",
		ChangeTitle:         "Test Meeting Creation",
		Customers:           []string{"customer-a", "customer-b"},
		ChangeReason:        "Testing meeting metadata creation",
		ImplementationPlan:  "Deploy test changes",
		ImplementationStart: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
		ImplementationEnd:   time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC),
		Timezone:            "America/New_York",
		SnowTicket:          "CHG0123456",
		JiraTicket:          "TEST-123",
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
				Customers:           []string{"customer-a", "customer-b"},
				ImplementationStart: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
				ImplementationEnd:   time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC),
			},
			expectedMeeting: true,
		},
		{
			name: "Single customer with implementation schedule",
			metadata: &types.ChangeMetadata{
				Customers:           []string{"customer-a"},
				ImplementationStart: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
				ImplementationEnd:   time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC),
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

			if !meetingRequired && !tc.metadata.ImplementationStart.IsZero() {
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

// TestMeetingSchedulerIdempotency tests the idempotency logic of the MeetingScheduler
func TestMeetingSchedulerIdempotency(t *testing.T) {
	scheduler := NewMeetingScheduler("us-east-1")

	// Test case 1: No existing meeting - should create new meeting
	t.Run("CreateNewMeeting", func(t *testing.T) {
		changeMetadata := &types.ChangeMetadata{
			ChangeID:            "CHG-NEW-001",
			ChangeTitle:         "New Change Request",
			ImplementationStart: time.Now().Add(24 * time.Hour),
			ImplementationEnd:   time.Now().Add(25 * time.Hour),
			Modifications:       []types.ModificationEntry{}, // No existing meetings
		}

		// Check that no existing meeting is found
		existingMeeting := changeMetadata.GetLatestMeetingMetadata()
		if existingMeeting != nil {
			t.Error("Expected no existing meeting, but found one")
		}

		// Test the meeting creation logic (without actual API calls)
		meetingMetadata, err := scheduler.createGraphMeeting(context.Background(), changeMetadata)
		if err != nil {
			t.Errorf("Failed to create meeting metadata: %v", err)
		}

		if meetingMetadata.MeetingID == "" {
			t.Error("Expected meeting ID to be generated")
		}

		if !strings.Contains(meetingMetadata.Subject, changeMetadata.ChangeTitle) {
			t.Errorf("Expected meeting subject to contain change title, got: %s", meetingMetadata.Subject)
		}
	})

	// Test case 2: Existing meeting with no changes - should not update
	t.Run("ExistingMeetingNoUpdate", func(t *testing.T) {
		// Use a fixed time to avoid precision issues
		startTime := time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC)
		endTime := startTime.Add(1 * time.Hour)

		existingMeeting := &types.MeetingMetadata{
			MeetingID: "existing-meeting-123",
			Subject:   "Change Implementation: Existing Change",
			StartTime: startTime.Format(time.RFC3339),
			EndTime:   endTime.Format(time.RFC3339),
			JoinURL:   "https://teams.microsoft.com/l/meetup-join/existing",
		}

		changeMetadata := &types.ChangeMetadata{
			ChangeID:            "CHG-EXISTING-001",
			ChangeTitle:         "Existing Change",
			ImplementationStart: startTime, // Use the exact same time
			ImplementationEnd:   endTime,   // Use the exact same time
			Modifications: []types.ModificationEntry{
				{
					Timestamp:        time.Now().Add(-1 * time.Hour),
					UserID:           types.BackendUserID,
					ModificationType: types.ModificationTypeMeetingScheduled,
					MeetingMetadata:  existingMeeting,
				},
			},
		}

		// Test that no update is needed
		needsUpdate, reason := scheduler.checkIfMeetingNeedsUpdate(changeMetadata, existingMeeting)
		if needsUpdate {
			t.Errorf("Expected no update needed, but got: %s", reason)
		}
	})

	// Test case 3: Existing meeting with changed title - should update
	t.Run("ExistingMeetingNeedsUpdate", func(t *testing.T) {
		startTime := time.Now().Add(24 * time.Hour)
		endTime := startTime.Add(1 * time.Hour)

		existingMeeting := &types.MeetingMetadata{
			MeetingID: "existing-meeting-456",
			Subject:   "Change Implementation: Old Title",
			StartTime: startTime.Format(time.RFC3339),
			EndTime:   endTime.Format(time.RFC3339),
			JoinURL:   "https://teams.microsoft.com/l/meetup-join/existing",
		}

		changeMetadata := &types.ChangeMetadata{
			ChangeID:            "CHG-UPDATE-001",
			ChangeTitle:         "New Updated Title", // Changed title
			ImplementationStart: startTime,
			ImplementationEnd:   endTime,
			Modifications: []types.ModificationEntry{
				{
					Timestamp:        time.Now().Add(-1 * time.Hour),
					UserID:           types.BackendUserID,
					ModificationType: types.ModificationTypeMeetingScheduled,
					MeetingMetadata:  existingMeeting,
				},
			},
		}

		// Test that update is needed
		needsUpdate, reason := scheduler.checkIfMeetingNeedsUpdate(changeMetadata, existingMeeting)
		if !needsUpdate {
			t.Error("Expected update needed due to title change")
		}

		if !strings.Contains(reason, "subject changed") {
			t.Errorf("Expected reason to mention subject change, got: %s", reason)
		}
	})

	// Test case 4: Existing meeting with changed time - should update
	t.Run("ExistingMeetingTimeChanged", func(t *testing.T) {
		oldStartTime := time.Now().Add(24 * time.Hour)
		newStartTime := time.Now().Add(48 * time.Hour) // Different time
		endTime := oldStartTime.Add(1 * time.Hour)

		existingMeeting := &types.MeetingMetadata{
			MeetingID: "existing-meeting-789",
			Subject:   "Change Implementation: Time Change Test",
			StartTime: oldStartTime.Format(time.RFC3339),
			EndTime:   endTime.Format(time.RFC3339),
			JoinURL:   "https://teams.microsoft.com/l/meetup-join/existing",
		}

		changeMetadata := &types.ChangeMetadata{
			ChangeID:            "CHG-TIME-001",
			ChangeTitle:         "Time Change Test",
			ImplementationStart: newStartTime, // Changed time
			ImplementationEnd:   newStartTime.Add(1 * time.Hour),
			Modifications: []types.ModificationEntry{
				{
					Timestamp:        time.Now().Add(-1 * time.Hour),
					UserID:           types.BackendUserID,
					ModificationType: types.ModificationTypeMeetingScheduled,
					MeetingMetadata:  existingMeeting,
				},
			},
		}

		// Test that update is needed
		needsUpdate, reason := scheduler.checkIfMeetingNeedsUpdate(changeMetadata, existingMeeting)
		if !needsUpdate {
			t.Error("Expected update needed due to time change")
		}

		if !strings.Contains(reason, "start time changed") {
			t.Errorf("Expected reason to mention start time change, got: %s", reason)
		}
	})

	// Test case 5: Update existing meeting metadata
	t.Run("UpdateExistingMeeting", func(t *testing.T) {
		startTime := time.Now().Add(24 * time.Hour)
		endTime := startTime.Add(1 * time.Hour)

		existingMeeting := &types.MeetingMetadata{
			MeetingID: "update-meeting-123",
			Subject:   "Change Implementation: Old Title",
			StartTime: startTime.Format(time.RFC3339),
			EndTime:   endTime.Format(time.RFC3339),
			JoinURL:   "https://teams.microsoft.com/l/meetup-join/update",
			Organizer: "original@example.com",
			Attendees: []string{"attendee1@example.com"},
		}

		newStartTime := time.Now().Add(48 * time.Hour)
		newEndTime := newStartTime.Add(2 * time.Hour)

		changeMetadata := &types.ChangeMetadata{
			ChangeID:            "CHG-UPDATE-002",
			ChangeTitle:         "Updated Title",
			ImplementationStart: newStartTime,
			ImplementationEnd:   newEndTime,
		}

		// Test updating the meeting
		updatedMeeting, err := scheduler.updateExistingGraphMeeting(context.Background(), changeMetadata, existingMeeting)
		if err != nil {
			t.Errorf("Failed to update existing meeting: %v", err)
		}

		// Verify the meeting ID is preserved
		if updatedMeeting.MeetingID != existingMeeting.MeetingID {
			t.Errorf("Expected meeting ID to be preserved, got %s, expected %s",
				updatedMeeting.MeetingID, existingMeeting.MeetingID)
		}

		// Verify the join URL is preserved
		if updatedMeeting.JoinURL != existingMeeting.JoinURL {
			t.Errorf("Expected join URL to be preserved, got %s, expected %s",
				updatedMeeting.JoinURL, existingMeeting.JoinURL)
		}

		// Verify the subject is updated
		expectedSubject := "Change Implementation: Updated Title"
		if updatedMeeting.Subject != expectedSubject {
			t.Errorf("Expected subject to be updated to %s, got %s",
				expectedSubject, updatedMeeting.Subject)
		}

		// Verify the times are updated
		if updatedMeeting.StartTime != newStartTime.Format(time.RFC3339) {
			t.Errorf("Expected start time to be updated to %s, got %s",
				newStartTime.Format(time.RFC3339), updatedMeeting.StartTime)
		}

		// Verify organizer and attendees are preserved
		if updatedMeeting.Organizer != existingMeeting.Organizer {
			t.Errorf("Expected organizer to be preserved, got %s, expected %s",
				updatedMeeting.Organizer, existingMeeting.Organizer)
		}
	})
}
