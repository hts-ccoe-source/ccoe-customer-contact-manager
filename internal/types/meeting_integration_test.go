package types

import (
	"testing"
	"time"
)

// TestMeetingSchedulingIdempotency tests the idempotency logic for meeting scheduling
func TestMeetingSchedulingIdempotency(t *testing.T) {
	changeMetadata := &ChangeMetadata{
		ChangeID:    "TEST-IDEMPOTENT-001",
		ChangeTitle: "Idempotency Test Change",
	}

	// Test 1: First meeting scheduling (should create new meeting)
	firstMeetingMetadata := &MeetingMetadata{
		MeetingID: "first-meeting-123",
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/first",
		StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
		Subject:   "First Meeting",
	}

	firstEntry, err := NewMeetingScheduledEntry(BackendUserID, firstMeetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create first meeting entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(firstEntry)
	if err != nil {
		t.Fatalf("Failed to add first meeting entry: %v", err)
	}

	// Verify first meeting is scheduled
	if !changeMetadata.HasMeetingScheduled() {
		t.Error("Expected meeting to be scheduled after first entry")
	}

	latestMeeting := changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Fatal("Expected to find meeting metadata after first scheduling")
	}

	if latestMeeting.MeetingID != "first-meeting-123" {
		t.Errorf("Expected first meeting ID 'first-meeting-123', got '%s'", latestMeeting.MeetingID)
	}

	// Test 2: Idempotent meeting update (should update existing meeting)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	updatedMeetingMetadata := &MeetingMetadata{
		MeetingID: "first-meeting-123", // Same meeting ID - should be an update
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/first-updated",
		StartTime: time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(49 * time.Hour).Format(time.RFC3339),
		Subject:   "Updated First Meeting",
	}

	updatedEntry, err := NewMeetingScheduledEntry(BackendUserID, updatedMeetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create updated meeting entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(updatedEntry)
	if err != nil {
		t.Fatalf("Failed to add updated meeting entry: %v", err)
	}

	// Verify the latest meeting is the updated one
	latestMeeting = changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Fatal("Expected to find meeting metadata after update")
	}

	if latestMeeting.MeetingID != "first-meeting-123" {
		t.Errorf("Expected same meeting ID 'first-meeting-123', got '%s'", latestMeeting.MeetingID)
	}

	if latestMeeting.Subject != "Updated First Meeting" {
		t.Errorf("Expected updated subject 'Updated First Meeting', got '%s'", latestMeeting.Subject)
	}

	// Test 3: New meeting scheduling (different meeting ID - should create new meeting)
	time.Sleep(1 * time.Millisecond)
	newMeetingMetadata := &MeetingMetadata{
		MeetingID: "second-meeting-456", // Different meeting ID - should be new
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/second",
		StartTime: time.Now().Add(72 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(73 * time.Hour).Format(time.RFC3339),
		Subject:   "Second Meeting",
	}

	newEntry, err := NewMeetingScheduledEntry(BackendUserID, newMeetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create new meeting entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(newEntry)
	if err != nil {
		t.Fatalf("Failed to add new meeting entry: %v", err)
	}

	// Verify the latest meeting is the new one
	latestMeeting = changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Fatal("Expected to find meeting metadata after new meeting")
	}

	if latestMeeting.MeetingID != "second-meeting-456" {
		t.Errorf("Expected new meeting ID 'second-meeting-456', got '%s'", latestMeeting.MeetingID)
	}

	// Verify we have 3 meeting entries total (first + update + new)
	meetingCount := 0
	for _, entry := range changeMetadata.Modifications {
		if entry.ModificationType == ModificationTypeMeetingScheduled {
			meetingCount++
		}
	}

	if meetingCount != 3 {
		t.Errorf("Expected 3 meeting scheduled entries, got %d", meetingCount)
	}
}

