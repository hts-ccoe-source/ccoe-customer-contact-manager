package lambda

import (
	"context"
	"errors"
	"testing"
)

// TestExecutionSummaryCompleteness verifies that all summary metrics are properly tracked
func TestExecutionSummaryCompleteness(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T, *ExecutionSummary)
	}{
		{
			name: "Message Processing Metrics",
			testFunc: func(t *testing.T, summary *ExecutionSummary) {
				// Test successful message processing
				summary.RecordSuccess()
				if summary.SuccessfulMessages != 1 {
					t.Errorf("Expected SuccessfulMessages=1, got %d", summary.SuccessfulMessages)
				}

				// Test retryable error
				err := errors.New("retryable error")
				summary.RecordRetryableError(err)
				if summary.RetryableErrors != 1 {
					t.Errorf("Expected RetryableErrors=1, got %d", summary.RetryableErrors)
				}
				if len(summary.ErrorMessages) != 1 {
					t.Errorf("Expected 1 error message, got %d", len(summary.ErrorMessages))
				}

				// Test non-retryable error
				err2 := errors.New("non-retryable error")
				summary.RecordNonRetryableError(err2)
				if summary.NonRetryableErrors != 1 {
					t.Errorf("Expected NonRetryableErrors=1, got %d", summary.NonRetryableErrors)
				}
				if len(summary.ErrorMessages) != 2 {
					t.Errorf("Expected 2 error messages, got %d", len(summary.ErrorMessages))
				}

				// Test discarded event
				summary.RecordDiscardedEvent()
				if summary.DiscardedEvents != 1 {
					t.Errorf("Expected DiscardedEvents=1, got %d", summary.DiscardedEvents)
				}
			},
		},
		{
			name: "Customer Processing Metrics",
			testFunc: func(t *testing.T, summary *ExecutionSummary) {
				// Test customer tracking
				summary.RecordCustomer("customer1")
				summary.RecordCustomer("customer2")
				summary.RecordCustomer("customer1") // Duplicate should be ignored

				if len(summary.CustomersProcessed) != 2 {
					t.Errorf("Expected 2 customers, got %d", len(summary.CustomersProcessed))
				}

				// Verify no duplicates
				customerMap := make(map[string]bool)
				for _, code := range summary.CustomersProcessed {
					if customerMap[code] {
						t.Errorf("Duplicate customer code found: %s", code)
					}
					customerMap[code] = true
				}
			},
		},
		{
			name: "Email Statistics",
			testFunc: func(t *testing.T, summary *ExecutionSummary) {
				// Test email sent
				summary.RecordEmailSent(5)
				if summary.EmailsSent != 1 {
					t.Errorf("Expected EmailsSent=1, got %d", summary.EmailsSent)
				}

				// Test email filtering
				summary.RecordEmailFiltering(20, 5, 15)
				if summary.EmailsBeforeFilter != 20 {
					t.Errorf("Expected EmailsBeforeFilter=20, got %d", summary.EmailsBeforeFilter)
				}
				if summary.EmailsFiltered != 15 {
					t.Errorf("Expected EmailsFiltered=15, got %d", summary.EmailsFiltered)
				}

				// Test email error
				summary.RecordEmailError()
				if summary.EmailErrors != 1 {
					t.Errorf("Expected EmailErrors=1, got %d", summary.EmailErrors)
				}
			},
		},
		{
			name: "Meeting Statistics",
			testFunc: func(t *testing.T, summary *ExecutionSummary) {
				// Test meeting scheduled
				summary.RecordMeetingScheduled(10)
				if summary.MeetingsScheduled != 1 {
					t.Errorf("Expected MeetingsScheduled=1, got %d", summary.MeetingsScheduled)
				}
				if summary.TotalAttendees != 10 {
					t.Errorf("Expected TotalAttendees=10, got %d", summary.TotalAttendees)
				}

				// Test meeting cancelled
				summary.RecordMeetingCancelled()
				if summary.MeetingsCancelled != 1 {
					t.Errorf("Expected MeetingsCancelled=1, got %d", summary.MeetingsCancelled)
				}

				// Test meeting updated
				summary.RecordMeetingUpdated()
				if summary.MeetingsUpdated != 1 {
					t.Errorf("Expected MeetingsUpdated=1, got %d", summary.MeetingsUpdated)
				}

				// Test meeting error
				summary.RecordMeetingError()
				if summary.MeetingErrors != 1 {
					t.Errorf("Expected MeetingErrors=1, got %d", summary.MeetingErrors)
				}

				// Test meeting attendee filtering
				summary.RecordMeetingAttendeeFiltering(20, 5, 2, 17)
				if summary.TotalAttendees != 30 { // 10 from scheduled + 20 from filtering
					t.Errorf("Expected TotalAttendees=30, got %d", summary.TotalAttendees)
				}
				if summary.FilteredAttendees != 5 {
					t.Errorf("Expected FilteredAttendees=5, got %d", summary.FilteredAttendees)
				}
				if summary.ManualAttendees != 2 {
					t.Errorf("Expected ManualAttendees=2, got %d", summary.ManualAttendees)
				}
				if summary.FinalAttendeeCount != 22 { // 5 from email sent + 17 from filtering
					t.Errorf("Expected FinalAttendeeCount=22, got %d", summary.FinalAttendeeCount)
				}
			},
		},
		{
			name: "S3 Operations",
			testFunc: func(t *testing.T, summary *ExecutionSummary) {
				// Test S3 download
				summary.RecordS3Download()
				if summary.S3Downloads != 1 {
					t.Errorf("Expected S3Downloads=1, got %d", summary.S3Downloads)
				}

				// Test S3 upload
				summary.RecordS3Upload()
				if summary.S3Uploads != 1 {
					t.Errorf("Expected S3Uploads=1, got %d", summary.S3Uploads)
				}

				// Test S3 delete
				summary.RecordS3Delete()
				if summary.S3Deletes != 1 {
					t.Errorf("Expected S3Deletes=1, got %d", summary.S3Deletes)
				}

				// Test S3 error
				summary.RecordS3Error()
				if summary.S3Errors != 1 {
					t.Errorf("Expected S3Errors=1, got %d", summary.S3Errors)
				}
			},
		},
		{
			name: "Change Request Processing",
			testFunc: func(t *testing.T, summary *ExecutionSummary) {
				// Test approval request
				summary.RecordApprovalRequest()
				if summary.ApprovalRequests != 1 {
					t.Errorf("Expected ApprovalRequests=1, got %d", summary.ApprovalRequests)
				}

				// Test approved change
				summary.RecordApprovedChange()
				if summary.ApprovedChanges != 1 {
					t.Errorf("Expected ApprovedChanges=1, got %d", summary.ApprovedChanges)
				}

				// Test completed change
				summary.RecordCompletedChange()
				if summary.CompletedChanges != 1 {
					t.Errorf("Expected CompletedChanges=1, got %d", summary.CompletedChanges)
				}

				// Test cancelled change
				summary.RecordCancelledChange()
				if summary.CancelledChanges != 1 {
					t.Errorf("Expected CancelledChanges=1, got %d", summary.CancelledChanges)
				}
			},
		},
		{
			name: "Timing Metrics",
			testFunc: func(t *testing.T, summary *ExecutionSummary) {
				// Test duration calculation
				if summary.StartTime.IsZero() {
					t.Error("StartTime should be set")
				}

				// Test duration before finalize
				duration1 := summary.DurationMs()
				if duration1 <= 0 {
					t.Error("Duration should be positive before finalize")
				}

				// Test finalize
				summary.Finalize()
				if summary.EndTime.IsZero() {
					t.Error("EndTime should be set after finalize")
				}

				// Test duration after finalize
				duration2 := summary.DurationMs()
				if duration2 <= 0 {
					t.Error("Duration should be positive after finalize")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := NewExecutionSummary()
			tt.testFunc(t, summary)
		})
	}
}

