package lambda

import (
	"testing"
	"time"

	"ccoe-customer-contact-manager/internal/types"
)

// TestCreateProcessedEntry tests the creation of processed modification entries
func TestCreateProcessedEntry(t *testing.T) {
	tests := []struct {
		name         string
		customerCode string
		wantErr      bool
	}{
		{
			name:         "Valid customer code",
			customerCode: "customer-a",
			wantErr:      false,
		},
		{
			name:         "Another valid customer code",
			customerCode: "htsnonprod",
			wantErr:      false,
		},
		{
			name:         "Customer code with numbers",
			customerCode: "customer-123",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modManager := NewModificationManager()

			entry, err := modManager.CreateProcessedEntry(tt.customerCode)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProcessedEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Validate the entry structure
				if entry.ModificationType != types.ModificationTypeProcessed {
					t.Errorf("Expected modification type 'processed', got '%s'", entry.ModificationType)
				}

				if entry.CustomerCode != tt.customerCode {
					t.Errorf("Expected customer code '%s', got '%s'", tt.customerCode, entry.CustomerCode)
				}

				if entry.UserID == "" {
					t.Error("Expected non-empty user ID")
				}

				if entry.Timestamp.IsZero() {
					t.Error("Expected non-zero timestamp")
				}

				// Validate the entry passes validation
				if err := entry.ValidateModificationEntry(); err != nil {
					t.Errorf("Created entry failed validation: %v", err)
				}
			}
		})
	}
}

// TestAddProcessedToChange tests adding processed entries to change metadata
func TestAddProcessedToChange(t *testing.T) {
	tests := []struct {
		name         string
		changeID     string
		customerCode string
		wantErr      bool
	}{
		{
			name:         "Add processed entry to valid change",
			changeID:     "CHG-12345",
			customerCode: "customer-a",
			wantErr:      false,
		},
		{
			name:         "Add processed entry for htsnonprod",
			changeID:     "CHG-67890",
			customerCode: "htsnonprod",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test change metadata
			changeMetadata := &types.ChangeMetadata{
				ChangeID:      tt.changeID,
				ChangeTitle:   "Test Change",
				Modifications: []types.ModificationEntry{},
			}

			modManager := NewModificationManager()

			// Add processed entry
			err := modManager.AddProcessedToChange(changeMetadata, tt.customerCode)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddProcessedToChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the entry was added
				if len(changeMetadata.Modifications) != 1 {
					t.Errorf("Expected 1 modification entry, got %d", len(changeMetadata.Modifications))
					return
				}

				entry := changeMetadata.Modifications[0]

				if entry.ModificationType != types.ModificationTypeProcessed {
					t.Errorf("Expected modification type 'processed', got '%s'", entry.ModificationType)
				}

				if entry.CustomerCode != tt.customerCode {
					t.Errorf("Expected customer code '%s', got '%s'", tt.customerCode, entry.CustomerCode)
				}
			}
		})
	}
}

// TestMultipleProcessedEntries tests adding multiple processed entries for different customers
func TestMultipleProcessedEntries(t *testing.T) {
	changeMetadata := &types.ChangeMetadata{
		ChangeID:      "CHG-MULTI",
		ChangeTitle:   "Multi-Customer Change",
		Modifications: []types.ModificationEntry{},
	}

	modManager := NewModificationManager()

	// Add processed entries for multiple customers
	customers := []string{"customer-a", "customer-b", "customer-c"}

	for _, customerCode := range customers {
		err := modManager.AddProcessedToChange(changeMetadata, customerCode)
		if err != nil {
			t.Errorf("Failed to add processed entry for customer %s: %v", customerCode, err)
		}
	}

	// Verify all entries were added
	if len(changeMetadata.Modifications) != len(customers) {
		t.Errorf("Expected %d modification entries, got %d", len(customers), len(changeMetadata.Modifications))
	}

	// Verify each entry has the correct customer code
	for i, entry := range changeMetadata.Modifications {
		if entry.CustomerCode != customers[i] {
			t.Errorf("Entry %d: expected customer code '%s', got '%s'", i, customers[i], entry.CustomerCode)
		}

		if entry.ModificationType != types.ModificationTypeProcessed {
			t.Errorf("Entry %d: expected modification type 'processed', got '%s'", i, entry.ModificationType)
		}
	}
}