// TestMeetingCancellationWorkflow tests the complete meeting cancellation workflow
func TestMeetingCancellationWorkflow(t *testing.T) {
	changeMetadata := &ChangeMetadata{
		ChangeID:    "TEST-CANCEL-001",
		ChangeTitle: "Cancellation Test Change",
	}

	// Step 1: Schedule a meeting
	meetingMetadata := &MeetingMetadata{
		MeetingID: "cancel-test-meeting-789",
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/cancel-test",
		StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
		Subject:   "Meeting to be Cancelled",
		Organizer: "test-organizer@example.com",
		Attendees: []string{"attendee1@example.com", "attendee2@example.com"},
	}

	scheduledEntry, err := NewMeetingScheduledEntry(BackendUserID, meetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create meeting scheduled entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(scheduledEntry)
	if err != nil {
		t.Fatalf("Failed to add meeting scheduled entry: %v", err)
	}

	// Verify meeting is scheduled
	if !changeMetadata.HasMeetingScheduled() {
		t.Error("Expected meeting to be scheduled")
	}

	// Step 2: Cancel the meeting
	time.Sleep(1 * time.Millisecond)
	cancelledEntry, err := NewMeetingCancelledEntry(BackendUserID)
	if err != nil {
		t.Fatalf("Failed to create meeting cancelled entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(cancelledEntry)
	if err != nil {
		t.Fatalf("Failed to add meeting cancelled entry: %v", err)
	}

	// Step 3: Verify cancellation is recorded
	hasCancellation := false
	for _, entry := range changeMetadata.Modifications {
		if entry.ModificationType == ModificationTypeMeetingCancelled {
			hasCancellation = true
			break
		}
	}

	if !hasCancellation {
		t.Error("Expected to find meeting cancellation entry")
	}

	// Step 4: Verify we still have meeting metadata (for historical purposes)
	latestMeeting := changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Error("Expected to still have meeting metadata after cancellation")
	} else if latestMeeting.MeetingID != "cancel-test-meeting-789" {
		t.Errorf("Expected meeting ID 'cancel-test-meeting-789', got '%s'", latestMeeting.MeetingID)
	}

	// Step 5: Test multiple cancellations (should be allowed)
	time.Sleep(1 * time.Millisecond)
	secondCancelledEntry, err := NewMeetingCancelledEntry(BackendUserID)
	if err != nil {
		t.Fatalf("Failed to create second meeting cancelled entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(secondCancelledEntry)
	if err != nil {
		t.Fatalf("Failed to add second meeting cancelled entry: %v", err)
	}

	// Count cancellation entries
	cancellationCount := 0
	for _, entry := range changeMetadata.Modifications {
		if entry.ModificationType == ModificationTypeMeetingCancelled {
			cancellationCount++
		}
	}

	if cancellationCount != 2 {
		t.Errorf("Expected 2 meeting cancellation entries, got %d", cancellationCount)
	}
}

// TestMeetingMetadataRetrieval tests various scenarios for retrieving meeting metadata
func TestMeetingMetadataRetrieval(t *testing.T) {
	tests := []struct {
		name                    string
		setupModifications      func() []ModificationEntry
		expectedMeetingID       string
		expectedHasMeeting      bool
		expectedLatestMeetingID string
	}{
		{
			name: "no meetings scheduled",
			setupModifications: func() []ModificationEntry {
				return []ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeCreated,
					},
					{
						Timestamp:        time.Now().Add(1 * time.Hour),
						UserID:           "906638888d-1234-5678-9abc-123456789012",
						ModificationType: ModificationTypeApproved,
					},
				}
			},
			expectedHasMeeting:      false,
			expectedLatestMeetingID: "",
		},
		{
			name: "single meeting scheduled",
			setupModifications: func() []ModificationEntry {
				meetingMetadata := &MeetingMetadata{
					MeetingID: "single-meeting-123",
					JoinURL:   "https://teams.microsoft.com/l/meetup-join/single",
					StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
					Subject:   "Single Meeting",
				}

				return []ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeCreated,
					},
					{
						Timestamp:        time.Now().Add(1 * time.Hour),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeMeetingScheduled,
						MeetingMetadata:  meetingMetadata,
					},
				}
			},
			expectedHasMeeting:      true,
			expectedLatestMeetingID: "single-meeting-123",
		},
		{
			name: "multiple meetings scheduled",
			setupModifications: func() []ModificationEntry {
				firstMeeting := &MeetingMetadata{
					MeetingID: "first-meeting-456",
					JoinURL:   "https://teams.microsoft.com/l/meetup-join/first",
					StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
					Subject:   "First Meeting",
				}

				secondMeeting := &MeetingMetadata{
					MeetingID: "second-meeting-789",
					JoinURL:   "https://teams.microsoft.com/l/meetup-join/second",
					StartTime: time.Now().Add(48 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(49 * time.Hour).Format(time.RFC3339),
					Subject:   "Second Meeting",
				}

				return []ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeCreated,
					},
					{
						Timestamp:        time.Now().Add(1 * time.Hour),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeMeetingScheduled,
						MeetingMetadata:  firstMeeting,
					},
					{
						Timestamp:        time.Now().Add(2 * time.Hour),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeMeetingScheduled,
						MeetingMetadata:  secondMeeting,
					},
				}
			},
			expectedHasMeeting:      true,
			expectedLatestMeetingID: "second-meeting-789",
		},
		{
			name: "meeting scheduled then cancelled",
			setupModifications: func() []ModificationEntry {
				meetingMetadata := &MeetingMetadata{
					MeetingID: "cancelled-meeting-101",
					JoinURL:   "https://teams.microsoft.com/l/meetup-join/cancelled",
					StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
					Subject:   "Cancelled Meeting",
				}

				return []ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeCreated,
					},
					{
						Timestamp:        time.Now().Add(1 * time.Hour),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeMeetingScheduled,
						MeetingMetadata:  meetingMetadata,
					},
					{
						Timestamp:        time.Now().Add(2 * time.Hour),
						UserID:           BackendUserID,
						ModificationType: ModificationTypeMeetingCancelled,
					},
				}
			},
			expectedHasMeeting:      true, // Still has meeting (just cancelled)
			expectedLatestMeetingID: "cancelled-meeting-101",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changeMetadata := &ChangeMetadata{
				ChangeID:      "TEST-RETRIEVAL-" + tt.name,
				ChangeTitle:   "Retrieval Test Change",
				Modifications: tt.setupModifications(),
			}

			// Test HasMeetingScheduled
			hasMeeting := changeMetadata.HasMeetingScheduled()
			if hasMeeting != tt.expectedHasMeeting {
				t.Errorf("Expected HasMeetingScheduled=%v, got %v", tt.expectedHasMeeting, hasMeeting)
			}

			// Test GetLatestMeetingMetadata
			latestMeeting := changeMetadata.GetLatestMeetingMetadata()
			if tt.expectedLatestMeetingID == "" {
				if latestMeeting != nil {
					t.Errorf("Expected no latest meeting metadata, got meeting ID '%s'", latestMeeting.MeetingID)
				}
			} else {
				if latestMeeting == nil {
					t.Errorf("Expected latest meeting metadata with ID '%s', got nil", tt.expectedLatestMeetingID)
				} else if latestMeeting.MeetingID != tt.expectedLatestMeetingID {
					t.Errorf("Expected latest meeting ID '%s', got '%s'", tt.expectedLatestMeetingID, latestMeeting.MeetingID)
				}
			}
		})
	}
}

