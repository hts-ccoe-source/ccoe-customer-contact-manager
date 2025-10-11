package types

import (
	"testing"
	"time"
)

func TestValidateModificationEntry(t *testing.T) {
	tests := []struct {
		name    string
		entry   ModificationEntry
		wantErr bool
	}{
		{
			name: "valid created entry",
			entry: ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           "906638888d-1234-5678-9abc-123456789012",
				ModificationType: ModificationTypeCreated,
			},
			wantErr: false,
		},
		{
			name: "valid backend system entry",
			entry: ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           BackendUserID,
				ModificationType: ModificationTypeMeetingCancelled,
			},
			wantErr: false,
		},
		{
			name: "valid meeting scheduled entry",
			entry: ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           BackendUserID,
				ModificationType: ModificationTypeMeetingScheduled,
				MeetingMetadata: &MeetingMetadata{
					MeetingID: "test-meeting-123",
					JoinURL:   "https://teams.microsoft.com/l/meetup-join/test",
					StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
					Subject:   "Test Meeting",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - empty timestamp",
			entry: ModificationEntry{
				UserID:           "906638888d-1234-5678-9abc-123456789012",
				ModificationType: ModificationTypeCreated,
			},
			wantErr: true,
		},
		{
			name: "invalid - empty user_id",
			entry: ModificationEntry{
				Timestamp:        time.Now(),
				ModificationType: ModificationTypeCreated,
			},
			wantErr: true,
		},
		{
			name: "invalid - empty modification_type",
			entry: ModificationEntry{
				Timestamp: time.Now(),
				UserID:    "906638888d-1234-5678-9abc-123456789012",
			},
			wantErr: true,
		},
		{
			name: "invalid - invalid modification_type",
			entry: ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           "906638888d-1234-5678-9abc-123456789012",
				ModificationType: "invalid_type",
			},
			wantErr: true,
		},
		{
			name: "invalid - meeting_scheduled without metadata",
			entry: ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           BackendUserID,
				ModificationType: ModificationTypeMeetingScheduled,
			},
			wantErr: true,
		},
		{
			name: "invalid - meeting metadata on non-meeting type",
			entry: ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           BackendUserID,
				ModificationType: ModificationTypeCreated,
				MeetingMetadata: &MeetingMetadata{
					MeetingID: "test-meeting-123",
					JoinURL:   "https://teams.microsoft.com/l/meetup-join/test",
					StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
					Subject:   "Test Meeting",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.ValidateModificationEntry()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModificationEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUserIDFormat(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{
			name:    "valid Identity Center user ID",
			userID:  "906638888d-1234-5678-9abc-123456789012",
			wantErr: false,
		},
		{
			name:    "valid backend system user ID",
			userID:  BackendUserID,
			wantErr: false,
		},
		{
			name:    "valid short user ID",
			userID:  "user123456",
			wantErr: false,
		},
		{
			name:    "invalid - empty user ID",
			userID:  "",
			wantErr: true,
		},
		{
			name:    "invalid - whitespace only",
			userID:  "   ",
			wantErr: true,
		},
		{
			name:    "invalid - too short",
			userID:  "user123",
			wantErr: true,
		},
		{
			name:    "invalid - special characters",
			userID:  "user@domain.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserIDFormat(tt.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUserIDFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddModificationEntryWithValidation(t *testing.T) {
	changeMetadata := &ChangeMetadata{
		ChangeID:    "TEST-001",
		ChangeTitle: "Test Change",
	}

	// Test adding valid entry
	validEntry := ModificationEntry{
		Timestamp:        time.Now(),
		UserID:           BackendUserID,
		ModificationType: ModificationTypeCreated,
	}

	err := changeMetadata.AddModificationEntry(validEntry)
	if err != nil {
		t.Errorf("AddModificationEntry() with valid entry failed: %v", err)
	}

	if len(changeMetadata.Modifications) != 1 {
		t.Errorf("Expected 1 modification entry, got %d", len(changeMetadata.Modifications))
	}

	// Test adding invalid entry
	invalidEntry := ModificationEntry{
		Timestamp:        time.Now(),
		UserID:           "", // Invalid - empty user ID
		ModificationType: ModificationTypeCreated,
	}

	err = changeMetadata.AddModificationEntry(invalidEntry)
	if err == nil {
		t.Error("AddModificationEntry() with invalid entry should have failed")
	}

	// Should still have only 1 entry (the valid one)
	if len(changeMetadata.Modifications) != 1 {
		t.Errorf("Expected 1 modification entry after failed add, got %d", len(changeMetadata.Modifications))
	}
}

func TestNewModificationEntryWithValidation(t *testing.T) {
	// Test valid entry creation
	entry, err := NewModificationEntry(ModificationTypeCreated, BackendUserID)
	if err != nil {
		t.Errorf("NewModificationEntry() with valid parameters failed: %v", err)
	}

	if entry.ModificationType != ModificationTypeCreated {
		t.Errorf("Expected modification type %s, got %s", ModificationTypeCreated, entry.ModificationType)
	}

	// Test invalid entry creation
	_, err = NewModificationEntry("invalid_type", BackendUserID)
	if err == nil {
		t.Error("NewModificationEntry() with invalid type should have failed")
	}

	_, err = NewModificationEntry(ModificationTypeCreated, "")
	if err == nil {
		t.Error("NewModificationEntry() with empty user ID should have failed")
	}
}

// TestEndToEndModificationTracking tests the complete modification tracking workflow
func TestEndToEndModificationTracking(t *testing.T) {
	// Initialize change metadata
	changeMetadata := &ChangeMetadata{
		ChangeID:    "TEST-E2E-001",
		ChangeTitle: "End-to-End Test Change",
		Status:      "draft",
	}

	// Test 1: Create initial modification entry
	createdEntry, err := NewModificationEntry(ModificationTypeCreated, "906638888d-1234-5678-9abc-123456789012")
	if err != nil {
		t.Fatalf("Failed to create initial modification entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(createdEntry)
	if err != nil {
		t.Fatalf("Failed to add created entry: %v", err)
	}

	if len(changeMetadata.Modifications) != 1 {
		t.Errorf("Expected 1 modification entry after creation, got %d", len(changeMetadata.Modifications))
	}

	// Test 2: Add update modification entry
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	updatedEntry, err := NewModificationEntry(ModificationTypeUpdated, "906638888d-5678-9abc-1234-567890123456")
	if err != nil {
		t.Fatalf("Failed to create updated modification entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(updatedEntry)
	if err != nil {
		t.Fatalf("Failed to add updated entry: %v", err)
	}

	if len(changeMetadata.Modifications) != 2 {
		t.Errorf("Expected 2 modification entries after update, got %d", len(changeMetadata.Modifications))
	}

	// Test 3: Add submission modification entry
	time.Sleep(1 * time.Millisecond)
	submittedEntry, err := NewModificationEntry(ModificationTypeSubmitted, "906638888d-1234-5678-9abc-123456789012")
	if err != nil {
		t.Fatalf("Failed to create submitted modification entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(submittedEntry)
	if err != nil {
		t.Fatalf("Failed to add submitted entry: %v", err)
	}

	changeMetadata.Status = "submitted"

	// Test 4: Add approval modification entry
	time.Sleep(1 * time.Millisecond)
	approvedEntry, err := NewModificationEntry(ModificationTypeApproved, "906638888d-9abc-1234-5678-901234567890")
	if err != nil {
		t.Fatalf("Failed to create approved modification entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(approvedEntry)
	if err != nil {
		t.Fatalf("Failed to add approved entry: %v", err)
	}

	changeMetadata.Status = "approved"

	// Test 5: Add meeting scheduled entry
	time.Sleep(1 * time.Millisecond)
	meetingMetadata := &MeetingMetadata{
		MeetingID: "test-meeting-e2e-123",
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/test-e2e",
		StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
		Subject:   "E2E Test Meeting",
	}

	meetingEntry, err := NewMeetingScheduledEntry(BackendUserID, meetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create meeting scheduled entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(meetingEntry)
	if err != nil {
		t.Fatalf("Failed to add meeting scheduled entry: %v", err)
	}

	// Verify final state
	if len(changeMetadata.Modifications) != 5 {
		t.Errorf("Expected 5 modification entries in complete workflow, got %d", len(changeMetadata.Modifications))
	}

	// Test approval filtering
	approvals := changeMetadata.GetApprovalEntries()
	if len(approvals) != 1 {
		t.Errorf("Expected 1 approval entry, got %d", len(approvals))
	}

	// Test meeting metadata retrieval
	latestMeeting := changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Error("Expected to find meeting metadata")
	} else if latestMeeting.MeetingID != "test-meeting-e2e-123" {
		t.Errorf("Expected meeting ID 'test-meeting-e2e-123', got '%s'", latestMeeting.MeetingID)
	}

	// Test meeting scheduled check
	if !changeMetadata.HasMeetingScheduled() {
		t.Error("Expected HasMeetingScheduled to return true")
	}

	// Validate entire change metadata
	err = changeMetadata.ValidateChangeMetadata()
	if err != nil {
		t.Errorf("Change metadata validation failed: %v", err)
	}
}

// TestMeetingLifecycleIntegration tests meeting scheduling and cancellation
func TestMeetingLifecycleIntegration(t *testing.T) {
	changeMetadata := &ChangeMetadata{
		ChangeID:    "TEST-MEETING-001",
		ChangeTitle: "Meeting Lifecycle Test",
	}

	// Test 1: Schedule meeting
	meetingMetadata := &MeetingMetadata{
		MeetingID: "lifecycle-meeting-123",
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/lifecycle",
		StartTime: time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(49 * time.Hour).Format(time.RFC3339),
		Subject:   "Lifecycle Test Meeting",
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

	latestMeeting := changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Fatal("Expected to find meeting metadata")
	}

	if latestMeeting.MeetingID != "lifecycle-meeting-123" {
		t.Errorf("Expected meeting ID 'lifecycle-meeting-123', got '%s'", latestMeeting.MeetingID)
	}

	// Test 2: Update meeting (schedule new meeting)
	time.Sleep(1 * time.Millisecond)
	updatedMeetingMetadata := &MeetingMetadata{
		MeetingID: "lifecycle-meeting-456",
		JoinURL:   "https://teams.microsoft.com/l/meetup-join/lifecycle-updated",
		StartTime: time.Now().Add(72 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(73 * time.Hour).Format(time.RFC3339),
		Subject:   "Updated Lifecycle Test Meeting",
	}

	updatedScheduledEntry, err := NewMeetingScheduledEntry(BackendUserID, updatedMeetingMetadata)
	if err != nil {
		t.Fatalf("Failed to create updated meeting scheduled entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(updatedScheduledEntry)
	if err != nil {
		t.Fatalf("Failed to add updated meeting scheduled entry: %v", err)
	}

	// Verify latest meeting is the updated one
	latestMeeting = changeMetadata.GetLatestMeetingMetadata()
	if latestMeeting == nil {
		t.Fatal("Expected to find updated meeting metadata")
	}

	if latestMeeting.MeetingID != "lifecycle-meeting-456" {
		t.Errorf("Expected updated meeting ID 'lifecycle-meeting-456', got '%s'", latestMeeting.MeetingID)
	}

	// Test 3: Cancel meeting
	time.Sleep(1 * time.Millisecond)
	cancelledEntry, err := NewMeetingCancelledEntry(BackendUserID)
	if err != nil {
		t.Fatalf("Failed to create meeting cancelled entry: %v", err)
	}

	err = changeMetadata.AddModificationEntry(cancelledEntry)
	if err != nil {
		t.Fatalf("Failed to add meeting cancelled entry: %v", err)
	}

	// Verify we have all expected entries
	if len(changeMetadata.Modifications) != 3 {
		t.Errorf("Expected 3 modification entries (2 scheduled + 1 cancelled), got %d", len(changeMetadata.Modifications))
	}

	// Count meeting-related entries
	meetingEntries := 0
	for _, entry := range changeMetadata.Modifications {
		if entry.ModificationType == ModificationTypeMeetingScheduled ||
			entry.ModificationType == ModificationTypeMeetingCancelled {
			meetingEntries++
		}
	}

	if meetingEntries != 3 {
		t.Errorf("Expected 3 meeting-related entries, got %d", meetingEntries)
	}
}

// TestEventLoopPrevention tests backend event identification logic
func TestEventLoopPrevention(t *testing.T) {
	// Test backend role ARN identification
	backendRoleARN := "arn:aws:iam::123456789012:role/backend-lambda-role"
	frontendRoleARN := "arn:aws:iam::123456789012:role/frontend-lambda-role"

	// Test S3 event with backend user identity (should be discarded)
	backendEvent := &S3EventRecord{
		EventName: "s3:ObjectCreated:Put",
		UserIdentity: &S3UserIdentity{
			Type:        "AssumedRole",
			PrincipalID: "AIDACKCEVSQ6C2EXAMPLE:backend-lambda",
			ARN:         backendRoleARN,
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
			Bucket: struct {
				Name string `json:"name"`
				ARN  string `json:"arn"`
			}{
				Name: "test-bucket",
				ARN:  "arn:aws:s3:::test-bucket",
			},
			Object: struct {
				Key       string `json:"key"`
				Size      int64  `json:"size"`
				ETag      string `json:"eTag"`
				VersionID string `json:"versionId,omitempty"`
			}{
				Key:  "changes/TEST-001.json",
				Size: 1024,
				ETag: "d41d8cd98f00b204e9800998ecf8427e",
			},
		},
	}

	// Test S3 event with frontend user identity (should be processed)
	frontendEvent := &S3EventRecord{
		EventName: "s3:ObjectCreated:Put",
		UserIdentity: &S3UserIdentity{
			Type:        "AssumedRole",
			PrincipalID: "AIDACKCEVSQ6C2EXAMPLE:frontend-lambda",
			ARN:         frontendRoleARN,
		},
		S3: backendEvent.S3, // Same S3 details
	}

	// Test S3 event without user identity (should be processed)
	noIdentityEvent := &S3EventRecord{
		EventName:    "s3:ObjectCreated:Put",
		UserIdentity: nil,
		S3:           backendEvent.S3,
	}

	// Simulate backend event processing logic
	shouldProcessBackendEvent := !isBackendGeneratedEvent(backendEvent, backendRoleARN)
	shouldProcessFrontendEvent := !isBackendGeneratedEvent(frontendEvent, backendRoleARN)
	shouldProcessNoIdentityEvent := !isBackendGeneratedEvent(noIdentityEvent, backendRoleARN)

	if shouldProcessBackendEvent {
		t.Error("Backend-generated event should be discarded, not processed")
	}

	if !shouldProcessFrontendEvent {
		t.Error("Frontend-generated event should be processed, not discarded")
	}

	if !shouldProcessNoIdentityEvent {
		t.Error("Event without user identity should be processed, not discarded")
	}
}

// Helper function to simulate backend event identification logic
func isBackendGeneratedEvent(event *S3EventRecord, backendRoleARN string) bool {
	if event.UserIdentity == nil {
		return false // Process events without user identity
	}

	return event.UserIdentity.ARN == backendRoleARN
}

// TestMeetingMetadataValidation tests comprehensive meeting metadata validation
func TestMeetingMetadataValidation(t *testing.T) {
	tests := []struct {
		name     string
		metadata *MeetingMetadata
		wantErr  bool
	}{
		{
			name: "valid complete metadata",
			metadata: &MeetingMetadata{
				MeetingID: "valid-meeting-123",
				JoinURL:   "https://teams.microsoft.com/l/meetup-join/valid",
				StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
				Subject:   "Valid Test Meeting",
				Organizer: "organizer@example.com",
				Attendees: []string{"attendee1@example.com", "attendee2@example.com"},
			},
			wantErr: false,
		},
		{
			name: "valid minimal metadata",
			metadata: &MeetingMetadata{
				MeetingID: "minimal-meeting-456",
				JoinURL:   "https://teams.microsoft.com/l/meetup-join/minimal",
				StartTime: time.Now().Add(48 * time.Hour).Format(time.RFC3339),
				EndTime:   time.Now().Add(49 * time.Hour).Format(time.RFC3339),
				Subject:   "Minimal Test Meeting",
			},
			wantErr: false,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			wantErr:  true,
		},
		{
			name: "empty meeting ID",
			metadata: &MeetingMetadata{
				MeetingID: "",
				JoinURL:   "https://teams.microsoft.com/l/meetup-join/test",
				StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
				Subject:   "Test Meeting",
			},
			wantErr: true,
		},
		{
			name: "empty join URL",
			metadata: &MeetingMetadata{
				MeetingID: "test-meeting-789",
				JoinURL:   "",
				StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
				Subject:   "Test Meeting",
			},
			wantErr: true,
		},
		{
			name: "invalid start time format",
			metadata: &MeetingMetadata{
				MeetingID: "test-meeting-invalid-start",
				JoinURL:   "https://teams.microsoft.com/l/meetup-join/test",
				StartTime: "invalid-time-format",
				EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
				Subject:   "Test Meeting",
			},
			wantErr: true,
		},
		{
			name: "start time after end time",
			metadata: &MeetingMetadata{
				MeetingID: "test-meeting-time-order",
				JoinURL:   "https://teams.microsoft.com/l/meetup-join/test",
				StartTime: time.Now().Add(25 * time.Hour).Format(time.RFC3339),
				EndTime:   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				Subject:   "Test Meeting",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.metadata != nil {
				err = tt.metadata.ValidateMeetingMetadata()
			} else {
				// Test nil metadata validation
				var nilMetadata *MeetingMetadata
				err = nilMetadata.ValidateMeetingMetadata()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMeetingMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestChangeMetadataValidation tests comprehensive change metadata validation
func TestChangeMetadataValidation(t *testing.T) {
	validModifications := []ModificationEntry{
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

	tests := []struct {
		name     string
		metadata *ChangeMetadata
		wantErr  bool
	}{
		{
			name: "valid complete metadata",
			metadata: &ChangeMetadata{
				ChangeID:      "VALID-001",
				ChangeTitle:   "Valid Test Change",
				Modifications: validModifications,
			},
			wantErr: false,
		},
		{
			name: "valid minimal metadata",
			metadata: &ChangeMetadata{
				ChangeID:    "MINIMAL-001",
				ChangeTitle: "Minimal Test Change",
			},
			wantErr: false,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			wantErr:  true,
		},
		{
			name: "empty change ID",
			metadata: &ChangeMetadata{
				ChangeID:    "",
				ChangeTitle: "Test Change",
			},
			wantErr: true,
		},
		{
			name: "empty change title",
			metadata: &ChangeMetadata{
				ChangeID:    "TEST-001",
				ChangeTitle: "",
			},
			wantErr: true,
		},
		{
			name: "invalid modification entry",
			metadata: &ChangeMetadata{
				ChangeID:    "INVALID-MOD-001",
				ChangeTitle: "Test Change with Invalid Modification",
				Modifications: []ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           "", // Invalid - empty user ID
						ModificationType: ModificationTypeCreated,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.metadata != nil {
				err = tt.metadata.ValidateChangeMetadata()
			} else {
				// Test nil metadata validation
				var nilMetadata *ChangeMetadata
				err = nilMetadata.ValidateChangeMetadata()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateChangeMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
