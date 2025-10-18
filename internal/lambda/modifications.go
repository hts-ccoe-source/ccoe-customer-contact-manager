package lambda

import (
	"fmt"
	"log"

	"ccoe-customer-contact-manager/internal/types"
)

// ModificationManager handles creation and management of modification entries in the backend
type ModificationManager struct {
	// BackendUserID is the user ID used for backend-generated modifications
	BackendUserID string
}

// NewModificationManager creates a new ModificationManager with backend role ARN from environment
func NewModificationManager() *ModificationManager {
	// Try to get the backend role ARN from environment
	backendUserID := getBackendRoleARNFromEnv()
	if backendUserID == "" {
		// Fallback to legacy constant if no role ARN is available
		log.Printf("⚠️  No backend role ARN found in environment, using legacy backend-system ID")
		backendUserID = types.BackendUserID
	} else {
		log.Printf("✅ Using backend role ARN as user ID: %s", backendUserID)
	}

	return &ModificationManager{
		BackendUserID: backendUserID,
	}
}

// NewModificationManagerWithUserID creates a new ModificationManager with custom backend user ID
func NewModificationManagerWithUserID(userID string) *ModificationManager {
	return &ModificationManager{
		BackendUserID: userID,
	}
}

// CreateMeetingScheduledEntry creates a modification entry for meeting scheduling
func (m *ModificationManager) CreateMeetingScheduledEntry(meetingMetadata *types.MeetingMetadata) (types.ModificationEntry, error) {
	log.Printf("📝 Creating meeting_scheduled modification entry with meeting ID: %s", meetingMetadata.MeetingID)

	entry, err := types.NewMeetingScheduledEntry(m.BackendUserID, meetingMetadata)
	if err != nil {
		return types.ModificationEntry{}, fmt.Errorf("failed to create meeting scheduled entry: %w", err)
	}

	log.Printf("✅ Created meeting_scheduled entry: %+v", entry)
	return entry, nil
}

// CreateMeetingCancelledEntry creates a modification entry for meeting cancellation
func (m *ModificationManager) CreateMeetingCancelledEntry() (types.ModificationEntry, error) {
	log.Printf("📝 Creating meeting_cancelled modification entry")

	entry, err := types.NewMeetingCancelledEntry(m.BackendUserID)
	if err != nil {
		return types.ModificationEntry{}, fmt.Errorf("failed to create meeting cancelled entry: %w", err)
	}

	log.Printf("✅ Created meeting_cancelled entry: %+v", entry)
	return entry, nil
}

// CreateProcessedEntry creates a modification entry for successful email delivery processing
func (m *ModificationManager) CreateProcessedEntry(customerCode string) (types.ModificationEntry, error) {
	log.Printf("📝 Creating processed modification entry for customer: %s", customerCode)

	entry, err := types.NewModificationEntry(types.ModificationTypeProcessed, m.BackendUserID)
	if err != nil {
		return types.ModificationEntry{}, fmt.Errorf("failed to create processed entry: %w", err)
	}

	// Add customer code to track which customer processed this change
	entry.CustomerCode = customerCode

	log.Printf("✅ Created processed entry for customer %s: %+v", customerCode, entry)
	return entry, nil
}

// AddMeetingScheduledToChange adds a meeting_scheduled modification entry to change metadata
func (m *ModificationManager) AddMeetingScheduledToChange(changeMetadata *types.ChangeMetadata, meetingMetadata *types.MeetingMetadata) error {
	entry, err := m.CreateMeetingScheduledEntry(meetingMetadata)
	if err != nil {
		return fmt.Errorf("failed to create meeting_scheduled entry: %w", err)
	}

	if err := changeMetadata.AddModificationEntry(entry); err != nil {
		return fmt.Errorf("failed to add meeting_scheduled entry: %w", err)
	}

	log.Printf("📋 Added meeting_scheduled entry to change %s", changeMetadata.ChangeID)
	return nil
}