// TestSummaryContextIntegration verifies context integration works correctly
func TestSummaryContextIntegration(t *testing.T) {
	// Create a new summary
	summary := NewExecutionSummary()

	// Create context with summary
	ctx := context.Background()
	ctx = ContextWithSummary(ctx, summary)

	// Retrieve summary from context
	retrievedSummary := SummaryFromContext(ctx)
	if retrievedSummary == nil {
		t.Fatal("Failed to retrieve summary from context")
	}

	// Verify it's the same summary
	retrievedSummary.RecordSuccess()
	if summary.SuccessfulMessages != 1 {
		t.Error("Summary from context should be the same instance")
	}

	// Test with context without summary
	emptyCtx := context.Background()
	nilSummary := SummaryFromContext(emptyCtx)
	if nilSummary != nil {
		t.Error("Should return nil for context without summary")
	}
}

// TestSummaryScenarios tests realistic Lambda execution scenarios
func TestSummaryScenarios(t *testing.T) {
	scenarios := []struct {
		name     string
		scenario func(*testing.T, *ExecutionSummary)
		verify   func(*testing.T, *ExecutionSummary)
	}{
		{
			name: "Single Message Success",
			scenario: func(t *testing.T, summary *ExecutionSummary) {
				summary.TotalMessages = 1
				summary.RecordCustomer("customer1")
				summary.RecordS3Download()
				summary.RecordApprovalRequest()
				summary.RecordEmailSent(5)
				summary.RecordS3Upload()
				summary.RecordSuccess()
			},
			verify: func(t *testing.T, summary *ExecutionSummary) {
				if summary.TotalMessages != 1 {
					t.Errorf("Expected TotalMessages=1, got %d", summary.TotalMessages)
				}
				if summary.SuccessfulMessages != 1 {
					t.Errorf("Expected SuccessfulMessages=1, got %d", summary.SuccessfulMessages)
				}
				if summary.S3Downloads != 1 {
					t.Errorf("Expected S3Downloads=1, got %d", summary.S3Downloads)
				}
				if summary.S3Uploads != 1 {
					t.Errorf("Expected S3Uploads=1, got %d", summary.S3Uploads)
				}
				if summary.ApprovalRequests != 1 {
					t.Errorf("Expected ApprovalRequests=1, got %d", summary.ApprovalRequests)
				}
				if summary.EmailsSent != 1 {
					t.Errorf("Expected EmailsSent=1, got %d", summary.EmailsSent)
				}
			},
		},
		{
			name: "Multiple Messages Mixed Success/Failure",
			scenario: func(t *testing.T, summary *ExecutionSummary) {
				summary.TotalMessages = 5

				// Message 1: Success
				summary.RecordCustomer("customer1")
				summary.RecordS3Download()
				summary.RecordApprovalRequest()
				summary.RecordEmailSent(5)
				summary.RecordS3Upload()
				summary.RecordSuccess()

				// Message 2: Success with meeting
				summary.RecordCustomer("customer2")
				summary.RecordS3Download()
				summary.RecordApprovedChange()
				summary.RecordMeetingScheduled(10)
				summary.RecordEmailSent(10)
				summary.RecordS3Upload()
				summary.RecordSuccess()

				// Message 3: Retryable error
				summary.RecordCustomer("customer3")
				summary.RecordS3Download()
				summary.RecordRetryableError(errors.New("temporary failure"))

				// Message 4: Non-retryable error
				summary.RecordNonRetryableError(errors.New("permanent failure"))

				// Message 5: Discarded backend event
				summary.RecordDiscardedEvent()
			},
			verify: func(t *testing.T, summary *ExecutionSummary) {
				if summary.TotalMessages != 5 {
					t.Errorf("Expected TotalMessages=5, got %d", summary.TotalMessages)
				}
				if summary.SuccessfulMessages != 2 {
					t.Errorf("Expected SuccessfulMessages=2, got %d", summary.SuccessfulMessages)
				}
				if summary.RetryableErrors != 1 {
					t.Errorf("Expected RetryableErrors=1, got %d", summary.RetryableErrors)
				}
				if summary.NonRetryableErrors != 1 {
					t.Errorf("Expected NonRetryableErrors=1, got %d", summary.NonRetryableErrors)
				}
				if summary.DiscardedEvents != 1 {
					t.Errorf("Expected DiscardedEvents=1, got %d", summary.DiscardedEvents)
				}
				if len(summary.CustomersProcessed) != 3 {
					t.Errorf("Expected 3 customers, got %d", len(summary.CustomersProcessed))
				}
				if summary.S3Downloads != 3 {
					t.Errorf("Expected S3Downloads=3, got %d", summary.S3Downloads)
				}
				if summary.S3Uploads != 2 {
					t.Errorf("Expected S3Uploads=2, got %d", summary.S3Uploads)
				}
				if summary.EmailsSent != 2 {
					t.Errorf("Expected EmailsSent=2, got %d", summary.EmailsSent)
				}
				if summary.MeetingsScheduled != 1 {
					t.Errorf("Expected MeetingsScheduled=1, got %d", summary.MeetingsScheduled)
				}
			},
		},
		{
			name: "Email Sending with Filtering",
			scenario: func(t *testing.T, summary *ExecutionSummary) {
				summary.TotalMessages = 1
				summary.RecordCustomer("customer1")
				summary.RecordS3Download()

				// Email with filtering: 20 recipients before, 15 filtered out, 5 sent
				summary.RecordEmailFiltering(20, 5, 15)
				summary.RecordEmailSent(5)

				summary.RecordS3Upload()
				summary.RecordSuccess()
			},
			verify: func(t *testing.T, summary *ExecutionSummary) {
				if summary.EmailsBeforeFilter != 20 {
					t.Errorf("Expected EmailsBeforeFilter=20, got %d", summary.EmailsBeforeFilter)
				}
				if summary.EmailsFiltered != 15 {
					t.Errorf("Expected EmailsFiltered=15, got %d", summary.EmailsFiltered)
				}
				if summary.EmailsSent != 1 {
					t.Errorf("Expected EmailsSent=1, got %d", summary.EmailsSent)
				}
			},
		},
		{
			name: "Meeting Scheduling with Attendee Filtering",
			scenario: func(t *testing.T, summary *ExecutionSummary) {
				summary.TotalMessages = 1
				summary.RecordCustomer("customer1")
				summary.RecordS3Download()

				// Meeting with attendee filtering: 30 total, 10 filtered, 3 manual, 23 final
				summary.RecordMeetingAttendeeFiltering(30, 10, 3, 23)
				summary.RecordMeetingScheduled(23)
				summary.RecordEmailSent(23)

				summary.RecordS3Upload()
				summary.RecordSuccess()
			},
			verify: func(t *testing.T, summary *ExecutionSummary) {
				if summary.TotalAttendees != 53 { // 30 from filtering + 23 from scheduled
					t.Errorf("Expected TotalAttendees=53, got %d", summary.TotalAttendees)
				}
				if summary.FilteredAttendees != 10 {
					t.Errorf("Expected FilteredAttendees=10, got %d", summary.FilteredAttendees)
				}
				if summary.ManualAttendees != 3 {
					t.Errorf("Expected ManualAttendees=3, got %d", summary.ManualAttendees)
				}
				if summary.MeetingsScheduled != 1 {
					t.Errorf("Expected MeetingsScheduled=1, got %d", summary.MeetingsScheduled)
				}
			},
		},
		{
			name: "S3 Operations with Errors",
			scenario: func(t *testing.T, summary *ExecutionSummary) {
				summary.TotalMessages = 3

				// Message 1: Successful S3 operations
				summary.RecordS3Download()
				summary.RecordS3Upload()
				summary.RecordSuccess()

				// Message 2: S3 download error
				summary.RecordS3Error()
				summary.RecordRetryableError(errors.New("S3 download failed"))

				// Message 3: S3 upload error
				summary.RecordS3Download()
				summary.RecordS3Error()
				summary.RecordRetryableError(errors.New("S3 upload failed"))
			},
			verify: func(t *testing.T, summary *ExecutionSummary) {
				if summary.S3Downloads != 2 {
					t.Errorf("Expected S3Downloads=2, got %d", summary.S3Downloads)
				}
				if summary.S3Uploads != 1 {
					t.Errorf("Expected S3Uploads=1, got %d", summary.S3Uploads)
				}
				if summary.S3Errors != 2 {
					t.Errorf("Expected S3Errors=2, got %d", summary.S3Errors)
				}
				if summary.RetryableErrors != 2 {
					t.Errorf("Expected RetryableErrors=2, got %d", summary.RetryableErrors)
				}
			},
		},
		{
			name: "Complete Change Workflow",
			scenario: func(t *testing.T, summary *ExecutionSummary) {
				summary.TotalMessages = 3

				// Message 1: Approval request
				summary.RecordCustomer("customer1")
				summary.RecordS3Download()
				summary.RecordApprovalRequest()
				summary.RecordEmailSent(5)
				summary.RecordS3Upload()
				summary.RecordSuccess()

				// Message 2: Approved with meeting
				summary.RecordCustomer("customer1")
				summary.RecordS3Download()
				summary.RecordApprovedChange()
				summary.RecordMeetingScheduled(10)
				summary.RecordEmailSent(10)
				summary.RecordS3Upload()
				summary.RecordSuccess()

				// Message 3: Completed
				summary.RecordCustomer("customer1")
				summary.RecordS3Download()
				summary.RecordCompletedChange()
				summary.RecordEmailSent(10)
				summary.RecordS3Upload()
				summary.RecordSuccess()
			},
			verify: func(t *testing.T, summary *ExecutionSummary) {
				if summary.TotalMessages != 3 {
					t.Errorf("Expected TotalMessages=3, got %d", summary.TotalMessages)
				}
				if summary.SuccessfulMessages != 3 {
					t.Errorf("Expected SuccessfulMessages=3, got %d", summary.SuccessfulMessages)
				}
				if len(summary.CustomersProcessed) != 1 {
					t.Errorf("Expected 1 customer, got %d", len(summary.CustomersProcessed))
				}
				if summary.ApprovalRequests != 1 {
					t.Errorf("Expected ApprovalRequests=1, got %d", summary.ApprovalRequests)
				}
				if summary.ApprovedChanges != 1 {
					t.Errorf("Expected ApprovedChanges=1, got %d", summary.ApprovedChanges)
				}
				if summary.CompletedChanges != 1 {
					t.Errorf("Expected CompletedChanges=1, got %d", summary.CompletedChanges)
				}
				if summary.MeetingsScheduled != 1 {
					t.Errorf("Expected MeetingsScheduled=1, got %d", summary.MeetingsScheduled)
				}
				if summary.EmailsSent != 3 {
					t.Errorf("Expected EmailsSent=3, got %d", summary.EmailsSent)
				}
				if summary.S3Downloads != 3 {
					t.Errorf("Expected S3Downloads=3, got %d", summary.S3Downloads)
				}
				if summary.S3Uploads != 3 {
					t.Errorf("Expected S3Uploads=3, got %d", summary.S3Uploads)
				}
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			summary := NewExecutionSummary()
			scenario.scenario(t, summary)
			scenario.verify(t, summary)
		})
	}
}
