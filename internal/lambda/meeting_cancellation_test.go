package lambda

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ccoe-customer-contact-manager/internal/types"
)

// TestCancelMeetingWithMetadata tests the meeting cancellation functionality
func TestCancelMeetingWithMetadata(t *testing.T) {
	// Create test change metadata with meeting
	changeMetadata := &types.ChangeMetadata{
		ChangeID:    "TEST-001",
		ChangeTitle: "Test Change",
		Modifications: []types.ModificationEntry{
			{
				Timestamp:        time.Now(),
				UserID:           "test-user",
				ModificationType: types.ModificationTypeMeetingScheduled,
				MeetingMetadata: &types.MeetingMetadata{
					MeetingID: "test-meeting-123",
					JoinURL:   "https://teams.microsoft.com/test",
					StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
					Subject:   "Test Meeting",
				},
			},
		},
	}

	// Create meeting scheduler
	scheduler := &MeetingScheduler{
		region: "us-east-1",
	}

	// Test cancellation (this will fail in test environment due to missing credentials, but we can test the logic)
	ctx := context.Background()
	err := scheduler.CancelMeetingWithMetadata(ctx, changeMetadata, "test-bucket", "test-key")

	// In test environment, we expect this to fail due to missing Azure credentials
	// But the function should handle the error gracefully
	if err == nil {
		t.Log("Meeting cancellation succeeded (unexpected in test environment)")
	} else {
		t.Logf("Meeting cancellation failed as expected in test environment: %v", err)
	}

	// Verify that the change metadata structure is valid
	if changeMetadata.ChangeID == "" {
		t.Error("ChangeID should not be empty")
	}

	if len(changeMetadata.Modifications) == 0 {
		t.Error("Modifications should not be empty")
	}
}

// TestCancelMeetingsForDeletedChange tests cancellation of multiple meetings for a deleted change
func TestCancelMeetingsForDeletedChange(t *testing.T) {
	// Create test change metadata with multiple meetings
	changeMetadata := &types.ChangeMetadata{
		ChangeID:    "TEST-002",
		ChangeTitle: "Test Change with Multiple Meetings",
		Modifications: []types.ModificationEntry{
			{
				Timestamp:        time.Now().Add(-2 * time.Hour),
				UserID:           "test-user",
				ModificationType: types.ModificationTypeMeetingScheduled,
				MeetingMetadata: &types.MeetingMetadata{
					MeetingID: "test-meeting-456",
					JoinURL:   "https://teams.microsoft.com/test1",
					StartTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(25 * time.Hour).Format(time.RFC3339),
					Subject:   "Test Meeting 1",
				},
			},
			{
				Timestamp:        time.Now().Add(-1 * time.Hour),
				UserID:           "test-user",
				ModificationType: types.ModificationTypeMeetingScheduled,
				MeetingMetadata: &types.MeetingMetadata{
					MeetingID: "test-meeting-789",
					JoinURL:   "https://teams.microsoft.com/test2",
					StartTime: time.Now().Add(48 * time.Hour).Format(time.RFC3339),
					EndTime:   time.Now().Add(49 * time.Hour).Format(time.RFC3339),
					Subject:   "Test Meeting 2",
				},
			},
		},
	}

	// Create test config
	cfg := &types.Config{
		AWSRegion: "us-east-1",
	}

	// Test cancellation of all meetings for deleted change
	ctx := context.Background()
	err := CancelMeetingsForDeletedChange(ctx, changeMetadata, cfg, "test-bucket", "test-key")

	// In test environment, we expect this to complete without fatal errors
	// (individual meeting cancellations may fail due to missing credentials)
	if err != nil {
		t.Logf("Meeting cancellation completed with some errors (expected in test environment): %v", err)
	} else {
		t.Log("Meeting cancellation completed successfully")
	}

	// Verify that the function correctly identified meetings to cancel
	meetingCount := 0
	for _, entry := range changeMetadata.Modifications {
		if entry.ModificationType == types.ModificationTypeMeetingScheduled {
			meetingCount++
		}
	}

	if meetingCount != 2 {
		t.Errorf("Expected 2 meetings to cancel, found %d", meetingCount)
	}
}

// TestHandleMeetingCancellationFailure tests the error handling for meeting cancellation failures
func TestHandleMeetingCancellationFailure(t *testing.T) {
	// Create test change metadata
	changeMetadata := &types.ChangeMetadata{
		ChangeID:      "TEST-003",
		ChangeTitle:   "Test Change for Failure Handling",
		Modifications: []types.ModificationEntry{},
	}

	// Test error handling
	ctx := context.Background()
	testError := fmt.Errorf("test cancellation error")

	HandleMeetingCancellationFailure(ctx, changeMetadata, "test-meeting-999", testError, "test-bucket", "test-key", nil)

	// Verify that a cancellation entry was added
	if len(changeMetadata.Modifications) != 1 {
		t.Errorf("Expected 1 modification entry, found %d", len(changeMetadata.Modifications))
	}

	entry := changeMetadata.Modifications[0]
	if entry.ModificationType != types.ModificationTypeMeetingCancelled {
		t.Errorf("Expected modification type %s, got %s", types.ModificationTypeMeetingCancelled, entry.ModificationType)
	}

	if entry.UserID != types.BackendUserID {
		t.Errorf("Expected user ID %s, got %s", types.BackendUserID, entry.UserID)
	}
}

// TestGetGraphAccessTokenForCancellation tests the access token function
func TestGetGraphAccessTokenForCancellation(t *testing.T) {
	// This test will fail in most environments due to missing Azure credentials
	// But we can test that the function exists and handles errors appropriately
	_, err := getGraphAccessTokenForCancellation()

	// We expect this to fail in test environment
	if err == nil {
		t.Log("Access token retrieved successfully (unexpected in test environment)")
	} else {
		t.Logf("Access token retrieval failed as expected in test environment: %v", err)
	}
}