// AddMeetingCancelledToChange adds a meeting_cancelled modification entry to change metadata
func (m *ModificationManager) AddMeetingCancelledToChange(changeMetadata *types.ChangeMetadata) error {
	entry, err := m.CreateMeetingCancelledEntry()
	if err != nil {
		return fmt.Errorf("failed to create meeting_cancelled entry: %w", err)
	}

	if err := changeMetadata.AddModificationEntry(entry); err != nil {
		return fmt.Errorf("failed to add meeting_cancelled entry: %w", err)
	}

	log.Printf("📋 Added meeting_cancelled entry to change %s", changeMetadata.ChangeID)
	return nil
}

// AddProcessedToChange adds a processed modification entry to change metadata
func (m *ModificationManager) AddProcessedToChange(changeMetadata *types.ChangeMetadata, customerCode string) error {
	entry, err := m.CreateProcessedEntry(customerCode)
	if err != nil {
		return fmt.Errorf("failed to create processed entry: %w", err)
	}

	if err := changeMetadata.AddModificationEntry(entry); err != nil {
		return fmt.Errorf("failed to add processed entry: %w", err)
	}

	log.Printf("📋 Added processed entry to change %s for customer %s", changeMetadata.ChangeID, customerCode)
	return nil
}

// CreateMeetingMetadataFromGraphResponse creates MeetingMetadata from Microsoft Graph API response
func (m *ModificationManager) CreateMeetingMetadataFromGraphResponse(graphResponse *types.GraphMeetingResponse) *types.MeetingMetadata {
	log.Printf("🔄 Converting Graph API response to MeetingMetadata")

	metadata := &types.MeetingMetadata{
		MeetingID: graphResponse.ID,
		Subject:   graphResponse.Subject,
	}

	// Extract start and end times if available
	if graphResponse.Start != nil {
		metadata.StartTime = graphResponse.Start.DateTime
	}

	if graphResponse.End != nil {
		metadata.EndTime = graphResponse.End.DateTime
	}

	// Extract join URL from online meeting info
	if graphResponse.OnlineMeeting != nil && graphResponse.OnlineMeeting.JoinURL != "" {
		metadata.JoinURL = graphResponse.OnlineMeeting.JoinURL
		log.Printf("📎 Extracted join URL from Graph response")
	} else {
		log.Printf("⚠️  No join URL found in Graph response")
	}

	log.Printf("✅ Created MeetingMetadata: ID=%s, Subject=%s, JoinURL=%s", metadata.MeetingID, metadata.Subject, metadata.JoinURL)
	return metadata
}

// CreateMeetingMetadata creates MeetingMetadata with provided details
func (m *ModificationManager) CreateMeetingMetadata(meetingID, joinURL, startTime, endTime, subject string) *types.MeetingMetadata {
	return &types.MeetingMetadata{
		MeetingID: meetingID,
		JoinURL:   joinURL,
		StartTime: startTime,
		EndTime:   endTime,
		Subject:   subject,
	}
}

// ValidateModificationEntry validates that a modification entry has required fields
func (m *ModificationManager) ValidateModificationEntry(entry types.ModificationEntry) error {
	if entry.Timestamp.IsZero() {
		return NewValidationError("modification entry timestamp cannot be zero")
	}

	if entry.UserID == "" {
		return NewValidationError("modification entry user_id cannot be empty")
	}

	if entry.ModificationType == "" {
		return NewValidationError("modification entry modification_type cannot be empty")
	}

	// Validate meeting metadata if present
	if entry.ModificationType == types.ModificationTypeMeetingScheduled {
		if entry.MeetingMetadata == nil {
			return NewValidationError("meeting_scheduled entry must have meeting_metadata")
		}

		if entry.MeetingMetadata.MeetingID == "" {
			return NewValidationError("meeting metadata must have meeting_id")
		}
	}

	return nil
}

// ValidationError represents a validation error for modification entries
type ValidationError struct {
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new ValidationError
func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}