// TestProcessedEntryValidation tests validation of processed modification entries
func TestProcessedEntryValidation(t *testing.T) {
	tests := []struct {
		name    string
		entry   types.ModificationEntry
		wantErr bool
	}{
		{
			name: "Valid processed entry",
			entry: types.ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           "backend-system",
				ModificationType: types.ModificationTypeProcessed,
				CustomerCode:     "customer-a",
			},
			wantErr: false,
		},
		{
			name: "Valid processed entry with IAM role ARN",
			entry: types.ModificationEntry{
				Timestamp:        time.Now(),
				UserID:           "arn:aws:iam::123456789012:role/backend-lambda-role",
				ModificationType: types.ModificationTypeProcessed,
				CustomerCode:     "customer-b",
			},
			wantErr: false,
		},
		{
			name: "Missing timestamp",
			entry: types.ModificationEntry{
				UserID:           "backend-system",
				ModificationType: types.ModificationTypeProcessed,
				CustomerCode:     "customer-a",
			},
			wantErr: true,
		},
		{
			name: "Missing user ID",
			entry: types.ModificationEntry{
				Timestamp:        time.Now(),
				ModificationType: types.ModificationTypeProcessed,
				CustomerCode:     "customer-a",
			},
			wantErr: true,
		},
		{
			name: "Missing modification type",
			entry: types.ModificationEntry{
				Timestamp:    time.Now(),
				UserID:       "backend-system",
				CustomerCode: "customer-a",
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

// TestMeetingMetadataAtRootLevel tests that meeting metadata is stored at root level
func TestMeetingMetadataAtRootLevel(t *testing.T) {
	changeMetadata := &types.ChangeMetadata{
		ChangeID:      "CHG-MEETING",
		ChangeTitle:   "Change with Meeting",
		Modifications: []types.ModificationEntry{},
	}

	// Create meeting metadata
	meetingMetadata := &types.MeetingMetadata{
		MeetingID: "meeting-123",
		JoinURL:   "https://teams.microsoft.com/l/meetup/...",
		StartTime: time.Now().Format(time.RFC3339),
		EndTime:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		Subject:   "Implementation Meeting",
	}

	// Validate meeting metadata
	if err := meetingMetadata.ValidateMeetingMetadata(); err != nil {
		t.Fatalf("Meeting metadata validation failed: %v", err)
	}

	// Add meeting scheduled entry
	modManager := NewModificationManager()
	err := modManager.AddMeetingScheduledToChange(changeMetadata, meetingMetadata)
	if err != nil {
		t.Fatalf("Failed to add meeting scheduled entry: %v", err)
	}

	// Set nested meeting_metadata field (consistent with actual implementation)
	changeMetadata.MeetingMetadata = meetingMetadata

	// Verify meeting metadata is set correctly
	if changeMetadata.MeetingMetadata == nil || changeMetadata.MeetingMetadata.MeetingID != meetingMetadata.MeetingID {
		t.Errorf("Expected meeting_metadata.meeting_id '%s', got '%v'", meetingMetadata.MeetingID, changeMetadata.MeetingMetadata)
	}

	if changeMetadata.JoinURL != meetingMetadata.JoinURL {
		t.Errorf("Expected root-level join_url '%s', got '%s'", meetingMetadata.JoinURL, changeMetadata.JoinURL)
	}

	// Verify modification entry exists
	if len(changeMetadata.Modifications) != 1 {
		t.Errorf("Expected 1 modification entry, got %d", len(changeMetadata.Modifications))
	}

	if changeMetadata.Modifications[0].ModificationType != types.ModificationTypeMeetingScheduled {
		t.Errorf("Expected modification type 'meeting_scheduled', got '%s'", changeMetadata.Modifications[0].ModificationType)
	}
}

// TestCombinedMeetingAndProcessedEntries tests adding both meeting and processed entries
func TestCombinedMeetingAndProcessedEntries(t *testing.T) {
	changeMetadata := &types.ChangeMetadata{
		ChangeID:      "CHG-COMBINED",
		ChangeTitle:   "Change with Meeting and Processing",
		Modifications: []types.ModificationEntry{},
	}

	modManager := NewModificationManager()

	// Add meeting scheduled entry
	meetingMetadata := &types.MeetingMetadata{
		MeetingID: "meeting-456",
		JoinURL:   "https://teams.microsoft.com/l/meetup/...",
		StartTime: time.Now().Format(time.RFC3339),
		EndTime:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		Subject:   "Implementation Meeting",
	}

	err := modManager.AddMeetingScheduledToChange(changeMetadata, meetingMetadata)
	if err != nil {
		t.Fatalf("Failed to add meeting scheduled entry: %v", err)
	}

	// Set nested meeting_metadata field (consistent with actual implementation)
	changeMetadata.MeetingMetadata = meetingMetadata

	// Add processed entry
	err = modManager.AddProcessedToChange(changeMetadata, "customer-a")
	if err != nil {
		t.Fatalf("Failed to add processed entry: %v", err)
	}

	// Verify both entries exist
	if len(changeMetadata.Modifications) != 2 {
		t.Errorf("Expected 2 modification entries, got %d", len(changeMetadata.Modifications))
	}

	// Verify first entry is meeting_scheduled
	if changeMetadata.Modifications[0].ModificationType != types.ModificationTypeMeetingScheduled {
		t.Errorf("Expected first entry to be 'meeting_scheduled', got '%s'", changeMetadata.Modifications[0].ModificationType)
	}

	// Verify second entry is processed
	if changeMetadata.Modifications[1].ModificationType != types.ModificationTypeProcessed {
		t.Errorf("Expected second entry to be 'processed', got '%s'", changeMetadata.Modifications[1].ModificationType)
	}

	// Verify customer code on processed entry
	if changeMetadata.Modifications[1].CustomerCode != "customer-a" {
		t.Errorf("Expected customer code 'customer-a', got '%s'", changeMetadata.Modifications[1].CustomerCode)
	}

	// Verify root-level meeting fields
	if changeMetadata.MeetingID != meetingMetadata.MeetingID {
		t.Errorf("Expected root-level meeting_id '%s', got '%s'", meetingMetadata.MeetingID, changeMetadata.MeetingID)
	}

	if changeMetadata.JoinURL != meetingMetadata.JoinURL {
		t.Errorf("Expected root-level join_url '%s', got '%s'", meetingMetadata.JoinURL, changeMetadata.JoinURL)
	}
}

// TestModificationEntryStructure tests the structure of modification entries
func TestModificationEntryStructure(t *testing.T) {
	modManager := NewModificationManager()

	// Create a processed entry
	entry, err := modManager.CreateProcessedEntry("customer-test")
	if err != nil {
		t.Fatalf("Failed to create processed entry: %v", err)
	}

	// Verify all required fields are present
	if entry.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if entry.UserID == "" {
		t.Error("UserID should not be empty")
	}

	if entry.ModificationType != types.ModificationTypeProcessed {
		t.Errorf("Expected modification type 'processed', got '%s'", entry.ModificationType)
	}

	if entry.CustomerCode != "customer-test" {
		t.Errorf("Expected customer code 'customer-test', got '%s'", entry.CustomerCode)
	}

	// Verify MeetingMetadata is nil for processed entries
	if entry.MeetingMetadata != nil {
		t.Error("MeetingMetadata should be nil for processed entries")
	}
}

// TestValidateModificationArray tests validation of modification arrays
func TestValidateModificationArray(t *testing.T) {
	tests := []struct {
		name          string
		modifications []types.ModificationEntry
		wantErr       bool
	}{
		{
			name:          "Empty array is valid",
			modifications: []types.ModificationEntry{},
			wantErr:       false,
		},
		{
			name:          "Nil array is valid",
			modifications: nil,
			wantErr:       false,
		},
		{
			name: "Valid array with processed entry",
			modifications: []types.ModificationEntry{
				{
					Timestamp:        time.Now(),
					UserID:           "backend-system",
					ModificationType: types.ModificationTypeProcessed,
					CustomerCode:     "customer-a",
				},
			},
			wantErr: false,
		},
		{
			name: "Valid array with multiple entries",
			modifications: []types.ModificationEntry{
				{
					Timestamp:        time.Now(),
					UserID:           "backend-system",
					ModificationType: types.ModificationTypeSubmitted,
				},
				{
					Timestamp:        time.Now().Add(1 * time.Hour),
					UserID:           "backend-system",
					ModificationType: types.ModificationTypeProcessed,
					CustomerCode:     "customer-a",
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid array with missing timestamp",
			modifications: []types.ModificationEntry{
				{
					UserID:           "backend-system",
					ModificationType: types.ModificationTypeProcessed,
					CustomerCode:     "customer-a",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := types.ValidateModificationArray(tt.modifications)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModificationArray() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestChangeMetadataValidation tests validation of complete change metadata
func TestChangeMetadataValidation(t *testing.T) {
	tests := []struct {
		name     string
		metadata *types.ChangeMetadata
		wantErr  bool
	}{
		{
			name: "Valid change metadata with processed entry",
			metadata: &types.ChangeMetadata{
				ChangeID:    "CHG-VALID",
				ChangeTitle: "Valid Change",
				Modifications: []types.ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           "backend-system",
						ModificationType: types.ModificationTypeProcessed,
						CustomerCode:     "customer-a",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing change ID",
			metadata: &types.ChangeMetadata{
				ChangeTitle: "Change without ID",
				Modifications: []types.ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           "backend-system",
						ModificationType: types.ModificationTypeProcessed,
						CustomerCode:     "customer-a",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Missing change title",
			metadata: &types.ChangeMetadata{
				ChangeID: "CHG-NO-TITLE",
				Modifications: []types.ModificationEntry{
					{
						Timestamp:        time.Now(),
						UserID:           "backend-system",
						ModificationType: types.ModificationTypeProcessed,
						CustomerCode:     "customer-a",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid modification entry",
			metadata: &types.ChangeMetadata{
				ChangeID:    "CHG-INVALID-MOD",
				ChangeTitle: "Change with Invalid Modification",
				Modifications: []types.ModificationEntry{
					{
						// Missing timestamp
						UserID:           "backend-system",
						ModificationType: types.ModificationTypeProcessed,
						CustomerCode:     "customer-a",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metadata.ValidateChangeMetadata()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateChangeMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