// TestGraphMeetingResponseConversion tests conversion from Microsoft Graph API responses
func TestGraphMeetingResponseConversion(t *testing.T) {
	tests := []struct {
		name        string
		response    *GraphMeetingResponse
		joinURL     string
		wantErr     bool
		expectedID  string
		description string
	}{
		{
			name: "valid graph response",
			response: &GraphMeetingResponse{
				ID:      "graph-meeting-123",
				Subject: "Test Graph Meeting",
				Start: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-15T14:00:00.0000000",
					TimeZone: "UTC",
				},
				End: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-15T15:00:00.0000000",
					TimeZone: "UTC",
				},
			},
			joinURL:     "https://teams.microsoft.com/l/meetup-join/graph-test",
			wantErr:     false,
			expectedID:  "graph-meeting-123",
			description: "Valid Graph API response should convert successfully",
		},
		{
			name: "graph response with body content",
			response: &GraphMeetingResponse{
				ID:      "graph-meeting-with-body-456",
				Subject: "Meeting with Body",
				Body: &struct {
					ContentType string `json:"contentType"`
					Content     string `json:"content"`
				}{
					ContentType: "html",
					Content:     "<p>Meeting description</p>",
				},
				Start: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-16T10:00:00.0000000",
					TimeZone: "Pacific Standard Time",
				},
				End: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-16T11:00:00.0000000",
					TimeZone: "Pacific Standard Time",
				},
			},
			joinURL:     "https://teams.microsoft.com/l/meetup-join/graph-body-test",
			wantErr:     false,
			expectedID:  "graph-meeting-with-body-456",
			description: "Graph response with body content should convert successfully",
		},
		{
			name:        "nil graph response",
			response:    nil,
			joinURL:     "https://teams.microsoft.com/l/meetup-join/nil-test",
			wantErr:     true,
			description: "Nil Graph response should return error",
		},
		{
			name: "graph response missing ID",
			response: &GraphMeetingResponse{
				ID:      "", // Missing ID
				Subject: "Meeting without ID",
				Start: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-15T14:00:00.0000000",
					TimeZone: "UTC",
				},
				End: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-15T15:00:00.0000000",
					TimeZone: "UTC",
				},
			},
			joinURL:     "https://teams.microsoft.com/l/meetup-join/no-id-test",
			wantErr:     true,
			description: "Graph response without ID should return error",
		},
		{
			name: "graph response with invalid datetime",
			response: &GraphMeetingResponse{
				ID:      "graph-meeting-invalid-time",
				Subject: "Meeting with Invalid Time",
				Start: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "invalid-datetime-format",
					TimeZone: "UTC",
				},
				End: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-15T15:00:00.0000000",
					TimeZone: "UTC",
				},
			},
			joinURL:     "https://teams.microsoft.com/l/meetup-join/invalid-time-test",
			wantErr:     true,
			description: "Graph response with invalid datetime should return error",
		},
		{
			name: "valid response with empty join URL",
			response: &GraphMeetingResponse{
				ID:      "graph-meeting-no-join",
				Subject: "Meeting without Join URL",
				Start: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-15T14:00:00.0000000",
					TimeZone: "UTC",
				},
				End: &struct {
					DateTime string `json:"dateTime"`
					TimeZone string `json:"timeZone"`
				}{
					DateTime: "2024-12-15T15:00:00.0000000",
					TimeZone: "UTC",
				},
			},
			joinURL:     "", // Empty join URL
			wantErr:     true,
			description: "Empty join URL should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := ConvertGraphResponseToMeetingMetadata(tt.response, tt.joinURL)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.description, err)
				return
			}

			if metadata == nil {
				t.Errorf("Expected metadata for %s, got nil", tt.description)
				return
			}

			if metadata.MeetingID != tt.expectedID {
				t.Errorf("Expected meeting ID '%s', got '%s'", tt.expectedID, metadata.MeetingID)
			}

			if metadata.JoinURL != tt.joinURL {
				t.Errorf("Expected join URL '%s', got '%s'", tt.joinURL, metadata.JoinURL)
			}

			// Verify the converted metadata is valid
			if err := metadata.ValidateMeetingMetadata(); err != nil {
				t.Errorf("Converted metadata validation failed: %v", err)
			}
		})
	}
}

// TestMeetingIntegrationWithChangeLifecycle tests meeting integration throughout change lifecycle
func TestMeetingIntegrationWithChangeLifecycle(t *testing.T) {
	changeMetadata := &ChangeMetadata{
		ChangeID:    "TEST-LIFECYCLE-001",
		ChangeTitle: "Lifecycle Integration Test",
		Status:      "draft",
	}

	// Phase 1: Create change
	createdEntry, err := NewModificationEntry(ModificationTypeCreated, "906638888d-1234-5678-9abc-123456789012")
	if err != nil {
		t.Fatalf("Failed to create modification entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(createdEntry)
	if err != nil {
		t.Fatalf("Failed to add created entry: %v", err)
	}

	// Phase 2: Submit change
	time.Sleep(1 * time.Millisecond)
	submittedEntry, err := NewModificationEntry(ModificationTypeSubmitted, "906638888d-1234-5678-9abc-123456789012")
	if err != nil {
		t.Fatalf("Failed to create submitted entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(submittedEntry)
	if err != nil {
		t.Fatalf("Failed to add submitted entry: %v", err)
	}

	changeMetadata.Status = "submitted"

	// Phase 3: Schedule meeting for change review
	time.Sleep(1 * time.Millisecond)
	reviewMeetingMetadata := &MeetingMetadata{
		MeetingID: "review-meeting-lifecycle-123",
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/review-lifecycle",
		StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
		Subject:   "Change Review Meeting - " + changeMetadata.ChangeID,
		Organizer: "change-manager@example.com",
		Attendees: []string{"reviewer1@example.com", "reviewer2@example.com"},
	}

	meetingEntry, err := NewMeetingScheduledEntry(BackendUserID, reviewMeetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create meeting entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(meetingEntry)
	if err != nil {
		t.Fatalf("Failed to add meeting entry: %v", err)
	}

	// Phase 4: Approve change
	time.Sleep(1 * time.Millisecond)
	approvedEntry, err := NewModificationEntry(ModificationTypeApproved, "906638888d-9abc-1234-5678-901234567890")
	if err != nil {
		t.Fatalf("Failed to create approved entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(approvedEntry)
	if err != nil {
		t.Fatalf("Failed to add approved entry: %v", err)
	}

	changeMetadata.Status = "approved"

	// Phase 5: Schedule implementation meeting
	time.Sleep(1 * time.Millisecond)
	implMeetingMetadata := &MeetingMetadata{
		MeetingID: "implementation-meeting-lifecycle-456",
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/impl-lifecycle",
		StartTime: time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(49 * time.Hour).Format(time.RFC3339),
		Subject:   "Implementation Meeting - " + changeMetadata.ChangeID,
		Organizer: "implementation-manager@example.com",
		Attendees: []string{"dev1@example.com", "dev2@example.com", "ops@example.com"},
	}

	implMeetingEntry, err := NewMeetingScheduledEntry(BackendUserID, implMeetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create implementation meeting entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(implMeetingEntry)
	if err != nil {
		t.Fatalf("Failed to add implementation meeting entry: %v", err)
	}

	// Verify complete lifecycle
	if len(changeMetadata.Modifications) != 5 {
		t.Errorf("Expected 5 modification entries in complete lifecycle, got %d", len(changeMetadata.Modifications))
	}

	// Verify meeting integration
	if !changeMetadata.HasMeetingScheduled() {
		t.Error("Expected meetings to be scheduled in lifecycle")
	}

	latestMeeting := changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Fatal("Expected to find latest meeting metadata")
	}

	if latestMeeting.MeetingID != "implementation-meeting-lifecycle-456" {
		t.Errorf("Expected latest meeting to be implementation meeting, got '%s'", latestMeeting.MeetingID)
	}

	// Verify approval entries
	approvals := changeMetadata.GetApprovalEntries()
	if len(approvals) != 1 {
		t.Errorf("Expected 1 approval entry, got %d", len(approvals))
	}

	// Count meeting entries
	meetingCount := 0
	for _, entry := range changeMetadata.Modifications {
		if entry.ModificationType == ModificationTypeMeetingScheduled {
			meetingCount++
		}
	}

	if meetingCount != 2 {
		t.Errorf("Expected 2 meeting entries (review + implementation), got %d", meetingCount)
	}

	// Validate entire change metadata
	err = changeMetadata.ValidateChangeMetadata()
	if err != nil {
		t.Errorf("Change metadata validation failed after complete lifecycle: %v", err)
	}
}
